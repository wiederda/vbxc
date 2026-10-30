package stdlib

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	valpkg "vbxc/value"

	"github.com/shirou/gopsutil/v3/process"
)

func initProcFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "proc."

	// Prozesse auflisten
	Register(ns+"Tasklist", "proc", "", "Gibt eine Liste aller laufenden Prozesse als Text zurück.", func(args []valpkg.Value) valpkg.Value {
		procs, err := process.Processes()
		if err != nil {
			return valpkg.Value{Str: "error: " + err.Error()}
		}

		var lines []string
		for _, p := range procs {
			pid := p.Pid
			ppid, _ := p.Ppid()
			name, _ := p.Name()
			exe, _ := p.Exe()
			cmdline, _ := p.Cmdline()
			lines = append(lines, fmt.Sprintf("PID=%d PPID=%d NAME=%s PATH=%s ARGS=%s",
				pid, ppid, name, exe, cmdline))
		}

		return valpkg.Value{Str: strings.Join(lines, "\n")}
	})

	// Prozess beenden per PID
	Register(ns+"Kill", "proc", "pid", "Beendet eine PID (geschützte Systemprozesse ausgenommen).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("PID fehlt")
		}
		pid := int32(args[0].Num)

		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.ErrorVal("Prozess nicht gefunden")
		}

		if isProtected(p) {
			return valpkg.ErrorVal("Zugriff verweigert: Systemprozess oder Eigenprozess")
		}

		if err := p.Kill(); err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		return valpkg.BoolVal(true)
	})

	// --- KillTree per PID ---
	Register(ns+"KillTree", "proc", "pid", "Beendet Prozessbaum (Systemprozesse geschützt).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("PID fehlt")
		}
		pid := int32(args[0].Num)

		// killTree übernimmt jetzt den isProtected-Check intern
		err := killTree(pid)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		return valpkg.BoolVal(true)
	})

	// Prozess(e) beenden per Pfad
	// proc.KillByPath(path, containsMode)
	Register(ns+"KillByPath", "proc", "path, [mode]", "Beendet Prozesse per Pfad (System geschützt). mode=1: Teilsuche.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("Pfad fehlt")
		}

		target := args[0].Str
		contains := (len(args) > 1 && args[1].Num == 1)

		// Pfad für exakte Vergleiche normieren
		if !contains {
			if abs, err := absPathVal(target); err == nil {
				target = abs
			}
		}

		procs, _ := process.Processes()
		killedCount := 0

		for _, p := range procs {
			exe, err := p.Exe()
			if err != nil || exe == "" {
				continue
			}

			if pathMatch(exe, target, contains) {
				if !isProtected(p) {
					if err := p.Kill(); err == nil {
						killedCount++
					}
				}
			}
		}

		return valpkg.NumVal(float64(killedCount))
	})

	Register(ns+"CountByPath", "proc", "path, [mode]", "Zählt Prozesse per Pfad. mode=1: Teilsuche.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("Pfad fehlt")
		}

		target := args[0].Str
		contains := (len(args) > 1 && valpkg.IsTruthy(args[1]))

		if !contains {
			if abs, err := absPathVal(target); err == nil {
				target = abs
			}
		}

		procs, _ := process.Processes()
		count := 0

		for _, p := range procs {
			exe, err := p.Exe()
			if err != nil || exe == "" {
				continue
			}

			if pathMatch(exe, target, contains) {
				count++
			}
		}

		return valpkg.NumVal(float64(count))
	})

	Register(ns+"IsScriptRunning", "proc", "",
		"Prüft automatisch, ob das aktuell ausgeführte Skript bereits in einer anderen Instanz läuft (Vergleich über den absoluten Skriptpfad in der Kommandozeile).",
		func(args []valpkg.Value) valpkg.Value {
			var scriptArg string
			for _, a := range os.Args[1:] {
				lower := strings.ToLower(a)
				if strings.HasSuffix(lower, ".vb") || strings.HasSuffix(lower, ".vbb") {
					scriptArg = a
					break
				}
			}

			if scriptArg == "" {
				return valpkg.BoolVal(false)
			}

			absScript, err := filepath.Abs(scriptArg)
			if err != nil {
				return valpkg.BoolVal(false)
			}

			selfPid := int32(os.Getpid())

			procs, err := process.Processes()
			if err != nil {
				return valpkg.BoolVal(false)
			}

			for _, p := range procs {
				if p.Pid == selfPid {
					continue
				}
				cmdline, err := p.Cmdline()
				if err != nil {
					continue
				}
				if strings.Contains(cmdline, absScript) {
					return valpkg.BoolVal(true)
				}
			}

			return valpkg.BoolVal(false)
		})

	Register(ns+"KillByName", "proc", "name, [mode]", "Beendet Prozesse per Name. mode=1: Teilsuche.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("Name fehlt")
		}

		target := strings.ToLower(valpkg.ToString(args[0]))
		contains := (len(args) > 1 && valpkg.IsTruthy(args[1]))

		procs, _ := process.Processes()
		killedCount := 0

		for _, p := range procs {
			name, err := p.Name()
			if err != nil || name == "" {
				continue
			}

			if nameMatch(name, target, contains) {
				if !isProtected(p) {
					if err := p.Kill(); err == nil {
						killedCount++
					}
				}
			}
		}

		return valpkg.NumVal(float64(killedCount))
	})

	// Prozessbaum beenden per Pfad
	Register(ns+"KillTreeByPath", "proc", "path, [contains]", "Beendet Prozessbäume via Pfad (Systemprozesse geschützt).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("Pfad fehlt")
		}

		target := args[0].Str
		contains := (len(args) > 1 && args[1].Num == 1)

		// Pfad für exakte Vergleiche normieren
		if !contains {
			if abs, err := absPathVal(target); err == nil {
				target = abs
			}
		}

		procs, err := process.Processes()
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		killedCount := 0
		for _, p := range procs {
			exe, err := p.Exe()
			if err != nil || exe == "" {
				continue // Kernel-Threads/Systemprozesse ohne Pfad ignorieren
			}

			// Plattform-Check (Windows ignoriert Case)
			if pathMatch(exe, target, contains) {
				if !isProtected(p) {
					if err := killTree(p.Pid); err == nil {
						killedCount++
					}
				}
			}
		}

		return valpkg.NumVal(float64(killedCount))
	})

	// Prozess existiert per PID
	Register(ns+"PidExists", "proc", "pid", "Prüft, ob eine PID aktiv ist (true/false).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindBool, Bool: false}
		}

		pid := int32(args[0].Num)
		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindBool, Bool: false}
		}

		running, err := p.IsRunning()
		if err != nil || !running {
			return valpkg.Value{Kind: valpkg.KindBool, Bool: false}
		}

		return valpkg.Value{Kind: valpkg.KindBool, Bool: true}
	})

	Register(ns+"GetPids", "proc", "", "Gibt ein Array mit allen laufenden PIDs zurück.", func(args []valpkg.Value) valpkg.Value {
		pids, err := process.Pids()
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		var result []valpkg.Value
		for _, pid := range pids {
			result = append(result, valpkg.NumVal(float64(pid)))
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	Register(ns+"GetChildPids", "proc", "pid", "Gibt ein Array mit den PIDs aller direkten Kindprozesse zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		pid := int32(args[0].Num)
		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		children, err := p.Children()
		if err != nil {
			// Keine Kinder gefunden oder Zugriff verweigert
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		var result []valpkg.Value
		for _, child := range children {
			result = append(result, valpkg.NumVal(float64(child.Pid)))
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	Register(ns+"ExistsByPath", "proc", "path", "Prüft, ob ein Programm unter diesem Pfad läuft (OS-unabhängig).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.BoolVal(false)
		}

		// Pfad bereinigen (entfernt doppelte Slashes, wandelt relative in absolute Pfade um)
		targetPath := args[0].Str
		if abs, err := absPathVal(targetPath); err == nil {
			targetPath = abs
		}

		procs, err := process.Processes()
		if err != nil {
			return valpkg.BoolVal(false)
		}

		for _, p := range procs {
			exe, err := p.Exe()
			if err != nil {
				continue
			}

			// --- Plattform-Logik ---
			if pathMatch(exe, targetPath, false) {
				return valpkg.BoolVal(true)
			}
		}

		return valpkg.BoolVal(false)
	})

	Register(ns+"ExistsByName", "proc", "name", "Prüft, ob ein Prozessname (z.B. 'init' oder 'System') aktiv ist.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.BoolVal(false)
		}
		target := strings.ToLower(args[0].Str)
		procs, _ := process.Processes()
		for _, p := range procs {
			n, _ := p.Name()
			if nameMatch(n, target, false) {
				return valpkg.BoolVal(true)
			}
		}
		return valpkg.BoolVal(false)
	})

	Register(ns+"Start", "proc", "path, [args...]", "Startet ein Programm im Hintergrund und gibt die neue PID zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("missing path")
		}
		path := args[0].Str

		// Nutze deine absPathVal Logik, um sicherzugehen, dass wir die richtige Datei finden
		// falls es kein System-Befehl (wie 'ls') sondern eine lokale Datei ist.
		if _, err := os.Stat(path); err == nil {
			if abs, errVal := absPathVal(path); errVal == nil {
				path = abs
			}
		}

		var cmdArgs []string
		for i := 1; i < len(args); i++ {
			cmdArgs = append(cmdArgs, args[i].Str)
		}

		cmd := exec.Command(path, cmdArgs...)
		if err := cmd.Start(); err != nil {
			return valpkg.Value{Str: "error: " + err.Error()}
		}

		return valpkg.Value{Kind: valpkg.KindNum, Num: float64(cmd.Process.Pid)}
	})

	// Arbeitsverzeichnis des aktuellen VBX-Prozesses ändern
	Register(ns+"SetCurrentDirectory", "proc", "path", "Setzt das aktuelle Arbeitsverzeichnis des VBX-Prozesses.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("SetCurrentDirectory: Pfad fehlt")
		}

		path := valpkg.ToString(args[0])
		if path == "" {
			return valpkg.ErrorVal("SetCurrentDirectory: Pfad ist leer")
		}

		if err := os.Chdir(path); err != nil {
			return valpkg.ErrorVal("SetCurrentDirectory: " + err.Error())
		}

		return valpkg.BoolVal(true)
	})

	Register(ns+"Exec", "proc", "cmd, [args...]", "Führt Befehl aus und wartet. Output geht direkt an die Konsole.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.BoolVal(false)
		}

		cmdName := valpkg.ToString(args[0])
		var cmdArgs []string
		for i := 1; i < len(args); i++ {
			cmdArgs = append(cmdArgs, valpkg.ToString(args[i]))
		}

		cmd := exec.Command(cmdName, cmdArgs...)

		// Wir leiten den Output direkt an die Konsole weiter (optional)
		// So sieht der User in Echtzeit, was passiert (z.B. apt-get Fortschritt)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		return valpkg.BoolVal(err == nil)
	})

	// proc.ExecEx(command, timeout_ms, [args...])
	Register(ns+"ExecEx", "proc", "cmd, timeout, args...", "Führt Befehl aus und liefert [Full, Stdout, Stderr, ExitCode]", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("ExecEx: command und timeout_ms benötigt")
		}

		cmdName := valpkg.ToString(args[0])
		timeoutMs := time.Duration(valpkg.ToNum(args[1])) * time.Millisecond

		// Alle weiteren Argumente sammeln
		var cmdArgs []string
		for i := 2; i < len(args); i++ {
			cmdArgs = append(cmdArgs, valpkg.ToString(args[i]))
		}

		return execExRun(cmdName, timeoutMs, "", cmdArgs...)
	})

	// proc.ExecExDir(command, timeout_ms, workingDir, [args...])
	Register(ns+"ExecExDir", "proc", "cmd, timeout, workingDir, args...",
		"Führt Befehl mit eigenem Arbeitsverzeichnis aus und liefert [Full, Stdout, Stderr, ExitCode]",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.ErrorVal("ExecExDir: command, timeout_ms und workingDir benötigt")
			}

			cmdName := valpkg.ToString(args[0])
			timeoutMs := time.Duration(valpkg.ToNum(args[1])) * time.Millisecond
			workingDir := valpkg.ToString(args[2])

			if workingDir == "" {
				return valpkg.ErrorVal("ExecExDir: workingDir ist leer")
			}

			// Alle weiteren Argumente sind Argumente des gestarteten Prozesses
			var cmdArgs []string
			for i := 3; i < len(args); i++ {
				cmdArgs = append(cmdArgs, valpkg.ToString(args[i]))
			}

			return execExRun(cmdName, timeoutMs, workingDir, cmdArgs...)
		})

	// Nach mal prüfen - ExecInteractive
	Register(ns+"ExecInteractive", "proc", "cmd, timeout, responses, args...",
		"Führt Befehl aus und reagiert automatisch auf Prompts. responses = Array von [muster, antwort] Paaren. Rückgabe: [Full, Stdout, Stderr, ExitCode]",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.ErrorVal("ExecInteractive: cmd, timeout, responses benötigt")
			}

			cmdName := valpkg.ToString(args[0])
			timeoutMs := time.Duration(valpkg.ToNum(args[1])) * time.Millisecond

			// responses: Array von [muster, antwort, muster, antwort, ...]
			// Wir bauen eine Map daraus
			responses := make(map[string]string)
			if args[2].Kind == valpkg.KindArr {
				arr := args[2].Arr
				for i := 0; i+1 < len(arr); i += 2 {
					pattern := strings.ToLower(valpkg.ToString(arr[i]))
					answer := valpkg.ToString(arr[i+1])
					responses[pattern] = answer
				}
			}

			// Weitere Argumente = cmd-Argumente
			var cmdArgs []string
			for i := 3; i < len(args); i++ {
				cmdArgs = append(cmdArgs, valpkg.ToString(args[i]))
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeoutMs)
			defer cancel()

			cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

			// Pipes für stdin, stdout, stderr
			stdin, err := cmd.StdinPipe()
			if err != nil {
				return valpkg.ErrorVal("stdin pipe fehlgeschlagen: " + err.Error())
			}

			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				return valpkg.ErrorVal("stdout pipe fehlgeschlagen: " + err.Error())
			}

			stderrPipe, err := cmd.StderrPipe()
			if err != nil {
				return valpkg.ErrorVal("stderr pipe fehlgeschlagen: " + err.Error())
			}

			if err := cmd.Start(); err != nil {
				return valpkg.ErrorVal("Prozess konnte nicht gestartet werden: " + err.Error())
			}

			var stdoutBuf, stderrBuf strings.Builder
			var mu sync.Mutex
			var stdinMu sync.Mutex
			var wg sync.WaitGroup

			handleOutput := func(reader io.Reader, isStdErr bool) {
				defer wg.Done()

				buf := make([]byte, 4096)
				var current strings.Builder

				for {
					n, err := reader.Read(buf)

					if n > 0 {
						for _, ch := range buf[:n] {
							current.WriteByte(ch)

							// Prompt prüfen
							if ch == '\n' || current.Len() >= 256 {
								text := strings.ToLower(current.String())

								for pattern, answer := range responses {
									if strings.Contains(text, pattern) {
										stdinMu.Lock()
										_, _ = stdin.Write([]byte(answer + "\n"))
										stdinMu.Unlock()
										break
									}
								}
							}

							// Ausgabe speichern bei Zeilenende
							if ch == '\n' {
								mu.Lock()
								if isStdErr {
									stderrBuf.WriteString(current.String())
								} else {
									stdoutBuf.WriteString(current.String())
								}
								mu.Unlock()

								current.Reset()
							}

							// sehr lange Zeilen begrenzen
							if current.Len() >= 256 {
								current.Reset()
							}
						}
					}

					if err != nil {
						if err != io.EOF {
							mu.Lock()
							if isStdErr {
								stderrBuf.WriteString("stderr read error: ")
								stderrBuf.WriteString(err.Error())
							} else {
								stdoutBuf.WriteString("stdout read error: ")
								stdoutBuf.WriteString(err.Error())
							}
							mu.Unlock()
						}
						return
					}
				}
			}

			// stdout lesen
			// stdout lesen
			// stdout lesen
			wg.Add(1)
			go handleOutput(stdoutPipe, false)

			// stderr lesen
			wg.Add(1)
			go handleOutput(stderrPipe, true)

			// Prozess abwarten
			err = cmd.Wait()

			// stdin schließen
			_ = stdin.Close()

			// Reader beenden
			wg.Wait()

			exitCode := getExitCode(err, ctx)

			stdout := stdoutBuf.String()
			stderr := stderrBuf.String()

			return buildExecResult(
				stdout+stderr,
				stdout,
				stderr,
				exitCode,
			)
		})

	// proc.Info(pid) -> Array mit Details
	Register(ns+"Info", "proc", "pid", "Liefert Details [Name, Status, Startzeit, User, PID] als Array.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("proc.Info(pid) erwartet eine PID")
		}

		pid := int32(args[0].Num)
		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.ErrorVal("Prozess nicht gefunden")
		}

		name, _ := p.Name()
		status, _ := p.Status()
		uTime, _ := p.CreateTime()
		username, _ := p.Username()

		// Status sicher lesen (Status() kann leer zurückkommen)
		statusText := ""
		if len(status) > 0 {
			statusText = status[0]
		}

		// Erstellungszeit in lesbares Datum wandeln
		startTime := time.Unix(uTime/1000, 0).Format("2006-01-02 15:04:05")

		return valpkg.Value{
			Kind: valpkg.KindArr,
			Arr: []valpkg.Value{
				valpkg.StrVal(name),         // [0] Name
				valpkg.StrVal(statusText),   // [1] Status
				valpkg.StrVal(startTime),    // [2] Startzeitpunkt
				valpkg.StrVal(username),     // [3] Besitzer
				valpkg.NumVal(float64(pid)), // [4] PID
			},
		}
	})

	// Priorität abfragen
	// Rückgabe: Prioritätswert (Windows: 0-5 Klassen, Unix: -20 bis 19)
	Register(ns+"GetPriority", "proc", "pid", "Gibt die Nice-Priorität zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.NumVal(0)
		}
		pid := int32(args[0].Num)
		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.NumVal(0)
		}

		priority, err := p.Nice()
		if err != nil {
			return valpkg.NumVal(0)
		}
		return valpkg.NumVal(float64(priority))
	})

	Register(ns+"CurrentPid", "proc", "", "Gibt die PID des aktuellen vbx-Interpreters zurück.", func(args []valpkg.Value) valpkg.Value {
		return valpkg.Value{Kind: valpkg.KindNum, Num: float64(os.Getpid())}
	})

	Register(ns+"ParentPid", "proc", "[pid]", "Gibt die PPID zurück (Default: Eigen-PPID).", func(args []valpkg.Value) valpkg.Value {
		var pid int32

		if len(args) == 0 {
			pid = int32(os.Getpid())
		} else {
			pid = int32(args[0].Num)
		}

		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindNum, Num: 0}
		}

		ppid, err := p.Ppid()
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindNum, Num: 0}
		}

		return valpkg.Value{Kind: valpkg.KindNum, Num: float64(ppid)}
	})

	Register(ns+"Memory", "proc", "pid, [type]", "RAM-Verbrauch der PID in Bytes. type=0: RSS (Physikalisch), type=1: VMS (Virtuell).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.NumVal(0)
		}
		pid := int32(args[0].Num)

		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.NumVal(0)
		}

		mem, err := p.MemoryInfo()
		if err != nil {
			return valpkg.NumVal(0)
		}

		// Wenn type=1, gib VMS zurück, sonst RSS
		if len(args) > 1 && args[1].Num == 1 {
			return valpkg.NumVal(float64(mem.VMS))
		}
		return valpkg.NumVal(float64(mem.RSS))
	})

	Register(ns+"CPU", "proc", "pid", "Gibt die CPU-Last der PID in Prozent zurück (misst über 100ms).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.NumVal(0)
		}
		pid := int32(args[0].Num)
		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.NumVal(0)
		}

		// gopsutil braucht ein Intervall, um die Last zu berechnen
		// 100ms ist ein guter Kompromiss zwischen Speed und Genauigkeit
		percent, err := p.Percent(100 * time.Millisecond)
		if err != nil {
			return valpkg.NumVal(0)
		}

		return valpkg.NumVal(percent)
	})

	Register(ns+"GetPidsByPath", "proc", "path", "Gibt Array mit PIDs zurück, die diesen Pfad nutzen.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		targetPath := args[0].Str

		if abs, err := absPathVal(targetPath); err == nil {
			targetPath = abs
		}
		procs, err := process.Processes()
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		var result []valpkg.Value

		for _, p := range procs {
			exe, err := p.Exe()
			if err != nil {
				continue
			}
			if pathMatch(exe, targetPath, false) {
				result = append(result, valpkg.Value{
					Kind: valpkg.KindNum,
					Num:  float64(p.Pid),
				})
			}
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	Register(ns+"GetPidsByName", "proc", "name", "Gibt Array mit PIDs zurück, die diesen Namen nutzen.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		targetName := args[0].Str
		procs, err := process.Processes()
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		var result []valpkg.Value

		for _, p := range procs {
			name, err := p.Name()
			if err != nil {
				continue
			}
			if nameMatch(name, targetName, false) {
				result = append(result, valpkg.Value{
					Kind: valpkg.KindNum,
					Num:  float64(p.Pid),
				})
			}
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	Register(ns+"GetPidsByNameEx", "proc", "name", "Suche nach Name. Liefert Array von [PID, Path] Paaren.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		target := strings.ToLower(args[0].Str)
		procs, _ := process.Processes()
		var result []valpkg.Value

		for _, p := range procs {
			name, _ := p.Name()
			// Match auf den Prozessnamen
			if nameMatch(name, target, false) {
				exe, _ := p.Exe() // Pfad dazu holen

				// Ein Unter-Array für diesen Treffer: [PID, Path]
				entry := valpkg.Value{
					Kind: valpkg.KindArr,
					Arr: []valpkg.Value{
						valpkg.NumVal(float64(p.Pid)),
						valpkg.StrVal(exe),
					},
				}
				result = append(result, entry)
			}
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	Register(ns+"GetPidsEx", "proc", "", "Gibt ein Array von Arrays zurück: [[PID, Path], ...]", func(args []valpkg.Value) valpkg.Value {
		procs, err := process.Processes()
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		var result []valpkg.Value
		for _, p := range procs {
			exe, _ := p.Exe() // Pfad abrufen

			// Ein kleines Unter-Array für jeden Prozess: [PID, Path]
			procEntry := valpkg.Value{
				Kind: valpkg.KindArr,
				Arr: []valpkg.Value{
					valpkg.NumVal(float64(p.Pid)),
					valpkg.StrVal(exe),
				},
			}
			result = append(result, procEntry)
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	Register(ns+"IsSystem", "proc", "pid", "Prüft, ob die PID geschützt ist (True/False).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.BoolVal(false)
		}

		pid := int32(args[0].Num)
		p, err := process.NewProcess(pid)
		if err != nil {
			// Ein nicht existierender Prozess ist kein geschützter Systemprozess
			return valpkg.BoolVal(false)
		}

		// Wir geben direkt das Ergebnis von isProtected als BoolVal zurück
		return valpkg.BoolVal(isProtected(p))
	})

	Register(ns+"UptimeString", "proc", "pid", "Gibt die Laufzeit als formatierten Text zurück (z.B. '2d 05:10:01').", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.StrVal("00:00:00")
		}
		pid := int32(args[0].Num)

		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.StrVal("00:00:00")
		}

		createTime, err := p.CreateTime()
		if err != nil {
			return valpkg.StrVal("00:00:00")
		}

		nowMs := time.Now().UnixNano() / int64(time.Millisecond)
		uptimeSec := (nowMs - createTime) / 1000

		return valpkg.StrVal(formatSeconds(uptimeSec))
	})

	Register(ns+"Uptime", "proc", "pid", "Gibt die Laufzeit des Prozesses in Sekunden zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.NumVal(0)
		}
		pid := int32(args[0].Num)

		p, err := process.NewProcess(pid)
		if err != nil {
			return valpkg.NumVal(0)
		}

		// CreateTime liefert Millisekunden (Unix Epoch)
		createTime, err := p.CreateTime()
		if err != nil {
			return valpkg.NumVal(0)
		}

		// Aktuelle Zeit in Millisekunden
		nowMs := time.Now().UnixNano() / int64(time.Millisecond)

		// Differenz in Sekunden
		uptimeSec := (nowMs - createTime) / 1000

		return valpkg.NumVal(float64(uptimeSec))
	})
}

// Hilfsfunktion: Prüft, ob ein Prozess geschützt ist
func isProtected(p *process.Process) bool {
	// 1. Eigene PID und System-Kern (0-10) schützen
	if p.Pid == int32(os.Getpid()) || p.Pid <= 10 {
		return true
	}

	// 2. Schutz des Eltern-Prozesses (z.B. CMD, Terminal, IDE)
	// Wir nutzen os.Getppid() direkt, ppid-Variable wird nicht mehr benötigt
	if p.Pid == int32(os.Getppid()) {
		return true
	}

	name, err := p.Name()
	if err != nil {
		return false
	}

	lowName := strings.ToLower(name)
	exe, _ := p.Exe()
	lowExe := strings.ToLower(exe)

	// Kritische Namen und Pfad-Bestandteile (Windows, Linux, macOS)
	critical := []string{
		"system", "services.exe", "wininit.exe", "csrss.exe", "lsass.exe",
		"init", "systemd", "launchd", "kernel_task", "kthreadd",
	}

	for _, c := range critical {
		if lowName == c || strings.Contains(lowExe, c) {
			return true
		}
	}
	return false
}

// Rekursiver Kill mit Schutz-Check
func killTree(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return err
	}

	// SICHERHEIT: Jeder Knoten im Baum wird geprüft
	if isProtected(p) {
		return fmt.Errorf("Prozess %d ist geschützt", pid)
	}

	// Erst die Kinder beenden (Bottom-Up)
	children, err := p.Children()
	if err == nil {
		for _, c := range children {
			_ = killTree(c.Pid) // Fehler bei Kindern ignorieren wir oft, um weiterzumachen
		}
	}

	return p.Kill()
}

func formatSeconds(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if days > 0 {
		return fmt.Sprintf("%dd %02d:%02d:%02d", days, hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}

func pathMatch(exe, target string, contains bool) bool {
	if runtime.GOOS == "windows" {
		if contains {
			return strings.Contains(
				strings.ToLower(exe),
				strings.ToLower(target),
			)
		}

		return strings.EqualFold(
			filepath.Clean(exe),
			filepath.Clean(target),
		)
	}

	if contains {
		return strings.Contains(exe, target)
	}

	return filepath.Clean(exe) == filepath.Clean(target)
}

func nameMatch(name, target string, contains bool) bool {
	name = strings.ToLower(name)
	target = strings.ToLower(target)

	if contains {
		return strings.Contains(name, target)
	}

	return name == target
}

// execExRun führt einen Prozess mit Timeout und optionalem Arbeitsverzeichnis aus.
func execExRun(cmdName string, timeoutMs time.Duration, workingDir string, cmdArgs ...string) valpkg.Value {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutMs)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	if workingDir != "" {
		cmd.Dir = workingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := getExitCode(err, ctx)

	return buildExecResult(
		stdout.String()+stderr.String(),
		stdout.String(),
		stderr.String(),
		exitCode,
	)
}

func getExitCode(err error, ctx context.Context) float64 {
	if err == nil {
		return 0
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return float64(exitErr.ExitCode())
	}

	if ctx.Err() == context.DeadlineExceeded {
		return -999
	}

	return -1
}

func buildExecResult(full, stdout, stderr string, exitCode float64) valpkg.Value {
	return valpkg.Value{
		Kind: valpkg.KindArr,
		Arr: []valpkg.Value{
			valpkg.StrVal(full),
			valpkg.StrVal(stdout),
			valpkg.StrVal(stderr),
			valpkg.NumVal(exitCode),
		},
	}
}

func init() {
	initProcFunctions()
}
