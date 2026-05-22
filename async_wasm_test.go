//go:build wasm

package jsvalue_test

import (
	"syscall/js"
	"testing"

	"github.com/tinywasm/jsvalue"
)

func TestAwaitPromise_resolve(t *testing.T) {
	p := js.Global().Get("Promise").Call("resolve", 42)
	v, err := jsvalue.AwaitPromise(p)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int() != 42 {
		t.Fatalf("want 42, got %d", v.Int())
	}
}

func TestAwaitPromise_reject(t *testing.T) {
	p := js.Global().Get("Promise").Call("reject", js.Global().Get("Error").New("boom"))
	_, err := jsvalue.AwaitPromise(p)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUint8ArrayClass_defined(t *testing.T) {
	if jsvalue.Uint8ArrayClass.IsUndefined() || jsvalue.Uint8ArrayClass.IsNull() {
		t.Fatal("Uint8ArrayClass is not defined")
	}
}

