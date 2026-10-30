//go:build windows
// +build windows

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
	stat := info.Sys().(*syscall.Win32FileAttributeData)
	create = time.Unix(0, stat.CreationTime.Nanoseconds())
	access = time.Unix(0, stat.LastAccessTime.Nanoseconds())
	modify = info.ModTime()
	return
}

func fileNamespaceFolderTimes(path string) (create, access, modify time.Time, err error) {
	return FileTimes(path)
}
