# Examples

Runnable programs on the generated WDK bindings. They are part of the root
module — run them directly on Windows:

```sh
go run ./examples/sysinfo
```

- **[`sysinfo`](sysinfo)** — host info through the user-mode Native API:
  `RtlGetVersion` (OS version) and `NtQuerySystemInformation`
  (`SystemBasicInformation`, processor count). Demonstrates the cross-module
  model — WDK functions filling go-bindings-win32 structs — and the `NTSTATUS`
  return convention.

Each example needs Windows to run (the generated files are build-tagged
`windows && (amd64 || arm64)`), but the whole module cross-compiles from any OS.
