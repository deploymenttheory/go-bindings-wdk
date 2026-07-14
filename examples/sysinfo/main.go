//go:build windows

// Command sysinfo prints host information through the WDK user-mode Native
// API, composing go-bindings-wdk functions with go-bindings-win32 types.
//
//	go run ./examples/sysinfo
package main

import (
	"fmt"
	"unsafe"

	"github.com/deploymenttheory/go-bindings-wdk/bindings/wdk/system/systeminformation"
	"github.com/deploymenttheory/go-bindings-wdk/bindings/wdk/system/systemservices"
	win32systeminformation "github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/systeminformation"
)

func main() {
	// RtlGetVersion fills a go-bindings-win32 OSVERSIONINFOW struct.
	var version win32systeminformation.OSVERSIONINFOW
	version.DwOSVersionInfoSize = uint32(unsafe.Sizeof(version))
	if status := systemservices.RtlGetVersion(&version); status != 0 {
		fmt.Printf("RtlGetVersion failed: NTSTATUS 0x%08X\n", uint32(status))
		return
	}
	fmt.Printf("Windows %d.%d build %d\n",
		version.DwMajorVersion, version.DwMinorVersion, version.DwBuildNumber)

	// NtQuerySystemInformation(SystemBasicInformation) — the processor count is
	// the last byte of the 64-byte SYSTEM_BASIC_INFORMATION layout.
	var buf [64]byte
	var returned uint32
	status := systeminformation.NtQuerySystemInformation(
		systeminformation.SystemBasicInformation,
		unsafe.Pointer(&buf[0]), uint32(len(buf)), &returned)
	if status != 0 {
		fmt.Printf("NtQuerySystemInformation failed: NTSTATUS 0x%08X\n", uint32(status))
		return
	}
	fmt.Printf("Logical processors: %d\n", buf[56])
}
