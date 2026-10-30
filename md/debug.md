# 🐛 debug.* – Debug- & Diagnosefunktionen

Dient zur Laufzeitanalyse, Performance-Messung, Assertionen und Log-Verwaltung.
Timer und CPU-Samples sind pro Skript-Instanz stateful – mehrfache Aufrufe liefern Delta-Werte.

---

## debug.Assert(condition [, msg])
- **Konkret:**
  Prüft eine Bedingung und bricht die Ausführung mit Fehlermeldung ab, wenn sie nicht erfüllt ist.
- **Parameter:**
  - `condition`: Zu prüfender Wert.
  - `msg`: Optional. Fehlermeldung bei Assertion-Fehler (Standard: `"Assertion failed"`).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Assertion-Fehler.

---

## debug.AssertEquals(expected, actual [, info])
- **Konkret:**
  Vergleicht zwei Werte und bricht mit Zeilennummer ab, wenn sie nicht übereinstimmen.
- **Parameter:**
  - `expected`: Erwarteter Wert.
  - `actual`: Tatsächlicher Wert.
  - `info`: Optional. Zusätzliche Fehlerbeschreibung.
- **Rückgabe:**
  `BoolVal` (`true`) bei Gleichheit, `ErrorVal` mit Zeilennummer bei Ungleichheit.

---

## debug.TimerStart()
- **Konkret:**
  Startet einen hochauflösenden Timer für Performance-Messungen.
  Überschreibt einen laufenden Timer.
- **Rückgabe:**
  `BoolVal` (`true`)

---

## debug.TimerMs()
- **Konkret:**
  Gibt die verstrichene Zeit seit `debug.TimerStart` in Millisekunden zurück (mit Nachkommastellen).
  Gibt `0` zurück wenn der Timer noch nicht gestartet wurde.
- **Rückgabe:**
  `NumVal` (Millisekunden als Fließkommazahl)

---

## debug.CPUUsage()
- **Konkret:**
  Gibt die CPU-Auslastung des aktuellen Prozesses normalisiert auf alle Kerne zurück (0–100 %).
  Erster Aufruf gibt `0` zurück und initialisiert das Sampling.
  Folgeaufrufe liefern den Delta-Wert seit dem letzten Aufruf.
- **Rückgabe:**
  `NumVal` (Prozent)

---

## debug.ThreadUsage()
- **Konkret:**
  Gibt die CPU-Auslastung des aktuellen Prozesses bezogen auf einen einzelnen Kern zurück (0–100 %).
  Erster Aufruf gibt `0` zurück und initialisiert das Sampling.
  Wert wird auf maximal `100` gedeckelt.
- **Rückgabe:**
  `NumVal` (Prozent)

---

## debug.CurrentCPU()
- **Konkret:**
  Gibt den Index des CPU-Kerns zurück, auf dem das Skript aktuell ausgeführt wird.
- **Hinweis:**
  Unter Go ist dieser Wert eine Annäherung, da der Scheduler Goroutinen schnell verschiebt.
- **Rückgabe:**
  `NumVal`

---

## debug.CPUCores()
- **Konkret:**
  Gibt die Anzahl der verfügbaren CPU-Kerne zurück.
- **Rückgabe:**
  `NumVal`

---

## debug.MemUsage()
- **Konkret:**
  Gibt den aktuell vom Prozess allokierten Arbeitsspeicher in Megabyte zurück.
- **Rückgabe:**
  `NumVal` (MB als Fließkommazahl)

---

## debug.OpenLog()
- **Konkret:**
  Erstellt eine Log-Datei im systemspezifischen Log-Verzeichnis und leitet alle `Print`-Ausgaben dorthin um.
  Dateiname: `<skriptname>_<YYYYMMDD_HHMM>.log`.
- **Log-Verzeichnisse:**
  - Windows: `%AppData%\vbmini\Logs`
  - macOS: `~/Library/Logs/vbmini`
  - Linux: `$XDG_CACHE_HOME/vbmini/Logs` (Fallback: `~/.cache/vbmini/Logs`)
- **Rückgabe:**
  `StrVal` (vollständiger Pfad der erstellten Log-Datei), `ErrorVal` bei Fehler.

---

## debug.AllocStart()
- **Konkret:**
  Startet die Messung der Heap-Allokationen.
  Merkt sich den aktuellen Stand von `TotalAlloc` und `Mallocs` aus den Go-Runtime-Speicherstatistiken.
  Überschreibt eine laufende Messung.
- **Rückgabe:**
  `BoolVal` (`true`)

---  

## debug.AllocBytes()
- **Konkret:**
  Gibt die seit `debug.AllocStart` allokierten Bytes zurück.
  Bezieht sich auf Heap-Allokationen insgesamt, nicht auf aktuell belegten Speicher.
- **Rückgabe:**
  `NumVal` (Bytes als Zahl)

---  

## debug.AllocCount()
- **Konkret:**
  Gibt die Anzahl der seit `debug.AllocStart` durchgeführten Heap-Allokationen zurück.
- **Rückgabe:**
  `NumVal` (Anzahl als Zahl)

---  

## debug.CloseLog()
- **Konkret:**
  Schreibt einen Abschluss-Header in die Log-Datei, flusht und schließt sie.
  Stellt die Ausgabe zurück auf die Konsole um.
  Gibt den Pfad der gespeicherten Datei auf stdout aus.
- **Rückgabe:**
  `BoolVal` (`true`)

---

## debug.CleanOldLogs([days])
- **Konkret:**
  Löscht alle `.log`-Dateien im Log-Verzeichnis, die älter als `days` Tage sind.
- **Parameter:**
  - `days`: Optional. Maximales Alter in Tagen (Standard: 7).
- **Rückgabe:**
  `NumVal`
  Anzahl der gelöschten Dateien.