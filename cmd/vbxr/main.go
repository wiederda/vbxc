// vbxr lädt und führt vorab erzeugte VBX-Maschinencode-Dateien aus.
package main

import (
	"fmt"
	"os"
	"strings"

	"vbxc/bytecode"
	"vbxc/jit"
	"vbxc/vm"
)

// Version wird beim Build per -ldflags "-X main.Version=..." gesetzt.
var Version = "dev"

func help() string {
	return fmt.Sprintf(`vbxr %s — Runtime für VBX-Dateien

Aufruf:
  vbxr <datei.vbb>            Maschinencode-Datei ausführen
  vbxr --jit-stats <datei.vbb> Lade-Statistik auf stderr ausgeben
  vbxr -h            diese Hilfe anzeigen
  vbxr -v            Version anzeigen`, Version)
}

func main() {
	var (
		jitStats bool
		path     string
	)

	for _, a := range os.Args[1:] {
		switch {
		case a == "-h" || a == "--help" || a == "help" || a == "/?":
			fmt.Println(help())
			return
		case a == "-v" || a == "--version" || a == "version":
			fmt.Println("vbxr " + Version)
			return
		case a == "--jit-stats":
			jitStats = true
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(os.Stderr, "unbekanntes Argument: %s\n\n%s\n", a, help())
			os.Exit(2)
		default:
			if path != "" {
				fmt.Fprintf(os.Stderr, "nur eine Datei erwartet, zweite: %s\n", a)
				os.Exit(2)
			}
			path = a
		}
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, help())
		os.Exit(2)
	}

	chunk, fileProg, err := load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if jitStats {
		s := fileProg.Stats
		fmt.Fprintf(os.Stderr, "[Maschinencode] %s: %d Regionen, %d Einstiege, %d Bytes Code, %d VM-Rückfälle\n",
			"Maschinencode aus Datei", s.Regions, s.Entries, s.CodeBytes, s.Skipped)
	}

	v := vm.New()
	v.SetJIT(fileProg)
	if err := v.Run(chunk); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func load(path string) (*bytecode.Chunk, *jit.Program, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	chunk, prog, err := jit.LoadRunFile(f)
	if err != nil {
		return nil, nil, err
	}
	if prog == nil {
		return nil, nil, fmt.Errorf("Datei enthält keinen vorab erzeugten Maschinencode")
	}
	return chunk, prog, nil
}
