// Analyse eines Code-Arrays: Welche Instruktionen kann der JIT ausführen,
// wie tief liegt der Stack an jeder Stelle (Dataflow) und wo liegen die
// Regionsgrenzen inklusive Einstiegen?
//
// Die Tiefe d[i] ist relativ zum Beginn des Code-Arrays. Ein Einstieg darf
// aus beliebiger tatsächlicher Tiefe kommen - Entry.Run leitet daraus den
// Basis-Offset ab (base = tatsächliche Tiefe - d[einstieg]).
package jit

import (
	"strings"

	"vbxc/bytecode"
	"vbxc/token"
	"vbxc/value"
)

// succ ist ein Nachfolger mit der Stack-Tiefe dort.
type succ struct {
	to    int32 // Ziel-Index oder -1 für Fall-Through
	depth int32
}

type region struct {
	start, end int32 // Bytecode-Bereich [start, end)
	maxRel     int32 // größte Stack-Tiefe relativ zum Regionsanfang
	entries    []int32
	slots      []slotDef
}

type slotDef struct {
	name  string
	local bool // true: nur aktueller Scope (GetRef), false: Scope-Kette
}

type codeInfo struct {
	chunk   *bytecode.Chunk
	code    []bytecode.Instr
	names   []string
	d       []int32 // Tiefe relativ zum Code-Anfang (-1 = unbekannt)
	bad     []bool  // Tiefenkonflikt (nicht kompilierbar)
	el      []bool  // unterstützt und Tiefe bekannt
	target  map[int32]bool
	regions []region
}

// analyze wertet ein Code-Array aus.
func analyze(chunk *bytecode.Chunk, code []bytecode.Instr) *codeInfo {
	n := len(code)
	ci := &codeInfo{
		chunk:  chunk,
		code:   code,
		names:  chunk.Names,
		d:      make([]int32, n),
		bad:    make([]bool, n),
		el:     make([]bool, n),
		target: make(map[int32]bool),
	}
	for i := range ci.d {
		ci.d[i] = -1
	}

	// Alle Sprungziele sammeln (die Tiefe ist für die Zielmenge egal).
	for i := range code {
		for _, s := range ci.succs(chunk, int32(i), 0) {
			if s.to >= 0 {
				ci.target[s.to] = true
			}
		}
	}

	// Dataflow: Tiefe pro Instruktion; Konflikt -> Region zerschneiden.
	type item struct{ i, d int32 }
	work := []item{{0, 0}}
	for len(work) > 0 {
		it := work[len(work)-1]
		work = work[:len(work)-1]
		i, d := it.i, it.d
		if i < 0 || int(i) >= n || ci.bad[i] {
			continue
		}
		if d < 0 {
			// Stack wäre negativ (defekter Bytecode) - nicht kompilieren.
			ci.markBad(chunk, i)
			continue
		}
		if ci.d[i] >= 0 {
			if ci.d[i] != d {
				ci.markBad(chunk, i)
			}
			continue
		}
		ci.d[i] = d
		for _, s := range ci.succs(chunk, i, d) {
			work = append(work, item{s.to, s.depth})
		}
	}

	// Unterstützungs-Maske.
	for i := range code {
		if i == n-1 && fallsThrough(code[i]) {
			continue // Durchlauf am Arrayende darf nicht in Maschinencode laufen
		}
		if ci.d[i] < minDepth(code[i]) {
			continue // Operanden lägen unterhalb des Stacks - nicht anfassen
		}
		ci.el[i] = supported(chunk, code[i]) && !ci.bad[i] && ci.d[i] >= 0
	}

	ci.buildRegions(chunk)
	return ci
}

// markBad markiert i als nicht kompilierbar und propagiert die Ungewissheit
// an alle Nachfolger (deren Tiefe ist danach nicht mehr eindeutig).
func (ci *codeInfo) markBad(chunk *bytecode.Chunk, i int32) {
	if i < 0 || int(i) >= len(ci.bad) || ci.bad[i] {
		return
	}
	ci.bad[i] = true
	d := ci.d[i]
	if d < 0 {
		return
	}
	for _, s := range ci.succs(chunk, i, d) {
		ci.markBad(chunk, s.to)
	}
}

func (ci *codeInfo) buildRegions(chunk *bytecode.Chunk) {
	n := int32(len(ci.code))
	for s := int32(0); s < n; {
		if !ci.el[s] {
			s++
			continue
		}
		e := s
		for e < n && ci.el[e] {
			e++
		}
		r := region{start: s, end: e}
		for i := s; i < e; i++ {
			d := ci.d[i]
			if d > r.maxRel {
				r.maxRel = d
			}
			for _, su := range ci.succs(chunk, i, d) {
				if su.depth > r.maxRel {
					r.maxRel = su.depth
				}
			}
			if i == s || ci.target[i] {
				r.entries = append(r.entries, i)
			}
		}
		r.slots = ci.collectSlots(s, e)
		ci.regions = append(ci.regions, r)
		s = e
	}
}

// collectSlots dedupliziert die Variablennamen der Region.
func (ci *codeInfo) collectSlots(s, e int32) []slotDef {
	var out []slotDef
	add := func(name string, local bool) {
		if name == "" {
			return
		}
		for _, d := range out {
			if d.name == name && d.local == local {
				return
			}
		}
		out = append(out, slotDef{name: name, local: local})
	}
	for i := s; i < e; i++ {
		in := ci.code[i]
		switch in.Op {
		case bytecode.OpLoad, bytecode.OpLoadForUpdate, bytecode.OpStore, bytecode.OpStoreRef,
			bytecode.OpCall, bytecode.OpArrStore, bytecode.OpArrGuard2D:
			add(ci.name(int(in.A)), false)
		case bytecode.OpArrCheck:
			add(ci.name(int(in.A)), false)
		case bytecode.OpForInit:
			add(ci.name(int(in.A)), true)
		case bytecode.OpForTest:
			add(ci.name(int(in.B)), true)
		case bytecode.OpEachInit:
			// Loop-Variablen der ForEach-Schleife (Key immer, Value bei
			// der Zwei-Variablen-Form): nur lesen/pruefen, nativ ohne
			// Anlegen.
			add(ci.name(int(in.A)), false)
			if in.B > 0 {
				add(ci.name(int(in.B-1)), false)
			}
		case bytecode.OpEachNext:
			add(ci.name(int(in.B)), false)
			if in.C > 0 {
				add(ci.name(int(in.C-1)), false)
			}
		}
	}
	return out
}

func (ci *codeInfo) name(idx int) string {
	if idx < 0 || idx >= len(ci.names) {
		return ""
	}
	return ci.names[idx]
}

// supported entscheidet, ob der Opcode vom Codegenerator beherrscht wird.
// Enthält alle Bedingungen, die schon zur Übersetzungszeit feststehen
// (Konstanten-Art, STEP 0, gültige Namen/Operanden).
func supported(chunk *bytecode.Chunk, in bytecode.Instr) bool {
	nameOK := func(idx int32) bool { return idx >= 0 && int(idx) < len(chunk.Names) }
	constOK := func(idx int32) bool { return idx >= 0 && int(idx) < len(chunk.Consts) }

	switch in.Op {
	case bytecode.OpConst:
		if !constOK(in.A) {
			return false
		}
		switch chunk.Consts[in.A].Kind {
		case value.KindNum, value.KindBool, value.KindNull:
			return true
		}
		return false

	case bytecode.OpLoad, bytecode.OpLoadForUpdate,
		bytecode.OpStore, bytecode.OpStoreRef:
		return nameOK(in.A)

	case bytecode.OpForInit:
		// STEP 0 ist immer ein Skript-Fehler - die Interpretation meldet das.
		return nameOK(in.A) && constOK(in.B) && chunk.Consts[in.B].Num != 0

	case bytecode.OpForTest:
		// C ist die Richtung; B ist die Schleifenvariable.
		return nameOK(in.B) && (in.C == 0 || in.C == 1)

	case bytecode.OpForStep:
		return constOK(in.A)

	case bytecode.OpEachInit:
		// Nativ nur fuer 1D-Arrays; Map/2D/andere laufzeitfaelle deopten.
		// Beide Loop-Variablen muessen zur Uebersetzungszeit benennbar
		// sein; das Existieren prueft Laufzeitflaggen.
		if !nameOK(in.A) {
			return false
		}
		if in.B != 0 && !nameOK(in.B-1) {
			return false
		}
		return true

	case bytecode.OpEachNext:
		// Das Aktelement wird nativ nur fuer Num/Bool/Null geschrieben;
		// andere Elemente deopten zur Laufzeit. Sprungziel A prueft die
		// Dataflow-Logik separat.
		return nameOK(in.B) && (in.C == 0 || nameOK(in.C-1))

	case bytecode.OpJumpIfError:
		return in.B >= 0

	case bytecode.OpPopN:
		return in.A >= 0

	case bytecode.OpCalc:
		switch token.Type(in.A) {
		case token.PLUS, token.MINUS, token.MUL, token.DIV:
			return true
		}
		return false

	case bytecode.OpCall:
		// Fast paths for numeric math calls and numeric arr(index) or
		// arr(i, j). Runtime type checks deopt all other cases to the VM.
		return nameOK(in.A) && (nativeMathConstantCall(chunk.Names[in.A], in.B) || nativeMathUnaryCall(chunk.Names[in.A], in.B) || nativeMathRoundCall(chunk.Names[in.A], in.B) || nativeMathBinaryCall(chunk.Names[in.A], in.B) || nativeMathClampCall(chunk.Names[in.A], in.B) || in.B == 1 || in.B == 2)

	case bytecode.OpArrStore:
		// In-bounds numeric writes into 1D (B==0) or 2D (B==1) arrays only.
		// Replacing a value that may contain Go pointers must go through the
		// VM's write barrier, so the element must already be numeric.
		return (in.B == 0 || in.B == 1) && nameOK(in.A)

	case bytecode.OpArrGuard2D:
		// Guard vor dem 2D-Store: bei Arr2D ohne Wirkung, sonst Platzhalter
		// pushen und zum Store springen. Beides ist nativ machbar.
		return nameOK(in.A)

	case bytecode.OpArrCheck:
		return nameOK(in.A)

	case bytecode.OpToIndex:
		return true

	case bytecode.OpSelectDispatch:
		// Numeric-only dispatch tables can be emitted as native comparisons.
		// String and boolean values still deopt so their cross-type and linear
		// scan semantics remain in the VM.
		return in.A >= 0 && int(in.A) < len(chunk.Selects) &&
			len(chunk.Selects[in.A].Num) > 0 && len(chunk.Selects[in.A].Str) == 0

	case bytecode.OpCaseEq:
		// Gleiche Kinds werden nativ entschieden (Num exakt, Bool, Null/Nil);
		// Strings (EqualFold) und Cross-Type deopten zur Laufzeit in die VM.
		return true

	case bytecode.OpCaseRange:
		// Nur Zahlenoperanden (ToFloatSafe auf Nicht-Zahlen -> 0); die
		// Guards der Codegenerierung deopten bei Nicht-Zahlen.
		return true

	case bytecode.OpCaseIs:
		// EvalBinary-Operatoren, die nativ auf zwei Zahlen entschieden
		// werden können. AND/OR (IsTruthy-Logik) bleibt in der Interpretation.
		switch token.Type(in.A) {
		case token.EQ, token.NEQ, token.GT, token.LT, token.GE, token.LE:
			return true
		}
		return false

	case bytecode.OpIsError:
		return true

	case bytecode.OpNull, bytecode.OpTrue, bytecode.OpFalse,
		bytecode.OpPop,
		bytecode.OpAdd, bytecode.OpSub, bytecode.OpMul, bytecode.OpDiv,
		bytecode.OpEq, bytecode.OpNeq, bytecode.OpLt, bytecode.OpGt,
		bytecode.OpLe, bytecode.OpGe,
		bytecode.OpNot, bytecode.OpNeg, bytecode.OpTruthy,
		bytecode.OpShortCircuit, bytecode.OpJump,
		bytecode.OpJumpIfFalse, bytecode.OpJumpIfTrue,
		bytecode.OpCheck, bytecode.OpDup:
		return true
	}
	return false
}

// nativeMathConstantCall: Argumentlose Konstanten-Aufrufe.
func nativeMathConstantCall(name string, argc int32) bool {
	return argc == 0 && (strings.EqualFold(name, "math.pi") || strings.EqualFold(name, "math.e"))
}

// nativeMathRoundCall: Round/RoundBank mit 1 oder 2 Argumenten. Die
// Native-Pfade behandeln nur den Fall Trunc(d) == 0 (pow == 1); alle
// anderen Nachkommastellen-Anfragen deoptimieren zur VM - dort steht
// exakt dasselbe Pow-Rgebnis. RoundBank (= math.RoundToEven) nutzt
// roundsd und braucht SSE4.1; Round ist eine reine Bit-Spiegelung
// (Go-Algorithmen) und läuft schon mit SSE2.
func nativeMathRoundCall(name string, argc int32) bool {
	if argc != 1 && argc != 2 {
		return false
	}
	switch strings.ToLower(name) {
	case "math.round":
		return true
	case "math.roundbank":
		return hasSSE41
	}
	return false
}

// nativeMathUnaryCall identifies math built-ins with direct machine-code
// implementations. Other argument kinds deopt to the VM, which retains the
// full mathArg conversion and error behavior.
// hasSSE41: ob der Prozessor die SSE4.1-Rundung (roundsd) beherrscht. Die
// Rundungs-Fastpaths (Floor/Ceil/Trunc/Int) sind bitidentisch, brauchen aber
// SSE4.1; ohne Fähigkeit bleiben die Aufrufe in der VM.
var hasSSE41 = cpuidNative()

func nativeMathUnaryCall(name string, argc int32) bool {
	if argc != 1 {
		return false
	}
	switch strings.ToLower(name) {
	case "math.abs", "math.sign", "math.sqrt", "math.degtorad", "math.radtodeg":
		return true
	case "math.floor", "math.ceil", "math.trunc", "math.int":
		// roundsd (SSE4.1) ist bitidentisch zu math.Floor/Ceil/Trunc.
		return hasSSE41
	default:
		return false
	}
}

func nativeMathBinaryCall(name string, argc int32) bool {
	if argc != 2 {
		return false
	}
	if strings.EqualFold(name, "math.min") || strings.EqualFold(name, "math.max") {
		return true
	}
	if strings.EqualFold(name, "math.percent") || strings.EqualFold(name, "math.percentof") {
		return true
	}
	// Mod: int(x) % int(y) - bitidentisch via cvttsd2si + idiv; die
	// Trap-/Konvertherrenfälle (int(y)==0, INT64_min, NaN) deopten.
	return strings.EqualFold(name, "math.mod")
}

// nativeMathClampCall: Clamp mit genau 3 Argumenten. Stdlib ist
// math.Max(lo, math.Min(x, hi)) - eine Komposition aus denselben
// Min/Max-Entscheidungen, die schon nativ laufen. Bei zweiten Argumenten
// (Nachkommastellen-Form gibt es nicht) oder falschen Argumenttypen
// deoptiert der Pfad in die VM, die denselben stdlib-Wert liefert.
func nativeMathClampCall(name string, argc int32) bool {
	return argc == 3 && strings.EqualFold(name, "math.clamp")
}

// minDepth ist die minimale Stack-Tiefe, die der Opcode beim Eintritt braucht
// (die Operanden liegen unter dem Ergebnis). Schützt den Codegenerator vor
// Adressen unterhalb der virtuellen Basis.
func minDepth(in bytecode.Instr) int32 {
	switch in.Op {
	case bytecode.OpConst, bytecode.OpNull, bytecode.OpTrue, bytecode.OpFalse,
		bytecode.OpLoad, bytecode.OpLoadForUpdate:
		return 0
	case bytecode.OpPop:
		return 1
	case bytecode.OpPopN:
		return in.A
	case bytecode.OpDup, bytecode.OpNot, bytecode.OpNeg, bytecode.OpTruthy,
		bytecode.OpCheck, bytecode.OpJumpIfError, bytecode.OpIsError,
		bytecode.OpStore, bytecode.OpStoreRef,
		bytecode.OpShortCircuit, bytecode.OpJumpIfFalse, bytecode.OpJumpIfTrue,
		bytecode.OpSelectDispatch, bytecode.OpToIndex:
		return 1
	case bytecode.OpCall:
		if in.B > 0 {
			return in.B
		}
		return 0
	case bytecode.OpArrStore:
		if in.B == 1 {
			return 3 // val, idx1, idx2
		}
		return 2
	case bytecode.OpCaseEq, bytecode.OpCaseIs:
		return 2
	case bytecode.OpCaseRange:
		return 3
	case bytecode.OpAdd, bytecode.OpSub, bytecode.OpMul, bytecode.OpDiv,
		bytecode.OpCalc, bytecode.OpEq, bytecode.OpNeq, bytecode.OpLt,
		bytecode.OpGt, bytecode.OpLe, bytecode.OpGe,
		bytecode.OpForInit, bytecode.OpForTest, bytecode.OpForStep:
		return 2
	case bytecode.OpEachInit:
		return 1 // iterierbar
	case bytecode.OpEachNext:
		return 3 // iterierbar, Zaehler, Schluesselliste
	}
	return 0
}

// fallsThrough sagt, ob die Instruktion ohne Sprung weiterläuft.
func fallsThrough(in bytecode.Instr) bool {
	switch in.Op {
	case bytecode.OpJump, bytecode.OpReturn, bytecode.OpHalt:
		return false
	}
	return true
}

// succs liefert die Nachfolger mit Stack-Tiefe - exakt nach vm.exec.
func (ci *codeInfo) succs(chunk *bytecode.Chunk, i int32, d int32) []succ {
	in := ci.code[i]
	next := func(dd int32) []succ { return []succ{{to: i + 1, depth: dd}} }
	jump := func(t, dd int32) []succ {
		if t < 0 || int(t) >= len(ci.code) {
			return nil
		}
		return []succ{{to: t, depth: dd}}
	}
	both := func(t, d1, d2 int32) []succ {
		out := next(d1)
		if t >= 0 && int(t) < len(ci.code) {
			out = append(out, succ{to: t, depth: d2})
		}
		return out
	}

	switch in.Op {
	case bytecode.OpHalt, bytecode.OpReturn:
		return nil

	case bytecode.OpConst, bytecode.OpNull, bytecode.OpTrue, bytecode.OpFalse,
		bytecode.OpLoad, bytecode.OpLoadField, bytecode.OpLoadForUpdate,
		bytecode.OpDup:
		return next(d + 1)

	case bytecode.OpPop:
		return next(d - 1)
	case bytecode.OpPopN:
		return next(d - in.A)

	case bytecode.OpDeclare, bytecode.OpStore, bytecode.OpStoreRef,
		bytecode.OpSetGlobal, bytecode.OpSetReturn, bytecode.OpParamDefault:
		return next(d - 1)

	case bytecode.OpAdd, bytecode.OpSub, bytecode.OpMul, bytecode.OpDiv,
		bytecode.OpConcat, bytecode.OpEq, bytecode.OpNeq, bytecode.OpLt,
		bytecode.OpGt, bytecode.OpLe, bytecode.OpGe, bytecode.OpCalc,
		bytecode.OpCaseEq, bytecode.OpCaseIs:
		return next(d - 1)
	case bytecode.OpCaseRange:
		return next(d - 2)
	case bytecode.OpMapIndex:
		return next(d - 1)
	case bytecode.OpCall:
		return next(d - in.B + 1)
	case bytecode.OpMakeArray:
		return next(d - in.A + 1)
	case bytecode.OpDimArray, bytecode.OpPublicArray:
		return next(d - in.B)
	case bytecode.OpArrStore:
		if in.B == 1 {
			return next(d - 3)
		}
		return next(d - 2)
	case bytecode.OpEachInit:
		return next(d + 2)
	case bytecode.OpEachNext:
		// exhausted→Sprungziel kommt zur Dataflow-Arbeitliste; die Tiefe
		// bleibt gleich (EachNext poppt nichts; EachInit hat iter/index/
		// keys gepusht, POPN am Ende räumt alle drei ab).
		return both(in.A, d, d)
	case bytecode.OpPrint:
		return next(d - 1 - in.A)

	// Ohne Tiefenänderung, ohne Sprung:
	case bytecode.OpNot, bytecode.OpNeg, bytecode.OpTruthy,
		bytecode.OpIsError, bytecode.OpErrorText, bytecode.OpCheck,
		bytecode.OpToIndex, bytecode.OpArrCheck, bytecode.OpCls,
		bytecode.OpDefine, bytecode.OpForInit,
		bytecode.OpForStep:
		return next(d)

	// For-Test: springt bei "nicht mehr", sonst läuft es weiter (Stack bleibt).
	case bytecode.OpForTest:
		return both(in.A, d, d)

	// Sprünge ohne Tiefenänderung (Ziel und Fall-Through):
	case bytecode.OpMapBase:
		return both(in.A, d, d)
	case bytecode.OpJumpIfArg:
		return both(in.A, d, d)
	case bytecode.OpArrGuard2D:
		// Nicht-2D: Platzhalter pushen und springen.
		return both(in.B, d, d+1)

	case bytecode.OpShortCircuit:
		// pop; bei Kurzschluss: push + Sprung (Tiefe wieder d).
		return both(in.A, d-1, d)
	case bytecode.OpJump:
		return jump(in.A, d)
	case bytecode.OpJumpIfFalse, bytecode.OpJumpIfTrue:
		return both(in.A, d-1, d-1)
	case bytecode.OpJumpIfError:
		// Fehlerpfad: "below" Werte werden verworfen.
		return both(in.A, d, d-in.B)

	case bytecode.OpSelectDispatch:
		if int(in.A) >= len(chunk.Selects) {
			return next(d)
		}
		t := &chunk.Selects[in.A]
		// Bool-Werte laufen immer weiter (linearer Scan), sonst: Wert weg.
		out := next(d)
		for _, tg := range t.Targets {
			if tg >= 0 && int(tg) < len(ci.code) {
				out = append(out, succ{to: tg, depth: d - 1})
			}
		}
		if t.Default >= 0 && int(t.Default) < len(ci.code) {
			out = append(out, succ{to: t.Default, depth: d - 1})
		}
		return out
	}
	// Unbekannte Opcodes: konservativ als Durchlauf.
	return next(d)
}
