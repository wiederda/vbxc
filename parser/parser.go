// Package parser erzeugt aus Tokens den AST.
// Gegenüber dem Interpreter: keine Abhängigkeit zu Environment/Value,
// Positionen in jedem Knoten, Include-Zustand pro Lauf statt global.
package parser

import (
	"fmt"
	"strconv"
	"strings"

	"vbxc/ast"
	"vbxc/lexer"
	. "vbxc/token" // Token-Konstanten bewusst ohne Präfix
)

// Options steuert die Teile, die der Parser nicht selbst kennt.
type Options struct {
	File    string // Name der Hauptdatei (nur für Positionen/Fehler)
	BaseDir string // Basis für relative Includes ("" = Arbeitsverzeichnis)
	// StrictIncludes lässt fehlende Include-Dateien als Parse-Fehler
	// durchschlagen. Für Build-/Prüfpfade aktivieren; der Interpreter kann
	// Includes weiterhin optional behandeln.
	StrictIncludes bool
	// IncludePaths ergänzt die Suche nach Include-Dateien. Wenn nil oder leer,
	// verwendet der Parser die Pfade aus VBX_INCLUDE_PATH.
	IncludePaths []string

	// ExtractUse entfernt "#use"-Zeilen aus dem Quelltext und liefert die
	// Modulnamen. nil = Zeilen unverändert lassen.
	ExtractUse func(lines []string) (cleaned []string, modules []string)

	// OnModules wird mit den gefundenen Modulen aufgerufen (Hauptdatei und
	// Includes). nil = ignorieren.
	OnModules func(modules []string)
}

type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return "Parse error: " + e.Message
}

// ParseSource lext und parst einen kompletten Quelltext.
func ParseSource(src string, opts Options) (stmts []ast.Stmt, err error) {
	src = strings.TrimPrefix(src, "\uFEFF")
	if opts.ExtractUse != nil {
		lines, modules := opts.ExtractUse(strings.Split(src, "\n"))
		if len(modules) > 0 && opts.OnModules != nil {
			opts.OnModules(modules)
		}
		src = strings.Join(lines, "\n")
	}

	p := &Parser{
		tokens:  lexer.Tokenize(src),
		file:    opts.File,
		baseDir: opts.BaseDir,
		opts:    opts,
		inc:     newIncludeState(),
	}

	defer func() {
		if r := recover(); r != nil {
			if msg, ok := r.(string); ok {
				stmts = nil
				err = &ParseError{Message: msg}
				return
			}
			panic(r)
		}
	}()

	return p.parse(), nil
}

// ---------------- Parser ----------------

type Parser struct {
	tokens    []Token
	pos       int
	Errors    []string
	loopDepth int
	loopStack []Type
	procStack []Type
	file      string // Datei, die gerade geparst wird
	baseDir   string // Verzeichnis dieser Datei
	opts      Options
	inc       *includeState
}

func (p *Parser) at(line int) ast.Loc {
	return ast.Loc{File: p.file, Line: line}
}

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) next() Token {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *Parser) peekNext() Token {
	if p.pos+1 >= len(p.tokens) {
		return Token{Type: EOF}
	}
	return p.tokens[p.pos+1]
}

func (p *Parser) skipStuff() {
	for p.peek().Type == NEWLINE || p.peek().Type == COMMENT {
		p.next()
	}
}

// currentLine ist die Zeile des Tokens an der aktuellen Position.
func (p *Parser) currentLine() int {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos].Line
	}
	if n := len(p.tokens); n > 0 {
		return p.tokens[n-1].Line
	}
	return 1
}

func (p *Parser) error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Meldungen vom Lexer tragen die Zeile schon ("Zeile 4: ...")
	if !strings.HasPrefix(msg, "Zeile ") {
		msg = fmt.Sprintf("Zeile %d: %s", p.currentLine(), msg)
	}

	p.Errors = append(p.Errors, msg)
	panic(msg)
}

func (p *Parser) expect(expected Type) Token {
	tok := p.next()
	if tok.Type != expected {
		p.error("Syntaxfehler: Erwartet wurde das Symbol '%v', aber '%v' gefunden.", expected, tok.Type)
	}
	return tok
}

func (p *Parser) expectIdentifier() string {
	tok := p.next()
	if tok.Type != IDENT {
		p.error("Syntaxfehler: Hier wird ein Name (Identifier) erwartet, aber '%v' gefunden.", tok.Type)
	}
	return tok.Value
}

func (p *Parser) match(expected Type) bool {
	if p.peek().Type == expected {
		p.next()
		return true
	}
	return false
}

func (p *Parser) expectEnd(expected Type) {
	p.skipStuff()

	if p.peek().Type != END {
		p.error("Struktur-Fehler: Der '%v'-Block wurde nicht geschlossen. Erwartet: 'End %v', gefunden: '%s'",
			expected, expected, p.peek().Value)
		return
	}
	p.next()
	p.skipStuff()

	if p.peek().Type != expected {
		p.error("Struktur-Fehler: Du versuchst mit 'End %s' zu schließen, aber der '%v'-Block ist noch offen.",
			p.peek().Value, expected)
		return
	}
	p.next()
}

func (p *Parser) parse() []ast.Stmt {
	var stmts []ast.Stmt
	for p.peek().Type != EOF {
		if p.peek().Type == NEWLINE || p.peek().Type == COMMENT {
			p.pos++
			continue
		}

		// ERROR-Token vom Lexer abfangen
		if p.peek().Type == ERROR {
			p.error("%s", p.peek().Value)
			p.pos++
			continue
		}

		s := p.parseStmt()
		if s != nil {
			stmts = append(stmts, s)
		}
	}
	return stmts
}

func (p *Parser) parseBlock(context string, stopTokens ...Type) []ast.Stmt {
	var stmts []ast.Stmt
	for p.peek().Type != EOF {
		p.skipStuff()
		t := p.peek().Type

		if t == END {
			return stmts
		}
		for _, stop := range stopTokens {
			if t == stop {
				return stmts
			}
		}

		if p.peek().Type == EOF {
			break
		}

		s := p.parseStmt()
		if s != nil {
			stmts = append(stmts, s)
		}
	}

	if p.peek().Type == EOF {
		p.error("Struktur-Fehler: Datei-Ende erreicht, aber der '%s'-Block ist noch offen.", context)
	}

	return stmts
}

// ---------------- Ausdrücke ----------------

func (p *Parser) parseExpr() ast.Expr {
	return p.parseOr()
}

func (p *Parser) parseOr() ast.Expr {
	left := p.parseAnd()
	for p.peek().Type == OR {
		op := p.next()
		right := p.parseAnd()
		left = &ast.BinOpNode{Loc: p.at(op.Line), Left: left, Op: OR, Right: right}
	}
	return left
}

func (p *Parser) parseAnd() ast.Expr {
	left := p.parseNot()
	for p.peek().Type == AND {
		op := p.next()
		right := p.parseNot()
		left = &ast.BinOpNode{Loc: p.at(op.Line), Left: left, Op: AND, Right: right}
	}
	return left
}

func (p *Parser) parseNot() ast.Expr {
	if p.peek().Type == NOT {
		op := p.next()
		return &ast.UnaryOpNode{Loc: p.at(op.Line), Op: NOT, Right: p.parseNot()}
	}
	return p.parseCompare()
}

func (p *Parser) parseCompare() ast.Expr {
	left := p.parseConcat()
	switch p.peek().Type {
	case LT, GT, LE, GE, EQ, NEQ:
		op := p.next()
		right := p.parseConcat()
		return &ast.BinOpNode{Loc: p.at(op.Line), Left: left, Op: op.Type, Right: right}
	}
	return left
}

func (p *Parser) parseConcat() ast.Expr {
	left := p.parseAddSub()
	for p.peek().Type == AMP {
		op := p.next()
		right := p.parseAddSub()
		left = &ast.BinOpNode{Loc: p.at(op.Line), Left: left, Op: op.Type, Right: right}
	}
	return left
}

func (p *Parser) parseAddSub() ast.Expr {
	left := p.parseTerm()
	for {
		switch p.peek().Type {
		case PLUS, MINUS:
			op := p.next()
			right := p.parseTerm()
			left = &ast.BinOpNode{Loc: p.at(op.Line), Left: left, Op: op.Type, Right: right}
		default:
			return left
		}
	}
}

func (p *Parser) parseTerm() ast.Expr {
	left := p.parseFactor()
	for {
		switch p.peek().Type {
		case MUL, DIV:
			op := p.next()
			right := p.parseFactor()
			left = &ast.BinOpNode{Loc: p.at(op.Line), Left: left, Op: op.Type, Right: right}
		default:
			return left
		}
	}
}

func (p *Parser) parseFactor() ast.Expr {
	tok := p.peek()

	if tok.Type == ERROR {
		p.error("%s", tok.Value)
		return nil
	}

	if tok.Type == NOT {
		p.next()
		right := p.parseFactor()
		return &ast.UnaryOpNode{Loc: p.at(tok.Line), Op: NOT, Right: right}
	}

	if tok.Type == MINUS {
		p.next()
		right := p.parseFactor()
		return &ast.UnaryOpNode{Loc: p.at(tok.Line), Op: MINUS, Right: right}
	}

	switch tok.Type {
	case NUMBER:
		p.next()
		v, _ := strconv.ParseFloat(tok.Value, 64)
		return &ast.NumberNode{Loc: p.at(tok.Line), Value: v}

	case LBRACE:
		return p.parseArrayLiteral()

	case BOOL:
		p.next()
		return &ast.BoolNode{Loc: p.at(tok.Line), Value: strings.ToLower(tok.Value) == "true"}

	case NOTHING:
		p.next()
		return &ast.NullNode{Loc: p.at(tok.Line)}

	case STRING:
		p.next()
		return &ast.StringNode{Loc: p.at(tok.Line), Value: tok.Value}

	case NEWLINE:
		p.error("Der Ausdruck ist unvollständig - am Zeilenende fehlt ein Wert.")
		return nil

	case IDENT:
		p.next()
		fullName := tok.Value
		for p.peek().Type == DOT {
			p.next()
			nextTok := p.next()
			if nextTok.Type == EOF {
				p.error("Syntaxfehler: Nach dem Punkt (.) wurde ein Name erwartet, aber das Dateiende erreicht.")
			}
			fullName += "." + nextTok.Value
		}

		var expr ast.Expr

		// Funktionsaufruf / Array-Zugriff via ()
		if p.peek().Type == LPAREN {
			p.next() // '(' konsumieren

			args := p.parseCallArguments(fullName)

			expr = &ast.CallExprNode{Loc: p.at(tok.Line), Name: fullName, Args: args}
		} else {
			expr = &ast.VarNode{Loc: p.at(tok.Line), Name: fullName}
		}

		// Map-Zugriff via [] - rein lesend, verkettbar
		for p.peek().Type == LBRACKET {
			br := p.next() // '['
			key := p.parseExpr()
			if p.next().Type != RBRACKET {
				p.error("Erwartet ']' nach Map-Zugriff")
			}
			expr = &ast.MapIndexNode{Loc: p.at(br.Line), Base: expr, Key: key}
		}

		return expr

	case LPAREN:
		p.next()
		expr := p.parseExpr()
		if p.next().Type != RPAREN {
			p.error("Erwartet ')'")
		}
		return expr

	default:
		if tok.Type == EOF {
			p.error("Der Ausdruck ist unvollständig - das Dateiende wurde erreicht, bevor er abgeschlossen war.")
			return nil
		}
		p.error("Unerwartetes Zeichen im Ausdruck: '%s' (Typ: %v).", tok.Value, tok.Type)
		return nil
	}
}

func (p *Parser) parseArrayLiteral() ast.Expr {
	open := p.next() // { konsumieren
	var elements []ast.Expr

	p.skipStuff()

	for p.peek().Type != RBRACE && p.peek().Type != EOF {
		elements = append(elements, p.parseExpr())

		p.skipStuff()
		if p.peek().Type == COMMA {
			p.next()
			p.skipStuff()
		} else {
			break
		}
	}

	if p.peek().Type != RBRACE {
		p.error("Erwartet '}' am Ende des Array-Literals")
	}
	p.next() // } konsumieren

	return &ast.ArrayLiteralNode{Loc: p.at(open.Line), Elements: elements}
}

func (p *Parser) parseCallArguments(functionName string) []ast.Expr {
	var args []ast.Expr

	// Die öffnende Klammer '(' wurde bereits konsumiert.

	// foo(
	if p.peek().Type == EOF {
		p.error("Syntaxfehler: Funktionsaufruf '%s' wurde nicht abgeschlossen. Erwartet wird ')'.", functionName)
	}

	// foo()
	if p.peek().Type == RPAREN {
		p.next()
		return args
	}

	for {
		args = append(args, p.parseExpr())

		switch p.peek().Type {

		case COMMA:
			p.next()

			// foo(a,
			if p.peek().Type == EOF {
				p.error("Syntaxfehler: Nach ',' im Funktionsaufruf '%s' wird ein Argument erwartet.", functionName)
			}

			// foo(a,)
			if p.peek().Type == RPAREN {
				p.error("Syntaxfehler: Nach ',' im Funktionsaufruf '%s' wird ein Argument erwartet.", functionName)
			}

			continue

		case RPAREN:
			p.next()
			return args

		case EOF:
			p.error("Syntaxfehler: Funktionsaufruf '%s' wurde nicht abgeschlossen. Erwartet wird ')'.", functionName)

		default:
			p.error("Syntaxfehler im Funktionsaufruf '%s': Erwartet wird ',' oder ')', gefunden wurde '%s'.",
				functionName, p.peek().Value)
		}
	}
}

// Konsumiert eine Kette von [Key][Key]... rein zur Fehlerprüfung.
func (p *Parser) rejectBracketAssignTarget() bool {
	found := false
	for p.peek().Type == LBRACKET {
		found = true
		p.next() // '['
		p.parseExpr()
		if p.next().Type != RBRACKET {
			p.error("Erwartet ']' nach Map-Zugriff")
		}
	}
	if found && p.peek().Type == EQ {
		p.error("Map-Zugriff mit '[...]' ist nur lesend erlaubt. Zuweisungen bitte über map.Set(...) vornehmen.")
	}
	return found
}

// Parameterliste
func (p *Parser) parseParams() []ast.ParamDef {
	var params []ast.ParamDef
	if p.peek().Type != LPAREN {
		return params
	}
	p.next() // '('

	seenOptional := false

	for p.peek().Type != RPAREN {
		isOptional := false

		// "Optional" ist kein eigenes Token, sondern ein IDENT
		if p.peek().Type == IDENT && strings.EqualFold(p.peek().Value, "Optional") {
			p.next()
			isOptional = true
		}

		t := p.next()
		if t.Type != IDENT {
			p.error("Erwartet Parametername")
		}
		name := t.Value

		var def ast.Expr
		if p.peek().Type == EQ {
			p.next() // '='
			def = p.parseExpr()
			isOptional = true // '= Wert' macht den Parameter implizit optional
		}

		if isOptional {
			seenOptional = true
			if def == nil {
				// "Optional x" ohne "= Wert" -> Fallback-Default
				def = &ast.NumberNode{Loc: p.at(t.Line), Value: 0}
			}
		} else if seenOptional {
			p.error("Pflichtparameter '%s' darf nicht nach einem optionalen Parameter stehen", name)
		}

		params = append(params, ast.ParamDef{Name: name, IsOptional: isOptional, Default: def})

		if p.peek().Type == COMMA {
			p.next()
		}
	}
	p.next() // ')'
	p.skipStuff()
	return params
}

func mapAssignOp(op Type) Type {
	switch op {
	case PLUS_ASSIGN:
		return PLUS
	case MINUS_ASSIGN:
		return MINUS
	case MUL_ASSIGN:
		return MUL
	case DIV_ASSIGN:
		return DIV
	}
	return op
}

// ---------------- Anweisungen ----------------

func (p *Parser) parseForEach() ast.Stmt {
	forTok := p.next() // FOR
	p.next()           // EACH

	keyVar := p.expectIdentifier()
	var valVar string

	if p.match(COMMA) {
		valVar = p.expectIdentifier()
	}

	p.expect(IN)
	iterable := p.parseExpr()

	p.skipStuff()

	// For Each ist für Continue/Exit vom Typ FOR.
	p.loopDepth++
	p.loopStack = append(p.loopStack, FOR)

	body := p.parseBlock("Next", NEXT)

	p.loopStack = p.loopStack[:len(p.loopStack)-1]
	p.loopDepth--

	if p.peek().Type != NEXT {
		p.error("Erwartet 'Next' am Ende von For Each, gefunden: '%s'", p.peek().Value)
	}
	p.next() // NEXT

	// Optional: Next k  oder  Next v
	if p.peek().Type == IDENT {
		name := p.peek().Value
		if strings.EqualFold(name, keyVar) || (valVar != "" && strings.EqualFold(name, valVar)) {
			p.next()
		}
	}

	return &ast.ForEachNode{
		Loc:      p.at(forTok.Line),
		KeyVar:   keyVar,
		ValVar:   valVar,
		Iterable: iterable,
		Body:     body,
	}
}

func (p *Parser) parseStmt() ast.Stmt {
	p.skipStuff()
	if p.peek().Type == EOF {
		return nil
	}

	if p.peek().Type == ERROR {
		p.error("%s", p.peek().Value)
		return nil
	}

	line := p.peek().Line

	switch p.peek().Type {

	case SELECT:
		p.next() // SELECT
		if p.peek().Type != CASE {
			p.error("Nach 'Select' muss 'Case' folgen (z.B. Select Case x)")
		}
		p.next() // CASE

		node := &ast.SelectNode{Loc: p.at(line), Expression: p.parseExpr()}
		p.skipStuff()

		for p.peek().Type == CASE {
			p.next() // CASE
			if p.peek().Type == ELSE {
				p.next()                                // ELSE
				node.Default = p.parseBlock("End", END) // bis zum END des Selects
			} else {
				branch := ast.CaseBranch{}
				for {
					var cond ast.Expr

					if p.peek().Type == IS {
						isTok := p.next() // 'Is'
						op := p.next()    // Operator (>, <, =, ...)
						val := p.parseExpr()
						cond = &ast.IsNode{Loc: p.at(isTok.Line), Operator: op.Type, Value: val}
					} else {
						left := p.parseExpr()

						if p.peek().Type == TO {
							toTok := p.next() // 'To'
							right := p.parseExpr()
							cond = &ast.RangeNode{Loc: p.at(toTok.Line), Low: left, High: right}
						} else {
							cond = left
						}
					}

					branch.Conditions = append(branch.Conditions, cond)

					if p.peek().Type != COMMA {
						break
					}
					p.next() // Komma
				}
				// Ein Case-Block endet beim nächsten CASE oder beim END
				branch.Body = p.parseBlock("Case", CASE, END)
				node.Cases = append(node.Cases, branch)
			}
			p.skipStuff()
		}
		p.expectEnd(SELECT)
		return node

	case DIM, PUBLIC, CONST:
		kind := p.peek().Type
		isPublic := kind == PUBLIC
		isConst := kind == CONST
		p.next()
		var stmts []ast.Stmt
		inLoopContext := p.loopDepth > 0

		for {
			nameTok := p.next()

			if nameTok.Type != IDENT {
				p.error("Erwartet Variablennamen nach DIM/PUBLIC/CONST, aber '%s' ist ein reserviertes Schlüsselwort und kann nicht als Name verwendet werden.", nameTok.Value)
			}

			name := nameTok.Value
			loc := p.at(nameTok.Line)

			if p.peek().Type == LPAREN {
				if isConst {
					p.error("'Const' unterstützt keine Arrays (Konstante '%s')", name)
				}
				p.next() // (
				var size1 ast.Expr
				var size2 ast.Expr

				if p.peek().Type != RPAREN {
					size1 = p.parseExpr()

					if p.peek().Type == COMMA {
						p.next()
						size2 = p.parseExpr()
					}
				}

				if p.next().Type != RPAREN {
					p.error("Erwartet ')' nach Array-Definition")
				}

				if isPublic {
					stmts = append(stmts, &ast.PublicArrayNode{Loc: loc, Name: name, Size1: size1, Size2: size2})
				} else {
					stmts = append(stmts, &ast.DimArrayNode{
						Loc:    loc,
						Name:   name,
						Size1:  size1,
						Size2:  size2,
						InLoop: inLoopContext,
					})
				}
			} else {
				var init ast.Expr = &ast.NumberNode{Loc: loc, Value: 0}

				if isConst && p.peek().Type != EQ {
					p.error("'Const %s' benötigt einen Wert (z.B. Const %s = 5)", name, name)
				}

				if p.peek().Type == EQ {
					p.next() // =
					if p.peek().Type == LBRACE {
						init = p.parseArrayLiteral()
					} else {
						init = p.parseExpr()
					}
				}

				if isPublic {
					stmts = append(stmts, &ast.PublicNode{Loc: loc, Name: name, Value: init})
				} else {
					stmts = append(stmts, &ast.AssignNode{
						Loc:           loc,
						Name:          name,
						Value:         init,
						IsDeclaration: true,
						InLoop:        inLoopContext,
						IsConst:       isConst,
					})
				}
			}

			if p.peek().Type != COMMA {
				break
			}
			p.next()
		}

		if len(stmts) == 1 {
			return stmts[0]
		}
		return &ast.MultiStmtNode{Loc: p.at(line), Stmts: stmts}

	case PRINT:
		p.next() // 'Print'

		// Print ohne Argument -> Leerzeile
		switch p.peek().Type {
		case NEWLINE, EOF, COMMENT:
			return &ast.PrintNode{Loc: p.at(line), Value: &ast.StringNode{Loc: p.at(line), Value: ""}}
		}

		node := &ast.PrintNode{Loc: p.at(line), Value: p.parseExpr()}

		if p.peek().Type == COMMA {
			p.next()
			node.Color = p.parseExpr()
		}

		return node

	case IF:
		p.next() // IF
		cond := p.parseExpr()
		if p.next().Type != THEN {
			p.error("Erwartet THEN nach IF")
		}
		p.skipStuff()

		branches := []ast.IfBranch{{Cond: cond, Body: p.parseBlock("If", ELSEIF, ELSE, END)}}

		p.skipStuff() // damit peek() das ELSEIF sieht

		for p.peek().Type == ELSEIF {
			p.next() // ELSEIF
			eCond := p.parseExpr()
			if p.next().Type != THEN {
				p.error("Erwartet THEN nach ELSEIF")
			}
			p.skipStuff()

			branches = append(branches, ast.IfBranch{Cond: eCond, Body: p.parseBlock("If", ELSEIF, ELSE, END)})

			p.skipStuff()
		}

		var elseBody []ast.Stmt
		if p.peek().Type == ELSE {
			p.next() // ELSE
			p.skipStuff()
			elseBody = p.parseBlock("If", ELSEIF, ELSE, END)
			p.skipStuff()
		}

		p.expectEnd(IF)
		return &ast.IfNode{Loc: p.at(line), Branches: branches, Else: elseBody}

	case FOR:
		if p.peekNext().Type == EACH {
			return p.parseForEach()
		}

		p.next() // FOR
		varTok := p.next()
		if varTok.Type != IDENT {
			p.error("Erwartet eine Schleifenvariable nach 'For', gefunden: '%s'", varTok.Value)
		}
		varName := varTok.Value
		p.expect(EQ)
		start := p.parseExpr()
		p.expect(TO)
		end := p.parseExpr()

		p.skipStuff()

		step := 1.0
		if p.peek().Type == STEP {
			p.next() // STEP
			multiplier := 1.0
			if p.peek().Type == MINUS {
				p.next() // '-'
				multiplier = -1.0
			}
			nv, ok := p.parseExpr().(*ast.NumberNode)
			if !ok {
				p.error("'Step' erwartet eine Zahl (z.B. Step 2 oder Step -1), keine Variable oder Berechnung.")
			}
			step = nv.Value * multiplier
		}

		p.loopDepth++
		p.loopStack = append(p.loopStack, FOR)

		body := p.parseBlock("For", NEXT)

		p.loopStack = p.loopStack[:len(p.loopStack)-1]
		p.loopDepth--

		if p.peek().Type != NEXT {
			p.error("Erwartet 'Next' am Ende der For-Schleife, gefunden: '%s'", p.peek().Value)
		}
		p.next() // NEXT

		if p.peek().Type == IDENT && strings.EqualFold(p.peek().Value, varName) {
			p.next()
		}

		return &ast.ForNode{
			Loc:     p.at(line),
			VarName: varName,
			Start:   start,
			End:     end,
			Step:    step,
			Body:    body,
		}

	case SUB:
		p.next() // SUB
		nameTok := p.next()
		if nameTok.Type != IDENT {
			p.error("Erwartet Sub-Name nach SUB")
		}

		if len(p.procStack) > 0 {
			p.error("Eine 'Sub'-Deklaration ist innerhalb einer anderen Sub/Function nicht erlaubt (umgebend: '%v').", p.procStack[len(p.procStack)-1])
		}

		params := p.parseParams()

		p.procStack = append(p.procStack, SUB)
		body := p.parseBlock("Sub", END)
		p.procStack = p.procStack[:len(p.procStack)-1]

		p.expectEnd(SUB)

		return &ast.SubNode{
			Loc:    p.at(line),
			Name:   nameTok.Value,
			Params: params,
			Body:   body,
		}

	case FUNCTION:
		p.next() // FUNCTION
		nameTok := p.next()
		if nameTok.Type != IDENT {
			p.error("Erwartet Funktionsname nach FUNCTION")
		}

		if len(p.procStack) > 0 {
			p.error("Eine 'Function'-Deklaration ist innerhalb einer anderen Sub/Function nicht erlaubt (umgebend: '%v').", p.procStack[len(p.procStack)-1])
		}

		params := p.parseParams()

		// VB.NET 'As Type' Teil überspringen
		if p.peek().Type == IDENT && strings.EqualFold(p.peek().Value, "as") {
			p.next() // 'As'
			p.next() // 'Integer', 'String', etc.
		}

		p.procStack = append(p.procStack, FUNCTION)
		body := p.parseBlock("Function", END)
		p.procStack = p.procStack[:len(p.procStack)-1]

		p.expectEnd(FUNCTION)

		return &ast.FuncNode{
			Loc:    p.at(line),
			Name:   nameTok.Value,
			Params: params,
			Body:   body,
		}

	case TRY:
		p.next() // TRY

		node := &ast.TryNode{
			Loc:     p.at(line),
			TryBody: p.parseBlock("Try", CATCH, FINALLY),
		}
		p.skipStuff()

		if p.peek().Type == CATCH {
			node.HasCatch = true
			p.next() // CATCH

			// Variablenname nur, wenn er in derselben Zeile steht
			if p.peek().Type == IDENT {
				node.CatchVarName = p.expectIdentifier()
			}

			p.skipStuff()
			node.CatchBody = p.parseBlock("Try", FINALLY)
			p.skipStuff()
		}
		if p.peek().Type == FINALLY {
			node.HasFinally = true
			p.next()
			p.skipStuff()
			node.FinallyBody = p.parseBlock("Try", FINALLY)
			p.skipStuff()
		}

		if !node.HasCatch && !node.HasFinally {
			p.error("Ein 'Try'-Block braucht mindestens einen 'Catch'- oder 'Finally'-Zweig.")
		}

		p.expectEnd(TRY)
		return node

	case RETURN:
		p.next()

		// Return ohne Wert (z.B. in einer Sub)
		switch p.peek().Type {
		case NEWLINE, EOF, END, COMMENT:
			return &ast.ReturnNode{Loc: p.at(line), Value: &ast.NullNode{Loc: p.at(line)}}
		}

		return &ast.ReturnNode{Loc: p.at(line), Value: p.parseExpr()}

	case WHILE:
		p.next() // WHILE
		cond := p.parseExpr()

		p.loopDepth++
		p.loopStack = append(p.loopStack, WHILE)

		body := p.parseBlock("While", END)

		p.loopStack = p.loopStack[:len(p.loopStack)-1]
		p.loopDepth--

		p.expectEnd(WHILE)
		return &ast.WhileNode{Loc: p.at(line), Condition: cond, Body: body}

	case DO:
		p.next() // DO

		var cond ast.Expr
		isUntil := false
		checkAtEnd := false

		// Kopf-Bedingung: nur, wenn While/Until in DERSELBEN Zeile steht
		if p.peek().Type == WHILE {
			p.next()
			cond = p.parseExpr()
		} else if p.peek().Type == UNTIL {
			p.next()
			cond = p.parseExpr()
			isUntil = true
		}

		p.loopDepth++
		p.loopStack = append(p.loopStack, DO)

		body := p.parseBlock("Do", LOOP)

		p.loopStack = p.loopStack[:len(p.loopStack)-1]
		p.loopDepth--

		if p.peek().Type != LOOP {
			p.error("Erwartet 'Loop' am Ende des Do-Blocks, gefunden: '%s'", p.peek().Value)
		}
		p.next() // LOOP

		// Fuß-Bedingung
		if cond == nil && (p.peek().Type == WHILE || p.peek().Type == UNTIL) {
			checkAtEnd = true
			if p.peek().Type == UNTIL {
				isUntil = true
			}
			p.next()
			cond = p.parseExpr()
		}

		return &ast.DoLoopNode{
			Loc:        p.at(line),
			Condition:  cond,
			Body:       body,
			IsUntil:    isUntil,
			CheckAtEnd: checkAtEnd,
		}

	case EXIT:
		p.next() // 'Exit'

		exitType := ""

		switch p.peek().Type {
		case SUB:
			exitType = "Sub"
			p.next()
		case FUNCTION:
			exitType = "Function"
			p.next()
		case FOR, WHILE, DO:
			if p.loopDepth == 0 {
				p.error("'Exit %s' darf nur innerhalb einer Schleife verwendet werden.", p.peek().Value)
			}
			exitType = p.peek().Value
			p.next()
		}

		return &ast.ExitNode{Loc: p.at(line), ExitType: exitType}

	case CONTINUE:
		p.next() // CONTINUE

		if p.loopDepth == 0 || len(p.loopStack) == 0 {
			p.error("'Continue' darf nur innerhalb einer Schleife verwendet werden.")
		}

		continueType := ""

		switch p.peek().Type {
		case FOR, WHILE, DO:
			continueType = p.peek().Value
			current := p.loopStack[len(p.loopStack)-1]

			if p.peek().Type != current {
				p.error("'Continue %s' passt nicht zur umgebenden Schleife (aktuell: '%v').",
					continueType, current)
			}

			p.next()
		}

		return &ast.ContinueNode{Loc: p.at(line), ContinueType: continueType}

	case INCLUDE:
		return p.parseInclude()

	case IDENT:
		nameTok := p.next()
		fullName := nameTok.Value
		for p.peek().Type == DOT {
			p.next()
			nextTok := p.next()
			if nextTok.Type == EOF {
				p.error("Syntaxfehler: Nach dem Punkt (.) wurde ein Name erwartet, aber das Dateiende erreicht.")
			}
			if nextTok.Type == ERROR {
				p.error("%s", nextTok.Value)
			}
			fullName += "." + nextTok.Value
		}

		// entry["path"] = ... explizit als ungültiges Zuweisungsziel abfangen
		if p.peek().Type == LBRACKET {
			p.rejectBracketAssignTarget()
			p.error("Ein Ausdruck mit '[...]' kann nicht als eigenständige Anweisung stehen.")
		}

		// Funktionsaufruf oder Array-Zugriff?
		if p.peek().Type == LPAREN {
			p.next() // '('

			args := p.parseCallArguments(fullName)

			// grp(i)["path"] = ...
			if p.peek().Type == LBRACKET {
				p.rejectBracketAssignTarget()
				p.error("Ein Ausdruck mit '[...]' kann nicht als eigenständige Anweisung stehen.")
			}

			// Fall A: Zuweisung an Array-Index
			if p.peek().Type == EQ {
				p.next()

				if len(args) == 0 {
					p.error("Array-Index erwartet")
				}

				node := &ast.ArrayAssignNode{
					Loc:   p.at(line),
					Name:  fullName,
					Index: args[0],
					Value: p.parseExpr(),
				}

				if len(args) > 1 {
					node.Index2 = args[1]
				}

				return node
			}

			// Fall A2: arr(i) += 1
			switch p.peek().Type {
			case PLUS_ASSIGN, MINUS_ASSIGN, MUL_ASSIGN, DIV_ASSIGN:
				if len(args) == 0 {
					p.error("Array-Index erwartet")
				}
				opTok := p.next()
				val := p.parseExpr()

				node := &ast.ArrayAssignNode{
					Loc:   p.at(line),
					Name:  fullName,
					Index: args[0],
					Value: &ast.BinOpNode{
						Loc:   p.at(opTok.Line),
						Left:  &ast.CallExprNode{Loc: p.at(line), Name: fullName, Args: args},
						Op:    mapAssignOp(opTok.Type),
						Right: val,
					},
				}
				if len(args) > 1 {
					node.Index2 = args[1]
				}
				return node
			}

			// Fall B: Funktionsaufruf als Anweisung
			return &ast.CallNode{Loc: p.at(line), Name: fullName, Args: args}
		}

		// Einfache Variablen-Zuweisung -> x = 10
		if p.peek().Type == EQ {
			p.next() // '='
			val := p.parseExpr()
			p.skipStuff()
			return &ast.AssignNode{Loc: p.at(line), Name: fullName, Value: val}
		}

		// Kombinierte Zuweisung -> x += 10
		switch p.peek().Type {
		case PLUS_ASSIGN, MINUS_ASSIGN, MUL_ASSIGN, DIV_ASSIGN:
			op := p.next().Type
			val := p.parseExpr()
			p.skipStuff()

			return &ast.CompoundAssignNode{
				Loc:   p.at(line),
				Left:  &ast.VarNode{Loc: p.at(line), Name: fullName},
				Op:    op,
				Right: val,
			}
		}

		// Fehlerfall: Alles, was kein Befehl ist
		tok := p.peek()
		prevTok := p.tokens[p.pos-1]

		if tok.Type == EOF || tok.Type == NEWLINE {
			p.error("Die Anweisung mit '%s' ist unvollständig. Hinweis: Ein Name darf nicht alleine stehen.", prevTok.Value)
			return nil
		}

		p.error("Unerwartetes Token '%s' nach '%s'.", tok.Value, prevTok.Value)
		return nil

	default:
		t := p.peek()
		p.error("Ich kann mit dem Zeichen '%s' (Typ: %v) an dieser Stelle nichts anfangen.", t.Value, t.Type)
		return nil
	}
}
