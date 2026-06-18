# PLAN — Eliminar `reflect` de `jsvalue` (fuga de ~72 KB al edge) · BREAKING CHANGE

> Este plan se despacha vía el workflow CodeJob. Ver skill: `agents-workflow`.
> **Estado:** LISTO PARA REVISIÓN DEL USUARIO.
> **Repo objetivo:** `github.com/tinywasm/jsvalue`.
> **Tipo:** breaking change (cambia cómo `ToJS`/`ToGo` manejan structs/slices).
> **Impacto:** elimina `reflect` del grafo wasm → ~72 KB menos de tablas de tipos en cualquier
> binario que importe `jsvalue` (medido en `goflare-demo/edge`: 234 KB → ~150 KB estimado).

## Prerequisito (PRIMERO — entorno del agente)

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

`jsvalue` y sus tests son **wasm-only** (`//go:build wasm`). `gotest` corre los tests wasm
automáticamente. Usar `gotest` (sin argumentos); **NO** `go test` directo.

## Problema (medido)

`jsvalue.go` importa `reflect` y lo usa en los `default` de `ToJS` y `ToGo` para convertir
structs/slices arbitrarios Go↔JS por reflexión. `reflect` arrastra **~72 KB de tablas de
tipos** (la sección `(unknown)` del binario). Medición real en `goflare-demo/edge`:

```
GOOS=js GOARCH=wasm go list -deps ./edge | grep reflect   → reflect (presente)
edge.wasm = 234 KB ; sección data = 79 KB (≈72 KB son tablas de reflect)
```

Cadena: `edge → goflare/d1 → jsvalue → reflect`. Pero **`goflare/d1` solo usa
`jsvalue.ScanValue`** (que es reflect-free). El `reflect` entra **solo** porque el paquete
`jsvalue` contiene `ToJS`/`ToGo` con `reflect` en sus ramas `default`.

### Evidencia de que el path reflect NO se usa en el ecosistema

```
grep -rn 'ToJS(|ToGo(' --include=*.go (todo el monorepo, sin /jsvalue/ ni _test) → 0 resultados
```

`ToJS`/`ToGo` **no tienen consumidores** en el ecosistema (solo el README y los tests de
`jsvalue` los referencian). El edge usa exclusivamente `ScanValue`/`ToAny`/`AwaitPromise`/
`Uint8ArrayClass` — todos reflect-free.

## Principio rector

> `reflect` es stdlib pesado: **prohibido en código wasm** (igual que `tinywasm/fmt` ya lo sacó).
> La conversión struct↔JS sin reflexión se hace con `fmt.Fielder` (Schema/Pointers generados por
> `ormc`), el mismo mecanismo que usa `tinywasm/json` (que es reflect-free).

## Decisión arquitectónica (resuelta — para revisión del usuario)

**Reemplazar las ramas `default` con reflexión por soporte basado en `fmt.Fielder`** (canónico
en el ecosistema), con fallback seguro. Se conserva todo lo demás (primitivos, `map`, `[]any`,
`[]string`, `[]int`, etc. ya tienen `case` explícito antes del `default`).

- **`ToJS(data any)`** — rama `default`:
  - Si `data` implementa `fmt.Fielder` → construir objeto JS: por cada `Field` de `Schema()`,
    clave = `Field.Name`, valor = `ToJS(<deref de Pointers()[i]>)`. Respetar `Field.OmitEmpty`
    (omitir si el valor es cero, usando `fmt.IsZero`).
  - Si implementa `fmt.FielderSlice` → array JS recorriendo `Len()`/`At(i)`.
  - Si no → fallback actual: `js.ValueOf(Convert(v).String())`.
- **`ToGo(jsVal js.Value, v any)`** — rama `default`:
  - Si `v` implementa `fmt.Fielder` → por cada `Field` de `Schema()`, leer `jsVal.Get(Field.Name)`
    y, si no es undefined/null, `ScanValue(jsField, Pointers()[i])`.
  - Si no → `return Err("jsvalue", "unsupported", "destination", "type")` (sin reflect).
- **Eliminar** el import `"reflect"` y TODO uso de `reflect.*` (incluyendo el deref genérico de
  punteros y el `reflect.MakeSlice`/`reflect.New` para slices de structs arbitrarios).

### Qué se pierde (aceptable: sin consumidores)

- Conversión automática de **structs planos por tags `json:`** vía reflexión.
- Conversión de **slices de tipos arbitrarios** (`[]MiStruct`) vía reflexión.

Ambos casos no tienen consumidores en el ecosistema. Quien los necesite usa un tipo `fmt.Fielder`
(generado por `ormc`) o un slice `fmt.FielderSlice`. Documentarlo en el README.

### Qué NO se toca

- `ScanValue` (reflect-free, lo usa el edge) — intacto.
- `ToAny` (reflect-free) — intacto.
- Todos los `case` de primitivos/`map`/`[]any`/`[]string`/`[]int`/`[]byte` en `ToJS`/`ToGo` —
  intactos.
- `async_wasm.go` (`AwaitPromise`), `Uint8ArrayClass` — intactos.

## Pasos de ejecución

### Stage 1 — quitar reflect de `jsvalue.go`
1. Eliminar `import "reflect"`.
2. Reescribir la rama `default` de `ToJS` según la Decisión (Fielder / FielderSlice / fallback
   string). Para leer el valor detrás de cada `Pointers()[i]` (que es un `any` apuntando al
   campo), reutilizar `fmt.ReadValues(schema, ptrs)` que ya desreferencia, y pasar cada valor a
   `ToJS(...)`.
3. Reescribir la rama `default` de `ToGo` según la Decisión (Fielder → `ScanValue` por campo;
   si no, error).
4. Verificar que no queda ningún identificador `reflect.` en el archivo.

### Stage 2 — actualizar tests
5. Los tests `jsvalue_test.go` y `benchmark_test.go` usan structs **planos** (`TestStruct`,
   `ComplexStruct`, `ByteStruct`, `TagStruct`) que dependían de reflexión. Convertirlos en tipos
   que implementen `fmt.Fielder` (con `Schema() []fmt.Field` y `Pointers() []any`) para seguir
   cubriendo el round-trip struct↔JS por el nuevo camino. Mantener la cobertura de:
   - objeto JS con claves = nombres de campo,
   - `OmitEmpty`,
   - campos `[]byte` (codificados como string),
   - tipos no soportados → fallback/error.
   Si algún test cubría exclusivamente la reflexión de structs arbitrarios (sin Fielder),
   reescribirlo al equivalente con Fielder o eliminarlo.

### Stage 3 — documentación (OBLIGATORIO)
6. **`README.md`**: actualizar las secciones `ToJS` y `ToGo`. Dejar claro que la conversión de
   structs/slices es vía `fmt.Fielder`/`fmt.FielderSlice` (no reflexión por tags), y que tipos
   no soportados caen en fallback (string) / error. Quitar cualquier afirmación de "uses
   reflection". Mantener los ejemplos de primitivos/maps/slices.

### Stage 4 — verificación
7. `gotest` verde (wasm).
8. `jsvalue` ya no importa `reflect`.

## Verificación (repo-local, ejecutable por el agente)

```bash
# 1. Sin reflect en el código del paquete:
grep -n 'reflect' jsvalue.go && echo "FALLA: aún usa reflect" || echo "OK: sin reflect"

# 2. El paquete wasm no arrastra reflect:
GOOS=js GOARCH=wasm go list -deps . | grep -E '^reflect$' && echo "FALLA: reflect en deps" || echo "OK: sin reflect en deps"

# 3. Tests verdes (wasm):
gotest
```

> Validación de tamaño aguas abajo (NO la hace el agente): en `goflare-demo`,
> `GOOS=js GOARCH=wasm go list -deps ./edge | grep '^reflect$'` debe quedar vacío y `edge.wasm`
> bajar ~72 KB (234 KB → ~150 KB).

## Checklist de calidad (obligatorio)

- **Sin strings hardcodeados repetidos:** las claves/mensajes de error repetidos → constantes o
  vía `fmt.Err(...)` con palabras. Nada de literales duplicados en la lógica.
- **Sin duplicación lógica:** reutilizar `ScanValue` para decodificar cada campo en `ToGo`
  (no reimplementar la conversión por tipo). Reutilizar `fmt.ReadValues` para leer valores en
  `ToJS`.
- **Reglas tinywasm:**
  - Nada de stdlib pesado en wasm: **cero `reflect`**. Usar `tinywasm/fmt` (no `errors`/`strconv`).
  - `jsvalue` ya importa `fmt` (dot-import) — `Fielder`/`Field`/`IsZero`/`ReadValues`/`Err` están
    disponibles sin nuevas dependencias.
  - El archivo sigue siendo `//go:build wasm` (jsvalue es interop JS, no aplica `!wasm`).

## Tabla de stages

| Stage | Objetivo | Entregable | Criterio de salida |
|---|---|---|---|
| 1 | `jsvalue.go` sin reflect | `ToJS`/`ToGo` con Fielder + fallback | `grep reflect jsvalue.go` vacío |
| 2 | Tests | structs de test → `fmt.Fielder`; cobertura preservada | compila y cubre round-trip |
| 3 | Documentación | `README.md` (ToJS/ToGo) actualizado | sin "reflection" en README |
| 4 | Verificación | — | `gotest` verde; deps wasm sin `reflect` |

## Nota (contexto del master plan)

Este es el mayor lever de tamaño que queda en el edge tras cerrar las fugas de
`regexp`/`html`/`dom`/diccionario. Ver `~/Dev/Project/tinywasm/docs/SIZE_OPTIMIZATION_MASTER_PLAN.md`.
La Fase D (`fetch` fuera del edge) NO es palanca de tamaño (código alcanzable despreciable) —
es solo limpieza arquitectónica y se trata aparte.
