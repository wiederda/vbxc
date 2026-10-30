// ------------------------
// stdlib_ssh.go
// ------------------------

package stdlib

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	valpkg "vbxc/value"

	"golang.org/x/crypto/ssh"
)

var (
	sshConnections = make(map[string]*ssh.Client)
	sshMu          sync.RWMutex
)

func initSSHFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "ssh."

	// ------------------------
	// ssh.Connect
	// ------------------------
	Register(ns+"Connect", "ssh", "host, user, keyPath, alias, [knownHostsPath], [port]",
		"Öffnet eine SSH-Verbindung per Key-Auth und speichert sie unter einem Alias für nachfolgende ssh.Exec-Aufrufe. "+
			"knownHostsPath optional - ohne Angabe wird der Host-Key nicht geprüft (InsecureIgnoreHostKey); "+
			"mit Angabe wird er wie bei sftp.Connect gegen eine known_hosts-Datei geprüft (Trust-on-First-Use). "+
			"port default 22. keyPath zeigt auf den privaten Key lokal.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.ErrorVal("ssh.Connect(host, user, keyPath, alias [, knownHostsPath, port]) benötigt mindestens 4 Argumente")
			}

			host := args[0].Str
			user := args[1].Str
			keyPath := args[2].Str
			alias := strings.ToLower(args[3].Str)

			absKeyPath, errVal := absPathVal(keyPath)
			if errVal != nil {
				return *errVal
			}

			port := 22
			if len(args) >= 6 {
				port = int(valpkg.ToNum(args[5]))
			}

			keyBytes, err := os.ReadFile(absKeyPath)
			if err != nil {
				return valpkg.ErrorVal("Private Key konnte nicht gelesen werden: " + err.Error())
			}

			signer, err := ssh.ParsePrivateKey(keyBytes)
			if err != nil {
				return valpkg.ErrorVal("Private Key konnte nicht geparst werden: " + err.Error())
			}

			hostKeyCallback, errVal := buildHostKeyCallback(args, 4)
			if errVal != nil {
				return *errVal
			}

			config := &ssh.ClientConfig{
				User:            user,
				Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
				HostKeyCallback: hostKeyCallback,
				Timeout:         10 * time.Second,
			}

			addr := fmt.Sprintf("%s:%d", host, port)
			client, err := ssh.Dial("tcp", addr, config)
			if err != nil {
				return valpkg.ErrorVal("SSH-Verbindung fehlgeschlagen: " + err.Error())
			}

			sshMu.Lock()
			if old, ok := sshConnections[alias]; ok {
				old.Close()
			}
			sshConnections[alias] = client
			sshMu.Unlock()

			return valpkg.BoolVal(true)
		})

	// ------------------------
	// ssh.Exec
	// ------------------------
	Register(ns+"Exec", "ssh", "alias, cmd",
		"Führt einen beliebigen Shell-Befehl auf einer per ssh.Connect geöffneten Verbindung aus. "+
			"Rückgabe: Map mit stdout, exitCode. Bei nicht-0 exitCode steht die Fehlerausgabe des Zielsystems in stdout "+
			"(kombinierter Stream, wie bei einer interaktiven Shell).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("ssh.Exec(alias, cmd) benötigt Alias und Befehl")
			}
			alias := strings.ToLower(args[0].Str)
			cmd := args[1].Str

			client, errVal := getSSHConn(alias)
			if errVal != nil {
				return *errVal
			}

			session, err := client.NewSession()
			if err != nil {
				return valpkg.ErrorVal("Session konnte nicht geöffnet werden: " + err.Error())
			}
			defer session.Close()

			var buf bytes.Buffer
			session.Stdout = &buf
			session.Stderr = &buf

			exitCode := 0
			runErr := session.Run(cmd)
			if runErr != nil {
				if exitErr, ok := runErr.(*ssh.ExitError); ok {
					exitCode = exitErr.ExitStatus()
				} else {
					return valpkg.ErrorVal("Befehl konnte nicht ausgeführt werden: " + runErr.Error())
				}
			}

			m := map[string]valpkg.Value{
				"stdout":   valpkg.StrVal(strings.TrimSpace(buf.String())),
				"exitCode": valpkg.NumVal(float64(exitCode)),
			}
			return valpkg.Value{Kind: valpkg.KindMap, Map: m}
		})

	// ------------------------
	// ssh.Close
	// ------------------------
	Register(ns+"Close", "ssh", "alias",
		"Schließt eine per ssh.Connect geöffnete Verbindung und entfernt den Alias.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("ssh.Close(alias) benötigt einen Alias")
			}
			alias := strings.ToLower(args[0].Str)

			sshMu.Lock()
			defer sshMu.Unlock()

			client, ok := sshConnections[alias]
			if !ok {
				return valpkg.BoolVal(false)
			}

			client.Close()
			delete(sshConnections, alias)

			return valpkg.BoolVal(true)
		})

	// ------------------------
	// ssh.ExecOnce
	// ------------------------
	// Für den CLI-Shortcut-Anwendungsfall gedacht: ein Shortcut kann immer
	// nur eine Funktion pro Aufruf mappen (ein Prozess, ein Funktionsaufruf,
	// Prozessende). ssh.Connect/ssh.Exec/ssh.Close sind für mehrere
	// Exec-Aufrufe INNERHALB eines laufenden VBX-Skripts gedacht (Alias
	// bleibt im Speicher, solange der Prozess läuft) - über zwei getrennte
	// Shortcut-Aufrufe (= zwei Prozesse) hinweg würde der Alias aber nicht
	// wiederverwendbar sein. ssh.ExecOnce kapselt Connect+Exec+Close in
	// einem Aufruf, kein explizites Close nötig (Prozessende räumt die
	// Verbindung ohnehin auf).
	Register(ns+"ExecOnce", "ssh", "host, user, keyPath, cmd, [printOutput], [knownHostsPath], [port]",
		"Verbindet, führt genau einen Befehl aus und schließt die Verbindung wieder - für Einzelaufrufe (z.B. CLI-Shortcuts), "+
			"bei denen sich ssh.Connect/ssh.Exec/ssh.Close (für mehrere Befehle in einem laufenden Skript gedacht) nicht lohnt. "+
			"printOutput (Default true) gibt stdout und Exit-Code zusätzlich direkt auf der Konsole aus - praktisch für "+
			"Shortcuts ohne eigenes Print, aber innerhalb eines Skripts oft unerwünscht: dort explizit False übergeben, "+
			"um nur die Rückgabe-Map (stdout, exitCode) selbst auszuwerten, ohne Konsolen-Rauschen. "+
			"Rückgabe: Map mit stdout, exitCode. port default 22, knownHostsPath optional.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.ErrorVal("ssh.ExecOnce(host, user, keyPath, cmd [, printOutput, knownHostsPath, port]) benötigt mindestens 4 Argumente")
			}

			host := args[0].Str
			user := args[1].Str
			keyPath := args[2].Str
			cmd := args[3].Str

			printOutput := true
			if len(args) >= 5 {
				printOutput = fileTruthy(args[4])
			}

			absKeyPath, errVal := absPathVal(keyPath)
			if errVal != nil {
				return *errVal
			}

			port := 22
			if len(args) >= 7 {
				port = int(valpkg.ToNum(args[6]))
			}

			keyBytes, err := os.ReadFile(absKeyPath)
			if err != nil {
				return valpkg.ErrorVal("Private Key konnte nicht gelesen werden: " + err.Error())
			}

			signer, err := ssh.ParsePrivateKey(keyBytes)
			if err != nil {
				return valpkg.ErrorVal("Private Key konnte nicht geparst werden: " + err.Error())
			}

			hostKeyCallback, errVal := buildHostKeyCallback(args, 5)
			if errVal != nil {
				return *errVal
			}

			config := &ssh.ClientConfig{
				User:            user,
				Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
				HostKeyCallback: hostKeyCallback,
				Timeout:         10 * time.Second,
			}

			addr := fmt.Sprintf("%s:%d", host, port)
			client, err := ssh.Dial("tcp", addr, config)
			if err != nil {
				return valpkg.ErrorVal("SSH-Verbindung fehlgeschlagen: " + err.Error())
			}
			defer client.Close()

			session, err := client.NewSession()
			if err != nil {
				return valpkg.ErrorVal("Session konnte nicht geöffnet werden: " + err.Error())
			}
			defer session.Close()

			var buf bytes.Buffer
			session.Stdout = &buf
			session.Stderr = &buf

			exitCode := 0
			runErr := session.Run(cmd)
			if runErr != nil {
				if exitErr, ok := runErr.(*ssh.ExitError); ok {
					exitCode = exitErr.ExitStatus()
				} else {
					return valpkg.ErrorVal("Befehl konnte nicht ausgeführt werden: " + runErr.Error())
				}
			}

			output := strings.TrimSpace(buf.String())

			if printOutput {
				if output != "" {
					fmt.Println(output)
				}
				fmt.Printf("Exit-Code: %d\n", exitCode)
			}

			m := map[string]valpkg.Value{
				"stdout":   valpkg.StrVal(output),
				"exitCode": valpkg.NumVal(float64(exitCode)),
			}
			return valpkg.Value{Kind: valpkg.KindMap, Map: m}
		})

	// ------------------------
	// ssh.RebootWithKey (Key-Auth)
	// ------------------------
	Register(ns+"RebootWithKey", "ssh", "host, user, keyPath, [delay], [knownHostsPath], [port]",
		"Löst per SSH-Key-Authentifizierung einen Reboot auf einem Zielsystem aus (shutdown -r). keyPath zeigt auf den "+
			"PRIVATEN Key lokal (z.B. aus GenerateSSHKey) - der öffentliche Key (.pub) muss vorher in ~/.ssh/authorized_keys "+
			"auf dem Zielserver eingetragen sein. "+
			"delay ist die Wartezeit in Sekunden bis zum Reboot (Default 30), damit die SSH-Session sauber schließt, bevor das System herunterfährt; "+
			"0 löst einen sofortigen Reboot aus. Mit knownHostsPath wird der Host-Key wie bei sftp.ConnectWithKey gegen eine known_hosts-Datei geprüft (optional). "+
			"port ist der SSH-Port (Default 22, nur bei Abweichung angeben).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.ErrorVal("ssh.RebootWithKey(host, user, keyPath [, delay, knownHostsPath, port]) benötigt mindestens 3 Argumente")
			}

			host := args[0].Str
			user := args[1].Str
			keyPath := args[2].Str

			delay := 30
			if len(args) >= 4 {
				delay = int(valpkg.ToNum(args[3]))
			}

			port := 22
			if len(args) >= 6 {
				port = int(valpkg.ToNum(args[5]))
			}

			absKeyPath, errVal := absPathVal(keyPath)
			if errVal != nil {
				return *errVal
			}

			keyBytes, err := os.ReadFile(absKeyPath)
			if err != nil {
				return valpkg.ErrorVal("Private Key konnte nicht gelesen werden: " + err.Error())
			}

			signer, err := ssh.ParsePrivateKey(keyBytes)
			if err != nil {
				return valpkg.ErrorVal("Private Key konnte nicht geparst werden: " + err.Error())
			}

			hostKeyCallback, errVal := buildHostKeyCallback(args, 4)
			if errVal != nil {
				return *errVal
			}

			config := &ssh.ClientConfig{
				User:            user,
				Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
				HostKeyCallback: hostKeyCallback,
				Timeout:         10 * time.Second,
			}

			return sshRebootExec(host, port, config, delay)
		})

	Register(ns+"GenerateSSHKey", "ssh", "[outFile, algo, bits, pass]",
		"Erstellt ein SSH-Paar (RSA/Ed25519). Schreibt physisch in .ssh oder Zielpfad.",
		func(args []valpkg.Value) valpkg.Value {
			// 1. DEFAULTS & DATEN-VORBEREITUNG
			home, _ := os.UserHomeDir()
			algo := "rsa"
			bits := 4096 // Absolute Untergrenze für RSA

			// 2. INPUT-CHECK & ALGO-WAHL
			if len(args) >= 2 && args[1].Str != "" {
				algo = strings.ToLower(args[1].Str)
			}

			// Bits korrigieren / setzen
			if algo == "ed25519" {
				bits = 256
			} else {
				algo = "rsa" // Fallback für Unbekanntes
				if len(args) >= 3 {
					uBits, _ := strconv.Atoi(args[2].Str)
					if uBits > bits {
						bits = uBits
					}
				}
			}

			// 3. PFAD-HANDLING (Erzwingt Verzeichnis-Struktur)
			var targetDir string
			if len(args) >= 1 && strings.TrimSpace(args[0].Str) != "" {
				// Wir nehmen den Input IMMER als Verzeichnis-Pfad
				targetDir, _ = absPathVal(args[0].Str)
			} else {
				// Default: ~/.ssh
				targetDir = filepath.Join(home, ".ssh")
			}

			// JETZT: Ordner erstellen, bevor wir überhaupt an Dateinamen denken!
			// Wenn targetDir "/root/ssh" ist, wird hier ein ORDNER namens "ssh" erstellt.
			if err := os.MkdirAll(targetDir, 0700); err != nil {
				return valpkg.ErrorVal("Berechtigungsfehler: Konnte Ordner nicht erstellen: " + err.Error())
			}

			// Erst wenn der Ordner sicher existiert, bauen wir den Dateipfad zusammen
			basePath := filepath.Join(targetDir, "id_"+algo+"_vbmini")

			// 4. SICHERHEITS-CHECK: EXISTENZ DER DATEI IM ORDNER
			privPath := basePath
			pubPath := basePath + ".pub"

			if _, err := os.Stat(privPath); err == nil {
				return valpkg.ErrorVal("Abbruch: Datei existiert bereits im Zielordner: " + privPath)
			}
			if _, err := os.Stat(pubPath); err == nil {
				return valpkg.ErrorVal("Abbruch: Public Key existiert bereits (" + pubPath + ").")
			}

			// --- AB HIER: PHYSIKALISCHE GENERIERUNG ---
			var privPem []byte
			var pubBytes []byte

			if algo == "ed25519" {
				// ED25519 LOGIK
				pub, priv, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					return valpkg.ErrorVal("Ed25519 Fehler: " + err.Error())
				}

				// MarshalPrivateKey liefert []byte (die rohen Daten)
				pemBlock, err := ssh.MarshalPrivateKey(priv, "")
				if err != nil {
					return valpkg.ErrorVal("Marshal Fehler: " + err.Error())
				}

				privPem = pem.EncodeToMemory(pemBlock)

				// Public Key Format
				pubKey, _ := ssh.NewPublicKey(pub)
				pubBytes = ssh.MarshalAuthorizedKey(pubKey)

			} else {
				// RSA LOGIK
				privateKey, err := rsa.GenerateKey(rand.Reader, bits)
				if err != nil {
					return valpkg.ErrorVal("RSA Fehler: " + err.Error())
				}

				// MarshalPKCS1PrivateKey liefert []byte
				rawPrivBytes := x509.MarshalPKCS1PrivateKey(privateKey)

				privPem = pem.EncodeToMemory(&pem.Block{
					Type:  "RSA PRIVATE KEY",
					Bytes: rawPrivBytes, // Korrekt: []byte in []byte Feld
				})

				// Public Key Format
				pubKey, _ := ssh.NewPublicKey(&privateKey.PublicKey)
				pubBytes = ssh.MarshalAuthorizedKey(pubKey)
			}

			// 5. SCHREIBEN DER DATEIEN
			// Private Key (0600)
			if err := os.WriteFile(privPath, privPem, 0600); err != nil {
				return valpkg.ErrorVal("Schreibfehler Private Key: " + err.Error())
			}

			// Public Key (0644)
			if err := os.WriteFile(pubPath, pubBytes, 0644); err != nil {
				return valpkg.ErrorVal("Schreibfehler Public Key: " + err.Error())
			}

			fmt.Printf("✔ SSH-Key (%s, %d Bit) erfolgreich erstellt.\n", algo, bits)
			//fmt.Printf("  Pfad: %s\n", outFile)

			return valpkg.StrVal(basePath)
		})
}

// getSSHConn holt eine bestehende Verbindung anhand des Alias.
func getSSHConn(alias string) (*ssh.Client, *valpkg.Value) {
	sshMu.RLock()
	defer sshMu.RUnlock()

	client, ok := sshConnections[alias]
	if !ok {
		v := valpkg.ErrorVal("SSH-Verbindung '" + alias + "' nicht gefunden. Zuerst ssh.Connect() aufrufen.")
		return nil, &v
	}
	return client, nil
}

// sshRebootExec baut die Verbindung auf (config enthält bereits die
// gewählte Auth-Methode), führt den Reboot-Befehl aus und liefert das
// BuiltinInfo-kompatible valpkg.Value zurück.
func sshRebootExec(host string, port int, config *ssh.ClientConfig, delay int) valpkg.Value {
	addr := fmt.Sprintf("%s:%d", host, port)
	sshConn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return valpkg.ErrorVal("SSH-Verbindung fehlgeschlagen: " + err.Error())
	}
	defer sshConn.Close()

	cmd := sshRebootCmd(delay)
	_, err = sshExecOnce(sshConn, cmd)
	// Bei delay>0 schließt die Session sauber, ein Fehler hier ist echt.
	// Bei delay=0 kann die Verbindung während des Reboots abreißen -
	// das ist erwartetes Verhalten und kein Fehlerfall.
	if err != nil && delay > 0 {
		return valpkg.ErrorVal("Reboot-Befehl fehlgeschlagen: " + err.Error())
	}

	return valpkg.BoolVal(true)
}

// sshRebootCmd baut den Shell-Befehl für einen verzögerten bzw. sofortigen
// Reboot. shutdown -r +N statt direktem reboot, damit die SSH-Session
// sauber schließt, bevor das System herunterfährt.
func sshRebootCmd(delaySeconds int) string {
	if delaySeconds <= 0 {
		return "sudo shutdown -r now"
	}
	mins := delaySeconds / 60
	if mins < 1 {
		mins = 1
	}
	return fmt.Sprintf("sudo shutdown -r +%d 'VBX: Automatischer Reboot ausgeloest'", mins)
}

// sshExecOnce öffnet eine einzelne Session auf einer bestehenden
// ssh.Client-Verbindung, führt cmd aus und schließt die Session wieder.
func sshExecOnce(conn *ssh.Client, cmd string) (string, error) {
	session, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf

	err = session.Run(cmd)
	return strings.TrimSpace(buf.String()), err
}

func init() { initSSHFunctions() }
