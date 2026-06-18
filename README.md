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

Last updated: 2025-12-11 14:40:00 (Post-Reflect/Map Removal)

```text
goos: js
goarch: wasm
pkg: github.com/tinywasm/jsvalue
BenchmarkToJS_Int     	 3296695	       321.5 ns/op	       8 B/op	       1 allocs/op
BenchmarkToJS_String  	  166338	      6485 ns/op	      24 B/op	       2 allocs/op
BenchmarkToJS_Struct  	   52161	     25773 ns/op	      40 B/op	       4 allocs/op
BenchmarkToGo_Int     	10449840	        96.63 ns/op	       0 B/op	       0 allocs/op
BenchmarkToGo_Struct  	   98745	     11681 ns/op	      48 B/op	       5 allocs/op
BenchmarkToGo_Any_Int 	 3016056	       392.3 ns/op	      24 B/op	       2 allocs/op
```

> **Note:** Binary size reduced significantly by removing `reflect` (~72 KB reduction in WASM) and `map` runtime. Struct/slice conversion now requires implementing `fmt.Encodable`/`fmt.Decodable`.
