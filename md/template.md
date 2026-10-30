# 📝 template.* – Template-Funktionen

Dient zur Erzeugung von Texten aus Vorlagen mit Platzhaltern.
Nutzt Go's `text/template`-Engine. Platzhalter werden mit `{{.Schlüssel}}` angegeben.
Datenquelle ist immer eine Map (`KindMap`) oder ein Array (`KindArr`).

---

## Template-Syntax

| Ausdruck | Beschreibung |
|----------|-------------|
| `{{.Name}}` | Wert des Schlüssels `Name` aus der Map |
| `{{.}}` | Gesamter Datenwert (z. B. bei einfachen Strings) |
| `{{if .Aktiv}}...{{end}}` | Bedingte Ausgabe |
| `{{range .Liste}}{{.}}{{end}}` | Iteration über ein Array |
| `{{.Name \| printf "%-10s"}}` | Formatierung via Pipeline |

---

## template.Render(templateStr, data)
- **Konkret:**
  Rendert einen Template-String mit einer Map oder einem Array als Datenquelle.
- **Parameter:**
  - `templateStr`: Template als String mit `{{.Schlüssel}}`-Platzhaltern.
  - `data`: `KindMap` oder `ArrVal` als Datenquelle.
- **Rückgabe:**
  `StrVal` (gerenderter Text), `ErrorVal` bei ungültigem Template oder Ausführungsfehler.

---

## template.RenderFile(path, data)
- **Konkret:**
  Lädt ein Template aus einer Datei und rendert es.
- **Parameter:**
  - `path`: Pfad zur Template-Datei.
  - `data`: `KindMap` oder `ArrVal` als Datenquelle.
- **Rückgabe:**
  `StrVal` (gerenderter Text), `ErrorVal` bei Lesefehler oder ungültigem Template.

---

## template.RenderToFile(templatePath, outPath, data)
- **Konkret:**
  Rendert ein Template aus einer Datei direkt in eine Ausgabedatei.
  Schreibt atomar (temp-Datei + Rename).
- **Parameter:**
  - `templatePath`: Pfad zur Template-Datei.
  - `outPath`: Pfad zur Ausgabedatei.
  - `data`: `KindMap` oder `ArrVal` als Datenquelle.
- **Rückgabe:**
  `StrVal` (`"OK"`), `ErrorVal` bei Fehler.