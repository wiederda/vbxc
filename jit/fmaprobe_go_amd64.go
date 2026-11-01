//go:build amd64

package jit

// Plan9-Assembler-FMA-Proben (fmaprobe_amd64.s). Die Signaturen sind an die
// Register-ABI angepasst: X0/X1/X2 respectively.
func fma213A(a, b, c float64) float64
func fma231A(a, b, c float64) float64
func fnmadd231A(a, b, c float64) float64
