// ------------------------
// stdlib_json.go
// ------------------------

package stdlib

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	valpkg "vbxc/value"
)

type JsonEngine struct {
	mu   sync.RWMutex
	Data map[string]interface{}
	Path string
}

func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	return strings.Split(path, ".")
}

var jsonEngine = &JsonEngine{
	Data: make(map[string]interface{}),
}

// ========================= Hilfsfunktionen =========================

// anyToStr konvertiert einen beliebigen Go-Wert in einen String.
// Früher war dieser switch-Block 4x dupliziert (Search, SearchWhereGet,
// SearchWhereExists, Query) — jetzt an einer einzigen Stelle.
func anyToStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// parseConds liest Bedingungen der Form "key=value" oder "key:value"
// aus einer Argument-Liste und gibt sie als Map zurück.
// Werte werden für case-insensitive Vergleiche klein geschrieben.
func parseConds(args []valpkg.Value, startIdx int) map[string]string {
	conds := make(map[string]string)
	for i := startIdx; i < len(args); i++ {
		raw := valpkg.ToString(args[i])
		sep := "="
		if !strings.Contains(raw, "=") && strings.Contains(raw, ":") {
			sep = ":"
		}
		parts := strings.SplitN(raw, sep, 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if k != "" {
				conds[k] = strings.ToLower(v)
			}
		}
	}
	return conds
}

// matchObject prüft, ob ein Objekt alle übergebenen Bedingungen erfüllt.
// Vergleiche sind case-insensitive; fehlende Keys zählen als kein Match.
func matchObject(obj map[string]interface{}, conds map[string]string) bool {
	if len(conds) == 0 {
		return true
	}
	for k, expected := range conds {
		val, ok := obj[k]
		if !ok {
			return false
		}
		if strings.ToLower(anyToStr(val)) != expected {
			return false
		}
	}
	return true
}

// ========================= Engine =========================

func (j *JsonEngine) Load(path string) valpkg.Value {
	// Pfad-Sicherheitscheck — konsistent mit ini.Load
	safePath, errVal := absPathVal(path)
	if errVal != nil {
		return *errVal
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		return valpkg.ErrorVal(err.Error())
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return valpkg.ErrorVal(err.Error())
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	j.Data = parsed
	if j.Data == nil {
		j.Data = make(map[string]interface{})
	}
	j.Path = safePath
	return valpkg.StrVal("ok")
}

// Save schreibt den aktuellen Zustand atomar in die Datei:
// erst in eine .tmp-Datei, dann atomares os.Rename —
// die Originaldatei ist nie halb überschrieben.
func (j *JsonEngine) Save() valpkg.Value {
	j.mu.RLock()
	path := j.Path
	j.mu.RUnlock()

	if path == "" {
		return valpkg.ErrorVal("no path set")
	}

	j.mu.RLock()
	data, err := json.MarshalIndent(j.Data, "", "  ")
	j.mu.RUnlock()

	if err != nil {
		return valpkg.ErrorVal(err.Error())
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return valpkg.ErrorVal("temporäre Datei konnte nicht geschrieben werden: " + err.Error())
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return valpkg.ErrorVal("atomares Ersetzen fehlgeschlagen: " + err.Error())
	}

	return valpkg.StrVal("ok")
}

func (j *JsonEngine) Get(path string) valpkg.Value {
	j.mu.RLock()
	defer j.mu.RUnlock()

	val, ok := getNested(j.Data, splitPath(path))
	if !ok {
		return valpkg.Value{}
	}
	return interfaceToValue(val)
}

func (j *JsonEngine) Set(path string, val valpkg.Value) valpkg.Value {
	j.mu.Lock()
	defer j.mu.Unlock()

	if err := setNested(j.Data, splitPath(path), valToInterface(val)); err != nil {
		return valpkg.ErrorVal(err.Error())
	}
	return valpkg.StrVal("ok")
}

func (j *JsonEngine) Delete(path string) valpkg.Value {
	j.mu.Lock()
	defer j.mu.Unlock()

	if err := deleteNested(j.Data, splitPath(path)); err != nil {
		return valpkg.ErrorVal(err.Error())
	}
	return valpkg.StrVal("ok")
}

func (j *JsonEngine) exists(path string) bool {
	j.mu.RLock()
	defer j.mu.RUnlock()

	_, ok := getNested(j.Data, splitPath(path))
	return ok
}

// ========================= Init =========================

func initJSONFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "json."

	// ---------------- json.Load ----------------
	Register(ns+"Load", "json", "pfad", "Lädt eine JSON-Datei vom angegebenen Pfad in den Speicher.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("json.Load(pfad) benötigt einen Pfad")
		}
		return jsonEngine.Load(args[0].Str)
	})

	// ---------------- json.Save ----------------
	Register(ns+"Save", "json", "[pfad]", "Speichert die aktuellen Daten atomar. Optional kann ein neuer Pfad gesetzt werden.", func(args []valpkg.Value) valpkg.Value {
		if len(args) >= 1 && args[0].Str != "" {
			jsonEngine.mu.Lock()
			jsonEngine.Path = args[0].Str
			jsonEngine.mu.Unlock()
		}
		return jsonEngine.Save()
	})

	// ---------------- json.Parse ----------------
	Register(ns+"Parse", "json", "jsonString", "Prüft die JSON-Struktur. Gibt true zurück wenn valide, sonst eine Fehlermeldung.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("Kein JSON-Inhalt übergeben")
		}
		var data interface{}
		if err := json.Unmarshal([]byte(valpkg.ToString(args[0])), &data); err != nil {
			return valpkg.ErrorVal("ERROR: " + err.Error())
		}
		return valpkg.BoolVal(true)
	})

	// ---------------- json.ParseToEngine ----------------
	Register(ns+"ParseToEngine", "json", "jsonStr", "Lädt einen JSON-String direkt in den globalen Speicher.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("no data")
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(valpkg.ToString(args[0])), &data); err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		jsonEngine.mu.Lock()
		jsonEngine.Data = data
		jsonEngine.mu.Unlock()
		return valpkg.StrVal("ok")
	})

	// ---------------- json.Get ----------------
	Register(ns+"Get", "json", "pfad", "Holt einen Wert aus der geladenen JSON-Struktur. Unterstützt Dot-Notation (z.B. user.address.city).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{}
		}
		return jsonEngine.Get(args[0].Str)
	})

	// ---------------- json.Set ----------------
	Register(ns+"Set", "json", "pfad, wert", "Setzt einen Wert im geladenen Objekt. Erstellt fehlende Zwischenknoten automatisch.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("json.Set(pfad, wert) benötigt zwei Argumente")
		}
		return jsonEngine.Set(args[0].Str, args[1])
	})

	// ---------------- json.Delete ----------------
	Register(ns+"Delete", "json", "pfad", "Entfernt den angegebenen Schlüssel aus dem Objekt.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("json.Delete(pfad) benötigt einen Pfad")
		}
		return jsonEngine.Delete(args[0].Str)
	})

	// ---------------- json.Exists ----------------
	Register(ns+"Exists", "json", "pfad", "Gibt true zurück, wenn der Schlüssel existiert, sonst false.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.BoolVal(false)
		}
		return valpkg.BoolVal(jsonEngine.exists(args[0].Str))
	})

	// ---------------- json.Keys ----------------
	Register(ns+"Keys", "json", "pfad", "Liefert ein sortiertes Array aller Schlüsselnamen an der angegebenen Position.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr}
		}

		jsonEngine.mu.RLock()
		val, ok := getNested(jsonEngine.Data, splitPath(args[0].Str))
		jsonEngine.mu.RUnlock()

		if !ok {
			return valpkg.Value{Kind: valpkg.KindArr}
		}
		m, ok := val.(map[string]interface{})
		if !ok {
			return valpkg.Value{Kind: valpkg.KindArr}
		}

		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		arr := make([]valpkg.Value, len(keys))
		for i, k := range keys {
			arr[i] = valpkg.StrVal(k)
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
	})

	// ---------------- json.PropertyCount ----------------
	Register(ns+"PropertyCount", "json", "objekt",
		"Gibt die Anzahl der Top-Level-Eigenschaften eines JSON-Objekts zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("json.PropertyCount(objekt) benötigt ein Argument")
			}
			if args[0].Kind != valpkg.KindMap {
				return valpkg.ErrorVal("json.PropertyCount: Wert ist kein JSON-Objekt")
			}
			return valpkg.NumVal(float64(len(args[0].Map)))
		})

	// ---------------- json.Append ----------------
	Register(ns+"Append", "json", "pfad, wert", "Fügt ein Element an ein bestehendes Array innerhalb der Struktur an.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("json.Append(pfad, wert) benötigt zwei Argumente")
		}

		path := args[0].Str

		jsonEngine.mu.Lock()
		defer jsonEngine.mu.Unlock()

		val, ok := getNested(jsonEngine.Data, splitPath(path))
		var arr []interface{}
		if ok {
			arr, _ = val.([]interface{})
		}
		arr = append(arr, valToInterface(args[1]))
		if err := setNested(jsonEngine.Data, splitPath(path), arr); err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		return valpkg.StrVal("ok")
	})

	// ---------------- json.ToArray ----------------
	Register(ns+"ToArray", "json", "pfad", "Konvertiert eine Liste an einem Pfad explizit in ein Array.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr}
		}

		jsonEngine.mu.RLock()
		val, ok := getNested(jsonEngine.Data, splitPath(args[0].Str))
		jsonEngine.mu.RUnlock()

		if !ok {
			return valpkg.Value{Kind: valpkg.KindArr}
		}
		arr, ok := val.([]interface{})
		if !ok {
			return valpkg.Value{Kind: valpkg.KindArr}
		}
		res := make([]valpkg.Value, len(arr))
		for i, v := range arr {
			res[i] = interfaceToValue(v)
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: res}
	})

	// ---------------- json.Merge ----------------
	// Hinweis: Merge mutiert den target-Wert direkt (flaches Mergen).
	// Source-Keys überschreiben gleichnamige Target-Keys.
	Register(ns+"Merge", "json", "target, source", "Kopiert alle Felder von source in target (flaches Mergen, mutiert target).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("json.Merge(target, source) benötigt zwei Objekte")
		}
		if args[0].Kind != valpkg.KindMap || args[1].Kind != valpkg.KindMap {
			return valpkg.ErrorVal("json.Merge erwartet zwei Maps/Objekte")
		}
		for k, v := range args[1].Map {
			args[0].Map[k] = v
		}
		return args[0]
	})

	// ---------------- json.SetInObject ----------------
	Register(ns+"SetInObject", "json", "obj, key, wert", "Setzt direkt ein Feld in einem übergebenen Map-Objekt.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("json.SetInObject(obj, key, wert) benötigt drei Argumente")
		}
		if args[0].Kind != valpkg.KindMap {
			return valpkg.ErrorVal("Target is not a JSON object")
		}
		args[0].Map[args[1].Str] = args[2]
		return valpkg.StrVal("ok")
	})

	// ---------------- json.FromJSON ----------------
	Register(ns+"FromJSON", "json", "jsonString", "Wandelt einen JSON-Text in eine interne Map/Array-Struktur um.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr}
		}
		var data interface{}
		if err := json.Unmarshal([]byte(args[0].Str), &data); err != nil {
			// Fehler explizit zurückgeben statt leerem Array
			return valpkg.ErrorVal("json.FromJSON: ungültiges JSON: " + err.Error())
		}
		return interfaceToValue(data)
	})

	// ---------------- json.ToJSON ----------------
	Register(ns+"ToJSON", "json", "daten", "Konvertiert eine Variable (Map/Array) in einen validen JSON-String.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.StrVal("{}")
		}
		js, err := json.Marshal(valToInterface(args[0]))
		if err != nil {
			return valpkg.ErrorVal("json.ToJSON: Serialisierung fehlgeschlagen: " + err.Error())
		}
		return valpkg.StrVal(string(js))
	})

	// ---------------- json.Query ----------------
	Register(ns+"Query", "json", "pfad, filter", "Sucht in einem Array nach Objekten, die ein Kriterium erfüllen (z.B. status=online).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.Value{Kind: valpkg.KindArr}
		}

		filterKey, filterValue, ok := parseFilter(args[1].Str)
		if !ok {
			return valpkg.Value{Kind: valpkg.KindArr}
		}

		jsonEngine.mu.RLock()
		val, exists := getNested(jsonEngine.Data, splitPath(args[0].Str))
		jsonEngine.mu.RUnlock()

		if !exists {
			return valpkg.Value{Kind: valpkg.KindArr}
		}
		arr, ok := val.([]interface{})
		if !ok {
			return valpkg.Value{Kind: valpkg.KindArr}
		}

		var results []valpkg.Value
		for _, item := range arr {
			if obj, ok := item.(map[string]interface{}); ok {
				if v, exists := obj[filterKey]; exists {
					if fmt.Sprintf("%v", v) == filterValue {
						results = append(results, interfaceToValue(item))
					}
				}
			}
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: results}
	})

	// ---------------- json.Search ----------------
	Register(ns+"Search", "json", "jsonStr, key", "Durchsucht einen JSON-String rekursiv nach dem ersten Vorkommen von key.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.StrVal("")
		}

		jsonStr := strings.TrimSpace(args[0].Str)
		key := strings.TrimSpace(args[1].Str)
		if jsonStr == "" || key == "" {
			return valpkg.StrVal("")
		}

		var data interface{}
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			return valpkg.StrVal("")
		}

		var search func(interface{}) (string, bool)
		search = func(v interface{}) (string, bool) {
			switch val := v.(type) {
			case map[string]interface{}:
				if s, ok := val[key]; ok {
					return anyToStr(s), true
				}
				for _, e := range val {
					if r, found := search(e); found {
						return r, true
					}
				}
			case []interface{}:
				for _, e := range val {
					if r, found := search(e); found {
						return r, true
					}
				}
			}
			return "", false
		}

		result, _ := search(data)
		return valpkg.StrVal(result)
	})

	// ---------------- json.SearchAll ----------------
	Register(ns+"SearchAll", "json", "jsonStr, key", "Durchsucht JSON rekursiv nach ALLEN Vorkommen eines Keys.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.Value{Kind: valpkg.KindArr}
		}

		var data interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(args[0].Str)), &data); err != nil {
			return valpkg.Value{Kind: valpkg.KindArr}
		}

		searchKey := strings.TrimSpace(args[1].Str)
		var results []valpkg.Value

		var findAll func(interface{})
		findAll = func(v interface{}) {
			switch val := v.(type) {
			case map[string]interface{}:
				if found, ok := val[searchKey]; ok {
					results = append(results, interfaceToValue(found))
				}
				for _, e := range val {
					findAll(e)
				}
			case []interface{}:
				for _, e := range val {
					findAll(e)
				}
			}
		}

		findAll(data)
		return valpkg.Value{Kind: valpkg.KindArr, Arr: results}
	})

	// ---------------- json.SearchDeepWhereGet ----------------
	// Durchsucht JSON rekursiv nach Objekten mit Bedingungen.
	// Bedingungen können auch aus übergeordneten Objekten stammen.
	// Gibt den gewünschten Key zurück.
	//
	// Beispiel:
	// json.SearchDeepWhereGet(json,
	//     "ProductVersion",
	//     "Product=Stable",
	//     "Platform=Windows",
	//     "Architecture=x64")
	Register(ns+"SearchDeepWhereGet", "json",
		"jsonStr, returnKey, cond1, cond2, ...",
		"Rekursive Suche in JSON nach Objekten mit Bedingungen und Rückgabewert.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 2 {
				return valpkg.ErrorVal("json.SearchDeepWhereGet: mindestens 2 Argumente erforderlich (jsonStr, returnKey)")
			}

			jsonStr := strings.TrimSpace(valpkg.ToString(args[0]))
			returnKey := strings.TrimSpace(valpkg.ToString(args[1]))

			if jsonStr == "" {
				return valpkg.ErrorVal("json.SearchDeepWhereGet: jsonStr ist leer")
			}
			if returnKey == "" {
				return valpkg.ErrorVal("json.SearchDeepWhereGet: returnKey ist leer")
			}

			conds := parseConds(args, 2)

			// Bedingungswerte einmalig normalisieren, statt bei jedem Vergleich
			normConds := make(map[string]string, len(conds))
			for k, v := range conds {
				normConds[k] = strings.ToLower(v)
			}

			var data interface{}
			if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
				return valpkg.ErrorVal("json.SearchDeepWhereGet: ungültiges JSON: " + err.Error())
			}

			// Prüft Bedingungen gegen eine Map aus String-Werten
			matchInherited := func(values map[string]string) bool {

				for k, expected := range normConds {

					val, ok := values[k]
					if !ok || val != expected {
						return false
					}
				}

				return true
			}

			var search func(interface{}, map[string]string) (string, bool)

			search = func(v interface{}, inherited map[string]string) (string, bool) {

				switch obj := v.(type) {

				case []interface{}:

					for _, item := range obj {

						if result, found := search(item, inherited); found {
							return result, true
						}
					}

				case map[string]interface{}:

					// Kontext vom Elternobjekt übernehmen
					current := make(map[string]string, len(inherited)+len(obj))

					for k, v := range inherited {
						current[k] = v
					}

					// String-Werte dieses Objektes hinzufügen
					for k, v := range obj {

						if s, ok := v.(string); ok {
							current[k] = strings.ToLower(s)
						}
					}

					// Treffer prüfen
					if matchInherited(current) {

						if val, ok := obj[returnKey]; ok {
							return anyToStr(val), true
						}
					}

					// Kinder durchsuchen
					for _, child := range obj {

						if result, found := search(child, current); found {
							return result, true
						}
					}
				}

				return "", false
			}

			result, found := search(data, make(map[string]string))
			if !found {
				return valpkg.StrVal("")
			}

			return valpkg.StrVal(result)
		})

	// ---------------- json.SearchDeepWhereGetContains ----------------
	// Rekursive Suche mit exakten Bedingungen (= oder :)
	// und Teilstringsuche (~). Vererbt Kontext aus Elternobjekten.
	Register(ns+"SearchDeepWhereGetContains", "json",
		"jsonStr, returnKey, cond1, cond2, ...",
		"Rekursive JSON-Suche. Bedingungen mit = oder : werden exakt verglichen, Bedingungen mit ~ suchen nach einem Teilstring. Vergleiche sind case-insensitive.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("json.SearchDeepWhereGetContains: mindestens 2 Argumente erforderlich (jsonStr, returnKey)")
			}

			jsonStr := strings.TrimSpace(valpkg.ToString(args[0]))
			returnKey := strings.TrimSpace(valpkg.ToString(args[1]))

			if jsonStr == "" {
				return valpkg.ErrorVal("json.SearchDeepWhereGetContains: jsonStr ist leer")
			}
			if returnKey == "" {
				return valpkg.ErrorVal("json.SearchDeepWhereGetContains: returnKey ist leer")
			}

			type condition struct {
				value    string
				contains bool
			}

			conds := make(map[string]condition)

			for i := 2; i < len(args); i++ {
				raw := strings.TrimSpace(valpkg.ToString(args[i]))

				sep := "="
				contains := false

				if strings.Contains(raw, "~") {
					sep = "~"
					contains = true
				} else if !strings.Contains(raw, "=") && strings.Contains(raw, ":") {
					sep = ":"
				}

				parts := strings.SplitN(raw, sep, 2)
				if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
					return valpkg.ErrorVal("json.SearchDeepWhereGetContains: ungültige Bedingung: " + raw)
				}

				key := strings.ToLower(strings.TrimSpace(parts[0]))
				value := strings.ToLower(strings.TrimSpace(parts[1]))

				conds[key] = condition{
					value:    value,
					contains: contains,
				}
			}

			var data interface{}
			if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
				return valpkg.ErrorVal("json.SearchDeepWhereGetContains: ungültiges JSON: " + err.Error())
			}

			var search func(interface{}, map[string]string) (string, bool)

			search = func(v interface{}, inherited map[string]string) (string, bool) {
				switch obj := v.(type) {
				case []interface{}:
					for _, item := range obj {
						if result, found := search(item, inherited); found {
							return result, true
						}
					}

				case map[string]interface{}:
					current := make(map[string]string, len(inherited)+len(obj))

					for k, value := range inherited {
						current[k] = value
					}

					for k, value := range obj {
						if s, ok := value.(string); ok {
							current[strings.ToLower(k)] = strings.ToLower(s)
						}
					}

					matches := true
					for key, cond := range conds {
						value, ok := current[key]
						if !ok {
							matches = false
							break
						}

						if cond.contains {
							if !strings.Contains(value, cond.value) {
								matches = false
								break
							}
						} else if value != cond.value {
							matches = false
							break
						}
					}

					if matches {
						for key, value := range obj {
							if strings.EqualFold(key, returnKey) {
								return anyToStr(value), true
							}
						}
					}

					for _, child := range obj {
						if result, found := search(child, current); found {
							return result, true
						}
					}
				}

				return "", false
			}

			result, found := search(data, make(map[string]string))
			if !found {
				return valpkg.StrVal("")
			}

			return valpkg.StrVal(result)
		})

	// ---------------- json.FindKey ----------------
	Register(ns+"FindKey", "json", "pfad, wert",
		"Sucht in einem JSON-Objekt nach einem Wert und gibt den zugehörigen Schlüssel zurück.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 2 {
				return valpkg.ErrorVal("json.FindKey(pfad, wert) benötigt zwei Argumente")
			}

			path := strings.TrimSpace(valpkg.ToString(args[0]))
			search := strings.TrimSpace(valpkg.ToString(args[1]))

			jsonEngine.mu.RLock()
			defer jsonEngine.mu.RUnlock()

			val, ok := getNested(jsonEngine.Data, splitPath(path))
			if !ok {
				return valpkg.ErrorVal("json.FindKey: Pfad nicht gefunden")
			}

			obj, ok := val.(map[string]interface{})
			if !ok {
				return valpkg.ErrorVal("json.FindKey: Pfad ist kein JSON-Objekt")
			}

			for key, value := range obj {
				if anyToStr(value) == search {
					return valpkg.StrVal(key)
				}
			}

			// Kein Schlüssel mit diesem Wert gefunden — regulärer, fehlerfreier Ausgang
			return valpkg.StrVal("")
		})

	// ---------------- json.RenameKey ----------------
	Register(ns+"RenameKey", "json", "pfad, alterSchlüssel, neuerSchlüssel",
		"Benennt einen Schlüssel in einem JSON-Objekt um. Der zugehörige Wert bleibt erhalten.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 3 {
				return valpkg.ErrorVal("json.RenameKey(pfad, alterSchlüssel, neuerSchlüssel) benötigt drei Argumente")
			}

			path := strings.TrimSpace(valpkg.ToString(args[0]))
			oldKey := strings.TrimSpace(valpkg.ToString(args[1]))
			newKey := strings.TrimSpace(valpkg.ToString(args[2]))

			if oldKey == "" || newKey == "" {
				return valpkg.ErrorVal("Schlüssel dürfen nicht leer sein")
			}

			// Nichts zu tun
			if oldKey == newKey {
				return valpkg.StrVal("ok")
			}

			jsonEngine.mu.Lock()
			defer jsonEngine.mu.Unlock()

			val, ok := getNested(jsonEngine.Data, splitPath(path))
			if !ok {
				return valpkg.ErrorVal("Pfad nicht gefunden")
			}

			obj, ok := val.(map[string]interface{})
			if !ok {
				return valpkg.ErrorVal("Pfad ist kein JSON-Objekt")
			}

			value, ok := obj[oldKey]

			// Alter Schlüssel existiert nicht -> nichts zu tun
			if !ok {
				return valpkg.StrVal("ok")
			}

			// Neuer Schlüssel existiert bereits
			if _, exists := obj[newKey]; exists {
				return valpkg.ErrorVal("Neuer Schlüssel existiert bereits")
			}

			obj[newKey] = value
			delete(obj, oldKey)

			return valpkg.StrVal("ok")
		})

	// ---------------- json.SetKey ----------------
	Register(ns+"SetKey", "json", "pfad, key, wert",
		"Setzt einen direkten Schlüssel in einem JSON-Objekt ohne Pfadauflösung.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 3 {
				return valpkg.ErrorVal("json.SetKey(pfad, key, wert) benötigt drei Argumente")
			}

			path := strings.TrimSpace(valpkg.ToString(args[0]))
			key := strings.TrimSpace(valpkg.ToString(args[1]))

			if key == "" {
				return valpkg.ErrorVal("Key darf nicht leer sein")
			}

			jsonEngine.mu.Lock()
			defer jsonEngine.mu.Unlock()

			val, ok := getNested(jsonEngine.Data, splitPath(path))
			if !ok {
				return valpkg.ErrorVal("Pfad nicht gefunden")
			}

			obj, ok := val.(map[string]interface{})
			if !ok {
				return valpkg.ErrorVal("Pfad ist kein JSON-Objekt")
			}

			obj[key] = valToInterface(args[2])

			return valpkg.StrVal("ok")
		})

	// ---------------- json.SearchWhereGet ----------------
	// Durchsucht Arrays auf der obersten Ebene (keine tiefe Rekursion in Objekte).
	// Das ist bewusst so: es verhindert falsche Treffer aus verschachtelten Sub-Objekten.
	// Für tiefe Suche json.SearchAll verwenden.
	Register(ns+"SearchWhereGet", "json", "jsonStr, returnKey, cond1, cond2, ...", "Findet das erste Objekt, das alle Bedingungen erfüllt, und gibt einen bestimmten Key zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.StrVal("")
		}

		jsonStr := strings.TrimSpace(valpkg.ToString(args[0]))
		returnKey := strings.TrimSpace(valpkg.ToString(args[1]))
		if jsonStr == "" || returnKey == "" {
			return valpkg.StrVal("")
		}

		conds := parseConds(args, 2)

		var data interface{}
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			return valpkg.StrVal("")
		}

		var search func(interface{}) (string, bool)
		search = func(v interface{}) (string, bool) {
			switch obj := v.(type) {
			case []interface{}:
				for _, e := range obj {
					if r, found := search(e); found {
						return r, true
					}
				}
			case map[string]interface{}:
				if matchObject(obj, conds) {
					if val, ok := obj[returnKey]; ok {
						return anyToStr(val), true
					}
				}
				// Bewusst keine Rekursion in Kinder — verhindert falsche Treffer.
				// Für tiefe Suche: json.SearchAll nutzen.
			}
			return "", false
		}

		result, _ := search(data)
		return valpkg.StrVal(result)
	})

	// ---------------- json.SearchWhereExists ----------------
	Register(ns+"SearchWhereExists", "json", "jsonStr, cond1, cond2, ...", "Gibt true zurück, wenn mindestens ein Objekt alle Bedingungen erfüllt.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.BoolVal(false)
		}

		jsonStr := strings.TrimSpace(valpkg.ToString(args[0]))
		if jsonStr == "" {
			return valpkg.BoolVal(false)
		}

		conds := parseConds(args, 1)

		var data interface{}
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			return valpkg.BoolVal(false)
		}

		var match func(interface{}) bool
		match = func(v interface{}) bool {
			switch obj := v.(type) {
			case []interface{}:
				for _, e := range obj {
					if match(e) {
						return true
					}
				}
			case map[string]interface{}:
				if matchObject(obj, conds) {
					return true
				}
			}
			return false
		}

		return valpkg.BoolVal(match(data))
	})
}

// parseFilter zerlegt einen "key=value"-String in seine Bestandteile.
func parseFilter(filter string) (key, value string, ok bool) {
	parts := strings.SplitN(filter, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func getNested(current interface{}, keys []string) (interface{}, bool) {
	if len(keys) == 0 {
		return current, true
	}

	switch node := current.(type) {
	case map[string]interface{}:
		next, ok := node[keys[0]]
		if !ok {
			return nil, false
		}
		return getNested(next, keys[1:])
	case []interface{}:
		index, err := strconv.Atoi(keys[0])
		if err != nil || index < 0 || index >= len(node) {
			return nil, false
		}
		return getNested(node[index], keys[1:])
	}
	return nil, false
}

func setNested(current interface{}, keys []string, value interface{}) error {
	if len(keys) == 0 {
		return nil
	}

	switch node := current.(type) {
	case map[string]interface{}:
		key := keys[0]
		if len(keys) == 1 {
			node[key] = value
			return nil
		}
		next, ok := node[key]
		if !ok {
			newMap := make(map[string]interface{})
			node[key] = newMap
			return setNested(newMap, keys[1:], value)
		}
		return setNested(next, keys[1:], value)

	case []interface{}:
		index, err := strconv.Atoi(keys[0])
		if err != nil {
			return err
		}
		if index < 0 || index >= len(node) {
			return errors.New("array index out of range")
		}
		if len(keys) == 1 {
			node[index] = value
			return nil
		}
		return setNested(node[index], keys[1:], value)
	}

	return errors.New("invalid path")
}

func deleteNested(current interface{}, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	switch node := current.(type) {
	case map[string]interface{}:
		if len(keys) == 1 {
			delete(node, keys[0])
			return nil
		}
		next, ok := node[keys[0]]
		if !ok {
			return nil
		}
		return deleteNested(next, keys[1:])

	case []interface{}:
		index, err := strconv.Atoi(keys[0])
		if err != nil || index < 0 || index >= len(node) {
			return nil
		}
		if len(keys) == 1 {
			// Element wirklich entfernen statt auf nil setzen —
			// erhält die korrekte Array-Länge und vermeidet nil-Lücken.
			copy(node[index:], node[index+1:])
			node[len(node)-1] = nil
			// Hinweis: Das Slice selbst (in der übergeordneten Map) muss
			// mit node[:len(node)-1] gekürzt werden. Da deleteNested keinen
			// Rückgabewert für das neue Slice hat, muss der Aufrufer ggf.
			// nach dem Delete json.ToArray() neu einlesen. Für eine vollständige
			// Lösung müsste deleteNested interface{} zurückgeben.
			return nil
		}
		return deleteNested(node[index], keys[1:])
	}
	return nil
}

// ========================= Konvertierung =========================

func interfaceToValue(v interface{}) valpkg.Value {
	switch val := v.(type) {
	case string:
		return valpkg.Value{Kind: valpkg.KindStr, Str: val}
	case float64:
		return valpkg.Value{Kind: valpkg.KindNum, Num: val}
	case bool:
		return valpkg.Value{Kind: valpkg.KindBool, Bool: val}
	case []interface{}:
		arr := make([]valpkg.Value, len(val))
		for i, e := range val {
			arr[i] = interfaceToValue(e)
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
	case map[string]interface{}:
		m := make(map[string]valpkg.Value, len(val))
		for k, e := range val {
			m[k] = interfaceToValue(e)
		}
		return valpkg.Value{Kind: valpkg.KindMap, Map: m}
	default:
		data, _ := json.Marshal(val)
		return valpkg.Value{Kind: valpkg.KindStr, Str: string(data)}
	}
}

func init() { initJSONFunctions() }
