# 🌍 env.* – Umgebungsvariablen

Dient zum Lesen und Setzen von Umgebungsvariablen des aktuellen Prozesses.

---

## env.Get(key)
- **Konkret:**
  Liest den Wert einer Umgebungsvariable.
  Gibt einen leeren String zurück wenn die Variable nicht gesetzt ist.
- **Parameter:**
  - `key`: Name der Umgebungsvariable (z. B. `"PATH"`, `"TEMP"`, `"HOME"`).
- **Rückgabe:**
  `StrVal`

---

## env.Set(key, value)
- **Konkret:**
  Setzt eine Umgebungsvariable für den aktuellen Prozess.
  Änderungen sind nur innerhalb des laufenden VBMini-Prozesses sichtbar und werden nicht an die übergeordnete Shell weitergegeben.
- **Parameter:**
  - `key`: Name der Umgebungsvariable.
  - `value`: Zu setzender Wert.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler.

---

## env.All()
- **Konkret:**
  Gibt alle aktuell gesetzten Umgebungsvariablen zurück.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen im Format `"KEY=VALUE"`.