# 🔐 acme.* – ACME-Zertifikatsfunktionen

Verwaltet ACME-Konten und unterstützt die Beantragung von TLS-Zertifikaten über ACME-kompatible Zertifizierungsstellen wie Let's Encrypt. Aktuell wird ausschließlich die **DNS-01-Validierung** unterstützt. Als erster DNS-Provider wird **ipv64.net** unterstützt.

---

## acme.Account(directory, keyPath, statePath, email, acceptTOS)

**Konkret:** Erstellt ein neues ACME-Konto beim angegebenen Server oder lädt ein
bereits vorhandenes Konto aus der gespeicherten Statusdatei (`statePath`). Der
private Account-Key wird bei Bedarf automatisch als RSA-4096-Key erzeugt und
unter `keyPath` gespeichert (falls dort noch keiner existiert).

Ein vorhandenes Konto wird nur wiederverwendet, wenn die gespeicherte Directory
URL in `statePath` mit der aktuell übergebenen `directory` übereinstimmt –
wechselt man z.B. von Staging zu Production, wird automatisch ein neues Konto
angelegt.

**Parameter:**
- `directory` – ACME Directory URL, oder einer der Kurznamen `letsencrypt-stage`
  (Staging-Server, zum Testen ohne Zertifikats-Limits) bzw. `letsencrypt-prod`
  (Production-Server, echte vertrauenswürdige Zertifikate). Kurznamen sind
  case-insensitive; jede andere URL wird unverändert als eigener ACME-Server
  behandelt.
- `keyPath` – Pfad zum privaten ACME Account-Key (wird erzeugt, falls nicht
  vorhanden)
- `statePath` – Pfad zur gespeicherten Account-Konfiguration (JSON)
- `email` – E-Mail-Adresse des ACME-Kontos
- `acceptTOS` – Zustimmung zu den Nutzungsbedingungen (muss `True` sein, sonst
  Fehler)

**Rückgabe:** Account-URL als String, oder ein Fehler (siehe `IsError`/`ErrorText`),
falls z.B. die Directory URL nicht erreichbar ist oder der Account-Key nicht
gelesen/erzeugt werden kann.

**Beispiel:**
```vb
url = acme.Account("letsencrypt-stage", "account.key", "account.json", "admin@example.de", True)

If IsError(url) Then
    Print "Fehler: " & ErrorText(url)
Else
    Print "ACME Account: " & url
End If
```

---

## acme.Issue(directory, domain, csrPath, outputPath, provider, token)

**Konkret:** Fordert ein Zertifikat per ACME DNS-01 an. Setzt automatisch den
nötigen DNS-TXT-Eintrag beim angegebenen DNS-Provider, wartet auf öffentliche
Sichtbarkeit und ACME-Validierung, löscht den TXT-Eintrag anschließend wieder
und speichert das ausgestellte Zertifikat unter `outputPath`.

> **Hinweis:** Diese Funktion ist aktuell noch nicht vollständig implementiert
> (fehlender Account-Key-Ablauf) und liefert derzeit immer einen Fehler zurück.
> Für den kompletten ACME-Ablauf `acme.Account` in Kombination mit den
> einzelnen ACME-Client-Schritten nutzen, bis `acme.Issue` fertiggestellt ist.

**Parameter:**
- `directory` – ACME Directory URL, oder einer der Kurznamen `letsencrypt-stage`
  bzw. `letsencrypt-prod` (siehe `acme.Account`)
- `domain` – Domain, für die das Zertifikat ausgestellt werden soll
- `csrPath` – Pfad zur vorhandenen CSR-Datei (PEM- oder DER-kodiert)
- `outputPath` – Zieldatei für das ausgestellte Zertifikat
- `provider` – Name des DNS-Providers, z.B. `ipv64`
- `token` – API-Token des DNS-Providers (providerspezifisch, siehe jeweilige
  Provider-Dokumentation)

**Rückgabe:** `True` bei Erfolg, oder ein Fehler (siehe `IsError`/`ErrorText`).

---