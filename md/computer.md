# 💻 computer.* – System-, Hardware- & Betriebssystemfunktionen

Dient zur Abfrage von Benutzer-, Hardware-, Netzwerk- und Betriebssysteminformationen sowie zur Systemsteuerung.
Plattformübergreifend (Windows, Linux, macOS). Einige Funktionen sind plattformspezifisch.

---

## computer.IsAdmin()
- **Konkret:**
  Prüft ob das aktuelle Programm mit Administrator- bzw. Root-Rechten läuft.
- **Rückgabe:**
  `BoolVal`

---

## computer.Disks()
- **Konkret:**
  Gibt alle verfügbaren Laufwerke bzw. Mountpoints zurück.
- **Rückgabe:**
  `ArrVal`
  Array aller erkannten Datenträgerpfade als `StrVal`.

---

## computer.CPUCount()
- **Konkret:**
  Gibt die Anzahl logischer Prozessoren zurück.
  Hyper-Threading / SMT wird berücksichtigt.
- **Rückgabe:**
  `NumVal`

---

## computer.CPUCores()
- **Konkret:**
  Gibt die Anzahl physischer CPU-Kerne zurück.
  Verwendet echte Hardware-Kerne statt logischer Threads.
- **Besonderheit:**
  Falls die Plattform keine physische Kernanzahl liefert, wird auf `CPUCount()` zurückgegriffen.
- **Rückgabe:**
  `NumVal`

---

## computer.USBReady()
- **Konkret:**
  Sucht nach angeschlossenen USB-Speichermedien.
- **Rückgabe:**
  `ArrVal`
  Format: `[Found, Path, Name, FileSystem]`

---

## computer.Mount(path [, user, pass, target])
- **Konkret:**
  Verbindet ein Netzlaufwerk oder Netzwerk-Share.
- **Parameter:**
  - `path`: Netzwerkpfad.
  - `user`: Optional. Benutzername.
  - `pass`: Optional. Passwort.
  - `target`: Optional. Ziel-Mountpoint oder Laufwerksbuchstabe.
- **Rückgabe:**
  `ArrVal`
  Format: `[OK, Path, Msg]`

---

## computer.Unmount(drive)
- **Konkret:**
  Trennt ein Netzlaufwerk oder einen Mountpoint.
- **Parameter:**
  - `drive`: Laufwerksbuchstabe oder Mountpoint.
- **Rückgabe:**
  `ArrVal`
  Format: `[OK, Drive, Msg]`

---

## computer.NextFreeLetter()
- **Konkret:**
  Sucht den nächsten freien Laufwerksbuchstaben.
- **Plattform:**
  Windows.
- **Rückgabe:**
  `ArrVal`
  Format: `[OK, Letter, Msg]`

---

## computer.DiskSpace(path)
- **Konkret:**
  Gibt Speicherinformationen eines Laufwerks oder Mountpoints zurück.
- **Parameter:**
  - `path`: Zielpfad.
- **Rückgabe:**
  `ArrVal`
  Format: `[Total, Free, Used]`
  Alle Werte in Bytes.

---

## computer.Distro()
- **Konkret:**
  Gibt die Betriebssystem-ID bzw. Distribution zurück.
- **Beispiele:**
  `"windows"`, `"ubuntu"`, `"darwin"`.
- **Rückgabe:**
  `StrVal`

---

## computer.NeedsReboot()
- **Konkret:**
  Prüft ob ein Neustart des Systems erforderlich ist.
- **Rückgabe:**
  `BoolVal`

---

## computer.Reboot()
- **Konkret:**
  Startet das Betriebssystem sofort neu.
- **Hinweis:**
  Benötigt normalerweise Administrator- oder Root-Rechte.
- **Rückgabe:**
  `ArrVal`
  Format: `[OK, Msg]`

---

## computer.Shutdown([wert], [einheit], [showOutput])
- **Konkret:**
  Fährt das Betriebssystem herunter.
  Ohne Argumente erfolgt das Herunterfahren sofort.
  Mit Wartezeit wird ein Countdown angezeigt, der per ENTER (oder Strg+C) abgebrochen werden kann. Bei Abbruch wird nicht heruntergefahren.
- **Parameter:**
  - `wert`: Optional. Wartezeit vor dem Herunterfahren (Standard: keine).
  - `einheit`: Optional. `"ms"`, `"s"`, `"m"`, `"h"` (Standard: `"s"`).
  - `showOutput`: Optional. `BoolVal` – steuert die Countdown-Anzeige (Standard: `true`). Bei `false` wird still gewartet und es gibt keine Abbruchmöglichkeit per ENTER.
- **Hinweis:**
  Benötigt normalerweise Administrator- oder Root-Rechte.
- **Rückgabe:**
  `ArrVal`
  Format: `[OK, Msg]`
  Bei Abbruch per ENTER: `[False, "Abgebrochen"]`.

---

## computer.Exit(code [, msg])
- **Konkret:**
  Beendet das aktuelle Programm sofort.
- **Parameter:**
  - `code`: Exit-Code.
  - `msg`: Optional. Fehlermeldung für stderr.
- **Rückgabe:**
  Keine.

---

## MACAddresses()
- **Konkret:**
  Liest alle aktiven Netzwerk-MAC-Adressen aus.
  Inaktive Interfaces und Null-Adressen werden ignoriert.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen im Format `"xx:xx:xx:xx:xx:xx"`.