> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan: jsvalue — Add ScanValue and ToAny (typed, no reflect)

## Context

`tinywasm/jsvalue` (module `github.com/tinywasm/jsvalue`) already has `ToJS` and `ToGo`.
`ToGo` uses `reflect` — correct for generic struct decoding but adds binary bloat in WASM.

`tinywasm/goflare/d1` (`rows_wasm.go`) reimplements two functions locally:
- `scanValue(v js.Value, dest any) error` — copies a JS value into a typed Go pointer
- `jsValueToAny(v js.Value) any` — converts a JS value to `any` with integer detection

These must be centralized in jsvalue as **exported** functions using a **type switch (no reflect)**,
so d1 (and future packages like indexdb) can import them directly.

Tests run via `gotest` — no TinyGo installation required.

## TinyWasm Constraints (mandatory)

- No `import "errors"`, `"fmt"`, `"strings"` from stdlib — use `github.com/tinywasm/fmt`.
- Files using `syscall/js` must have `//go:build wasm` as first line.
- **No `reflect`** in the new functions — type switch only. This is critical for WASM binary size.

## Public API to Add

```go
// ScanValue copies the JS value v into the Go pointer dest.
// Supports: *string, *int, *int64, *int32, *float64, *bool, *[]byte, *any.
// []byte is read from a Uint8Array using js.CopyBytesToGo.
// Returns an error for unsupported dest types.
func ScanValue(v js.Value, dest any) error

// ToAny converts a JS value to a Go any.
// Numbers that are exact integers are returned as int64; others as float64.
// null/undefined → nil. Objects and arrays → string fallback.
func ToAny(v js.Value) any
```

## Implementation

Add to **`jsvalue.go`** (existing file, already has `//go:build wasm`):

```go
// ScanValue copies the JS value v into the Go pointer dest.
// Supports *string, *int, *int64, *int32, *float64, *bool, *[]byte, *any.
// []byte is read from a Uint8Array via js.CopyBytesToGo.
func ScanValue(v js.Value, dest any) error {
	switch p := dest.(type) {
	case *string:
		*p = v.String()
	case *int:
		*p = v.Int()
	case *int64:
		*p = int64(v.Float())
	case *int32:
		*p = int32(v.Int())
	case *float64:
		*p = v.Float()
	case *bool:
		*p = v.Bool()
	case *[]byte:
		ua := Uint8ArrayClass.New(v)
		b := make([]byte, ua.Length())
		js.CopyBytesToGo(b, ua)
		*p = b
	case *any:
		*p = ToAny(v)
	default:
		return Errf("jsvalue: unsupported scan type: %T", dest)
	}
	return nil
}

// ToAny converts a JS value to a Go any.
// Integer numbers are returned as int64; non-integer numbers as float64.
func ToAny(v js.Value) any {
	switch v.Type() {
	case js.TypeNull, js.TypeUndefined:
		return nil
	case js.TypeBoolean:
		return v.Bool()
	case js.TypeNumber:
		f := v.Float()
		if f == float64(int64(f)) {
			return int64(f)
		}
		return f
	case js.TypeString:
		return v.String()
	default:
		return v.String()
	}
}
```

> `ScanValue` uses `Uint8ArrayClass` (already declared in `async_wasm.go`) — no new var needed.
> `Errf` comes from `. "github.com/tinywasm/fmt"` already imported in `jsvalue.go`.

## Tests to Add

Add to **`jsvalue_test.go`** (existing file, `//go:build wasm`):

```go
func TestScanValue(t *testing.T) {
	var s string
	if err := jsvalue.ScanValue(js.ValueOf("hello"), &s); err != nil || s != "hello" {
		t.Fatalf("string: got %q err %v", s, err)
	}
	var i int
	if err := jsvalue.ScanValue(js.ValueOf(42), &i); err != nil || i != 42 {
		t.Fatalf("int: got %d err %v", i, err)
	}
	var i64 int64
	if err := jsvalue.ScanValue(js.ValueOf(99), &i64); err != nil || i64 != 99 {
		t.Fatalf("int64: got %d err %v", i64, err)
	}
	var f float64
	if err := jsvalue.ScanValue(js.ValueOf(3.14), &f); err != nil || f != 3.14 {
		t.Fatalf("float64: got %f err %v", f, err)
	}
	var b bool
	if err := jsvalue.ScanValue(js.ValueOf(true), &b); err != nil || !b {
		t.Fatalf("bool: got %v err %v", b, err)
	}
	_, err := jsvalue.ScanValue(js.ValueOf(1), new(int8))
	if err == nil {
		t.Fatal("expected error for unsupported type *int8")
	}
}

func TestToAny(t *testing.T) {
	if v := jsvalue.ToAny(js.Null()); v != nil {
		t.Fatalf("null: got %v", v)
	}
	if v := jsvalue.ToAny(js.ValueOf("hi")); v != "hi" {
		t.Fatalf("string: got %v", v)
	}
	if v := jsvalue.ToAny(js.ValueOf(7)); v != int64(7) {
		t.Fatalf("int: got %T(%v)", v, v)
	}
	if v := jsvalue.ToAny(js.ValueOf(1.5)); v != 1.5 {
		t.Fatalf("float: got %T(%v)", v, v)
	}
	if v := jsvalue.ToAny(js.ValueOf(true)); v != true {
		t.Fatalf("bool: got %v", v)
	}
}
```

## Stages Summary

| # | Archivo | Acción |
|---|---|---|
| 1 | `jsvalue/jsvalue.go` | Agregar `ScanValue` y `ToAny` — type switch, sin reflect |
| 2 | `jsvalue/jsvalue_test.go` | Agregar `TestScanValue` y `TestToAny` |

## Verification

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
gotest
```

Todos los tests deben pasar. Sin regresiones en `TestAwaitPromise_*`, `TestUint8ArrayClass_defined`.

---

## Follow-up (separate plan — do NOT implement here)

After this ships, `tinywasm/goflare/d1` must be updated to replace its local `scanValue` and
`jsValueToAny` with `jsvalue.ScanValue` and `jsvalue.ToAny`. That is a separate plan in
`goflare/docs/PLAN.md`.
