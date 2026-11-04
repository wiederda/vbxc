# 📋 export.* – Export-Funktionen (CSV/XLSX)

Funktionen zum Lesen und Schreiben von CSV- und XLSX-Dateien. Arbeitet mit
`KindArr` (1D) und `KindArr2D` (2D).

---

## export.XLSXSheets(path)
- **Konkret:**
  Gibt die Namen aller Tabellenblätter einer XLSX-Datei zurück.
- **Parameter:**
  - `path`: Quelldatei.
- **Rückgabe:**
  `ArrVal` mit den Blattnamen als Strings.

---

## export.FromXLSX(path [, sheetName, exclude, column])
- **Konkret:**
  Lädt eine XLSX-Datei in ein Array.

  Ohne Angabe von `sheetName` wird das erste Tabellenblatt der Datei gelesen.
  Enthält das Tabellenblatt nur eine Spalte, wird ein 1D-Array (`KindArr`) zurückgegeben.
  Enthält das Tabellenblatt mehrere Spalten, wird ein 2D-Array (`KindArr2D`) zurückgegeben.

  Mit `exclude` können komplette Zeilen beim Einlesen ausgeschlossen werden. Eine Zeile wird übersprungen, wenn mindestens eine ihrer Zellen einen Wert aus dem Ausschluss-Array {} enthält.

  Mit `column` kann gezielt eine einzelne Spalte aus einer mehrspaltigen Tabelle eingelesen werden. Die Spaltennummer ist 0-basiert, wobei `0` der ersten Spalte (A), `1` der zweiten Spalte (B) usw. entspricht. Bei Verwendung von `column` wird ein 1D-Array (`KindArr`) zurückgegeben.

  Leere Zeilen werden ignoriert.

- **Parameter:**
  - `path`: Quelldatei.
  - `sheetName`: Optional. Name des zu lesenden Tabellenblatts (Standard: erstes Blatt).
  - `exclude`: Optional. Array mit Werten, deren Vorkommen in einer beliebigen Zelle zum Überspringen der gesamten Zeile führt.
  - `column`: Optional. 0-basierte Spaltennummer. Wird nur eine bestimmte Spalte einer mehrspaltigen Tabelle benötigt.

---

## export.ToXLSX(path, data [, sheetName, exclude, append, headers])
- **Konkret:**
  Speichert ein 1D- oder 2D-Array als XLSX-Datei (Excel-Format), ohne dass Excel oder eine andere Office-Anwendung installiert sein muss.
  Bei einem 1D-Array aus verschachtelten Arrays wird jedes innere Array als eigene Zeile geschrieben.
  Bei einem flachen 1D-Array aus einfachen Werten (Strings, Zahlen) wird jedes Element als eigene Zeile mit genau einer Zelle geschrieben.
  Die Funktion unterstützt mehrere Tabellenblätter innerhalb derselben XLSX-Datei.
  Wird eine bereits vorhandene XLSX-Datei angegeben, kann ein weiteres Tabellenblatt hinzugefügt werden.
  Standardmäßig wird ein bereits vorhandenes Tabellenblatt mit demselben Namen gelöscht und anschließend neu erstellt.
  Die Spaltenbreite richtet sich nach dem breitesten Inhalt der Spalte (Überschrift oder Daten, mindestens 8, höchstens 80 Zeichen), auch ohne headers.

  Mit `append=True` bleibt das vorhandene Tabellenblatt erhalten und die neuen Daten werden ab der ersten freien Zeile angehängt. Existiert das Tabellenblatt noch nicht, wird es neu erstellt. Überschriften werden nicht erneut geschrieben, wenn das Tabellenblatt bereits Daten enthält.

  Mit `exclude` können Zeilen ausgeschlossen werden. Eine Zeile wird nicht geschrieben, wenn mindestens eine ihrer Zellen einen Wert aus dem Ausschluss-Array `{}` enthält.

  Mit `headers` können Spaltenüberschriften angegeben werden. Die Überschriften werden in der ersten Zeile geschrieben und fett dargestellt.

  - **Parameter:**
  - `path`: Zieldatei.
  - `data`: `ArrVal` oder `KindArr2D`.
  - `sheetName`: Optional. Name des Tabellenblatts (Standard: `"Sheet1"`).
  - `exclude`: Optional. Array `{}` mit Werten, deren Vorkommen in einer beliebigen Zelle zum Überspringen der gesamten Zeile führt.
  - `append`: Optional. Boolean. Bei `True` werden die Daten an das vorhandene Tabellenblatt angehängt (Standard: `False`).
  - `headers`: Optional. Array `{}` mit den Spaltenüberschriften. Die Überschriften werden fett dargestellt.

---

## export.CSVHeaders(path [, sep])
- **Konkret:**
  Liest die erste nicht leere Zeile einer CSV-Datei und gibt sie als 1D-Array zurück. Gedacht für Dateien, deren erste Zeile die Spaltenüberschriften enthält.

  Verwendet dieselbe Bereinigung wie `export.FromCSV` (ungültige Steuerzeichen und Null-Bytes werden entfernt, Windows-1252 wird bei Bedarf nach UTF-8 konvertiert, einsame Anführungszeichen werden toleriert). Führende Leerzeilen werden übersprungen. Die Werte werden getrimmt.

  Zusammen mit `export.FromCSV(..., skipHeader=True)` und `export.ToXLSX(..., headers)` lassen sich Überschriften und Daten getrennt behandeln.

- **Parameter:**
  - `path`: Quelldatei.
  - `sep`: Optional. Trennzeichen (Standard: `;`).
- **Rückgabe:**
  `ArrVal` mit den Überschriften als Strings. Ist die Datei nicht lesbar oder enthält sie keine nicht leere Zeile, wird ein leeres Array zurückgegeben.

---

## export.FromCSV(path [, sep, exclude, skipHeader])
- **Konkret:**
  Lädt eine CSV-Datei in ein 2D-Array.

  Bereinigt automatisch ungültige Steuerzeichen und Null-Bytes.
  Konvertiert Windows-1252 nach UTF-8 falls nötig.
  Toleriert ungleichmäßige Spaltenanzahl und einsame Anführungszeichen.
  Leere Zeilen werden ignoriert.

  Mit `exclude` können komplette Zeilen beim Einlesen ausgeschlossen werden. Eine Zeile wird übersprungen, wenn mindestens eine ihrer Zellen einen Wert aus dem Ausschluss-Array {} enthält.

  Mit `skipHeader=True` wird die erste nicht leere Zeile (die Überschriften) nicht in das Ergebnis übernommen. Die Prüfung auf `exclude` erfolgt erst danach. Die Überschriften lassen sich getrennt mit `export.CSVHeaders` einlesen.

- **Parameter:**
  - `path`: Quelldatei.
  - `sep`: Optional. Trennzeichen (Standard: `;`).
  - `exclude`: Optional. Array mit Werten, deren Vorkommen in einer beliebigen Zelle zum Überspringen der gesamten Zeile führt.
  - `skipHeader`: Optional. Boolean. Bei `True` wird die erste nicht leere Zeile übersprungen (Standard: `False`).

---

## export.ToCSV(path, data [, sep, exclude, append])
- **Konkret:**
  Speichert ein 1D- oder 2D-Array als CSV-Datei.
  Bei einem 1D-Array aus verschachtelten Arrays wird jedes innere Array als eigene Zeile geschrieben.
  Bei einem flachen 1D-Array aus einfachen Werten (Strings, Zahlen) wird jedes Element als eigene Zeile mit genau einer Zelle geschrieben.

  Mit `exclude` können Zeilen ausgeschlossen werden. Eine Zeile wird nicht geschrieben, wenn mindestens eine ihrer Zellen einen Wert aus dem Ausschluss-Array {} enthält.

  Mit `append=True` werden die Daten an eine bereits vorhandene CSV-Datei angehängt. Ist die Datei noch nicht vorhanden, wird sie automatisch erstellt.

- **Parameter:**
  - `path`: Zieldatei.
  - `data`: `ArrVal` oder `KindArr2D`.
  - `sep`: Optional. Trennzeichen (Standard: `;`).
  - `exclude`: Optional. Array {} mit Werten, deren Vorkommen in einer beliebigen Zelle zum Überspringen der gesamten Zeile führt.
  - `append`: Optional. Boolean. Bei `True` werden die Daten an die bestehende Datei angehängt (Standard: `False`).