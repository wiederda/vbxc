package evaluator

// Ausführungspipeline für "vbxc <datei.vb>": 1:1 nachgebaut aus
// runfile.go des Interpreters. Der Unterschied zu vbx: kein .vbc/
// Entschlüsseln (in vbxc sind alle Builtins über init() registriert) und
// es wird nirgends etwas auf die Festplatte geschrieben. "#use" eines
// unbekannten Namens lädt ein WASM-Plugin (Paket plugin).

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/wiederda/pathutils"
	"vbxc/parser"
	"vbxc/plugin"
	"vbxc/stdlib"
)

// RunFile führt eine VB-Quelldatei direkt aus. version ist die
// vbxc-Version für die Prüfung einer "#requires"-Angabe.
func RunFile(filename, version string) error {
	_, err := runFileInternal(filename, version)
	return err
}

// runFileInternal ist die gemeinsame Pipeline: Pfad absichern, Endung
// prüfen, lesen, und anschließend parsen + ausführen.
func runFileInternal(filename, version string) (finalVal Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("[SYNTAX ERROR] %v", r)
		}
	}()

	// 1. Pfad absichern (entspricht absPathVal aus der stdlib)
	absName, absErr := pathutils.AbsPath(filename)
	if absErr != nil {
		return Value{}, fmt.Errorf("[SECURITY ERROR]: %s", absErr)
	}
	filename = absName

	// STUFE 1: Endungsprüfung -- .vbb ist Bytecode und läuft über die VM,
	// die Entscheidung trifft der CLI-Teil vorab.
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".vb" {
		return Value{}, fmt.Errorf("zugriff verweigert: '%s' ist kein gültiges Format", filename)
	}

	data, readErr := os.ReadFile(filename)
	if readErr != nil {
		return Value{}, fmt.Errorf("lesefehler: %v", readErr)
	}

	if isBinary(data) {
		return Value{}, fmt.Errorf("formatfehler: '%s' enthält ungültige Binärdaten", filename)
	}

	return runParsedInternal(data, filename, filepath.Dir(filename), version)
}

// runParsedInternal ist der gemeinsame Kern: Sanity-Check (looksLikeScript
// - fängt HTML-Fehlerseiten statt Skript-Rohtext ab), #requires/#use,
// Parsen, Registrieren der Functions/Subs und Ausführung.
func runParsedInternal(data []byte, label, baseDir, version string) (finalVal Value, err error) {
	// UTF-8-BOM entfernen (Notepad speichert häufig so)
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	if reason, ok := looksLikeScript(data); !ok {
		return Value{}, fmt.Errorf("formatfehler: '%s' - %s", label, reason)
	}

	lines := strings.Split(string(data), "\n")

	// #requires zuerst prüfen, bevor das Skript geladen wird. Der Check
	// greift nur bei einer echten Versionsnummer -- Dev-Bauten tragen "dev"
	// und würden jedes #requires sonst als "zu neu" melden.
	lines, reqVer := extractRequires(lines)
	if reqVer != "" && isNumericVersion(version) && isVersionGreater(reqVer, version) {
		return Value{}, fmt.Errorf(
			"inkompatibel: Skript (v%s) erfordert eine neuere vbxc (v%s)",
			reqVer,
			version,
		)
	}

	env := NewEnvironment(nil)

	// Plugin-Bindung: "#use"-Module aus Hauptdatei und Includes sammeln.
	// Eingebaute Namespaces überspringt plugin.LoadModules selbst; für
	// den Rest wird plugins/<name>.wasm geladen (vm.Run macht dasselbe
	// für die Bytecode-Pfade, siehe Chunk.Modules).
	var useModules []string

	stmts, parseErr := parser.ParseSource(strings.Join(lines, "\n"), parser.Options{
		File:    filepath.Base(label),
		BaseDir: baseDir,
		// #use-Zeilen rausschneiden (Metadaten) und die Zeilennummern
		// erhalten; inkl. eingebundener Dateien (siehe parser/include.go).
		ExtractUse: extractUse,
		OnModules:  func(m []string) { useModules = append(useModules, m...) },
	})
	if parseErr != nil {
		return Value{}, parseErr
	}

	if len(stmts) == 0 {
		return Value{}, fmt.Errorf("formatfehler: '%s' enthält keine erkennbaren VBX-Anweisungen", label)
	}

	// Module laden, die kein eingebautes Namespace sind: als WASM-Plugin
	// unter plugins/<name>.wasm (Details siehe Package-Doku). Bereits
	// geladene Plugins überspringt plugin.Load selbst.
	if err := plugin.LoadModules(useModules); err != nil {
		return Value{}, err
	}

	registerFuncsAndSubs(stmts)

	// Ausführung: ein Go-Panic (Bug in einer Funktion) soll nicht als
	// "SYNTAX ERROR" erscheinen, sondern als interner Fehler.
	var sig Signal
	stdlib.RunWithShutdownHook(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("[INTERNAL ERROR] %v", r)
			}
		}()
		finalVal, sig = evalStatements(stmts, env)
	})
	if err != nil {
		return Value{}, err
	}

	switch sig {
	case SignalError:
		return Value{}, fmt.Errorf("[RUNTIME ERROR] %s", finalVal.Str)
	case SignalExitSub, SignalExitFunction, SignalExitLoop, SignalNone:
		return finalVal, nil
	default:
		return Value{}, fmt.Errorf("[UNKNOWN SIGNAL] %v", sig)
	}
}

// extractRequires entfernt eine #requires-Zeile aus dem Code und gibt die// geforderte vbxc-Version zurück. Die Zeile wird durch eine leere Zeile
// ersetzt, damit die ursprünglichen Zeilennummern erhalten bleiben.
//
//	#requires 2.0.0
func extractRequires(lines []string) ([]string, string) {
	var cleanLines []string
	var requiredVersion string

	for _, line := range lines {
		trim := strings.TrimSpace(line)

		if strings.HasPrefix(strings.ToLower(trim), "#requires") {
			raw := strings.TrimSpace(trim[len("#requires"):])

			if raw != "" && requiredVersion == "" {
				requiredVersion = strings.Fields(raw)[0]
			}

			// #requires nicht an Lexer/Parser weitergeben,
			// die Zeile aber leer erhalten (Zeilennummern).
			cleanLines = append(cleanLines, "")
			continue
		}

		cleanLines = append(cleanLines, line)
	}

	return cleanLines, requiredVersion
}

// extractUse entfernt #use-Zeilen aus dem Code und gibt die gefundenen
// Module zurück (für den Parser-Hook parser.Options.ExtractUse).
func extractUse(lines []string) ([]string, []string) {
	var modules []string
	var cleanLines []string

	for _, line := range lines {
		trim := strings.TrimSpace(line)

		if strings.HasPrefix(strings.ToLower(trim), "#use") {
			raw := strings.TrimSpace(trim[4:])

			if raw != "" {
				for _, m := range strings.Split(raw, ",") {
					m = strings.TrimSpace(strings.ToLower(m))
					if m != "" {
						modules = append(modules, m)
					}
				}
			}

			// #use nicht an Lexer/Parser weitergeben, die Zeile aber leer
			// erhalten (damit Zeilennummern in Fehlermeldungen stimmen)
			cleanLines = append(cleanLines, "")
			continue
		}

		cleanLines = append(cleanLines, line)
	}

	return cleanLines, modules
}

// isNumericVersion prüft, ob eine Versionsangabe mit einer Ziffer beginnt
// (z.B. "1.0.3"). Dev-Bauten tragen "dev" und sind mit allem kompatibel.
func isNumericVersion(v string) bool {
	return v != "" && v[0] >= '0' && v[0] <= '9'
}

// isVersionGreater prüft, ob testVer über baseVer liegt (Versionsvergleich
// Teil für Teil, fehlende Teile zählen als 0 ).
func isVersionGreater(testVer, baseVer string) bool {
	tParts := strings.Split(testVer, ".")
	bParts := strings.Split(baseVer, ".")

	for i := 0; i < len(tParts) && i < len(bParts); i++ {
		var tNum, bNum int
		fmt.Sscanf(tParts[i], "%d", &tNum)
		fmt.Sscanf(bParts[i], "%d", &bNum)

		if tNum > bNum {
			return true
		}
		if tNum < bNum {
			return false
		}
	}
	// Falls das Skript 1.1.1 fordert, aber das System nur 1.1 ist
	return len(tParts) > len(bParts)
}

// isBinary prüft die ersten 512 Bytes einer Datei: Ein Null-Byte (0x00)
// bedeutet mit 99,9% Sicherheit eine Binärdatei, kein Quellcode.
func isBinary(data []byte) bool {
	limit := len(data)
	if limit > 512 {
		limit = 512
	}

	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// looksLikeScript prüft einen Byte-Sample auf typische Anzeichen dafür,
// dass es sich NICHT um ein Textskript handelt (HTML-Fehlerseite,
// GitHub-/GitLab-API-JSON-Antwort, falsches Encoding). Schneller
// Vorfilter mit spezifischer Fehlermeldung vor dem Parser.
func looksLikeScript(sample []byte) (reason string, ok bool) {
	// Nur die ersten paar KB betrachten - reicht für alle Heuristiken
	const sniffLimit = 8192
	if len(sample) > sniffLimit {
		sample = sample[:sniffLimit]
	}

	// 1. Muss gültiges UTF-8 sein (reines ASCII ist eine Teilmenge davon)
	if !utf8.Valid(sample) {
		return "Datei ist kein gültiges UTF-8 - vermutlich falsches Encoding oder beschädigter Download", false
	}

	// 2. Bekannte "das ist eindeutig kein Skript"-Signaturen/Präfixe
	// TrimLeft-Schnittmenge: Leerzeichen, Tabs, CR/LF und Byte Order Mark.
	trimmed := bytes.TrimLeft(sample, " \t\r\n\ufeff")
	lowerPrefix := strings.ToLower(string(trimmed))
	if len(lowerPrefix) > 512 {
		lowerPrefix = lowerPrefix[:512]
	}

	badPrefixes := []string{
		"<!doctype",
		"<html",
		"<?xml",
		"%pdf-",
	}
	for _, p := range badPrefixes {
		if strings.HasPrefix(lowerPrefix, p) {
			return "Inhalt sieht nach HTML/XML/PDF aus, nicht nach einem Skript " +
				"(typisch bei fehlgeschlagenem Git-Raw-Download, z.B. Fehlerseite statt Rohdatei)", false
		}
	}

	// 3. Spezialfall: GitHub-/GitLab-"Contents"-API-Antwort statt Rohdatei.
	// Erkennbar am JSON-Objekt mit den typischen Metadaten-Feldern dieses
	// Endpoints (der eigentliche Skriptinhalt steckt dann Base64-kodiert im
	// "content"-Feld).
	if strings.HasPrefix(lowerPrefix, "{") {
		full := strings.ToLower(string(sample))
		if strings.Contains(full, `"encoding":"base64"`) &&
			strings.Contains(full, `"content":`) &&
			strings.Contains(full, `"download_url"`) {
			return "Inhalt ist eine GitHub/GitLab-API-JSON-Antwort (contents-Endpoint), keine Skriptdatei - " +
				"vermutlich wurde die API-URL statt der 'download_url' (Raw-URL) aus dem JSON für den Download verwendet", false
		}
	}

	return "", true
}
