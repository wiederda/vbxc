package stdlib

// String namespace functions ported from the vbx interpreter.
import (
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"vbxc/value"
)

func init() {
	reg := func(name, params, desc string, fn func([]value.Value) value.Value) {
		Register(name, "string", params, desc, fn)
	}
	get := func(a []value.Value, i int) string {
		if i >= len(a) {
			return ""
		}
		return value.ToString(a[i])
	}
	reg("HexEncode", "val", "Wandelt Text in Hex um", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.StrVal("")
		}
		return value.StrVal(hex.EncodeToString([]byte(get(a, 0))))
	})
	reg("HexDecode", "hexStr", "Dekodiert Hex-Text", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.StrVal("")
		}
		b, e := hex.DecodeString(get(a, 0))
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.StrVal(string(b))
	})
	reg("CleanLines", "s", "Trimmt Zeilenenden und erhält Leerzeilen", func(a []value.Value) value.Value {
		s := strings.ReplaceAll(get(a, 0), "\r\n", "\n")
		lines := strings.Split(s, "\n")
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], " \t")
		}
		return value.StrVal(strings.Trim(strings.Join(lines, "\n"), "\n"))
	})
	reg("CleanAllLines", "s", "Entfernt Leerzeilen und trimmt verbleibende Zeilen", func(a []value.Value) value.Value {
		lines := strings.Split(strings.ReplaceAll(get(a, 0), "\r\n", "\n"), "\n")
		out := []string{}
		for _, s := range lines {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return value.StrVal(strings.Join(out, "\n"))
	})
	reg("CharAt", "str, index", "Gibt ein Zeichen an einer 1-basierten Position zurück", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.ErrorVal("usage: string.CharAt(str, index)")
		}
		r := []rune(get(a, 0))
		i := int(value.ToNum(a[1]))
		if i < 1 || i > len(r) {
			return value.StrVal("")
		}
		return value.StrVal(string(r[i-1]))
	})
	// vbx registriert Mid im string-Modul; vbxc behält zusätzlich den
	// unqualifizierten globalen Aufruf für bestehende Skripte.
	reg("Mid", "text, start, [length]", "Gibt einen Teilstring ab der 1-basierten Position zurück", func(a []value.Value) value.Value {
		return call("Mid", a)
	})
	reg("Join", "arr, [sep]", "Verbindet Array-Elemente; Standardtrenner ist Zeilenumbruch", func(a []value.Value) value.Value {
		if len(a) == 0 || a[0].Kind != value.KindArr {
			return value.StrVal("")
		}
		sep := "\n"
		if len(a) > 1 {
			sep = get(a, 1)
		}
		p := make([]string, len(a[0].Arr))
		for i, x := range a[0].Arr {
			p[i] = value.ToString(x)
		}
		return value.StrVal(strings.Join(p, sep))
	})
	reg("Choose", "n, v...", "Wählt ein Element anhand eines 1-basierten Index", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.Value{Kind: value.KindUndefined}
		}
		i := int(value.ToNum(a[0]))
		if i < 1 || i >= len(a) {
			return value.Value{Kind: value.KindUndefined}
		}
		return a[i]
	})
	reg("InsertAfterText", "text, suchtext, einfügen", "Fügt in jeder Zeile nach dem ersten Treffer Text ein", func(a []value.Value) value.Value {
		if len(a) < 3 {
			return value.ErrorVal("InsertAfterText erwartet drei Parameter")
		}
		search := get(a, 1)
		if search == "" {
			return value.ErrorVal("InsertAfterText: Der Suchtext darf nicht leer sein")
		}
		s := strings.ReplaceAll(strings.ReplaceAll(get(a, 0), "\r\n", "\n"), "\r", "\n")
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			if p := strings.Index(line, search); p >= 0 {
				p += len(search)
				lines[i] = line[:p] + get(a, 2) + line[p:]
			}
		}
		return value.StrVal(strings.Join(lines, "\n"))
	})
	reg("RemoveLine", "text, suchtext|zeile, [all]", "Entfernt Zeilen anhand Text oder 0-basierter Zeilennummer", func(a []value.Value) value.Value {
		if len(a) < 2 || a[0].Kind != value.KindStr {
			return value.ErrorVal("string.RemoveLine erwartet text und Suchtext oder Zeilennummer")
		}
		src := a[0].Str
		ending := "\n"
		if strings.Contains(src, "\r\n") {
			ending = "\r\n"
		} else if strings.Contains(src, "\r") {
			ending = "\r"
		}
		lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(src, "\r\n", "\n"), "\r", "\n"), "\n")
		all := false
		if len(a) > 2 {
			if a[2].Kind != value.KindBool {
				return value.ErrorVal("string.RemoveLine: all muss Boolean sein")
			}
			all = a[2].Bool
		}
		out := make([]string, 0, len(lines))
		if a[1].Kind == value.KindNum {
			i := int(a[1].Num)
			if i < 0 || i >= len(lines) {
				return a[0]
			}
			for j, s := range lines {
				if i != j {
					out = append(out, s)
				}
			}
		} else if a[1].Kind == value.KindStr {
			needle := a[1].Str
			removed := false
			for _, s := range lines {
				if needle != "" && strings.Contains(s, needle) && (all || !removed) {
					removed = true
					continue
				}
				out = append(out, s)
			}
		} else {
			return value.ErrorVal("string.RemoveLine erwartet String oder Zahl")
		}
		return value.StrVal(strings.Join(out, ending))
	})
	reg("Switch", "v...", "Gibt den Wert der ersten wahren Bedingung zurück", func(a []value.Value) value.Value {
		if len(a) < 2 || len(a)%2 != 0 {
			return value.Value{Kind: value.KindUndefined}
		}
		for i := 0; i < len(a); i += 2 {
			if value.IsTruthy(a[i]) {
				return a[i+1]
			}
		}
		return value.Value{Kind: value.KindUndefined}
	})
	reg("RegExp", "s, pattern", "Gibt den ersten Regex-Treffer und seine Gruppen als Array zurück", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.Value{Kind: value.KindArr, Arr: []value.Value{}}
		}
		re, e := regexp.Compile(get(a, 1))
		if e != nil {
			return value.ErrorVal("RegExp: invalid pattern: " + e.Error())
		}
		m := re.FindStringSubmatch(get(a, 0))
		out := make([]value.Value, len(m))
		for i, s := range m {
			out[i] = value.StrVal(s)
		}
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("Extract", "text, pattern", "Gibt die erste Regex-Gruppe zurück", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.StrVal("")
		}
		re, e := regexp.Compile(get(a, 1))
		if e != nil {
			return value.StrVal("")
		}
		m := re.FindStringSubmatch(get(a, 0))
		if len(m) > 1 {
			return value.StrVal(m[1])
		}
		return value.StrVal("")
	})
	reg("ExtractMax", "text, pattern", "Gibt den höchsten extrahierten Versionswert zurück", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.StrVal("")
		}
		re, e := regexp.Compile(get(a, 1))
		if e != nil {
			return value.StrVal("")
		}
		matches := re.FindAllStringSubmatch(get(a, 0), -1)
		versions := []string{}
		for _, m := range matches {
			if len(m) > 1 {
				versions = append(versions, m[1])
			}
		}
		if len(versions) == 0 {
			return value.StrVal("")
		}
		sort.Slice(versions, func(i, j int) bool {
			if len(versions[i]) != len(versions[j]) {
				return len(versions[i]) < len(versions[j])
			}
			return versions[i] < versions[j]
		})
		return value.StrVal(versions[len(versions)-1])
	})
	reg("PatternFromExample", "example, exact, position, [keep]", "Erzeugt ein Regex-Muster aus Beispieltext", func(a []value.Value) value.Value {
		if len(a) < 3 {
			return value.ErrorVal("PatternFromExample: expected example, exact, position")
		}
		example, position, keep := get(a, 0), get(a, 2), get(a, 3)
		if position != "start" && position != "end" && position != "anywhere" {
			return value.ErrorVal("PatternFromExample: invalid position")
		}
		exact := value.IsTruthy(a[1])
		pattern := classifyStringRuns(example, exact)
		if keep != "" {
			if i := strings.Index(example, keep); i >= 0 {
				pattern = classifyStringRuns(example[:i], exact) + regexp.QuoteMeta(keep) + classifyStringRuns(example[i+len(keep):], exact)
			}
		}
		if position == "start" {
			pattern = "^" + pattern
		} else if position == "end" {
			pattern += "$"
		}
		return value.StrVal(pattern)
	})
	reg("CombinePatterns", "patterns", "Verbindet Regex-Muster mit Oder", func(a []value.Value) value.Value {
		if len(a) == 0 || a[0].Kind != value.KindArr {
			return value.ErrorVal("CombinePatterns: patterns must be an array")
		}
		p := make([]string, len(a[0].Arr))
		for i, x := range a[0].Arr {
			p[i] = "(?:" + value.ToString(x) + ")"
		}
		return value.StrVal(strings.Join(p, "|"))
	})
	reg("WordCount", "s", "Zählt Wörter", func(a []value.Value) value.Value { return value.NumVal(float64(len(strings.Fields(get(a, 0))))) })
	reg("CharCount", "s", "Zählt Zeichen außer Leerraum", func(a []value.Value) value.Value {
		n := 0
		for _, r := range get(a, 0) {
			if !unicode.IsSpace(r) {
				n++
			}
		}
		return value.NumVal(float64(n))
	})
	reg("StartsWith", "s, prefix", "Prüft auf Präfix", func(a []value.Value) value.Value {
		return value.BoolVal(len(a) >= 2 && strings.HasPrefix(get(a, 0), get(a, 1)))
	})
	reg("EndsWith", "s, suffix", "Prüft auf Suffix", func(a []value.Value) value.Value {
		return value.BoolVal(len(a) >= 2 && strings.HasSuffix(get(a, 0), get(a, 1)))
	})
	reg("Repeat", "s, n", "Wiederholt Text n-mal", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.StrVal("")
		}
		return value.StrVal(strings.Repeat(get(a, 0), safeRepeatCount(value.ToNum(a[1]))))
	})
	reg("Reverse", "s", "Kehrt Zeichenreihenfolge um", func(a []value.Value) value.Value {
		r := []rune(get(a, 0))
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		return value.StrVal(string(r))
	})
	reg("PadLeft", "s, n, [c]", "Füllt links auf die gewünschte Zeichenlänge", func(a []value.Value) value.Value { return padString(get(a, 0), a, true) })
	reg("PadRight", "s, n, [c]", "Füllt rechts auf die gewünschte Zeichenlänge", func(a []value.Value) value.Value { return padString(get(a, 0), a, false) })
	reg("Val", "s", "Liest einen numerischen Präfix aus Text", func(a []value.Value) value.Value {
		s := strings.TrimLeft(get(a, 0), " ")
		var b strings.Builder
		hasDecimal, started := false, false
		for _, r := range s {
			switch {
			case r >= '0' && r <= '9':
				b.WriteRune(r)
				started = true
			case r == '.' && !hasDecimal:
				b.WriteRune(r)
				hasDecimal = true
				started = true
			case (r == '-' || r == '+') && !started:
				b.WriteRune(r)
				started = true
			default:
				if started {
					s = ""
					goto parsed
				}
			}
		}
	parsed:
		number := b.String()
		if number != "" && number != "-" && number != "+" && number != "." {
			if n, e := strconv.ParseFloat(number, 64); e == nil {
				return value.NumVal(n)
			}
		}
		return value.NumVal(0)
	})
	reg("Like", "text, pattern", "VB-Mustervergleich mit #, ? und *", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.BoolVal(false)
		}
		p := regexp.QuoteMeta(get(a, 1))
		p = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(p, "\\#", "[0-9]"), "\\?", "."), "\\*", ".*")
		ok, e := regexp.MatchString("^"+p+"$", get(a, 0))
		return value.BoolVal(e == nil && ok)
	})
	reg("TrimLines", "content", "Entfernt Leerraum am Zeilenende", func(a []value.Value) value.Value {
		lines := strings.Split(get(a, 0), "\n")
		for i, s := range lines {
			lines[i] = strings.TrimRight(s, " \t\r")
		}
		return value.StrVal(strings.Join(lines, "\n"))
	})
	reg("StrConv", "input", "Sichert gültige UTF-8-Darstellung", func(a []value.Value) value.Value {
		s := get(a, 0)
		if !utf8.ValidString(s) {
			s = strings.ToValidUTF8(s, "�")
		}
		return value.StrVal(s)
	})
	reg("StrComp", "s1, s2, [mode]", "Vergleicht zwei Strings; Mode 1 ignoriert Großschreibung", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.ErrorVal("string.StrComp erwartet zwei Strings")
		}
		x, y := get(a, 0), get(a, 1)
		if len(a) > 2 && value.ToNum(a[2]) != 0 {
			x, y = strings.ToLower(x), strings.ToLower(y)
		}
		return compareString(x, y)
	})
	reg("CompareText", "string1, string2", "Vergleicht Texte ohne Groß-/Kleinschreibung", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.ErrorVal("usage: string.CompareText(s1, s2)")
		}
		return compareString(strings.ToLower(get(a, 0)), strings.ToLower(get(a, 1)))
	})
	reg("TxtToSqlInsert", "data, table, dialect, [extras], [columns], [batchSize]", "Erzeugt SQL-INSERT-Anweisungen aus einem Array", func(a []value.Value) value.Value { return sqlInserts(a) })
	reg("Between", "s, start, end", "Extrahiert Text zwischen zwei Markern", func(a []value.Value) value.Value {
		if len(a) < 3 {
			return value.ErrorVal("string.Between benötigt 3 Argumente")
		}
		s, start, end := get(a, 0), get(a, 1), get(a, 2)
		i := strings.Index(s, start)
		if i < 0 {
			return value.StrVal("")
		}
		s = s[i+len(start):]
		j := strings.Index(s, end)
		if j < 0 {
			return value.StrVal("")
		}
		return value.StrVal(s[:j])
	})
	reg("Base32Encode", "val", "Kodiert Text als Base32", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.StrVal("")
		}
		return value.StrVal(base32.StdEncoding.EncodeToString([]byte(get(a, 0))))
	})
	reg("Base32Decode", "base32Str", "Dekodiert Base32-Text", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.StrVal("")
		}
		b, e := base32.StdEncoding.DecodeString(get(a, 0))
		if e != nil {
			return value.ErrorVal("encoding.Base32Decode: " + e.Error())
		}
		return value.StrVal(string(b))
	})
	reg("Space", "n", "Erzeugt n Leerzeichen", func(a []value.Value) value.Value {
		n := 0
		if len(a) > 0 {
			n = safeRepeatCount(value.ToNum(a[0]))
		}
		return value.StrVal(strings.Repeat(" ", n))
	})
	reg("StrDup", "n, char", "Wiederholt das erste Zeichen n-mal", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.StrVal("")
		}
		r := []rune(get(a, 1))
		if len(r) == 0 {
			return value.StrVal("")
		}
		return value.StrVal(strings.Repeat(string(r[0]), safeRepeatCount(value.ToNum(a[0]))))
	})

	Register("Wipe", "crypt", "byteArray", "Überschreibt ein Byte-Array in-place mit Nullen.", func(a []value.Value) value.Value {
		if len(a) != 1 || a[0].Kind != value.KindArr {
			return value.ErrorVal("crypt.Wipe erwartet genau ein Byte-Array")
		}
		for i := range a[0].Arr {
			a[0].Arr[i] = value.NumVal(0)
		}
		return value.BoolVal(true)
	})
	Register("WipeString", "crypt", "value", "Überschreibt den Speicherinhalt eines Strings mit Nullbytes.", func(a []value.Value) value.Value {
		if len(a) != 1 || a[0].Kind != value.KindStr {
			return value.ErrorVal("crypt.WipeString erwartet genau einen String")
		}
		s := a[0].Str
		if len(s) == 0 {
			return value.BoolVal(true)
		}
		data := unsafe.Slice(unsafe.StringData(s), len(s))
		for i := range data {
			data[i] = 0
		}
		return value.BoolVal(true)
	})
	Register("string.Wipe", "string", "byteArray", "Überschreibt ein Byte-Array in-place mit Nullen.", func(a []value.Value) value.Value {
		return call("crypt.Wipe", a)
	})
	Register("string.WipeString", "string", "value", "Überschreibt den Speicherinhalt eines Strings mit Nullbytes.", func(a []value.Value) value.Value {
		return call("crypt.WipeString", a)
	})
}

func safeRepeatCount(n float64) int {
	if math.IsNaN(n) || n < 0 {
		return 0
	}
	if n > 1_000_000 {
		return 1_000_000
	}
	return int(n)
}
func compareString(a, b string) value.Value {
	if a < b {
		return value.NumVal(-1)
	}
	if a > b {
		return value.NumVal(1)
	}
	return value.NumVal(0)
}
func padString(s string, a []value.Value, left bool) value.Value {
	if len(a) < 2 {
		return value.StrVal(s)
	}
	n := safeRepeatCount(value.ToNum(a[1]))
	r := []rune(s)
	if n <= len(r) {
		return value.StrVal(s)
	}
	fill := []rune(" ")
	if len(a) > 2 && value.ToString(a[2]) != "" {
		fill = []rune(value.ToString(a[2]))
	}
	var padding strings.Builder
	for i := 0; i < n-len(r); i++ {
		padding.WriteRune(fill[i%len(fill)])
	}
	p := padding.String()
	if left {
		return value.StrVal(p + s)
	}
	return value.StrVal(s + p)
}
func classifyStringRuns(s string, exact bool) string {
	r := []rune(s)
	var b strings.Builder
	for i := 0; i < len(r); {
		pred := func(x rune) bool {
			return (unicode.IsDigit(r[i]) && unicode.IsDigit(x)) || (unicode.IsLetter(r[i]) && unicode.IsLetter(x))
		}
		if unicode.IsDigit(r[i]) || unicode.IsLetter(r[i]) {
			j := i + 1
			for j < len(r) && pred(r[j]) {
				j++
			}
			if unicode.IsDigit(r[i]) {
				if exact {
					fmt.Fprintf(&b, `\d{%d}`, j-i)
				} else {
					b.WriteString(`\d+`)
				}
			} else if exact {
				fmt.Fprintf(&b, `[a-zA-Z]{%d}`, j-i)
			} else {
				b.WriteString(`[a-zA-Z]+`)
			}
			i = j
		} else {
			b.WriteString(regexp.QuoteMeta(string(r[i])))
			i++
		}
	}
	return b.String()
}
func sqlInserts(a []value.Value) value.Value {
	if len(a) < 3 {
		return value.ErrorVal("TxtToSqlInsert(data, table, dialect, [extras], [columns], [batchSize])")
	}
	data := a[0]
	if data.Kind != value.KindArr || len(data.Arr) == 0 {
		return value.StrVal("")
	}
	table, dialect := value.ToString(a[1]), strings.ToLower(value.ToString(a[2]))
	escape := func(v value.Value) string {
		if v.Kind == value.KindNum {
			return strconv.FormatFloat(v.Num, 'f', -1, 64)
		}
		return "'" + strings.ReplaceAll(value.ToString(v), "'", "''") + "'"
	}
	extras := []string{}
	if len(a) > 3 && a[3].Kind == value.KindArr {
		for _, x := range a[3].Arr {
			extras = append(extras, escape(x))
		}
	}
	cols := ""
	if len(a) > 4 && a[4].Kind == value.KindArr {
		c := make([]string, len(a[4].Arr))
		for i, x := range a[4].Arr {
			c[i] = value.ToString(x)
		}
		cols = " (" + strings.Join(c, ", ") + ")"
	}
	batch := 0
	if len(a) > 5 {
		batch = int(value.ToNum(a[5]))
	}
	var b strings.Builder
	switch dialect {
	case "sqlite", "postgres":
		b.WriteString("BEGIN;\n")
	case "mssql":
		b.WriteString("BEGIN TRANSACTION\n")
	}
	for i, row := range data.Arr {
		if i == 0 || (batch > 0 && i%batch == 0) {
			fmt.Fprintf(&b, "INSERT INTO %s%s VALUES\n", table, cols)
		}
		parts := append([]string{escape(row)}, extras...)
		b.WriteByte('(')
		b.WriteString(strings.Join(parts, ", "))
		b.WriteByte(')')
		last := i == len(data.Arr)-1 || batch > 0 && (i+1)%batch == 0
		if last {
			b.WriteString(";\n")
			if dialect == "mssql" {
				b.WriteString("GO\n")
			}
		} else {
			b.WriteString(",\n")
		}
	}
	switch dialect {
	case "sqlite", "postgres":
		b.WriteString("COMMIT;\n")
	case "mssql":
		b.WriteString("COMMIT\nGO\n")
	}
	return value.StrVal(b.String())
}
