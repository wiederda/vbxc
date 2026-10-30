//go:build amd64

// Automatische Codegen-Invalidierung für den Laufcache: Eine Kanarien-Region
// wird mit dem echten Codegenerator übersetzt, der Fingerabdruck (SHA-256 über
// erzeugten Maschinencode und Einstiegstabelle) fließt in den Cache-Key ein.
// Jede (Kanarien-sichtbare) Änderung am Parser, Compiler oder Codegenerator
// verändert so den Key - alte Cache-Dateien verlieren ihr Gültigkeit von
// selbst, ohne dass eine Versionskonstante von Hand angehoben werden muss.
package jit

import (
	"crypto/sha256"
	"encoding/binary"
	"sync"

	"vbxc/compiler"
	"vbxc/parser"
)

// canarySrc: bewusst breite Deckung der nativen Opgattungen - Variablenzellen,
// Arithmetik, Vergleiche (auch OpEq/OpNeq), Boolesche Logik, Verzweigungen
// und For-Schleifen, 1D-/2D-Array-Guards, Aufrufe der spezialisierten
// math.*-Builtins. Nicht abgedeckte Codegen-Änderungen invalidieren den Cache
// über die Versionkonstanten (cacheVersion/machineFileVersion in jit.go und
// file.go), die bei absichtlichen Generierungsänderungen mit angehoben werden.
const canarySrc = `
Dim a = 7
Dim b = -3.5
Dim s = a * 3 + b / 7
If a > b And s <= 21 Or Not (a = 0) Then
    s = s + 25
Else
    s = s - 1
End If
Dim i
For i = 1 To 5
    s = s + math.Abs(i) + math.Sign(b) + math.Sqrt(a)
    s = s + math.Floor(i / 2) + math.Ceil(i / 3) + math.Trunc(i / 4)
    s = s + math.DegToRad(i) + math.RadToDeg(i)
<<<<<<< HEAD
    s = s + math.Mod(i, 7) + math.Percent(i, 4) + math.PercentOf(i, 2 * i)
    s = s + math.Round(i * 0.7) + math.RoundBank(i * 1.3)
    s = s + math.Clamp(i, 1, 4)
=======
>>>>>>> 1a75fee5075b59ae56e731fda08bf18a063c2ad9
    If a = b Then
        s = s * 2
    End If
    If a <> b Then
        s = s + 1
    End If
Next
s = s + math.Pi() + math.E()
Dim arr(9)
arr(3) = 12
s = s + arr(3)
Dim m(4, 4)
m(1, 2) = 9
s = s + m(1, 2)
Print s
`

var (
	codegenOnce sync.Once
	codegenFP   [32]byte
)

// CodegenFingerprint liefert den SHA-256 des Kanarien-Maschinencodes. Er ist
// Teil des Cache-Keys (CacheKey) und damit der automatische Invalidierungs-
// hebel für Generierungsänderungen.
func CodegenFingerprint() [32]byte {
	codegenOnce.Do(func() {
		fp, err := computeCodegenFingerprint()
		if err != nil {
			// Ohne Fingerabdruck wäre der Cache-Key nicht mehr
			// versions-sicher; das ist ein Invariantenbruch und darf
			// laut versagen, nicht still weiterlaufen (staler Code
			// wäre die Folge).
			panic("jit: Codegen-Fingerprint konnte nicht berechnet werden: " + err.Error())
		}
		codegenFP = fp
	})
	return codegenFP
}

func computeCodegenFingerprint() ([32]byte, error) {
	stmts, err := parser.ParseSource(canarySrc, parser.Options{File: "jit-canary.vb"})
	if err != nil {
		return [32]byte{}, err
	}
	chunk, err := compiler.Compile("jit-canary.vb", stmts)
	if err != nil {
		return [32]byte{}, err
	}
	p := Prepare(chunk, 0)
	if p == nil {
		return [32]byte{}, err
	}
	h := sha256.New()
	h.Write(p.cacheCode)
	buf := make([]byte, 4)
	for _, e := range p.cacheEntries {
		binary.LittleEndian.PutUint32(buf, uint32(e.StubOffset))
		h.Write(buf)
		binary.LittleEndian.PutUint32(buf, uint32(e.IP))
		h.Write(buf)
		binary.LittleEndian.PutUint32(buf, uint32(e.DEntry))
		h.Write(buf)
		binary.LittleEndian.PutUint32(buf, uint32(e.MaxRel))
		h.Write(buf)
		for _, n := range e.Names {
			h.Write([]byte(n))
			h.Write([]byte{0})
		}
		for _, l := range e.Local {
			if l {
				h.Write([]byte{1})
			} else {
				h.Write([]byte{0})
			}
		}
	}
	var fp [32]byte
	copy(fp[:], h.Sum(nil))
	return fp, nil
}
