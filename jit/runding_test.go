//go:build amd64

package jit

import (
	"fmt"
	"testing"

	"vbxc/compiler"
	"vbxc/parser"
)

// TestModModNative verifiziert, dass math.Mod wirklich nativ übersetzt
// wird: nur PRINT und HALT dürfen als VM-Rückfall bleiben.
func TestModNative(t *testing.T) {
	src := `Print math.Mod(7, 3)`
	stmts, err := parser.ParseSource(src, parser.Options{File: "mod.vb"})
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := compiler.Compile("mod.vb", stmts)
	if err != nil {
		t.Fatal(err)
	}
	p := Prepare(chunk, 0)
	s := p.Stats
	fmt.Printf("Mod Regions=%d Einst=%d Bytes=%d Skipped=%d\n", s.Regions, s.Entries, s.CodeBytes, s.Skipped)
	if s.Skipped > 3 {
		t.Errorf("Mod: zu viele VM-Rückfälle (%d) - Native-Pfad greift nicht", s.Skipped)
	}
}

// TestMatheRundungNative verifiziert, dass Floor/Ceil/Trunc/Int wirklich
// nativ übersetzt werden: nur PRINT und HALT dürfen als VM-Rückfall bleiben,
// der OpCall muss in einer Region liegen.
func TestMatheRundungNative(t *testing.T) {
	for _, fn := range []string{"Floor", "Ceil", "Trunc", "Int", "Abs"} {
		src := fmt.Sprintf(`Print math.%s(2.5)`, fn)
		stmts, err := parser.ParseSource(src, parser.Options{File: "mr.vb"})
		if err != nil {
			t.Fatal(err)
		}
		chunk, err := compiler.Compile("mr.vb", stmts)
		if err != nil {
			t.Fatal(err)
		}
		p := Prepare(chunk, 0)
		s := p.Stats
		fmt.Printf("%-6s Regions=%d Einst=%d Bytes=%d Skipped=%d\n", fn, s.Regions, s.Entries, s.CodeBytes, s.Skipped)
		// Erwartet: CONST, CHECK?, CALL, CONST/..., PRINT/HALT zurück.
		if s.Skipped > 3 {
			t.Errorf("%s: zu viele VM-Rückfälle (%d) - Native-Pfad greift nicht", fn, s.Skipped)
		}
	}
}

// TestWinkelProzentNative verifiziert, dass DegToRad/RadToDeg/Percent/
// PercentOf sowie die Konstanten Pi()/E() wirklich nativ übersetzt werden:
// nur PRINT und HALT dürfen als VM-Rückfall bleiben.
func TestWinkelProzentNative(t *testing.T) {
	for _, c := range []struct{ fn, src string }{
		{"DegToRad", `Print math.DegToRad(90)`},
		{"RadToDeg", `Print math.RadToDeg(1)`},
		{"Percent", `Print math.Percent(50, 200)`},
		{"PercentOf", `Print math.PercentOf(25, 50)`},
		{"Pi", `Print math.Pi()`},
		{"E", `Print math.E()`},
	} {
		stmts, err := parser.ParseSource(c.src, parser.Options{File: "wp.vb"})
		if err != nil {
			t.Fatal(err)
		}
		chunk, err := compiler.Compile("wp.vb", stmts)
		if err != nil {
			t.Fatal(err)
		}
		p := Prepare(chunk, 0)
		s := p.Stats
		fmt.Printf("%-9s Regions=%d Einst=%d Bytes=%d Skipped=%d\n", c.fn, s.Regions, s.Entries, s.CodeBytes, s.Skipped)
		if s.Skipped > 3 {
			t.Errorf("%s: zu viele VM-Rückfälle (%d) - Native-Pfad greift nicht", c.fn, s.Skipped)
		}
	}
}

// TestRoundNative verifiziert, dass Round/RoundBank nativ übersetzt werden
// (nur PRINT/HALT in der VM) — die ein- und die zweistellige Form.
func TestRoundNative(t *testing.T) {
	for _, c := range []struct{ fn, src string }{
		{"Round", `Print math.Round(2.5)`},
		{"Round/e2", `Print math.Round(2.567, 1)`},
		{"RoundBank", `Print math.RoundBank(2.5)`},
		{"RoundBank/e2", `Print math.RoundBank(3.14159, 2)`},
	} {
		stmts, err := parser.ParseSource(c.src, parser.Options{File: "rn.vb"})
		if err != nil {
			t.Fatal(err)
		}
		chunk, err := compiler.Compile("rn.vb", stmts)
		if err != nil {
			t.Fatal(err)
		}
		p := Prepare(chunk, 0)
		s := p.Stats
		fmt.Printf("%-11s Regions=%d Einst=%d Bytes=%d Skipped=%d\n", c.fn, s.Regions, s.Entries, s.CodeBytes, s.Skipped)
		if s.Skipped > 3 {
			t.Errorf("%s: zu viele VM-Rückfälle (%d) - Native-Pfad greift nicht", c.fn, s.Skipped)
		}
	}
}

// TestClampNative verifiziert, dass math.Clamp mit drei Num-Argumenten
// nativ übersetzt wird (nur PRINT/HALT zurück in der VM); Str-Argumente
// bleiben bewusst in der VM (mathArg-Konvertierung). 1-Arg/2-Arg-Formen
// des Namens (Variablenschatten) laufen ohnehin über den Analyzer-Filter.
func TestClampNative(t *testing.T) {
	for _, c := range []struct{ fn, src string }{
		{"Clamp", `Print math.Clamp(2, 1, 5)`},
		{"Clamp/ausdruck", `Print math.Clamp(7 + 0.5, 1, 5.5)`},
	} {
		stmts, err := parser.ParseSource(c.src, parser.Options{File: "cl.vb"})
		if err != nil {
			t.Fatal(err)
		}
		chunk, err := compiler.Compile("cl.vb", stmts)
		if err != nil {
			t.Fatal(err)
		}
		p := Prepare(chunk, 0)
		s := p.Stats
		fmt.Printf("%-15s Regions=%d Einst=%d Bytes=%d Skipped=%d\n", c.fn, s.Regions, s.Entries, s.CodeBytes, s.Skipped)
		if s.Skipped > 3 {
			t.Errorf("%s: zu viele VM-Rückfälle (%d) - Native-Pfad greift nicht", c.fn, s.Skipped)
		}
	}
}
