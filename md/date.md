# 📅 date.* – Datums- & Zeitfunktionen

Dient zur Verarbeitung, Berechnung und Formatierung von Datums- und Zeitangaben.
Datum-Strings werden automatisch erkannt – unterstützte Formate: ISO (`2006-01-02`), Deutsch (`02.01.2006`), US (`01-02-2006`), RFC3339, jeweils mit und ohne Uhrzeit.
Standard-Ausgabeformat: `DD.MM.YYYY HH:mm:ss`.

---

## Datums-Teile (format_or_part)

Viele Funktionen akzeptieren einen optionalen `format_or_part`-Parameter:

- `"year"`, `"month"`, `"day"`, `"hour"`, `"minute"`, `"second"` – gibt den entsprechenden Zahlenwert zurück.
- `"weekday"` – Wochentag als Zahl (1=Mo bis 7=So).
- `"week"` – ISO-Kalenderwoche.
- `"DD.MM.YYYY"`, `"YYYY-MM-DD HH:mm"` etc. – formatiert das Datum nach dem angegebenen Layout.

---

## date.Now([format_or_part])
- **Konkret:**
  Gibt das aktuelle Datum und die aktuelle Uhrzeit zurück.
  Mit `format_or_part` kann ein bestimmter Teil oder ein Ausgabeformat angegeben werden.
- **Parameter:**
  - `format_or_part`: Optional.
- **Rückgabe:**
  `StrVal` (Standard: `DD.MM.YYYY HH:mm:ss`) oder `NumVal` bei Teilangabe.

---

## date.Today()
- **Konkret:**
  Gibt das heutige Datum um `00:00:00` zurück.
- **Rückgabe:**
  `StrVal`

---

## date.Parse(dateString [, format_or_part])
- **Konkret:**
  Parst einen Datum-String und gibt ihn im Standardformat zurück oder extrahiert einen Teil.
- **Parameter:**
  - `dateString`: Datum als String.
  - `format_or_part`: Optional.
- **Rückgabe:**
  `StrVal` oder `NumVal` bei Teilangabe. `"invalid date"` wenn das Datum nicht erkannt wird.

---

## date.Part(dateString, partOrLayout)
- **Konkret:**
  Extrahiert einen spezifischen Teil eines Datums oder formatiert es neu.
- **Parameter:**
  - `dateString`: Datum als String.
  - `partOrLayout`: Teilname oder Layoutstring.
- **Rückgabe:**
  `StrVal` oder `NumVal`.

---

## date.Year(dateString)
- **Konkret:**
  Extrahiert die Jahreszahl aus einem Datum.
- **Rückgabe:**
  `NumVal`

---

## date.Month(dateString)
- **Konkret:**
  Extrahiert den Monat (1–12) aus einem Datum.
- **Rückgabe:**
  `NumVal`

---

## date.Day(dateString)
- **Konkret:**
  Extrahiert den Tag (1–31) aus einem Datum.
- **Rückgabe:**
  `NumVal`

---

## date.Hour(dateString)
- **Konkret:**
  Extrahiert die Stunde (0–23) aus einer Zeitangabe.
- **Rückgabe:**
  `NumVal`

---

## date.Minute(dateString)
- **Konkret:**
  Extrahiert die Minute (0–59) aus einer Zeitangabe.
- **Rückgabe:**
  `NumVal`

---

## date.Second(dateString)
- **Konkret:**
  Extrahiert die Sekunde (0–59) aus einer Zeitangabe.
- **Rückgabe:**
  `NumVal`

---

## date.Weekday(dateString)
- **Konkret:**
  Gibt den Wochentag als Zahl zurück (ISO: 1=Montag, 7=Sonntag).
- **Rückgabe:**
  `NumVal`

---

## date.Week(dateString)
- **Konkret:**
  Gibt die ISO-Kalenderwoche eines Datums zurück.
- **Rückgabe:**
  `NumVal`

---

## date.Add(date, amount, unit [, format_or_part])
- **Konkret:**
  Addiert ein Zeitintervall zu einem Datum.
- **Parameter:**
  - `date`: Ausgangsdatum als String.
  - `amount`: Anzahl (negativ für Subtraktion).
  - `unit`: Einheit. Unterstützte Werte: `"d"` / `"day"`, `"m"` / `"month"`, `"y"` / `"year"`, `"h"` / `"hour"`, `"n"` / `"minute"`, `"s"` / `"second"`.
  - `format_or_part`: Optional.
- **Rückgabe:**
  `StrVal` oder `NumVal` bei Teilangabe.

---

## date.TimeAdd(clockStr, amount, unit)
- **Konkret:**
  Addiert Stunden oder Minuten zu einer reinen Uhrzeit (`HH:mm` oder `HH:mm:ss`).
  Läuft bei Mitternacht durch (kein Fehler bei Überlauf).
- **Parameter:**
  - `clockStr`: Uhrzeit als String.
  - `amount`: Anzahl (negativ für Subtraktion).
  - `unit`: `"h"` / `"hour"` oder `"n"` / `"minute"`.
- **Rückgabe:**
  `StrVal` (`HH:mm`)

---

## date.DateDiff(unit, d1, d2)
- **Konkret:**
  Berechnet die Differenz zwischen zwei Datumswerten in der angegebenen Einheit.
  Ergebnis ist positiv wenn `d2` nach `d1` liegt.
- **Parameter:**
  - `unit`: Einheit. Unterstützte Werte: `"y"` / `"year"`, `"m"` / `"month"`, `"d"` / `"day"`, `"w"` / `"week"`, `"h"` / `"hour"`, `"n"` / `"minute"`, `"s"` / `"second"`.
  - `d1`: Erstes Datum.
  - `d2`: Zweites Datum.
- **Rückgabe:**
  `NumVal`

---

## date.IsBefore(d1, d2)
- **Konkret:**
  Prüft ob `d1` zeitlich vor `d2` liegt.
- **Rückgabe:**
  `BoolVal`

---

## date.IsAfter(d1, d2)
- **Konkret:**
  Prüft ob `d1` zeitlich nach `d2` liegt.
- **Rückgabe:**
  `BoolVal`

---

## date.StartOfWeek(dateString [, format_or_part])
- **Konkret:**
  Gibt den Montag der Woche des angegebenen Datums zurück (ISO-Woche, Zeit: 00:00:00).
- **Parameter:**
  - `dateString`: Datum als String.
  - `format_or_part`: Optional.
- **Rückgabe:**
  `StrVal` oder `NumVal` bei Teilangabe.

---

## date.StartOfMonth(dateString [, format_or_part])
- **Konkret:**
  Gibt den ersten Tag des Monats zurück (Zeit: 00:00:00).
- **Parameter:**
  - `dateString`: Datum als String.
  - `format_or_part`: Optional.
- **Rückgabe:**
  `StrVal` oder `NumVal` bei Teilangabe.

---

## date.EndOfMonth(dateString [, format_or_part])
- **Konkret:**
  Gibt den letzten Tag des Monats zurück.
  Berücksichtigt Schaltjahre automatisch.
- **Parameter:**
  - `dateString`: Datum als String.
  - `format_or_part`: Optional.
- **Rückgabe:**
  `StrVal` oder `NumVal` bei Teilangabe.

---

## date.StartOfYear(dateString [, format_or_part])
- **Konkret:**
  Gibt den 1. Januar des Jahres zurück (Zeit: 00:00:00).
- **Parameter:**
  - `dateString`: Datum als String.
  - `format_or_part`: Optional.
- **Rückgabe:**
  `StrVal` oder `NumVal` bei Teilangabe.

---

## date.EndOfYear(dateString [, format_or_part])
- **Konkret:**
  Gibt den 31. Dezember des Jahres um 23:59:59 zurück.
- **Parameter:**
  - `dateString`: Datum als String.
  - `format_or_part`: Optional.
- **Rückgabe:**
  `StrVal` oder `NumVal` bei Teilangabe.

---

## date.Age(dateString | day, month, year)
- **Konkret:**
  Berechnet das Alter zum heutigen Tag.
  Akzeptiert entweder einen Datum-String oder Tag, Monat, Jahr als separate Zahlen.
  Gibt einen Fehler zurück wenn das Datum in der Zukunft liegt.
- **Parameter:**
  - Variante A: `dateString` – Geburtsdatum als String.
  - Variante B: `day`, `month`, `year` – als separate Zahlen.
- **Rückgabe:**
  `ArrVal`
  Format: `[Jahre, Monate, Tage]`

---

## date.AgeString(dateString | day, month, year)
- **Konkret:**
  Identisch zu `date.Age`, gibt das Ergebnis aber als lesbaren Text zurück.
- **Rückgabe:**
  `StrVal`
  Beispiel: `"35 Jahre, 4 Monate, 12 Tage"`.

---

## date.Unix([dateString])
- **Konkret:**
  Gibt den Unix-Timestamp zurück.
  Ohne Parameter: aktueller Zeitpunkt.
- **Parameter:**
  - `dateString`: Optional. Datum als String.
- **Rückgabe:**
  `NumVal` (Sekunden seit 01.01.1970 UTC)

---

## date.FromUnix(timestamp [, format_or_part])
- **Konkret:**
  Wandelt einen Unix-Timestamp in ein Datum um.
- **Parameter:**
  - `timestamp`: Unix-Timestamp als Zahl.
  - `format_or_part`: Optional.
- **Rückgabe:**
  `StrVal` oder `NumVal` bei Teilangabe.

---

## date.UTC([dateString] [, format])
- **Konkret:**
  Gibt die aktuelle Zeit in UTC zurück, oder konvertiert ein Datum nach UTC.
- **Parameter:**
  - `dateString`: Optional. Lokales Datum als String.
  - `format`: Optional. Ausgabeformat.
- **Rückgabe:**
  `StrVal`

---

## date.ToUTC(dateString)
- **Konkret:**
  Konvertiert ein lokales Datum in UTC.
- **Parameter:**
  - `dateString`: Datum als String.
- **Rückgabe:**
  `StrVal`

---

## date.Local([dateString] [, format])
- **Konkret:**
  Gibt die aktuelle Zeit in lokaler Zeitzone zurück, oder konvertiert ein Datum in die lokale Zeit.
- **Parameter:**
  - `dateString`: Optional. Datum als String.
  - `format`: Optional. Ausgabeformat.
- **Rückgabe:**
  `StrVal`

---

## date.ToLocal(dateString)
- **Konkret:**
  Konvertiert ein UTC-Datum in die lokale Zeitzone.
- **Parameter:**
  - `dateString`: Datum als String.
- **Rückgabe:**
  `StrVal`

---

## date.Timer([dateString])
- **Konkret:**
  Gibt die Anzahl der Sekunden seit Mitternacht zurück.
  Ohne Parameter: aktueller Zeitpunkt.
- **Parameter:**
  - `dateString`: Optional. Datum/Uhrzeit als String.
- **Rückgabe:**
  `NumVal`

---

## date.IsLeapYear([year_or_date])
- **Konkret:**
  Prüft ob ein Jahr ein Schaltjahr ist.
  Ohne Parameter: aktuelles Jahr.
- **Parameter:**
  - `year_or_date`: Optional. Jahreszahl oder Datum-String.
- **Rückgabe:**
  `BoolVal`

---

## date.NextLeapYear([year_or_date])
- **Konkret:**
  Gibt das nächste Schaltjahr nach dem angegebenen Jahr zurück.
  Ohne Parameter: nächstes Schaltjahr ab heute.
- **Parameter:**
  - `year_or_date`: Optional. Jahreszahl oder Datum-String.
- **Rückgabe:**
  `NumVal`

---

## date.CheckLayout(format)
- **Konkret:**
  Zeigt die interne Go-Übersetzung eines VB-Datumsformats an. Nützlich zum Debuggen von Formatstrings.
- **Parameter:**
  - `format`: VB-Layoutstring (z. B. `"DD.MM.YYYY HH:mm"`).
- **Rückgabe:**
  `StrVal`
  Beispiel: `"'DD.MM.YYYY' wird zu Go-Layout: '02.01.2006'"`.

---

## date.Compare(d1, d2)
- **Konkret:**
  Vergleicht zwei Datumswerte.
- **Parameter:**
  - `d1`: Erstes Datum als String.
  - `d2`: Zweites Datum als String.
- **Rückgabe:**
  `NumVal`
  `-1` wenn d1 älter als d2, `0` bei Gleichheit, `1` wenn d1 neuer als d2.
  `0` bei ungültigem Datum.