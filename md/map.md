# 🗺️ map.* – Map-Funktionen

Dient zur Erstellung und Verwaltung von Schlüssel/Wert-Strukturen (`KindMap`).
Alle mutierenden Operationen arbeiten In-Place auf der übergebenen Map.

---

## map.Create()
- **Konkret:**
  Erstellt eine leere Map.
- **Rückgabe:**
  `KindMap`

---

## map.Set(map, key, value)
- **Konkret:**
  Setzt einen Wert in der Map (In-Place).
- **Parameter:**
  - `map`: Ziel-Map.
  - `key`: Schlüssel als String.
  - `value`: Zu setzender Wert.
- **Rückgabe:**
  `KindMap` (die modifizierte Map)

---

## map.Get(map, key)
- **Konkret:**
  Liest einen Wert aus der Map.
  Gibt `NullVal` zurück wenn der Schlüssel nicht existiert.
- **Parameter:**
  - `map`: Quell-Map.
  - `key`: Schlüssel als String.
- **Rückgabe:**
  Entsprechender `Value`-Typ oder `NullVal`.

---

## map.ContainsKey(map, key)
- **Konkret:**
  Prüft ob ein Schlüssel in der Map existiert.
- **Parameter:**
  - `map`: Zu prüfende Map.
  - `key`: Schlüssel als String.
- **Rückgabe:**
  `BoolVal`

---

## map.Remove(map, key)
- **Konkret:**
  Entfernt einen Schlüssel aus der Map (In-Place).
  Existiert der Schlüssel nicht, wird kein Fehler zurückgegeben.
- **Parameter:**
  - `map`: Ziel-Map.
  - `key`: Zu entfernender Schlüssel.
- **Rückgabe:**
  `KindMap` (die modifizierte Map)

---

## map.Clear(map)
- **Konkret:**
  Entfernt alle Einträge aus der Map (In-Place).
- **Parameter:**
  - `map`: Zu leerende Map.
- **Rückgabe:**
  `KindMap` (leere Map)

---

## map.Keys(map)
- **Konkret:**
  Gibt alle Schlüssel der Map als Array zurück.
  Reihenfolge ist nicht garantiert.
- **Parameter:**
  - `map`: Quell-Map.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen.

---

## map.Values(map)
- **Konkret:**
  Gibt alle Werte der Map als Array zurück.
  Reihenfolge ist nicht garantiert.
- **Parameter:**
  - `map`: Quell-Map.
- **Rückgabe:**
  `ArrVal`

---

## map.Count(map)
- **Konkret:**
  Gibt die Anzahl der Einträge in der Map zurück.
- **Parameter:**
  - `map`: Quell-Map.
- **Rückgabe:**
  `NumVal`

---

## map.Clone(map)
- **Konkret:**
  Erstellt eine flache Kopie der Map (Shallow Copy).
- **Parameter:**
  - `map`: Quell-Map.
- **Rückgabe:**
  `KindMap`

---

## map.ToString(map)
- **Konkret:**
  Gibt eine lesbare String-Darstellung der Map zurück.
  Geeignet für Debug-Ausgaben.
- **Parameter:**
  - `map`: Quell-Map.
- **Rückgabe:**
  `StrVal`
  Format: `{key1: val1, key2: val2}`.