# 🔐 keystore.* – Java-Keystores verwalten

Dient zur Verwaltung von Java-Keystores (JKS, PKCS12, JCEKS): Aliase auflisten, prüfen, Details abfragen und löschen.
Arbeitet über das JDK-Werkzeug `keytool`, das auf dem Rechner installiert sein muss.

---

## Abfragen

## keystore.List(path, password [, onlyExpired, type, keytool])
- **Konkret:**
  Gibt die Aliase eines Keystores als Array zurück. Mit `onlyExpired` nur abgelaufene Einträge.
  Einträge ohne Gültigkeitsdatum (z. B. `SecretKeyEntry`) gelten nie als abgelaufen.
- **Parameter:**
  - `path`: `StrVal`. Pfad zur Keystore-Datei (relative Pfade werden aufgelöst).
  - `password`: `StrVal`. Store-Passwort.
  - `onlyExpired`: Optional. `BoolVal`. `True` = nur abgelaufene Einträge. Standard: `False`.
  - `type`: Optional. `StrVal`. `JKS`, `PKCS12` oder `JCEKS`. Leer = automatisch erkannt.
  - `keytool`: Optional. `StrVal`. Voller Pfad zu `keytool` (siehe Hinweise).
- **Rückgabe:**
  `ArrVal` mit den Aliasen (`StrVal`) bei Erfolg, Runtime Error bei technischem Fehler.

---

## keystore.Find(path, password, alias [, type, keytool])
- **Konkret:**
  Prüft, ob ein Alias im Keystore existiert.
- **Parameter:**
  - `path`: `StrVal`. Pfad zur Keystore-Datei.
  - `password`: `StrVal`. Store-Passwort.
  - `alias`: `StrVal`. Gesuchter Alias (darf nicht leer sein).
  - `type`: Optional. `StrVal`. `JKS`, `PKCS12` oder `JCEKS`.
  - `keytool`: Optional. `StrVal`. Voller Pfad zu `keytool`.
- **Rückgabe:**
  `BoolVal`: `True` = Alias vorhanden, `False` = nicht vorhanden. Technische Fehler (falsches Passwort, Datei nicht gefunden, `keytool` fehlt) sind ein Runtime Error.

---

## keystore.Info(path, password [, type, keytool])
- **Konkret:**
  Gibt alle Einträge eines Keystores mit Details als 2D-Array zurück.
- **Parameter:**
  - `path`: `StrVal`. Pfad zur Keystore-Datei.
  - `password`: `StrVal`. Store-Passwort.
  - `type`: Optional. `StrVal`. `JKS`, `PKCS12` oder `JCEKS`.
  - `keytool`: Optional. `StrVal`. Voller Pfad zu `keytool`.
- **Rückgabe:**
  `ArrVal` mit einer Zeile pro Eintrag, Spalten:
  | Spalte | Inhalt |
  |---|---|
  | 0 | Alias (`StrVal`) |
  | 1 | Typ, z. B. `PrivateKeyEntry`, `trustedCertEntry` (`StrVal`) |
  | 2 | Owner des ersten Zertifikats (`StrVal`) |
  | 3 | Gültig bis, UTC, `yyyy-MM-dd HH:mm:ss`, leer ohne Datum (`StrVal`) |
  | 4 | Abgelaufen (`BoolVal`) |

---

## Löschen

## keystore.Delete(path, password, alias [, type, keytool])
- **Konkret:**
  Löscht genau einen Alias aus dem Keystore. Das Löschen ist endgültig, vorher eine Kopie der Keystore-Datei anlegen.
- **Parameter:**
  - `path`: `StrVal`. Pfad zur Keystore-Datei.
  - `password`: `StrVal`. Store-Passwort.
  - `alias`: `StrVal`. Zu löschender Alias (darf nicht leer sein).
  - `type`: Optional. `StrVal`. `JKS`, `PKCS12` oder `JCEKS`.
  - `keytool`: Optional. `StrVal`. Voller Pfad zu `keytool`.
- **Rückgabe:**
  `ArrVal` `{BoolVal, StrVal}`:
  - `{True, "Erfolgreich gelöscht"}`: Alias entfernt.
  - `{False, "Eintrag konnte nicht gefunden werden"}`: Alias existiert nicht (kein Fehler).

  Technische Fehler sind ein Runtime Error.

---

## Hinweise

- **keytool-Suche:** Reihenfolge: Parameter `keytool`, Umgebungsvariable `VBX_KEYTOOL`, `JAVA_HOME/bin`, `PATH`. Wird nichts gefunden, folgt ein Runtime Error.
- **Passwort:** Wird nicht als Kommandozeilenargument übergeben, sondern per Umgebungsvariable nur an den `keytool`-Prozess. Es erscheint nicht in der Prozessliste.
- **UTC:** `keytool` wird auf Englisch und UTC gezwungen, alle Datumswerte sind UTC.
- **Aliase bei JKS:** `keytool` speichert Aliase in Kleinbuchstaben.
- **Mehrere Aliase löschen:** `keystore.Delete` löscht nur einen Alias pro Aufruf. Für z. B. alle abgelaufenen Einträge das Ergebnis von `keystore.List(..., True)` in einer Schleife abarbeiten.
- **Fehlermeldungen** beginnen mit dem Funktionsnamen, z. B. `keystore.Find: keytool: keystore password was incorrect`.
- Erfordert ein installiertes JDK/JRE mit `keytool`.