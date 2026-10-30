package stdlib

import (
	"container/list"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"vbxc/value"
)

const defaultVBFormat = "02.01.2006 15:04:05"

// ---------------- LRU Cache ----------------

type cacheEntry struct {
	key string
	val time.Time
}

var (
	dateCache      = make(map[string]*list.Element)
	dateCacheList  = list.New()
	dateCacheLimit = 100
)

// ---------------- VB Layout Übersetzung ----------------

func translateVBLayout(vb string) string {
	r := strings.NewReplacer(
		"YYYY", "2006",
		"YY", "06",
		"MM", "01",
		"DD", "02",
		"HH", "15",
		"mm", "04",
		"ss", "05",
	)
	return r.Replace(vb)
}

// ---------------- Deterministische Layouts ----------------
// Reihenfolge ist absichtlich gewählt!

var parseLayouts = []string{
	// ISO zuerst (keine Ambiguität)
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",

	// Deutsch
	"02.01.2006 15:04:05",
	"02.01.2006 15:04",
	"02.01.2006",

	// US (bewusst nach ISO)
	"01-02-2006 15:04:05",
	"01-02-2006 15:04",
	"01-02-2006",

	time.RFC3339,
}

// ---------------- Parsing ----------------

func parseDateSafe(s string) (time.Time, bool) {
	if el, ok := dateCache[s]; ok {
		dateCacheList.MoveToBack(el)
		return el.Value.(cacheEntry).val, true
	}

	for _, layout := range parseLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {

			// LRU eviction
			if dateCacheList.Len() >= dateCacheLimit {
				front := dateCacheList.Front()
				if front != nil {
					entry := front.Value.(cacheEntry)
					delete(dateCache, entry.key)
					dateCacheList.Remove(front)
				}
			}

			el := dateCacheList.PushBack(cacheEntry{s, t})
			dateCache[s] = el
			return t, true
		}
	}

	return time.Time{}, false
}

// ---------------- Helper ----------------

func formatVB(t time.Time, vbLayout string) string {
	if vbLayout == "" {
		return t.Format(defaultVBFormat)
	}
	return t.Format(translateVBLayout(vbLayout))
}

func datePart(t time.Time, part string) value.Value {
	switch strings.ToLower(part) {
	case "year":
		return value.NumVal(float64(t.Year()))
	case "month":
		return value.NumVal(float64(t.Month()))
	case "day":
		return value.NumVal(float64(t.Day()))
	case "hour":
		return value.NumVal(float64(t.Hour()))
	case "minute":
		return value.NumVal(float64(t.Minute()))
	case "second":
		return value.NumVal(float64(t.Second()))
	case "weekday":
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		return value.NumVal(float64(wd))
	case "week":
		_, w := t.ISOWeek()
		return value.NumVal(float64(w))
	default:
		return value.Value{
			Kind: value.KindStr,
			Str:  formatVB(t, part),
		}
	}
}

func parseClock(s string) (int, bool) {
	layouts := []string{
		"15:04:05",
		"15:04",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Hour()*60 + t.Minute(), true
		}
	}
	return 0, false
}

func formatClock(minutes int, withSeconds bool) string {
	minutes = ((minutes % 1440) + 1440) % 1440

	h := minutes / 60
	m := minutes % 60

	if withSeconds {
		return time.Date(0, 1, 1, h, m, 0, 0, time.UTC).Format("15:04:05")
	}
	return time.Date(0, 1, 1, h, m, 0, 0, time.UTC).Format("15:04")
}

// ---------------- Init ----------------

func initDateFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "date."

	// -------- date.Now([format_or_part]) --------
	Register(ns+"Now", "date", "[format_or_part]", "Gibt das aktuelle Datum/Uhrzeit zurück oder einen Teil davon.", func(args []value.Value) value.Value {
		t := time.Now()
		if len(args) > 0 && args[0].Kind == value.KindStr {
			// Prüft intern, ob es ein Teil (year, day...) oder ein Layout (DD.MM...) ist
			return datePart(t, args[0].Str)
		}
		return value.StrVal(t.Format(defaultVBFormat))
	})

	// -------- date.Compare(d1, d2) --------
	Register(ns+"Compare", "date", "d1, d2",
		"Vergleicht zwei Daten. Gibt -1 zurück wenn d1 älter ist, 0 bei Gleichheit und 1 wenn d1 neuer ist.",
		func(args []value.Value) value.Value {

			if len(args) < 2 {
				return value.NumVal(0)
			}

			t1, ok1 := parseDateSafe(args[0].Str)
			t2, ok2 := parseDateSafe(args[1].Str)

			if !ok1 || !ok2 {
				return value.NumVal(0)
			}

			if t1.Before(t2) {
				return value.NumVal(-1)
			}

			if t1.After(t2) {
				return value.NumVal(1)
			}

			return value.NumVal(0)
		})

	// -------- date.CheckLayout(vbFormat) --------
	Register(ns+"CheckLayout", "date", "format", "Zeigt die interne Go-Übersetzung des VB-Layouts.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.StrVal("")
		}
		// Zeigt z.B. "DD.MM.YYYY" -> "02.01.2006"
		return value.StrVal(fmt.Sprintf("'%s' wird zu Go-Layout: '%s'", args[0].Str, translateVBLayout(args[0].Str)))
	})

	// -------- date.StartOfMonth(date[, format_or_part]) --------
	Register(ns+"StartOfMonth", "date", "date[, format_or_part]", "Gibt den ersten Tag des Monats zurück.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.StrVal("")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.StrVal("invalid date")
		}

		// Auf den 1. des Monats setzen, Zeit auf 00:00:00
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())

		if len(args) > 1 {
			return datePart(start, args[1].Str)
		}
		return value.StrVal(start.Format(defaultVBFormat))
	})

	// -------- date.StartOfYear(date[, format_or_part]) --------
	Register(ns+"StartOfYear", "date", "date[, format_or_part]", "Gibt den ersten Tag des Jahres zurück.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.StrVal("")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.StrVal("invalid date")
		}

		start := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())

		if len(args) > 1 {
			return datePart(start, args[1].Str)
		}
		return value.StrVal(start.Format(defaultVBFormat))
	})

	// -------- date.EndOfYear(date[, format_or_part]) --------
	Register(ns+"EndOfYear", "date", "date[, format_or_part]", "Gibt den letzten Tag des Jahres zurück.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.StrVal("")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.StrVal("invalid date")
		}

		end := time.Date(t.Year(), 12, 31, 23, 59, 59, 0, t.Location())

		if len(args) > 1 {
			return datePart(end, args[1].Str)
		}
		return value.StrVal(end.Format(defaultVBFormat))
	})

	// -------- date.ToUTC(dateStr) --------
	Register(ns+"ToUTC", "date", "dateString", "Konvertiert ein lokales Datum in UTC.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.StrVal("")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.StrVal("invalid date")
		}
		return value.StrVal(t.UTC().Format(defaultVBFormat))
	})

	// -------- date.ToLocal(dateStr) --------
	Register(ns+"ToLocal", "date", "dateString", "Konvertiert ein UTC-Datum in die lokale Zeitzone.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.StrVal("")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.StrVal("invalid date")
		}
		return value.StrVal(t.Local().Format(defaultVBFormat))
	})

	// -------- date.Today() --------
	Register(ns+"Today", "date", "", "Gibt das heutige Datum um 00:00:00 zurück.", func(args []value.Value) value.Value {
		t := time.Now()
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		return value.StrVal(d.Format(defaultVBFormat))
	})

	// -------- date.Age(tag, monat, jahr) oder date.Age(dateStr) --------
	Register(ns+"Age", "date", "dateString | day, month, year",
		"Berechnet das Alter und gibt ein Array [Jahre, Monate, Tage] zurück.",
		func(args []value.Value) value.Value {

			y, m, d, err := calcAgeFromArgs(args)
			if err.Kind == value.KindError {
				return err
			}

			return value.Value{
				Kind: value.KindArr,
				Arr: []value.Value{
					value.NumVal(float64(y)),
					value.NumVal(float64(m)),
					value.NumVal(float64(d)),
				},
			}
		})

	// -------- date.AgeString(...) --------
	Register(ns+"AgeString", "date", "dateString | day, month, year",
		"Berechnet das Alter als Text (Jahre, Monate, Tage).",
		func(args []value.Value) value.Value {

			y, m, d, err := calcAgeFromArgs(args)
			if err.Kind == value.KindError {
				return err
			}

			return value.StrVal(fmt.Sprintf("%d Jahre, %d Monate, %d Tage", y, m, d))
		})

	// -------- date.Unix([date_string]) --------
	Register(ns+"Unix", "date", "[date_string]",
		"Gibt den Unix-Timestamp für jetzt oder ein spezifisches Datum zurück.", func(args []value.Value) value.Value {
			if len(args) > 0 && args[0].Str != "" {
				if t, ok := parseDateSafe(args[0].Str); ok {
					return value.NumVal(float64(t.Unix()))
				}
			}
			return value.NumVal(float64(time.Now().Unix()))
		})

	// -------- date.Parse(date[, format_or_part]) --------
	Register(ns+"Parse", "date", "[format_or_part]",
		"Parsen und Umformatieren oder Extrahieren von Teilen (year, month, day, etc.).", func(args []value.Value) value.Value {
			if len(args) < 1 {
				return value.StrVal("invalid date")
			}
			t, ok := parseDateSafe(args[0].Str)
			if !ok {
				return value.StrVal("invalid date")
			}

			if len(args) > 1 {
				// Unterstützt nun auch "year", "month" etc. via datePart
				return datePart(t, args[1].Str)
			}
			return value.StrVal(t.Format(defaultVBFormat))
		})

	// -------- date.FromUnix(timestamp[, format_or_part]) --------
	Register(ns+"FromUnix", "date", "timestamp, [format_or_part]",
		"Wandelt einen Unix-Timestamp in ein Datum oder einen Teil (year, etc.) um.", func(args []value.Value) value.Value {
			if len(args) < 1 {
				return value.StrVal("")
			}
			ts := int64(value.ToNum(args[0]))
			t := time.Unix(ts, 0)

			if len(args) > 1 {
				return datePart(t, args[1].Str)
			}
			return value.StrVal(t.Format(defaultVBFormat))
		})

	// -------- date.StartOfWeek(date[, format_or_part]) --------
	Register(ns+"StartOfWeek", "date", "[partOrLayout]", "Gibt den Montag der Woche eines Datums zurück.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.StrVal("")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.StrVal("invalid date")
		}

		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		} // Sonntag zu 7 machen (ISO)

		start := t.AddDate(0, 0, -(weekday - 1))
		// Auf den Beginn des Tages normalisieren
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())

		if len(args) > 1 {
			return datePart(start, args[1].Str)
		}
		return value.StrVal(start.Format(defaultVBFormat))
	})

	// -------- date.EndOfMonth(date[, format_or_part]) --------
	Register(ns+"EndOfMonth", "date", "[partOrLayout]", "Gibt den letzten Tag des Monats eines Datums zurück.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.StrVal("")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.StrVal("invalid date")
		}

		// Der 0. Tag des Folgemonats ist immer der letzte Tag des aktuellen Monats
		end := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location())

		if len(args) > 1 {
			return datePart(end, args[1].Str)
		}
		return value.StrVal(end.Format(defaultVBFormat))
	})

	// -------- date.Add(date, amount, unit, [format_or_part]) --------
	Register(ns+"Add", "date", "date, amount, unit, [part]", "Addiert Intervalle (d, m, y, h, n, s) zu einem Datum.", func(args []value.Value) value.Value {
		if len(args) < 3 {
			return value.StrVal("")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.StrVal("invalid date")
		}

		amount := int(value.ToNum(args[1]))
		unit := strings.ToLower(args[2].Str)

		switch unit {
		case "d", "day", "days":
			t = t.AddDate(0, 0, amount)
		case "m", "month", "months":
			t = t.AddDate(0, amount, 0)
		case "y", "year", "years":
			t = t.AddDate(amount, 0, 0)
		case "h", "hour", "hours":
			t = t.Add(time.Duration(amount) * time.Hour)
		case "n", "minute", "minutes":
			t = t.Add(time.Duration(amount) * time.Minute)
		case "s", "second", "seconds":
			t = t.Add(time.Duration(amount) * time.Second)
		default:
			return value.StrVal("invalid unit")
		}

		if len(args) >= 4 {
			return datePart(t, args[3].Str)
		}
		return value.StrVal(t.Format(defaultVBFormat))
	})

	// -------- date.TimeAdd(clock, amount, unit) --------
	Register(ns+"TimeAdd", "date", "clockStr, amount, unit", "Addiert Stunden oder Minuten zu einer reinen Uhrzeit (HH:mm).", func(args []value.Value) value.Value {
		if len(args) < 3 {
			return value.StrVal("")
		}
		minutes, ok := parseClock(args[0].Str)
		if !ok {
			return value.StrVal("invalid time")
		}

		amount := int(value.ToNum(args[1]))
		unit := strings.ToLower(args[2].Str)

		switch unit {
		case "h", "hour", "hours":
			minutes += amount * 60
		case "n", "minute", "minutes":
			minutes += amount
		default:
			return value.StrVal("invalid unit")
		}

		return value.StrVal(formatClock(minutes, false))
	})

	// -------- date.DateDiff(unit, d1, d2) --------
	Register(ns+"DateDiff", "date", "unit, d1, d2", "Berechnet die Differenz zwischen zwei Daten in der angegebenen Einheit.", func(args []value.Value) value.Value {
		if len(args) < 3 {
			return value.NumVal(0)
		}

		unit := strings.ToLower(args[0].Str)
		t1, ok1 := parseDateSafe(args[1].Str)
		t2, ok2 := parseDateSafe(args[2].Str)
		if !ok1 || !ok2 {
			return value.NumVal(0)
		}

		switch unit {
		case "y", "year", "yyyy":
			return value.NumVal(float64(t2.Year() - t1.Year()))
		case "m", "month":
			return value.NumVal(float64(int(t2.Month()) - int(t1.Month()) + (t2.Year()-t1.Year())*12))
		case "d", "day":
			d1 := time.Date(t1.Year(), t1.Month(), t1.Day(), 0, 0, 0, 0, t1.Location())
			d2 := time.Date(t2.Year(), t2.Month(), t2.Day(), 0, 0, 0, 0, t2.Location())
			return value.NumVal(math.Round(d2.Sub(d1).Hours() / 24))
		case "h", "hour":
			return value.NumVal(math.Trunc(t2.Sub(t1).Hours()))
		case "n", "minute":
			return value.NumVal(math.Trunc(t2.Sub(t1).Minutes()))
		case "s", "second":
			return value.NumVal(math.Trunc(t2.Sub(t1).Seconds()))
		case "w", "week":
			d1 := time.Date(t1.Year(), t1.Month(), t1.Day(), 0, 0, 0, 0, t1.Location())
			d2 := time.Date(t2.Year(), t2.Month(), t2.Day(), 0, 0, 0, 0, t2.Location())
			return value.NumVal(math.Trunc(d2.Sub(d1).Hours() / (24 * 7)))
		default:
			return value.ErrorVal("DateDiff: unbekannte Einheit '" + unit + "'")
		}
	})

	// -------- date.Year(date) --------
	Register(ns+"Year", "date", "dateString", "Extrahiert die Jahreszahl aus einem Datum.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.NumVal(0)
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.NumVal(0)
		}
		return value.NumVal(float64(t.Year()))
	})

	// -------- date.Month(date) --------
	Register(ns+"Month", "date", "dateString", "Extrahiert den Monat (1-12) aus einem Datum.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.NumVal(0)
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.NumVal(0)
		}
		return value.NumVal(float64(t.Month()))
	})

	// -------- date.Day(date) --------
	Register(ns+"Day", "date", "dateString", "Extrahiert den Tag (1-31) aus einem Datum.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.NumVal(0)
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.NumVal(0)
		}
		return value.NumVal(float64(t.Day()))
	})

	// -------- date.Weekday(date) --------
	Register(ns+"Weekday", "date", "dateString", "Gibt den Wochentag als Zahl zurück (1=Mo bis 7=So).", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.NumVal(0)
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.NumVal(0)
		}
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		return value.NumVal(float64(wd))
	})

	// -------- date.IsBefore(d1, d2) --------
	Register(ns+"IsBefore", "date", "d1, d2", "Prüft ob d1 vor d2 liegt.", func(args []value.Value) value.Value {
		if len(args) < 2 {
			return value.BoolVal(false)
		}
		t1, ok1 := parseDateSafe(args[0].Str)
		t2, ok2 := parseDateSafe(args[1].Str)
		if !ok1 || !ok2 {
			return value.BoolVal(false)
		}

		return value.BoolVal(t1.Before(t2)) // Gibt jetzt echtes true/false zurück
	})

	// -------- date.IsAfter(d1, d2) --------
	Register(ns+"IsAfter", "date", "d1, d2", "Prüft ob d1 nach d2 liegt.", func(args []value.Value) value.Value {
		if len(args) < 2 {
			return value.BoolVal(false)
		}
		t1, ok1 := parseDateSafe(args[0].Str)
		t2, ok2 := parseDateSafe(args[1].Str)
		if !ok1 || !ok2 {
			return value.BoolVal(false)
		}

		return value.BoolVal(t1.After(t2))
	})

	// -------- date.Hour(date) --------
	Register(ns+"Hour", "date", "dateString", "Extrahiert die Stunde aus einer Zeitangabe.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.NumVal(0)
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.NumVal(0)
		}
		return value.NumVal(float64(t.Hour()))
	})

	// -------- date.Minute(date) --------
	Register(ns+"Minute", "date", "dateString", "Extrahiert die Minute aus einer Zeitangabe.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.NumVal(0)
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.NumVal(0)
		}
		return value.NumVal(float64(t.Minute()))
	})

	// -------- date.Second(date) --------
	Register(ns+"Second", "date", "dateString", "Extrahiert die Sekunde aus einer Zeitangabe.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.NumVal(0)
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.NumVal(0)
		}
		return value.NumVal(float64(t.Second()))
	})

	// -------- date.Week(date) --------
	Register(ns+"Week", "date", "dateString", "Gibt die ISO-Kalenderwoche eines Datums zurück.", func(args []value.Value) value.Value {
		if len(args) < 1 {
			return value.NumVal(0)
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.NumVal(0)
		}
		_, w := t.ISOWeek()
		return value.NumVal(float64(w))
	})

	// -------- date.Part(date, part) --------
	Register(ns+"Part", "date", "dateString, partOrLayout", "Extrahiert einen spezifischen Teil eines Datums oder formatiert es neu.", func(args []value.Value) value.Value {
		if len(args) < 2 {
			return value.StrVal("")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return value.StrVal("invalid date")
		}
		return datePart(t, args[1].Str)
	})

	// -------- date.IsLeapYear([year_or_date]) --------
	Register(ns+"IsLeapYear", "date", "[year_or_date]", "Prüft ob ein Jahr ein Schaltjahr ist.", func(args []value.Value) value.Value {
		year := time.Now().Year()
		if len(args) >= 1 {
			if args[0].Kind == value.KindStr {
				if t, ok := parseDateSafe(args[0].Str); ok {
					year = t.Year()
				} else {
					val, _ := strconv.Atoi(args[0].Str)
					if val > 0 {
						year = val
					}
				}
			} else {
				year = int(value.ToNum(args[0]))
			}
		}
		isLeap := (year%4 == 0 && year%100 != 0) || (year%400 == 0)
		return value.BoolVal(isLeap)
	})

	// -------- date.NextLeapYear([year_or_date]) --------
	Register(ns+"NextLeapYear", "date", "[year_or_date]", "Gibt das nächste Schaltjahr nach dem angegebenen Jahr/Datum zurück.", func(args []value.Value) value.Value {
		start := time.Now().Year()
		if len(args) >= 1 {
			if args[0].Kind == value.KindStr {
				if t, ok := parseDateSafe(args[0].Str); ok {
					start = t.Year()
				} else {
					val, _ := strconv.Atoi(args[0].Str)
					if val > 0 {
						start = val
					}
				}
			} else {
				start = int(value.ToNum(args[0]))
			}
		}

		// Suche das nächste Schaltjahr (max 8 Jahre Versatz möglich bei Jahrhundertwende)
		for y := start + 1; y < start+20; y++ {
			if (y%4 == 0 && y%100 != 0) || (y%400 == 0) {
				return value.NumVal(float64(y))
			}
		}
		return value.NumVal(0)
	})

	// -------- date.UTC([date_string], [format]) --------
	Register(ns+"UTC", "date", "[dateString], [format]", "Gibt die aktuelle Zeit oder ein konvertiertes Datum in UTC zurück.", func(args []value.Value) value.Value {
		t := time.Now().UTC()
		format := defaultVBFormat

		// Falls ein Datum übergeben wurde, konvertiere dieses nach UTC
		if len(args) > 0 && args[0].Str != "" {
			if parsed, ok := parseDateSafe(args[0].Str); ok {
				t = parsed.UTC()
			} else {
				// Falls erstes Arg kein Datum, ist es vielleicht direkt das Format
				return value.StrVal(formatVB(t, args[0].Str))
			}
		}

		if len(args) > 1 {
			format = args[1].Str
		}
		return value.StrVal(formatVB(t, format))
	})

	// -------- date.Local([date_string], [format]) --------
	Register(ns+"Local", "date", "[dateString], [format]", "Gibt die aktuelle Zeit oder ein konvertiertes Datum in lokaler Zeit zurück.", func(args []value.Value) value.Value {
		t := time.Now().Local()
		format := defaultVBFormat

		if len(args) > 0 && args[0].Str != "" {
			if parsed, ok := parseDateSafe(args[0].Str); ok {
				t = parsed.Local()
			} else {
				return value.StrVal(formatVB(t, args[0].Str))
			}
		}

		if len(args) > 1 {
			format = args[1].Str
		}
		return value.StrVal(formatVB(t, format))
	})

	// -------- date.Timer() --------
	Register(ns+"Timer", "date", "[dateString]", "Sekunden seit Mitternacht.", func(args []value.Value) value.Value {
		t := time.Now()
		// Falls ein Datum übergeben wurde, berechne Timer für diesen Tag
		if len(args) > 0 {
			if parsed, ok := parseDateSafe(args[0].Str); ok {
				t = parsed
			}
		}
		seconds := t.Hour()*3600 + t.Minute()*60 + t.Second()
		return value.NumVal(float64(seconds))
	})

}

func internalCalcAge(day, month, year int) (int, int, int) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	geb := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)

	years := today.Year() - geb.Year()

	if today.Month() < geb.Month() ||
		(today.Month() == geb.Month() && today.Day() < geb.Day()) {
		years--
	}

	months := int(today.Month()) - int(geb.Month())
	if today.Day() < geb.Day() {
		months--
	}
	if months < 0 {
		months += 12
	}

	days := today.Day() - geb.Day()
	if days < 0 {
		prevMonth := today.AddDate(0, -1, 0)
		days += time.Date(prevMonth.Year(), prevMonth.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
	}

	return years, months, days
}

func calcAgeFromArgs(args []value.Value) (int, int, int, value.Value) {
	var d, m, y int
	if len(args) == 1 {
		if args[0].Kind != value.KindStr {
			return 0, 0, 0, value.ErrorVal("Ungültiges Datum")
		}
		t, ok := parseDateSafe(args[0].Str)
		if !ok {
			return 0, 0, 0, value.ErrorVal("Ungültiges Datum")
		}
		d, m, y = t.Day(), int(t.Month()), t.Year()
	} else if len(args) >= 3 {
		d = int(value.ToNum(args[0]))
		m = int(value.ToNum(args[1]))
		y = int(value.ToNum(args[2]))
	} else {
		return 0, 0, 0, value.ErrorVal("usage: date.Age(dateString | day, month, year)")
	}

	test := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local)
	if test.Day() != d || int(test.Month()) != m || test.Year() != y {
		return 0, 0, 0, value.ErrorVal("Ungültiges Datum")
	}
	if test.After(time.Now()) {
		return 0, 0, 0, value.ErrorVal("Datum liegt in der Zukunft")
	}

	yy, mm, dd := internalCalcAge(d, m, y)
	return yy, mm, dd, value.NullVal()
}

func init() { initDateFunctions() }
