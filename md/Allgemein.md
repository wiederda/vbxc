# 📖 VBX – Kurzreferenz

VBX ist eine modulare, in Go geschriebene Runtime für eine VB-Skriptsprache. Schlanker Kern, Komplexität wird in Module ausgelagert.
Weitere Funktionen können über Plugins eingebunden werden.

---

## Architektur

* **Core Runtime** – Sprachlogik, Kontrollstrukturen, Standardfunktionen
* **Modul-System** – Erweiterungen (Netzwerk, Kryptografie, Datenbanken, ...)
* **Shell-Interface** – Direkte Ausführung von Funktionen über die Kommandozeile

Über **580 Funktionen**, ohne den Kern aufzublähen. Weitere können über Plugins hinzugefügt werden.

---

## Aufruf

```bash
vbx <quelle.vb>
vbx <quelle.vbc>
vbx -modules=json,net <quelle.vb>

vbx -h
vbx -shell -h
vbx -const -h
vbx -modules=file -h
```

---

## Besonderheiten

**Verschlüsselte Skripte** – `vbx Build server.vb` erzeugt `server.vbc`: verschlüsselter Quelltext, Includes bereits aufgelöst, Zielsystem braucht nur die Runtime.

**Hintergrundprozesse** – `vbx Worker server.vb` startet einen eigenen Prozess (`.vb`/`.vbc`), liefert die Prozess-ID zurück, automatische Log-Datei.

**Ausführungsschutz** – prüft jede Datei vor Ausführung (Binärdaten, Encoding, Fehl-Downloads wie HTML-Fehlerseiten), parst das komplette Skript vorab.

**Shell-Interface** – `vbx -shell -h` zeigt direkt aufrufbare Funktionen.

---

## Modul-System

Standardmodule sind immer verfügbar. Weitere Module explizit aktivieren:

```vbx
#use json,net,crypt
```

oder per CLI: `vbx -modules=json,net script.vb`
Modul-Hilfe: `vbx -modules=json -h`

---

## Direktiven

Müssen am Anfang des Skripts stehen.

```vbx
#use zip
#requires 1.0.23
```

`#use` – lädt optionale Module. `#requires` – Mindestversion der Runtime.

---

## Include

```vbx
include "tools.vb"
include "lib/helpers.vb"
```

Relative Pfade, einmaliges Laden, rekursive Includes werden erkannt und Include-Schleifen führen zu einem Fehler.

---

## Namespaces

**Permanent:** app.*, array.*, date.*, file.*, folder.*, global.*, math.*

**Optional:** ad.*, cert.*, computer.*, convert.*, db.*, debug.*, env.*, export.*, geo.*, git.*, json.*, kuma.*, makemkv.*, map.*, net.*, picture.*, proc.*,  reg.*, service.*, sftp.*, smtp.*, ssh.*, string.*, template.*, websocket.*, win.*

**Plugins:** crypt.*, data.*, doc.*, docker.*, fin.*, ini.*, media.*, pgp.*, pqc.*, qr.*, rand.*, steg.*, tar.*, vault.*, xml.*, yaml.*, zip.*

Plugins werden im Verzeichnis `plugins` neben der VBX-Runtime gesucht. Alternativ kann das Plugin-Verzeichnis über `VBX_PLUGIN_PATH` festgelegt werden.

---

## Plugin-Cache

Die kompilierten WASM-Plugins werden von VBX in einem Compilation-Cache gespeichert.

Standardmäßig verwendet VBX dafür:

```text
<vbx-user-cache>/vbx/plugin-cache
C:\Users\<username>\AppData\Local\vbx\plugin-cache
```

Alternativ kann das Plugin-Cache über `VBX_PLUGIN_CACHE` festgelegt werden.

---

## Kommentare

```vbx
' Einzeilig
/' Mehrzeilig '/
```

---

## Datentypen

| Typ | Beschreibung |
|---|---|
| Number | Ganzzahlen und Fließkommazahlen |
| String | Zeichenketten |
| Boolean | Wahr/Falsch |
| Array | Eindimensionales Array |
| Array2D | Zweidimensionales Array |
| Map | Schlüssel/Wert-Struktur |
| Null | Leerer Wert, Literal `Nothing` (siehe `IsNothing`, `IsNull`) |

---

## Operatoren

| Kategorie            | Operatoren                 |
| -------------------- | -------------------------- |
| Arithmetisch         | `+` `-` `*` `/`            |
| Vergleich            | `=` `<>` `<` `>` `<=` `>=` |
| Logisch              | `And` `Or` `Not`           |
| String               | `&` (Verkettung)           |
| Erweiterte Zuweisung | `+=` `-=` `*=` `/=`        |

**`+`** richtet sich nach den Operanden:

| Operanden | Ergebnis |
|---|---|
| Zahl + Zahl | Addition |
| Text + Text | Verkettung (`"12" + "30"` ergibt `1230`) |
| Zahl + Text, Text als Zahl lesbar | Addition (`"12" + 30` ergibt `42`) |
| Zahl + Text, Text nicht als Zahl lesbar | Verkettung (`"Name: " + 5` ergibt `Name: 5`) |

**`-` `*` `/`** wandeln als Zahl lesbare Texte um (`7 - "2"` ergibt `5`), sonst gibt es einen Fehler. Division durch 0 ist ein Fehler. Die Vergleiche `<` `>` `<=` `>=` wandeln ebenso um (`2 <= "2"` ist wahr). `=` und `<>` vergleichen, sobald ein Operand ein Text ist, als Text (Groß-/Kleinschreibung zählt).

**`&`** verkettet immer als String.

**`And` / `Or`** werten den rechten Operanden nur aus, wenn das Ergebnis nicht schon feststeht (Short-Circuit): `If x <> 0 And 10 / x > 1 Then` ist sicher.

**Erweiterte Zuweisung:** `a += b` ist gleichbedeutend mit `a = a + b` (entsprechend `-=`, `*=`, `/=`). Bei einem Fehler bleibt die Variable unverändert.

---

## Klammern

| Typ | Verwendung |
|---|---|
| `( )` | Funktionsaufruf, Array-Zugriff via Index |
| `[ ]` | Map-Zugriff via Key, **nur lesend**, verkettbar |
| `{ }` | Array-Literal |

`[ ]` ist ausschließlich für Maps und nur lesend – schreibend dafür `map.Set(...)` verwenden.

---

## Variablen

**Lokal (`Dim`)** – nur im aktuellen Scope sichtbar, mehrere Deklarationen pro Zeile möglich. Schreibweise: `Dim x = 5`; ohne Startwert ist der Standardwert `0`. In Function/Sub können gleichnamige Variablen mit eigenem Wert existieren (Shadowing, VBX gibt dabei einen Hinweis aus).

**Global (`Public`)** – im gesamten Skript sichtbar, wird in Function/Sub genutzt, wenn keine lokale Variable gleichen Namens existiert.

**Konstant (`Const`)** – wie `Dim`, aber schreibgeschützt: ein Initialwert ist zwingend erforderlich, eine spätere Zuweisung (auch `+=` etc.) bricht das Skript mit einem Fehler ab. Keine Arrays. In Function/Sub kann eine lokale `Const` denselben Namen wie eine äußere `Const`/Variable tragen (Shadowing, wie bei `Dim` – ohne Hinweis-Ausgabe, da bei `Const` ein gewolltes Pattern). Nicht zu verwechseln mit den vb-Konstanten (siehe Abschnitt „Konstanten" unten) – `vbx -const -h` zeigt Letztere.

**Sichtbarkeit in Function/Sub:** Ein `Dim` auf oberster Ebene des Skripts ist in allen Function/Sub sichtbar (lesen und ändern), genau wie `Public`. Function/Sub sehen **nie** die lokalen Variablen ihres Aufrufers – ein benötigter Wert wird als Parameter übergeben oder `Public` deklariert.

---

## Arrays

`Dim a(n)` legt ein Array mit n+1 Elementen an (Index 0 bis n), `Dim m(r, c)` ein 2D-Array. Alle Elemente starten mit `0`. Lesen außerhalb der Grenzen und negative Indizes sind ein Fehler. Schreiben hinter das Ende eines 1D-Arrays vergrößert es automatisch, bei 2D-Arrays nicht. Bei einem 2D-Array liefert `m(i)` die ganze Zeile als 1D-Array, `m(i, j)` ein einzelnes Element.

`b = a` kopiert **nicht**: beide Variablen teilen sich dasselbe Array (wie in VB.NET), eine Änderung über `b` ist auch über `a` sichtbar. Das gilt auch, wenn ein Array an eine Function/Sub übergeben wird. Wächst eines der beiden Arrays, ist es danach vom anderen getrennt.

---

## Optionale Parameter

`Sub`/`Function` unterstützen optionale Parameter per `Optional name = wert` (müssen nach allen Pflichtparametern stehen; `Optional` ist kein reserviertes Wort, es wird nur in Parameterlisten erkannt). Auch `= wert` ohne das Schlüsselwort macht einen Parameter optional. Der Default-Ausdruck wird bei jedem Aufruf neu ausgewertet und kann auf vorherige Parameter zugreifen. Bei zu wenigen/zu vielen Argumenten liefert VBX einen Fehler mit der erwarteten Argumentanzahl (`min`–`max`).

---

## Fehlerbehandlung

Funktionen, die scheitern können, geben einen Fehlerwert (`ErrorVal`) zurück. Sobald dieser Wert verwendet wird – in einem Ausdruck, einer Bedingung, einer Zuweisung, bei `Dim` oder als eigenständiger Aufruf –, bricht das Skript mit einer Fehlermeldung ab. Ein Fehlerwert lässt sich also nicht in einer Variablen aufbewahren. Abfangen lässt sich das mit `Try/Catch`.

`IsError(wert)` – prüft, ob `wert` ein Fehlerwert ist. Funktioniert direkt auf einem Aufruf (`If IsError(Mid(s, 0)) Then`) und auf der Catch-Variablen.
`ErrorText(wert)` – gibt den Fehlertext als String zurück (leer, falls kein Fehler).

Beide sind Sprach-Kernfunktionen, immer verfügbar, unabhängig von `#use`.

### Try / Catch / Finally

Fängt einen Skriptabbruch innerhalb eines Blocks ab, statt das ganze Skript zu beenden:

```vbx
Try
    RiskyCall()
Catch err
    Print "Fehler: " & ErrorText(err)
Finally
    Print "Wird immer ausgeführt"
End Try
```

`Catch` ist optional, `Finally` ist optional – mindestens einer der beiden Zweige muss vorhanden sein. `Finally` läuft in jedem Fall (Erfolg, abgefangener Fehler, oder ein Signal wie `Return`/`Exit For` aus dem Try-Block).

`Try/Catch` fängt jeden Fehler im Block ab, auch bei `Dim x = RiskyCall()`, bei Zuweisungen und bei `+=`. Die Catch-Variable enthält den Fehlerwert, `ErrorText(err)` den Text. Bei verschachtelten Aufrufen enthält der Text die ganze Kette (`Fehler in Sub 'A': Fehler in Sub 'B': …`).

Ein `Try` außen um eine Schleife bricht bei einem Fehler die **gesamte Schleife** ab (restliche Durchläufe werden nicht mehr erreicht), lässt das Skript danach aber normal weiterlaufen. Um nur den fehlerhaften Durchlauf zu überspringen und mit dem Rest der Schleife fortzufahren, muss `Try` **innerhalb** des Schleifenkörpers stehen:

```vbx
For Each x In files
    Try
        RiskyCall(x)
    Catch err
        Print "Fehler bei " & x & ": " & ErrorText(err)
    End Try
Next
```

---

## Maps

Entstehen z. B. über `json.FromJSON` oder `map.Create`. Lesender Zugriff über `[ ]` (auch verkettbar, z. B. `arr(i)["key"]`). Schreibend nur über `map.Set(map, key, wert)`.

---

## Print & Farben

```vbx
Print "Wert: " & x
Print vbRed() & "Fehler" & vbNormal()
```

---

## vb-Konstanten

| Kategorie | Beispiele |
|---|---|
| Logik | `vbTrue()`, `vbFalse()`, `vbNullString()` |
| Formatierung | `vbCrLf()`, `vbNewLine()`, `vbTab()` |
| Farben | `vbBlack()`, `vbRed()`, `vbGreen()`, `vbYellow()`, `vbBlue()`, `vbWhite()`, `vbCyan()`, `vbMagenta()`, `vbLightGray()`, `vbGray()` |
| Hintergrundfarben | `vbBgBlack()`, `vbBgRed()`, `vbBgGreen()`, `vbBgYellow()`, `vbBgBlue()`, `vbBgWhite()`, `vbBgCyan()`, `vbBgMagenta()` |
| Stile | `vbBold()`, `vbUnderline()`, `vbNormal()` |

---

## Reservierte Schlüsselwörter

Folgende Wörter sind reserviert und können nicht als Namen für Variablen, Funktionen, Subs oder Parameter verwendet werden (Groß-/Kleinschreibung spielt keine Rolle):

| Kategorie | Schlüsselwörter |
|---|---|
| Deklaration | `Dim`, `Public`, `Const` |
| Bedingungen | `If`, `Then`, `Else`, `ElseIf` |
| Schleifen | `For`, `Next`, `Each`, `To`, `Step`, `While`, `Do`, `Loop`, `Until`, `In` |
| Schleifensteuerung | `Exit`, `Continue` |
| Prozeduren | `Sub`, `Function`, `Return` |
| Fallunterscheidung | `Select`, `Case`, `Is` |
| Fehlerbehandlung | `Try`, `Catch`, `Finally` |
| Logik | `And`, `Or`, `Not`, `True`, `False`, `Nothing` |
| Sonstiges | `Print`, `End`, `Include` |

---

## Kontrollstrukturen

| Struktur        | Syntax-Skelett                                                                        | Abschluss                |
| --------------- | ------------------------------------------------------------------------------------- | ------------------------ |
| **If**          | `If bed Then` … `[ElseIf bed Then …]` `[Else …]`                                      | `End If`                 |
| **Select Case** | `Select Case ausdruck` `Case wert / wert1, wert2 / x To y / Is > x` `[Case Else]`     | `End Select`             |
| **For**         | `For i = start To end [Step n]`                                                       | `Next [i]`               |
| **For Each**    | `For Each v In array / 2D-array / map` oder `For Each k, v In array / 2D-array / map` | `Next [v]`               |
| **While**       | `While bed`                                                                           | `End While`              |
| **Do Loop**     | `Do [While/Until bed]` … `Loop [While/Until bed]`                                     | `Loop`                   |
| **Continue**    | `Continue For` / `Continue While` / `Continue Do`                                     | –                        |
| **Try/Catch**   | `Try` … `[Catch var …]` `[Finally …]` (mind. Catch oder Finally nötig)                | `End Try`                |
| **Exit**        | `Exit For` / `Exit While` / `Exit Do` / `Exit Sub` / `Exit Function`                  | –                        |
| **Sub**         | `Sub Name(param1, param2 [, Optional param3 = wert])`                                 | `End Sub`                |
| **Function**    | `Function Name(...)` … `Return wert` oder `Name = wert`                               | `End Function`           |
| **Cls**         | `Cls()`                                                                               | –                        |
| **Print**       | `Print wert`                                                                          | –                        |

**For Each:** Mit einer Variable (`For Each v In …`) liefert ein 1D-Array die Elemente, ein 2D-Array die Zeilen (jeweils als 1D-Array), eine Map die Werte. Mit zwei Variablen (`For Each k, v In …`) liefern Arrays Index und Element, Maps Schlüssel und Wert. Maps werden nach Schlüssel sortiert durchlaufen.

**Select Case:** Es wird der erste passende `Case` ausgeführt. Überlappende Werte in mehreren `Case`-Zweigen sind erlaubt, der erste Treffer gewinnt.

**Continue:** `Continue For`, `Continue While` und `Continue Do` überspringen nur den Rest des aktuellen Schleifendurchlaufs und setzen die Schleife mit dem nächsten Durchlauf fort. Bei `Do … Loop While/Until` springt `Continue Do` zur Bedingung am Ende, die dann normal ausgewertet wird. Im Unterschied dazu beenden `Exit For`, `Exit While` und `Exit Do` die jeweilige Schleife vollständig.