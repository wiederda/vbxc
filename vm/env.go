package vm

import (
	"fmt"
	"os"

	"vbxc/jit"
	"vbxc/value"
)

// Environment ist der Scope aus dem Interpreter (Phase 1: nur die Methoden,
// die die VM braucht). Semantik und Meldungen sind unverändert.
type Environment struct {
	vars   map[string]*value.Value
	consts map[string]bool
	parent *Environment

	fnReturn        value.Value // Rückgabewert der laufenden Function/Sub
	currentFuncName string      // Name der laufenden Function ("" sonst)
}

func NewEnvironment(parent *Environment) *Environment {
	return &Environment{vars: make(map[string]*value.Value), parent: parent}
}

func NewEnvironmentWithCapacity(parent *Environment, capacity int) *Environment {
	return &Environment{vars: make(map[string]*value.Value, capacity), parent: parent}
}

func (e *Environment) root() *Environment {
	for e.parent != nil {
		e = e.parent
	}
	return e
}

// PutInternal setzt/legt einen Wert im aktuellen Scope an (Parameterübergabe).
func (e *Environment) PutInternal(name string, val value.Value) {
	if ptr, ok := e.vars[name]; ok {
		*ptr = val
		return
	}
	v := val
	e.vars[name] = &v
}

func (e *Environment) Get(name string) (value.Value, bool) {
	for env := e; env != nil; env = env.parent {
		if ptr, ok := env.vars[name]; ok {
			return *ptr, true
		}
	}
	return value.NilValue, false
}

func (e *Environment) GetRefStrictEnv(name string) (*Environment, *value.Value, bool) {
	for env := e; env != nil; env = env.parent {
		if ptr, ok := env.vars[name]; ok {
			return env, ptr, true
		}
	}
	return nil, nil, false
}

// GetRef sucht nur im aktuellen Scope und legt bei Bedarf neu an
// (für For-Schleifenvariablen).
func (e *Environment) GetRef(name string) *value.Value {
	if ptr, ok := e.vars[name]; ok {
		return ptr
	}
	newVal := &value.Value{Kind: value.KindNum, Num: 0}
	e.vars[name] = newVal
	return newVal
}

// VarRef liefert die Zelle über die komplette Scope-Kette, ohne anzulegen -
// die Auflösung des JIT. isConst gilt für den Scope, in dem der Name
// definiert ist (so prüft die VM OpStore: gefundEN und nicht const).
func (e *Environment) VarRef(name string) (ptr *value.Value, found, isConst bool) {
	defEnv, p, ok := e.GetRefStrictEnv(name)
	if !ok {
		return nil, false, false
	}
	return p, true, defEnv.isConstHere(name)
}

// LocalRef liefert nur den aktuellen Scope, ohne anzulegen. Entspricht
// GetRef ohne Anlegen - die For-Schleifen der VM suchen nur hier.
func (e *Environment) LocalRef(name string) (*value.Value, bool) {
	if ptr, ok := e.vars[name]; ok {
		return ptr, true
	}
	return nil, false
}

var _ jit.Env = (*Environment)(nil)

// isConstHere: ist der Name in DIESEM Scope als Const markiert?
func (e *Environment) isConstHere(name string) bool {
	return e.consts != nil && e.consts[name]
}

func (e *Environment) Define(name string, val value.Value, inLoop bool) {
	e.defineInternal(name, val, inLoop, false)
}

func (e *Environment) defineInternal(name string, val value.Value, inLoop bool, suppressShadowInfo bool) {
	const (
		colorYellow = "\033[33m"
		colorCyan   = "\033[36m"
		colorReset  = "\033[0m"
	)

	if ptr, existsLocally := e.vars[name]; existsLocally {
		if !inLoop {
			fmt.Fprintf(os.Stderr, "%s![HINWEIS]: Variable '%s' bereits deklariert.%s\n", colorYellow, name, colorReset)
		}
		*ptr = val // Pointer bleibt stabil
		return
	}

	if e.parent != nil && !suppressShadowInfo {
		if _, existsAbove := e.parent.Get(name); existsAbove {
			fmt.Fprintf(os.Stderr, "%s![INFO]: Variable '%s' wird im lokalen Scope verwendet (Shadowing).%s\n", colorCyan, name, colorReset)
		}
	}

	v := val
	e.vars[name] = &v
}

func (e *Environment) DefineConst(name string, val value.Value) {
	e.defineInternal(name, val, false, true)
}

func (e *Environment) MarkConst(name string) {
	if e.consts == nil {
		e.consts = make(map[string]bool)
	}
	e.consts[name] = true
}

// SetGlobal wandert zum obersten Parent und speichert dort (für Public).
func (e *Environment) SetGlobal(name string, val value.Value) {
	curr := e
	for curr.parent != nil {
		curr = curr.parent
	}
	if ptr, ok := curr.vars[name]; ok {
		*ptr = val
		return
	}
	v := val
	curr.vars[name] = &v
}
