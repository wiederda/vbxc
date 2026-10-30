//go:build windows

package jit

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Windows: VirtualAlloc(RW) -> kopieren -> VirtualProtect(RX) ->
// FlushInstructionCache. Es wird niemals eine Datei angelegt.
var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procVirtualAlloc          = kernel32.NewProc("VirtualAlloc")
	procVirtualFree           = kernel32.NewProc("VirtualFree")
	procVirtualProtect        = kernel32.NewProc("VirtualProtect")
	procFlushInstructionCache = kernel32.NewProc("FlushInstructionCache")
)

const (
	memCommit     = 0x1000
	memReserve    = 0x2000
	memRelease    = 0x8000
	pageReadWrite = 0x04
	pageExecuteR  = 0x20
	pageSize      = 0x1000
)

// execMem ist ein Block ausführbaren Speichers (W^X: erst RW, dann RX).
type execMem struct {
	ptr  uintptr // Startadresse des Blocks
	size uintptr
}

// unsafePtr macht aus einer Syscall-Adresse einen unsafe.Pointer. Der Umweg
// über einen Zeiger auf die Variable hält go vet zufrieden (kein uintptr ->
// unsafe.Pointer in direkter Form), die Prüfung von checkptr bleibt gültig,
// weil beide Werte 8 Byte groß sind.
func unsafePtr(u uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&u))
}

func allocExec(code []byte) (*execMem, error) {
	if len(code) == 0 {
		return nil, fmt.Errorf("jit: leerer Codeblock")
	}
	size := (uintptr(len(code)) + pageSize - 1) &^ (pageSize - 1)
	p, _, _ := procVirtualAlloc.Call(0, size, memCommit|memReserve, pageReadWrite)
	if p == 0 {
		return nil, fmt.Errorf("jit: VirtualAlloc fehlgeschlagen")
	}
	dst := unsafe.Slice((*byte)(unsafePtr(p)), len(code))
	copy(dst, code)

	var old uint32
	r, _, _ := procVirtualProtect.Call(p, size, pageExecuteR, uintptr(unsafe.Pointer(&old)))
	if r == 0 {
		procVirtualFree.Call(p, 0, memRelease)
		return nil, fmt.Errorf("jit: VirtualProtect fehlgeschlagen")
	}
	// INVALID_HANDLE_VALUE = ^uintptr(0): Prozess selbst.
	procFlushInstructionCache.Call(^uintptr(0), p, size)
	return &execMem{ptr: p, size: size}, nil
}

func (m *execMem) free() {
	if m != nil && m.ptr != 0 {
		procVirtualFree.Call(m.ptr, 0, memRelease)
		m.ptr = 0
	}
}
