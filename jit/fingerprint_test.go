//go:build amd64

package jit

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vbxc/bytecode"
	"vbxc/compiler"
	"vbxc/parser"
)

// kompiliert lädt und kompiliert ein Testskript.
func kompiliert(t *testing.T, src string) *bytecode.Chunk {
	t.Helper()
	stmts, err := parser.ParseSource(src, parser.Options{File: "ck.vb"})
	if err != nil {
		t.Fatalf("Parser: %v", err)
	}
	c, err := compiler.Compile("ck.vb", stmts)
	if err != nil {
		t.Fatalf("Compiler: %v", err)
	}
	return c
}

// TestCodegenFingerprintDeterministisch: der Fingerabdruck ist ungleich null
// und über Läufe hinweg identisch (Voraussetzung für stabile Cache-Keys).
func TestCodegenFingerprintDeterministisch(t *testing.T) {
	a := CodegenFingerprint()
	b := CodegenFingerprint()
	if a != b {
		t.Fatal("Codegen-Fingerprint ist nicht deterministisch")
	}
	var zero [32]byte
	if a == zero {
		t.Fatal("Codegen-Fingerprint ist leer")
	}
}

// TestCodegenDeterminismus: zweimal Prepare über frisch kompilierte Chunks
// derselben Quelle liefert byteidentischen Maschinencode. Nötig für
// Cache-Korrektheit und Fingerabdruck-Stabilität.
func TestCodegenDeterminismus(t *testing.T) {
	c := kompiliert(t, canarySrc)
	p1 := Prepare(c, 0)
	if p1 == nil {
		t.Fatal("Prepare ergab nichts (1)")
	}
	p2 := Prepare(c, 0)
	if p2 == nil {
		t.Fatal("Prepare ergab nichts (2)")
	}
	if !bytes.Equal(p1.cacheCode, p2.cacheCode) {
		t.Fatal("Codegen ist nicht deterministisch: zweiter Durchlauf weicht ab")
	}
}

// TestPrepareCachedRundreise: schreiben und neu laden einer Cache-Datei
// ergibt ein Programm mit identischem Code; ein geänderter Key gilt als
// Ungültigkeitsfall (Miss statt Hit).
func TestPrepareCachedRundreise(t *testing.T) {
	c := kompiliert(t, canarySrc)
	pfad := filepath.Join(t.TempDir(), "prog.vbb.cache")

	key := CacheKey([]byte("quelle"))
	p, hit := PrepareCached(c, pfad, key, 0)
	if hit {
		t.Fatal("erster Aufruf darf keinen Cache-Hit liefern")
	}
	written, err := os.ReadFile(pfad)
	if err != nil {
		t.Fatalf("Cache-Datei wurde nicht geschrieben: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("Cache-Datei ist leer")
	}

	p2, hit2 := PrepareCached(c, pfad, key, 0)
	if !hit2 {
		t.Fatal("zweiter Aufruf sollte aus dem Cache laden")
	}
	if !bytes.Equal(p.cacheCode, p2.cacheCode) {
		t.Fatal("geladener Code weicht vom erzeugten ab")
	}

	fremd := CacheKey([]byte("andere Quelle"))
	andererPfad := filepath.Join(t.TempDir(), "fremd.vbb.cache")
	if _, hit := PrepareCached(c, andererPfad, fremd, 0); hit {
		t.Fatal("fremder Key darf keinen Hit liefern")
	}
}

// TestMachineFileVersionAblehnung: eine VBXM-Datei mit alter Versionsnummer
// (hier künstlich als Version 1 verklebt) wird abgelehnt - Maschinencode aus
// einer Codegen-Welt vor einem Versionsbump läuft nicht still weiter.
func TestMachineFileVersionAblehnung(t *testing.T) {
	c := kompiliert(t, canarySrc)
	p := Prepare(c, 0)
	if p == nil {
		t.Fatal("Prepare ergab nichts")
	}
	sp, ok := p.Saved()
	if !ok {
		t.Fatal("Saved lieferte nichts")
	}
	var buf bytes.Buffer
	buf.WriteString("VBXM")
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if err := gob.NewEncoder(zw).Encode(machineFile{
		Version: 1, // alte Version
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		Chunk:   c,
		Code:    sp.Code,
		Entries: sp.Entries,
		Stats:   sp.Stats,
	}); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, err = LoadRunFile(&buf)
	if err == nil {
		t.Fatal("Version-1-Maschinencode-Datei wurde nicht abgelehnt")
	}
	if !strings.Contains(err.Error(), "Maschinencode-Version 1") {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
}

// TestMachineFileRundreise: eine korrekt versionierte Maschinencode-Datei
// lädt zurück und enthält denselben Code.
func TestMachineFileRundreise(t *testing.T) {
	c := kompiliert(t, canarySrc)
	p := Prepare(c, 0)
	if p == nil {
		t.Fatal("Prepare ergab nichts")
	}
	soll, ok := p.Saved()
	if !ok {
		t.Fatal("Saved lieferte nichts")
	}
	var buf bytes.Buffer
	if _, ok, err := WriteMachineFile(&buf, runtime.GOOS, runtime.GOARCH, c); err != nil || !ok {
		t.Fatalf("WriteMachineFile: ok=%v err=%v", ok, err)
	}
	_, prog, err := LoadRunFile(&buf)
	if err != nil {
		t.Fatalf("LoadRunFile: %v", err)
	}
	if prog == nil || !bytes.Equal(prog.cacheCode, soll.Code) {
		t.Fatal("geladener Maschinencode weicht ab")
	}
}
