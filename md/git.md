# 🌿 git.* – Git-Funktionen

Dient zum Klonen, Synchronisieren und Verwalten von Git-Repositories.
Basiert auf [go-git](https://github.com/go-git/go-git) – einer reinen Go-Implementierung
von Git. Es wird **kein installiertes Git-Binary benötigt**; alle Funktionen arbeiten
direkt auf dem `.git`-Verzeichnis.

Alle `path`-Parameter sind optional. Wird `path` weggelassen, wirkt die Funktion – wie
bei echten `git`-Kommandos im Terminal – auf das **aktuelle Arbeitsverzeichnis**.

**Einschränkung:** Kein Diff gegen das unversionierte Arbeitsverzeichnis (wie
`git diff` ohne Argumente). `git.Diff` vergleicht ausschließlich zwei Commits
(bzw. einen Commit gegen HEAD).

---

## Klonen

## git.Clone(url [, path])
- **Konkret:**
  Klont ein Git-Repository ohne Authentifizierung.
  Das Zielverzeichnis darf noch nicht existieren. Wird `path` weggelassen, wird der
  Verzeichnisname wie bei `git clone` aus dem letzten Pfadsegment der URL abgeleitet
  (ohne `.git`-Endung), relativ zum aktuellen Arbeitsverzeichnis.
- **Parameter:**
  - `url`: Repository-URL.
  - `path`: Optional. Zielverzeichnis.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `"error: ..."` bei Fehler.

---

## git.CloneWithToken(url, token [, path] [, username])
- **Konkret:**
  Klont ein Git-Repository per HTTPS mit Token-Authentifizierung (Basic-Auth, z. B.
  GitHub Personal Access Token). Das Zielverzeichnis darf noch nicht existieren. Wird
  `path` weggelassen, wird der Verzeichnisname wie bei `git.Clone` aus der URL
  abgeleitet.
- **Parameter:**
  - `url`: HTTPS-Repository-URL.
  - `token`: Zugriffstoken.
  - `path`: Optional. Zielverzeichnis.
  - `username`: Optional. Benutzername für Basic-Auth (Standard: `"x-access-token"`).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `"error: ..."` bei Fehler.

---

## git.CloneWithKey(url, keyPath [, path] [, knownHostsPath])
- **Konkret:**
  Klont ein Git-Repository per SSH mit Schlüssel-Authentifizierung. Das
  Zielverzeichnis darf noch nicht existieren. Wird `path` weggelassen, wird der
  Verzeichnisname wie bei `git.Clone` aus der URL abgeleitet.
  Der Host-Key wird wie bei `sftp.ConnectWithKey`/`ssh.RebootWithKey` behandelt: Mit
  `knownHostsPath` wird gegen eine known_hosts-Datei geprüft (TOFU – unbekannte Hosts
  werden beim ersten Kontakt automatisch eingetragen). Ohne `knownHostsPath` wird der
  Host-Key ungeprüft akzeptiert.
- **Parameter:**
  - `url`: SSH-URL (`git@host:user/repo.git`).
  - `keyPath`: Pfad zum privaten SSH-Schlüssel.
  - `path`: Optional. Zielverzeichnis.
  - `knownHostsPath`: Optional. Pfad zur known_hosts-Datei.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `"error: ..."` bei Fehler.

---

## Remote-Synchronisation

## git.Pull([path] [, keyPath] [, knownHostsPath] [, token] [, username])
- **Konkret:**
  Holt Änderungen vom Remote und führt sie in den aktuellen Branch zusammen
  (Fast-Forward). Für authentifizierte Remotes: entweder `keyPath` (SSH, optional
  mit `knownHostsPath` für TOFU-Host-Key-Prüfung wie bei `git.CloneWithKey`) oder
  `token` (HTTPS, optional mit `username`) angeben. Ohne beides läuft der Pull ohne
  Auth (nur für unauthentifizierte Remotes ausreichend). Um einen späteren
  Parameter zu setzen, ohne einen davorliegenden zu verwenden, muss ein leerer
  String übergeben werden.
- **Parameter:**
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
  - `keyPath`: Optional. Pfad zum privaten SSH-Schlüssel.
  - `knownHostsPath`: Optional. Pfad zur known_hosts-Datei.
  - `token`: Optional. HTTPS-Zugriffstoken.
  - `username`: Optional. Benutzername für Basic-Auth (Standard:
    `"x-access-token"`).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg (auch wenn bereits aktuell), `"error: ..."` bei Fehler.

---

## git.Push([path] [, keyPath] [, knownHostsPath] [, token] [, username])
- **Konkret:**
  Sendet lokale Commits an den Remote `origin` auf dem aktuellen Branch. Auth wie
  bei `git.Pull`.
- **Parameter:**
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
  - `keyPath`: Optional. Pfad zum privaten SSH-Schlüssel.
  - `knownHostsPath`: Optional. Pfad zur known_hosts-Datei.
  - `token`: Optional. HTTPS-Zugriffstoken.
  - `username`: Optional. Benutzername für Basic-Auth (Standard:
    `"x-access-token"`).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg (auch wenn nichts zu pushen war), `"error: ..."` bei
  Fehler.

---

## git.Fetch([path] [, keyPath] [, knownHostsPath] [, token] [, username])
- **Konkret:**
  Holt Änderungen vom Remote, ohne sie zu übernehmen. Aktualisiert
  Remote-Tracking-Branches, ohne den Arbeitsbereich zu verändern. Auth wie bei
  `git.Pull`.
- **Parameter:**
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
  - `keyPath`: Optional. Pfad zum privaten SSH-Schlüssel.
  - `knownHostsPath`: Optional. Pfad zur known_hosts-Datei.
  - `token`: Optional. HTTPS-Zugriffstoken.
  - `username`: Optional. Benutzername für Basic-Auth (Standard:
    `"x-access-token"`).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg (auch wenn bereits aktuell), `"error: ..."` bei Fehler.

---

## Änderungen

## git.Add(pattern [, path])
- **Konkret:**
  Fügt eine Datei zum Git-Index hinzu (staging).
- **Parameter:**
  - `pattern`: Dateipfad relativ zum Repo-Root.
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `"error: ..."` bei Fehler.

---

## git.Commit(message [, path])
- **Konkret:**
  Erstellt einen Commit mit allen gestagten Änderungen. Autor wird fest als
  `VBX <vbx@localhost>` gesetzt.
- **Parameter:**
  - `message`: Commit-Nachricht.
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `StrVal` (Hash des erzeugten Commits), `"error: ..."` bei Fehler.

---

## git.Remove(pattern [, path])
- **Konkret:**
  Entfernt eine Datei aus Git-Index und Arbeitsverzeichnis. Löscht die Datei
  physisch.
- **Parameter:**
  - `pattern`: Dateipfad relativ zum Repo-Root.
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `"error: ..."` bei Fehler.

---

## git.QuickPush(message [, pattern] [, path] [, keyPath] [, knownHostsPath] [, token] [, username])
- **Konkret:**
  Stagt alle Änderungen, erstellt einen Commit und pusht ihn – in einem Aufruf.
  Entspricht dem Dreischritt `git add` + `git commit -m` + `git push`, wie es z. B.
  der „Commit & Push"/„Sync"-Knopf in VSCode macht. **Kein Force-Push** – schlägt
  der Push fehl (z. B. weil der Remote neuere Commits hat), wird ein Fehler
  zurückgegeben; der lokale Commit bleibt in diesem Fall bestehen. Autor wird wie
  bei `git.Commit` als `VBX <vbx@localhost>` gesetzt. Auth-Parameter wie bei
  `git.Pull`.
- **Parameter:**
  - `message`: Commit-Nachricht.
  - `pattern`: Optional. Zu stagender Pfad relativ zum Repo-Root (Standard: `"."`,
    also alle Änderungen).
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
  - `keyPath`: Optional. Pfad zum privaten SSH-Schlüssel.
  - `knownHostsPath`: Optional. Pfad zur known_hosts-Datei.
  - `token`: Optional. HTTPS-Zugriffstoken.
  - `username`: Optional. Benutzername für Basic-Auth (Standard:
    `"x-access-token"`).
- **Rückgabe:**
  `StrVal` (Hash des erzeugten Commits), `"error: ..."` bei Fehler.

---

## git.Status([path])
- **Konkret:**
  Liefert den Status des Arbeitsverzeichnisses: geänderte, neu hinzugefügte,
  gelöschte und unversionierte Dateien.
- **Parameter:**
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `MapVal`
  Vier Einträge `modified`, `added`, `deleted`, `untracked`, jeweils `ArrVal` mit
  Dateipfaden.

---

## git.Log([limit] [, path])
- **Konkret:**
  Liefert die Commit-Historie ausgehend von HEAD (Autor, Nachricht, Datum).
- **Parameter:**
  - `limit`: Optional. Maximale Anzahl Commits (Standard: `0` = alle).
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `ArrVal`
  Array von `MapVal`-Einträgen mit `hash`, `author`, `message`, `date`.

---

## git.Diff(commitA [, commitB] [, path])
- **Konkret:**
  Zeigt den Diff zwischen zwei Commits. Wird `commitB` weggelassen oder leer
  gelassen, bedeutet das: Vergleich gegen HEAD. Ein Diff gegen das unversionierte
  Arbeitsverzeichnis (wie `git diff` ohne Argumente) wird **nicht** unterstützt.
- **Parameter:**
  - `commitA`: Älterer Commit-Hash.
  - `commitB`: Optional. Neuerer Commit-Hash (Standard: HEAD).
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `StrVal` (unified Diff), `"error: ..."` bei Fehler.

---

## git.ResetHard(commitHash [, path])
- **Konkret:**
  Setzt das Repository hart auf einen bestimmten Commit zurück. Alle lokalen
  Änderungen und Commits nach diesem Hash gehen unwiderruflich verloren. Typischer
  Anwendungsfall: mit `git.Log` den letzten funktionierenden Commit finden, dann
  hierher zurücksetzen.
- **Parameter:**
  - `commitHash`: Ziel-Commit-Hash (voll oder gekürzt).
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `"error: ..."` bei Fehler.

---

## git.Reset([mode] [, path])
- **Konkret:**
  Hebt das Staging von Änderungen auf (unstage), ohne Dateien im
  Arbeitsverzeichnis zu verändern.
  Der Unterschied zwischen den beiden Modi liegt darin, wie weit zurückgesetzt
  wird: `soft` bewegt nur den HEAD-Zeiger auf den letzten Commit – Index und
  Arbeitsverzeichnis bleiben unangetastet, alle vorher committeten Änderungen
  liegen danach wieder gestaged bereit (so, als wären sie gerade erst per
  `git.Add` hinzugefügt worden). `mixed` (Standard) setzt zusätzlich auch den
  Index zurück – die Änderungen bleiben zwar im Arbeitsverzeichnis erhalten,
  sind danach aber ungestaged und müssten erst wieder per `git.Add` hinzugefügt
  werden, um erneut committet zu werden. Das Arbeitsverzeichnis selbst wird in
  keinem der beiden Modi verändert (dafür ist `git.ResetHard` zuständig).
- **Parameter:**
  - `mode`: Optional. `"soft"` (nur HEAD bewegen, Änderungen bleiben gestaged)
    oder `"mixed"` (HEAD und Index bewegen, Änderungen werden ungestaged;
    Standard).
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `"error: ..."` bei Fehler.

---

## Branches

## git.CurrentBranch([path])
- **Konkret:**
  Liefert den Namen des aktuell ausgecheckten Branches.
- **Parameter:**
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `StrVal` (Branch-Name, z. B. `"main"`), `"error: ..."` bei Fehler.

---

## git.Checkout(branch [, create] [, path])
- **Konkret:**
  Wechselt den Branch oder erstellt einen neuen (`create = true`).
- **Parameter:**
  - `branch`: Branch-Name.
  - `create`: Optional. `BoolVal` (Standard: `false`).
  - `path`: Optional. Repository-Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `"error: ..."` bei Fehler.

---

## Sonstiges

## git.IsRepo([path])
- **Konkret:**
  Prüft, ob ein Verzeichnis ein gültiges Git-Repository ist (Vorhandensein eines
  gültigen `.git`-Verzeichnisses).
- **Parameter:**
  - `path`: Optional. Zu prüfender Pfad (Standard: aktuelles Arbeitsverzeichnis).
- **Rückgabe:**
  `BoolVal`