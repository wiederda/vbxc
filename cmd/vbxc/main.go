// vbxc: Compiler und Entwicklungs-CLI.
//
//	vbxc datei.vb                       Skript direkt ausführen
//	vbxc build windows|linux datei.vb [-o datei.vbb] Maschinencode-Datei schreiben
//	vbxc dis    datei.vb|datei.vbb      Datei lesbar ausgeben
//	vbxc ast    datei.vb                AST mit Zeilennummern
//	vbxc tokens datei.vb                Tokens mit Zeilennummern
//
// Ohne Befehl läuft die Datei direkt – es wird nichts auf die Festplatte
// geschrieben. Eine Maschinencode-Datei entsteht nur mit "build".
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vbxc/ast"
	"vbxc/bytecode"
	"vbxc/compiler"
	"vbxc/evaluator"
	"vbxc/jit"
	"vbxc/lexer"
	"vbxc/parser"
	"vbxc/plugin"
	"vbxc/stdlib"
)

// Version wird beim Build per -ldflags "-X main.Version=..." gesetzt.
var Version = "dev"

// blankDirectives ist nur ein Platzhalter für die Entwicklung: Zeilen, die
// mit '#' beginnen (#use, #requires), werden geleert, damit der Lexer nicht
// stolpert und die Zeilennummern erhalten bleiben. Der echte Ersatz ist
// ExtractUse aus dem Interpreter.
func blankDirectives(lines []string) ([]string, []string) {
	var modules []string
	out := make([]string, len(lines))
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "#") {
			if rest, ok := strings.CutPrefix(strings.ToLower(t), "#use"); ok {
				for _, module := range strings.Split(rest, ",") {
					module = strings.Trim(strings.TrimSpace(module), `"'`)
					if module != "" {
						modules = append(modules, module)
					}
				}
			}
			continue // Zeile bleibt leer
		}
		out[i] = l
	}
	return out, modules
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, `Aufruf:
  vbxc <datei.vb>                                  Skript direkt ausführen
  vbxc build  windows|linux <datei.vb> [-o <datei.vbb>]
                                      erzeugt eine Maschinencode-Datei (.vbb)
  vbxc -dryrun <datei.vb>             Skript prüfen, ohne es auszuführen
  vbxc -shell -h                      Shell-Shortcuts anzeigen
  vbxc dis    <datei.vb | datei`+bytecode.Extension+`>
  vbxc ast    <datei.vb>
  vbxc tokens <datei.vb>
  vbxc help [namespace]             Hilfe und Built-ins anzeigen
  vbxc -h [namespace]               Hilfe und Built-ins anzeigen
  vbxc -v                           Version anzeigen`)
	os.Exit(2)
}

func printHelp(filter string, showConstants bool) {
	const (
		bold    = "\033[1m"
		green   = "\033[32m"
		cyan    = "\033[36m"
		magenta = "\033[1;35m"
		white   = "\033[37m"
		reset   = "\033[0m"
	)
	builtins := stdlib.Builtins()
	filter = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(filter)), "-")
	if showConstants || filter == "const" {
		printConstantsHelp(builtins)
		return
	}

	counts := make(map[string]int)
	for _, info := range builtins {
		module := strings.ToLower(info.Module)
		if module == "" {
			module = "global"
		}
		counts[module]++
	}
	standard := map[string]bool{"app": true, "array": true, "date": true, "file": true, "folder": true, "global": true, "math": true}
	if filter == "" {
		standardCount, additionalCount := 0, 0
		for name, count := range counts {
			if standard[name] {
				standardCount += count
			} else {
				additionalCount += count
			}
		}
		fmt.Printf("Version: %s%s%s | Funktionen: %sStandard %d%s / %sWeitere %d%s / %sGesamt %d%s\n",
			bold, Version, reset, green, standardCount, reset, cyan, additionalCount, reset, magenta, len(builtins), reset)
		fmt.Println("\n" + bold + "Kurz-Übersicht:" + reset)
		fmt.Println("  vbxc <quelle.vb>                  Skript direkt ausführen")
		fmt.Println("  vbxc build windows <quelle.vb> [-o <ziel.vbb>]  Maschinencode für Windows erzeugen")
		fmt.Println("  vbxc build linux <quelle.vb> [-o <ziel.vbb>]    Maschinencode für Linux erzeugen")
		fmt.Println("  vbxc -dryrun <quelle.vb>          Skript prüfen, ohne es auszuführen")
		fmt.Println("  vbxc -shell -h                    Shell-Shortcuts anzeigen")
		fmt.Println("  vbxc -h                           Diese Übersicht")
		fmt.Println("  vbxc -modules=file -h             Hilfe zu einem Modul anzeigen")
		fmt.Println("  vbxc -const -h                    Verfügbare Konstanten")
		fmt.Println("  vbxc -v                           Versionsnummer")
		fmt.Println("\nInclude-Suche: VBX_INCLUDE_PATH ergänzt lokale Quellordner für Includes.")
		fmt.Println("Unter Windows mehrere Ordner mit Semikolon trennen.")
		fmt.Println("\n" + bold + "Module/Funktionsgruppen (Standard):" + reset)
		for _, name := range []string{"app", "array", "date", "file", "folder", "global", "math"} {
			if count := counts[name]; count > 0 {
				fmt.Printf("  %s%-12s%s (%s%2d Funktionen%s)\n", green, name, reset, white, count, reset)
			}
		}
		fmt.Println("\n" + bold + "Weitere Module:" + reset)
		modules := make([]string, 0, len(counts))
		for name := range counts {
			if !standard[name] {
				modules = append(modules, name)
			}
		}
		sort.Strings(modules)
		for _, name := range modules {
			fmt.Printf("  %s%-12s%s (%s%2d Funktionen%s)\n", cyan, name, reset, white, counts[name], reset)
		}
		//fmt.Println("\nDetails: vbxc help <modul>  |  vbxc -modules=<modul> -h")
		return
	}

	names := make([]string, 0)
	for name, info := range builtins {
		module := strings.ToLower(info.Module)
		lowName := strings.ToLower(name)
		match := module == filter || lowName == filter || strings.HasPrefix(lowName, filter+".")
		if filter == "global" {
			match = module == "global" || module == ""
		}
		if match {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		if filter != "" {
			fmt.Fprintf(os.Stderr, "Keine Funktionen für %q gefunden.\n", filter)
		}
		return
	}
	fmt.Printf("%sAktivierter Filter: %s%s\n", bold, cyan, filter+reset)
	for _, name := range names {
		info := builtins[name]
		params := ""
		if info.Params != "" {
			params = "(" + info.Params + ")"
		}
		deprecated := ""
		if info.Deprecated != "" {
			deprecated = "[veraltet: " + info.Deprecated + "] "
		}
		fmt.Printf("  %s%-34s%s %s%s\n", green, name+params, reset, deprecated, info.Beschreibung)
	}
}

func printConstantsHelp(builtins map[string]stdlib.BuiltinInfo) {
	const (
		bold  = "\033[1m"
		green = "\033[32m"
		cyan  = "\033[36m"
		reset = "\033[0m"
	)
	names := make([]string, 0)
	for name, info := range builtins {
		if strings.EqualFold(info.Module, "const") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	fmt.Printf("\n%sVerfügbare Konstanten%s\n", bold, reset)
	fmt.Println(strings.Repeat("-", 72))
	fmt.Printf("%s%-30s%s | %sBeschreibung%s\n", cyan, "Konstante", reset, cyan, reset)
	fmt.Println(strings.Repeat("-", 72))
	for _, name := range names {
		fmt.Printf("%s%-30s%s | %s\n", green, name+"()", reset, builtins[name].Beschreibung)
	}
	fmt.Println(strings.Repeat("-", 72))
}

func readSource(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		die(err)
	}
	return strings.TrimPrefix(string(data), "\uFEFF")
}

func parseOptions(path string, modules *[]string) parser.Options {
	return parser.Options{
		File:           filepath.Base(path),
		BaseDir:        filepath.Dir(path),
		StrictIncludes: true,
		ExtractUse:     blankDirectives,
		OnModules:      func(m []string) { *modules = append(*modules, m...) },
	}
}

func parseFile(path string) []ast.Stmt {
	var modules []string
	stmts, err := parser.ParseSource(readSource(path), parseOptions(path, &modules))
	if err != nil {
		die(err)
	}
	return stmts
}

func compileFile(path string) *bytecode.Chunk {
	var modules []string
	stmts, err := parser.ParseSource(readSource(path), parseOptions(path, &modules))
	if err != nil {
		die(err)
	}
	chunk, err := compiler.Compile(filepath.Base(path), stmts)
	if err != nil {
		die(err)
	}
	chunk.Modules = modules
	return chunk
}

// loadRunChunk liest eine .vbb-Datei (Bytecode oder Maschinencode).
// Maschinencode-Dateien liefern ein lauffähiges JIT-Programm mit.
func loadRunChunk(path string) (*bytecode.Chunk, *jit.Program) {
	f, err := os.Open(path)
	if err != nil {
		die(err)
	}
	defer f.Close()
	c, prog, err := jit.LoadRunFile(f)
	if err != nil {
		die(fmt.Errorf("%s: %w", path, err))
	}
	return c, prog
}

// loadChunk liest eine .vbb-Datei (Bytecode oder Maschinencode) oder
// kompiliert Quelltext. Maschinencode-Dateien liefern ein lauffähiges
// JIT-Programm mit.
func loadChunk(path string) (*bytecode.Chunk, *jit.Program) {
	if strings.EqualFold(filepath.Ext(path), bytecode.Extension) {
		return loadRunChunk(path)
	}
	return compileFile(path), nil
}

func build(args []string) {
	if len(args) != 2 && len(args) != 4 {
		usage()
	}
	target := strings.ToLower(strings.TrimSpace(args[0]))
	in := args[1]
	out := strings.TrimSuffix(in, filepath.Ext(in)) + bytecode.Extension
	if len(args) == 4 {
		if args[2] != "-o" || strings.TrimSpace(args[3]) == "" {
			usage()
		}
		out = args[3]
	}
	if target != "windows" && target != "linux" {
		die(fmt.Errorf("Ziel-OS %q nicht unterstützt; erwartet: windows oder linux", args[0]))
	}
	inAbs, err := filepath.Abs(in)
	if err != nil {
		die(fmt.Errorf("Quelldatei %q: %w", in, err))
	}
	outAbs, err := filepath.Abs(out)
	if err != nil {
		die(fmt.Errorf("Zieldatei %q: %w", out, err))
	}
	samePath := inAbs == outAbs
	if filepath.VolumeName(inAbs) != "" {
		samePath = strings.EqualFold(inAbs, outAbs)
	}
	if samePath {
		die(fmt.Errorf("Zieldatei darf die Quelldatei nicht überschreiben: %s", in))
	}
	chunk := compileFile(in)

	// Nicht-blockierende Plugin-Prüfung: Auf der Build-Maschine fehlende
	// Plugin-WASM-Dateien sind hier kein Fehler (Build- und
	// Ausführmaschine können unterschiedliche Plugin-Verzeichnisse
	// haben), aber der Hinweis verhindert späteres Rätselraten.
	if missing := plugin.FindMissing(chunk.Modules); len(missing) > 0 {
		fmt.Fprintf(os.Stderr,
			"Hinweis: %s benötigt WASM-Plugins, die hier nicht gefunden wurden: %s\n",
			in, strings.Join(missing, ", "))
	}

	f, err := os.Create(out)
	if err != nil {
		die(err)
	}

	stats, _, err := jit.WriteMachineFile(f, target, "amd64", chunk)
	if err != nil {
		f.Close()
		os.Remove(out)
		die(err)
	}
	if err := f.Close(); err != nil {
		die(err)
	}
	fmt.Printf("%s: %d Regionen, %d Einstiege, %d Bytes Maschinencode (%d Instruktionen VM-Rückfall) -> %s [%s/amd64]\n",
		in, stats.Regions, stats.Entries, stats.CodeBytes, stats.Skipped, out, target)
}

// runScript führt eine .vb-Datei direkt aus – dabei wird nichts auf die
// Festplatte geschrieben. Maschinencode-Dateien führt vbxr aus.
func runScript(path string) {
	if strings.EqualFold(filepath.Ext(path), bytecode.Extension) {
		die(fmt.Errorf("vbxc führt erzeugte Maschinencode-Dateien nicht aus; bitte mit vbxr starten: %s", path))
	}
	if _, err := os.Stat(path); err != nil && filepath.Ext(path) == "" {
		// Weder vorhanden noch mit Endung: eher ein vertippter Befehl.
		fmt.Fprintf(os.Stderr, "Unbekannter Befehl: %s\n\n", path)
		usage()
	}

	if err := evaluator.RunFile(path, Version); err != nil {
		die(err)
	}
}

func main() {
	if len(os.Args) == 1 {
		printHelp("", false)
		return
	}
	cmd := os.Args[1]
	if cmd == "-dryrun" {
		if len(os.Args) != 3 {
			usage()
		}
		warnings, err := dryRun(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n[DRYRUN FEHLER] %v\n", err)
			os.Exit(1)
		}
		if warnings > 0 {
			fmt.Printf("[DRYRUN] %d Warnung(en) gefunden; das Skript kann zur Laufzeit fehlschlagen.\n", warnings)
			os.Exit(1)
		}
		fmt.Println("[DRYRUN OK] Skript ist syntaktisch gültig.")
		return
	}
	if cmd == "-shell" {
		filter := ""
		for _, arg := range os.Args[2:] {
			if arg != "-h" && arg != "--help" {
				filter = strings.TrimPrefix(arg, "-")
			}
		}
		printShellHelp(filter)
		return
	}
	if strings.HasPrefix(strings.ToLower(cmd), "-modules=") {
		var requestedModules, positional []string
		showHelp := false
		for _, arg := range os.Args[1:] {
			switch {
			case strings.HasPrefix(strings.ToLower(arg), "-modules="):
				requestedModules = append(requestedModules, strings.Split(arg[len("-modules="):], ",")...)
			case arg == "-h" || arg == "--help" || arg == "/?":
				showHelp = true
			default:
				positional = append(positional, arg)
			}
		}
		if err := plugin.LoadModules(requestedModules); err != nil {
			die(fmt.Errorf("Modul(e) %q konnten nicht geladen werden: %w", strings.Join(requestedModules, ","), err))
		}
		if showHelp || len(positional) == 0 {
			filter := ""
			if len(requestedModules) == 1 {
				filter = strings.TrimSpace(requestedModules[0])
			}
			printHelp(filter, false)
			return
		}
		if used, err := executeShortcut(strings.TrimSuffix(positional[0], "()"), positional[1:]); used {
			if err != nil {
				die(err)
			}
			return
		}
		if len(positional) != 1 {
			usage()
		}
		runScript(positional[0])
		return
	}
	if cmd == "help" || cmd == "-h" || cmd == "--help" || cmd == "/?" || cmd == "-const" || (strings.HasPrefix(cmd, "-") && cmd != "-" && cmd != "-v" && cmd != "--version") {
		filter := ""
		showConstants := cmd == "-const"
		if strings.HasPrefix(cmd, "-") && cmd != "-h" && cmd != "--help" && cmd != "/?" && cmd != "-const" {
			filter = strings.TrimPrefix(cmd, "-")
		}
		for _, arg := range os.Args[2:] {
			switch {
			case arg == "-const":
				showConstants = true
			case arg == "-h" || arg == "--help" || arg == "/?" || arg == "help":
			case strings.HasPrefix(arg, "-"):
				filter = strings.TrimPrefix(arg, "-")
			default:
				filter = arg
			}
		}
		printHelp(filter, showConstants)
		return
	}
	if cmd == "-v" || cmd == "--version" {
		fmt.Println("vbxc " + Version)
		return
	}
	if cmd == "run" {
		fmt.Fprintln(os.Stderr, "Der Befehl 'run' gibt es nicht mehr: vbxc <datei.vb> führt das Skript direkt aus.")
		usage()
	}
	args := os.Args[2:]

	switch cmd {
	case "build":
		build(args)

	case "dis":
		if len(args) != 1 {
			usage()
		}
		c, prog := loadChunk(args[0])
		if prog != nil {
			s := prog.Stats
			fmt.Printf("Maschinencode-Datei: %d Regionen, %d Einstiege, %d Bytes Code\n\n",
				s.Regions, s.Entries, s.CodeBytes)
		}
		bytecode.Disassemble(os.Stdout, c)

	case "ast":
		if len(args) != 1 {
			usage()
		}
		for _, s := range parseFile(args[0]) {
			ast.Dump(os.Stdout, s)
		}

	case "tokens":
		if len(args) != 1 {
			usage()
		}
		lines, _ := blankDirectives(strings.Split(readSource(args[0]), "\n"))
		for _, t := range lexer.Tokenize(strings.Join(lines, "\n")) {
			fmt.Printf("%4d  %-10v %q\n", t.Line, t.Type, t.Value)
		}

	default:
		// Kein Befehl: die Datei selbst ist das Argument.
		if len(os.Args) != 2 {
			if used, err := executeShortcut(strings.TrimSuffix(cmd, "()"), os.Args[2:]); used {
				if err != nil {
					fmt.Fprintf(os.Stderr, "\033[31m[SHORTCUT FEHLER]\033[0m %v\n", err)
					os.Exit(1)
				}
				return
			}
			usage()
		}
		if used, err := executeShortcut(strings.TrimSuffix(cmd, "()"), nil); used {
			if err != nil {
				fmt.Fprintf(os.Stderr, "\033[31m[SHORTCUT FEHLER]\033[0m %v\n", err)
				os.Exit(1)
			}
			return
		}
		runScript(cmd)
	}
}
