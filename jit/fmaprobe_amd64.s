//go:build amd64

// FMA-Encodingsproben: der echte (plan9-)Assembler kodiert hier dieselben
// FMA-Varianten, die der JIT-Encodierer (asm_amd64.go) per Hand erzeugt.
// ABI0: Argumente liegen auf dem Stack, das Ergebnis wird dorthin
// zurückgeschrieben. (a=3, b=2, c=10 in den Tests.)
#include "textflag.h"

// func fma213A(a, b, c float64) float64: ret = a*b + c
TEXT ·fma213A(SB), NOSPLIT, $0
	MOVSD a+0(FP), X0
	MOVSD b+8(FP), X1
	MOVSD c+16(FP), X2
	VFMADD213SD X2, X1, X0 // X0 = X0*X1 + X2
	MOVSD X0, ret+24(FP)
	RET

// func fma231A(a, b, c float64) float64: ret = a + b*c
TEXT ·fma231A(SB), NOSPLIT, $0
	MOVSD a+0(FP), X0
	MOVSD b+8(FP), X1
	MOVSD c+16(FP), X2
	VFMADD231SD X2, X1, X0 // X0 = X0 + X1*X2
	MOVSD X0, ret+24(FP)
	RET

// func fnmadd231A(a, b, c float64) float64: ret = -(b*c) + a
TEXT ·fnmadd231A(SB), NOSPLIT, $0
	MOVSD a+0(FP), X0
	MOVSD b+8(FP), X1
	MOVSD c+16(FP), X2
	VFNMADD231SD X2, X1, X0 // X0 = -(X1*X2) + X0
	MOVSD X0, ret+24(FP)
	RET
