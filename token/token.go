// Package token enthält die Token-Typen und die Keyword-Tabelle.
// Reihenfolge der Konstanten ist identisch zum Interpreter.
package token

type Type int

const (
	EOF Type = iota
	IDENT
	NUMBER
	STRING
	PLUS
	MINUS
	MUL
	DIV
	ASSIGN
	LPAREN
	RPAREN
	DIM
	PRINT
	IF
	THEN
	ELSE
	ELSEIF
	LT
	GT
	LE
	GE
	EQ
	NEQ
	FOR
	NEXT
	TO
	IS
	STEP
	AMP
	SUB
	COMMA
	FUNCTION
	RETURN
	DOT
	AND
	BOOL
	OR
	NOT
	WHILE
	DO
	LOOP
	EXIT
	SELECT
	CASE
	END
	EACH
	IN
	NEWLINE
	INCLUDE
	COMMENT
	LBRACKET // [
	RBRACKET // ]
	PUBLIC
	LBRACE // {
	RBRACE // }
	PLUS_ASSIGN
	MINUS_ASSIGN
	MUL_ASSIGN
	DIV_ASSIGN
	ERROR
	UNTIL
	CONTINUE
	CONST
	TRY
	CATCH
	FINALLY
	NOTHING
)

// Token trägt jetzt zusätzlich die Zeile, in der es steht (neu gegenüber
// dem Interpreter). Block-Kommentare tragen ihre Startzeile.
type Token struct {
	Type  Type
	Value string
	Line  int
}

// Indexierte Schreibweise: Reihenfolge der Einträge kann nicht mehr
// versehentlich von den Konstanten abweichen.
var names = [...]string{
	EOF:          "EOF",
	IDENT:        "IDENT",
	NUMBER:       "NUMBER",
	STRING:       "STRING",
	PLUS:         "+",
	MINUS:        "-",
	MUL:          "*",
	DIV:          "/",
	ASSIGN:       "=",
	LPAREN:       "(",
	RPAREN:       ")",
	DIM:          "DIM",
	PRINT:        "PRINT",
	IF:           "IF",
	THEN:         "THEN",
	ELSE:         "ELSE",
	ELSEIF:       "ELSEIF",
	LT:           "<",
	GT:           ">",
	LE:           "<=",
	GE:           ">=",
	EQ:           "=",
	NEQ:          "<>",
	FOR:          "FOR",
	NEXT:         "NEXT",
	TO:           "TO",
	IS:           "IS",
	STEP:         "STEP",
	AMP:          "&",
	SUB:          "SUB",
	COMMA:        ",",
	FUNCTION:     "FUNCTION",
	RETURN:       "RETURN",
	DOT:          ".",
	AND:          "AND",
	BOOL:         "BOOL",
	OR:           "OR",
	NOT:          "NOT",
	WHILE:        "WHILE",
	DO:           "DO",
	LOOP:         "LOOP",
	EXIT:         "EXIT",
	SELECT:       "SELECT",
	CASE:         "CASE",
	END:          "END",
	EACH:         "EACH",
	IN:           "IN",
	NEWLINE:      "NEWLINE",
	INCLUDE:      "INCLUDE",
	COMMENT:      "COMMENT",
	LBRACKET:     "[",
	RBRACKET:     "]",
	PUBLIC:       "PUBLIC",
	LBRACE:       "{",
	RBRACE:       "}",
	PLUS_ASSIGN:  "+=",
	MINUS_ASSIGN: "-=",
	MUL_ASSIGN:   "*=",
	DIV_ASSIGN:   "/=",
	ERROR:        "ERROR",
	UNTIL:        "UNTIL",
	CONTINUE:     "CONTINUE",
	CONST:        "CONST",
	TRY:          "TRY",
	CATCH:        "CATCH",
	FINALLY:      "FINALLY",
	NOTHING:      "NOTHING",
}

func (t Type) String() string {
	if t >= 0 && int(t) < len(names) && names[t] != "" {
		return names[t]
	}
	return "TOKEN"
}

var Keywords = map[string]Type{
	"dim":      DIM,
	"public":   PUBLIC,
	"print":    PRINT,
	"if":       IF,
	"then":     THEN,
	"else":     ELSE,
	"elseif":   ELSEIF,
	"for":      FOR,
	"next":     NEXT,
	"each":     EACH,
	"is":       IS,
	"to":       TO,
	"step":     STEP,
	"sub":      SUB,
	"function": FUNCTION,
	"return":   RETURN,
	"in":       IN,
	"include":  INCLUDE,
	"const":    CONST,

	"true":    BOOL,
	"false":   BOOL,
	"nothing": NOTHING,
	"and":     AND,
	"or":      OR,
	"not":     NOT,

	"while":    WHILE,
	"do":       DO,
	"loop":     LOOP,
	"until":    UNTIL,
	"exit":     EXIT,
	"continue": CONTINUE,

	"case":    CASE,
	"select":  SELECT,
	"end":     END,
	"try":     TRY,
	"catch":   CATCH,
	"finally": FINALLY,
}
