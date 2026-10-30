//go:build !amd64

// Fallback für alles außer x86-64: es wird nichts übersetzt, die VM arbeitet
// interpretiert (identische Semantik, nur langsamer).
package jit

import (
	"fmt"

	"vbxc/bytecode"
)

func compile(p *Program, c *bytecode.Chunk) error {
	return fmt.Errorf("jit: Architektur wird nicht unterstützt")
}
