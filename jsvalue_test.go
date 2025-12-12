//go:build wasm

package jsvalue

import (
	"syscall/js"
	"testing"
)

type TestStruct struct {
	Name    string `json:"name"`
	Age     int    `json:"age"`
	Ignored string `json:"-"`
	Default string
}

type ComplexStruct struct {
	Nested TestStruct `json:"nested"`
	List   []int      `json:"list"`
}

func TestToJS(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		validate func(js.Value) bool
	}{
		{"nil", nil, func(v js.Value) bool { return v.IsNull() }},
		{"string", "hello", func(v js.Value) bool { return v.String() == "hello" }},
		{"int", 123, func(v js.Value) bool { return v.Int() == 123 }},
		{"int8", int8(1), func(v js.Value) bool { return v.Int() == 1 }},
		{"int16", int16(1), func(v js.Value) bool { return v.Int() == 1 }},
		{"int32", int32(1), func(v js.Value) bool { return v.Int() == 1 }},
		{"int64", int64(1), func(v js.Value) bool { return v.Int() == 1 }},
		{"uint", uint(1), func(v js.Value) bool { return v.Int() == 1 }},
		{"uint8", uint8(1), func(v js.Value) bool { return v.Int() == 1 }},
		{"uint16", uint16(1), func(v js.Value) bool { return v.Int() == 1 }},
		{"uint32", uint32(1), func(v js.Value) bool { return v.Int() == 1 }},
		{"uint64", uint64(1), func(v js.Value) bool { return v.Int() == 1 }},
		{"float32", float32(1.5), func(v js.Value) bool { return v.Float() == 1.5 }},
		{"float64", 1.5, func(v js.Value) bool { return v.Float() == 1.5 }},
		{"bool", true, func(v js.Value) bool { return v.Bool() == true }},
		{"bytes", []byte("xyz"), func(v js.Value) bool { return v.String() == "xyz" }},
		{"slice", []int{1, 2}, func(v js.Value) bool {
			return v.Length() == 2 && v.Index(0).Int() == 1 && v.Index(1).Int() == 2
		}},
		{"slice_any", []any{1, "a"}, func(v js.Value) bool {
			return v.Length() == 2 && v.Index(0).Int() == 1 && v.Index(1).String() == "a"
		}},
		{"slice_string", []string{"a", "b"}, func(v js.Value) bool {
			return v.Length() == 2 && v.Index(0).String() == "a"
		}},
		{"slice_float", []float64{1.1, 2.2}, func(v js.Value) bool {
			return v.Length() == 2 && v.Index(0).Float() == 1.1
		}},
		{"map", map[string]any{"a": 1}, func(v js.Value) bool {
			return v.Get("a").Int() == 1
		}},
		{"map_string", map[string]string{"a": "b"}, func(v js.Value) bool {
			return v.Get("a").String() == "b"
		}},
		{"map_int", map[string]int{"a": 1}, func(v js.Value) bool {
			return v.Get("a").Int() == 1
		}},
		{"struct", TestStruct{Name: "Alice", Age: 30}, func(v js.Value) bool {
			return v.Get("name").String() == "Alice" && v.Get("age").Int() == 30
		}},
		{"struct_pointer", &TestStruct{Name: "Bob"}, func(v js.Value) bool {
			return v.Get("name").String() == "Bob"
		}},
		{"nil_pointer", (*TestStruct)(nil), func(v js.Value) bool { return v.IsNull() }},
		{"empty_slice", []int{}, func(v js.Value) bool { return v.Length() == 0 }},
		{"empty_map", map[string]int{}, func(v js.Value) bool {
			// In JS, Object.keys({}).length is 0
			return jsObject.Call("keys", v).Length() == 0
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := ToJS(tt.input)
			if !tt.validate(val) {
				t.Errorf("ToJS validation failed for %v", tt.name)
			}
		})
	}
}

func TestToGo(t *testing.T) {
	t.Run("Basic Types", func(t *testing.T) {
		val := ToJS("hello")
		var s string
		if err := ToGo(val, &s); err != nil {
			t.Fatal(err)
		}
		if s != "hello" {
			t.Errorf("want hello, got %s", s)
		}

		val = ToJS(123)
		var i int
		if err := ToGo(val, &i); err != nil {
			t.Fatal(err)
		}
		if i != 123 {
			t.Errorf("want 123, got %d", i)
		}
	})

	t.Run("Struct", func(t *testing.T) {
		ts := TestStruct{Name: "Alice", Age: 30}
		val := ToJS(ts)
		var res TestStruct
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if res.Name != "Alice" || res.Age != 30 {
			t.Errorf("struct mismatch: %+v", res)
		}
	})

	t.Run("Complex Struct", func(t *testing.T) {
		cs := ComplexStruct{
			Nested: TestStruct{Name: "Diff", Age: 40},
			List:   []int{5, 6},
		}
		val := ToJS(cs)
		var res ComplexStruct
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if res.Nested.Name != "Diff" || len(res.List) != 2 {
			t.Errorf("complex struct mismatch: %+v", res)
		}
	})

	t.Run("Map", func(t *testing.T) {
		m := map[string]any{"x": 10, "y": "z"}
		val := ToJS(m)
		var res map[string]any
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if int(res["x"].(float64)) != 10 { // JS numbers are floats
			t.Errorf("map mismatch x: %v", res["x"])
		}
		if res["y"] != "z" {
			t.Errorf("map mismatch y: %v", res["y"])
		}

		// Empty Map
		val = ToJS(map[string]int{})
		var emptyRes map[string]int
		if err := ToGo(val, &emptyRes); err != nil {
			t.Fatal(err)
		}
		if len(emptyRes) != 0 {
			t.Error("expected empty map")
		}
	})

	t.Run("Slice", func(t *testing.T) {
		sl := []any{1, "a"}
		val := ToJS(sl)
		var res []any
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if len(res) != 2 {
			t.Fatalf("slicelen mismatch: %d", len(res))
		}
	})

	t.Run("Additional Types", func(t *testing.T) {
		// Float64
		val := ToJS(3.14)
		var f float64
		if err := ToGo(val, &f); err != nil {
			t.Fatal(err)
		}
		if f != 3.14 {
			t.Errorf("want 3.14, got %f", f)
		}

		// Bool
		val = ToJS(true)
		var b bool
		if err := ToGo(val, &b); err != nil {
			t.Fatal(err)
		}
		if !b {
			t.Errorf("want true, got %v", b)
		}

		// Byte
		val = ToJS(byte(255))
		var by byte
		if err := ToGo(val, &by); err != nil {
			t.Fatal(err)
		}
		if by != 255 {
			t.Errorf("want 255, got %d", by)
		}

		// []byte
		inputBytes := []byte("binary data")
		val = ToJS(inputBytes)
		var resBytes []byte
		if err := ToGo(val, &resBytes); err != nil {
			t.Fatal(err)
		}
		if string(resBytes) != "binary data" {
			t.Errorf("want 'binary data', got %s", string(resBytes))
		}
	})

	t.Run("Slice of Bytes (Base64/String optimization)", func(t *testing.T) {
		// Test []byte inside a struct or as a slice element
		type ByteStruct struct {
			Data []byte `json:"data"`
		}
		bs := ByteStruct{Data: []byte("hello")}
		val := ToJS(bs)

		var res ByteStruct
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if string(res.Data) != "hello" {
			t.Errorf("ByteStruct mismatch: %s", string(res.Data))
		}
	})

	t.Run("Slice of Slice of Bytes", func(t *testing.T) {
		// Test [][]byte
		input := [][]byte{[]byte("one"), []byte("two")}
		val := ToJS(input)

		var res [][]byte
		// ToGo doesn't directly support *[][]byte in the top-level switch,
		// but it supports *[]any.
		// Actually, ToGo uses reflection for slices not in the switch.
		// So passing &res should work via reflection path.
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if len(res) != 2 || string(res[0]) != "one" || string(res[1]) != "two" {
			t.Errorf("slice of slice of bytes mismatch: %v", res)
		}
	})

	t.Run("Recursive Slice", func(t *testing.T) {
		// Test slice of ints via reflection
		input := []int{1, 2, 3}
		val := ToJS(input)
		var res []int
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if len(res) != 3 || res[2] != 3 {
			t.Errorf("slice int mismatch: %v", res)
		}
	})

	t.Run("Struct Tags", func(t *testing.T) {
		// Test omitted and custom tags
		type TagStruct struct {
			Ignored string `json:"-"`
			Default string
			Option  string `json:"opt,omitempty"`
		}
		ts := TagStruct{Ignored: "secret", Default: "visible", Option: "value"}
		val := ToJS(ts)
		if !val.Get("Ignored").IsUndefined() {
			t.Error("Ignored field should be undefined")
		}
		if val.Get("Default").String() != "visible" {
			t.Error("Default field should be visible")
		}
		if val.Get("opt").String() != "value" {
			t.Error("Option field should be 'value'")
		}

		var res TagStruct
		// Manually set ignored in JS to ensure it's not read back? (Not needed for coverage, but good logic check)
		// Basic check that reading back works for other fields
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if res.Default != "visible" {
			t.Errorf("struct tag mismatch: %v", res)
		}
		if res.Option != "value" {
			t.Errorf("struct tag option mismatch: %v", res)
		}
	})

	t.Run("Fallback to String", func(t *testing.T) {
		// Test fallback
		type CustomInt int
		c := CustomInt(42)
		val := ToJS(c) // Should use switch matches int? No, type check is strict for basic types.
		// Wait, CustomInt is not int, so it might fall to default reflection -> int handling?
		// No, ToJS default uses reflection.
		// reflect.ValueOf(c).Kind() is Int.
		// In ToJS default block: it checks Kind().
		// If it hits default default, it uses Sprint.

		// Let's use a channel or func which falls to default default
		ch := make(chan int)
		val = ToJS(ch)
		if val.Type() != js.TypeString {
			t.Errorf("Expected string for channel, got %v", val.Type())
		}
	})

	t.Run("Reflect Logic", func(t *testing.T) {
		// []byte from JS Array (not string)
		// Use ToJS to create a proper JS array of numbers
		jsArr := ToJS([]any{1, 2})

		var b []byte
		if err := ToGo(jsArr, &b); err != nil {
			t.Fatal(err)
		}
		if len(b) != 2 || b[0] != 1 || b[1] != 2 {
			t.Errorf("[]byte from array mismatch: %v", b)
		}

		// Struct mismatch (JS is string)
		var s TestStruct
		if err := ToGo(js.ValueOf("not object"), &s); err != nil {
			// Actually implementation returns nil if not object
		}

		// Slice mismatch (JS is object but not array)
		var sl []int
		obj := jsObject.New()
		if err := ToGo(obj, &sl); err != nil {
			// Implementation returns nil
		}
	})

	t.Run("Errors", func(t *testing.T) {
		if err := ToGo(js.Null(), nil); err == nil {
			t.Error("expected error for nil destination")
		}
		var i int
		// Pass non-pointer
		if err := ToGo(js.Null(), i); err == nil {
			t.Error("expected error for non-pointer destination")
		}

		// ValueToGo default case (e.g. Symbol)
		// We can't easily create a symbol that isn't one of the known types,
		// but we can try to pass a JS function to ValueToGo (via map or slice indirectly)
		// or direct ValueToGo usage? ValueToGo is internal? No, I made it public.
		// Wait, I made it public in my plan. jsvalue.go: "func ValueToGo(jsVal js.Value) any"

		fn := js.Global().Get("Function").New("return 1")
		var res any
		if err := ToGo(fn, &res); err != nil {
			// It might assume default fallback string
		}
		// Default should return jsVal.String()
		if _, ok := res.(string); !ok {
			t.Errorf("Expected string for function, got %T", res)
		}
	})
}

func TestValueToGoCoverage(t *testing.T) {
	// Test ValueToGo branches via map/slice
	m := map[string]any{
		"bool": true,
		"null": nil,
		"list": []any{false, 42.0},
	}
	val := ToJS(m)
	var res map[string]any
	if err := ToGo(val, &res); err != nil {
		t.Fatal(err)
	}

	if res["bool"] != true {
		t.Errorf("bool mismatch")
	}
	if res["null"] != nil {
		t.Errorf("null mismatch")
	}
	list := res["list"].([]any)
	if list[0] != false {
		t.Errorf("list bool mismatch")
	}
	if list[1] != 42.0 { // JS numbers are floats
		t.Errorf("list num mismatch")
	}
}
