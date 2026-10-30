package vm

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"

	"vbxc/compiler"
	"vbxc/jit"
	"vbxc/parser"
)

// jitProgramme beanspruchen beide Pfade: Schleifen, Vergleiche, Verzweigungen,
// Fehlerwerte, Konkatenation, Funktionen. Alle Ausgaben müssen byte-identisch
// sein - der JIT ändert nie die Semantik, nur die Geschwindigkeit.
var jitProgramme = map[string]string{
	"arithmetik": `
Dim i
Dim summe
summe = 0
For i = 1 To 1000
    summe = summe + i * 2 - 3
Next
Print "summe=" & summe
Print 10 / 4
Print -7
Print 0 - 0
Print 1 + 2 * 3
Print 3.5 * 2
`,
	"vergleiche": `
Dim a
Dim b
a = 0
b = 20
Print a < b
Print b < a
Print a = b
Print a <= b
Print a >= b
Print a <> b
Print 1 = "1"
Print "a" = "a"
Print "a" <> "a"
Print Not (3 > 5)
Print (5 > 3) And Not (2 > 4)
Print (0 = 0) Or (9 < 1)
`,
	"gleichheit-bool-null": `
Dim a = True
Dim b = False
Print a = True
Print a = False
Print a <> False
Print b <> b
Print a = b
Dim d
Print d = d
Print a = 1
Print a = "true"
`,
	"gleichheit-str": `
Dim a
Dim b
a = "hallo welt abcdef"
b = "hallo welt abcdef"
Print a = b
Print a <> b
b = "hallo welt abcdeg"
Print a = b
b = "hallo welt abcdefgh"
Print a = b
b = "hallo welt"
Print a = b
Print b = a
b = "hallo welt abcdef"
Print a <> b
Print "" = ""
a = ""
b = ""
Print a = b
Print a = "x"
Print a <> "x"
`,
	"pfade": `
Dim j
j = 0
While j < 20
    If (j / 2) * 2 = j Then
        Print "gerade " & j
    Else
        Print "ungerade " & j
    End If
    j = j + 1
End While

Dim t = 0
Do
    t += 1
Loop Until t >= 3
Print "t=" & t

Dim n = 10
While n > 0
    n -= 3
End While
Print "n=" & n

Dim c = 0
Do
    c += 1
    If c = 2 Then
        Continue Do
    End If
    If c > 4 Then
        Exit Do
    End If
    Print "c=" & c
Loop
`,
	"matrix": `
Dim m(9, 9)
Dim i
Dim j
For i = 0 To 9
    For j = 0 To 9
        m(i, j) = (i + 1) * (j + 2)
    Next
Next
Dim s = 0
For i = 0 To 9
    For j = 0 To 9
        s = s + m(i, j)
    Next
Next
Print s
Print m(3, 4)
Print m(0, 0)
m(1, 1) = m(2, 1) + 100
Print m(1, 1)
Print m(1.7, 2.9)
`,
	"weiter": `
Const LIMIT = 5
Dim total = 0
Dim i
For i = 1 To LIMIT
    If i = 3 Then
        Continue For
    End If
    total += i
Next
Print "total=" & total

Dim f = 0
For i = 10 To 1 Step -3
    f += i
Next
Print "f=" & f

Dim s = ""
For i = 1 To 5
    s = s & "x"
Next
Print s
`,
	"funktionen": `
Function doppel(x)
    Return x * 2
End Function

Function Fak(n)
    If n <= 1 Then
        Return 1
    End If
    Return n * Fak(n - 1)
End Function

Sub zaehle(n)
    Dim k
    For k = 1 To n
        Print "k=" & k
    Next
End Sub

Print doppel(21)
Print Fak(10)
zaehle(4)
`,
	"fehler": `
Print IsError(1 / 0)
Print ErrorText(1 / 0)
Print IsError(undefinedVar)
Print ErrorText(undefinedVar & "x")
Print IsError(Left("abc", undefinedVar))
`,
	// "wechsel" zwingt teils die Interpretation: Arrays und Maps deopten,
	// Strings bei CaseEq ebenfalls. Die Ausgabe muss trotzdem identisch
	// sein (Deopt-Guards).
	"wechsel": `
Dim a(2)
a(0) = 3
a(1) = 1
a(2) = 2
Dim i
Dim s = ""
For i = 0 To 2
    s = s & a(i)
Next
Print s
Print array.Count(a)

Dim m = map.Create()
map.Set(m, "x", 42)
Print map.Get(m, "x")
Print map.Get(m, "fehlt")

Dim v = 2
Select Case v
    Case 1
        Print "eins"
    Case 2
        Print "zwei"
    Case Else
        Print "anders"
End Select

Dim t = 0
Do
    t += 1
Loop Until t >= 3
Print "t=" & t
`,
	// "selectcase" deckt alle Critical Paths des linearen Scans nativ ab:
	// Dispatch (Zahlen), Bool-Durchlauf, CaseRange, CaseIs (alle Operatoren
	// inkl. NaN-Fall), CaseEq (Bool/Null), Strings (Deopt) und IsError.
	"selectcase": `
Dim x = 5
Select Case x
    Case 1
        Print "eins"
    Case 2 To 8
        Print "zwei bis acht"
    Case Else
        Print "anders"
End Select

x = 12
Select Case x
    Case Is > 10
        Print "gross"
    Case Is > 100
        Print "riesig"
    Case Else
        Print "klein"
End Select

x = 3
Select Case x
    Case Is <= 0
        Print "kein"
    Case Is <> 0
        Print "ungleich null"
End Select

x = 0
Select Case x
    Case Is <> 0
        Print "ungleich null"
    Case Else
        Print "null"
End Select

Dim b = True
Select Case b
    Case True
        Print "wahr"
    Case False
        Print "unwahr"
End Select

Select Case x
    Case "a"
        Print "a"
    Case 3
        Print "drei"
    Case Else
        Print "x"
End Select

Dim s2 = "hallo"
Select Case s2
    Case "HALLO"
        Print "gross"
    Case "x"
        Print "x"
    Case Else
        Print "keins"
End Select

Print IsError(1 / 0)   ' true
Print IsError(42)
Print IsError("text")

Dim lo = 2
Dim hi = 6
Select Case x
    Case lo To hi
        Print "in bereich"
    Case Else
        Print "auserhalb"
End Select
`,
	// Rundungs-Fastpaths (math.Floor/Ceil/Trunc/Int): Brüche an der 0,
	// Vorzeichen von -0, Grenzbreiche jenseits von 2^31.
	"mathe-rundung": `
Print math.Floor(2.5)
Print math.Floor(-2.5)
Print math.Ceil(2.5)
Print math.Ceil(-2.5)
Print math.Trunc(-2.5)
Print math.Int(3.7)
Print math.Int(-0.4)
Print math.Trunc(-0.9)
Print math.Floor(-0.99)
Print math.Ceil(0.01)
Print math.Floor(-0.0)
Print math.Trunc(1500000000000000)
Print math.Ceil(-1500000000000000)
Print math.Floor(1)
Print math.Ceil(-1)
Dim x = -3.14
Print math.Floor(x)
Print math.Ceil(x)
Print Int(x)
Dim i
Dim s = 0.0
For i = 1 To 100000
    s = s + math.Floor(i / 7) + math.Ceil(i / 13)
Next
Print s
`,
	// Mod-Fastpath: stdlib-Semantik float64(int(x) % int(y)) exakt
	// spiegeln - Vorzeichen folgt dem Dividenden, Brucheile werden
	// vorab abgeschnitten, int(y)==0 gibt den Fehlerwert "Modulo
	// durch 0". Ueber-/Unterlauf-Konversionen (1e20, 9.3e18) deopten
	// und muessendenselben Wert wie die VM liefern.
	"mathe-mod": `
Print math.Mod(7, 3)
Print math.Mod(-7, 3)
Print math.Mod(7, -3)
Print math.Mod(-7, -3)
Print math.Mod(7.9, 3.9)
Print math.Mod(-7.9, 2.9)
Print math.Mod(1000000000000000, 7)
Print math.Mod(1000000000000000000, 13)
Print math.Mod(100000000000000000000, 13)
Print math.Mod(9300000000000000000, 7)
Print IsError(math.Mod(7, 0))
Print IsError(math.Mod(7, 0.5))
Print IsError(math.Mod(7, -0.5))
Print math.Mod("7", 3)
Dim a = 3
Dim b = 5
Print math.Mod(a * 100 + 7, b)
Print math.Mod(7, 2.9999999)
Dim i
Dim s = 0
For i = 1 To 100000
    s = s + math.Mod(i, 7)
Next
Print s
`,
	// Round-/RoundBank-Fastpaths: Round ist eine bitgenaue Spiegelung des
	// Go-Algorithmus (Bithalb-Aufrundung), RoundBank == math.RoundToEven ==
	// round-to-nearest-ties-even (roundsd Modus 0). Nur die Trunc(d)-0-
	// Form (inkl. |d| < 1) laeuft nativ; andere Nachkommastellen deoptieren
	// und muessen dasselbe Pow-Ergebnis geben. Trap-Faelle:
	// 0.49999999999999994 (die beruechtete x+0.5-Falle), -0.0-Vorzeichen,
	// .5-Grenzwerte (halb von Null weg) und grosse Ganzzahlgrenzen.
	"mathe-run": `
Print math.Round(2.5)
Print math.Round(-2.5)
Print math.Round(3.5)
Print math.Round(-3.5)
Print math.Round(0.5)
Print math.Round(-0.5)
Print math.Round(0)
Print math.Round(-0.0)
Print math.Round(1234567890123456.5)
Print math.RoundBank(1234567890123456.5)
Print math.Round(1234567890123457.5)
Print math.RoundBank(1234567890123457.5)
Print math.Round(4503599627370495.5)
Print math.RoundBank(4503599627370495.5)
Print math.Round(2.4999999999999996)
Print math.Round(-2.4999999999999996)
Print math.RoundBank(2.5)
Print math.RoundBank(-2.5)
Print math.RoundBank(3.5)
Print math.RoundBank(-3.5)
Print math.RoundBank(0.5)
Print math.RoundBank(-0.5)
Print math.RoundBank(0)
Print math.RoundBank(2.4999999999999996)
Print math.Round(2.567, 1)
Print math.RoundBank(12345.678, 2)
Print IsError(math.Round(True))
Print IsError(math.RoundBank("x"))
Print math.Round("9.5")
Dim eps = 0.00000000000000005551115123125783
Print math.Round(0.5 - eps)
Print math.Round(0.5 + eps)
Print math.RoundBank(0.5 - eps)
Print math.RoundBank(0.5 + eps)
Print math.Round(9300000000000000000)
Print math.Round(0.5 - eps, 1)
Dim i
Dim s = 0.0
For i = 1 To 100000
    s = s + math.Round(i * 0.7) + math.RoundBank(i * 0.3)
Next
Print s
`,
	// Clamp-Fastpath: stdlib ist math.Max(lo, math.Min(x, hi)). Traps:
	// Grenzwerte x < lo / x > hi (beide Teilentscheidungen), Equal-Zeros
	// (Min waehlt -0, Max +0), Str-Argumente (Deopt-Pfad fingert dieselbe
	// Konvertierung), vertauschte Grenzen (Min schraenkt auf das
	// Intervall ein) und die eps-Trap an 1. NaN ist im Quelltext nicht
	// konstruierbar; sein Verhalten deckt sich mit den nativen
	// Min/Max-Deopts (jeder Teilpfad deoptiert bei ccP, der VM-Wert ist
	// Max(lo, Min(NaN, hi)) = NaN).
	"mathe-clamp": `
Print math.Clamp(2, 1, 5)
Print math.Clamp(0, 1, 5)
Print math.Clamp(7, 1, 5)
Print math.Clamp(2.5, 1, 5.5)
Print math.Clamp(-5, -3, 8)
Print math.Clamp(1500000000000000, 1, 5)
Dim eps = 0.00000000000000005551115123125783
Print math.Clamp(1 - eps, 1, 5)
Print math.Clamp(1 + eps, 1, 5)
Print math.Clamp(-0.0, -0.0, 5)
Print math.Clamp(0.0, -0.0, 5)
Print math.Clamp(-0.0, 0.0, 5)
Print math.Clamp(0, 5, 1)
Print math.Clamp(0, -0.0, -0.0)
Print math.Clamp(2, 4, 3)
Print math.Clamp("7", 1, 5)
Print IsError(math.Clamp(True, 1, 5))
Print IsError(math.Clamp(2, "x", 5))
Print IsError(math.Clamp(2, 1, False))
Dim i
Dim s = 0.0
For i = 1 To 100000
    s = s + math.Clamp(i, 10, 90) + math.Clamp(-i, -50, 50)
Next
Print s
`,
	// Winkel-/Prozent-/Konstanten-Fastpaths: stdlib-Reihenfolge der
	// Operationen spiegeln (DegToRad: x * Pi / 180; RadToDeg: x * 180 / Pi;
	// Percent: p / 100 * v; PercentOf: p / t * 100). DegToRad/RadToDeg
	// laufen fuer alle Num-Werte nativ (IEEE-Bitidentik ohne Sonderfaelle);
	// PercentOf deoptiert bei Nenner 0 oder NaN und liefert denselben
	// Fehlerwert wie die VM..math.Pi()/math.E() sind native Konstanten.
	// Str-Argumente (Deopt-Pfad) muessen gleich konvertiert werden,
	// ungueltige Argumente denselben Error-Wert liefern.
	"mathe-winkel-prozent": `
Print math.Pi()
Print math.E()
Print math.DegToRad(0)
Print math.DegToRad(90)
Print math.DegToRad(180)
Print math.DegToRad(360)
Print math.DegToRad(-270)
Print math.DegToRad(12.345)
Print math.DegToRad(-0.001)
Print math.RadToDeg(0)
Print math.RadToDeg(1)
Print math.RadToDeg(-2.5)
Print math.RadToDeg(math.Pi())
Print math.RadToDeg(math.DegToRad(123.456))
Print math.Percent(25, 200)
Print math.Percent(-50, 200)
Print math.Percent(2.5, 180)
Print math.Percent(9300000000000000000, -2)
Print math.PercentOf(25, 50)
Print math.PercentOf(9300000000000000000, 300000000000000000000)
Print IsError(math.PercentOf(3, 0))
Print IsError(math.PercentOf(3, 0.0))
Print IsError(math.DegToRad("nix"))
Print IsError(math.RadToDeg(True))
Print IsError(math.Percent(1, "y"))
Print math.DegToRad("90")
Print math.Percent("50", "200")
Dim i
Dim s = 0
For i = 1 To 100000
    s = s + math.DegToRad(i) + math.RadToDeg(i / 1000)
    s = s + math.Percent(i, 3) + math.PercentOf(i, 2 * i)
Next
Print s
`,
	// ForEach-Fastpath: reine Num-Arrays laufen nativ (auch Zwei-Variablen-
	// Form, verschachtelt, Exit For, leer, vor dem Loop undeklarierte
	// Loop-Variablen), Bool/Null-Elemente nativ; Str-Elemente, 2D und
	// Map deoptieren pro Fall und muessen identische Ausgaben liefern.
	"each": `
Dim a(5)
For i = 0 To 5
    a(i) = i * 2 + 1
Next
Dim t = 0
Dim x
For Each x In a
    t = t + x
Next
Print t

t = 0
For Each k, v In a
    t = t + k * 10 + v
Next
Print t

Dim b(3)
b(0) = True
b(1) = False
b(2) = True
b(3) = False
Dim ct = 0
Dim z
For Each z In b
    ct = ct + 1
Next
Print ct

Dim big(999)
For i = 0 To 999
    big(i) = i
Next
t = 0
For Each y In big
    t = t + y
Next
Print t

t = 0
For Each kk, vv In big
    t = t + kk
Next
Print t

For Each neu In big
    t = t + 1
Next
Print t

t = 0
For Each e In big
    t = t + e
    If e = 5 Then
        Exit For
    End If
Next
Print t

Dim leer(0)
t = -1
Dim w
For Each w In leer
    t = 100
Next
Print t

Dim ws(3)
ws(0) = "a"
ws(1) = "b"
ws(2) = "c"
ws(3) = "d"
Dim joined = ""
Dim sv
For Each sv In ws
    joined = joined & sv
Next
Print joined

Dim m(2, 2)
m(0, 0) = 1
m(1, 1) = 5
Dim tt = 0
Dim q
For Each q In m
    tt = tt + q(0)
Next
Print tt

Dim mp = map.Create()
map.Set(mp, "a", 1)
map.Set(mp, "b", 2)
Dim ks = ""
Dim mk
For Each mk In mp
    ks = ks & mk
Next
Print ks

Dim gemischt(3)
gemischt(0) = 1
gemischt(1) = "zwei"
gemischt(2) = 3
gemischt(3) = True
Dim desc = ""
Dim g
For Each g In gemischt
    desc = desc & g
Next
Print desc

t = 0
For Each y In big
    For Each w2 In leer
        t = t + 1
    Next
    t = t + 1
Next
Print t
`,
}

// runProgramm kompiliert ein Skript und führt es aus (JIT an oder aus).
func runProgramm(t *testing.T, src string, useJIT bool) (string, error) {
	t.Helper()
	stmts, err := parser.ParseSource(src, parser.Options{File: "jit_test.vb"})
	if err != nil {
		t.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("jit_test.vb", stmts)
	if err != nil {
		t.Fatalf("Compiler: %v", err)
	}

	var prog *jit.Program
	if useJIT {
		prog = jit.Prepare(chunk, 0)
	}
	var buf bytes.Buffer
	v := New()
	v.Out = &buf
	v.SetJIT(prog)
	err = v.Run(chunk)
	if prog != nil {
		prog.Close()
	}
	return buf.String(), err
}

// TestJITParitaet stellt sicher, dass JIT und Interpreter dieselbe Ausgabe
// und denselben Fehler liefern.
func TestJITParitaet(t *testing.T) {
	for name, src := range jitProgramme {
		t.Run(name, func(t *testing.T) {
			want, wantErr := runProgramm(t, src, false)
			got, gotErr := runProgramm(t, src, true)
			if wantErr != nil {
				// Sonst wären beide Wege gleichermaßen gescheitert und der
				// Vergleich "identisch" ohne Aussage.
				t.Fatalf("interpretiertes Programm schlug fehl: %v", wantErr)
			}
			if fmt.Sprint(wantErr) != fmt.Sprint(gotErr) {
				t.Errorf("Fehler weicht ab:\n interpretiert: %v\n JIT:           %v", wantErr, gotErr)
			}
			if want != got {
				t.Errorf("Ausgabe weicht ab:\n--- interpretiert ---\n%s\n--- JIT ---\n%s", want, got)
			}
		})
	}
}

// TestJITParitaet2DFehler vergleicht Fehlerfälle des nativen 2D-Pfads
// (Grenzverletzungen, Nicht-Zahlen) direkt zwischen Interpretation und JIT.
// Beide Wege dürfen scheitern - aber identisch.
func TestJITParitaet2DFehler(t *testing.T) {
	cases := map[string]string{
		"lese-grenze": `
Dim m(3, 3)
Print m(4, 0)
`,
		"lese-grenze-negativ": `
Dim m(3, 3)
Print m(-1, 0)
`,
		"schreibe-grenze": `
Dim m(3, 3)
m(1, 9) = 5
Print m(1, 1)
`,
		"schreibe-grenze-negativ": `
Dim m(3, 3)
m(1, -1) = 5
Print m(1, 1)
`,
		"nicht-zahl-index": `
Dim m(3, 3)
Print m("x", 0)
`,
		"1d-mit-zwei-indizes": `
Dim a(5)
a(1, 2) = 7
Print a(1)
`,
		"string-in-zelle": `
Dim m(4, 4)
m(2, 2) = "hallo"
Print m(2, 2)
m(2, 2) = 7
Print m(2, 2)
`,
		"mit-fehlerwert": `
Dim m(4, 4)
Dim x = 0
m(x / 0, 1) = 2
Print m(0, 1)
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			want, wantErr := runProgramm(t, src, false)
			got, gotErr := runProgramm(t, src, true)
			if fmt.Sprint(wantErr) != fmt.Sprint(gotErr) {
				t.Errorf("Fehler weicht ab:\n interpretiert: %v\n JIT:           %v", wantErr, gotErr)
			}
			if want != got {
				t.Errorf("Ausgabe weicht ab:\n--- interpretiert ---\n%s\n--- JIT ---\n%s", want, got)
			}
		})
	}
}

// TestJITUebersetzt prüft, dass auf unterstützten Plattformen wirklich
// Maschinencode entsteht (sonst wäre der Paritätstest ohne Aussage).
func TestJITUebersetzt(t *testing.T) {
	if runtime.GOARCH != "amd64" || runtime.GOOS == "js" {
		t.Skip("JIT nur auf x86-64")
	}
	stmts, err := parser.ParseSource(jitProgramme["arithmetik"], parser.Options{File: "jit_test.vb"})
	if err != nil {
		t.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("jit_test.vb", stmts)
	if err != nil {
		t.Fatalf("Compiler: %v", err)
	}
	prog := jit.Prepare(chunk, 0)
	if prog == nil {
		t.Fatal("Prepare lieferte nil - nichts wurde übersetzt")
	}
	defer prog.Close()
	if prog.Stats.Entries == 0 || prog.Stats.Regions == 0 || prog.Stats.CodeBytes == 0 {
		t.Errorf("unerwartete Statistik: %+v", prog.Stats)
	}
	if prog.Stats.CodeBytes < 1000 {
		t.Errorf("unerwartet wenig Code: %d Bytes", prog.Stats.CodeBytes)
	}
}

// benchSkript ist ein Zahlenkranz ohne Ein-/Ausgabe (für die Messung).
const benchSkript = `
Dim i
Dim s
s = 0
For i = 1 To 20000
    s = s + i * 3 - 1
Next
Print s
`

func bench(b *testing.B, useJIT bool) {
	stmts, err := parser.ParseSource(benchSkript, parser.Options{File: "bench.vb"})
	if err != nil {
		b.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("bench.vb", stmts)
	if err != nil {
		b.Fatalf("Compiler: %v", err)
	}
	var prog *jit.Program
	if useJIT {
		prog = jit.Prepare(chunk, 0)
		if prog == nil {
			b.Skip("JIT nicht verfügbar")
		}
		defer prog.Close()
	}
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := New()
		v.Out = &buf
		v.SetJIT(prog)
		if err := v.Run(chunk); err != nil {
			b.Fatalf("Run: %v", err)
		}
		buf.Reset()
	}
}

func BenchmarkInterpretiert(b *testing.B) { bench(b, false) }
func BenchmarkJIT(b *testing.B)           { bench(b, true) }

// matrixBench: verschachtelte Schleifen mit 2D-Zugriff (Schreiben und
// Lesen) - beide sparen in der JIT-Version den ganzen VM-Loop ein.
const matrixBenchSkript = `
Dim m(99, 99)
Dim i
Dim j
Dim s = 0
For i = 0 To 99
    For j = 0 To 99
        m(i, j) = i * j + 1
        s = s + m(i, j)
    Next
Next
Print s
`

func benchMatrix(b *testing.B, useJIT bool) {
	stmts, err := parser.ParseSource(matrixBenchSkript, parser.Options{File: "bench.vb"})
	if err != nil {
		b.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("bench.vb", stmts)
	if err != nil {
		b.Fatalf("Compiler: %v", err)
	}
	var prog *jit.Program
	if useJIT {
		prog = jit.Prepare(chunk, 0)
		if prog == nil {
			b.Skip("JIT nicht verfügbar")
		}
		defer prog.Close()
	}
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := New()
		v.Out = &buf
		v.SetJIT(prog)
		if err := v.Run(chunk); err != nil {
			b.Fatalf("Run: %v", err)
		}
		buf.Reset()
	}
}

func BenchmarkMatrixInterpretiert(b *testing.B) { benchMatrix(b, false) }
func BenchmarkMatrixJIT(b *testing.B)           { benchMatrix(b, true) }

// eachBenchSkript: ForEach über ein millionenelementiges Num-Array - der
// Fastpath läuft die ganze Schleife nativ (EachInit/EachNext + Body), die
// Interpretation zahlt pro Element vier VM-Ops.
const eachBenchSkript = `
Dim a(999999)
Dim i
For i = 0 To 999999
    a(i) = i
Next
Dim s = 0
Dim x
For Each x In a
    s = s + x
Next
Print s
`

func benchEach(b *testing.B, useJIT bool) {
	stmts, err := parser.ParseSource(eachBenchSkript, parser.Options{File: "bench.vb"})
	if err != nil {
		b.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("bench.vb", stmts)
	if err != nil {
		b.Fatalf("Compiler: %v", err)
	}
	var prog *jit.Program
	if useJIT {
		prog = jit.Prepare(chunk, 0)
		if prog == nil {
			b.Skip("JIT nicht verfügbar")
		}
		defer prog.Close()
	}
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := New()
		v.Out = &buf
		v.SetJIT(prog)
		if err := v.Run(chunk); err != nil {
			b.Fatalf("Run: %v", err)
		}
		buf.Reset()
	}
}

func BenchmarkForEachInterpretiert(b *testing.B) { benchEach(b, false) }
func BenchmarkForEachJIT(b *testing.B)           { benchEach(b, true) }

// winkelBenchSkript: drei native Built-in-Calls pro Iteration (DegToRad,
// Percent und PercentOf) ueber eine millionenelementige Schleife - die
// Fastpaths ersetzen je Fall VM-Dispatch plus stdlib-Registerlookup.
const winkelBenchSkript = `
Dim r = 0
Dim i
For i = 1 To 1000000
    r = r + math.DegToRad(i) + math.Percent(i, 3) + math.PercentOf(i, 2 * i)
Next
Print r
`

func benchWinkel(b *testing.B, useJIT bool) {
	stmts, err := parser.ParseSource(winkelBenchSkript, parser.Options{File: "bench.vb"})
	if err != nil {
		b.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("bench.vb", stmts)
	if err != nil {
		b.Fatalf("Compiler: %v", err)
	}
	var prog *jit.Program
	if useJIT {
		prog = jit.Prepare(chunk, 0)
		if prog == nil {
			b.Skip("JIT nicht verfügbar")
		}
		defer prog.Close()
	}
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := New()
		v.Out = &buf
		v.SetJIT(prog)
		if err := v.Run(chunk); err != nil {
			b.Fatalf("Run: %v", err)
		}
		buf.Reset()
	}
}

func BenchmarkWinkelProzentInterpretiert(b *testing.B) { benchWinkel(b, false) }
func BenchmarkWinkelProzentJIT(b *testing.B)           { benchWinkel(b, true) }

// roundBenchSkript: zwei native Built-in-Calls pro Iteration (Round mit
// Bit-Spiegelung und RoundBank mit roundsd) ueber eine Millionenelementige
// Schleife.
const roundBenchSkript = `
Dim s = 0.0
Dim i
For i = 1 To 1000000
    s = s + math.Round(i * 0.7) + math.RoundBank(i * 0.3)
Next
Print s
`

func benchRound(b *testing.B, useJIT bool) {
	stmts, err := parser.ParseSource(roundBenchSkript, parser.Options{File: "bench.vb"})
	if err != nil {
		b.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("bench.vb", stmts)
	if err != nil {
		b.Fatalf("Compiler: %v", err)
	}
	var prog *jit.Program
	if useJIT {
		prog = jit.Prepare(chunk, 0)
		if prog == nil {
			b.Skip("JIT nicht verfügbar")
		}
		defer prog.Close()
	}
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := New()
		v.Out = &buf
		v.SetJIT(prog)
		if err := v.Run(chunk); err != nil {
			b.Fatalf("Run: %v", err)
		}
		buf.Reset()
	}
}

func BenchmarkRundungInterpretiert(b *testing.B) { benchRound(b, false) }
func BenchmarkRundungJIT(b *testing.B)           { benchRound(b, true) }

// clampBenchSkript: ein nativer Clamp-Call pro Iteration mit aufflippenden
// Grenzen, sodass die Schleife beide Teilentscheidungen (Min/Max) trifft.
const clampBenchSkript = `
Dim s = 0.0
Dim i
For i = 1 To 1000000
    s = s + math.Clamp(i * 0.001, 100, 900)
Next
Print s
`

func benchClamp(b *testing.B, useJIT bool) {
	stmts, err := parser.ParseSource(clampBenchSkript, parser.Options{File: "bench.vb"})
	if err != nil {
		b.Fatalf("Parser: %v", err)
	}
	chunk, err := compiler.Compile("bench.vb", stmts)
	if err != nil {
		b.Fatalf("Compiler: %v", err)
	}
	var prog *jit.Program
	if useJIT {
		prog = jit.Prepare(chunk, 0)
		if prog == nil {
			b.Skip("JIT nicht verfügbar")
		}
		defer prog.Close()
	}
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := New()
		v.Out = &buf
		v.SetJIT(prog)
		if err := v.Run(chunk); err != nil {
			b.Fatalf("Run: %v", err)
		}
		buf.Reset()
	}
}

func BenchmarkClampInterpretiert(b *testing.B) { benchClamp(b, false) }
func BenchmarkClampJIT(b *testing.B)           { benchClamp(b, true) }
