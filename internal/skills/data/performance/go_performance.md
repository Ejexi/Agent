---
name: go-performance
description: Go-specific performance patterns — goroutine leaks, memory allocation, profiling, sync primitives
---

# Go Performance Patterns

## Goroutine Leaks

The most common Go performance issue. Goroutines blocked forever consume ~8KB stack each.

**Detection:**
```go
// Runtime check
runtime.NumGoroutine()  // should stabilise, not grow

// pprof goroutine profile
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

**Common causes:**
```go
// ❌ Channel send with no receiver (blocks forever)
go func() {
    ch <- result  // if caller returned, this leaks
}()

// ✅ Always use context cancellation
go func() {
    select {
    case ch <- result:
    case <-ctx.Done():
    }
}()

// ❌ Ticker not stopped
t := time.NewTicker(time.Second)
go func() {
    for range t.C { ... }
}()

// ✅ Always defer Stop()
t := time.NewTicker(time.Second)
defer t.Stop()
```

## Memory Allocation Hotspots

```go
// ❌ String concatenation in loop (O(n²) allocations)
var s string
for _, v := range items {
    s += v  // new allocation each iteration
}

// ✅ strings.Builder
var sb strings.Builder
sb.Grow(estimatedSize)  // pre-allocate
for _, v := range items {
    sb.WriteString(v)
}

// ❌ Append without pre-allocation
var results []Finding
for _, item := range items {
    results = append(results, parse(item))
}

// ✅ Pre-allocate
results := make([]Finding, 0, len(items))

// ❌ []byte → string conversion copies
s := string(b)  // copy

// ✅ Avoid conversion in hot path
// use bytes.Equal, bytes.Contains instead
```

## sync.Mutex vs sync.RWMutex

```go
// Use RWMutex when reads >> writes
type Cache struct {
    mu   sync.RWMutex      // not sync.Mutex
    data map[string][]byte
}

func (c *Cache) Get(k string) []byte {
    c.mu.RLock()           // concurrent readers allowed
    defer c.mu.RUnlock()
    return c.data[k]
}

func (c *Cache) Set(k string, v []byte) {
    c.mu.Lock()            // exclusive write
    defer c.mu.Unlock()
    c.data[k] = v
}
```

## sync.Pool for Frequent Allocations

```go
var bufPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 4096)
    },
}

func process() {
    buf := bufPool.Get().([]byte)
    buf = buf[:0]  // reset length, keep capacity
    defer bufPool.Put(buf)
    // use buf...
}
```

## Context Propagation

Always propagate context — enables cancellation and deadline across goroutine boundaries:

```go
// ❌ Background context ignores cancellation
go func() {
    result, err := llm.Generate(context.Background(), msgs, opts)
}()

// ✅ Propagate the caller's context
go func() {
    result, err := llm.Generate(ctx, msgs, opts)
}()
```

## Profiling Commands

```bash
# CPU profile
go tool pprof -http :8080 cpu.prof

# Memory profile  
go tool pprof -http :8080 mem.prof

# Benchmark with profiling
go test -bench=. -cpuprofile cpu.prof -memprofile mem.prof ./...

# Escape analysis (see what allocates on heap)
go build -gcflags="-m" ./...

# Race detector (always run in CI)
go test -race ./...
```
