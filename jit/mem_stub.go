//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package jit

import "fmt"

// execMem ist ein Block ausführbaren Speichers (W^X).
type execMem struct {
	ptr  uintptr // Startadresse des Blocks
	size uintptr
}

func allocExec(code []byte) (*execMem, error) {
	return nil, fmt.Errorf("jit: ausführbarer Speicher wird auf dieser Plattform nicht unterstützt")
}

func (m *execMem) free() {}
