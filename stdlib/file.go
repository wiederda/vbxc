// ------------------------
// stdlib_file.go
// ------------------------

package stdlib

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/wiederda/pathutils"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
	valpkg "vbxc/value"
)

//var replaceAfterRunPath string

// expectStr liefert args[i].Str als geprüften String-Parameter zurück,
// oder einen *valpkg.Value-Fehler, falls das Argument fehlt oder kein String ist.
// fnSig ist die Funktionssignatur für die Fehlermeldung, z.B. "file.Delete(path)".
func expectStr(args []valpkg.Value, i int, fnSig string) (string, *valpkg.Value) {
	if len(args) <= i {
		v := valpkg.ErrorVal(fmt.Sprintf("%s: Argument %d fehlt", fnSig, i+1))
		return "", &v
	}
	if args[i].Kind != valpkg.KindStr {
		v := valpkg.ErrorVal(fmt.Sprintf("%s: erwarte einen String für Argument %d, habe %s bekommen",
			fnSig, i+1, valpkg.GetKindName(args[i].Kind)))
		return "", &v
	}
	return args[i].Str, nil
}

// InitFileFunctions registriert File-Funktionen inkl. erweiterter Features
func initFileFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "file."

	// ------------------------
	// Write
	// ------------------------
	Register(ns+"WriteAllText", "file", "path, content", "Schreibt den gesamten Text in eine Datei (überschreibt Vorhandenes).", func(args []valpkg.Value) valpkg.Value {
		// 1. Parameter-Check: Wir brauchen Pfad und Inhalt
		if len(args) < 2 {
			return valpkg.ErrorVal("WriteAllText erwartet zwei Parameter: Pfad und Inhalt")
		}

		pathStr, errS := expectStr(args, 0, "file.WriteAllText(path, content)")
		if errS != nil {
			return *errS
		}

		// 2. Deine Sicherheitsfunktion nutzen!
		// Sie liefert den absoluten Pfad und einen Zeiger auf einen Error-valpkg.Value
		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal // Fehler direkt an VB zurückgeben (z.B. Sicherheitsfehler)
		}

		// 3. Datei schreiben
		// Wir nutzen den von absPathVal gelieferten 'path'
		err := os.WriteFile(path, []byte(args[1].Str), 0644)
		if err != nil {
			return valpkg.ErrorVal("Schreibfehler in WriteAllText: " + err.Error())
		}

		return valpkg.NullVal()
	})

	Register(ns+"ProductVersion", "file", "path", "Gibt die Produktversion einer exe/dll zurück (Windows).", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.ProductVersion(path)")
		if errS != nil {
			return *errS
		}
		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}
		v, err := productVersion(path)
		if err != nil {
			return valpkg.ErrorVal("file.ProductVersion: " + err.Error())
		}
		return valpkg.StrVal(v)
	})

	Register(ns+"FindWords", "file", "words, inputFile, outputFile", "Sucht beliebige Wörter in einer Datei und schreibt alle Trefferzeilen mit Zeilennummer in eine Ausgabedatei.", func(args []valpkg.Value) valpkg.Value {
		// 1. Parameter prüfen
		if len(args) < 3 {
			return valpkg.ErrorVal("FindWords erwartet drei Parameter: Suchwörter, Eingabedatei und Ausgabedatei")
		}

		// 2. Suchwörter prüfen
		if args[0].Kind != valpkg.KindArr {
			return valpkg.ErrorVal("FindWords: Der erste Parameter muss ein Array mit Suchwörtern sein")
		}

		var words []string

		for i, item := range args[0].Arr {
			if item.Kind != valpkg.KindStr {
				return valpkg.ErrorVal(fmt.Sprintf(
					"FindWords: Suchwort an Position %d muss ein String sein",
					i,
				))
			}

			if item.Str != "" {
				words = append(words, item.Str)
			}
		}

		if len(words) == 0 {
			return valpkg.ErrorVal("FindWords: Das Suchwort-Array darf nicht leer sein")
		}

		// 3. Eingabedatei
		inputStr, errS := expectStr(args, 1, "file.FindWords(words, inputFile, outputFile)")
		if errS != nil {
			return *errS
		}

		inputPath, errVal := absPathVal(inputStr)
		if errVal != nil {
			return *errVal
		}

		// 4. Ausgabedatei
		outputStr, errS := expectStr(args, 2, "file.FindWords(words, inputFile, outputFile)")
		if errS != nil {
			return *errS
		}

		outputPath, errVal := absPathVal(outputStr)
		if errVal != nil {
			return *errVal
		}

		// 5. Eingabedatei öffnen
		input, err := os.Open(inputPath)
		if err != nil {
			return valpkg.ErrorVal("Lesefehler in FindWords: " + err.Error())
		}
		defer input.Close()

		// 6. Ausgabedatei erzeugen/überschreiben
		output, err := os.Create(outputPath)
		if err != nil {
			return valpkg.ErrorVal("Schreibfehler in FindWords: " + err.Error())
		}
		defer output.Close()

		// 7. Datei zeilenweise durchsuchen
		scanner := bufio.NewScanner(input)

		// Größere Zeilen erlauben als der Scanner-Standard von 64 KB
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

		lineNumber := 0
		hitCount := 0

		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()

			for _, word := range words {
				if strings.Contains(line, word) {
					fmt.Fprintf(output, "%d: %s\n", lineNumber, line)
					hitCount++
					break
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return valpkg.ErrorVal("Lesefehler in FindWords: " + err.Error())
		}

		return valpkg.NumVal(float64(hitCount))
	})

	Register(ns+"FindLine", "file", "line, inputFile", "Prüft, ob eine Zeile exakt (ohne führende/nachfolgende Leerzeichen) in einer Datei vorkommt. Gibt true beim ersten Treffer zurück, sonst false.", func(args []valpkg.Value) valpkg.Value {
		// 1. Parameter prüfen
		if len(args) < 2 {
			return valpkg.ErrorVal("FindLine erwartet zwei Parameter: Zeile und Eingabedatei")
		}

		// 2. Suchzeile prüfen
		lineStr, errS := expectStr(args, 0, "file.FindLine(line, inputFile)")
		if errS != nil {
			return *errS
		}
		lineStr = strings.TrimSpace(lineStr)
		if lineStr == "" {
			return valpkg.ErrorVal("FindLine: Die Suchzeile darf nicht leer sein")
		}

		// 3. Eingabedatei
		inputStr, errS := expectStr(args, 1, "file.FindLine(line, inputFile)")
		if errS != nil {
			return *errS
		}

		inputPath, errVal := absPathVal(inputStr)
		if errVal != nil {
			return *errVal
		}

		// 4. Eingabedatei öffnen
		input, err := os.Open(inputPath)
		if err != nil {
			return valpkg.ErrorVal("Lesefehler in FindLine: " + err.Error())
		}
		defer input.Close()

		// 5. Zeilenweise vergleichen, beim ersten Treffer abbrechen
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == lineStr {
				return valpkg.BoolVal(true)
			}
		}

		if err := scanner.Err(); err != nil {
			return valpkg.ErrorVal("Lesefehler in FindLine: " + err.Error())
		}

		return valpkg.BoolVal(false)
	})

	Register(ns+"FindWord", "file", "word, inputFile", "Prüft, ob ein Wort in einer Datei vorkommt. Gibt true beim ersten Treffer zurück, sonst false.", func(args []valpkg.Value) valpkg.Value {
		// 1. Parameter prüfen
		if len(args) < 2 {
			return valpkg.ErrorVal("FindWord erwartet zwei Parameter: Suchwort und Eingabedatei")
		}

		// 2. Suchwort prüfen
		wordStr, errS := expectStr(args, 0, "file.FindWord(word, inputFile)")
		if errS != nil {
			return *errS
		}
		if wordStr == "" {
			return valpkg.ErrorVal("FindWord: Das Suchwort darf nicht leer sein")
		}

		// 3. Eingabedatei
		inputStr, errS := expectStr(args, 1, "file.FindWord(word, inputFile)")
		if errS != nil {
			return *errS
		}

		inputPath, errVal := absPathVal(inputStr)
		if errVal != nil {
			return *errVal
		}

		// 4. Eingabedatei öffnen
		input, err := os.Open(inputPath)
		if err != nil {
			return valpkg.ErrorVal("Lesefehler in FindWord: " + err.Error())
		}
		defer input.Close()

		// 5. Zeilenweise suchen, beim ersten Treffer abbrechen
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

		for scanner.Scan() {
			if containsWholeWord(scanner.Text(), wordStr) {
				return valpkg.BoolVal(true)
			}
		}

		if err := scanner.Err(); err != nil {
			return valpkg.ErrorVal("Lesefehler in FindWord: " + err.Error())
		}

		return valpkg.BoolVal(false)
	})

	// ------------------------
	// Create
	// ------------------------
	Register(ns+"Create", "file", "path", "Erstellt eine leere Datei, falls diese noch nicht existiert.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.Create(path)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		// Verzeichnis muss existieren
		if _, err := os.Stat(filepath.Dir(path)); err != nil {
			return valpkg.ErrorVal("Verzeichnis existiert nicht: " + filepath.Dir(path))
		}

		// Nur erstellen wenn sie noch nicht existiert
		if _, err := os.Stat(path); os.IsNotExist(err) {
			f, err := os.Create(path)
			if err != nil {
				return valpkg.ErrorVal("Datei konnte nicht erstellt werden: " + err.Error())
			}
			f.Close()
		}

		return valpkg.NullVal()
	})

	// ------------------------
	// StreamWrite
	// ------------------------
	Register(ns+"StreamWrite", "file", "path, content", "Hängt Text gepuffert an eine Datei an (erstellt diese, falls nicht vorhanden).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.StreamWrite(path, content) benötigt 2 Argumente")
		}

		pathStr, errS := expectStr(args, 0, "file.StreamWrite(path, content)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		// Verzeichnis muss existieren
		if _, err := os.Stat(filepath.Dir(path)); err != nil {
			return valpkg.ErrorVal("Verzeichnis existiert nicht: " + filepath.Dir(path))
		}

		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return valpkg.ErrorVal("Stream konnte nicht geöffnet werden: " + err.Error())
		}
		defer f.Close()

		writer := bufio.NewWriter(f)
		if _, err := writer.WriteString(args[1].Str); err != nil {
			return valpkg.ErrorVal("Fehler beim Schreiben in den Stream: " + err.Error())
		}

		if err := writer.Flush(); err != nil {
			return valpkg.ErrorVal("Fehler beim Flushen des Buffers: " + err.Error())
		}

		return valpkg.NullVal()
	})

	// ---------------- Exists ----------------
	Register(ns+"Exists", "file", "path",
		"Prüft, ob eine Datei existiert.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 1 {
				return valpkg.BoolVal(false)
			}

			if args[0].Kind != valpkg.KindStr {
				return valpkg.ErrorVal(fmt.Sprintf(
					"file.Exists: erwarte einen Dateipfad (String), habe %s bekommen",
					valpkg.GetKindName(args[0].Kind)))
			}

			if args[0].Str == "" {
				return valpkg.BoolVal(false)
			}

			path, errVal := absPathVal(args[0].Str)
			if errVal != nil {
				return valpkg.BoolVal(false)
			}

			info, err := os.Stat(path)
			if err != nil {
				return valpkg.BoolVal(false)
			}

			return valpkg.BoolVal(!info.IsDir())
		})

	// --- Namespace: file. ---
	Register(ns+"CreateSymlink", "file", "target, linkPath, [replaceExisting]", "Erstellt einen Symlink auf eine DATEI.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.CreateSymlink(target, linkPath, [replaceExisting]) benötigt mindestens 2 Pfade")
		}

		target, errS1 := expectStr(args, 0, "file.CreateSymlink(target, linkPath)")
		if errS1 != nil {
			return *errS1
		}
		linkPath, errS2 := expectStr(args, 1, "file.CreateSymlink(target, linkPath)")
		if errS2 != nil {
			return *errS2
		}

		replaceExisting := false
		if len(args) >= 3 {
			replaceExisting = args[2].Bool
		}

		return createSymlinkInternal(target, linkPath, replaceExisting)
	})

	// ---------------- Base64Encode ----------------
	Register(ns+"Base64Encode", "file", "inFile, outFile", "Liest eine Datei ein und speichert sie Base64-kodiert ab.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.Base64Encode(in, out)")
		}

		inFileStr, errS1 := expectStr(args, 0, "file.Base64Encode(inFile, outFile)")
		if errS1 != nil {
			return *errS1
		}
		outFileStr, errS2 := expectStr(args, 1, "file.Base64Encode(inFile, outFile)")
		if errS2 != nil {
			return *errS2
		}

		inFile, e1 := absPathVal(inFileStr)
		outFile, e2 := absPathVal(outFileStr)
		if e1 != nil {
			return *e1
		}
		if e2 != nil {
			return *e2
		}

		// Zielverzeichnis muss existieren
		if _, err := os.Stat(filepath.Dir(outFile)); err != nil {
			return valpkg.ErrorVal("Zielverzeichnis existiert nicht: " + filepath.Dir(outFile))
		}

		in, err := os.Open(inFile)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		defer in.Close()
		out, err := os.Create(outFile)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		defer out.Close()

		enc := base64.NewEncoder(base64.StdEncoding, out)
		io.Copy(enc, in)
		enc.Close()
		return valpkg.NullVal()
	})

	// ---------------- Base64Decode ----------------
	Register(ns+"Base64Decode", "file", "inFile, outFile", "Dekodiert eine Base64-Datei und speichert das Ergebnis als Binärdatei.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.Base64Decode(infile, outfile) benötigt 2 Argumente")
		}

		inFileStr, errS1 := expectStr(args, 0, "file.Base64Decode(inFile, outFile)")
		if errS1 != nil {
			return *errS1
		}
		outFileStr, errS2 := expectStr(args, 1, "file.Base64Decode(inFile, outFile)")
		if errS2 != nil {
			return *errS2
		}

		inFile, e1 := absPathVal(inFileStr)
		if e1 != nil {
			return *e1
		}

		outFile, e2 := absPathVal(outFileStr)
		if e2 != nil {
			return *e2
		}

		// Zielverzeichnis muss existieren
		if _, err := os.Stat(filepath.Dir(outFile)); err != nil {
			return valpkg.ErrorVal("Zielverzeichnis existiert nicht: " + filepath.Dir(outFile))
		}

		in, err := os.Open(inFile)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Öffnen der Quelldatei: " + err.Error())
		}
		defer in.Close()

		out, err := os.Create(outFile)
		if err != nil {
			return valpkg.ErrorVal("Zieldatei konnte nicht erstellt werden: " + err.Error())
		}
		defer out.Close()

		dec := base64.NewDecoder(base64.StdEncoding, in)
		if _, err := io.Copy(out, dec); err != nil {
			return valpkg.ErrorVal("Dekodierungsfehler: " + err.Error())
		}

		return valpkg.NullVal()
	})

	// ---------------- Delete ----------------
	Register(ns+"Delete", "file",
		"path",
		"Löscht eine Datei.",
		func(args []valpkg.Value) valpkg.Value {

			pathStr, errS := expectStr(args, 0, "file.Delete(path)")
			if errS != nil {
				return *errS
			}
			if pathStr == "" {
				return valpkg.ErrorVal("file.Delete: Pfad fehlt")
			}

			path, errVal := absPathVal(pathStr)
			if errVal != nil {
				return valpkg.ErrorVal("file.Delete: ungültiger Pfad")
			}

			err := os.Remove(path)
			if err != nil {
				// "nicht existiert" ist kein echter Fehler
				if os.IsNotExist(err) {
					return valpkg.NullVal()
				}
				return valpkg.ErrorVal("file.Delete: " + err.Error() + " path=" + path)
			}

			return valpkg.NullVal()
		})

	// ---------------- Copy ----------------
	Register(ns+"Copy", "file",
		"src, dst, [overwrite]",
		"Kopiert eine Datei. Mit overwrite=true wird ein bestehendes Ziel überschrieben, sonst schlägt der Aufruf fehl, falls es existiert.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 2 {
				return valpkg.ErrorVal("file.Copy: benötigt src und dst")
			}

			srcStr, errS1 := expectStr(args, 0, "file.Copy(src, dst, [overwrite])")
			if errS1 != nil {
				return *errS1
			}
			dstStr, errS2 := expectStr(args, 1, "file.Copy(src, dst, [overwrite])")
			if errS2 != nil {
				return *errS2
			}

			overwrite := false
			if len(args) >= 3 {
				overwrite = fileTruthy(args[2])
			}

			src, e1 := absPathVal(srcStr)
			if e1 != nil {
				return valpkg.ErrorVal("file.Copy: ungültiger Quellpfad")
			}

			dst, e2 := absPathVal(dstStr)
			if e2 != nil {
				return valpkg.ErrorVal("file.Copy: ungültiger Zielpfad")
			}

			if _, err := os.Stat(src); err != nil {
				return valpkg.ErrorVal("file.Copy: Quelle fehlt: " + src)
			}

			if _, err := os.Stat(filepath.Dir(dst)); err != nil {
				return valpkg.ErrorVal("file.Copy: Zielverzeichnis fehlt: " + filepath.Dir(dst))
			}

			if _, err := os.Stat(dst); err == nil && !overwrite {
				return valpkg.ErrorVal("file.Copy: Ziel existiert bereits: " + dst)
			}

			if err := copyFile(src, dst); err != nil {
				return valpkg.ErrorVal("file.Copy: " + err.Error())
			}

			return valpkg.NullVal()
		})

	// ---------------- Move ----------------
	Register(ns+"Move", "file",
		"src, dst, [overwrite]",
		"Verschiebt oder benennt eine Datei um (Rename + Cross-Drive Fallback). Mit overwrite=true wird ein bestehendes Ziel überschrieben, sonst schlägt der Aufruf fehl, falls es existiert.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 2 {
				return valpkg.ErrorVal("file.Move: benötigt src und dst")
			}

			srcStr, errS1 := expectStr(args, 0, "file.Move(src, dst, [overwrite])")
			if errS1 != nil {
				return *errS1
			}
			dstStr, errS2 := expectStr(args, 1, "file.Move(src, dst, [overwrite])")
			if errS2 != nil {
				return *errS2
			}

			overwrite := false
			if len(args) >= 3 {
				overwrite = fileTruthy(args[2])
			}

			src, e1 := absPathVal(srcStr)
			if e1 != nil {
				return valpkg.ErrorVal("file.Move: ungültiger Quellpfad")
			}

			dst, e2 := absPathVal(dstStr)
			if e2 != nil {
				return valpkg.ErrorVal("file.Move: ungültiger Zielpfad")
			}

			if _, err := os.Stat(src); err != nil {
				return valpkg.ErrorVal("file.Move: Quelle existiert nicht: " + src)
			}

			if _, err := os.Stat(filepath.Dir(dst)); err != nil {
				return valpkg.ErrorVal("file.Move: Zielverzeichnis fehlt: " + filepath.Dir(dst))
			}

			if _, err := os.Stat(dst); err == nil && !overwrite {
				return valpkg.ErrorVal("file.Move: Zieldatei existiert bereits: " + dst)
			}

			// 1. Versuch: Rename (schnell, atomar)
			if err := os.Rename(src, dst); err == nil {
				return valpkg.NullVal()
			}

			// 2. Fallback: Copy + Delete (Cross-Drive)
			if err := copyAndDelete(src, dst); err != nil {
				return valpkg.ErrorVal("file.Move: Cross-Drive-Fallback fehlgeschlagen: " + err.Error() + " src=" + src + " dst=" + dst)
			}

			return valpkg.NullVal()
		})

	// ------------------------
	// file.Compare (Optimiert)
	// ------------------------
	// Hinweis: Nutzt bewusst valpkg.ToString() statt expectStr(), da hier auch
	// Zahlen/Null als Pfad toleriert werden sollen (siehe Kommentar unten).
	Register(ns+"Compare", "file", "pfad1, pfad2", "Vergleicht zwei Dateien auf Gleichheit.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.Compare(path1, path2) benötigt 2 Pfade")
		}

		// Sicherheit: Falls Zahlen oder Null als Pfad übergeben werden
		p1Raw := valpkg.ToString(args[0])
		p2Raw := valpkg.ToString(args[1])

		p1, e1 := absPathVal(p1Raw)
		p2, e2 := absPathVal(p2Raw)
		if e1 != nil {
			return *e1
		}
		if e2 != nil {
			return *e2
		}

		// Performance-Check: Wenn es derselbe Pfad ist, sind sie identisch
		if p1 == p2 {
			return valpkg.NumVal(0)
		}

		f1, err1 := os.Open(p1)
		if err1 != nil {
			return valpkg.ErrorVal("Fehler beim Öffnen von Pfad 1: " + err1.Error())
		}
		defer f1.Close()

		f2, err2 := os.Open(p2)
		if err2 != nil {
			return valpkg.ErrorVal("Fehler beim Öffnen von Pfad 2: " + err2.Error())
		}
		defer f2.Close()

		// Schneller Vorab-Check: Wenn die Dateigrößen unterschiedlich sind,
		// müssen wir gar nicht erst die Bytes lesen.
		stat1, _ := f1.Stat()
		stat2, _ := f2.Stat()
		if stat1.Size() != stat2.Size() {
			return valpkg.NumVal(1) // Unterschiedlich
		}

		// Byte-für-Byte Vergleich mit Buffer (4KB ist meist optimal)
		buf1 := make([]byte, 4096)
		buf2 := make([]byte, 4096)

		for {
			n1, r1 := f1.Read(buf1)
			n2, r2 := f2.Read(buf2)

			// Vergleiche die Anzahl gelesener Bytes und den Inhalt
			// bytes.Equal ist deutlich schneller als ein String-Cast
			if n1 != n2 || !bytes.Equal(buf1[:n1], buf2[:n2]) {
				return valpkg.NumVal(1) // Unterschiedlich
			}

			// Ende beider Dateien erreicht
			if r1 == io.EOF && r2 == io.EOF {
				break
			}

			// Falls ein anderer Fehler beim Lesen auftritt
			if r1 != nil && r1 != io.EOF {
				return valpkg.ErrorVal("Lesefehler Datei 1: " + r1.Error())
			}
			if r2 != nil && r2 != io.EOF {
				return valpkg.ErrorVal("Lesefehler Datei 2: " + r2.Error())
			}
		}

		return valpkg.NumVal(0) // Identisch
	})

	// ---------------- Size ----------------
	// Gruppe korrigiert: "files" -> "file" (war Tippfehler, brach die
	// einheitliche Gruppierung aller anderen file.*-Funktionen)
	Register(ns+"Size", "file", "pfad", "Gibt die Größe einer Datei in Bytes zurück.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.Size(pfad)")
		if errS != nil {
			return *errS
		}
		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		info, err := os.Stat(path)
		if err != nil {
			return valpkg.ErrorVal("Größe konnte nicht ermittelt werden: " + err.Error())
		}

		size := float64(info.Size())
		if len(args) >= 2 && args[1].Str == "human" {
			return valpkg.StrVal(fileFormatSize(size)) // Hilfsfunktion nutzen
		}
		return valpkg.NumVal(size)
	})

	// ---------------- ModTime ----------------
	Register(ns+"ModTime", "file", "path", "Gibt den Zeitpunkt der letzten Änderung im ISO-Format (RFC3339) zurück.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.ModTime(path)")
		if errS != nil {
			return *errS
		}
		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}
		info, err := os.Stat(path)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen der ModTime von '" + path + "': " + err.Error())
		}
		return valpkg.StrVal(info.ModTime().Format(time.RFC3339))
	})

	// ---------------- CreateTime ----------------
	Register(ns+"CreateTime", "file", "path", "Gibt den Zeitpunkt der Erstellung im ISO-Format (RFC3339) zurück.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.CreateTime(path)")
		if errS != nil {
			return *errS
		}
		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}
		c, _, _, err := FileTimes(path)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen der CreateTime von '" + path + "': " + err.Error())
		}
		return valpkg.StrVal(c.Format(time.RFC3339))
	})

	// ---------------- AccessTime ----------------
	Register(ns+"AccessTime", "file", "path", "Gibt den Zeitpunkt des letzten Zugriffs im ISO-Format (RFC3339) zurück.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.AccessTime(path)")
		if errS != nil {
			return *errS
		}
		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}
		_, a, _, err := FileTimes(path)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen der AccessTime von '" + path + "': " + err.Error())
		}
		return valpkg.StrVal(a.Format(time.RFC3339))
	})

	// ---------------- Replace / ReplaceAll ----------------
	Register(ns+"Replace", "file", "path, alt, neu", "Ersetzt das erste Vorkommen von alt durch neu.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("file.Replace(path, pattern, newContent) benötigt 3 Argumente")
		}

		pathStr, errS := expectStr(args, 0, "file.Replace(path, alt, neu)")
		if errS != nil {
			return *errS
		}

		// 1. Pfad-Sicherheitscheck (Gibt (string, *valpkg.Value) zurück)
		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		pattern := args[1].Str
		newContent := args[2].Str

		// 2. Datei einlesen
		data, err := os.ReadFile(path)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen: " + err.Error())
		}

		// 3. Zeilenweise verarbeiten
		// Wir normalisieren die Zeilenumbrüche für den Split
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines := strings.Split(content, "\n")

		found := false
		for i, line := range lines {
			if strings.Contains(line, pattern) {
				lines[i] = newContent
				found = true
				break // Nur das erste Vorkommen ersetzen
			}
		}

		// 4. Nur schreiben, wenn wirklich etwas gefunden wurde (optimiert die SSD)
		if found {
			err = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
			if err != nil {
				return valpkg.ErrorVal("Fehler beim Schreiben: " + err.Error())
			}
		}

		return valpkg.NullVal()
	})

	Register(ns+"Search", "file", "pfad, muster", "Sucht nach einem Textmuster innerhalb einer Datei.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.Search(path, pattern) benötigt Pfad und Suchmuster")
		}

		pathStr, errS := expectStr(args, 0, "file.Search(pfad, muster)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}
		pattern := args[1].Str

		// Nutzt unseren Shared-Open-Helper (Windows-sicher!)
		file, err := openFileShared(path)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht geöffnet werden: " + err.Error())
		}
		defer file.Close()

		var results []string
		scanner := bufio.NewScanner(file)

		// Puffer auf 1MB erhöhen für extrem lange Log-Zeilen (z.B. Stacktraces)
		const maxLogLineSize = 1024 * 1024
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, maxLogLineSize)

		for scanner.Scan() {
			line := scanner.Text()
			// Tipp: In BASIC ist es oft hilfreich, Case-Insensitive zu suchen
			if strings.Contains(strings.ToLower(line), strings.ToLower(pattern)) {
				results = append(results, line)
			}
		}

		if err := scanner.Err(); err != nil {
			return valpkg.ErrorVal("Fehler beim Scannen (Zeile zu lang?): " + err.Error())
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: stringSliceToValueSlice(results)}
	})

	Register(ns+"SearchDateRange", "file", "pfad, von, bis", "Findet Dateien in einem Zeitraum (YYYY-MM-DD).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("SearchDateRange(path, start, end, [layout], [max]) fehlt Argument")
		}

		pathStr, errS := expectStr(args, 0, "file.SearchDateRange(pfad, von, bis)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}
		startStr := args[1].Str
		endStr := args[2].Str

		// Benutzerfreundliches Format (Default: YYYY-MM-DD HH:mm:SS)
		userLayout := "YYYY-MM-DD HH:mm:SS"
		if len(args) >= 4 && args[3].Str != "" {
			userLayout = args[3].Str
		}

		// INTERNE KONVERTIERUNG zu Go-Standard (2006-01-02...)
		layout := mapUserLayoutToGo(userLayout)

		maxResults := 5000
		if len(args) >= 5 {
			maxResults = int(valpkg.ToNum(args[4]))
		}

		// Ab hier bleibt die Logik gleich...
		startTime, errS2 := time.Parse(layout, startStr)
		endTime, errE := time.Parse(layout, endStr)
		if errS2 != nil || errE != nil {
			return valpkg.ErrorVal("Zeit-Parsing Fehler: Passt das Datum zum Format " + userLayout + "?")
		}

		// 3. Shared Open (Windows-sicher)
		file, err := openFileShared(path)
		if err != nil {
			return valpkg.ErrorVal("Datei-Fehler: " + err.Error())
		}
		defer file.Close()

		var results []string
		scanner := bufio.NewScanner(file)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 1024*1024) // 1MB Zeilen-Limit

		isInRange := false
		layoutLen := len(layout)

		for scanner.Scan() {
			// Abbruch, wenn Limit erreicht
			if len(results) >= maxResults {
				break
			}

			line := scanner.Text()

			// Datumserkennung am Zeilenanfang
			if len(line) >= layoutLen {
				lineDateStr := line[:layoutLen]
				if lineTime, err := time.Parse(layout, lineDateStr); err == nil {
					// Neues Datum gefunden -> Bereich neu prüfen
					isInRange = (lineTime.After(startTime) || lineTime.Equal(startTime)) &&
						(lineTime.Before(endTime) || lineTime.Equal(endTime))
				}
			}

			// Zeile hinzufügen, wenn wir im Bereich sind (auch Folgezeilen ohne Datum)
			if isInRange {
				results = append(results, line)
			}
		}

		if err := scanner.Err(); err != nil {
			return valpkg.ErrorVal("Scan-Fehler: " + err.Error())
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: stringSliceToValueSlice(results)}
	})

	Register(ns+"ReplaceAll", "file", "path, alt, neu", "Ersetzt alle Vorkommen von alt durch neu.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("file.ReplaceAll(path, old, new) benötigt 3 Argumente")
		}

		pathStr, errS := expectStr(args, 0, "file.ReplaceAll(path, alt, neu)")
		if errS != nil {
			return *errS
		}

		// 1. Pfad validieren (Gibt path und *valpkg.Value zurück)
		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal // Sicherheits-Check
		}

		oldStr := args[1].Str
		newStr := args[2].Str

		// 2. Datei komplett einlesen
		data, err := os.ReadFile(path)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen der Datei: " + err.Error())
		}

		// 3. Ersetzung im Speicher durchführen
		content := strings.ReplaceAll(string(data), oldStr, newStr)

		// 4. Datei überschreiben
		// Wir nutzen 0644 (Standard-Rechte), falls die Datei neu erstellt wird
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Speichern der Änderungen: " + err.Error())
		}

		return valpkg.NullVal() // Erfolg
	})

	Register(ns+"GitBlobHash", "file", "path",
		"Berechnet den Git-Blob-SHA1 einer Datei.",
		func(args []valpkg.Value) valpkg.Value {

			pathStr, errS := expectStr(args, 0, "file.GitBlobHash(path)")
			if errS != nil {
				return *errS
			}

			path, errVal := absPathVal(pathStr)
			if errVal != nil {
				return *errVal
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return valpkg.ErrorVal("GitHash-Fehler: " + err.Error())
			}

			header := fmt.Sprintf("blob %d\x00", len(data))

			h := sha1.New()
			h.Write([]byte(header))
			h.Write(data)

			return valpkg.StrVal(hex.EncodeToString(h.Sum(nil)))
		})

	Register(ns+"GitBlobCompare", "file", "path, remoteSha",
		"Vergleicht den Git-Blob-SHA1 einer lokalen Datei mit einem erwarteten SHA.",
		func(args []valpkg.Value) valpkg.Value {
			pathStr, errS := expectStr(args, 0, "file.GitBlobCompare(path, remoteSha)")
			if errS != nil {
				return *errS
			}
			if len(args) < 2 || args[1].Kind != valpkg.KindStr {
				return valpkg.ErrorVal("file.GitBlobCompare erwartet einen SHA-String")
			}
			path, errVal := absPathVal(pathStr)
			if errVal != nil {
				return *errVal
			}
			if _, err := os.Stat(path); err != nil {
				if os.IsNotExist(err) {
					return valpkg.NumVal(-1)
				}
				return valpkg.ErrorVal("GitBlobCompare: " + err.Error())
			}
			info, ok := Lookup("file.GitBlobHash")
			if !ok {
				return valpkg.ErrorVal("file.GitBlobHash ist nicht verfügbar")
			}
			hash := info.Fn([]valpkg.Value{valpkg.StrVal(path)})
			if hash.Kind == valpkg.KindError {
				return hash
			}
			if strings.EqualFold(strings.TrimSpace(hash.Str), strings.TrimSpace(args[1].Str)) {
				return valpkg.NumVal(0)
			}
			return valpkg.NumVal(1)
		})

	// ---------------- Hash ----------------
	Register(ns+"Hash", "file", "path [, algo]", "Berechnet den Hash-Wert einer Datei und gibt ihn als Hex-String zurück.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.Hash(path [, algo])")
		if errS != nil {
			return *errS
		}
		algo := "md5"
		if len(args) >= 2 {
			algo = strings.ToLower(strings.TrimSpace(args[1].Str))
		}
		h, err := hashSingleFile(pathStr, algo)
		if err != nil {
			return valpkg.ErrorVal("Hash-Fehler: " + err.Error())
		}
		return valpkg.StrVal(h)
	})

	Register(ns+"HashBatch", "file", "paths [, algo, workers]",
		"Berechnet Hashes mehrerer Dateien parallel. Gibt eine Map path->hash zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 || args[0].Kind != valpkg.KindArr {
				return valpkg.ErrorVal("file.HashBatch(paths [, algo, workers]) benötigt ein Array von Pfaden")
			}

			algo := "md5"
			if len(args) >= 2 {
				algo = strings.ToLower(strings.TrimSpace(args[1].Str))
			}

			workers := 8
			if len(args) >= 3 {
				w := int(valpkg.ToNum(args[2]))
				if w > 0 {
					workers = w
				}
			}

			paths := make([]string, 0, len(args[0].Arr))
			for _, v := range args[0].Arr {
				paths = append(paths, v.Str)
			}

			type result struct {
				path string
				hash string
				err  string
			}

			jobs := make(chan string, len(paths))
			results := make(chan result, len(paths))

			var wg sync.WaitGroup
			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for p := range jobs {
						h, err := hashSingleFile(p, algo)
						if err != nil {
							results <- result{path: p, err: err.Error()}
							continue
						}
						results <- result{path: p, hash: h}
					}
				}()
			}

			for _, p := range paths {
				jobs <- p
			}
			close(jobs)

			go func() {
				wg.Wait()
				close(results)
			}()

			out := make(map[string]valpkg.Value)
			for r := range results {
				if r.err != "" {
					out[r.path] = valpkg.ErrorVal(r.err)
					continue
				}
				out[r.path] = valpkg.StrVal(r.hash)
			}

			return valpkg.Value{Kind: valpkg.KindMap, Map: out}
		})

	Register(ns+"HashVerify", "file", "hash, algo",
		"Prüft, ob ein String ein gültiger Hash-Wert des angegebenen Algorithmus ist (Format/Länge, keine Dateiprüfung).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("file.HashVerify(hash, algo) benötigt Hash und Algorithmus")
			}
			h := strings.TrimSpace(args[0].Str)
			algo := strings.ToLower(strings.TrimSpace(args[1].Str))

			expectedLen := map[string]int{
				"md5":    32,
				"sha1":   40,
				"sha224": 56,
				"sha256": 64,
				"sha384": 96,
				"sha512": 128,
			}

			l, ok := expectedLen[algo]
			if !ok {
				return valpkg.ErrorVal("unbekannter Hash-Algorithmus: " + algo)
			}

			if len(h) != l {
				return valpkg.BoolVal(false)
			}

			for _, c := range h {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					return valpkg.BoolVal(false)
				}
			}
			return valpkg.BoolVal(true)
		})

	Register(ns+"VerifyHash", "file", "path, expectedHash, [algo]",
		"Prüft, ob eine Datei tatsächlich den erwarteten Hash-Wert hat.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("file.VerifyHash(path, expectedHash [, algo]) benötigt Pfad und erwarteten Hash")
			}
			pathStr, errS := expectStr(args, 0, "file.VerifyHash(path, expectedHash [, algo])")
			if errS != nil {
				return *errS
			}
			algo := "sha256"
			if len(args) >= 3 {
				algo = strings.ToLower(strings.TrimSpace(args[2].Str))
			}
			h, err := hashSingleFile(pathStr, algo)
			if err != nil {
				return valpkg.ErrorVal("Hash-Fehler: " + err.Error())
			}
			return valpkg.BoolVal(strings.EqualFold(h, strings.TrimSpace(args[1].Str)))
		})

	// ---------------- Rename ----------------
	Register(ns+"Rename", "file", "alt, neu", "Benennt eine Datei um.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.Rename(oldPath, newPath) benötigt 2 Argumente")
		}

		oldStr, errS1 := expectStr(args, 0, "file.Rename(alt, neu)")
		if errS1 != nil {
			return *errS1
		}
		newStr, errS2 := expectStr(args, 1, "file.Rename(alt, neu)")
		if errS2 != nil {
			return *errS2
		}

		// 1. Quellpfad validieren
		oldPath, e1 := absPathVal(oldStr)
		if e1 != nil {
			return *e1 // Abbruch, falls Quelle illegal (z.B. C:\ auf Linux)
		}

		// 2. Zielpfad validieren
		newPath, e2 := absPathVal(newStr)
		if e2 != nil {
			return *e2 // Abbruch, falls Ziel illegal
		}

		// 3. Die eigentliche Operation
		err := os.Rename(oldPath, newPath)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Umbenennen/Verschieben: " + err.Error())
		}

		return valpkg.NullVal()
	})

	// ------------------------
	// AppendAllText
	// ------------------------
	Register(ns+"AppendAllText", "file", "path, text", "Hängt Text an eine Datei an (erstellt diese, falls nicht vorhanden).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.AppendAllText(path, text) benötigt Pfad und Text")
		}

		if args[0].Kind != valpkg.KindStr {
			return valpkg.ErrorVal("file.AppendAllText: erwartet einen Text-Pfad, erhalten: " + valpkg.GetKindName(args[0].Kind))
		}

		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}

		// Verzeichnis muss existieren
		if _, err := os.Stat(filepath.Dir(path)); err != nil {
			return valpkg.ErrorVal("Verzeichnis existiert nicht: " + filepath.Dir(path))
		}

		// Klarere Meldung, falls der Zielpfad selbst ein Verzeichnis ist
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return valpkg.ErrorVal("Der angegebene Pfad ist ein Verzeichnis, keine Datei: " + path)
		}

		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht geöffnet werden: " + err.Error())
		}
		defer f.Close()

		if _, err := f.WriteString(args[1].Str); err != nil {
			return valpkg.ErrorVal("Schreibfehler beim Anhängen: " + err.Error())
		}

		return valpkg.NullVal()
	})

	// ------------------------
	// AppendLine
	// ------------------------
	Register(ns+"AppendLine", "file", "path, line", "Hängt eine Zeile an eine Datei an (fügt automatisch einen Zeilenumbruch ein, erstellt die Datei falls nötig).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.AppendLine(path, line) benötigt Pfad und Zeile")
		}

		pathStr, errS := expectStr(args, 0, "file.AppendLine(path, line)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		// Verzeichnis muss existieren
		if _, err := os.Stat(filepath.Dir(path)); err != nil {
			return valpkg.ErrorVal("Verzeichnis existiert nicht: " + filepath.Dir(path))
		}

		// Prüfen, ob die Datei bereits existiert und ob ihr letztes Byte
		// ein Zeilenumbruch ist. Ohne diese Prüfung würde die neue Zeile
		// direkt hinter den bestehenden Inhalt geschrieben, falls die
		// Datei nicht bereits mit \n endet (z.B. bei einer Datei, die
		// zuvor per file.Write ohne abschließenden Umbruch erzeugt wurde).
		needsLeadingNewline := false
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			rf, err := os.Open(path)
			if err != nil {
				return valpkg.ErrorVal("Datei konnte nicht zum Prüfen geöffnet werden: " + err.Error())
			}
			lastByte := make([]byte, 1)
			_, err = rf.ReadAt(lastByte, info.Size()-1)
			rf.Close()
			if err != nil {
				return valpkg.ErrorVal("Letztes Byte konnte nicht gelesen werden: " + err.Error())
			}
			if lastByte[0] != '\n' {
				needsLeadingNewline = true
			}
		}

		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht geöffnet werden: " + err.Error())
		}
		defer f.Close()

		content := args[1].Str + "\n"
		if needsLeadingNewline {
			content = "\n" + content
		}

		if _, err := f.WriteString(content); err != nil {
			return valpkg.ErrorVal("Schreibfehler beim Anhängen: " + err.Error())
		}

		return valpkg.BoolVal(true)
	})

	// ---------------- HasContent ----------------
	// Fix: Vorher fehlte der len(args)-Check komplett -> Panic (index out
	// of range) bei Aufruf ohne Argument. Jetzt wie alle anderen Funktionen
	// mit expectStr abgesichert.
	Register(ns+"HasContent", "file", "pfad", "Prüft, ob eine Datei existiert und mehr als 0 Byte Inhalt hat. Gibt bei nicht existierender Datei false zurück.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.HasContent(pfad)")
		if errS != nil {
			return *errS
		}
		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			return valpkg.BoolVal(false)
		}
		return valpkg.BoolVal(info.Size() > 0)
	})

	// ---------------- ReadAllLines ----------------
	Register(ns+"ReadAllLines", "file", "pfad", "Liest eine Textdatei zeilenweise in ein Array.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.ReadAllLines(pfad)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		// --- NEU: Shared Access nutzen ---
		file, err := openFileShared(path)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Öffnen der Zeilen: " + err.Error())
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen der Zeilen: " + err.Error())
		}

		// Normalisierung der Zeilenenden
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		content = strings.TrimRight(content, "\n")

		lines := strings.Split(content, "\n")

		return valpkg.Value{
			Kind: valpkg.KindArr,
			Arr:  stringSliceToValueSlice(lines),
		}
	})

	Register(ns+"InsertAfter", "file", "pfad, ausgabepfad, suchtext, einfügen", "Fügt in jeder Zeile nach der ersten Fundstelle des Suchtextes den angegebenen Text ein. Ist der Ausgabepfad leer, wird die Originaldatei überschrieben.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 4 {
			return valpkg.ErrorVal("InsertAfter erwartet vier Parameter: Pfad, Ausgabepfad, Suchtext und Einfügetext")
		}

		// --- Eingabepfad ---
		pathStr, errS := expectStr(args, 0, "file.InsertAfter(pfad, ausgabepfad, suchtext, einfügen)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		// --- Ausgabepfad ---
		outputStr, errS := expectStr(args, 1, "file.InsertAfter(pfad, ausgabepfad, suchtext, einfügen)")
		if errS != nil {
			return *errS
		}

		outputPath := path

		// Wenn ein Ausgabepfad angegeben wurde, diesen verwenden.
		// Bei "" wird die Originaldatei überschrieben.
		if outputStr != "" {
			outputPath, errVal = absPathVal(outputStr)
			if errVal != nil {
				return *errVal
			}
		}

		// --- Suchtext ---
		searchText, errS := expectStr(args, 2, "file.InsertAfter(pfad, ausgabepfad, suchtext, einfügen)")
		if errS != nil {
			return *errS
		}

		// --- Einfügetext ---
		insertText, errS := expectStr(args, 3, "file.InsertAfter(pfad, ausgabepfad, suchtext, einfügen)")
		if errS != nil {
			return *errS
		}

		if searchText == "" {
			return valpkg.ErrorVal("InsertAfter: Der Suchtext darf nicht leer sein")
		}

		// --- Datei öffnen ---
		file, err := openFileShared(path)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Öffnen der Datei: " + err.Error())
		}
		defer file.Close()

		// --- Datei lesen ---
		data, err := io.ReadAll(file)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen der Datei: " + err.Error())
		}

		// --- Zeilenenden normalisieren ---
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		content = strings.TrimRight(content, "\n")

		lines := strings.Split(content, "\n")

		// --- Erste Fundstelle pro Zeile bearbeiten ---
		for i, line := range lines {
			pos := strings.Index(line, searchText)

			if pos >= 0 {
				insertPos := pos + len(searchText)
				lines[i] = line[:insertPos] + insertText + line[insertPos:]
			}
		}

		// --- Ergebnis erzeugen ---
		output := strings.Join(lines, "\n")

		// --- Ergebnis schreiben ---
		if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
			return valpkg.ErrorVal("Fehler beim Schreiben der Datei: " + err.Error())
		}

		return valpkg.NullVal()
	})

	Register(ns+"TextStats", "file", "path", "Zählt Wörter, Zeichen (ohne Leerzeichen) und Leerzeichen im Inhalt einer Datei.", func(args []valpkg.Value) valpkg.Value {
		// --- Pfad ---
		pathStr, errS := expectStr(args, 0, "file.TextStats(path)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		// --- Datei öffnen ---
		file, err := openFileShared(path)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Öffnen der Datei: " + err.Error())
		}
		defer file.Close()

		// --- Datei lesen ---
		data, err := io.ReadAll(file)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen der Datei: " + err.Error())
		}

		content := string(data)

		// --- Wörter zählen ---
		// strings.Fields kümmert sich um mehrfache/verschiedene
		// Whitespace-Zeichen, analog zu string.WordCount.
		wordCount := len(strings.Fields(content))

		// --- Zeichen (ohne Leerzeichen) UND Leerzeichen in einem Durchlauf zählen ---
		// Satz-/Sonderzeichen zählen bewusst zu charCount mit -- bei
		// beliebigem Text (Brief, Aufsatz, ...) wäre es sonst willkürlich,
		// was als "Zeichen" zählt und was nicht. charCount + spaceCount
		// ergibt damit die Gesamtzeichenzahl des Dateiinhalts.
		charCount := 0
		spaceCount := 0

		for _, r := range content {
			if unicode.IsSpace(r) {
				spaceCount++
			} else {
				charCount++
			}
		}

		// --- Ergebnis zurückgeben ---
		return valpkg.Value{
			Kind: valpkg.KindArr,
			Arr: []valpkg.Value{
				valpkg.NumVal(float64(wordCount)),
				valpkg.NumVal(float64(charCount)),
				valpkg.NumVal(float64(spaceCount)),
			},
		}
	})

	// ---------------- LineCount ----------------
	Register(ns+"LineCount", "file", "path", "Gibt die Anzahl der Zeilen einer Textdatei zurück.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.LineCount(path)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		file, err := openFileShared(path)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht geöffnet werden: " + err.Error())
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 1024*1024) // 1MB Zeilen-Limit, konsistent mit file.Search

		count := 0
		for scanner.Scan() {
			count++
		}

		if err := scanner.Err(); err != nil {
			return valpkg.ErrorVal("Fehler beim Zählen der Zeilen (Zeile zu lang?): " + err.Error())
		}

		return valpkg.NumVal(float64(count))
	})

	// ---------------- Head ----------------
	Register(ns+"Head", "file", "path, n", "Gibt die ersten n Zeilen einer Datei zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.Head(path, n) benötigt Pfad und Zeilenanzahl")
		}

		pathStr, errS := expectStr(args, 0, "file.Head(path, n)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		n := int(valpkg.ToNum(args[1]))
		if n <= 0 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		if n > 5000 {
			n = 5000 // konsistent mit dem Limit von file.Tail
		}

		file, err := openFileShared(path)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht geöffnet werden: " + err.Error())
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 1024*1024) // 1MB Zeilen-Limit, konsistent mit file.Search

		var results []string
		for scanner.Scan() && len(results) < n {
			results = append(results, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen (Zeile zu lang?): " + err.Error())
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: stringSliceToValueSlice(results)}
	})

	// ---------------- ReadAllText ----------------
	Register(ns+"ReadAllText", "file", "pfad", "Liest den gesamten Inhalt einer Datei als String.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.ReadAllText(pfad)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		// --- NEU: Shared Access nutzen ---
		file, err := openFileShared(path)
		if err != nil {
			return valpkg.ErrorVal("Lesefehler in ReadAllText: " + err.Error())
		}
		defer file.Close()

		// Den gesamten Inhalt aus dem Handle lesen
		content, err := io.ReadAll(file)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Stream-Lesen: " + err.Error())
		}

		return valpkg.Value{Kind: valpkg.KindStr, Str: string(content)}
	})

	// ---------------- Ext ----------------
	// Hinweis: Fehlendes Argument liefert weiterhin "" (kein Fehler,
	// bewusstes Verhalten). Falscher Typ (z.B. Array) liefert jetzt aber
	// einen echten Fehler statt eines stillen "".
	Register(ns+"Ext", "file", "pfad", "Gibt die Dateiendung zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.StrVal("")
		}
		if args[0].Kind != valpkg.KindStr {
			return valpkg.ErrorVal(fmt.Sprintf("file.Ext: erwarte einen Dateipfad (String), habe %s bekommen", valpkg.GetKindName(args[0].Kind)))
		}

		// 1. Pfad validieren (Wichtig: absPathVal gibt 2 Werte zurück!)
		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal // Sicherheits-Stopp
		}

		// 2. Erweiterung extrahieren
		ext := filepath.Ext(path)

		// 3. Den Punkt entfernen (aus ".txt" wird "txt")
		return valpkg.StrVal(strings.TrimPrefix(ext, "."))
	})

	// ---------------- Name ----------------
	Register(ns+"Name", "file", "pfad", "Gibt den Dateinamen ohne Verzeichnispfad zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.StrVal("")
		}
		if args[0].Kind != valpkg.KindStr {
			return valpkg.ErrorVal(fmt.Sprintf("file.Name: erwarte einen Dateipfad (String), habe %s bekommen", valpkg.GetKindName(args[0].Kind)))
		}

		// 1. Pfad validieren und entpacken
		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal // Sicherheits-Stopp bei illegalen Pfaden
		}

		// 2. Basisname holen (z.B. "test.txt")
		base := filepath.Base(path)

		// 3. Endung holen (z.B. ".txt")
		ext := filepath.Ext(base)

		// 4. Endung abschneiden und zurückgeben
		return valpkg.StrVal(strings.TrimSuffix(base, ext))
	})

	// ---------------- Dir ----------------
	Register(ns+"Dir", "file", "pfad", "Gibt das Verzeichnis (Parent) eines Pfades zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.StrVal("")
		}
		if args[0].Kind != valpkg.KindStr {
			return valpkg.ErrorVal(fmt.Sprintf("file.Dir: erwarte einen Dateipfad (String), habe %s bekommen", valpkg.GetKindName(args[0].Kind)))
		}

		// 1. Pfad validieren (Gibt (string, *valpkg.Value) zurück)
		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal // Sicherheits-Stopp
		}

		// 2. Das übergeordnete Verzeichnis ermitteln
		// filepath.Dir gibt unter Windows "\" oder unter Linux "/" zurück,
		// wenn kein Pfad mehr darüber liegt.
		dir := filepath.Dir(path)

		return valpkg.StrVal(dir)
	})

	// ------------------------
	// Pfad zusammensetzen
	// ------------------------
	// Hinweis: Nutzt bewusst toStringSafe() für alle Segmente, da Join
	// beliebige Werte (auch Zahlen etc.) tolerant in Pfadteile umwandeln soll.
	Register(ns+"Join", "file", "pfad, teil, ...", "Verbindet beliebig viele Pfadsegmente sicher miteinander.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.Value{Kind: valpkg.KindStr, Str: ""}
		}

		parts := make([]string, len(args))
		for i, v := range args {
			// Wandelt alles zuverlässig in String um
			parts[i] = fileStringSafe(v, "")
		}

		return valpkg.Value{Kind: valpkg.KindStr, Str: filepath.Join(parts...)}
	})

	Register(ns+"ReadBytes", "file", "path",
		"Liest eine Datei als Byte-Array ein und gibt ein Array von Zahlen (0–255) zurück.",
		func(args []valpkg.Value) valpkg.Value {
			pathStr, errS := expectStr(args, 0, "file.ReadBytes(path)")
			if errS != nil {
				return *errS
			}

			path, errVal := absPathVal(pathStr)
			if errVal != nil {
				return *errVal
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return valpkg.ErrorVal("Fehler beim Lesen: " + err.Error())
			}

			arr := make([]valpkg.Value, len(data))
			for i, b := range data {
				arr[i] = valpkg.NumVal(float64(b))
			}
			return valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
		})

	Register(ns+"WriteBytes", "file", "path, byteArray",
		"Schreibt ein Byte-Array (Zahlen 0–255) in eine Datei.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("file.WriteBytes(path, byteArray) benötigt 2 Argumente")
			}
			if args[1].Kind != valpkg.KindArr {
				return valpkg.ErrorVal("file.WriteBytes: zweites Argument muss ein Array sein")
			}

			pathStr, errS := expectStr(args, 0, "file.WriteBytes(path, byteArray)")
			if errS != nil {
				return *errS
			}

			path, errVal := absPathVal(pathStr)
			if errVal != nil {
				return *errVal
			}

			// Zielverzeichnis muss existieren
			if _, err := os.Stat(filepath.Dir(path)); err != nil {
				return valpkg.ErrorVal("Zielverzeichnis existiert nicht: " + filepath.Dir(path))
			}

			buf := make([]byte, len(args[1].Arr))
			for i, v := range args[1].Arr {
				buf[i] = byte(v.Num)
			}

			if err := os.WriteFile(path, buf, 0644); err != nil {
				return valpkg.ErrorVal("Fehler beim Schreiben: " + err.Error())
			}

			return valpkg.NullVal()
		})

	// ------------------------------------------------------------
	// file.Tail -- neuer 4. Parameter "silent" (BoolVal, Default false)
	//
	// silent=false (Standard): unverändertes bisheriges Verhalten,
	// Endlos-Loop mit fmt.Print direkt auf die Konsole (Shell-Nutzung).
	//
	// silent=true: KEIN Print. Statt der Endlosschleife wird nur EIN
	// Warteintervall (die "refresh"-Dauer) abgewartet und dann true
	// zurückgegeben, falls sich die Dateigröße geändert hat (gewachsen
	// oder rotiert/gekürzt), sonst false. Das Skript wickelt die
	// Wiederholung selbst über eine eigene Do Loop ab -- analog zu
	// file.Watch, nur mit derselben "refresh"-Parametersyntax wie Tail.
	// ------------------------------------------------------------

	Register(ns+"Tail", "file", "path, [lines], [refresh], [silent]", "Zeigt die letzten Zeilen einer Datei an. Bei 'refresh' (z.B. '1s') wird die Datei live überwacht. Mit silent=True: kein Print, gibt stattdessen einmalig true/false zurück (für Skript-Nutzung in einer eigenen Schleife).", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.Tail(path, ...)")
		if errS != nil {
			return *errS
		}

		// 1. Pfad auflösen
		path, eVal := absPathVal(pathStr)
		if eVal != nil {
			return *eVal
		}

		// 2. Anzahl der Zeilen (Default: 10)
		requestedLines := 10
		if len(args) >= 2 {
			requestedLines = int(valpkg.ToNum(args[1]))
		}
		if requestedLines <= 0 {
			return valpkg.StrVal("")
		}
		if requestedLines > 5000 {
			requestedLines = 5000
		}

		// 3. Zeit-Parsing (Flexibel: "1s", "500ms" oder reine Zahl als ms)
		var d time.Duration
		isFollowMode := false

		if len(args) >= 3 {
			isFollowMode = true
			durationStr := args[2].Str

			parsed, err := time.ParseDuration(durationStr)
			if err != nil {
				ms, numErr := strconv.Atoi(durationStr)
				if numErr == nil {
					d = time.Duration(ms) * time.Millisecond
				} else {
					d = time.Duration(valpkg.ToNum(args[2])) * time.Millisecond
				}
			} else {
				d = parsed
			}
		}

		// --- NEU: silent-Parameter (4. Argument) ---
		silent := false
		if len(args) >= 4 {
			silent = fileTruthy(args[3])
		}

		// --- Phase 1: Die letzten X Zeilen lesen ---
		file, err := openFileShared(path)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht geöffnet werden: " + err.Error())
		}
		defer file.Close()

		stat, _ := file.Stat()
		filesize := stat.Size()
		var result []byte

		if filesize > 0 {
			var lineCount int
			var offset int64
			chunkSize := int64(4096)

			for offset < filesize && lineCount < requestedLines {
				if filesize-offset < chunkSize {
					chunkSize = filesize - offset
				}
				offset += chunkSize
				file.Seek(filesize-offset, 0)

				buf := make([]byte, chunkSize)
				file.Read(buf)

				for i := len(buf) - 1; i >= 0; i-- {
					if buf[i] == '\n' {
						lineCount++
						if lineCount > requestedLines {
							buf = buf[i+1:]
							break
						}
					}
				}
				result = append(buf, result...)
			}
		}

		// Wenn kein Follow-Modus (Intervall <= 0 oder nicht angegeben): String zurückgeben
		if !isFollowMode || d <= 0 {
			return valpkg.StrVal(string(result))
		}

		// --- NEU: silent-Modus -- EIN Warteintervall, dann true/false ---
		if silent {
			time.Sleep(d)

			newStat, err := os.Stat(path)
			if err != nil {
				return valpkg.ErrorVal("Datei während der Überwachung verloren: " + err.Error())
			}

			currentSize := newStat.Size()
			if currentSize != filesize {
				// gewachsen ODER rotiert/gekürzt -> in beiden Fällen "Änderung"
				return valpkg.BoolVal(true)
			}
			return valpkg.BoolVal(false)
		}

		// --- Phase 2: AutoRefresh-Modus (Blockierend, unverändert) ---
		fmt.Print(string(result))
		lastSize := filesize

		for {
			time.Sleep(d)

			newStat, err := os.Stat(path)
			if err != nil {
				break
			}

			currentSize := newStat.Size()
			if currentSize > lastSize {
				file.Seek(lastSize, 0)
				newBytes := make([]byte, currentSize-lastSize)
				_, err = file.Read(newBytes)
				if err == nil {
					fmt.Print(string(newBytes))
				}
				lastSize = currentSize
			} else if currentSize < lastSize {
				fmt.Println("\n--- Datei wurde rotiert/gekürzt ---")
				lastSize = currentSize
				file.Seek(0, 0)
			}
		}
		return valpkg.NullVal()
	})

	// file.Watch(path [, timeout_ms])
	Register(ns+"Watch", "file", "path, [timeoutMs]", "Wartet, bis die Datei geändert wird. Gibt True bei Änderung, False bei Timeout zurück.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.Watch(path)")
		if errS != nil {
			return *errS
		}

		path, eVal := absPathVal(pathStr)
		if eVal != nil {
			return *eVal
		}

		// Optionaler Timeout, damit das Skript nicht ewig hängt
		timeout := 0
		if len(args) >= 2 {
			timeout = int(valpkg.ToNum(args[1]))
		}

		info, err := os.Stat(path)
		if err != nil {
			return valpkg.ErrorVal("Datei existiert nicht: " + err.Error())
		}

		lastMod := info.ModTime()
		start := time.Now()

		// Wir "pollen" hier in einem sehr feinen Intervall (100ms).
		// Das ist CPU-schonend, reagiert aber quasi sofort.
		for {
			// Prüfen, ob Timeout abgelaufen ist
			if timeout > 0 && time.Since(start).Milliseconds() > int64(timeout) {
				return valpkg.BoolVal(false) // Zeit abgelaufen, keine Änderung
			}

			currentInfo, err := os.Stat(path)
			if err != nil {
				return valpkg.ErrorVal("Datei während der Überwachung verloren: " + err.Error())
			}

			// Hat sich das Änderungsdatum oder die Größe geändert?
			if currentInfo.ModTime().After(lastMod) || currentInfo.Size() != info.Size() {
				return valpkg.BoolVal(true) // Treffer!
			}

			// Kurze Pause, um die CPU nicht auf 100% zu jagen
			time.Sleep(100 * time.Millisecond)
		}
	})

	// ------------------------------------------------------------
	// file.WatchLog -- neue Parameter "silent" (4.) und "timeoutMs" (5.)
	//
	// silent=false (Standard): unverändertes bisheriges Verhalten
	// (Endlos-Loop, druckt Statuszeilen + Treffer farbig auf die Konsole).
	//
	// silent=true: keine Konsolenausgabe. Wartet auf den ERSTEN Treffer
	// des Suchmusters (oder bis timeoutMs abgelaufen ist) und gibt dann
	// true (Treffer) bzw. false (Timeout) zurück. Ohne timeoutMs (oder 0)
	// wartet der Aufruf unbegrenzt, analog zu file.Watch.
	//
	// Hinweis: Nutzt bewusst valpkg.ToString() für path/pattern (siehe Kommentar
	// bei file.Compare), daher hier kein expectStr().
	// ------------------------------------------------------------

	// file.WatchLog(path, pattern [, style, silent, timeoutMs])
	Register(ns+"WatchLog", "file", "path, pattern, [style], [silent], [timeoutMs]", "Überwacht ein Log live auf ein Suchmuster und hebt Zeilen farbig hervor. Mit silent=True: kein Print, wartet auf ersten Treffer (oder Timeout) und gibt true/false zurück.", func(args []valpkg.Value) valpkg.Value {

		if len(args) < 2 {
			return valpkg.ErrorVal("watchlog(path, pattern, [style]) benötigt mindestens Pfad und Suchmuster")
		}

		path := valpkg.ToString(args[0])
		pattern := valpkg.ToString(args[1])

		ansiSequence := ""

		// --- STYLE HANDLING (UNIFIED) ---
		if len(args) >= 3 {
			switch args[2].Kind {
			case valpkg.KindNum:
				ansiSequence = getAnsiCode(args[2].Num)
			case valpkg.KindStr:
				ansiSequence = args[2].Str
			}
		}

		// --- NEU: silent (4. Argument) ---
		silent := false
		if len(args) >= 4 {
			silent = fileTruthy(args[3])
		}

		// --- NEU: timeoutMs (5. Argument, nur relevant bei silent) ---
		timeoutMs := 0
		if len(args) >= 5 {
			timeoutMs = int(valpkg.ToNum(args[4]))
		}

		return watchLogInternal(path, pattern, ansiSequence, silent, timeoutMs)
	})

	Register(ns+"UpdateValue", "file", "path, search, newValue",
		"Sucht die erste Zeile, die mit 'search' beginnt oder identisch ist, und ersetzt sie.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.ErrorVal("Parameter: Pfad, Suche/Eindeutiger String, Neuer Wert")
			}

			path, errVal := absPathVal(valpkg.ToString(args[0]))
			if errVal != nil {
				return *errVal
			}
			search := valpkg.ToString(args[1])
			newValue := valpkg.ToString(args[2])

			// 1. Datei einlesen
			input, err := os.ReadFile(path)
			if err != nil {
				return valpkg.ErrorVal("Lesefehler: " + err.Error())
			}

			content := string(input)
			// Zeilenumbruch-Stil erkennen (für späteres Speichern)
			isCRLF := strings.Contains(content, "\r\n")

			// Normalisieren und Splitten
			lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

			foundIndex := -1
			for i, line := range lines {
				// Wir prüfen auf den Anfang der Zeile (Prefix-Match)
				// Das erlaubt "Port=" ebenso wie "Vollständige Zeile"
				if strings.HasPrefix(strings.TrimSpace(line), search) {
					foundIndex = i
					break // Wir nehmen das erste Vorkommen, das eindeutig genug ist
				}
			}

			if foundIndex == -1 {
				return valpkg.BoolVal(false) // Nichts gefunden -> Skript kann reagieren
			}

			// Zeile ersetzen
			lines[foundIndex] = newValue

			// 3. Zusammenbauen im Originalformat
			joinChar := "\n"
			if isCRLF {
				joinChar = "\r\n"
			}

			output := strings.Join(lines, joinChar)
			err = os.WriteFile(path, []byte(output), 0644)
			if err != nil {
				return valpkg.ErrorVal("Schreibfehler: " + err.Error())
			}

			return valpkg.BoolVal(true)
		})

	Register(ns+"ReadValue", "file", "path, prefix",
		"Sucht eine Zeile mit 'prefix', schneidet diesen ab und gibt den getrimmten Rest (valpkg.Value) zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("Parameter: Pfad, Präfix")
			}

			path, errVal := absPathVal(valpkg.ToString(args[0]))
			if errVal != nil {
				return *errVal
			}
			prefix := valpkg.ToString(args[1])

			input, err := os.ReadFile(path)
			if err != nil {
				return valpkg.ErrorVal("Lesefehler: " + err.Error())
			}

			// Normalisierung auf LF und Splitten
			lines := strings.Split(strings.ReplaceAll(string(input), "\r\n", "\n"), "\n")

			for _, line := range lines {
				trimmedLine := strings.TrimSpace(line)

				// Wir prüfen, ob die Zeile mit dem Präfix beginnt
				if strings.HasPrefix(trimmedLine, prefix) {
					// 1. Präfix abschneiden
					rawVal := strings.TrimPrefix(trimmedLine, prefix)
					// 2. Eventuelle Leerzeichen um den Wert herum entfernen
					finalVal := strings.TrimSpace(rawVal)

					return valpkg.StrVal(finalVal)
				}
			}

			return valpkg.StrVal("") // Nichts gefunden
		})

	Register(ns+"UniqueLines", "file", "path, caseSensitive", "Entfernt doppelte Zeilen aus einer Datei.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.UniqueLines(path, caseSensitive) benötigt 2 Argumente")
		}

		pathStr, errS := expectStr(args, 0, "file.UniqueLines(path, caseSensitive)")
		if errS != nil {
			return *errS
		}

		path, errPtr := processTargetPath(pathStr)
		if errPtr != nil {
			return *errPtr
		}

		caseSensitive := args[1].Bool

		// 1. Datei einlesen
		content, err := os.ReadFile(path)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht gelesen werden: " + err.Error())
		}

		// 2. Zeilenweise verarbeiten
		lines := strings.Split(string(content), "\n")
		seen := make(map[string]struct{})
		var result []string

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			if trimmed == "" {
				continue
			}

			compareLine := trimmed
			if !caseSensitive {
				compareLine = strings.ToLower(trimmed)
			}

			if _, exists := seen[compareLine]; !exists {
				seen[compareLine] = struct{}{}
				result = append(result, trimmed)
			}
		}

		// 3. Ergebnis zurückschreiben (oder als String zurückgeben)
		newContent := strings.Join(result, "\n")
		err = os.WriteFile(path, []byte(newContent), 0644)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Schreiben: " + err.Error())
		}

		return valpkg.BoolVal(true)
	})

	Register(ns+"GetDuplicates", "file", "path, caseSensitive", "Findet doppelte Zeilen und gibt sie als Array zurück (alphabetisch sortiert).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("file.GetDuplicates(path, caseSensitive) benötigt 2 Argumente")
		}

		pathStr, errS := expectStr(args, 0, "file.GetDuplicates(path, caseSensitive)")
		if errS != nil {
			return *errS
		}

		path, errPtr := processTargetPath(pathStr)
		if errPtr != nil {
			return *errPtr
		}

		caseSensitive := args[1].Bool

		// 1. Datei einlesen
		content, err := os.ReadFile(path)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht gelesen werden: " + err.Error())
		}

		// 2. Zeilenweise zählen
		lines := strings.Split(string(content), "\n")
		counts := make(map[string]int)

		// Erst alle vorkommen zählen
		for _, line := range lines {
			trimmed := strings.TrimRight(line, "\r")
			if trimmed == "" {
				continue
			}

			compareLine := trimmed
			if !caseSensitive {
				compareLine = strings.ToLower(trimmed)
			}
			counts[compareLine]++
		}

		// 3. Nur die Einträge sammeln, die öfter als 1x vorkommen
		dupeLines := make([]string, 0, len(counts))
		for line, count := range counts {
			if count > 1 {
				dupeLines = append(dupeLines, line)
			}
		}

		// Deterministische Reihenfolge, da Map-Iteration in Go nicht stabil ist
		sort.Strings(dupeLines)

		duplicates := make([]valpkg.Value, len(dupeLines))
		for i, line := range dupeLines {
			duplicates[i] = valpkg.StrVal(line)
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: duplicates}
	})

	Register(ns+"SecureDelete", "file", "path", "Hochsicherheits-Löschung: In-Place AES-Verschlüsselung + Zufalls-Rename + Truncate.", func(args []valpkg.Value) valpkg.Value {
		pathStr, errS := expectStr(args, 0, "file.SecureDelete(path)")
		if errS != nil {
			return *errS
		}

		path, errVal := absPathVal(pathStr)
		if errVal != nil {
			return *errVal
		}

		if err := secureDeleteFile(path); err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		return valpkg.BoolVal(true)
	})
}

func stringSliceToValueSlice(s []string) []valpkg.Value {
	arr := make([]valpkg.Value, len(s))
	for i, str := range s {
		arr[i] = valpkg.Value{Kind: valpkg.KindStr, Str: str}
	}
	return arr
}

// Hilfswriter für Base64 mit Umbrüchen
type base64LineWriter struct {
	w     io.Writer
	limit int
	count int
}

func newBase64LineWriter(w io.Writer, limit int) *base64LineWriter {
	return &base64LineWriter{w: w, limit: limit}
}

func (lw *base64LineWriter) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		remain := lw.limit - lw.count
		if remain <= 0 {
			if _, err := lw.w.Write([]byte{'\n'}); err != nil {
				return total, err
			}
			lw.count = 0
			remain = lw.limit
		}
		n := len(p)
		if n > remain {
			n = remain
		}
		if _, err := lw.w.Write(p[:n]); err != nil {
			return total, err
		}
		lw.count += n
		total += n
		p = p[n:]
	}
	return total, nil
}

func (lw *base64LineWriter) Close() error {
	if lw.count > 0 {
		if _, err := lw.w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}

// Hilfsfunktion: Datei kopieren
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Kein MkdirAll — Verzeichnis muss bereits existieren
	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		out.Close()
		return err
	}

	return out.Close()
}

// Hilfsfunktion für Kontrast-Check
func getSafeColor(fg int, bg int) string {
	// Wenn Vordergrund Gelb (3) oder Weiß (7) ist und kein Hintergrund gesetzt wurde
	// erzwingen wir einen dunklen Hintergrund oder schalten auf eine dunklere Farbe um.
	if bg == 0 {
		if fg == 3 { // Gelb auf Standard (evtl. Weiß) -> Schwer lesbar
			return "1;33" // Wir machen es wenigstens FETT Gelb, das hilft oft
		}
		if fg == 11 { // Light Yellow
			return "33" // Downgrade auf normales Dunkelgelb
		}
	}
	return ""
}

func mapUserLayoutToGo(userLayout string) string {
	r := strings.NewReplacer(
		"YYYY", "2006",
		"MM", "01",
		"DD", "02",
		"HH", "15",
		"mm", "04", // Achtung: mm für Minuten, MM für Monat
		"SS", "05",
	)
	return r.Replace(userLayout)
}

func copyAndDelete(src, dst string) error {
	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("delete after copy failed: %w", err)
	}

	return nil
}

func hashSingleFile(path string, algo string) (string, error) {
	absPath, errVal := absPathVal(path)
	if errVal != nil {
		return "", errors.New(errVal.Str)
	}

	f, err := os.Open(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var h hash.Hash
	switch algo {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha224":
		h = sha256.New224()
	case "sha256":
		h = sha256.New()
	case "sha384":
		h = sha512.New384()
	case "sha512":
		h = sha512.New()
	default:
		return "", fmt.Errorf("unbekannter Hash-Algorithmus: %s", algo)
	}

	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func init() { initFileFunctions() }
func absPathVal(raw string) (string, *valpkg.Value) {
	absolute, err := pathutils.AbsPath(raw)
	if err != nil {
		v := valpkg.ErrorVal("Systemfehler: Pfad konnte nicht aufgelöst werden: " + err.Error())
		return "", &v
	}
	return absolute, nil
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// containsWholeWord prüft, ob word in line als eigenständiges Wort vorkommt.
// Die Grenzen werden nur geprüft, wenn das Suchwort selbst an der
// jeweiligen Seite mit einem Wortzeichen beginnt bzw. endet.
func containsWholeWord(line, word string) bool {
	first, _ := utf8.DecodeRuneInString(word)
	last, _ := utf8.DecodeLastRuneInString(word)
	checkStart := isWordRune(first)
	checkEnd := isWordRune(last)

	offset := 0
	for {
		idx := strings.Index(line[offset:], word)
		if idx < 0 {
			return false
		}
		start := offset + idx
		end := start + len(word)

		okStart := true
		if checkStart && start > 0 {
			r, _ := utf8.DecodeLastRuneInString(line[:start])
			okStart = !isWordRune(r)
		}

		okEnd := true
		if checkEnd && end < len(line) {
			r, _ := utf8.DecodeRuneInString(line[end:])
			okEnd = !isWordRune(r)
		}

		if okStart && okEnd {
			return true
		}

		// Teiltreffer: direkt hinter dem Trefferanfang weitersuchen
		_, size := utf8.DecodeRuneInString(line[start:])
		offset = start + size
	}
}
