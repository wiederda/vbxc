package stdlib

// Array namespace ported from vbx. Operations return updated arrays because
// the bytecode VM passes Value copies to builtins.
import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"vbxc/value"
)

func init() {
	reg := func(name, params, desc string, fn func([]value.Value) value.Value) {
		Register(name, "array", params, desc, fn)
	}
	arrayArg := func(a []value.Value, n int, name string) ([]value.Value, value.Value, bool) {
		if len(a) <= n || (a[n].Kind != value.KindArr && a[n].Kind != value.KindArr2D) {
			return nil, value.ErrorVal(name + ": Argument muss ein Array sein"), false
		}
		if a[n].Kind == value.KindArr2D {
			return nil, value.ErrorVal(name + ": erwartet ein 1D-Array"), false
		}
		return a[n].Arr, value.Value{}, true
	}
	reg("Create", "[args...]", "Erstellt ein 1D- oder 2D-Array", func(a []value.Value) value.Value {
		if len(a) == 2 && a[0].Kind == value.KindNum && a[1].Kind == value.KindNum {
			rows, cols := safeSize(a[0].Num), safeSize(a[1].Num)
			if rows != 0 && cols > 10_000_000/rows {
				return value.ErrorVal("array.Create: Array-Größe ist zu groß")
			}
			m := make([][]value.Value, rows)
			for i := range m {
				m[i] = make([]value.Value, cols)
				for j := range m[i] {
					m[i][j] = value.NumVal(0)
				}
			}
			return value.Value{Kind: value.KindArr2D, Arr2D: m}
		}
		if len(a) == 1 && a[0].Kind == value.KindNum {
			n := safeSize(a[0].Num)
			out := make([]value.Value, n)
			for i := range out {
				out[i] = value.NumVal(0)
			}
			return value.Value{Kind: value.KindArr, Arr: out}
		}
		if len(a) > 0 && a[0].Kind == value.KindArr {
			rows := make([][]value.Value, len(a))
			for i, v := range a {
				if v.Kind == value.KindArr {
					rows[i] = append([]value.Value(nil), v.Arr...)
				} else {
					rows[i] = []value.Value{v}
				}
			}
			return value.Value{Kind: value.KindArr2D, Arr2D: rows}
		}
		return value.Value{Kind: value.KindArr, Arr: append([]value.Value(nil), a...)}
	})
	count := func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.NumVal(0)
		}
		dim := 1
		if len(a) > 1 {
			dim = int(value.ToNum(a[1]))
		}
		switch a[0].Kind {
		case value.KindArr:
			if dim == 1 {
				return value.NumVal(float64(len(a[0].Arr)))
			}
		case value.KindArr2D:
			if dim == 1 {
				return value.NumVal(float64(len(a[0].Arr2D)))
			}
			if dim == 2 && len(a[0].Arr2D) > 0 {
				return value.NumVal(float64(len(a[0].Arr2D[0])))
			}
		}
		return value.NumVal(0)
	}
	reg("Count", "array, [dim]", "Gibt die Anzahl der Elemente einer Dimension zurück", func(a []value.Value) value.Value { return count(a) })
	reg("Length", "array, [dim]", "Alias für Count", func(a []value.Value) value.Value { return count(a) })
	for _, bound := range []string{"LBound", "UBound"} {
		b := bound
		reg(b, "array, [dim]", "Gibt die Array-Grenze zurück", func(a []value.Value) value.Value {
			if len(a) == 0 {
				return value.NumVal(-1)
			}
			dim := 1
			if len(a) > 1 {
				dim = int(value.ToNum(a[1]))
			}
			n := -1
			switch a[0].Kind {
			case value.KindArr:
				if dim == 1 {
					n = len(a[0].Arr)
				}
			case value.KindArr2D:
				if dim == 1 {
					n = len(a[0].Arr2D)
				} else if dim == 2 && len(a[0].Arr2D) > 0 {
					n = len(a[0].Arr2D[0])
				}
			}
			if n == 0 {
				return value.NumVal(-1)
			}
			if b == "LBound" {
				if n > 0 {
					return value.NumVal(0)
				}
				return value.NumVal(-1)
			}
			return value.NumVal(float64(n - 1))
		})
	}
	reg("First", "array", "Gibt das erste Element zurück", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.First")
		if !ok {
			return e
		}
		if len(v) == 0 {
			return value.NilValue
		}
		return v[0]
	})
	reg("Last", "array", "Gibt das letzte Element zurück", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Last")
		if !ok {
			return e
		}
		if len(v) == 0 {
			return value.NilValue
		}
		return v[len(v)-1]
	})
	reg("Add", "arr, value", "Fügt ein Element am Ende hinzu; Ergebnis zuweisen", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Add")
		if !ok {
			return e
		}
		if len(a) < 2 {
			return value.ErrorVal("array.Add: Wert fehlt")
		}
		out := append([]value.Value(nil), v...)
		out = append(out, a[1])
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("Append", "arr, value", "Alias für Add", func(a []value.Value) value.Value { return call("array.Add", a) })
	reg("Push", "arr, value", "Alias für Add", func(a []value.Value) value.Value { return call("array.Add", a) })
	reg("Prepend", "arr, value", "Fügt ein Element am Anfang hinzu", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Prepend")
		if !ok {
			return e
		}
		if len(a) < 2 {
			return value.ErrorVal("array.Prepend: Wert fehlt")
		}
		out := make([]value.Value, 0, len(v)+1)
		out = append(out, a[1])
		out = append(out, v...)
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("Insert", "arr, index, value", "Fügt ein Element an der 0-basierten Position ein", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Insert")
		if !ok {
			return e
		}
		if len(a) < 3 {
			return value.ErrorVal("array.Insert: Argumente fehlen")
		}
		i := int(value.ToNum(a[1]))
		if i < 0 {
			i = 0
		}
		if i > len(v) {
			i = len(v)
		}
		out := make([]value.Value, 0, len(v)+1)
		out = append(out, v[:i]...)
		out = append(out, a[2])
		out = append(out, v[i:]...)
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("RemoveAt", "arr, index", "Entfernt Element an 0-basierter Position", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.RemoveAt")
		if !ok {
			return e
		}
		if len(a) < 2 {
			return value.ErrorVal("array.RemoveAt: Index fehlt")
		}
		i := int(value.ToNum(a[1]))
		if i < 0 || i >= len(v) {
			return value.Value{Kind: value.KindArr, Arr: append([]value.Value(nil), v...)}
		}
		out := append([]value.Value(nil), v[:i]...)
		out = append(out, v[i+1:]...)
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("Remove", "arr, value", "Entfernt das erste passende Element", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Remove")
		if !ok {
			return e
		}
		if len(a) < 2 {
			return value.ErrorVal("array.Remove: Wert fehlt")
		}
		for i, x := range v {
			if value.ToString(x) == value.ToString(a[1]) {
				out := append([]value.Value(nil), v[:i]...)
				out = append(out, v[i+1:]...)
				return value.Value{Kind: value.KindArr, Arr: out}
			}
		}
		return value.Value{Kind: value.KindArr, Arr: append([]value.Value(nil), v...)}
	})
	reg("Clear", "arr", "Gibt ein leeres Array zurück", func(a []value.Value) value.Value {
		if _, e, ok := arrayArg(a, 0, "array.Clear"); !ok {
			return e
		}
		return value.Value{Kind: value.KindArr, Arr: []value.Value{}}
	})
	reg("Clone", "array", "Erstellt eine flache Kopie", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("array.Clone: Argument fehlt")
		}
		if a[0].Kind == value.KindArr {
			return value.Value{Kind: value.KindArr, Arr: append([]value.Value(nil), a[0].Arr...)}
		}
		if a[0].Kind == value.KindArr2D {
			m := make([][]value.Value, len(a[0].Arr2D))
			for i := range m {
				m[i] = append([]value.Value(nil), a[0].Arr2D[i]...)
			}
			return value.Value{Kind: value.KindArr2D, Arr2D: m}
		}
		return value.ErrorVal("array.Clone: Argument muss ein Array sein")
	})
	reg("Reverse", "array", "Gibt ein umgekehrtes Array zurück", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Reverse")
		if !ok {
			return e
		}
		out := append([]value.Value(nil), v...)
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("Join", "arr, sep", "Verbindet Array-Elemente zu Text", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Join")
		if !ok {
			return e
		}
		sep := ""
		if len(a) > 1 {
			sep = value.ToString(a[1])
		}
		p := make([]string, len(v))
		for i, x := range v {
			p[i] = value.ToString(x)
		}
		return value.StrVal(strings.Join(p, sep))
	})
	reg("Contains", "arr, value", "Prüft, ob ein Wert enthalten ist", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Contains")
		if !ok {
			return e
		}
		if len(a) < 2 {
			return value.BoolVal(false)
		}
		for _, x := range v {
			if value.ToString(x) == value.ToString(a[1]) {
				return value.BoolVal(true)
			}
		}
		return value.BoolVal(false)
	})
	reg("IndexOf", "arr, value", "Gibt die 0-basierte Fundposition oder -1 zurück", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.IndexOf")
		if !ok {
			return e
		}
		if len(a) > 1 {
			for i, x := range v {
				if value.ToString(x) == value.ToString(a[1]) {
					return value.NumVal(float64(i))
				}
			}
		}
		return value.NumVal(-1)
	})
	reg("Unique", "array", "Entfernt doppelte Werte", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Unique")
		if !ok {
			return e
		}
		out := []value.Value{}
		for _, x := range v {
			found := false
			for _, y := range out {
				if value.ValuesAreEqual(x, y) {
					found = true
					break
				}
			}
			if !found {
				out = append(out, x)
			}
		}
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("Merge", "array, array", "Verbindet zwei Arrays", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Merge")
		if !ok {
			return e
		}
		w, e, ok := arrayArg(a, 1, "array.Merge")
		if !ok {
			return e
		}
		out := append([]value.Value(nil), v...)
		out = append(out, w...)
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("IsEmpty", "array", "Prüft, ob das Array leer ist", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.BoolVal(true)
		}
		return value.BoolVal((a[0].Kind == value.KindArr && len(a[0].Arr) == 0) || (a[0].Kind == value.KindArr2D && len(a[0].Arr2D) == 0))
	})
	reg("Sort", "array", "Sortiert Zahlen oder Texte aufsteigend", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Sort")
		if !ok {
			return e
		}
		out := append([]value.Value(nil), v...)
		numeric := true
		for _, x := range out {
			if x.Kind != value.KindNum {
				numeric = false
				break
			}
		}
		sort.SliceStable(out, func(i, j int) bool {
			if numeric {
				return out[i].Num < out[j].Num
			}
			return strings.ToLower(value.ToString(out[i])) < strings.ToLower(value.ToString(out[j]))
		})
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("NaturalSort", "array", "Sortiert Texte nach natürlicher Reihenfolge", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.NaturalSort")
		if !ok {
			return e
		}
		out := append([]value.Value(nil), v...)
		parts := regexp.MustCompile(`\d+|\D+`)
		less := func(x, y string) bool {
			xs, ys := parts.FindAllString(x, -1), parts.FindAllString(y, -1)
			for i := 0; i < len(xs) && i < len(ys); i++ {
				xn, xe := strconv.ParseUint(xs[i], 10, 64)
				yn, ye := strconv.ParseUint(ys[i], 10, 64)
				if xe == nil && ye == nil {
					if xn != yn {
						return xn < yn
					}
					continue
				}
				xp, yp := strings.ToLower(xs[i]), strings.ToLower(ys[i])
				if xp != yp {
					return xp < yp
				}
			}
			return len(xs) < len(ys)
		}
		sort.SliceStable(out, func(i, j int) bool { return less(value.ToString(out[i]), value.ToString(out[j])) })
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("SortBy", "arr, pattern, [desc]", "Sortiert nach Regex-Capture-Gruppe", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.SortBy")
		if !ok {
			return e
		}
		if len(a) < 2 {
			return value.ErrorVal("array.SortBy: Regex fehlt")
		}
		re, err := regexp.Compile(value.ToString(a[1]))
		if err != nil {
			return value.ErrorVal(err.Error())
		}
		desc := len(a) > 2 && value.IsTruthy(a[2])
		out := append([]value.Value(nil), v...)
		sort.SliceStable(out, func(i, j int) bool {
			mi, mj := re.FindStringSubmatch(value.ToString(out[i])), re.FindStringSubmatch(value.ToString(out[j]))
			if len(mi) < 2 {
				return false
			}
			if len(mj) < 2 {
				return true
			}
			if desc {
				return mi[1] > mj[1]
			}
			return mi[1] < mj[1]
		})
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("CleanArray", "array", "Trimmt Textelemente und entfernt Steuerzeichen", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("array.CleanArray: Argument fehlt")
		}
		clean := func(x value.Value) value.Value {
			if x.Kind != value.KindStr {
				return x
			}
			s := strings.TrimSpace(x.Str)
			s = strings.Map(func(r rune) rune {
				if unicode.IsControl(r) && r != '\t' {
					return -1
				}
				return r
			}, s)
			return value.StrVal(s)
		}
		if a[0].Kind == value.KindArr {
			out := make([]value.Value, len(a[0].Arr))
			for i, x := range a[0].Arr {
				out[i] = clean(x)
			}
			return value.Value{Kind: value.KindArr, Arr: out}
		}
		if a[0].Kind == value.KindArr2D {
			m := make([][]value.Value, len(a[0].Arr2D))
			for i, row := range a[0].Arr2D {
				m[i] = make([]value.Value, len(row))
				for j, x := range row {
					m[i][j] = clean(x)
				}
			}
			return value.Value{Kind: value.KindArr2D, Arr2D: m}
		}
		return value.ErrorVal("array.CleanArray: Argument muss ein Array sein")
	})
	reg("Chunk", "arr, size", "Teilt Array in Teilarrays", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Chunk")
		if !ok {
			return e
		}
		if len(a) < 2 {
			return value.ErrorVal("array.Chunk: Größe fehlt")
		}
		n := int(value.ToNum(a[1]))
		if n < 1 {
			return value.ErrorVal("array.Chunk: Größe muss positiv sein")
		}
		rows := make([][]value.Value, 0, (len(v)+n-1)/n)
		for i := 0; i < len(v); i += n {
			end := int(math.Min(float64(i+n), float64(len(v))))
			rows = append(rows, append([]value.Value(nil), v[i:end]...))
		}
		return value.Value{Kind: value.KindArr2D, Arr2D: rows}
	})
	reg("Find", "arr, pattern, [case]", "Filtert Strings mit Wildcards * und ?", func(a []value.Value) value.Value {
		v, e, ok := arrayArg(a, 0, "array.Find")
		if !ok {
			return e
		}
		if len(a) < 2 {
			return value.ErrorVal("array.Find: Suchmuster fehlt")
		}
		pattern := value.ToString(a[1])
		if len(a) < 3 || !value.IsTruthy(a[2]) {
			pattern = strings.ToLower(pattern)
		}
		rx := regexp.QuoteMeta(pattern)
		rx = "^" + strings.ReplaceAll(strings.ReplaceAll(rx, "\\*", ".*"), "\\?", ".") + "$"
		re, err := regexp.Compile(rx)
		if err != nil {
			return value.ErrorVal(err.Error())
		}
		out := []value.Value{}
		for _, x := range v {
			s := value.ToString(x)
			if len(a) < 3 || !value.IsTruthy(a[2]) {
				s = strings.ToLower(s)
			}
			if re.MatchString(s) {
				out = append(out, x)
			}
		}
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("KindOf", "value", "Gibt den Werttyp zurück", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.StrVal("undefined")
		}
		names := map[value.Kind]string{value.KindNum: "number", value.KindStr: "string", value.KindArr: "array", value.KindArr2D: "array2d", value.KindObj: "object", value.KindNull: "null", value.KindBool: "boolean", value.KindMap: "map", value.KindNil: "nil", value.KindBytes: "bytes", value.KindError: "error", value.KindUndefined: "undefined"}
		if n, ok := names[a[0].Kind]; ok {
			return value.StrVal(n)
		}
		return value.StrVal(fmt.Sprintf("kind-%d", a[0].Kind))
	})
}

func safeSize(f float64) int {
	if f < 0 || math.IsNaN(f) {
		return 0
	}
	if f > 1000000 {
		return 1000000
	}
	return int(f)
}
