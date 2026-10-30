# 🌍 geo.* – Geolokalisierungsfunktionen

Dient zur Auflösung von Ortsnamen in geografische Koordinaten.
Nutzt die OpenStreetMap Nominatim API. Erfordert eine aktive Internetverbindung.

---

## geo.Lookup(city)
- **Konkret:**
  Sucht Geodaten zu einem Ortsnamen via OpenStreetMap Nominatim.
  Gibt den ersten Treffer zurück.
- **Parameter:**
  - `city`: Ortsname (z. B. `"Berlin"`, `"Hamburg"`).
- **Rückgabe:**
  `ArrVal`
  Format: `[Stadt, Region, Land, Lat, Lon]`
  Bei Fehler: Array mit einem `"error: ..."` String als einzigem Element.