package evaluator

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"vbxc/stdlib"
	. "vbxc/token"
)

// ---------------- Evaluator / Value ----------------
//
// Die Laufzeit-Typen kommen aus den portierten Paketen (ast, token, value,
// stdlib) -- die Aliase halten den Code 1:1 im Interpreter-Stil.
type Signal int

const (
	SignalNone Signal = iota
	SignalExitLoop
	SignalExitSub
	SignalExitFunction
	SignalReturn
	SignalError
	SignalContinueLoop
)

type ExecResult struct {
	Signal Signal
	Value  Value
}

// GetRef liefert einen Pointer auf den Wert in der Map
type Environment struct {
	vars            map[string]*Value
	consts          map[string]bool
	parent          *Environment
	currentLine     int
	currentFile     string
	currentFuncName string
	fnReturn        Value

	varVersion uint64
}

func tryParseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func literalValue(e Expr) (Value, bool) {
	switch n := e.(type) {
	case *NumberNode:
		return NumVal(n.Value), true
	case *StringNode:
		return Value{Kind: KindStr, Str: n.Value}, true
	// Im Interpreter steckten Bool-/Null-Literale als Value direkt im AST;
	// vbxc hat dafür eigene Knoten (ast.BoolNode, ast.NullNode).
	case *BoolNode:
		return BoolVal(n.Value), true
	case *NullNode:
		return NullVal(), true
	default:
		return Value{}, false
	}
}

func calculateBinaryOp(op TokenType, l Value, r Value) Value {
	if l.Kind == KindNum && r.Kind == KindNum {
		switch op {
		case PLUS:
			return NumVal(l.Num + r.Num)
		case MINUS:
			return NumVal(l.Num - r.Num)
		case MUL:
			return NumVal(l.Num * r.Num)
		case DIV:
			if r.Num != 0 {
				return NumVal(l.Num / r.Num)
			}
			// bei 0 weiter unten: gleiche Fehlermeldung wie sonst
		}
	}

	switch op {

	case PLUS:
		// Zwei Strings werden als Strings verkettet.
		if l.Kind == KindStr && r.Kind == KindStr {
			return StrVal(l.Str + r.Str)
		}

		// Zahlen werden addiert.
		if ln, ok := tryNumber(l); ok {
			if rn, ok := tryNumber(r); ok {
				return NumVal(ln + rn)
			}
		}

		return StrVal(ToString(l) + ToString(r))

	case MINUS:
		ln, lok := tryNumber(l)
		rn, rok := tryNumber(r)
		if !lok || !rok {
			return ErrorVal("Operanden müssen numerisch sein")
		}
		return NumVal(ln - rn)

	case MUL:
		ln, lok := tryNumber(l)
		rn, rok := tryNumber(r)
		if !lok || !rok {
			return ErrorVal("Operanden müssen numerisch sein")
		}
		return NumVal(ln * rn)

	case DIV:
		ln, lok := tryNumber(l)
		rn, rok := tryNumber(r)
		if !lok || !rok {
			return ErrorVal("Operanden müssen numerisch sein")
		}
		if rn == 0 {
			return ErrorVal("Division durch 0")
		}
		return NumVal(ln / rn)
	}

	return ErrorVal("Nicht unterstützter Operator")
}

func mapAssignOp(op TokenType) TokenType {
	switch op {
	case PLUS_ASSIGN:
		return PLUS
	case MINUS_ASSIGN:
		return MINUS
	case MUL_ASSIGN:
		return MUL
	case DIV_ASSIGN:
		return DIV
	default:
		panic("invalid compound operator")
	}
}

// Lazy gebauter Index für qualifizierte Builtin-Aufrufe (modul.funktion),
// vermeidet den linearen Scan über die gesamte builtins-Map bei jedem Aufruf.

func buildBuiltinModuleIndex() {
	builtins := currentBuiltins()
	builtinModuleIndex = make(map[string]BuiltinInfo, len(builtins))
	for builtinName, info := range builtins {
		if info.Module == "" {
			continue
		}
		key := strings.ToLower(info.Module) + "." + strings.ToLower(builtinName)
		builtinModuleIndex[key] = info
	}
	builtinModuleIndexGen = stdlib.Generation()
}

func findBuiltinByModuleAndName(module, name string) (BuiltinInfo, bool) {
	// Snapshots aktuell halten (stdlib.Generation hat sich erhöht ->
	// neu aufbauen), dann wie bisher nachschlagen.
	gen := stdlib.Generation()
	if gen != builtinsGen || gen != builtinModuleIndexGen {
		currentBuiltins()
		buildBuiltinModuleIndex()
	}

	// 1. Direkter qualifizierter Name:
	//    z.B. "app.ExecutablePath"
	if info, ok := builtins[module+"."+name]; ok {
		return info, true
	}

	// 2. Indizierter Lookup statt linearem Scan über builtins.
	key := strings.ToLower(module) + "." + strings.ToLower(name)
	if info, ok := builtinModuleIndex[key]; ok {
		return info, true
	}

	return BuiltinInfo{}, false
}

func (e *Environment) Get(name string) (Value, bool) {
	for env := e; env != nil; env = env.parent {
		if ptr, ok := env.vars[name]; ok {
			return *ptr, true
		}
	}
	return NilValue, false
}

func (e *Environment) Set(name string, val Value) {
	env := e
	for {
		if ptr, ok := env.vars[name]; ok {
			*ptr = val
			return
		}
		if env.parent == nil {
			break
		}
		env = env.parent
	}
	// nirgendwo gefunden -> wie vorher: im obersten (Root-)Scope neu anlegen
	copyVal := val
	env.vars[name] = &copyVal
}

func (e *Environment) root() *Environment {
	for e.parent != nil {
		e = e.parent
	}
	return e
}

func (e *Environment) Update(name string, val Value) error {
	for env := e; env != nil; env = env.parent {
		if ptr, ok := env.vars[name]; ok {
			*ptr = val
			return nil
		}
	}
	return fmt.Errorf("Variable '%s' nicht gefunden. Nutze DIM oder PUBLIC.", name)
}

// GetRefStrict liefert einen Pointer auf den Wert, OHNE ihn bei Fehlen anzulegen.
// Für strikte Zugriffe wie Array-Zuweisungen (im Unterschied zu GetRef,
// das für interne Schleifenvariablen automatisch neu anlegt).
func (e *Environment) GetRefStrict(name string) (*Value, bool) {
	for env := e; env != nil; env = env.parent {
		if ptr, ok := env.vars[name]; ok {
			return ptr, true
		}
	}
	return nil, false
}

func (e *Environment) GetRefStrictEnv(name string) (*Environment, *Value, bool) {
	for env := e; env != nil; env = env.parent {
		if ptr, ok := env.vars[name]; ok {
			return env, ptr, true
		}
	}

	return nil, nil, false
}

func (e *Environment) GetRef(name string) *Value {
	if ptr, ok := e.vars[name]; ok {
		return ptr
	}
	// Nur für interne Schleifenvariablen (FOR i = ...) gedacht!
	// Nicht für Nutzervariablen verwenden.
	newVal := &Value{Kind: KindNum, Num: 0}
	e.vars[name] = newVal
	return newVal
}

func (e *Environment) PutInternal(name string, val Value) {
	if ptr, ok := e.vars[name]; ok {
		*ptr = val // Pointer bleibt stabil -> gecachte Zugriffe bleiben gültig
		return
	}
	v := val
	e.vars[name] = &v
}

// Define erstellt IMMER eine Variable im aktuellen Scope (für DIM)
func (e *Environment) Define(name string, val Value, inLoop bool) {
	e.defineInternal(name, val, inLoop, false)
}

func (e *Environment) defineInternal(name string, val Value, inLoop bool, suppressShadowInfo bool) {
	const (
		colorYellow = "\033[33m"
		colorCyan   = "\033[36m"
		colorReset  = "\033[0m"
	)

	if ptr, existsLocally := e.vars[name]; existsLocally {
		if !inLoop {
			fmt.Fprintf(os.Stderr, "%s![HINWEIS]: Variable '%s' bereits deklariert.%s\n", colorYellow, name, colorReset)
		}
		*ptr = val // Pointer bleibt stabil -> gecachte Zugriffe bleiben gültig
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

// DefineConst wie Define, unterdrückt aber den Shadowing-Hinweis:
// Const innen mit gleichem Namen wie außen ist bei Const ein
// gewolltes, häufiges Pattern (z.B. Function-Parameter/Konstanten
// gleich benennen wie global), kein warnungswürdiger Sonderfall.
func (e *Environment) DefineConst(name string, val Value) {
	e.defineInternal(name, val, false, true)
}

// MarkConst markiert einen im AKTUELLEN Environment bereits vorhandenen
// Namen als schreibgeschützt. Wird direkt nach Define() für eine
// Const-Deklaration aufgerufen.
func (e *Environment) MarkConst(name string) {
	if e.consts == nil {
		e.consts = make(map[string]bool)
	}
	e.consts[name] = true
}

// IsConst prüft, ob [name] als Const markiert ist. Läuft dieselbe
// Scope-Kette ab wie Get/Update: es zählt das Environment, in dem die
// Variable TATSÄCHLICH liegt (wichtig bei Shadowing - eine lokale Dim
// mit gleichem Namen wie eine äußere Const ist eigenständig und NICHT
// schreibgeschützt).
func (e *Environment) IsConst(name string) bool {
	for env := e; env != nil; env = env.parent {
		if _, ok := env.vars[name]; ok {
			return env.consts != nil && env.consts[name]
		}
	}
	return false
}

// SetGlobal wandert zum obersten Parent und speichert dort (für PUBLIC)
func (e *Environment) SetGlobal(name string, val Value) {
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

func NewEnvironment(parent *Environment) *Environment {
	return &Environment{
		vars:   make(map[string]*Value),
		parent: parent, // Wichtig für die Scope-Kette!
	}
}

func NewEnvironmentWithCapacity(parent *Environment, capacity int) *Environment {
	return &Environment{
		vars:   make(map[string]*Value, capacity),
		parent: parent,
	}
}

// ExitNode/ContinueNode/SelectNode/CaseBranch und die übrigen Knoten kommen
// aus dem ast-Paket.

func evalFunctionCall(name string, args []Expr, env *Environment) Value {
	// 1. Array-Zugriff prüfen
	// ACHTUNG: Get liefert jetzt (Value, bool)
	v, found := env.Get(name)
	if found && (v.Kind == KindArr || v.Kind == KindArr2D || v.Kind == KindMap || v.Kind == KindObj) {

		if v.Kind == KindObj && len(args) == 1 {

			keyVal := evalExpr(args[0], env)
			key := ToString(keyVal)

			m := v.Obj.Fields

			if val, ok := m[key]; ok {
				return val
			}

			return NilValue
		}

		if v.Kind == KindMap && len(args) == 1 {
			keyVal := evalExpr(args[0], env)
			if keyVal.Kind == KindError {
				return keyVal
			}

			key := ToString(keyVal)

			if val, ok := v.Map[key]; ok {
				return val
			}

			return NilValue
		}

		// --- FALL A: 1D-Zugriff oder Zeilen-Zugriff ---
		if len(args) == 1 {
			idxVal := evalExpr(args[0], env)
			if idxVal.Kind == KindError {
				return idxVal
			}
			idx := int(toNumVal(idxVal))

			if v.Kind == KindArr {
				if idx < 0 || idx >= len(v.Arr) {
					return ErrorVal("Index out of bounds")
				}
				return v.Arr[idx]
			}
			if v.Kind == KindArr2D {
				if idx < 0 || idx >= len(v.Arr2D) {
					return ErrorVal("Index out of bounds")
				}
				// Gibt die ganze Zeile als 1D-Array zurück
				return Value{Kind: KindArr, Arr: v.Arr2D[idx]}
			}
		}

		// --- FALL B: 2D-Zugriff (m(0,0)) ---
		if len(args) == 2 && v.Kind == KindArr2D {
			idx1Val := evalExpr(args[0], env)
			idx2Val := evalExpr(args[1], env)
			if idx1Val.Kind == KindError {
				return idx1Val
			}
			if idx2Val.Kind == KindError {
				return idx2Val
			}

			idx1 := int(toNumVal(idx1Val))
			idx2 := int(toNumVal(idx2Val))

			if idx1 < 0 || idx1 >= len(v.Arr2D) || idx2 < 0 || idx2 >= len(v.Arr2D[0]) {
				return ErrorVal("Matrix-Index out of bounds")
			}
			return v.Arr2D[idx1][idx2]
		}
	}

	// direkt VOR "2. Argumente auswerten" einfügen, analog zum Dump-Sonderfall weiter unten:
	if strings.EqualFold(name, "IsError") {
		if len(args) < 1 {
			return BoolVal(false)
		}
		val := evalExpr(args[0], env)
		return BoolVal(val.Kind == KindError)
	}

	// NEU: ErrorText direkt danach einfügen
	if strings.EqualFold(name, "ErrorText") {
		if len(args) < 1 {
			return StrVal("")
		}
		val := evalExpr(args[0], env)
		if val.Kind == KindError {
			return StrVal(val.Str)
		}
		return StrVal("")
	}

	// 2. Argumente auswerten
	evaluated := make([]Value, len(args))
	for i, arg := range args {
		val := evalExpr(arg, env)
		if val.Kind == KindError {
			return val
		}
		evaluated[i] = val
	}

	// 3. Builtins prüfen
	// HIER passiert die Magie: Wir fangen den Namen ab, BEVOR
	// wir in der builtins-Map nachsehen.
	if name == "Dump" {
		dumpEnvironment(env) // Hier ist env bekannt (aus dem Funktionskopf)
		return NullVal()
	}

	// 3. Builtins
	if info, ok := currentBuiltins()[name]; ok {
		// Deprecation-Warnung: einmalig pro Funktionsname und Prozesslauf,
		// unabhängig davon, ob -dryrun jemals aufgerufen wird -- die
		// Warnung ist an den ECHTEN Aufruf gekoppelt, nicht an ein
		// optionales Vorab-Flag, geht also nie verloren.
		if info.Deprecated != "" && !deprecationWarned[name] {
			deprecationWarned[name] = true
			fmt.Fprintf(
				os.Stderr,
				"\033[33m[WARNUNG]\033[0m '%s' ist veraltet - benutze stattdessen %s\n",
				name,
				info.Deprecated,
			)
		}

		// Nicht f(evaluated), sondern info.Fn(evaluated)
		return info.Fn(evaluated)
	}

	// --- 4. User-defined FUNCTION ---
	if fn, ok := funcs[name]; ok {
		required := requiredParams(fn.Params)

		if len(evaluated) < required || len(evaluated) > len(fn.Params) {
			return ErrorVal(fmt.Sprintf(
				"Funktion '%s' erwartet %d bis %d Argumente, erhalten: %d",
				name, required, len(fn.Params), len(evaluated),
			))
		}

		// Anzahl der Parameter ist bekannt -> Map direkt passend anlegen
		local := NewEnvironmentWithCapacity(env.root(), len(fn.Params)) // bzw. len(s.Params)

		local.fnReturn = NumVal(0)
		local.currentFuncName = name

		for i, p := range fn.Params {
			if i < len(evaluated) {
				local.PutInternal(p.Name, evaluated[i])
			} else {
				defVal := evalExpr(p.Default, local)
				if defVal.Kind == KindError {
					return defVal
				}
				local.PutInternal(p.Name, defVal)
			}
		}

		retVal, sig := evalStatements(fn.Body, local)
		if sig == SignalError {
			return ErrorVal(fmt.Sprintf(
				"Fehler in Funktion '%s': %s",
				name, retVal.Str,
			))
		}

		return local.fnReturn
	}

	// --- 5. User-defined SUB ---
	if s, ok := subs[name]; ok {
		required := requiredParams(s.Params)

		if len(evaluated) < required || len(evaluated) > len(s.Params) {
			return ErrorVal(fmt.Sprintf("Sub '%s' erwartet %d bis %d Argumente, erhalten: %d",
				name, required, len(s.Params), len(evaluated)))
		}

		local := NewEnvironmentWithCapacity(env.root(), len(s.Params)) // bzw. len(s.Params)
		for i, p := range s.Params {
			if i < len(evaluated) {
				local.PutInternal(p.Name, evaluated[i])
			} else {
				defVal := evalExpr(p.Default, local)
				if defVal.Kind == KindError {
					return defVal
				}
				local.PutInternal(p.Name, defVal)
			}
		}

		retVal, sig := evalStatements(s.Body, local)
		if sig == SignalError {
			return ErrorVal(fmt.Sprintf("Fehler in Sub '%s': %s", name, retVal.Str))
		}
		return NumVal(0)
	}

	return ErrorVal(fmt.Sprintf("Funktion oder Sub '%s' nicht gefunden", name))
}

var subs = make(map[string]*SubNode)
var funcs = make(map[string]*FuncNode)

// BuiltinInfo und die Registry kommen aus dem stdlib-Paket (die ~500
// Funktionen sind dort über init() vorregistriert, "#use" ist nur Metadaten).
type BuiltinInfo = stdlib.BuiltinInfo

// Verhindert Warnungs-Spam: pro Funktionsname nur einmal pro Prozesslauf.
var deprecationWarned = make(map[string]bool)

// Snapshot der stdlib-Registry. Werden zur Laufzeit noch Funktionen
// registriert (Plugin-Ladung über "#use"), zeigt
// stdlib.Generation() eine höhere Version an und der Snapshot wird
// routine-mäßig neu aufgebaut.
var (
	builtins              map[string]stdlib.BuiltinInfo
	builtinsGen           uint64
	builtinModuleIndex    map[string]BuiltinInfo
	builtinModuleIndexGen uint64
)

func currentBuiltins() map[string]stdlib.BuiltinInfo {
	gen := stdlib.Generation()
	if builtins == nil || gen != builtinsGen {
		builtins = stdlib.Builtins()
		builtinsGen = gen
	}
	return builtins
}

// 1. Die Listen-Funktion (Plural)
func evalStatements(stmts []Stmt, env *Environment) (Value, Signal) {
	var lastVal Value
	var sig Signal
	for _, s := range stmts {
		// Wir rufen für jedes Element die Single-Variante auf
		lastVal, sig = evalSingleStatement(s, env)
		if sig != SignalNone {
			return lastVal, sig
		}
	}
	return lastVal, SignalNone
}

// 2. Die Einzel-Funktion (Singular)
func evalSingleStatement(s Stmt, env *Environment) (Value, Signal) {
	// KEINE Schleife mehr hier!
	switch n := s.(type) {

	case *AssignNode:

		// ============================================================
		// Function-Return
		// ============================================================

		if env.currentFuncName != "" && n.Name == env.currentFuncName {
			val := evalExpr(n.Value, env)

			if val.Kind == KindError {
				return val, SignalError
			}

			env.fnReturn = val
			return NullVal(), SignalNone
		}

		// ============================================================
		// Deklaration
		// ============================================================

		if n.IsDeclaration {
			val := evalExpr(n.Value, env)

			if val.Kind == KindError {
				return val, SignalError
			}

			if env.consts != nil && env.consts[n.Name] {
				return ErrorVal(fmt.Sprintf(
					"Fehler: '%s' wurde bereits als Konstante deklariert",
					n.Name,
				)), SignalError
			}

			if n.IsConst {
				env.DefineConst(n.Name, val)
				env.MarkConst(n.Name)
			} else {
				env.Define(n.Name, val, n.InLoop)
			}

			return NullVal(), SignalNone
		}

		// ============================================================
		// Normale Zuweisung
		// ============================================================

		// Ziel-Cache: steckt im Interpreter als cached*-Feldern am Knoten.
		c := nodeCacheFor(n)

		// ------------------------------------------------------------
		// Fast-Path für eine konstante Zahl:
		//
		//     y = 123
		//
		// Kein evalExpr() und kein NumberNode -> NumVal().
		// ------------------------------------------------------------

		if numberNode, ok := n.Value.(*NumberNode); ok {
			if c.lookupEnv == env && c.ptr != nil {
				if c.isConst {
					return ErrorVal(fmt.Sprintf(
						"Fehler: Es wird versucht die Konstante '%s' zu verändern",
						n.Name,
					)), SignalError
				}

				*c.ptr = Value{
					Kind: KindNum,
					Num:  numberNode.Value,
				}

				return NullVal(), SignalNone
			}
		}

		// ------------------------------------------------------------
		// Normale Auswertung
		// ------------------------------------------------------------

		val := evalExpr(n.Value, env)

		if val.Kind == KindError {
			return val, SignalError
		}

		// Fast-Path:
		// Das Environment und der Value-Pointer wurden bereits beim
		// ersten Zugriff ermittelt.
		if c.lookupEnv == env && c.ptr != nil {
			if c.isConst {
				return ErrorVal(fmt.Sprintf(
					"Fehler: Es wird versucht die Konstante '%s' zu verändern",
					n.Name,
				)), SignalError
			}

			*c.ptr = val
			return NullVal(), SignalNone
		}

		// Erster Zugriff:
		// Variable suchen und gleichzeitig das tatsächliche Environment
		// merken.
		defEnv, ptr, found := env.GetRefStrictEnv(n.Name)

		if !found {
			scopeInfo := "Public (global)"
			if env.parent != nil {
				scopeInfo = "lokal (Dim) oder Public (global)"
			}

			return ErrorVal(fmt.Sprintf(
				"Variable '%s' muss erst mit Dim oder Public deklariert werden (nicht gefunden in %s)",
				n.Name,
				scopeInfo,
			)), SignalError
		}

		// Const-Status direkt im gefundenen Environment prüfen.
		isConst := defEnv.consts != nil && defEnv.consts[n.Name]

		// Cache aufbauen.
		c.lookupEnv = env
		c.ptr = ptr
		c.isConst = isConst

		if isConst {
			return ErrorVal(fmt.Sprintf(
				"Fehler: Es wird versucht die Konstante '%s' zu verändern",
				n.Name,
			)), SignalError
		}

		*ptr = val

		return NullVal(), SignalNone

	case *CompoundAssignNode:
		v, ok := n.Left.(*VarNode)
		if !ok {
			return ErrorVal("only variables supported"), SignalError
		}

		var current Value

		c := nodeCacheFor(n)

		if c.lookupEnv == env && c.ptr != nil {
			if c.isConst {
				return ErrorVal(fmt.Sprintf(
					"Fehler: Es wird versucht die Konstante '%s' zu verändern",
					v.Name,
				)), SignalError
			}
			current = *c.ptr
		} else {
			defEnv, ptr, found := env.GetRefStrictEnv(v.Name)
			if !found {
				return ErrorVal("undefined variable: " + v.Name), SignalError
			}

			isConst := defEnv.consts != nil && defEnv.consts[v.Name]

			c.lookupEnv = env
			c.ptr = ptr
			c.isConst = isConst

			if isConst {
				return ErrorVal(fmt.Sprintf(
					"Fehler: Es wird versucht die Konstante '%s' zu verändern",
					v.Name,
				)), SignalError
			}

			current = *ptr
		}

		rhs := evalExpr(n.Right, env)
		if rhs.Kind == KindError {
			return rhs, SignalError
		}

		result := calculateBinaryOp(mapAssignOp(n.Op), current, rhs)
		if result.Kind == KindError {
			return result, SignalError // Variable bleibt unverändert
		}

		*c.ptr = result
		return result, SignalNone

	case *PrintNode:
		// 1. Gesamten Ausdruck auswerten (inkl. Verkettungen von Text & Farbe)
		val := evalExpr(n.Value, env)
		if val.Kind == KindError {
			return val, SignalError
		}

		outputStr := ToString(val)

		// --- LOGGING ---
		// Solange ein debug-Log offen ist, landet die Print-Ausgabe dort
		// (mit Zeitstempel) statt auf der Konsole -- wie im Interpreter.
		if lf := stdlib.ActiveLog(); lf != nil {
			timestamp := time.Now().Format("15:04:05")
			fmt.Fprintf(lf, "[%s] [PRINT] %s\n", timestamp, outputStr)
			return NullVal(), SignalNone
		}

		// --- AUSGABE ---
		// Wir hängen IMMER einen Reset an, um das Terminal sauber zu halten.
		// Das \033[0m wirkt sowohl nach n.Color als auch nach Inline-Farben.
		if n.Color != nil {
			colorVal := evalExpr(n.Color, env)
			if colorVal.Kind == KindStr {
				fmt.Printf("%s%s\033[0m\n", colorVal.Str, outputStr)
				return NullVal(), SignalNone
			}
			if colorVal.Kind == KindNum {
				c := int(colorVal.Num)
				fmt.Printf("\033[%dm%s\033[0m\n", 30+(c%10), outputStr)
				return NullVal(), SignalNone
			}
		}

		// Standardfall: Text (evtl. mit Inline-ANSI) + Reset
		fmt.Print(outputStr + "\033[0m\n")
		return NullVal(), SignalNone

	case *WhileNode:
		// ------------------------------------------------------------
		// Fast-Path: WHILE-Body besteht aus genau einer Anweisung
		// ------------------------------------------------------------
		if len(n.Body) == 1 {
			body := n.Body[0]

			// --------------------------------------------------------
			// Spezial-Path:
			// WHILE ... : x += wert : WEND
			// --------------------------------------------------------
			if bodyNode, ok := body.(*CompoundAssignNode); ok {
				if v, ok := bodyNode.Left.(*VarNode); ok {

					// Variable einmalig auflösen.
					var ptr *Value
					c := nodeCacheFor(bodyNode)

					if c.lookupEnv == env && c.ptr != nil {
						if c.isConst {
							return ErrorVal(fmt.Sprintf(
								"Fehler: Es wird versucht die Konstante '%s' zu verändern",
								v.Name,
							)), SignalError
						}

						ptr = c.ptr
					} else {
						defEnv, p, found := env.GetRefStrictEnv(v.Name)
						if !found {
							return ErrorVal("undefined variable: " + v.Name), SignalError // GEÄNDERT (1)
						}

						isConst := defEnv.consts != nil && defEnv.consts[v.Name]

						c.lookupEnv = env
						c.ptr = p
						c.isConst = isConst

						if isConst {
							return ErrorVal(fmt.Sprintf(
								"Fehler: Es wird versucht die Konstante '%s' zu verändern",
								v.Name,
							)), SignalError
						}

						ptr = p
					}

					assignOp := mapAssignOp(bodyNode.Op)

					for {
						cond := evalExpr(n.Condition, env)
						if cond.Kind == KindError {
							return cond, SignalError
						}

						if !isTruthy(cond) {
							break
						}

						rhs := evalExpr(bodyNode.Right, env)
						if rhs.Kind == KindError {
							return rhs, SignalError // GEÄNDERT (2)
						}

						result := calculateBinaryOp(assignOp, *ptr, rhs)
						if result.Kind == KindError { // GEÄNDERT (3): erst prüfen ...
							return result, SignalError
						}
						*ptr = result // ... dann speichern
					}

					return NullVal(), SignalNone
				}
			}

			// --------------------------------------------------------
			// Normaler Single-Statement-Fast-Path (unverändert)
			// --------------------------------------------------------
			for {
				cond := evalExpr(n.Condition, env)
				if cond.Kind == KindError {
					return cond, SignalError
				}

				if !isTruthy(cond) {
					break
				}

				rv, sig := evalSingleStatement(body, env)

				switch sig {
				case SignalNone, SignalContinueLoop:
					// weiter
				case SignalExitLoop:
					return NullVal(), SignalNone
				default:
					return rv, sig
				}
			}

			return NullVal(), SignalNone
		}

		// ------------------------------------------------------------
		// Normaler Pfad: mehrere Anweisungen im WHILE-Body (unverändert)
		// ------------------------------------------------------------
		for {
			cond := evalExpr(n.Condition, env)
			if cond.Kind == KindError {
				return cond, SignalError
			}

			if !isTruthy(cond) {
				break
			}

			rv, sig := evalStatements(n.Body, env)

			switch sig {
			case SignalNone, SignalContinueLoop:
				// weiter
			case SignalExitLoop:
				return NullVal(), SignalNone
			default:
				return rv, sig
			}
		}

		return NullVal(), SignalNone

	case *DoLoopNode:
		runBody := func() (Value, Signal, bool) {
			rv, sig := evalStatements(n.Body, env)
			switch sig {
			case SignalNone:
				return Value{}, SignalNone, false
			case SignalContinueLoop:
				// Bei Do ... Loop: direkt zur Fuß-Bedingung springen
				return Value{}, SignalContinueLoop, false
			case SignalExitLoop:
				return Value{}, SignalNone, true
			default:
				return rv, sig, true
			}
		}

		for {
			// Kopf-Bedingung
			if !n.CheckAtEnd && n.Condition != nil {
				cond := evalExpr(n.Condition, env)
				if cond.Kind == KindError {
					return cond, SignalError
				}
				if n.IsUntil == isTruthy(cond) {
					break
				}
			}

			rv, sig, stop := runBody()
			if stop {
				if sig != SignalNone {
					return rv, sig
				}
				break
			}

			// Fuß-Bedingung
			if n.CheckAtEnd && n.Condition != nil {
				cond := evalExpr(n.Condition, env)
				if cond.Kind == KindError {
					return cond, SignalError
				}
				if n.IsUntil == isTruthy(cond) {
					break
				}
			}
		}

	case *ForEachNode:
		iterVal := evalExpr(n.Iterable, env)
		if iterVal.Kind == KindError {
			return iterVal, SignalError
		}

		keyPtr := env.GetRef(n.KeyVar)
		var valPtr *Value
		if n.ValVar != "" {
			valPtr = env.GetRef(n.ValVar)
		}

		// runBody führt den Loop-Body aus und übersetzt das Signal in:
		// - stop=false: weiter zur nächsten Iteration (None/Continue)
		// - stop=true, sig==None: Schleife regulär beenden (ExitLoop)
		// - stop=true, sig!=None: Signal nach außen durchreichen (z.B. Return)
		runBody := func() (Value, Signal, bool) {
			rv, sig := evalStatements(n.Body, env)
			switch sig {
			case SignalNone, SignalContinueLoop:
				return Value{}, SignalNone, false
			case SignalExitLoop:
				return Value{}, SignalNone, true
			default:
				return rv, sig, true
			}
		}

		if iterVal.Kind == KindArr {
			for i, val := range iterVal.Arr {
				if valPtr != nil {
					*keyPtr = NumVal(float64(i))
					*valPtr = val
				} else {
					*keyPtr = val
				}

				if rv, sig, stop := runBody(); stop {
					if sig != SignalNone {
						return rv, sig
					}
					break
				}
			}

		} else if iterVal.Kind == KindArr2D {
			for i, row := range iterVal.Arr2D {
				// Wir wandeln die [][]Value-Zeile in ein normales Array um.
				rowVal := Value{Kind: KindArr, Arr: row}

				if valPtr != nil {
					*keyPtr = NumVal(float64(i))
					*valPtr = rowVal
				} else {
					*keyPtr = rowVal
				}

				if rv, sig, stop := runBody(); stop {
					if sig != SignalNone {
						return rv, sig
					}
					break
				}
			}

		} else if iterVal.Kind == KindMap {
			keys := make([]string, 0, len(iterVal.Map))
			for k := range iterVal.Map {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				val := iterVal.Map[k]

				if valPtr != nil {
					*keyPtr = StrVal(k)
					*valPtr = val
				} else {
					*keyPtr = val
				}

				if rv, sig, stop := runBody(); stop {
					if sig != SignalNone {
						return rv, sig
					}
					break
				}
			}

		} else {
			return ErrorVal("FOR EACH: Erwartet Array oder Map"), SignalError
		}
	case *ForNode:
		startVal := evalExpr(n.Start, env)
		if startVal.Kind == KindError {
			return startVal, SignalError
		}
		startNum := toNumVal(startVal)

		endVal := evalExpr(n.End, env)
		if endVal.Kind == KindError {
			return endVal, SignalError
		}
		endNum := toNumVal(endVal)

		step := n.Step
		if step == 0 {
			return ErrorVal("FOR mit STEP 0 ist nicht erlaubt"), SignalError
		}

		varPtr := env.GetRef(n.VarName)

		// ------------------------------------------------------------
		// Fast-Path: FOR-Body besteht aus genau einer Anweisung
		// ------------------------------------------------------------
		if len(n.Body) == 1 {
			body := n.Body[0]

			// --------------------------------------------------------
			// Spezial-Path:
			// FOR ... : x += wert : NEXT
			// --------------------------------------------------------
			if bodyNode, ok := body.(*CompoundAssignNode); ok {
				if v, ok := bodyNode.Left.(*VarNode); ok {

					// Variable einmalig auflösen.
					var ptr *Value
					c := nodeCacheFor(bodyNode)

					if c.lookupEnv == env && c.ptr != nil {
						if c.isConst {
							return ErrorVal(fmt.Sprintf(
								"Fehler: Es wird versucht die Konstante '%s' zu verändern",
								v.Name,
							)), SignalError
						}

						ptr = c.ptr
					} else {
						defEnv, p, found := env.GetRefStrictEnv(v.Name)
						if !found {
							return ErrorVal("undefined variable: " + v.Name), SignalError // GEÄNDERT (1)
						}

						isConst := defEnv.consts != nil && defEnv.consts[v.Name]

						c.lookupEnv = env
						c.ptr = p
						c.isConst = isConst

						if isConst {
							return ErrorVal(fmt.Sprintf(
								"Fehler: Es wird versucht die Konstante '%s' zu verändern",
								v.Name,
							)), SignalError
						}

						ptr = p
					}

					assignOp := mapAssignOp(bodyNode.Op)

					if step > 0 {
						for cur := startNum; cur <= endNum; cur += step {
							varPtr.Kind = KindNum
							varPtr.Num = cur

							rhs := evalExpr(bodyNode.Right, env)
							if rhs.Kind == KindError {
								return rhs, SignalError // GEÄNDERT (2)
							}

							result := calculateBinaryOp(assignOp, *ptr, rhs)
							if result.Kind == KindError { // GEÄNDERT (3)
								return result, SignalError
							}
							*ptr = result
						}
					} else {
						for cur := startNum; cur >= endNum; cur += step {
							varPtr.Kind = KindNum
							varPtr.Num = cur

							rhs := evalExpr(bodyNode.Right, env)
							if rhs.Kind == KindError {
								return rhs, SignalError // GEÄNDERT (2)
							}

							result := calculateBinaryOp(assignOp, *ptr, rhs)
							if result.Kind == KindError { // GEÄNDERT (3)
								return result, SignalError
							}
							*ptr = result
						}
					}

					return NullVal(), SignalNone
				}
			}

			// --------------------------------------------------------
			// Normaler Single-Statement-Fast-Path (unverändert)
			// --------------------------------------------------------
			if step > 0 {
				for v := startNum; v <= endNum; v += step {
					varPtr.Kind = KindNum
					varPtr.Num = v

					rv, sig := evalSingleStatement(body, env)

					switch sig {
					case SignalNone, SignalContinueLoop:
						// weiter
					case SignalExitLoop:
						return NullVal(), SignalNone
					default:
						return rv, sig
					}
				}
			} else {
				for v := startNum; v >= endNum; v += step {
					varPtr.Kind = KindNum
					varPtr.Num = v

					rv, sig := evalSingleStatement(body, env)

					switch sig {
					case SignalNone, SignalContinueLoop:
						// weiter
					case SignalExitLoop:
						return NullVal(), SignalNone
					default:
						return rv, sig
					}
				}
			}

			return NullVal(), SignalNone
		}

		// ------------------------------------------------------------
		// Normaler Pfad: mehrere Anweisungen im FOR-Body (unverändert)
		// ------------------------------------------------------------
		if step > 0 {
			for v := startNum; v <= endNum; v += step {
				varPtr.Kind = KindNum
				varPtr.Num = v

				rv, sig := evalStatements(n.Body, env)

				switch sig {
				case SignalNone, SignalContinueLoop:
					// weiter
				case SignalExitLoop:
					return NullVal(), SignalNone
				default:
					return rv, sig
				}
			}
		} else {
			for v := startNum; v >= endNum; v += step {
				varPtr.Kind = KindNum
				varPtr.Num = v

				rv, sig := evalStatements(n.Body, env)

				switch sig {
				case SignalNone, SignalContinueLoop:
					// weiter
				case SignalExitLoop:
					return NullVal(), SignalNone
				default:
					return rv, sig
				}
			}
		}

		return NullVal(), SignalNone

	case *SelectNode:
		valToTest := evalExpr(n.Expression, env)
		if valToTest.Kind == KindError {
			return valToTest, SignalError
		}

		// Dispatch-Cache: steckte im Interpreter als Feld am SelectNode,
		// hier in der Neben-Map (s. selectDispatchFor).
		d := selectDispatchFor(n)
		if !d.built {
			d.built = true
			d.ok = true
			d.numCache = make(map[float64]int)
			d.strCache = make(map[string]int)

		buildLoop:
			for idx, branch := range n.Cases {
				for _, cond := range branch.Conditions {
					lit, isLiteral := literalValue(cond)
					if !isLiteral {
						d.ok = false
						break buildLoop
					}
					switch lit.Kind {
					case KindNum:
						if _, dup := d.numCache[lit.Num]; !dup {
							d.numCache[lit.Num] = idx
						}
					case KindStr:
						key := strings.ToLower(lit.Str)
						if _, dup := d.strCache[key]; !dup {
							d.strCache[key] = idx
						}
					default:
						// Bool/Null-Case-Werte (selten) -> Fastpath deaktiviert,
						// linearer Scan uebernimmt komplett.
						d.ok = false
						break buildLoop
					}
				}
			}
			if !d.ok {
				d.numCache = nil
				d.strCache = nil
			}
		}

		if d.ok && valToTest.Kind != KindBool {
			var idx int
			found := false

			switch valToTest.Kind {
			case KindNum:
				idx, found = d.numCache[valToTest.Num]
				if !found && len(d.strCache) > 0 {
					// Cross-Type (z.B. Case "10"), nur pruefen wenn ueberhaupt
					// String-Cases existieren - kostet sonst nichts extra.
					idx, found = d.strCache[strings.ToLower(ToString(valToTest))]
				}
			case KindStr:
				idx, found = d.strCache[strings.ToLower(valToTest.Str)]
				if !found && len(d.numCache) > 0 {
					if numKey, ok := tryParseFloat(valToTest.Str); ok {
						idx, found = d.numCache[numKey]
					}
				}
			}

			if found {
				rv, sig := evalStatements(n.Cases[idx].Body, env)
				if sig != SignalNone {
					return rv, sig
				}
				return Value{}, SignalNone
			}
			if len(n.Default) > 0 {
				rv, sig := evalStatements(n.Default, env)
				if sig != SignalNone {
					return rv, sig
				}
			}
			return Value{}, SignalNone
		}

		// Fallback: linearer Scan (unverändert, deckt Range/Is/dynamische Ausdrücke ab)
		matched := false
		for _, branch := range n.Cases {
			for _, condExpr := range branch.Conditions {
				isMatch := false

				switch c := condExpr.(type) {
				case *RangeNode:
					target := toFloatSafe([]Value{valToTest}, 0)
					lowVal := evalExpr(c.Low, env)
					low := toFloatSafe([]Value{lowVal}, 0)
					highVal := evalExpr(c.High, env)
					high := toFloatSafe([]Value{highVal}, 0)
					if target >= low && target <= high {
						isMatch = true
					}

				case *IsNode:
					res := evalBinary(valToTest, c.Operator, evalExpr(c.Value, env))
					if res.Kind == KindBool && res.Bool {
						isMatch = true
					}

				default:
					condVal := evalExpr(condExpr, env)
					if valuesAreEqual(valToTest, condVal) {
						isMatch = true
					}
				}

				if isMatch {
					matched = true
					rv, sig := evalStatements(branch.Body, env)
					if sig != SignalNone {
						return rv, sig
					}
					break
				}
			}
			if matched {
				break
			}
		}

		if !matched && len(n.Default) > 0 {
			rv, sig := evalStatements(n.Default, env)
			if sig != SignalNone {
				return rv, sig
			}
		}

	case *PublicNode:
		val := evalExpr(n.Value, env)
		if val.Kind == KindError {
			return val, SignalError
		}
		// SetGlobal wandert die Kette hoch bis zum Root-Environment
		env.SetGlobal(n.Name, val)
		return NullVal(), SignalNone

	case *PublicArrayNode:
		// 1. Dimension
		s1Val := evalExpr(n.Size1, env)
		if s1Val.Kind == KindError {
			return s1Val, SignalError
		}
		size1 := int(toNumVal(s1Val)) + 1

		var finalArray Value

		if n.Size2 != nil {
			// 2D Fall
			s2Val := evalExpr(n.Size2, env)
			if s2Val.Kind == KindError {
				return s2Val, SignalError
			}
			size2 := int(toNumVal(s2Val)) + 1

			matrix := make([][]Value, size1)
			for i := range matrix {
				row := make([]Value, size2)
				for j := range row {
					row[j] = NumVal(0)
				}
				matrix[i] = row
			}
			finalArray = Value{Kind: KindArr2D, Arr2D: matrix}
		} else {
			// 1D Fall
			arr := make([]Value, size1)
			for i := range arr {
				arr[i] = NumVal(0)
			}
			finalArray = Value{Kind: KindArr, Arr: arr}
		}

		// WICHTIG: SetGlobal statt Define/Set
		env.SetGlobal(n.Name, finalArray)
		return NullVal(), SignalNone

	case *DimArrayNode:
		// 1. Dimension auswerten
		size1 := 0
		if n.Size1 != nil {
			s1Val := evalExpr(n.Size1, env)
			if s1Val.Kind == KindError {
				return s1Val, SignalError
			}
			size1 = int(toNumVal(s1Val)) + 1 // +1 für BASIC-Konvention
		}

		// Prüfen: Ist es ein 2D-Array?
		if n.Size2 != nil {
			// 2. Dimension auswerten
			s2Val := evalExpr(n.Size2, env)
			if s2Val.Kind == KindError {
				return s2Val, SignalError
			}
			size2 := int(toNumVal(s2Val)) + 1

			// Matrix (KindArr2D) erstellen
			matrix := make([][]Value, size1)
			for i := range matrix {
				row := make([]Value, size2)
				for j := range row {
					row[j] = NumVal(0) // Mit 0 initialisieren
				}
				matrix[i] = row
			}

			// Lokal im Environment speichern
			env.Define(n.Name, Value{Kind: KindArr2D, Arr2D: matrix}, n.InLoop)

		} else {
			// Normales 1D-Array (KindArr) erstellen
			arr := make([]Value, size1)
			for i := range arr {
				arr[i] = NumVal(0)
			}

			// Lokal im Environment speichern
			env.Define(n.Name, Value{Kind: KindArr, Arr: arr}, n.InLoop)
		}

		return NullVal(), SignalNone

	case *ArrayAssignNode:
		defEnv, vp, found := env.GetRefStrictEnv(n.Name)
		if !found {
			return ErrorVal("Variable nicht deklariert: " + n.Name), SignalError
		}

		if defEnv.consts != nil && defEnv.consts[n.Name] {
			return ErrorVal(fmt.Sprintf(
				"Fehler: Es wird versucht die Konstante '%s' zu verändern",
				n.Name,
			)), SignalError
		}

		val := evalExpr(n.Value, env)
		if val.Kind == KindError {
			return val, SignalError
		}

		idx1, errV := toIndex(evalExpr(n.Index, env))
		if errV.Kind == KindError {
			return errV, SignalError
		}

		switch vp.Kind {
		case KindArr2D:
			if n.Index2 == nil {
				return ErrorVal("Zweiter Index fehlt"), SignalError
			}
			idx2, errV := toIndex(evalExpr(n.Index2, env))
			if errV.Kind == KindError {
				return errV, SignalError
			}

			if idx1 < 0 || idx1 >= len(vp.Arr2D) || idx2 < 0 || idx2 >= len(vp.Arr2D[idx1]) {
				return ErrorVal(fmt.Sprintf("Matrix-Index (%d,%d) außerhalb der Grenzen", idx1, idx2)), SignalError
			}
			vp.Arr2D[idx1][idx2] = val

		case KindArr:
			if idx1 < 0 {
				return ErrorVal("Index negativ"), SignalError
			}
			if idx1 >= len(vp.Arr) {
				newArr := make([]Value, idx1+1)
				copy(newArr, vp.Arr)
				for i := len(vp.Arr); i < len(newArr); i++ {
					newArr[i] = NumVal(0)
				}
				vp.Arr = newArr
			}
			vp.Arr[idx1] = val

		default:
			return ErrorVal(fmt.Sprintf("Variable '%s' ist kein Array", n.Name)), SignalError
		}

		return NullVal(), SignalNone

	case *ReturnNode:
		val := evalExpr(n.Value, env)
		if val.Kind == KindError {
			return val, SignalError
		}
		env.fnReturn = val // statt: env.Set("_fnReturn", val)
		return val, SignalReturn

	case *TryNode:
		rv, sig := evalStatements(n.TryBody, env)

		if sig == SignalError && n.CatchBody != nil {
			env.PutInternal(n.CatchVarName, rv) // rv ist der ErrorVal, der SignalError ausgelöst hat
			rv, sig = evalStatements(n.CatchBody, env)
		}

		if n.FinallyBody != nil {
			frv, fsig := evalStatements(n.FinallyBody, env)
			if fsig != SignalNone {
				return frv, fsig // Finally-Signal überschreibt das ursprüngliche (Standard-Verhalten wie in Java/C#)
			}
		}

		return rv, sig

	case *IfNode:
		executed := false
		for _, br := range n.Branches {
			cond := evalExpr(br.Cond, env)
			if cond.Kind == KindError {
				return cond, SignalError
			}
			if isTruthy(cond) {
				rv, sig := evalStatements(br.Body, env)
				if sig != SignalNone {
					return rv, sig
				}
				executed = true
				break
			}
		}
		if !executed && len(n.Else) > 0 {
			rv, sig := evalStatements(n.Else, env)
			if sig != SignalNone {
				return rv, sig
			}
		}

	case *ExitNode:
		switch n.ExitType {
		case "Function":
			return Value{}, SignalExitFunction
		case "Sub":
			return Value{}, SignalExitSub
		default:
			return Value{}, SignalExitLoop
		}

	case *ContinueNode:
		return Value{}, SignalContinueLoop

	case *MultiStmtNode:
		// HIER SPAREN WIR DEN SPEICHER:
		// n.Stmts ist schon ein Slice, wir geben es direkt weiter.
		return evalStatements(n.Stmts, env)

	case *SubNode:
		subs[n.Name] = n

	case *FuncNode:
		funcs[n.Name] = n

	case *CallNode:
		// 1. Sonderfall: Cls (Bildschirm löschen)
		if strings.EqualFold(n.Name, "Cls") {
			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				// Windows nutzt den internen CMD-Befehl 'cls'
				cmd = exec.Command("cmd", "/c", "cls")
			} else {
				// Ubuntu/Linux und macOS nutzen den 'clear' Befehl
				cmd = exec.Command("clear")
			}

			// Den Output des System-Befehls direkt an unser Terminal leiten
			cmd.Stdout = os.Stdout
			err := cmd.Run()

			if err != nil {
				// Falls 'clear' oder 'cls' fehlschlagen, nutzen wir die ANSI-Sequenz als Backup
				fmt.Print("\033[H\033[2J\033[3J")
			}
			return NullVal(), SignalNone
		}

		// 2. Reguläre Funktionsaufrufe (wie FromCSV, KindOf, etc.)
		val := evalFunctionCall(n.Name, n.Args, env)

		// Fehlerprüfung: Wenn die Funktion einen Error zurückgibt
		if val.Kind == KindError {
			return val, SignalError
		}

		// Rückgabe des Funktionswerts (oder NullVal, falls nichts zurückkommt)
		return val, SignalNone
	}
	return Value{}, SignalNone
}

func evalBinary(left Value, op TokenType, right Value) Value {
	// Numerische Vergleiche: Wandelt beide Seiten in Floats um
	lNum := toFloatSafe([]Value{left}, 0)
	rNum := toFloatSafe([]Value{right}, 0)

	switch op {
	case GT: // OK
		return Value{Kind: KindBool, Bool: lNum > rNum}
	case LT: // Geändert von LESS zu LT
		return Value{Kind: KindBool, Bool: lNum < rNum}
	case GE: // OK
		return Value{Kind: KindBool, Bool: lNum >= rNum}
	case LE: // OK
		return Value{Kind: KindBool, Bool: lNum <= rNum}
	case EQ: // Geändert von EQUAL zu EQ
		return Value{Kind: KindBool, Bool: valuesAreEqual(left, right)}
	case NEQ: // Geändert von NOTEQUAL zu NEQ
		return Value{Kind: KindBool, Bool: !valuesAreEqual(left, right)}
	case AND: // OK
		return Value{Kind: KindBool, Bool: isTruthy(left) && isTruthy(right)}
	case OR: // OK
		return Value{Kind: KindBool, Bool: isTruthy(left) || isTruthy(right)}
	}

	return ErrorVal(fmt.Sprintf("Operator %v wird im Select Case noch nicht unterstützt", op))
}

// func evalExpr(e Expr, env map[string]Value) Value {
func evalExpr(e Expr, env *Environment) Value {
	switch n := e.(type) {
	// Bool-/Null-Literale kommen als eigene AST-Knoten (im Interpreter
	// stand hier ein Value direkt im AST).
	case *BoolNode:
		return BoolVal(n.Value)
	case *NullNode:
		return NullVal()

	case *UnaryOpNode:
		rightVal := evalExpr(n.Right, env)
		switch n.Op {
		case NOT:
			return Value{Kind: KindBool, Bool: !isTruthy(rightVal)}
		case MINUS:
			if rightVal.Kind == KindNum {
				return NumVal(-rightVal.Num)
			}
			return ErrorVal("Minus erfordert eine Zahl")
		}

	case *ArrayLiteralNode:
		elements := make([]Value, 0, len(n.Elements))
		for _, expr := range n.Elements {
			val := evalExpr(expr, env)
			if val.Kind == KindError {
				return val
			}
			elements = append(elements, val)
		}
		return Value{Kind: KindArr, Arr: elements}

	case *NumberNode:
		return NumVal(n.Value)
	case *StringNode:
		return Value{Kind: KindStr, Str: n.Value}

	// Funktionsaufrufe im Ausdruck sind CallExprNode (ast.CallNode ist eine
	// Anweisung und damit kein Expr), MultiStmtNode ist ebenfalls eine
	// Anweisung -- beide Fälle sind hier nicht erreichbar.
	case *CallExprNode:
		return evalFunctionCall(n.Name, n.Args, env)

	case *MapIndexNode:
		baseVal := evalExpr(n.Base, env)
		if baseVal.Kind == KindError {
			return baseVal
		}

		if baseVal.Kind != KindMap {
			return ErrorVal("Map-Zugriff mit '[...]' ist nur auf Maps möglich")
		}

		keyVal := evalExpr(n.Key, env)
		if keyVal.Kind == KindError {
			return keyVal
		}
		key := ToString(keyVal)

		if val, ok := baseVal.Map[key]; ok {
			return val
		}
		return NilValue

	case *VarNode:
		// Knoten-Cache (im Interpreter als Felder am Knoten).
		c := nodeCacheFor(n)

		// 1. Punkt-Notation (Objekte) — einmalig prüfen und cachen
		if !c.dotChecked {
			if idx := strings.IndexByte(n.Name, '.'); idx >= 0 {
				c.isDotted = true
				c.objName = n.Name[:idx]
				c.fieldName = n.Name[idx+1:]
			}
			c.dotChecked = true
		}

		if c.isDotted {
			objVal, found := env.Get(c.objName)

			if !found {
				if _, ok := findBuiltinByModuleAndName(c.objName, c.fieldName); ok {
					return ErrorVal(fmt.Sprintf(
						"Funktion '%s.%s' muss mit '()' aufgerufen werden",
						c.objName,
						c.fieldName,
					))
				}

				return ErrorVal(fmt.Sprintf(
					"Objekt '%s' ist nicht definiert",
					c.objName,
				))
			}

			if objVal.Kind != KindObj {
				return ErrorVal(c.objName + " ist kein Object")
			}

			if objVal.Obj == nil {
				return NilValue
			}

			m := objVal.Obj.Fields
			if val, ok := m[c.fieldName]; ok {
				return val
			}
			return NilValue
		}

		// 2. Basis-Variable holen.
		//
		// Bei wiederholten Zugriffen aus derselben Environment wird die Scope-Kette
		// nicht erneut durchsucht: Environment und Pointer werden am Knoten gecacht.
		// Das ist sicher, solange Define/PutInternal/SetGlobal den Pointer einer
		// bestehenden Variable nie ersetzen, sondern durch ihn schreiben.
		// Bekannte Grenze: Wird in derselben Environment später eine gleichnamige
		// lokale Variable deklariert, liest ein bereits gecachter Knoten weiter die
		// äußere.
		var v Value

		if c.lookupEnv == env && c.defEnv != nil && c.ptr != nil {
			v = *c.ptr
		} else {
			defEnv, ptr, found := env.GetRefStrictEnv(n.Name)

			if !found {
				msg := "Variable '%s' ist nicht als Public (global) definiert"
				if env.parent != nil {
					msg = "Variable '%s' ist weder lokal (Dim) noch als Public (global) definiert"
				}
				return ErrorVal(fmt.Sprintf(msg, n.Name))
			}

			c.lookupEnv = env
			c.defEnv = defEnv
			c.ptr = ptr
			v = *ptr
		}

		if v.Kind == KindNil {
			return ErrorVal("undefined variable: " + n.Name)
		}

		// 3. Index-Zugriff entfällt: vbxc/ast.VarNode trägt keine Indizes.
		// Index1/Index2 waren im Interpreter ohnehin toter Code (der Parser
		// setzt sie nie) -- Array- und Map-Zugriffe laufen über CallExprNode.

		// 4. Einfache Variable (kein Index)
		return v

	case *BinOpNode:
		c := nodeCacheFor(n)
		if c.isConst {
			return c.value
		}

		// Constant Folding: zwei Zahlenliterale mit + - * einmalig verrechnen.
		if left, ok := n.Left.(*NumberNode); ok {
			if right, ok := n.Right.(*NumberNode); ok {
				switch n.Op {
				case PLUS, MINUS, MUL:
					c.value = calculateBinaryOp(n.Op, NumVal(left.Value), NumVal(right.Value))
					c.isConst = true
					return c.value
				}
			}
		}

		// Short-Circuit: rechte Seite nur auswerten, wenn nötig.
		if n.Op == AND || n.Op == OR {
			l := evalExpr(n.Left, env)
			if l.Kind == KindError {
				return l
			}
			lt := isTruthy(l)
			if (n.Op == AND && !lt) || (n.Op == OR && lt) {
				return BoolVal(lt)
			}
			r := evalExpr(n.Right, env)
			if r.Kind == KindError {
				return r
			}
			return BoolVal(isTruthy(r))
		}

		l := evalExpr(n.Left, env)
		if l.Kind == KindError {
			return l
		}
		r := evalExpr(n.Right, env)
		if r.Kind == KindError {
			return r
		}

		switch n.Op {
		case PLUS:
			// Zwei Strings werden als Strings verkettet.
			if l.Kind == KindStr && r.Kind == KindStr {
				return StrVal(l.Str + r.Str)
			}

			// Zahlen werden addiert.
			if ln, ok := tryNumber(l); ok {
				if rn, ok := tryNumber(r); ok {
					return NumVal(ln + rn)
				}
			}

			return StrVal(ToString(l) + ToString(r))

		case MINUS, MUL, DIV:
			if l.Kind == KindNum && r.Kind == KindNum {
				return calculateBinaryOp(n.Op, l, r)
			}
			ln, err := requireNumber(l, n.Op.String())
			if err.Kind == KindError {
				return err
			}
			rn, err := requireNumber(r, n.Op.String())
			if err.Kind == KindError {
				return err
			}
			return calculateBinaryOp(n.Op, NumVal(ln), NumVal(rn))

		case AMP:
			return StrVal(ToString(l) + ToString(r))

		case EQ, NEQ:
			var eq bool
			switch {
			case l.Kind == KindBool && r.Kind == KindBool:
				eq = l.Bool == r.Bool
			case l.Kind == KindStr || r.Kind == KindStr:
				eq = ToString(l) == ToString(r)
			case l.Kind == KindNum && r.Kind == KindNum:
				eq = l.Num == r.Num
			default: // gemischte Typen (z.B. Bool vs Num)
				eq = valuesAreEqual(l, r)
			}
			return BoolVal(eq == (n.Op == EQ))

		case LT, GT, LE, GE:
			if l.Kind == KindNum && r.Kind == KindNum {
				return BoolVal(compareNum(n.Op, l.Num, r.Num))
			}
			ln, err := requireNumber(l, n.Op.String())
			if err.Kind == KindError {
				return err
			}
			rn, err := requireNumber(r, n.Op.String())
			if err.Kind == KindError {
				return err
			}
			return BoolVal(compareNum(n.Op, ln, rn))

		default:
			return ErrorVal(fmt.Sprintf("Unbekannter Operator: %v", n.Op))
		}

	default:
		// Falls ein Operator durch den Parser gerutscht ist,
		// den der Evaluator noch nicht kennt.
		return ErrorVal(fmt.Sprintf("Unbekannter Ausdruckstyp: %T", e))

	} // Ende switch n.Op

	// Falls wir hier landen, ist etwas im Programmfluss
	// fundamental schiefgelaufen (sollte theoretisch nie passieren)
	return ErrorVal("Kritischer Fehler in der Ausdrucks-Auswertung")
}

// Hilfsfunktion: Alle FuncNode/SubNode rekursiv registrieren. Die Zahl der
// Pflichtparameter wird beim Aufruf aus der Parameterliste berechnet
// (requiredParams) -- die AST-Knoten tragen in vbxc keinen Zustand mehr.
func registerFuncsAndSubs(stmts []Stmt) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *FuncNode:
			funcs[n.Name] = n

		case *SubNode:
			subs[n.Name] = n

		case *MultiStmtNode:
			registerFuncsAndSubs(n.Stmts)
		}
	}
}

func valuesAreEqual(l, r Value) bool {
	// 1. Wenn Typen gleich sind: Direkter Vergleich
	if l.Kind == r.Kind {
		switch l.Kind {
		case KindNum:
			return l.Num == r.Num
		case KindStr:
			return strings.EqualFold(l.Str, r.Str) // Case-Insensitive wie in VB!
		case KindBool:
			return l.Bool == r.Bool
		case KindNull, KindNil:
			return true
		}
	}

	// 2. Cross-Type Vergleich: Zahl vs String
	// Erlaubt: Case 10 ... wenn x = "10"
	if (l.Kind == KindNum && r.Kind == KindStr) || (l.Kind == KindStr && r.Kind == KindNum) {
		return toNumVal(l) == toNumVal(r)
	}

	// 3. Boolean Checks (VB: -1 ist True, 0 ist False)
	if l.Kind == KindBool || r.Kind == KindBool {
		return isTruthy(l) == isTruthy(r)
	}

	return false
}

// Hilfsfunktion
func isPrimitive(v Value) bool {
	return v.Kind == KindNum || v.Kind == KindStr || v.Kind == KindBool
}
