// ------------------------
// stdlib_array.go
// ------------------------

package stdlib

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	valpkg "vbxc/value"

	"github.com/xuri/excelize/v2"
)

// InitArrayFunctions registriert array-Funktionen
func initExportFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "export."

	Register(ns+"XLSXSheets", "export", "path", "Gibt die Namen aller Tabellenblätter einer XLSX-Datei zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		f, err := excelize.OpenFile(args[0].Str)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		defer f.Close()

		names := f.GetSheetList()
		result := make([]valpkg.Value, len(names))
		for i, n := range names {
			result[i] = valpkg.StrVal(n)
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	Register(ns+"FromXLSX", "export", "path, [sheetName, exclude, column]", "Lädt eine XLSX-Datei in ein Array. Bei einer Spalte wird ein 1D-Array, bei mehreren Spalten ein 2D-Array zurückgegeben. Mit column kann gezielt eine Spalte eingelesen werden. Leere Zeilen werden ignoriert.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr:  []valpkg.Value{},
			}
		}

		path := args[0].Str

		// ------------------------------------------------------------
		// XLSX öffnen
		// ------------------------------------------------------------

		f, err := excelize.OpenFile(path)
		if err != nil {
			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr:  []valpkg.Value{},
			}
		}
		defer f.Close()

		// ------------------------------------------------------------
		// Tabellenblatt
		// ------------------------------------------------------------

		sheetName := ""

		if len(args) >= 2 &&
			args[1].Kind == valpkg.KindStr &&
			args[1].Str != "" {

			sheetName = args[1].Str

		} else {

			sheetName = f.GetSheetName(0)
		}

		// ------------------------------------------------------------
		// Ausschlusswerte
		// ------------------------------------------------------------

		excludeSet := make(map[string]struct{})

		if len(args) >= 3 && args[2].Kind == valpkg.KindArr {
			for _, ex := range args[2].Arr {
				excludeSet[valpkg.ToString(ex)] = struct{}{}
			}
		}

		rowExcluded := func(row []string) bool {
			if len(excludeSet) == 0 {
				return false
			}

			for _, cell := range row {
				value := strings.TrimSpace(cell)

				if _, exists := excludeSet[value]; exists {
					return true
				}
			}

			return false
		}

		// ------------------------------------------------------------
		// Leere Zeile
		//
		// Eine Zeile gilt als leer, wenn alle vorhandenen
		// Zellen nach TrimSpace leer sind.
		// ------------------------------------------------------------

		rowEmpty := func(row []string) bool {
			for _, cell := range row {
				if strings.TrimSpace(cell) != "" {
					return false
				}
			}

			return true
		}

		// ------------------------------------------------------------
		// Spalte
		//
		// -1 = alle Spalten
		//  0 = A
		//  1 = B
		//  2 = C
		// ------------------------------------------------------------

		column := -1

		if len(args) >= 4 && args[3].Kind == valpkg.KindNum {
			column = int(args[3].Num)

			if column < 0 {
				column = -1
			}
		}

		// ------------------------------------------------------------
		// XLSX lesen
		// ------------------------------------------------------------

		rows, err := f.GetRows(sheetName)
		if err != nil {
			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr:  []valpkg.Value{},
			}
		}

		// ============================================================
		// GEZIELTE SPALTE
		// ============================================================

		if column >= 0 {

			result := make([]valpkg.Value, 0, len(rows))

			for _, row := range rows {

				// Ausschluss und komplett leere Zeilen ignorieren
				if rowExcluded(row) || rowEmpty(row) {
					continue
				}

				// Spalte existiert in dieser Zeile nicht
				if column >= len(row) {
					result = append(
						result,
						valpkg.StrVal(""),
					)

				} else {

					result = append(
						result,
						valpkg.StrVal(strings.TrimSpace(row[column])),
					)
				}
			}

			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr:  result,
			}
		}

		// ============================================================
		// ALLE SPALTEN
		// ============================================================

		var filteredRows [][]string

		maxColumns := 0

		for _, row := range rows {

			// Ausschluss und komplett leere Zeilen ignorieren
			if rowExcluded(row) || rowEmpty(row) {
				continue
			}

			filteredRows = append(filteredRows, row)

			if len(row) > maxColumns {
				maxColumns = len(row)
			}
		}

		// ------------------------------------------------------------
		// Keine Daten
		// ------------------------------------------------------------

		if len(filteredRows) == 0 {
			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr:  []valpkg.Value{},
			}
		}

		// ============================================================
		// EINE SPALTE -> 1D-ARRAY
		// ============================================================

		if maxColumns == 1 {

			result := make([]valpkg.Value, 0, len(filteredRows))

			for _, row := range filteredRows {

				if len(row) == 0 {
					result = append(
						result,
						valpkg.StrVal(""),
					)
				} else {
					result = append(
						result,
						valpkg.StrVal(strings.TrimSpace(row[0])),
					)
				}
			}

			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr:  result,
			}
		}

		// ============================================================
		// MEHRERE SPALTEN -> 2D-ARRAY
		// ============================================================

		res2D := make([][]valpkg.Value, 0, len(filteredRows))

		for _, row := range filteredRows {

			resRow := make([]valpkg.Value, len(row))

			for j, cell := range row {
				resRow[j] = valpkg.StrVal(
					strings.TrimSpace(cell),
				)
			}

			res2D = append(res2D, resRow)
		}

		return valpkg.Value{
			Kind:  valpkg.KindArr2D,
			Arr2D: res2D,
		}
	})

	Register(ns+"ToXLSX", "export", "path, data, [sheetName, exclude, append, headers]", "Speichert ein Array oder 2D-Array als XLSX-Datei. Vorhandene Blätter werden standardmäßig ersetzt. Mit append=True werden Daten angehängt. Mit headers können optionale Spaltenüberschriften angegeben werden.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  "error: filename and data required",
			}
		}

		filename := args[0].Str
		data := args[1]

		// ------------------------------------------------------------
		// Parameter
		// ------------------------------------------------------------

		sheetName := "Sheet1"

		if len(args) >= 3 &&
			args[2].Kind == valpkg.KindStr &&
			args[2].Str != "" {
			sheetName = args[2].Str
		}

		// ------------------------------------------------------------
		// Ausschlusswerte
		// ------------------------------------------------------------

		excludeSet := make(map[string]struct{})

		if len(args) >= 4 && args[3].Kind == valpkg.KindArr {
			for _, ex := range args[3].Arr {
				excludeSet[valpkg.ToString(ex)] = struct{}{}
			}
		}

		// ------------------------------------------------------------
		// Append
		// ------------------------------------------------------------

		appendMode := false

		if len(args) >= 5 && args[4].Kind == valpkg.KindBool {
			appendMode = args[4].Bool
		}

		// ------------------------------------------------------------
		// Überschriften
		// ------------------------------------------------------------

		var headers []valpkg.Value

		if len(args) >= 6 && args[5].Kind == valpkg.KindArr {
			headers = args[5].Arr
		}

		// ------------------------------------------------------------
		// Daten prüfen
		// ------------------------------------------------------------

		if data.Kind != valpkg.KindArr && data.Kind != valpkg.KindArr2D {
			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  "error: data must be Array or Array2D",
			}
		}

		// ------------------------------------------------------------
		// XLSX öffnen oder neu erstellen
		// ------------------------------------------------------------

		var f *excelize.File
		var err error

		if _, statErr := os.Stat(filename); statErr == nil {
			f, err = excelize.OpenFile(filename)
			if err != nil {
				return valpkg.Value{
					Kind: valpkg.KindStr,
					Str:  "error: " + err.Error(),
				}
			}
		} else {
			f = excelize.NewFile()
		}

		defer f.Close()

		// ------------------------------------------------------------
		// Blatt suchen
		// ------------------------------------------------------------

		sheetIndex, err := f.GetSheetIndex(sheetName)
		if err != nil {
			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  "error: " + err.Error(),
			}
		}

		// ------------------------------------------------------------
		// Blatt existiert
		// ------------------------------------------------------------

		if sheetIndex >= 0 {

			if !appendMode {

				// Vorhandenes Blatt löschen
				if err := f.DeleteSheet(sheetName); err != nil {
					return valpkg.Value{
						Kind: valpkg.KindStr,
						Str:  "error: " + err.Error(),
					}
				}

				// Blatt neu anlegen
				sheetIndex, err = f.NewSheet(sheetName)
				if err != nil {
					return valpkg.Value{
						Kind: valpkg.KindStr,
						Str:  "error: " + err.Error(),
					}
				}
			}

		} else {

			// --------------------------------------------------------
			// Blatt existiert noch nicht
			// --------------------------------------------------------

			sheetIndex, err = f.NewSheet(sheetName)
			if err != nil {
				return valpkg.Value{
					Kind: valpkg.KindStr,
					Str:  "error: " + err.Error(),
				}
			}
		}

		_ = sheetIndex

		// ------------------------------------------------------------
		// Vorhandene Zeilen ermitteln
		// ------------------------------------------------------------

		rows, err := f.GetRows(sheetName)
		if err != nil {
			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  "error: " + err.Error(),
			}
		}

		// ------------------------------------------------------------
		// Startzeile
		// ------------------------------------------------------------

		rowIdx := 1

		if appendMode && len(rows) > 0 {
			rowIdx = len(rows) + 1
		}

		// ------------------------------------------------------------
		// Zeile schreiben
		// ------------------------------------------------------------

		writeRow := func(rowIdx int, row []valpkg.Value) error {
			for colIdx, cell := range row {

				cellRef, err := excelize.CoordinatesToCellName(
					colIdx+1,
					rowIdx,
				)

				if err != nil {
					return err
				}

				if err := f.SetCellValue(
					sheetName,
					cellRef,
					valToInterface(cell),
				); err != nil {
					return err
				}
			}

			return nil
		}

		// ------------------------------------------------------------
		// Überschriften schreiben
		// ------------------------------------------------------------

		if len(headers) > 0 {

			// Bei Append nur schreiben, wenn das Blatt leer ist.
			if !appendMode || len(rows) == 0 {

				headerStyle, err := f.NewStyle(&excelize.Style{
					Font: &excelize.Font{
						Bold: true,
					},
				})

				if err != nil {
					return valpkg.Value{
						Kind: valpkg.KindStr,
						Str:  "error: " + err.Error(),
					}
				}

				for colIdx, cell := range headers {

					cellRef, err := excelize.CoordinatesToCellName(
						colIdx+1,
						rowIdx,
					)

					if err != nil {
						return valpkg.Value{
							Kind: valpkg.KindStr,
							Str:  "error: " + err.Error(),
						}
					}

					if err := f.SetCellValue(
						sheetName,
						cellRef,
						valToInterface(cell),
					); err != nil {
						return valpkg.Value{
							Kind: valpkg.KindStr,
							Str:  "error: " + err.Error(),
						}
					}

					if err := f.SetCellStyle(
						sheetName,
						cellRef,
						cellRef,
						headerStyle,
					); err != nil {
						return valpkg.Value{
							Kind: valpkg.KindStr,
							Str:  "error: " + err.Error(),
						}
					}
				}

				rowIdx++
			}
		}

		// ------------------------------------------------------------
		// Zeile ausschließen
		// ------------------------------------------------------------

		rowExcluded := func(row []valpkg.Value) bool {
			if len(excludeSet) == 0 {
				return false
			}

			for _, cell := range row {
				if _, exists := excludeSet[valpkg.ToString(cell)]; exists {
					return true
				}
			}

			return false
		}

		// ------------------------------------------------------------
		// Daten schreiben
		// ------------------------------------------------------------

		if data.Kind == valpkg.KindArr2D {

			for _, row := range data.Arr2D {

				if rowExcluded(row) {
					continue
				}

				if err := writeRow(rowIdx, row); err != nil {
					return valpkg.Value{
						Kind: valpkg.KindStr,
						Str:  "error: " + err.Error(),
					}
				}

				rowIdx++
			}

		} else {

			for _, rowVal := range data.Arr {

				var row []valpkg.Value

				if rowVal.Kind == valpkg.KindArr {
					row = rowVal.Arr
				} else {
					row = []valpkg.Value{rowVal}
				}

				if rowExcluded(row) {
					continue
				}

				if err := writeRow(rowIdx, row); err != nil {
					return valpkg.Value{
						Kind: valpkg.KindStr,
						Str:  "error: " + err.Error(),
					}
				}

				rowIdx++
			}
		}

		// ------------------------------------------------------------
		// Leeres Sheet1 von excelize.NewFile() entfernen
		// ------------------------------------------------------------

		if sheetName != "Sheet1" {
			if idx, err := f.GetSheetIndex("Sheet1"); err == nil && idx >= 0 {
				_ = f.DeleteSheet("Sheet1")
			}
		}

		// ------------------------------------------------------------
		// Aktives Blatt setzen
		// ------------------------------------------------------------

		if idx, err := f.GetSheetIndex(sheetName); err == nil && idx >= 0 {
			f.SetActiveSheet(idx)
		}

		// ------------------------------------------------------------
		// Datei speichern
		// ------------------------------------------------------------

		if err := f.SaveAs(filename); err != nil {
			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  "error: " + err.Error(),
			}
		}

		return valpkg.Value{
			Kind: valpkg.KindStr,
			Str:  "ok",
		}
	})

	Register(ns+"FromCSV", "export", "path, [sep, exclude]", "Lädt eine CSV-Datei in ein 2D-Array. Leere Zeilen werden ignoriert. (Default-Separator: ;)", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{
				Kind:  valpkg.KindArr2D,
				Arr2D: [][]valpkg.Value{},
			}
		}

		path := args[0].Str

		// ------------------------------------------------------------
		// Separator
		// ------------------------------------------------------------

		separator := ';'

		if len(args) >= 2 &&
			args[1].Kind == valpkg.KindStr &&
			args[1].Str != "" {

			separator = rune(args[1].Str[0])
		}

		// ------------------------------------------------------------
		// Ausschlusswerte
		// ------------------------------------------------------------

		excludeSet := make(map[string]struct{})

		if len(args) >= 3 && args[2].Kind == valpkg.KindArr {

			for _, ex := range args[2].Arr {
				excludeSet[valpkg.ToString(ex)] = struct{}{}
			}
		}

		rowExcluded := func(row []string) bool {

			if len(excludeSet) == 0 {
				return false
			}

			for _, cell := range row {

				value := strings.TrimSpace(cell)

				if _, exists := excludeSet[value]; exists {
					return true
				}
			}

			return false
		}

		// ------------------------------------------------------------
		// Leere Zeile
		//
		// Eine Zeile gilt als leer, wenn alle Zellen nach
		// TrimSpace keinen Inhalt enthalten.
		// ------------------------------------------------------------

		rowEmpty := func(row []string) bool {

			for _, cell := range row {

				if strings.TrimSpace(cell) != "" {
					return false
				}
			}

			return true
		}

		// ------------------------------------------------------------
		// Datei komplett roh einlesen
		// ------------------------------------------------------------

		rawContent, err := os.ReadFile(path)
		if err != nil {
			return valpkg.Value{
				Kind:  valpkg.KindArr2D,
				Arr2D: [][]valpkg.Value{},
			}
		}

		rawContent = ensureUTF8(rawContent)

		// ------------------------------------------------------------
		// Sanitizing
		// ------------------------------------------------------------

		cleanContent := make([]byte, 0, len(rawContent))

		for i := 0; i < len(rawContent); i++ {

			b := rawContent[i]

			// Erlaube:
			// Druckbare ASCII/UTF8
			// Tab, LF, CR
			//
			// Entferne:
			// Steuerzeichen unter 32
			if b >= 32 ||
				b == '\n' ||
				b == '\r' ||
				b == '\t' {

				cleanContent = append(cleanContent, b)
			}
		}

		// ------------------------------------------------------------
		// CSV lesen
		// ------------------------------------------------------------

		reader := csv.NewReader(bytes.NewReader(cleanContent))

		reader.Comma = separator
		reader.LazyQuotes = true
		reader.FieldsPerRecord = -1

		records, err := reader.ReadAll()

		if err != nil {

			fmt.Println(
				"CSV-Parser-Error nach Reinigung:",
				err,
			)

			return valpkg.Value{
				Kind:  valpkg.KindArr2D,
				Arr2D: [][]valpkg.Value{},
			}
		}

		// ------------------------------------------------------------
		// In 2D-valpkg.Value-Array umwandeln
		// ------------------------------------------------------------

		res2D := make([][]valpkg.Value, 0, len(records))

		for _, row := range records {

			// Ausschluss prüfen
			if rowExcluded(row) {
				continue
			}

			// Leere Zeilen ignorieren
			if rowEmpty(row) {
				continue
			}

			resRow := make([]valpkg.Value, len(row))

			for j, cell := range row {

				resRow[j] = valpkg.Value{
					Kind: valpkg.KindStr,
					Str:  strings.TrimSpace(cell),
				}
			}

			res2D = append(res2D, resRow)
		}

		return valpkg.Value{
			Kind:  valpkg.KindArr2D,
			Arr2D: res2D,
		}
	})

	Register(ns+"ToCSV", "export", "path, data, [sep, exclude, append]", "Speichert ein Array oder 2D-Array als CSV-Datei. Mit exclude können Zeilen ausgeschlossen und mit append Daten angehängt werden.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  "error: filename and data required",
			}
		}

		filename := args[0].Str
		data := args[1]

		// ------------------------------------------------------------
		// Separator
		// ------------------------------------------------------------

		separator := ';'

		if len(args) >= 3 && args[2].Kind == valpkg.KindStr && args[2].Str != "" {
			separator = rune(args[2].Str[0])
		}

		// ------------------------------------------------------------
		// Ausschlusswerte
		// ------------------------------------------------------------

		excludeSet := make(map[string]struct{})

		if len(args) >= 4 && args[3].Kind == valpkg.KindArr {
			for _, ex := range args[3].Arr {
				excludeSet[valpkg.ToString(ex)] = struct{}{}
			}
		}

		rowExcluded := func(row []valpkg.Value) bool {
			if len(excludeSet) == 0 {
				return false
			}

			for _, cell := range row {
				if _, exists := excludeSet[valpkg.ToString(cell)]; exists {
					return true
				}
			}

			return false
		}

		// ------------------------------------------------------------
		// Daten prüfen
		// ------------------------------------------------------------

		if data.Kind != valpkg.KindArr && data.Kind != valpkg.KindArr2D {
			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  "error: data must be Array or Array2D",
			}
		}

		// ------------------------------------------------------------
		// Datei öffnen
		// ------------------------------------------------------------

		var f *os.File
		var err error

		if len(args) >= 5 && args[4].Kind == valpkg.KindBool && args[4].Bool {
			// Append
			f, err = os.OpenFile(
				filename,
				os.O_CREATE|os.O_WRONLY|os.O_APPEND,
				0644,
			)
		} else {
			// Datei neu erstellen / überschreiben
			f, err = os.Create(filename)
		}

		if err != nil {
			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  "error: " + err.Error(),
			}
		}

		defer f.Close()

		// ------------------------------------------------------------
		// CSV Writer
		// ------------------------------------------------------------

		writer := csv.NewWriter(f)
		writer.Comma = separator

		// ------------------------------------------------------------
		// Eine Zeile schreiben
		// ------------------------------------------------------------

		writeRow := func(row []valpkg.Value) error {
			if rowExcluded(row) {
				return nil
			}

			record := make([]string, len(row))

			for i, cell := range row {
				val := valToInterface(cell)

				if val == nil {
					record[i] = ""
				} else {
					record[i] = fmt.Sprintf("%v", val)
				}
			}

			return writer.Write(record)
		}

		// ------------------------------------------------------------
		// Daten schreiben
		// ------------------------------------------------------------

		if data.Kind == valpkg.KindArr2D {

			for _, row := range data.Arr2D {
				if err := writeRow(row); err != nil {
					return valpkg.Value{
						Kind: valpkg.KindStr,
						Str:  "error: " + err.Error(),
					}
				}
			}

		} else {

			for _, rowVal := range data.Arr {

				if rowVal.Kind == valpkg.KindArr {

					if err := writeRow(rowVal.Arr); err != nil {
						return valpkg.Value{
							Kind: valpkg.KindStr,
							Str:  "error: " + err.Error(),
						}
					}

				} else {

					// Flaches Array:
					// jedes Element wird zu einer eigenen Zeile
					// mit genau einer Spalte.

					if err := writeRow([]valpkg.Value{rowVal}); err != nil {
						return valpkg.Value{
							Kind: valpkg.KindStr,
							Str:  "error: " + err.Error(),
						}
					}
				}
			}
		}

		// ------------------------------------------------------------
		// Flush prüfen
		// ------------------------------------------------------------

		writer.Flush()

		if err := writer.Error(); err != nil {
			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  "error: " + err.Error(),
			}
		}

		return valpkg.Value{
			Kind: valpkg.KindStr,
			Str:  "ok",
		}
	})
}

func init() { initExportFunctions() }
