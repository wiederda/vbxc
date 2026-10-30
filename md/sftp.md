# 📡 sftp.* – SFTP-Funktionen

Dient zur Verbindung, Navigation und Übertragung von Dateien via SFTP.
Verbindungen werden über einen Alias verwaltet – `sftp.Connect` oder `sftp.ConnectWithKey` muss zuerst aufgerufen werden.
Standardmäßig wird der Host-Key nicht geprüft (praktisch im eigenen, vertrauten Netz). Bei Verbindungen außerhalb des eigenen Netzes wird dringend empfohlen, `knownHostsPath` anzugeben.

---

## sftp.Connect(host, user, password, alias [, knownHostsPath, port])
- **Konkret:**
  Öffnet eine SFTP-Verbindung per Passwort-Authentifizierung und speichert sie unter einem Alias.
  Existiert unter dem Alias bereits eine Verbindung, wird diese vorher sauber geschlossen (kein Leak).
  Ohne `knownHostsPath` wird der Host-Key nicht geprüft – ausreichend für Verbindungen im eigenen, vertrauten Netz, aber unsicher bei Verbindungen über das Internet oder zu fremden Servern (anfällig für Man-in-the-Middle).
  Mit `knownHostsPath` wird der Host-Key gegen eine `known_hosts`-Datei geprüft; ist der Host dort nicht verzeichnet, schlägt die Verbindung mit einem Fehler fehl.
- **Parameter:**
  - `host`: Hostname oder IP-Adresse.
  - `user`: Benutzername.
  - `password`: Passwort.
  - `alias`: Frei wählbarer Name für diese Verbindung.
  - `knownHostsPath`: Optional. Pfad zu einer `known_hosts`-Datei zur Host-Key-Verifikation. Empfohlen außerhalb des eigenen Netzes.
  - `port`: Optional. SSH/SFTP-Port (Standard meist `22`).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler (Verbindung fehlgeschlagen, Host-Key nicht vertrauenswürdig, etc.).
- **Timeout:** 10 Sekunden.

---

## sftp.ConnectWithKey(host, user, keyPath, alias [, knownHostsPath, port])
- **Konkret:**
  Öffnet eine SFTP-Verbindung per Private-Key-Authentifizierung statt Passwort.
  Passt gut zu Schlüsseln aus `GenerateSSHKey`.
  Verhalten bezüglich `knownHostsPath` identisch zu `sftp.Connect`.
- **Parameter:**
  - `host`: Hostname oder IP-Adresse.
  - `user`: Benutzername.
  - `keyPath`: Pfad zum privaten SSH-Schlüssel.
  - `alias`: Frei wählbarer Name für diese Verbindung.
  - `knownHostsPath`: Optional. Pfad zu einer `known_hosts`-Datei. Empfohlen außerhalb des eigenen Netzes.
  - `port`: Optional. SSH/SFTP-Port (Standard meist `22`).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler (Key nicht lesbar/parsbar, Verbindung fehlgeschlagen, etc.).
- **Timeout:** 10 Sekunden.

---

## sftp.Close(alias)
- **Konkret:**
  Schließt eine SFTP-Verbindung und entfernt den Alias.
- **Parameter:**
  - `alias`: Verbindungsalias.
- **Rückgabe:**
  `BoolVal` (`true`) wenn gefunden und geschlossen, `false` wenn Alias unbekannt.

---

## sftp.Upload(alias, localPath, remotePath)
- **Konkret:**
  Lädt eine lokale Datei auf den SFTP-Server hoch.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `localPath`: Pfad zur lokalen Quelldatei.
  - `remotePath`: Zielpfad auf dem Server.
- **Rückgabe:**
  `NumVal` (Anzahl übertragener Bytes), `ErrorVal` bei Fehler (lokale Datei nicht lesbar, Remote-Datei kann nicht erstellt werden, etc.).

---

## sftp.Download(alias, remotePath, localPath)
- **Konkret:**
  Lädt eine Datei vom SFTP-Server herunter.
  Das lokale Zielverzeichnis muss bereits existieren.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `remotePath`: Quellpfad auf dem Server.
  - `localPath`: Zielpfad auf dem lokalen Rechner.
- **Rückgabe:**
  `NumVal` (Anzahl übertragener Bytes), `ErrorVal` bei Fehler (Remote-Datei nicht lesbar, lokales Zielverzeichnis existiert nicht, etc.).

---

## sftp.DownloadFolder(alias, remotePath, localBasePath [, recursive, progress])
- **Konkret:**
  Lädt einen kompletten Remote-Ordner herunter, ohne dass die enthaltenen Dateinamen vorab bekannt sein müssen.
  Legt lokal automatisch einen Ordner mit dem Namen des letzten `remotePath`-Segments an (z. B. `remotePath = "Movies/Zurück.in.die.Zukunft.1985"` → lokaler Ordner `Zurück.in.die.Zukunft.1985`, nicht der komplette Pfad).
  Mit `recursive = true` werden auch Unterordner mit identischer Struktur angelegt und heruntergeladen; ohne (Standard) werden nur Dateien der obersten Ebene übertragen, Unterordner ignoriert.
  Mit `progress = true` wird der Fortschritt als `[aktuell/gesamt] Dateiname` auf der Konsole ausgegeben (vorab wird die Gesamtzahl ermittelt).
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `remotePath`: Zu ladendes Verzeichnis auf dem Server.
  - `localBasePath`: Lokales Basisverzeichnis, in dem der neue Ordner angelegt wird.
  - `recursive`: Optional. `BoolVal` – bei `true` werden Unterordner mit einbezogen (Standard: `false`).
  - `progress`: Optional. `BoolVal` – bei `true` Fortschrittsausgabe auf der Konsole (Standard: `false`).
- **Rückgabe:**
  `NumVal` (Anzahl heruntergeladener Dateien), `ErrorVal` bei Fehler.

**Beispiel (kompletter Workflow):**
```vbx
#use sftp

sftp.Connect("IP", Port, "meinUser", "meinPasswort", "nas")

Dim inhalt = sftp.List("nas", "Test")
Inspect(inhalt)

Dim anzahl = sftp.DownloadFolder("nas", "Test/Zurück.in.die.Zukunft.1985", "C:\Downloads")
Print "Heruntergeladen: " & anzahl & " Dateien"

sftp.Close("nas")
```

---

## sftp.FindByExt(alias, remotePath, ext [, all])
- **Konkret:**
  Sucht Dateien mit einer bestimmten Endung in einem Remote-Verzeichnis, ohne dass der genaue Dateiname bekannt sein muss.
  Durchsucht nur die direkte Ebene von `remotePath`, nicht rekursiv.
  Ergebnisse werden natürlich sortiert (wie im Explorer), bevor bei `all = false` die erste Datei ausgewählt wird – damit "erste Datei" konsistent und nachvollziehbar ist, statt von der zufälligen Serverreihenfolge abzuhängen.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `remotePath`: Zu durchsuchendes Verzeichnis auf dem Server.
  - `ext`: Dateiendung, mit oder ohne führenden Punkt (z. B. `"xlsx"` oder `".xlsx"`).
  - `all`: Optional. `BoolVal` – bei `true` werden alle Treffer als Array zurückgegeben, bei `false` (Standard) nur der erste Treffer als einzelner String.
- **Rückgabe:**
  Bei `all = false`: `StrVal` mit dem vollständigen Remote-Pfad der ersten gefundenen Datei, oder leerer String `""` wenn keine Datei gefunden wurde.
  Bei `all = true`: `ArrVal` mit allen gefundenen Remote-Pfaden (leer, wenn keine gefunden wurde).
  `ErrorVal` bei unbekanntem Alias oder wenn das Verzeichnis nicht gelesen werden kann.

---

## sftp.DownloadByExt(alias, remotePath, ext, localBasePath [, all])
- **Konkret:**
  Sucht Dateien mit einer bestimmten Endung in einem Remote-Verzeichnis und lädt sie direkt herunter, ohne dass der genaue Dateiname bekannt sein muss. Kombiniert `sftp.FindByExt` und `sftp.Download` in einem Aufruf.
  Der lokale Dateiname entspricht immer dem Remote-Dateinamen (keine Umbenennung). Für einen abweichenden lokalen Namen stattdessen `sftp.FindByExt` gefolgt von `sftp.Download` mit explizitem `localPath` nutzen.
  Das lokale Zielverzeichnis wird automatisch angelegt, falls es noch nicht existiert.
  Durchsucht nur die direkte Ebene von `remotePath`, nicht rekursiv.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `remotePath`: Zu durchsuchendes Verzeichnis auf dem Server.
  - `ext`: Dateiendung, mit oder ohne führenden Punkt (z. B. `"xlsx"` oder `".xlsx"`).
  - `localBasePath`: Lokales Zielverzeichnis.
  - `all`: Optional. `BoolVal` – bei `true` werden alle Treffer heruntergeladen, bei `false` (Standard) nur die erste natürlich sortierte Datei.
- **Rückgabe:**
  `NumVal` (Anzahl heruntergeladener Dateien, `0` wenn keine passende Datei gefunden wurde), `ErrorVal` bei Verbindungs- oder Übertragungsfehler.

---

## sftp.List(alias, remotePath [, sortBy, desc])
- **Konkret:**
  Listet den Inhalt eines Remote-Verzeichnisses.
  `remotePath` bezieht sich auf das Dateisystem des SFTP-Servers, nicht auf den lokalen Rechner. Je nach Server-Konfiguration (z. B. Synology-Freigaben) kann der sichtbare Wurzelpfad vom vollen Dateisystempfad abweichen – im Zweifel zuerst mit `"."` oder `"/"` testen.
  Optional wird das Ergebnis direkt sortiert zurückgegeben, ohne dass danach ein separater Sortier-Aufruf nötig ist.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `remotePath`: Zu listendes Verzeichnis auf dem Server.
  - `sortBy`: Optional. Sortierkriterium: `"name"` (Standard, auch wenn nicht angegeben), `"size"` oder `"modTime"`.
  - `desc`: Optional. `BoolVal` – bei `true` absteigend sortieren (z. B. neueste Datei zuerst bei `"modTime"`). Standard: aufsteigend.
- **Rückgabe:**
  `ArrVal`
  Array von Maps mit folgenden Schlüsseln:

  | Schlüssel | Typ      | Inhalt                          |
  |-----------|----------|----------------------------------|
  | `name`    | `StrVal` | Datei-/Ordnername               |
  | `size`    | `NumVal` | Größe in Bytes                  |
  | `isDir`   | `BoolVal`| `true` bei Verzeichnis           |
  | `modTime` | `StrVal` | Letzte Änderung (ISO 8601/RFC3339) |

  `ErrorVal` wenn der Alias unbekannt ist oder das Verzeichnis nicht gelesen werden kann.

**Beispiel:**
```vbx
#use sftp

sftp.Connect("IP", Port, "test", "meinPasswort", "nas")

Dim inhalt = sftp.List("nas", "TEST")
For Each eintrag In inhalt
    Print eintrag["name"] & " - " & FormatSize(eintrag["size"]) & " - Verzeichnis: " & eintrag["isDir"]
Next

sftp.Close("nas")
```

**Verbindung zu einem fremden NAS außerhalb des eigenen Netzes, mit Host-Key-Prüfung:**
```vbx
sftp.Connect("extern.example.com", 22, "test", "passwort", "nas2", "/home/test/.ssh/known_hosts")
```