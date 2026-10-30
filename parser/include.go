package parser

import (
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"runtime"
	"strings"

	"vbxc/ast"
	"vbxc/lexer"
)

// includeState ersetzt die package-globalen Maps und den Mutex des
// Interpreters. Eine Instanz gilt für einen kompletten Parse-Lauf und wird
// an alle Sub-Parser weitergereicht.
//
// Der Include-Cache (Pfad -> geparste Statements) entfällt: durch
// "Include-once" (seen) kann innerhalb eines Laufs ohnehin kein Treffer
// entstehen.
type includeState struct {
	active map[string]bool // Dateien, die gerade eingelesen werden (Schleifenschutz)
	seen   map[string]bool // Dateien, die in diesem Lauf schon eingebunden wurden
}

func newIncludeState() *includeState {
	return &includeState{
		active: make(map[string]bool),
		seen:   make(map[string]bool),
	}
}

// resolveIncludePath löst einen relativen Include-Pfad zuerst relativ zur
// einbindenden Datei und dann relativ zum Arbeitsverzeichnis auf. Fehlt ein
// absoluter Zielsystem-Pfad auf dem Build-Rechner, kann VBX_INCLUDE_PATH
// einen passenden Quellordner bereitstellen.
func resolveIncludePath(baseDir, include string, includePaths []string) string {
	portableInclude := strings.ReplaceAll(include, `\`, "/")
	absoluteInclude := filepath.IsAbs(include) || strings.HasPrefix(portableInclude, "/") ||
		(len(portableInclude) >= 3 && portableInclude[1] == ':' && portableInclude[2] == '/')
	if !absoluteInclude && baseDir != "" {
		candidate := filepath.Join(baseDir, include)
		if fileExists(candidate) {
			return candidate
		}
	}
	if fileExists(include) {
		return include
	}
	if baseDir != "" && runtime.GOOS == "windows" && absoluteInclude {
		// Bei Cross-Builds stehen absolute Zielsystem-Pfade (z. B.
		// /home/updates/helper.vb) nicht unter diesem Pfad auf Windows.
		// Suche denselben Dateinamen neben dem Quellskript sowie in den
		// üblichen Include-Ordnern, ohne das Skript ändern zu müssen.
		base := slashpath.Base(portableInclude)
		for _, dir := range []string{"", "include", "includes"} {
			candidate := filepath.Join(baseDir, dir, base)
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	if len(includePaths) == 0 {
		includePaths = filepath.SplitList(os.Getenv("VBX_INCLUDE_PATH"))
	}
	normalized := strings.ReplaceAll(include, `\`, "/")
	portableRel := strings.TrimLeft(normalized, "/")
	base := slashpath.Base(normalized)
	for _, root := range includePaths {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		// Pfad unter der Quellwurzel erhalten, falls die lokale Ablage die
		// Zielverzeichnisstruktur spiegelt (oder der Pfad relativ angegeben ist).
		candidate := filepath.Join(root, filepath.FromSlash(portableRel))
		if fileExists(candidate) {
			return candidate
		}
		// Für übliche Zielpfade wie /home/updates/SendGotify.vb genügt ein
		// Include-Root, der direkt auf den lokalen Quellordner zeigt.
		candidate = filepath.Join(root, filepath.FromSlash(base))
		if fileExists(candidate) {
			return candidate
		}
	}
	return include
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (p *Parser) parseInclude() ast.Stmt {
	incTok := p.next() // INCLUDE
	loc := p.at(incTok.Line)

	pathExpr := p.parseExpr()
	pathNode, ok := pathExpr.(*ast.StringNode)
	if !ok {
		p.error("Nach 'include' wird ein Dateipfad als String erwartet.")
	}
	path := resolveIncludePath(p.baseDir, pathNode.Value, p.opts.IncludePaths)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if p.opts.StrictIncludes {
			p.error("Include-Datei '%s' nicht gefunden.", path)
		}
		// Optionales Include: Datei nicht vorhanden -> überspringen, aber sichtbar machen
		fmt.Fprintf(os.Stderr, "\033[33m![HINWEIS]: Include-Datei '%s' nicht gefunden - wird übersprungen.\033[0m\n", path)
		return &ast.MultiStmtNode{Loc: loc}
	}

	absPath, absErr := filepath.Abs(path)
	if absErr != nil {
		absPath = path
	}

	// 1. Include-Schleife: die Datei wird gerade eingelesen
	if p.inc.active[absPath] {
		p.error("Include-Schleife erkannt: '%s' bindet sich (direkt oder indirekt) selbst ein.", path)
	}
	// 2. Include-once: in diesem Lauf schon eingebunden -> nichts mehr tun
	if p.inc.seen[absPath] {
		return &ast.MultiStmtNode{Loc: loc}
	}
	p.inc.seen[absPath] = true
	p.inc.active[absPath] = true
	defer delete(p.inc.active, absPath)

	content, err := os.ReadFile(path)
	if err != nil {
		p.error("Konnte Datei '%s' nicht öffnen: %v", path, err)
	}

	// UTF-8-BOM entfernen (Notepad speichert häufig so)
	text := strings.TrimPrefix(string(content), "\uFEFF")

	lines := strings.Split(text, "\n")
	if p.opts.ExtractUse != nil {
		var modules []string
		lines, modules = p.opts.ExtractUse(lines)
		if len(modules) > 0 && p.opts.OnModules != nil {
			p.opts.OnModules(modules)
		}
	}

	// Gleicher Parser-Kontext, relative Includes ab dem Ordner der Include-Datei.
	// Positionen im Include tragen den Dateinamen.
	sub := &Parser{
		tokens:    lexer.Tokenize(strings.Join(lines, "\n")),
		loopDepth: p.loopDepth,
		file:      path,
		baseDir:   filepath.Dir(absPath),
		opts:      p.opts,
		inc:       p.inc,
	}

	// Fehler aus der Include-Datei mit Dateinamen weitergeben
	stmts := func() []ast.Stmt {
		defer func() {
			if r := recover(); r != nil {
				if msg, ok := r.(string); ok {
					p.error("In inkludierter Datei '%s': %s", path, msg)
				}
				panic(r)
			}
		}()
		return sub.parse()
	}()

	return &ast.MultiStmtNode{Loc: loc, Stmts: stmts}
}
