package stdlib

import (
	"os"
	valpkg "vbxc/value"
)

func initEnvFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "env."

	Register(ns+"Get", "env", "key", "Liest eine Umgebungsvariable (z.B. 'PATH' oder 'TEMP') aus.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("env.Get(key) benötigt einen Namen")
		}
		key := args[0].Str
		val := os.Getenv(key)
		return valpkg.StrVal(val)
	})

	Register(ns+"Set", "env", "key, value", "Setzt eine Umgebungsvariable für den aktuellen Prozess. Gibt True bei Erfolg zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("env.Set(key, value) benötigt zwei Argumente")
		}
		err := os.Setenv(args[0].Str, args[1].Str)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Setzen der Env-Variable: " + err.Error())
		}
		return valpkg.BoolVal(true)
	})

	Register(ns+"All", "env", "", "Gibt alle verfügbaren Umgebungsvariablen als Array im Format 'KEY=VALUE' zurück.", func(args []valpkg.Value) valpkg.Value {
		vars := os.Environ()
		return valpkg.Value{Kind: valpkg.KindArr, Arr: stringSliceToValueSlice(vars)}
	})
}

func init() { initEnvFunctions() }
