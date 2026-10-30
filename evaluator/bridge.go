package evaluator

// Brücke zwischen dem 1:1 aus dem Interpreter übernommenen Evaluator und
// den portierten Paketen von vbxc.
//
// In vbx lagen AST, Token, Laufzeit-Werte und die Builtins-Registry in
// einem einzigen Package; in vbxc stecken sie in ast, token, value und
// stdlib. Die Aliase und Wrapper hier unten halten evaluator.go dadurch
// unverändert im Interpreter-Stil -- geändert wurde nur dort, wo das AST
// echte Interfaces verlangt (Bool-/Null-Literale statt Value direkt im
// AST) oder Zustand braucht (Select-Dispatch-Cache).

import (
	"sync"

	"vbxc/ast"
	. "vbxc/token"
	"vbxc/value"
)

// ---------------- Token / AST ----------------

// TokenType ist der altgewohnte Name für den Token-Typ aus vbxc/token
// (per Dot-Import sind PLUS, MINUS, EQ, ... ohnehin direkt sichtbar).
type TokenType = Type

type (
	Expr = ast.Expr
	Stmt = ast.Stmt
)

// Ausdrücke
type (
	NumberNode       = ast.NumberNode
	StringNode       = ast.StringNode
	BoolNode         = ast.BoolNode
	NullNode         = ast.NullNode
	VarNode          = ast.VarNode
	CallExprNode     = ast.CallExprNode
	UnaryOpNode      = ast.UnaryOpNode
	BinOpNode        = ast.BinOpNode
	MapIndexNode     = ast.MapIndexNode
	ArrayLiteralNode = ast.ArrayLiteralNode
	RangeNode        = ast.RangeNode
	IsNode           = ast.IsNode
)

// Anweisungen
type (
	AssignNode         = ast.AssignNode
	CompoundAssignNode = ast.CompoundAssignNode
	ArrayAssignNode    = ast.ArrayAssignNode
	DimArrayNode       = ast.DimArrayNode
	PublicNode         = ast.PublicNode
	PublicArrayNode    = ast.PublicArrayNode
	PrintNode          = ast.PrintNode
	IfBranch           = ast.IfBranch
	IfNode             = ast.IfNode
	CaseBranch         = ast.CaseBranch
	SelectNode         = ast.SelectNode
	ForNode            = ast.ForNode
	ForEachNode        = ast.ForEachNode
	WhileNode          = ast.WhileNode
	DoLoopNode         = ast.DoLoopNode
	ParamDef           = ast.ParamDef
	SubNode            = ast.SubNode
	FuncNode           = ast.FuncNode
	CallNode           = ast.CallNode
	ReturnNode         = ast.ReturnNode
	ExitNode           = ast.ExitNode
	ContinueNode       = ast.ContinueNode
	TryNode            = ast.TryNode
	MultiStmtNode      = ast.MultiStmtNode
)

// ---------------- Laufzeit-Werte ----------------

type (
	Kind   = value.Kind
	Value  = value.Value
	Object = value.Object
)

const (
	KindInvalid   = value.KindInvalid
	KindUndefined = value.KindUndefined
	KindNum       = value.KindNum
	KindStr       = value.KindStr
	KindArr       = value.KindArr
	KindArr2D     = value.KindArr2D
	KindObj       = value.KindObj
	KindNull      = value.KindNull
	KindBool      = value.KindBool
	KindError     = value.KindError
	KindMap       = value.KindMap
	KindNil       = value.KindNil
	KindNone      = value.KindNone
	KindBytes     = value.KindBytes
)

var NilValue = value.NilValue

// ---------------- Helfer aus value/ ----------------
//
// Die Wrapper behalten die alten (teils unexportierten) Namen bei, damit
// evaluator.go unverändert bleibt.

func NumVal(f float64) Value   { return value.NumVal(f) }
func StrVal(s string) Value    { return value.StrVal(s) }
func BoolVal(b bool) Value     { return value.BoolVal(b) }
func NullVal() Value           { return value.NullVal() }
func ErrorVal(m string) Value  { return value.ErrorVal(m) }
func ToString(v Value) string  { return value.ToString(v) }
func toNumVal(v Value) float64 { return value.ToNum(v) }
func isTruthy(v Value) bool    { return value.IsTruthy(v) }
func tryNumber(v Value) (float64, bool) {
	return value.TryNumber(v)
}
func compareNum(op TokenType, a, b float64) bool { return value.CompareNum(op, a, b) }
func toIndex(v Value) (int, Value)               { return value.ToIndex(v) }
func toFloatSafe(args []Value, index int) float64 {
	return value.ToFloatSafe(args, index)
}
func requireNumber(v Value, context string) (float64, Value) {
	return value.RequireNumber(v, context)
}

// requiredParams zählt die Pflichtparameter eines User-Functions/Subs --
// im Interpreter stand der Wert als Feld am Knoten, in vbxc wird er aus
// der Paramterliste berechnet (ast.RequiredParams).
func requiredParams(params []ParamDef) int { return ast.RequiredParams(params) }

// ---------------- Select-Dispatch-Cache ----------------
//
// Der Fastpath für SELECT/CASE steckte im Interpreter als Feld am
// ast.SelectNode; der vbxc-AST ist rein datengetrieben, deshalb liegt der
// Zustand hier in einer Neben-Map (pro Knoten einmalig aufgebaut).

type selectDispatch struct {
	numCache map[float64]int
	strCache map[string]int
	ok       bool
	built    bool
}

var selectDispatches sync.Map // *SelectNode -> *selectDispatch

func selectDispatchFor(n *SelectNode) *selectDispatch {
	if v, found := selectDispatches.Load(n); found {
		return v.(*selectDispatch)
	}
	v, _ := selectDispatches.LoadOrStore(n, &selectDispatch{})
	return v.(*selectDispatch)
}

// ---------------- Knoten-Caches ----------------
//
// AssignNode, CompoundAssignNode, VarNode und BinOpNode tragen im
// Interpreter Cache-Felder (Ziel-Environment, Value-Pointer, Const-Flag,
// Punkt-Notation, Constant-Folding). Der vbxc-AST bleibt davon frei --
// der Zustand liegt pro Knoten in dieser Neben-Map.
type nodeCache struct {
	lookupEnv *Environment // cachedLookupEnv
	defEnv    *Environment // cachedDefEnv
	ptr       *Value       // cachedPtr
	isConst   bool         // cachedConst
	value     Value        // cachedValue (Constant Folding)

	dotChecked bool // Punkt-Notation einmalig splitten
	isDotted   bool
	objName    string
	fieldName  string
}

var nodeCaches sync.Map // AST-Knoten -> *nodeCache

func nodeCacheFor(n any) *nodeCache {
	if v, found := nodeCaches.Load(n); found {
		return v.(*nodeCache)
	}
	v, _ := nodeCaches.LoadOrStore(n, &nodeCache{})
	return v.(*nodeCache)
}
