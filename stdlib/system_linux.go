//go:build !windows

package stdlib

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func systemUptimeSeconds() (float64, error) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("System-Uptime nicht verfügbar: %w", err)
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0, fmt.Errorf("System-Uptime konnte nicht gelesen werden")
	}
	return strconv.ParseFloat(f[0], 64)
}
func systemMemory() (uint64, uint64, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, fmt.Errorf("Systemspeicher nicht verfügbar: %w", err)
	}
	var total, available uint64
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		n, e := strconv.ParseUint(f[1], 10, 64)
		if e != nil {
			continue
		}
		switch strings.TrimSuffix(f[0], ":") {
		case "MemTotal":
			total = n * 1024
		case "MemAvailable":
			available = n * 1024
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("Systemspeicher konnte nicht gelesen werden")
	}
	return total, available, nil
}
func formatUptime(seconds float64) string {
	s := int(seconds)
	days := s / 86400
	s %= 86400
	hours := s / 3600
	s %= 3600
	minutes := s / 60
	s %= 60
	if days > 0 {
		return fmt.Sprintf("%d Tage, %02d:%02d:%02d", days, hours, minutes, s)
	}
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, s)
}
func processIsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
