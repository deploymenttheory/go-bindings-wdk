# Cross-module types

`Windows.Wdk.winmd` is built on `Windows.Win32.winmd` and references its types
(handles, `NTSTATUS`, `UNICODE_STRING`, `OBJECT_ATTRIBUTES` consumers, security
descriptors, …). This module emits **only** the `Windows.Wdk.*` packages; every
reference to a Win32 type resolves to an **import of the published
`go-bindings-win32` module**.

So a WDK struct or function signature composes both trees:

```go
import (
	"github.com/deploymenttheory/go-bindings-wdk/bindings/wdk/system/registry"
	win32foundation "github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
)

// NtDeleteKey takes a win32 HANDLE and returns a win32 NTSTATUS, but the
// function itself lives in the WDK tree.
var handle win32foundation.HANDLE
status := registry.NtDeleteKey(handle)
```

There is **one type universe**: a `win32foundation.HANDLE` a WDK function
returns is the *same Go type* a go-bindings-win32 function consumes, so the two
trees interoperate directly. The generator achieves this by loading both winmds
into a single resolution context (external namespaces carry a reserved `Win32.`
prefix in the IR) and routing external imports to the win32 module — it never
emits a second, incompatible copy of a Win32 type.

## The version-pin invariant

Because Win32 types are imported (not copied), the win32 metadata used at
generation time **must match** the `go-bindings-win32` release pinned in
`go.mod`. Both are recorded in `metadata/winmd/PROVENANCE.json` (a two-record
array: the WDK package version and the win32 package version). CI compiles the
generated `bindings/wdk` against the pinned win32 module, so any drift — a Win32
type whose shape changed between the fetched winmd and the published module —
fails the build rather than miscompiling silently.

When bumping either side: update the pin, re-fetch, regenerate, and confirm the
compile. The weekly `winmd-update.yml` only bumps the WDK package; the win32 pin
stays fixed until you deliberately move it (and the go-bindings-win32 dependency
with it).

## Why some functions aren't emitted

Kernel-mode-only DDIs have no user-mode export (`ntdll.dll`) in the metadata, so
they are not emitted as callable functions — their **types, enums, and
constants still are**. A handful of functions are also skipped because they take
by-value structs or large integers that `syscall.SyscallN` cannot marshal;
these are tracked in `metadata/diagnostics-baseline.json` and ratcheted in CI.
