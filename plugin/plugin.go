// Package plugin bindet WASM-Plugins an vbxc.
//
// Ein Plugin ist ein WASM-Modul (Datei plugins/<namespace>.wasm neben der
// ausführbaren Datei, überschreibbar via VBX_PLUGIN_PATH) mit den Exports:
//
//	vbx_abi_version() -> i32   (muss 1 liefern)
//	alloc(len i32) -> i32      (Speicher für Host-Antworten)
//	dealloc(ptr i32, len i32)  (Freigabe)
//	vbx_call(namePtr, nameLen, argsPtr, argsLen) -> ptrLen
//	vbx_describe() -> ptrLen   (JSON-Liste der Funktionen)
//
// Registrierte Funktionen erscheinen im VBX-Skript als "namespace.name"
// über die normale stdlib-Registry; Argumente und Rückgaben werden als
// JSON über die Plugin-Grenze gereicht (Kind-Zuordnung in wasm.go).
//
// Die Bindung passiert beim direkten Interpretieren eines .vb-Skripts
// über den Evaluator ("vbxc skript.vb") und beim Ausführen von Bytecode:
// Chunk.Modules (aus "#use" beim Kompilieren) lädt vm.Run am Anfang von
// Run. #use eines nicht-eingebauten Namens lädt plugins/<name>.wasm.
//
// Diagnose: Umgebungsvariable VBX_PLUGIN_DEBUG (jeder Wert außer "" und
// "0") schaltet Plugin-Debug-Ausgaben auf stdout ein.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vbxc/stdlib"
)

// pluginDebug schaltet Diagnose-Ausgaben auf stdout ein. Im Original war
// das eine Compile-Zeit-Konstante; vbxc liest stattdessen die
// Umgebungsvariable VBX_PLUGIN_DEBUG.
var pluginDebug = func() bool {
	v := os.Getenv("VBX_PLUGIN_DEBUG")
	return v != "" && v != "0"
}()

func pluginDebugf(format string, args ...interface{}) {
	if pluginDebug {
		fmt.Printf("[PLUGIN DEBUG] "+format+"\n", args...)
	}
}

// validateNamespace verhindert Pfad-Escape über "#use"-Angaben
// (z. B. "../evil" oder "a/b"): erlaubt sind nur Buchstaben, Ziffern und
// Unterstrich/Bindestrich. Der Name ist vorher klein geschrieben.
func validateNamespace(ns string) error {
	if ns == "" {
		return fmt.Errorf("leerer Modul-/Plugin-Name")
	}
	for _, r := range ns {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return fmt.Errorf(
				"ungültiger Plugin-Name %q: nur Buchstaben, Ziffern, '_' und '-' erlaubt",
				ns,
			)
		}
	}
	return nil
}

// builtinNamespaces liefert die kleingeschriebenen Namespace-Namen aller in
// der stdlib-Registry registrierten Module (beim Aufruf — Plugin-Funktionen
// erscheinen dort ebenfalls wieder, sobald sie registriert sind).
func builtinNamespaces() map[string]bool {
	set := make(map[string]bool)
	for _, info := range stdlib.Builtins() {
		if info.Module != "" {
			set[strings.ToLower(info.Module)] = true
		}
	}
	return set
}

// IsBuiltinNamespace: true, wenn es bereits ein eingebautes Modul (oder ein
// geladenes Plugin) mit diesem Namespace gibt.
func IsBuiltinNamespace(ns string) bool {
	return builtinNamespaces()[strings.ToLower(strings.TrimSpace(ns))]
}

// PluginPath liefert den Pfad, unter dem Load das Plugin <ns>.wasm sucht.
func PluginPath(ns string) (string, error) {
	dir, err := pluginDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ns+".wasm"), nil
}

// LoadModules lädt eine "#use"-Modulliste (z. B. Chunk.Modules eines
// Bytecode-Chunks oder die "#use"-Zeilen eines direkt interpretierten
// Skripts). Eingebaute Namespaces werden übersprungen (Alle Builtins in
// vbxc sind statisch über init() registriert); unbekannte Namen müssen
// als WASM-Plugin verfügbar sein. Doppelte Namen werden dedupliziert,
// jede Fehlermeldung nennt das betreffende Modul.
func LoadModules(names []string) error {
	if len(names) == 0 {
		return nil
	}
	builtins := builtinNamespaces()
	seen := make(map[string]bool, len(names))
	for _, group := range names {
		for _, mod := range strings.Split(group, ",") {
			mod = strings.ToLower(strings.Trim(strings.TrimSpace(mod), `"'`))
			if mod == "" || seen[mod] || builtins[mod] {
				continue
			}
			seen[mod] = true
			if err := Load(mod); err != nil {
				return err
			}
		}
	}
	return nil
}

// FindMissing prüft ohne Ladeversuch, für welchen der angegebenen
// "#use"-Namen kein Plugin-WASM existiert (und es kein eingebautes Modul
// ist). Für Fehlerdiagnostik bereits beim Kompilieren (vbxc build) —
// Build-Maschine und Ausführmaschine können unterschiedliche
// Plugin-Verzeichnisse haben.
func FindMissing(names []string) []string {
	var missing []string
	builtins := builtinNamespaces()
	seen := make(map[string]bool, len(names))
	for _, group := range names {
		for _, mod := range strings.Split(group, ",") {
			mod = strings.ToLower(strings.Trim(strings.TrimSpace(mod), `"'`))
			if mod == "" || seen[mod] || builtins[mod] {
				continue
			}
			seen[mod] = true
			if err := validateNamespace(mod); err != nil {
				missing = append(missing, mod)
				continue
			}
			path, err := PluginPath(mod)
			if err != nil {
				missing = append(missing, mod)
				continue
			}
			if _, err := os.Stat(path); err != nil {
				missing = append(missing, mod)
			}
		}
	}
	return missing
}
