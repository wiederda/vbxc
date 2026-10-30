// ------------------------
// stdlib_sftp.go
// ------------------------

package stdlib

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	valpkg "vbxc/value"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SftpConn struct {
	client  *sftp.Client
	sshConn *ssh.Client
}

var (
	sftpConnections = make(map[string]*SftpConn)
	sftpMu          sync.RWMutex
)

func initSftpFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "sftp."

	// ------------------------
	// sftp.Connect
	// ------------------------
	Register(ns+"Connect", "sftp", "host, user, password, alias, [knownHostsPath], [port]",
		"Öffnet eine SFTP-Verbindung per Passwort-Authentifizierung und speichert sie unter einem Alias. Mit knownHostsPath wird der Host-Key gegen eine known_hosts-Datei geprüft (empfohlen bei Verbindungen außerhalb des eigenen Netzes). port ist der SSH-Port (Default 22, nur bei Abweichung angeben).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.ErrorVal("sftp.Connect(host, user, password, alias [, knownHostsPath, port]) benötigt mindestens 4 Argumente")
			}

			host := args[0].Str
			user := args[1].Str
			password := args[2].Str
			alias := strings.ToLower(args[3].Str)

			port := 22
			if len(args) >= 6 {
				port = int(valpkg.ToNum(args[5]))
			}

			hostKeyCallback, errVal := buildHostKeyCallback(args, 4)
			if errVal != nil {
				return *errVal
			}

			config := &ssh.ClientConfig{
				User: user,
				Auth: []ssh.AuthMethod{
					ssh.Password(password),
				},
				HostKeyCallback: hostKeyCallback,
				Timeout:         10 * time.Second,
			}

			addr := fmt.Sprintf("%s:%d", host, port)
			sshConn, err := ssh.Dial("tcp", addr, config)
			if err != nil {
				return valpkg.ErrorVal("SSH-Verbindung fehlgeschlagen: " + err.Error())
			}

			client, err := sftp.NewClient(sshConn)
			if err != nil {
				sshConn.Close()
				return valpkg.ErrorVal("SFTP-Client konnte nicht erstellt werden: " + err.Error())
			}

			sftpMu.Lock()
			if old, ok := sftpConnections[alias]; ok {
				old.client.Close()
				old.sshConn.Close()
			}
			sftpConnections[alias] = &SftpConn{client: client, sshConn: sshConn}
			sftpMu.Unlock()

			return valpkg.BoolVal(true)
		})

	// ------------------------
	// sftp.ConnectWithKey
	// ------------------------
	Register(ns+"ConnectWithKey", "sftp", "host, user, keyPath, alias, [knownHostsPath], [port]",
		"Öffnet eine SFTP-Verbindung per Private-Key-Authentifizierung (z.B. aus global.GenerateSSHKey) und speichert sie unter einem Alias. port ist der SSH-Port (Default 22, nur bei Abweichung angeben).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.ErrorVal("sftp.ConnectWithKey(host, user, keyPath, alias [, knownHostsPath, port]) benötigt mindestens 4 Argumente")
			}

			host := args[0].Str
			user := args[1].Str
			keyPath := args[2].Str
			alias := strings.ToLower(args[3].Str)

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
				User: user,
				Auth: []ssh.AuthMethod{
					ssh.PublicKeys(signer),
				},
				HostKeyCallback: hostKeyCallback,
				Timeout:         10 * time.Second,
			}

			addr := fmt.Sprintf("%s:%d", host, port)
			sshConn, err := ssh.Dial("tcp", addr, config)
			if err != nil {
				return valpkg.ErrorVal("SSH-Verbindung fehlgeschlagen: " + err.Error())
			}

			client, err := sftp.NewClient(sshConn)
			if err != nil {
				sshConn.Close()
				return valpkg.ErrorVal("SFTP-Client konnte nicht erstellt werden: " + err.Error())
			}

			sftpMu.Lock()
			if old, ok := sftpConnections[alias]; ok {
				old.client.Close()
				old.sshConn.Close()
			}
			sftpConnections[alias] = &SftpConn{client: client, sshConn: sshConn}
			sftpMu.Unlock()

			return valpkg.BoolVal(true)
		})

	// ------------------------
	// sftp.Close
	// ------------------------
	Register(ns+"Close", "sftp", "alias",
		"Schließt eine SFTP-Verbindung und entfernt den Alias.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("sftp.Close(alias) benötigt einen Alias")
			}
			alias := strings.ToLower(args[0].Str)

			sftpMu.Lock()
			defer sftpMu.Unlock()

			conn, ok := sftpConnections[alias]
			if !ok {
				return valpkg.BoolVal(false)
			}

			conn.client.Close()
			conn.sshConn.Close()
			delete(sftpConnections, alias)

			return valpkg.BoolVal(true)
		})

	// ------------------------
	// sftp.Upload
	// ------------------------
	Register(ns+"Upload", "sftp", "alias, localPath, remotePath",
		"Lädt eine lokale Datei auf den SFTP-Server hoch.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.ErrorVal("sftp.Upload(alias, localPath, remotePath) benötigt 3 Argumente")
			}
			alias := strings.ToLower(args[0].Str)
			localPath := args[1].Str
			remotePath := args[2].Str

			conn, errVal := getSftpConn(alias)
			if errVal != nil {
				return *errVal
			}

			absLocal, errVal := absPathVal(localPath)
			if errVal != nil {
				return *errVal
			}

			localFile, err := os.Open(absLocal)
			if err != nil {
				return valpkg.ErrorVal("Lokale Datei konnte nicht geöffnet werden: " + err.Error())
			}
			defer localFile.Close()

			remoteFile, err := conn.client.Create(remotePath)
			if err != nil {
				return valpkg.ErrorVal("Remote-Datei konnte nicht erstellt werden: " + err.Error())
			}
			defer remoteFile.Close()

			written, err := io.Copy(remoteFile, localFile)
			if err != nil {
				return valpkg.ErrorVal("Upload fehlgeschlagen: " + err.Error())
			}

			return valpkg.NumVal(float64(written))
		})

	// ------------------------
	// sftp.Download
	// ------------------------
	Register(ns+"Download", "sftp", "alias, remotePath, localPath",
		"Lädt eine Datei vom SFTP-Server herunter.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.ErrorVal("sftp.Download(alias, remotePath, localPath) benötigt 3 Argumente")
			}
			alias := strings.ToLower(args[0].Str)
			remotePath := args[1].Str
			localPath := args[2].Str

			conn, errVal := getSftpConn(alias)
			if errVal != nil {
				return *errVal
			}

			absLocal, errVal := absPathVal(localPath)
			if errVal != nil {
				return *errVal
			}

			if _, err := os.Stat(filepath.Dir(absLocal)); err != nil {
				return valpkg.ErrorVal("Zielverzeichnis existiert nicht: " + filepath.Dir(absLocal))
			}

			remoteFile, err := conn.client.Open(remotePath)
			if err != nil {
				return valpkg.ErrorVal("Remote-Datei konnte nicht geöffnet werden: " + err.Error())
			}
			defer remoteFile.Close()

			localFile, err := os.Create(absLocal)
			if err != nil {
				return valpkg.ErrorVal("Lokale Datei konnte nicht erstellt werden: " + err.Error())
			}
			defer localFile.Close()

			written, err := io.Copy(localFile, remoteFile)
			if err != nil {
				return valpkg.ErrorVal("Download fehlgeschlagen: " + err.Error())
			}

			return valpkg.NumVal(float64(written))
		})

	// ------------------------
	// sftp.DownloadFolder
	// ------------------------
	Register(ns+"DownloadFolder", "sftp", "alias, remotePath, localBasePath, [recursive], [progress]",
		"Lädt einen kompletten Remote-Ordner herunter. Legt lokal einen Ordner mit demselben Namen wie das letzte Segment von remotePath an (falls nicht vorhanden) und lädt alle enthaltenen Dateien hinein.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.ErrorVal("sftp.DownloadFolder(alias, remotePath, localBasePath [, recursive, progress]) benötigt mindestens 3 Argumente")
			}
			alias := strings.ToLower(args[0].Str)
			remotePath := strings.TrimSuffix(args[1].Str, "/")
			localBasePath := args[2].Str

			recursive := false
			if len(args) >= 4 {
				recursive = fileTruthy(args[3])
			}

			showProgress := false
			if len(args) >= 5 {
				showProgress = fileTruthy(args[4])
			}

			conn, errVal := getSftpConn(alias)
			if errVal != nil {
				return *errVal
			}

			absLocalBase, errVal := absPathVal(localBasePath)
			if errVal != nil {
				return *errVal
			}

			folderName := path.Base(remotePath)
			localTarget := filepath.Join(absLocalBase, folderName)

			if err := os.MkdirAll(localTarget, 0755); err != nil {
				return valpkg.ErrorVal("Lokaler Zielordner konnte nicht angelegt werden: " + err.Error())
			}

			// Erst zählen, damit die Fortschrittsanzeige "X von Y" zeigen kann,
			// statt nur einen laufenden Zähler ohne Gesamtzahl.
			total := 0
			if showProgress {
				total, _ = countRemoteFiles(conn, remotePath, recursive)
			}

			current := 0
			count, err := downloadFolderRecursiveP(conn, remotePath, localTarget, recursive, showProgress, total, &current)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			return valpkg.NumVal(float64(count))
		})

	// ------------------------
	// sftp.FindByExt
	// ------------------------
	Register(ns+"FindByExt", "sftp", "alias, remotePath, ext, [all]",
		"Sucht Dateien mit einer bestimmten Endung in einem Remote-Verzeichnis, ohne den genauen Dateinamen zu kennen.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.ErrorVal("sftp.FindByExt(alias, remotePath, ext [, all]) benötigt mindestens 3 Argumente")
			}
			alias := strings.ToLower(args[0].Str)
			remotePath := args[1].Str
			ext := strings.ToLower(strings.TrimPrefix(args[2].Str, "."))

			all := false
			if len(args) >= 4 {
				all = fileTruthy(args[3])
			}

			conn, errVal := getSftpConn(alias)
			if errVal != nil {
				return *errVal
			}

			entries, err := conn.client.ReadDir(remotePath)
			if err != nil {
				return valpkg.ErrorVal("Verzeichnis konnte nicht gelesen werden: " + err.Error())
			}

			var treffer []string
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				fileExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
				if fileExt == ext {
					treffer = append(treffer, remotePath+"/"+name)
				}
			}

			sort.SliceStable(treffer, func(i, j int) bool {
				return naturalLess(treffer[i], treffer[j])
			})

			if all {
				result := make([]valpkg.Value, len(treffer))
				for i, t := range treffer {
					result[i] = valpkg.StrVal(t)
				}
				return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
			}

			if len(treffer) == 0 {
				return valpkg.StrVal("")
			}
			return valpkg.StrVal(treffer[0])
		})

	// ------------------------
	// sftp.DownloadByExt
	// ------------------------
	Register(ns+"DownloadByExt", "sftp", "alias, remotePath, ext, localBasePath, [all]",
		"Sucht Dateien mit einer bestimmten Endung in einem Remote-Verzeichnis und lädt sie direkt herunter, ohne die genauen Dateinamen zu kennen. Kombiniert sftp.FindByExt und sftp.Download in einem Aufruf.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.ErrorVal("sftp.DownloadByExt(alias, remotePath, ext, localBasePath [, all]) benötigt mindestens 4 Argumente")
			}
			alias := strings.ToLower(args[0].Str)
			remotePath := args[1].Str
			ext := strings.ToLower(strings.TrimPrefix(args[2].Str, "."))
			localBasePath := args[3].Str

			all := false
			if len(args) >= 5 {
				all = fileTruthy(args[4])
			}

			conn, errVal := getSftpConn(alias)
			if errVal != nil {
				return *errVal
			}

			absLocalBase, errVal := absPathVal(localBasePath)
			if errVal != nil {
				return *errVal
			}

			if err := os.MkdirAll(absLocalBase, 0755); err != nil {
				return valpkg.ErrorVal("Lokales Zielverzeichnis konnte nicht angelegt werden: " + err.Error())
			}

			entries, err := conn.client.ReadDir(remotePath)
			if err != nil {
				return valpkg.ErrorVal("Verzeichnis konnte nicht gelesen werden: " + err.Error())
			}

			var treffer []string
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				fileExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(e.Name()), "."))
				if fileExt == ext {
					treffer = append(treffer, e.Name())
				}
			}

			sort.SliceStable(treffer, func(i, j int) bool {
				return naturalLess(treffer[i], treffer[j])
			})

			if len(treffer) == 0 {
				return valpkg.NumVal(0)
			}

			// Ohne 'all': nur die erste (natürlich sortierte) Datei laden
			if !all {
				treffer = treffer[:1]
			}

			downloaded := 0
			for _, name := range treffer {
				remoteFilePath := remotePath + "/" + name
				localFilePath := filepath.Join(absLocalBase, name)

				remoteFile, err := conn.client.Open(remoteFilePath)
				if err != nil {
					return valpkg.ErrorVal(fmt.Sprintf("Remote-Datei konnte nicht geöffnet werden (%s): %s", name, err.Error()))
				}

				localFile, err := os.Create(localFilePath)
				if err != nil {
					remoteFile.Close()
					return valpkg.ErrorVal(fmt.Sprintf("Lokale Datei konnte nicht erstellt werden (%s): %s", name, err.Error()))
				}

				_, copyErr := io.Copy(localFile, remoteFile)
				remoteFile.Close()
				localFile.Close()

				if copyErr != nil {
					return valpkg.ErrorVal(fmt.Sprintf("Download fehlgeschlagen (%s): %s", name, copyErr.Error()))
				}

				downloaded++
			}

			return valpkg.NumVal(float64(downloaded))
		})

	// ------------------------
	// sftp.List
	// ------------------------
	Register(ns+"List", "sftp", "alias, remotePath, [sortBy], [desc]",
		"Listet den Inhalt eines Remote-Verzeichnisses. Gibt ein Array von Maps zurück (name, size, isDir, modTime). Optional direkt sortiert.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("sftp.List(alias, remotePath [, sortBy, desc]) benötigt Alias und Pfad")
			}
			alias := strings.ToLower(args[0].Str)
			remotePath := args[1].Str

			sortBy := ""
			if len(args) >= 3 {
				sortBy = args[2].Str
			}
			desc := false
			if len(args) >= 4 {
				desc = fileTruthy(args[3])
			}

			conn, errVal := getSftpConn(alias)
			if errVal != nil {
				return *errVal
			}

			entries, err := conn.client.ReadDir(remotePath)
			if err != nil {
				return valpkg.ErrorVal("Verzeichnis konnte nicht gelesen werden: " + err.Error())
			}

			if sortBy != "" {
				sort.SliceStable(entries, func(i, j int) bool {
					var less bool
					switch strings.ToLower(sortBy) {
					case "size":
						less = entries[i].Size() < entries[j].Size()
					case "modtime":
						less = entries[i].ModTime().Before(entries[j].ModTime())
					default: // "name"
						less = entries[i].Name() < entries[j].Name()
					}
					if desc {
						return !less
					}
					return less
				})
			}

			result := make([]valpkg.Value, len(entries))
			for i, e := range entries {
				m := map[string]valpkg.Value{
					"name":    valpkg.StrVal(e.Name()),
					"size":    valpkg.NumVal(float64(e.Size())),
					"isDir":   valpkg.BoolVal(e.IsDir()),
					"modTime": valpkg.StrVal(e.ModTime().Format(time.RFC3339)),
				}
				result[i] = valpkg.Value{Kind: valpkg.KindMap, Map: m}
			}

			return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
		})
}

// getSftpConn holt eine bestehende Verbindung anhand des Alias.
func getSftpConn(alias string) (*SftpConn, *valpkg.Value) {
	sftpMu.RLock()
	defer sftpMu.RUnlock()

	conn, ok := sftpConnections[alias]
	if !ok {
		v := valpkg.ErrorVal("SFTP-Verbindung '" + alias + "' nicht gefunden. Zuerst sftp.Connect() aufrufen.")
		return nil, &v
	}
	return conn, nil
}

func countRemoteFiles(conn *SftpConn, remotePath string, recursive bool) (int, error) {
	entries, err := conn.client.ReadDir(remotePath)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			if recursive {
				subCount, err := countRemoteFiles(conn, remotePath+"/"+entry.Name(), true)
				if err != nil {
					return count, err
				}
				count += subCount
			}
			continue
		}
		count++
	}
	return count, nil
}

func downloadFolderRecursiveP(conn *SftpConn, remotePath, localTarget string, recursive, showProgress bool, total int, current *int) (int, error) {
	entries, err := conn.client.ReadDir(remotePath)
	if err != nil {
		return 0, fmt.Errorf("Verzeichnis konnte nicht gelesen werden (%s): %w", remotePath, err)
	}
	count := 0
	for _, entry := range entries {
		remoteItemPath := remotePath + "/" + entry.Name()
		if entry.IsDir() {
			if !recursive {
				continue
			}
			localSubTarget := filepath.Join(localTarget, entry.Name())
			if err := os.MkdirAll(localSubTarget, 0755); err != nil {
				return count, fmt.Errorf("Unterordner konnte nicht angelegt werden (%s): %w", localSubTarget, err)
			}
			subCount, err := downloadFolderRecursiveP(conn, remoteItemPath, localSubTarget, true, showProgress, total, current)
			count += subCount
			if err != nil {
				return count, err
			}
			continue
		}
		localFilePath := filepath.Join(localTarget, entry.Name())
		if showProgress {
			*current++
			fmt.Printf("[%d/%d] %s\n", *current, total, entry.Name())
		}
		remoteFile, err := conn.client.Open(remoteItemPath)
		if err != nil {
			return count, fmt.Errorf("Remote-Datei konnte nicht geöffnet werden (%s): %w", remoteItemPath, err)
		}
		localFile, err := os.Create(localFilePath)
		if err != nil {
			remoteFile.Close()
			return count, fmt.Errorf("Lokale Datei konnte nicht erstellt werden (%s): %w", localFilePath, err)
		}
		_, copyErr := io.Copy(localFile, remoteFile)
		remoteFile.Close()
		localFile.Close()
		if copyErr != nil {
			return count, fmt.Errorf("Download fehlgeschlagen (%s): %w", entry.Name(), copyErr)
		}
		count++
	}
	return count, nil
}

func init() { initSftpFunctions() }
