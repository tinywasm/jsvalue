---
PLAN: "refactor: jsvalue becomes codec-only; async moves to webtyp/await"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — extract the async bridge out of `jsvalue`

## Why

`jsvalue` is a JS↔Go **codec**: `ScanValue`, the object/array writers and
readers. `AwaitPromise` and `AwaitRequest`, in `async_wasm.go`, are not a
codec — they block a goroutine on a JS event. That is a different
responsibility, and it is duplicated: `webtyp/indexdb` already imports
`jsvalue` and still implements its own version of the same wait, in
`tx.go`'s `processCursorRequest` (a different shape — a loop, not a one-shot —
but built from the identical two-listener primitive).

`https://github.com/webtyp/await/blob/main/docs/PLAN.md` extracts that
primitive into its own zero-dependency module. This plan removes the
duplicate copy from `jsvalue`.

**Prerequisite: `webtyp.com/await` must be released before this
plan starts** — this plan does not create it.

## Ecosystem rules

`jsvalue` compiles to WASM; `async_wasm.go` already carries `//go:build wasm`.
No change to that constraint here — this plan removes a file, it does not add
imports.

## Changes

### 1. Delete `async_wasm.go` and `async_wasm_test.go`

```bash
rm async_wasm.go async_wasm_test.go
```

`AwaitPromise` and `AwaitRequest` are **removed, not aliased**. This package
does not keep a compatibility shim — every caller updates its import, listed
in §3. A thin wrapper here would leave two names for the same function
forever, which is the exact duplication this plan exists to remove.

### 2. `go.mod` — no dependency changes

`jsvalue` currently requires `webtyp/fmt` and `webtyp/model` for its codec
work; both stay, `async_wasm.go` was their only user for `fmt` in this
specific file but other codec files also import it — verify with
`grep -rln '"webtyp.com/fmt"' *.go` before assuming it can be
dropped, and do not drop it if any codec file still uses it.

### 3. Update the ecosystem's call sites — report if out of reach

Search this monorepo checkout for every caller:

```bash
grep -rln "jsvalue.AwaitPromise\|jsvalue.AwaitRequest" ..
```

Expected hits and the exact substitution for each:

| Repo | File | Change |
|---|---|---|
| `webtyp/indexdb` | `execute.go` (6 call sites) | own plan, `PLAN.md` — do not edit here, just confirm it references this one |
| `webtyp/goflare` | `r2/bucket.go`, `d1/adapter.go` | **out of scope**: add `import "webtyp.com/await"`, replace `jsvalue.AwaitPromise(x)` with `await.Promise(x)`, drop the `jsvalue` import if nothing else in the file uses it. If you cannot reach this repository from this dispatch, state that in the PR description instead of skipping silently. |

If a caller is found that is not in this table, do not silently change its
import — flag it in the PR description; a call site nobody accounted for is
worth a human look before an automated rename.

## Documentation

`README.md` — remove the async example if one exists; add one sentence
pointing to `webtyp/await` for "block a goroutine on a Promise or a JS
event", so a future reader who needs that does not reintroduce it here.

## Acceptance checklist

```bash
grep -rn "AwaitPromise\|AwaitRequest" .              # → empty
GOOS=js GOARCH=wasm go build ./...
gotest -tinygo
```
