# PLAN — `jsvalue` al codec tipado: fuera `reflect` + `map` + `any`-switch (0-alloc) · BREAKING

> Este plan se despacha vía el workflow CodeJob. Ver skill: `agents-workflow`.
> **Estado:** LISTO PARA REVISIÓN DEL USUARIO.
> **Repo objetivo:** `github.com/tinywasm/jsvalue` (wasm-only, `//go:build wasm`).
> **Depende de (GATE):** `tinywasm/fmt` con el contrato `Encodable`/`Decodable`/`FieldWriter`/
> `FieldReader` publicado (ver `fmt/docs/PLAN.md`).
> **Tipo:** breaking change. Elimina `reflect` (~72 KB de tablas en wasm) **y** `map` (arrastra
> el runtime de hashmap de TinyGo) **y** el `switch` de tipos infinito. `jsvalue` se toca **una
> sola vez** (va directo al codec; sin intermedio basura).

## Reglas permanentes del repo → `AGENTS.md`

Las restricciones del ecosistema (wasm-only `//go:build wasm` sin contraparte `!wasm`; **no
`reflect`**; **no `map`**; no stdlib → `tinywasm/fmt`; 0-alloc Go-side; contrato único de
`[]byte`; `gotest`) están en [`AGENTS.md`](../AGENTS.md). Este plan NO las repite completas; solo
inlinea lo crítico de la tarea (ver Checklist).

## Prerequisito (PRIMERO — entorno del agente)

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

`jsvalue` y sus tests son **wasm-only**. Si `gotest` no corre wasm en el entorno aislado,
workaround: `go test -c` + `node`. Criterio de salida: **tests verdes en wasm**.

## Problema (medido)

`jsvalue.go` usa `reflect` en los `default` de `ToJS`/`ToGo` (~72 KB de tablas de tipos en wasm)
y `map` en varios `case` (`map[string]any/string/int`) + en `ToAny`. Ambos inflan el binario en
TinyGo. La causa raíz es el **tipo borrado (`any`)** en el límite, que obliga a `switch` infinito
/ `reflect` / `map`. Evidencia: `ToJS`/`ToGo` **no tienen consumidores** en el ecosistema (solo
README + tests); el edge usa solo `ScanValue` (reflect/map-free para primitivos).

## Decisión arquitectónica (resuelta — para revisión del usuario)

Migrar la conversión de structs/slices al **codec tipado de `fmt`** (patrón visitor). `jsvalue`
implementa los writer/reader concretos sobre `js.Value`. Se eliminan `reflect`, los `case` de
`map`, y el `default` por reflexión.

### Contrato de `fmt` (ya publicado, para referencia)

```go
type FieldWriter interface { String(name,val string); Int(name string,val int64); Uint(...); Float(...); Bool(...); Bytes(name string,val []byte); Null(name string); Object(name string,val Encodable); Array(name string,n int,each func(i int,a ArrayWriter)) }
type ArrayWriter interface { String(val string); Int(val int64); Float(val float64); Bool(val bool); Bytes(val []byte); Object(val Encodable) }
type Encodable interface { EncodeFields(w FieldWriter) }
type FieldReader interface { String(name string)(string,bool); Int(...); Uint(...); Float(...); Bool(...); Bytes(...)([]byte,bool); Object(name string,into Decodable) bool; Array(name string)(ArrayReader,bool) }
type ArrayReader interface { Len() int; String(i int) string; Int(i int) int64; Float(i int) float64; Bool(i int) bool; Bytes(i int) []byte; Object(i int,into Decodable) bool }
type Decodable interface { DecodeFields(r FieldReader) error }
```

### Lo que se IMPLEMENTA (archivo nuevo `codec_wasm.go`)

- `jsObjectWriter` (`fmt.FieldWriter`): escribe a un `js.Object` vía `obj.Set(name, …)`. Cada
  método mapea a `js.ValueOf` del valor tipado; `Bytes` → ver regla de `[]byte` abajo; `Object`
  → crea sub-objeto y recursa con `EncodeFields`; `Array` → `Array.New(n)` + `ArrayWriter`.
- `jsArrayWriter` (`fmt.ArrayWriter`): `arr.SetIndex(i, …)`.
- `jsObjectReader` (`fmt.FieldReader`): `jsVal.Get(name)` + extracción tipada (`.String()`,
  `.Int()`, `.Float()`, `.Bool()`); presencia = `!IsUndefined() && !IsNull()`.
- `jsArrayReader` (`fmt.ArrayReader`): `jsVal.Index(i)`.

### Lo que se REESCRIBE en `jsvalue.go`

- **`ToJS(data any) js.Value`**: conservar los `case` de **primitivos** (string, bool, ints,
  floats, `[]byte`, `[]string`, `[]int`, `[]any`). **ELIMINAR** los `case` de `map[string]*`.
  **`default`**: si `data` implementa `fmt.Encodable` → crear `js.Object`, envolver en
  `jsObjectWriter` y llamar `EncodeFields`. Si no → fallback `js.ValueOf(Convert(v).String())`.
  **Quitar el import `reflect` y todo `reflect.*`.**
- **`ToGo(jsVal js.Value, v any) error`**: conservar los `case` de punteros a primitivos y
  `*[]byte` (con la regla de `[]byte` abajo). **ELIMINAR** los `case` de `*map[string]*`.
  **`default`**: si `v` implementa `fmt.Decodable` → envolver `jsVal` en `jsObjectReader` y
  llamar `DecodeFields`. Si no → `Err("jsvalue","unsupported","destination","type")`.
  **Quitar `reflect`.**
- **`ToAny(v js.Value) any`**: para `TypeObject` que es **array** → `[]any` (OK, sin map). Para
  objeto NO-array → **devolver el `js.Value` crudo** (no construir `map[string]any`). Quitar el
  `map`. Documentar: para decodificar campos de un objeto, usar un tipo `fmt.Decodable` + `ToGo`.
- **`ScanValue`**: se conserva (primitivos, reflect/map-free). Alinear `*[]byte` para que acepte
  **string Y Uint8Array** (un solo contrato de `[]byte`, ver abajo).

### Regla única de `[]byte` (arregla el drift de 3 codificaciones)

`[]byte` ↔ **JS string** es el contrato canónico de `ToJS`/`Encode`. La decodificación
(`ScanValue *[]byte`, `FieldReader.Bytes`) acepta **string Y Uint8Array** (Uint8Array porque los
blobs de D1 llegan así). Una sola regla, implementada en un único helper reutilizado por
`ScanValue` y `jsObjectReader.Bytes`.

## Pasos de ejecución

### Stage 1 — writers/readers concretos
1. Crear `codec_wasm.go` (`//go:build wasm`) con `jsObjectWriter`, `jsArrayWriter`,
   `jsObjectReader`, `jsArrayReader` implementando las interfaces de `fmt`. Helper único de
   `[]byte` (string|Uint8Array).

### Stage 2 — reescribir `jsvalue.go`
2. Quitar `import "reflect"`. Reescribir `default` de `ToJS` (Encodable) y `ToGo` (Decodable).
   Eliminar los `case` de `map`. Ajustar `ToAny` (sin map). Alinear `ScanValue *[]byte`.
3. Confirmar: sin `reflect.` y sin `map[` en todo `jsvalue.go` y `codec_wasm.go`.

### Stage 3 — tests
4. Reescribir `jsvalue_test.go` y `benchmark_test.go`: los structs de test (`TestStruct`,
   `ComplexStruct`, `ByteStruct`, `TagStruct`) implementan `fmt.Encodable`/`fmt.Decodable` (a
   mano en el test; **no** usar `map`, **no** depender de tags por reflexión). El test que usaba
   `map[string]int` se reescribe a un `Encodable`. Cubrir: objeto con claves = nombres de campo,
   anidado, array, `[]byte` (string↔Uint8Array), y tipo no soportado → fallback/error.
5. Test de asignaciones: `testing.AllocsPerRun` sobre el round-trip con writer/reader reusados →
   afirmar **0 asignaciones del heap Go** en el camino de codec (la creación del objeto JS es
   del lado JS y no cuenta).

### Stage 4 — actualizar el benchmark existente (antes/después) — OBLIGATORIO
**YA EXISTE** `benchmark_test.go` (`BenchmarkToJS_*`/`ToGo_*`) y la sección **"Performance
Results"** en `README.md`. **NO crear** un doc nuevo; **actualizar** lo existente:
6. **Ajustar `benchmark_test.go`:** los benches de structs (`BenchmarkToJS_Struct`,
   `BenchmarkToGo_Struct`) pasan por el codec (el tipo de bench implementa `fmt.Encodable`/
   `Decodable`). **Eliminar** los benches del camino map que ya no existe (`BenchmarkToGo_Any_Map`,
   `BenchmarkToGo_Map_Reuse`).
7. **Tamaño de binario wasm (headline):** medir un `main.go` representativo (usa `ToJS`/`ToGo`
   de un tipo `Encodable`) con `tinygo build -target wasm -opt=z -no-debug` **antes** (con
   reflect/map) y **después** (codec). Registrar el delta esperado (cae el bloque `reflect`,
   ~72 KB) en la sección **"Performance Results"** del `README.md`.
8. **Asignaciones:** confirmar `AllocsPerRun==0` (Stage 3) y reflejar los `allocs/op` nuevos en
   esa misma sección. Actualizar "Last updated".

### Stage 5 — documentación (OBLIGATORIO)
9. **`README.md`**: reescribir `ToJS`/`ToGo`/`ToAny`. Dejar claro: structs/slices vía
   `fmt.Encodable`/`fmt.Decodable` (no reflexión, no tags, no map); `ToAny` ya no devuelve `map`
   para objetos (devuelve el `js.Value`); regla única de `[]byte`. Quitar toda mención a
   "reflection"/"map". Enlazar `docs/BENCHMARK.md`.

## Verificación (repo-local, ejecutable por el agente)

```bash
# 1. Sin reflect ni map en el paquete:
grep -nE '"reflect"|reflect\.|map\[' jsvalue.go codec_wasm.go && echo "FALLA" || echo "OK: sin reflect/map"

# 2. El paquete wasm no arrastra reflect:
GOOS=js GOARCH=wasm go list -deps . | grep -E '^reflect$' && echo "FALLA: reflect en deps" || echo "OK"

# 3. Tests verdes (wasm) + 0-alloc:
gotest    # o: go test -c && node ... (workaround)
```

> Validación de tamaño aguas abajo (NO la hace el agente): en `goflare-demo`,
> `GOOS=js GOARCH=wasm go list -deps ./edge | grep -E '^reflect$'` debe quedar vacío y
> `edge.wasm` bajar ~72 KB (234 KB → ~150 KB).

## Checklist de calidad (obligatorio)

- **Sin `reflect`** (0 referencias).
- **Sin `map`** en `jsvalue.go` ni `codec_wasm.go` (ni `case map[...]`, ni `ToAny`→map).
- **Sin `any` en el camino de structs/slices** (va por el codec tipado). `ToJS(any)`/`ToGo(any)`
  mantienen `any` solo en su firma de entrada (compatibilidad), pero el dispatch de structs es
  por interfaz, no por reflexión.
- **0 asignaciones Go-side** en el codec (writer/reader reusan estado).
- **Regla única de `[]byte`** en un solo helper.
- **Sin duplicación:** `jsObjectReader.Bytes` y `ScanValue *[]byte` comparten el helper de `[]byte`.
- Reglas genéricas del ecosistema: ver [`AGENTS.md`](../AGENTS.md).

## Tabla de stages

| Stage | Objetivo | Entregable | Criterio de salida |
|---|---|---|---|
| 1 | Writers/readers JS | `codec_wasm.go` | implementan las interfaces de `fmt` |
| 2 | Reescribir conversores | `jsvalue.go` sin reflect/map | `grep` (verif. 1) limpio |
| 3 | Tests + 0-alloc | structs de test = `Encodable`; `AllocsPerRun==0` | `gotest` wasm verde |
| 4 | Comparativa antes/después | **actualizar `benchmark_test.go` + "Performance Results" del README** (tamaño wasm + allocs Antes\|Después) | delta de `reflect` registrado; benches de map eliminados |
| 5 | Documentación | `README.md` actualizado | sin "reflection"/"map" |

## Nota (coordinación)

GATE: `fmt` (contrato) debe estar publicado. Para **uso real** desde modelos, estos deben
implementar `Encodable`/`Decodable` (los genera `ormc` — ver `orm/docs/PLAN.md`); pero `jsvalue`
**compila y testea** con tipos de test que implementan el contrato a mano, así que NO depende de
`ormc` para cerrar este plan. `tinywasm/json` y `fmt.Fielder` no cambian. Ver
`~/Dev/Project/tinywasm/docs/SIZE_OPTIMIZATION_MASTER_PLAN.md`.
