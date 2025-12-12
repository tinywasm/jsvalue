//go:build wasm

package jsvalue

import (
	"syscall/js"
	"testing"
)

// Prevent compiler optimizations
var (
	resJS  js.Value
	resErr error
)

func BenchmarkToJS_Int(b *testing.B) {
	input := 12345
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resJS = ToJS(input)
	}
}

func BenchmarkToJS_String(b *testing.B) {
	input := "hello world"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resJS = ToJS(input)
	}
}

func BenchmarkToJS_Struct(b *testing.B) {
	input := TestStruct{Name: "Alice", Age: 30}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resJS = ToJS(input)
	}
}

func BenchmarkToGo_Int(b *testing.B) {
	val := ToJS(12345)
	var out int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resErr = ToGo(val, &out)
	}
}

func BenchmarkToGo_Struct(b *testing.B) {
	input := TestStruct{Name: "Alice", Age: 30}
	val := ToJS(input)
	var out TestStruct
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resErr = ToGo(val, &out)
	}
}

func BenchmarkToGo_Any_Int(b *testing.B) {
	val := ToJS(12345)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// New API requires dest
		var out any
		resErr = ToGo(val, &out)
	}
}

func BenchmarkToGo_Any_Map(b *testing.B) {
	input := map[string]any{"a": 1, "b": "c"}
	val := ToJS(input)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// New API
		var out any
		resErr = ToGo(val, &out)
	}
}

func BenchmarkToGo_Map_Reuse(b *testing.B) {
	input := map[string]any{"a": 1, "b": "c"}
	val := ToJS(input)
	var out map[string]any
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resErr = ToGo(val, &out)
	}
}
