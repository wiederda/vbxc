//go:build windows

package stdlib

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"vbxc/value"
)

var (
	winUser32              = syscall.NewLazyDLL("user32.dll")
	winGetForegroundWindow = winUser32.NewProc("GetForegroundWindow")
	winGetWindowTextW      = winUser32.NewProc("GetWindowTextW")
)

func init() {
	const ns = "win."
	Register(ns+"GetActiveTitle", "win", "", "Gibt den Titel des aktuell im Vordergrund befindlichen Fensters zurück.", func(args []value.Value) value.Value {
		hwnd, _, _ := winGetForegroundWindow.Call()
		if hwnd == 0 {
			return value.ErrorVal("Kein Fenster im Vordergrund")
		}
		buf := make([]uint16, 256)
		n, _, _ := winGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n == 0 {
			return value.ErrorVal("Fenster-Titel konnte nicht gelesen werden")
		}
		return value.StrVal(syscall.UTF16ToString(buf))
	})
	Register(ns+"GetEvent", "win", "logName, level, [count]", "Liest Ereignisse eines Typs aus dem Windows-EventLog.", func(args []value.Value) value.Value {
		if len(args) < 2 {
			return value.ErrorVal("win.GetEvent benötigt Log-Name und Level")
		}
		level := strings.ToLower(value.ToString(args[1]))
		levelID, style, label := 4, 37, "INFO "
		switch level {
		case "fehler", "error", "err", "f", "2":
			levelID, style, label = 2, 31, "ERROR"
		case "warnung", "warn", "warning", "w", "3":
			levelID, style, label = 3, 33, "WARN "
		case "information", "info", "i", "4":
			levelID, style, label = 4, 37, "INFO "
		}
		return runWinEventQuery(value.ToString(args[0]), fmt.Sprintf("*[System[(Level=%d)]]", levelID), winCount(args, 2), label, style, true)
	})
	Register(ns+"SearchEventLog", "win", "log, text, [count]", "Durchsucht ein EventLog nach Text in den Event-Daten oder Providern.", func(args []value.Value) value.Value {
		if len(args) < 2 {
			return value.ErrorVal("win.SearchEventLog benötigt Log-Name und Suchtext")
		}
		search := strings.ReplaceAll(value.ToString(args[1]), "'", "''")
		query := fmt.Sprintf("*[EventData[Data[contains(.,'%s')]]] or *[System[Provider[@Name='%s']]]", search, search)
		return runWinEventQuery(value.ToString(args[0]), query, winCount(args, 2), "SEARCH: "+search, 36, true)
	})
	Register(ns+"FindEventID", "win", "log, id, [count]", "Sucht im angegebenen EventLog nach einer spezifischen Event-ID.", func(args []value.Value) value.Value {
		if len(args) < 2 {
			return value.ErrorVal("win.FindEventID benötigt Log-Name und Event-ID")
		}
		id := strings.TrimSpace(value.ToString(args[1]))
		if _, err := strconv.Atoi(id); err != nil {
			return value.ErrorVal("win.FindEventID: Event-ID muss eine Ganzzahl sein")
		}
		return runWinEventQuery(value.ToString(args[0]), fmt.Sprintf("*[System[(EventID=%s)]]", id), winCount(args, 2), "ID: "+id, 95, false)
	})
}

func winCount(args []value.Value, index int) int {
	count := 5
	if len(args) > index {
		count = int(value.ToNum(args[index]))
	}
	if count < 1 {
		return 1
	}
	return count
}

func runWinEventQuery(logInput, query string, count int, label string, style int, showID bool) value.Value {
	logName := "System"
	switch strings.ToLower(logInput) {
	case "anwendung", "application", "app":
		logName = "Application"
	case "sicherheit", "security", "sec":
		logName = "Security"
	case "installation", "setup":
		logName = "Setup"
	case "system", "sys":
		logName = "System"
	default:
		return value.ErrorVal("Unbekanntes EventLog: " + logInput)
	}
	cmd := exec.Command("wevtutil", "qe", logName, "/q:"+query, "/c:"+strconv.Itoa(count), "/rd:true", "/f:text")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return value.ErrorVal("EventLog-Fehler: " + strings.TrimSpace(string(output)))
	}
	fmt.Printf("\033[90m--- [%s] %s (%d Einträge) ---\033[0m\n", strings.ToUpper(logName), label, count)
	for _, event := range strings.Split(string(output), "\r\n\r\n") {
		if strings.TrimSpace(event) == "" {
			continue
		}
		var timestamp, source, message, id, category string
		for _, line := range strings.Split(event, "\r\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "Date:"):
				timestamp = strings.TrimPrefix(line, "Date: ")
			case strings.HasPrefix(line, "Source:"):
				source = strings.TrimPrefix(line, "Source: ")
			case strings.HasPrefix(line, "Event ID:"):
				id = strings.TrimPrefix(line, "Event ID: ")
			case strings.HasPrefix(line, "Task Category:"):
				category = strings.TrimPrefix(line, "Task Category: ")
			case strings.HasPrefix(line, "Message:"):
				message = strings.TrimPrefix(line, "Message: ")
			}
		}
		if len(timestamp) > 19 {
			timestamp = timestamp[:19]
		}
		if category == "None" || category == "Keine" || category == "" {
			category = "-"
		}
		if showID {
			fmt.Printf("\033[%dm%s\033[0m [%s] \033[96mID:%s\033[0m \033[90m(%s)\033[0m %s: %s\n", style, label, timestamp, id, category, source, message)
		} else {
			fmt.Printf("\033[%dm%s\033[0m [%s] \033[90m(%s)\033[0m %s: %s\n", style, label, timestamp, category, source, message)
		}
	}
	return value.BoolVal(true)
}
