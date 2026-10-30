# 🌐 net.* – Netzwerkfunktionen

Dient zur Kommunikation über HTTP, TCP und DNS sowie zur Abfrage lokaler und externer Netzwerkinformationen.

---

## net.Ping(host [, port])
- **Konkret:**
  Prüft die TCP-Erreichbarkeit eines Hosts.
  Timeout: 2 Sekunden.
- **Parameter:**
  - `host`: Hostname oder IP.
  - `port`: Optional. Port (Standard: `443`).
- **Rückgabe:**
  `BoolVal`

---

## net.Connect(host, port)
- **Konkret:**
  Prüft eine TCP-Verbindung zu einem Host und Port.
  Timeout: 3 Sekunden.
- **Parameter:**
  - `host`: Hostname oder IP.
  - `port`: Port als String oder Zahl.
- **Rückgabe:**
  `BoolVal`

---

## net.Available(url)
- **Konkret:**
  Prüft ob eine URL erreichbar ist (HTTP-Statuscode 200–299).
  Versucht zuerst HEAD, dann GET als Fallback.
  Timeout: 5 Sekunden.
- **Parameter:**
  - `url`: Vollständige URL.
- **Rückgabe:**
  `BoolVal`

---

## net.Get(url [, token, retries, baseDelayMs, maxDelayMs])
- **Konkret:**
  Führt eine HTTP-GET-Anfrage aus.
  Speichert den HTTP-Statuscode intern (abrufbar via `net.LastStatus`).
  Bei Netzwerkfehlern, HTTP 5xx oder HTTP 429 (Rate-Limit) wird die Anfrage bis zu `retries`-mal wiederholt, mit exponentiellem Backoff zwischen den Versuchen. Andere 4xx-Statuscodes (401, 403, 404, ...) werden nicht wiederholt, da ein erneuter Versuch am Ergebnis nichts ändern würde.
- **Parameter:**
  - `url`: Ziel-URL.
  - `token`: Optional. Auth-Token, siehe [Auth-Token-Präfixe](#auth-token-präfixe).
  - `retries`: Optional. Anzahl zusätzlicher Versuche nach dem ersten (Standard: `0` – kein Retry, ein einziger Versuch).
  - `baseDelayMs`: Optional. Basis-Wartezeit vor dem zweiten Versuch in Millisekunden (Standard: `500`). Verdoppelt sich mit jedem weiteren Versuch.
  - `maxDelayMs`: Optional. Obergrenze für die Wartezeit zwischen Versuchen (Standard: `30000`).
- **Rückgabe:**
  `StrVal` (Response-Body) bei Erfolg (HTTP 2xx).
  `ErrorVal` nach Ausschöpfen aller Versuche bzw. bei nicht wiederholbarem Statuscode – Meldung enthält HTTP-Status und Body bzw. den 

---

## net.Post(url, data [, token, retries, baseDelayMs, maxDelayMs])
- **Konkret:**
  Führt eine HTTP-POST-Anfrage aus.
  Erkennt JSON automatisch (`{` oder `[` am Anfang) und setzt `Content-Type: application/json`.
  Sonst: `application/x-www-form-urlencoded`.
  Retry-Verhalten identisch zu `net.Get` (5xx/429/Netzwerkfehler mit exponentiellem Backoff, andere 4xx sofort final).
- **Parameter:**
  - `url`: Ziel-URL.
  - `data`: Request-Body als String.
  - `token`: Optional. Auth-Token (gleiche Präfixe wie `net.Get`).
  - `retries`: Optional. Siehe `net.Get` (Standard: `0`).
  - `baseDelayMs`: Optional. Siehe `net.Get` (Standard: `500`).
  - `maxDelayMs`: Optional. Siehe `net.Get` (Standard: `30000`).
- **Rückgabe:**
  `StrVal` (Response-Body) bei Erfolg (HTTP 2xx).
  `ErrorVal` nach Ausschöpfen aller Versuche bzw. bei nicht wiederholbarem Statuscode.

---

---

## net.PostFile(url, dateipfad, contentType [, token, retries, baseDelayMs, maxDelayMs, timeoutMs])
- **Konkret:**
  Sendet den rohen Binärinhalt einer Datei per POST, mit explizit vorgegebenem Content-Type – im Unterschied zu `net.Post`, das den Body nur als String mit automatischer JSON/Form-Erkennung sendet.
  Gedacht für Datei-Uploads, bei denen der Empfänger einen bestimmten Content-Type erwartet, z. B. GitHub-Release-Asset-Uploads (`application/zip`).
  Kein festes Timeout wie bei `net.Get`/`net.Post` (15s) – Standard hier `15000` ms, über `timeoutMs` bei großen Dateien/langsamen Verbindungen erhöhbar.
  Retry-Verhalten identisch zu `net.Get`/`net.Post` (5xx/429/Netzwerkfehler mit exponentiellem Backoff, andere 4xx sofort final).
- **Parameter:**
  - `url`: Ziel-URL.
  - `dateipfad`: Pfad zur hochzuladenden Datei.
  - `contentType`: MIME-Type des Bodys (z. B. `"application/zip"`). Leer → `"application/octet-stream"`.
  - `token`: Optional. Auth-Token (gleiche Präfixe wie `net.Get`, siehe [Auth-Token-Präfixe](#auth-token-präfixe)).
  - `retries`: Optional. Siehe `net.Get` (Standard: `0`).
  - `baseDelayMs`: Optional. Siehe `net.Get` (Standard: `500`).
  - `maxDelayMs`: Optional. Siehe `net.Get` (Standard: `30000`).
  - `timeoutMs`: Optional. Timeout in Millisekunden (Standard: `15000`).
- **Rückgabe:**
  `StrVal` (Response-Body) bei Erfolg (HTTP 2xx).
  `ErrorVal` nach Ausschöpfen aller Versuche bzw. bei nicht wiederholbarem Statuscode.

```vb
result = net.PostFile(uploadUrl, "release.zip", "application/zip", "gh:" & githubToken)

If IsError(result) Then
    Print "Fehler beim Hochladen: " & ErrorText(result)
End If
```

---

## net.PostForm(url, token, title, message [, link])
- **Konkret:**
  Sendet eine JSON-POST-Anfrage mit `title` und `message` als Felder.
  Geeignet für Push-Dienste wie Gotify.
  Optional kann ein `link` übergeben werden: Er wird als Markdown-Link (`[link](link)`) an `message` angehängt und zusätzlich über `extras` gesetzt – `client::display` (Content-Type `text/markdown`, damit der Link in der Nachrichtenliste als echter Hyperlink gerendert wird) sowie `client::notification` (macht die Notification selbst anklickbar, öffnet `link` beim Antippen).
  Achtung: `token` wird hier direkt als URL-Parameter angehängt – **keine** Präfix-Logik wie bei `net.Get`/`net.Download`.
- **Parameter:**
  - `url`: Basis-URL des Dienstes.
  - `token`: Auth-Token (wird als URL-Parameter angehängt).
  - `title`: Nachrichtentitel.
  - `message`: Nachrichtentext.
  - `link`: Optional. Link, der an die Nachricht angehängt und über `extras` klickbar gemacht wird (siehe oben).
- **Rückgabe:**
  `StrVal` (Response-Body), `ErrorVal` bei Fehler oder HTTP 4xx/5xx.

```vb
' Ohne Link
net.PostForm(url, token, "Titel", "Nachricht")

' Mit klickbarem Link
net.PostForm(url, token, "Titel", "Nachricht", "https://example.com")
```

---

## net.Download(url [, path, token, retries, baseDelayMs, maxDelayMs])
- **Konkret:**
  Lädt eine Datei per HTTP-GET herunter und speichert sie lokal.
  Ohne Pfadangabe wird der Dateiname aus dem letzten Segment der URL abgeleitet (Fallback: `"downloaded_file"`).
  Kein Timeout auf den HTTP-Client selbst – geeignet für große Dateien.
  Es wird kein Verzeichnis automatisch angelegt; das Zielverzeichnis muss existieren. Eine bereits vorhandene Datei am Zielpfad wird ohne Rückfrage überschrieben (auch zwischen Retry-Versuchen).
  Der `User-Agent`-Header ist fest auf `"VBX/1.0"` gesetzt.
  Retry-Verhalten identisch zu `net.Get`/`net.Post`.
- **Parameter:**
  - `url`: Download-URL.
  - `path`: Optional. Zielpfad inkl. Dateiname. Relative Pfade werden über `absPathVal` aufgelöst.
  - `token`: Optional. Auth-Token, siehe [Auth-Token-Präfixe](#auth-token-präfixe).
  - `retries`: Optional. Siehe `net.Get` (Standard: `0`).
  - `baseDelayMs`: Optional. Siehe `net.Get` (Standard: `500`).
  - `maxDelayMs`: Optional. Siehe `net.Get` (Standard: `30000`).
- **Rückgabe:**
  `StrVal` (der Zielpfad) bei Erfolg.
  `ErrorVal` bei fehlender/leerer URL, ungültigem Zielpfad, nicht wiederholbarem HTTP-Statuscode, oder nach Ausschöpfen aller Versuche bei Netzwerkfehlern/5xx/429.

---

## Auth-Token-Präfixe

Gilt für den `token`-Parameter von `net.Get` und `net.Download` (intern: `setAuthHeader`). Die Präfixe sind case-sensitive und werden per `strings.HasPrefix` erkannt, danach wird der Präfix entfernt und der Rest als Header-Wert verwendet.

| Präfix | Header | Beispiel | Verwendung |
|--------|--------|----------|------------|
| `gl:`  | `PRIVATE-TOKEN: <wert>` | `"gl:glpat-xxx"` | GitLab Personal Access Token |
| `gh:`  | `Authorization: Bearer <wert>` | `"gh:ghp_xxx"` | GitHub Token |
| `b:`   | `Authorization: Bearer <wert>` | `"b:mein-token"` | Bearer-Token erzwingen |
| `a:`   | `Authorization: <wert>` (unverändert) | `"a:Basic dXNlcjpwYXNz"` | Kompletter, selbst zusammengesetzter Authorization-Header |
| *(kein Präfix)* | `Authorization: Bearer <wert>` | `"mein-token"` | Standardfall: Bearer |

Ist `token` leer (nach Trim), wird **kein** Auth-Header gesetzt.

```vb
result = net.Get(url, "gh:" & githubToken)
result = net.Download(url, path, "gl:" & gitlabToken)
result = net.Download(url, path, "a:" & "Basic " & base64Cred)
```

---

## net.Status([url])
- **Konkret:**
  Ohne Parameter: gibt den HTTP-Statuscode der letzten `Get`/`Post`-Anfrage zurück.
  Mit URL: führt einen HEAD-Request durch und gibt dessen Statuscode zurück.
- **Parameter:**
  - `url`: Optional. URL für aktive Prüfung.
- **Rückgabe:**
  `NumVal` (HTTP-Statuscode, `0` bei Fehler oder noch keiner Anfrage)

---

## net.LastStatus()
- **Konkret:**
  Gibt den HTTP-Statuscode der letzten `Get`/`Post`/`PostForm`-Anfrage zurück.
- **Rückgabe:**
  `NumVal`

---

## net.Resolve(host, type [, dnsServer])
- **Konkret:**
  Löst einen Hostnamen in eine einzelne IPv4- oder IPv6-Adresse auf.
  Optional kann ein spezifischer DNS-Server für die Anfrage verwendet werden. Die Auflösung ist mit einem Timeout von 5 Sekunden abgesichert.
- **Parameter:**
  - `host`: Hostname.
  - `type`: Adresstyp – `"IP4"` oder `"IP6"`. Bei anderen/leeren Werten wird die erste gefundene Adresse zurückgegeben (unabhängig vom Typ).
  - `dnsServer`: Optional. DNS-Server für die Anfrage (z. B. `"8.8.8.8"`).
- **Rückgabe:**
  `StrVal` (aufgelöste Adresse). Leerer String wenn keine Adresse zum gewünschten Typ gefunden wurde. `ErrorVal` bei fehlenden Pflichtargumenten oder fehlgeschlagener Auflösung.

---

## net.ResolveAll(host)
- **Konkret:**
  Löst einen Hostnamen in alle verfügbaren IPv4- und IPv6-Adressen auf.
- **Parameter:**
  - `host`: Hostname.
- **Rückgabe:**
  `ArrVal`
  Format: `[1, 4, "ip", 6, "ip", ...]`
  Index 0 = `1` bei Erfolg, `0` bei Fehler. Danach abwechselnd IP-Version (`4` oder `6`) und IP-String.

---

## net.CanResolve(host [, dnsServer])
- **Konkret:**
  Prüft ob ein Hostname aufgelöst werden kann.
  Protokoll-Präfixe und Pfade werden automatisch entfernt.
  Optional kann ein spezifischer DNS-Server angegeben werden.
- **Parameter:**
  - `host`: Hostname oder URL.
  - `dnsServer`: Optional. DNS-Server (z. B. `"8.8.8.8"` oder `"8.8.8.8:53"`).
- **Rückgabe:**
  `BoolVal`

---

## net.LocalIPs()
- **Konkret:**
  Gibt alle aktiven lokalen IPv4-Adressen zurück.
  Loopback, Link-Local und nicht-spezifizierte Adressen werden ignoriert.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen.

---

## net.MACs()
- **Konkret:**
  Listet die MAC-Adressen aller aktiven Netzwerkinterfaces auf.
  Null-Adressen werden ignoriert.
- **Rückgabe:**
  `ArrVal`
  Array von `StrVal`-Einträgen im Format `"xx:xx:xx:xx:xx:xx"`.

---

## net.PublicIP()
- **Konkret:**
  Ermittelt die externe (öffentliche) IP-Adresse über externe Provider.
  Fragt nacheinander `ipify.org`, `ifconfig.me` und `checkip.amazonaws.com` ab.
  Timeout: 5 Sekunden je Provider.
- **Rückgabe:**
  `StrVal`
  Leerer String wenn kein Provider erreichbar ist.

---

## net.IPIsValid(ip)

- **Konkret:** Prüft, ob ein String eine gültige IPv4- oder IPv6-Adresse ist.
- **Parameter:** `ip` – zu prüfender String
- **Rückgabe:** `True` wenn gültig, sonst `False`

```vb
If net.IPIsValid("192.168.1.1") Then Print "OK"
If net.IPIsValid(net.PublicIP()) Then Print "Externe IP ist gültig"
```

---

## net.WakeOnLan(mac [, broadcastIP, port])
- **Konkret:**
  Sendet ein Wake-on-LAN Magic Packet per UDP-Broadcast, um ein Gerät im lokalen Netzwerk aus dem Standby/ausgeschalteten Zustand zu starten.
  Löst das bisherige eigenständige `wol`-CLI-Tool ab.
  Da WoL ein reines Fire-and-Forget-UDP-Paket ist, gibt es keine Bestätigung, ob das Zielgerät tatsächlich aufwacht (abhängig von BIOS/Netzwerkkarten-Einstellungen und Netzwerktopologie) – `True` bedeutet nur, dass das Paket erfolgreich gesendet wurde.
- **Parameter:**
  - `mac`: Ziel-MAC-Adresse. Akzeptierte Formate: `"XX:XX:XX:XX:XX:XX"`, `"XX-XX-XX-XX-XX-XX"` oder ohne Trennzeichen (`"XXXXXXXXXXXX"`).
  - `broadcastIP`: Optional. Broadcast-Adresse des Zielnetzes (Standard: `"255.255.255.255"`).
  - `port`: Optional. Ziel-Port (Standard: `9`, der klassische WoL-Port).
- **Rückgabe:**
  `BoolVal(true)` wenn das Magic Packet erfolgreich gesendet wurde.
  `ErrorVal` bei ungültiger MAC-Adresse oder falls die UDP-Verbindung/das Senden fehlschlägt.

```vb
net.WakeOnLan("00:1A:2B:3C:4D:5E")

' Mit eigenem Subnetz und Standard-Port
result = net.WakeOnLan("00-1A-2B-3C-4D-5E", "192.168.0.255")
If IsError(result) Then
    Print "Fehler: " & ErrorText(result)
End If
```

---

## net.Html(text)
- **Konkret:**
  Ersetzt HTML-Sonderzeichen und deutsche Umlaute durch HTML-Entities.
  Beispiele: `<` → `&lt;`, `ä` → `&auml;`, `ß` → `&szlig;`.
- **Parameter:**
  - `text`: Zu kodierender String.
- **Rückgabe:**
  `StrVal`