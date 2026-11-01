//go:build amd64

package jit

import "testing"

// TestFMAProbeAsm prüft die plan9-assemblierten FMA-Proben direkt auf der
// Hardware (a=3, b=2, c=10).
func TestFMAProbeAsm(t *testing.T) {
	cases := []struct {
		name string
		f    func(float64, float64, float64) float64
		want float64
	}{
		{"fma213 X0 = X0*X1 + X2", fma213A, 3*2 + 10},
		{"fma231 X0 = X0 + X1*X2", fma231A, 3 + 2*10},
		{"fnmadd231 X0 = -(X1*X2) + X0", fnmadd231A, -(2*10) + 3},
	}
	for _, c := range cases {
		if got := c.f(3, 2, 10); got != c.want {
			t.Errorf("%s: %+v, wollen %+v", c.name, got, c.want)
		} else {
			t.Logf("%s: %v ok", c.name, got)
		}
	}
}
