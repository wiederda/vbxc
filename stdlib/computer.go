package stdlib

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"runtime"

	"github.com/shirou/gopsutil/v3/cpu"
	valpkg "vbxc/value"
)

// ---------------- User & System Info ----------------

func UserName() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.Username
}

func ComputerName() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}

func UserDomain() string {
	return "" // nur Windows implementiert Domain
}

func OS() string {
	return runtime.GOOS
}

func Arch() string {
	return runtime.GOARCH
}

func CPUCount() int {
	return runtime.NumCPU()
}

// ---------------- Network Info ----------------

func MACAddresses() []string {
	var macs []string
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || len(iface.HardwareAddr) == 0 {
			continue
		}
		mac := iface.HardwareAddr.String()
		if mac == "00:00:00:00:00:00" {
			continue
		}
		macs = append(macs, mac)
	}
	return macs
}

// ---------------- Admin Check ----------------

func IsAdmin() bool {
	return isAdmin()
}

func initComputerFunctions() {
	ns := "computer."
	Register(ns+"MACAddresses", "computer", "", "Listet die Hardware-Adressen (MAC) aktiver Netzwerkschnittstellen auf.", func(args []valpkg.Value) valpkg.Value {
		macs := MACAddresses()
		out := make([]valpkg.Value, len(macs))
		for i, mac := range macs {
			out[i] = valpkg.StrVal(mac)
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: out}
	})

	// Identität & OS

	Register(ns+"IsAdmin", "computer", "", "Prüft auf Administrator-Rechte.", func(args []valpkg.Value) valpkg.Value { return valpkg.BoolVal(IsAdmin()) })

	Register(ns+"Disks", "computer", "", "Gibt ein Array mit allen verfügbaren Laufwerken/Mountpoints zurück.", func(args []valpkg.Value) valpkg.Value {
		disks, err := Disks()
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindStr, Str: "error: " + err.Error()}
		}
		var arr []valpkg.Value
		for _, d := range disks {
			arr = append(arr, valpkg.Value{Kind: valpkg.KindStr, Str: d})
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
	})

	// -------- Logical CPUs (Threads) --------
	Register(ns+"CPUCount", "computer", "", "Gibt die Anzahl der logischen Prozessoren zurück (inkl. Hyper-Threading).", func(args []valpkg.Value) valpkg.Value {
		return valpkg.NumVal(float64(runtime.NumCPU()))
	})

	// -------- Physical Cores (Echte Kerne) --------
	Register(ns+"CPUCores", "computer", "", "Gibt die Anzahl der echten physischen Rechenkerne zurück.", func(args []valpkg.Value) valpkg.Value {
		// gopsutil kann zwischen logisch und physisch unterscheiden
		cores, err := cpu.Counts(false) // false = nur physische Kerne
		if err != nil || cores == 0 {
			// Fallback: Wenn wir es nicht ermitteln können,
			// nehmen wir die logischen / 2 (grobe Schätzung) oder einfach NumCPU
			return valpkg.NumVal(float64(runtime.NumCPU()))
		}
		return valpkg.NumVal(float64(cores))
	})

	// Mount & USB
	Register(ns+"USBReady", "computer", "", "Sucht nach angeschlossenen USB-Sticks.", func(args []valpkg.Value) valpkg.Value {
		found, path, name, fs := FindAvailableUSB()
		return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{valpkg.BoolVal(found), valpkg.StrVal(path), valpkg.StrVal(name), valpkg.StrVal(fs)}}
	})

	// System-Control
	Register(ns+"Exit", "computer", "code, [msg]", "Beendet das Programm sofort.", func(args []valpkg.Value) valpkg.Value {
		code := 0
		if len(args) > 0 {
			code = int(valpkg.ToNum(args[0]))
		}
		if len(args) > 1 {
			fmt.Fprintln(os.Stderr, args[1].Str)
		}
		os.Exit(code)
		return valpkg.Value{}
	})

	// ---------------- Mount / Unmount (Neu) ----------------
	Register(ns+"Mount", "computer", "path, [user], [pass], [target]",
		"Verbindet ein Netzlaufwerk. Gibt [OK, Path, Msg] zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("usage: computer.Mount(path, [user], [pass], [target])")
			}

			path := args[0].Str
			user := ""
			if len(args) >= 2 {
				user = args[1].Str
			}
			pass := ""
			if len(args) >= 3 {
				pass = args[2].Str
			}
			target := ""
			if len(args) >= 4 {
				target = args[3].Str
			}

			res, err := Mount(target, path, user, pass)

			// Wir bauen ein Array (Slice) von Values
			arr := make([]valpkg.Value, 3)
			if err != nil {
				arr[0] = valpkg.BoolVal(false)
				arr[1] = valpkg.StrVal("")
				arr[2] = valpkg.StrVal(err.Error())
			} else {
				arr[0] = valpkg.BoolVal(true)
				arr[1] = valpkg.StrVal(res)
				arr[2] = valpkg.StrVal("")
			}

			return valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
		})

	Register(ns+"Unmount", "computer", "drive",
		"Trennt ein Netzlaufwerk. Gibt [OK, Drive, Msg] zurück.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 1 {
				return valpkg.ErrorVal("usage: computer.Unmount(drive)")
			}

			drive := args[0].Str
			err := Unmount(drive)

			arr := make([]valpkg.Value, 3)

			if err != nil {
				arr[0] = valpkg.BoolVal(false)
				arr[1] = valpkg.StrVal("")
				arr[2] = valpkg.StrVal(err.Error())
			} else {
				arr[0] = valpkg.BoolVal(true)
				arr[1] = valpkg.StrVal(drive)
				arr[2] = valpkg.StrVal("")
			}

			return valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
		})

	Register(ns+"NextFreeLetter", "computer", "",
		"Sucht den nächsten freien Laufwerksbuchstaben (Windows). Gibt [OK, Letter, Msg] zurück.",
		func(args []valpkg.Value) valpkg.Value {
			// Wir holen uns beide Infos aus der plattformspezifischen Funktion
			letter, errMsg := GetNextFreeDriveLetter()

			res := make([]valpkg.Value, 3)
			if letter == "" {
				// Fehlerfall (entweder Linux oder Windows voll)
				res[0] = valpkg.BoolVal(false)
				res[1] = valpkg.StrVal("")
				res[2] = valpkg.StrVal(errMsg)
			} else {
				// Erfolgsfall
				res[0] = valpkg.BoolVal(true)
				res[1] = valpkg.StrVal(letter)
				res[2] = valpkg.StrVal("")
			}

			return valpkg.Value{Kind: valpkg.KindArr, Arr: res}
		})

	// ---------------- System-Befehle (Neu) ----------------

	// REBOOT
	Register(ns+"Reboot", "computer", "", "Startet das System sofort neu. Gibt [OK, Msg] zurück.",
		func(args []valpkg.Value) valpkg.Value {
			err := Reboot()

			res := make([]valpkg.Value, 2)
			if err != nil {
				res[0] = valpkg.BoolVal(false)
				res[1] = valpkg.StrVal(err.Error())
			} else {
				// Falls der Befehl akzeptiert wurde, geben wir True zurück.
				// Das System fährt dann meistens innerhalb von Sekunden runter.
				res[0] = valpkg.BoolVal(true)
				res[1] = valpkg.StrVal("")
			}
			return valpkg.Value{Kind: valpkg.KindArr, Arr: res}
		})

	Register(ns+"DiskSpace", "computer", "path", "Gibt Speicherinformationen eines Laufwerks zurück (total, free, used)", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("usage: system.DiskSpace(path)")
		}

		path := valpkg.ToString(args[0])

		total, free, used, err := DiskSpace(path)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		// Rückgabe als Array: [total, free, used]
		return valpkg.Value{
			Kind: valpkg.KindArr,
			Arr: []valpkg.Value{
				valpkg.NumVal(float64(total)),
				valpkg.NumVal(float64(free)),
				valpkg.NumVal(float64(used)),
			},
		}
	})

	// SHUTDOWN
	Register(ns+"Shutdown", "computer", "", "Fährt das System sofort herunter. Gibt [OK, Msg] zurück.",
		func(args []valpkg.Value) valpkg.Value {
			err := Shutdown()

			res := make([]valpkg.Value, 2)
			if err != nil {
				res[0] = valpkg.BoolVal(false)
				res[1] = valpkg.StrVal(err.Error())
			} else {
				res[0] = valpkg.BoolVal(true)
				res[1] = valpkg.StrVal("")
			}
			return valpkg.Value{Kind: valpkg.KindArr, Arr: res}
		})

	Register(ns+"Distro", "computer", "", "Gibt die Distro oder das OS zurück.", func(args []valpkg.Value) valpkg.Value {
		// Diese Funktion kommt aus den OS-spezifischen Dateien
		return valpkg.StrVal(getOSID())
	})

	Register(ns+"NeedsReboot", "computer", "", "Prüft, ob ein Neustart erforderlich ist.", func(args []valpkg.Value) valpkg.Value {
		return valpkg.BoolVal(checkNeedsReboot())
	})
}

// Helper für die Formatierung
func computerFormatUptime(seconds float64) string {
	s := int(seconds)
	days := s / 86400
	s %= 86400
	hours := s / 3600
	s %= 3600
	minutes := s / 60
	secs := s % 60

	if days > 0 {
		return fmt.Sprintf("%d Tage, %02d:%02d:%02d", days, hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}

func init() { initComputerFunctions() }
