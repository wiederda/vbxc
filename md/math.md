# 🔢 math.* – Mathematische Funktionen

Dient zur Durchführung mathematischer Berechnungen.
Alle Funktionen akzeptieren Zahlen und numerische Strings (Komma als Dezimaltrennzeichen wird automatisch akzeptiert).

---

## math.Abs(n)
- **Konkret:**
  Gibt den absoluten Wert (Betrag) einer Zahl zurück.
- **Rückgabe:**
  `NumVal`

---

## math.Sign(n)
- **Konkret:**
  Gibt das Vorzeichen einer Zahl zurück.
- **Rückgabe:**
  `NumVal`
  `-1` (negativ), `0` (null), `1` (positiv).

---

## math.Round(v [, d])
- **Konkret:**
  Rundet kaufmännisch auf `d` Nachkommastellen.
- **Parameter:**
  - `v`: Zu rundende Zahl.
  - `d`: Optional. Anzahl Nachkommastellen (Standard: `0`).
- **Rückgabe:**
  `NumVal`

---

## math.RoundBank(v [, d])
- **Konkret:**
  Rundet auf die nächste gerade Zahl (Banker's Rounding / kaufmännisches Runden bei 0,5).
  Vermeidet systematische Rundungsfehler in Summen.
- **Parameter:**
  - `v`: Zu rundende Zahl.
  - `d`: Optional. Anzahl Nachkommastellen (Standard: `0`).
- **Rückgabe:**
  `NumVal`

---

## math.Ceil(n)
- **Konkret:**
  Rundet auf die nächste ganze Zahl auf.
- **Rückgabe:**
  `NumVal`

---

## math.Floor(n)
- **Konkret:**
  Rundet auf die nächste ganze Zahl ab.
- **Rückgabe:**
  `NumVal`

---

## math.Trunc(n)
- **Konkret:**
  Schneidet Nachkommastellen ab (Richtung Null).
- **Rückgabe:**
  `NumVal`

---

## math.Int(n)
- **Konkret:**
  Schneidet alle Nachkommastellen ab. Alias für `math.Trunc`.
- **Rückgabe:**
  `NumVal`

---

## math.Clamp(v, min, max)
- **Konkret:**
  Hält einen Wert innerhalb der angegebenen Grenzen.
  Gibt `min` zurück wenn `v < min`, `max` wenn `v > max`, sonst `v`.
- **Parameter:**
  - `v`: Zu begrenzender Wert.
  - `min`: Untere Grenze.
  - `max`: Obere Grenze.
- **Rückgabe:**
  `NumVal`

---

## math.Min(a, b)
- **Konkret:**
  Gibt die kleinere der beiden Zahlen zurück.
- **Rückgabe:**
  `NumVal`

---

## math.Max(a, b)
- **Konkret:**
  Gibt die größere der beiden Zahlen zurück.
- **Rückgabe:**
  `NumVal`

---

## math.Pow(basis, exp)
- **Konkret:**
  Berechnet die Potenz (`basis ^ exp`).
- **Rückgabe:**
  `NumVal`

---

## math.Sqrt(n)
- **Konkret:**
  Berechnet die Quadratwurzel.
  Gibt einen Fehler zurück wenn `n < 0`.
- **Rückgabe:**
  `NumVal`

---

## math.Root(v, n)
- **Konkret:**
  Berechnet die `n`-te Wurzel von `v`.
  Gibt einen Fehler zurück wenn `n = 0`.
- **Parameter:**
  - `v`: Basis.
  - `n`: Wurzelexponent.
- **Rückgabe:**
  `NumVal`

---

## math.Cbrt(n)
- **Konkret:**
  Berechnet die Kubikwurzel (3. Wurzel).
- **Rückgabe:**
  `NumVal`

---

## math.Mod(a, b)
- **Konkret:**
  Berechnet den Rest der Ganzzahl-Division (Modulo).
  Gibt einen Fehler zurück wenn `b = 0`.
- **Parameter:**
  - `a`: Dividend.
  - `b`: Divisor.
- **Rückgabe:**
  `NumVal`

---

## math.Log(n)
- **Konkret:**
  Berechnet den natürlichen Logarithmus (Basis `e`).
  Gibt einen Fehler zurück wenn `n <= 0`.
- **Rückgabe:**
  `NumVal`

---

## math.LogBase(n, base)
- **Konkret:**
  Berechnet den Logarithmus von `n` zur angegebenen Basis.
  Gibt einen Fehler zurück wenn `n <= 0`, `base <= 0` oder `base = 1`.
- **Parameter:**
  - `n`: Wert.
  - `base`: Logarithmusbasis.
- **Rückgabe:**
  `NumVal`

---

## math.Exp(n)
- **Konkret:**
  Berechnet die Exponentialfunktion (`e ^ n`).
- **Rückgabe:**
  `NumVal`

---

## math.Sin(n)
- **Konkret:**
  Berechnet den Sinus (Eingabe in Radiant).
- **Rückgabe:**
  `NumVal`

---

## math.Cos(n)
- **Konkret:**
  Berechnet den Kosinus (Eingabe in Radiant).
- **Rückgabe:**
  `NumVal`

---

## math.Tan(n)
- **Konkret:**
  Berechnet den Tangens (Eingabe in Radiant).
- **Rückgabe:**
  `NumVal`

---

## math.Asin(n)
- **Konkret:**
  Berechnet den Arkussinus (Ausgabe in Radiant).
- **Rückgabe:**
  `NumVal`

---

## math.Acos(n)
- **Konkret:**
  Berechnet den Arkuskosinus (Ausgabe in Radiant).
- **Rückgabe:**
  `NumVal`

---

## math.Atan(n)
- **Konkret:**
  Berechnet den Arkustangens (Ausgabe in Radiant).
- **Rückgabe:**
  `NumVal`

---

## math.Atan2(y, x)
- **Konkret:**
  Berechnet den Arkustangens aus zwei Koordinaten (`y/x`).
  Berücksichtigt den Quadranten korrekt.
- **Parameter:**
  - `y`: Y-Koordinate.
  - `x`: X-Koordinate.
- **Rückgabe:**
  `NumVal` (Radiant)

---

## math.DegToRad(deg)
- **Konkret:**
  Rechnet Grad in Bogenmaß (Radiant) um.
- **Rückgabe:**
  `NumVal`

---

## math.RadToDeg(rad)
- **Konkret:**
  Rechnet Bogenmaß (Radiant) in Grad um.
- **Rückgabe:**
  `NumVal`

---

## math.Percent(p, v)
- **Konkret:**
  Berechnet `p` Prozent von `v`.
- **Parameter:**
  - `p`: Prozentwert.
  - `v`: Gesamtwert.
- **Rückgabe:**
  `NumVal`
  Beispiel: `math.Percent(15, 200)` → `30`.

---

## math.PercentOf(part, total)
- **Konkret:**
  Berechnet wie viel Prozent `part` von `total` sind.
  Gibt einen Fehler zurück wenn `total = 0`.
- **Parameter:**
  - `part`: Teilwert.
  - `total`: Gesamtwert.
- **Rückgabe:**
  `NumVal`
  Beispiel: `math.PercentOf(30, 200)` → `15`.

---

## math.Pi()
- **Konkret:**
  Gibt die Kreiszahl π zurück.
- **Rückgabe:**
  `NumVal` (`3.141592653589793`)

---

## math.E()
- **Konkret:**
  Gibt die Eulersche Zahl zurück.
- **Rückgabe:**
  `NumVal` (`2.718281828459045`)

---

## math.Val(s)
- **Konkret:**
  Wandelt einen String in eine Zahl um.
  Gibt `0` zurück wenn keine Zahl erkennbar ist (kein Fehler).
  Komma als Dezimaltrennzeichen wird akzeptiert.
- **Parameter:**
  - `s`: String mit numerischem Inhalt.
- **Rückgabe:**
  `NumVal`