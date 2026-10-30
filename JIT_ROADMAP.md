# JIT-Roadmap: Bytecode schrittweise nativ ausführen

## Ziel und Rahmen

Der JIT soll unter `amd64` mehr VBX-Code direkt als Maschinencode ausführen.
Die VM bleibt dauerhaft als Fallback erhalten und ist die Referenz für
Konvertierungen, Fehler und Sonderfälle. Unterstützte Laufzeitfälle dürfen
deoptieren: Der JIT kehrt vor dem betroffenen Opcode zurück, und die VM führt
ihn vollständig aus.

Windows und Linux sind die unterstützten Zielplattformen. macOS ist wegen
`amd64` grundsätzlich ein möglicher Build, wurde aber nicht getestet und ist
hier kein zugesichertes Ziel.

Aufwand meint den relativen Implementierungs- und Absicherungsaufwand:

- **Niedrig:** klare primitive Semantik, wenige Guards, keine Go-Zeiger
- **Mittel:** mehrere Laufzeitfälle, zusätzliche Encoderarbeit oder genaue
  Sonderfallbehandlung
- **Hoch:** Go-Objekte/GC, VM-Aufrufkonvention, Scope- oder Fehlersemantik

## Bereits nativ (mit Einschränkungen)

- Zahlenarithmetik, numerische Vergleiche, boolesche Logik und Sprünge
- Schleifensteuerung für `For` mit numerischen Variablen
- begrenzte Variablen-Lese-/Schreibpfade für Zahl, Bool und Null
- ausgewählte `Select Case`- und Vergleichs-Opcodes
- numerische 1D-/2D-Arrayzugriffe und Stores unter Guards
- `math.Abs`, `math.Sign`, `math.Sqrt`, `math.Min` und `math.Max` für
  numerische Argumente
- `math.Floor`, `math.Ceil`, `math.Trunc` und `math.Int` für numerische
  Argumente (ab SSE4.1 bitidentisch über `roundsd`; ohne CPU-Fähigkeit
  deoptieren die Aufrufe zur VM)
- `math.Mod` für numerische Argumente (`cvttsd2si`+`idiv` mit Deopt der
  Grenzfälle int(y)==0, INT64-min und Nicht-Num-Operanden)
- `math.DegToRad` und `math.RadToDeg` für numerische Argumente; die Fast Paths
  behalten die Operationsreihenfolge der Standardbibliothek bei
<<<<<<< HEAD
- `math.Percent` und `math.PercentOf` für numerische Argumente; bei
  `PercentOf` mit Nenner 0 oder NaN deoptiert der JIT zur VM, damit der
  vorhandene Fehler- bzw. Sonderfallpfad erhalten bleibt
- `math.Pi()` und `math.E()` als native numerische Konstanten; ein
  gleichnamiger Wert im Skript-Scope führt weiterhin zur VM-Auflösung
=======
>>>>>>> 1a75fee5075b59ae56e731fda08bf18a063c2ad9
- `For Each` über 1D-Arrays (`OpEachInit`/`OpEachNext`): Elemente der
  Kinds Num/Bool/Null werden nativ in die Loop-Variablen geschrieben,
  die Zwei-Variablen-Form (`For Each k, v`) schreibt den Num-Schlüssel
  mit; Str-/Arr-/Map-Elemente, 2D, Maps und fehlende Loop-Variablen
  deoptieren pro Fall zur VM (Details: CHANGES.md, Runde 6)

Bei anderen Werten, dynamischen Fehlerfällen und überschatteten Built-ins
deoptieren diese Pfade zur VM.

## Vorgeschlagene Reihenfolge

| Priorität | Bereich | Aufwand | Warum noch VM / Abgrenzung |
|---|---|---:|---|
<<<<<<< HEAD
| 1 | ~~Weitere einfache numerische `math.*`-Funktionen, zuerst `Floor`, `Ceil`, `Trunc` und `Int`~~ **erledigt** | Mittel | `roundsd` (SSE4.1) liefert `Floor`/`Ceil`/`Trunc`/`Int` bitidentisch zu Go (NaN, ±Inf, ±0 laufen durch); CPU-Fähigkeit wird geprüft, Reste deoptieren. `Round` und `RoundBank` sind inzwischen für `Trunc(d)==0` ebenfalls nativ abgedeckt; Fälle mit anderen Rundungsstellen deoptieren. |
| 2 | Numerische Fast Paths für weitere `math.*`-Funktionen | Mittel bis hoch | `DegToRad`, `RadToDeg`, `Percent`, `PercentOf`, `Round` (Bit-Spiegelung des Go-Algorithmus) und `RoundBank` (roundsd Ties-Even) sind nativ (Runde 7/8; nur `Trunc(d)==0` bei Round/RoundBank). NaN als `d` deoptiert korrekt zur VM. Offen: `Pow`, `Log`, `Exp` und trigonometrische Funktionen - sie brauchen Spezialfälle oder eine Bibliotheksimplementierung. Zunächst nur Funktionen aufnehmen, deren Ergebnis und Fehlerfälle exakt abgeglichen sind. |
| 3 | Häufige einfache Standardfunktionen mit primitiven Ergebnissen | Mittel | `math.Pi()` und `math.E()` liefern ihre numerischen Konstanten nativ; `math.Clamp` läuft als Komposition `Max(lo, Min(x, hi))` aus den nativen Min/Max-Pfaden (Runde 9). Viele globale/string-/convert-Funktionen konvertieren Strings, erstellen Strings oder geben Fehler zurück. Pro Funktion einzeln entscheiden; Konvertierungen und Fehler weiter deoptieren lassen. |
=======
| 1 | ~~Weitere einfache numerische `math.*`-Funktionen, zuerst `Floor`, `Ceil`, `Trunc` und `Int`~~ **erledigt** | Mittel | `roundsd` (SSE4.1) liefert `Floor`/`Ceil`/`Trunc`/`Int` bitidentisch zu Go (NaN, ±Inf, ±0 laufen durch); CPU-Fähigkeit wird geprüft, Reste deoptieren. `Round`/`RoundBank` bleiben wie geplant separat. |
| 2 | Numerische Fast Paths für weitere `math.*`-Funktionen | Mittel bis hoch | `DegToRad` und `RadToDeg` sind für numerische Argumente nativ. Funktionen wie `Pow`, `Log`, `Exp`, trigonometrische Funktionen und Prozent-Helfer brauchen Spezialfälle oder eine Bibliotheksimplementierung. Zunächst nur Funktionen aufnehmen, deren Ergebnis und Fehlerfälle exakt abgeglichen sind. |
| 3 | Häufige einfache Standardfunktionen mit primitiven Ergebnissen | Mittel | Viele globale/string-/convert-Funktionen konvertieren Strings, erstellen Strings oder geben Fehler zurück. Pro Funktion einzeln entscheiden; Konvertierungen und Fehler weiter deoptieren lassen. |
>>>>>>> 1a75fee5075b59ae56e731fda08bf18a063c2ad9
| 4 | Variablenzugriffe für weitere Value-Kinds | Hoch | Strings, Arrays, Maps, Objekte und Fehler enthalten Go-Zeiger. Lesen/Schreiben muss GC-sicher sein und bei Stores die Write Barrier beachten. |
| 5 | Allgemeine Array-Lesezugriffe und -Stores | Hoch | Der heutige native Pfad deckt nur numerische Elemente und enge Fälle ab. Beliebige Werte erfordern sichere Kopien von Go-Zeigern, Bounds-/Typfehler und 1D-/2D-Semantik. |
| 6 | Array-Erzeugung, Dimensionierung ~~und `For Each`~~ (ForEach: **erledigt**, siehe Runde 6) | Hoch | `For Each` über 1D-Arrays läuft nativ (EachInit/EachNext, Elemente Num/Bool/Null, sonst Deopt). Die Erzeugung/Dimensionierung von Arrays im Maschinencode wäre ein neuer Teil: Allokationen müssen über Go erfolgen; als erstes wäre ein sicherer Laufzeit-Helper statt direkter Allokation im Maschinencode zu prüfen. |
| 7 | String-Verkettung und Stringvergleiche mit VBX-Semantik | Hoch | Erzeugt Go-Strings und damit GC-sichtbare Zeiger. Vergleichsregeln müssen mit der VM übereinstimmen (u. a. case-insensitive Pfade). |
| 8 | Map-Zugriffe | Hoch | Map-Lookup/-Mutation, dynamische Schlüssel, Fehler und Go-Runtime-Aufrufe sind eng an die VM und Go-Laufzeit gebunden. Zunächst VM-Helper untersuchen, keine Map-Manipulation per Rohzeiger. |
| 9 | User-Funktionen und allgemeine Built-in-Aufrufe | Hoch | Benötigen Argument-/Parameterbehandlung, Defaults, Scopes, Rückgaben und Fehlerbehandlung. Sinnvoll erst nach einer stabilen JIT-Aufrufkonvention oder über VM-Call-Helper. |

## Bewusst bei der VM lassen

- `Print`, `Cls` und sonstige Ein-/Ausgabe: Seiteneffekte und Plattform-I/O
  bringen kaum Nutzen durch Inline-Maschinencode.
- Datei-, Netzwerk-, Datenbank-, Prozess- und Plugin-Funktionen: externe
  Seiteneffekte; die VM ist hier der natürliche Aufrufpfad.
- Fehlerobjekte erzeugen und Fehlertexte formatieren: Die VM bleibt die
  Semantik-Referenz; ein JIT-Pfad kann später nur dann sinnvoll sein, wenn er
  exakt dieselbe Fehlerbehandlung nutzt.

Diese Bereiche dürfen weiterhin innerhalb eines Skripts vorkommen. Der JIT
führt die angrenzenden unterstützten Regionen aus und übergibt an der Stelle
an die VM.

## Kriterien für jeden neuen Fast Path

1. VM-Verhalten für normale Werte, Randfälle und Fehler dokumentieren.
2. Nur Fälle nativ ausführen, deren Voraussetzungen zur Laufzeit geprüft sind.
3. Bei jeder Unsicherheit vor der Änderung des VM-Zustands deoptimieren.
4. Go-Zeiger nie ohne passende GC-/Write-Barrier-Strategie kopieren oder
   schreiben.
5. Differentialtests mit aktiviertem JIT und reinem VM-Pfad ergänzen;
   Windows/amd64 und Linux/amd64 bauen. macOS erst nach eigenem Build- und
   Laufzeittest als unterstützt markieren.
6. Erst nach Messung entscheiden, ob der Fast Path den Aufwand rechtfertigt.

Die Prioritäten sind eine Arbeitsreihenfolge, keine Zusage, jeden VBX-Befehl
aus dem JIT heraus nativ zu implementieren. Die VM bleibt dauerhaft verfügbar.
