# Getting started

`go-bindings-wdk` gives you the typed **user-mode Native API** — the `Nt*` /
`Rtl*` `ntdll.dll` exports and the kernel-shaped types they use.

```sh
go get github.com/deploymenttheory/go-bindings-wdk
```

**Requirements:** Go 1.25+; runs on **Windows amd64 or arm64**. Generated files
carry `//go:build windows && (amd64 || arm64)`, so you can cross-compile from
any OS (`GOOS=windows go build ./...`); only running needs Windows. The
dependencies are our own `go-bindings-win32` (for cross-assembly types + the
runtime) and, transitively, `go-winmd` (generator-time).

## A first call

```go
//go:build windows

import (
	"github.com/deploymenttheory/go-bindings-wdk/bindings/wdk/system/systemservices"
	win32systeminformation "github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/systeminformation"
)

var info win32systeminformation.OSVERSIONINFOW
info.DwOSVersionInfoSize = uint32(unsafe.Sizeof(info))
if status := systemservices.RtlGetVersion(&info); status == 0 { // NTSTATUS 0 = STATUS_SUCCESS
	fmt.Printf("Windows %d.%d build %d\n",
		info.DwMajorVersion, info.DwMinorVersion, info.DwBuildNumber)
}
```

Note the two imports: the *function* comes from this module (`bindings/wdk/…`),
the *struct it fills* comes from go-bindings-win32 (`bindings/win32/…`). That is
the cross-assembly model — see [cross-module-types.md](cross-module-types.md).

## Namespaces

WDK namespaces map like win32's: `Windows.Wdk.System.SystemServices` →
`bindings/wdk/system/systemservices`. `go run ./cmd/generate list` prints every
emitted namespace with construct counts.

## Errors

WDK functions return **`NTSTATUS`** (0 = `STATUS_SUCCESS`; negative = failure).
See [errors.md](errors.md).

## Documentation

- [Cross-module types](cross-module-types.md) — how Wdk composes with win32
- [Errors](errors.md) — the NTSTATUS domain
- [`CLAUDE.md`](../CLAUDE.md) — the dual-assembly generator architecture
