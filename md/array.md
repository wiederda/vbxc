# 📋 array.* – Array-Funktionen

Funktioniert mit `KindArr` (1D) und `KindArr2D` (2D). Modifizierende Operationen
(Add, Insert, Remove, RemoveAt, Clear, Prepend, Append) geben das veränderte
Array zurück – der Rückgabewert muss der Variable stets erneut zugewiesen
werden, reine Aufrufe ohne Zuweisung haben keine Wirkung auf die Variable.

---

## array.Create([args...])
- **Konkret:**
  Erstellt Arrays in vier Varianten abhängig von den übergebenen Argumenten.
- **Varianten:**
  - `array.Create(10)` – 1D-Array mit 10 Elementen, alle `0`.
  - `array.Create(3, 3)` – Leere 3×3-Matrix (`KindArr2D`), alle `0`.
  - `array.Create({1,2}, {3,4})` – 2D-Matrix aus Array-Literalen.
  - `array.Create(1, 2, "Test")` – 1D-Array aus den übergebenen Werten.
- **Rückgabe:**
  `ArrVal` oder `KindArr2D`.

---

## array.Add(arr, val)
- **Konkret:**
  Fügt einen Wert am Ende eines 1D-Arrays ein und gibt das erweiterte Array zurück.
  **Wichtig:** Trotz der internen In-Place-Modifikation muss der Rückgabewert der
  Variable erneut zugewiesen werden (`arr = array.Add(arr, val)`), da je nach
  interner Speicherverwaltung sonst nicht sichergestellt ist, dass die Variable
  selbst die neue Länge sieht.
- **Parameter:**
  - `arr`: 1D-Array.
  - `val`: Einzufügender Wert.
- **Rückgabe:**
  `ArrVal` (das erweiterte Array – muss der Variable neu zugewiesen werden)

---

## array.Push(arr, val)
- **Konkret:**
  Fügt einen Wert am Ende eines 1D-Arrays ein und gibt das erweiterte Array zurück.
  **Wichtig:** Trotz der internen In-Place-Modifikation muss der Rückgabewert der
  Variable erneut zugewiesen werden (`arr = array.Push(arr, val)`), da je nach
  interner Speicherverwaltung sonst nicht sichergestellt ist, dass die Variable
  selbst die neue Länge sieht. Funktional identisch zu `array.Add`.
- **Parameter:**
  - `arr`: 1D-Array.
  - `val`: Einzufügender Wert.
- **Rückgabe:**
  `ArrVal` (das erweiterte Array – muss der Variable neu zugewiesen werden)

---

## array.Append(arr, value)
- **Konkret:**
  Alias für `array.Add`. Gleicher Hinweis zur Neuzuweisung gilt hier ebenso.
- **Rückgabe:**
  `ArrVal` (muss der Variable neu zugewiesen werden)

---

## array.Prepend(arr, value)
- **Konkret:**
  Fügt einen Wert am Anfang eines Arrays ein (Index 0).
  **Wichtig:** Rückgabewert muss der Variable erneut zugewiesen werden
  (`arr = array.Prepend(arr, value)`), da intern über `array.Insert` implementiert.
- **Parameter:**
  - `arr`: 1D-Array.
  - `value`: Einzufügender Wert.
- **Rückgabe:**
  `ArrVal` (muss der Variable neu zugewiesen werden)

---

## array.Insert(arr, index, value)
- **Konkret:**
  Fügt einen Wert an einer bestimmten Position ein.
  Alle nachfolgenden Elemente werden verschoben.
  Bei negativem Index wird am Anfang eingefügt.
  Bei Index größer als die Länge wird am Ende angehängt.
  **Wichtig:** Rückgabewert muss der Variable erneut zugewiesen werden
  (`arr = array.Insert(arr, index, value)`).
- **Parameter:**
  - `arr`: 1D-Array.
  - `index`: Zielposition (0-basiert).
  - `value`: Einzufügender Wert.
- **Rückgabe:**
  `ArrVal` (muss der Variable neu zugewiesen werden)

---

## array.Remove(arr, value)
- **Konkret:**
  Entfernt das erste Vorkommen eines Wertes aus dem Array.
  Vergleich erfolgt als String.
  **Wichtig:** Rückgabewert muss der Variable erneut zugewiesen werden
  (`arr = array.Remove(arr, value)`).
- **Parameter:**
  - `arr`: 1D-Array.
  - `value`: Zu entfernender Wert.
- **Rückgabe:**
  `ArrVal` (muss der Variable neu zugewiesen werden)

---

## array.RemoveAt(arr, index)
- **Konkret:**
  Entfernt das Element an einer bestimmten Position.
  Ignoriert ungültige Indizes ohne Fehler.
  **Wichtig:** Rückgabewert muss der Variable erneut zugewiesen werden
  (`arr = array.RemoveAt(arr, index)`).
- **Parameter:**
  - `arr`: 1D-Array.
  - `index`: Position (0-basiert).
- **Rückgabe:**
  `ArrVal` (muss der Variable neu zugewiesen werden)

---

## array.Clear(array)
- **Konkret:**
  Leert ein bestehendes Array. Die Variable bleibt ein Array, enthält aber
  keine Elemente mehr.
  **Wichtig:** Rückgabewert muss der Variable erneut zugewiesen werden
  (`arr = array.Clear(arr)`).
- **Parameter:**
  - `array`: 1D-Array.
- **Rückgabe:**
  `ArrVal` (leer, muss der Variable neu zugewiesen werden)

---

## array.Count(array [, dim])
- **Konkret:**
  Gibt die Anzahl der Elemente zurück.
  Bei 2D-Arrays: `dim=1` = Zeilen (Standard), `dim=2` = Spalten.
  Bei Strings: Zeichenanzahl.
- **Parameter:**
  - `array`: 1D- oder 2D-Array.
  - `dim`: Optional. Dimension für 2D-Arrays (Standard: `1`).
- **Rückgabe:**
  `NumVal`

---

## array.Length(array [, dim])
- **Konkret:**
  Alias für `array.Count`.
- **Rückgabe:**
  `NumVal`

---

## array.LBound(arr [, dim])
- **Konkret:**
  Gibt die Untergrenze der angegebenen Dimension zurück.
  In VBMini immer `0` (0-basierte Arrays).
  Gibt `-1` zurück wenn das Array leer ist oder die Dimension nicht existiert.
- **Parameter:**
  - `arr`: 1D- oder 2D-Array.
  - `dim`: Optional. Dimension (Standard: `1`).
- **Rückgabe:**
  `NumVal`

---

## array.UBound(arr [, dim])
- **Konkret:**
  Gibt die Obergrenze der angegebenen Dimension zurück (letzter gültiger Index).
  Gibt `-1` zurück wenn das Array leer ist oder die Dimension nicht existiert.
- **Parameter:**
  - `arr`: 1D- oder 2D-Array.
  - `dim`: Optional. Dimension (Standard: `1`).
- **Rückgabe:**
  `NumVal`

---

## array.First(array)
- **Konkret:**
  Gibt das erste Element zurück.
  Bei 2D-Arrays: gibt die erste Zeile als `ArrVal` zurück.
- **Parameter:**
  - `array`: 1D- oder 2D-Array.
- **Rückgabe:**
  Erstes Element oder `NullVal` wenn leer.

---

## array.Last(array)
- **Konkret:**
  Gibt das letzte Element zurück.
  Bei 2D-Arrays: gibt die letzte Zeile als `ArrVal` zurück.
- **Parameter:**
  - `array`: 1D- oder 2D-Array.
- **Rückgabe:**
  Letztes Element oder `NullVal` wenn leer.

---

## array.Contains(arr, value)
- **Konkret:**
  Prüft ob ein Wert im Array enthalten ist.
  Vergleich erfolgt als String.
- **Parameter:**
  - `arr`: 1D-Array.
  - `value`: Gesuchter Wert.
- **Rückgabe:**
  `BoolVal`

---

## array.IndexOf(arr, value)
- **Konkret:**
  Gibt die Position des ersten Treffers zurück (0-basiert).
  Vergleich erfolgt als String.
- **Parameter:**
  - `arr`: 1D-Array.
  - `value`: Gesuchter Wert.
- **Rückgabe:**
  `NumVal`
  Position des Treffers, oder `-1` wenn nicht gefunden.

---

## array.Find(arr, pattern [, case])
- **Konkret:**
  Filtert ein Array mit Wildcard-Muster (`*`, `?`).
  Ohne Wildcards wird `Contains` genutzt.
  Suche ist standardmäßig case-insensitiv.
- **Parameter:**
  - `arr`: 1D-Array.
  - `pattern`: Suchmuster (z. B. `"Log_*.txt"`).
  - `case`: Optional. `BoolVal` – bei `true` case-sensitiv.
- **Rückgabe:**
  `ArrVal` aller Treffer.

---

## array.Sort(array)
- **Konkret:**
  Sortiert ein Array und gibt eine Kopie zurück (nicht In-Place).
  Zahlen werden numerisch sortiert, alles andere lexikografisch.
- **Parameter:**
  - `array`: 1D-Array.
- **Rückgabe:**
  `ArrVal` (sortierte Kopie)

---

## array.NaturalSort(array)
- **Konkret:**
  Sortiert ein Array nach einer natürlichen, menschenlesbaren Reihenfolge und gibt eine sortierte Kopie zurück.
  Im Gegensatz zu `array.Sort` berücksichtigt `NaturalSort` Zahlen innerhalb von Texten. Dadurch werden beispielsweise `Film2` und `Film10` korrekt sortiert.
  Die Sortierung berücksichtigt außerdem Sonderzeichen, Groß-/Kleinschreibung und deutsche Umlaute.
  Die Sortierreihenfolge orientiert sich an der Sortierung des Windows Explorers. Dabei werden Sonderzeichen vor Zahlen und Zahlen vor Buchstaben eingeordnet.
- **Parameter:**
  - `array`: 1D-Array.
- **Rückgabe:**
  `ArrVal` (sortierte Kopie)

---

## array.Reverse(array)
- **Konkret:**
  Gibt ein neues Array mit umgekehrter Reihenfolge zurück (nicht In-Place).
- **Parameter:**
  - `array`: 1D-Array.
- **Rückgabe:**
  `ArrVal`

---

## array.Unique(array)
- **Konkret:**
  Entfernt doppelte Werte und gibt eine Kopie zurück.
  Vergleich ist typsicher (Zahl `1` ≠ String `"1"`).
- **Parameter:**
  - `array`: 1D-Array.
- **Rückgabe:**
  `ArrVal`

---

## array.Merge(array, array)
- **Konkret:**
  Verbindet zwei 1D-Arrays zu einem neuen Array.
- **Parameter:**
  - Zwei `ArrVal`.
- **Rückgabe:**
  `ArrVal`

---

## array.Clone(array)
- **Konkret:**
  Erstellt eine flache Kopie eines Arrays (Shallow Copy).
- **Parameter:**
  - `array`: 1D-Array.
- **Rückgabe:**
  `ArrVal`

---

## array.Chunk(arr, size)
- **Konkret:**
  Unterteilt ein 1D-Array in gleichgroße Teilarrays.
  Das letzte Chunk kann kleiner sein wenn die Gesamtlänge nicht aufgeht.
- **Parameter:**
  - `arr`: 1D-Array.
  - `size`: Maximale Größe je Teilarray.
- **Rückgabe:**
  `KindArr2D`

---

## array.Join(arr, sep)
- **Konkret:**
  Verbindet alle Elemente eines Arrays zu einem String.
- **Parameter:**
  - `arr`: 1D-Array.
  - `sep`: Trennzeichen.
- **Rückgabe:**
  `StrVal`

---

## array.CleanArray(array)
- **Konkret:**
  Trimmt alle String-Elemente und entfernt Steuerzeichen unter ASCII 32 (außer Tab).
  Funktioniert mit 1D- und 2D-Arrays.
- **Parameter:**
  - `array`: 1D- oder 2D-Array.
- **Rückgabe:**
  `ArrVal` oder `KindArr2D` (bereinigt)

---

## array.IsEmpty(array)
- **Konkret:**
  Prüft ob ein Array leer ist oder kein Array ist.
- **Parameter:**
  - `array`: Zu prüfender Wert.
- **Rückgabe:**
  `BoolVal`
  `true` wenn leer oder kein Array.

---

## array.KindOf(value)
- **Konkret:**
  Gibt den Datentyp eines Wertes als lesbaren String zurück.
- **Parameter:**
  - `value`: Beliebiger Wert.
- **Rückgabe:**
  `StrVal`
  Mögliche Werte: `"Zahl (Double)"`, `"Text (String)"`, `"Boolean"`, `"Null/Nothing"`, `"Array (1D)"`, `"Array (2D)"`, `"Objekt"`, `"Fehler-Objekt"`, `"unbekannter Typ"`.

---

## array.SortBy(arr, pattern [, desc])
- **Konkret:**
  Sortiert ein Array von Strings anhand eines per Regex extrahierten Schlüssels.
  Nutzt die erste Capture Group des Patterns als Sortierkriterium, gibt aber
  die originalen (unveränderten) Array-Elemente in sortierter Reihenfolge zurück.
  Elemente, bei denen das Pattern nicht matcht, werden immer ans Ende sortiert,
  unabhängig von der gewählten Richtung.
- **Parameter:**
  - `arr`: 1D-Array (Strings).
  - `pattern`: Go-Regex-Pattern mit mindestens einer Capture Group `()`.
  - `desc`: Optional. `BoolVal` – bei `true` absteigend (Z-A) sortieren (Standard: `false`, aufsteigend A-Z).
- **Rückgabe:**
  `ArrVal` (sortierte Kopie der Original-Elemente)
- **Beispiel:**
```vbx
  Dim blocks = array.Split(content, "---")
  Dim sorted = array.SortBy(blocks, "##\s+([A-Za-z0-9_.]+)\s*\(")
```