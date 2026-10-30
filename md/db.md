# 🗄️ db.* – Datenbankfunktionen

Dient zur Verbindung, Abfrage und Verwaltung von Datenbanken.
Unterstützte Treiber: `sqlite` / `sqlite3`, `postgres` / `pg`, `mssql` / `sqlserver`.
Verbindungen werden über einen Alias verwaltet – `db.Open` muss zuerst aufgerufen werden.

---

## db.Open(driver, connStr, alias)
- **Konkret:**
  Öffnet eine Datenbankverbindung und speichert sie unter einem Alias.
  Führt nach dem Verbinden automatisch einen Ping durch.
  SQLite: relative Pfade werden relativ zum aktuellen Arbeitsverzeichnis aufgelöst.
- **Parameter:**
  - `driver`: Treiber. Unterstützte Werte: `"sqlite"`, `"sqlite3"`, `"postgres"`, `"pg"`, `"mssql"`, `"sqlserver"`.
  - `connStr`: Connection-String oder Dateipfad (SQLite).
  - `alias`: Frei wählbarer Name für diese Verbindung.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler.

---

## db.Close(alias)
- **Konkret:**
  Schließt eine Datenbankverbindung und entfernt den Alias.
- **Parameter:**
  - `alias`: Verbindungsalias.
- **Rückgabe:**
  `BoolVal` (`true`) wenn gefunden und geschlossen, `false` wenn Alias unbekannt.

---

## db.DriverName(alias)
- **Konkret:**
  Gibt den Treibernamen der Verbindung zurück.
- **Parameter:**
  - `alias`: Verbindungsalias.
- **Rückgabe:**
  `StrVal` (z. B. `"mssql"`, `"sqlite"`, `"postgres"`). `"unknown"` bei unbekanntem Alias.

---

## db.Query(alias, sql)
- **Konkret:**
  Führt ein SELECT aus und gibt alle Ergebniszeilen zurück.
  Binärdaten (Blobs) werden automatisch als Base64-String kodiert.
  Steuerzeichen (außer Tab, CR, LF) werden aus Strings entfernt.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `sql`: SELECT-Statement.
- **Rückgabe:**
  `ArrVal`
  Array von Zeilen. Jede Zeile ist ein `ArrVal` mit den Spaltenwerten.

---

## db.QueryParams(alias, sql, params)
- **Konkret:**
  Führt ein SELECT mit echter Parameter-Bindung aus und gibt das Ergebnis identisch zu `db.Query` zurück (gleiche Steuerzeichen-Bereinigung, gleiche Blob-Erkennung/Base64-Kodierung).
  `?` im SQL-Text wird analog zu `db.ExecParams` durch dialektspezifische Platzhalter ersetzt.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `sql`: SELECT-Statement mit `?` als Platzhalter.
  - `params`: `ArrVal` mit den Werten, in der Reihenfolge der `?`-Platzhalter im SQL.
- **Rückgabe:**
  `ArrVal` (identisches Format zu `db.Query`: Array von Zeilen-Arrays). `ErrorVal` bei falscher Platzhalteranzahl, SQL-Fehler oder unbekanntem Alias.
- **Beispiel:**
```vbx
  Dim rows = db.QueryParams("main", "SELECT pfad, hash FROM dateien WHERE typ = ?", {typ})
  For Each row In rows
      Print row(0) & " -> " & row(1)
  Next
```

---

## db.QueryArray(alias, sql)
- **Konkret:**
  Führt ein SELECT aus und gibt das Ergebnis als 2D-Array zurück.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `sql`: SELECT-Statement.
- **Rückgabe:**
  `KindArr2D`

---

## db.Exec(alias, sqlString)
- **Konkret:**
  Führt ein oder mehrere SQL-Statements aus.
  MSSQL: trennt am `GO`-Keyword. Andere Treiber: trennen am Semikolon.
  Kapselt automatisch in eine Transaktion, wenn kein `COMMIT`/`ROLLBACK` im SQL vorhanden ist.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `sqlString`: SQL-Statement oder mehrteiliges Script.
- **Rückgabe:**
  `ArrVal`
  Array mit den Ergebnissen je Batch.

---

## db.ExecParams(alias, sql, params)
- **Konkret:**
  Führt ein einzelnes SQL-Statement mit echter Parameter-Bindung aus. Kein Batch-Splitting (anders als `db.Exec`), kein manuelles String-Escaping nötig.
  `?` im SQL-Text wird der Reihe nach durch die dialektspezifischen Platzhalter ersetzt (SQLite: `?`, Postgres: `$1,$2,...`, MSSQL: `@p1,@p2,...`) und die zugehörigen Werte werden gebunden, nicht in den SQL-Text eingesetzt.
  Empfohlen für alle Fälle, in denen Werte aus unsicheren Quellen (z. B. Dateinamen, Benutzereingaben) in ein SQL-Statement eingebettet werden müssen – verhindert SQL-Injection zuverlässiger als manuelles Escaping von Anführungszeichen, da auch Zeichen wie `;` oder `--` innerhalb eines gebundenen Werts niemals als SQL-Syntax interpretiert werden.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `sql`: SQL-Statement mit `?` als Platzhalter für jeden zu bindenden Wert.
  - `params`: `ArrVal` mit den Werten, in der Reihenfolge der `?`-Platzhalter im SQL.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei falscher Platzhalteranzahl, SQL-Fehler oder unbekanntem Alias.
- **Beispiel:**
```vbx
  db.ExecParams("main", "DELETE FROM dateien WHERE pfad = ?", {pfad})
  db.ExecParams("main", "UPDATE dateien SET hash = ? WHERE id = ?", {neuerHash, id})
```

---

## db.ExecFile(alias, path [, dryRun])
- **Konkret:**
  Liest eine `.sql`-Datei und führt sie aus.
  Splitting und Transaktionshandling identisch zu `db.Exec`.
  Mit `dryRun = true` wird am Ende ein Rollback statt Commit ausgeführt.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `path`: Pfad zur SQL-Datei.
  - `dryRun`: Optional. `BoolVal` – bei `true` werden Änderungen nicht übernommen.
- **Rückgabe:**
  `ArrVal` mit Batch-Ergebnissen, `ErrorVal` bei Fehler.

---

## db.Insert(alias, table, columns, values)
- **Konkret:**
  Fügt einen einzelnen Datensatz über Spalten- und Werte-Arrays ein.
  Dialektspezifisches Quoting und Platzhalter werden automatisch gesetzt (MSSQL: `[..]`/`@p1,@p2`; Postgres: `"..."`/`$1,$2`; sonst `"..."`/`?`).
  `columns` und `values` müssen gleich lang sein.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `table`: Zieltabelle.
  - `columns`: Array der Spaltennamen.
  - `values`: Array der Werte, gleiche Reihenfolge wie `columns`.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei fehlenden/unpassenden Argumenten oder SQL-Fehler.
- **Beispiel:**
  ```vb
  db.Insert("main", "Kunden", {"Name", "Alter"}, {"Max", "30"})
  ```

---

## db.CopyTable(sourceAlias, targetAlias, sourceTable, targetTable)
- **Konkret:**
  Kopiert alle Zeilen einer Tabelle von einer Verbindung in eine andere, auch zwischen unterschiedlichen Treibern (z. B. SQLite → Postgres).
  Liest per `SELECT *` und schreibt zeilenweise per vorbereitetem INSERT innerhalb einer Transaktion auf der Zielverbindung.
  Platzhalter werden dialektspezifisch anhand der Zielverbindung gewählt (SQLite: `?`, Postgres: `$1,$2,...`, MSSQL: `@p1,@p2,...`). Werte werden zusätzlich pro Spalte zwischen Quell- und Zieldialekt konvertiert.
  Die Zieltabelle muss vor dem Aufruf bereits existieren (`CopyTable` legt sie nicht selbst an) und in Spaltenanzahl/-namen zur Quelltabelle passen.
- **Parameter:**
  - `sourceAlias`: Alias der Quellverbindung.
  - `targetAlias`: Alias der Zielverbindung.
  - `sourceTable`: Quelltabelle.
  - `targetTable`: Zieltabelle.
- **Rückgabe:**
  `NumVal` (Anzahl kopierter Zeilen), `ErrorVal` bei fehlenden Argumenten, unbekanntem Alias oder Fehler.
- **Hinweis zu Auto-Increment-Spalten:**
  `SELECT *` liest auch Primärschlüssel-/ID-Spalten mit und schreibt deren Werte explizit in die Zieltabelle. Bei einer Ziel-ID vom Typ `SERIAL` (Postgres) oder `AUTOINCREMENT` (SQLite) wird dabei der interne Sequenz-Zähler der Zieldatenbank **nicht automatisch aktualisiert**. Nach dem Kopieren empfiehlt sich (Postgres-Beispiel):
```vbx
  db.Exec(targetAlias, "SELECT setval(pg_get_serial_sequence('tabelle', 'id'), (SELECT MAX(id) FROM tabelle))")
```
  um Konflikte bei nachfolgenden `db.Insert`-Aufrufen zu vermeiden.

---

## db.SyncTable(sourceAlias, targetAlias, table, idColumn [, batchSize])
- **Konkret:**
  Synchronisiert eine einzelne Tabelle von der Quell- zur Zieldatenbank.
  Tabellen- und Spaltennamen werden case-insensitive aufgelöst (z. B. funktioniert die Synchronisation zwischen einer MSSQL-Tabelle `Hoerspiel` und einer Postgres-Tabelle `hoerspiel` ohne manuelle Anpassung).
  Jede Quellzeile wird mit der entsprechenden Zielzeile (per ID) verglichen. Nur bei tatsächlicher inhaltlicher Abweichung wird ein `UPDATE` ausgeführt; identische Zeilen werden übersprungen und zählen als `unchanged`. Zeilen, deren ID im Ziel nicht existiert, werden per `INSERT` angelegt. Zeilen, deren ID im Ziel existiert, aber nicht mehr in der aktuellen Quellmenge vorkommt, werden per `DELETE` entfernt.
  Die Verarbeitung läuft in Batches: alle `batchSize` Zeilen wird ein Commit ausgeführt, statt die komplette Synchronisation in einer einzigen langen Transaktion zu halten (vermeidet unnötiges WAL-Wachstum und lang gehaltene Locks bei großen Tabellen).
- **Parameter:**
  - `sourceAlias`: Alias der Quellverbindung (siehe `db.Open`).
  - `targetAlias`: Alias der Zielverbindung (siehe `db.Open`).
  - `table`: Tabellenname. Muss in Quelle und Ziel existieren (Groß-/Kleinschreibung egal).
  - `idColumn`: Name der ID-/Primärschlüssel-Spalte. Muss ebenfalls in beiden Tabellen existieren (Groß-/Kleinschreibung egal).
  - `batchSize`: Optional. Anzahl Zeilen pro Commit-Batch. Standard: `500`.
- **Rückgabe:**
  `ArrVal` `[Insert, Update, Delete, Unchanged]` (vier Zahlen)
  `ErrorVal` bei fehlenden Argumenten, ungültigem Tabellen-/Spaltennamen, nicht gefundener Verbindung/Tabelle/Spalte, oder DB-Fehlern während Lesen/Schreiben.

```vb
result = db.SyncTable("src", "dst", "Hoerspiel", "ID")
Print "Neu: " & result(0) & " Geändert: " & result(1) & " Gelöscht: " & result(2) & " Unverändert: " & result(3)

' mit expliziter Batch-Größe
db.SyncTable("src", "dst", "Hoerspiel", "ID", 200)
```

---

## db.SyncTables(sourceAlias, targetAlias, table1, table2, ...)
- **Konkret:**
  Synchronisiert mehrere Tabellen nacheinander in einem Aufruf. Nutzt intern dieselbe Logik wie `db.SyncTable` (Change-Detection, Batch-Commits mit Default-`batchSize` 500).
  Achtung: Es wird angenommen, dass die ID-Spalte **denselben Namen wie die Tabelle** trägt (`idColumn = table`). Tabellen mit abweichender ID-Spalte (z. B. eine Tabelle `Sprecher` mit ID-Spalte `id` statt `Sprecher`) müssen stattdessen einzeln über `db.SyncTable(...)` mit explizitem `idColumn` synchronisiert werden.
  Ein individueller `batchSize` pro Tabelle ist über `SyncTables` nicht möglich (siehe `db.SyncTable`, falls das benötigt wird).
- **Parameter:**
  - `sourceAlias`: Alias der Quellverbindung.
  - `targetAlias`: Alias der Zielverbindung.
  - `table1, table2, ...`: Beliebig viele Tabellennamen (mindestens einer).
- **Rückgabe:**
  `ArrVal` verschachtelt: pro Tabelle ein Eintrag `[Tabellenname, Insert, Update, Delete, Unchanged]`, abschließend ein Summen-Eintrag `["Gesamt", Insert, Update, Delete, Unchanged]`.
  `ErrorVal` bei fehlenden Argumenten oder falls eine der Tabellen fehlschlägt (bricht den gesamten Aufruf ab, bereits synchronisierte Tabellen bleiben aber committet).

```vb
result = db.SyncTables("src", "dst", "ID_Hoerspiel", "Rolle")

For Each row In result
    Print row(0) & ": Insert=" & row(1) & " Update=" & row(2) & " Delete=" & row(3) & " Unverändert=" & row(4)
Next
```

---

## db.ClearTable(alias, table)
- **Konkret:**
  Löscht alle Datensätze einer Tabelle.
  MSSQL und SQLite: `DELETE FROM table`.
  Postgres: `TRUNCATE TABLE table RESTART IDENTITY CASCADE` (setzt Sequenzen zurück, löscht kaskadierend abhängige Zeilen).
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `table`: Zu leerende Tabelle.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei unbekanntem Alias, nicht unterstütztem Treiber oder SQL-Fehler.

---

## db.Count(alias, table [, filter, val])
- **Konkret:**
  Zählt Zeilen in einer Tabelle.
  Mit Filter: zählt nur Zeilen die die Bedingung erfüllen.
  MSSQL: `?`-Platzhalter werden automatisch in `@p1` umgewandelt.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `table`: Tabellenname.
  - `filter`: Optional. WHERE-Bedingung (z. B. `"Status = ?"`).
  - `val`: Optional. Wert für den Platzhalter.
- **Rückgabe:**
  `NumVal`

---

## db.GetLastID(alias [, sql])
- **Konkret:**
  Gibt die zuletzt erzeugte Auto-Increment-ID zurück.
  Ohne SQL: automatisch je nach Treiber (`SCOPE_IDENTITY()` / `last_insert_rowid()`).
  Postgres: erfordert eigenes SQL mit `RETURNING id`.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `sql`: Optional. Eigenes SQL zum Abrufen der ID.
- **Rückgabe:**
  `NumVal`, `ErrorVal` bei Postgres ohne SQL.

---

## db.TableExists(alias, tableName)
- **Konkret:**
  Prüft ob eine Tabelle in der Datenbank existiert.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `tableName`: Tabellenname.
- **Rückgabe:**
  `BoolVal`

---

## db.ListTables(alias)
- **Konkret:**
  Gibt alle Tabellennamen der Datenbank zurück.
- **Parameter:**
  - `alias`: Verbindungsalias.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen.

---

## db.ListColumns(alias, table)
- **Konkret:**
  Gibt alle Spaltennamen einer Tabelle zurück.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `table`: Tabellenname.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen.

---

## db.ListViews(alias)
- **Konkret:**
  Gibt alle Views der Datenbank zurück.
  Format: `Schema.Name` (z. B. `"dbo.MeineView"`).
- **Parameter:**
  - `alias`: Verbindungsalias.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen.

---

## db.ListProcedures(alias)
- **Konkret:**
  Gibt alle Stored Procedures zurück (MSSQL und Postgres).
  SQLite wird nicht unterstützt (leeres Array).
  Format: `Schema.Name`.
- **Parameter:**
  - `alias`: Verbindungsalias.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen.

---

## db.ListDatabases(alias)
- **Konkret:**
  Listet alle Datenbanken auf dem Server auf.
  MSSQL: ohne Systemdatenbanken (`master`, `tempdb`, `model`, `msdb`).
  Postgres: ohne Templates und `postgres`.
  SQLite: gibt immer `["main"]` zurück.
- **Parameter:**
  - `alias`: Verbindungsalias.
- **Rückgabe:**
  `ArrVal`

---

## db.DatabaseName(alias)
- **Konkret:**
  Gibt den Namen der aktuell verbundenen Datenbank zurück.
- **Parameter:**
  - `alias`: Verbindungsalias.
- **Rückgabe:**
  `StrVal`

---

## db.SetBlob(alias, table, col, whereCol, whereVal, b64Data)
- **Konkret:**
  Speichert Binärdaten (als Base64-String) in einer Tabellenspalte via UPDATE.
  Dialektspezifische Platzhalter werden automatisch gesetzt.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `table`: Tabellenname.
  - `col`: Zielspalte.
  - `whereCol`: WHERE-Spalte.
  - `whereVal`: WHERE-Wert.
  - `b64Data`: Binärdaten als Base64-String.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler.

---

## db.GetBlob(alias, table, col, whereCol, whereVal)
- **Konkret:**
  Liest Binärdaten aus einer Tabellenspalte und gibt sie als Base64-String zurück.
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `table`: Tabellenname.
  - `col`: Quellspalte.
  - `whereCol`: WHERE-Spalte.
  - `whereVal`: WHERE-Wert.
- **Rückgabe:**
  `StrVal` (Base64-kodiert). Leerer String wenn kein Datensatz gefunden.

---

## db.Backup(alias, dbName, targetPath)
- **Konkret:**
  Erstellt ein Datenbank-Backup via SQL-Befehl.
  MSSQL: `BACKUP DATABASE ... TO DISK`.
  SQLite: `VACUUM INTO`.
  Postgres: nicht unterstützt (für Postgres `db.Export` verwenden).
- **Parameter:**
  - `alias`: Verbindungsalias.
  - `dbName`: Name der Datenbank.
  - `targetPath`: Zieldatei.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler.

---

## db.Export(driver, connStr, dbName, targetPath)
- **Konkret:**
  Exportiert eine Datenbank via externe System-Tools.
  Postgres: nutzt `pg_dump`. MSSQL: nutzt `sqlcmd`.
  Die Tools müssen im PATH verfügbar sein.
- **Parameter:**
  - `driver`: Treiber (`"postgres"` oder `"mssql"`).
  - `connStr`: Connection-String.
  - `dbName`: Name der Datenbank.
  - `targetPath`: Zieldatei.
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler oder fehlendem Tool.

---

## db.Optimize(sql, dialect)
- **Konkret:**
  Bereinigt und optimiert einen SQL-String nach Dialekt-spezifischen Regeln.
- **Parameter:**
  - `sql`: SQL-Statement.
  - `dialect`: Dialekt (`"mssql"`, `"sqlite"`, `"postgres"`).
- **Rückgabe:**
  `StrVal` (optimierter SQL-String).

---

## db.OptimizeFile(path, dialect [, outPath])
- **Konkret:**
  Liest eine `.sql`-Datei, optimiert sie und schreibt das Ergebnis zurück.
  Ohne `outPath` wird die Quelldatei überschrieben.
  Nur `.sql`-Dateien werden akzeptiert.
- **Parameter:**
  - `path`: Pfad zur SQL-Datei.
  - `dialect`: Dialekt.
  - `outPath`: Optional. Zielpfad (muss ebenfalls `.sql` enden).
- **Rückgabe:**
  `StrVal` (Bestätigungsmeldung), `ErrorVal` bei Fehler.