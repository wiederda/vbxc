// ------------------------
// stdlib_7z.go
// ------------------------

package stdlib

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	valpkg "vbxc/value"

	"github.com/bodgit/sevenzip"
)

// =============================================================================
// STATE: konfigurierbarer 7z-Binary-Pfad (für portable Installationen)
// =============================================================================

var (
	custom7zPathMu sync.RWMutex
	custom7zPath   string
)

// =============================================================================
// HELPER-FUNKTIONEN
// =============================================================================

// open7z öffnet ein 7z-Archiv, mit oder ohne Passwort.
func open7z(absPath, password string) (*sevenzip.ReadCloser, error) {
	if password != "" {
		return sevenzip.OpenReaderWithPassword(absPath, password)
	}
	return sevenzip.OpenReader(absPath)
}

// open7zFriendly öffnet ein 7z-Archiv wie open7z, übersetzt Fehler aber
// in eine für Skript-Autoren verständliche Meldung (statt der rohen
// internen sevenzip-Fehlerkette).
func open7zFriendly(absPath, password string) (*sevenzip.ReadCloser, error) {
	r, err := open7z(absPath, password)
	if err != nil {
		if password == "" {
			return nil, fmt.Errorf("archiv konnte nicht geöffnet werden (möglicherweise passwortgeschützt): %w", err)
		}
		return nil, fmt.Errorf("archiv konnte nicht geöffnet werden (falsches passwort oder beschädigtes archiv): %w", err)
	}
	return r, nil
}

// find7zBinary sucht das 7z-Kommandozeilentool in dieser Reihenfolge:
//  1. explizit über 7z.SetBinaryPath gesetzter Pfad
//  2. Umgebungsvariable VBX_7Z_PATH
//  3. System-PATH (7z, 7zz, 7za)
//  4. bekannte Standard-Installationsorte je Betriebssystem
func find7zBinary() (string, error) {
	custom7zPathMu.RLock()
	configured := custom7zPath
	custom7zPathMu.RUnlock()

	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
		return "", fmt.Errorf("konfigurierter 7z-Pfad '%s' existiert nicht", configured)
	}

	if envPath := os.Getenv("VBX_7Z_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	for _, name := range []string{"7z", "7zz", "7za"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}

	var candidates []string
	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			`C:\Program Files\7-Zip\7z.exe`,
			`C:\Program Files (x86)\7-Zip\7z.exe`,
		}
	default:
		candidates = []string{
			"/usr/bin/7z", "/usr/local/bin/7z",
			"/usr/bin/7zz", "/usr/local/bin/7zz",
			"/opt/homebrew/bin/7zz",
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", fmt.Errorf("7z-Binary nicht gefunden (weder im PATH noch an Standardorten) — Pfad ggf. über 7z.SetBinaryPath() setzen")
}

// extract7zTo entpackt einen geöffneten Reader nach absDest, mit Zip-Slip-Schutz.
func extract7zTo(r *sevenzip.ReadCloser, absDest string) error {
	buf := make([]byte, 1024*1024)
	destPrefix := filepath.Clean(absDest) + string(os.PathSeparator)

	for _, f := range r.File {
		targetPath := filepath.Join(absDest, f.Name)

		if !strings.HasPrefix(filepath.Clean(targetPath)+string(os.PathSeparator), destPrefix) {
			return fmt.Errorf("zip-slip erkannt: '%s' liegt außerhalb des zielordners", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, f.Mode()); err != nil {
				return fmt.Errorf("verzeichnis '%s' konnte nicht erstellt werden: %w", targetPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("elternverzeichnis für '%s' konnte nicht erstellt werden: %w", targetPath, err)
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("datei '%s' konnte nicht erstellt werden: %w", targetPath, err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("eintrag '%s' konnte nicht geöffnet werden: %w", f.Name, err)
		}

		_, copyErr := io.CopyBuffer(outFile, rc, buf)
		outFile.Close()
		rc.Close()

		if copyErr != nil {
			return fmt.Errorf("entpacken von '%s' fehlgeschlagen: %w", f.Name, copyErr)
		}
	}

	return nil
}

// =============================================================================
// REGISTRIERUNG
// =============================================================================

func initSevenZipFunctions() {
	ns := "7z."

	// ---------------------------------------------------------------------------
	// 7z.SetBinaryPath(path)  →  bool
	// ---------------------------------------------------------------------------
	Register(ns+"SetBinaryPath", "7z", "path",
		"Legt den Pfad zu einer 7z.exe/7z-Binary explizit fest (z.B. für portable Installationen).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("usage: 7z.SetBinaryPath(path)")
			}

			absP, err := absPathStrict(args[0].Str)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			if _, err := os.Stat(absP); err != nil {
				return valpkg.ErrorVal(fmt.Sprintf("datei '%s' nicht gefunden", absP))
			}

			custom7zPathMu.Lock()
			custom7zPath = absP
			custom7zPathMu.Unlock()

			return valpkg.BoolVal(true)
		})

	Register(ns+"IsValid", "7z", "archivePath [, password]",
		"Prüft, ob eine Datei ein gültiges 7z-Archiv ist (Inhalt, nicht nur Dateiendung).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("usage: 7z.IsValid(archivePath [, password])")
			}

			absP, err := absPathStrict(args[0].Str)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			password := ""
			if len(args) >= 2 {
				password = args[1].Str
			}

			r, err := open7z(absP, password)
			if err != nil {
				return valpkg.BoolVal(false)
			}
			defer r.Close()

			return valpkg.BoolVal(true)
		})

	// ---------------------------------------------------------------------------
	// 7z.Extract(archivePath, dest [, password])  →  null | error
	// ---------------------------------------------------------------------------
	Register(ns+"Extract", "7z", "archivePath, dest [, password]",
		"Entpackt ein 7z-Archiv. Nutzt zunächst die native Go-Implementierung; fällt bei Fehlern automatisch auf eine 7z-Binary zurück (System-PATH, Standardpfad oder via 7z.SetBinaryPath gesetzt) — deckt Fälle ab, die die reine Go-Bibliothek nicht unterstützt (z.B. bestimmte Solid-Block-/Coder-Kombinationen).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("usage: 7z.Extract(archivePath, dest [, password])")
			}

			absArchive, err := absPathStrict(args[0].Str)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			absDest, err := absPathStrict(args[1].Str)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			password := ""
			if len(args) >= 3 {
				password = args[2].Str
			}

			if r, err := open7z(absArchive, password); err == nil {
				extractErr := extract7zTo(r, absDest)
				r.Close()

				if extractErr == nil {
					return valpkg.NullVal()
				}
			}

			// Fallback: externes Binary (System-PATH, Standardpfad,
			// oder via 7z.SetBinaryPath gesetzter portabler Pfad)
			if err := extract7zViaBinary(absArchive, absDest, password); err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"entpacken fehlgeschlagen (go-implementierung UND fallback-binary): %s",
					err.Error(),
				))
			}

			return valpkg.NullVal()
		})

	// ---------------------------------------------------------------------------
	// 7z.List(archivePath [, password])  →  Array von Maps {Name, Size, IsDir, ModTime}
	// ---------------------------------------------------------------------------
	Register(ns+"List", "7z", "archivePath [, password]",
		"Gibt Details über den Inhalt eines 7z-Archivs zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("usage: 7z.List(archivePath [, password])")
			}

			absP, err := absPathStrict(args[0].Str)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			password := ""
			if len(args) >= 2 {
				password = args[1].Str
			}

			r, err := open7zFriendly(absP, password)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer r.Close()

			results := make([]valpkg.Value, 0, len(r.File))
			for _, f := range r.File {
				entry := map[string]valpkg.Value{
					"Name":    valpkg.StrVal(f.Name),
					"Size":    valpkg.NumVal(float64(f.FileInfo().Size())),
					"IsDir":   valpkg.BoolVal(f.FileInfo().IsDir()),
					"ModTime": valpkg.StrVal(f.FileInfo().ModTime().Format("2006-01-02 15:04:05")),
				}
				results = append(results, valpkg.Value{Kind: valpkg.KindMap, Map: entry})
			}

			return valpkg.Value{Kind: valpkg.KindArr, Arr: results}
		})

	// ---------------------------------------------------------------------------
	// 7z.ListNames(archivePath [, password])  →  Array von Strings
	// ---------------------------------------------------------------------------
	Register(ns+"ListNames", "7z", "archivePath [, password]",
		"Gibt ein Array mit allen Dateinamen im 7z-Archiv zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("usage: 7z.ListNames(archivePath [, password])")
			}

			absP, err := absPathStrict(args[0].Str)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			password := ""
			if len(args) >= 2 {
				password = args[1].Str
			}

			r, err := open7zFriendly(absP, password)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer r.Close()

			names := make([]valpkg.Value, 0, len(r.File))
			for _, f := range r.File {
				names = append(names, valpkg.StrVal(f.Name))
			}

			return valpkg.Value{Kind: valpkg.KindArr, Arr: names}
		})

	// ---------------------------------------------------------------------------
	// 7z.Create(archivePath, files... [, password])  →  bool
	// Nutzt das externe 7z-Binary (kein pure-Go-Encoder verfügbar).
	// ---------------------------------------------------------------------------
	Register(ns+"Create", "7z", "archivePath, files... [, password]",
		"Erstellt ein 7z-Archiv über eine 7z-Binary (System-PATH, Standardpfad oder via 7z.SetBinaryPath gesetzt).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("usage: 7z.Create(archivePath, files... [, password])")
			}

			bin, err := find7zBinary()
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			files, pass := extractFilesAndPass(args)
			if len(files) == 0 {
				return valpkg.ErrorVal("keine quelldateien angegeben")
			}

			absArchive, err := absPathStrict(args[0].Str)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			cmdArgs := []string{"a"}
			if pass != "" {
				cmdArgs = append(cmdArgs, "-p"+pass, "-mhe=on")
			}
			cmdArgs = append(cmdArgs, absArchive)
			cmdArgs = append(cmdArgs, files...)

			cmd := exec.Command(bin, cmdArgs...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf("7z-fehler: %v (%s)", err, strings.TrimSpace(string(out))))
			}

			return valpkg.BoolVal(true)
		})
}

// absPathStrict ist wie filepath.Abs, gibt aber einen expliziten Fehler zurück
// statt ihn stillschweigend zu verwerfen.
func absPathStrict(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("pfad '%s' konnte nicht aufgelöst werden: %w", p, err)
	}
	return abs, nil
}

// extractFilesAndPass trennt Dateipfade und optionales Passwort aus den Argumenten.
// Unterstützt zwei Aufrufkonventionen:
//   - (zipPath, []array, [pass])
//   - (zipPath, file1, file2, ..., [pass])
func extractFilesAndPass(args []valpkg.Value) ([]string, string) {
	var files []string
	pass := ""

	// Konvention A: zweites Argument ist ein Array
	if len(args) >= 2 && args[1].Kind == valpkg.KindArr {
		for _, v := range args[1].Arr {
			files = append(files, v.Str)
		}
		if len(args) >= 3 {
			pass = args[2].Str
		}
		return files, pass
	}

	// Konvention B: variadische Dateipfade, letztes Argument optional Passwort
	lastIdx := len(args) - 1
	if lastIdx > 1 && !looksLikeFilePath(args[lastIdx].Str) {
		pass = args[lastIdx].Str
		lastIdx--
	}

	for i := 1; i <= lastIdx; i++ {
		files = append(files, args[i].Str)
	}
	return files, pass
}

// looksLikeFilePath erkennt ob ein String ein Dateipfad ist.
func looksLikeFilePath(s string) bool {
	return strings.ContainsAny(s, "/\\") ||
		strings.HasPrefix(s, ".") ||
		filepath.IsAbs(s)
}

func extract7zViaBinary(absArchive, absDest, password string) error {
	bin, err := find7zBinary()
	if err != nil {
		return fmt.Errorf("kein 7z-binary verfügbar (weder im PATH noch via 7z.SetBinaryPath gesetzt): %w", err)
	}

	if err := os.MkdirAll(absDest, 0755); err != nil {
		return fmt.Errorf("zielverzeichnis konnte nicht erstellt werden: %w", err)
	}

	cmdArgs := []string{"x", "-y", "-o" + absDest}
	if password != "" {
		cmdArgs = append(cmdArgs, "-p"+password)
	}
	cmdArgs = append(cmdArgs, absArchive)

	cmd := exec.Command(bin, cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("7z-binary-fehler: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	return nil
}

func init() {
	initSevenZipFunctions()
}
