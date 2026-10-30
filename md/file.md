# 📁 file.* – Datei- & Pfadfunktionen

Dient zum Lesen, Schreiben, Suchen, Überwachen und Verwalten von Dateien und Pfaden.
Plattformübergreifend (Windows, Linux, macOS). Schreiboperationen nutzen `absPathVal` zur Pfadabsicherung.

---

## Rückgabekonvention

- **Aktionen** (schreiben, kopieren, löschen, umbenennen ...) liefern `BoolVal` (`true`) bei Erfolg und `ErrorVal` bei Fehler.
- **Datenfunktionen** (lesen, suchen, hashen ...) liefern ihren Wert, bei Fehler `ErrorVal`.
- Fehler werden mit `IsError()` und `ErrorText()` geprüft. Ein Vergleich wie `If file.Move(...) = true` ist für den Erfolgsfall zulässig, ein `ErrorVal` als eigenständige Anweisung ohne Zuweisung bricht jedoch das Skript ab (außer innerhalb von `Try/Catch`).
- Ausnahme im Namespace `net`: `net.Download` liefert bei Erfolg den Zielpfad (`StrVal`), nicht `true`.

---

## file.WriteAllText(path, content)
- **Konkret:**
  Schreibt den gesamten Text in eine Datei. Vorhandene Inhalte werden überschrieben.
- **Parameter:**
  - `path`: Zieldatei.
  - `content`: Zu schreibender Text.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.

---

## file.Create(path)
- **Konkret:**
  Erstellt eine leere Datei, falls diese noch nicht existiert.
  Das Zielverzeichnis muss bereits vorhanden sein.
- **Parameter:**
  - `path`: Zielpfad der Datei.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg, auch wenn die Datei bereits existierte), `ErrorVal` bei Fehler.

---

## file.StreamWrite(path, content)
- **Konkret:**
  Hängt Text gepuffert an eine Datei an.
  Erstellt die Datei, falls sie noch nicht existiert.
  Das Zielverzeichnis muss vorhanden sein.
- **Parameter:**
  - `path`: Zieldatei.
  - `content`: Anzuhängender Text.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.

---

## file.AppendAllText(path, text)
- **Konkret:**
  Hängt Text ungepuffert an eine Datei an.
  Erstellt die Datei, falls sie noch nicht existiert.
  Das Zielverzeichnis muss vorhanden sein.
- **Parameter:**
  - `path`: Zieldatei.
  - `text`: Anzuhängender Text.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.

---

## file.AppendLine(path, line)
- **Konkret:**
  Hängt eine Zeile an eine Datei an. Fügt automatisch einen Zeilenumbruch (`\n`) ein.
  Erstellt die Datei, falls sie noch nicht existiert. Das Zielverzeichnis muss vorhanden sein.
- **Parameter:**
  - `path`: Zieldatei.
  - `line`: Anzuhängende Zeile (ohne eigenen Zeilenumbruch).
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.

---

## file.Exists(path)
- **Konkret:**
  Prüft, ob eine Datei unter dem angegebenen Pfad existiert.
  Verzeichnisse zählen nicht als Datei.
- **Parameter:**
  - `path`: Zu prüfender Pfad.
- **Rückgabe:**
  `BoolVal`

---

## file.TextStats(path)
- **Konkret:**
  Liest eine Datei und zählt Wörter, Zeichen (ohne Leerzeichen) und Leerzeichen im Inhalt.
- **Parameter:**
  - `path`: Quelldatei.
- **Rückgabe:**
  `ArrVal`
  Format: `[WordCount, CharCount, SpaceCount]`. `CharCount` zählt alle Zeichen außer Leerzeichen (Buchstaben, Ziffern sowie Satz-/Sonderzeichen zählen mit) — `CharCount + SpaceCount` ergibt die Gesamtzeichenzahl des Dateiinhalts. `ErrorVal` bei Lesefehler.

---

## file.FindWords(words, inputFile, outputFile)
- **Konkret:**
  Sucht beliebige Wörter aus einem Array in einer Datei und schreibt alle Trefferzeilen mit vorangestellter Zeilennummer in eine Ausgabedatei.
  `words` muss ein Array aus Strings sein (leere Strings werden ignoriert, das Array darf nach dem Filtern nicht komplett leer sein).
  Eine Zeile gilt als Treffer, sobald mindestens eines der Suchwörter darin vorkommt (Teilstring-Suche, Groß-/Kleinschreibung wird beachtet).
  Die Ausgabedatei wird erzeugt bzw. überschrieben, falls sie schon existiert.
  Unterstützt auch sehr lange Zeilen (bis 16 MB pro Zeile).
- **Rückgabe:**
  `NumVal` (Anzahl der gefundenen Trefferzeilen)

---

## file.FindLine(line, inputFile)
- **Konkret:**
  Prüft, ob eine Zeile exakt in einer Datei vorkommt, und bricht beim ersten Treffer ab.
  `line` muss ein nicht-leerer String sein.
  Verglichen wird die gesamte Zeile, führende und nachfolgende Leerzeichen/Tabs werden auf beiden Seiten ignoriert. Teiltreffer innerhalb einer Zeile zählen nicht (z. B. `x = 1 '@platform linux` trifft nicht auf `'@platform linux`). Groß-/Kleinschreibung wird beachtet.
  Geeignet für Direktiven wie `'@platform linux`.
  Es wird keine Ausgabedatei erzeugt, die Eingabedatei wird nur gelesen.
  Unterstützt auch sehr lange Zeilen (bis 16 MB pro Zeile).
- **Rückgabe:**
  `BoolVal` (`true`, wenn die Zeile mindestens einmal vorkommt, sonst `false`)

---

## file.FindWord(word, inputFile)
- **Konkret:**
  Prüft, ob ein Wort als eigenständiges Wort in einer Datei vorkommt, und bricht beim ersten Treffer ab.
  `word` muss ein nicht-leerer String sein.
  Die Suche erfolgt zeilenweise. Ein Treffer zählt nur, wenn direkt davor und danach kein Buchstabe, keine Ziffer und kein Unterstrich steht (Teiltreffer in längeren Wörtern wie "Testing" für "Test" werden ignoriert). Groß-/Kleinschreibung wird beachtet.
  Beginnt oder endet das Suchwort selbst mit einem Sonderzeichen (z. B. `foo.`), wird auf dieser Seite keine Wortgrenze geprüft.
  Es wird keine Ausgabedatei erzeugt, die Eingabedatei wird nur gelesen.
  Unterstützt auch sehr lange Zeilen (bis 16 MB pro Zeile).
- **Rückgabe:**
  `BoolVal` (`true`, wenn das Wort mindestens einmal als ganzes Wort vorkommt, sonst `false`)

---

## file.HasContent(path)
- **Konkret:**
  Prüft, ob eine Datei existiert und mehr als 0 Byte Inhalt hat.
- **Parameter:**
  - `path`: Zu prüfender Pfad.
- **Rückgabe:**
  `BoolVal`
  `true` = Datei existiert und ist größer als 0 Byte. `false` = Datei existiert nicht oder ist leer.

---

## file.Delete(path)
- **Konkret:**
  Löscht eine Datei. Existiert die Datei nicht, wird kein Fehler zurückgegeben.
- **Parameter:**
  - `path`: Zu löschende Datei.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler. Eine nicht vorhandene Datei ist kein Fehler und ergibt ebenfalls `true`.

---

## file.Copy(src, dst, [overwrite])
- **Konkret:**
  Kopiert eine Datei.
  Ohne `overwrite` (Standard `false`) schlägt der Aufruf fehl, falls das Ziel bereits existiert.
  Das Zielverzeichnis muss vorhanden sein.
- **Parameter:**
  - `src`: Quelldatei.
  - `dst`: Zieldatei.
  - `overwrite`: Optional. `BoolVal` – bei `true` wird ein bereits vorhandenes Ziel überschrieben. Standard: `false`.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler (Quelle fehlt, Zielverzeichnis fehlt, Ziel existiert bereits ohne `overwrite`).

---

## file.Move(src, dst, [overwrite])
- **Konkret:**
  Verschiebt oder benennt eine Datei um.
  Versucht zuerst ein atomares Rename; bei Cross-Drive-Operationen wird auf Copy+Delete zurückgegriffen.
  Ohne `overwrite` (Standard `false`) schlägt der Aufruf fehl, falls das Ziel bereits existiert.
- **Parameter:**
  - `src`: Quelldatei.
  - `dst`: Zieldatei.
  - `overwrite`: Optional. `BoolVal` – bei `true` wird ein bereits vorhandenes Ziel überschrieben. Standard: `false`.
    Typischer Fall: `file.Move(datei, datei & ".old", true)` scheitert nicht an einer übrig gebliebenen `.old`.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.

---

## file.Rename(oldPath, newPath)
- **Konkret:**
  Benennt eine Datei um (direktes `os.Rename`).
  Kann auch zum Verschieben innerhalb desselben Dateisystems genutzt werden.
- **Parameter:**
  - `oldPath`: Aktueller Pfad.
  - `newPath`: Neuer Pfad.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.
- **Hinweis:** Anders als `file.Move` gibt es hier **keinen** Cross-Drive-Fallback und **keine** Prüfung, ob `newPath` bereits existiert – Verhalten bei existierendem Ziel ist plattformabhängig.

---

## file.Compare(path1, path2)
- **Konkret:**
  Vergleicht zwei Dateien byteweise auf Gleichheit.
  Schneller Vorab-Check via Dateigröße; gleiche Pfade gelten direkt als identisch.
- **Parameter:**
  - `path1`: Erste Datei.
  - `path2`: Zweite Datei.
- **Rückgabe:**
  `NumVal`
  `0` = identisch, `1` = unterschiedlich. `ErrorVal` bei Lesefehler.

---

## file.Size(path, [unit])
- **Konkret:**
  Gibt die Größe einer Datei zurück.
- **Parameter:**
  - `path`: Zieldatei.
  - `unit`: Optional. Bei `"human"` wird die Größe als lesbarer String zurückgegeben (z. B. `"4.2 MB"`).
- **Rückgabe:**
  `NumVal` (Bytes) oder `StrVal` bei `"human"`.

---

## file.Hash(path, [algo])
- **Konkret:**
  Berechnet den kryptografischen Hash einer Datei.
  Streamt die Datei direkt in den Hash-Generator (speichereffizient).
- **Parameter:**
  - `path`: Zieldatei.
  - `algo`: Algorithmus. Unterstützt `"md5"` (Standard), `"sha1"`, `"sha224"`, `"sha256"`, `"sha384"`, `"sha512"`.
- **Rückgabe:**
  `StrVal`
  Hex-kodierter Hash-String.

---

## file.HashBatch(paths, [algo, workers])
- **Konkret:**
  Berechnet Hashes mehrerer Dateien parallel über einen Worker-Pool (Standard: 8 Worker).
  Nutzt intern dieselbe Hash-Logik wie `file.Hash`.
- **Parameter:**
  - `paths`: `ArrVal` mit Dateipfaden (`StrVal`).
  - `algo`: Optional. Algorithmus, identisch zu `file.Hash` (Standard: `"md5"`).
  - `workers`: Optional. Anzahl paralleler Worker (Standard: 8).
- **Rückgabe:**
  `MapVal` – Pfad (Schlüssel) auf Hash (`StrVal`) oder `ErrorVal` bei Lesefehler der jeweiligen Datei.

---

## file.HashVerify(hash, algo)
- **Konkret:**
  Prüft, ob ein String rein formal ein gültiger Hash-Wert des angegebenen Algorithmus ist (korrekte Länge, nur Hex-Zeichen).
  Greift nicht auf eine Datei zu – reine String-Prüfung. Nützlich um Ergebnisse aus `file.Hash`/`file.HashBatch` auf Plausibilität zu prüfen (z. B. um `ErrorVal`-Werte oder abgeschnittene/korrupte Hashes zu erkennen), bevor sie weiterverarbeitet oder in eine Datenbank geschrieben werden.
- **Parameter:**
  - `hash`: Zu prüfender String.
  - `algo`: Algorithmus. Unterstützt `"md5"`, `"sha1"`, `"sha224"`, `"sha256"`, `"sha384"`, `"sha512"`.
- **Rückgabe:**
  `BoolVal`. `ErrorVal` bei unbekanntem Algorithmus.
- **Beispiel:**
```vbx
  Dim h = file.Hash("test.mp4", "sha256")

  If file.HashVerify(h, "sha256") = False Then
      Print "Fehler beim Hashen: " & h
  Else
      Print "Hash gültig: " & h
  End If
```

---

## file.VerifyHash(path, expectedHash, [algo])
- **Konkret:**
  Berechnet den Hash einer Datei und vergleicht ihn mit einem erwarteten Wert (case-insensitiv).
  Nützlich zur Integritätsprüfung, z. B. nach einem Download oder Backup.
- **Parameter:**
  - `path`: Zieldatei.
  - `expectedHash`: Erwarteter Hash-Wert als String.
  - `algo`: Optional. Algorithmus (Standard: `"sha256"`).
- **Rückgabe:**
  `BoolVal` (`true` bei Übereinstimmung), `ErrorVal` bei Lesefehler.
- **Beispiel:**
```vbx
  If file.VerifyHash("download.zip", erwarteterHash, "sha256") Then
      Print "Datei OK"
  Else
      Print "Datei beschädigt oder manipuliert"
  End If
```

---

## file.GitBlobHash(path)
- **Konkret:**
  Berechnet den Git-Blob-SHA1 einer Datei – kompatibel zu `git hash-object`.
  Es wird der git-übliche `"blob <Länge>\0"`-Header vor den Dateiinhalt gehasht.
- **Parameter:**
  - `path`: Zieldatei.
- **Rückgabe:**
  `StrVal`
  Hex-kodierter SHA1-String, `ErrorVal` bei Lesefehler.
- **Hinweis:**
  Bei nicht vorhandener Datei liefert `GitBlobHash` einen `ErrorVal`, keinen leeren String. Für Update-Prüfungen gegen einen Remote-SHA siehe `file.GitBlobCompare`.

---

## file.GitBlobCompare(path, remoteSha)
- **Konkret:**
  Vergleicht den Git-Blob-SHA1 einer lokalen Datei mit einem Remote-SHA (z. B. Feld `sha` der GitHub-API).
  Gedacht für Update-Prüfungen: Eine nicht vorhandene Datei ist ein eigener Rückgabewert und kein Fehler.
  Der Vergleich ist case-insensitiv, Leerzeichen um `remoteSha` werden ignoriert.
- **Parameter:**
  - `path`: Lokale Datei.
  - `remoteSha`: Erwarteter Git-Blob-SHA1 als String.
- **Rückgabe:**
  `NumVal`
  `0` = identisch, `1` = unterschiedlich, `-1` = Datei nicht vorhanden. `ErrorVal` bei Lesefehler oder ungültigem Parameter.
  (Konvention analog zu `file.Compare`: `0` = gleich.)
- **Beispiel:**
```vbx
  Dim cmp = file.GitBlobCompare(localFile, remoteHash)
  If cmp <> 0 Then
      ' 1 = geändert, -1 = neu -> laden
  End If
```

---

## file.ModTime(path)
- **Konkret:**
  Gibt den Zeitpunkt der letzten Änderung einer Datei zurück.
- **Parameter:**
  - `path`: Zieldatei.
- **Rückgabe:**
  `StrVal`
  Format: ISO 8601 / RFC3339 (z. B. `"2024-03-15T14:32:00+01:00"`).

---

## file.CreateTime(path)
- **Konkret:**
  Gibt den Erstellungszeitpunkt einer Datei zurück.
- **Parameter:**
  - `path`: Zieldatei.
- **Rückgabe:**
  `StrVal`
  Format: ISO 8601 / RFC3339.

---

## file.AccessTime(path)
- **Konkret:**
  Gibt den Zeitpunkt des letzten Zugriffs auf eine Datei zurück.
- **Parameter:**
  - `path`: Zieldatei.
- **Rückgabe:**
  `StrVal`
  Format: ISO 8601 / RFC3339.

---

## file.ProductVersion(path)
- **Konkret:**
  Liest die numerische Produktversion einer ausführbaren Datei (exe/dll) aus der Versionsressource (`VS_FIXEDFILEINFO`).
  **Nur unter Windows verfügbar.** Unter Linux/macOS liefert die Funktion einen `ErrorVal`.
- **Parameter:**
  - `path`: Pfad zur exe/dll.
- **Rückgabe:**
  `StrVal`
  Format: `"Major.Minor.Build.Revision"` (z. B. `"10.0.19041.1"`). `ErrorVal`, falls die Datei keine Versionsinfo enthält, nicht lesbar ist oder die Plattform nicht unterstützt wird.
- **Hinweis:**
  Gelesen wird die numerische Version, nicht der Text aus der Stringtabelle. Trägt ein Programm dort etwas Abweichendes ein (z. B. `"1.2.3-beta"`), stimmt das Ergebnis nicht zwingend damit überein.

---

## file.ReadAllText(path)
- **Konkret:**
  Liest den gesamten Inhalt einer Datei als einen String.
  Nutzt Shared-Access (Windows-kompatibel).
- **Parameter:**
  - `path`: Quelldatei.
- **Rückgabe:**
  `StrVal`

---

## file.ReadAllLines(path)
- **Konkret:**
  Liest eine Textdatei zeilenweise ein.
  Normalisiert Zeilenumbrüche (`\r\n` → `\n`), entfernt abschließende Leerzeilen.
  Nutzt Shared-Access (Windows-kompatibel).
- **Parameter:**
  - `path`: Quelldatei.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen, ein Eintrag pro Zeile.

---

## file.LineCount(path)
- **Konkret:**
  Gibt die Anzahl der Zeilen einer Textdatei zurück.
  Nutzt Shared-Access (Windows-kompatibel), unterstützt Zeilen bis 1 MB Länge.
- **Parameter:**
  - `path`: Quelldatei.
- **Rückgabe:**
  `NumVal`, `ErrorVal` bei Lesefehler.

---

## file.Head(path, n)
- **Konkret:**
  Gibt die ersten `n` Zeilen einer Datei zurück.
  Nutzt Shared-Access (Windows-kompatibel), unterstützt Zeilen bis 1 MB Länge.
- **Parameter:**
  - `path`: Quelldatei.
  - `n`: Anzahl Zeilen (Max: 5000, analog zu `file.Tail`).
- **Rückgabe:**
  `ArrVal`, `ErrorVal` bei Lesefehler.

---

## file.ReadBytes(path)
- **Konkret:**
  Liest eine Datei als Byte-Array ein.
- **Parameter:**
  - `path`: Quelldatei.
- **Rückgabe:**
  `ArrVal`
  Array von `NumVal`-Einträgen (Werte 0–255).

---

## file.WriteBytes(path, byteArray)
- **Konkret:**
  Schreibt ein Byte-Array in eine Datei.
  Das Zielverzeichnis muss vorhanden sein.
- **Parameter:**
  - `path`: Zieldatei.
  - `byteArray`: `ArrVal` mit `NumVal`-Einträgen (0–255).
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.

---

## file.Replace(path, pattern, newContent)
- **Konkret:**
  Sucht zeilenweise nach dem ersten Vorkommen von `pattern` und ersetzt die gesamte Zeile durch `newContent`.
  Schreibt nur bei tatsächlichem Fund.
- **Parameter:**
  - `path`: Zieldatei.
  - `pattern`: Suchmuster (Substring).
  - `newContent`: Ersetzungszeile.
- **Rückgabe:**
  `BoolVal`
  `true` = Zeile gefunden und ersetzt, `false` = nicht gefunden (Datei bleibt unverändert). `ErrorVal` bei Lese-/Schreibfehler.

---

## file.ReplaceAll(path, old, new)
- **Konkret:**
  Ersetzt alle Vorkommen von `old` durch `new` im gesamten Dateiinhalt (nicht zeilenweise).
- **Parameter:**
  - `path`: Zieldatei.
  - `old`: Zu ersetzender Text.
  - `new`: Ersatztext.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg, auch wenn nichts ersetzt wurde), `ErrorVal` bei Lese-/Schreibfehler.

---

## file.InsertAfter(pfad, ausgabepfad, suchtext, einfügen)
- **Konkret:**
  Fügt in jeder Zeile direkt nach der ersten Fundstelle des Suchtextes den angegebenen Text ein.
  Nur das erste Vorkommen pro Zeile wird berücksichtigt, weitere Treffer in derselben Zeile bleiben unverändert.
  Ist der Ausgabepfad leer (`""`), wird die Originaldatei überschrieben, andernfalls in den Ausgabepfad geschrieben (Zielverzeichnis muss vorhanden sein).
- **Parameter:**
  - `pfad`: Quelldatei.
  - `ausgabepfad`: Zieldatei, oder `""` um die Quelldatei zu überschreiben.
  - `suchtext`: Suchmuster (Substring). Darf nicht leer sein.
  - `einfügen`: Text, der direkt nach dem Suchtext eingefügt wird.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei leerem Suchtext, fehlenden Parametern oder Lese-/Schreibfehler.
- **Hinweis:**
  Zeilenumbrüche werden beim Verarbeiten normalisiert: die Ausgabedatei hat immer `\n`-Zeilenenden (auch wenn das Original `\r\n` verwendet hat) und keine abschließende Leerzeile am Dateiende, selbst wenn das Original eine hatte.

---

## file.Search(path, pattern)
- **Konkret:**
  Durchsucht eine Datei zeilenweise nach einem Muster.
  Suche ist case-insensitiv.
  Unterstützt Zeilen bis 1 MB Länge (z. B. große Log-Dateien).
  Nutzt Shared-Access (Windows-kompatibel).
- **Parameter:**
  - `path`: Quelldatei.
  - `pattern`: Suchmuster.
- **Rückgabe:**
  `ArrVal`
  Array aller Zeilen, die das Muster enthalten.

---

## file.SearchDateRange(path, start, end, [layout], [max])
- **Konkret:**
  Liest Zeilen aus einer Datei, deren Zeitstempel (am Zeilenanfang) in einem angegebenen Zeitraum liegen.
  Folgezeilen ohne eigenen Zeitstempel werden dem letzten erkannten Datum zugeordnet.
- **Parameter:**
  - `path`: Quelldatei.
  - `start`: Startdatum als String.
  - `end`: Enddatum als String.
  - `layout`: Datumsformat (Standard: `"YYYY-MM-DD HH:mm:SS"`).
  - `max`: Maximale Trefferanzahl (Standard: 5000).
- **Rückgabe:**
  `ArrVal`
  Array aller Zeilen im Zeitraum.

---

## file.ReadValue(path, prefix)
- **Konkret:**
  Sucht die erste Zeile, die mit `prefix` beginnt, und gibt den Wert dahinter zurück (getrimmt).
  Geeignet für einfache Key-Value-Konfigurationsdateien.
- **Parameter:**
  - `path`: Quelldatei.
  - `prefix`: Präfix (z. B. `"Port="`).
- **Rückgabe:**
  `StrVal` – Wert nach dem Präfix, oder leerer String wenn nicht gefunden. `ErrorVal` bei echtem Lesefehler (z. B. Datei fehlt).
- **Fix:** Der Pfad läuft jetzt über `absPathVal` (vorher ungeprüft) – gleiche Absicherung wie bei den übrigen `file.*`-Funktionen. Außerdem war "nicht gefunden" vorher nicht von "Datei nicht lesbar" unterscheidbar (beides `""`); jetzt liefert ein Lesefehler `ErrorVal`.

---

## file.UpdateValue(path, search, newValue)
- **Konkret:**
  Sucht die erste Zeile, die mit `search` beginnt (Prefix-Match, getrimmt), und ersetzt sie vollständig durch `newValue`.
  Erhält den originalen Zeilenumbruch-Stil (`\r\n` oder `\n`).
- **Parameter:**
  - `path`: Zieldatei.
  - `search`: Zeilenpräfix zum Auffinden der Zeile.
  - `newValue`: Neue vollständige Zeile.
- **Rückgabe:**
  `BoolVal`
  `true` = Zeile gefunden und ersetzt, `false` = nicht gefunden. `ErrorVal` bei Lese-/Schreibfehler.
- **Fix:** Der Pfad läuft jetzt über `absPathVal` (vorher ungeprüft).

---

## file.UniqueLines(path, caseSensitive)
- **Konkret:**
  Entfernt doppelte Zeilen aus einer Datei und schreibt das Ergebnis zurück.
  Leere Zeilen werden ignoriert.
- **Parameter:**
  - `path`: Zieldatei.
  - `caseSensitive`: `BoolVal` – ob Groß-/Kleinschreibung berücksichtigt wird.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.

---

## file.GetDuplicates(path, caseSensitive)
- **Konkret:**
  Findet alle Zeilen, die mehr als einmal in der Datei vorkommen.
  Leere Zeilen werden ignoriert.
- **Parameter:**
  - `path`: Quelldatei.
  - `caseSensitive`: `BoolVal` – ob Groß-/Kleinschreibung berücksichtigt wird.
- **Rückgabe:**
  `ArrVal`
  Array der duplizierten Zeilen (als `StrVal`), **alphabetisch sortiert**.

---

## file.Tail(path, [lines], [refresh], [silent])
- **Konkret:**
  Gibt die letzten `n` Zeilen einer Datei zurück.
  Bei Angabe eines Refresh-Intervalls wechselt die Funktion in einen Live-Modus.
  Ohne `silent` (Standard): blockierend, gibt neue Zeilen kontinuierlich auf stdout aus (Shell-Nutzung).
  Mit `silent = True`: kein Print. Wartet einmalig die `refresh`-Dauer ab und gibt zurück, ob sich die Datei in dieser Zeit geändert hat (gewachsen oder rotiert/gekürzt) – gedacht zum Aufruf in einer eigenen Skript-Schleife.
- **Parameter:**
  - `path`: Quelldatei.
  - `lines`: Anzahl der Zeilen (Standard: 10, Max: 5000).
  - `refresh`: Aktualisierungsintervall (z. B. `"1s"`, `"500ms"` oder Zahl in ms). Ohne diesen Parameter kein Live-Modus.
  - `silent`: Optional. `BoolVal`. Bei `true`: kein Print, einmalige Prüfung statt Endlosschleife (siehe Rückgabe).
- **Rückgabe:**
  - Ohne `refresh`: `StrVal` (die letzten `n` Zeilen).
  - Mit `refresh`, ohne `silent`: `NullVal` (blockierend, kehrt nur zurück, wenn die Datei währenddessen verschwindet).
  - Mit `refresh` und `silent = True`: `BoolVal` – `true`, wenn sich die Datei innerhalb des einen `refresh`-Intervalls geändert hat, sonst `false`.

---

## file.Watch(path, [timeoutMs])
- **Konkret:**
  Wartet, bis sich die Datei ändert (Änderungsdatum oder Größe). Pollt in 100-ms-Intervallen. Kein Print – schon von Anfang an skriptfähig.
- **Parameter:**
  - `path`: Zu überwachende Datei.
  - `timeoutMs`: Optional. Maximale Wartezeit in Millisekunden. Ohne Angabe: unbegrenzt.
- **Rückgabe:**
  `BoolVal`
  `true` bei Änderung, `false` bei Timeout.

---

## file.WatchLog(path, pattern, [style], [silent], [timeoutMs])
- **Konkret:**
  Überwacht eine Logdatei live und hebt Zeilen, die `pattern` enthalten, farbig hervor.
  Ohne `silent` (Standard): blockierend, druckt Statuszeilen und Treffer direkt auf stdout (Shell-Nutzung).
  Mit `silent = True`: kein Print. Wartet auf den ersten Treffer des Musters (oder bis `timeoutMs` abgelaufen ist) und gibt dann zurück, ob ein Treffer gefunden wurde – gedacht zum Aufruf in einer eigenen Skript-Schleife.
- **Parameter:**
  - `path`: Zu überwachende Logdatei.
  - `pattern`: Suchmuster (Substring, case-insensitive).
  - `style`: Optional. ANSI-Escape-Code (`StrVal`) oder numerischer Stilcode (`NumVal`). Wirkt nur ohne `silent`.
  - `silent`: Optional. `BoolVal`. Bei `true`: kein Print, wartet auf Treffer statt fortlaufend auszugeben.
  - `timeoutMs`: Optional. Nur relevant bei `silent = True`. Maximale Wartezeit in Millisekunden auf den ersten Treffer. Ohne Angabe (oder `0`): unbegrenzt.
- **Rückgabe:**
  - Ohne `silent`: `NullVal` (blockierend, läuft dauerhaft).
  - Mit `silent = True`: `BoolVal` – `true` bei Treffer, `false` bei Timeout.
- **Hinweis:**
  Anders als bei `file.Tail` (das im `silent`-Modus nur EIN Poll-Intervall abwartet) wartet `file.WatchLog` im `silent`-Modus fortlaufend, bis wirklich ein Treffer für `pattern` auftaucht oder `timeoutMs` abläuft – weil ein Muster nicht bei jedem Poll-Zyklus zwangsläufig erscheint.

---

## file.SecureDelete(path)
- **Konkret:**
  Löscht eine Datei sicher durch In-Place-Verschlüsselung (AES), zufälliges Umbenennen und anschließendes Truncate.
- **Parameter:**
  - `path`: Zu löschende Datei.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.

---

## file.Base64Encode(inFile, outFile)
- **Konkret:**
  Liest eine Datei ein und speichert sie Base64-kodiert in einer neuen Datei.
  Das Zielverzeichnis muss vorhanden sein.
- **Parameter:**
  - `inFile`: Quelldatei.
  - `outFile`: Zieldatei.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.
- **Fix:** Prüft jetzt wie `file.Base64Decode`, ob das Zielverzeichnis existiert, bevor geschrieben wird (vorher fehlte dieser Check, was zu einem rohen OS-Fehler statt einer sprechenden Meldung führte). Fehler beim Kodieren und beim Abschließen des Encoders werden jetzt als `ErrorVal` gemeldet statt verschluckt.

---

## file.Base64Decode(inFile, outFile)
- **Konkret:**
  Dekodiert eine Base64-kodierte Datei und speichert das Ergebnis als Binärdatei.
  Das Zielverzeichnis muss vorhanden sein.
- **Parameter:**
  - `inFile`: Base64-Quelldatei.
  - `outFile`: Zieldatei.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.

---

## file.CreateSymlink(target, linkPath, [replaceExisting])
- **Konkret:**
  Erstellt einen symbolischen Link auf eine Datei.
- **Parameter:**
  - `target`: Ziel des Symlinks.
  - `linkPath`: Pfad des zu erstellenden Links.
  - `replaceExisting`: Optional, `BoolVal` (Standard `false`). Steuert, ob ein bereits vorhandener Link/Datei an `linkPath` ersetzt wird.
- **Rückgabe:**
  `BoolVal` (`true` bei Erfolg), `ErrorVal` bei Fehler.
- **Fix:** Der dritte Parameter wurde bisher entgegengenommen, aber ignoriert (`createSymlinkInternal` erhielt intern immer `false`). Er wird jetzt tatsächlich ausgewertet und durchgereicht.

---

## file.Ext(path)
- **Konkret:**
  Gibt die Dateiendung zurück (ohne führenden Punkt).
- **Parameter:**
  - `path`: Dateipfad.
- **Rückgabe:**
  `StrVal`
  Beispiel: `"txt"`, `"go"`, `""` bei keiner Endung.

---

## file.Name(path)
- **Konkret:**
  Gibt den Dateinamen ohne Verzeichnispfad und ohne Endung zurück.
- **Parameter:**
  - `path`: Dateipfad.
- **Rückgabe:**
  `StrVal`
  Beispiel: `"readme"` für `/home/user/readme.md`.

---

## file.Dir(path)
- **Konkret:**
  Gibt das übergeordnete Verzeichnis eines Pfades zurück.
- **Parameter:**
  - `path`: Dateipfad.
- **Rückgabe:**
  `StrVal`
  Beispiel: `"/home/user"` für `"/home/user/readme.md"`.

---

## file.Join(path, part, ...)
- **Konkret:**
  Verbindet mehrere Pfadsegmente plattformsicher zu einem vollständigen Pfad.
- **Parameter:**
  - Beliebig viele Segmente als `StrVal`.
- **Rückgabe:**
  `StrVal`