package stdlib

import (
	"strings"
	"unicode"
)

func charClass(r rune) int {
	if unicode.IsDigit(r) {
		return 1
	}
	if unicode.IsLetter(r) {
		return 2
	}
	return 0 // Symbole, Satzzeichen, Unterstrich, Leerzeichen, etc.
}

func normalizeForSort(r rune) (rune, int) {
	switch unicode.ToLower(r) {
	case 'ä':
		return 'a', 1
	case 'ö':
		return 'o', 1
	case 'ü':
		return 'u', 1
	default:
		return unicode.ToLower(r), 0
	}
}

func naturalLess(a, b string) bool {
	ra := []rune(a)
	rb := []rune(b)

	i, j := 0, 0

	for i < len(ra) && j < len(rb) {

		// Zahlen als Zahlen vergleichen
		if unicode.IsDigit(ra[i]) && unicode.IsDigit(rb[j]) {

			startI := i
			startJ := j

			for i < len(ra) && unicode.IsDigit(ra[i]) {
				i++
			}

			for j < len(rb) && unicode.IsDigit(rb[j]) {
				j++
			}

			numA := string(ra[startI:i])
			numB := string(rb[startJ:j])

			numATrim := strings.TrimLeft(numA, "0")
			numBTrim := strings.TrimLeft(numB, "0")

			if numATrim == "" {
				numATrim = "0"
			}

			if numBTrim == "" {
				numBTrim = "0"
			}

			if len(numATrim) != len(numBTrim) {
				return len(numATrim) < len(numBTrim)
			}

			if numATrim != numBTrim {
				return numATrim < numBTrim
			}

			if len(numA) != len(numB) {
				return len(numA) < len(numB)
			}

			continue
		}

		// Zeichenklasse NICHT verändern.
		// Damit bleibt:
		// Sonderzeichen -> Zahlen -> Buchstaben
		classA := charClass(ra[i])
		classB := charClass(rb[j])

		if classA != classB {
			return classA < classB
		}

		// Buchstaben normalisieren.
		ca, umlautA := normalizeForSort(ra[i])
		cb, umlautB := normalizeForSort(rb[j])

		if ca != cb {
			return ca < cb
		}

		// Gleicher Grundbuchstabe:
		// normaler Buchstabe vor Umlaut.
		//
		// a < ä
		// o < ö
		// u < ü
		if umlautA != umlautB {
			return umlautA < umlautB
		}

		i++
		j++
	}

	return len(ra) < len(rb)
}
