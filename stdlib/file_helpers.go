package stdlib

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	valpkg "vbxc/value"
)

func fileLongPath(path string) string {
	if runtime.GOOS == "windows" && !strings.HasPrefix(path, `\\?\`) {
		if absolute, err := filepath.Abs(path); err == nil && len(absolute) >= 248 {
			return `\\?\` + absolute
		}
	}
	return path
}

func fileTruthy(v valpkg.Value) bool {
	switch v.Kind {
	case valpkg.KindBool:
		return v.Bool
	case valpkg.KindNum:
		return v.Num != 0
	case valpkg.KindStr:
		return v.Str != ""
	case valpkg.KindArr:
		return len(v.Arr) != 0 || len(v.Arr2D) != 0
	case valpkg.KindObj:
		return v.Obj != nil && len(v.Obj.Fields) != 0
	default:
		return false
	}
}
func fileFormatSize(size float64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%.0f B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", size/float64(div), "KMGTPE"[exp])
}
func createSymlinkInternal(target, linkPath string, replaceExisting bool) valpkg.Value {
	t, err := filepath.Abs(target)
	if err != nil {
		return valpkg.ErrorVal(err.Error())
	}
	l, err := filepath.Abs(linkPath)
	if err != nil {
		return valpkg.ErrorVal(err.Error())
	}
	info, err := os.Stat(t)
	if err != nil {
		return valpkg.BoolVal(false)
	}
	if info.IsDir() {
		return valpkg.ErrorVal("file.CreateSymlink: Ziel ist keine Datei")
	}
	if replaceExisting {
		if _, err = os.Lstat(l); err == nil {
			if err = os.Remove(l); err != nil {
				return valpkg.BoolVal(false)
			}
		}
	}
	if err = os.Symlink(t, l); err != nil {
		return valpkg.BoolVal(false)
	}
	return valpkg.BoolVal(true)
}
func fileStringSafe(v valpkg.Value, def string) string {
	s := valpkg.ToString(v)
	if s == "" {
		return def
	}
	return s
}

func getArg(args []valpkg.Value, i int) valpkg.Value {
	if i >= 0 && i < len(args) {
		return args[i]
	}
	return valpkg.Value{Kind: valpkg.KindInvalid}
}

func formatHuman(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	div, exp := int64(1024), 0
	for n := b / 1024; n >= 1024; n /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.2f %s", float64(b)/float64(div), units[exp])
}

func copyFileInternalBuffered(src, dst string, mode os.FileMode) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(dst)
		}
	}()
	_, err = io.Copy(out, in)
	return err
}

func FolderTimes(path string) (create, access, modify time.Time, err error) {
	return fileNamespaceFolderTimes(path)
}

func processTargetPath(p string) (string, *valpkg.Value) { return absPathVal(p) }

func getAnsiCode(v float64) string {
	switch int(v) {
	case -1:
		return "\033[0m"
	case 100:
		return "\033[1m"
	case 200:
		return "\033[4m"
	case 300:
		return "\033[7m"
	}
	if v >= 1000 {
		idx := int(v) / 1000
		if idx <= 7 {
			return fmt.Sprintf("\033[%dm", 40+idx)
		}
		return fmt.Sprintf("\033[48;5;%dm", idx)
	}
	if v >= 0 && v <= 7 {
		return fmt.Sprintf("\033[%dm", 30+int(v))
	}
	if v >= 8 && v <= 15 {
		return fmt.Sprintf("\033[%dm", 90+(int(v)-8))
	}
	return ""
}
