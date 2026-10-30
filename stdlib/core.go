package stdlib

// Core contains the portable, dependency-free builtins shared with vbx.
import (
	"encoding/hex"
	"math"
	"strings"
	"unicode/utf8"

	"vbxc/value"
)

func need(args []value.Value, n int, name string) (value.Value, bool) {
	if len(args) < n {
		return value.ErrorVal(name + ": zu wenige Argumente"), false
	}
	return value.Value{}, true
}

func strArg(a []value.Value, i int) string {
	if i >= len(a) {
		return ""
	}
	return value.ToString(a[i])
}
func numArg(a []value.Value, i int) float64 {
	if i >= len(a) {
		return 0
	}
	return value.ToNum(a[i])
}

func init() {
	reg := func(name, params, desc string, fn func([]value.Value) value.Value) {
		Register(name, "global", params, desc, fn)
	}
	reg("Len", "wert", "Zeichenanzahl eines Strings oder Elementanzahl eines Arrays", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "Len"); !ok {
			return e
		}
		switch a[0].Kind {
		case value.KindArr:
			return value.NumVal(float64(len(a[0].Arr)))
		case value.KindArr2D:
			return value.NumVal(float64(len(a[0].Arr2D)))
		}
		return value.NumVal(float64(utf8.RuneCountInString(value.ToString(a[0]))))
	})
	reg("Length", "wert", "Alias für Len", func(a []value.Value) value.Value { return call("Len", a) })
	reg("UCase", "text", "Großbuchstaben", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "UCase"); !ok {
			return e
		}
		return value.StrVal(strings.ToUpper(strArg(a, 0)))
	})
	reg("ToUpper", "text", "Wandelt Text in Großbuchstaben um", func(a []value.Value) value.Value { return call("UCase", a) })
	reg("LCase", "text", "Kleinbuchstaben", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "LCase"); !ok {
			return e
		}
		return value.StrVal(strings.ToLower(strArg(a, 0)))
	})
	reg("ToLower", "text", "Wandelt Text in Kleinbuchstaben um", func(a []value.Value) value.Value { return call("LCase", a) })
	reg("Trim", "text", "Entfernt Leerraum am Anfang und Ende", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "Trim"); !ok {
			return e
		}
		return value.StrVal(strings.TrimSpace(strArg(a, 0)))
	})
	reg("TrimStart", "text", "Entfernt Leerraum am Anfang", func(a []value.Value) value.Value {
		if e, ok := need(a, 2, "TrimStart"); !ok {
			return e
		}
		return value.StrVal(strings.TrimLeft(strArg(a, 0), strArg(a, 1)))
	})
	reg("TrimEnd", "text", "Entfernt Leerraum am Ende", func(a []value.Value) value.Value {
		if e, ok := need(a, 2, "TrimEnd"); !ok {
			return e
		}
		return value.StrVal(strings.TrimRight(strArg(a, 0), strArg(a, 1)))
	})
	reg("Left", "text, anzahl", "Linke Zeichen", func(a []value.Value) value.Value {
		if e, ok := need(a, 2, "Left"); !ok {
			return e
		}
		r := []rune(strArg(a, 0))
		n := clampCount(numArg(a, 1), len(r))
		return value.StrVal(string(r[:n]))
	})
	reg("Right", "text, anzahl", "Rechte Zeichen", func(a []value.Value) value.Value {
		if e, ok := need(a, 2, "Right"); !ok {
			return e
		}
		r := []rune(strArg(a, 0))
		n := clampCount(numArg(a, 1), len(r))
		return value.StrVal(string(r[len(r)-n:]))
	})
	reg("Mid", "text, start, [anzahl]", "Teilstring (Start ab 1)", func(a []value.Value) value.Value {
		if len(a) < 2 || len(a) > 3 {
			return value.ErrorVal("Mid erwartet zwei oder drei Parameter: text, start [, length]")
		}
		r := []rune(strArg(a, 0))
		start, valid := value.ToInt(a[1])
		if !valid || start < 1 {
			return value.ErrorVal("Mid: start muss eine Zahl >= 1 sein")
		}
		start--
		if start >= len(r) {
			return value.StrVal("")
		}
		end := len(r)
		if len(a) > 2 {
			n, ok := value.ToInt(a[2])
			if !ok {
				return value.ErrorVal("Mid: length muss eine Zahl sein")
			}
			if n <= 0 {
				return value.StrVal("")
			}
			end = start + n
			if end > len(r) {
				end = len(r)
			}
		}
		return value.StrVal(string(r[start:end]))
	})
	reg("Contains", "text, suchtext", "Prüft, ob Text einen Teilstring enthält", func(a []value.Value) value.Value {
		if e, ok := need(a, 2, "Contains"); !ok {
			return e
		}
		return value.BoolVal(strings.Contains(strings.ToLower(strArg(a, 0)), strings.ToLower(strArg(a, 1))))
	})
	reg("IndexOf", "text, suchtext", "Gibt den 0-basierten Index des ersten Teilstrings oder -1 zurück", func(a []value.Value) value.Value {
		if e, ok := need(a, 2, "IndexOf"); !ok {
			return e
		}
		return value.NumVal(float64(strings.Index(strArg(a, 0), strArg(a, 1))))
	})
	reg("InStr", "text, suchtext", "Gibt die 1-basierte Fundposition oder 0 zurück", func(a []value.Value) value.Value {
		if e, ok := need(a, 2, "InStr"); !ok {
			return e
		}
		text, needle := []rune(strArg(a, 0)), []rune(strArg(a, 1))
		if len(needle) == 0 {
			return value.NumVal(1)
		}
		for i := 0; i+len(needle) <= len(text); i++ {
			match := true
			for j := range needle {
				if text[i+j] != needle[j] {
					match = false
					break
				}
			}
			if match {
				return value.NumVal(float64(i + 1))
			}
		}
		return value.NumVal(0)
	})
	reg("Replace", "text, alt, neu", "Ersetzt alle Vorkommen eines Teilstrings", func(a []value.Value) value.Value {
		if e, ok := need(a, 3, "Replace"); !ok {
			return e
		}
		return value.StrVal(strings.ReplaceAll(strArg(a, 0), strArg(a, 1), strArg(a, 2)))
	})
	reg("Split", "text, trenner, [removeEmpty], [unique]", "Teilt Text in ein Array", func(a []value.Value) value.Value {
		if e, ok := need(a, 2, "Split"); !ok {
			return e
		}
		if strArg(a, 1) == "" {
			return value.ErrorVal("separator cannot be empty")
		}
		parts := strings.Split(strArg(a, 0), strArg(a, 1))
		out := make([]value.Value, 0, len(parts))
		seen := map[string]bool{}
		removeEmpty := len(a) > 2 && value.IsTruthy(a[2])
		unique := len(a) > 3 && value.IsTruthy(a[3])
		for _, s := range parts {
			s = strings.TrimSpace(s)
			if removeEmpty && s == "" {
				continue
			}
			if unique && seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, value.StrVal(s))
		}
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("Join", "array, [trenner]", "Verbindet Array-Elemente", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "Join"); !ok {
			return e
		}
		if a[0].Kind != value.KindArr {
			return value.ErrorVal("Join: Argument muss ein Array sein")
		}
		sep := "\n"
		if len(a) > 1 {
			sep = strArg(a, 1)
		}
		parts := make([]string, len(a[0].Arr))
		for i, v := range a[0].Arr {
			parts[i] = value.ToString(v)
		}
		return value.StrVal(strings.Join(parts, sep))
	})
	reg("HexEncode", "text", "Wandelt Text in Hex um", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "HexEncode"); !ok {
			return e
		}
		return value.StrVal(hex.EncodeToString([]byte(strArg(a, 0))))
	})
	reg("HexDecode", "hex", "Dekodiert Hex-Text", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "HexDecode"); !ok {
			return e
		}
		b, err := hex.DecodeString(strArg(a, 0))
		if err != nil {
			return value.ErrorVal(err.Error())
		}
		return value.StrVal(string(b))
	})
	reg("ToString", "wert", "Wandelt einen Wert in Text um", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "ToString"); !ok {
			return e
		}
		return value.StrVal(value.ToString(a[0]))
	})
	reg("ToInt", "wert", "Wandelt einen Wert in eine Ganzzahl um", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "ToInt"); !ok {
			return e
		}
		return value.NumVal(math.Trunc(value.ToNum(a[0])))
	})
	reg("ToFloat", "wert", "Wandelt einen Wert in eine Zahl um", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "ToFloat"); !ok {
			return e
		}
		return value.NumVal(value.ToNum(a[0]))
	})
	reg("ToBool", "wert", "Wandelt einen Wert in Boolean um", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "ToBool"); !ok {
			return e
		}
		return value.BoolVal(value.IsTruthy(a[0]))
	})
	reg("Chr", "code", "Gibt das Zeichen zum Unicode-Wert zurück", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "Chr"); !ok {
			return e
		}
		return value.StrVal(string(rune(int(value.ToNum(a[0])))))
	})
	reg("Asc", "text", "Gibt den Unicode-Wert des ersten Zeichens zurück", func(a []value.Value) value.Value {
		if e, ok := need(a, 1, "Asc"); !ok {
			return e
		}
		r := []rune(strArg(a, 0))
		if len(r) == 0 {
			return value.NumVal(0)
		}
		return value.NumVal(float64(r[0]))
	})
}

func call(name string, a []value.Value) value.Value { info, _ := Lookup(name); return info.Fn(a) }
func clampCount(n float64, max int) int {
	if n < 0 {
		return 0
	}
	if n > float64(max) {
		return max
	}
	return int(n)
}
