package value

import (
	"fmt"

	"vbxc/token"
)

// Die Funktionen hier bilden die Zweige von "case *BinOpNode" / "*UnaryOpNode"
// in evalExpr nach. Fehler-Operanden werden wie dort durchgereicht
// (links vor rechts). Unäre Operatoren prüfen, wie im Original, nicht auf Fehler.

func firstError(l, r Value) (Value, bool) {
	if l.Kind == KindError {
		return l, true
	}
	if r.Kind == KindError {
		return r, true
	}
	return Value{}, false
}

// Add ist der PLUS-Zweig: Strings verketten, Zahlen addieren, sonst als Text.
func Add(l, r Value) Value {
	if e, ok := firstError(l, r); ok {
		return e
	}
	if l.Kind == KindStr && r.Kind == KindStr {
		return StrVal(l.Str + r.Str)
	}
	if ln, ok := TryNumber(l); ok {
		if rn, ok := TryNumber(r); ok {
			return NumVal(ln + rn)
		}
	}
	return StrVal(ToString(l) + ToString(r))
}

// Arith ist der MINUS/MUL/DIV-Zweig.
func Arith(op token.Type, l, r Value) Value {
	if e, ok := firstError(l, r); ok {
		return e
	}
	if l.Kind == KindNum && r.Kind == KindNum {
		return CalculateBinaryOp(op, l, r)
	}
	ln, err := RequireNumber(l, op.String())
	if err.Kind == KindError {
		return err
	}
	rn, err := RequireNumber(r, op.String())
	if err.Kind == KindError {
		return err
	}
	return CalculateBinaryOp(op, NumVal(ln), NumVal(rn))
}

// Concat ist der AMP-Zweig.
func Concat(l, r Value) Value {
	if e, ok := firstError(l, r); ok {
		return e
	}
	return StrVal(ToString(l) + ToString(r))
}

// Equal ist der EQ/NEQ-Zweig (negate = NEQ).
func Equal(l, r Value, negate bool) Value {
	if e, ok := firstError(l, r); ok {
		return e
	}
	var eq bool
	switch {
	case l.Kind == KindBool && r.Kind == KindBool:
		eq = l.Bool == r.Bool
	case l.Kind == KindStr || r.Kind == KindStr:
		eq = ToString(l) == ToString(r)
	case l.Kind == KindNum && r.Kind == KindNum:
		eq = l.Num == r.Num
	default: // gemischte Typen (z.B. Bool vs Num)
		eq = ValuesAreEqual(l, r)
	}
	return BoolVal(eq == !negate)
}

// Compare ist der LT/GT/LE/GE-Zweig.
func Compare(op token.Type, l, r Value) Value {
	if e, ok := firstError(l, r); ok {
		return e
	}
	if l.Kind == KindNum && r.Kind == KindNum {
		return BoolVal(CompareNum(op, l.Num, r.Num))
	}
	ln, err := RequireNumber(l, op.String())
	if err.Kind == KindError {
		return err
	}
	rn, err := RequireNumber(r, op.String())
	if err.Kind == KindError {
		return err
	}
	return BoolVal(CompareNum(op, ln, rn))
}

// Not und Neg: keine Fehlerprüfung, wie im Original.
func Not(v Value) Value { return BoolVal(!IsTruthy(v)) }

func Neg(v Value) Value {
	if v.Kind == KindNum {
		return NumVal(-v.Num)
	}
	return ErrorVal("Minus erfordert eine Zahl")
}

// TryParseFloat ist tryParseFloat aus dem Evaluator (Trim + ParseFloat).
func TryParseFloat(s string) (float64, bool) { return tryParseFloat(s) }

// EvalBinary ist evalBinary aus dem Evaluator (Vergleich bei "Case Is ...").
func EvalBinary(left Value, op token.Type, right Value) Value {
	lNum := ToFloatSafe([]Value{left}, 0)
	rNum := ToFloatSafe([]Value{right}, 0)

	switch op {
	case token.GT:
		return BoolVal(lNum > rNum)
	case token.LT:
		return BoolVal(lNum < rNum)
	case token.GE:
		return BoolVal(lNum >= rNum)
	case token.LE:
		return BoolVal(lNum <= rNum)
	case token.EQ:
		return BoolVal(ValuesAreEqual(left, right))
	case token.NEQ:
		return BoolVal(!ValuesAreEqual(left, right))
	case token.AND:
		return BoolVal(IsTruthy(left) && IsTruthy(right))
	case token.OR:
		return BoolVal(IsTruthy(left) || IsTruthy(right))
	}

	return ErrorVal(fmt.Sprintf("Operator %v wird im Select Case noch nicht unterstützt", op))
}
