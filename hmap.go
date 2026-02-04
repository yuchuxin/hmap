package hmap

import (
	"hash/maphash"
	"maps"
	"sync"
)

const (
	defaultShardCount = 1 << 5 // 32
)

type MapItem[V any] struct {
	mut  *sync.RWMutex
	maps map[string]V
}

type Map[V any] struct {
	maps []MapItem[V]
	seed maphash.Seed
	pool sync.Pool
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

	m := &Map[V]{
		seed: maphash.MakeSeed(),
		maps: make([]MapItem[V], shardCount),
		pool: sync.Pool{
			New: func() any {
				return new(maphash.Hash)
			}},
	}
	for i := range m.maps {
		m.maps[i] = MapItem[V]{
			maps: make(map[string]V),
			mut:  &sync.RWMutex{},
		}
	}
	return m
}

func (m *Map[V]) getIndex(key string) int {
	h := m.pool.Get().(*maphash.Hash)
	defer m.pool.Put(h)
	h.Reset()
	h.SetSeed(m.seed)
	h.WriteString(key)
	return int(h.Sum64() & uint64(len(m.maps)-1))
}

func (m *Map[V]) Set(key string, value V, onlyIfNotExist ...bool) V {
	ifNotExist := false
	if len(onlyIfNotExist) > 0 {
		ifNotExist = onlyIfNotExist[0]
	}
	index := m.getIndex(key)
	if ifNotExist {
		val, ok := func() (V, bool) {
			m.maps[index].mut.RLock()
			defer m.maps[index].mut.RUnlock()
			val, ok := m.maps[index].maps[key]
			return val, ok
		}()
		if ok {
			return val
		}
	}

	m.maps[index].mut.Lock()
	defer m.maps[index].mut.Unlock()
	if ifNotExist {
		if val, ok := m.maps[index].maps[key]; ok {
			return val
		}
	}
	m.maps[index].maps[key] = value
	return value
}

func (m *Map[V]) Get(key string, defaultValue ...V) (V, bool) {
	index := m.getIndex(key)
	m.maps[index].mut.RLock()
	defer m.maps[index].mut.RUnlock()
	value, ok := m.maps[index].maps[key]
	if !ok && len(defaultValue) > 0 {
		value = defaultValue[0]
	}
	return value, ok
}

func (m *Map[V]) Delete(key string) bool {
	index := m.getIndex(key)
	m.maps[index].mut.Lock()
	defer m.maps[index].mut.Unlock()
	_, ok := m.maps[index].maps[key]
	if ok {
		delete(m.maps[index].maps, key)
	}
	return ok
}

func (m *Map[V]) Len() int {
	var count int
	for i := range m.maps {
		func() {
			m.maps[i].mut.RLock()
			defer m.maps[i].mut.RUnlock()
			count += len(m.maps[i].maps)
		}()
	}
	return count
}

func (m *Map[V]) GetMaps(index int) map[string]V {
	if index < 0 || index >= len(m.maps) {
		return nil
	}
	m.maps[index].mut.RLock()
	defer m.maps[index].mut.RUnlock()
	result := make(map[string]V, len(m.maps[index].maps))
	maps.Copy(result, m.maps[index].maps)
	return result
}

func (m *Map[V]) GetAllMaps() map[string]V {
	re := map[string]V{}
	for i := range m.maps {
		func() {
			m.maps[i].mut.RLock()
			defer m.maps[i].mut.RUnlock()
			maps.Copy(re, m.maps[i].maps)
		}()
	}
	return re
}

func (m *Map[V]) Clear() {
	for i := range m.maps {
		func() {
			m.maps[i].mut.Lock()
			defer m.maps[i].mut.Unlock()
			m.maps[i].maps = make(map[string]V)
		}()
	}
}

func (m *Map[V]) Range(f func(key string, value V) bool) {
	for i := range m.maps {
		m.maps[i].mut.RLock()
		tmpMaps := make(map[string]V, len(m.maps[i].maps))
		maps.Copy(tmpMaps, m.maps[i].maps)
		m.maps[i].mut.RUnlock()
		for key, value := range tmpMaps {
			if !f(key, value) {
				return
			}
		}
	}
}

func (m *Map[V]) ShardCount() int {
	return len(m.maps)
}
