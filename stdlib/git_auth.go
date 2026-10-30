package stdlib

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"vbxc/value"
)

func buildHostKeyCallback(args []value.Value, idx int) (ssh.HostKeyCallback, *value.Value) {
	if len(args) <= idx || args[idx].Str == "" {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	path, errVal := absPathVal(args[idx].Str)
	if errVal != nil {
		return nil, errVal
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			v := value.ErrorVal("known_hosts-Verzeichnis konnte nicht erstellt werden: " + err.Error())
			return nil, &v
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			v := value.ErrorVal("known_hosts-Datei konnte nicht erstellt werden: " + err.Error())
			return nil, &v
		}
		_ = f.Close()
	}
	base, err := knownhosts.New(path)
	if err != nil {
		v := value.ErrorVal("known_hosts-Datei konnte nicht geladen werden: " + err.Error())
		return nil, &v
	}
	callback := ssh.HostKeyCallback(func(host string, remote net.Addr, key ssh.PublicKey) error {
		err := base(host, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
			if backupErr := gitBackupFileOld(path); backupErr != nil {
				return fmt.Errorf("Backup von known_hosts fehlgeschlagen: %w", backupErr)
			}
			line := knownhosts.Line([]string{knownhosts.Normalize(host)}, key)
			f, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
			if openErr != nil {
				return fmt.Errorf("known_hosts konnte nicht zum Schreiben geöffnet werden: %w", openErr)
			}
			defer f.Close()
			if _, writeErr := f.WriteString(line + "\n"); writeErr != nil {
				return fmt.Errorf("Host-Key konnte nicht gespeichert werden: %w", writeErr)
			}
			return nil
		}
		return err
	})
	return callback, nil
}

func gitBackupFileOld(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() == 0 {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".old", b, 0600)
}
