package stdlib

// Math namespace ported from vbx.
import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"vbxc/value"
)

func init() {
	reg1 := func(name, desc string, fn func(float64) float64) {
		Register(name, "math", "n", desc, func(a []value.Value) value.Value {
			x, err := mathArg(a, 0, name)
			if err.Kind == value.KindError {
				return err
			}
			return value.NumVal(fn(x))
		})
	}
	reg2 := func(name, params, desc string, fn func(float64, float64) float64) {
		Register(name, "math", params, desc, func(a []value.Value) value.Value {
			x, err := mathArg(a, 0, name)
			if err.Kind == value.KindError {
				return err
			}
			y, err := mathArg(a, 1, name)
			if err.Kind == value.KindError {
				return err
			}
			return value.NumVal(fn(x, y))
		})
	}
	reg1("Abs", "Gibt den absoluten Wert zurück", math.Abs)
	reg1("Sin", "Berechnet den Sinus im Bogenmaß", math.Sin)
	reg1("Cos", "Berechnet den Cosinus im Bogenmaß", math.Cos)
	reg1("Tan", "Berechnet den Tangens im Bogenmaß", math.Tan)
	reg1("Asin", "Berechnet den Arkussinus", math.Asin)
	reg1("Acos", "Berechnet den Arkuskosinus", math.Acos)
	reg1("Atan", "Berechnet den Arkustangens", math.Atan)
	reg1("Ceil", "Rundet auf die nächste ganze Zahl auf", math.Ceil)
	reg1("Floor", "Rundet auf die nächste ganze Zahl ab", math.Floor)
	reg1("Trunc", "Schneidet die Nachkommastellen ab", math.Trunc)
	reg1("Exp", "Berechnet e hoch n", math.Exp)
	reg1("Log10", "Berechnet den Logarithmus zur Basis 10", math.Log10)
	reg1("Cbrt", "Berechnet die Kubikwurzel", math.Cbrt)
	reg2("Min", "a, b", "Gibt die kleinere Zahl zurück", math.Min)
	reg2("Max", "a, b", "Gibt die größere Zahl zurück", math.Max)
	reg2("Pow", "basis, exp", "Berechnet die Potenz", math.Pow)
	reg2("Atan2", "y, x", "Berechnet den Arkustangens aus zwei Koordinaten", math.Atan2)
	Register("Pi", "math", "", "Gibt die Konstante π zurück", func([]value.Value) value.Value { return value.NumVal(math.Pi) })
	Register("E", "math", "", "Gibt die Eulersche Zahl zurück", func([]value.Value) value.Value { return value.NumVal(math.E) })
	Register("Int", "math", "n", "Schneidet die Nachkommastellen ab", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "Int")
		if e.Kind == value.KindError {
			return e
		}
		return value.NumVal(math.Trunc(x))
	})
	Register("Val", "math", "s", "Wandelt Text in eine Zahl um; bei Fehler 0", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.NumVal(0)
		}
		s := strings.TrimSpace(value.ToString(a[0]))
		s = strings.ReplaceAll(s, ",", ".")
		x, e := strconv.ParseFloat(s, 64)
		if e != nil {
			return value.NumVal(0)
		}
		return value.NumVal(x)
	})
	Register("Mod", "math", "a, b", "Berechnet den Rest der Ganzzahldivision", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "Mod")
		if e.Kind == value.KindError {
			return e
		}
		y, e := mathArg(a, 1, "Mod")
		if e.Kind == value.KindError {
			return e
		}
		if int(y) == 0 {
			return value.ErrorVal("Modulo durch 0")
		}
		return value.NumVal(float64(int(x) % int(y)))
	})
	Register("LogBase", "math", "n, base", "Berechnet den Logarithmus zu einer beliebigen Basis", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "LogBase")
		if e.Kind == value.KindError {
			return e
		}
		base, e := mathArg(a, 1, "LogBase")
		if e.Kind == value.KindError {
			return e
		}
		if x <= 0 || base <= 0 || base == 1 {
			return value.ErrorVal("LogBase: Ungültige Eingabewerte")
		}
		return value.NumVal(math.Log(x) / math.Log(base))
	})
	Register("Sqrt", "math", "n", "Berechnet die Quadratwurzel", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "Sqrt")
		if e.Kind == value.KindError {
			return e
		}
		if x < 0 {
			return value.ErrorVal("Sqrt: Wert darf nicht negativ sein")
		}
		return value.NumVal(math.Sqrt(x))
	})
	Register("Root", "math", "v, n", "Berechnet die n-te Wurzel von v", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "Root")
		if e.Kind == value.KindError {
			return e
		}
		n, e := mathArg(a, 1, "Root")
		if e.Kind == value.KindError {
			return e
		}
		if n == 0 {
			return value.ErrorVal("Root: Die 0-te Wurzel ist nicht definiert")
		}
		return value.NumVal(math.Pow(x, 1/n))
	})
	Register("Sign", "math", "n", "Gibt -1, 0 oder 1 zurück", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "Sign")
		if e.Kind == value.KindError {
			return e
		}
		if x < 0 {
			return value.NumVal(-1)
		}
		if x > 0 {
			return value.NumVal(1)
		}
		return value.NumVal(0)
	})
	for _, item := range []struct {
		name string
		fn   func(float64) float64
	}{{"Round", math.Round}, {"RoundBank", math.RoundToEven}} {
		s := item
		Register(s.name, "math", "v, [d]", "Rundet auf die angegebene Anzahl Nachkommastellen", func(a []value.Value) value.Value {
			x, e := mathArg(a, 0, s.name)
			if e.Kind == value.KindError {
				return e
			}
			d := 0.0
			if len(a) > 1 {
				d, e = mathArg(a, 1, s.name)
				if e.Kind == value.KindError {
					return e
				}
			}
			pow := math.Pow(10, math.Trunc(d))
			return value.NumVal(s.fn(x*pow) / pow)
		})
	}
	Register("Clamp", "math", "v, min, max", "Hält einen Wert innerhalb der Grenzen", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "Clamp")
		if e.Kind == value.KindError {
			return e
		}
		lo, e := mathArg(a, 1, "Clamp")
		if e.Kind == value.KindError {
			return e
		}
		hi, e := mathArg(a, 2, "Clamp")
		if e.Kind == value.KindError {
			return e
		}
		return value.NumVal(math.Max(lo, math.Min(x, hi)))
	})
	Register("Log", "math", "n", "Berechnet den natürlichen Logarithmus", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "Log")
		if e.Kind == value.KindError {
			return e
		}
		if x <= 0 {
			return value.ErrorVal("Log nur für Werte > 0")
		}
		return value.NumVal(math.Log(x))
	})
	Register("DegToRad", "math", "deg", "Rechnet Grad in Bogenmaß um", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "DegToRad")
		if e.Kind == value.KindError {
			return e
		}
		return value.NumVal(x * math.Pi / 180)
	})
	Register("RadToDeg", "math", "rad", "Rechnet Bogenmaß in Grad um", func(a []value.Value) value.Value {
		x, e := mathArg(a, 0, "RadToDeg")
		if e.Kind == value.KindError {
			return e
		}
		return value.NumVal(x * 180 / math.Pi)
	})
	Register("Percent", "math", "p, v", "Berechnet p Prozent von v", func(a []value.Value) value.Value {
		p, e := mathArg(a, 0, "Percent")
		if e.Kind == value.KindError {
			return e
		}
		x, e := mathArg(a, 1, "Percent")
		if e.Kind == value.KindError {
			return e
		}
		return value.NumVal(p / 100 * x)
	})
	Register("PercentOf", "math", "part, total", "Berechnet den Prozentanteil", func(a []value.Value) value.Value {
		p, e := mathArg(a, 0, "PercentOf")
		if e.Kind == value.KindError {
			return e
		}
		t, e := mathArg(a, 1, "PercentOf")
		if e.Kind == value.KindError {
			return e
		}
		if t == 0 {
			return value.ErrorVal("Division durch 0 in PercentOf")
		}
		return value.NumVal(p / t * 100)
	})
}

func mathArg(args []value.Value, index int, name string) (float64, value.Value) {
	if index < 0 || index >= len(args) {
		return 0, value.ErrorVal(fmt.Sprintf("%s: Fehlendes Argument an Position %d", name, index+1))
	}
	v := args[index]
	if v.Kind == value.KindNum {
		return v.Num, value.Value{}
	}
	if v.Kind == value.KindStr {
		s := strings.ReplaceAll(strings.TrimSpace(v.Str), ",", ".")
		if n, e := strconv.ParseFloat(s, 64); e == nil {
			return n, value.Value{}
		}
	}
	return 0, value.ErrorVal(fmt.Sprintf("%s: Argument %d ist keine gültige Zahl", name, index+1))
}
