//go:build amd64

package jit

import (
	"fmt"
	"testing"

	"vbxc/bytecode"
	"vbxc/compiler"
	"vbxc/parser"
)

// nichtÜbersetzte kompiliert ein Skript und listet die nicht übersetzbaren
// Instruktionen (die im VM-Loop laufen). Hilft, Freischalt-Kandidaten für
// den Codegenerator zu finden.
func nichtÜbersetzte(t *testing.T, src string) string {
	t.Helper()
	stmts, err := parser.ParseSource(src, parser.Options{File: "coverage.vb"})
	if err != nil {
		t.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("coverage.vb", stmts)
	if err != nil {
		t.Fatalf("Compiler: %v", err)
	}
	var out []string
	list := func(label string, code []bytecode.Instr, pos []bytecode.Pos) {
		ci := analyze(chunk, code)
		for i, in := range code {
			if ci.el[i] {
				continue
			}
			line := 0
			if pos != nil && i < len(pos) {
				line = int(pos[i].Line)
			}
			out = append(out, fmt.Sprintf("%-8s ip=%d %v A=%d B=%d C=%d (Zeile %d)", label, i, in.Op, in.A, in.B, in.C, line))
		}
	}
	list("main", chunk.Code, chunk.Pos)
	for _, f := range chunk.Funcs {
		list(f.Name, f.Code, f.Pos)
	}
	return fmt.Sprint(out)
}

// functionDetail drückt die Prozedur-Opcodes aus chunk.Funcs als lesbaren
// Namen (der Opcode trägt den Funktionsindex in A).
func funcName(chunk *bytecode.Chunk, idx int32) string {
	if int(idx) < len(chunk.Funcs) {
		return chunk.Funcs[idx].Name
	}
	return fmt.Sprintf("func#%d", idx)
}

func TestCoverageProbe(t *testing.T) {
	progs := map[string]string{
		"schleife": `
Function Slow(n)
    Dim s = 0
    For i = 1 To n
        s = s + i
    Next i
    Return s
End Function

Print Slow(100000)
`,
		"mix": `
Dim arr(5)
For i = 0 To 4
    arr(i) = i * 2
Next
Dim t = 0
For Each x In arr
    t = t + x
Next
Print arr(2)
Print "x" & 1
Print t
`,
		"matrix": `
Dim m(19, 19)
Dim i
Dim j
For i = 0 To 19
    For j = 0 To 19
        m(i, j) = (i + 1) * (j + 2)
    Next
Next
Dim s = 0
For i = 0 To 19
    For j = 0 To 19
        s = s + m(i, j)
    Next
Next
Print s
`,
	}
	for name, src := range progs {
		if got := nichtÜbersetzte(t, src); got != "" {
			fmt.Printf("=== %s ===\n%s\n", name, got)
		}
	}
}
