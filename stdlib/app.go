package stdlib

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"vbxc/value"
)

func init() {
	str := func(s string) value.Value { return value.StrVal(s) }
	Register("ScriptPath", "app", "", "Gibt den vollständigen Pfad des aktuell ausgeführten Skripts zurück.", func([]value.Value) value.Value { return str(activeScriptPath()) })
	Register("ScriptDir", "app", "", "Gibt das Verzeichnis der Skript-Datei zurück.", func([]value.Value) value.Value { return str(filepath.Dir(activeScriptPath())) })
	Register("StartupPath", "app", "", "Gibt das Verzeichnis der ausführbaren Datei zurück.", func([]value.Value) value.Value {
		exe, e := os.Executable()
		if e != nil {
			return str("")
		}
		return str(filepath.Dir(exe))
	})
	Register("ExecutablePath", "app", "", "Gibt den vollständigen Pfad der ausführbaren Datei zurück.", func([]value.Value) value.Value {
		exe, e := os.Executable()
		if e != nil {
			return str("")
		}
		return str(exe)
	})
	Register("SetCurrentDirectory", "app", "path", "Setzt das aktuelle Arbeitsverzeichnis. Gibt true bei Erfolg zurück, sonst einen Fehler.", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return value.ErrorVal("SetCurrentDirectory erwartet einen Pfad")
		}
		path := value.ToString(a[0])
		if path == "" {
			return value.ErrorVal("SetCurrentDirectory: Der Pfad darf nicht leer sein")
		}
		if err := os.Chdir(path); err != nil {
			return value.ErrorVal("SetCurrentDirectory: " + err.Error())
		}
		return value.BoolVal(true)
	})
	Register("CurrentDirectory", "app", "", "Gibt das aktuelle Arbeitsverzeichnis zurück.", func([]value.Value) value.Value {
		dir, e := os.Getwd()
		if e != nil {
			return str("")
		}
		return str(dir)
	})
	Register("TempPath", "app", "", "Gibt den Pfad zum temporären Verzeichnis zurück.", func([]value.Value) value.Value { return str(os.TempDir()) })
	Register("SpecialFolder", "app", "name", "Gibt Standardpfade wie home, desktop, temp oder appdata zurück.", func(a []value.Value) value.Value {
		if len(a) == 0 {
			return str("")
		}
		name := strings.ToLower(value.ToString(a[0]))
		home := ""
		if u, e := user.Current(); e == nil {
			home = u.HomeDir
		}
		if home == "" {
			home = os.Getenv("HOME")
		}
		folder := ""
		if runtime.GOOS == "windows" {
			switch name {
			case "appdata":
				folder = os.Getenv("APPDATA")
			case "localappdata":
				folder = os.Getenv("LOCALAPPDATA")
			case "programdata":
				folder = os.Getenv("PROGRAMDATA")
			case "home":
				folder = home
			case "desktop":
				folder = filepath.Join(home, "Desktop")
			case "documents":
				folder = filepath.Join(home, "Documents")
			case "downloads":
				folder = filepath.Join(home, "Downloads")
			case "pictures":
				folder = filepath.Join(home, "Pictures")
			case "music":
				folder = filepath.Join(home, "Music")
			case "videos":
				folder = filepath.Join(home, "Videos")
			}
		} else {
			switch name {
			case "home":
				folder = home
			case "desktop":
				folder = filepath.Join(home, "Desktop")
			case "documents":
				folder = filepath.Join(home, "Documents")
			case "downloads":
				folder = filepath.Join(home, "Downloads")
			case "pictures":
				folder = filepath.Join(home, "Pictures")
			case "music":
				folder = filepath.Join(home, "Music")
			case "videos":
				folder = filepath.Join(home, "Videos")
			}
		}
		if name == "temp" {
			folder = os.TempDir()
		}
		return str(folder)
	})
}

func activeScriptPath() string {
	for _, arg := range os.Args[1:] {
		ext := strings.ToLower(filepath.Ext(arg))
		if ext == ".vb" || ext == ".vbb" {
			return absoluteOrOriginal(arg)
		}
	}
	return ""
}
func absoluteOrOriginal(path string) string {
	abs, e := filepath.Abs(path)
	if e != nil {
		return path
	}
	return abs
}
