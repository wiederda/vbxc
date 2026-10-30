package evaluator

// Debug-Helfer, 1:1 aus stdlib_debug.go des Interpreters. Dort hingen sie
// am Hauptpackage, in vbxc gehören sie zum Evaluator: "Dump" ist keine
// stdlib-Funktion, sondern wird in evalFunctionCall abgefangen.

import (
	"fmt"
	"sort"
	"strings"
)

// dumpEnvironment zeigt den kompletten Scope-Stapel einer laufenden
// Ausführung an (Aufruf: Dump).
func dumpEnvironment(e *Environment) {
	current := e
	depth := 0

	fmt.Println("\n" + strings.Repeat("=", 45))
	fmt.Println("       ENVIRONMENT STACK DUMP")
	fmt.Println(strings.Repeat("=", 45))

	for current != nil {
		scopeName := "GLOBAL / PUBLIC"
		if current.parent != nil {
			scopeName = fmt.Sprintf("LOCAL SCOPE (Ebene %d)", depth)
		}

		fmt.Printf("\n[%s]\n", scopeName)

		if len(current.vars) == 0 {
			fmt.Println("  (leer)")
		} else {
			// 1. Namen in einen Slice sammeln
			var keys []string
			for name := range current.vars {
				// Interne Variablen (Start mit _) ignorieren
				if !strings.HasPrefix(name, "_") {
					keys = append(keys, name)
				}
			}

			// 2. Namen alphabetisch sortieren
			sort.Strings(keys)

			// 3. In der sortierten Reihenfolge ausgeben
			for _, name := range keys {
				ptr := current.vars[name]
				v := *ptr
				valStr := formatValue(v)
				fmt.Printf("  %-15s : %-10s = %s\n", name, v.Kind.String(), valStr)
			}
		}

		current = current.parent
		depth++
	}
	fmt.Println(strings.Repeat("=", 45) + "\n")
}

// formatValue liefert die farbige Vorschau eines Wertes für den Dump.
func formatValue(v Value) string {
	// ANSI Farbcodes
	const (
		reset  = "\033[0m"
		green  = "\033[32m"
		yellow = "\033[33m"
		blue   = "\033[34m"
		cyan   = "\033[36m"
		red    = "\033[31m"
		gray   = "\033[90m"
	)

	switch v.Kind {
	case KindNum:
		return fmt.Sprintf(yellow+"%g"+reset, v.Num)
	case KindStr:
		return fmt.Sprintf(green+"\"%s\""+reset, v.Str)
	case KindBool:
		return fmt.Sprintf(cyan+"%t"+reset, v.Bool)

	case KindArr: // 1D-Array Vorschau
		length := len(v.Arr)
		if length == 0 {
			return gray + "Array(leer)" + reset
		}

		limit := 3
		if length < limit {
			limit = length
		}

		var items []string
		for i := 0; i < limit; i++ {
			items = append(items, formatValue(v.Arr[i]))
		}

		preview := strings.Join(items, ", ")
		if length > limit {
			preview += gray + ", ..." + reset
		}
		return fmt.Sprintf(blue+"Array(len:%d)"+reset+" [%s]", length, preview)

	case KindArr2D: // 2D-Array (Matrix) Vorschau
		rows := len(v.Arr2D)
		if rows == 0 {
			return gray + "Matrix(leer)" + reset
		}
		cols := len(v.Arr2D[0])

		var firstRow []string
		colLimit := 3
		if cols < colLimit {
			colLimit = cols
		}

		for j := 0; j < colLimit; j++ {
			firstRow = append(firstRow, formatValue(v.Arr2D[0][j]))
		}

		rowPreview := strings.Join(firstRow, gray+" | "+reset)
		if cols > colLimit {
			rowPreview += gray + " | ..." + reset
		}

		return fmt.Sprintf(blue+"Matrix(%dx%d)"+reset+" [Row 0: %s]", rows, cols, rowPreview)

	default:
		return red + "nil" + reset
	}
}
