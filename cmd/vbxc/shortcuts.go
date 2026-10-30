package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"vbxc/plugin"
	"vbxc/stdlib"
	"vbxc/value"
)

type cliShortcut struct {
	internal string
	args     string
	desc     string
}

// CLI aliases carried over from vbx. Entries are shown only when vbxc has
// the corresponding builtin (or plugin) available.
var cliShortcutTable = map[string]cliShortcut{
	"aesdecrypt":     {"crypt.AESDecrypt", "ciphertext, [password]", "AES-256-GCM entschlüsseln"},
	"aesencrypt":     {"crypt.AESEncrypt", "text, [password]", "AES-256-GCM verschlüsseln"},
	"agestring":      {"date.AgeString", "tag, monat, jahr [, stunde [, minute]]", "Alter als lesbarer Text"},
	"createconf":     {"cert.CreateConf", "", "OpenSSL-Konfiguration erzeugen"},
	"distro":         {"computer.Distro", "", "Linux-Distribution anzeigen"},
	"file64decode":   {"file.Base64Decode", "infile, outfile", "Base64-Datei dekodieren"},
	"file64encode":   {"file.Base64Encode", "infile, outfile", "Datei Base64-kodieren"},
	"filereplaceall": {"file.ReplaceAll", "path, old, new", "Text in einer Datei ersetzen"},
	"findeventid":    {"win.FindEventID", "log, id, count", "Windows-Event-ID suchen"},
	"foldercount":    {"folder.Count", "path", "Dateien und Unterordner zählen"},
	"getevent":       {"win.GetEvent", "log, level, count", "Windows-Events abrufen"},
	"gzcreate":       {"tar.GzCreate", "tarGzPath, paths...", "TAR.GZ erstellen"},
	"gzcreateflat":   {"tar.GzCreateFlat", "tarGzPath, paths...", "TAR.GZ ohne Ordnerstruktur erstellen"},
	"categories":     {"convert.Categories", "", "Einheitenkategorien anzeigen"},
	"units":          {"convert.Units", "category", "Einheiten einer Kategorie anzeigen"},
	"gzextract":      {"tar.GzExtract", "tarGzPath, paths...", "TAR.GZ entpacken"},
	"randomstring":   {"crypt.RandomString", "[length]", "Zufälligen String erzeugen"},
	"searchevent":    {"win.SearchEventLog", "log, text, count", "Windows-Eventlog durchsuchen"},
	"string64decode": {"DecodeBase64", "text", "Base64-Text dekodieren"},
	"string64encode": {"EncodeBase64", "text", "Text nach Base64 kodieren"},
	"tail":           {"file.Tail", "path, lines, [refresh]", "Dateiende anzeigen"},
	"unblock":        {"Unblock", "path", "Windows-Zone.Identifier entfernen"},
	"watchlog":       {"file.WatchLog", "path, pattern, style", "Textdatei live überwachen"},
	"zipcreate":      {"zip.Create", "zipPath, paths..., [password]", "ZIP erstellen"},
	"zipcreateflat":  {"zip.CreateFlat", "zipPath, paths..., [password]", "ZIP ohne Ordnerstruktur erstellen"},
	"zipextract":     {"zip.Extract", "zipPath, dest, [password]", "ZIP entpacken"},
	"worker":         {"Worker", "vbPath", "VB-Skript im Hintergrund starten"},
	"optimizefile":   {"db.OptimizeFile", "path, dialect [, outPath]", "SQL-Datei optimieren"},
	"uptime":         {"UptimeString", "", "Systemlaufzeit anzeigen"},
	"convnewline":    {"ConvNewLine", "content, [target]", "Zeilenumbrüche normalisieren"},
	"reboot":         {"computer.Reboot", "", "System neu starten"},
	"shutdown":       {"computer.Shutdown", "", "System herunterfahren"},
	"printformat":    {"PrintFormat", "", "Formatierungsoptionen anzeigen"},
	"copyfolder":     {"folder.Copy", "src, dst, [progress]", "Ordner rekursiv kopieren"},
	"rename":         {"file.Rename", "old, new", "Datei umbenennen"},
	"generatesshkey": {"ssh.GenerateSSHKey", "[outFile, algo, bits, pass]", "SSH-Schlüsselpaar erzeugen"},
	"remoteboot":     {"ssh.RebootWithKey", "host, user, keyPath, [delay], [knownHostsPath], [port]", "Remote-System per SSH neu starten"},
	"remoteexec":     {"ssh.ExecOnce", "host, user, keyPath, cmd, [printOutput], [knownHostsPath], [port]", "Einmaligen SSH-Befehl ausführen"},
	"wol":            {"net.WakeOnLan", "mac, [broadcastIP], [port]", "Wake-on-LAN-Paket senden"},
	"pqcidentity":    {"pqc.SetupIdentity", "path, encrypt", "PQC-Schlüssel erzeugen"},
	"pgpidentity":    {"pgp.SetupIdentity", "folder, name, email, [password]", "PGP-Schlüssel erzeugen"},
	"qcsignidentity": {"pqc.SetupSignIdentity", "path, encrypt", "Signaturschlüssel erzeugen"},
}

func lookupShortcutBuiltin(internal string) (stdlib.BuiltinInfo, bool) {
	if info, ok := stdlib.Lookup(internal); ok {
		return info, true
	}
	parts := strings.SplitN(internal, ".", 2)
	if len(parts) != 2 {
		return stdlib.BuiltinInfo{}, false
	}
	return stdlib.FindByModuleAndName(parts[0], parts[1])
}

func shortcutAvailable(internal string) bool {
	if _, ok := lookupShortcutBuiltin(internal); ok {
		return true
	}
	parts := strings.SplitN(internal, ".", 2)
	if len(parts) != 2 {
		return false
	}
	path, err := plugin.PluginPath(parts[0])
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func shortcutBuiltin(internal string) (stdlib.BuiltinInfo, bool) {
	if info, ok := lookupShortcutBuiltin(internal); ok {
		return info, true
	}
	parts := strings.SplitN(internal, ".", 2)
	if len(parts) != 2 || plugin.LoadModules([]string{parts[0]}) != nil {
		return stdlib.BuiltinInfo{}, false
	}
	return lookupShortcutBuiltin(internal)
}

func availableShortcuts() map[string]cliShortcut {
	out := make(map[string]cliShortcut)
	for alias, shortcut := range cliShortcutTable {
		if shortcutAvailable(shortcut.internal) {
			out[alias] = shortcut
		}
	}
	return out
}

func printShellHelp(filter string) {
	const reset = "\033[0m"
	shortcuts := availableShortcuts()
	keys := make([]string, 0, len(shortcuts))
	filter = strings.ToLower(strings.TrimSpace(filter))
	for key, shortcut := range shortcuts {
		if filter == "" || strings.Contains(key, filter) || strings.Contains(strings.ToLower(shortcut.internal), filter) || strings.Contains(strings.ToLower(shortcut.desc), filter) {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := shortcuts[keys[i]], shortcuts[keys[j]]
		if a.internal != b.internal {
			return a.internal < b.internal
		}
		return keys[i] < keys[j]
	})
	fmt.Println("\n\033[1mVerfügbare Shell-Befehle:\033[0m")
	if len(keys) == 0 {
		fmt.Printf("  \033[31m(Keine Shell-Befehle für Filter '%s' gefunden)\033[0m\n", filter)
		return
	}
	for _, key := range keys {
		shortcut := shortcuts[key]
		fmt.Printf("  \033[36m%-18s\033[0m \033[33m%-44s\033[0m -> \033[32m%s\033[0m %s%s\n",
			key, shortcut.args, shortcut.internal, shortcut.desc, reset)
	}
	fmt.Println("\nAufruf: vbxc <befehl> [argumente ...]")
}

func executeShortcut(alias string, args []string) (bool, error) {
	shortcut, ok := availableShortcuts()[strings.ToLower(alias)]
	if !ok {
		return false, nil
	}
	info, ok := shortcutBuiltin(shortcut.internal)
	if !ok {
		return true, fmt.Errorf("Funktion %q ist nicht verfügbar", shortcut.internal)
	}
	values := make([]value.Value, len(args))
	for i, arg := range args {
		values[i] = value.StrVal(strings.Trim(arg, " \"'"))
	}
	if info.ParamCount >= 0 && len(values) != info.ParamCount {
		return true, fmt.Errorf("%s erwartet %d Argumente, erhalten: %d\nSyntax: %s %s", shortcut.internal, info.ParamCount, len(values), shortcut.internal, info.Params)
	}
	result := info.Fn(values)
	switch result.Kind {
	case value.KindError:
		return true, fmt.Errorf("%s", result.Str)
	case value.KindBool, value.KindNull, value.KindNil, value.KindNone:
		// Viele Kommandos melden ihren Erfolg selbst; boolesche Ergebnisse
		// bleiben wie in vbx ohne zusätzliche Ausgabe.
	case value.KindArr:
		for _, item := range result.Arr {
			fmt.Println(value.ToString(item))
		}
	default:
		fmt.Println(value.ToString(result))
	}
	return true, nil
}
