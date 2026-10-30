// Package compiler übersetzt den AST in Bytecode (Phase 1).
//
// Fehlerwerte: Ausdrücke liefern Fehler als Wert (wie evalExpr im Interpreter),
// erst Anweisungen machen daraus einen Skript-Fehler (OpCheck). Damit die
// Auswertungsreihenfolge identisch bleibt (rechte Seite wird nicht mehr
// ausgewertet, wenn links schon ein Fehler steckt), springt OpJumpIfError
// über den Rest des Ausdrucks.
package compiler

import (
	"fmt"
	"strings"

	"vbxc/ast"
	"vbxc/bytecode"
	"vbxc/token"
	"vbxc/value"
)

type Error struct {
	File string
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s: Zeile %d: %s", e.File, e.Line, e.Msg)
	}
	return fmt.Sprintf("Zeile %d: %s", e.Line, e.Msg)
}

// loop merkt sich Sprungziele für Exit/Continue der innersten Schleife.
type loop struct {
	breakJumps    []int
	continueJumps []int
	continueTo    int // >= 0, wenn das Ziel schon feststeht (While-Kopf)
	bodyDepth     int // Stack-Temporaries innerhalb des Schleifenkörpers
}

type compiler struct {
	chunk *bytecode.Chunk
	names map[string]int32
	files map[string]int32

	// Die Einheit, die gerade übersetzt wird (Hauptprogramm oder eine Function/Sub)
	code     []bytecode.Instr
	pos      []bytecode.Pos
	loops    []*loop
	depth    int    // Anzahl Stack-Slots, die Anweisungen gerade belegen (For: 2)
	nesting  int    // Verschachtelung in Blöcken; 0 = oberste Ebene (wird vorab registriert)
	funcName string // Name der umgebenden Function ("" im Hauptprogramm und in Subs)
}

// Compile übersetzt eine Anweisungsliste in einen Chunk.
func Compile(name string, stmts []ast.Stmt) (chunk *bytecode.Chunk, err error) {
	c := &compiler{
		chunk: &bytecode.Chunk{Name: name},
		names: make(map[string]int32),
		files: make(map[string]int32),
	}

	defer func() {
		if r := recover(); r != nil {
			if ce, ok := r.(*Error); ok {
				chunk, err = nil, ce
				return
			}
			panic(r)
		}
	}()

	c.stmts(stmts)
	c.emit(ast.Loc{}, bytecode.OpHalt)
	c.chunk.Code, c.chunk.Pos = c.code, c.pos
	if verr := c.chunk.Validate(); verr != nil {
		return nil, fmt.Errorf("interner Compiler-Fehler: %w", verr)
	}
	return c.chunk, nil
}

// ---------------- Hilfsfunktionen ----------------

func (c *compiler) fail(loc ast.Loc, format string, args ...any) {
	panic(&Error{File: loc.File, Line: loc.Line, Msg: fmt.Sprintf(format, args...)})
}

func (c *compiler) fileIdx(f string) int32 {
	if i, ok := c.files[f]; ok {
		return i
	}
	i := int32(len(c.chunk.Files))
	c.chunk.Files = append(c.chunk.Files, f)
	c.files[f] = i
	return i
}

func (c *compiler) name(s string) int32 {
	if i, ok := c.names[s]; ok {
		return i
	}
	i := int32(len(c.chunk.Names))
	c.chunk.Names = append(c.chunk.Names, s)
	c.names[s] = i
	return i
}

func (c *compiler) constant(v value.Value) int32 {
	c.chunk.Consts = append(c.chunk.Consts, v)
	return int32(len(c.chunk.Consts) - 1)
}

func (c *compiler) emit(loc ast.Loc, op bytecode.Op, operands ...int32) int {
	in := bytecode.Instr{Op: op}
	if len(operands) > 0 {
		in.A = operands[0]
	}
	if len(operands) > 1 {
		in.B = operands[1]
	}
	if len(operands) > 2 {
		in.C = operands[2]
	}
	c.code = append(c.code, in)
	c.pos = append(c.pos, bytecode.Pos{File: c.fileIdx(loc.File), Line: int32(loc.Line)})
	return len(c.code) - 1
}

func (c *compiler) emitConst(loc ast.Loc, v value.Value) {
	c.emit(loc, bytecode.OpConst, c.constant(v))
}

func (c *compiler) here() int { return len(c.code) }

// patch lässt den Sprung bei idx auf die aktuelle Position zeigen.
func (c *compiler) patch(idx int) { c.code[idx].A = int32(c.here()) }

func (c *compiler) patchAll(list []int) {
	for _, idx := range list {
		c.patch(idx)
	}
}

// ---------------- Ausdrücke ----------------

var binOps = map[token.Type]bytecode.Op{
	token.PLUS:  bytecode.OpAdd,
	token.MINUS: bytecode.OpSub,
	token.MUL:   bytecode.OpMul,
	token.DIV:   bytecode.OpDiv,
	token.AMP:   bytecode.OpConcat,
	token.EQ:    bytecode.OpEq,
	token.NEQ:   bytecode.OpNeq,
	token.LT:    bytecode.OpLt,
	token.GT:    bytecode.OpGt,
	token.LE:    bytecode.OpLe,
	token.GE:    bytecode.OpGe,
}

// fold rechnet reine Zahlenausdrücke (+ - *, unäres Minus) zur Compile-Zeit aus.
func fold(e ast.Expr) (float64, bool) {
	switch n := e.(type) {
	case *ast.NumberNode:
		return n.Value, true
	case *ast.UnaryOpNode:
		if n.Op == token.MINUS {
			if v, ok := fold(n.Right); ok {
				return -v, true
			}
		}
	case *ast.BinOpNode:
		switch n.Op {
		case token.PLUS, token.MINUS, token.MUL:
			l, ok1 := fold(n.Left)
			r, ok2 := fold(n.Right)
			if ok1 && ok2 {
				return value.CalculateBinaryOp(n.Op, value.NumVal(l), value.NumVal(r)).Num, true
			}
		}
	}
	return 0, false
}

func (c *compiler) expr(e ast.Expr) {
	loc := e.Pos()

	if v, ok := fold(e); ok {
		c.emitConst(loc, value.NumVal(v))
		return
	}

	switch n := e.(type) {
	case *ast.StringNode:
		c.emitConst(loc, value.StrVal(n.Value))

	case *ast.BoolNode:
		if n.Value {
			c.emit(loc, bytecode.OpTrue)
		} else {
			c.emit(loc, bytecode.OpFalse)
		}

	case *ast.NullNode:
		c.emit(loc, bytecode.OpNull)

	case *ast.VarNode:
		// "obj.feld": Aufteilung am ERSTEN Punkt, wie im Interpreter
		if i := strings.IndexByte(n.Name, '.'); i >= 0 {
			c.emit(loc, bytecode.OpLoadField, c.name(n.Name[:i]), c.name(n.Name[i+1:]))
		} else {
			c.emit(loc, bytecode.OpLoad, c.name(n.Name))
		}

	case *ast.UnaryOpNode:
		c.expr(n.Right)
		switch n.Op {
		case token.NOT:
			c.emit(loc, bytecode.OpNot)
		case token.MINUS:
			c.emit(loc, bytecode.OpNeg)
		default:
			c.fail(loc, "unbekannter unärer Operator %v", n.Op)
		}

	case *ast.BinOpNode:
		c.binary(loc, n)

	case *ast.CallExprNode:
		c.call(loc, n.Name, n.Args)

	case *ast.ArrayLiteralNode:
		var jumps []int
		for i, el := range n.Elements {
			c.expr(el)
			jumps = append(jumps, c.emit(loc, bytecode.OpJumpIfError, -1, int32(i)))
		}
		c.emit(loc, bytecode.OpMakeArray, int32(len(n.Elements)))
		c.patchAll(jumps)

	case *ast.MapIndexNode:
		c.expr(n.Base)
		j1 := c.emit(loc, bytecode.OpJumpIfError, -1, 0)
		j2 := c.emit(loc, bytecode.OpMapBase, -1)
		c.expr(n.Key)
		j3 := c.emit(loc, bytecode.OpJumpIfError, -1, 1)
		c.emit(loc, bytecode.OpMapIndex)
		c.patchAll([]int{j1, j2, j3})

	case *ast.RangeNode, *ast.IsNode:
		c.fail(loc, "Range/Is gehören zu Select Case (Phase 2)")

	default:
		c.fail(loc, "unbekannter Ausdruckstyp %T", e)
	}
}

func (c *compiler) binary(loc ast.Loc, n *ast.BinOpNode) {
	switch n.Op {
	case token.AND, token.OR:
		isOr := int32(0)
		if n.Op == token.OR {
			isOr = 1
		}
		c.expr(n.Left)
		jErr := c.emit(loc, bytecode.OpJumpIfError, -1, 0)
		jShort := c.emit(loc, bytecode.OpShortCircuit, -1, isOr)
		c.expr(n.Right)
		c.emit(loc, bytecode.OpTruthy)
		c.patchAll([]int{jErr, jShort})
		return
	}

	op, ok := binOps[n.Op]
	if !ok {
		c.fail(loc, "unbekannter Operator %v", n.Op)
	}
	c.expr(n.Left)
	jErr := c.emit(loc, bytecode.OpJumpIfError, -1, 0)
	c.expr(n.Right)
	c.emit(loc, op)
	c.patch(jErr)
}

// call übersetzt Funktionsaufruf ODER Array-Zugriff (arr(i)) - der Parser
// kann das nicht unterscheiden, die VM entscheidet zur Laufzeit.
func (c *compiler) call(loc ast.Loc, name string, args []ast.Expr) {
	// Sonderfälle wie in evalFunctionCall: nur das ERSTE Argument wird
	// ausgewertet, ein Fehlerwert darin ist genau der Prüfgegenstand.
	if strings.EqualFold(name, "IsError") {
		if len(args) < 1 {
			c.emit(loc, bytecode.OpFalse)
			return
		}
		c.expr(args[0])
		c.emit(loc, bytecode.OpIsError)
		return
	}
	if strings.EqualFold(name, "ErrorText") {
		if len(args) < 1 {
			c.emitConst(loc, value.StrVal(""))
			return
		}
		c.expr(args[0])
		c.emit(loc, bytecode.OpErrorText)
		return
	}

	var jumps []int
	for i, a := range args {
		c.expr(a)
		// Fehler im i-ten Argument: Aufruf entfällt, die i davor liegenden
		// Argumente werden verworfen, der Fehlerwert ist das Ergebnis.
		jumps = append(jumps, c.emit(loc, bytecode.OpJumpIfError, -1, int32(i)))
	}
	c.emit(loc, bytecode.OpCall, c.name(name), int32(len(args)))
	c.patchAll(jumps)
}

// ---------------- Anweisungen ----------------

func (c *compiler) stmts(list []ast.Stmt) {
	for _, s := range list {
		c.stmt(s)
	}
}

// block übersetzt den Körper eines Konstrukts (If, Schleife, Select, ...).
func (c *compiler) block(list []ast.Stmt) {
	c.nesting++
	c.stmts(list)
	c.nesting--
}

func assignOp(loc ast.Loc, c *compiler, op token.Type) token.Type {
	switch op {
	case token.PLUS_ASSIGN:
		return token.PLUS
	case token.MINUS_ASSIGN:
		return token.MINUS
	case token.MUL_ASSIGN:
		return token.MUL
	case token.DIV_ASSIGN:
		return token.DIV
	}
	c.fail(loc, "ungültiger Zuweisungsoperator %v", op)
	return op
}

func (c *compiler) currentLoop(loc ast.Loc, what string) *loop {
	if len(c.loops) == 0 {
		c.fail(loc, "'%s' außerhalb einer Schleife", what)
	}
	return c.loops[len(c.loops)-1]
}

// popTo räumt Stack-Temporaries ab, bevor Exit/Continue aus einem
// verschachtelten Konstrukt herausspringt.
func (c *compiler) popTo(loc ast.Loc, depth int) {
	if c.depth > depth {
		c.emit(loc, bytecode.OpPopN, int32(c.depth-depth))
	}
}

func (c *compiler) pushLoop() *loop {
	l := &loop{continueTo: -1, bodyDepth: c.depth}
	c.loops = append(c.loops, l)
	return l
}

func (c *compiler) popLoop() { c.loops = c.loops[:len(c.loops)-1] }

func (c *compiler) stmt(s ast.Stmt) {
	loc := s.Pos()

	switch n := s.(type) {
	case *ast.MultiStmtNode:
		c.stmts(n.Stmts) // zählt nicht als Verschachtelung (wie registerFuncsAndSubs)

	case *ast.AssignNode:
		c.expr(n.Value)
		c.emit(loc, bytecode.OpCheck)
		if c.funcName != "" && n.Name == c.funcName {
			// Im Interpreter greift das VOR der Deklarations-Prüfung (auch "Dim Name = x").
			c.emit(loc, bytecode.OpSetReturn)
		} else if n.IsDeclaration {
			var flags int32
			if n.IsConst {
				flags |= bytecode.FlagConst
			}
			if n.InLoop {
				flags |= bytecode.FlagInLoop
			}
			c.emit(loc, bytecode.OpDeclare, c.name(n.Name), flags)
		} else {
			c.emit(loc, bytecode.OpStore, c.name(n.Name))
		}

	case *ast.CompoundAssignNode:
		v, ok := n.Left.(*ast.VarNode)
		if !ok {
			c.fail(loc, "nur Variablen werden bei += / -= / *= / /= unterstützt")
		}
		nm := c.name(v.Name)
		c.emit(loc, bytecode.OpLoadForUpdate, nm)
		c.expr(n.Right)
		c.emit(loc, bytecode.OpCheck)
		c.emit(loc, bytecode.OpCalc, int32(assignOp(loc, c, n.Op)))
		c.emit(loc, bytecode.OpCheck)
		c.emit(loc, bytecode.OpStoreRef, nm)

	case *ast.PublicNode:
		c.expr(n.Value)
		c.emit(loc, bytecode.OpCheck)
		c.emit(loc, bytecode.OpSetGlobal, c.name(n.Name))

	case *ast.PrintNode:
		c.expr(n.Value)
		c.emit(loc, bytecode.OpCheck)
		hasColor := int32(0)
		if n.Color != nil {
			c.expr(n.Color)
			hasColor = 1
		}
		c.emit(loc, bytecode.OpPrint, hasColor)

	case *ast.CallNode:
		if strings.EqualFold(n.Name, "Cls") {
			c.emit(loc, bytecode.OpCls)
			return
		}
		c.call(loc, n.Name, n.Args)
		c.emit(loc, bytecode.OpCheck)
		c.emit(loc, bytecode.OpPop)

	case *ast.IfNode:
		var endJumps []int
		for _, br := range n.Branches {
			c.expr(br.Cond)
			c.emit(loc, bytecode.OpCheck)
			skip := c.emit(loc, bytecode.OpJumpIfFalse, -1)
			c.block(br.Body)
			endJumps = append(endJumps, c.emit(loc, bytecode.OpJump, -1))
			c.patch(skip)
		}
		c.block(n.Else)
		c.patchAll(endJumps)

	case *ast.WhileNode:
		head := c.here()
		c.expr(n.Condition)
		c.emit(loc, bytecode.OpCheck)
		exit := c.emit(loc, bytecode.OpJumpIfFalse, -1)
		l := c.pushLoop()
		l.continueTo = head
		c.block(n.Body)
		c.emit(loc, bytecode.OpJump, int32(head))
		c.patch(exit)
		c.patchAll(l.breakJumps)
		c.popLoop()

	case *ast.DoLoopNode:
		head := c.here()
		var exitJumps []int
		cond := func() {
			c.expr(n.Condition)
			c.emit(loc, bytecode.OpCheck)
			op := bytecode.OpJumpIfFalse // While: raus, wenn unwahr
			if n.IsUntil {
				op = bytecode.OpJumpIfTrue // Until: raus, wenn wahr
			}
			exitJumps = append(exitJumps, c.emit(loc, op, -1))
		}
		if !n.CheckAtEnd && n.Condition != nil {
			cond()
		}
		l := c.pushLoop()
		c.block(n.Body)
		c.patchAll(l.continueJumps) // Continue springt zur Fuß-Bedingung
		if n.CheckAtEnd && n.Condition != nil {
			cond()
		}
		c.emit(loc, bytecode.OpJump, int32(head))
		c.patchAll(exitJumps)
		c.patchAll(l.breakJumps)
		c.popLoop()

	case *ast.ForNode:
		c.expr(n.Start)
		c.emit(loc, bytecode.OpCheck)
		c.expr(n.End)
		c.emit(loc, bytecode.OpCheck)
		nm := c.name(n.VarName)
		step := c.constant(value.NumVal(n.Step))
		c.emit(loc, bytecode.OpForInit, nm, step)
		c.depth += 2 // [aktuell, ende] liegen während der Schleife auf dem Stack

		head := c.here()
		dir := int32(0)
		if n.Step > 0 {
			dir = 1
		}
		test := c.emit(loc, bytecode.OpForTest, -1, nm, dir)

		l := c.pushLoop()
		c.block(n.Body)
		c.patchAll(l.continueJumps)
		c.emit(loc, bytecode.OpForStep, step)
		c.emit(loc, bytecode.OpJump, int32(head))

		c.patch(test) // normales Ende und "Exit For" landen beide hier
		c.patchAll(l.breakJumps)
		c.popLoop()
		c.emit(loc, bytecode.OpPopN, 2)
		c.depth -= 2

	case *ast.ExitNode:
		if n.ExitType == "Sub" || n.ExitType == "Function" {
			c.emit(loc, bytecode.OpReturn, 0)
			return
		}
		if len(c.loops) == 0 {
			// Wie im Interpreter: ein "Exit" ohne Schleife beendet den umgebenden Körper.
			if n.ExitType == "" {
				c.emit(loc, bytecode.OpReturn, 0)
				return
			}
			c.fail(loc, "'Exit %s' außerhalb einer Schleife", n.ExitType)
		}
		l := c.currentLoop(loc, "Exit")
		c.popTo(loc, l.bodyDepth)
		l.breakJumps = append(l.breakJumps, c.emit(loc, bytecode.OpJump, -1))

	case *ast.ContinueNode:
		l := c.currentLoop(loc, "Continue")
		c.popTo(loc, l.bodyDepth)
		if l.continueTo >= 0 {
			c.emit(loc, bytecode.OpJump, int32(l.continueTo))
		} else {
			l.continueJumps = append(l.continueJumps, c.emit(loc, bytecode.OpJump, -1))
		}

	case *ast.DimArrayNode:
		dims := c.arraySizes(loc, n.Size1, n.Size2)
		inLoop := int32(0)
		if n.InLoop {
			inLoop = 1
		}
		c.emit(loc, bytecode.OpDimArray, c.name(n.Name), dims, inLoop)

	case *ast.PublicArrayNode:
		if n.Size1 == nil {
			// Im Interpreter wertet "Public a()" ein nil-Ausdruck aus und
			// endet mit genau dieser Meldung.
			c.emitConst(loc, value.ErrorVal("Unbekannter Ausdruckstyp: <nil>"))
			c.emit(loc, bytecode.OpCheck)
			return
		}
		dims := c.arraySizes(loc, n.Size1, n.Size2)
		c.emit(loc, bytecode.OpPublicArray, c.name(n.Name), dims)

	case *ast.ArrayAssignNode:
		// Reihenfolge wie im Interpreter: erst Variable prüfen, dann Wert,
		// dann Index, und der zweite Index nur bei einem 2D-Array.
		nm := c.name(n.Name)
		c.emit(loc, bytecode.OpArrCheck, nm)
		c.expr(n.Value)
		c.emit(loc, bytecode.OpCheck)
		c.expr(n.Index)
		c.emit(loc, bytecode.OpToIndex)
		hasIdx2 := int32(0)
		if n.Index2 != nil {
			guard := c.emit(loc, bytecode.OpArrGuard2D, nm, -1)
			c.expr(n.Index2)
			c.emit(loc, bytecode.OpToIndex)
			c.code[guard].B = int32(c.here())
			hasIdx2 = 1
		}
		c.emit(loc, bytecode.OpArrStore, nm, hasIdx2)

	case *ast.ForEachNode:
		c.expr(n.Iterable)
		c.emit(loc, bytecode.OpCheck)
		key := c.name(n.KeyVar)
		val := int32(0)
		if n.ValVar != "" {
			val = c.name(n.ValVar) + 1
		}
		c.emit(loc, bytecode.OpEachInit, key, val)
		c.depth += 3 // [iterierbar, index, schlüssel-liste]

		head := c.here()
		next := c.emit(loc, bytecode.OpEachNext, -1, key, val)
		l := c.pushLoop()
		l.continueTo = head
		c.block(n.Body)
		c.emit(loc, bytecode.OpJump, int32(head))

		c.patch(next) // normales Ende und "Exit For" landen beide hier
		c.patchAll(l.breakJumps)
		c.popLoop()
		c.emit(loc, bytecode.OpPopN, 3)
		c.depth -= 3

	case *ast.SelectNode:
		c.selectStmt(loc, n)
	case *ast.ReturnNode:
		c.expr(n.Value)
		c.emit(loc, bytecode.OpCheck)
		c.emit(loc, bytecode.OpReturn, 1)

	case *ast.SubNode:
		c.definition(loc, n.Name, n.Params, n.Body, false)

	case *ast.FuncNode:
		c.definition(loc, n.Name, n.Params, n.Body, true)
	case *ast.TryNode:
		c.fail(loc, "Try/Catch/Finally ist noch nicht implementiert (Phase 4)")

	default:
		c.fail(loc, "unbekannter Anweisungstyp %T", s)
	}
}

// arraySizes wertet die Größenausdrücke aus (je mit Fehlerprüfung) und liefert
// die Anzahl der Dimensionen. Ohne Größe ("Dim a()") sind es 0.
func (c *compiler) arraySizes(loc ast.Loc, size1, size2 ast.Expr) int32 {
	if size1 == nil {
		return 0
	}
	c.expr(size1)
	c.emit(loc, bytecode.OpCheck)
	if size2 == nil {
		return 1
	}
	c.expr(size2)
	c.emit(loc, bytecode.OpCheck)
	return 2
}

// selectStmt übersetzt Select Case. Wie im Interpreter gibt es zwei Wege:
//
//   - Sind ALLE Bedingungen reine Zahl-/String-Literale, schlägt OpSelectDispatch
//     den Zweig in einer Tabelle nach (Fast-Path). Bool-Werte nimmt er davon aus.
//   - Sonst (und für Bool) läuft der lineare Scan: jede Bedingung der Reihe
//     nach, Range/Is/Ausdrücke werden erst beim Prüfen ausgewertet.
//
// Während des Scans liegt der Prüfwert als Temporary auf dem Stack und wird
// vor dem Sprung in einen Körper wieder entfernt, sodass Körper ohne
// zusätzliche Stack-Tiefe laufen (wichtig für Exit/Continue darin).
func (c *compiler) selectStmt(loc ast.Loc, n *ast.SelectNode) {
	c.expr(n.Expression)
	c.emit(loc, bytecode.OpCheck)

	tbl := bytecode.SelectTable{
		Num:     make(map[float64]int),
		Str:     make(map[string]int),
		Targets: make([]int32, len(n.Cases)),
	}
	dispatchOK := true
	for i, br := range n.Cases {
		for _, cond := range br.Conditions {
			switch lit := cond.(type) {
			case *ast.NumberNode:
				if _, dup := tbl.Num[lit.Value]; !dup {
					tbl.Num[lit.Value] = i
				}
			case *ast.StringNode:
				key := strings.ToLower(lit.Value)
				if _, dup := tbl.Str[key]; !dup {
					tbl.Str[key] = i
				}
			default:
				dispatchOK = false // Range, Is, Bool/Null, Variable, Ausdruck, ...
			}
		}
	}

	tIdx := -1
	if dispatchOK {
		tIdx = len(c.chunk.Selects)
		c.chunk.Selects = append(c.chunk.Selects, tbl)
		c.emit(loc, bytecode.OpSelectDispatch, int32(tIdx))
	}

	// ---- linearer Scan ----
	c.depth++
	bodyJumps := make([][]int, len(n.Cases))
	for i, br := range n.Cases {
		for _, cond := range br.Conditions {
			c.emit(loc, bytecode.OpDup)
			switch cc := cond.(type) {
			case *ast.RangeNode:
				c.expr(cc.Low)
				c.expr(cc.High)
				c.emit(loc, bytecode.OpCaseRange)
			case *ast.IsNode:
				c.expr(cc.Value)
				c.emit(loc, bytecode.OpCaseIs, int32(cc.Operator))
			default:
				c.expr(cond)
				c.emit(loc, bytecode.OpCaseEq)
			}
			next := c.emit(loc, bytecode.OpJumpIfFalse, -1)
			c.emit(loc, bytecode.OpPop) // Prüfwert weg, dann in den Körper
			bodyJumps[i] = append(bodyJumps[i], c.emit(loc, bytecode.OpJump, -1))
			c.patch(next)
		}
	}
	c.emit(loc, bytecode.OpPop) // kein Treffer
	noMatch := c.emit(loc, bytecode.OpJump, -1)
	c.depth--

	// ---- Körper ----
	var endJumps []int
	for i, br := range n.Cases {
		addr := int32(c.here())
		for _, j := range bodyJumps[i] {
			c.code[j].A = addr
		}
		if tIdx >= 0 {
			c.chunk.Selects[tIdx].Targets[i] = addr
		}
		c.block(br.Body)
		endJumps = append(endJumps, c.emit(loc, bytecode.OpJump, -1))
	}

	defaultAddr := int32(c.here())
	c.code[noMatch].A = defaultAddr
	if tIdx >= 0 {
		c.chunk.Selects[tIdx].Default = defaultAddr
	}
	c.block(n.Default)
	c.patchAll(endJumps)
}

// definition übersetzt eine Sub/Function in eine eigene Einheit.
//
// Wie im Interpreter wird die Definition an ihrer Stelle im Programm
// (OpDefine) UND - wenn sie auf oberster Ebene steht - schon vor dem Start
// registriert (registerFuncsAndSubs). Dadurch darf man Funktionen aufrufen, die
// weiter unten stehen.
func (c *compiler) definition(loc ast.Loc, name string, params []ast.ParamDef, body []ast.Stmt, isFunc bool) {
	idx := int32(len(c.chunk.Funcs))
	fn := bytecode.Func{Name: name, IsFunc: isFunc, Required: int32(ast.RequiredParams(params))}
	for _, p := range params {
		fn.Params = append(fn.Params, p.Name)
	}
	c.chunk.Funcs = append(c.chunk.Funcs, fn)
	topLevel := c.nesting == 0

	// Einheit wechseln
	sCode, sPos, sLoops, sDepth, sNest, sFunc := c.code, c.pos, c.loops, c.depth, c.nesting, c.funcName
	c.code, c.pos, c.loops, c.depth, c.nesting = nil, nil, nil, 0, 1
	c.funcName = ""
	if isFunc {
		c.funcName = name
	}

	// Prolog: fehlende optionale Parameter bekommen ihren Standardwert (in der
	// Umgebung der Funktion ausgewertet; ein Fehlerwert beendet den Aufruf).
	for i, p := range params {
		if p.IsOptional {
			given := c.emit(loc, bytecode.OpJumpIfArg, -1, int32(i))
			c.expr(p.Default)
			c.emit(loc, bytecode.OpParamDefault, c.name(p.Name))
			c.patch(given)
		}
	}
	c.stmts(body)
	c.emit(loc, bytecode.OpReturn, 0)

	c.chunk.Funcs[idx].Code, c.chunk.Funcs[idx].Pos = c.code, c.pos
	c.code, c.pos, c.loops, c.depth, c.nesting, c.funcName = sCode, sPos, sLoops, sDepth, sNest, sFunc

	if topLevel {
		c.chunk.Preregister = append(c.chunk.Preregister, idx)
	}
	c.emit(loc, bytecode.OpDefine, idx)
}
