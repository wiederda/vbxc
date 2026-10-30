# 🌐 Global – Kern- & Hilfsfunktionen

Enthält grundlegende Typ-Konvertierungen, String-Operationen, Hashing, Kryptografie, Prozesssteuerung und Systemabfragen.
Kein Namespace-Präfix – alle Funktionen sind direkt aufrufbar.

---

## ToInt(value)
- **Konkret:**
  Wandelt einen Wert in eine Ganzzahl um. Nachkommastellen werden abgeschnitten.
  Komma als Dezimaltrennzeichen wird akzeptiert.
- **Parameter:**
  - `value`: Zahl oder String.
- **Rückgabe:**
  `NumVal`

---

## ToFloat(value)
- **Konkret:**
  Wandelt einen Wert in eine Fließkommazahl um.
  Komma als Dezimaltrennzeichen wird akzeptiert.
- **Parameter:**
  - `value`: Zahl oder String.
- **Rückgabe:**
  `NumVal`

---

## ToBool(value)
- **Konkret:**
  Prüft den Wahrheitsgehalt eines Wertes.
- **Parameter:**
  - `value`: Beliebiger Wert.
- **Rückgabe:**
  `BoolVal`

---

## ToString(value)
- **Konkret:**
  Wandelt einen beliebigen Wert in seine String-Repräsentation um.
- **Parameter:**
  - `value`: Beliebiger Wert.
- **Rückgabe:**
  `StrVal`

---

## Chr(code)
- **Konkret:**
  Gibt das Zeichen zum angegebenen Unicode-Codepoint zurück.
- **Parameter:**
  - `code`: Unicode-Wert als Zahl (z. B. `65` → `"A"`).
- **Rückgabe:**
  `StrVal`

---

## Asc(char)
- **Konkret:**
  Gibt den Unicode-Wert des ersten Zeichens eines Strings zurück.
- **Parameter:**
  - `char`: String (es wird nur das erste Zeichen ausgewertet).
- **Rückgabe:**
  `NumVal`

---

## Length(s)
- **Konkret:**
  Gibt die Anzahl der Zeichen (Runen) zurück.
  Zählt Unicode-Zeichen korrekt, nicht Bytes.
- **Parameter:**
  - `s`: String.
- **Rückgabe:**
  `NumVal`

---

## TrimStart(s, cutset)
- **Konkret:**
  Entfernt alle Zeichen aus `cutset` am Anfang des Strings.
- **Parameter:**
  - `s`: String.
  - `cutset`: Zu entfernende Zeichen.
- **Rückgabe:**
  `StrVal`

---

## TrimEnd(s, cutset)
- **Konkret:**
  Entfernt alle Zeichen aus `cutset` am Ende des Strings.
- **Parameter:**
  - `s`: String.
  - `cutset`: Zu entfernende Zeichen.
- **Rückgabe:**
  `StrVal`

---

## Trim(s, [cutset])
- **Konkret:**
  Entfernt Leerzeichen (oder angegebene Zeichen) an beiden Enden.
- **Parameter:**
  - `s`: String.
  - `cutset`: Optional. Zu entfernende Zeichen. Standard: Whitespace.
- **Rückgabe:**
  `StrVal`

---

## ToLower(s)
- **Konkret:**
  Wandelt einen String vollständig in Kleinschreibung um.
- **Parameter:**
  - `s`: String.
- **Rückgabe:**
  `StrVal`

---

## ToUpper(s)
- **Konkret:**
  Wandelt einen String vollständig in Großschreibung um.
- **Parameter:**
  - `s`: String.
- **Rückgabe:**
  `StrVal`

---

## Left(s, n)
- **Konkret:**
  Extrahiert die ersten `n` Zeichen eines Strings (Runen-basiert).
- **Parameter:**
  - `s`: String.
  - `n`: Anzahl der Zeichen.
- **Rückgabe:**
  `StrVal`

---

## Right(s, n)
- **Konkret:**
  Extrahiert die letzten `n` Zeichen eines Strings (Runen-basiert).
- **Parameter:**
  - `s`: String.
  - `n`: Anzahl der Zeichen.
- **Rückgabe:**
  `StrVal`

---

## Substring(s, start, [len])
- **Konkret:**
  Gibt einen Teilstring zurück (0-basierter Index, Runen-basiert).
- **Parameter:**
  - `s`: String.
  - `start`: Startposition (0-basiert).
  - `len`: Optional. Anzahl der Zeichen. Standard: bis Stringende.
- **Rückgabe:**
  `StrVal`

---

## Mid(text, start, [length])
- **Konkret:**
  Gibt einen Teilstring ab der angegebenen Position zurück (1-basiert, wie in klassischem VB, Runen-basiert wie `Left`/`Right`/`Substring`).
  Ohne `length`: alles ab `start` bis zum Ende.
  Liegt `start` hinter dem Stringende, wird ein leerer String zurückgegeben.
  Ist `length` `0` oder negativ, wird ebenfalls ein leerer String zurückgegeben.
  Reicht `length` über das Stringende hinaus, wird bis zum Ende abgeschnitten.
- **Parameter:**
  - `text`: Quellstring.
  - `start`: 1-basierte Startposition.
  - `length`: Optional. Anzahl der Zeichen. Standard: bis Stringende.
- **Rückgabe:**
  `StrVal`
  `ErrorVal` bei falscher Parameterzahl, ungültigem `start` (kein Zahlwert oder `< 1`) oder ungültigem `length` (kein Zahlwert) – prüfbar mit `IsError`/`ErrorText`.

---

## Contains(s, sub)
- **Konkret:**
  Prüft, ob ein Teilstring im Text enthalten ist.
  Die Suche ist case-insensitiv.
- **Parameter:**
  - `s`: String.
  - `sub`: Gesuchter Teilstring.
- **Rückgabe:**
  `BoolVal`

---

## InStr(s, search)
- **Konkret:**
  Sucht den Teilstring `search` innerhalb von `s` und gibt dessen 1-basierte Startposition zurück.
  Die Suche erfolgt Runen-basiert, damit Unicode-Zeichen korrekt behandelt werden.
  Ist `search` leer, wird (wie in klassischem VB) Position `1` zurückgegeben.
  Wird der Teilstring nicht gefunden, ist das Ergebnis `0`.
- **Parameter:**
  - `s`: Der zu durchsuchende Text.
  - `search`: Der gesuchte Teilstring.
- **Rückgabe:**
  `NumVal`
  1-basierte Position des ersten Vorkommens von `search` in `s`, oder `0`, wenn nicht gefunden.
  `ErrorVal`, wenn einer der Pflichtparameter fehlt – prüfbar mit `IsError`/`ErrorText`.

---

## IndexOf(s, sub)
- **Konkret:**
  Gibt den Index des ersten Vorkommens von `sub` zurück.
- **Parameter:**
  - `s`: String.
  - `sub`: Gesuchter Teilstring.
- **Rückgabe:**
  `NumVal`
  `-1`, wenn nicht gefunden.

---

## LastIndexOf(s, sub)
- **Konkret:**
  Gibt den Index des letzten Vorkommens von `sub` zurück.
- **Parameter:**
  - `s`: String.
  - `sub`: Gesuchter Teilstring.
- **Rückgabe:**
  `NumVal`
  `-1`, wenn nicht gefunden.

---

## Replace(s, find, repl)
- **Konkret:**
  Ersetzt alle Vorkommen von `find` durch `repl` im String.
- **Parameter:**
  - `s`: Quellstring.
  - `find`: Zu ersetzender Text.
  - `repl`: Ersatztext.
- **Rückgabe:**
  `StrVal`

---

## ReplaceMany(text, find1, repl1, find2, repl2, ...)
- **Konkret:**
  Ersetzt mehrere Teilstrings in einem Text in einem Aufruf.
- **Parameter:**
  - `text`: Quelltext.
  - `find, repl`: Beliebig viele Suchtext-/Ersetzungstext-Paare.
    Bei einer ungeraden Anzahl zusätzlicher Argumente wird das letzte Argument ignoriert.
- **Rückgabe:**
  `StrVal`

---

## Split(s, sep, [removeEmpty], [unique])
- **Konkret:**
  Zerlegt einen String anhand eines Trennzeichens in ein Array.
  Die einzelnen Teile werden getrimmt. Leere Elemente bleiben standardmäßig erhalten.
  Bei `sep = " "` wird `strings.Fields` verwendet: mehrere aufeinanderfolgende Leerzeichen zählen als ein Trennzeichen, leere Elemente werden automatisch vermieden.
  `removeEmpty` und `unique` können unabhängig voneinander oder gemeinsam verwendet werden.
- **Parameter:**
  - `s`: Quellstring.
  - `sep`: Trennzeichen.
  - `removeEmpty`: Optional. `BoolVal` – bei `true` werden leere Elemente entfernt.
  - `unique`: Optional. `BoolVal` – bei `true` werden Duplikate entfernt. Die Reihenfolge des ersten Auftretens bleibt erhalten.
- **Rückgabe:**
  `ArrVal`
- **Beispiel:**
```vbx
Split("a;b;;a;c;", ";")              ' ["a", "b", "", "a", "c", ""]
Split("a;b;;a;c;", ";", true)        ' ["a", "b", "a", "c"]
Split("a;b;;a;c;", ";", true, true)  ' ["a", "b", "c"]
```

---

## EncodeBase64(s)
- **Konkret:**
  Kodiert einen String als Base64.
- **Parameter:**
  - `s`: Quellstring.
- **Rückgabe:**
  `StrVal`

---

## DecodeBase64(s)
- **Konkret:**
  Dekodiert einen Base64-String zurück in Klartext.
- **Parameter:**
  - `s`: Base64-kodierter String.
- **Rückgabe:**
  `StrVal`
  Bei Fehler: `StrVal` mit Fehlerpräfix `"error: ..."`.

---

## URLEncode(val)
- **Konkret:**
  Kodiert einen String für die sichere Verwendung in URLs (Query-Escape).
- **Parameter:**
  - `val`: Zu kodierender String.
- **Rückgabe:**
  `StrVal`
- **Beispiel:**
```vbx
Print URLEncode("Hallo Welt!")
' Hallo+Welt%21
```

---

## URLDecode(urlStr)
- **Konkret:**
  Dekodiert einen URL-kodierten String zurück in Klartext.
- **Parameter:**
  - `urlStr`: URL-kodierter String.
- **Rückgabe:**
  `StrVal`

---

## PunyEncode(domain)
- **Konkret:**
  Kodiert eine Unicode-Domain in Punycode (IDNA2008/UTS#46).
- **Parameter:**
  - `domain`: Domain als Unicode-String (z. B. `"münchen.de"`).
- **Rückgabe:**
  `ArrVal`
  Format: `[OK, EncodedDomain, Msg]`

---

## PunyDecode(domain)
- **Konkret:**
  Dekodiert eine Punycode-Domain zurück in Unicode.
- **Parameter:**
  - `domain`: Punycode-Domain (z. B. `"xn--mnchen-3ya.de"`).
- **Rückgabe:**
  `ArrVal`
  Format: `[OK, DecodedDomain, Msg]`

---

## MD5(s)
- **Konkret:**
  Erzeugt einen MD5-Hash des übergebenen Textes.
- **Parameter:**
  - `s`: Quellstring.
- **Rückgabe:**
  `StrVal`
  Hex-kodierter Hash.

---

## SHA1(s)
- **Konkret:**
  Erzeugt einen SHA1-Hash (für Legacy-Kompatibilität).
- **Parameter:**
  - `s`: Quellstring.
- **Rückgabe:**
  `StrVal`
  Hex-kodierter Hash.

---

## SHA256(s)
- **Konkret:**
  Erzeugt einen SHA256-Hash des Textes.
- **Parameter:**
  - `s`: Quellstring.
- **Rückgabe:**
  `StrVal`
  Hex-kodierter Hash.

---

## SHA512(s)
- **Konkret:**
  Erzeugt einen SHA512-Hash des Textes.
- **Parameter:**
  - `s`: Quellstring.
- **Rückgabe:**
  `StrVal`
  Hex-kodierter Hash.

---

## Int([max])
- **Konkret:**
  Gibt eine zufällige Ganzzahl zurück.
  Ohne Parameter: beliebige Ganzzahl. Mit `max`: Wert von `0` bis `max-1`.
- **Parameter:**
  - `max`: Optional. Obere Grenze (exklusiv).
- **Rückgabe:**
  `NumVal`

---

## Input([prompt])
- **Konkret:**
  Liest eine Benutzereingabe von der Konsole.
  Zahlen werden automatisch als `NumVal` zurückgegeben.
- **Parameter:**
  - `prompt`: Optional. Anzuzeigender Text vor der Eingabe.
- **Rückgabe:**
  `NumVal` oder `StrVal` (automatische Erkennung).

---

## Password([prompt])
- **Konkret:**
  Liest eine Eingabe von der Konsole, ohne sie anzuzeigen (Echo unterdrückt).
  Falls stdin kein echtes Terminal ist (z. B. Pipe), wird auf normales Zeilenlesen zurückgefallen.
- **Parameter:**
  - `prompt`: Optional. Text, der vor der Eingabe angezeigt wird.
- **Rückgabe:**
  `StrVal`
  `ErrorVal` bei Lesefehler.

---

## Alert(text)
- **Konkret:**
  Gibt Text aus und wartet auf Bestätigung via ENTER.
- **Parameter:**
  - `text`: Anzuzeigender Text.
- **Rückgabe:**
  `BoolVal` (`true`)

---

## Confirm(text)
- **Konkret:**
  Zeigt Text an und wartet auf eine J/N-Eingabe.
  Akzeptiert: `j`, `ja`, `y`, `yes`.
- **Parameter:**
  - `text`: Anzuzeigender Text.
- **Rückgabe:**
  `BoolVal`

---

## Sleep(wert, [einheit], [showOutput])
- **Konkret:**
  Pausiert die Ausführung.
  Bei Sekunden/Minuten/Stunden wird ein Countdown angezeigt, der per ENTER übersprungen werden kann.
- **Parameter:**
  - `wert`: Zeitwert.
  - `einheit`: Optional. `"ms"`, `"s"`, `"m"`, `"h"` (Standard: `"ms"`).
  - `showOutput`: Optional. `BoolVal` – steuert die Countdown-Anzeige (Standard: `true`).
- **Rückgabe:**
  `NumVal` (übergebener Zeitwert)

---

## Wait(pid)
- **Konkret:**
  Pausiert das Skript, bis der Prozess mit der angegebenen PID beendet ist.
  Pollt in 250-ms-Intervallen.
- **Parameter:**
  - `pid`: Prozess-ID als Zahl.
- **Rückgabe:**
  `BoolVal`
  `true`, sobald der Prozess beendet ist oder nicht (mehr) existiert.

---

## Inspect(value)
- **Konkret:**
  Debug-Hilfsfunktion: Zeigt die oberste Struktur eines Wertes auf der Konsole (Typ, bei Maps alle Schlüssel mit Typ und Wert), ohne rekursiv in die Tiefe zu gehen.
  Bei Arrays wird nur das erste Element als Beispiel gezeigt, nicht jedes Element einzeln – nützlich, um schnell zu verstehen, welche Felder z. B. `sftp.List` oder `folder.FindDuplicates` zurückgeben, ohne sich durch hunderte identisch aufgebaute Einträge scrollen zu müssen.
- **Parameter:**
  - `value`: Beliebiger Wert.
- **Rückgabe:**
  `NullVal`
  Die eigentliche Ausgabe erfolgt direkt auf der Konsole, nicht als Rückgabewert.

---

## Format(expression, style)
- **Konkret:**
  Universelle Formatierung für Zahlen, Datumsangaben und Text.
- **Parameter:**
  - `expression`: Zahl oder Datumsstring.
  - `style`: Formatstring.
    - Stile (Zahlen):
      - `"0.00"` – Festkomma
      - `"Currency"` – Währung mit €
      - `"Percent"` – Prozent
      - `"Hex"` – Hexadezimal
    - Stile (Datum):
      - `"YYYY-MM-DD"` – ISO
      - `"DD.MM.YYYY"` – Deutsch
      - `"ddd"` – Wochentag (Mo, Di …)
      - `"dddd"` – Wochentag ausgeschrieben (Montag, Dienstag …)
      - `"MMM"` – Monatsname (Jan, Feb …)
      - `"MMMM"` – Monatsname ausgeschrieben (Januar, Februar …)
      - `"HH"` – Stunde (24h, zweistellig)
      - `"mm"` – Minute (zweistellig, **kleingeschrieben**)
      - `"ss"` / `"SS"` – Sekunde (zweistellig, beide Schreibweisen möglich)
- **Rückgabe:**
  `StrVal`
- **Beispiel:**
```vbx
Print Format(file.AccessTime(path), "YYYY-MM-DD HH:mm:ss")
' 2026-08-04 12:42:25
```
- **Hinweis:**
  `MM` (großgeschrieben) steht immer für den Monat, auch mehrfach im selben Formatstring.
  Für Zeitangaben `HH:mm:ss` verwenden, nicht `HH:MM:SS` – letzteres würde zweimal den Monat einsetzen statt Minute und Sekunde.

---

## PrintFormat()
- **Konkret:**
  Gibt eine Übersicht aller verfügbaren `Format()`-Optionen auf der Konsole aus.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `NullVal`

---

## FormatSize(bytes)
- **Konkret:**
  Wandelt Bytes in eine lesbare Größenangabe um (z. B. `"4.2 MB"`).
- **Parameter:**
  - `bytes`: Dateigröße als Zahl.
- **Rückgabe:**
  `StrVal`

---

## Default(val, default)
- **Konkret:**
  Gibt `default` zurück, wenn `val` `null` oder ein leerer String ist.
- **Parameter:**
  - `val`: Zu prüfender Wert.
  - `default`: Fallback-Wert.
- **Rückgabe:**
  Ursprünglicher oder Fallback-Wert.

---

## Compare(v1, v2)
- **Konkret:**
  Vergleicht zwei Versionsnummern segmentweise (Segmente durch `.` getrennt) als Ganzzahlen.
  Es werden beliebig viele Segmente unterstützt (z. B. `0.9.0` oder `1.2.3.4`). Fehlende Segmente zählen als `0`, d. h. `0.9.0` und `0.9.0.0` sind gleich.
  Ein führendes `v`/`V` (z. B. `v1.2.0`) sowie alles ab dem ersten `-`, `+` oder Leerzeichen (z. B. `-beta`) wird ignoriert.
  Segmente, die keine Zahl sind, zählen als `0`. Ein leerer String zählt als Version ohne Segmente (also `0`).
  Der Vergleich ist numerisch, nicht alphabetisch: `0.10.0` ist neuer als `0.9.0`.
- **Parameter:**
  - `v1`: Erste Version.
  - `v2`: Zweite Version.
- **Rückgabe:**
  `NumVal`
  `1`, wenn `v1` neuer ist als `v2`. `-1`, wenn `v1` älter ist. `0`, wenn beide gleich sind.
  `ErrorVal`, wenn weniger als zwei Parameter übergeben werden – prüfbar mit `IsError`/`ErrorText`.
- **Hinweis:**
  Typischer Einsatz ist der Update-Check gegen die Versionsnummer eines GitHub-Releases:
```vbx
If Compare(latestVersion, installedVersion) > 0 Then
    Print "Update verfügbar"
End If
```
  Ist keine Version installiert (leerer String), liefert `Compare` immer `1`, sofern die neue Version mindestens ein Segment größer als `0` hat. Eine Sonderbehandlung für "nicht installiert" ist dann nicht nötig.
  Pre-Release-Suffixe werden nicht bewertet: `1.0.0-beta` und `1.0.0` gelten als gleich.

---

## ConvNewLine(content, [target])
- **Konkret:**
  Normalisiert Zeilenumbrüche in einem String.
  Intern werden zunächst alle Varianten auf `\n` vereinheitlicht.
- **Parameter:**
  - `content`: String mit gemischten Zeilenumbrüchen.
  - `target`: Optional. `"lf"`, `"crlf"`, `"cr"` oder `"auto"` (OS-abhängig, Standard).
- **Rückgabe:**
  `StrVal`

---

## IsNumeric(v)
- **Konkret:**
  Prüft, ob ein Wert eine Zahl ist oder als Zahl interpretiert werden kann.
- **Parameter:**
  - `v`: Zu prüfender Wert.
- **Rückgabe:**
  `BoolVal`

---

## IsInteger(v)
- **Konkret:**
  Prüft, ob ein Wert eine Ganzzahl ohne Nachkommastellen ist.
- **Parameter:**
  - `v`: Zu prüfender Wert.
- **Rückgabe:**
  `BoolVal`

---

## IsPositive(n)
- **Konkret:**
  Prüft, ob `n > 0`.
- **Parameter:**
  - `n`: Zahl.
- **Rückgabe:**
  `BoolVal`

---

## IsNothing(b)
- **Konkret:**
  Prüft, ob ein Wert nicht initialisiert ist (`Undefined`, `Null`, `Nil`, `None`).
- **Parameter:**
  - `b`: Zu prüfender Wert.
- **Rückgabe:**
  `BoolVal`

---

## IsNull(val)
- **Konkret:**
  Prüft, ob ein Wert `Null`, `Nothing` oder nicht initialisiert ist (`KindNull`, `KindNil`, `KindNone`).
- **Parameter:**
  - `val`: Zu prüfender Wert.
- **Rückgabe:**
  `BoolVal`

---

## IsPrime(n)
- **Konkret:**
  Prüft, ob `n` eine Primzahl ist.
  Optimierter Algorithmus mit 6k±1-Schritten bis zur Quadratwurzel.
- **Parameter:**
  - `n`: Ganzzahl.
- **Rückgabe:**
  `BoolVal`

---

## IsArray(val)
- **Konkret:**
  Prüft, ob ein Wert ein Array ist (`KindArr` oder `KindArr2D`).
- **Parameter:**
  - `val`: Zu prüfender Wert.
- **Rückgabe:**
  `BoolVal`

---

## IsDate(val)
- **Konkret:**
  Prüft, ob ein String als Datum parsebar ist.
  Unterstützte Formate identisch zu `date.*`: ISO, Deutsch, US, RFC3339.
- **Parameter:**
  - `val`: Zu prüfender Wert (nur `StrVal` liefert `true`).
- **Rückgabe:**
  `BoolVal`

---

## IsString(val)
- **Konkret:**
  Prüft, ob der Wert ein String ist (`KindStr`).
- **Parameter:**
  - `val`: Zu prüfender Wert.
- **Rückgabe:**
  `BoolVal`

---

## IsMap(val)
- **Konkret:**
  Prüft, ob der Wert eine Map ist (`KindMap`).
- **Parameter:**
  - `val`: Zu prüfender Wert.
- **Rückgabe:**
  `BoolVal`

---

## Uptime()
- **Konkret:**
  Gibt die System-Laufzeit in Sekunden zurück.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `NumVal`

---

## UptimeString()
- **Konkret:**
  Gibt die System-Uptime als lesbaren Text zurück (z. B. `"2h 15m"`).
- **Parameter:**
  Keine.
- **Rückgabe:**
  `StrVal`

---

## SystemMemory()
- **Konkret:**
  Gibt Arbeitsspeicher-Informationen des Systems zurück.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `ArrVal`
  Format: `[Total, Available, UsedPercent]`
  Bytes für Total/Available, Prozent für UsedPercent.

---

## UserName()
- **Konkret:**
  Gibt den aktuellen Benutzernamen zurück.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `StrVal`

---

## ComputerName()
- **Konkret:**
  Gibt den Hostnamen des Rechners zurück.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `StrVal`

---

## UserDomain()
- **Konkret:**
  Gibt die Windows-Domäne zurück. Fallback auf den Hostnamen (Linux/macOS).
- **Parameter:**
  Keine.
- **Rückgabe:**
  `StrVal`

---

## Arch()
- **Konkret:**
  Gibt die Prozessor-Architektur zurück.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `StrVal`
  Beispiele: `"amd64"`, `"arm64"`.

---

## OS()
- **Konkret:**
  Gibt das Betriebssystem zurück.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `StrVal`
  Beispiele: `"windows"`, `"linux"`, `"darwin"`.

---

## ToClipboard(text)
- **Konkret:**
  Kopiert Text in die Zwischenablage.
  Windows: `clip`. macOS: `pbcopy`. Linux: `xclip`.
- **Parameter:**
  - `text`: Zu kopierender Text.
- **Rückgabe:**
  `BoolVal`
  `true` bei Erfolg. `ErrorVal`, wenn das Betriebssystem nicht unterstützt wird.

---

## FromClipboard()
- **Konkret:**
  Liest den aktuellen Textinhalt der Zwischenablage.
  Windows: `Get-Clipboard` (PowerShell). macOS: `pbpaste`. Linux: `xclip`.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `StrVal`
  Der gelesene Text. `ErrorVal`, wenn die Zwischenablage nicht gelesen werden kann oder das Betriebssystem nicht unterstützt wird.

---

## ClipboardHasText()
- **Konkret:**
  Prüft, ob aktuell Text in der Zwischenablage liegt.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `BoolVal`
  `true`, wenn Text vorhanden ist. `false` auch dann, wenn z. B. ein Bild statt Text in der Zwischenablage liegt.
  `ErrorVal`, wenn das Betriebssystem nicht unterstützt wird.

---

## ClipboardClear()
- **Konkret:**
  Leert die Zwischenablage.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `BoolVal`
  `true` bei Erfolg. `ErrorVal`, wenn das Betriebssystem nicht unterstützt wird.

---

## Unblock(path)
- **Konkret:**
  Hebt die NTFS-Sicherheitsblockierung (`Zone.Identifier` ADS) von Dateien auf.
  Auf Nicht-Windows-Systemen ist die Funktion ein No-op und gibt `0` zurück.
- **Parameter:**
  - `path`: Datei oder Verzeichnis.
- **Rückgabe:**
  `NumVal`
  Anzahl der entsperrten Dateien.

---

## GC()
- **Konkret:**
  Erzwingt den Go-Garbage-Collector, um ungenutzten Speicher sofort freizugeben.
- **Parameter:**
  Keine.
- **Rückgabe:**
  `NullVal`

---

## LoadModule(name)
- **Konkret:**
  Lädt ein optionales Modul (nativ oder als WASM-Plugin) zur Laufzeit, unabhängig von `#use`.
  War das Modul bereits geladen, wird sofort `True` zurückgegeben, ohne erneut zu laden.
- **Parameter:**
  - `name`: Modulname (z. B. `"zip"`, `"pgp"`, `"yaml"`).
- **Rückgabe:**
  `BoolVal` bei Erfolg, sonst `ErrorVal` (prüfbar mit `IsError`/`ErrorText`).

---

## Worker(path)
- **Konkret:**
  Startet ein `.vb`- oder `.vbc`-Skript als Hintergrundprozess.
  Stdout und Stderr werden in `<skriptpfad>.log` umgeleitet.
  Nur `.vb`- und `.vbc`-Dateien sind erlaubt.
- **Parameter:**
  - `path`: Pfad zum Skript.
- **Rückgabe:**
  `StrVal`
  PID des gestarteten Prozesses.

---

## WorkerPool(paths, maxParallel)
- **Konkret:**
  Startet mehrere `.vb`-/`.vbc`-Skripte über `Worker` mit einer Obergrenze an gleichzeitig laufenden Prozessen.
  Sobald ein Skript fertig ist, wird automatisch das nächste aus der Warteschlange gestartet.
  Kehrt erst zurück, wenn alle Skripte durchgelaufen sind.
- **Parameter:**
  - `paths`: Array von Skript-Pfaden.
  - `maxParallel`: Maximale Anzahl gleichzeitig laufender Worker. Werte `< 1` werden auf `1` gesetzt.
- **Rückgabe:**
  `ArrVal`
  Array von `BoolVal` (Erfolg pro Skript), in derselben Reihenfolge wie `paths`.
  `false` an einer Position, wenn das jeweilige Skript nicht gestartet werden konnte.
- **Beispiel:**
```vbx
DIM shares = {"dedup_share1.vbx", "dedup_share2.vbx", "dedup_share3.vbx", "dedup_share4.vbx"}
DIM results = WorkerPool(shares, 2)   ' nie mehr als 2 gleichzeitig

FOR EACH i, ok IN results
    IF ok THEN
        PRINT shares(i) & ": OK"
    ELSE
        PRINT shares(i) & ": Fehler beim Start"
    END IF
NEXT
```
- **Hinweis:**
  `WorkerPool` blockiert das aufrufende Skript, bis alle übergebenen Skripte durchgelaufen sind – anders als `Worker`, das sofort zurückkehrt.
  Für den Fire-and-Forget-Effekt aus Nutzersicht das aufrufende Skript wiederum selbst per `Worker` starten:
```vbx
' orchestrator.vbx
DIM shares = {"dedup_share1.vbx", "dedup_share2.vbx", "dedup_share3.vbx"}
DIM results = WorkerPool(shares, 2)
gotify.Send("NAS-Dedup über alle Shares abgeschlossen")
```
```vbx
Worker("orchestrator.vb")   ' aufrufendes Skript bekommt sofort die Kontrolle zurück
```

---

## Build(quelle)
- **Konkret:**
  Verschlüsselt ein `.vb`-Skript mit Magic-Header (`VBC!`) und speichert es als `.vbc`-Datei.
  Löst `#include`-Direktiven rekursiv auf. Erkennt zirkuläre Abhängigkeiten.
- **Parameter:**
  - `quelle`: Pfad zur `.vb`-Quelldatei.
- **Rückgabe:**
  `StrVal`
  Pfad der erzeugten `.vbc`-Datei.
- **Hinweis:**
  ⚠️ `.vbc` schützt vor beiläufigem Lesen, ist aber keine kryptografische Geheimhaltung – der Interpreter muss selbst entschlüsseln können, wer gezielt herankommen will, kann das umgehen.
  Keine Passwörter oder Secrets im Klartext ins Skript schreiben; stattdessen zur Laufzeit über Umgebungsvariablen, Docker Secrets oder Config-File einbinden.

---

## IsCompiled(path)
- **Konkret:**
  Prüft, ob eine Datei ein gültiges verschlüsseltes `.vbc`-Skript mit Magic-Header ist.
- **Parameter:**
  - `path`: Dateipfad.
- **Rückgabe:**
  `BoolVal`