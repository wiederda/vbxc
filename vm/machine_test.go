package vm

import (
	"bytes"
	"runtime"
	"strconv"
	"testing"

	"vbxc/bytecode"
	"vbxc/compiler"
	"vbxc/jit"
	"vbxc/parser"
)

// kompiliere ist die Testhilfe für die Maschinencode-Datei-Tests.
func kompiliere(t *testing.T, src string) *bytecode.Chunk {
	t.Helper()
	stmts, err := parser.ParseSource(src, parser.Options{File: "machine_test.vb"})
	if err != nil {
		t.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("machine_test.vb", stmts)
	if err != nil {
		t.Fatalf("Compiler: %v", err)
	}
	return chunk
}

// TestMaschinencodeDateiRoundtrip übersetzt einen Chunk in eine
// Maschinencode-Datei, lädt sie zurück und führt sie aus. Ergebnis muss
// identisch zur Interpretation sein.
func TestMaschinencodeDateiRoundtrip(t *testing.T) {
	src := `
Dim i
Dim summe
summe = 0
For i = 1 To 2000
    summe = summe + i * 2 - 3
Next
Print "summe=" & summe
Print "text " & (7 < 8)
`
	chunk := kompiliere(t, src)

	var buf bytes.Buffer
	if _, ok, err := jit.WriteMachineFile(&buf, runtime.GOOS, runtime.GOARCH, chunk); err != nil || !ok {
		t.Fatalf("WriteMachineFile: ok=%v err=%v", ok, err)
	}

	c, prog, err := jit.LoadRunFile(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadRunFile: %v", err)
	}
	if prog == nil {
		t.Fatal("LoadRunFile: kein Programm geliefert")
	}
	if !bytes.Equal([]byte(fmtChunkCode(c)), []byte(fmtChunkCode(chunk))) {
		t.Fatal("Chunk aus Datei weicht vom Original ab")
	}

	var out bytes.Buffer
	v := New()
	v.Out = &out
	v.SetJIT(prog)
	if err := v.Run(c); err != nil {
		t.Fatalf("Run (Maschinencode): %v", err)
	}

	// Referenz: interpretiert
	var ref bytes.Buffer
	v2 := New()
	v2.Out = &ref
	if err := v2.Run(chunk); err != nil {
		t.Fatalf("Run (interpretiert): %v", err)
	}
	if out.String() != ref.String() {
		t.Errorf("Ausgabe weicht ab:\n--- Maschinencode ---\n%s\n--- interpretiert ---\n%s", out.String(), ref.String())
	}
}

// fmtChunkCode gibt die Instruktionen des Chunks (Hauptprogramm + Funcs)
// als vergleichbare Zeichenkette zurück.
func fmtChunkCode(c *bytecode.Chunk) string {
	var s string
	for _, in := range c.Code {
		s += strconv.Itoa(int(in.Op)) + "|" + strconv.Itoa(int(in.A)) + "|" + strconv.Itoa(int(in.B)) + ";"
	}
	for _, f := range c.Funcs {
		for _, in := range f.Code {
			s += strconv.Itoa(int(in.Op)) + "|" + strconv.Itoa(int(in.A)) + "|" + strconv.Itoa(int(in.B)) + ";"
		}
	}
	return s
}

// TestMaschinencodeDateiPlattformMismatch: eine für ein fremdes OS
// erzeugte Datei wird abgelehnt.
func TestMaschinencodeDateiPlattformMismatch(t *testing.T) {
	fremd := "windows"
	if runtime.GOOS == "windows" {
		fremd = "linux"
	}
	chunk := kompiliere(t, "Print 1"+`
Dim i
For i = 1 To 10
    Print i
Next
`)
	var buf bytes.Buffer
	if _, ok, err := jit.WriteMachineFile(&buf, fremd, "amd64", chunk); err != nil || !ok {
		t.Fatalf("WriteMachineFile: ok=%v err=%v", ok, err)
	}

	_, _, err := jit.LoadRunFile(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("fremde Plattform wurde akzeptiert")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("läuft nicht auf")) {
		t.Errorf("unerwartete Fehlermeldung: %v", err)
	}
}
