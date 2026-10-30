//go:build amd64

// Codegenerator: erzeugt aus kompilierten Bytecode-Regionen x86-64-Maschinen-
// code. Die Instruktionen sind so gewählt, dass sie niemals einen lebenden
// Go-Zeiger schreiben (nur Kind/Num/Bool bzw. NULL in den Zeigerfeldern) -
// dadurch braucht der Code keine Write-Barrier. Nicht unterstützte Werte und
// Aufrufe deopten in die VM; einfache numerische math.*-Aufrufe (Abs, Sign,
// Sqrt, Min, Max, Mod, Clamp, Floor, Ceil, Trunc, Int, DegToRad, RadToDeg,
// Percent, PercentOf, Round, RoundBank, Pi(), E()) sind spezialisiert. Round
// spiegelt den Go-Algorithmus bitidentisch als Integer-Logik, RoundBank ist
// über roundsd (SSE4.1) zu math.RoundToEven identisch; nur die Trunc(d)-0-Form
// (inkl. |d| < 1) läuft nativ, andere Nachkommastellen deoptieren.
//
// Registerregeln in Op-Körpern: RBX hält den ctx-Zeiger (exit() und der
// Stub-Epilog lesen ihn nach dem Op wieder!), RBP die Variablenzellen,
// R15 die Eingangstiefe, R13 den Slot-Basiszeiger. Scratch sind nur
// RAX, RCX, RDX, RSI und RDI.
package jit

import (
	"errors"
	"math"
	"strings"
	"unsafe"

	"vbxc/bytecode"
	"vbxc/token"
	"vbxc/value"
)

var errNoCode = errors.New("jit: kein kompilierbarer Code")

// Feld-Offsets und Kind-Konstanten aus der echten Definition abgeleitet.
var probe value.Value

const (
	offKind     = int32(unsafe.Offsetof(probe.Kind))
	offNum      = int32(unsafe.Offsetof(probe.Num))
	offBool     = int32(unsafe.Offsetof(probe.Bool))
	offStrPtr   = int32(unsafe.Offsetof(probe.Str))       // ptr im String-Header
	offStrLen   = int32(unsafe.Offsetof(probe.Str)) + 8   // len im String-Header
	offArrData  = int32(unsafe.Offsetof(probe.Arr))       // data im Slice-Header
	offArrLen   = int32(unsafe.Offsetof(probe.Arr)) + 8   // len im Slice-Header
	offArr2D    = int32(unsafe.Offsetof(probe.Arr2D))     // data im Slice-Header
	offArr2DLen = int32(unsafe.Offsetof(probe.Arr2D)) + 8 // len im Slice-Header

	// rowHeaderSize: Größe des []value.Value-Slice-Headers in Arr2D — die
	// Zeiger-Rows werden im 2D-Zugriff per imul+add adressiert.
	rowHeaderSize = int32(unsafe.Sizeof(value.Value{}.Arr))

	kNum   = int32(value.KindNum)
	kStr   = int32(value.KindStr)
	kArr   = int32(value.KindArr)
	kArr2D = int32(value.KindArr2D)
	kNull  = int32(value.KindNull)
	kBool  = int32(value.KindBool)
	kError = int32(value.KindError)
	kObj   = int32(value.KindObj)
	kNil   = int32(value.KindNil)

	// ctx-Felder als int32 (die Encoder arbeiten mit int32-Displacements).
	ctxStack  = int32(offCtxStack)
	ctxBase   = int32(offCtxBase)
	ctxDepth  = int32(offCtxDepth)
	ctxBudget = int32(offCtxBudget)
	ctxVars   = int32(offCtxVars)
)

// fSignMask dient dem negativen Vorzeichen: XOR mit dem oberen Bit kehrt das
// Vorzeichen einer float64 um - exakt wie "-x" im Interpreter (auch fuer NaN,
// wo eine Multiplikation mit -1.0 undefiniert waere).
var fSignMask = [2]uint64{0x8000000000000000, 0}

func init() {
	if unsafe.Sizeof("") != 16 || unsafe.Sizeof(value.Value{}) != uintptr(slotSize) {
		panic("jit: unerwartetes Speicher-Layout")
	}
	// 2D-Zugriff adressiert die Row-Header (Slice-Header von []value.Value)
	// per imul; damit die Verschiebungen stimmen, muss die Größe feststehen.
	if rowHeaderSize != 24 {
		panic("jit: unerwartetes Slice-Header-Layout für Arr2D")
	}
}

// compile erzeugt für alle Code-Arrays des Chunks Regionen, Einstiege und
// Maschinencode in EINEM ausführbaren Speicherblock.
func compile(p *Program, chunk *bytecode.Chunk) error {
	type job struct {
		key        uintptr
		arrayIndex int
		ci         *codeInfo
		r          *region
	}
	type stubbed struct {
		e          *Entry
		stub       int
		key        uintptr
		arrayIndex int
	}

	// Alle Code-Arrays: Hauptprogramm + Function/Sub-Körper.
	arrays := codeArrays(chunk)

	var jobs []job
	for arrayIndex, ca := range arrays {
		key := uintptr(unsafe.Pointer(&ca[0]))
		ci := analyze(chunk, ca)
		for i := range ci.el {
			if !ci.el[i] {
				p.Stats.Skipped++
			}
		}
		p.tables[key] = make([]*Entry, len(ca))
		for i := range ci.regions {
			jobs = append(jobs, job{key, arrayIndex, ci, &ci.regions[i]})
			p.Stats.Regions++
		}
	}
	if len(jobs) == 0 {
		return errNoCode
	}

	a := newAsm()
	var stubs []stubbed

	for _, j := range jobs {
		g := &cg{
			a:       a,
			chunk:   chunk,
			ci:      j.ci,
			r:       j.r,
			body:    make(map[int32]int, j.r.end-j.r.start),
			slotIdx: make(map[slotDef]int, len(j.r.slots)),
		}
		for i, sd := range j.r.slots {
			g.slotIdx[sd] = i
		}
		g.epi = a.label()
		for ip := j.r.start; ip < j.r.end; ip++ {
			g.body[ip] = a.label()
		}

		names := make([]string, len(j.r.slots))
		local := make([]bool, len(j.r.slots))
		for i, sd := range j.r.slots {
			names[i] = sd.name
			local[i] = sd.local
		}
		for _, ip := range j.r.entries {
			e := &Entry{
				ip:     ip,
				dEntry: j.ci.d[ip],
				maxRel: j.r.maxRel,
				names:  names,
				local:  local,
				slots:  make([]varSlot, len(j.r.slots)),
				budget: p.budget,
				code:   j.ci.code,
			}
			stub := a.label()
			g.emitStub(e, stub)
			stubs = append(stubs, stubbed{e, stub, j.key, j.arrayIndex})
		}
		g.emitBody()
	}

	a.finish()
	if len(a.buf) == 0 {
		return errNoCode
	}
	mem, err := allocExec(a.buf)
	if err != nil {
		return err
	}
	p.mem = mem
	p.cacheCode = append([]byte(nil), a.buf...)
	codeBase := mem.ptr
	for _, s := range stubs {
		fv := make([]uintptr, 1)
		fv[0] = codeBase + uintptr(a.labels[s.stub])
		s.e.fv = fv
		s.e.fn = *(*func(*ctx) int32)(unsafe.Pointer(&fv))
		p.tables[s.key][s.e.ip] = s.e
		p.cacheEntries = append(p.cacheEntries, cacheEntry{ArrayIndex: s.arrayIndex, IP: s.e.ip,
			StubOffset: a.labels[s.stub], DEntry: s.e.dEntry, MaxRel: s.e.maxRel,
			Names: s.e.names, Local: s.e.local})
		p.Stats.Entries++
	}
	p.Stats.CodeBytes = len(a.buf)
	return nil
}

// cg erzeugt den Code einer Region.
type cg struct {
	a       *asm
	chunk   *bytecode.Chunk
	ci      *codeInfo
	r       *region
	body    map[int32]int
	epi     int
	slotIdx map[slotDef]int
}

// ---------------------------------------------------------------- Hülfen

// exit schreibt Resume-ip und Stack-Tiefe zurück und springt in den Epilog.
func (g *cg) exit(ip, depth int32) {
	g.a.movRM(rax, rbx, ctxBase) // Basis-Offset (Byte)
	g.a.leaRM(rax, rax, depth*slotBytes)
	g.a.movMR(rbx, ctxDepth, rax)
	g.a.movRI(rax, uint64(uint32(ip)))
	g.a.jmp(g.epi)
}

// deoptBlock markiert das Deopt-Label eines Ops (Rückkehr VOR der Ausführung).
func (g *cg) deoptBlock(lbl int, ip int, depth int32) {
	if g.a.refs[lbl] == 0 {
		return // nie referenziert: toter Code sparen
	}
	g.a.mark(lbl)
	g.exit(int32(ip), depth)
}

// fall springt zum Nachfolger oder verlässt die Region.
func (g *cg) fall(i int, depth int32) {
	next := int32(i + 1)
	if next >= g.r.start && next < g.r.end {
		g.a.jmp(g.body[next])
		return
	}
	g.exit(next, depth)
}

// jumpTo springt zum Ziel (in der Region) oder verlässt die Region.
// Rückwärtssprünge zählen das Safepoint-Budget.
func (g *cg) jumpTo(i int, target, depth int32) {
	if target >= g.r.start && target < g.r.end {
		if target < int32(i) {
			lbl := g.a.label()
			g.a.decM(rbx, ctxBudget)
			g.a.jcc(ccE, lbl) // Budget aufgebraucht -> zurück zur Interpretation
			g.a.jmp(g.body[target])
			g.a.mark(lbl)
			g.exit(target, depth)
			return
		}
		g.a.jmp(g.body[target])
		return
	}
	g.exit(target, depth)
}

// pushPrologue leert den Ziel-Slot, wenn er außerhalb der Eingangstiefe liegt
// (dann nicht von der GC gescannt - alte Zeiger könnten dort hängen). Innerhalb
// der Eingangstiefe ist der Slot gültig und wird NICHT angerührt.
func (g *cg) pushPrologue(rel int32) {
	skip := g.a.label()
	g.a.cmpRI(r15, rel*slotBytes) // Eingangstiefe*128 > rel*128 => bedeckt
	g.a.jcc(ccG, skip)
	g.a.pxor(3, 3)
	for k := int32(0); k < 7; k++ {
		g.a.movdquS(r13, rel*slotBytes+16+k*16, 3)
	}
	g.a.mark(skip)
}

func (g *cg) wKind(reg, kind int32) { g.a.movMI(r13, reg*slotBytes+offKind, kind) }

func (g *cg) wKindReg(rel int32, r int) { g.a.movMR(r13, rel*slotBytes+offKind, r) }
func (g *cg) wNumX(rel int32, x int)    { g.a.movsdS(r13, rel*slotBytes+offNum, x) }
func (g *cg) wNumReg(rel int32, r int)  { g.a.movMR(r13, rel*slotBytes+offNum, r) }
func (g *cg) wBoolImm(rel, v int32) {
	g.a.movMI(r13, rel*slotBytes+offBool, v) // 8 Bytes: Bool + Padding
}
func (g *cg) wBoolReg(rel int32, r int) { g.a.movMR(r13, rel*slotBytes+offBool, r) }

// slotIdxOf liefert den Tabellenindex einer Variablen der Region. Die Menge
// der Namen stammt aus derselben Opcode-Menge wie in collectSlots - ein
// Fehlschlag wäre also ein interner Fehler.
func (g *cg) slotIdxOf(name string, local bool) int {
	i, ok := g.slotIdx[slotDef{name: name, local: local}]
	if !ok {
		panic("jit: interner Fehler - Variablen-Slot fehlt")
	}
	return i
}

// cellAndFlags lädt die aufgelöste Zelle (RAX) und deren Flags (RDX).
func (g *cg) cellAndFlags(idx int) {
	g.a.movRI(rcx, uint64(uint32(idx*16)))
	g.a.movRR(rax, rbp)
	g.a.addRR(rax, rcx)    // &slots[idx]
	g.a.movRM(rdx, rax, 8) // flags
	g.a.movRM(rax, rax, 0) // ptr
}

// guardNum: Kind == KindNum am Slot, sonst Deopt.
func (g *cg) guardNum(rel int32, deopt int) {
	g.a.movRM(rax, r13, rel*slotBytes)
	g.a.cmpRI(rax, kNum)
	g.a.jcc(ccNE, deopt)
}

// branchTruthy verzweigt nach VB-Wahrheit (IsTruthy) des Slots. Liefert die
// Labels für wahr/falsch; das Deopt-Label kommt vom Aufrufer (Objekt-Felder
// und ggf. Fehlerwerte sind maschinell nicht entscheidbar). Endet immer mit
// einem Sprung, nie mit Durchlauf.
func (g *cg) branchTruthy(rel int32, deoptLbl int, deoptErr bool) (truthy, falsy int) {
	truthy = g.a.label()
	falsy = g.a.label()
	ln := g.a.label()
	lb := g.a.label()
	lst := g.a.label()
	la := g.a.label()

	g.a.movRM(rax, r13, rel*slotBytes) // Kind
	g.a.cmpRI(rax, kNum)
	g.a.jcc(ccE, ln)
	g.a.cmpRI(rax, kBool)
	g.a.jcc(ccE, lb)
	g.a.cmpRI(rax, kStr)
	g.a.jcc(ccE, lst)
	g.a.cmpRI(rax, kArr)
	g.a.jcc(ccE, la)
	g.a.cmpRI(rax, kObj)
	g.a.jcc(ccE, deoptLbl)
	if deoptErr {
		g.a.cmpRI(rax, kError)
		g.a.jcc(ccE, deoptLbl)
	}
	// Alles Übrige (Null, Nil, Map, Bytes, Arr2D, ...) ist immer falsch.
	g.a.jmp(falsy)

	// Num: != 0 (NaN gilt als wahr)
	g.a.mark(ln)
	g.a.movsdL(0, r13, rel*slotBytes+offNum)
	g.a.xorpd(2, 2)
	g.a.ucomisd(0, 2)
	g.a.jcc(ccP, truthy)
	g.a.jcc(ccE, falsy)
	g.a.jmp(truthy)

	// Bool
	g.a.mark(lb)
	g.a.movRM(rsi, r13, rel*slotBytes+offBool)
	g.a.cmpRI(rsi, 0)
	g.a.jcc(ccE, falsy)
	g.a.jmp(truthy)

	// Str: len != 0
	g.a.mark(lst)
	g.a.cmpMI(r13, rel*slotBytes+offStrLen, 0)
	g.a.jcc(ccE, falsy)
	g.a.jmp(truthy)

	// Arr: len(Arr) != 0 || len(Arr2D) != 0
	g.a.mark(la)
	g.a.cmpMI(r13, rel*slotBytes+offArrLen, 0)
	g.a.jcc(ccNE, truthy)
	g.a.cmpMI(r13, rel*slotBytes+offArr2DLen, 0)
	g.a.jcc(ccNE, truthy)
	g.a.jmp(falsy)

	return truthy, falsy
}

// emitStub: Registerrahmen, Kontext laden, in den Regionstext springen.
func (g *cg) emitStub(e *Entry, stub int) {
	g.a.mark(stub)
	for _, r := range []int{rbx, rbp, rdi, rsi, r12, r13, r14, r15} {
		g.a.push(r)
	}
	g.a.movRR(rbx, rax)           // ctx (Arg0 in AX)
	g.a.movRM(r12, rbx, ctxStack) // &vm.stack
	g.a.movRM(r12, r12, 0)        // Datenzeiger
	g.a.movRM(r13, rbx, ctxBase)  // Basis-Offset
	g.a.addRR(r13, r12)           // virtuelle Basis (Slot 0)
	g.a.movRM(rbp, rbx, ctxVars)  // Auflösungstabelle
	g.a.movRI(r15, uint64(int64(e.dEntry)*int64(slotBytes)))
	g.a.jmp(g.body[e.ip])
}

// emitBody erzeugt den Regionstext und den Epilog.
func (g *cg) emitBody() {
	for ip := g.r.start; ip < g.r.end; ip++ {
		g.a.mark(g.body[ip])
		g.emitOp(int(ip))
	}
	g.a.mark(g.epi)
	for _, r := range []int{r15, r14, r13, r12, rsi, rdi, rbp, rbx} {
		g.a.pop(r)
	}
	g.a.ret()
}

// ---------------------------------------------------------------- Opcodes

func (g *cg) emitOp(i int) {
	in := g.ci.code[i]
	d := g.ci.d[i]
	sz := slotBytes
	dep := g.a.label()

	switch in.Op {

	// ---------- Konstanten und Stack ----------
	case bytecode.OpNull, bytecode.OpTrue, bytecode.OpFalse, bytecode.OpConst:
		g.pushPrologue(d)
		switch in.Op {
		case bytecode.OpNull:
			g.wKind(d, kNull)
		case bytecode.OpTrue:
			g.wKind(d, kBool)
			g.wBoolImm(d, 1)
		case bytecode.OpFalse:
			g.wKind(d, kBool)
			g.wBoolImm(d, 0)
		default:
			cv := g.chunk.Consts[in.A]
			switch cv.Kind {
			case value.KindNum:
				g.wKind(d, kNum)
				g.a.movRI(rdx, math.Float64bits(cv.Num))
				g.wNumReg(d, rdx)
			case value.KindBool:
				g.wKind(d, kBool)
				if cv.Bool {
					g.wBoolImm(d, 1)
				} else {
					g.wBoolImm(d, 0)
				}
			default:
				g.wKind(d, kNull)
			}
		}
		g.fall(i, d+1)

	case bytecode.OpPop:
		g.fall(i, d-1)
	case bytecode.OpPopN:
		g.fall(i, d-in.A)

	case bytecode.OpDup:
		g.a.movRM(rsi, r13, (d-1)*sz) // Quell-Kind (int64)
		kn := g.a.label()
		kb := g.a.label()
		g.a.cmpRI(rsi, kNum)
		g.a.jcc(ccE, kn)
		g.a.cmpRI(rsi, kBool)
		g.a.jcc(ccE, kb)
		g.a.cmpRI(rsi, kNull)
		g.a.jcc(ccNE, dep)
		// Null: nur das Kind kopieren.
		g.pushPrologue(d)
		g.wKindReg(d, rsi)
		g.fall(i, d+1)
		g.a.mark(kn)
		g.pushPrologue(d)
		g.wKindReg(d, rsi)
		g.a.movRM(rdx, r13, (d-1)*sz+offNum)
		g.wNumReg(d, rdx)
		g.fall(i, d+1)
		g.a.mark(kb)
		g.pushPrologue(d)
		g.wKindReg(d, rsi)
		g.a.movRM(rdx, r13, (d-1)*sz+offBool)
		g.wBoolReg(d, rdx)
		g.fall(i, d+1)
		g.deoptBlock(dep, i, d)
		return

	// ---------- Variablen ----------
	case bytecode.OpLoad, bytecode.OpLoadForUpdate:
		g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.A)), false))
		if in.Op == bytecode.OpLoad {
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccE, dep)
		} else {
			g.a.cmpRI(rdx, 1) // gefunden und nicht const
			g.a.jcc(ccNE, dep)
		}
		g.pushPrologue(d)
		g.a.movRM(rsi, rax, offKind)
		ln := g.a.label()
		lb := g.a.label()
		lz := g.a.label()
		done := g.a.label()
		g.a.cmpRI(rsi, kNum)
		g.a.jcc(ccE, ln)
		g.a.cmpRI(rsi, kBool)
		g.a.jcc(ccE, lb)
		g.a.cmpRI(rsi, kNull)
		g.a.jcc(ccE, lz)
		g.a.jmp(dep)
		g.a.mark(ln)
		g.wKind(d, kNum)
		g.a.movRM(rdx, rax, offNum)
		g.wNumReg(d, rdx)
		g.a.jmp(done)
		g.a.mark(lb)
		g.wKind(d, kBool)
		g.a.movRM(rdx, rax, offBool)
		g.wBoolReg(d, rdx)
		g.a.jmp(done)
		g.a.mark(lz)
		g.wKind(d, kNull)
		g.a.mark(done)
		g.fall(i, d+1)

	case bytecode.OpCall:
		callName := g.ci.name(int(in.A))
		if nativeMathConstantCall(callName, in.B) {
			// A variable with the same name wins over a builtin in the VM.
			g.cellAndFlags(g.slotIdxOf(callName, false))
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccNE, dep)
			constant := math.E
			if strings.EqualFold(callName, "math.pi") {
				constant = math.Pi
			}
			g.pushPrologue(d)
			g.wKind(d, kNum)
			g.a.movRI(rdx, math.Float64bits(constant))
			g.wNumReg(d, rdx)
			g.fall(i, d+1)
			g.deoptBlock(dep, i, d)
			return
		}
		if nativeMathUnaryCall(callName, in.B) {
			// A variable with the same name wins over a builtin in the VM.
			// If one exists, leave the complete call to the VM.
			g.cellAndFlags(g.slotIdxOf(callName, false))
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccNE, dep)

			arg := d - 1
			g.a.cmpMI(r13, arg*sz+offKind, kNum)
			g.a.jcc(ccNE, dep)
			switch strings.ToLower(callName) {
			case "math.abs":
				g.a.movRM(rdx, r13, arg*sz+offNum)
				g.a.movRI(rcx, 0x7fffffffffffffff)
				g.a.andRR(rdx, rcx)
				g.wKind(arg, kNum)
				g.wNumReg(arg, rdx)

			case "math.sign":
				negative, positive, zero, done := g.a.label(), g.a.label(), g.a.label(), g.a.label()
				g.a.movsdL(0, r13, arg*sz+offNum)
				g.a.xorpd(1, 1)
				g.a.ucomisd(0, 1)
				g.a.jcc(ccP, zero) // NaN compares as zero in math.Sign.
				g.a.jcc(ccB, negative)
				g.a.jcc(ccA, positive)
				g.a.jmp(zero)

				g.a.mark(negative)
				g.a.movRI(rdx, math.Float64bits(-1))
				g.a.jmp(done)
				g.a.mark(positive)
				g.a.movRI(rdx, math.Float64bits(1))
				g.a.jmp(done)
				g.a.mark(zero)
				g.a.movRI(rdx, math.Float64bits(0))
				g.a.mark(done)
				g.wKind(arg, kNum)
				g.wNumReg(arg, rdx)

			case "math.sqrt":
				g.a.movsdL(0, r13, arg*sz+offNum)
				g.a.xorpd(1, 1)
				g.a.ucomisd(0, 1)
				g.a.jcc(ccB, dep) // Negative values use the VM's ErrorVal path.
				g.a.sqrtsd(0, 0)
				g.wKind(arg, kNum)
				g.wNumX(arg, 0)

			case "math.degtorad":
				// stdlib: x * math.Pi / 180. Reihenfolge der Operationen
				// entspricht dem Go-Ausdruck, damit auch Rundungen gleich sind.
				g.a.movsdL(xmm0, r13, arg*sz+offNum)
				g.a.movRI(rdx, math.Float64bits(math.Pi))
				g.a.movqX(xmm1, rdx)
				g.a.arithSD(0x59, xmm0, xmm1) // mulsd
				g.a.movRI(rdx, math.Float64bits(180))
				g.a.movqX(xmm1, rdx)
				g.a.arithSD(0x5E, xmm0, xmm1) // divsd
				g.wKind(arg, kNum)
				g.wNumX(arg, xmm0)

			case "math.radtodeg":
				// stdlib: x * 180 / math.Pi (mit derselben Reihenfolge).
				g.a.movsdL(xmm0, r13, arg*sz+offNum)
				g.a.movRI(rdx, math.Float64bits(180))
				g.a.movqX(xmm1, rdx)
				g.a.arithSD(0x59, xmm0, xmm1) // mulsd
				g.a.movRI(rdx, math.Float64bits(math.Pi))
				g.a.movqX(xmm1, rdx)
				g.a.arithSD(0x5E, xmm0, xmm1) // divsd
				g.wKind(arg, kNum)
				g.wNumX(arg, xmm0)

			// roundsd (SSE4.1) rundet bitidentisch zu Go (NaN, ±Inf, ±0
			// laufen unverändert durch); supported() schaltet die Fälle nur
			// mit CPU-Fähigkeit frei.
			case "math.floor":
				g.a.movsdL(0, r13, arg*sz+offNum)
				g.a.roundsdM(0, r13, arg*sz+offNum, 1) // gegen -unendlich
				g.wKind(arg, kNum)
				g.wNumX(arg, 0)

			case "math.ceil":
				g.a.movsdL(0, r13, arg*sz+offNum)
				g.a.roundsdM(0, r13, arg*sz+offNum, 2) // gegen +unendlich
				g.wKind(arg, kNum)
				g.wNumX(arg, 0)

			case "math.trunc", "math.int": // Int == Trunc (stdlib)
				g.a.movsdL(0, r13, arg*sz+offNum)
				g.a.roundsdM(0, r13, arg*sz+offNum, 3) // gegen 0
				g.wKind(arg, kNum)
				g.wNumX(arg, 0)
			}
			g.fall(i, d)
			g.deoptBlock(dep, i, d)
			return
		}
		if nativeMathRoundCall(callName, in.B) {
			// math.Round/math.RoundBank: stdlib ist fn(x*pow)/pow mit
			// pow = Pow(10, Trunc(d)). Fuer Trunc(d) == 0 (inkl. |d| < 1)
			// ist pow == 1 und x*1/1 exakt - das Ergebnis ist fn(x).
			// Alles andere deoptiert (VM rechnet dasselbe Pow).
			// A variable with the same name wins over a builtin in the VM.
			g.cellAndFlags(g.slotIdxOf(callName, false))
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccNE, dep)
			argc := int(in.B)
			left := d - int32(argc)
			g.a.cmpMI(r13, left*sz+offKind, kNum)
			g.a.jcc(ccNE, dep)
			if argc == 2 {
				dArg := d - 1
				g.a.cmpMI(r13, dArg*sz+offKind, kNum)
				g.a.jcc(ccNE, dep)
				// Trunc(d) == 0 <=> |d| < 1 (NaN scheitert an ucomisd).
				g.a.movRM(rax, r13, dArg*sz+offNum)
				g.a.movRI(rsi, 0x7fffffffffffffff) // Vorzeichen weg
				g.a.andRR(rax, rsi)
				g.a.movqX(0, rax) // xmm0 = |d|
				g.a.movRI(rdx, math.Float64bits(1.0))
				g.a.movqX(1, rdx)
				g.a.ucomisd(0, 1)
				g.a.jcc(ccP, dep)  // NaN d: Pow(10, NaN) ergibt NaN, also VM-Pfad
				g.a.jcc(ccAE, dep) // |d| >= 1 -> VM-Pfad (Pow)
			}
			isBank := strings.EqualFold(callName, "math.roundbank")
			if isBank {
				// math.RoundToEven == IEEE roundToIntegralTiesToEven ==
				// roundsd Modus 0 (bitidentisch; differentialgetestet).
				g.a.movsdL(0, r13, left*sz+offNum)
				g.a.roundsdM(0, r13, left*sz+offNum, 0)
				g.wKind(left, kNum)
				g.wNumX(left, 0)
			} else {
				// bitgenaue Spiegelung des Go-Algorithmus
				// (Go src/math/floor.go, func Round):
				//   e = (bits>>52) & 0x7FF
				//   e < 1023: bits &= -1<<63; falls e == 1022: bits |= uvone
				//   e < 1075: bits += 1<<51 >> (e-1023)
				//             bits &^= ~(0x000FFFFFFFFFFFFF >> (e-1023))
				//   sonst (>=2^52, Infinity, NaN; exakt ganzzahlige
				//   Werte): unverändert
				// GPR: RAX=raw bits, RCX=Exponent, RDX/RSI=Masks.
				g.a.movRM(rax, r13, left*sz+offNum)
				g.a.movRR(rcx, rax)
				g.a.shrRI(rcx, 52)
				g.a.andRI(rcx, 0x7FF)
				lA := g.a.label()
				lB := g.a.label()
				done := g.a.label()
				g.a.cmpRI(rcx, 1023)
				g.a.jcc(ccB, lA)
				g.a.cmpRI(rcx, 1075)
				g.a.jcc(ccB, lB)
				g.a.jmp(done) // ganzzahlig/Infinity/NaN unverändert

				// |x| < 1: +-0, ab 0.5 +-1.
				g.a.mark(lA)
				g.a.movRI(rsi, 0x8000000000000000)
				g.a.andRR(rax, rsi)
				g.a.cmpRI(rcx, 1022)
				g.a.jcc(ccNE, done)
				g.a.movRI(rsi, 0x3ff0000000000000) // uvone: +-1.0
				g.a.orRR(rax, rsi)
				g.a.jmp(done)

				// |x| >= 1 halbe Aufrundung + Bruchbits löschen
				// (e-1023 als Shift-Zahl im CL).
				g.a.mark(lB)
				g.a.subRI(rcx, 1023)
				g.a.movRI(rdx, 0x0008000000000000) // 2^51
				g.a.shrRC(rdx)
				g.a.addRR(rax, rdx)
				g.a.movRI(rsi, 0x000fffffffffffff) // fracMask
				g.a.shrRC(rsi)
				g.a.notRR(rsi)
				g.a.andRR(rax, rsi)
				g.a.mark(done)
				g.a.movMR(r13, left*sz+offNum, rax)
			}
			g.fall(i, d+1-int32(argc))
			g.deoptBlock(dep, i, d)
			return
		}
		if nativeMathBinaryCall(callName, in.B) {
			// Non-numeric operands and NaN deopt to preserve mathArg and Go math
			// semantics. Equal zeros need explicit sign handling (Min prefers -0,
			// Max prefers +0).
			g.cellAndFlags(g.slotIdxOf(callName, false))
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccNE, dep)
			left, right := d-2, d-1
			g.a.cmpMI(r13, left*sz+offKind, kNum)
			g.a.jcc(ccNE, dep)
			g.a.cmpMI(r13, right*sz+offKind, kNum)
			g.a.jcc(ccNE, dep)
			if strings.EqualFold(callName, "math.mod") {
				// stdlib: float64(int(x) % int(y)), Fehlerwert bei
				// int(y) == 0. cvttsd2si spiegelt int() bitidentisch
				// (amd64); deopt bei int(y)==0 ("Modulo durch 0"),
				// INT64_min (auch NaN/Inf/uebergross - dort ist die
				// Go-Konvertierung implementierungsabhaengig) sowie
				// INT64_min % -1 (Hardware-Trap-Fall).
				g.a.movsdL(0, r13, left*sz+offNum)
				g.a.cvttsd2si(rax, 0)
				g.a.movsdL(0, r13, right*sz+offNum)
				g.a.cvttsd2si(rcx, 0)
				g.a.cmpRI(rcx, 0)
				g.a.jcc(ccE, dep) // int(y) == 0 -> VM-Fehlerwert
				g.a.movRI(rsi, 0x8000000000000000)
				g.a.cmpRR(rax, rsi)
				g.a.jcc(ccE, dep)
				g.a.cmpRR(rcx, rsi)
				g.a.jcc(ccE, dep)
				g.a.cqo()
				g.a.idivR(rcx)      // Rest in RDX
				g.a.cvtsi2sdX(0, 2) // xmm0 <- float64(int64(RDX))
				g.wKind(left, kNum)
				g.wNumX(left, 0)
				g.fall(i, d-1)
				g.deoptBlock(dep, i, d)
				return
			}
			if strings.EqualFold(callName, "math.percent") || strings.EqualFold(callName, "math.percentof") {
				// Keep the stdlib operation order: Percent = p / 100 * v;
				// PercentOf = part / total * 100. Zero denominator in PercentOf
				// returns its VM error value, so leave that case to the VM.
				isPercentOf := strings.EqualFold(callName, "math.percentof")
				if isPercentOf {
					g.a.xorpd(xmm1, xmm1)
					g.a.movsdL(xmm1, r13, right*sz+offNum)
					g.a.ucomisd(xmm1, xmm1)
					g.a.jcc(ccP, dep)
					g.a.xorpd(xmm2, xmm2)
					g.a.ucomisd(xmm1, xmm2)
					g.a.jcc(ccE, dep)
					g.a.movsdL(xmm0, r13, left*sz+offNum)
					g.a.arithSD(0x5E, xmm0, xmm1) // part / total
					g.a.movRI(rdx, math.Float64bits(100))
					g.a.movqX(xmm1, rdx)
					g.a.arithSD(0x59, xmm0, xmm1) // * 100
				} else {
					g.a.movsdL(xmm0, r13, left*sz+offNum)
					g.a.movRI(rdx, math.Float64bits(100))
					g.a.movqX(xmm1, rdx)
					g.a.arithSD(0x5E, xmm0, xmm1) // p / 100
					g.a.movsdL(xmm1, r13, right*sz+offNum)
					g.a.arithSD(0x59, xmm0, xmm1) // * v
				}
				g.wKind(left, kNum)
				g.wNumX(left, xmm0)
				g.fall(i, d-1)
				g.deoptBlock(dep, i, d)
				return
			}
			isMin := strings.EqualFold(callName, "math.min")
			chooseLeft, chooseRight, done := g.a.label(), g.a.label(), g.a.label()
			g.a.movsdL(0, r13, left*sz+offNum)
			g.a.movsdL(1, r13, right*sz+offNum)
			g.a.ucomisd(0, 1)
			g.a.jcc(ccP, dep) // NaN
			if isMin {
				g.a.jcc(ccB, chooseLeft)
				g.a.jcc(ccA, chooseRight)
			} else {
				g.a.jcc(ccA, chooseLeft)
				g.a.jcc(ccB, chooseRight)
			}
			// Equal: for non-zero values the left bit pattern is exact. For
			// signed zeros, OR selects -0 for Min and AND selects +0 for Max.
			g.a.movXQ(rdx, 0)
			g.a.movRR(rcx, rdx)
			g.a.movRI(rsi, 0x7fffffffffffffff)
			g.a.andRR(rcx, rsi)
			g.a.cmpRI(rcx, 0)
			g.a.jcc(ccNE, chooseLeft)
			g.a.movXQ(rcx, 1)
			if isMin {
				g.a.orRR(rdx, rcx)
			} else {
				g.a.andRR(rdx, rcx)
			}
			g.a.movqX(0, rdx)
			g.a.jmp(done)
			g.a.mark(chooseLeft)
			g.a.movsdL(0, r13, left*sz+offNum)
			g.a.jmp(done)
			g.a.mark(chooseRight)
			g.a.movsdL(0, r13, right*sz+offNum)
			g.a.mark(done)
			g.wKind(left, kNum)
			g.wNumX(left, 0)
			g.fall(i, d-1)
			g.deoptBlock(dep, i, d)
			return
		}
		if nativeMathClampCall(callName, in.B) {
			// stdlib: math.Max(lo, math.Min(x, hi)) - eine Komposition
			// aus denselben Min/Max-Vergleichen wie die nativen
			// 2-Arg-Formen. Nicht-Num-Argumente und NaN (ccP) deopten,
			// bevor irgendein Slot beschrieben wird; Equal-Zeros
			// entscheiden wie die Vorlage: Min waehlt -0 (OR), Max +0
			// (AND). Bei lo = NaN liefert beide Pfade NaN (VM denaturiert
			// denselben Wert), deshalb darf der zweite Deopt nach der
			// Min-Teilrechnung keine Zelle veraendert haben.
			g.cellAndFlags(g.slotIdxOf(callName, false))
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccNE, dep)
			xs, loS, hiS := d-3, d-2, d-1
			g.a.cmpMI(r13, xs*sz+offKind, kNum)
			g.a.jcc(ccNE, dep)
			g.a.cmpMI(r13, loS*sz+offKind, kNum)
			g.a.jcc(ccNE, dep)
			g.a.cmpMI(r13, hiS*sz+offKind, kNum)
			g.a.jcc(ccNE, dep)

			// Min(x, hi): Ergebnis in xmm0 (Vorlage: left=x, right=hi).
			minHi, minDone := g.a.label(), g.a.label()
			g.a.movsdL(0, r13, xs*sz+offNum)
			g.a.movsdL(1, r13, hiS*sz+offNum)
			g.a.ucomisd(0, 1)
			g.a.jcc(ccP, dep)     // NaN in x oder hi
			g.a.jcc(ccB, minDone) // x < hi: xmm0 bleibt x
			g.a.jcc(ccA, minHi)   // x > hi: xmm0 = hi
			// Equal: +0/-0-Bits querpruefen, Min waehlt -0 (OR).
			g.a.movXQ(rdx, 0)
			g.a.movRR(rcx, rdx)
			g.a.movRI(rsi, 0x7fffffffffffffff)
			g.a.andRR(rcx, rsi)
			g.a.cmpRI(rcx, 0)
			g.a.jcc(ccNE, minDone) // nicht ±0: identische Bits, x genuegt
			g.a.movXQ(rcx, 1)
			g.a.orRR(rdx, rcx)
			g.a.movqX(0, rdx)
			g.a.jmp(minDone)
			g.a.mark(minHi)
			g.a.movsdL(0, r13, hiS*sz+offNum)
			g.a.mark(minDone)

			// Max(m, lo) mit m = Min(x, hi) in xmm0 (Vorlage: left=m,
			// right=lo); ucomisd(0,1) vergleicht also xmm0 vs xmm1.
			maxLo, maxDone := g.a.label(), g.a.label()
			g.a.movsdL(1, r13, loS*sz+offNum)
			g.a.ucomisd(0, 1)
			g.a.jcc(ccP, dep)     // NaN in lo (m kann kein NaN sein)
			g.a.jcc(ccA, maxDone) // m > lo: xmm0 bleibt m
			g.a.jcc(ccB, maxLo)   // m < lo: xmm0 = lo
			// Equal: Max waehlt +0 (AND).
			g.a.movXQ(rdx, 0)
			g.a.movRR(rcx, rdx)
			g.a.movRI(rsi, 0x7fffffffffffffff)
			g.a.andRR(rcx, rsi)
			g.a.cmpRI(rcx, 0)
			g.a.jcc(ccNE, maxDone)
			g.a.movXQ(rcx, 1)
			g.a.andRR(rdx, rcx)
			g.a.movqX(0, rdx)
			g.a.jmp(maxDone)
			g.a.mark(maxLo)
			g.a.movsdL(0, r13, loS*sz+offNum)
			g.a.mark(maxDone)

			g.wKind(xs, kNum)
			g.wNumX(xs, 0)
			g.fall(i, d-2)
			g.deoptBlock(dep, i, d)
			return
		}

		// Guarded arr(index) / arr(i, j) fast path. Only copy a numeric
		// element into the existing index slot; all other cases return to
		// the VM.
		g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.A)), false))
		g.a.cmpRI(rdx, 0)
		g.a.jcc(ccE, dep)
		if in.B == 2 {
			// arr(i, j): nummerisches Element eines 2D-Arrays lesen. Fehler
			// (Typen, Grenzen, Nicht-Zahl im Element) deopten in die VM, die
			// denselben Fehlerwert setzt wie indexAccess.
			g.a.cmpMI(rax, offKind, kArr2D)
			g.a.jcc(ccNE, dep)

			idx1 := (d - 2) * sz
			idx2 := (d - 1) * sz
			g.a.movRM(rsi, rax, offArr2DLen) // Länge der Antwort-Rows
			g.a.movRM(rax, rax, offArr2D)    // auf den outer-Datenzeiger wechseln
			g.a.movRM(rdx, r13, idx1+offKind)
			g.a.cmpRI(rdx, kNum)
			g.a.jcc(ccNE, dep)
			g.a.movsdL(0, r13, idx1+offNum)
			g.a.cvttsd2si(rdx, 0) // trunc wie int(...) in der VM
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccL, dep)
			g.a.cmpRR(rdx, rsi)
			g.a.jcc(ccGE, dep)
			// Row-Header: outer + i*24; daraus Datenzeiger und Row-Länge.
			g.a.imulRI(rdx, rowHeaderSize)
			g.a.addRR(rax, rdx)
			g.a.movRM(rcx, rax, 0) // Row-Data
			g.a.movRM(rsi, rax, 8) // Row-Länge
			g.a.movRM(rdx, r13, idx2+offKind)
			g.a.cmpRI(rdx, kNum)
			g.a.jcc(ccNE, dep)
			g.a.movsdL(0, r13, idx2+offNum)
			g.a.cvttsd2si(rdx, 0)
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccL, dep)
			g.a.cmpRR(rdx, rsi)
			g.a.jcc(ccGE, dep)
			// Element: row + j*slotBytes
			g.a.imulRI(rdx, slotBytes)
			g.a.addRR(rcx, rdx)
			g.a.movRM(rsi, rcx, offKind)
			g.a.cmpRI(rsi, kNum)
			g.a.jcc(ccNE, dep)
			g.a.movRM(rdx, rcx, offNum)
			g.wKind(d-2, kNum)
			g.wNumReg(d-2, rdx)
			g.fall(i, d-1)
		} else {
			g.a.cmpMI(rax, offKind, kArr)
			g.a.jcc(ccNE, dep)
			g.a.movRM(rsi, rax, offArrLen)
			g.a.movRM(rax, rax, offArrData)

			idx := (d - 1) * sz
			g.a.movRM(rdx, r13, idx+offKind)
			g.a.cmpRI(rdx, kNum)
			g.a.jcc(ccNE, dep)
			g.a.movsdL(0, r13, idx+offNum)
			g.a.cvttsd2si(rdx, 0)
			// arr(i) uses int(ToNum(i)): truncate first, then check bounds.
			// This preserves valid fractional indices such as -0.5 -> 0.
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccL, dep)
			g.a.cmpRR(rdx, rsi)
			g.a.jcc(ccGE, dep)
			g.a.imulRI(rdx, slotBytes)
			g.a.addRR(rax, rdx)
			g.a.movRM(rsi, rax, offKind)
			g.a.cmpRI(rsi, kNum)
			g.a.jcc(ccNE, dep)
			g.a.movRM(rdx, rax, offNum)
			g.wKind(d-1, kNum)
			g.wNumReg(d-1, rdx)
			g.fall(i, d)
		}

	case bytecode.OpArrCheck:
		g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.A)), false))
		g.a.cmpRI(rdx, 1) // present and not const
		g.a.jcc(ccNE, dep)
		g.fall(i, d)

	case bytecode.OpToIndex:
		idx := (d - 1) * sz
		g.a.movRM(rax, r13, idx+offKind)
		g.a.cmpRI(rax, kNum)
		g.a.jcc(ccNE, dep)
		g.a.movsdL(0, r13, idx+offNum)
		g.a.ucomisd(0, 0) // NaN is an error in ToIndex.
		g.a.jcc(ccP, dep)
		g.a.movRI(rcx, math.Float64bits(float64(math.MaxInt32)))
		g.a.movqX(1, rcx)
		g.a.ucomisd(0, 1)
		g.a.jcc(ccP, dep)
		g.a.jcc(ccA, dep)
		g.a.movRI(rcx, math.Float64bits(float64(math.MinInt32)))
		g.a.movqX(1, rcx)
		g.a.ucomisd(0, 1)
		g.a.jcc(ccP, dep)
		g.a.jcc(ccB, dep)
		g.a.cvttsd2si(rdx, 0)
		g.a.cvtsi2sdX(0, rdx)
		g.wKind(d-1, kNum)
		g.wNumX(d-1, 0)
		g.fall(i, d)

	case bytecode.OpArrGuard2D:
		// Guard vor dem 2D-Store. Bei Arr2D läuft es ohne Wirkung weiter;
		// sonst wird ein NIL-Platzhalter auf den Stack geschrieben und zum
		// Store gesprungen (dessen Guards kehren dann zur VM zurück).
		lbl := g.a.label()
		g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.A)), false))
		g.a.cmpRI(rdx, 0)
		g.a.jcc(ccE, dep)
		g.a.cmpMI(rax, offKind, kArr2D)
		g.a.jcc(ccNE, lbl)
		g.fall(i, d)
		g.a.mark(lbl)
		g.pushPrologue(d) // NIL in einen neuen Slot -> GC-sicher
		g.wKind(d, kNil)
		g.jumpTo(i, in.B, d+1)

	case bytecode.OpArrStore:
		// Store into an existing numeric element only. Replacing a value that
		// may contain Go pointers must go through the VM's write barrier.
		g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.A)), false))
		g.a.cmpRI(rdx, 0)
		g.a.jcc(ccE, dep)
		if in.B == 1 {
			// arr(i, j) = numVal: nummerisches Element eines 2D-Arrays
			// überschreiben. Die Guards entsprechen dem 1D-Pfad; jeder
			// Abweichungsfall (Typen, Grenzen, Nicht-Zahl) kehrt zur VM
			// zurück, die denselben Skript-Fehler meldet.
			g.a.cmpMI(rax, offKind, kArr2D)
			g.a.jcc(ccNE, dep)

			val := (d - 3) * sz
			idx1 := (d - 2) * sz
			idx2 := (d - 1) * sz
			// Wert muss Zahl sein.
			g.a.movRM(rdx, r13, val+offKind)
			g.a.cmpRI(rdx, kNum)
			g.a.jcc(ccNE, dep)
			// Länge der outer-Dimension + idx1-Prüfung.
			g.a.movRM(rsi, rax, offArr2DLen)
			g.a.movRM(rax, rax, offArr2D) // auf den outer-Datenzeiger wechseln
			g.a.movRM(rdx, r13, idx1+offKind)
			g.a.cmpRI(rdx, kNum)
			g.a.jcc(ccNE, dep)
			g.a.movsdL(0, r13, idx1+offNum)
			g.a.cvttsd2si(rdx, 0)
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccL, dep)
			g.a.cmpRR(rdx, rsi)
			g.a.jcc(ccGE, dep)
			// Row-Header → Datenzeiger + Row-Länge.
			g.a.imulRI(rdx, rowHeaderSize)
			g.a.addRR(rax, rdx)
			g.a.movRM(rcx, rax, 0)
			g.a.movRM(rsi, rax, 8)
			// idx2-Prüfung + Elementadresse.
			g.a.movRM(rdx, r13, idx2+offKind)
			g.a.cmpRI(rdx, kNum)
			g.a.jcc(ccNE, dep)
			g.a.movsdL(0, r13, idx2+offNum)
			g.a.cvttsd2si(rdx, 0)
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccL, dep)
			g.a.cmpRR(rdx, rsi)
			g.a.jcc(ccGE, dep)
			g.a.imulRI(rdx, slotBytes)
			g.a.addRR(rcx, rdx)
			// Ziel-Element muss bereits Zahl sein (sonst Write-Barrier).
			g.a.movRM(rdx, rcx, offKind)
			g.a.cmpRI(rdx, kNum)
			g.a.jcc(ccNE, dep)
			g.a.movRM(rdx, r13, val+offNum)
			g.a.movMR(rcx, offNum, rdx)
			g.fall(i, d-3)
		}
		g.a.cmpMI(rax, offKind, kArr)
		g.a.jcc(ccNE, dep)
		g.a.movRM(rsi, rax, offArrLen)
		g.a.movRM(rax, rax, offArrData)

		idx := (d - 1) * sz
		val := (d - 2) * sz
		g.a.movRM(rdx, r13, val+offKind)
		g.a.cmpRI(rdx, kNum)
		g.a.jcc(ccNE, dep)
		g.a.movRM(rdx, r13, idx+offKind)
		g.a.cmpRI(rdx, kNum)
		g.a.jcc(ccNE, dep)
		g.a.movsdL(0, r13, idx+offNum)
		g.a.xorpd(2, 2)
		g.a.ucomisd(0, 2)
		g.a.jcc(ccP, dep)
		g.a.jcc(ccB, dep)
		g.a.cvtsi2sdX(1, rsi)
		g.a.ucomisd(0, 1)
		g.a.jcc(ccP, dep)
		g.a.jcc(ccAE, dep)
		g.a.movRI(rcx, math.Float64bits(float64(math.MaxInt32)))
		g.a.movqX(1, rcx)
		g.a.ucomisd(0, 1)
		g.a.jcc(ccP, dep)
		g.a.jcc(ccA, dep) // ToIndex rejects values above MaxInt32.
		g.a.cvttsd2si(rdx, 0)
		g.a.imulRI(rdx, slotBytes)
		g.a.addRR(rax, rdx)
		g.a.movRM(rsi, rax, offKind)
		g.a.cmpRI(rsi, kNum)
		g.a.jcc(ccNE, dep)
		g.a.movRM(rdx, r13, val+offNum)
		g.a.movMR(rax, offNum, rdx)
		g.fall(i, d-2)

	case bytecode.OpStore, bytecode.OpStoreRef:
		src := (d - 1) * sz
		g.a.movRM(rsi, r13, src) // Quell-Kind
		ky := g.a.label()
		wn := g.a.label()
		wb := g.a.label()
		done := g.a.label()
		g.a.cmpRI(rsi, kNum)
		g.a.jcc(ccE, ky)
		g.a.cmpRI(rsi, kBool)
		g.a.jcc(ccE, ky)
		g.a.cmpRI(rsi, kNull)
		g.a.jcc(ccE, ky)
		g.a.jmp(dep)
		g.a.mark(ky)
		g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.A)), false))
		if in.Op == bytecode.OpStore {
			g.a.cmpRI(rdx, 1) // gefunden und nicht const
			g.a.jcc(ccNE, dep)
		} else {
			g.a.cmpRI(rdx, 0) // nur gefunden (Const darf hier rein)
			g.a.jcc(ccE, dep)
		}
		g.a.cmpRI(rsi, kNum)
		g.a.jcc(ccE, wn)
		g.a.cmpRI(rsi, kBool)
		g.a.jcc(ccE, wb)
		g.a.movMI(rax, offKind, kNull)
		g.a.jmp(done)
		g.a.mark(wn)
		g.a.movMI(rax, offKind, kNum)
		g.a.movRM(rdx, r13, src+offNum)
		g.a.movMR(rax, offNum, rdx)
		g.a.jmp(done)
		g.a.mark(wb)
		g.a.movMI(rax, offKind, kBool)
		g.a.movRM(rdx, r13, src+offBool)
		g.a.movMR(rax, offBool, rdx)
		g.a.mark(done)
		g.fall(i, d-1)

	// ---------- Rechnen ----------
	case bytecode.OpAdd, bytecode.OpSub, bytecode.OpMul,
		bytecode.OpDiv, bytecode.OpCalc:
		g.guardNum(d-2, dep)
		g.guardNum(d-1, dep)
		g.a.movsdL(0, r13, (d-2)*sz+offNum)
		g.a.movsdL(1, r13, (d-1)*sz+offNum)
		var op byte
		switch in.Op {
		case bytecode.OpAdd:
			op = 0x58
		case bytecode.OpSub:
			op = 0x5C
		case bytecode.OpMul:
			op = 0x59
		case bytecode.OpDiv:
			op = 0x5E
		default: // OpCalc: Token sichert supported() ab
			switch token.Type(in.A) {
			case token.PLUS:
				op = 0x58
			case token.MINUS:
				op = 0x5C
			case token.MUL:
				op = 0x59
			default:
				op = 0x5E
			}
		}
		if op == 0x5E {
			// Division durch 0 (oder NaN-Divisor) -> Interpretation,
			// die "Division durch 0" liefert.
			g.a.xorpd(2, 2)
			g.a.ucomisd(1, 2)
			g.a.jcc(ccE, dep)
		}
		g.a.arithSD(op, 0, 1)
		g.pushPrologue(d - 2)
		g.wKind(d-2, kNum)
		g.wNumX(d-2, 0)
		g.fall(i, d-1)

	// ---------- Vergleichen ----------
	case bytecode.OpLt, bytecode.OpGt, bytecode.OpLe, bytecode.OpGe:
		g.guardNum(d-2, dep)
		g.guardNum(d-1, dep)
		g.a.movsdL(0, r13, (d-2)*sz+offNum)
		g.a.movsdL(1, r13, (d-1)*sz+offNum)
		g.a.ucomisd(0, 1)
		g.a.jcc(ccP, dep) // NaN -> Interpretation (Go-Vergleich)
		var cc int
		switch in.Op {
		case bytecode.OpLt:
			cc = ccB
		case bytecode.OpGt:
			cc = ccA
		case bytecode.OpLe:
			cc = ccBE
		default:
			cc = ccAE
		}
		// Wichtig: das Ergebnisregister VOR dem setcc auf 0 setzen, ohne die
		// Flags anzufassen (xor wrde sie lschen - setcc liest sie sonst aus).
		g.a.movRI(rdx, 0)
		g.a.setcc(cc, rdx)
		g.pushPrologue(d - 2)
		g.wKind(d-2, kBool)
		g.wBoolReg(d-2, rdx)
		g.fall(i, d-1)

	case bytecode.OpEq, bytecode.OpNeq:
		// Wert-Gleichheit nativ (value.Equal): gleiche Kinds entscheiden
		// maschinell - Num exakt (NaN ist ungleich), Bool-Feld und Strings
		// case-sensitiv per Lngen-/Bytevergleich. Gemischte Kinds und
		// Fehlerwerte deopten in die VM (ToString/ToNum-Semantik).
		g.a.movRM(rax, r13, (d-2)*sz+offKind)
		g.a.cmpRI(rax, kError)
		g.a.jcc(ccE, dep)
		g.a.movRM(rdx, r13, (d-1)*sz+offKind)
		g.a.cmpRI(rdx, kError)
		g.a.jcc(ccE, dep)
		g.a.cmpRR(rax, rdx)
		g.a.jcc(ccNE, dep) // gemischte Kinds -> VM (ValuesAreEqual)

		eqTrue := g.a.label()
		eqFalse := g.a.label()
		lnum := g.a.label()
		lbool := g.a.label()
		lstr := g.a.label()

		// Gleiche Kills ohne Fastpath: Null/Nil sind nie ungleich, alle
		// uebrigen (Arr, Map, Obj, ...) nie gleich.
		g.a.cmpRI(rax, kNull)
		g.a.jcc(ccE, eqTrue)
		g.a.cmpRI(rax, kNil)
		g.a.jcc(ccE, eqTrue)
		g.a.cmpRI(rax, kNum)
		g.a.jcc(ccE, lnum)
		g.a.cmpRI(rax, kBool)
		g.a.jcc(ccE, lbool)
		g.a.cmpRI(rax, kStr)
		g.a.jcc(ccE, lstr)
		g.a.jmp(eqFalse)

		// Num/Num: exakter Float-Vergleich; NaN ist ungleich (kein Deopt).
		g.a.mark(lnum)
		g.a.movsdL(0, r13, (d-2)*sz+offNum)
		g.a.movsdL(1, r13, (d-1)*sz+offNum)
		g.a.ucomisd(0, 1)
		g.a.jcc(ccP, eqFalse) // NaN
		g.a.jcc(ccE, eqTrue)
		g.a.jmp(eqFalse)

		// Bool/Bool: nur das Byte vergleichen (Padding unberuehrt lassen).
		g.a.mark(lbool)
		g.a.movzxRM8(rax, r13, (d-2)*sz+offBool)
		g.a.movzxRM8(rdx, r13, (d-1)*sz+offBool)
		g.a.cmpRR(rax, rdx)
		g.a.jcc(ccE, eqTrue)
		g.a.jmp(eqFalse)

		// Str/Str: eerst die Lngen, dann die Bytes (case-sensitiv, wie Go ==).
		// rdx ist nach dem Kind-Dispatch frei und dient als zweiter Scratch;
		// rbx ist tabu (ctx-Zeiger).
		g.a.mark(lstr)
		g.a.movRM(rsi, r13, (d-2)*sz+offStrPtr)
		g.a.movRM(rdx, r13, (d-2)*sz+offStrLen)
		g.a.movRM(rdi, r13, (d-1)*sz+offStrPtr)
		g.a.movRM(rcx, r13, (d-1)*sz+offStrLen)
		g.a.cmpRR(rcx, rdx)
		g.a.jcc(ccNE, eqFalse)
		lchunk := g.a.label()
		lrest := g.a.label()
		g.a.mark(lchunk)
		g.a.cmpRI(rcx, 8)
		g.a.jcc(ccB, lrest)
		g.a.movRM(rax, rsi, 0)
		g.a.movRM(rdx, rdi, 0)
		g.a.cmpRR(rax, rdx)
		g.a.jcc(ccNE, eqFalse)
		g.a.addRI(rsi, 8)
		g.a.addRI(rdi, 8)
		g.a.subRI(rcx, 8)
		g.a.jmp(lchunk)
		g.a.mark(lrest)
		g.a.cmpRI(rcx, 0)
		g.a.jcc(ccE, eqTrue)
		g.a.movzxRM8(rax, rsi, 0)
		g.a.movzxRM8(rdx, rdi, 0)
		g.a.cmpRR(rax, rdx)
		g.a.jcc(ccNE, eqFalse)
		g.a.addRI(rsi, 1)
		g.a.addRI(rdi, 1)
		g.a.subRI(rcx, 1)
		g.a.jmp(lrest)

		// Ergebnis-Bool schreiben; OpNeq kehrt um (eq == !negate).
		done := g.a.label()
		g.a.mark(eqTrue)
		g.a.movRI(rdx, 0)
		if in.Op == bytecode.OpEq {
			g.a.movRI(rdx, 1)
		}
		g.a.jmp(done)
		g.a.mark(eqFalse)
		g.a.movRI(rdx, 1)
		if in.Op == bytecode.OpEq {
			g.a.movRI(rdx, 0)
		}
		g.a.mark(done)
		g.pushPrologue(d - 2)
		g.wKind(d-2, kBool)
		g.wBoolReg(d-2, rdx)
		g.fall(i, d-1)

	// ---------- Logik ----------
	case bytecode.OpNeg:
		// -x ist eine exakte Vorzeichenbit-Flippe (auch fuer NaN), genau wie
		// das unaere Minus im Interpreter.
		g.guardNum(d-1, dep)
		g.a.movsdL(0, r13, (d-1)*sz+offNum)
		g.a.movRI(rcx, fSignMask[0])
		g.a.movqX(1, rcx)
		g.a.xorpd(0, 1)
		g.pushPrologue(d - 1)
		g.wKind(d-1, kNum)
		g.wNumX(d-1, 0)
		g.fall(i, d)

	case bytecode.OpNot, bytecode.OpTruthy:
		truthy, falsy := g.branchTruthy(d-1, dep, in.Op == bytecode.OpTruthy)
		wd := g.a.label()
		g.a.mark(truthy)
		g.a.movRI(rdx, 0) // Not: !wahr; Truthy: wahr -> umdrehen unten
		if in.Op == bytecode.OpTruthy {
			g.a.movRI(rdx, 1)
		}
		g.a.jmp(wd)
		g.a.mark(falsy)
		g.a.movRI(rdx, 0)
		if in.Op == bytecode.OpNot {
			g.a.movRI(rdx, 1)
		}
		g.a.mark(wd)
		g.pushPrologue(d - 1)
		g.wKind(d-1, kBool)
		g.wBoolReg(d-1, rdx)
		g.fall(i, d)

	case bytecode.OpShortCircuit:
		truthy, falsy := g.branchTruthy(d-1, dep, false)
		lt := g.a.label()
		g.a.mark(truthy)
		g.a.movRI(rdx, 1)
		g.a.jmp(lt)
		g.a.mark(falsy)
		g.a.movRI(rdx, 0)
		g.a.mark(lt)
		// Genommen genau dann, wenn Wahrheit == (B == 1).
		taken := g.a.label()
		g.a.cmpRI(rdx, 1)
		if in.B == 1 {
			g.a.jcc(ccE, taken)
			g.fall(i, d-1)
		} else {
			g.a.jcc(ccNE, taken)
			g.fall(i, d-1)
		}
		g.a.mark(taken)
		g.pushPrologue(d - 1)
		g.wKind(d-1, kBool)
		g.wBoolReg(d-1, rdx)
		g.jumpTo(i, in.A, d)

	// ---------- Sprünge ----------
	case bytecode.OpJump:
		g.jumpTo(i, in.A, d)

	case bytecode.OpJumpIfFalse, bytecode.OpJumpIfTrue:
		truthy, falsy := g.branchTruthy(d-1, dep, false)
		if in.Op == bytecode.OpJumpIfFalse {
			g.a.mark(truthy)
			g.fall(i, d-1)
			g.a.mark(falsy)
			g.jumpTo(i, in.A, d-1)
		} else {
			g.a.mark(falsy)
			g.fall(i, d-1)
			g.a.mark(truthy)
			g.jumpTo(i, in.A, d-1)
		}

	case bytecode.OpSelectDispatch:
		t := &g.chunk.Selects[in.A]
		// Numeric values use the compile-time dispatch table directly. Bool
		// values fall through natively (VM: "continue" ohne Pop), damit der
		// nachfolgende Scan nativ weiterläuft. Everything else deopts.
		g.a.movRM(rsi, r13, (d-1)*sz+offKind)
		boolFall := g.a.label()
		g.a.cmpRI(rsi, kBool)
		g.a.jcc(ccE, boolFall)
		g.a.cmpRI(rsi, kNum)
		g.a.jcc(ccNE, dep)
		g.a.movsdL(0, r13, (d-1)*sz+offNum)
		fallback := g.a.label()
		for key, idx := range t.Num {
			hit := g.a.label()
			next := g.a.label()
			g.a.movRI(rcx, math.Float64bits(key))
			g.a.movqX(1, rcx)
			g.a.ucomisd(0, 1)
			g.a.jcc(ccP, fallback) // NaN is never equal to a table key.
			g.a.jcc(ccE, hit)
			g.a.jmp(next)
			g.a.mark(hit)
			g.jumpTo(i, t.Targets[idx], d-1)
			g.a.mark(next)
		}
		g.a.mark(fallback)
		g.jumpTo(i, t.Default, d-1)
		g.a.mark(boolFall)
		g.fall(i, d)

	case bytecode.OpCaseEq:
		// [wert, bedingung] -> Bool. cond liegt oben (d-1), v darunter.
		// Native entscheidbar bei gleichen Kinds: Num (exakt, NaN ->
		// Interpretation), Bool, Null/Nil (immer gleich). Sonst deopten
		// (Strings via EqualFold, Cross-Type, gemischte Kinds).
		vs := (d - 2) * sz
		cs := (d - 1) * sz
		ln := g.a.label()
		lb := g.a.label()
		lz := g.a.label()
		lergebnis := g.a.label()
		g.a.movRM(rsi, r13, vs)
		g.a.movRM(rdx, r13, cs)
		g.a.cmpRR(rsi, rdx)
		g.a.jcc(ccNE, dep)
		g.a.cmpRI(rsi, kNum)
		g.a.jcc(ccE, ln)
		g.a.cmpRI(rsi, kBool)
		g.a.jcc(ccE, lb)
		g.a.cmpRI(rsi, kNull)
		g.a.jcc(ccE, lz)
		g.a.cmpRI(rsi, kNil)
		g.a.jcc(ccE, lz)
		g.a.jmp(dep)
		// Num: v.Num == cond.Num exakt (NaN -> Interpretation, ergibt false).
		g.a.mark(ln)
		g.a.movsdL(0, r13, vs+offNum)
		g.a.movsdL(1, r13, cs+offNum)
		g.a.ucomisd(0, 1)
		g.a.jcc(ccP, dep)
		g.a.movRI(rdx, 0)
		g.a.setcc(ccE, rdx)
		g.a.jmp(lergebnis)
		// Bool: v.Bool == cond.Bool.
		g.a.mark(lb)
		g.a.movRM(rdx, r13, vs+offBool)
		g.a.movRM(rsi, r13, cs+offBool)
		g.a.cmpRR(rdx, rsi)
		g.a.movRI(rdx, 0)
		g.a.setcc(ccE, rdx)
		g.a.jmp(lergebnis)
		// Null == Null (und Nil == Nil): immer wahr.
		g.a.mark(lz)
		g.a.movRI(rdx, 1)
		g.a.mark(lergebnis)
		g.pushPrologue(d - 2)
		g.wKind(d-2, kBool)
		g.wBoolReg(d-2, rdx)
		g.fall(i, d-1)

	case bytecode.OpCaseRange:
		// [wert, low, high] -> Bool: v >= low && v <= hi (float64-Vergleiche,
		// jede NaN-Beteiligung liefert false wie im Interpreter).
		g.guardNum(d-3, dep)
		g.guardNum(d-2, dep)
		g.guardNum(d-1, dep)
		done := g.a.label()
		out := g.a.label()
		g.a.movsdL(0, r13, (d-3)*sz+offNum) // v
		g.a.movsdL(1, r13, (d-2)*sz+offNum) // low
		g.a.ucomisd(0, 1)
		g.a.jcc(ccB, out)                   // v < low; unordered (NaN in v/low) -> CF=1 -> false
		g.a.movsdL(1, r13, (d-1)*sz+offNum) // high
		g.a.ucomisd(0, 1)
		g.a.jcc(ccP, out) // unordered: v <= high ist false
		g.a.jcc(ccA, out) // v > high
		g.a.movRI(rdx, 1) // im Bereich
		g.a.jmp(done)
		g.a.mark(out)
		g.a.movRI(rdx, 0)
		g.a.mark(done)
		g.pushPrologue(d - 3)
		g.wKind(d-3, kBool)
		g.wBoolReg(d-3, rdx)
		g.fall(i, d-2)

	case bytecode.OpCaseIs:
		// [wert, vergleichswert] -> BoolVal(evalBinary(...)). Native für zwei
		// Zahlen: GT/LT/GE/LE und EQ/NEQ (NaN-Sonderfälle wie equals).
		g.a.movRM(rax, r13, (d-2)*sz)
		g.a.cmpRI(rax, kNum)
		g.a.jcc(ccNE, dep)
		g.a.movRM(rax, r13, (d-1)*sz)
		g.a.cmpRI(rax, kNum)
		g.a.jcc(ccNE, dep)
		g.a.movsdL(0, r13, (d-2)*sz+offNum) // v
		g.a.movsdL(1, r13, (d-1)*sz+offNum) // cmp
		g.a.ucomisd(0, 1)
		var cc int
		naVal := int32(0) // Ergebniswert im unordered-Fall (NaN)
		switch token.Type(in.A) {
		case token.EQ:
			cc, naVal = ccE, 0
		case token.NEQ:
			cc, naVal = ccNE, 1
		case token.GT:
			cc = ccA
		case token.LT:
			cc = ccB
		case token.GE:
			cc = ccAE
		default: // token.LE
			cc = ccBE
		}
		// Ergebnisregister vor dem setcc auf den NaN-Wert setzen (movRI
		// verändert die Flags von ucomisd nicht); unordered springt direkt.
		g.a.movRI(rdx, uint64(uint32(naVal)))
		wr := g.a.label()
		g.a.jcc(ccP, wr)
		g.a.setcc(cc, rdx)
		g.a.mark(wr)
		g.pushPrologue(d - 2)
		g.wKind(d-2, kBool)
		g.wBoolReg(d-2, rdx)
		g.fall(i, d-1)

	case bytecode.OpIsError:
		// Pop -> Bool: Kind == KindError. Immer maschinell entscheidbar.
		g.a.movRM(rax, r13, (d-1)*sz) // Kind
		g.a.movRI(rdx, 0)
		g.a.cmpRI(rax, kError)
		g.a.setcc(ccE, rdx)
		g.pushPrologue(d - 1)
		g.wKind(d-1, kBool)
		g.wBoolReg(d-1, rdx)
		g.fall(i, d)

	case bytecode.OpJumpIfError, bytecode.OpCheck:
		// Fehler oben: zur Interpretation (die den Fehler auslöst bzw.
		// die Stack-Korrektur macht).
		g.a.movRM(rax, r13, (d-1)*sz)
		g.a.cmpRI(rax, kError)
		g.a.jcc(ccE, dep)
		g.fall(i, d)

	// ---------- For ----------
	case bytecode.OpForInit:
		// Beide Stack-Werte sind Zahlen (sonst: Interpretation wandelt um),
		// die Schleifenvariable muss im aktuellen Scope existieren.
		g.guardNum(d-2, dep)
		g.guardNum(d-1, dep)
		g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.A)), true))
		g.a.cmpRI(rdx, 0)
		g.a.jcc(ccE, dep)
		g.fall(i, d)

	case bytecode.OpForTest:
		g.guardNum(d-2, dep) // cur
		g.guardNum(d-1, dep) // end
		g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.B)), true))
		g.a.cmpRI(rdx, 0)
		g.a.jcc(ccE, dep) // fehlt: Interpretation legt die Variable an
		g.a.movRR(rsi, rax)
		g.a.movsdL(0, r13, (d-2)*sz+offNum) // cur
		g.a.movsdL(1, r13, (d-1)*sz+offNum) // end
		g.a.ucomisd(0, 1)
		out := g.a.label()
		if in.C == 1 { // aufsteigend: weiter bei cur <= end
			g.a.jcc(ccP, out)
			g.a.jcc(ccA, out)
		} else { // absteigend: weiter bei cur >= end
			g.a.jcc(ccP, out)
			g.a.jcc(ccB, out)
		}
		g.a.movMI(rsi, offKind, kNum)
		g.a.movRM(rdx, r13, (d-2)*sz+offNum)
		g.a.movMR(rsi, offNum, rdx)
		g.fall(i, d)
		g.a.mark(out)
		g.jumpTo(i, in.A, d)

	case bytecode.OpForStep:
		g.a.movsdL(0, r13, (d-2)*sz+offNum)
		g.a.movRI(rcx, math.Float64bits(g.chunk.Consts[in.A].Num))
		g.a.movqX(1, rcx)
		g.a.arithSD(0x58, 0, 1)
		g.a.movsdS(r13, (d-2)*sz+offNum, 0)
		g.fall(i, d)

	case bytecode.OpEachInit:
		// ForEach-Start nativ: iterierbar muss ein 1D-Array sein (Maps
		// bauen die Schluesselliste in Go, 2D kopiert Rows - beides in
		// der VM). Die Loop-Variablen muessen existieren; das nativ
		// anzulegen (GetRef) waere ein Go-Aufruf - fehlende deoptieren
		// in die VM, die BothInit mit Anlegen ausfuehrt.
		g.a.cmpMI(r13, (d-1)*sz+offKind, kArr)
		g.a.jcc(ccNE, dep)
		g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.A)), false))
		g.a.cmpRI(rdx, 0)
		g.a.jcc(ccE, dep)
		if in.B > 0 {
			g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.B-1)), false))
			g.a.cmpRI(rdx, 0)
			g.a.jcc(ccE, dep)
		}
		// Zaehler (Num 0) und Schluesselliste (Null fuer 1D-Arrays) pushen.
		g.pushPrologue(d)
		g.a.movRI(rdx, 0)
		g.wKind(d, kNum)
		g.wNumReg(d, rdx)
		g.pushPrologue(d + 1)
		g.wKind(d+1, kNull)
		g.fall(i, d+2)

	case bytecode.OpEachNext:
		// ForEach-Iteration nativ: Stack = [iterierbar(d-3), Zaehler(d-2),
		// Schluesselliste(d-1)]. Nur 1D-Arrays und nur Elemente, die ohne
		// Zeiger geschrieben werden duerfen (Num/Bool/Null); Str-/Arr-/
		// Map-Elemente und 2D-Rows deoptieren pro Iteration in die VM.
		iter := (d - 3) * sz
		cnt := (d - 2) * sz
		g.a.cmpMI(r13, iter+offKind, kArr)
		g.a.jcc(ccNE, dep)
		g.a.cmpMI(r13, cnt+offKind, kNum)
		g.a.jcc(ccNE, dep)
		g.a.movsdL(0, r13, cnt+offNum)
		g.a.cvttsd2si(rax, 0)
		g.a.cmpRI(rax, 0)
		g.a.jcc(ccL, dep) // i < 0 ist unmoeglich; Falls doch: gleiche VM-Panik
		out := g.a.label()
		g.a.movRM(rdx, r13, iter+offArrLen)
		g.a.cmpRR(rax, rdx)
		g.a.jcc(ccGE, out)  // erschöpft -> zum Schleifenende
		g.a.movRR(rdi, rax) // i retten: RAX wird gleich beim Datenzeiger mul'iert
		g.a.movRM(rcx, r13, iter+offArrData)
		g.a.imulRI(rax, slotBytes)
		g.a.addRR(rcx, rax) // rcx = &Elemente[i]

		g.a.cvtsi2sdX(1, rdi) // xmm1 = float64(i) (für die Schlüsselvariable)

		// Value-Variablen: Zwei-Variablen-Form (C > 0) schreibt key und
		// val; Einvariablen-Form (C == 0) schreibt nur Names[B].
		valName := int(in.B)
		if in.C > 0 {
			valName = int(in.C - 1)
		}
		valIdx := g.slotIdxOf(g.ci.name(valName), false)

		// Schlüsselvariablen schreiben (nur in der Zwei-Variablen-Form,
		// C > 0: key = NumVal(i)). float(i) liegt in xmm1.
		writeKey := func() {
			if in.C > 0 {
				g.cellAndFlags(g.slotIdxOf(g.ci.name(int(in.B)), false))
				g.a.cmpRI(rdx, 0)
				g.a.jcc(ccE, dep)
				g.a.movMI(rax, offKind, kNum)
				g.a.movXQ(rdx, 1)
				g.a.movMR(rax, offNum, rdx)
			}
		}

		// WICHTIG: der Zaehler wird erst NACH dem Element-Dispatch
		// fortgeschrieben (auf "done"). Jeder Deopt-Pfad (Str-/Arr-/
		// Map-Element, fehlende Loop-Variable, int()-Randfälle) muss einen
		// unberührten Zustand hinterlassen: die VM setzt die Ausführung
		// VOR dem OpEachNext fort und liest den Zaehler erneut - eine
		// vorgezogene Erhöhung würde dort ein Element überspringen.
		g.a.movRM(rsi, rcx, offKind)
		wnum := g.a.label()
		wbool := g.a.label()
		wnull := g.a.label()
		done := g.a.label()
		g.a.cmpRI(rsi, kNum)
		g.a.jcc(ccE, wnum)
		g.a.cmpRI(rsi, kBool)
		g.a.jcc(ccE, wbool)
		g.a.cmpRI(rsi, kNull)
		g.a.jcc(ccE, wnull)
		g.a.jmp(dep) // Str/Arr/Map-Element -> VM (Zeiger-Schreibverbot)

		g.a.mark(wnum)
		// Element-Num in xmm2 sichern, bevor cellAndFlags rcx verbraucht.
		g.a.movsdL(2, rcx, offNum)
		g.cellAndFlags(valIdx)
		g.a.cmpRI(rdx, 0)
		g.a.jcc(ccE, dep)
		g.a.movMI(rax, offKind, kNum)
		g.a.movXQ(rdx, 2)
		g.a.movMR(rax, offNum, rdx)
		writeKey()
		g.a.jmp(done)

		g.a.mark(wbool)
		// Bool-Feld als 8 Byte kopieren (Bool + Padding, wie OpStore).
		g.a.movsdL(2, rcx, offBool)
		g.cellAndFlags(valIdx)
		g.a.cmpRI(rdx, 0)
		g.a.jcc(ccE, dep)
		g.a.movMI(rax, offKind, kBool)
		g.a.movXQ(rdx, 2)
		g.a.movMR(rax, offBool, rdx)
		writeKey()
		g.a.jmp(done)

		g.a.mark(wnull)
		g.cellAndFlags(valIdx)
		g.a.cmpRI(rdx, 0)
		g.a.jcc(ccE, dep)
		g.a.movMI(rax, offKind, kNull)
		writeKey()

		// Nur angeschriebene Elemente kommen hierher: jetzt erst den
		// Zaehler auf float64(i+1) setzen (rax = i liegt in RDI).
		g.a.mark(done)
		g.a.movRR(rax, rdi)
		g.a.addRI(rax, 1)
		g.a.cvtsi2sdX(0, 0)
		g.wKind(d-2, kNum)
		g.wNumX(d-2, 0)
		g.fall(i, d)
		g.a.mark(out)
		g.jumpTo(i, in.A, d)

	default:
		// Kann nicht vorkommen (supported() filtert), aber nicht stumm lassen.
		g.fall(i, d)
	}

	g.deoptBlock(dep, i, d)
}
