package bytecode

import (
	"strings"

	"vbxc/value"
)

// SelectTable ist der Dispatch-Cache des Interpreters (dispatchNumCache /
// dispatchStrCache), nur dass er hier zur Compile-Zeit gebaut wird. Er
// existiert nur, wenn ALLE Case-Bedingungen reine Zahl-/String-Literale sind.
type SelectTable struct {
	Num     map[float64]int // Zahl-Literal -> Index des Case-Zweigs
	Str     map[string]int  // String-Literal (kleingeschrieben) -> Zweig
	Targets []int32         // Zweig -> Adresse des Körpers
	Default int32           // Adresse des Else-Zweigs (oder Ende)
}

// Lookup entspricht dem Fast-Path in evalSingleStatement (Zahl/String,
// einschließlich Cross-Type-Fallback). Bool-Werte nimmt der Aufrufer vorher aus.
func (t *SelectTable) Lookup(v value.Value) (int, bool) {
	switch v.Kind {
	case value.KindNum:
		if idx, ok := t.Num[v.Num]; ok {
			return idx, true
		}
		if len(t.Str) > 0 {
			idx, ok := t.Str[strings.ToLower(value.ToString(v))]
			return idx, ok
		}
	case value.KindStr:
		if idx, ok := t.Str[strings.ToLower(v.Str)]; ok {
			return idx, true
		}
		if len(t.Num) > 0 {
			if key, ok := value.TryParseFloat(v.Str); ok {
				idx, ok := t.Num[key]
				return idx, ok
			}
		}
	}
	return 0, false
}
