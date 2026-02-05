# hmap 使用文档

## 概述

`hmap` 是一个高性能的并发安全分片 Map 库，通过将数据分散到多个独立分片中，每个分片使用独立的读写锁，从而减少锁竞争，提升高并发场景下的读写性能。

## 环境要求

*   **Go 1.21+** (依赖标准库 `maps` 包)

## 核心特性

- **分片存储**：默认 64 个分片，支持自定义分片数（自动调整为 2 的幂次）
- **并发安全**：每个分片独立加锁，支持高并发读写
- **泛型支持**：支持任意值类型
- **零依赖**：仅使用 Go 标准库

## 注意点

Key 类型：目前仅支持 string 类型。如果需要其他类型的 Key，建议在外部转为 string 或修改源码中的 hash 计算部分

---

## 安装

```bash
go get your-module-path/hmap
```

---

## 快速开始

```go
package main

import (
    "fmt"
    "your-module-path/hmap"
)

func main() {
    // 创建默认 64 分片的 Map
    m := hmap.New[string]()

    // 设置值
    m.Set("name", "Alice")
    m.Set("city", "Beijing")

    // 获取值
    if value, ok := m.Get("name"); ok {
        fmt.Println("name:", value)
    }

    // 删除值
    m.Delete("city")

    // 获取长度
    fmt.Println("length:", m.Len())
}
```

---

## API 参考

### New

```go
func New[V any](inShardCount ...int) *Map[V]
```

创建一个新的分片 Map 实例。

**参数**：
- `inShardCount`（可选）：分片数量，默认为 64

**分片数调整规则**：
- 小于 1 → 调整为 1
- 大于 65536 → 调整为 65536
- 其他值 → 向上取整到最近的 2 的幂次

**示例**：

```go
// 使用默认 64 分片
m1 := hmap.New[string]()

// 指定 128 分片
m2 := hmap.New[int](128)

// 指定 100 分片，实际会调整为 128（向上取整到 2 的幂次）
m3 := hmap.New[bool](100)
```

---

### Set

```go
func (m *Map[V]) Set(key string, value V)
```

设置键值对，如果 key 已存在则覆盖原有值。

**示例**：

```go
m := hmap.New[int]()
m.Set("counter", 1)
m.Set("counter", 2)  // 覆盖为 2
```

---

### SetWithNotExist

```go
func (m *Map[V]) SetWithNotExist(key string, value V) (V, bool)
```

仅在 key 不存在时设置值（原子操作）。

**返回值**：
- 当 key **不存在**时：设置成功，返回 `(新设置的值, true)`
- 当 key **已存在**时：设置失败，返回 `(已存在的旧值, false)`

**示例**：

```go
m := hmap.New[string]()

// 首次设置，key 不存在
val, ok := m.SetWithNotExist("name", "Alice")
// val = "Alice", ok = true

// 再次设置，key 已存在
val, ok = m.SetWithNotExist("name", "Bob")
// val = "Alice"（返回旧值）, ok = false（设置失败）
```

---

### Get

```go
func (m *Map[V]) Get(key string) (V, bool)
```

获取指定 key 的值。

**返回值**：
- 当 key 存在时：返回 `(值, true)`
- 当 key 不存在时：返回 `(零值, false)`

**示例**：

```go
m := hmap.New[string]()
m.Set("name", "Alice")

value, ok := m.Get("name")     // value = "Alice", ok = true
value, ok = m.Get("notexist")  // value = "", ok = false
```

---

### GetWithDefault

```go
func (m *Map[V]) GetWithDefault(key string, defaultValue V) (V, bool)
```

获取指定 key 的值，如果不存在则返回默认值。

**返回值**：
- 当 key 存在时：返回 `(值, true)`
- 当 key 不存在时：返回 `(defaultValue, false)`

**注意**：此方法**不会**将默认值写入 Map。

**示例**：

```go
m := hmap.New[int]()
m.Set("age", 25)

val, ok := m.GetWithDefault("age", 0)       // val = 25, ok = true
val, ok = m.GetWithDefault("height", 170)   // val = 170, ok = false
```

---

### Delete

```go
func (m *Map[V]) Delete(key string) bool
```

删除指定 key。

**返回值**：
- `true`：key 存在且已删除
- `false`：key 不存在

**示例**：

```go
m := hmap.New[string]()
m.Set("name", "Alice")

ok := m.Delete("name")      // ok = true
ok = m.Delete("notexist")   // ok = false
```

---

### DeleteIf

```go
func (m *Map[V]) DeleteIf(key string, delIf func(V) bool) bool
```

条件删除：仅当 `delIf` 函数返回 `true` 时删除指定 key（原子操作）。

**参数**：
- `delIf`：判断函数，接收当前值，返回 `true` 表示执行删除

**返回值**：
- `true`：key 存在且满足条件并已删除
- `false`：key 不存在，或 `delIf` 返回 `false`

**示例**：

```go
m := hmap.New[int]()
m.Set("counter", 10)

// 仅当值大于 5 时删除
ok := m.DeleteIf("counter", func(v int) bool {
    return v > 5
})
// ok = true，已删除

m.Set("counter", 3)
ok = m.DeleteIf("counter", func(v int) bool {
    return v > 5
})
// ok = false，值为 3 不满足条件，未删除
```

> ⚠️ **警告**：参见 [注意事项 - 回调函数中的死锁风险](#回调函数中的死锁风险)

---

### Len

```go
func (m *Map[V]) Len() int
```

返回 Map 中的键值对总数。

**示例**：

```go
m := hmap.New[string]()
m.Set("a", "1")
m.Set("b", "2")
fmt.Println(m.Len())  // 输出: 2
```

> ⚠️ **注意**：参见 [注意事项 - 统计类方法的数据一致性](#统计类方法的数据一致性)

---

### LenWithSlice

```go
func (m *Map[V]) LenWithSlice() []int
```

返回每个分片中的键值对数量，用于调试和观察数据分布是否均匀。

**示例**：

```go
m := hmap.New[string](4)  // 4 个分片
m.Set("a", "1")
m.Set("b", "2")
m.Set("c", "3")

counts := m.LenWithSlice()
fmt.Println(counts)  // 例如: [1, 0, 2, 0]
```

> ⚠️ **注意**：参见 [注意事项 - 统计类方法的数据一致性](#统计类方法的数据一致性)

---

### Clear

```go
func (m *Map[V]) Clear()
```

清空 Map 中的所有数据。

**示例**：

```go
m := hmap.New[string]()
m.Set("a", "1")
m.Set("b", "2")
m.Clear()
fmt.Println(m.Len())  // 输出: 0
```

> ⚠️ **注意**：参见 [注意事项 - 统计类方法的数据一致性](#统计类方法的数据一致性)

---

### Range

```go
func (m *Map[V]) Range(f func(key string, value V) bool)
```

遍历 Map 中的所有键值对。

**参数**：
- `f`：遍历回调函数，返回 `false` 时提前终止遍历

**示例**：

```go
m := hmap.New[int]()
m.Set("a", 1)
m.Set("b", 2)
m.Set("c", 3)

m.Range(func(key string, value int) bool {
    fmt.Printf("%s: %d\n", key, value)
    return true  // 继续遍历
})

// 提前终止
m.Range(func(key string, value int) bool {
    if value > 1 {
        return false  // 停止遍历
    }
    fmt.Println(key)
    return true
})
```

> ⚠️ **注意**：参见 [注意事项 - Range 的弱一致性与内存开销](#range-的弱一致性与内存开销)

---

### Prune

```go
func (m *Map[V]) Prune(f func(key string, value V) (bool, error)) (int, int, error)
```

批量条件清理：遍历所有数据，根据条件删除符合要求的键值对。

**参数**：
- `f`：判断函数，对每个键值对调用
  - 返回 `(true, nil)`：删除该键值对
  - 返回 `(false, nil)`：保留该键值对
  - 返回 `(_, error)`：立即停止清理，返回错误

**返回值**：
- `delNum`：已删除的数量
- `nowNum`：保留的数量
- `error`：`f` 函数返回的错误（如有）

**示例**：

```go
m := hmap.New[int]()
m.Set("a", 1)
m.Set("b", 5)
m.Set("c", 10)
m.Set("d", 15)

// 删除所有值小于 10 的键值对
deleted, remaining, err := m.Prune(func(key string, value int) (bool, error) {
    return value < 10, nil
})

fmt.Printf("删除: %d, 保留: %d, 错误: %v\n", deleted, remaining, err)
// 输出: 删除: 2, 保留: 2, 错误: <nil>
```

> ⚠️ **警告**：参见 [注意事项 - 回调函数中的死锁风险](#回调函数中的死锁风险)

---

### ShardCount

```go
func (m *Map[V]) ShardCount() int
```

返回 Map 的分片数量。

**示例**：

```go
m := hmap.New[string](100)
fmt.Println(m.ShardCount())  // 输出: 128（100 向上取整到 2 的幂次）
```

---

## 重要注意事项

### 必须使用 New() 创建实例

`Map` 结构体的零值**不可用**，直接声明使用会导致 panic：

```go
// ❌ 错误：会导致 nil panic
var m hmap.Map[string]
m.Set("key", "value")  // panic!

// ❌ 错误：同样会 panic
m := hmap.Map[string]{}
m.Set("key", "value")  // panic!

// ✅ 正确：使用 New() 创建
m := hmap.New[string]()
m.Set("key", "value")
```

---

### 引用类型值的修改

当值类型 `V` 是指针或包含引用的类型（如 `*struct`、`[]T`、`map[K]V` 等）时，通过 `Get()` 获取的值与 Map 内部存储的是同一引用。直接修改会影响 Map 中的数据，且**绕过了锁保护**。

```go
type User struct {
    Name string
}

m := hmap.New[*User]()
m.Set("alice", &User{Name: "Alice"})

// 获取后修改会直接影响 Map 中的数据
user, _ := m.Get("alice")
user.Name = "Bob"  // ⚠️ Map 中的数据也被修改了！

// 此时再次获取
user2, _ := m.Get("alice")
fmt.Println(user2.Name)  // 输出: Bob
```

**建议**：
- 如需修改，考虑使用 `Set()` 重新设置完整的值
- 如需并发安全地修改值内部字段，需要在值类型内部实现额外的同步机制

---

### 统计类方法的数据一致性

以下方法在高并发场景下可能返回不精确的结果：

| 方法 | 说明 |
|------|------|
| `Len()` | 逐个分片统计，遍历过程中其他分片可能被修改 |
| `LenWithSlice()` | 同上 |
| `Clear()` | 逐个分片清空，不保证瞬时一致性 |

这是为了**避免全局锁**导致的性能瓶颈而做出的设计权衡。如果业务需要精确值，请在外部实现额外的同步机制。

---

### Range 的弱一致性与内存开销

`Range()` 方法采用**分片快照**策略：

1. 对每个分片加读锁，复制该分片数据到临时 map，解锁
2. 遍历临时 map，调用回调函数
3. 重复处理下一个分片

**一致性说明**：
- 遍历期间对**已处理分片**的修改不会体现在当前遍历中
- 遍历期间对**未处理分片**的修改可能会/可能不会体现
- 这是为了避免长时间持有锁导致阻塞而做出的妥协

**内存开销**：
- 每个分片会创建完整的数据副本
- 数据量较大时会产生显著的内存开销
- 建议在数据量可控的场景下使用，或分批处理

---

### 回调函数中的死锁风险

`DeleteIf()` 和 `Prune()` 方法会在**持有分片锁**的情况下调用回调函数。

如果在回调函数中对 Map 进行读写操作，且操作的 key 恰好落在**当前正在处理的分片**，将导致**死锁**：

```go
m := hmap.New[int]()
m.Set("a", 1)
m.Set("b", 2)

// ❌ 危险：可能死锁
m.Prune(func(key string, value int) (bool, error) {
    // 如果 "other_key" 与当前 key 在同一分片，将死锁
    m.Set("other_key", 100)
    return false, nil
})

// ❌ 危险：可能死锁
m.DeleteIf("a", func(v int) bool {
    // 如果 "b" 与 "a" 在同一分片，将死锁
    _, _ = m.Get("b")
    return true
})
```

**建议**：
- 回调函数中**不要**对同一个 Map 实例进行任何读写操作
- 如确需操作，将待操作的 key 收集起来，在回调结束后统一处理

---

## 设计说明

### 为什么分片数必须是 2 的幂次？

分片索引计算使用位运算 `hash & (len-1)` 代替取模运算 `hash % len`，位运算性能更高。这要求分片数必须是 2 的幂次。

### 分片数选择建议

| 场景 | 建议分片数 |
|------|-----------|
| 小规模数据（< 1万） | 16 - 32 |
| 中等规模（1万 - 100万） | 64（默认）- 256 |
| 大规模数据（> 100万） | 256 - 1024 |
| 极高并发写入 | 根据 CPU 核心数和并发度调整 |

分片数过少会增加锁竞争，过多会增加内存开销。可使用 `LenWithSlice()` 观察数据分布是否均匀，以辅助调优。

---

## 完整示例

```go
package main

import (
    "fmt"
    "sync"
    "your-module-path/hmap"
)

func main() {
    // 创建 256 分片的 Map
    m := hmap.New[int](256)

    // 并发写入
    var wg sync.WaitGroup
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            key := fmt.Sprintf("key_%d", n)
            m.Set(key, n)
        }(i)
    }
    wg.Wait()

    fmt.Printf("总数: %d\n", m.Len())
    fmt.Printf("分片数: %d\n", m.ShardCount())

    // 条件清理：删除值小于 500 的数据
    deleted, remaining, _ := m.Prune(func(key string, value int) (bool, error) {
        return value < 500, nil
    })
    fmt.Printf("删除: %d, 保留: %d\n", deleted, remaining)

    // 遍历
    count := 0
    m.Range(func(key string, value int) bool {
        count++
        return count < 5  // 只打印前 5 个
    })
}
```