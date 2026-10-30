package value

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"vbxc/token"
)

// ===========================================================================
// ORIGINAL aus dem Interpreter (1:1 übernommen, nur umbenannt/exportiert)
// ===========================================================================

func tryParseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// CalculateBinaryOp ist calculateBinaryOp aus dem Evaluator.
func CalculateBinaryOp(op token.Type, l Value, r Value) Value {
	if l.Kind == KindNum && r.Kind == KindNum {
		switch op {
		case token.PLUS:
			return NumVal(l.Num + r.Num)
		case token.MINUS:
			return NumVal(l.Num - r.Num)
		case token.MUL:
			return NumVal(l.Num * r.Num)
		case token.DIV:
			if r.Num != 0 {
				return NumVal(l.Num / r.Num)
			}
			// bei 0 weiter unten: gleiche Fehlermeldung wie sonst
		}
	}

	switch op {

	case token.PLUS:
		if l.Kind == KindStr && r.Kind == KindStr {
			return StrVal(l.Str + r.Str)
		}
		if ln, ok := TryNumber(l); ok {
			if rn, ok := TryNumber(r); ok {
				return NumVal(ln + rn)
			}
		}
		return StrVal(ToString(l) + ToString(r))

	case token.MINUS:
		ln, lok := TryNumber(l)
		rn, rok := TryNumber(r)
		if !lok || !rok {
			return ErrorVal("Operanden müssen numerisch sein")
		}
		return NumVal(ln - rn)

	case token.MUL:
		ln, lok := TryNumber(l)
		rn, rok := TryNumber(r)
		if !lok || !rok {
			return ErrorVal("Operanden müssen numerisch sein")
		}
		return NumVal(ln * rn)

	case token.DIV:
		ln, lok := TryNumber(l)
		rn, rok := TryNumber(r)
		if !lok || !rok {
			return ErrorVal("Operanden müssen numerisch sein")
		}
		if rn == 0 {
			return ErrorVal("Division durch 0")
		}
		return NumVal(ln / rn)
	}

	return ErrorVal("Nicht unterstützter Operator")
}

// ValuesAreEqual ist valuesAreEqual aus dem Evaluator.
func ValuesAreEqual(l, r Value) bool {
	// 1. Wenn Typen gleich sind: Direkter Vergleich
	if l.Kind == r.Kind {
		switch l.Kind {
		case KindNum:
			return l.Num == r.Num
		case KindStr:
			return strings.EqualFold(l.Str, r.Str) // Case-Insensitive wie in VB!
		case KindBool:
			return l.Bool == r.Bool
		case KindNull, KindNil:
			return true
		}
	}

	// 2. Cross-Type Vergleich: Zahl vs String
	if (l.Kind == KindNum && r.Kind == KindStr) || (l.Kind == KindStr && r.Kind == KindNum) {
		return ToNum(l) == ToNum(r)
	}

	// 3. Boolean Checks (VB: -1 ist True, 0 ist False)
	if l.Kind == KindBool || r.Kind == KindBool {
		return IsTruthy(l) == IsTruthy(r)
	}

	return false
}

// ===========================================================================
// ORIGINAL aus helpers.go des Interpreters (1:1, nur exportiert wo nötig):
//
//	ToFloat, ToNum (toNumVal), IsTruthy (isTruthy), ToString, ToBool, ToInt,
//	TryNumber (tryNumber), CompareNum (compareNum), ToIndex (toIndex),
//	ToFloatSafe (toFloatSafe), RequireNumber (requireNumber), FormatNum
// ===========================================================================

// formatNum: Ganzzahlen ohne Exponent, sonst kürzeste exakte Darstellung.
func formatNum(n float64) string {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return strconv.FormatFloat(n, 'g', -1, 64)
	}
	abs := math.Abs(n)
	if n == math.Trunc(n) && abs < 1e15 {
		return strconv.FormatInt(int64(n), 10)
	}
	if abs >= 1e-6 && abs < 1e21 {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return strconv.FormatFloat(n, 'g', -1, 64) // sehr klein/groß
}

// ToInt konvertiert Value zu int
func ToInt(v Value) (int, bool) {
	switch v.Kind {
	case KindNum:
		return int(v.Num), true

	case KindStr:
		s := strings.TrimSpace(v.Str)
		if s == "" {
			return 0, false
		}
		i, err := strconv.Atoi(s)
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

// ToFloat konvertiert Value zu float64
func ToFloat(v Value) (float64, bool) {

	switch v.Kind {

	case KindNum:
		return v.Num, true

	case KindStr:
		s := strings.TrimSpace(v.Str)
		if s == "" {
			return 0, false
		}

		// deutsches Dezimal-Komma erlauben
		if strings.Count(s, ",") == 1 && !strings.Contains(s, ".") {
			s = strings.Replace(s, ",", ".", 1)
		}

		f, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return f, true
		}
	}

	return 0, false
}

// ToNum ist toNumVal: wandelt jeden Value in eine float64 Zahl um
func ToNum(v Value) float64 {
	if f, ok := ToFloat(v); ok {
		return f
	}
	return 0
}

// IsTruthy ist isTruthy: prüft, ob ein Value "wahr" ist (VB-Style)
func IsTruthy(v Value) bool {
	switch v.Kind {
	case KindBool:
		return v.Bool
	case KindNum:
		return v.Num != 0
	case KindStr:
		return v.Str != ""
	case KindArr:
		return len(v.Arr) != 0 || len(v.Arr2D) != 0
	case KindObj:
		if v.Obj == nil {
			return false
		}
		return len(v.Obj.Fields) != 0
	case KindNull:
		return false
	default:
		return false
	}
}

// ToString konvertiert Value zu string
func ToString(v Value) string {
	switch v.Kind {

	case KindStr:
		return v.Str

	case KindNum:
		return formatNum(v.Num)

	case KindBool:
		if v.Bool {
			return "true"
		}
		return "false"

	case KindNull:
		return ""

	case KindArr:
		parts := make([]string, len(v.Arr))
		for i, e := range v.Arr {
			parts[i] = ToString(e)
		}
		return strings.Join(parts, ",")

	case KindArr2D:
		rows := make([]string, len(v.Arr2D))
		for i, row := range v.Arr2D {
			cols := make([]string, len(row))
			for j, cell := range row {
				cols[j] = ToString(cell)
			}
			rows[i] = strings.Join(cols, ",")
		}
		return strings.Join(rows, "\n")

	case KindObj:
		if v.Obj != nil {
			return fmt.Sprintf("%v", v.Obj)
		}
		return ""

	case KindError:
		return "Fehler: " + v.Str

	default:
		return ""
	}
}

// ToBool konvertiert Value zu bool (0/false -> false, alles andere -> true)
func ToBool(v Value) bool {
	switch v.Kind {
	case KindBool:
		return v.Bool

	case KindNum:
		return v.Num != 0

	case KindStr:
		s := strings.ToLower(strings.TrimSpace(v.Str))
		// Explizite Prüfung auf positive Werte
		return s == "true" || s == "1" || s == "yes" || s == "on" || s == "wahr"

	case KindNull:
		return false

	case KindArr:
		return len(v.Arr) > 0

	case KindArr2D:
		return len(v.Arr2D) > 0

	case KindObj:
		return v.Obj != nil

	default:
		return false
	}
}

// TryNumber ist tryNumber: Zahl oder als Zahl lesbarer String, ohne
// Fehlerwert/Allokation. "Inf"/"NaN" gelten NICHT als Zahl.
func TryNumber(v Value) (float64, bool) {
	switch v.Kind {
	case KindNum:
		return v.Num, true
	case KindStr:
		s := strings.TrimSpace(v.Str)
		if s == "" {
			return 0, false
		}
		s = strings.ReplaceAll(s, ",", ".")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// CompareNum ist compareNum: numerischer Vergleich für LT, GT, LE, GE.
func CompareNum(op token.Type, a, b float64) bool {
	switch op {
	case token.LT:
		return a < b
	case token.GT:
		return a > b
	case token.LE:
		return a <= b
	default: // GE
		return a >= b
	}
}

// ToIndex ist toIndex: wandelt einen Index-Ausdruck in int um und lehnt
// Fehlerwerte, NaN/Inf und absurd große Werte ab.
func ToIndex(v Value) (int, Value) {
	if v.Kind == KindError {
		return 0, v
	}
	f := ToNum(v)
	if math.IsNaN(f) || math.IsInf(f, 0) || f > math.MaxInt32 || f < math.MinInt32 {
		return 0, ErrorVal("Ungültiger Index: " + ToString(v))
	}
	return int(f), Value{}
}

// ToFloatSafe ist toFloatSafe: float64-Wert eines Arguments oder 0.
func ToFloatSafe(args []Value, index int) float64 {
	if index < 0 || index >= len(args) {
		return 0
	}
	v := args[index]

	switch v.Kind {
	case KindNum:
		return v.Num
	case KindStr:
		s := strings.TrimSpace(v.Str)
		s = strings.Replace(s, ",", ".", 1) // Komma zu Punkt
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	return 0
}

// RequireNumber ist requireNumber.
func RequireNumber(v Value, context string) (float64, Value) {
	switch v.Kind {
	case KindNum:
		return v.Num, Value{}
	case KindStr:
		s := strings.TrimSpace(v.Str)
		s = strings.ReplaceAll(s, ",", ".")
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, Value{}
		}
	}

	msg := fmt.Sprintf("Fehler bei '%s': '%s' ist keine gültige Zahl.", context, GetKindName(v.Kind))
	return 0, ErrorVal(msg)
}

func (k Kind) String() string {
	switch k {
	case KindNum:
		return "Number"
	case KindStr:
		return "String"
	case KindBool:
		return "Boolean"
	case KindArr:
		return "1D Array"
	case KindArr2D:
		return "2D Array"
	case KindObj:
		return "Object"
	case KindNull:
		return "Null"
	case KindError:
		return "Error"
	default:
		return "Invalid"
	}
}

// PLATZHALTER: GetKindName steht nicht in helpers.go, sondern woanders im
// Interpreter. Bis das Original da ist, liefert es Kind.String() - nur die
// Meldung von RequireNumber hängt daran.
func GetKindName(k Kind) string { return k.String() }
