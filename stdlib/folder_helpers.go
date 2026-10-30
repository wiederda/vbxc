package stdlib

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	valpkg "vbxc/value"
)

func fileHash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:]), nil
}

func quickHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b := make([]byte, 4096)
	n, _ := f.Read(b)
	h := sha256.Sum256(b[:n])
	return string(h[:]), nil
}

func FolderEmpty(args []valpkg.Value) valpkg.Value {
	if len(args) == 0 || args[0].Str == "" {
		return valpkg.ErrorVal("folder.EmptyFolder benötigt einen Pfad")
	}
	path, errVal := absPathVal(args[0].Str)
	if errVal != nil {
		return *errVal
	}
	force := len(args) > 1 && valpkg.ToBool(args[1])
	if filepath.Dir(path) == path {
		return valpkg.ErrorVal("folder.EmptyFolder: Wurzelverzeichnis wird nicht geleert: " + path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return valpkg.ErrorVal("Ordner konnte nicht gelesen werden: " + err.Error())
	}
	var failed []valpkg.Value
	for _, e := range entries {
		p := filepath.Join(path, e.Name())
		if force {
			mode := os.FileMode(0666)
			if e.IsDir() {
				mode = 0777
			}
			_ = os.Chmod(p, mode)
		}
		if os.RemoveAll(p) != nil {
			failed = append(failed, valpkg.StrVal(e.Name()))
		}
	}
	return valpkg.Value{Kind: valpkg.KindArr, Arr: failed}
}

func FolderIsEmpty(args []valpkg.Value) valpkg.Value {
	if len(args) == 0 {
		return valpkg.ErrorVal("FolderIsEmpty benötigt einen Pfad")
	}
	path, errVal := absPathVal(args[0].Str)
	if errVal != nil {
		return *errVal
	}
	f, err := os.Open(path)
	if err != nil {
		return valpkg.ErrorVal("Fehler beim Öffnen des Ordners: " + err.Error())
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return valpkg.BoolVal(true)
	}
	if err != nil {
		return valpkg.ErrorVal("Fehler beim Lesen des Ordners: " + err.Error())
	}
	return valpkg.BoolVal(false)
}

func folderIgnoreNames(v valpkg.Value) map[string]bool {
	out := map[string]bool{}
	if v.Kind == valpkg.KindArr {
		for _, x := range v.Arr {
			out[valpkg.ToString(x)] = true
		}
		return out
	}
	for _, x := range strings.Split(v.Str, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out[x] = true
		}
	}
	return out
}
