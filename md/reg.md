# 🗝️ reg.* – Windows-Registry-Funktionen

Dient zum Lesen, Schreiben, Prüfen und Löschen von Registry-Einträgen.
**Plattform: Ausschließlich Windows.**

---

## reg.Read(root, path, name)
- **Konkret:**
  Liest einen String-Wert aus der Registry.
- **Parameter:**
  - `root`: Stammschlüssel. `"HKCU"` (Current User) oder `"HKLM"` (Local Machine).
  - `path`: Pfad zum Registry-Key (z. B. `"Software\MyApp"`).
  - `name`: Name des Wertes.
- **Rückgabe:**
  `StrVal` bei Erfolg, `ErrorVal` bei Fehler.

---

## reg.Write(root, path, name, data)
- **Konkret:**
  Schreibt einen String-Wert in die Registry.
  Erstellt den Key automatisch, falls er nicht existiert.
- **Parameter:**
  - `root`: Stammschlüssel. `"HKCU"` oder `"HKLM"`.
  - `path`: Pfad zum Registry-Key.
  - `name`: Name des Wertes.
  - `data`: Zu schreibender String.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `ErrorVal` bei Fehler.

---

## reg.WriteProtectedValue(root, path, name, data)
- **Konkret:**
  Speichert einen String DPAPI-verschlüsselt in der Registry.
  Der Wert wird vor dem Schreiben mit `CryptProtectData` (Windows-DPAPI) verschlüsselt und Base64-kodiert abgelegt. Erstellt den Key automatisch, falls er nicht existiert.
- **Parameter:**
  - `root`: Stammschlüssel. `"HKCU"` oder `"HKLM"`.
  - `path`: Pfad zum Registry-Key.
  - `name`: Name des Wertes.
  - `data`: Zu verschlüsselnder und zu schreibender String.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `ErrorVal` bei Fehler.
- **Hinweis:**
  Der verschlüsselte Wert ist an den Windows-Benutzer **und** den Rechner gebunden (DPAPI-Scope `CurrentUser`). Er kann nur vom selben Benutzerkonto auf derselben Maschine wieder entschlüsselt werden – ein Kopieren des Registry-Werts auf einen anderen Rechner oder unter einem anderen Benutzerkonto führt beim Lesen zu einem Fehler.

---

## reg.ReadProtectedValue(root, path, name)
- **Konkret:**
  Liest einen mit `reg.WriteProtectedValue` gespeicherten String aus der Registry und entschlüsselt ihn per DPAPI für den aktuell angemeldeten Windows-Benutzer.
- **Parameter:**
  - `root`: Stammschlüssel. `"HKCU"` oder `"HKLM"`.
  - `path`: Pfad zum Registry-Key.
  - `name`: Name des Wertes.
- **Rückgabe:**
  `StrVal` (entschlüsselter Klartext) bei Erfolg, `ErrorVal` bei Fehler.
- **Hinweis:**
  Schlägt fehl, wenn der Wert nicht mit `reg.WriteProtectedValue` geschrieben wurde, unter einem anderen Benutzerkonto liegt oder auf einem anderen Rechner erzeugt wurde.

---

## reg.ReadProtectedValueBytes(root, path, name)
- **Konkret:**
  Wie `reg.ReadProtectedValue`, gibt den entschlüsselten Wert aber als Byte-Array (`KindArr` mit Werten 0–255) statt als String zurück. Dadurch kann der Speicher nach Gebrauch gezielt mit `crypt.Wipe` überschrieben werden – bei einem String ist das nicht möglich.
- **Parameter:**
  - `root`: Stammschlüssel. `"HKCU"` oder `"HKLM"`.
  - `path`: Pfad zum Registry-Key.
  - `name`: Name des Wertes.
- **Rückgabe:**
  `Value` vom Typ `KindArr` (Byte-Array) bei Erfolg, `ErrorVal` bei Fehler.
- **Hinweis:**
  Empfohlen für Passwörter, API-Tokens oder andere sensible Werte, bei denen der Speicher nach Gebrauch aktiv gelöscht werden soll. Siehe `crypt.md` für den vollständigen Lifecycle mit `crypt.BytesToString` und `crypt.Wipe`/`crypt.WipeString`.

---

## reg.Exists(root, path [, name])
- **Konkret:**
  Prüft, ob ein Registry-Key oder ein spezifischer Wert existiert.
  Mit 2 Argumenten wird nur das Vorhandensein des Keys geprüft.
  Mit 3 Argumenten wird zusätzlich der Wert geprüft.
- **Parameter:**
  - `root`: Stammschlüssel. `"HKCU"` oder `"HKLM"`.
  - `path`: Pfad zum Registry-Key.
  - `name`: Optional. Name des Wertes.
- **Rückgabe:**
  `BoolVal`

---

## reg.Delete(root, path [, name])
- **Konkret:**
  Löscht einen einzelnen Wert oder einen gesamten Key rekursiv inklusive aller Unterkeys.
  Mit 2 Argumenten wird der gesamte Pfad rekursiv gelöscht.
  Mit 3 Argumenten wird nur der angegebene Wert gelöscht.
- **Hinweis:**
  Die rekursive Key-Löschung ist unwiderruflich. Alle Unterkeys werden ohne Rückfrage entfernt.
  Gilt auch für `reg.WriteProtectedValue`-Werte: `reg.Delete` unterscheidet nicht zwischen normalen und DPAPI-verschlüsselten Werten.
- **Parameter:**
  - `root`: Stammschlüssel. `"HKCU"` oder `"HKLM"`.
  - `path`: Pfad zum Registry-Key.
  - `name`: Optional. Name des zu löschenden Wertes.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `ErrorVal` bei Fehler.