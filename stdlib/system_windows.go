//go:build windows

package stdlib

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	getTickCount64       = kernel32.NewProc("GetTickCount64")
	globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	openProcess          = kernel32.NewProc("OpenProcess")
	getExitCodeProcess   = kernel32.NewProc("GetExitCodeProcess")
	closeHandle          = kernel32.NewProc("CloseHandle")
)

type memoryStatusEx struct {
	Length, MemoryLoad                                                                                   uint32
	TotalPhys, AvailPhys, TotalPageFile, AvailPageFile, TotalVirtual, AvailVirtual, AvailExtendedVirtual uint64
}

func systemUptimeSeconds() (float64, error) {
	v, _, e := getTickCount64.Call()
	if v == 0 && e != syscall.Errno(0) {
		return 0, e
	}
	return float64(v) / 1000, nil
}
func systemMemory() (uint64, uint64, error) {
	s := memoryStatusEx{}
	s.Length = uint32(unsafe.Sizeof(s))
	ok, _, e := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&s)))
	if ok == 0 {
		return 0, 0, fmt.Errorf("GlobalMemoryStatusEx: %w", e)
	}
	return s.TotalPhys, s.AvailPhys, nil
}
func formatUptime(seconds float64) string {
	s := int(seconds)
	days := s / 86400
	s %= 86400
	hours := s / 3600
	s %= 3600
	minutes := s / 60
	s %= 60
	if days > 0 {
		return fmt.Sprintf("%d Tage, %02d:%02d:%02d", days, hours, minutes, s)
	}
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, s)
}
func processIsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, _, _ := openProcess.Call(0x1000, 0, uintptr(uint32(pid)))
	if h == 0 {
		return false
	}
	defer closeHandle.Call(h)
	var code uint32
	ok, _, _ := getExitCodeProcess.Call(h, uintptr(unsafe.Pointer(&code)))
	return ok != 0 && code == 259
}
