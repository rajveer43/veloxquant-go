//go:build windows

package system

import (
	"syscall"
	"unsafe"
)

// probe fills in Windows-specific fields. Apple Silicon does not apply on
// Windows, so AppleSilicon is always false here.
func probe(info *Info) {
	info.AppleSilicon = false
	info.CPUModel = ""
}

// memStatusEx mirrors the Win32 MEMORYSTATUSEX structure used by
// GlobalMemoryStatusEx.
type memStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// memoryStats returns total and available physical memory in bytes on
// Windows via the kernel32 GlobalMemoryStatusEx API.
func memoryStats() (total uint64, available uint64, err error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GlobalMemoryStatusEx")

	var status memStatusEx
	status.Length = uint32(unsafe.Sizeof(status))

	ret, _, callErr := proc.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 {
		return 0, 0, callErr
	}

	return status.TotalPhys, status.AvailPhys, nil
}
