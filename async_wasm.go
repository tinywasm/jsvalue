//go:build wasm

package jsvalue

import (
	"syscall/js"

	. "github.com/tinywasm/fmt"
)

// Uint8ArrayClass is the JS Uint8Array constructor.
var Uint8ArrayClass = js.Global().Get("Uint8Array")

// AwaitPromise blocks the current goroutine until the JS Promise p resolves or rejects.
// Safe in WASM: channel receive yields the goroutine; the JS event loop keeps running.
func AwaitPromise(p js.Value) (js.Value, error) {
	resultCh := make(chan js.Value, 1)
	errCh := make(chan error, 1)

	then := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resultCh <- args[0]
		return js.Undefined()
	})
	defer then.Release()

	catch := js.FuncOf(func(_ js.Value, args []js.Value) any {
		errVal := args[0]
		msg := "promise rejected"
		if !errVal.IsNull() && !errVal.IsUndefined() {
			msg = errVal.Call("toString").String()
		}
		errCh <- Errf("jsvalue: %s", msg)
		return js.Undefined()
	})
	defer catch.Release()

	p.Call("then", then).Call("catch", catch)
	select {
	case v := <-resultCh:
		return v, nil
	case err := <-errCh:
		return js.Value{}, err
	}
}

// AwaitRequest blocks the current goroutine until the IndexedDB request req
// fires its "success" or "error" event.
// Ported from tinywasm/indexdb processRequest — proven pattern.
func AwaitRequest(req js.Value) (js.Value, error) {
	done := make(chan struct{}, 1)
	var result js.Value
	var reqErr error

	onSuccess := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		result = req.Get("result")
		done <- struct{}{}
		return nil
	})
	defer onSuccess.Release()

	onError := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		errVal := req.Get("error")
		msg := "unknown IndexedDB error"
		if errVal.Truthy() {
			msg = errVal.Get("message").String()
		}
		reqErr = Err("jsvalue: IndexedDB request failed:", msg)
		done <- struct{}{}
		return nil
	})
	defer onError.Release()

	req.Call("addEventListener", "success", onSuccess)
	req.Call("addEventListener", "error", onError)

	<-done
	return result, reqErr
}
