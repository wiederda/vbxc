//go:build windows

package stdlib

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	valpkg "vbxc/value"
)

var (
	modVersion   = syscall.NewLazyDLL("version.dll")
	procVerSize  = modVersion.NewProc("GetFileVersionInfoSizeW")
	procVerInfo  = modVersion.NewProc("GetFileVersionInfoW")
	procVerQuery = modVersion.NewProc("VerQueryValueW")
)

type vsFixedFileInfo struct {
	Signature, StrucVersion                    uint32
	FileVersionMS, FileVersionLS               uint32
	ProductVersionMS, ProductVersionLS         uint32
	FileFlagsMask, FileFlags, FileOS, FileType uint32
	FileSubtype, FileDateMS, FileDateLS        uint32
}

func watchLogInternal(path, pattern, ansiSequence string, silent bool, timeoutMs int) valpkg.Value {
	// Wir nutzen hier kein defer file.Close(), da wir das Handle bei Rotation selbst schließen müssen
	var file *os.File
	var lastPos int64

	// Hilfsfunktion zum "Shared" Öffnen
	openShared := func(p string) (*os.File, error) {
		pathPtr, _ := syscall.UTF16PtrFromString(p)
		handle, err := syscall.CreateFile(
			pathPtr,
			syscall.GENERIC_READ,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
			nil,
			syscall.OPEN_EXISTING,
			syscall.FILE_ATTRIBUTE_NORMAL,
			0,
		)
		if err != nil {
			return nil, err
		}
		return os.NewFile(uintptr(handle), p), nil
	}

	// Initiales Öffnen
	f, err := openShared(path)
	if err != nil {
		return valpkg.ErrorVal("Datei konnte nicht geöffnet werden: " + err.Error())
	}
	file = f
	info, _ := file.Stat()
	lastPos = info.Size()

	if !silent {
		fmt.Printf("\033[90m[WATCH] Aktiv: %s | Filter: '%s'\033[0m\n", path, pattern)
	}

	start := time.Now()

	for {
		// --- NEU: Timeout-Check (nur im silent-Modus relevant) ---
		if silent && timeoutMs > 0 && time.Since(start).Milliseconds() > int64(timeoutMs) {
			return valpkg.BoolVal(false)
		}

		currInfo, err := os.Stat(path)
		if err != nil {
			// Datei kurzzeitig weg (z.B. während Rotation)? Warten.
			time.Sleep(200 * time.Millisecond)
			continue
		}

		currSize := currInfo.Size()

		// ROTATION CHECK
		if currSize < lastPos {
			if !silent {
				fmt.Println("\033[90m[WATCH] Log-Rotation erkannt. Starte neu...\033[0m")
			}
			file.Close()
			f, err := openShared(path)
			if err == nil {
				file = f
				lastPos = 0 // Von vorn anfangen in der neuen Datei
			}
			continue
		}

		if currSize > lastPos {
			newBytes := make([]byte, currSize-lastPos)
			// ReadAt ist super, weil es den internen Cursor nicht bewegt
			_, err := file.ReadAt(newBytes, lastPos)
			if err == nil {
				lines := strings.Split(string(newBytes), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && strings.Contains(strings.ToLower(line), strings.ToLower(pattern)) {
						if silent {
							// --- NEU: sofort zurückgeben, kein Print ---
							return valpkg.BoolVal(true)
						}
						ts := time.Now().Format("15:04:05")
						if ansiSequence != "" {
							fmt.Printf("%s[%s] %s\033[0m\n", ansiSequence, ts, line)
						} else {
							fmt.Printf("[%s] %s\n", ts, line)
						}
					}
				}
			}
			lastPos = currSize
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// openFileShared öffnet eine Datei unter Windows so, dass andere Prozesse
// (wie Wildfly) weiterhin darin schreiben, lesen oder sie löschen können.
func openFileShared(path string) (*os.File, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)

	if err != nil {
		return nil, err
	}

	// Verwandelt das Win32-Handle in ein Standard Go *os.File Objekt
	return os.NewFile(uintptr(handle), path), nil
}

// productVersion liest die numerische Produktversion (VS_FIXEDFILEINFO) einer exe/dll.
func productVersion(path string) (string, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}

	var handle uint32
	size, _, e := procVerSize.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&handle)))
	if size == 0 {
		return "", fmt.Errorf("keine Versionsinfo gefunden: %v", e)
	}

	buf := make([]byte, size)
	r, _, e := procVerInfo.Call(uintptr(unsafe.Pointer(p)), 0, size, uintptr(unsafe.Pointer(&buf[0])))
	if r == 0 {
		return "", fmt.Errorf("GetFileVersionInfo: %v", e)
	}

	sub, _ := syscall.UTF16PtrFromString(`\`)
	var ptr unsafe.Pointer
	var l uint32
	r, _, e = procVerQuery.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(sub)),
		uintptr(unsafe.Pointer(&ptr)),
		uintptr(unsafe.Pointer(&l)),
	)
	if r == 0 || ptr == nil {
		return "", fmt.Errorf("VerQueryValue: %v", e)
	}

	ffi := (*vsFixedFileInfo)(ptr)
	return fmt.Sprintf("%d.%d.%d.%d",
		ffi.ProductVersionMS>>16, ffi.ProductVersionMS&0xFFFF,
		ffi.ProductVersionLS>>16, ffi.ProductVersionLS&0xFFFF), nil
}
