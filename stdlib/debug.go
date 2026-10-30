package stdlib

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	valpkg "vbxc/value"
)

var logFile *os.File
var currentScriptName string = "unnamed_script"
var scriptPath string
var lastLogPath string
var lastSampleTime time.Time
var lastThreadTime float64
var lastThreadSample time.Time

var lastCPUTime float64
var lastCPUSample time.Time

var allocStartBytes uint64
var allocStartCount uint64

// InitStringFunctions registriert String-Funktionen
func initDebugFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "debug."

	// Globaler Zeitstempel (oder im Interpreter-Objekt)
	var globalTimerStart = time.Now()

	// In deiner Built-in-Initialisierung
	Register(ns+"Assert", "debug", "condition, [msg]", "Prüft eine Bedingung und bricht bei 'False' mit Fehlermeldung ab.", func(args []valpkg.Value) valpkg.Value {
		if len(args) == 0 {
			return valpkg.ErrorVal("ASSERT erwartet mindestens ein Argument")
		}

		// Nutze deine neue isTruthy Funktion
		if !fileTruthy(args[0]) {
			msg := "Assertion failed"
			if len(args) > 1 {
				msg = valpkg.ToString(args[1])
			}

			// Das ErrorVal sorgt in evalFunctionCall dafür,
			// dass der Interpreter den Fehler erkennt und stoppt.
			return valpkg.ErrorVal("[ASSERT] " + msg)
		}

		// Wenn alles okay ist, geben wir True zurück (VB-Style)
		return valpkg.Value{Kind: valpkg.KindBool, Bool: true}
	})

	Register(ns+"AssertEquals", "debug", "expected, actual, [info]", "Vergleicht zwei Werte und bricht bei Ungleichheit mit Zeilennummer ab.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("AssertEquals erwartet 2 Argumente")
		}

		expected := args[0]
		actual := args[1]

		if !valpkg.ValuesAreEqual(expected, actual) {
			// JETZT nutzen wir das übergebene env!
			// Da env ein Pointer ist, sieht diese Funktion immer
			// den aktuellsten Stand von currentLine.
			msg := fmt.Sprintf("Assertion failed: Erwartet [%s], erhalten [%s]",
				valpkg.ToString(expected), valpkg.ToString(actual))

			if len(args) > 2 {
				msg += " - Info: " + valpkg.ToString(args[2])
			}

			return valpkg.ErrorVal(msg)
		}

		return valpkg.Value{Kind: valpkg.KindBool, Bool: true}
	})

	// Timer Start
	Register(ns+"TimerStart", "debug", "", "Startet einen hochauflösenden Timer für Performance-Messungen.", func(args []valpkg.Value) valpkg.Value {
		globalTimerStart = time.Now()
		return valpkg.Value{Kind: valpkg.KindBool, Bool: true} // Einfach True zurückgeben
	})

	// Timer MS
	Register(ns+"TimerMs", "debug", "", "Gibt die verstrichene Zeit seit TimerStart in Millisekunden (mit Nachkommastellen) zurück.", func(args []valpkg.Value) valpkg.Value {
		if globalTimerStart.IsZero() {
			return valpkg.Value{Kind: valpkg.KindNum, Num: 0}
		}
		// Wir nehmen Nanosekunden und teilen durch 1.000.000 für echte Komma-Millisekunden
		// ODER wir geben direkt Mikrosekunden zurück.
		ms := float64(time.Since(globalTimerStart).Nanoseconds()) / 1e6
		return valpkg.Value{Kind: valpkg.KindNum, Num: ms}
	})

	// 1. Systemweite Last (0-100% über alle Kerne)
	Register(ns+"CPUUsage", "debug", "", "Last im Gesamtsystem.", func(args []valpkg.Value) valpkg.Value {
		now := time.Now()
		total := processCPUTime()

		if lastCPUSample.IsZero() {
			lastCPUTime, lastCPUSample = total, now
			return valpkg.NumVal(0)
		}

		seconds := now.Sub(lastCPUSample).Seconds()
		// Hier teilen wir durch die Anzahl der Kerne
		percent := ((total - lastCPUTime) / seconds) * 100 / float64(runtime.NumCPU())

		lastCPUTime, lastCPUSample = total, now // Update
		return valpkg.NumVal(percent)
	})

	// 2. Thread-Last (0-100% bezogen auf EINEN Kern)
	Register(ns+"ThreadUsage", "debug", "", "Last eines Kerns.", func(args []valpkg.Value) valpkg.Value {
		now := time.Now()
		total := processCPUTime()

		if lastThreadSample.IsZero() {
			lastThreadTime, lastThreadSample = total, now
			return valpkg.NumVal(0)
		}

		seconds := now.Sub(lastThreadSample).Seconds()
		percent := ((total - lastThreadTime) / seconds) * 100

		lastThreadTime, lastThreadSample = total, now // Update
		if percent > 100 {
			percent = 100
		} // Deckeln auf 100%
		return valpkg.NumVal(percent)
	})

	// 3. Aktueller Kern-Index
	Register(ns+"CurrentCPU", "debug", "", "Gibt den Index des CPU-Kerns zurück, auf dem das Skript gerade läuft.", func(args []valpkg.Value) valpkg.Value {
		// runtime.NumCPU() gibt die Gesamtanzahl,
		// aber es gibt keine Standard-Go-Funktion für die aktuelle Core-ID.
		// Unter Windows/Linux/Unix nutzen wir einen Trick oder syscall.
		return valpkg.NumVal(float64(getCurrentCPU()))
	})

	Register(ns+"CPUCores", "debug", "", "Gibt die Anzahl der verfügbaren CPU-Kerne zurück.", func(args []valpkg.Value) valpkg.Value {
		return valpkg.NumVal(float64(runtime.NumCPU()))
	})

	Register(ns+"MemUsage", "debug", "", "Gibt den aktuell vom Skript belegten Arbeitsspeicher in Megabyte (MB) zurück.", func(args []valpkg.Value) valpkg.Value {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return valpkg.NumVal(float64(m.Alloc) / 1024 / 1024)
	})

	Register(ns+"CloseLog", "debug", "", "Schließt die aktive Log-Datei und stellt die Ausgabe wieder auf die Konsole um.", func(args []valpkg.Value) valpkg.Value {
		if logFile != nil {
			fmt.Fprintln(logFile, "--- LOG ENDE: "+time.Now().Format("15:04:05")+" ---")
			logFile.Sync() // Erzwingt das Schreiben auf die Disk
			logFile.Close()
			logFile = nil // Wichtig: Setzt den Status zurück auf "Konsole"
			fmt.Printf("\n\033[90m[INFO] Log gespeichert unter: %s\033[0m\n", lastLogPath)
		}
		return valpkg.Value{Kind: valpkg.KindBool, Bool: true}
	})

	Register(ns+"OpenLog", "debug", "", "Erstellt eine automatische Log-Datei im System-Log-Ordner und leitet 'Print' dorthin um.", func(args []valpkg.Value) valpkg.Value {
		logDir := getLogDir()

		// Zeitstempel im Format: 20260319_1933
		// Go nutzt das Referenzdatum: 2006 (Year) 01 (Month) 02 (Day) 15 (Hour) 04 (Minute)
		timestamp := time.Now().Format("20060102_1504")

		// Den Skriptnamen säubern (Dateiendung .vbs entfernen)
		scriptName := currentScriptName
		if scriptName == "unnamed_script" {
			scriptName = filepath.Base(activeScriptPath())
		}
		cleanName := strings.TrimSuffix(scriptName, filepath.Ext(scriptName))

		// Finaler Dateiname: mein_skript_20260319_1933.log
		fileName := fmt.Sprintf("%s_%s.log", cleanName, timestamp)
		fullPath := filepath.Join(logDir, fileName)

		f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return valpkg.ErrorVal("Log-Fehler: " + err.Error())
		}
		logFile = f

		lastLogPath = fullPath

		fmt.Fprintln(logFile, "--- SESSION START: "+time.Now().Format("15:04:05")+" ---")
		return valpkg.StrVal(fullPath)
	})

	Register(ns+"CleanOldLogs", "debug", "[days]", "Löscht alte Log-Dateien (Standard: 7 Tage), um Speicherplatz zu sparen.", func(args []valpkg.Value) valpkg.Value {
		logDir := getLogDir()

		// Standardwert: 7 Tage
		days := 7
		if len(args) > 0 && args[0].Kind == valpkg.KindNum {
			days = int(args[0].Num)
		}

		// Zeitgrenze berechnen
		cutoff := time.Now().AddDate(0, 0, -days)

		files, err := os.ReadDir(logDir)
		if err != nil {
			return valpkg.ErrorVal("Konnte Log-Verzeichnis nicht lesen: " + err.Error())
		}

		deletedCount := 0
		for _, file := range files {
			if file.IsDir() {
				continue
			}

			info, err := file.Info()
			if err != nil {
				continue
			}

			// Wenn die Datei älter als der Cutoff ist und auf .log endet
			if info.ModTime().Before(cutoff) && filepath.Ext(file.Name()) == ".log" {
				err := os.Remove(filepath.Join(logDir, file.Name()))
				if err == nil {
					deletedCount++
				}
			}
		}

		// Wir geben die Anzahl der gelöschten Dateien zurück
		return valpkg.Value{Kind: valpkg.KindNum, Num: float64(deletedCount)}
	})

	Register(ns+"AllocStart", "debug", "", "Startet die Messung der Heap-Allokationen.", func(args []valpkg.Value) valpkg.Value {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		allocStartBytes = m.TotalAlloc
		allocStartCount = m.Mallocs

		return valpkg.BoolVal(true)
	})

	Register(ns+"AllocBytes", "debug", "", "Gibt die seit AllocStart allokierten Bytes zurück.", func(args []valpkg.Value) valpkg.Value {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		return valpkg.NumVal(float64(m.TotalAlloc - allocStartBytes))
	})

	Register(ns+"AllocCount", "debug", "", "Gibt die seit AllocStart durchgeführten Heap-Allokationen zurück.", func(args []valpkg.Value) valpkg.Value {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		return valpkg.NumVal(float64(m.Mallocs - allocStartCount))
	})

}

func init() { initDebugFunctions() }
func getLogDir() string {
	var baseDir string
	var err error

	switch runtime.GOOS {
	case "windows":
		// C:\Users\Name\AppData\Roaming
		baseDir, err = os.UserConfigDir()
	case "darwin":
		// /Users/Name/Library/Logs
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, "Library", "Logs")
	case "linux":
		// /home/name/.cache oder /var/log (wir nehmen User-Cache/Logs)
		// XDG_CACHE_HOME ist der Standard für User-Logs unter Linux
		baseDir = os.Getenv("XDG_CACHE_HOME")
		if baseDir == "" {
			home, _ := os.UserHomeDir()
			baseDir = filepath.Join(home, ".cache")
		}
	default:
		// Fallback für andere (BSD etc.)
		baseDir, _ = os.UserHomeDir()
	}

	if err != nil || baseDir == "" {
		return "." // Letzter Ausweg: Aktueller Ordner
	}

	// Projekt-Unterordner erstellen (WICHTIG: Ersetze "MeinVBLang" durch dein Projekt-Namen)
	finalPath := filepath.Join(baseDir, "vbmini", "Logs")

	// Ordner-Struktur anlegen (0755 = Standard-Rechte)
	os.MkdirAll(finalPath, 0755)

	return finalPath
}

// In einer separaten Datei oder am Ende deiner main.go
func getCurrentCPU() int {
	// Dieser Wert ist unter Go oft schwer zu greifen, da der Scheduler
	// Goroutinen extrem schnell verschiebt.
	// Wir nutzen eine Annäherung oder rufen das System-API auf.
	return runtime.GOMAXPROCS(0) // Platzhalter: In der Realität nutzt man oft CGO oder syscalls wie sched_getcpu
}

type debugOutputWriter struct{ fallback io.Writer }

func (w debugOutputWriter) Write(p []byte) (int, error) {
	if logFile != nil {
		return logFile.Write(p)
	}
	return w.fallback.Write(p)
}

// OutputWriter routes VM output to an active debug log while preserving its
// configured destination after the log is closed.
func OutputWriter(fallback io.Writer) io.Writer { return debugOutputWriter{fallback: fallback} }

// ActiveLog liefert die aktuell offene Debug-Logdatei oder nil. Der
// Evaluator schreibt Print-Ausgaben damit wie der Interpreter direkt und
// mit Zeitstempel ins Log, die VM nutzt stattdessen OutputWriter.
func ActiveLog() *os.File { return logFile }
