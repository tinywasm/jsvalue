# jsvalue
<img src="docs/img/badges.svg">
Efficient conversions between JavaScript and Go for a WebAssembly environment with support for TinyGo.

## Usage

### Importing

```go
import "github.com/tinywasm/jsvalue"
```

### ToJS

Converts Go values to `syscall/js.Value`. For structs and slices, they must implement `fmt.Encodable`.

```go
// Basic types
val := jsvalue.ToJS(123)
val := jsvalue.ToJS("hello")

// Encodable types (0-alloc on Go side)
user := &User{Name: "Alice"}
jsVal := jsvalue.ToJS(user)
```

### ToGo

Converts `syscall/js.Value` to Go values, populating a pointer destination. For structs and slices, they must implement `fmt.Decodable`.

```go
// Integers (Zero allocation)
var n int
err := jsvalue.ToGo(jsVal, &n)

// Structs (implementing fmt.Decodable)
var user User
err := jsvalue.ToGo(jsObj, &user)
```

### ToAny

Converts a JS value to a Go `any`.
- Integer numbers are returned as `int64`.
- Non-integer numbers as `float64`.
- For objects, it returns the raw `js.Value` (to avoid `map` runtime overhead).
- Arrays are returned as `[]any`.

## Performance Results

Last updated: 2026-06-18 (Post-Reflect/Map Removal — codec migration)

### Binary size (wasm, `-opt=z -no-debug`)

| | Before (reflect+map) | After (codec) | Delta |
|---|---|---|---|
| `jsvalue` consumer binary | ~72 KB reflect tables + map runtime | reflect removed | **~−72 KB** |

### Benchmarks (goos: js, goarch: wasm — measured 2026-06-18)

```text
pkg: github.com/tinywasm/jsvalue
BenchmarkToJS_Int     	143221300	        16.80 ns/op	       0 B/op	       0 allocs/op
BenchmarkToJS_String  	 1000000	      2029 ns/op	       8 B/op	       1 allocs/op
BenchmarkToJS_Struct  	  313754	      8153 ns/op	      40 B/op	       4 allocs/op
BenchmarkToGo_Int     	153262053	        15.57 ns/op	       0 B/op	       0 allocs/op
BenchmarkToGo_Struct  	  404035	      5862 ns/op	      40 B/op	       4 allocs/op
BenchmarkToGo_Any_Int 	14609425	       163.1 ns/op	      24 B/op	       2 allocs/op
```

Note: `allocs/op` reflects JS bridge allocations (`syscall/js` boxes each value passed to
`js.Value.Set/Get` as `interface{}`). This is inherent to the Go↔JS bridge and cannot be
eliminated. There is no `reflect` in the Go-side codec path.

> **Before (reflect-based):** struct conversion used `reflect.ValueOf` per call — each `ToJS`/`ToGo`
> on a struct triggered reflect type lookup + value iteration (~3× slower, plus ~72 KB of reflect
> type tables in the wasm binary). Removed along with all `map[string]*` cases. Struct/slice
> conversion now requires implementing `fmt.Encodable`/`fmt.Decodable`.
