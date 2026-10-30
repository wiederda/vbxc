// Package vm führt Bytecode aus.
//
// Fehlermodell (wie im Interpreter): Ausdrücke liefern Fehler als Wert auf dem
// Stack. Erst OpCheck (Anweisungsebene) macht daraus einen Skript-Fehler, der
// Run beendet. Phase 4 hängt hier einen Handler-Stack für Try/Catch ein.
package vm

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"

	"vbxc/bytecode"
	"vbxc/jit"
	"vbxc/plugin"
	"vbxc/stdlib"
	"vbxc/token"
	"vbxc/value"
)

type ScriptError struct {
	Msg  string
	File string
	Line int
}

func (e *ScriptError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s: Zeile %d: %s", e.File, e.Line, e.Msg)
	}
	return fmt.Sprintf("Zeile %d: %s", e.Line, e.Msg)
}

type VM struct {
	Out io.Writer
	Env *Environment

	// jit ist die optionale Übersetzung des Chunks in ausführbaren Speicher
	// (nil = interpretiert). Die Semantik ist in beiden Fällen dieselbe.
	jit *jit.Program

	stack      []value.Value
	deprecated map[string]bool

	funcs   map[string]*bytecode.Func // registrierte Functions
	subs    map[string]*bytecode.Func // registrierte Subs
	depth   int                       // aktuelle Aufruftiefe
	callPos bytecode.Pos              // Position des letzten OpCall (für Meldungen)
	callIn  *bytecode.Chunk
}

// Maximale Verschachtelung von Function/Sub-Aufrufen. Anders als im
// Interpreter endet eine Endlosrekursion damit in einem Skript-Fehler statt
// im Go-Stack-Overflow.
const maxCallDepth = 10000

func New() *VM {
	return &VM{
		Out:        stdlib.OutputWriter(os.Stdout),
		Env:        NewEnvironment(nil),
		deprecated: make(map[string]bool),
		funcs:      make(map[string]*bytecode.Func),
		subs:       make(map[string]*bytecode.Func),
	}
}

func (vm *VM) push(v value.Value) { vm.stack = append(vm.stack, v) }

// SetJIT aktiviert die Ausführung in ausführbarem Speicher (x86-64). nil
// schaltet ab - die VM arbeitet dann interpretiert (identische Semantik).
func (vm *VM) SetJIT(p *jit.Program) {
	if p != nil {
		vm.jit = p
	}
}

// entryTable liefert die JIT-Einstiege eines Code-Arrays, nil ohne JIT.
func (vm *VM) entryTable(code []bytecode.Instr) []*jit.Entry {
	if vm.jit == nil {
		return nil
	}
	return vm.jit.TableFor(code)
}

func (vm *VM) pop() value.Value {
	n := len(vm.stack) - 1
	v := vm.stack[n]
	vm.stack = vm.stack[:n]
	return v
}

func (vm *VM) top() value.Value { return vm.stack[len(vm.stack)-1] }

// Run führt den Chunk aus. Ein Skript-Fehler kommt als *ScriptError zurück.
func (vm *VM) Run(c *bytecode.Chunk) (err error) {
	// Ein Panic ist ein Fehler im Compiler oder in beschädigtem Bytecode,
	// kein Skript-Fehler - als solcher melden statt abstürzen. Die Tiefenbegrenzung
	// meldet sich ebenfalls so (als *ScriptError).
	defer func() {
		if r := recover(); r != nil {
			if se, ok := r.(*ScriptError); ok {
				err = se
				return
			}
			err = fmt.Errorf("interner Fehler in der VM: %v", r)
		}
	}()

	// Module aus "#use" nachladen: Eingebaute Namespaces überspringt
	// LoadModules; unbekannte Namen müssen als WASM-Plugin vorliegen
	// (plugins/<name>.wasm,Suchpfad siehe Dokumentation des Pakets plugin).
	// Gilt für alle Bytecode-Pfade (vbxc datei.vbb, vbxr, JIT an/aus);
	// lädt nichts, wenn keine Module hinterlegt sind.
	if err := plugin.LoadModules(c.Modules); err != nil {
		return err
	}

	vm.stack = vm.stack[:0]
	vm.depth = 0
	vm.funcs = make(map[string]*bytecode.Func)
	vm.subs = make(map[string]*bytecode.Func)
	for _, idx := range c.Preregister {
		vm.register(c, int(idx))
	}

	_, err = vm.exec(c, c.Code, c.Pos, vm.Env, 0)
	return err
}

func (vm *VM) register(c *bytecode.Chunk, idx int) {
	f := &c.Funcs[idx]
	if f.IsFunc {
		vm.funcs[f.Name] = f
	} else {
		vm.subs[f.Name] = f
	}
}

// execResult: early=true heißt, der Aufruf endet mit val als Ergebnis (nicht
// mit dem normalen Rückgabewert) - so, wenn ein Standardwert ein Fehlerwert ist.
type execResult struct {
	early bool
	val   value.Value
}

// exec führt eine Einheit (Hauptprogramm oder Function/Sub) in der Umgebung env
// aus. argc ist die Zahl der übergebenen Argumente (für optionale Parameter).
func (vm *VM) exec(c *bytecode.Chunk, code []bytecode.Instr, pos []bytecode.Pos, env *Environment, argc int) (res execResult, err error) {
	base := len(vm.stack)
	fail := func(at int, msg string) error {
		p := pos[at]
		file := ""
		if int(p.File) < len(c.Files) {
			file = c.Files[p.File]
		}
		return &ScriptError{Msg: msg, File: file, Line: int(p.Line)}
	}
	ip := 0

	// JIT: die Einstiegstabelle wird pro Code-Array einmalig geholt. Nach
	// einem Fenster wird genau EINE Instruktion interpretiert, bevor erneut
	// eingestiegen wird - sonst entstünde eine Endlosschleife, wenn der
	// Resume- ip selbst ein Einstieg der Region ist (skip-once).
	tab := vm.entryTable(code)
	skipEntry := false

	for {
		if len(tab) > 0 && !skipEntry {
			if e := tab[ip]; e != nil {
				ip = int(e.Run(env, &vm.stack))
				skipEntry = true
				continue
			}
		} else {
			skipEntry = false
		}

		in := code[ip]
		ip++

		switch in.Op {

		case bytecode.OpHalt:
			// Invariante: Anweisungen hinterlassen nichts auf dem Stack.
			// Ein Rest hier ist ein Compiler-Fehler, kein Skript-Fehler.
			if len(vm.stack) != base {
				return res, fmt.Errorf("interner Fehler: %d Werte auf dem Stack nach Programmende", len(vm.stack)-base)
			}
			return res, nil

		// ---------- Konstanten und Stack ----------
		case bytecode.OpConst:
			vm.push(c.Consts[in.A])
		case bytecode.OpNull:
			vm.push(value.NullVal())
		case bytecode.OpTrue:
			vm.push(value.BoolVal(true))
		case bytecode.OpFalse:
			vm.push(value.BoolVal(false))
		case bytecode.OpPop:
			vm.pop()
		case bytecode.OpPopN:
			vm.stack = vm.stack[:len(vm.stack)-int(in.A)]

		// ---------- Variablen ----------
		case bytecode.OpLoad:
			vm.push(vm.loadVar(env, c.Names[in.A]))

		case bytecode.OpLoadField:
			vm.push(vm.loadField(env, c.Names[in.A], c.Names[in.B]))

		case bytecode.OpLoadForUpdate:
			name := c.Names[in.A]
			defEnv, ptr, found := env.GetRefStrictEnv(name)
			if !found {
				return res, fail(ip-1, "undefined variable: "+name)
			}
			if defEnv.isConstHere(name) {
				return res, fail(ip-1, constMsg(name))
			}
			vm.push(*ptr)

		case bytecode.OpDeclare:
			name := c.Names[in.A]
			val := vm.pop()
			if env.isConstHere(name) {
				return res, fail(ip-1, fmt.Sprintf("Fehler: '%s' wurde bereits als Konstante deklariert", name))
			}
			if in.B&bytecode.FlagConst != 0 {
				env.DefineConst(name, val)
				env.MarkConst(name)
			} else {
				env.Define(name, val, in.B&bytecode.FlagInLoop != 0)
			}

		case bytecode.OpStore:
			name := c.Names[in.A]
			val := vm.pop()
			defEnv, ptr, found := env.GetRefStrictEnv(name)
			if !found {
				scopeInfo := "Public (global)"
				if env.parent != nil {
					scopeInfo = "lokal (Dim) oder Public (global)"
				}
				return res, fail(ip-1, fmt.Sprintf(
					"Variable '%s' muss erst mit Dim oder Public deklariert werden (nicht gefunden in %s)",
					name, scopeInfo))
			}
			if defEnv.isConstHere(name) {
				return res, fail(ip-1, constMsg(name))
			}
			*ptr = val

		case bytecode.OpStoreRef:
			name := c.Names[in.A]
			val := vm.pop()
			_, ptr, found := env.GetRefStrictEnv(name)
			if !found {
				return res, fail(ip-1, "undefined variable: "+name)
			}
			*ptr = val

		case bytecode.OpSetGlobal:
			env.SetGlobal(c.Names[in.A], vm.pop())

		// ---------- Rechnen / Vergleichen ----------
		case bytecode.OpAdd:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Add(l, r))
		case bytecode.OpSub:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Arith(token.MINUS, l, r))
		case bytecode.OpMul:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Arith(token.MUL, l, r))
		case bytecode.OpDiv:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Arith(token.DIV, l, r))
		case bytecode.OpConcat:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Concat(l, r))
		case bytecode.OpEq:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Equal(l, r, false))
		case bytecode.OpNeq:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Equal(l, r, true))
		case bytecode.OpLt:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Compare(token.LT, l, r))
		case bytecode.OpGt:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Compare(token.GT, l, r))
		case bytecode.OpLe:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Compare(token.LE, l, r))
		case bytecode.OpGe:
			r, l := vm.pop(), vm.pop()
			vm.push(value.Compare(token.GE, l, r))
		case bytecode.OpCalc:
			r, l := vm.pop(), vm.pop()
			vm.push(value.CalculateBinaryOp(token.Type(in.A), l, r))

		// ---------- Logik ----------
		case bytecode.OpNot:
			vm.push(value.Not(vm.pop()))
		case bytecode.OpNeg:
			vm.push(value.Neg(vm.pop()))
		case bytecode.OpTruthy:
			r := vm.pop()
			if r.Kind == value.KindError {
				vm.push(r)
			} else {
				vm.push(value.BoolVal(value.IsTruthy(r)))
			}
		case bytecode.OpShortCircuit:
			l := vm.pop()
			lt := value.IsTruthy(l)
			isOr := in.B == 1
			if (!isOr && !lt) || (isOr && lt) {
				vm.push(value.BoolVal(lt))
				ip = int(in.A)
			}

		// ---------- Sprünge ----------
		case bytecode.OpJump:
			ip = int(in.A)
		case bytecode.OpJumpIfFalse:
			if !value.IsTruthy(vm.pop()) {
				ip = int(in.A)
			}
		case bytecode.OpJumpIfTrue:
			if value.IsTruthy(vm.pop()) {
				ip = int(in.A)
			}
		case bytecode.OpJumpIfError:
			if t := vm.top(); t.Kind == value.KindError {
				if below := int(in.B); below > 0 {
					n := len(vm.stack)
					vm.stack[n-1-below] = t
					vm.stack = vm.stack[:n-below]
				}
				ip = int(in.A)
			}
		case bytecode.OpCheck:
			if t := vm.top(); t.Kind == value.KindError {
				return res, fail(ip-1, t.Str)
			}

		// ---------- Aufrufe ----------
		case bytecode.OpCall:
			n := int(in.B)
			// The call consumes its arguments synchronously. Borrow the argument
			// window from the VM stack instead of allocating and copying a slice
			// for every call (notably expensive for recursion and arr(i)).
			stackLen := len(vm.stack)
			argStart := stackLen - n
			args := vm.stack[argStart:stackLen]
			vm.stack = vm.stack[:argStart]
			vm.callPos, vm.callIn = pos[ip-1], c
			vm.push(vm.call(c, env, c.Names[in.A], args))

		case bytecode.OpIsError:
			vm.push(value.BoolVal(vm.pop().Kind == value.KindError))

		case bytecode.OpErrorText:
			if v := vm.pop(); v.Kind == value.KindError {
				vm.push(value.StrVal(v.Str))
			} else {
				vm.push(value.StrVal(""))
			}

		// ---------- Arrays / Maps ----------
		case bytecode.OpMakeArray:
			n := int(in.A)
			elems := make([]value.Value, n)
			copy(elems, vm.stack[len(vm.stack)-n:])
			vm.stack = vm.stack[:len(vm.stack)-n]
			vm.push(value.Value{Kind: value.KindArr, Arr: elems})

		case bytecode.OpMapBase:
			if vm.top().Kind != value.KindMap {
				vm.stack[len(vm.stack)-1] = value.ErrorVal("Map-Zugriff mit '[...]' ist nur auf Maps möglich")
				ip = int(in.A)
			}

		case bytecode.OpMapIndex:
			key := value.ToString(vm.pop())
			base := vm.pop()
			if val, ok := base.Map[key]; ok {
				vm.push(val)
			} else {
				vm.push(value.NilValue)
			}

		// ---------- For ----------
		case bytecode.OpForInit:
			n := len(vm.stack)
			vm.stack[n-2] = value.NumVal(value.ToNum(vm.stack[n-2]))
			vm.stack[n-1] = value.NumVal(value.ToNum(vm.stack[n-1]))
			if c.Consts[in.B].Num == 0 {
				return res, fail(ip-1, "FOR mit STEP 0 ist nicht erlaubt")
			}
			env.GetRef(c.Names[in.A])

		case bytecode.OpForTest:
			n := len(vm.stack)
			cur, end := vm.stack[n-2].Num, vm.stack[n-1].Num
			var more bool
			if in.C == 1 {
				more = cur <= end
			} else {
				more = cur >= end
			}
			if !more {
				ip = int(in.A)
				continue
			}
			p := env.GetRef(c.Names[in.B])
			p.Kind = value.KindNum
			p.Num = cur

		case bytecode.OpForStep:
			vm.stack[len(vm.stack)-2].Num += c.Consts[in.A].Num

		// ---------- Arrays ----------
		case bytecode.OpDimArray, bytecode.OpPublicArray:
			dims := int(in.B)
			var s1, s2 value.Value
			if dims == 2 {
				s2 = vm.pop()
			}
			if dims >= 1 {
				s1 = vm.pop()
			}
			arr, msg := makeArray(dims, s1, s2)
			if msg != "" {
				return res, fail(ip-1, msg)
			}
			if in.Op == bytecode.OpDimArray {
				env.Define(c.Names[in.A], arr, in.C == 1)
			} else {
				env.SetGlobal(c.Names[in.A], arr)
			}

		case bytecode.OpArrCheck:
			name := c.Names[in.A]
			defEnv, _, found := env.GetRefStrictEnv(name)
			if !found {
				return res, fail(ip-1, "Variable nicht deklariert: "+name)
			}
			if defEnv.isConstHere(name) {
				return res, fail(ip-1, constMsg(name))
			}

		case bytecode.OpToIndex:
			idx, errV := value.ToIndex(vm.pop())
			if errV.Kind == value.KindError {
				return res, fail(ip-1, errV.Str)
			}
			vm.push(value.NumVal(float64(idx)))

		case bytecode.OpArrGuard2D:
			if _, ptr, found := env.GetRefStrictEnv(c.Names[in.A]); !found || ptr.Kind != value.KindArr2D {
				vm.push(value.NilValue) // Platzhalter für den (nicht ausgewerteten) zweiten Index
				ip = int(in.B)
			}

		case bytecode.OpArrStore:
			name := c.Names[in.A]
			hasIdx2 := in.B == 1
			var idx2v value.Value
			if hasIdx2 {
				idx2v = vm.pop()
			}
			idx1 := int(vm.pop().Num)
			val := vm.pop()

			_, vp, found := env.GetRefStrictEnv(name)
			if !found {
				return res, fail(ip-1, "Variable nicht deklariert: "+name)
			}
			switch vp.Kind {
			case value.KindArr2D:
				if !hasIdx2 || idx2v.Kind != value.KindNum {
					return res, fail(ip-1, "Zweiter Index fehlt")
				}
				idx2 := int(idx2v.Num)
				if idx1 < 0 || idx1 >= len(vp.Arr2D) || idx2 < 0 || idx2 >= len(vp.Arr2D[idx1]) {
					return res, fail(ip-1, fmt.Sprintf("Matrix-Index (%d,%d) außerhalb der Grenzen", idx1, idx2))
				}
				vp.Arr2D[idx1][idx2] = val

			case value.KindArr:
				if idx1 < 0 {
					return res, fail(ip-1, "Index negativ")
				}
				if idx1 >= len(vp.Arr) {
					newArr := make([]value.Value, idx1+1)
					copy(newArr, vp.Arr)
					for i := len(vp.Arr); i < len(newArr); i++ {
						newArr[i] = value.NumVal(0)
					}
					vp.Arr = newArr
				}
				vp.Arr[idx1] = val

			default:
				return res, fail(ip-1, fmt.Sprintf("Variable '%s' ist kein Array", name))
			}

		// ---------- For Each ----------
		case bytecode.OpEachInit:
			iter := vm.top()
			env.GetRef(c.Names[in.A])
			if in.B > 0 {
				env.GetRef(c.Names[in.B-1])
			}
			keys := value.NullVal()
			switch iter.Kind {
			case value.KindArr, value.KindArr2D:
			case value.KindMap:
				names := make([]string, 0, len(iter.Map))
				for k := range iter.Map {
					names = append(names, k)
				}
				sort.Strings(names)
				list := make([]value.Value, len(names))
				for i, k := range names {
					list[i] = value.StrVal(k)
				}
				keys = value.Value{Kind: value.KindArr, Arr: list}
			default:
				return res, fail(ip-1, "FOR EACH: Erwartet Array oder Map")
			}
			vm.push(value.NumVal(0))
			vm.push(keys)

		case bytecode.OpEachNext:
			n := len(vm.stack)
			iter, keys := vm.stack[n-3], vm.stack[n-1]
			i := int(vm.stack[n-2].Num)

			var length int
			switch iter.Kind {
			case value.KindArr:
				length = len(iter.Arr)
			case value.KindArr2D:
				length = len(iter.Arr2D)
			case value.KindMap:
				length = len(keys.Arr)
			}
			if i >= length {
				ip = int(in.A)
				continue
			}

			var key, val value.Value
			switch iter.Kind {
			case value.KindArr:
				key, val = value.NumVal(float64(i)), iter.Arr[i]
			case value.KindArr2D:
				key, val = value.NumVal(float64(i)), value.Value{Kind: value.KindArr, Arr: iter.Arr2D[i]}
			case value.KindMap:
				k := keys.Arr[i].Str
				key, val = value.StrVal(k), iter.Map[k]
			}
			if in.C > 0 {
				*env.GetRef(c.Names[in.B]) = key
				*env.GetRef(c.Names[in.C-1]) = val
			} else {
				*env.GetRef(c.Names[in.B]) = val
			}
			vm.stack[n-2].Num = float64(i + 1)

		// ---------- Select Case ----------
		case bytecode.OpDup:
			vm.push(vm.top())

		case bytecode.OpSelectDispatch:
			if vm.top().Kind == value.KindBool {
				continue // Bool geht immer über den linearen Scan
			}
			t := &c.Selects[in.A]
			v := vm.pop()
			if idx, found := t.Lookup(v); found {
				ip = int(t.Targets[idx])
			} else {
				ip = int(t.Default)
			}

		case bytecode.OpCaseEq:
			cond, v := vm.pop(), vm.pop()
			vm.push(value.BoolVal(value.ValuesAreEqual(v, cond)))

		case bytecode.OpCaseRange:
			high, low, v := vm.pop(), vm.pop(), vm.pop()
			target := value.ToFloatSafe([]value.Value{v}, 0)
			lo := value.ToFloatSafe([]value.Value{low}, 0)
			hi := value.ToFloatSafe([]value.Value{high}, 0)
			vm.push(value.BoolVal(target >= lo && target <= hi))

		case bytecode.OpCaseIs:
			cmp, v := vm.pop(), vm.pop()
			res := value.EvalBinary(v, token.Type(in.A), cmp)
			vm.push(value.BoolVal(res.Kind == value.KindBool && res.Bool))

		// ---------- Sub/Function ----------
		case bytecode.OpDefine:
			vm.register(c, int(in.A))

		case bytecode.OpSetReturn:
			env.fnReturn = vm.pop()

		case bytecode.OpReturn:
			if in.A == 1 {
				env.fnReturn = vm.pop()
			}
			vm.stack = vm.stack[:base]
			return res, nil

		case bytecode.OpJumpIfArg:
			if argc > int(in.B) {
				ip = int(in.A)
			}

		case bytecode.OpParamDefault:
			v := vm.pop()
			if v.Kind == value.KindError {
				vm.stack = vm.stack[:base]
				return execResult{early: true, val: v}, nil
			}
			env.PutInternal(c.Names[in.A], v)

		// ---------- Ausgabe ----------
		case bytecode.OpPrint:
			var color value.Value
			hasColor := in.A == 1
			if hasColor {
				color = vm.pop()
			}
			out := value.ToString(vm.pop())
			if hasColor {
				if color.Kind == value.KindStr {
					fmt.Fprintf(vm.Out, "%s%s\033[0m\n", color.Str, out)
					continue
				}
				if color.Kind == value.KindNum {
					cn := int(color.Num)
					fmt.Fprintf(vm.Out, "\033[%dm%s\033[0m\n", 30+(cn%10), out)
					continue
				}
			}
			fmt.Fprint(vm.Out, out+"\033[0m\n")

		case bytecode.OpCls:
			clearScreen()

		default:
			return res, fail(ip-1, fmt.Sprintf("unbekannter Opcode %v", in.Op))
		}
	}
}

func constMsg(name string) string {
	return fmt.Sprintf("Fehler: Es wird versucht die Konstante '%s' zu verändern", name)
}

// loadVar: einfache Variable (ohne Punkt), Meldungen wie im Interpreter.
func (vm *VM) loadVar(env *Environment, name string) value.Value {
	_, ptr, found := env.GetRefStrictEnv(name)
	if !found {
		msg := "Variable '%s' ist nicht als Public (global) definiert"
		if env.parent != nil {
			msg = "Variable '%s' ist weder lokal (Dim) noch als Public (global) definiert"
		}
		return value.ErrorVal(fmt.Sprintf(msg, name))
	}
	if ptr.Kind == value.KindNil {
		return value.ErrorVal("undefined variable: " + name)
	}
	return *ptr
}

// loadField: "obj.feld" - Objektfeld lesen.
func (vm *VM) loadField(env *Environment, obj, field string) value.Value {
	objVal, found := env.Get(obj)
	if !found {
		if _, ok := stdlib.FindByModuleAndName(obj, field); ok {
			return value.ErrorVal(fmt.Sprintf("Funktion '%s.%s' muss mit '()' aufgerufen werden", obj, field))
		}
		return value.ErrorVal(fmt.Sprintf("Objekt '%s' ist nicht definiert", obj))
	}
	if objVal.Kind != value.KindObj {
		return value.ErrorVal(obj + " ist kein Object")
	}
	if objVal.Obj == nil {
		return value.NilValue
	}
	if val, ok := objVal.Obj.Fields[field]; ok {
		return val
	}
	return value.NilValue
}

// call: Reihenfolge wie evalFunctionCall - erst Array/Map/Objekt-Zugriff auf
// eine gleichnamige Variable, dann Builtins, dann Functions, dann Subs.
// (Ein Builtin gleichen Namens verdeckt also eine eigene Function.)
func (vm *VM) call(c *bytecode.Chunk, env *Environment, name string, args []value.Value) value.Value {
	if _, v, found := env.GetRefStrictEnv(name); found {
		if r, ok := indexAccess(v, args); ok {
			return r
		}
	}

	if info, ok := stdlib.Lookup(name); ok {
		if info.Deprecated != "" && !vm.deprecated[name] {
			vm.deprecated[name] = true
			fmt.Fprintf(os.Stderr, "\033[33m[WARNUNG]\033[0m '%s' ist veraltet - benutze stattdessen %s\n", name, info.Deprecated)
		}
		return info.Fn(args)
	}

	if f, ok := vm.funcs[name]; ok {
		return vm.callUser(c, env, f, args)
	}
	if f, ok := vm.subs[name]; ok {
		return vm.callUser(c, env, f, args)
	}

	return value.ErrorVal(fmt.Sprintf("Funktion oder Sub '%s' nicht gefunden", name))
}

// callUser ruft eine Function/Sub auf. Ein Fehler im Körper kommt - wie im
// Interpreter - als Fehlerwert "Fehler in Funktion 'x': ..." beim Aufrufer an,
// der ihn mit seiner nächsten Anweisung zum Skript-Fehler macht.
func (vm *VM) callUser(c *bytecode.Chunk, env *Environment, f *bytecode.Func, args []value.Value) value.Value {
	kind := "Sub"
	if f.IsFunc {
		kind = "Funktion"
	}

	if len(args) < int(f.Required) || len(args) > len(f.Params) {
		return value.ErrorVal(fmt.Sprintf("%s '%s' erwartet %d bis %d Argumente, erhalten: %d",
			kind, f.Name, f.Required, len(f.Params), len(args)))
	}

	if vm.depth >= maxCallDepth {
		file := ""
		if vm.callIn != nil && int(vm.callPos.File) < len(vm.callIn.Files) {
			file = vm.callIn.Files[vm.callPos.File]
		}
		panic(&ScriptError{
			Msg:  fmt.Sprintf("Maximale Aufruftiefe (%d) überschritten in %s '%s'", maxCallDepth, kind, f.Name),
			File: file,
			Line: int(vm.callPos.Line),
		})
	}

	local := NewEnvironmentWithCapacity(env.root(), len(f.Params))
	if f.IsFunc {
		local.fnReturn = value.NumVal(0)
		local.currentFuncName = f.Name
	}
	for i, a := range args {
		local.PutInternal(f.Params[i], a)
	}

	base := len(vm.stack)
	vm.depth++
	res, err := vm.exec(c, f.Code, f.Pos, local, len(args))
	vm.depth--
	vm.stack = vm.stack[:base]

	if err != nil {
		se, ok := err.(*ScriptError)
		if !ok {
			panic(err) // interner Fehler: bis Run durchreichen
		}
		return value.ErrorVal(fmt.Sprintf("Fehler in %s '%s': %s", kindLabel(f), f.Name, se.Msg))
	}
	if res.early {
		return res.val
	}
	if f.IsFunc {
		return local.fnReturn
	}
	return value.NumVal(0)
}

func kindLabel(f *bytecode.Func) string {
	if f.IsFunc {
		return "Funktion"
	}
	return "Sub"
}

// indexAccess bildet den Array-Zugriff-Block am Anfang von evalFunctionCall nach.
func indexAccess(v *value.Value, args []value.Value) (value.Value, bool) {
	switch v.Kind {
	case value.KindArr, value.KindArr2D, value.KindMap, value.KindObj:
	default:
		return value.Value{}, false
	}

	if v.Kind == value.KindObj && len(args) == 1 {
		if v.Obj == nil {
			return value.NilValue, true
		}
		if val, ok := v.Obj.Fields[value.ToString(args[0])]; ok {
			return val, true
		}
		return value.NilValue, true
	}

	if v.Kind == value.KindMap && len(args) == 1 {
		if val, ok := v.Map[value.ToString(args[0])]; ok {
			return val, true
		}
		return value.NilValue, true
	}

	if len(args) == 1 {
		idx := 0
		if args[0].Kind == value.KindNum {
			idx = int(args[0].Num)
		} else {
			idx = int(value.ToNum(args[0]))
		}
		if v.Kind == value.KindArr {
			if idx < 0 || idx >= len(v.Arr) {
				return value.ErrorVal("Index out of bounds"), true
			}
			return v.Arr[idx], true
		}
		if v.Kind == value.KindArr2D {
			if idx < 0 || idx >= len(v.Arr2D) {
				return value.ErrorVal("Index out of bounds"), true
			}
			return value.Value{Kind: value.KindArr, Arr: v.Arr2D[idx]}, true
		}
	}

	if len(args) == 2 && v.Kind == value.KindArr2D {
		i1, i2 := int(value.ToNum(args[0])), int(value.ToNum(args[1]))
		if i1 < 0 || i1 >= len(v.Arr2D) || i2 < 0 || i2 >= len(v.Arr2D[0]) {
			return value.ErrorVal("Matrix-Index out of bounds"), true
		}
		return v.Arr2D[i1][i2], true
	}

	return value.Value{}, false
}

func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		fmt.Print("\033[H\033[2J\033[3J")
	}
}

// makeArray erzeugt das (mit 0 gefüllte) Array für Dim/Public. Größen sind wie
// im Interpreter "obere Grenze + 1". Anders als dort führt eine negative oder
// absurd große Größe zu einem Skript-Fehler statt zu einer Panic.
func makeArray(dims int, s1, s2 value.Value) (value.Value, string) {
	const maxCells = 1 << 28

	size1 := 0
	if dims >= 1 {
		size1 = int(value.ToNum(s1)) + 1
	}
	if dims == 2 {
		size2 := int(value.ToNum(s2)) + 1
		if size1 < 0 || size2 < 0 || size1*size2 > maxCells {
			return value.Value{}, "Ungültige Array-Größe"
		}
		matrix := make([][]value.Value, size1)
		for i := range matrix {
			row := make([]value.Value, size2)
			for j := range row {
				row[j] = value.NumVal(0)
			}
			matrix[i] = row
		}
		return value.Value{Kind: value.KindArr2D, Arr2D: matrix}, ""
	}

	if size1 < 0 || size1 > maxCells {
		return value.Value{}, "Ungültige Array-Größe"
	}
	arr := make([]value.Value, size1)
	for i := range arr {
		arr[i] = value.NumVal(0)
	}
	return value.Value{Kind: value.KindArr, Arr: arr}, ""
}
