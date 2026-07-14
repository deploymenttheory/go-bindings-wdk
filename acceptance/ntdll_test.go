//go:build windows

// Package acceptance drives generated WDK bindings against the live user-mode
// Native API (ntdll.dll). Cross-assembly types come from the published
// go-bindings-win32 module — the same type universe, composed.
package acceptance

import (
	"testing"
	"unsafe"

	"github.com/deploymenttheory/go-bindings-wdk/bindings/wdk/system/systeminformation"
	"github.com/deploymenttheory/go-bindings-wdk/bindings/wdk/system/systemservices"
	win32systeminformation "github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/systeminformation"
)

// TestRtlGetVersion fills a go-bindings-win32 OSVERSIONINFOW struct through a
// go-bindings-wdk function — the cross-module composition end-to-end.
func TestRtlGetVersion(t *testing.T) {
	var info win32systeminformation.OSVERSIONINFOW
	info.DwOSVersionInfoSize = uint32(unsafe.Sizeof(info))
	status := systemservices.RtlGetVersion(&info)
	if status != 0 {
		t.Fatalf("RtlGetVersion NTSTATUS = 0x%08X", uint32(status))
	}
	if info.DwMajorVersion < 10 {
		t.Errorf("MajorVersion = %d, want >= 10", info.DwMajorVersion)
	}
	t.Logf("Windows %d.%d build %d", info.DwMajorVersion, info.DwMinorVersion, info.DwBuildNumber)
}

// TestNtQuerySystemInformation queries SystemBasicInformation and checks the
// processor count field is sane.
func TestNtQuerySystemInformation(t *testing.T) {
	// SYSTEM_BASIC_INFORMATION (undocumented layout; NumberOfProcessors is
	// the last byte of the 64-bit layout at offset 56 in a 64-byte struct).
	var buffer [64]byte
	var returned uint32
	status := systeminformation.NtQuerySystemInformation(
		systeminformation.SystemBasicInformation,
		unsafe.Pointer(&buffer[0]), uint32(len(buffer)), &returned)
	if status != 0 {
		t.Fatalf("NtQuerySystemInformation NTSTATUS = 0x%08X", uint32(status))
	}
	if processors := buffer[56]; processors == 0 {
		t.Error("NumberOfProcessors = 0")
	}
}
