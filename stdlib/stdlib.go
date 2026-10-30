// Package stdlib enthält die Builtin-Registry. Signatur und Felder entsprechen
// BuiltinInfo aus dem Interpreter, damit die ~500 Funktionen später ohne
// Änderung übernommen werden können.
package stdlib

import (
	"strings"

	"vbxc/value"
)

type BuiltinInfo struct {
	Fn           func([]value.Value) value.Value
	Beschreibung string
	Params       string
	Module       string
	ParamCount   int
	Deprecated   string
	Category     string
}

var builtins = make(map[string]BuiltinInfo)

// registryGen zählt Änderungen an der Registry. Der Evaluator snapshottet
// Builtins zum Package-Start (Performance); über die Generation erfährt er,
// dass sich seitdem etwas geändert hat (Plugin-Ladung) und muss neu
// lesen. Das hält Register trotzdem billig: init()-Zeit zählt einmal.
var registryGen uint64

// Generation liefert den aktuellen Registry-Stand (für Snapshots).
func Generation() uint64 {
	return registryGen
}

// Register legt eine Funktion an. Mit Namespace lautet der Aufrufname
// "namespace.name", ohne (leer) einfach "name".
func Register(name, namespace, params, description string, fn func([]value.Value) value.Value) {
	key := name
	if namespace != "" && namespace != "global" {
		if namespace == "const" || strings.Contains(name, ".") {
			key = name
		} else {
			key = namespace + "." + name
		}
	}
	paramCount := 0
	p := strings.TrimSpace(params)
	if p != "" && p != "-" {
		if strings.Contains(p, "[") || strings.Contains(p, "...") {
			paramCount = -1
		} else {
			paramCount = len(strings.Split(p, ","))
		}
	}
	builtins[key] = BuiltinInfo{Fn: fn, Beschreibung: description, Params: params, Module: namespace, ParamCount: paramCount}
	registryGen++ // Snapshots (Evaluator) müssen neu aufgebaut werden
}

// RegisterConstant legt eine Konstante mit ihrer Hilfe-Kategorie an.
func RegisterConstant(category, name, description string, fn func([]value.Value) value.Value) {
	Register(name, "const", "", description, fn)
	info := builtins[name]
	info.Category = category
	builtins[name] = info
}

func Lookup(name string) (BuiltinInfo, bool) {
	info, ok := builtins[name]
	return info, ok
}

// Builtins returns a snapshot of all registered built-ins for CLI help and tools.
func Builtins() map[string]BuiltinInfo {
	result := make(map[string]BuiltinInfo, len(builtins))
	for name, info := range builtins {
		result[name] = info
	}
	return result
}

// FindByModuleAndName: Gegenstück zu findBuiltinByModuleAndName (nur für die
// Fehlermeldung "muss mit '()' aufgerufen werden").
func FindByModuleAndName(module, name string) (BuiltinInfo, bool) {
	if info, ok := builtins[module+"."+name]; ok {
		return info, true
	}
	want := strings.ToLower(module) + "." + strings.ToLower(name)
	for key, info := range builtins {
		if info.Module != "" && strings.ToLower(key) == want {
			return info, true
		}
	}
	return BuiltinInfo{}, false
}
