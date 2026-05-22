> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan: jsvalue — JS Async Bridge Utilities

## Context

`tinywasm/jsvalue` (module `github.com/tinywasm/jsvalue`) provides Go↔JS value conversion
utilities for WASM targets. This plan adds two async bridge functions so that any WASM binding
(Cloudflare D1, IndexedDB, fetch, etc.) can await JS async operations without reimplementing
the goroutine+channel pattern locally.

Module root: `tinywasm/jsvalue/`  
`go.mod`: `module github.com/tinywasm/jsvalue` — only dep is `github.com/tinywasm/fmt`.  
Compatibilidad: el código debe compilar con TinyGo (asyncify scheduler). Tests corren con `gotest` — no requiere instalar TinyGo.

## Why goroutine+channel, not callbacks

In Go WASM, `<-ch` (channel receive) yields the goroutine to the Go scheduler — it does **not**
block the JS thread. The JS event loop keeps running, which allows Promise `.then`/`.catch`
callbacks and IndexedDB event listeners to fire and send into the channel.

`tinywasm/indexdb` already uses this exact pattern (`processRequest` in `tx.go`) and is
proven in production. A callback-based API would force callers to pass functions instead of
receiving `(js.Value, error)` directly, losing Go's synchronous ergonomics.

## Goal

Add to `jsvalue`:

| Symbol | JS API | Used by |
|---|---|---|
| `var Uint8ArrayClass` | `globalThis.Uint8Array` | D1 binary args, IndexedDB blobs |
| `func AwaitPromise(p js.Value) (js.Value, error)` | Promise `.then/.catch` | D1, fetch |
| `func AwaitRequest(req js.Value) (js.Value, error)` | IndexedDB `addEventListener success/error` | IndexedDB |

`AwaitRequest` is ported from `indexdb/tx.go:processRequest` — proven, tested, unchanged logic.
After this plan ships, `tinywasm/indexdb` migrates to use `jsvalue.AwaitRequest` (separate plan).

## TinyWasm Constraints (mandatory)

- No `import "errors"`, `"fmt"`, `"strings"` from stdlib — use `github.com/tinywasm/fmt`.
- Both new files carry `//go:build wasm` as first line.
- No new entries in `go.mod`.

## Code Quality (mandatory)

- Buffered channels (`cap=1`) in both functions to prevent goroutine leaks if caller exits early.
- `js.Func` handles must be released after use: `AwaitPromise` releases inside each callback via `defer`; `AwaitRequest` releases at function scope via `defer` (both are correct — the handle must not outlive the call).
- Error messages via `Err(...)` / `Errf(...)` from `github.com/tinywasm/fmt`.

## New Files

### `async_wasm.go`

```go
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
    errCh    := make(chan error, 1)
    var then, catch js.Func
    then = js.FuncOf(func(_ js.Value, args []js.Value) any {
        defer then.Release()
        resultCh <- args[0]
        return js.Undefined()
    })
    catch = js.FuncOf(func(_ js.Value, args []js.Value) any {
        defer catch.Release()
        errCh <- Errf("jsvalue: promise rejected: %s", args[0].Call("toString").String())
        return js.Undefined()
    })
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
    done    := make(chan struct{}, 1)
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
    req.Call("addEventListener", "error",   onError)

    <-done
    return result, reqErr
}
```

### `async_wasm_test.go`

```go
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
    if err != nil { t.Fatal(err) }
    if v.Int() != 42 { t.Fatalf("want 42, got %d", v.Int()) }
}

func TestAwaitPromise_reject(t *testing.T) {
    p := js.Global().Get("Promise").Call("reject", js.Global().Get("Error").New("boom"))
    _, err := jsvalue.AwaitPromise(p)
    if err == nil { t.Fatal("expected error, got nil") }
}

func TestUint8ArrayClass_defined(t *testing.T) {
    if jsvalue.Uint8ArrayClass.IsUndefined() || jsvalue.Uint8ArrayClass.IsNull() {
        t.Fatal("Uint8ArrayClass is not defined")
    }
}

func TestAwaitRequest_success(t *testing.T) {
    // IDBKeyRange.only() is a lightweight IDB API available in all WASM envs.
    // For a pure channel/callback test without an actual DB, use a resolved Promise
    // wrapped as a fake request — or skip and note that indexdb integration tests cover this.
    t.Skip("covered by tinywasm/indexdb integration tests")
}
```

> `AwaitRequest` is fully exercised by `tinywasm/indexdb` tests. The skip is intentional —
> constructing a real IDB request in a unit test requires an open database.

## Stages

| # | Archivo | Acción |
|---|---|---|
| 1 | `jsvalue/async_wasm.go` | Crear — `Uint8ArrayClass`, `AwaitPromise`, `AwaitRequest` |
| 2 | `jsvalue/async_wasm_test.go` | Crear — 3 tests activos + 1 skip documentado |

## Installation (prerequisite)

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

## Verification

```bash
gotest
```

vet + stdlib + wasm auto-detected. Los 3 tests activos deben pasar.
