package hmap

import (
	"hash/maphash"
	"maps"
	"sync"
)

const (
	defaultShardCount = 1 << 6 // 64
)

type mapItem[V any] struct {
	sync.RWMutex
	data map[string]V
	_    [32]byte // padding to avoid false sharing
}

// Map字段名首字母均小写，避免外部使用时跳过New()函数使用结构体创建对象
// 所有字段均为指针类型，避免外部修改结构体字段或数据复制导致数据不一致
type Map[V any] struct {
	datas []*mapItem[V]
	seed  *maphash.Seed
}

func trueShards(shardCount int) int {
	if shardCount < 1 {
		return 1
	}
	if shardCount > 1<<16 {
		return 1 << 16
	}
	// 向上取整到 2 的幂次
	n := shardCount - 1
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}

func New[V any](inShardCount ...int) *Map[V] {
	shardCount := defaultShardCount
	if len(inShardCount) > 0 {
		shardCount = trueShards(inShardCount[0])
	}

	seed := maphash.MakeSeed()
	m := &Map[V]{
		seed:  &seed,
		datas: make([]*mapItem[V], shardCount),
	}
	for i := range m.datas {
		m.datas[i] = &mapItem[V]{
			data: make(map[string]V),
		}
	}
	return m
}

func (m *Map[V]) getIndex(key string) int {
	hash := maphash.String(*m.seed, key)
	return int(hash & uint64(len(m.datas)-1))
}

func (m *Map[V]) Set(key string, value V) {
	index := m.getIndex(key)
	m.datas[index].Lock()
	defer m.datas[index].Unlock()
	m.datas[index].data[key] = value
}

func (m *Map[V]) SetWithNotExist(key string, value V) (V, bool) {
	index := m.getIndex(key)
	m.datas[index].Lock()
	defer m.datas[index].Unlock()
	if val, ok := m.datas[index].data[key]; ok {
		return val, false
	}
	m.datas[index].data[key] = value
	return value, true
}

func (m *Map[V]) Get(key string) (V, bool) {
	index := m.getIndex(key)
	m.datas[index].RLock()
	defer m.datas[index].RUnlock()
	value, ok := m.datas[index].data[key]
	return value, ok
}

func (m *Map[V]) GetWithDefault(key string, defaultValue V) (V, bool) {
	index := m.getIndex(key)
	m.datas[index].RLock()
	defer m.datas[index].RUnlock()
	value, ok := m.datas[index].data[key]
	if !ok {
		value = defaultValue
	}
	return value, ok
}

func (m *Map[V]) Delete(key string) bool {
	index := m.getIndex(key)

	m.datas[index].Lock()
	defer m.datas[index].Unlock()
	_, ok := m.datas[index].data[key]
	if !ok {
		return false
	}
	delete(m.datas[index].data, key)
	return true
}

// 为保证数据一致性，锁内执行delIf函数
// 在delIf函数中进行读取，修改等操作，可能导致key落在同一片分片导致死锁
func (m *Map[V]) DeleteIf(key string, delIf func(V) bool) bool {
	index := m.getIndex(key)

	m.datas[index].Lock()
	defer m.datas[index].Unlock()
	val, ok := m.datas[index].data[key]
	if !ok {
		return false
	}
	if !delIf(val) {
		return false
	}
	delete(m.datas[index].data, key)
	return true
}

func (m *Map[V]) Len() int {
	var count int
	for i := range m.datas {
		func() {
			m.datas[i].RLock()
			defer m.datas[i].RUnlock()
			count += len(m.datas[i].data)
		}()
	}
	return count
}

// 主要是用于内部调试，观察分片数据是否倾斜
func (m *Map[V]) LenWithSlice() []int {
	counts := make([]int, 0, len(m.datas))
	for i := range m.datas {
		func() {
			m.datas[i].RLock()
			defer m.datas[i].RUnlock()
			counts = append(counts, len(m.datas[i].data))
		}()
	}
	return counts
}

func (m *Map[V]) Clear() {
	for i := range m.datas {
		func() {
			m.datas[i].Lock()
			defer m.datas[i].Unlock()
			m.datas[i].data = make(map[string]V)
		}()
	}
}

func (m *Map[V]) Range(f func(key string, value V) bool) {
	for i := range m.datas {
		m.datas[i].RLock()
		tmpMaps := make(map[string]V, len(m.datas[i].data))
		maps.Copy(tmpMaps, m.datas[i].data)
		m.datas[i].RUnlock()
		for key, value := range tmpMaps {
			if !f(key, value) {
				return
			}
		}
	}
}

// Prune方法会在持有锁的情况下进行分片遍历
// 如果在 f 函数中进行读取，修改等操作，可能导致key落在同一片分片导致死锁
// f 函数返回值：
//
//	true ：删除该数据
//	false：保留该数据
//	error：停止清洗操作，返回错误
func (m *Map[V]) Prune(f func(key string, value V) (bool, error)) (int, int, error) {
	delNum := 0
	nowNum := 0
	for i := range m.datas {
		err := func() error {
			m.datas[i].Lock()
			defer m.datas[i].Unlock()
			for key, value := range m.datas[i].data {
				ok, err := f(key, value)
				if err != nil {
					return err
				}
				if ok {
					delNum++
					delete(m.datas[i].data, key)
				} else {
					nowNum++
				}
			}
			return nil
		}()
		if err != nil {
			return delNum, nowNum, err
		}
	}
	return delNum, nowNum, nil
}

func (m *Map[V]) ShardCount() int {
	return len(m.datas)
}
