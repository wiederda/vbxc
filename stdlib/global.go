package stdlib

// Additional mandatory global builtins shared with the vbx interpreter.
import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"vbxc/bytecode"
	"vbxc/compiler"
	"vbxc/parser"
	"vbxc/value"
)

func init() {
	reg := func(name, params, desc string, fn func([]value.Value) value.Value) {
		Register(name, "global", params, desc, fn)
	}
	get := func(a []value.Value, i int) string {
		if i >= len(a) {
			return ""
		}
		return value.ToString(a[i])
	}
	reg("MD5", "s", "Erzeugt einen MD5-Hash", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("MD5: Argument fehlt")
		}
		h := md5.Sum([]byte(get(a, 0)))
		return value.StrVal(hex.EncodeToString(h[:]))
	})
	reg("SHA1", "s", "Erzeugt einen SHA1-Hash", func(a []value.Value) value.Value {
		h := sha1.Sum([]byte(get(a, 0)))
		return value.StrVal(hex.EncodeToString(h[:]))
	})
	reg("SHA256", "s", "Erzeugt einen SHA256-Hash", func(a []value.Value) value.Value {
		h := sha256.Sum256([]byte(get(a, 0)))
		return value.StrVal(hex.EncodeToString(h[:]))
	})
	reg("SHA512", "s", "Erzeugt einen SHA512-Hash", func(a []value.Value) value.Value {
		h := sha512.Sum512([]byte(get(a, 0)))
		return value.StrVal(fmt.Sprintf("%x", h[:]))
	})
	reg("LastIndexOf", "s, sub", "Gibt die letzte Fundposition zurück", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.NumVal(-1)
		}
		return value.NumVal(float64(strings.LastIndex(get(a, 0), get(a, 1))))
	})
	reg("Compare", "v1, v2", "Vergleicht zwei Major.Minor.Patch-Versionen", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.ErrorVal("version.Compare: zwei Versionen benötigt")
		}
		parse := func(s string) [3]int {
			var v [3]int
			p := strings.Split(s, ".")
			for i := 0; i < len(p) && i < 3; i++ {
				v[i], _ = strconv.Atoi(p[i])
			}
			return v
		}
		x, y := parse(get(a, 0)), parse(get(a, 1))
		for i := 0; i < 3; i++ {
			if x[i] > y[i] {
				return value.NumVal(1)
			}
			if x[i] < y[i] {
				return value.NumVal(-1)
			}
		}
		return value.NumVal(0)
	})
	reg("ReplaceMany", "text, find1, repl1, ...", "Ersetzt mehrere Teilstrings", func(a []value.Value) value.Value {
		if len(a) < 3 || (len(a)-1)%2 != 0 {
			return value.ErrorVal("ReplaceMany benötigt Such- und Ersatztexte als Paare")
		}
		s := get(a, 0)
		for i := 1; i < len(a); i += 2 {
			s = strings.ReplaceAll(s, get(a, i), get(a, i+1))
		}
		return value.StrVal(s)
	})
	reg("EncodeBase64", "s", "Codiert Text als Base64", func(a []value.Value) value.Value {
		if len(a) == 0 || a[0].Kind != value.KindStr {
			return value.StrVal("error: expected string")
		}
		return value.StrVal(base64.StdEncoding.EncodeToString([]byte(a[0].Str)))
	})
	reg("DecodeBase64", "s", "Dekodiert Base64-Text", func(a []value.Value) value.Value {
		if len(a) == 0 || a[0].Kind != value.KindStr {
			return value.StrVal("error: expected string")
		}
		b, e := base64.StdEncoding.DecodeString(a[0].Str)
		if e != nil {
			return value.StrVal("error: " + e.Error())
		}
		return value.StrVal(string(b))
	})
	reg("Substring", "s, start, [len]", "Gibt einen Teilstring mit 0-basierter Position zurück", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return value.ErrorVal("Substring erwartet String und StartIndex")
		}
		r := []rune(get(a, 0))
		i := int(value.ToNum(a[1]))
		if i < 0 {
			i = 0
		}
		if i >= len(r) {
			return value.StrVal("")
		}
		n := len(r) - i
		if len(a) > 2 {
			n = int(value.ToNum(a[2]))
			if n < 0 {
				n = 0
			}
			if n > len(r)-i {
				n = len(r) - i
			}
		}
		return value.StrVal(string(r[i : i+n]))
	})
	reg("Int", "[max]", "Gibt eine Zufallszahl von 0 bis max-1 zurück", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.NumVal(float64(rand.Int()))
		}
		n := int(value.ToNum(a[0]))
		if n <= 0 {
			return value.NumVal(0)
		}
		return value.NumVal(float64(rand.Intn(n)))
	})
	reg("URLEncode", "val", "Codiert Text für eine URL", func(a []value.Value) value.Value { return value.StrVal(url.QueryEscape(get(a, 0))) })
	reg("URLDecode", "urlStr", "Dekodiert URL-kodierten Text", func(a []value.Value) value.Value {
		s, e := url.QueryUnescape(get(a, 0))
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.StrVal(s)
	})
	reg("GC", "-", "Erzwingt einen Garbage Collection-Lauf", func([]value.Value) value.Value { runtime.GC(); return value.NullVal() })
	reg("LoadModule", "name", "Prüft, ob ein Modul in dieser Runtime verfügbar ist", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("LoadModule: Modulname fehlt")
		}
		name := strings.ToLower(strings.TrimSpace(get(a, 0)))
		if name == "" {
			return value.ErrorVal("LoadModule: Modulname ist leer")
		}
		for _, info := range builtins {
			if strings.ToLower(info.Module) == name {
				return value.BoolVal(true)
			}
		}
		return value.ErrorVal("Modul nicht gefunden: " + name)
	})
	reg("Format", "expression, style", "Formatiert Zahlen, Datum oder Text", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.StrVal("")
		}
		if len(a) < 2 {
			return value.StrVal(value.ToString(a[0]))
		}
		return formatBuiltin(a[0], get(a, 1))
	})
	reg("PrintFormat", "", "Gibt eine Übersicht der Formatoptionen aus", func([]value.Value) value.Value {
		fmt.Println("Format(wert, style): currency, percent, hex oder Zahlenmuster wie 0.00")
		return value.NullVal()
	})
	reg("FormatSize", "bytes", "Formatiert eine Byte-Anzahl", func(a []value.Value) value.Value {
		n := value.ToNum(first(a))
		units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
		i := 0
		for n >= 1024 && i < len(units)-1 {
			n /= 1024
			i++
		}
		return value.StrVal(fmt.Sprintf("%.1f %s", n, units[i]))
	})
	reg("Default", "val, default", "Gibt bei Null oder leerem Text den Standardwert zurück", func(a []value.Value) value.Value {
		if len(a) < 2 {
			return first(a)
		}
		if a[0].Kind == value.KindNull || a[0].Kind == value.KindNil || a[0].Kind == value.KindUndefined || (a[0].Kind == value.KindStr && a[0].Str == "") {
			return a[1]
		}
		return a[0]
	})
	reg("Trim", "s, [cutset]", "Entfernt Leerraum oder Zeichen an beiden Enden", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("Trim: Text fehlt")
		}
		if len(a) > 1 {
			return value.StrVal(strings.Trim(get(a, 0), get(a, 1)))
		}
		return value.StrVal(strings.TrimSpace(get(a, 0)))
	})
	reg("IsArray", "val", "Prüft auf Array oder Matrix", func(a []value.Value) value.Value {
		return value.BoolVal(len(a) > 0 && (a[0].Kind == value.KindArr || a[0].Kind == value.KindArr2D))
	})
	reg("IsMap", "val", "Prüft auf Map", func(a []value.Value) value.Value { return value.BoolVal(len(a) > 0 && a[0].Kind == value.KindMap) })
	reg("IsString", "val", "Prüft auf String", func(a []value.Value) value.Value { return value.BoolVal(len(a) > 0 && a[0].Kind == value.KindStr) })
	reg("IsNull", "val", "Prüft auf Null oder nicht initialisierten Wert", func(a []value.Value) value.Value { return nullLike(a) })
	reg("IsNothing", "val", "Prüft auf nicht initialisierten Wert", func(a []value.Value) value.Value { return nullLike(a) })
	reg("IsNumeric", "v", "Prüft, ob ein Wert numerisch ist", func(a []value.Value) value.Value {
		if len(a) != 1 {
			return value.ErrorVal("IsNumeric erwartet genau 1 Argument")
		}
		if a[0].Kind == value.KindNum {
			return value.BoolVal(true)
		}
		if a[0].Kind != value.KindStr {
			return value.BoolVal(false)
		}
		s := strings.ReplaceAll(strings.TrimSpace(a[0].Str), ",", ".")
		_, e := strconv.ParseFloat(s, 64)
		return value.BoolVal(e == nil && s != "")
	})
	reg("IsInteger", "v", "Prüft, ob ein Wert ganzzahlig ist", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.BoolVal(false)
		}
		switch a[0].Kind {
		case value.KindBool:
			return value.BoolVal(true)
		case value.KindNum:
			return value.BoolVal(a[0].Num == math.Trunc(a[0].Num))
		case value.KindStr:
			s := strings.ReplaceAll(strings.TrimSpace(a[0].Str), ",", ".")
			n, e := strconvParse(s)
			return value.BoolVal(e == nil && n == math.Trunc(n))
		}
		return value.BoolVal(false)
	})
	reg("IsDate", "val", "Prüft, ob ein Wert ein parsebares Datum ist", func(a []value.Value) value.Value {
		if len(a) == 0 || a[0].Kind != value.KindStr {
			return value.BoolVal(false)
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02", "02.01.2006", "2.1.2006", "02.01.06", "1/2/2006", "2006-01-02 15:04:05"} {
			if _, e := time.Parse(layout, a[0].Str); e == nil {
				return value.BoolVal(true)
			}
		}
		return value.BoolVal(false)
	})
	reg("IsPositive", "n", "Prüft, ob eine Zahl positiv ist", func(a []value.Value) value.Value {
		return value.BoolVal(len(a) > 0 && a[0].Kind == value.KindNum && a[0].Num > 0)
	})
	reg("IsPrime", "n", "Prüft, ob eine ganze Zahl prim ist", func(a []value.Value) value.Value {
		if len(a) == 0 || a[0].Kind != value.KindNum {
			return value.BoolVal(false)
		}
		n := int64(a[0].Num)
		if float64(n) != a[0].Num || n < 2 {
			return value.BoolVal(false)
		}
		for i := int64(2); i*i <= n; i++ {
			if n%i == 0 {
				return value.BoolVal(false)
			}
		}
		return value.BoolVal(true)
	})
	reg("ConvNewLine", "content, [target]", "Normalisiert Zeilenenden", func(a []value.Value) value.Value {
		s := strings.ReplaceAll(strings.ReplaceAll(get(a, 0), "\r\n", "\n"), "\r", "\n")
		target := "auto"
		if len(a) > 1 {
			target = strings.ToLower(get(a, 1))
		}
		if target == "auto" || target == "" {
			if runtime.GOOS == "windows" {
				target = "crlf"
			} else {
				target = "lf"
			}
		}
		switch target {
		case "crlf":
			return value.StrVal(strings.ReplaceAll(s, "\n", "\r\n"))
		case "lf":
			return value.StrVal(s)
		case "cr":
			return value.StrVal(strings.ReplaceAll(s, "\n", "\r"))
		default:
			return value.ErrorVal("Unbekanntes Zielformat: " + target)
		}
	})
	reg("OS", "", "Gibt das Betriebssystem zurück", func([]value.Value) value.Value { return value.StrVal(runtime.GOOS) })
	reg("Arch", "", "Gibt die Prozessorarchitektur zurück", func([]value.Value) value.Value { return value.StrVal(runtime.GOARCH) })
	reg("UserName", "", "Gibt den aktuellen Benutzernamen zurück", func([]value.Value) value.Value {
		u, e := user.Current()
		if e != nil {
			return value.StrVal("")
		}
		return value.StrVal(u.Username)
	})
	reg("ComputerName", "", "Gibt den Hostnamen zurück", func([]value.Value) value.Value {
		n, e := os.Hostname()
		if e != nil {
			return value.StrVal("")
		}
		return value.StrVal(n)
	})
	reg("Inspect", "value", "Zeigt Typ und oberste Struktur eines Wertes", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.NullVal()
		}
		v := a[0]
		switch v.Kind {
		case value.KindArr:
			fmt.Printf("Array (%d Elemente)\n", len(v.Arr))
			if len(v.Arr) > 0 {
				fmt.Printf("Beispiel (erstes Element): %s\n", value.ToString(v.Arr[0]))
			}
		case value.KindArr2D:
			cols := 0
			if len(v.Arr2D) > 0 {
				cols = len(v.Arr2D[0])
			}
			fmt.Printf("Array2D (%d Zeilen x %d Spalten)\n", len(v.Arr2D), cols)
		case value.KindMap:
			keys := make([]string, 0, len(v.Map))
			for k := range v.Map {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			fmt.Printf("Map: %s\n", strings.Join(keys, ", "))
		case value.KindObj:
			if v.Obj != nil {
				keys := make([]string, 0, len(v.Obj.Fields))
				for k := range v.Obj.Fields {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				fmt.Printf("%s: %s\n", v.Obj.TypeName, strings.Join(keys, ", "))
			}
		default:
			fmt.Printf("%v: %s\n", v.Kind, value.ToString(v))
		}
		return value.NullVal()
	})
	reg("ToClipboard", "text", "Kopiert Text in die Zwischenablage", func(a []value.Value) value.Value {
		if e := clipboardWrite(get(a, 0)); e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.NullVal()
	})
	reg("FromClipboard", "", "Liest Text aus der Zwischenablage", func([]value.Value) value.Value {
		s, e := clipboardRead()
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.StrVal(s)
	})
	reg("ClipboardHasText", "", "Prüft, ob die Zwischenablage Text enthält", func([]value.Value) value.Value { s, e := clipboardRead(); return value.BoolVal(e == nil && s != "") })
	reg("ClipboardClear", "", "Leert die Zwischenablage", func([]value.Value) value.Value {
		if e := clipboardWrite(""); e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.NullVal()
	})
	reg("UserDomain", "", "Gibt die Benutzer-Domäne oder den Hostnamen zurück", func([]value.Value) value.Value {
		if d := os.Getenv("USERDOMAIN"); d != "" {
			return value.StrVal(d)
		}
		n, e := os.Hostname()
		if e != nil {
			return value.StrVal("")
		}
		return value.StrVal(n)
	})
	reg("Sleep", "wert, [einheit], [showOutput]", "Wartet die angegebene Dauer", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("Sleep: Dauer fehlt")
		}
		duration := value.ToNum(a[0])
		unit := "ms"
		if len(a) > 1 {
			unit = strings.ToLower(get(a, 1))
		}
		switch unit {
		case "s", "sec", "second", "seconds":
			duration *= 1000
		case "m", "min", "minute", "minutes":
			duration *= 60000
		case "h", "hour", "hours":
			duration *= 3600000
		}
		if duration > 0 && duration < 86400000 {
			time.Sleep(time.Duration(duration) * time.Millisecond)
		}
		return value.NumVal(value.ToNum(a[0]))
	})
	reg("Worker", "path", "Startet ein VBX-Skript im Hintergrund", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("Pfad fehlt")
		}
		pid, e := startScriptWorker(get(a, 0))
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.StrVal(strconv.Itoa(pid))
	})
	reg("WorkerPool", "paths[], maxParallel", "Startet mehrere Skripte parallel und wartet auf alle", func(a []value.Value) value.Value {
		if len(a) < 2 || a[0].Kind != value.KindArr {
			return value.ErrorVal("WorkerPool erwartet ein Array und eine Zahl")
		}
		limit := int(value.ToNum(a[1]))
		if limit < 1 {
			limit = 1
		}
		sem := make(chan struct{}, limit)
		var wg sync.WaitGroup
		out := make([]value.Value, len(a[0].Arr))
		for i, p := range a[0].Arr {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, path string) {
				defer wg.Done()
				defer func() { <-sem }()
				pid, e := startScriptWorker(path)
				if e != nil {
					out[i] = value.BoolVal(false)
					return
				}
				out[i] = value.BoolVal(waitScriptWorker(pid))
			}(i, value.ToString(p))
		}
		wg.Wait()
		return value.Value{Kind: value.KindArr, Arr: out}
	})
	reg("Wait", "pid", "Wartet bis ein Prozess beendet ist", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("PID fehlt")
		}
		pid := int(value.ToNum(a[0]))
		if v, ok := workerProcesses.LoadAndDelete(pid); ok {
			cmd := v.(*exec.Cmd)
			return value.BoolVal(cmd.Wait() == nil)
		}
		for processIsRunning(pid) {
			time.Sleep(250 * time.Millisecond)
		}
		return value.BoolVal(true)
	})
	reg("Input", "[prompt]", "Liest eine Zeile und erkennt Zahlen automatisch", func(a []value.Value) value.Value {
		if len(a) > 0 {
			fmt.Print(get(a, 0))
		}
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return value.StrVal("")
		}
		s := scanner.Text()
		n := strings.Replace(strings.TrimSpace(s), ",", ".", 1)
		if x, e := strconv.ParseFloat(n, 64); e == nil {
			return value.NumVal(x)
		}
		return value.StrVal(s)
	})
	reg("Alert", "text", "Zeigt Text an und wartet auf Enter", func(a []value.Value) value.Value {
		fmt.Println(get(a, 0))
		fmt.Print("... [Enter]")
		bufio.NewReader(os.Stdin).ReadString('\n')
		return value.BoolVal(true)
	})
	reg("Confirm", "text", "Zeigt Text an und liest Ja/Nein", func(a []value.Value) value.Value {
		fmt.Print(get(a, 0))
		s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		s = strings.ToLower(strings.TrimSpace(s))
		return value.BoolVal(s == "j" || s == "ja" || s == "y" || s == "yes")
	})
	reg("Password", "[prompt]", "Liest ein Passwort ohne es anzuzeigen", func(a []value.Value) value.Value {
		s, e := readPassword(get(a, 0))
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.StrVal(s)
	})
	reg("CanRun", "path", "Kompiliert ein Skript zur Syntaxprüfung, ohne es auszuführen", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("CanRun: Pfad fehlt")
		}
		chunk, e := compileVBXFile(get(a, 0))
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		_ = chunk
		return value.BoolVal(true)
	})
	reg("CanRunString", "content, [label]", "Kompiliert Quelltext zur Syntaxprüfung, ohne ihn auszuführen", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("CanRunString: content fehlt")
		}
		label := "<string>"
		if len(a) > 1 && get(a, 1) != "" {
			label = get(a, 1)
		}
		chunk, e := compileVBXText(get(a, 0), label, "")
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		_ = chunk
		return value.BoolVal(true)
	})
	reg("Build", "quelle", "Kompiliert eine VBX-Quelldatei in Bytecode", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("Pfad fehlt")
		}
		source := get(a, 0)
		chunk, e := compileVBXFile(source)
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		dest := availableBytecodePath(source)
		f, e := os.Create(dest)
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		if e = bytecode.Write(f, chunk); e != nil {
			f.Close()
			os.Remove(dest)
			return value.ErrorVal(e.Error())
		}
		if e = f.Close(); e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.StrVal("Gespeichert als: " + dest)
	})
	reg("IsCompiled", "path", "Prüft, ob eine gültige VBXC-Bytecode-Datei vorliegt", func(a []value.Value) value.Value {
		if len(a) == 0 || !strings.EqualFold(filepath.Ext(get(a, 0)), bytecode.Extension) {
			return value.BoolVal(false)
		}
		f, e := os.Open(get(a, 0))
		if e != nil {
			return value.BoolVal(false)
		}
		defer f.Close()
		_, e = bytecode.Read(f)
		return value.BoolVal(e == nil)
	})
	reg("Unblock", "path", "Entfernt Windows-Zone.Identifier-Markierungen", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("file.Unblock(path) benötigt einen Pfad")
		}
		if runtime.GOOS != "windows" {
			return value.NumVal(0)
		}
		count := 0
		err := filepath.WalkDir(get(a, 0), func(p string, d os.DirEntry, e error) error {
			if e != nil || d.IsDir() {
				return nil
			}
			ads := p + ":Zone.Identifier"
			if _, e = os.Stat(ads); e == nil {
				if os.Remove(ads) == nil {
					count++
				}
			}
			return nil
		})
		if err != nil {
			return value.ErrorVal("Unblock fehlgeschlagen: " + err.Error())
		}
		return value.NumVal(float64(count))
	})
	reg("Uptime", "", "Gibt die System-Laufzeit in Sekunden zurück", func([]value.Value) value.Value {
		n, e := systemUptimeSeconds()
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.NumVal(n)
	})
	reg("UptimeString", "", "Gibt System-Laufzeit lesbar zurück", func([]value.Value) value.Value {
		n, e := systemUptimeSeconds()
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.StrVal(formatUptime(n))
	})
	reg("SystemMemory", "-", "Gibt System-RAM Gesamt, verfügbar und Nutzungsprozent zurück", func([]value.Value) value.Value {
		total, available, e := systemMemory()
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		used := 0.0
		if total > 0 {
			used = (1 - float64(available)/float64(total)) * 100
		}
		return value.Value{Kind: value.KindArr, Arr: []value.Value{value.NumVal(float64(total)), value.NumVal(float64(available)), value.NumVal(used)}}
	})
	reg("PunyEncode", "domain", "IDNA-kodiert eine Domain", func(a []value.Value) value.Value {
		s, e := punyDomain(get(a, 0), false)
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.StrVal(s)
	})
	reg("PunyDecode", "domain", "Dekodiert eine IDNA-Domain", func(a []value.Value) value.Value {
		s, e := punyDomain(get(a, 0), true)
		if e != nil {
			return value.ErrorVal(e.Error())
		}
		return value.StrVal(s)
	})
}

func first(a []value.Value) value.Value {
	if len(a) == 0 {
		return value.Value{Kind: value.KindUndefined}
	}
	return a[0]
}

var workerProcesses sync.Map

func startScriptWorker(script string) (int, error) {
	abs, err := filepath.Abs(script)
	if err != nil {
		return 0, err
	}
	ext := strings.ToLower(filepath.Ext(abs))
	if ext != ".vb" && ext != ".vbb" {
		return 0, fmt.Errorf("Worker darf nur .vb- oder .vbb-Dateien ausführen")
	}
	log, err := os.OpenFile(abs+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	self, err := os.Executable()
	if err != nil {
		log.Close()
		return 0, err
	}
	// Der Prozess läuft ohne Befehl: vbxc <datei> führt die Datei direkt aus.
	cmd := exec.Command(self, abs)
	cmd.Stdout = log
	cmd.Stderr = log
	if err = cmd.Start(); err != nil {
		log.Close()
		return 0, err
	}
	workerProcesses.Store(cmd.Process.Pid, cmd)
	fmt.Fprintf(log, "\n--- Worker PID %d gestartet am %s ---\n", cmd.Process.Pid, time.Now().Format("15:04:05"))
	log.Close()
	return cmd.Process.Pid, nil
}
func compileVBXFile(path string) (*bytecode.Chunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	source := strings.TrimPrefix(string(data), "\uFEFF")
	return compileVBXText(source, path, filepath.Dir(path))
}
func compileVBXText(source, file, baseDir string) (*bytecode.Chunk, error) {
	var modules []string
	extract := func(lines []string) ([]string, []string) {
		out := make([]string, len(lines))
		var found []string
		for i, line := range lines {
			trim := strings.TrimSpace(line)
			lower := strings.ToLower(trim)
			if strings.HasPrefix(trim, "#") {
				if rest, ok := strings.CutPrefix(lower, "#use"); ok {
					found = append(found, strings.Trim(strings.TrimSpace(rest), `"`))
				}
				continue
			}
			out[i] = line
		}
		return out, found
	}
	opts := parser.Options{File: filepath.Base(file), BaseDir: baseDir, ExtractUse: extract, OnModules: func(m []string) { modules = append(modules, m...) }}
	stmts, err := parser.ParseSource(source, opts)
	if err != nil {
		return nil, err
	}
	chunk, err := compiler.Compile(filepath.Base(file), stmts)
	if err != nil {
		return nil, err
	}
	chunk.Modules = modules
	return chunk, nil
}
func availableBytecodePath(source string) string {
	base := strings.TrimSuffix(source, filepath.Ext(source))
	dest := base + bytecode.Extension
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return dest
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, bytecode.Extension)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
func waitScriptWorker(pid int) bool {
	v, ok := workerProcesses.LoadAndDelete(pid)
	if !ok {
		return false
	}
	return v.(*exec.Cmd).Wait() == nil
}

func nullLike(a []value.Value) value.Value {
	if len(a) == 0 {
		return value.BoolVal(true)
	}
	switch a[0].Kind {
	case value.KindNull, value.KindNil, value.KindNone, value.KindUndefined:
		return value.BoolVal(true)
	}
	return value.BoolVal(false)
}
func strconvParse(s string) (float64, error) { return strconv.ParseFloat(s, 64) }

func formatBuiltin(v value.Value, style string) value.Value {
	if v.Kind != value.KindNum {
		return value.StrVal(value.ToString(v))
	}
	low := strings.ToLower(style)
	switch low {
	case "currency":
		return value.StrVal(fmt.Sprintf("%.2f €", v.Num))
	case "percent":
		return value.StrVal(fmt.Sprintf("%.2f%%", v.Num*100))
	case "hex":
		return value.StrVal(fmt.Sprintf("%X", int64(v.Num)))
	}
	if strings.ContainsAny(style, "0#.,") {
		dec := 0
		intDigits := 0
		if idx := strings.IndexAny(style, ".,"); idx >= 0 {
			dec = len(style) - idx - 1
			intDigits = strings.Count(style[:idx], "0")
		} else {
			intDigits = strings.Count(style, "0")
		}
		s := strconv.FormatFloat(v.Num, 'f', dec, 64)
		s = strings.ReplaceAll(s, ".", ",")
		negative := strings.HasPrefix(s, "-")
		if negative {
			s = s[1:]
		}
		parts := strings.SplitN(s, ",", 2)
		for len(parts[0]) < intDigits {
			parts[0] = "0" + parts[0]
		}
		s = parts[0]
		if len(parts) == 2 {
			s += "," + parts[1]
		}
		if negative {
			s = "-" + s
		}
		return value.StrVal(s)
	}
	if strings.Contains(style, "%") {
		return value.StrVal(fmt.Sprintf(style, v.Num))
	}
	return value.StrVal(value.ToString(v))
}

func clipboardWrite(s string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("clip.exe")
	case "darwin":
		cmd = exec.Command("pbcopy")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard", "-i")
	}
	cmd.Stdin = strings.NewReader(s)
	if err := cmd.Run(); err == nil {
		return nil
	} else if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return fmt.Errorf("Zwischenablage konnte nicht beschrieben werden: %w", err)
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		fallback := exec.Command("xsel", "--clipboard", "--input")
		fallback.Stdin = strings.NewReader(s)
		return fallback.Run()
	}
	return fmt.Errorf("Zwischenablage konnte nicht beschrieben werden")
}
func clipboardRead() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw")
	case "darwin":
		cmd = exec.Command("pbpaste")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	}
	b, err := cmd.Output()
	if err == nil {
		return strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r"), nil
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		b, err = exec.Command("xsel", "--clipboard", "--output").Output()
		if err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("Zwischenablage konnte nicht gelesen werden: %w", err)
}

func readPassword(prompt string) (string, error) {
	if runtime.GOOS == "windows" {
		script := "$s=Read-Host -Prompt $env:VBX_PROMPT -AsSecureString; $p=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($s); try {[Console]::Write([Runtime.InteropServices.Marshal]::PtrToStringBSTR($p))} finally {[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($p)}"
		cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", script)
		cmd.Env = append(os.Environ(), "VBX_PROMPT="+prompt)
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	if prompt != "" {
		fmt.Print(prompt)
	}
	stty := exec.Command("stty", "-echo")
	stty.Stdin = os.Stdin
	if err := stty.Run(); err != nil {
		return "", fmt.Errorf("Passworteingabe benötigt ein Terminal: %w", err)
	}
	defer func() {
		restore := exec.Command("stty", "echo")
		restore.Stdin = os.Stdin
		_ = restore.Run()
		fmt.Println()
	}()
	s, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimRight(s, "\r\n"), err
}
func punyDomain(domain string, decode bool) (string, error) {
	labels := strings.Split(domain, ".")
	for i, label := range labels {
		if decode {
			if strings.HasPrefix(strings.ToLower(label), "xn--") {
				r, e := punyDecode(label[4:])
				if e != nil {
					return "", e
				}
				labels[i] = string(r)
			}
		} else if !isASCII(label) {
			code, e := punyEncode([]rune(strings.ToLower(label)))
			if e != nil {
				return "", e
			}
			labels[i] = "xn--" + code
		}
	}
	return strings.Join(labels, "."), nil
}
func isASCII(s string) bool {
	for _, r := range s {
		if r >= 128 {
			return false
		}
	}
	return true
}
func punyAdapt(delta, numPoints int, first bool) int {
	if first {
		delta /= 700
	} else {
		delta /= 2
	}
	delta += delta / numPoints
	k := 0
	for delta > 455 {
		delta /= 35
		k += 36
	}
	return k + 36*delta/(delta+38)
}
func punyDigit(d int) byte {
	if d < 26 {
		return byte('a' + d)
	}
	return byte('0' + d - 26)
}
func punyValue(c rune) (int, bool) {
	if c >= 'a' && c <= 'z' {
		return int(c - 'a'), true
	}
	if c >= 'A' && c <= 'Z' {
		return int(c - 'A'), true
	}
	if c >= '0' && c <= '9' {
		return int(c-'0') + 26, true
	}
	return 0, false
}
func punyEncode(input []rune) (string, error) {
	const base = 36
	out := strings.Builder{}
	b := 0
	for _, r := range input {
		if r < 128 {
			out.WriteRune(r)
			b++
		}
	}
	h := b
	if b > 0 && h < len(input) {
		out.WriteByte('-')
	}
	n, delta, bias := 128, 0, 72
	for h < len(input) {
		m := int(^uint(0) >> 1)
		for _, r := range input {
			if int(r) >= n && int(r) < m {
				m = int(r)
			}
		}
		if m == int(^uint(0)>>1) || (m-n) > (int(^uint(0)>>1)-delta)/(h+1) {
			return "", fmt.Errorf("punycode overflow")
		}
		delta += (m - n) * (h + 1)
		n = m
		for _, r := range input {
			c := int(r)
			if c < n {
				delta++
				if delta < 0 {
					return "", fmt.Errorf("punycode overflow")
				}
			}
			if c == n {
				q := delta
				for k := base; ; k += base {
					t := 1
					if k > bias+26 {
						t = 26
					} else if k > bias {
						t = k - bias
					}
					if q < t {
						break
					}
					out.WriteByte(punyDigit(t + (q-t)%(base-t)))
					q = (q - t) / (base - t)
				}
				out.WriteByte(punyDigit(q))
				bias = punyAdapt(delta, h+1, h == b)
				delta = 0
				h++
			}
		}
		delta++
		n++
	}
	return out.String(), nil
}
func punyDecode(input string) ([]rune, error) {
	const base = 36
	out := []rune{}
	n, i, bias := 128, 0, 72
	d := strings.LastIndexByte(input, '-')
	if d >= 0 {
		for _, r := range input[:d] {
			if r >= 128 {
				return nil, fmt.Errorf("invalid basic punycode")
			}
			out = append(out, r)
		}
		input = input[d+1:]
	}
	for pos := 0; pos < len(input); {
		oldi, w := i, 1
		for k := base; ; k += base {
			if pos >= len(input) {
				return nil, fmt.Errorf("invalid punycode")
			}
			digit, ok := punyValue(rune(input[pos]))
			pos++
			if !ok || digit > (int(^uint(0)>>1)-i)/w {
				return nil, fmt.Errorf("invalid punycode")
			}
			i += digit * w
			t := 1
			if k > bias+26 {
				t = 26
			} else if k > bias {
				t = k - bias
			}
			if digit < t {
				break
			}
			if w > int(^uint(0)>>1)/(base-t) {
				return nil, fmt.Errorf("punycode overflow")
			}
			w *= base - t
		}
		bias = punyAdapt(i-oldi, len(out)+1, oldi == 0)
		n += i / (len(out) + 1)
		i %= len(out) + 1
		if n > utf8.MaxRune || n >= 0xD800 && n <= 0xDFFF {
			return nil, fmt.Errorf("invalid Unicode code point")
		}
		out = append(out, 0)
		copy(out[i+1:], out[i:])
		out[i] = rune(n)
		i++
	}
	return out, nil
}
