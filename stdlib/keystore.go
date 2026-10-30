// ------------------------
// stdlib_keystore.go
// ------------------------

package stdlib

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"vbxc/value"
)

// keytool-Datumsformat (englisch, UTC erzwungen): "Mon Jan 02 15:04:05 UTC 2006"
const ksDateLayout = "Mon Jan 02 15:04:05 MST 2006"

// ---------- keytool-Bridge ----------

type ksEntry struct {
	Alias   string
	Type    string
	Owner   string
	Until   time.Time
	HasDate bool // SecretKeyEntry hat z.B. kein Gültigkeitsdatum
}

func (e ksEntry) Expired(now time.Time) bool {
	return e.HasDate && e.Until.Before(now)
}

// keytoolPath: VBX_KEYTOOL hat Vorrang, dann JAVA_HOME/bin, dann PATH.
func keytoolPath(explicitPath string) string {
	if p := strings.TrimSpace(explicitPath); p != "" {
		return p
	}

	exe := "keytool"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}

	if p, err := exec.LookPath(exe); err == nil {
		return p
	}

	return ""
}

// runKeytool ruft keytool auf. Das Passwort wird per Umgebungsvariable
// übergeben (-storepass:env), damit es nicht in der Prozessliste steht.
// Sprache und Zeitzone werden erzwungen, damit die Ausgabe parsebar bleibt.
func runKeytool(keytool, password string, args ...string) (string, error) {
	kt := keytoolPath(keytool)
	if kt == "" {
		return "", errors.New("keytool nicht gefunden (Pfad, JAVA_HOME oder PATH prüfen)")
	}

	full := append([]string{
		"-J-Duser.language=en",
		"-J-Duser.country=US",
		"-J-Duser.timezone=UTC",
	}, args...)

	full = append(full, "-storepass:env", "VBX_KS_PASS")

	cmd := exec.Command(kt, full...)
	cmd.Env = append(os.Environ(), "VBX_KS_PASS="+password)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("keytool: %s", strings.TrimSpace(string(out)))
	}

	return string(out), nil
}

// ksParse liest die Ausgabe von "keytool -list -v".
// Bei Zertifikatsketten zählt jeweils das erste Zertifikat (Leaf).
func ksParse(out string) []ksEntry {
	var entries []ksEntry
	var cur *ksEntry
	flush := func() {
		if cur != nil {
			entries = append(entries, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "Alias name:"):
			flush()
			cur = &ksEntry{Alias: strings.TrimSpace(strings.TrimPrefix(line, "Alias name:"))}
		case cur == nil:
			// Kopfzeilen vor dem ersten Eintrag ignorieren
		case strings.HasPrefix(line, "Entry type:"):
			cur.Type = strings.TrimSpace(strings.TrimPrefix(line, "Entry type:"))
		case strings.HasPrefix(line, "Owner:") && cur.Owner == "":
			cur.Owner = strings.TrimSpace(strings.TrimPrefix(line, "Owner:"))
		case strings.HasPrefix(line, "Valid from:") && !cur.HasDate:
			if i := strings.Index(line, "until:"); i >= 0 {
				raw := strings.TrimSpace(line[i+len("until:"):])
				if t, err := time.Parse(ksDateLayout, raw); err == nil {
					cur.Until, cur.HasDate = t, true
				}
			}
		}
	}
	flush()
	return entries
}

func ksListEntries(path, password, ksType, keytool string) ([]ksEntry, error) {
	args := []string{"-list", "-v", "-keystore", path}
	if ksType != "" {
		args = append(args, "-storetype", ksType)
	}

	out, err := runKeytool(keytool, password, args...)
	if err != nil {
		return nil, err
	}
	return ksParse(out), nil
}

// ksValueToStrings: Array von Str/Num oder einzelner Wert -> []string
func ksValueToStrings(v value.Value) []string {
	var out []string
	if v.Kind == value.KindArr {
		for _, item := range v.Arr {
			if s := strings.TrimSpace(value.ToString(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	if s := strings.TrimSpace(value.ToString(v)); s != "" {
		out = append(out, s)
	}
	return out
}

func ksStr(s string) value.Value { return value.Value{Kind: value.KindStr, Str: s} }

// ---------- Registrierung ----------

func initKeystoreFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "keystore."

	Register(ns+"List", "keystore", "path, password, [onlyExpired], [type]", "Gibt die Aliase eines Java-Keystores als Array zurück. Mit onlyExpired=True nur abgelaufene Einträge; das Ergebnis kann direkt an keystore.Delete übergeben werden. type optional (JKS, PKCS12, JCEKS), sonst automatisch erkannt.", func(args []value.Value) value.Value {
		if len(args) < 2 {
			return value.ErrorVal("keystore.List: path und password benötigt")
		}
		path, err := filepath.Abs(value.ToString(args[0])) // TODO: auf absPathVal umstellen
		if err != nil {
			return value.ErrorVal("keystore.List: ungültiger Pfad: " + err.Error())
		}
		password := value.ToString(args[1])
		onlyExpired := len(args) >= 3 && value.IsTruthy(args[2])
		ksType := ""
		if len(args) >= 4 {
			ksType = strings.TrimSpace(value.ToString(args[3]))
		}
		keytool := ""
		if len(args) >= 5 { // List
			keytool = strings.TrimSpace(value.ToString(args[4]))
		}

		entries, err := ksListEntries(path, password, ksType, keytool)
		if err != nil {
			return value.ErrorVal("keystore.List: " + err.Error())
		}

		now := time.Now()
		aliases := []value.Value{}
		for _, e := range entries {
			if onlyExpired && !e.Expired(now) {
				continue
			}
			aliases = append(aliases, ksStr(e.Alias))
		}
		return value.Value{Kind: value.KindArr, Arr: aliases}
	})

	Register(ns+"Find", "keystore",
		"path, password, alias, [type], [keytool]",
		"Prüft, ob ein Alias in einem Java-Keystore existiert. Gibt True zurück, wenn der Alias vorhanden ist, andernfalls False. Technische Fehler werden als Runtime Error gemeldet.",
		func(args []value.Value) value.Value {
			if len(args) < 3 {
				return value.ErrorVal("keystore.Find: path, password und alias benötigt")
			}

			path, err := filepath.Abs(value.ToString(args[0]))
			if err != nil {
				return value.ErrorVal("keystore.Find: ungültiger Pfad: " + err.Error())
			}

			password := value.ToString(args[1])
			alias := strings.TrimSpace(value.ToString(args[2]))

			if alias == "" {
				return value.ErrorVal("keystore.Find: Alias darf nicht leer sein")
			}

			ksType := ""
			if len(args) >= 4 {
				ksType = strings.TrimSpace(value.ToString(args[3]))
			}

			keytool := ""
			if len(args) >= 5 {
				keytool = strings.TrimSpace(value.ToString(args[4]))
			}

			cmdArgs := []string{
				"-list",
				"-alias", alias,
				"-keystore", path,
			}

			if ksType != "" {
				cmdArgs = append(cmdArgs, "-storetype", ksType)
			}

			out, err := runKeytool(keytool, password, cmdArgs...)
			if err == nil {
				return value.BoolVal(true)
			}

			msg := strings.ToLower(out + " " + err.Error())

			if strings.Contains(msg, "does not exist") ||
				strings.Contains(msg, "not found") ||
				strings.Contains(msg, "existiert nicht") {
				return value.BoolVal(false)
			}

			return value.ErrorVal("keystore.Find: " + err.Error())
		})

	Register(ns+"Info", "keystore", "path, password, [type]", "Gibt alle Einträge eines Java-Keystores als 2D-Array zurück: [Alias, Typ, Owner, GültigBis (UTC, yyyy-MM-dd HH:mm:ss, leer falls ohne Datum), Abgelaufen (Bool)].", func(args []value.Value) value.Value {
		if len(args) < 2 {
			return value.ErrorVal("keystore.Info: path und password benötigt")
		}
		path, err := filepath.Abs(value.ToString(args[0])) // TODO: auf absPathVal umstellen
		if err != nil {
			return value.ErrorVal("keystore.Info: ungültiger Pfad: " + err.Error())
		}
		password := value.ToString(args[1])
		ksType := ""
		if len(args) >= 3 {
			ksType = strings.TrimSpace(value.ToString(args[2]))
		}
		keytool := ""
		if len(args) >= 4 {
			keytool = strings.TrimSpace(value.ToString(args[3]))
		}

		entries, err := ksListEntries(path, password, ksType, keytool)
		if err != nil {
			return value.ErrorVal("keystore.Info: " + err.Error())
		}

		now := time.Now()
		rows := []value.Value{}
		for _, e := range entries {
			until := ""
			if e.HasDate {
				until = e.Until.Format("2006-01-02 15:04:05")
			}
			rows = append(rows, value.Value{Kind: value.KindArr, Arr: []value.Value{
				ksStr(e.Alias),
				ksStr(e.Type),
				ksStr(e.Owner),
				ksStr(until),
				value.BoolVal(e.Expired(now)),
			}})
		}
		return value.Value{Kind: value.KindArr, Arr: rows}
	})

	Register(ns+"Delete", "keystore",
		"path, password, alias, [type], [keytool]",
		"Löscht einen Alias aus einem Java-Keystore. Rückgabe: Array {Boolean, Meldung}. True = erfolgreich gelöscht, False = Alias nicht gefunden. Technische Fehler werden als Runtime Error gemeldet.",
		func(args []value.Value) value.Value {
			if len(args) < 3 {
				return value.ErrorVal("keystore.Delete: path, password und alias benötigt")
			}

			path, err := filepath.Abs(value.ToString(args[0]))
			if err != nil {
				return value.ErrorVal("keystore.Delete: ungültiger Pfad: " + err.Error())
			}

			password := value.ToString(args[1])
			alias := strings.TrimSpace(value.ToString(args[2]))

			if alias == "" {
				return value.ErrorVal("keystore.Delete: Alias darf nicht leer sein")
			}

			ksType := ""
			if len(args) >= 4 {
				ksType = strings.TrimSpace(value.ToString(args[3]))
			}

			keytool := ""
			if len(args) >= 5 {
				keytool = strings.TrimSpace(value.ToString(args[4]))
			}

			cmdArgs := []string{
				"-delete",
				"-alias", alias,
				"-keystore", path,
			}

			if ksType != "" {
				cmdArgs = append(cmdArgs, "-storetype", ksType)
			}

			out, err := runKeytool(keytool, password, cmdArgs...)
			if err != nil {
				msg := strings.ToLower(strings.TrimSpace(out))

				// Ein fehlender Alias ist kein technischer Fehler.
				if strings.Contains(msg, "does not exist") ||
					strings.Contains(msg, "not found") ||
					strings.Contains(msg, "existiert nicht") {
					return value.Value{Kind: value.KindArr, Arr: []value.Value{
						value.BoolVal(false),
						value.StrVal("Eintrag konnte nicht gefunden werden"),
					}}
				}

				return value.ErrorVal("keystore.Delete: " + err.Error())
			}

			return value.Value{Kind: value.KindArr, Arr: []value.Value{
				value.BoolVal(true),
				value.StrVal("Erfolgreich gelöscht"),
			}}
		})

}

func init() {
	initKeystoreFunctions()
}
