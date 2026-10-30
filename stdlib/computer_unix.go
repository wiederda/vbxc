//go:build linux || darwin
// +build linux darwin

package stdlib

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type DiskInfo struct {
	Total uint64
	Free  uint64
	Used  uint64
}

func isAdmin() bool {
	return os.Geteuid() == 0
}

// DiskSpace für Unix/Linux/macOS
func DiskSpace(path string) (total, free, used uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(path, &stat)
	if err != nil {
		return 0, 0, 0, err
	}
	total = stat.Blocks * uint64(stat.Bsize)
	free = stat.Bavail * uint64(stat.Bsize)
	used = total - free
	return total, free, used, nil
}

func getUptimeSeconds() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	// /proc/uptime enthält zwei Zahlen: [uptime] [idle time]
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		uptime, _ := strconv.ParseFloat(fields[0], 64)
		return uptime
	}
	return 0
}

func getOSID() string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "linux"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}

	if err := scanner.Err(); err != nil {
		return "linux"
	}

	return "linux"
}

func checkNeedsReboot() bool {
	// Pfad A: Debian/Ubuntu (Datei-basiert)
	if _, err := os.Stat("/var/run/reboot-required"); err == nil {
		return true
	}

	// Pfad B: Manueller Kernel-Vergleich (Der Sicherheitsgurt für Rocky/RHEL)
	// Funktioniert auch, wenn dnf-utils NICHT installiert sind.

	// 1. Laufender Kernel (wie 'uname -r')
	outRunning, err := exec.Command("uname", "-r").Output()
	if err == nil {
		running := strings.TrimSpace(string(outRunning))

		// 2. Höchste installierte Kernel-Version via RPM suchen
		cmd := exec.Command("rpm", "-q", "kernel", "--queryformat", "%{VERSION}-%{RELEASE}.%{ARCH}\n")
		out, err := cmd.Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) > 0 {
				installed := strings.TrimSpace(lines[len(lines)-1])
				// Wenn die installierte Version nicht im String des laufenden Kernels vorkommt -> Reboot!
				if installed != "" && !strings.Contains(running, installed) {
					return true
				}
			}
		}
	}

	// Pfad C: Rocky/RHEL (Tool-basiert via dnf-utils)
	// Prüft zusätzlich, ob Bibliotheken (SSL, Glibc) einen Neustart brauchen.
	path, err := exec.LookPath("needs-restarting")
	if err == nil {
		cmd := exec.Command(path, "-r")
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				// Exit 1 bei 'needs-restarting -r' bedeutet: Reboot nötig
				if exitError.ExitCode() == 1 {
					return true
				}
			}
		}
	}

	return false
}

// Disks – listet root und mountpoints
func Disks() ([]string, error) {
	var disks []string
	disks = append(disks, "/")
	for _, base := range []string{"/mnt", "/media", "/Volumes"} {
		files, _ := os.ReadDir(base)
		for _, f := range files {
			if f.IsDir() {
				disks = append(disks, base+"/"+f.Name())
			}
		}
	}
	return disks, nil
}

func FindAvailableUSB() (bool, string, string, string) {
	scanPaths := []string{"/media", "/mnt", "/Volumes"}
	for _, base := range scanPaths {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				fullPath := filepath.Join(base, entry.Name())
				ready, name, fs := GetDriveDetails(fullPath) // fs wird hier empfangen
				if ready {
					return true, fullPath, name, fs // fs wird jetzt zurückgegeben!
				}
			}
		}
	}
	return false, "", "", ""
}

func GetDriveDetails(path string) (ready bool, name string, fs string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false, "", ""
	}

	// Mount-Status prüfen (Linux/macOS sinnvoll)
	mounted := false
	out, err := exec.Command("mount").Output()
	if err == nil {
		mounted = strings.Contains(string(out), path)
	}

	// Filesystem ermitteln
	fsType := "unknown"

	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err == nil {

		// sauberer: auf unsigned casten
		switch uint64(stat.Type) {

		// Linux
		case 0xEF53:
			fsType = "ext4"

		case 0x6969:
			fsType = "nfs"

		// FAT32
		case 0x4d44:
			fsType = "fat32"

		// NTFS (oft via FUSE)
		case 0x5346544e:
			fsType = "ntfs"
		}
	}

	return mounted, path, fsType
}

func Mount(driveLetter, networkPath, user, password string) (string, error) {

	var targetName string

	if driveLetter != "" {
		targetName = driveLetter
	} else {
		cleanPath := strings.Trim(networkPath, "\\/")
		parts := strings.Split(cleanPath, "/")
		if len(parts) == 0 || parts[len(parts)-1] == "" {
			targetName = "network_share"
		} else {
			targetName = parts[len(parts)-1]
		}
	}

	mountPoint := "/mnt/" + targetName

	// Existiert bereits?
	if _, err := os.Stat(mountPoint); err == nil {
		return "", fmt.Errorf("mountpoint existiert bereits: %s", mountPoint)
	}

	// Verzeichnis erstellen
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return "", fmt.Errorf("konnte mountpoint nicht erstellen: %v", err)
	}

	var cmd *exec.Cmd

	// Protokoll automatisch erkennen:
	// NFS:  "192.168.1.100:/share"  → enthält ":"
	// CIFS: "//server/share"        → fängt mit // oder \\ an
	if strings.Contains(networkPath, ":") && !strings.HasPrefix(networkPath, "//") && !strings.HasPrefix(networkPath, "\\\\") {
		// -------------------------------------------------------
		// NFS
		// -------------------------------------------------------
		// NFS braucht kein User/Pass — Zugriff über IP-Freigabe am NAS
		// Optionen: vers=4 (NFSv4), hard (wartet bei Verbindungsabbruch),
		//           intr (unterbrechbar), rsize/wsize 1MB für Netzwerk-Performance
		args := []string{
			"-t", "nfs",
			networkPath,
			mountPoint,
			"-o", "vers=4,hard,intr,rsize=1048576,wsize=1048576",
		}
		cmd = exec.Command("mount", args...)

	} else {
		// -------------------------------------------------------
		// CIFS / SMB
		// -------------------------------------------------------
		linuxPath := strings.ReplaceAll(networkPath, "\\", "/")
		if !strings.HasPrefix(linuxPath, "//") {
			linuxPath = "//" + strings.TrimPrefix(linuxPath, "/")
		}

		args := []string{"-t", "cifs", linuxPath, mountPoint}

		var options []string
		if user != "" {
			options = append(options, "username="+user)
		}
		if password != "" {
			options = append(options, "password="+password)
		}
		// iocharset für Umlaute, vers=3.0 für beste Kompatibilität
		options = append(options, "iocharset=utf8", "vers=3.0")

		args = append(args, "-o", strings.Join(options, ","))
		cmd = exec.Command("mount", args...)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(mountPoint)
		return "", fmt.Errorf("mount fehlgeschlagen: %v - %s", err, string(out))
	}

	return mountPoint, nil
}

func Unmount(driveLetter string) error {

	cmd := exec.Command("umount", driveLetter)
	out, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("unmount fehlgeschlagen: %v - %s", err, string(out))
	}

	return nil
}

// GetNextFreeDriveLetter ist unter Linux ein "Soft-Fail"
func GetNextFreeDriveLetter() (string, string) {
	return "", "Laufwerksbuchstaben werden unter Linux/macOS nicht unterstützt"
}

func Reboot() error {
	cmd := exec.Command("/sbin/reboot")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reboot fehlgeschlagen (läuft der Prozess als root?): %w", err)
	}
	return nil
}

func Shutdown() error {
	// Fährt das System sofort herunter
	return exec.Command("shutdown", "-h", "now").Run()
}

// RunCmd für Unix
func RunCmd(cmd string) error {
	c := exec.Command("sh", "-c", cmd)
	return c.Run()
}
