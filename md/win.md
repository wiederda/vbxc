# 🪟 win.* – Windows-Systemfunktionen

Dient zur Abfrage von Fensterstatus und Windows-Ereignisprotokollen.
**Plattform: Ausschließlich Windows.**
EventLog-Funktionen benötigen Administratorrechte.

---

## win.GetActiveTitle()
- **Konkret:**
  Gibt den Titel des aktuell im Vordergrund befindlichen Fensters zurück.
  Nutzt `GetForegroundWindow` und `GetWindowTextW` aus `user32.dll`.
- **Rückgabe:**
  `StrVal` bei Erfolg, `ErrorVal` wenn kein Fenster aktiv oder Titel nicht lesbar.

---

## win.GetEvent(logName, level [, count])
- **Konkret:**
  Liest Ereignisse eines bestimmten Schweregrads aus einem Windows-EventLog.
  Gibt die Ergebnisse farbig formatiert auf stdout aus.
- **Parameter:**
  - `logName`: Log-Name. Unterstützte Werte (deutsch und englisch):
    `"System"`, `"Application"` / `"Anwendung"`, `"Security"` / `"Sicherheit"`, `"Setup"` / `"Installation"`.
  - `level`: Schweregrad. Unterstützte Werte:
    `"Error"` / `"Fehler"`, `"Warn"` / `"Warnung"`, `"Info"` / `"Information"`.
  - `count`: Optional. Anzahl der zurückzugebenden Einträge (Standard: 5).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei fehlendem Log oder fehlenden Rechten.

---

## win.SearchEventLog(log, text [, count])
- **Konkret:**
  Durchsucht ein EventLog nach einem bestimmten Text in den Eventdaten oder Provider-Namen.
  Gibt die Ergebnisse farbig formatiert auf stdout aus.
- **Parameter:**
  - `log`: Log-Name (gleiche Werte wie bei `win.GetEvent`).
  - `text`: Suchtext.
  - `count`: Optional. Anzahl der zurückzugebenden Einträge (Standard: 5).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei fehlendem Log oder fehlenden Rechten.

---

## win.FindEventID(log, id [, count])
- **Konkret:**
  Sucht im angegebenen EventLog nach Einträgen mit einer spezifischen Event-ID.
  Gibt die Ergebnisse farbig formatiert auf stdout aus.
- **Parameter:**
  - `log`: Log-Name (gleiche Werte wie bei `win.GetEvent`).
  - `id`: Event-ID als String oder Zahl.
  - `count`: Optional. Anzahl der zurückzugebenden Einträge (Standard: 5).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei fehlendem Log oder fehlenden Rechten.