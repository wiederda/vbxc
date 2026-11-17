//go:build amd64

package jit

import (
	"testing"
	"unsafe"

	"vbxc/bytecode"
	"vbxc/compiler"
	"vbxc/parser"
	"vbxc/value"
)

// emptyEnv reicht für Regionen ohne Variablenzugriffe.
type emptyEnv struct{}

func (emptyEnv) VarRef(string) (*value.Value, bool, bool) { return nil, false, false }
func (emptyEnv) LocalRef(string) (*value.Value, bool)     { return nil, false }

func mustConcatChunk(t *testing.T, src string) *bytecode.Chunk {
	t.Helper()
	stmts, err := parser.ParseSource(src, parser.Options{File: "concat.vb"})
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := compiler.Compile("concat.vb", stmts)
	if err != nil {
		t.Fatal(err)
	}
	return chunk
}

func concatIPs(chunk *bytecode.Chunk) []int {
	var ips []int
	for i := range chunk.Code {
		if chunk.Code[i].Op == bytecode.OpConcat {
			ips = append(ips, i)
		}
	}
	return ips
}

func firstEntry(p *Program, code []bytecode.Instr) *Entry {
	for _, e := range p.TableFor(code) {
		if e != nil {
			return e
		}
	}
	return nil
}

func inArena(p *Program, s string) bool {
	if p.arena == nil || len(p.arena.buf) == 0 {
		return false
	}
	base := uintptr(unsafe.Pointer(&p.arena.buf[0]))
	ptr := uintptr(unsafe.Pointer(unsafe.StringData(s)))
	return ptr >= base && ptr+uintptr(len(s)) <= base+uintptr(len(p.arena.buf))
}

// TestStringKonkatNative: Zwei String-Operanden werden ohne VM-Aufruf im
// vorab angelegten Arena-Speicher verkettet; der Maschinencode adressiert das
// Ergebnis als normalen Go-String.
func TestStringKonkatNative(t *testing.T) {
	chunk := mustConcatChunk(t, `Dim a = "x" & "y"`)
	ips := concatIPs(chunk)
	if len(ips) != 1 {
		t.Fatalf("CONCAT-Instruktionen: %d (erwartet 1)", len(ips))
	}
	ci := analyze(chunk, chunk.Code)
	if !ci.el[ips[0]] {
		t.Fatal("CONCAT ist nicht als nativ unterstützt markiert")
	}

	p := Prepare(chunk, 0)
	if p == nil {
		t.Fatal("Prepare lieferte kein Programm")
	}
	defer p.Close()
	if p.arena == nil {
		t.Fatal("kein String-Arena angelegt")
	}
	e := firstEntry(p, chunk.Code)
	if e == nil {
		t.Fatal("kein Einstieg gefunden")
	}

	stk := make([]value.Value, 0, 8)
	ip := e.Run(emptyEnv{}, &stk)
	if len(stk) != 1 {
		t.Fatalf("Stacktiefe nach CONCAT: %d (erwartet 1)", len(stk))
	}
	if stk[0].Kind != value.KindStr || stk[0].Str != "xy" {
		t.Fatalf("Ergebnis: %#v (erwartet Str \"xy\")", stk[0])
	}
	if !inArena(p, stk[0].Str) {
		t.Error("Ergebnis zeigt nicht in das String-Arena")
	}
	if p.arena.off != 2 {
		t.Errorf("Arena-Offset: %d (erwartet 2)", p.arena.off)
	}
	if ip <= int32(ips[0]) {
		t.Errorf("Resume-IP %d liegt nicht hinter CONCAT %d", ip, ips[0])
	}
}

// TestStringKonkatArenaVoll: Ist das Arena zu klein, deoptet nur der
// betroffene CONCAT. Bereits erfolgreich verkettete Strings bleiben gültig,
// der Offset wird korrekt fortgeschrieben.
func TestStringKonkatArenaVoll(t *testing.T) {
	chunk := mustConcatChunk(t, `Dim d = ("a" & "b") & "cdefg"`)
	ips := concatIPs(chunk)
	if len(ips) != 2 {
		t.Fatalf("CONCAT-Instruktionen: %d (erwartet 2)", len(ips))
	}

	p := Prepare(chunk, 0)
	if p == nil {
		t.Fatal("Prepare lieferte kein Programm")
	}
	defer p.Close()

	full := p.arena.buf
	p.arena.buf = full[:4] // 2 Bytes reichen für "ab", nicht mehr für "abcdefg"
	e := firstEntry(p, chunk.Code)
	if e == nil {
		t.Fatal("kein Einstieg gefunden")
	}

	stk := make([]value.Value, 0, 8)
	ip := e.Run(emptyEnv{}, &stk)
	if ip != int32(ips[1]) {
		t.Fatalf("Resume-IP: %d (erwartet CONCAT %d)", ip, ips[1])
	}
	if len(stk) != 2 {
		t.Fatalf("Stacktiefe nach Deopt: %d (erwartet 2)", len(stk))
	}
	if stk[0].Kind != value.KindStr || stk[0].Str != "ab" {
		t.Fatalf("linker Operand: %#v (erwartet \"ab\")", stk[0])
	}
	if !inArena(p, stk[0].Str) {
		t.Error("bereits erzeugter String zeigt nicht in das Arena")
	}
	if stk[1].Kind != value.KindStr || stk[1].Str != "cdefg" {
		t.Fatalf("rechter Operand: %#v (erwartet \"cdefg\")", stk[1])
	}
	if p.arena.off != 2 {
		t.Errorf("Arena-Offset nach Deopt: %d (erwartet 2)", p.arena.off)
	}

	// Mit vollem Arena läuft dieselbe Region vollständig nativ durch.
	p.arena.buf = full
	p.arena.off = 0
	stk = make([]value.Value, 0, 8)
	ip = e.Run(emptyEnv{}, &stk)
	if ip == int32(ips[1]) {
		t.Fatal("CONCAT ist trotz ausreichendem Arena deoptet")
	}
	if len(stk) != 1 {
		t.Fatalf("Stacktiefe: %d (erwartet 1)", len(stk))
	}
	if stk[0].Kind != value.KindStr || stk[0].Str != "abcdefg" {
		t.Fatalf("Ergebnis: %#v (erwartet Str \"abcdefg\")", stk[0])
	}
	if !inArena(p, stk[0].Str) {
		t.Error("Ergebnis zeigt nicht in das String-Arena")
	}
	if p.arena.off != 9 {
		t.Errorf("Arena-Offset: %d (erwartet 9)", p.arena.off)
	}
}

// TestStringKonkatInstall: Auch der Weg über gespeicherten Maschinencode
// (Cache/.vbb) legt das Arena an und führt CONCAT nativ aus.
func TestStringKonkatInstall(t *testing.T) {
	chunk := mustConcatChunk(t, `Dim d = "a" & "b"`)
	p := Prepare(chunk, 0)
	if p == nil {
		t.Fatal("Prepare lieferte kein Programm")
	}
	saved, ok := p.Saved()
	p.Close()
	if !ok {
		t.Fatal("Saved lieferte kein Programm")
	}

	q, err := Install(chunk, saved, 0)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	defer q.Close()
	if q.arena == nil {
		t.Fatal("Install hat kein String-Arena angelegt")
	}
	e := firstEntry(q, chunk.Code)
	if e == nil {
		t.Fatal("kein Einstieg gefunden")
	}
	stk := make([]value.Value, 0, 8)
	e.Run(emptyEnv{}, &stk)
	if len(stk) != 1 || stk[0].Kind != value.KindStr || stk[0].Str != "ab" {
		t.Fatalf("Ergebnis nach Install: %#v", stk)
	}
	if !inArena(q, stk[0].Str) {
		t.Error("Ergebnis zeigt nicht in das String-Arena")
	}
}

// TestStringKonkatDeoptNichtString: Nicht-String-Operanden müssen vor jeder
// Stack-Veränderung in die VM zurückkehren (ToString-Semantik).
func TestStringKonkatDeoptNichtString(t *testing.T) {
	chunk := mustConcatChunk(t, `Dim c = "x" & 1`)
	ips := concatIPs(chunk)
	if len(ips) != 1 {
		t.Fatalf("CONCAT-Instruktionen: %d (erwartet 1)", len(ips))
	}

	p := Prepare(chunk, 0)
	if p == nil {
		t.Fatal("Prepare lieferte kein Programm")
	}
	defer p.Close()

	e := firstEntry(p, chunk.Code)
	if e == nil {
		t.Fatal("kein Einstieg gefunden")
	}
	stk := make([]value.Value, 0, 8)
	ip := e.Run(emptyEnv{}, &stk)
	if ip != int32(ips[0]) {
		t.Fatalf("Resume-IP: %d (erwartet CONCAT %d)", ip, ips[0])
	}
	if len(stk) != 2 {
		t.Fatalf("Stacktiefe: %d (erwartet 2)", len(stk))
	}
	if stk[0].Str != "x" || stk[1].Kind != value.KindNum || stk[1].Num != 1 {
		t.Fatalf("Operanden verändert: %#v / %#v", stk[0], stk[1])
	}
	if p.arena.off != 0 {
		t.Errorf("Arena wurde trotz Deopt belegt: %d", p.arena.off)
	}
}
