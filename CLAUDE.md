# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repository.

## What this is

`go-bindings-wdk` generates idiomatic Go bindings for the **Windows Driver Kit
metadata surface** (`Windows.Wdk.*`) from Microsoft's
[wdkmetadata](https://github.com/microsoft/wdkmetadata). It is the sister of
`go-bindings-win32`, sharing the same [go-winmd](https://github.com/deploymenttheory/go-winmd)
reader and the same generator architecture. In practice it surfaces the typed
**user-mode Native API** (`Nt*`/`Rtl*` `ntdll.dll` exports) plus the kernel-shaped
types they consume.

## Commands

```sh
go build ./...

# Fetch BOTH winmds: the WDK metadata (latest) + the pinned win32 metadata
# (must match the go-bindings-win32 release in go.mod).
go run ./cmd/generate fetch-metadata --win32-version <pin>

# Ingest projects BOTH winmds → IR (Wdk local, Win32. external).
go run ./cmd/generate ingest

go run ./cmd/generate validate
go run ./cmd/generate bindings --diagnostics-baseline metadata/diagnostics-baseline.json
go run ./cmd/generate abitest
go run ./cmd/generate list
go run ./cmd/generate diff --old <dir> --new metadata/wdk
go run ./cmd/inspect metadata/wdk/Foundation.w32meta.json

go test ./internal/...     # ingest + emit unit tests
go test ./acceptance/      # live ntdll calls + ABI layout gate (Windows)
```

## The dual-assembly architecture (the key difference from win32)

`Windows.Wdk.winmd` has TypeRefs into `Windows.Win32.*`, so the generator must
resolve types across both assemblies while emitting only the WDK ones.

- **Identity scheme.** Local (emitted) namespaces keep bare short names
  (`Foundation` ≙ `Windows.Wdk.Foundation`). External win32 namespaces carry a
  reserved `Win32.` prefix in the IR (`Win32.System.Com` ≙
  `Windows.Win32.System.Com`). `pipeline.IsExternal(ns) =
  strings.HasPrefix(ns, "Win32.")`.
- **Two-run ingest.** `runIngest` opens both winmds: run 1 projects the full
  win32 assembly into `Win32.`-prefixed external IR (`metadata/win32/`); run 2
  projects the WDK assembly (`metadata/wdk/`) seeded with run 1's `KindIndex()`
  as `ExtraKinds`, so cross-assembly `TypeRef`s become
  `ApiRef{Api: "Win32...", TargetKind: …}` and WDK COM chains record
  `BaseInterfaceApi: "Win32.System.Com"`.
- **Merged Registry.** `pipeline.LoadAll(wdkDir, win32Dir)` loads both;
  `VtableStartSlot` walks base chains into the win32 IR automatically.
  `FunctionOwner` prefers the local owner when a name (`Nt*`/`Rtl*`) exists in
  both assemblies.
- **Emit-local-only.** `EmitAll` skips `IsExternal` namespaces;
  `computeSkippedTypes` still spans all namespaces so refs to win32-skipped
  structs degrade identically to the published win32 module.
- **Import routing.** `typemap.Mapper.ImportPathFor` sends external namespaces
  to `<win32Module>/bindings/win32/<pkg>` and local ones to
  `<wdkModule>/bindings/wdk/<pkg>`. The runtime is imported from the win32
  module — **`bindings/runtime/win32` is NOT copied here.** Key consts:
  `win32ModulePath`, `LocalBindingsRoot = "/bindings/wdk/"`,
  `foundationApi = "Win32.Foundation"`, `apiRootPrefix = "Windows.Wdk."`.

## The version-pin invariant

The win32 metadata this repo fetches for resolution **must match** the winmd
version of the `go-bindings-win32` release pinned in `go.mod` (both recorded in
`metadata/winmd/PROVENANCE.json`, a two-record array). If they drift, a
cross-assembly type could resolve to a shape the published win32 module doesn't
have. CI compiles `bindings/wdk` against the pinned win32 module to catch this.

## User-mode vs kernel

Functions dispatch through `syscall.SyscallN` against their ImplMap DLL —
`ntdll.dll` exports are user-mode callable; kernel-only DDIs have no user-mode
export in the metadata and are not emitted as functions (their types, enums,
and constants still are). Lazy loading means an unused `ntoskrnl.exe`-class proc
never loads.

## Everything else mirrors go-bindings-win32

The emitter (`internal/codegen/emit/raw`), IR (`internal/win32meta`), ingest,
typemap, naming, determinism gate, diagnostics ratchet, and ABI test are the
copied win32 generator. See that repo's CLAUDE.md for the shared internals; the
sections above cover only what differs. Upstream wdkmetadata is **experimental
(0.x)** — expect coverage gaps; the weekly `winmd-update.yml` opens a
regeneration PR on new releases. Never hand-edit `bindings/`.
