//go:build !windows
// +build !windows

package stdlib

import valpkg "vbxc/value"

func initRegistryFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "reg."

	Register(ns+"Read", "", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})

	Register(ns+"Write", "", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})

	Register(ns+"Exists", "", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})

	Register(ns+"Delete", "", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})
	Register(ns+"WriteProtectedValue", "", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})
	Register(ns+"ReadProtectedValue", "", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})
	Register(ns+"ReadProtectedValueBytes", "", "", "", func(args []valpkg.Value) valpkg.Value {
		return valpkg.StrVal("error: registry not available on this OS")
	})
}

func init() {
	initRegistryFunctions()
}
