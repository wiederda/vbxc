# 📋 json.* – JSON-Funktionen

Dient zum Laden, Lesen, Schreiben, Suchen und Konvertieren von JSON-Daten.
Arbeitet mit einem internen Dokumenten-Zustand – `json.Load` oder `json.ParseToEngine` muss vor `Get`/`Set`/`Delete` aufgerufen werden.
Pfade werden mit Punkt-Notation angegeben. Array-Zugriff über numerischen Index: `users.0.name`.
Thread-sicher via `sync.RWMutex`.

---

## json.Load(path)
- **Konkret:**
  Lädt eine JSON-Datei in den internen Arbeitsspeicher.
- **Parameter:**
  - `path`: Pfad zur JSON-Datei.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `ErrorVal` bei Fehler.

---

## json.Save([path])
- **Konkret:**
  Speichert den aktuellen Zustand atomar in die Datei (temp-Datei + Rename).
  Mit Pfadangabe wird der neue Pfad als Ziel gesetzt.
  Ohne Pfad wird der zuletzt geladene Pfad verwendet.
- **Parameter:**
  - `path`: Optional. Neuer Zielpfad.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `ErrorVal` bei Fehler.

---

## json.Parse(jsonString)
- **Konkret:**
  Prüft einen JSON-String auf syntaktische Korrektheit ohne ihn zu laden.
- **Parameter:**
  - `jsonString`: JSON als String.
- **Rückgabe:**
  `BoolVal` (`true`) wenn valide, `ErrorVal` bei Syntaxfehler.

---

## json.ParseToEngine(jsonStr)
- **Konkret:**
  Lädt einen JSON-String direkt in den internen Arbeitsspeicher.
  Alternative zu `json.Load` wenn die Daten nicht aus einer Datei kommen.
- **Parameter:**
  - `jsonStr`: JSON als String (muss ein Objekt `{}` sein).
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `ErrorVal` bei Fehler.

---

## json.Get(path)
- **Konkret:**
  Liest einen Wert aus der geladenen JSON-Struktur.
  Unterstützt Dot-Notation für verschachtelte Objekte und numerische Indizes für Arrays.
- **Parameter:**
  - `path`: Pfad zum Wert (z. B. `"user.address.city"`, `"users.0.name"`).
- **Rückgabe:**
  Entsprechender `Value`-Typ. Leerer Value wenn Pfad nicht gefunden.

---

## json.Set(path, value)
- **Konkret:**
  Setzt einen Wert in der geladenen Struktur.
  Fehlende Zwischenknoten werden automatisch als leere Maps angelegt.
- **Parameter:**
  - `path`: Pfad zum Zielschlüssel.
  - `value`: Zu setzender Wert.
- **Rückgabe:**
  `StrVal` (`"OK"`), `ErrorVal` bei Fehler.

---

## json.Delete(path)
- **Konkret:**
  Entfernt einen Schlüssel aus der geladenen Struktur.
  Existiert der Pfad nicht, wird kein Fehler zurückgegeben.
- **Parameter:**
  - `path`: Pfad zum zu löschenden Schlüssel.
- **Rückgabe:**
  `StrVal` (`"OK"`), `ErrorVal` bei Fehler.

---

## json.RenameKey(path, alterSchlüssel, neuerSchlüssel)
- **Konkret:**
  Benennt einen Schlüssel in einem JSON-Objekt an einem Pfad um, der zugehörige Wert bleibt erhalten.
  Sind `alterSchlüssel` und `neuerSchlüssel` identisch, wird nichts verändert, aber trotzdem Erfolg zurückgegeben.
- **Parameter:**
  - `path`: Pfad zum Objekt, das den umzubenennenden Schlüssel enthält.
  - `alterSchlüssel`: Bisheriger Schlüsselname.
  - `neuerSchlüssel`: Neuer Schlüsselname.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg. `ErrorVal` wenn Schlüssel leer sind, der Pfad nicht gefunden wurde, der Pfad kein Objekt ist, oder `alterSchlüssel` nicht existiert.

---

## json.Exists(path)
- **Konkret:**
  Prüft ob ein Schlüssel in der geladenen Struktur vorhanden ist.
- **Parameter:**
  - `path`: Zu prüfender Pfad.
- **Rückgabe:**
  `BoolVal`

---

## json.Keys(path)
- **Konkret:**
  Gibt alle Schlüsselnamen eines Objekts an einem Pfad zurück (alphabetisch sortiert).
- **Parameter:**
  - `path`: Pfad zum Objekt.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen. Leer wenn Pfad nicht gefunden oder kein Objekt.

---

## json.PropertyCount(objekt)
- **Konkret:**
  Gibt die Anzahl der Top-Level-Eigenschaften eines JSON-Objekts zurück.
  Verschachtelte Objekte werden nicht rekursiv mitgezählt – nur die direkten Schlüssel.
- **Parameter:**
  - `objekt`: Ein JSON-Objekt (Map), typischerweise aus `json.FromJSON` oder `json.Get`.
- **Rückgabe:**
  `NumVal` – Anzahl der Top-Level-Eigenschaften. `ErrorVal`, falls `objekt` kein JSON-Objekt ist (z. B. ein Array).

---

## json.Append(path, value)
- **Konkret:**
  Fügt einen Wert an ein Array in der geladenen Struktur an.
  Existiert kein Array am Pfad, wird ein neues erstellt.
- **Parameter:**
  - `path`: Pfad zum Array.
  - `value`: Anzuhängender Wert.
- **Rückgabe:**
  `StrVal` (`"OK"`), `ErrorVal` bei Fehler.

---

## json.ToArray(path)
- **Konkret:**
  Liest ein Array aus der geladenen Struktur und gibt es als `ArrVal` zurück.
- **Parameter:**
  - `path`: Pfad zum Array.
- **Rückgabe:**
  `ArrVal`. Leer wenn Pfad nicht gefunden oder kein Array.

---

## json.Merge(target, source)
- **Konkret:**
  Kopiert alle Felder von `source` in `target` (flaches Mergen, In-Place).
  Source-Keys überschreiben gleichnamige Target-Keys.
  Verschachtelte Objekte werden nicht tief zusammengeführt.
- **Parameter:**
  - `target`: `KindMap`.
  - `source`: `KindMap`.
- **Rückgabe:**
  `KindMap` (das modifizierte `target`).

---

## json.SetInObject(obj, key, value)
- **Konkret:**
  Setzt direkt ein Feld in einem übergebenen Map-Objekt (ohne den Engine-Zustand zu nutzen).
- **Parameter:**
  - `obj`: `KindMap`.
  - `key`: Schlüsselname.
  - `value`: Zu setzender Wert.
- **Rückgabe:**
  `StrVal` (`"OK"`), `ErrorVal` wenn `obj` keine Map ist.

---

## json.FromJSON(jsonString)
- **Konkret:**
  Wandelt einen JSON-String in eine interne Map- oder Array-Struktur um.
  Unabhängig vom Engine-Zustand – kein `Load` nötig.
- **Parameter:**
  - `jsonString`: JSON als String.
- **Rückgabe:**
  `KindMap` oder `ArrVal`, `ErrorVal` bei ungültigem JSON.

---

## json.ToJSON(data)
- **Konkret:**
  Konvertiert eine interne Map- oder Array-Struktur in einen JSON-String.
- **Parameter:**
  - `data`: `KindMap` oder `ArrVal`.
- **Rückgabe:**
  `StrVal` (kompakter JSON-String), `ErrorVal` bei Fehler.

---

## json.Query(path, filter)
- **Konkret:**
  Durchsucht ein Array in der geladenen Struktur nach Objekten die ein Kriterium erfüllen.
  Filter-Format: `"key=value"`.
- **Parameter:**
  - `path`: Pfad zum Array.
  - `filter`: Filterbedingung als String.
- **Rückgabe:**
  `ArrVal` aller Treffer.

---

## json.Search(jsonStr, key)
- **Konkret:**
  Durchsucht einen JSON-String rekursiv nach dem **ersten** Vorkommen eines Schlüssels.
  Unabhängig vom Engine-Zustand.
- **Parameter:**
  - `jsonStr`: JSON als String.
  - `key`: Gesuchter Schlüsselname.
- **Rückgabe:**
  `StrVal` (Wert des ersten Treffers). Leerer String wenn nicht gefunden.

---

## json.SearchAll(jsonStr, key)
- **Konkret:**
  Durchsucht einen JSON-String rekursiv nach **allen** Vorkommen eines Schlüssels.
  Unabhängig vom Engine-Zustand.
- **Parameter:**
  - `jsonStr`: JSON als String.
  - `key`: Gesuchter Schlüsselname.
- **Rückgabe:**
  `ArrVal` aller gefundenen Werte.

---

## json.SearchWhereGet(jsonStr, returnKey, cond1, cond2, ...)
- **Konkret:**
  Findet das erste Objekt in einem Array das alle Bedingungen erfüllt und gibt einen bestimmten Key zurück.
  Bedingungen: `"key=value"` oder `"key:value"`. Vergleich ist case-insensitiv.
  Sucht nur auf der obersten Array-Ebene (keine tiefe Rekursion in Sub-Objekte).
- **Parameter:**
  - `jsonStr`: JSON als String.
  - `returnKey`: Key dessen Wert zurückgegeben wird.
  - `cond1, cond2, ...`: Filterbedingungen.
- **Rückgabe:**
  `StrVal` (Wert des gesuchten Keys). Leerer String wenn kein Treffer.

---

## json.SearchDeepWhereGetContains(jsonStr, returnKey, cond1, cond2, ...)
- **Konkret:**
  Erweiterung von [json.SearchDeepWhereGet](/path/to/md/json.md) um Teilstring-Bedingungen.
  Durchsucht ein JSON-Dokument rekursiv über alle Verschachtelungsebenen nach dem ersten Objekt, das alle Bedingungen erfüllt, und gibt den Wert von `returnKey` zurück.
  Bedingungen aus übergeordneten Objekten werden vererbt (ein Kind überschreibt gleichnamige Werte des Elternobjekts). Die Bedingungen dürfen sich daher auf mehrere Ebenen verteilen.
  Für Bedingungen zählen nur String-Werte (Zahlen, Booleans und `null` werden nicht verglichen). Alle Vergleiche sind case-insensitiv, der zurückgegebene Wert behält seine Originalschreibweise.
  Trifft ein Objekt zu, enthält aber `returnKey` nicht, wird in den Kindobjekten weitergesucht.
- **Parameter:**
  - `jsonStr`: JSON als String.
  - `returnKey`: Key dessen Wert beim ersten Treffer zurückgegeben wird.
  - `cond1, cond2, ...`: Filterbedingungen (alle müssen erfüllt sein):
    - `"key=value"` oder `"key:value"`: exakter Vergleich (`:` gilt nur, wenn die Bedingung kein `=` enthält)
    - `"key~teil"`: Teilstring-Suche (enthält)
    Getrennt wird am ersten Trennzeichen, der Wert darf also weitere enthalten (`"zeit:12:30"` ergibt Key `zeit`, Wert `12:30`). Enthält eine Bedingung ein `~`, hat es Vorrang vor `=` und `:`.
- **Rückgabe:**
  `StrVal` (Wert von `returnKey` beim ersten Treffer, Zahlen und verschachtelte Werte werden in einen String konvertiert). Leerer String wenn kein passendes Objekt gefunden wurde. `ErrorVal` bei ungültigem JSON, leerem `jsonStr`/`returnKey`, ungültiger Bedingung oder weniger als 2 Argumenten.

---

## json.SearchWhereExists(jsonStr, cond1, cond2, ...)
- **Konkret:**
  Prüft ob mindestens ein Objekt im JSON alle Bedingungen erfüllt.
  Bedingungen: `"key=value"` oder `"key:value"`. Vergleich ist case-insensitiv.
- **Parameter:**
  - `jsonStr`: JSON als String.
  - `cond1, cond2, ...`: Filterbedingungen.
- **Rückgabe:**
  `BoolVal`

---

## json.SearchDeepWhereGet(jsonStr, returnKey, cond1, cond2, ...)
- **Konkret:**
  Durchsucht ein JSON-Dokument rekursiv über alle Verschachtelungsebenen nach einem Objekt, das alle angegebenen Bedingungen erfüllt, und gibt den Wert des angegebenen Rückgabeschlüssels zurück.
  Im Unterschied zu [json.SearchWhereGet](/path/to/md/json.md) werden Bedingungen aus übergeordneten Objekten vererbt: ein verschachteltes Objekt muss die Bedingungen nicht selbst enthalten, solange ein Elternobjekt sie erfüllt. Der Vergleich ist case-insensitiv.
- **Parameter:**
  - `jsonStr`: JSON als String.
  - `returnKey`: Key dessen Wert beim ersten Treffer zurückgegeben wird.
  - `cond1, cond2, ...`: Filterbedingungen im Format `"key=value"`.
- **Rückgabe:**
  `StrVal` (Wert von `returnKey` beim ersten Treffer). Leerer String wenn kein passendes Objekt gefunden wurde. `ErrorVal` bei ungültigem JSON oder fehlenden Pflichtargumenten.

---

## json.FindKey(path, value)
- **Konkret:**
  Durchsucht ein JSON-Objekt an einem gegebenen Pfad nach einem bestimmten Wert und gibt den dazugehörigen Schlüssel zurück.
  Der Wertevergleich ist case-sensitiv. Nur die oberste Ebene des Objekts wird durchsucht (keine Rekursion).
- **Parameter:**
  - `path`: Pfad zum zu durchsuchenden Objekt.
  - `value`: Gesuchter Wert (wird intern über `anyToStr` in einen String konvertiert und verglichen).
- **Rückgabe:**
  `StrVal` (Name des ersten Schlüssels mit passendem Wert). Leerer String wenn kein Treffer. `ErrorVal` wenn der Pfad nicht existiert oder kein Objekt ist.