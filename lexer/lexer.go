// Package lexer zerlegt VBX-Quelltext in Tokens.
// Gegenüber dem Interpreter: jedes Token trägt seine Zeile.
package lexer

import (
	"fmt"
	"strings"
	"unicode"

	. "vbxc/token" // Token-Konstanten (IDENT, PLUS, ...) bewusst ohne Präfix
)

// nur ASCII-Ziffern zählen als Zahl
func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// Tokenize ist der Standard-Einstieg (Startzeile 1).
func Tokenize(input string) []Token {
	return TokenizeFrom(input, 1)
}

// TokenizeFrom wie Tokenize, aber mit frei wählbarer Startzeile
// (für Ausdrücke in interpolierten Strings).
func TokenizeFrom(input string, startLine int) []Token {
	tokens := make([]Token, 0, len(input)/3+16)

	runes := []rune(input)
	i := 0
	line := startLine

	emit := func(t Type, value string) {
		tokens = append(tokens, Token{Type: t, Value: value, Line: line})
	}

	emitError := func(format string, args ...any) {
		emit(ERROR, fmt.Sprintf("Zeile %d: %s",
			line,
			fmt.Sprintf(format, args...),
		))
	}

	for i < len(runes) {
		ch := runes[i]

		// 1. BLOCK-KOMMENTAR /' ... '/
		if ch == '/' && i+1 < len(runes) && runes[i+1] == '\'' {
			commentLine := line // Startzeile merken
			i += 2
			start := i

			for i+1 < len(runes) &&
				!(runes[i] == '\'' && runes[i+1] == '/') {
				if runes[i] == '\n' {
					line++
				}
				i++
			}

			if i+1 >= len(runes) {
				emitError("Nicht geschlossener Block-Kommentar")
				break
			}

			tokens = append(tokens, Token{Type: COMMENT, Value: string(runes[start:i]), Line: commentLine})

			i += 2
			continue
		}

		// Einzeiliger Kommentar
		if ch == '\'' {
			i++
			start := i

			for i < len(runes) && runes[i] != '\n' {
				i++
			}

			emit(COMMENT, string(runes[start:i]))
			continue
		}

		// Doppel-Operatoren (+=, -=, *=, /=, <>, <=, >=)
		if i+1 < len(runes) {
			switch ch {
			case '+', '-', '*', '/', '<', '>':
				next := runes[i+1]
				switch {
				case ch == '+' && next == '=':
					emit(PLUS_ASSIGN, "+=")
					i += 2
					continue
				case ch == '-' && next == '=':
					emit(MINUS_ASSIGN, "-=")
					i += 2
					continue
				case ch == '*' && next == '=':
					emit(MUL_ASSIGN, "*=")
					i += 2
					continue
				case ch == '/' && next == '=':
					emit(DIV_ASSIGN, "/=")
					i += 2
					continue
				case ch == '<' && next == '>':
					emit(NEQ, "<>")
					i += 2
					continue
				case ch == '<' && next == '=':
					emit(LE, "<=")
					i += 2
					continue
				case ch == '>' && next == '=':
					emit(GE, ">=")
					i += 2
					continue
				}
			}
		}

		// $"..." interpolierter String
		if ch == '$' && i+1 < len(runes) && runes[i+1] == '"' {
			i += 2 // $" konsumieren
			tokenizeInterpolatedString(runes, &i, line, emit, emitError)
			continue
		}

		switch {
		case ch == '\r':
			i++

		case ch == '\n':
			emit(NEWLINE, "\n")
			line++
			i++

		case unicode.IsSpace(ch):
			i++

		// Operatoren
		case ch == '+':
			emit(PLUS, string(ch))
			i++
		case ch == '-':
			emit(MINUS, string(ch))
			i++
		case ch == '*':
			emit(MUL, string(ch))
			i++
		case ch == '/':
			emit(DIV, string(ch))
			i++
		case ch == '=':
			emit(EQ, string(ch))
			i++
		case ch == '(':
			emit(LPAREN, string(ch))
			i++
		case ch == ')':
			emit(RPAREN, string(ch))
			i++
		case ch == '{':
			emit(LBRACE, string(ch))
			i++
		case ch == '}':
			emit(RBRACE, string(ch))
			i++
		case ch == '[':
			emit(LBRACKET, string(ch))
			i++
		case ch == ']':
			emit(RBRACKET, string(ch))
			i++
		case ch == '&':
			emit(AMP, string(ch))
			i++
		case ch == ',':
			emit(COMMA, string(ch))
			i++
		case ch == '.':
			emit(DOT, string(ch))
			i++

		// Vergleichsoperatoren
		case ch == '<':
			emit(LT, "<")
			i++

		case ch == '>':
			emit(GT, ">")
			i++

		// Strings
		case ch == '"':
			j := i + 1
			var sb strings.Builder
			for j < len(runes) {
				if runes[j] == '\n' || runes[j] == '\r' {
					break
				}
				if runes[j] == '"' {
					if j+1 < len(runes) && runes[j+1] == '"' {
						sb.WriteRune('"')
						j += 2
						continue
					}
					break
				}
				sb.WriteRune(runes[j])
				j++
			}
			if j >= len(runes) || runes[j] != '"' {
				emitError("Unterminierter String")
				i = j
				continue
			}
			emit(STRING, sb.String())
			i = j + 1

		// Zahlen / Identifier mit führender Zahl (7z, 7zip, 123abc, 7_z)
		case isASCIIDigit(ch):
			j := i

			for j < len(runes) && isASCIIDigit(runes[j]) {
				j++
			}

			if j < len(runes) && (unicode.IsLetter(runes[j]) || runes[j] == '_') {
				for j < len(runes) &&
					(unicode.IsLetter(runes[j]) ||
						unicode.IsDigit(runes[j]) ||
						runes[j] == '_') {
					j++
				}

				emit(IDENT, string(runes[i:j]))
				i = j
				continue
			}

			// Normale Zahl, eventuell mit Dezimalpunkt
			if j < len(runes) && runes[j] == '.' {
				j++

				for j < len(runes) && isASCIIDigit(runes[j]) {
					j++
				}
			}

			emit(NUMBER, string(runes[i:j]))
			i = j

		// Identifier / Keywords
		case unicode.IsLetter(ch) || ch == '_':
			// 1. Zeilenfortsetzung?
			if ch == '_' {
				j := i + 1
				for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
					j++
				}
				if j < len(runes) && runes[j] == '\'' {
					for j < len(runes) && runes[j] != '\n' && runes[j] != '\r' {
						j++
					}
				}
				if j < len(runes) && (runes[j] == '\n' || runes[j] == '\r') {
					i = j
					if runes[i] == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
						i++
					}
					i++    // Newline-Zeichen überspringen
					line++ // Zeile mitzählen
					continue
				}
				// sonst: Identifier wie _temp, läuft in den normalen Scanner
			}

			// 2. Normaler Identifier oder Keyword
			j := i
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}

			word := string(runes[i:j])

			// Fast-Path: nur kleinschreiben, wenn überhaupt ein Großbuchstabe drin ist
			lw := word
			for _, r := range word {
				if unicode.IsUpper(r) {
					lw = strings.ToLower(word)
					break
				}
			}

			if tok, ok := Keywords[lw]; ok {
				emit(tok, word)
			} else {
				emit(IDENT, word)
			}

			i = j

		// unbekanntes Zeichen -> Fehler statt Endlosschleife
		default:
			emitError("Unerwartetes Zeichen '%c' (U+%04X)", ch, ch)
			i++
		}
	}

	emit(EOF, "")
	return tokens
}

// tokenizeInterpolatedString liest den Rest eines $"..."-Strings ab der Position
// direkt nach dem öffnenden " und emittiert ihn als normale &-Verkettung:
// ( "text" & (ausdruck) & "text" ).
func tokenizeInterpolatedString(runes []rune, i *int, line int, emit func(Type, string), emitError func(string, ...any)) {
	var segments []string     // Text-Segmente
	var exprSegments []string // {Ausdruck}-Segmente, parallel dazu
	var sb strings.Builder
	segType := make([]bool, 0) // false=Text, true=Ausdruck

	flushText := func() {
		segments = append(segments, sb.String())
		exprSegments = append(exprSegments, "")
		segType = append(segType, false)
		sb.Reset()
	}

	for *i < len(runes) {
		ch := runes[*i]

		if ch == '\n' || ch == '\r' {
			emitError("Unterminierter interpolierter String")
			return
		}

		if ch == '"' {
			if *i+1 < len(runes) && runes[*i+1] == '"' {
				sb.WriteRune('"')
				*i += 2
				continue
			}
			break // Ende des Strings
		}

		if ch == '{' {
			if *i+1 < len(runes) && runes[*i+1] == '{' {
				sb.WriteRune('{')
				*i += 2
				continue
			}
			flushText()
			*i++ // '{' konsumieren

			// Ausdruck bis zur passenden '}' einsammeln (Klammertiefe zählen)
			var exprSb strings.Builder
			depth := 0
			for *i < len(runes) {
				c := runes[*i]
				if c == '}' && depth == 0 {
					break
				}
				if c == '{' {
					depth++
				}
				if c == '}' {
					depth--
				}
				exprSb.WriteRune(c)
				*i++
			}
			if *i >= len(runes) || runes[*i] != '}' {
				emitError("Erwartet '}' nach interpoliertem Ausdruck")
				return
			}
			*i++ // '}' konsumieren

			segments = append(segments, "")
			exprSegments = append(exprSegments, exprSb.String())
			segType = append(segType, true)
			continue
		}

		if ch == '}' {
			if *i+1 < len(runes) && runes[*i+1] == '}' {
				sb.WriteRune('}')
				*i += 2
				continue
			}
			emitError("Unerwartete '}' im interpolierten String")
			return
		}

		sb.WriteRune(ch)
		*i++
	}

	if *i >= len(runes) || runes[*i] != '"' {
		emitError("Unterminierter interpolierter String")
		return
	}
	*i++ // schließendes '"' konsumieren
	flushText()

	emit(LPAREN, "(")
	first := true
	for idx, isExpr := range segType {
		if !first {
			emit(AMP, "&")
		}
		first = false
		if isExpr {
			emit(LPAREN, "(")
			subTokens := TokenizeFrom(exprSegments[idx], line)
			for _, t := range subTokens {
				if t.Type == EOF {
					continue
				}
				emit(t.Type, t.Value)
			}
			emit(RPAREN, ")")
		} else {
			emit(STRING, segments[idx])
		}
	}
	if first {
		emit(STRING, "") // leerer String $""
	}
	emit(RPAREN, ")")
}
