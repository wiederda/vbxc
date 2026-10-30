// ------------------------
// service_unix.go
//go:build linux || darwin
// +build linux darwin

// ------------------------

package stdlib

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	valpkg "vbxc/value"
)

func initServiceFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "service."

	// Parameter: [server], [showDisplayNames]
	// Hinweis: [server] wird unter Unix/macOS aktuell ignoriert (lokaler Fokus)
	Register(ns+"List", "service", "[server], [showDisplayNames]", "Gibt ein Array aller Dienste (Linux) oder Prozesse (macOS) zurück.", func(args []valpkg.Value) valpkg.Value {
		var out []byte
		var err error
		showDisplayNames := false

		// Wir prüfen nur den zweiten Parameter für die Namen-Details
		if len(args) >= 2 {
			showDisplayNames = (args[1].Str == "true" || (args[1].Kind == valpkg.KindBool && args[1].Bool))
		}

		if runtime.GOOS == "linux" {
			// systemctl gibt uns Namen und Beschreibungen
			out, err = exec.Command("systemctl", "list-units", "--type=service", "--no-pager", "--no-legend").Output()
		} else { // darwin
			out, err = exec.Command("ps", "-ax", "-o", "comm=").Output()
		}

		if err != nil {
			return valpkg.Value{Str: "error: " + err.Error(), Kind: valpkg.KindStr}
		}

		lines := bytes.Split(out, []byte("\n"))
		var resultValues []valpkg.Value

		for _, l := range lines {
			lineStr := strings.TrimSpace(string(l))
			if lineStr == "" {
				continue
			}

			entry := ""
			if runtime.GOOS == "linux" {
				fields := strings.Fields(lineStr)
				if len(fields) >= 1 {
					name := fields[0]
					if showDisplayNames && len(fields) > 4 {
						// systemd: UNIT LOAD ACTIVE SUB DESCRIPTION
						description := strings.Join(fields[4:], " ")
						entry = name + ":" + description
					} else {
						entry = name
					}
				}
			} else { // macOS
				entry = lineStr // Prozesspfad/Name
			}

			if entry != "" {
				resultValues = append(resultValues, valpkg.Value{Kind: valpkg.KindStr, Str: entry})
			}
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: resultValues}
	})

	// Ergänzung für service_unix.go
	Register(ns+"Restart", "service", "name", "Startet einen Dienst neu (Linux systemd).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Str: "error: missing service name", Kind: valpkg.KindStr}
		}
		name := args[0].Str
		if runtime.GOOS != "linux" {
			return valpkg.Value{Str: "error: restart not supported on macOS", Kind: valpkg.KindStr}
		}

		cmd := exec.Command("sudo", "systemctl", "restart", name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return valpkg.Value{Str: fmt.Sprintf("error: %s", string(out)), Kind: valpkg.KindStr}
		}
		return valpkg.Value{Str: "OK", Kind: valpkg.KindStr}
	})

	// In service_unix.go ergänzen:
	Register(ns+"Status", "service", "name", "Gibt den exakten Status-String des Dienstes zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.StrVal("Unknown")
		}

		// "systemctl is-active" gibt den Status als String aus (active, inactive, failed, etc.)
		out, _ := exec.Command("systemctl", "is-active", args[0].Str).Output()
		return valpkg.StrVal(strings.TrimSpace(string(out)))
	})

	// Parameter: name
	Register(ns+"IsRunning", "service", "name", "Prüft, ob ein Dienst (Linux) oder Prozess (macOS) aktiv ist.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Num: 0, Kind: valpkg.KindNum}
		}
		name := args[0].Str
		var cmd *exec.Cmd

		if runtime.GOOS == "linux" {
			cmd = exec.Command("systemctl", "is-active", "--quiet", name)
		} else { // macOS
			cmd = exec.Command("pgrep", "-x", name)
		}

		err := cmd.Run()
		if err != nil {
			return valpkg.Value{Num: 0, Kind: valpkg.KindNum}
		}
		return valpkg.Value{Num: 1, Kind: valpkg.KindNum}
	})

	// Parameter: name
	Register(ns+"Start", "service", "name", "Startet einen Dienst (Linux systemd).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Str: "error: missing service name", Kind: valpkg.KindStr}
		}
		name := args[0].Str
		if runtime.GOOS != "linux" {
			return valpkg.Value{Str: "error: start not supported on macOS", Kind: valpkg.KindStr}
		}

		cmd := exec.Command("sudo", "systemctl", "start", name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return valpkg.Value{Str: fmt.Sprintf("error: %s", string(out)), Kind: valpkg.KindStr}
		}
		return valpkg.Value{Str: "OK", Kind: valpkg.KindStr}
	})

	// Parameter: name
	Register(ns+"Stop", "service", "name", "Stoppt einen Dienst (Linux systemd).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Str: "error: missing service name", Kind: valpkg.KindStr}
		}
		name := args[0].Str
		if runtime.GOOS != "linux" {
			return valpkg.Value{Str: "error: stop not supported on macOS", Kind: valpkg.KindStr}
		}

		cmd := exec.Command("sudo", "systemctl", "stop", name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return valpkg.Value{Str: fmt.Sprintf("error: %s", string(out)), Kind: valpkg.KindStr}
		}
		return valpkg.Value{Str: "OK", Kind: valpkg.KindStr}
	})

	// Parameter: name, type
	Register(ns+"SetStartType", "service", "name, type", "Ändert den Autostart (auto, disabled, manual).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.Value{Str: "error: missing service name or type", Kind: valpkg.KindStr}
		}
		name := args[0].Str
		mode := strings.ToLower(args[1].Str)

		if runtime.GOOS != "linux" {
			return valpkg.Value{Str: "error: SetStartType only supported on Linux", Kind: valpkg.KindStr}
		}

		action := "disable"
		if mode == "auto" || mode == "automatic" {
			action = "enable"
		}

		cmd := exec.Command("sudo", "systemctl", action, name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return valpkg.Value{Str: fmt.Sprintf("error: %s", string(out)), Kind: valpkg.KindStr}
		}
		return valpkg.Value{Str: "OK", Kind: valpkg.KindStr}
	})

	Register(ns+"Install", "service", "name, displayName, exePath, [startType]", "Installiert einen neuen systemd-Dienst (Linux).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.Value{Kind: valpkg.KindStr, Str: "error: name, displayName und exePath benötigt"}
		}
		if runtime.GOOS != "linux" {
			return valpkg.Value{Kind: valpkg.KindStr, Str: "error: Install nur unter Linux unterstützt"}
		}

		name := args[0].Str
		displayName := args[1].Str
		exePath := args[2].Str
		startType := "auto"
		if len(args) >= 4 && args[3].Str != "" {
			startType = strings.ToLower(args[3].Str)
		}

		// WantedBy je nach Starttyp
		wantedBy := "multi-user.target"
		installSection := fmt.Sprintf("[Install]\nWantedBy=%s\n", wantedBy)
		if startType == "disabled" {
			installSection = "" // kein [Install] Block = nicht aktiviert
		}

		unitContent := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
ExecStart=%s
Restart=on-failure

%s`, displayName, exePath, installSection)

		unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", name)

		// Prüfen ob Dienst bereits existiert
		if _, err := exec.Command("systemctl", "status", name).Output(); err == nil {
			return valpkg.Value{Kind: valpkg.KindStr, Str: "error: Dienst '" + name + "' existiert bereits"}
		}

		// Unit-Datei schreiben via tee (funktioniert auch ohne direkten Dateizugriff als root)
		cmd := exec.Command("sudo", "tee", unitPath)
		cmd.Stdin = strings.NewReader(unitContent)
		if out, err := cmd.CombinedOutput(); err != nil {
			return valpkg.Value{Kind: valpkg.KindStr, Str: fmt.Sprintf("error: Unit-Datei konnte nicht geschrieben werden: %s", string(out))}
		}

		// daemon-reload
		if out, err := exec.Command("sudo", "systemctl", "daemon-reload").CombinedOutput(); err != nil {
			return valpkg.Value{Kind: valpkg.KindStr, Str: fmt.Sprintf("error: daemon-reload fehlgeschlagen: %s", string(out))}
		}

		// Aktivieren falls nicht disabled
		if startType != "disabled" {
			if out, err := exec.Command("sudo", "systemctl", "enable", name).CombinedOutput(); err != nil {
				return valpkg.Value{Kind: valpkg.KindStr, Str: fmt.Sprintf("error: enable fehlgeschlagen: %s", string(out))}
			}
		}

		return valpkg.Value{Kind: valpkg.KindStr, Str: "OK"}
	})

	Register(ns+"Delete", "service", "name", "Stoppt, deaktiviert und löscht einen systemd-Dienst (Linux).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindStr, Str: "error: missing service name"}
		}
		if runtime.GOOS != "linux" {
			return valpkg.Value{Kind: valpkg.KindStr, Str: "error: Delete nur unter Linux unterstützt"}
		}

		name := args[0].Str
		unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", name)

		// 1. Stoppen
		exec.Command("sudo", "systemctl", "stop", name).Run()

		// 2. Deaktivieren
		exec.Command("sudo", "systemctl", "disable", name).Run()

		// 3. Unit-Datei löschen
		if out, err := exec.Command("sudo", "rm", "-f", unitPath).CombinedOutput(); err != nil {
			return valpkg.Value{Kind: valpkg.KindStr, Str: fmt.Sprintf("error: Unit-Datei konnte nicht gelöscht werden: %s", string(out))}
		}

		// 4. daemon-reload
		if out, err := exec.Command("sudo", "systemctl", "daemon-reload").CombinedOutput(); err != nil {
			return valpkg.Value{Kind: valpkg.KindStr, Str: fmt.Sprintf("error: daemon-reload fehlgeschlagen: %s", string(out))}
		}

		// 5. Reset-Failed um alten Status zu löschen
		exec.Command("sudo", "systemctl", "reset-failed", name).Run()

		return valpkg.Value{Kind: valpkg.KindStr, Str: "OK"}
	})
}

func init() { initServiceFunctions() }
