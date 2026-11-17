//go:build amd64

package jit

import (
	"testing"

	"vbxc/bytecode"
	"vbxc/compiler"
	"vbxc/parser"
	"vbxc/value"
)

// TestStringKonstanteNative: String-Literale werden im Maschinencode als
// RIP-relative Zeiger auf eingebettete Bytes abgelegt und sind keine
// Regionsgrenze mehr.
func TestStringKonstanteNative(t *testing.T) {
	stmts, err := parser.ParseSource(`Println "hallo welt"`, parser.Options{File: "str.vb"})
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := compiler.Compile("str.vb", stmts)
	if err != nil {
		t.Fatal(err)
	}

	ci := analyze(chunk, chunk.Code)
	found := false
	for i, in := range chunk.Code {
		if in.Op != bytecode.OpConst || in.A < 0 || int(in.A) >= len(chunk.Consts) {
			continue
		}
		if chunk.Consts[in.A].Kind != value.KindStr {
			continue
		}
		found = true
		if !ci.el[i] {
			t.Fatalf("String-Konstante an IP %d ist nicht nativ unterstützt", i)
		}
	}
	if !found {
		t.Fatal("keine String-Konstante im Bytecode gefunden")
	}

	p := Prepare(chunk, 0)
	if p == nil {
		t.Fatal("Prepare lieferte kein Programm")
	}
	defer p.Close()
	if p.Stats.Skipped > 2 { // PRINTLN + HALT
		t.Errorf("zu viele VM-Rückfälle: %d (erwartet <= 2)", p.Stats.Skipped)
	}
}
