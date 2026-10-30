// ----------------------------------------
// helper_sql.go | mssql, postgres, sqlite
// ----------------------------------------

package stdlib

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
	valpkg "vbxc/value"
)

type SyncResult struct {
	Table     string
	Insert    int
	Update    int
	Unchanged int
	Delete    int
}

func dbIsTruthy(v valpkg.Value) bool {
	switch v.Kind {
	case valpkg.KindBool:
		return v.Bool
	case valpkg.KindNum:
		return v.Num != 0
	case valpkg.KindStr:
		return v.Str != ""
	case valpkg.KindArr:
		return len(v.Arr) != 0 || len(v.Arr2D) != 0
	case valpkg.KindObj:
		return v.Obj != nil && len(v.Obj.Fields) != 0
	default:
		return false
	}
}

func dbConvertToValue(raw any) valpkg.Value {
	if raw == nil {
		return valpkg.StrVal("")
	}
	if ptr, ok := raw.(*any); ok {
		return dbConvertToValue(*ptr)
	}
	switch v := raw.(type) {
	case int:
		return valpkg.NumVal(float64(v))
	case int32:
		return valpkg.NumVal(float64(v))
	case int64:
		return valpkg.NumVal(float64(v))
	case float32:
		return valpkg.NumVal(float64(v))
	case float64:
		return valpkg.NumVal(v)
	case bool:
		return valpkg.BoolVal(v)
	case time.Time:
		return valpkg.StrVal(formatVB(v, ""))
	case string:
		if t, ok := parseDateSafe(v); ok {
			return valpkg.StrVal(formatVB(t, ""))
		}
		return valpkg.StrVal(v)
	case []byte:
		s := string(v)
		if t, ok := parseDateSafe(s); ok {
			return valpkg.StrVal(formatVB(t, ""))
		}
		return valpkg.StrVal(s)
	default:
		return valpkg.StrVal(fmt.Sprintf("%v", v))
	}
}

var connections = make(map[string]*sql.DB)
var drivers = make(map[string]string)
var validNameMSSQL = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// SQLRunner erlaubt es uns, Funktionen zu schreiben, die sowohl
// mit einer DB-Verbindung als auch mit einer Transaktion arbeiten.
type SQLRunner interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

func quoteIdent(driver, name string) string {
	switch driver {
	case "postgres", "pg":
		return `"` + name + `"`
	default: // mssql, sqlserver, sqlite, sqlite3
		return "[" + name + "]"
	}
}

// placeholder liefert den positionellen SQL-Platzhalter für den jeweiligen Dialekt.
// pos ist 1-indiziert (erster Parameter = 1).
func placeholder(driver string, pos int) string {
	switch driver {
	case "mssql", "sqlserver":
		return fmt.Sprintf("@p%d", pos)
	case "postgres", "pg":
		return fmt.Sprintf("$%d", pos)
	default: // sqlite, sqlite3
		return "?"
	}
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}

func tableExistsDB(db *sql.DB, driver string, table string) (bool, error) {

	driver = strings.ToLower(driver)

	switch driver {

	case "sqlite", "sqlite3":

		var name string

		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)

		if err == sql.ErrNoRows {
			return false, nil
		}

		return err == nil, err

	case "postgres", "pg":

		var exists bool

		err := db.QueryRow(
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)",
			table,
		).Scan(&exists)

		return exists, err

	case "mssql", "sqlserver":

		var exists int

		err := db.QueryRow(
			"SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME=@p1",
			table,
		).Scan(&exists)

		return exists > 0, err

	default:
		return false, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

// resolveTableName ermittelt den tatsächlichen Tabellennamen in der DB,
// unabhängig von Groß-/Kleinschreibung des übergebenen Namens.
// Wichtig bei Sync zwischen DBs mit unterschiedlicher Namens-Konvention
// (z.B. MSSQL "Hoerspiel" <-> Postgres "hoerspiel").
func resolveTableName(db *sql.DB, driver string, table string) (string, error) {
	driver = strings.ToLower(driver)

	var query string

	switch driver {
	case "sqlite", "sqlite3":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND LOWER(name)=LOWER(?)"
	case "postgres", "pg":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND LOWER(table_name)=LOWER($1)"
	case "mssql", "sqlserver":
		query = "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE LOWER(TABLE_NAME)=LOWER(@p1)"
	default:
		return "", fmt.Errorf("nicht unterstützter Treiber: %s", driver)
	}

	var actual string
	err := db.QueryRow(query, table).Scan(&actual)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("Tabelle nicht gefunden: %s", table)
	}
	if err != nil {
		return "", err
	}

	return actual, nil
}

func syncTable(sourceAlias string, targetAlias string, table string, idColumn string, batchSize int) (SyncResult, error) {

	if batchSize <= 0 {
		batchSize = 500
	}

	result := SyncResult{
		Table: table,
	}

	// -------------------------------------------------
	// Validierung
	// -------------------------------------------------

	if !isSafeDBName(table) {
		return result, fmt.Errorf("ungültiger Tabellenname: %s", table)
	}

	if !isSafeDBName(idColumn) {
		return result, fmt.Errorf("ungültige ID-Spalte: %s", idColumn)
	}

	// -------------------------------------------------
	// Verbindungen holen
	// -------------------------------------------------

	src, srcDriver, err := getConn(strings.ToLower(sourceAlias))
	if err != nil {
		return result, fmt.Errorf("Quellverbindung nicht gefunden: %s", sourceAlias)
	}

	dst, dstDriver, err := getConn(strings.ToLower(targetAlias))
	if err != nil {
		return result, fmt.Errorf("Zielverbindung nicht gefunden: %s", targetAlias)
	}

	// -------------------------------------------------
	// Tabellen auflösen (echte Schreibweise ermitteln)
	// -------------------------------------------------

	srcTable, err := resolveTableName(src, srcDriver, table)
	if err != nil {
		return result, fmt.Errorf("Quelltabelle existiert nicht: %s", table)
	}

	dstTable, err := resolveTableName(dst, dstDriver, table)
	if err != nil {
		return result, fmt.Errorf("Zieltabelle existiert nicht: %s", table)
	}

	// -------------------------------------------------
	// Spalten prüfen
	// -------------------------------------------------

	srcCols, err := getTableColumns(src, srcDriver, srcTable)
	if err != nil {
		return result, fmt.Errorf("Quellspalten: %w", err)
	}

	dstCols, err := getTableColumns(dst, dstDriver, dstTable)
	if err != nil {
		return result, fmt.Errorf("Zielspalten: %w", err)
	}

	// Mapping: Quell-Spaltenname (lowercase) -> tatsächlicher Zielname (echte Schreibweise)
	colMap := make(map[string]string, len(dstCols))
	for _, dcol := range dstCols {
		colMap[strings.ToLower(dcol)] = dcol
	}

	for _, col := range srcCols {
		if _, found := colMap[strings.ToLower(col)]; !found {
			return result, fmt.Errorf(
				"Spalte fehlt im Ziel: %s",
				col,
			)
		}
	}

	// idColumn im Ziel-Casing auflösen
	dstIDColumn, ok := colMap[strings.ToLower(idColumn)]
	if !ok {
		return result, fmt.Errorf("ID-Spalte nicht im Ziel gefunden: %s", idColumn)
	}

	// -------------------------------------------------
	// Quelle komplett einlesen
	// -------------------------------------------------

	srcRows, err := src.Query(
		"SELECT * FROM " + quoteIdent(srcDriver, srcTable),
	)
	if err != nil {
		return result, err
	}

	columns, err := srcRows.Columns()
	if err != nil {
		srcRows.Close()
		return result, err
	}

	idIndex := -1

	for i, col := range columns {
		if strings.EqualFold(col, idColumn) {
			idIndex = i
			break
		}
	}

	if idIndex < 0 {
		srcRows.Close()
		return result,
			fmt.Errorf("ID-Spalte nicht gefunden: %s", idColumn)
	}

	var allValues [][]any

	for srcRows.Next() {

		values := make([]any, len(columns))
		scan := make([]any, len(columns))

		for i := range values {
			scan[i] = &values[i]
		}

		if err := srcRows.Scan(scan...); err != nil {
			srcRows.Close()
			return result, err
		}

		allValues = append(allValues, values)
	}

	if err := srcRows.Err(); err != nil {
		srcRows.Close()
		return result, err
	}

	srcRows.Close()

	// -------------------------------------------------
	// Ziel komplett einlesen, um echte Änderungen zu erkennen
	// (Signatur pro Zeile, keyed nach ID als String)
	// -------------------------------------------------

	dstRows, err := dst.Query(
		"SELECT * FROM " + quoteIdent(dstDriver, dstTable),
	)
	if err != nil {
		return result, err
	}

	dstColumns, err := dstRows.Columns()
	if err != nil {
		dstRows.Close()
		return result, err
	}

	dstIDIndex := -1
	for i, col := range dstColumns {
		if strings.EqualFold(col, dstIDColumn) {
			dstIDIndex = i
			break
		}
	}

	if dstIDIndex < 0 {
		dstRows.Close()
		return result, fmt.Errorf("ID-Spalte im Ziel-Resultset nicht gefunden: %s", dstIDColumn)
	}

	// key = string(ID) -> Signatur der restlichen Spalten
	existingSignatures := make(map[string]string)

	for dstRows.Next() {

		values := make([]any, len(dstColumns))
		scan := make([]any, len(dstColumns))

		for i := range values {
			scan[i] = &values[i]
		}

		if err := dstRows.Scan(scan...); err != nil {
			dstRows.Close()
			return result, err
		}

		key := dbValueSignature(values[dstIDIndex])
		existingSignatures[key] = rowSignature(values, dstIDIndex)
	}

	if err := dstRows.Err(); err != nil {
		dstRows.Close()
		return result, err
	}

	dstRows.Close()

	// -------------------------------------------------
	// SQL vorbereiten (immer mit Ziel-Schreibweise!)
	// -------------------------------------------------

	var setClauses []string

	pos := 0

	for i, col := range columns {

		if i == idIndex {
			continue
		}

		pos++

		dstCol := colMap[strings.ToLower(col)]

		setClauses = append(
			setClauses,
			quoteIdent(dstDriver, dstCol)+"="+placeholder(dstDriver, pos),
		)
	}

	updateSQL := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s=%s",
		quoteIdent(dstDriver, dstTable),
		strings.Join(setClauses, ","),
		quoteIdent(dstDriver, dstIDColumn),
		placeholder(dstDriver, pos+1),
	)

	var insertCols []string
	var insertValues []string

	for i, col := range columns {

		dstCol := colMap[strings.ToLower(col)]

		insertCols = append(
			insertCols,
			quoteIdent(dstDriver, dstCol),
		)

		insertValues = append(
			insertValues,
			placeholder(dstDriver, i+1),
		)
	}

	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		quoteIdent(dstDriver, dstTable),
		strings.Join(insertCols, ","),
		strings.Join(insertValues, ","),
	)

	// -------------------------------------------------
	// Transaktion (mit periodischem Commit alle batchSize Zeilen)
	// -------------------------------------------------

	tx, err := dst.Begin()
	if err != nil {
		return result, err
	}

	updateStmt, err := tx.Prepare(updateSQL)
	if err != nil {
		tx.Rollback()
		return result, err
	}

	insertStmt, err := tx.Prepare(insertSQL)
	if err != nil {
		updateStmt.Close()
		tx.Rollback()
		return result, err
	}

	var sourceIDs []any
	rowsSinceCommit := 0

	// -------------------------------------------------
	// Daten abgleichen (nur bei echter Änderung schreiben)
	// -------------------------------------------------

	for _, values := range allValues {

		idValue := values[idIndex]
		sourceIDs = append(sourceIDs, idValue)

		key := dbValueSignature(idValue)
		srcSig := rowSignature(values, idIndex)

		existingSig, existsInTarget := existingSignatures[key]

		if existsInTarget && existingSig == srcSig {
			// Zeile ist identisch -> nichts zu tun
			result.Unchanged++
			continue
		}

		if !existsInTarget {
			// Neue Zeile -> INSERT
			_, err := insertStmt.Exec(values...)
			if err != nil {
				updateStmt.Close()
				insertStmt.Close()
				tx.Rollback()
				return result, err
			}
			result.Insert++

		} else {
			// Zeile existiert, aber Inhalt weicht ab -> UPDATE
			var updateValues []any
			for i, v := range values {
				if i == idIndex {
					continue
				}
				updateValues = append(updateValues, v)
			}
			updateValues = append(updateValues, idValue)

			_, err := updateStmt.Exec(updateValues...)
			if err != nil {
				updateStmt.Close()
				insertStmt.Close()
				tx.Rollback()
				return result, err
			}
			result.Update++
		}

		rowsSinceCommit++

		if rowsSinceCommit >= batchSize {

			updateStmt.Close()
			insertStmt.Close()

			if err := tx.Commit(); err != nil {
				return result, err
			}

			tx, err = dst.Begin()
			if err != nil {
				return result, err
			}

			updateStmt, err = tx.Prepare(updateSQL)
			if err != nil {
				tx.Rollback()
				return result, err
			}

			insertStmt, err = tx.Prepare(insertSQL)
			if err != nil {
				updateStmt.Close()
				tx.Rollback()
				return result, err
			}

			rowsSinceCommit = 0
		}
	}

	updateStmt.Close()
	insertStmt.Close()

	// -------------------------------------------------
	// Löschen verwaister Datensätze
	// -------------------------------------------------

	if len(sourceIDs) == 0 {

		_, err := tx.Exec(
			"DELETE FROM " + quoteIdent(dstDriver, dstTable),
		)

		if err != nil {
			tx.Rollback()
			return result, err
		}

	} else {

		params := make([]string, len(sourceIDs))

		for i := range sourceIDs {
			params[i] = placeholder(dstDriver, i+1)
		}

		sqlText := fmt.Sprintf(
			"DELETE FROM %s WHERE %s NOT IN (%s)",
			quoteIdent(dstDriver, dstTable),
			quoteIdent(dstDriver, dstIDColumn),
			strings.Join(params, ","),
		)

		res, err := tx.Exec(sqlText, sourceIDs...)

		if err != nil {
			tx.Rollback()
			return result, err
		}

		deleted, _ := res.RowsAffected()

		result.Delete = int(deleted)
	}

	if err := tx.Commit(); err != nil {
		return result, err
	}

	return result, nil
}

// dbValueSignature wandelt einen beliebigen DB-Scan-Wert in eine vergleichbare
// String-Repräsentation um. Wird genutzt, um Quell- und Ziel-Zeilen auf
// inhaltliche Gleichheit zu prüfen, unabhängig von Treiber-spezifischen
// Go-Typen (z.B. int64 vs float64 vs []byte für denselben SQL-Typ).
func dbValueSignature(v any) string {
	switch t := v.(type) {
	case nil:
		return "\x00NULL\x00"
	case []byte:
		return string(t)
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func rowSignature(values []any, skipIndex int) string {
	var b strings.Builder
	for i, v := range values {
		if i == skipIndex {
			continue
		}
		b.WriteString(dbValueSignature(v))
		b.WriteByte('\x1F') // Unit Separator als Trennzeichen
	}
	return b.String()
}

func getTableColumns(db *sql.DB, driver string, table string) ([]string, error) {

	var query string

	switch driver {

	case "sqlite", "sqlite3":
		query = "PRAGMA table_info(" + quoteIdent(driver, table) + ")"

	case "postgres", "pg":
		query = `
			SELECT column_name 
			FROM information_schema.columns
			WHERE table_name = $1
			ORDER BY ordinal_position
		`

	case "mssql", "sqlserver":
		query = `
			SELECT COLUMN_NAME
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_NAME = @p1
			ORDER BY ORDINAL_POSITION
		`

	default:
		return nil, fmt.Errorf("nicht unterstützter Treiber: %s", driver)
	}

	rows, err := db.Query(query, table)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var columns []string

	switch driver {

	case "sqlite", "sqlite3":

		for rows.Next() {

			var cid int
			var name string
			var typ string
			var notnull int
			var dflt any
			var pk int

			err := rows.Scan(
				&cid,
				&name,
				&typ,
				&notnull,
				&dflt,
				&pk,
			)

			if err != nil {
				return nil, err
			}

			columns = append(columns, name)
		}

	default:

		for rows.Next() {

			var name string

			if err := rows.Scan(&name); err != nil {
				return nil, err
			}

			columns = append(columns, name)
		}
	}

	return columns, rows.Err()
}

func convertDBValue(srcDriver, dstDriver string, col *sql.ColumnType, value any) any {
	_ = srcDriver

	dbType := strings.ToUpper(col.DatabaseTypeName())

	switch v := value.(type) {

	case nil:
		return nil

	case []byte:
		switch dbType {
		case "VARBINARY", "IMAGE", "BLOB", "BYTEA", "BINARY":
			return v
		}

		return string(v)

	case time.Time:
		return v

	case bool:
		if dstDriver == "sqlite" || dstDriver == "sqlite3" {
			if v {
				return 1
			}
			return 0
		}
		return v

	default:
		return value
	}
}

// readSQLFile prüft die Endung und liest den Inhalt der SQL-Datei ein
func readSQLFile(path string) (string, error) {
	// 1. Endung prüfen
	if !strings.HasSuffix(strings.ToLower(path), ".sql") {
		return "", fmt.Errorf("nur .sql Dateien sind erlaubt")
	}

	// 2. Existenz prüfen
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("datei nicht gefunden: %s", path)
	}

	// 3. Lesen
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("fehler beim Lesen: %v", err)
	}

	return string(content), nil
}

// OptimizeSmart bereitet SQL-Strings für die Batch-Ausführung vor.
// Er entfernt Kommentare, normalisiert Trenner und schützt Strings.
func OptimizeSQL(raw string, dialect string) string {
	dialect = strings.ToLower(dialect)

	// 1. Kommentare entfernen (Mehrzeilig /* */ und Einzeilig --)
	reMulti := regexp.MustCompile(`(?s)/\*.*?\*/`)
	clean := reMulti.ReplaceAllString(raw, "")

	reSingle := regexp.MustCompile(`(?m)--.*$`)
	clean = reSingle.ReplaceAllString(clean, "")

	// 2. In Zeilen zerlegen für Keyword-Analyse
	lines := strings.Split(clean, "\n")
	var batches []string
	var currentBatch []string

	// Start-Keywords, die einen neuen logischen Block einleiten
	keywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE",
		"CREATE", "DROP", "ALTER", "IF", "BEGIN",
		"WITH", "EXEC", "TRUNCATE", "MERGE", "GRANT",
		"COMMIT", "ROLLBACK", "TRANSACTION", // NEU hinzugefügt
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		upperLine := strings.ToUpper(trimmed)
		isNewCommand := false

		// Prüfen, ob die Zeile mit einem Kommando startet
		for _, k := range keywords {
			if strings.HasPrefix(upperLine, k) {
				isNewCommand = true
				break
			}
		}

		// Wenn ein neuer Befehl startet, alten Batch verarbeiten
		if isNewCommand && len(currentBatch) > 0 {
			batches = append(batches, finalizeBatch(currentBatch))
			currentBatch = []string{}
		}

		currentBatch = append(currentBatch, trimmed)
	}

	// Letzten Block hinzufügen
	if len(currentBatch) > 0 {
		batches = append(batches, finalizeBatch(currentBatch))
	}

	// 3. Zusammenbau mit Dialekt-Trennern
	if dialect == "mssql" || dialect == "sqlserver" {
		// MSSQL braucht das GO als harten Batch-Separator für den Exec-Handler
		return strings.Join(batches, "\nGO\n") + "\nGO"
	}

	// Standard (Postgres/SQLite) nutzt nur Semikolon
	return strings.Join(batches, "\n")
}

// finalizeBatch säubert ein einzelnes SQL-Statement
func finalizeBatch(lines []string) string {
	full := strings.Join(lines, " ")

	// REGEX: Mehrfache Semikolons am Ende (inkl. Whitespaces dazwischen)
	// auf genau EIN Semikolon reduzieren.
	reMultiSemi := regexp.MustCompile(`\s*;[\s;]*$`)

	// Falls am Ende Semikolons sind -> ersetzen durch eines
	if reMultiSemi.MatchString(full) {
		full = reMultiSemi.ReplaceAllString(full, ";")
	} else {
		// Falls gar kein Semikolon da ist (und es kein Spezialfall ist) -> eins dran
		trimmed := strings.TrimSpace(full)
		if trimmed != "" && !strings.HasSuffix(trimmed, ";") {
			full = trimmed + ";"
		}
	}

	return full
}

func getConn(alias string) (*sql.DB, string, error) {
	alias = strings.ToLower(alias)
	db, ok := connections[alias]
	if !ok {
		return nil, "", fmt.Errorf("verbindung '%s' nicht gefunden", alias)
	}
	return db, drivers[alias], nil
}

func getTableExistsSQL(driver, tableName string) (string, []interface{}) {
	switch driver {
	case "mssql", "sqlserver":
		return "SELECT COUNT(*) FROM sys.tables WHERE name = @p1", []interface{}{tableName}
	case "postgres", "pg":
		return "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1", []interface{}{tableName}
	case "sqlite", "sqlite3":
		return "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", []interface{}{tableName}
	default:
		return "", nil
	}
}

func getListTablesSQL(driver string) string {
	switch driver {
	case "mssql", "sqlserver":
		return "SELECT name FROM sys.tables ORDER BY name"
	case "postgres", "pg":
		return "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name"
	case "sqlite", "sqlite3":
		return "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	default:
		return ""
	}
}

func isSafeDBName(name string) bool {
	// Dein Regex: /^[a-zA-Z0-9_]+$/
	return validNameMSSQL.MatchString(name)
}

func getListColumnsSQL(driver, table string) (string, []interface{}) {
	switch driver {
	case "mssql", "sqlserver":
		return "SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = @p1 ORDER BY ORDINAL_POSITION", []interface{}{table}
	case "postgres", "pg":
		return "SELECT column_name FROM information_schema.columns WHERE table_name = $1 ORDER BY ordinal_position", []interface{}{table}
	case "sqlite", "sqlite3":
		return fmt.Sprintf("PRAGMA table_info([%s])", table), nil // SQLite nutzt PRAGMA statt SELECT
	}
	return "", nil
}

func getDatabaseNameSQL(driver string) string {
	switch driver {
	case "mssql", "sqlserver":
		return "SELECT DB_NAME()"
	case "postgres", "pg":
		return "SELECT current_database()"
	case "sqlite", "sqlite3":
		return "SELECT name FROM pragma_database_list WHERE seq = 0"
	}
	return ""
}

func executeSQL(dbOrTx interface{}, batch string) (valpkg.Value, error) {
	batch = strings.TrimSpace(batch)
	if batch == "" {
		return valpkg.Value{Kind: valpkg.KindNull}, nil
	}

	// 1. Robuste Typerkennung für SELECT/Queries
	// Wir ignorieren führende Kommentare bei der Typerkennung
	trimmedUpper := strings.ToUpper(batch)
	re := regexp.MustCompile(`(?s)^(--.*?\n|/\*.*?\*/\s*)*`)
	cleanStart := re.ReplaceAllString(trimmedUpper, "")

	// Ein Batch gilt als Query, wenn er mit Daten-liefernden Befehlen beginnt
	isQuery := strings.HasPrefix(cleanStart, "SELECT") ||
		strings.HasPrefix(cleanStart, "WITH") ||
		strings.HasPrefix(cleanStart, "EXEC") ||
		strings.HasPrefix(cleanStart, "SHOW")

	var err error

	// 2. Ausführung je nach Typ
	if isQuery {
		var rows *sql.Rows
		switch conn := dbOrTx.(type) {
		case *sql.Tx:
			rows, err = conn.Query(batch)
		case *sql.DB:
			rows, err = conn.Query(batch)
		default:
			return valpkg.Value{}, fmt.Errorf("ungültiger Datenbank-Kontext (Query)")
		}

		if err != nil {
			return valpkg.Value{}, err
		}
		// Übergibt an deine sqlToArr2D (inkl. Header-Logik)
		return sqlToArr2D(rows), nil

	} else {
		var res sql.Result
		switch conn := dbOrTx.(type) {
		case *sql.Tx:
			res, err = conn.Exec(batch)
		case *sql.DB:
			res, err = conn.Exec(batch)
		default:
			return valpkg.Value{}, fmt.Errorf("ungültiger Datenbank-Kontext (Exec)")
		}

		if err != nil {
			return valpkg.Value{}, err
		}

		ra, _ := res.RowsAffected()
		// Rückgabe einer Erfolgsmeldung für das Result-Array
		return valpkg.StrVal(fmt.Sprintf("OK: %d Zeilen betroffen", ra)), nil
	}
}

func sqlToArr2D(rows *sql.Rows) valpkg.Value {
	defer rows.Close()
	cols, _ := rows.Columns()
	allRows := [][]valpkg.Value{}

	// --- NEU: Header-Zeile als allererstes hinzufügen ---
	headerRow := make([]valpkg.Value, len(cols))
	for i, colName := range cols {
		headerRow[i] = valpkg.StrVal(colName) // Spaltennamen sind immer Strings
	}
	allRows = append(allRows, headerRow)
	// ----------------------------------------------------

	vals := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	cleanFunc := func(s string) string {
		s = strings.TrimSpace(s)
		return strings.Map(func(r rune) rune {
			// Behalte druckbare Zeichen (>=32) oder Tab (9)
			if r >= 32 || r == 9 || r == 10 || r == 13 {
				return r
			}
			return -1
		}, s)
	}

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}

		rowValues := make([]valpkg.Value, len(cols))
		for i, v := range vals {
			if v == nil {
				rowValues[i] = valpkg.Value{Kind: valpkg.KindNull}
				continue
			}

			switch val := v.(type) {
			case string:
				rowValues[i] = valpkg.StrVal(cleanFunc(val))
			case []byte:
				if isBinaryData(val) {
					rowValues[i] = valpkg.StrVal(base64.StdEncoding.EncodeToString(val))
				} else {
					rowValues[i] = valpkg.StrVal(cleanFunc(string(val)))
				}
			default:
				rowValues[i] = dbConvertToValue(v)
			}
		}
		allRows = append(allRows, rowValues)
	}

	return valpkg.Value{Kind: valpkg.KindArr2D, Arr2D: allRows}
}

func isBinaryData(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Wir prüfen die ersten 512 Bytes (reicht für die Erkennung)
	checkLen := len(data)
	if checkLen > 512 {
		checkLen = 512
	}

	nullCount := 0
	for i := 0; i < checkLen; i++ {
		b := data[i]

		// 1. Steuerzeichen (außer Tab, LineFeed, CarriageReturn)
		// 0-8, 11-12, 14-31 sind verdächtig
		if b < 32 && b != 9 && b != 10 && b != 13 {
			// Wenn wir ein Null-Byte finden, ist es fast sicher Binär (z.B. EXE/JPG)
			if b == 0 {
				nullCount++
			}
			// Bei mehr als 2 "echten" Steuerzeichen oder einem Null-Byte -> Binär
			if nullCount > 0 || i > 10 {
				return true
			}
		}
	}

	// 2. UTF-8 Validierung: Wenn es kein gültiges UTF-8 ist,
	// muss es als Base64 kodiert werden, damit der JSON/VB-Transport nicht bricht.
	if !utf8.Valid(data) {
		return true
	}

	return false
}

// Hilfs-Wrapper, der den Pointer-Check sauber kapselt
func dbProcessTargetPath(p string) (string, *valpkg.Value) {
	abs, errVal := absPathVal(p) // Nutzt deine Funktion, die (string, *valpkg.Value) liefert
	if errVal != nil {
		return "", errVal // errVal ist hier bereits ein *valpkg.Value, also ist nil-Check erlaubt
	}
	return abs, nil
}

func valueToSQL(v valpkg.Value) any {
	switch v.Kind {

	case valpkg.KindStr:
		return v.Str

	case valpkg.KindNum:
		return v.Num

	case valpkg.KindBool:
		return v.Bool

	case valpkg.KindNull:
		return nil

	case valpkg.KindArr:
		return nil

	default:
		return v.Str
	}
}
