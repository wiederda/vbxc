package stdlib

import (
	"strings"

	"vbxc/value"
)

func ensureUTF8(b []byte) []byte { return []byte(strings.ToValidUTF8(string(b), "\uFFFD")) }

func valToInterface(v value.Value) any {
	switch v.Kind {
	case value.KindNum:
		return v.Num
	case value.KindStr:
		return v.Str
	case value.KindBool:
		return v.Bool
	case value.KindArr:
		a := make([]any, len(v.Arr))
		for i, x := range v.Arr {
			a[i] = valToInterface(x)
		}
		return a
	case value.KindArr2D:
		a := make([][]any, len(v.Arr2D))
		for i, row := range v.Arr2D {
			a[i] = make([]any, len(row))
			for j, x := range row {
				a[i][j] = valToInterface(x)
			}
		}
		return a
	case value.KindObj:
		if v.Obj == nil {
			return nil
		}
		m := make(map[string]any, len(v.Obj.Fields))
		for k, x := range v.Obj.Fields {
			m[k] = valToInterface(x)
		}
		return m
	case value.KindMap:
		m := make(map[string]any, len(v.Map))
		for k, x := range v.Map {
			m[k] = valToInterface(x)
		}
		return m
	default:
		return nil
	}
}
