// ------------------------
// stdlib_map.go
// ------------------------

package stdlib

import (
	"fmt"
	"vbxc/value"
)

// InitMapFunctions registriert Map-Funktionen
func initMapFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "map."

	// ------------------------
	// Create
	// ------------------------
	Register(ns+"Create", "map", "", "Erstellt eine leere Map", func(args []value.Value) value.Value {
		return value.Value{
			Kind: value.KindMap,
			Map:  make(map[string]value.Value),
		}
	})

	// ------------------------
	// Set (mutierend)
	// ------------------------
	Register(ns+"Set", "map", "map, key, value", "Setzt einen Wert in der Map (In-Place).", func(args []value.Value) value.Value {
		if len(args) < 3 || args[0].Kind != value.KindMap {
			return args[0]
		}

		m := args[0].Map
		key := value.ToString(args[1])

		m[key] = args[2]
		return args[0]
	})

	// ------------------------
	// Get
	// ------------------------
	Register(ns+"Get", "map", "map, key", "Liest einen Wert aus der Map", func(args []value.Value) value.Value {
		if len(args) < 2 || args[0].Kind != value.KindMap {
			return value.NullVal()
		}

		m := args[0].Map
		key := value.ToString(args[1])

		v, ok := m[key]
		if !ok {
			return value.NullVal()
		}

		return v
	})

	// ------------------------
	// ContainsKey
	// ------------------------
	Register(ns+"ContainsKey", "map", "map, key", "Prüft, ob ein Schlüssel existiert", func(args []value.Value) value.Value {
		if len(args) < 2 || args[0].Kind != value.KindMap {
			return value.BoolVal(false)
		}

		m := args[0].Map
		key := value.ToString(args[1])

		_, ok := m[key]
		return value.BoolVal(ok)
	})

	// ------------------------
	// Remove
	// ------------------------
	Register(ns+"Remove", "map", "map, key", "Entfernt einen Schlüssel aus der Map", func(args []value.Value) value.Value {
		if len(args) < 2 || args[0].Kind != value.KindMap {
			return args[0]
		}

		m := args[0].Map
		key := value.ToString(args[1])

		delete(m, key)
		return args[0]
	})

	// ------------------------
	// Keys
	// ------------------------
	Register(ns+"Keys", "map", "map", "Gibt alle Schlüssel als Array zurück", func(args []value.Value) value.Value {
		if len(args) < 1 || args[0].Kind != value.KindMap {
			return value.Value{Kind: value.KindArr, Arr: []value.Value{}}
		}

		m := args[0].Map
		keys := make([]value.Value, 0, len(m))

		for k := range m {
			keys = append(keys, value.StrVal(k))
		}

		return value.Value{Kind: value.KindArr, Arr: keys}
	})

	// ------------------------
	// Values
	// ------------------------
	Register(ns+"Values", "map", "map", "Gibt alle Werte als Array zurück", func(args []value.Value) value.Value {
		if len(args) < 1 || args[0].Kind != value.KindMap {
			return value.Value{Kind: value.KindArr, Arr: []value.Value{}}
		}

		m := args[0].Map
		values := make([]value.Value, 0, len(m))

		for _, v := range m {
			values = append(values, v)
		}

		return value.Value{Kind: value.KindArr, Arr: values}
	})

	// ------------------------
	// Count
	// ------------------------
	Register(ns+"Count", "map", "map", "Anzahl der Einträge in der Map", func(args []value.Value) value.Value {
		if len(args) < 1 || args[0].Kind != value.KindMap {
			return value.NumVal(0)
		}

		return value.NumVal(float64(len(args[0].Map)))
	})

	// ------------------------
	// Clear
	// ------------------------
	Register(ns+"Clear", "map", "map", "Leert die Map (mutierend)", func(args []value.Value) value.Value {
		if len(args) < 1 || args[0].Kind != value.KindMap {
			return args[0]
		}

		m := args[0].Map
		for k := range m {
			delete(m, k)
		}

		return args[0]
	})

	// ------------------------
	// Clone
	// ------------------------
	Register(ns+"Clone", "map", "map", "Erstellt eine Kopie der Map (shallow copy)", func(args []value.Value) value.Value {
		if len(args) < 1 || args[0].Kind != value.KindMap {
			return value.Value{}
		}

		src := args[0].Map
		dst := make(map[string]value.Value, len(src))

		for k, v := range src {
			dst[k] = v
		}

		return value.Value{
			Kind: value.KindMap,
			Map:  dst,
		}
	})

	// ------------------------
	// ToString (Debug)
	// ------------------------
	Register(ns+"ToString", "map", "map", "Gibt eine einfache String-Darstellung zurück", func(args []value.Value) value.Value {
		if len(args) < 1 || args[0].Kind != value.KindMap {
			return value.StrVal("{}")
		}

		m := args[0].Map
		out := "{"

		first := true
		for k, v := range m {
			if !first {
				out += ", "
			}
			first = false

			out += fmt.Sprintf("%s: %s", k, value.ToString(v))
		}

		out += "}"

		return value.StrVal(out)
	})
}

func init() {
	initMapFunctions()
}
