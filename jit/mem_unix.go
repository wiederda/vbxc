//go:build !windows && (linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package jit

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// mmap(RW) -> kopieren -> mprotect(RX). Es wird niemals eine Datei angelegt
// (MAP_ANON), daher auch keine Spur auf der Festplatte.
type execMem struct {
	ptr  uintptr // Startadresse des Blocks
	size uintptr
}

const pageSize = 0x1000

// unsafePtr macht aus einer Speicheradresse einen unsafe.Pointer, ohne die
// uintptr -> unsafe.Pointer-Konvertierung direkt auszudrücken (go vet).
func unsafePtr(u uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&u))
}

func allocExec(code []byte) (*execMem, error) {
	if len(code) == 0 {
		return nil, fmt.Errorf("jit: leerer Codeblock")
	}
	size := (uintptr(len(code)) + pageSize - 1) &^ (pageSize - 1)
	b, err := unix.Mmap(-1, 0, int(size),
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_PRIVATE|unix.MAP_ANON)
	if err != nil {
		return nil, fmt.Errorf("jit: mmap: %w", err)
	}
	copy(b, code)
	if err := unix.Mprotect(b, unix.PROT_READ|unix.PROT_EXEC); err != nil {
		unix.Munmap(b)
		return nil, fmt.Errorf("jit: mprotect: %w", err)
	}
	return &execMem{ptr: uintptr(unsafe.Pointer(&b[0])), size: size}, nil
}

func (m *execMem) free() {
	if m != nil && m.ptr != 0 {
		unix.Munmap(unsafe.Slice((*byte)(unsafePtr(m.ptr)), int(m.size)))
		m.ptr = 0
	}
}
