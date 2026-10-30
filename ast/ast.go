// Package ast enthält nur Datenstrukturen: keine Caches, keine Abhängigkeit
// zu Value/Environment. Jeder Knoten kennt seine Position (Datei + Zeile).
package ast

import "vbxc/token"

// Loc wird in jeden Knoten eingebettet.
type Loc struct {
	File string // "" = Hauptdatei bzw. unbekannt
	Line int
}

func (l Loc) Pos() Loc { return l }

type Node interface{ Pos() Loc }

// Expr und Stmt sind echte Interfaces mit Marker-Methode: ein Knoten, der in
// keiner der beiden Gruppen steht, lässt sich gar nicht erst einsetzen.
type Expr interface {
	Node
	exprNode()
}

type Stmt interface {
	Node
	stmtNode()
}

// ---------------- Ausdrücke ----------------

type NumberNode struct {
	Loc
	Value float64
}

type StringNode struct {
	Loc
	Value string
}

// BoolNode ersetzt das bisherige "Value{Kind: KindBool}" direkt im AST.
type BoolNode struct {
	Loc
	Value bool
}

// NullNode ist das Literal "Nothing" (bisher NullVal() direkt im AST).
type NullNode struct{ Loc }

// VarNode: Name kann Punkte enthalten ("objekt.feld"); ob das ein Objektfeld
// oder ein Namespace ist, entscheidet der Compiler/die VM.
type VarNode struct {
	Loc
	Name string
}

// CallExprNode ist Funktionsaufruf ODER Array-Zugriff "arr(i)": der Parser
// kann das nicht unterscheiden, das passiert erst zur Laufzeit.
type CallExprNode struct {
	Loc
	Name string
	Args []Expr
}

type UnaryOpNode struct {
	Loc
	Op    token.Type // NOT oder MINUS
	Right Expr
}

type BinOpNode struct {
	Loc
	Left  Expr
	Op    token.Type
	Right Expr
}

// MapIndexNode: m["key"], nur lesend, verkettbar.
type MapIndexNode struct {
	Loc
	Base Expr
	Key  Expr
}

type ArrayLiteralNode struct {
	Loc
	Elements []Expr
}

// RangeNode und IsNode kommen nur in Case-Bedingungen vor.
type RangeNode struct {
	Loc
	Low  Expr
	High Expr
}

type IsNode struct {
	Loc
	Operator token.Type
	Value    Expr
}

func (*NumberNode) exprNode()       {}
func (*StringNode) exprNode()       {}
func (*BoolNode) exprNode()         {}
func (*NullNode) exprNode()         {}
func (*VarNode) exprNode()          {}
func (*CallExprNode) exprNode()     {}
func (*UnaryOpNode) exprNode()      {}
func (*BinOpNode) exprNode()        {}
func (*MapIndexNode) exprNode()     {}
func (*ArrayLiteralNode) exprNode() {}
func (*RangeNode) exprNode()        {}
func (*IsNode) exprNode()           {}

// ---------------- Anweisungen ----------------

type AssignNode struct {
	Loc
	Name          string
	Value         Expr
	IsDeclaration bool
	InLoop        bool
	IsConst       bool
}

type CompoundAssignNode struct {
	Loc
	Left  Expr // aktuell immer *VarNode
	Op    token.Type
	Right Expr
}

type ArrayAssignNode struct {
	Loc
	Name   string
	Index  Expr
	Index2 Expr // nil bei 1D
	Value  Expr
}

type DimArrayNode struct {
	Loc
	Name   string
	Size1  Expr // nil bei "Dim a()"
	Size2  Expr // nil bei 1D
	InLoop bool
}

type PublicNode struct {
	Loc
	Name  string
	Value Expr
}

type PublicArrayNode struct {
	Loc
	Name  string
	Size1 Expr
	Size2 Expr // nil bei 1D
}

type PrintNode struct {
	Loc
	Value Expr
	Color Expr // nil, wenn keine Farbe angegeben
}

type IfBranch struct {
	Cond Expr
	Body []Stmt
}

type IfNode struct {
	Loc
	Branches []IfBranch
	Else     []Stmt
}

type CaseBranch struct {
	Conditions []Expr
	Body       []Stmt
}

type SelectNode struct {
	Loc
	Expression Expr
	Cases      []CaseBranch
	Default    []Stmt
}

type ForNode struct {
	Loc
	VarName string
	Start   Expr
	End     Expr
	Step    float64 // zur Parse-Zeit konstant
	Body    []Stmt
}

type ForEachNode struct {
	Loc
	KeyVar   string
	ValVar   string // "" bei Arrays
	Iterable Expr
	Body     []Stmt
}

type WhileNode struct {
	Loc
	Condition Expr
	Body      []Stmt
}

type DoLoopNode struct {
	Loc
	Condition  Expr // nil = Endlosschleife
	Body       []Stmt
	CheckAtEnd bool // false = Prüfung oben, true = Prüfung unten
	IsUntil    bool // Until statt While
}

type ParamDef struct {
	Name       string
	IsOptional bool
	Default    Expr // nil, wenn kein Default
}

type SubNode struct {
	Loc
	Name   string
	Params []ParamDef
	Body   []Stmt
}

type FuncNode struct {
	Loc
	Name   string
	Params []ParamDef
	Body   []Stmt
}

// RequiredParams zählt die Pflichtparameter (Pflicht steht immer vor optional).
func RequiredParams(params []ParamDef) int {
	n := 0
	for _, p := range params {
		if !p.IsOptional {
			n++
		}
	}
	return n
}

// CallNode: Aufruf als eigenständige Anweisung (Fehler brechen das Skript ab).
type CallNode struct {
	Loc
	Name string
	Args []Expr
}

type ReturnNode struct {
	Loc
	Value Expr // *NullNode, wenn ohne Wert
}

type ExitNode struct {
	Loc
	ExitType string // "", "Sub", "Function" oder Schleifenart wie getippt
}

type ContinueNode struct {
	Loc
	ContinueType string
}

// TryNode hat explizite HasCatch/HasFinally-Flags. Vorher stand "nil" für
// "Zweig fehlt", dadurch galt ein Catch nur mit Kommentar (leerer Body = nil)
// fälschlich als nicht vorhanden.
type TryNode struct {
	Loc
	TryBody      []Stmt
	HasCatch     bool
	CatchVarName string // "" wenn ohne Variable
	CatchBody    []Stmt
	HasFinally   bool
	FinallyBody  []Stmt
}

// MultiStmtNode bündelt mehrere Statements (Dim a, b / eingebundene Dateien).
type MultiStmtNode struct {
	Loc
	Stmts []Stmt
}

func (*AssignNode) stmtNode()         {}
func (*CompoundAssignNode) stmtNode() {}
func (*ArrayAssignNode) stmtNode()    {}
func (*DimArrayNode) stmtNode()       {}
func (*PublicNode) stmtNode()         {}
func (*PublicArrayNode) stmtNode()    {}
func (*PrintNode) stmtNode()          {}
func (*IfNode) stmtNode()             {}
func (*SelectNode) stmtNode()         {}
func (*ForNode) stmtNode()            {}
func (*ForEachNode) stmtNode()        {}
func (*WhileNode) stmtNode()          {}
func (*DoLoopNode) stmtNode()         {}
func (*SubNode) stmtNode()            {}
func (*FuncNode) stmtNode()           {}
func (*CallNode) stmtNode()           {}
func (*ReturnNode) stmtNode()         {}
func (*ExitNode) stmtNode()           {}
func (*ContinueNode) stmtNode()       {}
func (*TryNode) stmtNode()            {}
func (*MultiStmtNode) stmtNode()      {}
