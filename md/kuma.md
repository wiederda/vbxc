# 🩺 kuma.* – Uptime Kuma Steuerung

Dient zur Steuerung einer Uptime Kuma Instanz (Wartungsfenster, Monitor-Pause) über die interne Socket.IO-API.
Erfordert Zugangsdaten eines Uptime-Kuma-Benutzers. Bei aktivierter 2FA zusätzlich das TOTP-Secret.

---

## Wartung

## kuma.SetMaintenance(server, user, password, monitorIDs, minutes [, secret])
- **Konkret:**
  Legt eine einmalige Wartungspause an und ordnet sie den angegebenen Monitoren zu.
  Die Wartung läuft nach Ablauf von `minutes` automatisch aus.
- **Parameter:**
  - `server`: `StrVal`. URL der Uptime Kuma Instanz (z. B. `"http://192.168.100.8:3001"`).
  - `user`: `StrVal`. Benutzername.
  - `password`: `StrVal`. Passwort.
  - `monitorIDs`: `ArrVal`. IDs der betroffenen Monitore (z. B. `{5, 7}`).
  - `minutes`: Dauer der Wartungspause in Minuten.
  - `secret`: Optional. `StrVal`. TOTP-Secret bei aktivierter 2FA. Leer lassen, falls keine 2FA aktiv ist.
- **Rückgabe:**
  `StrVal` (maintenanceID) bei Erfolg, `"error: ..."` bei Fehler.

---

## kuma.StopMaintenance(server, user, password, maintenanceID [, secret])
- **Konkret:**
  Beendet eine laufende Wartungspause vorzeitig.
- **Parameter:**
  - `server`: `StrVal`. URL der Uptime Kuma Instanz.
  - `user`: `StrVal`. Benutzername.
  - `password`: `StrVal`. Passwort.
  - `maintenanceID`: ID der Wartung, von `kuma.SetMaintenance` zurückgegeben.
  - `secret`: Optional. `StrVal`. TOTP-Secret bei aktivierter 2FA.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `"error: ..."` bei Fehler.

---

## kuma.DeleteMaintenance(server, user, password, maintenanceID [, secret])
- **Konkret:**
  Löscht ein Wartungsfenster vollständig, z. B. nach Abschluss eines Updates, um die Übersicht in Kuma sauber zu halten (statt alte Wartungen anzusammeln).
- **Parameter:**
  - `server`: `StrVal`. URL der Uptime Kuma Instanz.
  - `user`: `StrVal`. Benutzername.
  - `password`: `StrVal`. Passwort.
  - `maintenanceID`: ID der Wartung, von `kuma.SetMaintenance` zurückgegeben.
  - `secret`: Optional. `StrVal`. TOTP-Secret bei aktivierter 2FA.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `"error: ..."` bei Fehler.

---

## Monitore

## kuma.PauseMonitor(server, user, password, monitorID [, secret])
- **Konkret:**
  Pausiert einen einzelnen Monitor direkt. Im Gegensatz zur Maintenance-Funktion werden die Checks komplett gestoppt.
- **Parameter:**
  - `server`: `StrVal`. URL der Uptime Kuma Instanz.
  - `user`: `StrVal`. Benutzername.
  - `password`: `StrVal`. Passwort.
  - `monitorID`: ID des zu pausierenden Monitors.
  - `secret`: Optional. `StrVal`. TOTP-Secret bei aktivierter 2FA.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `"error: ..."` bei Fehler.

---

## kuma.ResumeMonitor(server, user, password, monitorID [, secret])
- **Konkret:**
  Setzt einen mit `kuma.PauseMonitor` pausierten Monitor wieder fort.
- **Parameter:**
  - `server`: `StrVal`. URL der Uptime Kuma Instanz.
  - `user`: `StrVal`. Benutzername.
  - `password`: `StrVal`. Passwort.
  - `monitorID`: ID des Monitors.
  - `secret`: Optional. `StrVal`. TOTP-Secret bei aktivierter 2FA.
- **Rückgabe:**
  `StrVal` (`"OK"`) bei Erfolg, `"error: ..."` bei Fehler.

---

## Hinweise

- Monitor-IDs finden sich in der Uptime-Kuma-Oberfläche in der URL (`/dashboard/5` → ID `5`).
- `kuma.SetMaintenance` stoppt die Checks nicht, sondern unterdrückt nur Benachrichtigungen (Monitor wird als "under maintenance" markiert). Für ein echtes Stoppen der Checks `kuma.PauseMonitor`/`kuma.ResumeMonitor` verwenden.
- `kuma.StopMaintenance` deaktiviert die Wartung nur (bleibt als Eintrag bestehen), `kuma.DeleteMaintenance` entfernt sie komplett. Für ein abgeschlossenes Update z. B. beides nacheinander aufrufen.
- Das TOTP-Secret ist der Base32-String aus der 2FA-Einrichtung (Text unter dem QR-Code), nicht der 6-stellige Bestätigungscode. Wird nur einmalig beim Einrichten angezeigt.
- Erfordert `go get github.com/gorilla/websocket`.
- Nur Socket.IO-v4 per WebSocket unterstützt, kein Polling-Fallback.
- Getestet gegen Uptime Kuma 2.5.0.