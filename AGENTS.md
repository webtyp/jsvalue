# AGENTS.md — webtyp/jsvalue

Working notes for AI agents operating in this library. For end-user docs see [README.md](README.md).
Plans for code changes live in [docs/PLAN.md](docs/PLAN.md) and link back here for the standing
rules below (do not duplicate them in plans).

## Mission

`webtyp/jsvalue` is the **Go ↔ JavaScript value bridge** for TinyGo WASM (over `syscall/js`).
It converts Go values to/from `js.Value`: `ToJS`, `ToGo`, `ScanValue`, `ToAny`, plus
`Uint8ArrayClass`. It is the thin boundary used by `goflare/d1`, `indexdb`,
etc. Blocking a goroutine on a Promise or a JS event is **out of scope** — it
lives in `webtyp.com/await`; do not reintroduce it here.

## Ecosystem restrictions (do NOT violate)

- **WASM-only package.** Every file is `//go:build wasm`. There is **no `!wasm` counterpart** —
  do NOT add build-tag splits expecting a backend side; this package only exists in WASM builds.
  Consumers that are agnostic must not import `jsvalue` from backend code.
- **No `reflect`.** It pulls ~72 KB of type tables into the WASM binary (measured). All
  struct/slice conversion goes through the typed codec (`fmt.Encodable`/`fmt.Decodable`), not
  reflection. This is the single biggest size lever in the edge worker.
- **No `map`.** TinyGo pulls in the hashmap runtime, inflating the binary. No `map[...]...` in
  conversion paths (incl. `ToAny` for objects → return the `js.Value`, not a `map`).
- **No standard library for strings/errors.** Use `webtyp.com/fmt` (`fmt.Err`,
  `fmt.Errf`, `fmt.Convert`, ...). `syscall/js` is the only heavy import allowed (it is the JS
  boundary itself).
- **0-allocation on the Go side** of the conversion path (reuse writer/reader state). Creating
  the JS object/array allocates on the JS side and is unavoidable; that does not count.
- **Single `[]byte` contract.** `[]byte` → JS `Uint8Array` on encode (`encodeBytes`,
  `js.CopyBytesToJS` — never `string(val)`: a Go string crosses to JS decoded from UTF-8, so
  any byte that isn't valid UTF-8 gets silently replaced with U+FFFD, no error, no panic,
  just corrupted binary data; fixed 2026-09-23, was the encode side's real bug for years).
  Decode accepts **both** JS string and `Uint8Array` (`decodeBytes`) — string only for
  backward compatibility with data written before the fix, never write it again. One helper
  per direction, reused by every writer/reader — never re-implement per call site.

## Serialization codec (target contract)

Struct/slice conversion uses the typed visitor codec from `webtyp/fmt`
(`FieldWriter`/`FieldReader` + `Encodable`/`Decodable`): typed calls, no `any`, no `map`, no
`reflect`. `jsvalue` implements the concrete `js.Value` writer/reader. See `docs/PLAN.md` and
`../fmt/docs/CODEC_AND_FIELDER.md`.

## Testing

```bash
go install webtyp.com/devflow/cmd/gotest@latest   # once
gotest            # runs the WASM tests (NOT `go test`)
```

Tests are WASM-only. If `gotest` cannot run WASM in the sandbox, workaround: `go test -c` + run
with `node`. Exit criterion is still: **WASM tests green**. Benchmarks live in `benchmark_test.go`
and the "Performance Results" section of `README.md` — update those when conversion changes.

## Publishing

```bash
gopush 'message'   # tests + tag + push + dependency bumps (NOT git commit/push directly)
```

## Related

- [`webtyp/fmt`](https://github.com/webtyp/fmt) — the codec contract + `CODEC_AND_FIELDER.md`.
- [`webtyp/goflare`](https://github.com/webtyp/goflare) — `d1` uses `jsvalue.ScanValue`.
