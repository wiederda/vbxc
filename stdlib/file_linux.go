//go:build !windows

package stdlib

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	valpkg "vbxc/value"
)

func watchLogInternal(path, pattern, ansiSequence string, silent bool, timeoutMs int) valpkg.Value {
	var lastPos int64

	// Initiales Öffnen (Unix-Standard)
	file, err := os.Open(path)
	if err != nil {
		return valpkg.ErrorVal("Datei konnte nicht geöffnet werden: " + err.Error())
	}
	defer file.Close()

	info, _ := file.Stat()
	lastPos = info.Size()

	if !silent {
		fmt.Printf("\033[90m[WATCH] Aktiv: %s | Filter: '%s' (Unix Mode)\033[0m\n", path, pattern)
	}

	start := time.Now()

	for {
		// --- NEU: Timeout-Check (nur im silent-Modus relevant) ---
		if silent && timeoutMs > 0 && time.Since(start).Milliseconds() > int64(timeoutMs) {
			return valpkg.BoolVal(false)
		}

		currInfo, err := os.Stat(path)
		if err != nil {
			// Datei wurde ggf. gerade verschoben/gelöscht (Rotation)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		currSize := currInfo.Size()

		// ROTATION CHECK (Datei wurde geleert oder ersetzt)
		if currSize < lastPos {
			if !silent {
				fmt.Println("\033[90m[WATCH] Log-Rotation erkannt. Setze Stream zurück...\033[0m")
			}
			file.Close()
			f, err := os.Open(path)
			if err == nil {
				file = f
				lastPos = 0
			}
			continue
		}

		if currSize > lastPos {
			// Wir lesen direkt ab der letzten bekannten Position
			buffer := make([]byte, currSize-lastPos)
			_, err := file.ReadAt(buffer, lastPos)
			if err == nil {
				lines := strings.Split(string(buffer), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && strings.Contains(strings.ToLower(line), strings.ToLower(pattern)) {
						if silent {
							// --- NEU: sofort zurückgeben, kein Print ---
							return valpkg.BoolVal(true)
						}
						ts := time.Now().Format("15:04:05")
						if ansiSequence != "" {
							fmt.Printf("%s[%s] %s\033[0m\n", ansiSequence, ts, line)
						} else {
							fmt.Printf("[%s] %s\n", ts, line)
						}
					}
				}
			}
			lastPos = currSize
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func openFileShared(path string) (*os.File, error) {
	return os.Open(path)
}

func productVersion(path string) (string, error) {
	return "", errors.New("nur unter Windows verfügbar")
}
