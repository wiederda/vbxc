# ⚙️ service.* – Windows-Dienstverwaltung

Dient zur Verwaltung von Windows-Diensten: Auflisten, Starten, Stoppen, Installieren und Löschen.
**Plattform: Ausschließlich Windows.**
Alle Funktionen unterstützen optionale Remote-Verwaltung über den `server`-Parameter.

---

## service.List([server, showDisplayNames])
- **Konkret:**
  Gibt ein Array aller auf dem System registrierten Dienste zurück.
  Mit `showDisplayNames` wird jeder Eintrag im Format `"Name:Anzeigename"` zurückgegeben.
  Bei fehlendem Zugriff auf einen Dienst wird `"Name:?"` verwendet.
- **Parameter:**
  - `server`: Optional. Hostname für Remote-Verwaltung. Leerstring = lokal.
  - `showDisplayNames`: Optional. `BoolVal` – Anzeigenamen anhängen.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen.

---

## service.Status(name [, server])
- **Konkret:**
  Ruft den aktuellen Betriebsstatus eines Dienstes ab.
- **Parameter:**
  - `name`: Interner Dienstname.
  - `server`: Optional. Hostname für Remote-Verwaltung.
- **Rückgabe:**
  `StrVal`
  Mögliche Werte: `"Running"`, `"Stopped"`, `"StartPending"`, `"StopPending"`, `"Paused"`, `"PausePending"`, `"ContinuePending"`, `"Unknown"`.
  Bei Fehler: `StrVal` mit Präfix `"error: ..."`.

---

## service.Start(name [, server])
- **Konkret:**
  Startet einen Dienst.
  Gibt den Status unmittelbar nach dem Start-Befehl zurück.
- **Parameter:**
  - `name`: Interner Dienstname.
  - `server`: Optional. Hostname für Remote-Verwaltung.
- **Rückgabe:**
  `StrVal`
  Aktueller Dienststatus oder `"error: ..."` bei Fehler.

---

## service.Stop(name [, server])
- **Konkret:**
  Sendet einen Stop-Befehl an einen Dienst.
  Gibt den Status unmittelbar nach dem Stop-Befehl zurück.
- **Parameter:**
  - `name`: Interner Dienstname.
  - `server`: Optional. Hostname für Remote-Verwaltung.
- **Rückgabe:**
  `StrVal`
  Aktueller Dienststatus oder `"error: ..."` bei Fehler.

---

## service.Restart(name [, server])
- **Konkret:**
  Stoppt einen Dienst und startet ihn anschließend neu.
  Läuft asynchron im Hintergrund. Wartet jeweils max. 10 Sekunden auf Stop bzw. Start.
  Fehler werden intern geloggt, nicht an den Aufrufer zurückgegeben.
- **Hinweis:**
  Die Funktion kehrt sofort zurück. Der Dienst ist beim Rückgabezeitpunkt noch nicht neu gestartet.
- **Parameter:**
  - `name`: Interner Dienstname.
  - `server`: Optional. Hostname für Remote-Verwaltung.
- **Rückgabe:**
  `StrVal` (`"Restarting"`)

---

## service.SetStartType(name, startType [, server])
- **Konkret:**
  Ändert den Starttyp eines Dienstes.
  `"delayed"` setzt den Starttyp auf Auto mit aktiviertem `DelayedAutoStart`-Flag.
- **Parameter:**
  - `name`: Interner Dienstname.
  - `startType`: Starttyp als String. Gültige Werte: `"auto"`, `"manual"`, `"disabled"`, `"delayed"`.
  - `server`: Optional. Hostname für Remote-Verwaltung.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `"error: ..."` bei Fehler.

---

## service.Install(name, displayName, exePath [, startType, server])
- **Konkret:**
  Installiert einen neuen Windows-Dienst.
  Bricht ab, wenn ein Dienst mit diesem Namen bereits existiert.
- **Parameter:**
  - `name`: Interner Dienstname.
  - `displayName`: Anzeigename in der Dienste-Verwaltung.
  - `exePath`: Vollständiger Pfad zur ausführbaren Datei.
  - `startType`: Optional. Starttyp (Standard: `"auto"`). Gültige Werte: `"auto"`, `"manual"`, `"disabled"`.
  - `server`: Optional. Hostname für Remote-Verwaltung.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `"error: ..."` bei Fehler.

---

## service.Delete(name [, server])
- **Konkret:**
  Löscht einen Windows-Dienst.
  Stoppt den Dienst automatisch, falls er noch läuft. Wartet max. 10 Sekunden auf vollständigen Stop.
- **Parameter:**
  - `name`: Interner Dienstname.
  - `server`: Optional. Hostname für Remote-Verwaltung.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `"error: ..."` bei Fehler.