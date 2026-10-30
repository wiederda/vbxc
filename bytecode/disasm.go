package bytecode

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"vbxc/token"
	"vbxc/value"
)

// Operandenart pro Slot (A, B, C):
//
//	k Konstante   F Funktion   n Name   v optionaler Name (+1)   j Sprungziel   i Zahl   o Operator   f Flags   - ungenutzt
type opMeta struct {
	name     string
	operands string
}

var meta = map[Op]opMeta{
	OpHalt: {"HALT", "---"},

	OpConst: {"CONST", "k--"},
	OpNull:  {"NULL", "---"},
	OpTrue:  {"TRUE", "---"},
	OpFalse: {"FALSE", "---"},
	OpPop:   {"POP", "---"},
	OpPopN:  {"POPN", "i--"},

	OpLoad:          {"LOAD", "n--"},
	OpLoadField:     {"LOADFIELD", "nn-"},
	OpLoadForUpdate: {"LOADUPD", "n--"},
	OpDeclare:       {"DECLARE", "nf-"},
	OpStore:         {"STORE", "n--"},
	OpStoreRef:      {"STOREREF", "n--"},
	OpSetGlobal:     {"SETGLOBAL", "n--"},

	OpAdd:    {"ADD", "---"},
	OpSub:    {"SUB", "---"},
	OpMul:    {"MUL", "---"},
	OpDiv:    {"DIV", "---"},
	OpConcat: {"CONCAT", "---"},
	OpEq:     {"EQ", "---"},
	OpNeq:    {"NEQ", "---"},
	OpLt:     {"LT", "---"},
	OpGt:     {"GT", "---"},
	OpLe:     {"LE", "---"},
	OpGe:     {"GE", "---"},
	OpCalc:   {"CALC", "o--"},

	OpNot:          {"NOT", "---"},
	OpNeg:          {"NEG", "---"},
	OpTruthy:       {"TRUTHY", "---"},
	OpShortCircuit: {"SHORT", "ji-"},

	OpJump:        {"JMP", "j--"},
	OpJumpIfFalse: {"JMPF", "j--"},
	OpJumpIfTrue:  {"JMPT", "j--"},
	OpJumpIfError: {"JMPERR", "ji-"},
	OpCheck:       {"CHECK", "---"},

	OpCall:      {"CALL", "ni-"},
	OpIsError:   {"ISERROR", "---"},
	OpErrorText: {"ERRORTEXT", "---"},

	OpMakeArray: {"ARRAY", "i--"},
	OpMapBase:   {"MAPBASE", "j--"},
	OpMapIndex:  {"MAPINDEX", "---"},

	OpForInit: {"FORINIT", "nk-"},
	OpForTest: {"FORTEST", "jni"},
	OpForStep: {"FORSTEP", "k--"},

	OpDimArray:    {"DIMARRAY", "nii"},
	OpPublicArray: {"PUBARRAY", "ni-"},
	OpArrCheck:    {"ARRCHECK", "n--"},
	OpToIndex:     {"TOINDEX", "---"},
	OpArrGuard2D:  {"ARRGUARD2D", "nj-"},
	OpArrStore:    {"ARRSTORE", "ni-"},

	OpEachInit: {"EACHINIT", "nv-"},
	OpEachNext: {"EACHNEXT", "jnv"},

	OpDup:            {"DUP", "---"},
	OpSelectDispatch: {"SELECT", "i--"},
	OpCaseEq:         {"CASEEQ", "---"},
	OpCaseRange:      {"CASERANGE", "---"},
	OpCaseIs:         {"CASEIS", "o--"},

	OpDefine:       {"DEFINE", "F--"},
	OpReturn:       {"RETURN", "i--"},
	OpSetReturn:    {"SETRETURN", "---"},
	OpJumpIfArg:    {"JMPARG", "ji-"},
	OpParamDefault: {"PARAMDEF", "n--"},

	OpPrint: {"PRINT", "i--"},
	OpCls:   {"CLS", "---"},
}

func (o Op) String() string {
	if m, ok := meta[o]; ok {
		return m.name
	}
	return "OP?" + strconv.Itoa(int(o))
}

func constText(v value.Value) string {
	if v.Kind == value.KindStr {
		return strconv.Quote(v.Str)
	}
	return value.ToString(v)
}

// Disassemble schreibt den Chunk lesbar nach w: erst das Hauptprogramm, dann
// jede Sub/Function.
func Disassemble(w io.Writer, c *Chunk) {
	disasmUnit(w, c, c.Name, c.Code, c.Pos)
	for _, f := range c.Funcs {
		kind := "Sub"
		if f.IsFunc {
			kind = "Function"
		}
		fmt.Fprintln(w)
		disasmUnit(w, c, fmt.Sprintf("%s %s(%s)", kind, f.Name, strings.Join(f.Params, ", ")), f.Code, f.Pos)
	}
}

func disasmUnit(w io.Writer, c *Chunk, title string, code []Instr, posTab []Pos) {
	fmt.Fprintf(w, "== %s ==\n", title)
	lastLine, lastFile := int32(-1), int32(-1)
	for i, in := range code {
		m, ok := meta[in.Op]
		if !ok {
			m = opMeta{in.Op.String(), "---"}
		}

		pos := posTab[i]
		lineCol := "   |"
		if pos.Line != lastLine || pos.File != lastFile {
			lineCol = fmt.Sprintf("%4d", pos.Line)
			lastLine, lastFile = pos.Line, pos.File
		}

		fmt.Fprintf(w, "%04d %s  %-10s", i, lineCol, m.name)

		ops := [3]int32{in.A, in.B, in.C}
		for k := 0; k < 3; k++ {
			x := ops[k]
			switch m.operands[k] {
			case 'k':
				fmt.Fprintf(w, " %s", constText(c.Consts[x]))
			case 'F':
				fmt.Fprintf(w, " %s", c.Funcs[x].Name)
			case 'n':
				fmt.Fprintf(w, " %s", c.Names[x])
			case 'j':
				fmt.Fprintf(w, " ->%04d", x)
			case 'i':
				fmt.Fprintf(w, " %d", x)
			case 'v': // optionaler Name (Index+1, 0 = keiner)
				if x == 0 {
					fmt.Fprint(w, " _")
				} else {
					fmt.Fprintf(w, " %s", c.Names[x-1])
				}
			case 'o':
				fmt.Fprintf(w, " %v", token.Type(x))
			case 'f':
				if x&FlagConst != 0 {
					fmt.Fprint(w, " [const]")
				}
				if x&FlagInLoop != 0 {
					fmt.Fprint(w, " [inloop]")
				}
			}
		}
		fmt.Fprintln(w)
	}
}
