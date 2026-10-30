// Package value enthält die Laufzeit-Werte, 1:1 aus dem Interpreter.
package value

type Kind int

const (
	KindInvalid Kind = iota
	//KindUnknown
	KindUndefined
	KindNum
	KindStr
	KindArr
	KindArr2D
	KindObj
	KindNull
	KindBool
	KindError
	KindMap
	KindNil
	KindNone
	KindBytes
)

type Object struct {
	TypeName string
	Fields   map[string]Value
}

type Value struct {
	Kind  Kind
	Num   float64
	Str   string
	Bool  bool
	Arr   []Value
	Arr2D [][]Value
	Obj   *Object
	Map   map[string]Value
	Bytes []byte
}

var NilValue = Value{Kind: KindNil}

func NumVal(f float64) Value  { return Value{Kind: KindNum, Num: f} }
func StrVal(s string) Value   { return Value{Kind: KindStr, Str: s} }
func BoolVal(b bool) Value    { return Value{Kind: KindBool, Bool: b} }
func NullVal() Value          { return Value{Kind: KindNull} }
func ErrorVal(m string) Value { return Value{Kind: KindError, Str: m} }
