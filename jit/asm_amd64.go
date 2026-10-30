//go:build amd64

// Kleiner x86-64-Encoder für den JIT.
//
// Bewusst auf das reduziert, was der Codegenerator braucht: keine komplette
// Assembler-Frontend, sondern die wenigen Anweisungen, die aus Bytecode
// Maschinencode erzeugen. Alle Sprünge sind relativ (rel32), Labels werden
// am Ende aufgelöst.
package jit

import "encoding/binary"

// Register (x86-64). Die Kurznamen entsprechen der Bezeichnung im
// Codegenerator.
const (
	rAX = 0
	rCX = 1
	rDX = 2
	rBX = 3
	rSP = 4
	rBP = 5
	rSI = 6
	rDI = 7

	rax = rAX
	rcx = rCX
	rdx = rDX
	rbx = rBX
	rsp = rSP
	rbp = rBP
	rsi = rSI
	rdi = rDI
	r8  = 8
	r9  = 9
	r10 = 10
	r11 = 11
	r12 = 12
	r13 = 13
	r14 = 14
	r15 = 15
)

// Bedingungen für jcc/setcc (Nibble-Wert).
const (
	ccB  = 2  // below (CF=1)
	ccAE = 3  // above or equal (CF=0)
	ccE  = 4  // equal (ZF=1)
	ccNE = 5  // not equal (ZF=0)
	ccBE = 6  // below or equal (CF=1 | ZF=1)
	ccA  = 7  // above (CF=0 & ZF=0)
	ccP  = 10 // parity (PF=1) = ungerichtet (NaN)
	ccL  = 12 // less (SF!=OF)
	ccGE = 13 // greater or equal (SF==OF)
	ccG  = 15 // greater (SF==OF & ZF=0)
)

// XMM-Register
const (
	xmm0 = 0
	xmm1 = 1
	xmm2 = 2
)

type jumpFix struct {
	at    int
	label int
}

type asm struct {
	buf    []byte
	labels []int // Position des Labels oder -1
	refs   []int // wie oft auf das Label verwiesen wird (toter Code sparen)
	jumps  []jumpFix
}

func newAsm() *asm { return &asm{} }

func (a *asm) label() int {
	a.labels = append(a.labels, -1)
	a.refs = append(a.refs, 0)
	return len(a.labels) - 1
}

func (a *asm) mark(l int) { a.labels[l] = len(a.buf) }

func (a *asm) emit(b ...byte) { a.buf = append(a.buf, b...) }

func (a *asm) u32(v uint32) { a.buf = binary.LittleEndian.AppendUint32(a.buf, v) }

func (a *asm) u64(v uint64) { a.buf = binary.LittleEndian.AppendUint64(a.buf, v) }

// fix reserviert ein rel32-Feld für label (am Ende aufgelöst) und zählt die
// Referenz, damit ungenutzte Blöcke wegfallen.
func (a *asm) fix(label int) {
	a.refs[label]++
	a.jumps = append(a.jumps, jumpFix{at: len(a.buf), label: label})
	a.u32(0)
}

// finish löst alle Sprung-Relativadressen auf.
func (a *asm) finish() {
	for _, j := range a.jumps {
		target := a.labels[j.label]
		if target < 0 {
			panic("jit: unmarkiertes Label")
		}
		off := int32(target) - int32(j.at+4)
		binary.LittleEndian.PutUint32(a.buf[j.at:], uint32(off))
	}
}

// rex setzt das REX-Präfix. w = 64-Bit-Operand, reg/index/rm tragen Bit 3.
func (a *asm) rex(w bool, reg, index, rm int) {
	r := byte(0x40)
	if w {
		r |= 8
	}
	if reg&8 != 0 {
		r |= 4
	}
	if index&8 != 0 {
		r |= 2
	}
	if rm&8 != 0 {
		r |= 1
	}
	if r != 0x40 {
		a.buf = append(a.buf, r)
	}
}

// rexByte erzwingt REX für Byte-Register: ab SPL (4) ist REX nötig, sonst
// würden AH/CH/DH/BH gemeint.
func (a *asm) rexByte(reg int) {
	if reg >= 8 {
		a.buf = append(a.buf, 0x41)
	} else if reg >= 4 {
		a.buf = append(a.buf, 0x40)
	}
}

func (a *asm) modrm(reg, rm int) {
	a.buf = append(a.buf, 0xC0|byte(reg&7)<<3|byte(rm&7))
}

// modrmMem ModRM für [base+disp] inkl. SIB und Displacements.
func (a *asm) modrmMem(reg, base int, disp int32) {
	rm := base & 7
	var mod byte
	switch {
	case disp == 0 && rm != 5:
		mod = 0
	case disp >= -128 && disp <= 127:
		mod = 1
	default:
		mod = 2
	}
	a.buf = append(a.buf, mod<<6|byte(reg&7)<<3|byte(rm))
	if rm == 4 { // RSP/R12 braucht eine SIB-Basis
		a.buf = append(a.buf, 0x24)
	}
	switch mod {
	case 1:
		a.buf = append(a.buf, byte(int8(disp)))
	case 2:
		a.u32(uint32(disp))
	}
}

// ---------------- Bewegungen ----------------

func (a *asm) push(r int) {
	if r >= 8 {
		a.emit(0x41)
	}
	a.emit(byte(0x50 + (r & 7)))
}

func (a *asm) pop(r int) {
	if r >= 8 {
		a.emit(0x41)
	}
	a.emit(byte(0x58 + (r & 7)))
}

// movRI lädt eine 64-Bit-Konstante (movabs).
func (a *asm) movRI(r int, imm uint64) {
	a.rex(true, 0, 0, r)
	a.emit(byte(0xB8 + (r & 7)))
	a.u64(imm)
}

// movRR: dst <- src
func (a *asm) movRR(dst, src int) {
	a.rex(true, src, 0, dst)
	a.emit(0x89)
	a.modrm(src, dst)
}

// movRM: dst <- [base+disp]
func (a *asm) movRM(dst, base int, disp int32) {
	a.rex(true, dst, 0, base)
	a.emit(0x8B)
	a.modrmMem(dst, base, disp)
}

// movMR: [base+disp] <- src
func (a *asm) movMR(base int, disp int32, src int) {
	a.rex(true, src, 0, base)
	a.emit(0x89)
	a.modrmMem(src, base, disp)
}

// movMI: qword [base+disp] <- imm32 (vorzeichenert erweitert)
func (a *asm) movMI(base int, disp int32, imm int32) {
	a.rex(true, 0, 0, base)
	a.emit(0xC7)
	a.modrmMem(0, base, disp)
	a.u32(uint32(imm))
}

// movMI8: byte [base+disp] <- imm8
func (a *asm) movMI8(base int, disp int32, imm byte) {
	a.rex(false, 0, 0, base)
	a.emit(0xC6)
	a.modrmMem(0, base, disp)
	a.emit(imm)
}

// cmpMI: qword [base+disp] <-> imm32 (vorzeichenert)
func (a *asm) cmpMI(base int, disp int32, imm int32) {
	a.rex(true, 0, 0, base)
	a.emit(0x81)
	a.modrmMem(7, base, disp)
	a.u32(uint32(imm))
}

// movMR8: byte [base+disp] <- src8
func (a *asm) movMR8(base int, disp int32, src int) {
	a.rexByte(src)
	a.rex(false, src, 0, base)
	a.emit(0x88)
	a.modrmMem(src, base, disp)
}

// movRM8: dst8 <- byte [base+disp] (keine Zero-Extension!)
func (a *asm) movRM8(dst, base int, disp int32) {
	a.rexByte(dst)
	a.rex(false, dst, 0, base)
	a.emit(0x8A)
	a.modrmMem(dst, base, disp)
}

// movzxRM8: dst <- zero-extendiertes byte [base+disp] (0F B6 /r)
func (a *asm) movzxRM8(dst, base int, disp int32) {
	a.rex(false, dst, 0, base)
	a.emit(0x0F, 0xB6)
	a.modrmMem(dst, base, disp)
}

// lea: dst <- base+disp
func (a *asm) leaRM(dst, base int, disp int32) {
	a.rex(true, dst, 0, base)
	a.emit(0x8D)
	a.modrmMem(dst, base, disp)
}

// ---------------- Rechnen / Vergleichen ----------------

func (a *asm) addRR(dst, src int) {
	a.rex(true, src, 0, dst)
	a.emit(0x01)
	a.modrm(src, dst)
}

func (a *asm) addRI(dst int, imm int32) {
	a.rex(true, 0, 0, dst)
	a.emit(0x81)
	a.modrm(0, dst)
	a.u32(uint32(imm))
}

func (a *asm) subRI(dst int, imm int32) {
	a.rex(true, 0, 0, dst)
	a.emit(0x81)
	a.modrm(5, dst)
	a.u32(uint32(imm))
}

func (a *asm) cmpRI(dst int, imm int32) {
	a.rex(true, 0, 0, dst)
	a.emit(0x81)
	a.modrm(7, dst)
	a.u32(uint32(imm))
}

func (a *asm) cmpRR(dst, src int) {
	a.rex(true, src, 0, dst)
	a.emit(0x39)
	a.modrm(src, dst)
}

func (a *asm) testRR(x, y int) {
	a.rex(true, x, 0, y)
	a.emit(0x85)
	a.modrm(x, y)
}

func (a *asm) xorRR(dst, src int) {
	a.rex(true, src, 0, dst)
	a.emit(0x31)
	a.modrm(src, dst)
}

func (a *asm) andRR(dst, src int) {
	a.rex(true, src, 0, dst)
	a.emit(0x21)
	a.modrm(src, dst)
}

// andRI: dst &= imm32 (sign-erweitert auf 64 Bit)
func (a *asm) andRI(dst int, imm int32) {
	a.rex(true, dst, 0, dst)
	a.emit(0x81)
	a.modrm(4, dst)
	a.u32(uint32(imm))
}

func (a *asm) orRR(dst, src int) {
	a.rex(true, src, 0, dst)
	a.emit(0x09)
	a.modrm(src, dst)
}

// imulRI: dst *= imm32
func (a *asm) imulRI(dst int, imm int32) {
	a.rex(true, dst, 0, dst)
	a.emit(0x69)
	a.modrm(dst, dst)
	a.u32(uint32(imm))
}

// subRR: dst -= src
func (a *asm) subRR(dst, src int) {
	a.rex(true, src, 0, dst)
	a.emit(0x29) // SUB r/m64, r64 (dst = m/r, src = reg)
	a.modrm(src, dst)
}

// notRR: dst = ^dst (alle Bits invertieren)
func (a *asm) notRR(dst int) {
	a.rex(true, dst, 0, dst)
	a.emit(0xF7)
	a.modrm(2, dst)
}

// shlRI(shrRI): dst um imm8 nach links/rechts verschieben.
func (a *asm) shlRI(dst int, n int) {
	a.rex(true, dst, 0, dst)
	a.emit(0xC1)
	a.modrm(4, dst)
	a.emit(byte(n))
}

func (a *asm) shrRI(dst int, n int) {
	a.rex(true, dst, 0, dst)
	a.emit(0xC1)
	a.modrm(5, dst)
	a.emit(byte(n))
}

// shlRC/shrRC: Shifts um CL (Bitanzahl vorher in ein Register laden; die
// Shift-Befehle lesen CL nur und mutieren es nicht).
func (a *asm) shlRC(dst int) {
	a.rex(true, dst, 0, dst)
	a.emit(0xD3)
	a.modrm(4, dst)
}

func (a *asm) shrRC(dst int) {
	a.rex(true, dst, 0, dst)
	a.emit(0xD3)
	a.modrm(5, dst)
}

// decM: qword [base+disp]--
func (a *asm) decM(base int, disp int32) {
	a.rex(true, 0, 0, base)
	a.emit(0xFF)
	a.modrmMem(1, base, disp)
}

// setcc schreibt 0/1 in ein Byte-Register.
func (a *asm) setcc(cc int, r int) {
	a.rexByte(r)
	a.emit(0x0F, byte(0x90+cc))
	a.modrm(0, r)
}

// ---------------- Sprünge ----------------

func (a *asm) jmp(label int) {
	a.emit(0xE9)
	a.fix(label)
}

func (a *asm) jcc(cc int, label int) {
	a.emit(0x0F, byte(0x80+cc))
	a.fix(label)
}

func (a *asm) ret() { a.emit(0xC3) }

// ---------------- SSE2 ----------------

// sseRex: REX für SSE-Anweisungen (Präfix, dann REX, dann Opcode).
func (a *asm) sseRex(reg, base int, w bool) {
	r := byte(0x40)
	if w {
		r |= 8
	}
	if reg&8 != 0 {
		r |= 4
	}
	if base&8 != 0 {
		r |= 1
	}
	if r != 0x40 {
		a.buf = append(a.buf, r)
	}
}

func (a *asm) pxor(x, y int) {
	a.emit(0x66) // REX muss VOR dem Präfix-Byte, aber NACH 66 stehen
	if x&8 != 0 || y&8 != 0 {
		a.rex(false, x, 0, y)
	}
	a.emit(0x0F, 0xEF)
	a.modrm(x, y)
}

// movsdL: xmm <- qword [base+disp]
func (a *asm) movsdL(x, base int, disp int32) {
	a.emit(0xF2)
	a.sseRex(x, base, false)
	a.emit(0x0F, 0x10)
	a.modrmMem(x, base, disp)
}

// movsdS: qword [base+disp] <- xmm
func (a *asm) movsdS(base int, disp int32, x int) {
	a.emit(0xF2)
	a.sseRex(x, base, false)
	a.emit(0x0F, 0x11)
	a.modrmMem(x, base, disp)
}

// movdquS: xmmword [base+disp] <- xmm (nicht ausgerichtet)
func (a *asm) movdquS(base int, disp int32, x int) {
	a.emit(0xF3)
	a.sseRex(x, base, false)
	a.emit(0x0F, 0x7F)
	a.modrmMem(x, base, disp)
}

func (a *asm) arithSD(op byte, x, y int) { // addsd/subsd/mulsd/divsd
	a.emit(0xF2)
	if x&8 != 0 || y&8 != 0 {
		a.rex(false, x, 0, y)
	}
	a.emit(0x0F, op)
	a.modrm(x, y)
}

func (a *asm) ucomisd(x, y int) {
	a.emit(0x66)
	if x&8 != 0 || y&8 != 0 {
		a.rex(false, x, 0, y)
	}
	a.emit(0x0F, 0x2E)
	a.modrm(x, y)
}

func (a *asm) sqrtsd(dst, src int) {
	a.emit(0xF2)
	if dst&8 != 0 || src&8 != 0 {
		a.rex(false, dst, 0, src)
	}
	a.emit(0x0F, 0x51)
	a.modrm(dst, src)
}

func (a *asm) xorpd(x, y int) {
	a.emit(0x66)
	if x&8 != 0 || y&8 != 0 {
		a.rex(false, x, 0, y)
	}
	a.emit(0x0F, 0x57)
	a.modrm(x, y)
}

// xorpdM: xmm ^= qword [base+disp] - wird genutzt, um ein Vorzeichenbit zu
// flippen (exakt wie das unäre Minus im Interpreter, auch für NaN).
func (a *asm) xorpdM(x, base int, disp int32) {
	a.emit(0x66)
	a.sseRex(x, base, false)
	a.emit(0x0F, 0x57)
	a.modrmMem(x, base, disp)
}

// roundsdM: xmm <- gerundetes qword [base+disp] (SSE4.1, 66 0F 3A 0B /r ib).
// mode: 1 = gegen -unendlich (Floor), 2 = gegen +unendlich (Ceil), 3 = gegen 0
// (Trunc). NaN, ±Inf und ±0 laufen durch - exakt wie math.Floor/Ceil/Trunc
// in Go (bitidentisch).
func (a *asm) roundsdM(x, base int, disp int32, mode byte) {
	a.emit(0x66)
	a.sseRex(x, base, false)
	a.emit(0x0F, 0x3A, 0x0B)
	a.modrmMem(x, base, disp)
	a.emit(mode)
}

// movqX: xmm <- r64 (Ganzzahl in XMM-Register)
func (a *asm) movqX(x, r int) {
	a.emit(0x66)
	a.rex(true, x, 0, r)
	a.emit(0x0F, 0x6E)
	a.modrm(x, r)
}

// movXQ: r64 <- xmm low qword.
func (a *asm) movXQ(r, x int) {
	a.emit(0x66)
	a.rex(true, x, 0, r)
	a.emit(0x0F, 0x7E)
	a.modrm(x, r)
}

// cvtsi2sdX: xmm <- float64(int64(r)).
func (a *asm) cvtsi2sdX(x, r int) {
	a.emit(0xF2)
	a.rex(true, x, 0, r)
	a.emit(0x0F, 0x2A)
	a.modrm(x, r)
}

// cvttsd2si: r <- int64(trunc(xmm)).
func (a *asm) cvttsd2si(r, x int) {
	a.emit(0xF2)
	a.rex(true, r, 0, x)
	a.emit(0x0F, 0x2C)
	a.modrm(r, x)
}

// cqo: RDX:RAX <- Vorzeichenwort von RAX (Vorbereitung für signed idiv).
func (a *asm) cqo() {
	a.emit(0x48, 0x99)
}

// idivR: RDX:RAX signed durch r dividieren; Quotient landet in RAX, der
// Rest in RDX. Ein Trap (INT64_MIN / -1) muss vom Aufrufer vorher
// ausgeschlossen werden.
func (a *asm) idivR(r int) {
	a.rex(true, 0, 0, r)
	a.emit(0xF7)
	a.modrm(7, r)
}
