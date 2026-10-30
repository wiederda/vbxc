# 🗂️ app.* – Anwendungs- & Pfadfunktionen

Dient zur Abfrage von Pfaden der laufenden Anwendung und systemspezifischen Standardverzeichnissen.
Plattformübergreifend (Windows, Linux, macOS).

---

## app.StartupPath()
- **Konkret:**
  Gibt das Verzeichnis zurück, in dem die ausführbare Datei liegt.
- **Rückgabe:**
  `StrVal`

---

## app.ExecutablePath()
- **Konkret:**
  Gibt den vollständigen Pfad der ausführbaren Datei zurück (inkl. Dateiname).
- **Rückgabe:**
  `StrVal`

---

## app.ScriptPath()
- **Konkret:**
  Gibt den vollständigen Pfad (inkl. Dateiname) der aktuell ausgeführten Skript-Datei zurück.
  Anders als `app.CurrentDirectory()` bezieht sich dieser Pfad auf den Speicherort
  des Skripts selbst, unabhängig davon, aus welchem Verzeichnis `vbx` gestartet wurde.
- **Rückgabe:**
  `StrVal`

---

## app.ScriptDir()
- **Konkret:**
  Gibt das Verzeichnis zurück, in dem die aktuell ausgeführte Skript-Datei liegt.
  Entspricht dem Ordneranteil von `app.ScriptPath()`.
  Nützlich, um Pfade relativ zum Skript aufzubauen, z. B. für Build- oder Release-Skripte,
  die unabhängig vom aktuellen Arbeitsverzeichnis funktionieren sollen.
- **Rückgabe:**
  `StrVal`

---

## app.SetCurrentDirectory(path)
- **Konkret:**
  Setzt das aktuelle Arbeitsverzeichnis des Prozesses auf `path`.
  `path` muss ein nicht-leerer String sein und auf ein existierendes Verzeichnis zeigen. Relative Pfade werden relativ zum bisherigen Arbeitsverzeichnis aufgelöst.
  Die Änderung gilt für den gesamten Prozess und wirkt sich auf alle folgenden relativen Dateipfade aus.
  Das aktuelle Verzeichnis lässt sich mit `app.CurrentDirectory()` abfragen.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), bei einem Fehler (Pfad existiert nicht, kein Verzeichnis, keine Berechtigung) ein Fehlerwert

---

## app.CurrentDirectory()
- **Konkret:**
  Gibt das aktuelle Arbeitsverzeichnis zurück (Working Directory).
  Das ist das Verzeichnis von dem aus VBMini gestartet wurde, nicht das Verzeichnis der Executable.
- **Rückgabe:**
  `StrVal`

---

## app.TempPath()
- **Konkret:**
  Gibt den Pfad zum temporären Verzeichnis des Betriebssystems zurück.
- **Rückgabe:**
  `StrVal`
  Beispiele: `"/tmp"` (Linux/macOS), `"C:\Users\Name\AppData\Local\Temp"` (Windows).

---

## app.SpecialFolder(name)
- **Konkret:**
  Gibt einen systemspezifischen Standardpfad zurück.
  Auf Windows werden Umgebungsvariablen ausgewertet.
  Auf Linux/macOS werden Pfade relativ zum Home-Verzeichnis aufgebaut.
- **Parameter:**
  - `name`: Name des Ordners (case-insensitiv).
- **Unterstützte Werte:**

  | Name             | Windows                        | Linux / macOS              |
  |------------------|--------------------------------|----------------------------|
  | `"home"`         | –                              | `~`                        |
  | `"desktop"`      | –                              | `~/Desktop`                |
  | `"documents"`    | –                              | `~/Documents`              |
  | `"downloads"`    | –                              | `~/Downloads`              |
  | `"pictures"`     | –                              | `~/Pictures`               |
  | `"music"`        | –                              | `~/Music`                  |
  | `"videos"`       | –                              | `~/Videos`                 |
  | `"temp"`         | `%TEMP%`                       | `/tmp`                     |
  | `"appdata"`      | `%APPDATA%`                    | –                          |
  | `"localappdata"` | `%LOCALAPPDATA%`               | –                          |
  | `"programdata"`  | `%PROGRAMDATA%`                | –                          |

- **Rückgabe:**
  `StrVal`
  Leerer String wenn der Name nicht unterstützt wird oder der Pfad nicht ermittelt werden kann.