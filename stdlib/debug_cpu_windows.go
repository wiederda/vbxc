//go:build windows

package stdlib

import (
	"syscall"
	"unsafe"
)

var getProcessTimesProc = syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessTimes")

type debugFiletime struct{ Low, High uint32 }

func processCPUTime() float64 {
	var creation, exit, kernel, user debugFiletime
	h, _ := syscall.GetCurrentProcess()
	r, _, _ := getProcessTimesProc.Call(uintptr(h), uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if r == 0 {
		return 0
	}
	toTicks := func(t debugFiletime) uint64 { return uint64(t.High)<<32 | uint64(t.Low) }
	return float64(toTicks(kernel)+toTicks(user)) / 1e7
}
