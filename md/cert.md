# cert.* – Zertifikat-Modul

Das Modul `cert` stellt Funktionen zur PKI-Verwaltung bereit: Schlüsselerzeugung, CSR-Erstellung, Zertifikat-Signierung sowie Export in verschiedene Formate (PEM, DER, PFX, PKCS#7).

---

### `cert.GenerateKey(outFile, algo, bits)`

Erzeugt einen privaten Schlüssel im PKCS#8-Format.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `outFile` | String | Zieldatei für den Key (PEM) |
| `algo` | String | Algorithmus: `rsa` oder `ecdsa` |
| `bits` | String/Int | RSA: Mindestens 4096. ECDSA: 256, 384, 521 (Standard: 384) |

**Rückgabe:** `Bool` – `true` bei Erfolg.

**Hinweise:**
- RSA-Schlüssel unter 4096 Bit werden automatisch auf 4096 angehoben. Mit einem Konsolenhinweis.
- Die Datei wird mit Berechtigung `0600` gespeichert.

---

### `cert.CreateCSR(subject, keyPath, outFile [, SANs])`

Erstellt eine Certificate Signing Request (CSR).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `subject` | String | Common Name (CN) |
| `keyPath` | String | Pfad zum privaten Schlüssel |
| `outFile` | String | Ausgabedatei (PEM) |
| `SANs` | String (optional) | Kommagetrennte DNS-Namen oder IP-Adressen. Unterstützt Wildcards (`*.example.com`) und Umlaute/IDN (werden automatisch nach Punycode konvertiert) |

**Rückgabe:** `Bool`

**Hinweise:**
- Der `subject` (CN) wird automatisch als erster SAN-Eintrag übernommen, da moderne Clients nur SANs zur Hostname-Validierung heranziehen, nicht mehr den CN.
- Internationalisierte Domainnamen (z. B. `müller.de`) werden automatisch nach Punycode/ACE konvertiert (`xn--mller-kva.de`), da SAN-Einträge laut Standard ASCII sein müssen. Der CN im Zertifikat selbst bleibt unverändert in der Original-Schreibweise.
- Wildcard-Domains (`*.example.com`) werden unterstützt – der `*.`-Teil bleibt erhalten, nur der Domainteil wird ggf. nach Punycode konvertiert.

---

### `cert.CreateCSRConf(confPath, keyPath, outCSR)`

Erstellt eine CSR auf Basis einer OpenSSL-Konfigurationsdatei. Liest `CN`, `DNS.*` und `IP.*`-Einträge aus der Datei.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `confPath` | String | Pfad zur `.conf`-Datei |
| `keyPath` | String | Pfad zum privaten Schlüssel |
| `outCSR` | String | Ausgabedatei (PEM) |

**Rückgabe:** `Bool`

**Hinweise:**
- `DNS.*`-Einträge werden automatisch nach Punycode/ACE konvertiert und unterstützen Wildcards (`*.example.com`); ungültige Domainnamen in der Konfigurationsdatei werden stillschweigend übersprungen statt den Aufruf abzubrechen.
- Fehlt der `CN`-Eintrag in der Konfigurationsdatei, schlägt der Aufruf fehl.

---

### `cert.CreateFromConf(confPath, outKey, outCSR [, algo, bits])`

Erstellt in einem Schritt einen neuen Private Key und eine CSR auf Basis einer OpenSSL-Konfigurationsdatei. Liest `CN`, `DNS.*` und `IP.*`-Einträge aus der Datei.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `confPath` | String | Pfad zur `.conf`-Datei |
| `outKey` | String | Zieldatei für den neuen Key (PEM, PKCS#8) |
| `outCSR` | String | Zieldatei für die CSR (PEM) |
| `algo` | String (optional) | Algorithmus: `rsa` oder `ecdsa` (Standard: `ecdsa`) |
| `bits` | String/Int (optional) | RSA: Mindestens 4096 (kleinere Werte werden automatisch angehoben, mit Konsolenhinweis). ECDSA: 256, 384, 521 (Standard: 384) |

**Rückgabe:** `Bool`

**Hinweise:**
- Kombiniert `cert.GenerateKey` und `cert.CreateCSRConf` in einem Aufruf – nützlich, wenn kein bereits vorhandener Key wiederverwendet werden soll.
- `DNS.*`-Einträge in der Konfigurationsdatei werden automatisch nach Punycode/ACE konvertiert und unterstützen Wildcards (`*.example.com`).
- Fehlt der `CN`-Eintrag in der Konfigurationsdatei, schlägt der Aufruf fehl.
- Der Key wird mit Berechtigung `0600` gespeichert.

---

### `cert.CreateSelfSigned(subject, keyPath, outCert [, days, SANs, isCA])`

Erstellt ein selbstsigniertes Zertifikat.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `subject` | String | Common Name (CN) |
| `keyPath` | String | Pfad zum privaten Schlüssel |
| `outCert` | String | Ausgabedatei (PEM) |
| `days` | Int (optional) | Gültigkeitsdauer in Tagen (Standard: 365) |
| `SANs` | String (optional) | Kommagetrennte DNS-Namen oder IPs. Unterstützt Wildcards (`*.example.com`) und Umlaute/IDN (werden automatisch nach Punycode konvertiert) |
| `isCA` | String (optional) | `"true"` für CA-Zertifikat |

**Rückgabe:** `Null` bei Erfolg, `Error` bei Fehler.

**Hinweise:**
- Unterstützte Key-Typen: RSA (`RSA PRIVATE KEY`), ECDSA (`EC PRIVATE KEY`).
- PKCS#8-Keys (`PRIVATE KEY`) werden **nicht** unterstützt – dafür `cert.GenerateKey` verwenden und den erzeugten Key direkt nutzen.
- Der `subject` (CN) wird automatisch als erster SAN-Eintrag übernommen, analog zu `cert.CreateCSR`.
- Internationalisierte Domainnamen und Wildcards (`*.example.com`) werden wie bei `cert.CreateCSR` behandelt.

---

### `cert.SelfSignCSR(csrPath, keyPath, outCert [, days, isCA])`

Erstellt aus einer vorhandenen CSR ein selbstsigniertes Zertifikat. Subject und SANs werden aus der CSR übernommen, Aussteller und Inhaber sind identisch.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `csrPath` | String | Pfad zur CSR-Datei |
| `keyPath` | String | Pfad zum privaten Schlüssel, der zur CSR gehört |
| `outCert` | String | Ausgabedatei (PEM) |
| `days` | Int (optional) | Gültigkeitsdauer in Tagen (Standard: 365) |
| `isCA` | String (optional) | `"true"` für CA-Zertifikat |

**Rückgabe:** `Bool`

**Hinweise:**
- Die CSR-Signatur wird vor der Verarbeitung validiert.
- Zusätzlich wird geprüft, ob der öffentliche Schlüssel der CSR tatsächlich zum übergebenen privaten Schlüssel passt – bei einer Nichtübereinstimmung schlägt der Aufruf fehl, statt ein unbrauchbares Zertifikat zu erzeugen.
- DNS-SAN-Einträge aus der CSR werden vor der Übernahme nach Punycode/ACE validiert – enthält die CSR einen ungültigen SAN-Eintrag (z. B. rohes Unicode), schlägt der Aufruf fehl.
- Unterstützte Key-Typen: RSA, ECDSA, PKCS#8 (siehe `loadPrivateKey`).

---

### `cert.SignCSR(csrPath, caCert, caKey, outCert [, days])`

Signiert eine CSR mit einer CA und stellt ein gültiges Zertifikat aus.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `csrPath` | String | Pfad zur CSR-Datei |
| `caCert` | String | Pfad zum CA-Zertifikat |
| `caKey` | String | Pfad zum CA-Schlüssel |
| `outCert` | String | Ausgabedatei (PEM) |
| `days` | Int (optional) | Gültigkeitsdauer in Tagen (Standard: 365) |

**Rückgabe:** `Bool`

**Hinweise:**
- Die CSR-Signatur wird vor der Verarbeitung validiert.
- DNS-SAN-Einträge aus der CSR werden vor der Übernahme nach Punycode/ACE validiert – enthält die CSR einen ungültigen SAN-Eintrag (z. B. rohes Unicode), schlägt der Aufruf fehl.
- Das ausgestellte Zertifikat erhält `ExtKeyUsageServerAuth`.

---

### `cert.ExportPEM(certPath, outFile)`

Kopiert ein Zertifikat unverändert als PEM-Datei.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `certPath` | String | Quelldatei |
| `outFile` | String | Zieldatei |

**Rückgabe:** `Null` bei Erfolg, `Error` bei Fehler.

---

### `cert.ExportDER(certPath, outFile)`

Konvertiert ein PEM-Zertifikat in das binäre DER-Format.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `certPath` | String | PEM-Quelldatei |
| `outFile` | String | DER-Ausgabedatei |

**Rückgabe:** `Null` bei Erfolg, `Error` bei Fehler.

**Hinweise:**
- Nur PEM-Blöcke vom Typ `CERTIFICATE` werden akzeptiert. Private Keys werden explizit abgelehnt.

---

### `cert.GetPublicKey(certPath)`

Extrahiert den öffentlichen Schlüssel aus einem Zertifikat.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `certPath` | String | Pfad zum Zertifikat (PEM) |

**Rückgabe:** `String` – öffentlicher Schlüssel im PEM-Format (`PUBLIC KEY`).

---

### `cert.ExportPFX(certPath, keyPath, outFile, password [, friendlyName, mode])`

Exportiert Zertifikat und Key in einen PKCS#12-Container (`.pfx`/`.p12`).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `certPath` | String | PEM-Datei (kann Zertifikatskette enthalten) |
| `keyPath` | String | Pfad zum privaten Schlüssel |
| `outFile` | String | Ausgabedatei |
| `password` | String | Passwort für den Container |
| `friendlyName` | String (optional) | Anzeigename (wird geloggt, nicht eingebettet) |
| `mode` | String (optional) | `"legacy"` (Standard, RC2) oder `"modern"` |

**Rückgabe:** `Bool`

**Hinweise:**
- Das erste Zertifikat in der PEM-Datei wird als Leaf-Zertifikat verwendet, alle weiteren als Chain.

---

### `cert.ExportPKCS7(certPath, outFile)`

Erstellt einen PKCS#7-Container aus einem oder mehreren Zertifikaten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `certPath` | String | Quelldatei (PEM, auch Bundles) |
| `outFile` | String | Ausgabedatei (binär) |

**Rückgabe:** `Bool`

---

### `cert.Combine(cert1, ..., outFile)`

Kombiniert mehrere Zertifikate zu einer PEM-Kette.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `cert1, ...` | String | Beliebig viele Zertifikatspfade (PEM oder DER) |
| `outFile` | String | Ausgabedatei (letztes Argument) |

**Rückgabe:** `Bool`

**Hinweise:**
- DER-Dateien werden automatisch erkannt und konvertiert.

---

### `cert.CreateConf([cn, dnsArray, outFile])`

Erstellt eine OpenSSL-Konfigurationsdatei mit SAN-Einträgen.

**Ohne Argumente:** Interaktiver Modus mit Eingabe-Prompts.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `cn` | String (optional) | Common Name |
| `dnsArray` | Array (optional) | Zusätzliche DNS-Namen und/oder IP-Adressen |
| `outFile` | String (optional) | Zielordner oder Dateiname |

**Rückgabe:** `Bool`

**Hinweise:**
- Ist `outFile` ein Ordner (oder leer), wird `<CN>.conf` als Dateiname verwendet.
- Existiert die Zieldatei bereits, wird eine Überschreib-Bestätigung eingeholt.
- CN und zusätzliche DNS-Einträge werden vor dem Schreiben nach Punycode/ACE konvertiert (Wildcards `*.example.com` werden unterstützt). Im interaktiven Modus wird bei einem ungültigen Eintrag nur gewarnt und weitergemacht; im Skript-Modus (`CreateConf(cn, dnsArray, ...)`) bricht ein ungültiger Eintrag den Aufruf mit `Error` ab.

---

## Interne Hilfsfunktionen

| Funktion | Beschreibung |
|----------|--------------|
| `loadPrivateKey(path)` | Lädt RSA-, EC- und PKCS#8-Keys aus PEM |
| `loadAllCerts(path)` | Liest alle `CERTIFICATE`-Blöcke aus einer PEM-Datei |
| `parseAndValidateCSR(data)` | Dekodiert und validiert eine CSR inkl. Signaturprüfung |
| `newSerial()` | Erzeugt eine kryptografisch sichere 128-Bit-Seriennummer |
| `ensureDir(path)` | Erstellt übergeordnete Verzeichnisse falls nötig |
| `generatePrivateKey(algo, bits)` | Erzeugt einen RSA- oder ECDSA-Key (genutzt von `cert.GenerateKey` und `cert.CreateFromConf`); RSA unter 4096 Bit wird mit Konsolenhinweis automatisch angehoben |
| `parseVbxConf(confStr)` | Parst `CN`, `DNS.*` (Punycode-konvertiert, Wildcard-fähig) und `IP.*` aus einer von `cert.CreateConf` erzeugten Datei |
| `toACEHostname(name)` | Konvertiert einen Domainnamen nach Punycode/ACE, mit Wildcard-Unterstützung (`*.example.com`) |
| `buildSANs(subject, sansCSV)` | Baut DNS-/IP-SAN-Listen aus Subject+SAN-String: CN als erster Eintrag, Punycode-Konvertierung, Deduplizierung |

---

## Abhängigkeiten

| Paket | Verwendung |
|-------|------------|
| `github.com/fullsailor/pkcs7` | PKCS#7-Export |
| `software.sslmate.com/src/go-pkcs12` | PFX/PKCS#12-Export |
| `golang.org/x/net/idna` | Punycode/ACE-Konvertierung für internationalisierte Domainnamen (IDN) und Wildcard-SANs |
| Go Standardbibliothek (`crypto/x509`, `crypto/rsa`, `crypto/ecdsa`, …) | Kern-PKI-Operationen |