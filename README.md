# go-bindings-wdk

[![Go Reference](https://pkg.go.dev/badge/github.com/deploymenttheory/go-bindings-wdk.svg)](https://pkg.go.dev/github.com/deploymenttheory/go-bindings-wdk)
[![CI](https://github.com/deploymenttheory/go-bindings-wdk/actions/workflows/ci.yml/badge.svg)](https://github.com/deploymenttheory/go-bindings-wdk/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Idiomatic Go bindings for the **Windows Driver Kit metadata surface**
(`Windows.Wdk.*`), generated from Microsoft's
[wdkmetadata](https://github.com/microsoft/wdkmetadata). The sister of
[go-bindings-win32](https://github.com/deploymenttheory/go-bindings-win32),
built on the same [go-winmd](https://github.com/deploymenttheory/go-winmd)
ECMA-335 reader and the same generate-don't-handwrite doctrine: committed
metadata, byte-deterministic regeneration, ABI layout assertions, live
acceptance tests.

What you get in practice is the **typed user-mode Native API**: the `Nt*` /
`Rtl*` `ntdll.dll` exports (registry transactions, filesystem, memory,
system information, threading…) with full struct/enum/constant definitions —
including the kernel-shaped types (`IRP`, `OBJECT_ATTRIBUTES` consumers,
`KEY_*` information classes) that user-mode systems tools otherwise
hand-transcribe from headers.

```go
import (
	"github.com/deploymenttheory/go-bindings-wdk/bindings/wdk/system/systemservices"
	win32systeminformation "github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/systeminformation"
)

var info win32systeminformation.OSVERSIONINFOW
info.DwOSVersionInfoSize = uint32(unsafe.Sizeof(info))
if status := systemservices.RtlGetVersion(&info); status == 0 {
	fmt.Printf("Windows %d.%d build %d\n", info.DwMajorVersion, info.DwMinorVersion, info.DwBuildNumber)
}
```

## One type universe with go-bindings-win32

`Windows.Wdk.winmd` references types defined in `Windows.Win32.*`. This
generator loads **both** winmds for resolution but emits only the
`Windows.Wdk.*` packages — cross-assembly references are **imports of the
published go-bindings-win32 module** (`win32foundation.HANDLE`,
`win32foundation.NTSTATUS`, …), so WDK handles and structs compose directly
with Win32 functions. The pinned win32metadata version in
`metadata/winmd/PROVENANCE.json` must match the winmd version of the
go-bindings-win32 release in `go.mod`; CI compiles the generated tree
against that release to catch skew.

Functions dispatch through `syscall.SyscallN` against `ntdll.dll` only —
kernel-mode-only DDIs carry no user-mode export in the metadata and are not
emitted as functions (their types, constants, and enums are).

## Status

Upstream wdkmetadata is **experimental** (`0.x-experimental`) with growing
coverage; a weekly workflow opens a regeneration PR when Microsoft ships a
new release. Constructs that can't be represented faithfully are skipped
with diagnostics ratcheted in `metadata/diagnostics-baseline.json`.

## How it's built

```sh
go run ./cmd/generate fetch-metadata   # WDK (latest) + win32 (pinned) winmds
go run ./cmd/generate ingest           # both winmds → IR (Wdk local, Win32. external)
go run ./cmd/generate bindings         # IR → bindings/wdk (self-cleaning, deterministic)
```

Generated code (`bindings/`) is never hand-edited — fix the generator under
`internal/` and regenerate.

## Examples & docs

- [`examples/sysinfo`](examples/sysinfo) — runnable: OS version + processor
  count via the Native API.
- [Getting started](docs/getting-started.md)
- [Cross-module types](docs/cross-module-types.md) — how Wdk composes with the
  win32 module, and the version-pin invariant
- [Errors](docs/errors.md) — the NTSTATUS domain
- [`CLAUDE.md`](CLAUDE.md) — the dual-assembly generator architecture

## License

[MIT](LICENSE).
