//go:build windows
// +build windows

package stdlib

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// ---------------- Disk Info ----------------

type DiskInfo struct {
	Total uint64
	Free  uint64
	Used  uint64
}

// ---------------- Admin Check ----------------

func isAdmin() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false
	}
	member, err := token.IsMember(adminSID) // NEU: token statt tok
	if err != nil {
		return false
	}
	return member
}

var (
	computerKernel32           = syscall.NewLazyDLL("computerKernel32.dll")
	computerProcGetTickCount64 = computerKernel32.NewProc("GetTickCount64")
	procGetLogicalDrives       = computerKernel32.NewProc("GetLogicalDrives")
)

func getOSID() string {
	return "windows"
}

func checkNeedsReboot() bool {
	// Standardmäßig false für Windows (oder später Registry-Check einbauen)
	return false
}

func getUptimeSeconds() float64 {
	// GetTickCount64 gibt Millisekunden seit Systemstart zurück
	ret, _, _ := computerProcGetTickCount64.Call()
	return float64(ret) / 1000.0
}

func FindAvailableUSB() (bool, string, string, string) { // 4 Rückgabewerte
	bitmask, err := windows.GetLogicalDrives()
	if err != nil {
		return false, "", "", ""
	}

	for i := 0; i < 26; i++ {
		if bitmask&(1<<uint(i)) != 0 {
			drive := string('A'+uint8(i)) + ":\\"
			lp, _ := windows.UTF16PtrFromString(drive)

			if windows.GetDriveType(lp) == windows.DRIVE_REMOVABLE {
				ready, name, fs := GetDriveDetails(drive) // fs wird hier empfangen
				if ready {
					return true, drive, name, fs // fs wird hier benutzt -> Compiler glücklich!
				}
			}
		}
	}
	return false, "", "", ""
}

func GetDriveDetails(path string) (ready bool, name string, fs string) {
	drive := path
	if !strings.HasSuffix(drive, "\\") {
		drive += "\\"
	}

	ptr, err := windows.UTF16PtrFromString(drive)
	if err != nil {
		return false, "", ""
	}

	var volumeName [256]uint16
	var fsName [256]uint16
	var serial, maxLen, flags uint32

	err = windows.GetVolumeInformation(
		ptr,
		&volumeName[0], uint32(len(volumeName)),
		&serial,
		&maxLen,
		&flags,
		&fsName[0], uint32(len(fsName)),
	)

	if err != nil {
		return false, "", ""
	}

	return true, windows.UTF16ToString(volumeName[:]), windows.UTF16ToString(fsName[:])
}

// ---------------- DiskSpace ----------------

func DiskSpace(path string) (total, free, used uint64, err error) {
	lp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, 0, err
	}
	var freeBytesAvailable, totalBytes, totalFree uint64
	err = windows.GetDiskFreeSpaceEx(lp, &freeBytesAvailable, &totalBytes, &totalFree)
	if err != nil {
		return 0, 0, 0, err
	}
	return totalBytes, totalFree, totalBytes - totalFree, nil
}

// ---------------- Disks ----------------

func Disks() ([]string, error) {
	bitmask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	var drives []string
	for i := 0; i < 26; i++ {
		if bitmask&(1<<uint(i)) != 0 {
			drives = append(drives, string('A'+uint8(i))+":\\")
		}
	}
	return drives, nil
}

// ---------------- Mount / Unmount ----------------
func Mount(driveLetter, networkPath, user, password string) (string, error) {

	// Pfad normalisieren: Linux-Slashes → Windows-Backslashes
	networkPath = strings.ReplaceAll(networkPath, "/", "\\")

	// Laufwerksbuchstabe: sicherstellen dass er mit : endet (z.B. "Z" → "Z:")
	if driveLetter != "" && !strings.HasSuffix(driveLetter, ":") {
		driveLetter += ":"
	}

	if driveLetter == "" {
		return "", fmt.Errorf("kein Laufwerksbuchstabe angegeben — bitte computer.GetNextFreeDriveLetter() verwenden")
	}

	// net use Z: \\server\share [password] [/user:domain\user] /persistent:no
	args := []string{"use", driveLetter, networkPath}

	if user != "" {
		if password != "" {
			args = append(args, password)
		}
		args = append(args, "/user:"+user)
	}

	args = append(args, "/persistent:no")

	cmd := exec.Command("net", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("net use fehlgeschlagen: %v - %s", err, string(out))
	}

	return driveLetter, nil
}

func Unmount(driveLetter string) error {
	cmd := fmt.Sprintf("net use %s /delete", driveLetter)
	return RunCmd(cmd)
}

// GetNextFreeDriveLetter sucht von Z abwärts nach einem freien Buchstaben.
// Rückgabe: (Buchstabe, Fehler-Nachricht)
func getLogicalDrives() (map[string]bool, error) {
	used := make(map[string]bool)

	ret, _, err := procGetLogicalDrives.Call()
	if ret == 0 {
		return nil, err
	}

	mask := uint32(ret)

	for i := 0; i < 26; i++ {
		if mask&(1<<i) != 0 {
			drive := string(rune('A'+i)) + ":"
			used[drive] = true
		}
	}

	return used, nil
}

func getNetworkDrives() map[string]bool {
	used := make(map[string]bool)

	cmd := exec.Command("cmd", "/C", "net use")
	out, err := cmd.Output()
	if err != nil {
		return used
	}

	// Achtung: Parst die Ausgabe von 'net use' die je nach Windows-Version
	// und Systemsprache unterschiedlich formatiert sein kann.
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) >= 2 && line[1] == ':' {
			drive := line[:2]
			used[drive] = true
		}
	}

	return used
}

func GetNextFreeDriveLetter() (string, string) {
	logical, err := getLogicalDrives()
	if err != nil {
		return "", "Fehler beim Lesen der Laufwerke"
	}

	network := getNetworkDrives()

	for char := 'Z'; char >= 'D'; char-- {
		drive := string(char) + ":"

		if !logical[drive] && !network[drive] {
			return drive, ""
		}
	}

	return "", "Keine freien Laufwerksbuchstaben verfügbar"
}

// ---------------- Reboot / Shutdown ----------------

func Reboot() error {
	// /r = reboot, /t 0 = sofort
	return exec.Command("shutdown", "/r", "/t", "0").Run()
}

func Shutdown() error {
	// /s = shutdown, /t 0 = sofort
	return exec.Command("shutdown", "/s", "/t", "0").Run()
}

// ---------------- RunCmd ----------------

func RunCmd(cmd string) error {
	c := exec.Command("cmd", "/C", cmd)
	// Verhindert das Aufpoppen eines CMD-Fensters
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return c.Run()
}
