package stdlib

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	valpkg "vbxc/value"

	_ "github.com/denisenkom/go-mssqldb" // MSSQL
	_ "github.com/lib/pq"                // Postgres
	_ "modernc.org/sqlite"               // SQLCipher (ersetzt das normale sqlite3)
)

func initDBFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "db."

	Register(ns+"Open", "db", "driver, connStr, alias", "Öffnet eine DB-Verbindung (MSSQL, SQLite, PG).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("driver, connStr/path und alias benötigt")
		}

		driver := strings.ToLower(args[0].Str)
		connStr := args[1].Str
		alias := strings.ToLower(args[2].Str)

		var db *sql.DB
		var err error

		switch driver {
		case "sqlite", "sqlite3":
			if !filepath.IsAbs(connStr) {
				cwd, _ := os.Getwd()
				connStr = filepath.Join(cwd, connStr)
			}
			db, err = sql.Open("sqlite", connStr)

		case "postgres", "pg", "mssql", "sqlserver":
			// Für MSSQL/PG nutzen wir direkt den Driver-Namen und den Connection-String
			driverName := driver
			if driver == "pg" {
				driverName = "postgres"
			}
			if driver == "mssql" {
				driverName = "sqlserver"
			}

			db, err = sql.Open(driverName, connStr)

		default:
			return valpkg.ErrorVal("Unsupported driver: " + driver)
		}

		if err != nil {
			return valpkg.ErrorVal(driver + " Open Error: " + err.Error())
		}

		if err = db.Ping(); err != nil {
			db.Close()
			return valpkg.ErrorVal(driver + " Ping failed: " + err.Error())
		}

		// In die globalen Maps speichern
		connections[alias] = db
		drivers[alias] = driver

		return valpkg.BoolVal(true) // WICHTIG für deine VB-Abfrage!
	})

	Register(ns+"Insert", "db", "alias, table, columns, values", "Fügt einen Datensatz in eine Tabelle ein.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 4 {
			return valpkg.ErrorVal("alias, table, columns und values benötigt")
		}

		alias := strings.ToLower(args[0].Str)
		table := args[1].Str

		if !isSafeDBName(table) {
			return valpkg.ErrorVal("Ungültiger Tabellenname: " + table)
		}

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		columns := args[2].Arr
		values := args[3].Arr

		if len(columns) == 0 || len(columns) != len(values) {
			return valpkg.ErrorVal("columns und values müssen gleiche Länge haben")
		}

		var colNames []string
		var placeholders []string
		var params []any

		for i, col := range columns {
			name := col.Str

			if !isSafeDBName(name) {
				return valpkg.ErrorVal("Ungültiger Spaltenname: " + name)
			}

			colNames = append(colNames, quoteIdent(driver, name))

			switch driver {
			case "mssql", "sqlserver":
				placeholders = append(placeholders, fmt.Sprintf("@p%d", i+1))
			case "postgres", "pg":
				placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
			default:
				placeholders = append(placeholders, "?")
			}

			params = append(params, valueToSQL(values[i]))
		}

		sqlText := fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)",
			quoteIdent(driver, table),
			strings.Join(colNames, ","),
			strings.Join(placeholders, ","),
		)

		_, err = db.Exec(sqlText, params...)
		if err != nil {
			return valpkg.ErrorVal("Insert failed: " + err.Error())
		}

		return valpkg.BoolVal(true)
	})

	Register(ns+"CopyTable", "db", "sourceAlias, targetAlias, sourceTable, targetTable", "Kopiert Daten zwischen zwei geöffneten Datenbankverbindungen.", func(args []valpkg.Value) valpkg.Value {

		if len(args) < 4 {
			return valpkg.ErrorVal("db.CopyTable(sourceAlias, targetAlias, sourceTable, targetTable) benötigt 4 Argumente")
		}

		sourceAlias := strings.ToLower(args[0].Str)
		targetAlias := strings.ToLower(args[1].Str)

		sourceTable := args[2].Str
		targetTable := args[3].Str

		if !isSafeDBName(sourceTable) {
			return valpkg.ErrorVal("Ungültiger Quelltabellenname: " + sourceTable)
		}
		if !isSafeDBName(targetTable) {
			return valpkg.ErrorVal("Ungültiger Zieltabellenname: " + targetTable)
		}

		src, srcDriver, err := getConn(sourceAlias)
		if err != nil {
			return valpkg.ErrorVal("Quellverbindung nicht gefunden: " + sourceAlias)
		}

		dst, dstDriver, err := getConn(targetAlias)
		if err != nil {
			return valpkg.ErrorVal("Zielverbindung nicht gefunden: " + targetAlias)
		}

		rows, err := src.Query("SELECT * FROM " + quoteIdent(srcDriver, sourceTable))
		if err != nil {
			return valpkg.ErrorVal("Lesefehler " + sourceTable + ": " + err.Error())
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return valpkg.ErrorVal("Spalten konnten nicht gelesen werden: " + err.Error())
		}

		columnTypes, err := rows.ColumnTypes()
		if err != nil {
			return valpkg.ErrorVal("Spaltentypen konnten nicht gelesen werden: " + err.Error())
		}

		colNames := make([]string, len(columns))
		placeholders := make([]string, len(columns))

		for i, col := range columns {

			colNames[i] = quoteIdent(dstDriver, col)

			switch dstDriver {
			case "mssql", "sqlserver":
				placeholders[i] = fmt.Sprintf("@p%d", i+1)

			case "postgres", "pg":
				placeholders[i] = fmt.Sprintf("$%d", i+1)

			default:
				placeholders[i] = "?"
			}
		}

		sqlText := fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)",
			quoteIdent(dstDriver, targetTable),
			strings.Join(colNames, ","),
			strings.Join(placeholders, ","),
		)

		tx, err := dst.Begin()
		if err != nil {
			return valpkg.ErrorVal("Transaktion konnte nicht gestartet werden: " + err.Error())
		}

		stmt, err := tx.Prepare(sqlText)
		if err != nil {
			tx.Rollback()
			return valpkg.ErrorVal("INSERT Vorbereitung fehlgeschlagen: " + err.Error())
		}
		defer stmt.Close()

		count := 0

		for rows.Next() {

			values := make([]any, len(columns))
			scan := make([]any, len(columns))

			for i := range values {
				scan[i] = &values[i]
			}

			if err := rows.Scan(scan...); err != nil {
				tx.Rollback()
				return valpkg.ErrorVal("Lesefehler: " + err.Error())
			}

			params := make([]any, len(values))

			for i := range values {
				params[i] = convertDBValue(
					srcDriver,
					dstDriver,
					columnTypes[i],
					values[i],
				)
			}

			if _, err := stmt.Exec(params...); err != nil {
				tx.Rollback()
				return valpkg.ErrorVal("Insert Fehler: " + err.Error())
			}

			count++
		}

		if err := rows.Err(); err != nil {
			tx.Rollback()
			return valpkg.ErrorVal(err.Error())
		}

		if err := tx.Commit(); err != nil {
			return valpkg.ErrorVal("Commit fehlgeschlagen: " + err.Error())
		}

		return valpkg.NumVal(float64(count))
	})

	Register(ns+"SyncTable", "db",
		"sourceAlias, targetAlias, table, idColumn [, batchSize]",
		"Synchronisiert eine Tabelle. Schreibt nur bei echter inhaltlicher Änderung. batchSize (Default 500) bestimmt, alle wie viele Zeilen committet wird.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 4 {
				return valpkg.ErrorVal("db.SyncTable(sourceAlias,targetAlias,table,idColumn [, batchSize])")
			}

			batchSize := 0 // 0 -> syncTable() nutzt intern den Default (500)
			if len(args) >= 5 {
				batchSize = int(valpkg.ToNum(args[4]))
			}

			result, err := syncTable(
				args[0].Str,
				args[1].Str,
				args[2].Str,
				args[3].Str,
				batchSize,
			)

			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr: []valpkg.Value{
					valpkg.NumVal(float64(result.Insert)),
					valpkg.NumVal(float64(result.Update)),
					valpkg.NumVal(float64(result.Delete)),
					valpkg.NumVal(float64(result.Unchanged)),
				},
			}
		})

	Register(ns+"SyncTables", "db",
		"sourceAlias, targetAlias, table1, table2, ...",
		"Synchronisiert mehrere Tabellen (nutzt intern den Default-batchSize von 500). Schreibt nur bei echter inhaltlicher Änderung.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 3 {
				return valpkg.ErrorVal("db.SyncTables benötigt Tabellen")
			}

			var result []valpkg.Value

			totalInsert := 0
			totalUpdate := 0
			totalDelete := 0
			totalUnchanged := 0

			for i := 2; i < len(args); i++ {

				table := args[i].Str

				r, err := syncTable(
					args[0].Str,
					args[1].Str,
					table,
					table, // ID-Spalte = Tabellenname
					0,     // Default-batchSize (500)
				)

				if err != nil {
					return valpkg.ErrorVal(err.Error())
				}

				result = append(result, valpkg.Value{
					Kind: valpkg.KindArr,
					Arr: []valpkg.Value{
						valpkg.StrVal(r.Table),
						valpkg.NumVal(float64(r.Insert)),
						valpkg.NumVal(float64(r.Update)),
						valpkg.NumVal(float64(r.Delete)),
						valpkg.NumVal(float64(r.Unchanged)),
					},
				})

				totalInsert += r.Insert
				totalUpdate += r.Update
				totalDelete += r.Delete
				totalUnchanged += r.Unchanged
			}

			result = append(result, valpkg.Value{
				Kind: valpkg.KindArr,
				Arr: []valpkg.Value{
					valpkg.StrVal("Gesamt"),
					valpkg.NumVal(float64(totalInsert)),
					valpkg.NumVal(float64(totalUpdate)),
					valpkg.NumVal(float64(totalDelete)),
					valpkg.NumVal(float64(totalUnchanged)),
				},
			})

			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr:  result,
			}
		})

	Register(ns+"ClearTable", "db", "alias, table", "Löscht alle Daten aus einer Tabelle.", func(args []valpkg.Value) valpkg.Value {

		if len(args) < 2 {
			return valpkg.ErrorVal("db.ClearTable(alias, table) benötigt 2 Argumente")
		}

		alias := strings.ToLower(args[0].Str)
		table := args[1].Str

		if !isSafeDBName(table) {
			return valpkg.ErrorVal("Ungültiger Tabellenname: " + table)
		}

		conn, driver, err := getConn(alias)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		var sqlText string

		switch driver {
		case "mssql", "sqlserver":
			sqlText = "DELETE FROM " + quoteIdent(driver, table)
		case "postgres", "pg":
			sqlText = "TRUNCATE TABLE " + quoteIdent(driver, table) + " RESTART IDENTITY CASCADE"
		case "sqlite", "sqlite3":
			sqlText = "DELETE FROM " + quoteIdent(driver, table)
		default:
			return valpkg.ErrorVal("ClearTable nicht unterstützt für: " + driver)
		}

		_, err = conn.Exec(sqlText)
		if err != nil {
			return valpkg.ErrorVal("ClearTable Fehler: " + err.Error())
		}

		return valpkg.BoolVal(true)
	})

	Register(ns+"Backup", "db", "alias, dbName, targetPath", "Internes Server-Backup via SQL.", func(args []valpkg.Value) valpkg.Value {
		alias := strings.ToLower(args[0].Str)
		dbName := args[1].Str
		targetPath := args[2].Str

		db, ok := connections[alias]
		if !ok {
			return valpkg.ErrorVal("Alias nicht gefunden")
		}
		driver := drivers[alias]

		var query string
		switch driver {
		case "mssql", "sqlserver":
			query = fmt.Sprintf("BACKUP DATABASE [%s] TO DISK = '%s' WITH FORMAT", dbName, targetPath)
		case "sqlite", "sqlite3":
			query = fmt.Sprintf("VACUUM INTO '%s'", targetPath)
		default:
			return valpkg.ErrorVal("Backup via SQL für " + driver + " nicht möglich.")
		}

		_, err := db.Exec(query)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		return valpkg.BoolVal(true)
	})

	Register(ns+"Export", "db", "driver, connStr, dbName, targetPath", "Netzwerk-Export via System-Tools.", func(args []valpkg.Value) valpkg.Value {
		driver := strings.ToLower(args[0].Str)
		connStr := args[1].Str
		dbName := args[2].Str

		// Hier nutzen wir deine absPath Logik
		// 'safePath' ist jetzt der bereinigte, absolute Pfad
		safePath, errPtr := dbProcessTargetPath(args[3].Str)
		if errPtr != nil {
			return *errPtr
		}

		switch driver {
		case "postgres", "pg":
			pgPath, err := exec.LookPath("pg_dump")
			if err != nil {
				return valpkg.ErrorVal("pg_dump nicht gefunden")
			}

			// WICHTIG: Nutze hier 'safePath'
			cmd := exec.Command(pgPath, "--dbname="+connStr, "-f", safePath, dbName)

			output, err := cmd.CombinedOutput()
			if err != nil {
				return valpkg.ErrorVal(string(output))
			}

		case "mssql", "sqlserver":
			sqlcmdPath, err := exec.LookPath("sqlcmd")
			if err != nil {
				return valpkg.ErrorVal("sqlcmd nicht gefunden")
			}

			// WICHTIG: Auch hier 'safePath' nutzen
			query := fmt.Sprintf("BACKUP DATABASE [%s] TO DISK = '%s' WITH FORMAT", dbName, safePath)

			cmd := exec.Command(sqlcmdPath, "-S", connStr, "-Q", query)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return valpkg.ErrorVal(string(output))
			}

		default:
			return valpkg.ErrorVal("Unsupported driver: " + driver)
		}

		return valpkg.BoolVal(true)
	})

	Register(ns+"Query", "db", "alias, sql", "Führt ein SELECT aus und gibt ein Array von Zeilen-Arrays zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		alias := args[0].Str
		query := args[1].Str

		// Verbindung über Alias holen
		db, _, err := getConn(alias)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		rows, err := db.Query(query)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		defer rows.Close()

		cols, _ := rows.Columns()
		allRows := make([]valpkg.Value, 0)

		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}

		// Deine bewährte Säuberungs-Logik
		clean := func(s string) string {
			s = strings.TrimSpace(s)
			return strings.Map(func(r rune) rune {
				if r >= 32 || r == 9 || r == 10 || r == 13 {
					return r
				}
				return -1
			}, s)
		}

		for rows.Next() {
			row := make([]valpkg.Value, len(cols))
			if err := rows.Scan(ptrs...); err != nil {
				continue
			}

			for i, v := range vals {
				if v == nil {
					row[i] = valpkg.Value{Kind: valpkg.KindNull}
					continue
				}

				switch val := v.(type) {
				case string:
					row[i] = valpkg.StrVal(clean(val))
				case []byte:
					// Hier greift die neue Binary-Logik für Blobs (MSSQL/SQLite)
					if isBinaryData(val) {
						row[i] = valpkg.StrVal(base64.StdEncoding.EncodeToString(val))
					} else {
						row[i] = valpkg.StrVal(clean(string(val)))
					}
				default:
					row[i] = dbConvertToValue(v)
				}
			}
			// Jede Zeile ist ein valpkg.KindArr
			allRows = append(allRows, valpkg.Value{Kind: valpkg.KindArr, Arr: row})
		}

		// Das Gesamtergebnis ist ein valpkg.KindArr von valpkg.KindArrs
		return valpkg.Value{Kind: valpkg.KindArr, Arr: allRows}
	})

	Register(ns+"QueryParams", "db", "alias, sql, params", "Führt ein parametrisiertes SELECT aus (echte Werte-Bindung, kein String-Escaping nötig).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		alias := args[0].Str
		sqlTemplate := args[1].Str

		if args[2].Kind != valpkg.KindArr {
			return valpkg.ErrorVal("db.QueryParams: dritter Parameter muss ein Array sein")
		}
		params := args[2].Arr

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		// '?' im Template durch dialektspezifische Platzhalter ersetzen
		var finalSQL strings.Builder
		pos := 0
		paramIdx := 0
		for pos < len(sqlTemplate) {
			if sqlTemplate[pos] == '?' {
				if paramIdx >= len(params) {
					return valpkg.ErrorVal("db.QueryParams: zu wenige Werte für die Platzhalter im SQL übergeben")
				}
				finalSQL.WriteString(placeholder(driver, paramIdx+1))
				paramIdx++
				pos++
				continue
			}
			finalSQL.WriteByte(sqlTemplate[pos])
			pos++
		}

		if paramIdx != len(params) {
			return valpkg.ErrorVal("db.QueryParams: Anzahl der Platzhalter passt nicht zur Anzahl der übergebenen Werte")
		}

		goParams := make([]interface{}, len(params))
		for i, p := range params {
			goParams[i] = valToInterface(p)
		}

		rows, err := db.Query(finalSQL.String(), goParams...)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		defer rows.Close()

		cols, _ := rows.Columns()
		allRows := make([]valpkg.Value, 0)

		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}

		// Identische Säuberungs-Logik wie in db.Query
		clean := func(s string) string {
			s = strings.TrimSpace(s)
			return strings.Map(func(r rune) rune {
				if r >= 32 || r == 9 || r == 10 || r == 13 {
					return r
				}
				return -1
			}, s)
		}

		for rows.Next() {
			row := make([]valpkg.Value, len(cols))
			if err := rows.Scan(ptrs...); err != nil {
				continue
			}

			for i, v := range vals {
				if v == nil {
					row[i] = valpkg.Value{Kind: valpkg.KindNull}
					continue
				}

				switch val := v.(type) {
				case string:
					row[i] = valpkg.StrVal(clean(val))
				case []byte:
					if isBinaryData(val) {
						row[i] = valpkg.StrVal(base64.StdEncoding.EncodeToString(val))
					} else {
						row[i] = valpkg.StrVal(clean(string(val)))
					}
				default:
					row[i] = dbConvertToValue(v)
				}
			}
			allRows = append(allRows, valpkg.Value{Kind: valpkg.KindArr, Arr: row})
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: allRows}
	})

	// ---------------- QueryArray (Unified) ----------------
	Register(ns+"QueryArray", "db", "alias, sql", "Gibt 2D-Array zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("alias, sql benötigt")
		}
		db, _, err := getConn(args[0].Str)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		rows, err := db.Query(args[1].Str)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		return sqlToArr2D(rows)
	})

	// ---------------- Close (Unified) ----------------
	Register(ns+"Close", "db", "alias", "Schließt eine spezifische DB-Verbindung.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.BoolVal(false)
		}
		alias := strings.ToLower(args[0].Str)

		if db, ok := connections[alias]; ok {
			db.Close()
			delete(connections, alias)
			delete(drivers, alias)
			return valpkg.BoolVal(true)
		}
		return valpkg.BoolVal(false)
	})

	// ---------------- db.TableExists ----------------
	Register(ns+"TableExists", "db", "alias, tableName", "Prüft, ob eine Tabelle existiert.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.BoolVal(false)
		}
		alias, tableName := args[0].Str, args[1].Str

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.BoolVal(false)
		}

		query, params := getTableExistsSQL(driver, tableName)
		if query == "" {
			return valpkg.BoolVal(false)
		}

		var count int
		err = db.QueryRow(query, params...).Scan(&count)
		return valpkg.BoolVal(err == nil && count > 0)
	})

	// ---------------- db.ListTables ----------------
	Register(ns+"ListTables", "db", "alias", "Gibt eine Liste aller Tabellen als Array zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		alias := args[0].Str

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		query := getListTablesSQL(driver)
		if query == "" {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		rows, err := db.Query(query)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		defer rows.Close()

		var tables []valpkg.Value
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil {
				tables = append(tables, valpkg.StrVal(name))
			}
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: tables}
	})

	// --- db.Count (Deine Count-Logik) ---
	Register(ns+"Count", "db", "alias, table [, filter, val]", "Zählt Zeilen.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.NumVal(0)
		}
		alias, table := args[0].Str, args[1].Str

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.NumVal(0)
		}

		if !isSafeDBName(table) {
			return valpkg.NumVal(0)
		}

		// Basis-Query (eckige Klammern funktionieren in MSSQL und SQLite)
		query := fmt.Sprintf("SELECT COUNT(*) FROM [%s]", table)

		var count int
		if len(args) >= 4 {
			filter := args[2].Str // z.B. "Status = ?" oder "ID > @p1"
			val := args[3].Str

			// Falls der User ein einfaches "Status = ?" schickt,
			// passen wir es für MSSQL automatisch an, falls nötig.
			if (driver == "mssql" || driver == "sqlserver") && strings.Contains(filter, "?") {
				filter = strings.Replace(filter, "?", "@p1", 1)
			}

			query += " WHERE " + filter
			err = db.QueryRow(query, val).Scan(&count)
		} else {
			err = db.QueryRow(query).Scan(&count)
		}

		if err != nil {
			return valpkg.NumVal(0)
		}
		return valpkg.NumVal(float64(count))
	})

	Register(ns+"GetLastID", "db", "alias [, sql]", "Gibt die zuletzt erzeugte ID zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.NumVal(0)
		}
		alias := args[0].Str

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.NumVal(0)
		}

		// Fall 1: User übergibt eigenes SQL (wie in deinem Original)
		if len(args) >= 2 && args[1].Str != "" {
			var id int64
			err := db.QueryRow(args[1].Str).Scan(&id)
			if err != nil {
				return valpkg.NumVal(0)
			}
			return valpkg.NumVal(float64(id))
		}

		// Fall 2: Automatik-Modus je nach Datenbank
		var lastID int64
		switch driver {
		case "mssql", "sqlserver":
			// Bei MSSQL muss man meist explizit nachfragen (SCOPE_IDENTITY ist am sichersten)
			err = db.QueryRow("SELECT SCOPE_IDENTITY()").Scan(&lastID)
		case "sqlite", "sqlite3":
			// SQLite hat eine spezialisierte Abfrage
			err = db.QueryRow("SELECT last_insert_rowid()").Scan(&lastID)
		case "postgres", "pg":
			// Postgres braucht meist ein "RETURNING id" im INSERT,
			// oder man fragt die Sequence ab (falls bekannt):
			return valpkg.ErrorVal("Postgres benötigt 'RETURNING id' im INSERT-Statement")
		}

		if err != nil {
			return valpkg.NumVal(0)
		}
		return valpkg.NumVal(float64(lastID))
	})

	Register(ns+"SetBlob", "db", "alias, table, col, whereCol, whereVal, b64Data", "Speichert Blobs (Base64-String erforderlich).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 6 {
			return valpkg.ErrorVal("Argumente fehlen")
		}
		alias, table, col, wCol, wVal, data := args[0].Str, args[1].Str, args[2].Str, args[3].Str, args[4].Str, args[5].Str

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		if !isSafeDBName(table) || !isSafeDBName(col) || !isSafeDBName(wCol) {
			return valpkg.ErrorVal("Ungültige Struktur-Namen")
		}

		blobData, _ := base64.StdEncoding.DecodeString(data)

		var query string
		// Hier der Tagged Switch für die Dialekt-Unterscheidung
		switch driver {
		case "mssql", "sqlserver":
			// MSSQL Dialekt mit @p1, @p2
			query = fmt.Sprintf("UPDATE [%s] SET [%s]=@p1 WHERE [%s]=@p2", table, col, wCol)

		case "postgres", "pg":
			// Postgres Dialekt mit $1, $2
			query = fmt.Sprintf("UPDATE \"%s\" SET \"%s\"=$1 WHERE \"%s\"=$2", table, col, wCol)

		default:
			// SQLite / MySQL Dialekt mit ?
			query = fmt.Sprintf("UPDATE [%s] SET [%s]=? WHERE [%s]=?", table, col, wCol)
		}

		_, err = db.Exec(query, blobData, wVal)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}
		return valpkg.BoolVal(true)
	})

	Register(ns+"GetBlob", "db", "alias, table, col, whereCol, whereVal", "Liest einen Blob und gibt ihn als Base64-String zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 5 {
			return valpkg.StrVal("error: missing arguments (alias, table, col, whereCol, whereVal)")
		}

		alias := args[0].Str
		table := args[1].Str
		column := args[2].Str
		whereCol := args[3].Str
		whereVal := args[4].Str

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.StrVal("error: " + err.Error())
		}

		// Sicherheits-Check (nutzt deine existierende Funktion)
		if !isSafeDBName(table) || !isSafeDBName(column) || !isSafeDBName(whereCol) {
			return valpkg.StrVal("error: invalid structure names")
		}

		var query string
		var data []byte

		// Dialekt-Anpassung
		switch driver {
		case "mssql", "sqlserver":
			query = fmt.Sprintf("SELECT [%s] FROM [%s] WHERE [%s] = @p1", column, table, whereCol)
		case "postgres", "pg":
			query = fmt.Sprintf("SELECT \"%s\" FROM \"%s\" WHERE \"%s\" = $1", column, table, whereCol)
		default: // sqlite, mysql
			query = fmt.Sprintf("SELECT [%s] FROM [%s] WHERE [%s] = ?", column, table, whereCol)
		}

		err = db.QueryRow(query, whereVal).Scan(&data)
		if err != nil {
			if err == sql.ErrNoRows {
				return valpkg.StrVal("") // Oder valpkg.NullVal, falls kein Datensatz gefunden wurde
			}
			return valpkg.StrVal("error: " + err.Error())
		}

		// Rückgabe als Base64 (identisch zu deinem Original)
		return valpkg.StrVal(base64.StdEncoding.EncodeToString(data))
	})

	Register(ns+"ListProcedures", "db", "alias", "Listet Stored Procedures (MSSQL/Postgres).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		alias := args[0].Str
		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		var query string
		switch driver {
		case "mssql", "sqlserver":
			// MSSQL nutzt INFORMATION_SCHEMA.ROUTINES
			query = `
            SELECT ROUTINE_SCHEMA, ROUTINE_NAME 
            FROM INFORMATION_SCHEMA.ROUTINES 
            WHERE ROUTINE_TYPE = 'PROCEDURE' 
            ORDER BY ROUTINE_SCHEMA, ROUTINE_NAME`
		case "postgres", "pg":
			// Postgres nutzt ebenfalls INFORMATION_SCHEMA, aber oft kleingeschrieben
			query = `
            SELECT routine_schema, routine_name 
            FROM information_schema.routines 
            WHERE routine_type = 'PROCEDURE' 
            AND routine_schema NOT IN ('pg_catalog', 'information_schema')
            ORDER BY routine_schema, routine_name`
		default:
			// SQLite unterstützt keine Stored Procedures
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		rows, err := db.Query(query)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		defer rows.Close()

		var result []valpkg.Value
		for rows.Next() {
			var schema, name string
			if err := rows.Scan(&schema, &name); err != nil {
				continue
			}
			// Format: dbo.MyProcedure oder public.my_proc
			fullName := schema + "." + name
			result = append(result, valpkg.StrVal(fullName))
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	Register(ns+"ListViews", "db", "alias", "Listet Views der Datenbank auf.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		alias := args[0].Str
		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		var query string
		switch driver {
		case "mssql", "sqlserver":
			// MSSQL: Standard-Views via INFORMATION_SCHEMA
			query = `
            SELECT TABLE_SCHEMA, TABLE_NAME 
            FROM INFORMATION_SCHEMA.VIEWS 
            ORDER BY TABLE_SCHEMA, TABLE_NAME`
		case "postgres", "pg":
			// Postgres: Views ohne System-Schemas
			query = `
            SELECT table_schema, table_name 
            FROM information_schema.views 
            WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
            ORDER BY table_schema, table_name`
		case "sqlite", "sqlite3":
			// SQLite: Views liegen in sqlite_master
			query = "SELECT 'main', name FROM sqlite_master WHERE type = 'view' ORDER BY name"
		default:
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		rows, err := db.Query(query)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		defer rows.Close()

		var result []valpkg.Value
		for rows.Next() {
			var schema, name string
			if err := rows.Scan(&schema, &name); err != nil {
				continue
			}
			// Konsistentes Format: Schema.Name (bzw. main.Name bei SQLite)
			fullName := schema + "." + name
			result = append(result, valpkg.StrVal(fullName))
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	// ---------------- db.DatabaseName ----------------
	Register(ns+"DatabaseName", "db", "alias", "Gibt den Namen der aktuellen DB zurück.", func(args []valpkg.Value) valpkg.Value {
		alias := args[0].Str
		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.StrVal("")
		}

		query := getDatabaseNameSQL(driver)
		var name string
		err = db.QueryRow(query).Scan(&name)
		if err != nil {
			return valpkg.StrVal("")
		}
		return valpkg.StrVal(name)
	})

	// ---------------- db.ListDatabases (MSSQL Spezial) ----------------
	Register(ns+"ListDatabases", "db", "alias", "Listet Datenbanken auf dem Server (MSSQL/PG) oder 'main' bei SQLite.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		alias := args[0].Str
		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		var query string
		switch driver {
		case "mssql", "sqlserver":
			// MSSQL: Alle Datenbanken außer System-DBs (optional filterbar)
			query = "SELECT name FROM sys.databases WHERE name NOT IN ('master', 'tempdb', 'model', 'msdb') ORDER BY name"
		case "postgres", "pg":
			// Postgres: Echte User-Datenbanken
			query = "SELECT datname FROM pg_database WHERE datistemplate = false AND datname != 'postgres' ORDER BY datname"
		case "sqlite", "sqlite3":
			// SQLite: Hat keine Server-Liste, wir geben den Standard-Namespace zurück
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{valpkg.StrVal("main")}}
		default:
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		rows, err := db.Query(query)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		defer rows.Close()

		var res []valpkg.Value
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil {
				res = append(res, valpkg.StrVal(strings.TrimSpace(name)))
			}
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: res}
	})

	Register(ns+"DriverName", "db", "alias", "Gibt den Treiber-Typ zurück (mssql, pg, sqlite).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.StrVal("")
		}
		_, driver, err := getConn(args[0].Str)
		if err != nil {
			return valpkg.StrVal("unknown")
		}
		return valpkg.StrVal(driver)
	})

	Register(ns+"Exec", "db", "alias, sqlString", "Führt SQL aus.", func(args []valpkg.Value) valpkg.Value {
		alias := args[0].Str
		sqlContent := args[1].Str

		// 1. Verbindung holen (Alias wurde vorher per Open gesetzt)
		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		// 2. Batch-Splitting je nach Driver
		var batches []string
		if driver == "mssql" || driver == "sqlserver" {
			// MSSQL braucht GO-Splitting
			re := regexp.MustCompile(`(?im)^\s*GO\s*$`)
			batches = re.Split(sqlContent, -1)
		} else {
			// SQLite & Postgres trennen am Semikolon
			batches = strings.Split(sqlContent, ";")
		}

		// 3. Ausführung (Transaktions-Check)
		// Wir schauen kurz, ob der User selbst BEGIN/COMMIT im String hat
		upper := strings.ToUpper(sqlContent)
		hasManualTx := strings.Contains(upper, "COMMIT") || strings.Contains(upper, "ROLLBACK")

		var finalResults []valpkg.Value

		// Fall A: User steuert Transaktion selbst (oder SQLite im Auto-Commit)
		if hasManualTx {
			for _, b := range batches {
				if t := strings.TrimSpace(b); t != "" {
					res, err := executeSQL(db, t) // Direkt auf DB
					if err != nil {
						return valpkg.ErrorVal(err.Error())
					}
					finalResults = append(finalResults, res)
				}
			}
		} else {
			// Fall B: Wir kapseln alles in eine Go-Transaktion (Sicherheitsnetz)
			tx, _ := db.Begin()
			for _, b := range batches {
				if t := strings.TrimSpace(b); t != "" {
					res, err := executeSQL(tx, t) // Auf Transaktion
					if err != nil {
						tx.Rollback()
						return valpkg.ErrorVal(err.Error())
					}
					finalResults = append(finalResults, res)
				}
			}
			tx.Commit()
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: finalResults}
	})

	Register(ns+"ExecParams", "db", "alias, sql, params", "Führt ein einzelnes parametrisiertes SQL-Statement aus (kein Batch-Splitting, echte Werte-Bindung).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("usage: db.ExecParams(alias, sql, paramsArray)")
		}

		alias := args[0].Str
		sqlTemplate := args[1].Str

		if args[2].Kind != valpkg.KindArr {
			return valpkg.ErrorVal("db.ExecParams: dritter Parameter muss ein Array sein")
		}
		params := args[2].Arr

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		// '?' im Template durch dialektspezifische Platzhalter ersetzen,
		// in der Reihenfolge des Vorkommens.
		var finalSQL strings.Builder
		pos := 0
		paramIdx := 0
		for pos < len(sqlTemplate) {
			if sqlTemplate[pos] == '?' {
				if paramIdx >= len(params) {
					return valpkg.ErrorVal("db.ExecParams: zu wenige Werte für die Platzhalter im SQL übergeben")
				}
				finalSQL.WriteString(placeholder(driver, paramIdx+1))
				paramIdx++
				pos++
				continue
			}
			finalSQL.WriteByte(sqlTemplate[pos])
			pos++
		}

		if paramIdx != len(params) {
			return valpkg.ErrorVal("db.ExecParams: Anzahl der Platzhalter passt nicht zur Anzahl der übergebenen Werte")
		}

		goParams := make([]interface{}, len(params))
		for i, p := range params {
			goParams[i] = valToInterface(p)
		}

		_, err = db.Exec(finalSQL.String(), goParams...)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		return valpkg.BoolVal(true)
	})

	// ---------------- db.ListColumns ----------------
	Register(ns+"ListColumns", "db", "alias, table", "Gibt Spaltennamen zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		alias, table := args[0].Str, args[1].Str
		db, driver, err := getConn(alias)
		if err != nil || !isSafeDBName(table) {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		query, params := getListColumnsSQL(driver, table)
		rows, err := db.Query(query, params...)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}
		defer rows.Close()

		var columns []valpkg.Value
		for rows.Next() {
			var col string
			if driver == "sqlite" {
				// SQLite PRAGMA gibt CID, Name, Type, NotNull, PK zurück (Name ist an Index 1)
				var cid, notnull, pk int
				var dType interface{}
				rows.Scan(&cid, &col, &dType, &notnull, &dType, &pk)
			} else {
				rows.Scan(&col)
			}
			columns = append(columns, valpkg.StrVal(col))
		}
		return valpkg.Value{Kind: valpkg.KindArr, Arr: columns}
	})

	// NOTE:
	// Deaktiviert, da der verwendete SQLite-Treiber (z. B. modernc.org/sqlite)
	// keine SQLCipher-Unterstützung bietet (kein PRAGMA key/rekey).
	/*
		Register(ns+"ChangePassword", "db", "alias, oldKey, newKey", "Ändert das Passwort einer verschlüsselten SQLite-DB.", func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.StrVal("error: alias, oldKey, newKey required")
			}

			alias := args[0].Str
			oldKey := args[1].Str
			newKey := args[2].Str

			db, driver, err := getConn(alias)
			if err != nil {
				return valpkg.StrVal("error: " + err.Error())
			}

			// Sicherheits-Check: Rekey funktioniert nur bei SQLite/SQLCipher
			if driver != "sqlite" && driver != "sqlite3" {
				return valpkg.StrVal("error: ChangePassword is only supported for SQLite (SQLCipher)")
			}

			// 1. Alten Key zur Verifizierung/Öffnung setzen
			// Wir nutzen fmt.Sprintf, da PRAGMAs oft keine Parameter-Platzhalter (?) unterstützen
			if _, err := db.Exec(fmt.Sprintf("PRAGMA key = '%s'", oldKey)); err != nil {
				return valpkg.StrVal("error setting old key: " + err.Error())
			}

			// 2. Rekey (Verschlüsselung ändern oder setzen)
			if _, err := db.Exec(fmt.Sprintf("PRAGMA rekey = '%s'", newKey)); err != nil {
				return valpkg.StrVal("error rekey failed: " + err.Error())
			}

			return valpkg.StrVal("ok")
		})*/

	Register(ns+"ExecFile", "db", "alias, path [, dryRun]", "Führt SQL-Script aus.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("alias und pfad benötigt")
		}
		alias, path := args[0].Str, args[1].Str

		dryRun := len(args) > 2 && dbIsTruthy(args[2])

		db, driver, err := getConn(alias)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		sqlContent, err := readSQLFile(path)
		if err != nil {
			return valpkg.ErrorVal("read error: " + err.Error())
		}

		// --- 1. WEICHE FÜR MANUELLE TRANSAKTIONEN ---
		upperSql := strings.ToUpper(sqlContent)
		hasManualTx := strings.Contains(upperSql, "COMMIT") || strings.Contains(upperSql, "ROLLBACK")

		// --- 2. SPLITTING ---
		var batches []string
		if driver == "mssql" || driver == "sqlserver" {
			re := regexp.MustCompile(`(?im)^\s*GO\s*$`)
			batches = re.Split(sqlContent, -1)
		} else {
			batches = strings.Split(sqlContent, ";")
		}

		var tx *sql.Tx
		var runner SQLRunner = db // Default: Direkt auf die DB

		// Nur Transaktion starten, wenn der User keine eigenen Commits im File hat
		if !hasManualTx {
			tx, err = db.Begin()
			if err != nil {
				return valpkg.ErrorVal("tx error: " + err.Error())
			}
			runner = tx
		}

		var finalResults []valpkg.Value
		for _, batch := range batches {
			batch = strings.TrimSpace(batch)
			if batch == "" {
				continue
			}

			// runner ist entweder tx oder db
			resVal, err := executeSQL(runner, batch)
			if err != nil {
				if tx != nil {
					tx.Rollback()
				}
				return valpkg.ErrorVal("Fehler im SQL-Abschnitt: " + err.Error() + "\nBatch: " + batch)
			}

			if resVal.Kind != valpkg.KindNull {
				finalResults = append(finalResults, resVal)
			}
		}

		// --- 3. ABSCHLUSS ---
		if tx != nil {
			if dryRun {
				tx.Rollback()
			} else {
				if err := tx.Commit(); err != nil {
					return valpkg.ErrorVal("commit error: " + err.Error())
				}
			}
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: finalResults}
	})

	Register(ns+"Optimize", "db", "raw, dialect", "Bereinigt SQL intelligent nach Keywords.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("SQL und Dialekt benötigt")
		}

		// Einfacher Aufruf der externen Funktion
		result := OptimizeSQL(args[0].Str, args[1].Str)

		return valpkg.StrVal(result)
	})

	Register(ns+"OptimizeFile", "db", "path, dialect [, outPath]", "Optimiert eine SQL-Datei direkt.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("Pfad und Dialekt benötigt")
		}
		path := args[0].Str
		dialect := args[1].Str

		// 1. Dateiendung prüfen (Case-Insensitive)
		if !strings.HasSuffix(strings.ToLower(path), ".sql") {
			return valpkg.ErrorVal("Nur .sql Dateien sind erlaubt.")
		}

		// 2. Datei einlesen
		content, err := os.ReadFile(path)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht gelesen werden: " + err.Error())
		}

		// 3. Optimierung anwenden
		optimized := OptimizeSQL(string(content), dialect)

		// 4. Zielpfad bestimmen (Überschreiben oder neuer Pfad)
		targetPath := path
		if len(args) > 2 && args[2].Str != "" {
			targetPath = args[2].Str
			if !strings.HasSuffix(strings.ToLower(targetPath), ".sql") {
				return valpkg.ErrorVal("Zielpfad muss ebenfalls auf .sql enden.")
			}
		}

		// 5. Speichern
		err = os.WriteFile(targetPath, []byte(optimized), 0644)
		if err != nil {
			return valpkg.ErrorVal("Datei konnte nicht gespeichert werden: " + err.Error())
		}

		return valpkg.StrVal("Erfolgreich optimiert: " + targetPath)
	})
}

func init() { initDBFunctions() }
