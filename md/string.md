# 🔤 string.* – String-Operationen

Dient zur Verarbeitung, Analyse, Formatierung und Konvertierung von Strings.
Alle Funktionen sind Runen-basiert und damit korrekt für Unicode/UTF-8.

---

## string.HexEncode(val)
- **Konkret:**
  Wandelt einen String in seine Hex-Darstellung um.
- **Parameter:**
  - `val`: Quellstring.
- **Rückgabe:**
  `StrVal`
  Beispiel: `"ABC"` → `"414243"`.

---

## string.HexDecode(hexStr)
- **Konkret:**
  Wandelt einen Hex-String zurück in Text.
- **Parameter:**
  - `hexStr`: Hex-kodierter String.
- **Rückgabe:**
  `StrVal` bei Erfolg, `ErrorVal` bei ungültigem Hex.

---

## string.CharAt(str, index)
- **Konkret:**
  Gibt das Zeichen an einer bestimmten Position zurück (1-basiert, Runen-basiert).
  Gibt leeren String zurück wenn der Index außerhalb liegt.
- **Parameter:**
  - `str`: Quellstring.
  - `index`: Position (1-basiert).
- **Rückgabe:**
  `StrVal`

---

## string.Join(arr [, sep])
- **Konkret:**
  Verbindet alle Elemente eines Arrays zu einem String.
- **Parameter:**
  - `arr`: `ArrVal` mit zu verbindenden Elementen.
  - `sep`: Optional. Trennzeichen (Standard: `"\n"`).
- **Rückgabe:**
  `StrVal`

---

## string.StartsWith(s, prefix)
- **Konkret:**
  Prüft, ob ein String mit dem angegebenen Präfix beginnt.
- **Parameter:**
  - `s`: String.
  - `prefix`: Gesuchtes Präfix.
- **Rückgabe:**
  `BoolVal`

---

## string.EndsWith(s, suffix)
- **Konkret:**
  Prüft, ob ein String mit dem angegebenen Suffix endet.
- **Parameter:**
  - `s`: String.
  - `suffix`: Gesuchtes Suffix.
- **Rückgabe:**
  `BoolVal`

---

## string.Repeat(s, n)
- **Konkret:**
  Wiederholt einen String `n`-mal.
- **Parameter:**
  - `s`: String.
  - `n`: Anzahl Wiederholungen.
- **Rückgabe:**
  `StrVal`

---

## string.Reverse(s)
- **Konkret:**
  Kehrt die Reihenfolge aller Zeichen um (Runen-basiert).
- **Parameter:**
  - `s`: String.
- **Rückgabe:**
  `StrVal`

---

## string.PadLeft(s, n [, c])
- **Konkret:**
  Füllt einen String links auf eine Mindestlänge auf.
  Ist der String bereits lang genug, wird er unverändert zurückgegeben.
- **Parameter:**
  - `s`: String.
  - `n`: Zielbreite.
  - `c`: Optional. Füllzeichen (Standard: Leerzeichen).
- **Rückgabe:**
  `StrVal`

---

## string.PadRight(s, n [, c])
- **Konkret:**
  Füllt einen String rechts auf eine Mindestlänge auf.
  Ist der String bereits lang genug, wird er unverändert zurückgegeben.
- **Parameter:**
  - `s`: String.
  - `n`: Zielbreite.
  - `c`: Optional. Füllzeichen (Standard: Leerzeichen).
- **Rückgabe:**
  `StrVal`

---

## string.WordCount(s)
- **Konkret:**
  Zählt die Anzahl der Wörter in einem Text.
  Mehrfache Leerzeichen werden korrekt behandelt.
- **Parameter:**
  - `s`: String.
- **Rückgabe:**
  `NumVal`

---

## string.CharCount(s)
- **Konkret:**
  Zählt alle Zeichen im Text außer Leerzeichen. Buchstaben, Ziffern sowie Satz- und Sonderzeichen zählen mit.
- **Parameter:**
  - `s`: String.
- **Rückgabe:**
  `NumVal`

---

## string.Between(s, start, end)
- **Konkret:**
  Extrahiert den Text zwischen zwei Markern (erste Fundstelle).
  Gibt leeren String zurück wenn ein Marker nicht gefunden wird.
- **Parameter:**
  - `s`: Quellstring.
  - `start`: Linker Marker.
  - `end`: Rechter Marker.
- **Rückgabe:**
  `StrVal`

---

## string.Val(n)
- **Konkret:**
  Extrahiert den numerischen Wert am Anfang eines Strings (von links).
  Akzeptiert Vorzeichen und einen Dezimalpunkt.
  Bricht beim ersten nicht-numerischen Zeichen ab.
- **Parameter:**
  - `n`: String mit führendem Zahlenwert.
- **Rückgabe:**
  `NumVal`
  Beispiel: `"42px"` → `42`, `"abc"` → `0`.

---

## string.CleanLines(s)
- **Konkret:**
  Trimmt Leerzeichen und Tabs am Ende jeder Zeile.
  Leerzeilen bleiben erhalten.
  Führende und abschließende Leerzeilen werden entfernt.
- **Parameter:**
  - `s`: String.
- **Rückgabe:**
  `StrVal`

---

## string.CleanAllLines(s)
- **Konkret:**
  Entfernt alle Leerzeilen und trimmt verbleibende Zeilen vollständig.
  Ergebnis ist ein kompakter Block ohne Leerzeilen.
- **Parameter:**
  - `s`: String.
- **Rückgabe:**
  `StrVal`

---

## string.TrimLines(content)
- **Konkret:**
  Entfernt Leerzeichen, Tabs und `\r` am Ende jeder einzelnen Zeile.
  Leerzeilen bleiben erhalten.
- **Parameter:**
  - `content`: String.
- **Rückgabe:**
  `StrVal`

---

## string.RegExp(s, pattern)
- **Konkret:**
  Führt einen regulären Ausdruck aus und gibt alle Treffer inklusive Capture Groups zurück.
  Index 0 = vollständiger Treffer, Index 1+ = Capture Groups.
- **Parameter:**
  - `s`: Quellstring.
  - `pattern`: Go-Regex-Pattern.
- **Rückgabe:**
  `ArrVal`
  Leeres Array wenn kein Treffer. `ErrorVal` bei ungültigem Pattern.

---

## string.PatternFromExample(example, exact, position [, keep])
- **Konkret:**
  Leitet aus einem Beispielstring ein Regex-Muster ab. Ziffernfolgen werden zu `\d`, Buchstabenfolgen zu `[a-zA-Z]` generalisiert, Sonderzeichen (`_`, `.`, `-` etc.) bleiben literal und werden bei Bedarf escaped.
- **Parameter:**
  - `example`: Beispieltext, aus dem das Muster abgeleitet wird.
  - `exact`: `Bool`. `true` generalisiert mit exakter Länge (`\d{8}`, `[a-zA-Z]{3}`), `false` mit beliebiger Länge (`\d+`, `[a-zA-Z]+`).
  - `position`: `"start"` (Muster wird mit `^` verankert), `"end"` (Muster wird mit `$` verankert) oder `"anywhere"` (kein Anker).
  - `keep`: Optional. Literaler Teilstring innerhalb von `example`, der NICHT generalisiert werden soll (z. B. `"IMG"`, `"Bericht"`). Kommt der Teilstring in `example` nicht vor, wird er ignoriert.
- **Rückgabe:**
  `StrVal` mit dem generierten Regex-Muster. `ErrorVal` bei ungültigem `position`-Wert.
  Beispiel: `string.PatternFromExample("_20210910_0952", true, "end")` → `"_\d{8}_\d{4}$"`.

---

## string.CombinePatterns(patterns)
- **Konkret:**
  Kombiniert mehrere Regex-Muster (Array von Strings) per Oder-Verknüpfung (`|`) zu einem einzigen Muster. Jedes Teilmuster wird in eine nicht-capturing Gruppe `(?:...)` geklammert, damit sich Anker (`^`/`$`) und Vorrangregeln der einzelnen Muster nicht gegenseitig stören.
- **Parameter:**
  - `patterns`: `ArrVal` mit einzelnen Regex-Mustern, z. B. erzeugt per `string.PatternFromExample`.
- **Rückgabe:**
  `StrVal` mit dem kombinierten Muster. Leeres Array liefert einen leeren String zurück. `ErrorVal` wenn `patterns` kein Array ist.
  Beispiel: `["_\d{8}_\d{4}$", "_old$"]` → `"(?:_\d{8}_\d{4}$)|(?:_old$)"`.
  Kann direkt an `string.RegExp` übergeben werden.

---

## string.Extract(text, pattern)
- **Konkret:**
  Extrahiert den Wert der ersten Capture Group eines Regex-Patterns.
  Gibt leeren String zurück wenn kein Treffer oder keine Gruppe.
- **Parameter:**
  - `text`: Quellstring.
  - `pattern`: Go-Regex-Pattern mit mindestens einer Capture Group `()`.
- **Rückgabe:**
  `StrVal`

---

## string.WipeString(value)
- **Konkret:**
  Überschreibt den tatsächlichen Speicherinhalt eines Strings mit Nullbytes (`0x00`). Im Unterschied zu `x = ""` wird hier nicht nur die Variable auf einen neuen String umgebogen, sondern der ursprüngliche Speicherbereich selbst gelöscht.
- **Parameter:**
  - `value`: Ein String (`KindStr`).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei falschem Argumenttyp.
- **Hinweis:**
  Nutzt intern `unsafe`, um Go's String-Immutability bewusst zu umgehen. Nach dem Aufruf hat die Variable weiterhin die ursprüngliche Länge, besteht aber nur noch aus Nullbytes (verifiziert: `Len()` bleibt bei Umlauten sogar größer als die sichtbare Zeichenzahl, weil einzelne Mehrbyte-UTF-8-Sequenzen durch mehrere Einzel-Nullbytes ersetzt werden).

  **Wichtige Einschränkungen:**
  - Löscht nur den Speicherbereich, auf den *dieser eine* String zeigt. Wurde der Wert vorher verkettet (`&`), umgewandelt oder anderweitig zu einem neuen String verarbeitet, hat diese Kopie ein eigenes Backing-Array und bleibt von `string.WipeString` unberührt.
  - Sollte immer der *letzte* Umgang mit dem Wert sein – jede Nutzung danach liest nur noch Nullbytes.
  - Nur für dynamisch zur Laufzeit erzeugte Strings (z. B. via `crypt.BytesToString`) sicher. **Niemals** auf String-Literale aus dem Quellcode anwenden (z. B. `Dim x = "MeinPasswort"` gefolgt von `string.WipeString(x)`) – Go kann identische String-Literale intern deduplizieren/teilen, wodurch ein Wipe unabsichtlich andere, unabhängige Stellen im Programm beschädigen könnte.

---

## string.Wipe(byteArray)
- **Konkret:**
  Überschreibt ein Byte-Array (z. B. von `reg.ReadProtectedValueBytes`) in-place mit Nullen.
- **Parameter:**
  - `byteArray`: Ein Array vom Typ `KindArr` mit Byte-Werten (0–255).
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei falschem Argumenttyp.
- **Hinweis:**
  Arrays sind in VBX mutable (im Gegensatz zu Strings). `string.Wipe` nutzt das aus und überschreibt den tatsächlichen Speicherinhalt, nicht nur eine Kopie – verifiziert per Test: Nach dem Aufruf sind alle Elemente der Original-Variable `0`.
  Sollte aufgerufen werden, sobald ein per `reg.ReadProtectedValueBytes` gelesener Wert nicht mehr benötigt wird.

---

## string.InsertAfterText(text, suchtext, einfügen)
- **Konkret:**
  Fügt in jeder Zeile direkt nach der ersten Fundstelle des Suchtextes den angegebenen Text ein und gibt das Ergebnis als neuen String zurück.
  Nur das erste Vorkommen pro Zeile wird berücksichtigt, weitere Treffer in derselben Zeile bleiben unverändert.
  Arbeitet komplett im Speicher (keine Datei), Gegenstück zu `file.InsertAfter`.
- **Parameter:**
  - `text`: Quelltext.
  - `suchtext`: Suchmuster (Substring). Darf nicht leer sein.
  - `einfügen`: Text, der direkt nach dem Suchtext eingefügt wird.
- **Rückgabe:**
  `StrVal`, `ErrorVal` bei leerem Suchtext oder fehlenden Parametern.
- **Hinweis:**
  Zeilenumbrüche werden vor der Verarbeitung normalisiert (`\r\n`/`\r` → `\n`) — das Ergebnis hat daher immer `\n`-Zeilenenden, auch wenn `text` ursprünglich `\r\n` verwendet hat.

---

## string.ExtractMax(text, pattern)
- **Konkret:**
  Findet alle Treffer der ersten Capture Group und gibt den höchsten Wert zurück.
  Sortierung nach Länge, dann lexikografisch (geeignet für Versionsnummern).
- **Parameter:**
  - `text`: Quellstring.
  - `pattern`: Go-Regex-Pattern mit mindestens einer Capture Group.
- **Rückgabe:**
  `StrVal`
  Höchster gefundener Wert, oder leerer String wenn kein Treffer.

---

## string.Like(text, pattern)
- **Konkret:**
  Einfacher Mustervergleich im VB-Stil. Vollständiger Match (Anfang bis Ende).
- **Parameter:**
  - `text`: Quellstring.
  - `pattern`: Muster mit folgenden Wildcards:
    - `#` – eine Ziffer
    - `?` – ein beliebiges Zeichen
    - `*` – beliebig viele Zeichen
- **Rückgabe:**
  `BoolVal`

---

## string.StrComp(s1, s2 [, mode])
- **Konkret:**
  Vergleicht zwei Strings lexikografisch.
- **Parameter:**
  - `s1`: Erster String.
  - `s2`: Zweiter String.
  - `mode`: Optional. `0` = case-sensitive (Standard), `1` = case-insensitive.
- **Rückgabe:**
  `NumVal`
  `-1` wenn s1 < s2, `0` wenn gleich, `1` wenn s1 > s2.

---

## string.CompareText(s1, s2)
- **Konkret:**
  Vergleicht zwei Strings immer case-insensitiv.
  Kurzform von `string.StrComp(s1, s2, 1)`.
- **Parameter:**
  - `s1`: Erster String.
  - `s2`: Zweiter String.
- **Rückgabe:**
  `NumVal`
  `-1`, `0` oder `1`.

---

## string.Choose(n, v...)
- **Konkret:**
  Gibt das Element an Position `n` aus einer Liste zurück (1-basiert).
  Gibt `Undefined` zurück wenn der Index außerhalb liegt.
- **Parameter:**
  - `n`: Index (1-basiert).
  - `v...`: Beliebig viele Auswahloptionen.
- **Rückgabe:**
  Gewählter Wert oder `KindUndefined`.

---

## string.Switch(v...)
- **Konkret:**
  Wertet Bedingung-Wert-Paare aus und gibt den Wert des ersten wahren Paares zurück.
  Erwartet eine gerade Anzahl an Argumenten.
  Gibt `Undefined` zurück wenn keine Bedingung zutrifft.
- **Parameter:**
  - `v...`: Abwechselnd Bedingung und Wert (z. B. `cond1, val1, cond2, val2`).
- **Rückgabe:**
  Erster Wert dessen Bedingung `true` ist, oder `KindUndefined`.

---

## string.StrConv(input)
- **Konkret:**
  Konvertiert einen String sicher nach UTF-8.
  Ungültige Byte-Sequenzen werden durch `U+FFFD` (Replacement Character) ersetzt.
- **Parameter:**
  - `input`: String mit potenziell ungültigen Sequenzen.
- **Rückgabe:**
  `StrVal` (valides UTF-8).

---

## string.RemoveLine(text, suchtext|zeile [, all])
- **Konkret:**
  Entfernt eine oder mehrere Zeilen aus einem mehrzeiligen Text. Der zweite
  Parameter bestimmt den Modus: eine **Zahl** entfernt die Zeile an diesem
  nullbasierten Index; ein **String** entfernt jede Zeile, die diesen Text
  enthält. Das ursprüngliche Zeilenende-Format (`\r\n` unter Windows, `\n`
  unter Linux/macOS, `\r` bei sehr alten Mac-Dateien) wird intern erkannt und
  für die Rückgabe beibehalten. Bei Dateien mit **gemischten** Zeilenenden
  gewinnt der zuerst im Text gefundene Stil für die gesamte Ausgabe.
  Bei Textsuche wird standardmäßig nur die **erste** Treffer-Zeile entfernt;
  mit `all = True` werden alle passenden Zeilen entfernt. Bei Zeilennummer
  spielt `all` keine Rolle (es gibt nur eine Zeile mit diesem Index).
  Existiert die Zeilennummer nicht (außerhalb des gültigen Bereichs) oder ist
  der Suchtext leer, wird der Text unverändert zurückgegeben.
- **Parameter:**
  - `text`: Mehrzeiliger String.
  - `suchtext|zeile`: String (Text, der in der zu entfernenden Zeile enthalten
    sein muss) oder Zahl (nullbasierter Zeilenindex).
  - `all` *(optional)*: Boolean – bei Textsuche `True`, um alle Treffer zu
    entfernen statt nur den ersten (Default: `False`). Bei Zeilennummer ohne
    Wirkung.
- **Rückgabe:**
  `StrVal` – der Text ohne die entfernte(n) Zeile(n), mit demselben
  Zeilenende-Format wie der ursprüngliche `text`.

---

## string.TxtToSqlInsert(data, table, dialect [, extras, columns, batchSize])
- **Konkret:**
  Generiert SQL-INSERT-Statements aus einem Array.
  Umschließt die Ausgabe mit Transaktionsblöcken je nach Dialekt.
  Werte werden automatisch escaped (`'` → `''`).
- **Parameter:**
  - `data`: `ArrVal` mit einzufügenden Werten (ein Wert pro Zeile).
  - `table`: Zielterbellenname.
  - `dialect`: SQL-Dialekt. Unterstützt: `"sqlite"`, `"postgres"`, `"mssql"`.
  - `extras`: Optional. `ArrVal` – zusätzliche Spalten-Werte pro Zeile.
  - `columns`: Optional. `ArrVal` – Spaltennamen für den INSERT-Header.
  - `batchSize`: Optional. `NumVal` – Anzahl Zeilen pro INSERT-Statement (0 = kein Batching).
- **Rückgabe:**
  `StrVal`
  Fertiges SQL-Script.

---

## string.Space(n)
- **Konkret:**
  Gibt einen String mit `n` Leerzeichen zurück.
- **Parameter:**
  - `n`: Anzahl Leerzeichen. Negative Werte ergeben einen leeren String.
- **Rückgabe:**
  `StrVal`

---

## string.StrDup(n, char)
- **Konkret:**
  Gibt ein einzelnes Zeichen `n`-mal wiederholt zurück.
  Wird ein mehrzeichen String übergeben, wird nur das erste Zeichen verwendet.
- **Parameter:**
  - `n`: Anzahl Wiederholungen. Negative Werte ergeben einen leeren String.
  - `char`: Zu wiederholendes Zeichen.
- **Rückgabe:**
  `StrVal`
  Beispiel: `string.StrDup(5, "*")` → `"*****"`.

---

## string.Base32Encode(val)
- **Konkret:**
  Wandelt Text in Base32 um (Standard-Alphabet, RFC 4648, mit Padding).
- **Parameter:**
  - `val`: Zu kodierender Text.
- **Rückgabe:**
  `StrVal`
  Beispiel: `"Hello"` → `"JBSWY3DP"`.

---

## string.Base32Decode(base32Str)
- **Konkret:**
  Wandelt einen Base32-String zurück in Text.
- **Parameter:**
  - `base32Str`: Base32-kodierter String (Standard-Alphabet mit Padding).
- **Rückgabe:**
  `StrVal`, `ErrorVal` bei ungültigem Base32-String.