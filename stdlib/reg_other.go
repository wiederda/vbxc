//go:build !windows
// +build !windows

package stdlib

import valpkg "vbxc/value"

func initRegistryFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "reg."

	Register(ns+"Read", "reg", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})

	Register(ns+"Write", "reg", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})

	Register(ns+"Exists", "reg", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})

	Register(ns+"Delete", "reg", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})
	Register(ns+"WriteProtectedValue", "reg", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})
	Register(ns+"ReadProtectedValue", "reg", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})
	Register(ns+"ReadProtectedValueBytes", "reg", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})
}

func init() {
	initRegistryFunctions()
}
