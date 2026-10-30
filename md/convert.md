# 🔄 convert.* – Einheitenkonvertierung

Dient zur Umrechnung von Einheiten zwischen verschiedenen Kategorien sowie zur Konvertierung von Zahlensystemen.

---

## convert.Unit(category, from, to, value)
- **Konkret:**
  Konvertiert einen Wert von einer Einheit in eine andere innerhalb einer Kategorie.
  Temperatur wird über Kelvin als Zwischenschritt umgerechnet (Intervallskala).
  Alle anderen Kategorien nutzen Faktoren (Verhältnisskala).
- **Parameter:**
  - `category`: Einheitenkategorie (case-insensitiv).
  - `from`: Quelleinheit.
  - `to`: Zieleinheit.
  - `value`: Zu konvertierender Wert.
- **Unterstützte Kategorien und Einheiten:**

  **length** – `mm`, `cm`, `m`, `meter`, `km`, `inch`, `ft`, `foot`, `yard`, `mile`

  **weight** – `mg`, `g`, `gram`, `kg`, `t`, `oz`, `lb`, `pound`

  **volume** – `ml`, `l`, `liter`, `gal`, `gallon`

  **speed** – `kmh`, `mph`

  **temp** – `C`, `F`, `K`, `R`

- **Rückgabe:**
  `NumVal`, `ErrorVal` bei unbekannter Kategorie oder Einheit.

---

## convert.Categories()
- **Konkret:**
  Gibt alle verfügbaren Einheitenkategorien zurück.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen.

---

## convert.Units(category)
- **Konkret:**
  Gibt alle verfügbaren Einheiten einer Kategorie zurück.
- **Parameter:**
  - `category`: Kategoriename.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen, `ErrorVal` bei unbekannter Kategorie.

---

## convert.HexToDec(hexStr)
- **Konkret:**
  Wandelt einen Hexadezimal-String in eine Dezimalzahl um.
  Optionales `0x`-Präfix wird automatisch entfernt.
- **Parameter:**
  - `hexStr`: Hex-String (z. B. `"0xFF"` oder `"FF"`).
- **Rückgabe:**
  `NumVal`, `ErrorVal` bei ungültigem Hex-String.

---

## convert.DecToHex(val)
- **Konkret:**
  Wandelt eine Dezimalzahl in einen Hexadezimal-String um.
- **Parameter:**
  - `val`: Ganzzahl.
- **Rückgabe:**
  `StrVal`
  Format: `"0x..."` in Großbuchstaben. Beispiel: `255` → `"0xFF"`.