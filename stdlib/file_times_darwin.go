//go:build darwin
// +build darwin

package stdlib

import (
	"os"
	"syscall"
	"time"
)

func FileTimes(path string) (create, access, modify time.Time, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	stat := info.Sys().(*syscall.Stat_t)
	create = time.Unix(int64(stat.Ctimespec.Sec), int64(stat.Ctimespec.Nsec))
	access = time.Unix(int64(stat.Atimespec.Sec), int64(stat.Atimespec.Nsec))
	modify = info.ModTime()
	return
}

func fileNamespaceFolderTimes(path string) (create, access, modify time.Time, err error) {
	return FileTimes(path)
}
