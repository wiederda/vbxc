# 🔌 websocket.* – WebSocket-Verbindungen

Dient zum Aufbau und Betrieb von WebSocket-Verbindungen (`ws://` und `wss://`) sowie zur Steuerung von OBS Studio über das OBS-WebSocket-Protokoll v5 (Aufnahme starten, stoppen und abfragen).

Jede Verbindung wird über ein **Handle** (`NumVal`) angesprochen, das `websocket.Connect` bzw. `websocket.OBSConnect` zurückgibt. Eingehende Nachrichten werden im Hintergrund gelesen und in einer Warteschlange (64 Nachrichten) zwischengespeichert, bis sie per `websocket.Receive` abgeholt werden.

---

## websocket.Connect(url [, headers, timeoutMs])
- **Konkret:**
  Stellt eine WebSocket-Verbindung her. Unterstützt `ws://` und `wss://`.
  Die URL muss ein Schema (`ws`/`wss`) und einen Host enthalten.
- **Parameter:**
  - `url`: Ziel-URL, z. B. `"ws://127.0.0.1:4455"`.
  - `headers`: Optional. Map mit zusätzlichen HTTP-Headern für den Handshake (z. B. für Authentifizierung). Leere Schlüssel werden ignoriert.
  - `timeoutMs`: Optional. Timeout für den Verbindungsaufbau in Millisekunden (Standard: `5000`). Muss größer als `0` sein.
- **Rückgabe:**
  `NumVal` (Handle) bei Erfolg.
  `ErrorVal` bei leerer oder ungültiger URL, ungültigem Timeout oder fehlgeschlagenem Verbindungsaufbau (Meldung enthält ggf. den HTTP-Status des Handshakes).

```vb
Dim ws = websocket.Connect("wss://example.com/socket", Nothing, 3000)
If IsError(ws) Then
    Print "Fehler: " & ErrorText(ws)
End If
```

---

## websocket.Send(handle, message)
- **Konkret:**
  Sendet eine Textnachricht über eine bestehende Verbindung.
  Schreibzugriffe sind mit einem Timeout von 10 Sekunden abgesichert.
- **Parameter:**
  - `handle`: Handle der Verbindung.
  - `message`: Nachrichtentext. Andere Typen werden in Text umgewandelt.
- **Rückgabe:**
  `BoolVal(true)` bei Erfolg.
  `ErrorVal` bei ungültigem oder geschlossenem Handle, geschlossener Verbindung oder Sendefehler.

---

## websocket.Receive(handle [, timeoutMs])
- **Konkret:**
  Empfängt die nächste Textnachricht aus der Warteschlange.
  Ohne `timeoutMs` wird blockierend gewartet, bis eine Nachricht eintrifft oder die Verbindung geschlossen wird.
  Mit `timeoutMs` wird nach Ablauf der Zeit ein leerer String zurückgegeben – die Verbindung bleibt dabei bestehen.
  Binärnachrichten werden nicht unterstützt.
- **Parameter:**
  - `handle`: Handle der Verbindung.
  - `timeoutMs`: Optional. Maximale Wartezeit in Millisekunden. Muss größer als `0` sein.
- **Rückgabe:**
  `StrVal` (Nachrichtentext). Leerer String bei Timeout.
  `ErrorVal` bei ungültigem Handle, geschlossener Verbindung, ungültigem Timeout oder empfangener Binärnachricht.

```vb
Do
    msg = websocket.Receive(ws, 1000)
    If msg <> "" Then Print msg
Loop Until Not websocket.IsConnected(ws)
```

---

## websocket.IsConnected(handle)
- **Konkret:**
  Prüft, ob die Verbindung noch nicht als geschlossen erkannt wurde.
  Eine abgebrochene Verbindung wird erst erkannt, sobald der Hintergrund-Leser den Abbruch bemerkt hat.
- **Parameter:**
  - `handle`: Handle der Verbindung.
- **Rückgabe:**
  `BoolVal`
  `False` auch bei ungültigem oder bereits geschlossenem Handle.

---

## websocket.Close(handle)
- **Konkret:**
  Schließt die Verbindung (mit regulärer Close-Nachricht) und entfernt das Handle.
  Das Handle ist danach ungültig.
  Eine über `websocket.OBSStartRecord` gestartete OBS-Aufnahme wird dadurch **nicht** beendet – dafür vorher `websocket.OBSStopRecord` aufrufen.
- **Parameter:**
  - `handle`: Handle der Verbindung.
- **Rückgabe:**
  `BoolVal(true)` bei Erfolg.
  `ErrorVal` bei ungültigem oder bereits geschlossenem Handle.

---

## websocket.OBSConnect(url, password [, timeoutMs])
- **Konkret:**
  Verbindet sich mit OBS-WebSocket v5 und führt die Anmeldung (Hello/Identify, inkl. Passwort-Authentifizierung) durch.
  Ein Handle wird erst nach erfolgreicher Anmeldung zurückgegeben. Schlägt ein Schritt fehl, wird die Verbindung wieder geschlossen.
  Das Handle kann auch mit `websocket.Send`, `websocket.Receive`, `websocket.IsConnected` und `websocket.Close` verwendet werden.
  Die WebSocket-Schnittstelle von OBS muss aktiviert sein (OBS: Werkzeuge → WebSocket-Servereinstellungen; Standardport `4455`).
- **Parameter:**
  - `url`: URL des OBS-WebSocket-Servers, z. B. `"ws://127.0.0.1:4455"`.
  - `password`: Passwort des OBS-WebSocket-Servers. Leerer String, wenn die Authentifizierung in OBS deaktiviert ist.
  - `timeoutMs`: Optional. Timeout in Millisekunden für den Verbindungsaufbau und jeden Anmeldeschritt (Standard: `5000`). Muss größer als `0` sein.
- **Rückgabe:**
  `NumVal` (Handle) bei Erfolg.
  `ErrorVal` bei fehlenden Argumenten, Verbindungsfehler, ungültigem Timeout, fehlendem Passwort (obwohl OBS eines verlangt) oder fehlgeschlagener Anmeldung.

```vb
Dim ws = websocket.OBSConnect("ws://127.0.0.1:4455", "geheim")
If IsError(ws) Then
    Print "OBS nicht erreichbar: " & ErrorText(ws)
End If
```

---

## websocket.OBSGetRecordStatus(handle)
- **Konkret:**
  Prüft, ob OBS aktuell eine Aufnahme durchführt.
  Timeout: 5 Sekunden.
- **Parameter:**
  - `handle`: Handle einer mit `websocket.OBSConnect` aufgebauten Verbindung.
- **Rückgabe:**
  `BoolVal` – `True`, wenn OBS aufnimmt.
  `ErrorVal` bei ungültigem Handle oder fehlgeschlagener Anfrage.

---

## websocket.OBSStartRecord(handle)
- **Konkret:**
  Startet die OBS-Aufnahme mit den aktuell in OBS eingestellten Aufnahmeoptionen (Pfad, Format, Encoder).
  Vor dem Start wird der Status geprüft – nimmt OBS bereits auf, wird keine zweite Aufnahme gestartet, sondern ein Fehler zurückgegeben.
  Timeouts: 5 Sekunden für die Statusprüfung, 10 Sekunden für den Start.
  Die so gestartete Aufnahme wird bei **Strg+C** bzw. `SIGTERM` automatisch gestoppt (siehe [Verhalten beim Abbruch](#verhalten-beim-abbruch)).
- **Parameter:**
  - `handle`: Handle einer mit `websocket.OBSConnect` aufgebauten Verbindung.
- **Rückgabe:**
  `BoolVal(true)`, sobald OBS den Start bestätigt hat.
  `ErrorVal` bei ungültigem Handle, bereits laufender Aufnahme oder fehlgeschlagener Anfrage.

```vb
Dim ws = websocket.OBSConnect("ws://127.0.0.1:4455", "geheim")

If Not websocket.OBSGetRecordStatus(ws) Then
    websocket.OBSStartRecord(ws)
End If
```

---

## websocket.OBSStopRecord(handle)
- **Konkret:**
  Stoppt die OBS-Aufnahme und wartet auf die Bestätigung von OBS.
  Läuft keine Aufnahme, wird ein Fehler zurückgegeben.
  Timeouts: 5 Sekunden für die Statusprüfung, 30 Sekunden für den Stopp (OBS finalisiert dabei die Aufnahmedatei).
- **Parameter:**
  - `handle`: Handle einer mit `websocket.OBSConnect` aufgebauten Verbindung.
- **Rückgabe:**
  `BoolVal(true)` bei Erfolg.
  `ErrorVal` bei ungültigem Handle, nicht laufender Aufnahme oder fehlgeschlagener Anfrage.

```vb
Dim ok = websocket.OBSStopRecord(ws)
If IsError(ok) Then
    Print "Fehler: " & ErrorText(ok)
End If

websocket.Close(ws)
```

---

## Verhalten beim Abbruch

OBS ist ein eigenständiges Programm und beendet eine Aufnahme nicht von selbst, wenn die Verbindung abbricht. Deshalb gilt für Aufnahmen, die per `websocket.OBSStartRecord` gestartet wurden:

| Situation | Aufnahme |
|-----------|----------|
| Strg+C in der Konsole oder `SIGTERM` | wird von VBX gestoppt (Wartezeit bis zu 10 Sekunden) |
| `websocket.OBSStopRecord` im Skript | wird gestoppt |
| `websocket.Close` ohne vorheriges `OBSStopRecord` | läuft weiter |
| Beenden durch den VBX Editor (harter Prozess-Abbruch) | läuft weiter |
| Von OBS selbst oder manuell gestartet | wird von VBX nicht angefasst |

Beim harten Abbruch durch den VBX Editor kann VBX nicht mehr reagieren – die Aufnahme muss dann in OBS manuell beendet werden.

---

## Hinweise zu Nachrichten und Ereignissen

- OBS sendet laufend Ereignisse (z. B. bei Szenenwechseln). Die OBS-Funktionen (`OBSGetRecordStatus`, `OBSStartRecord`, `OBSStopRecord`) warten auf die zugehörige Antwort und **verwerfen** Ereignisse, die währenddessen eintreffen.
- Wer Ereignisse selbst auswerten will, liest sie mit `websocket.Receive` aus. Nachrichten, die gerade nicht abgeholt werden, bleiben bis zu 64 Stück in der Warteschlange.
- Mehrere Verbindungen gleichzeitig sind möglich; jede hat ihr eigenes Handle.