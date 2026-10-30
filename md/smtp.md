# 📧 smtp.* – E-Mail-Versand

Dient zum Versenden von E-Mails über SMTP, SMTP/TLS und SMTP/STARTTLS.
Alle Funktionen erwarten ein Objekt mit Verbindungs- und Nachrichtenfeldern.
Mehrere Empfänger werden kommagetrennt angegeben.

---

## Mail-Objekt (gemeinsame Struktur)

Alle drei Sendefunktionen erwarten ein Objekt mit folgenden Feldern:

| Feld       | Pflicht | Beschreibung                                             |
|------------|---------|----------------------------------------------------------|
| `to`       | ✔       | Empfänger (kommagetrennt)                                |
| `server`   | ✔       | SMTP-Serverhostname                                      |
| `subject`  |         | Betreff                                                  |
| `body`     |         | Nachrichteninhalt                                        |
| `cc`       |         | CC-Empfänger (kommagetrennt)                             |
| `bcc`      |         | BCC-Empfänger (kommagetrennt, nicht im Header sichtbar)  |
| `from`     |         | Absenderadresse (Standard: `user` oder `noreply@localhost`) |
| `user`     |         | SMTP-Benutzername für Authentifizierung                  |
| `pass`     |         | SMTP-Passwort für Authentifizierung                      |
| `port`     |         | Port (Standardwert je nach Funktion)                     |
| `html`     |         | Bei gesetztem Wert: `Content-Type: text/html`            |

---

## smtp.Send(mailObject)
- **Konkret:**
  Sendet eine E-Mail über unverschlüsseltes SMTP (Port 25).
  Authentifizierung optional – wird nur genutzt wenn `user` und `pass` gesetzt sind.
- **Parameter:**
  - `mailObject`: Objekt mit den oben beschriebenen Feldern.
- **Standardport:** 25
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler.

---

## smtp.SendTLS(mailObject)
- **Konkret:**
  Sendet eine E-Mail über SMTP mit sofortigem TLS (Implicit TLS, Port 465).
  Die TLS-Verbindung wird vor der SMTP-Kommunikation aufgebaut (kein STARTTLS-Upgrade).
  Mindestens TLS 1.2.
- **Parameter:**
  - `mailObject`: Objekt mit den oben beschriebenen Feldern.
- **Standardport:** 465
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler.

---

## smtp.SendSTARTTLS(mailObject)
- **Konkret:**
  Sendet eine E-Mail über SMTP mit STARTTLS-Upgrade (Port 587).
  Verbindung startet unverschlüsselt und wird via STARTTLS auf TLS 1.2+ hochgestuft.
  Schlägt fehl, wenn der Server STARTTLS nicht unterstützt.
- **Parameter:**
  - `mailObject`: Objekt mit den oben beschriebenen Feldern.
- **Standardport:** 587
- **Rückgabe:**
  `BoolVal` (`true`) bei Erfolg, `ErrorVal` bei Fehler.