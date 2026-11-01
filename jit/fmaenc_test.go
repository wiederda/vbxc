//go:build amd64

package jit

import (
	"encoding/hex"
	"fmt"
	"math"
	"testing"
	"unsafe"
)

// runScalarFF erzeugt ein Maschinencode-Fragment func(float64) float64
// (Register-ABI für unsicher gecastete Funktionswerte: Argument in X0,
// Ergebnis in X0) und führt es aus. r10/r11 als Scratch.
func runScalarFF(t *testing.T, code func(a *asm), name string) func(float64) float64 {
	t.Helper()
	a := newAsm()
	code(a)
	a.ret()
	a.finish()
	m, err := allocExec(a.buf)
	if err != nil {
		t.Fatalf("%s: allocExec: %v", name, err)
	}
	t.Cleanup(m.free)
	fmt.Printf("%s: %s\n", name, hex.EncodeToString(a.buf))
	fv := []uintptr{m.ptr}
	return *(*func(float64) float64)(unsafe.Pointer(&fv))
}

// loadXmmImm lädt eine float64-Konstante über r10 in xmm x.
func loadXmmImm(a *asm, x int, v float64) {
	a.movRI(10, math.Float64bits(v))
	a.movqX(x, 10)
}

// TestVexEncodings validiert die VEX/FMA-Encoder und die CVTS/CVTT-Varianten
// direkt auf der Hardware: das Ergebnis entscheidet über Operandenreihenfolge
// und Opcodes, kein Disassembler nötig.
func TestVexEncodings(t *testing.T) {
	// vaddSD(dst=1, src1=0, src2=2): X2 = 5.0; erwartet X1 = X0 + 5 -> zurück X0 = X1.
	vadd := runScalarFF(t, func(a *asm) {
		loadXmmImm(a, 2, 5.0)
		a.vaddSD(1, 0, 2)
		a.movXQ(10, 1) // r10 <- Bits(X1)
		a.movqX(0, 10) // X0 <- Bits(r10)
	}, "vaddSD 1,0,2")
	if got := vadd(3.0); got != 8.0 {
		t.Errorf("vaddSD(1,0,2) mit 3+5: %+v, wollen 8", got)
	}
	// Serie von Sonden:登録 und FMA-Fähigkeit klären.
	if hasFMA {
		t.Log("CPU: FMA3 ja")
	} else {
		t.Log("CPU: FMA3 NEIN")
	}
	// p0: nur X1 zurücklesen (Integrität).
	p0 := runScalarFF(t, func(a *asm) {
		loadXmmImm(a, 0, 3.0)
		loadXmmImm(a, 1, 2.0)
		loadXmmImm(a, 2, 10.0)
		a.movXQ(10, 1)
		a.movqX(0, 10)
	}, "p0 X1 lesen")
	if got := p0(0); got != 2.0 {
		t.Errorf("p0 X1: %+v, wollen 2", got)
	}
	// p1: legacy mulsd X1,X0 -> X1 = 6 (SSE-Referenz).
	p1 := runScalarFF(t, func(a *asm) {
		loadXmmImm(a, 0, 3.0)
		loadXmmImm(a, 1, 2.0)
		loadXmmImm(a, 2, 10.0)
		a.arithSD(0x59, 1, 0) // mulsd xmm1, xmm0
		a.movXQ(10, 1)
		a.movqX(0, 10)
	}, "p1 mulsd")
	if got := p1(0); got != 6.0 {
		t.Errorf("p1 mulsd: %+v, wollen 6", got)
	}
	// p2: VADDSD 231-Form? gibt es nicht; stattdessen: X1 = X0 + X2 via VADDSD(1, 0, 2).
	p2 := runScalarFF(t, func(a *asm) {
		loadXmmImm(a, 0, 3.0)
		loadXmmImm(a, 1, 2.0)
		loadXmmImm(a, 2, 10.0)
		a.vaddSD(1, 0, 2)
		a.movXQ(10, 1)
		a.movqX(0, 10)
	}, "p2 vaddsd")
	if got := p2(0); got != 13.0 {
		t.Errorf("p2 vaddsd: %+v, wollen 13", got)
	}
	// vfmadd213SD(dst=1, src1=0, src2=2): X0=3, X1=2, X2=10 -> X1 = X1*X0+X2 = 16.
	fma := runScalarFF(t, func(a *asm) {
		loadXmmImm(a, 0, 3.0)
		loadXmmImm(a, 1, 2.0)
		loadXmmImm(a, 2, 10.0)
		a.vfmadd213SD(1, 0, 2)
		a.movXQ(10, 1)
		a.movqX(0, 10)
	}, "vfmadd213SD")
	if got := fma(0); got != 16.0 {
		t.Errorf("vfmadd213SD: %+v, wollen 16", got)
	}
	// vfnmadd231SD(dst=0, src1=2, src2=1): X0=3, X1=2, X2=10 -> X0 = -(X1*X2)+X0 = -17.
	fnma := runScalarFF(t, func(a *asm) {
		loadXmmImm(a, 0, 3.0)
		loadXmmImm(a, 1, 2.0)
		loadXmmImm(a, 2, 10.0)
		a.vfnmadd231SD(0, 2, 1)
	}, "vfnmadd231SD")
	if got := fnma(0); got != -17.0 {
		t.Errorf("vfnmadd231SD: %+v, wollen -17", got)
	}
	// cvtsd2si32 (RNE): 2.6 -> 3 (CVTT würde 2 liefern).
	rne := runScalarFF(t, func(a *asm) {
		a.cvtsd2si32(10, 0)
		a.cvtsi2sd32(0, 10)
	}, "cvtsd2si32 2.6")
	if got := rne(2.6); got != 3.0 {
		t.Errorf("cvtsd2si32(2.6): %+v, wollen 3 (RNE)", got)
	}
	if got := rne(-2.6); got != -3.0 {
		t.Errorf("cvtsd2si32(-2.6): %+v, wollen -3", got)
	}
	if got := rne(2.5); got != 2.0 {
		t.Errorf("cvtsd2si32(2.5): %+v, wollen 2 (RNE-even)", got)
	}
	// cvttsd2si32 (trunc): 2.6 -> 2, zur Kontrolle.
	trunc := runScalarFF(t, func(a *asm) {
		a.cvttsd2si32(10, 0)
		a.cvtsi2sd32(0, 10)
	}, "cvttsd2si32 2.6")
	if got := trunc(2.6); got != 2.0 {
		t.Errorf("cvttsd2si32(2.6): %+v, wollen 2", got)
	}
}
