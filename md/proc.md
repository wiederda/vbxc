# ⚙️ proc.* – Prozessverwaltung

Dient zur Überwachung, Steuerung und Ausführung von Prozessen.
Systemprozesse (PID ≤ 10, eigener Prozess, Elternprozess, kritische Systemdienste) sind vor Kill-Operationen geschützt.

---

## proc.Tasklist()
- **Konkret:**
  Gibt eine Liste aller laufenden Prozesse als formatierten Text zurück.
- **Rückgabe:**
  `StrVal`
  Eine Zeile pro Prozess: `PID=x PPID=x NAME=x PATH=x ARGS=x`.

---

## proc.IsScriptRunning()
- **Konkret:**
  Prüft automatisch, ob das aktuell ausgeführte Skript bereits in einer anderen Instanz läuft.
  Der Skriptpfad wird selbstständig aus den Aufruf-Argumenten ermittelt (erstes Argument, das auf `.vb` oder `.vbc` endet – funktioniert unabhängig von vorangestellten Subcommands wie `worker` oder Flags wie `-modules=...`) und in einen absoluten Pfad umgewandelt, bevor er gegen die Kommandozeile aller anderen laufenden Prozesse verglichen wird. Der eigene Prozess wird dabei ausgenommen.
  Kein Parameter nötig – funktioniert unabhängig davon, aus welchem Verzeichnis oder mit welchen zusätzlichen Argumenten das Skript gestartet wird.
- **Rückgabe:**
  `BoolVal`
  `true`, wenn eine andere Instanz desselben Skripts (gleicher absoluter Pfad) bereits läuft, sonst `false`. Auch `false`, wenn kein `.vb`/`.vbc`-Argument im Aufruf gefunden wird.

---

## proc.GetPids()
- **Konkret:**
  Gibt ein Array mit allen aktuell laufenden PIDs zurück.
- **Rückgabe:**
  `ArrVal`
  Array von `NumVal`-Einträgen.

---

## proc.GetPidsEx()
- **Konkret:**
  Gibt ein Array mit Details zu allen Prozessen zurück.
- **Rückgabe:**
  `ArrVal`
  Array von `[PID, Path]`-Arrays.

---

## proc.GetPidsByName(name)
- **Konkret:**
  Gibt alle PIDs zurück deren Prozessname exakt übereinstimmt (case-insensitiv).
- **Parameter:**
  - `name`: Prozessname.
- **Rückgabe:**
  `ArrVal`
  Array von `NumVal`-Einträgen.

---

## proc.GetPidsByNameEx(name)
- **Konkret:**
  Gibt alle Prozesse zurück deren Name übereinstimmt (case-insensitiv).
  Liefert mehr Details als `GetPidsByName`.
- **Parameter:**
  - `name`: Prozessname.
- **Rückgabe:**
  `ArrVal`
  Array von `[PID, Path]`-Arrays.

---

## proc.GetPidsByPath(path)
- **Konkret:**
  Gibt alle PIDs zurück deren ausführbarer Pfad exakt übereinstimmt.
- **Parameter:**
  - `path`: Vollständiger Pfad zur Executable.
- **Rückgabe:**
  `ArrVal`
  Array von `NumVal`-Einträgen.

---

## proc.GetChildPids(pid)
- **Konkret:**
  Gibt die PIDs aller direkten Kindprozesse zurück.
- **Parameter:**
  - `pid`: Eltern-PID.
- **Rückgabe:**
  `ArrVal`
  Array von `NumVal`-Einträgen. Leer wenn keine Kinder oder Zugriff verweigert.

---

## proc.PidExists(pid)
- **Konkret:**
  Prüft ob eine PID aktuell aktiv ist.
- **Parameter:**
  - `pid`: Zu prüfende PID.
- **Rückgabe:**
  `BoolVal`

---

## proc.ExistsByName(name)
- **Konkret:**
  Prüft ob ein Prozess mit diesem Namen läuft (case-insensitiv, exakter Match).
- **Parameter:**
  - `name`: Prozessname.
- **Rückgabe:**
  `BoolVal`

---

## proc.ExistsByPath(path)
- **Konkret:**
  Prüft ob ein Prozess unter diesem Pfad läuft.
  Windows: case-insensitiv. Linux/macOS: case-sensitiv.
- **Parameter:**
  - `path`: Pfad zur Executable.
- **Rückgabe:**
  `BoolVal`

---

## proc.IsSystem(pid)
- **Konkret:**
  Prüft ob eine PID als geschützter Systemprozess eingestuft ist.
- **Parameter:**
  - `pid`: Zu prüfende PID.
- **Rückgabe:**
  `BoolVal`

---

## proc.Info(pid)
- **Konkret:**
  Gibt Details zu einem Prozess zurück.
- **Parameter:**
  - `pid`: Ziel-PID.
- **Rückgabe:**
  `ArrVal`
  Format: `[Name, Status, Startzeit, Benutzer, PID]`

---

## proc.CurrentPid()
- **Konkret:**
  Gibt die PID des aktuellen VBMini-Interpreters zurück.
- **Rückgabe:**
  `NumVal`

---

## proc.ParentPid([pid])
- **Konkret:**
  Gibt die Parent-PID (PPID) zurück.
  Ohne Parameter: PPID des eigenen Prozesses.
- **Parameter:**
  - `pid`: Optional. Ziel-PID.
- **Rückgabe:**
  `NumVal`

---

## proc.Memory(pid [, type])
- **Konkret:**
  Gibt den RAM-Verbrauch eines Prozesses in Bytes zurück.
- **Parameter:**
  - `pid`: Ziel-PID.
  - `type`: Optional. `0` = RSS / physikalisch (Standard), `1` = VMS / virtuell.
- **Rückgabe:**
  `NumVal` (Bytes)

---

## proc.CPU(pid)
- **Konkret:**
  Gibt die CPU-Auslastung eines Prozesses in Prozent zurück.
  Misst über ein 100ms-Intervall.
- **Parameter:**
  - `pid`: Ziel-PID.
- **Rückgabe:**
  `NumVal` (Prozent)

---

## proc.GetPriority(pid)
- **Konkret:**
  Gibt die Nice-Priorität eines Prozesses zurück.
  Unix: `-20` (höchste) bis `19` (niedrigste). Windows: Prioritätsklasse.
- **Parameter:**
  - `pid`: Ziel-PID.
- **Rückgabe:**
  `NumVal`

---

## proc.Uptime(pid)
- **Konkret:**
  Gibt die Laufzeit eines Prozesses in Sekunden zurück.
- **Parameter:**
  - `pid`: Ziel-PID.
- **Rückgabe:**
  `NumVal`

---

## proc.UptimeString(pid)
- **Konkret:**
  Gibt die Laufzeit eines Prozesses als lesbaren Text zurück.
- **Parameter:**
  - `pid`: Ziel-PID.
- **Rückgabe:**
  `StrVal`
  Format: `HH:MM:SS` oder `Xd HH:MM:SS` bei mehr als einem Tag.

---

## proc.Kill(pid)
- **Konkret:**
  Beendet einen Prozess per PID.
  Systemprozesse und der eigene Prozess sind geschützt.
- **Parameter:**
  - `pid`: Ziel-PID.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler oder geschütztem Prozess.

---

## proc.KillTree(pid)
- **Konkret:**
  Beendet einen Prozess und alle seine Kindprozesse rekursiv (Bottom-Up).
  Jeder Knoten im Baum wird auf Schutz geprüft.
- **Parameter:**
  - `pid`: Wurzel-PID des zu beendenden Baums.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler oder geschütztem Prozess.

---

## proc.KillByName(name [, mode])
- **Konkret:**
  Beendet alle Prozesse mit diesem Namen.
  Systemprozesse werden übersprungen.
- **Parameter:**
  - `name`: Prozessname (case-insensitiv).
  - `mode`: Optional. `1` = Teilsuche (enthält den Namen).
- **Rückgabe:**
  `NumVal`
  Anzahl erfolgreich beendeter Prozesse.

---

## proc.KillByPath(path [, mode])
- **Konkret:**
  Beendet alle Prozesse die unter diesem Pfad laufen.
  Systemprozesse werden übersprungen.
  Windows: case-insensitiv.
- **Parameter:**
  - `path`: Pfad zur Executable.
  - `mode`: Optional. `1` = Teilsuche (Pfad enthält den String).
- **Rückgabe:**
  `NumVal`
  Anzahl erfolgreich beendeter Prozesse.

---

## proc.KillTreeByPath(path [, mode])
- **Konkret:**
  Beendet Prozessbäume aller Prozesse die unter diesem Pfad laufen.
- **Parameter:**
  - `path`: Pfad zur Executable.
  - `mode`: Optional. `1` = Teilsuche.
- **Rückgabe:**
  `NumVal`
  Anzahl erfolgreich beendeter Prozessbäume.

---

## proc.CountByPath(path [, mode])
- **Konkret:**
  Zählt laufende Prozesse die diesem Pfad entsprechen.
- **Parameter:**
  - `path`: Pfad zur Executable.
  - `mode`: Optional. `1` = Teilsuche.
- **Rückgabe:**
  `NumVal`

---

## proc.Start(path [, args...])
- **Konkret:**
  Startet ein Programm im Hintergrund und kehrt sofort zurück.
- **Parameter:**
  - `path`: Pfad zur Executable oder Systembefehl.
  - `args...`: Optional. Weitere Argumente.
- **Rückgabe:**
  `NumVal` (PID des gestarteten Prozesses), `"error: ..."` bei Fehler.

---

## proc.Exec(cmd [, args...])
- **Konkret:**
  Führt einen Befehl aus und wartet auf dessen Beendigung.
  Stdout und Stderr werden direkt auf die Konsole ausgegeben (Echtzeit-Output).
- **Parameter:**
  - `cmd`: Befehlsname oder Pfad.
  - `args...`: Optional. Weitere Argumente.
- **Rückgabe:**
  `BoolVal` (`true` bei Exit-Code 0).

---

## proc.ExecEx(cmd, timeout, args...)
- **Konkret:**
  Führt einen Befehl mit Timeout aus und gibt Output und Exit-Code zurück.
  Bei Timeout-Überschreitung: Exit-Code `-999`.
  Bei Systemfehler (Befehl nicht gefunden): Exit-Code `-1`.
- **Parameter:**
  - `cmd`: Befehlsname oder Pfad.
  - `timeout`: Timeout in Millisekunden.
  - `args...`: Optional. Weitere Argumente.
- **Rückgabe:**
  `ArrVal`
  Format: `[Kombiniert, Stdout, Stderr, ExitCode]`

---

## proc.ExecInteractive(cmd, timeout, responses, args...)
- **Konkret:**
  Führt einen Befehl aus und reagiert automatisch auf Prompts.
  Nützlich für interaktive Installationsprogramme.
  Responses ist ein flaches Array: `[Muster1, Antwort1, Muster2, Antwort2, ...]`.
  Muster werden case-insensitiv gegen stdout und stderr geprüft.
- **Parameter:**
  - `cmd`: Befehlsname oder Pfad.
  - `timeout`: Timeout in Millisekunden.
  - `responses`: `ArrVal` mit Muster-Antwort-Paaren.
  - `args...`: Optional. Weitere Befehlsargumente.
- **Rückgabe:**
  `ArrVal`
  Format: `[Kombiniert, Stdout, Stderr, ExitCode]`