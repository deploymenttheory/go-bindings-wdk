# Error handling

WDK functions are Native API (`Nt*`/`Rtl*`) exports, so they report status via
**`NTSTATUS`**, not `HRESULT` or `GetLastError`.

## NTSTATUS

`NTSTATUS` is a 32-bit status code returned directly. `0` is `STATUS_SUCCESS`;
a negative value is a failure (`STATUS_*` codes). The bindings return it as the
typed value — you compare against the `STATUS_*` constants rather than getting a
Go `error`:

```go
status := registry.NtDeleteKey(handle)
switch {
case status == 0: // STATUS_SUCCESS
case status < 0:  // failure — inspect the STATUS_* code
}
```

This is deliberate: unlike Win32's `HRESULT` (which the win32 bindings lower to
`error` because the whole surface uses it uniformly), the Native API's status
space is its own channel, and forcing it into `error` would hide the specific
code. Compare against the generated `STATUS_*`/`NTSTATUS` constants, or convert
at your call site if you prefer an `error`.

## Contrast with go-bindings-win32

The win32 bindings surface four error domains (`GetLastError`, `HRESULT`,
`NTSTATUS`, and subsystem `DWORD` codes) and lower the first two to Go `error`.
The WDK surface is almost entirely `NTSTATUS`, returned as-is. If you mix the two
trees, expect win32 flat APIs to return `error`/`(T, error)` and WDK Native APIs
to return `NTSTATUS`.
