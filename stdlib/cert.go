// ------------------------
// stdlib_cert.go
// ------------------------

package stdlib

import (
	"bufio"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	valpkg "vbxc/value"

	"github.com/fullsailor/pkcs7"
	"golang.org/x/net/idna"
	"software.sslmate.com/src/go-pkcs12"
)

func initCertFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "cert."

	// ---------------- cert.GenerateKey ----------------
	// ---------------- cert.GenerateKey ----------------
	Register(ns+"GenerateKey", "cert",
		"outFile, algo, bits",
		"Erzeugt einen Private Key (RSA ≥4096 oder ECDSA P256/P384/P521) im PKCS#8-Format.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 3 {
				return valpkg.ErrorVal("usage: cert.GenerateKey(outFile, algo, bits)")
			}

			outFile, errVal := absPathVal(args[0].Str)
			if errVal != nil {
				return *errVal
			}

			bits, _ := strconv.Atoi(args[2].Str)

			key, err := generatePrivateKey(args[1].Str, bits)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			der, err := x509.MarshalPKCS8PrivateKey(key)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			err = os.WriteFile(outFile, pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: der,
			}), 0600)

			return valpkg.BoolVal(err == nil)
		})

	// ---------------- cert.CreateCSR ----------------
	Register(ns+"CreateCSR", "cert",
		"subject, keyPath, outFile [, SANs]",
		"Erstellt eine Certificate Signing Request (CSR) mit optionalen DNS/IP SANs.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 3 {
				return valpkg.ErrorVal("usage: cert.CreateCSR(subject, keyPath, outFile [, SANs])")
			}

			keyPath, _ := absPathVal(args[1].Str)
			outPath, _ := absPathVal(args[2].Str)

			priv, err := loadPrivateKey(keyPath)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			subject := args[0].Str
			sansCSV := ""
			if len(args) >= 4 {
				sansCSV = args[3].Str
			}

			dns, ips, err := buildSANs(subject, sansCSV)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			tmpl := x509.CertificateRequest{
				Subject:     pkix.Name{CommonName: subject},
				DNSNames:    dns,
				IPAddresses: ips,
			}

			csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &tmpl, priv)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			err = os.WriteFile(outPath, pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE REQUEST",
				Bytes: csrBytes,
			}), 0644)

			return valpkg.BoolVal(err == nil)
		})

	// ---------------- cert.SelfSignCSR ----------------
	Register(ns+"SelfSignCSR", "cert",
		"csrPath, keyPath, outCert [, days, isCA]",
		"Erstellt aus einer vorhandenen CSR ein selbstsigniertes Zertifikat (Subject/SANs kommen aus der CSR, signiert mit dem zugehörigen Private Key).",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 3 {
				return valpkg.ErrorVal("usage: cert.SelfSignCSR(csrPath, keyPath, outCert [, days, isCA])")
			}

			csrPath, errVal1 := absPathVal(args[0].Str)
			if errVal1 != nil {
				return *errVal1
			}
			keyPath, errVal2 := absPathVal(args[1].Str)
			if errVal2 != nil {
				return *errVal2
			}
			outPath, errVal3 := absPathVal(args[2].Str)
			if errVal3 != nil {
				return *errVal3
			}

			days := 365
			if len(args) >= 4 {
				if d, err := strconv.Atoi(args[3].Str); err == nil {
					days = d
				}
			}
			isCA := false
			if len(args) >= 5 && strings.ToLower(args[4].Str) == "true" {
				isCA = true
			}

			if err := ensureDir(outPath); err != nil {
				return valpkg.ErrorVal("Ordner-Fehler: " + err.Error())
			}

			// 1. CSR laden, parsen und Signatur prüfen (stellt sicher, dass der
			//    öffentliche Schlüssel wirklich zum mitgelieferten Private Key passt)
			csrData, err := os.ReadFile(csrPath)
			if err != nil {
				return valpkg.ErrorVal("CSR konnte nicht gelesen werden: " + err.Error())
			}
			csr, err := parseAndValidateCSR(csrData)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			// 2. Private Key laden
			priv, err := loadPrivateKey(keyPath)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			// 3. Sicherstellen, dass Key und CSR tatsächlich zusammengehören
			pub := publicKey(priv)
			pubBytes, err := x509.MarshalPKIXPublicKey(pub)
			if err != nil {
				return valpkg.ErrorVal("Public-Key-Vergleichsfehler: " + err.Error())
			}
			csrPubBytes, err := x509.MarshalPKIXPublicKey(csr.PublicKey)
			if err != nil {
				return valpkg.ErrorVal("Public-Key-Vergleichsfehler (CSR): " + err.Error())
			}
			if !bytes.Equal(pubBytes, csrPubBytes) {
				return valpkg.ErrorVal("Private Key passt nicht zum Public Key der CSR")
			}

			// 4. SAN-Einträge aus der CSR absichern (fremde CSR könnte rohes
			//    Unicode im DNS-SAN enthalten, was gegen RFC 5280/6125 verstößt)
			var sanitizedDNS []string
			for _, d := range csr.DNSNames {
				ace, err := toACEHostname(d)
				if err != nil {
					return valpkg.ErrorVal("Ungültiger SAN-Eintrag in CSR ('" + d + "'): " + err.Error())
				}
				sanitizedDNS = append(sanitizedDNS, ace)
			}

			// 5. Zertifikat-Template aus der CSR aufbauen (selbstsigniert: Issuer = Subject)
			serial := newSerial()
			tmpl := &x509.Certificate{
				SerialNumber:          serial,
				Subject:               csr.Subject,
				DNSNames:              sanitizedDNS,
				IPAddresses:           csr.IPAddresses,
				NotBefore:             time.Now().Add(-5 * time.Minute),
				NotAfter:              time.Now().AddDate(0, 0, days),
				BasicConstraintsValid: true,
				IsCA:                  isCA,
				KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			}
			if isCA {
				tmpl.KeyUsage |= x509.KeyUsageCertSign
			}

			// 6. Selbst signieren
			certBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, csr.PublicKey, priv)
			if err != nil {
				return valpkg.ErrorVal("Zertifikat-Erstellung fehlgeschlagen: " + err.Error())
			}

			if err := os.WriteFile(outPath, pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: certBytes,
			}), 0644); err != nil {
				return valpkg.ErrorVal("Schreibfehler: " + err.Error())
			}

			return valpkg.BoolVal(true)
		})

	// ---------------- ExportPEM ----------------
	Register(ns+"ExportPEM", "cert", "certPath, outFile", "Kopiert ein Zertifikat unverändert ins PEM-Format.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("usage: cert.ExportPEM(certPath, outFile)")
		}

		inP, errVal1 := absPathVal(args[0].Str)
		if errVal1 != nil {
			return *errVal1
		}

		outP, errVal2 := absPathVal(args[1].Str)
		if errVal2 != nil {
			return *errVal2
		}

		certData, err := os.ReadFile(inP)
		if err != nil {
			return valpkg.ErrorVal("Zertifikat konnte nicht gelesen werden: " + err.Error())
		}

		if err := os.WriteFile(outP, certData, 0644); err != nil {
			return valpkg.ErrorVal("Fehler beim Schreiben der PEM-Datei: " + err.Error())
		}

		return valpkg.NullVal()
	})

	// ---------------- ExportDER ----------------
	Register(ns+"ExportDER", "cert", "certPath, outFile", "Konvertiert ein PEM-Zertifikat in das binäre DER-Format.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("usage: cert.ExportDER(certPath, outFile)")
		}

		inP, errVal1 := absPathVal(args[0].Str)
		if errVal1 != nil {
			return *errVal1
		}

		outP, errVal2 := absPathVal(args[1].Str)
		if errVal2 != nil {
			return *errVal2
		}

		certData, err := os.ReadFile(inP)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Lesen der Zertifikatsdatei: " + err.Error())
		}

		block, _ := pem.Decode(certData)
		if block == nil {
			return valpkg.ErrorVal("Datei ist kein gültiges PEM-Format")
		}

		if block.Type != "CERTIFICATE" {
			return valpkg.ErrorVal("Export abgebrochen: PEM-Block ist vom Typ '" + block.Type + "', erwartet wurde 'CERTIFICATE'")
		}

		if err := os.WriteFile(outP, block.Bytes, 0644); err != nil {
			return valpkg.ErrorVal("Fehler beim Schreiben der DER-Datei: " + err.Error())
		}

		return valpkg.NullVal()
	})

	// ---------------- GetPublicKey ----------------
	Register(ns+"GetPublicKey", "cert", "certPath", "Extrahiert den öffentlichen Schlüssel aus einem Zertifikat.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("usage: cert.GetPublicKey(certPath)")
		}

		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return valpkg.ErrorVal("Zertifikat konnte nicht gelesen werden: " + err.Error())
		}

		block, _ := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" {
			return valpkg.ErrorVal("Kein gültiges Zertifikat im PEM-Format gefunden")
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return valpkg.ErrorVal("Zertifikat-Parse-Fehler: " + err.Error())
		}

		pubBytes, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
		if err != nil {
			return valpkg.ErrorVal("Fehler beim Exportieren des Public Keys: " + err.Error())
		}

		pubPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pubBytes,
		})

		return valpkg.StrVal(string(pubPEM))
	})

	// ---------------- cert.CreateFromConf ----------------
	Register(ns+"CreateFromConf", "cert",
		"confPath, outKey, outCSR [, algo, bits]",
		"Erstellt in einem Schritt einen neuen Private Key und eine CSR basierend auf einer OpenSSL-Konfigurationsdatei (CN/SANs aus cert.CreateConf).",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 3 {
				return valpkg.ErrorVal("usage: cert.CreateFromConf(confPath, outKey, outCSR [, algo, bits])")
			}

			confP, errVal1 := absPathVal(args[0].Str)
			if errVal1 != nil {
				return *errVal1
			}
			keyOutP, errVal2 := absPathVal(args[1].Str)
			if errVal2 != nil {
				return *errVal2
			}
			csrOutP, errVal3 := absPathVal(args[2].Str)
			if errVal3 != nil {
				return *errVal3
			}

			algo := "ecdsa"
			bits := 384
			if len(args) >= 4 && args[3].Str != "" {
				algo = args[3].Str
			}
			if len(args) >= 5 {
				if b, err := strconv.Atoi(args[4].Str); err == nil {
					bits = b
				}
			} else if strings.ToLower(algo) == "rsa" {
				bits = 4096
			}

			if err := ensureDir(keyOutP); err != nil {
				return valpkg.ErrorVal("Ordner-Fehler (Key): " + err.Error())
			}
			if err := ensureDir(csrOutP); err != nil {
				return valpkg.ErrorVal("Ordner-Fehler (CSR): " + err.Error())
			}

			confData, err := os.ReadFile(confP)
			if err != nil {
				return valpkg.ErrorVal("Config-Ladefehler: " + err.Error())
			}
			cn, dnsNames, ips := parseVbxConf(string(confData))
			if cn == "" {
				return valpkg.ErrorVal("Kein CN in der Konfigurationsdatei gefunden")
			}

			priv, err := generatePrivateKey(algo, bits)
			if err != nil {
				return valpkg.ErrorVal("Key-Erstellung fehlgeschlagen: " + err.Error())
			}

			der, err := x509.MarshalPKCS8PrivateKey(priv)
			if err != nil {
				return valpkg.ErrorVal("Key-Marshal-Fehler: " + err.Error())
			}
			if err := os.WriteFile(keyOutP, pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: der,
			}), 0600); err != nil {
				return valpkg.ErrorVal("Key-Schreibfehler: " + err.Error())
			}

			csrTemplate := x509.CertificateRequest{
				Subject:     pkix.Name{CommonName: cn},
				DNSNames:    dnsNames,
				IPAddresses: ips,
			}

			csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, priv)
			if err != nil {
				return valpkg.ErrorVal("CSR-Erstellung fehlgeschlagen: " + err.Error())
			}
			if err := os.WriteFile(csrOutP, pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE REQUEST",
				Bytes: csrBytes,
			}), 0644); err != nil {
				return valpkg.ErrorVal("CSR-Schreibfehler: " + err.Error())
			}

			return valpkg.BoolVal(true)
		})

	// ---------------- cert.CreateSelfSigned ----------------
	Register(ns+"CreateSelfSigned", "cert", "subject, keyPath, outCert [, days, SANs, isCA]", "Erstellt ein selbstsigniertes Zertifikat.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("usage: cert.CreateSelfSigned(subject, keyPath, outCert [, days, SANs, isCA])")
		}

		kPath, errVal1 := absPathVal(args[1].Str)
		if errVal1 != nil {
			return *errVal1
		}

		oPath, errVal2 := absPathVal(args[2].Str)
		if errVal2 != nil {
			return *errVal2
		}

		subject := args[0].Str
		days := 365
		if len(args) >= 4 {
			if d, err := strconv.Atoi(args[3].Str); err == nil {
				days = d
			}
		}

		keyData, err := os.ReadFile(kPath)
		if err != nil {
			return valpkg.ErrorVal("Key-Lesefehler: " + err.Error())
		}

		block, _ := pem.Decode(keyData)
		if block == nil {
			return valpkg.ErrorVal("Ungültiger Private Key (PEM erwartet)")
		}

		var priv crypto.PrivateKey
		switch block.Type {
		case "RSA PRIVATE KEY":
			priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		case "EC PRIVATE KEY":
			priv, err = x509.ParseECPrivateKey(block.Bytes)
		default:
			return valpkg.ErrorVal("Nicht unterstützter Key-Typ: " + block.Type)
		}
		if err != nil {
			return valpkg.ErrorVal("Key-Parse-Fehler: " + err.Error())
		}

		sansCSV := ""
		if len(args) >= 5 {
			sansCSV = args[4].Str
		}
		dnsNames, ipAddresses, err := buildSANs(subject, sansCSV)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		isCA := false
		if len(args) >= 6 && strings.ToLower(args[5].Str) == "true" {
			isCA = true
		}

		serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
		tmpl := x509.Certificate{
			SerialNumber: serial,
			Subject: pkix.Name{
				CommonName: subject,
			},
			NotBefore:             time.Now().Add(-5 * time.Minute),
			NotAfter:              time.Now().Add(time.Duration(days) * 24 * time.Hour),
			DNSNames:              dnsNames,
			IPAddresses:           ipAddresses,
			BasicConstraintsValid: true,
			IsCA:                  isCA,
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		}

		if isCA {
			tmpl.KeyUsage |= x509.KeyUsageCertSign
		}

		certBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, publicKey(priv), priv)
		if err != nil {
			return valpkg.ErrorVal("Zertifikat-Erstellung fehlgeschlagen: " + err.Error())
		}

		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
		if err := os.WriteFile(oPath, certPEM, 0644); err != nil {
			return valpkg.ErrorVal("Speicherfehler: " + err.Error())
		}

		return valpkg.NullVal()
	})

	// ---------------- cert.SignCSR ----------------
	Register(ns+"SignCSR", "cert",
		"csrPath, caCert, caKey, outCert [, days]",
		"Signiert eine CSR mit einer CA und erzeugt ein gültiges Zertifikat.",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 4 {
				return valpkg.ErrorVal("usage: cert.SignCSR(...)")
			}

			csrPath, _ := absPathVal(args[0].Str)
			caCertPath, _ := absPathVal(args[1].Str)
			caKeyPath, _ := absPathVal(args[2].Str)
			outPath, _ := absPathVal(args[3].Str)

			days := 365
			if len(args) >= 5 {
				days, _ = strconv.Atoi(args[4].Str)
			}

			csrData, _ := os.ReadFile(csrPath)
			csr, err := parseAndValidateCSR(csrData)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			caCertData, _ := os.ReadFile(caCertPath)
			block, _ := pem.Decode(caCertData)
			caCert, _ := x509.ParseCertificate(block.Bytes)

			caKey, err := loadPrivateKey(caKeyPath)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			// SAN-Einträge aus der CSR absichern (fremde CSR könnte rohes
			// Unicode im DNS-SAN enthalten, was gegen RFC 5280/6125 verstößt)
			var sanitizedDNS []string
			for _, d := range csr.DNSNames {
				ace, err := toACEHostname(d)
				if err != nil {
					return valpkg.ErrorVal("Ungültiger SAN-Eintrag in CSR ('" + d + "'): " + err.Error())
				}
				sanitizedDNS = append(sanitizedDNS, ace)
			}

			tmpl := &x509.Certificate{
				SerialNumber: newSerial(),
				Subject:      csr.Subject,
				DNSNames:     sanitizedDNS,
				IPAddresses:  csr.IPAddresses,
				NotBefore:    time.Now().Add(-5 * time.Minute),
				NotAfter:     time.Now().AddDate(0, 0, days),
				KeyUsage:     x509.KeyUsageDigitalSignature,
				ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}

			certBytes, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, csr.PublicKey, caKey)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			err = os.WriteFile(outPath, pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: certBytes,
			}), 0644)

			return valpkg.BoolVal(err == nil)
		})

	// ---------------- ExportPFX ----------------
	Register(ns+"ExportPFX", "cert", "certP, keyP, outP, pass [, fName, mode]", "Exportiert Zertifikat und Key in einen PKCS#12 (PFX) Container.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 4 {
			return valpkg.ErrorVal("usage: cert.ExportPFX(certP, keyP, outP, pass [, fName, mode])")
		}

		certP, keyP, outP, password := args[0].Str, args[1].Str, args[2].Str, args[3].Str
		fName := ""
		mode := "legacy"
		if len(args) >= 5 {
			fName = args[4].Str
		}
		if len(args) >= 6 {
			mode = strings.ToLower(args[5].Str)
		}

		certData, err := os.ReadFile(certP)
		if err != nil {
			return valpkg.ErrorVal("Datei nicht lesbar: " + err.Error())
		}

		var allCerts []*x509.Certificate
		rest := certData
		for {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}

			if block.Type == "CERTIFICATE" {
				c, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					return valpkg.ErrorVal("Zertifikat-Fehler: " + err.Error())
				}
				allCerts = append(allCerts, c)
			}
		}

		if len(allCerts) == 0 {
			return valpkg.ErrorVal("Keine Zertifikate in der Datei gefunden!")
		}

		leaf := allCerts[0]
		var chain []*x509.Certificate
		if len(allCerts) > 1 {
			chain = allCerts[1:]
		}

		key, err := loadPrivateKey(keyP)
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		var encoder *pkcs12.Encoder
		if mode == "modern" {
			encoder = pkcs12.Modern
		} else {
			encoder = pkcs12.LegacyRC2
		}

		if fName != "" {
			fmt.Printf("PFX Export: %s\n", fName)
		}

		pfxData, err := encoder.Encode(key, leaf, chain, password)
		if err != nil {
			return valpkg.ErrorVal("Encoding Fehler: " + err.Error())
		}

		if err := os.WriteFile(outP, pfxData, 0644); err != nil {
			return valpkg.ErrorVal("Schreibfehler: " + err.Error())
		}

		return valpkg.BoolVal(true)
	})

	// ---------------- Combine ----------------
	Register(ns+"Combine", "cert", "cert1, ..., outFile", "Kombiniert mehrere Zertifikate zu einer PEM-Kette (Chain).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("usage: cert.Combine(cert1, ..., outFile)")
		}

		outP, _ := absPathVal(args[len(args)-1].Str)
		if err := ensureDir(outP); err != nil {
			return valpkg.ErrorVal("Ordner-Fehler: " + err.Error())
		}

		sourceFiles := args[:len(args)-1]
		var combinedPEM []byte

		for _, fileVal := range sourceFiles {
			path, _ := absPathVal(fileVal.Str)
			data, err := os.ReadFile(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf("Fehler beim Lesen von %s: %v", path, err))
			}

			block, _ := pem.Decode(data)
			if block != nil {
				current := data
				for {
					var b *pem.Block
					var rest []byte
					b, rest = pem.Decode(current)
					if b == nil {
						break
					}
					if b.Type == "CERTIFICATE" {
						combinedPEM = append(combinedPEM, pem.EncodeToMemory(b)...)
					}
					current = rest
				}
			} else {
				cert, err := x509.ParseCertificate(data)
				if err == nil {
					combinedPEM = append(combinedPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})...)
				}
			}
		}

		err := os.WriteFile(outP, combinedPEM, 0644)
		return valpkg.BoolVal(err == nil)
	})

	// ---------------- ExportPKCS7 ----------------
	Register(ns+"ExportPKCS7", "cert", "certPath, outFile", "Erstellt einen PKCS#7 Container aus einem oder mehreren Zertifikaten.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("usage: cert.ExportPKCS7(certPath, outFile)")
		}

		certPath, _ := absPathVal(args[0].Str)
		outFile, _ := absPathVal(args[1].Str)

		if err := ensureDir(outFile); err != nil {
			return valpkg.ErrorVal("Ordner-Fehler: " + err.Error())
		}

		certs, err := loadAllCerts(certPath)
		if err != nil {
			return valpkg.ErrorVal("Zertifikat-Ladefehler: " + err.Error())
		}

		p7, err := pkcs7.NewSignedData([]byte{})
		if err != nil {
			return valpkg.ErrorVal("PKCS7-Init Fehler: " + err.Error())
		}

		for _, c := range certs {
			p7.AddCertificate(c)
		}

		out, err := p7.Finish()
		if err != nil {
			return valpkg.ErrorVal("PKCS7-Finish Fehler: " + err.Error())
		}

		if err := os.WriteFile(outFile, out, 0644); err != nil {
			return valpkg.ErrorVal("Schreibfehler: " + err.Error())
		}

		return valpkg.BoolVal(true)
	})

	// ---------------- cert.CreateConf ----------------
	Register(ns+"CreateConf", "cert", "[cn, dnsArray, outFile]", "Assistent (interaktiv) oder Funktion zum Erstellen einer OpenSSL-Konfigurationsdatei.", func(args []valpkg.Value) valpkg.Value {
		var cn string
		var dnsNames []string
		var ipNames []string
		var outFile string
		scanner := bufio.NewScanner(os.Stdin)

		ask := func(prompt string, defaultVal string) string {
			suffix := ""
			if defaultVal != "" {
				suffix = " [" + defaultVal + "]"
			}
			fmt.Print(prompt + suffix + ": ")
			if scanner.Scan() {
				input := strings.TrimSpace(scanner.Text())
				if input == "" {
					return defaultVal
				}
				return input
			}
			return defaultVal
		}

		if len(args) == 0 {
			fmt.Println("\n--- VBMini Zertifikats-Assistent ---")
			cn = ask("Common Name (CN)", "localhost")
			if aceCN, err := toACEHostname(cn); err == nil {
				dnsNames = append(dnsNames, aceCN) // CN ist immer der erste DNS (als Punycode/ACE)
			} else {
				fmt.Printf("\x1b[33mWarnung:\x1b[0m CN '%s' konnte nicht nach Punycode konvertiert werden (%v) — wird nicht als SAN aufgenommen\n", cn, err)
			}

			fmt.Print("Möchten Sie weitere DNS/IP-Adressen hinzufügen? (j/n): ")
			if scanner.Scan() && strings.ToLower(strings.TrimSpace(scanner.Text())) == "j" {
				for {
					entry := ask("Zusätzlicher Eintrag (leer zum Beenden)", "")
					if entry == "" {
						break
					}

					if ip := net.ParseIP(entry); ip != nil {
						ipNames = append(ipNames, entry)
					} else if ace, err := toACEHostname(entry); err == nil {
						dnsNames = append(dnsNames, ace)
					} else {
						fmt.Printf("\x1b[33mWarnung:\x1b[0m '%s' ist weder eine gültige IP noch ein gültiger Domainname (%v) — übersprungen\n", entry, err)
					}
				}
			}
			outFile = ask("Zielordner oder Dateiname", "")
		} else {
			// Direkte Übergabe aus einem Skript: CreateConf(cn, dnsArray, [outFile])
			cn = args[0].Str
			if aceCN, err := toACEHostname(cn); err == nil {
				dnsNames = append(dnsNames, aceCN)
			} else {
				return valpkg.ErrorVal("Ungültiger CN '" + cn + "': " + err.Error())
			}
			if len(args) > 1 {
				for _, v := range args[1].Arr {
					if ip := net.ParseIP(v.Str); ip != nil {
						ipNames = append(ipNames, v.Str)
					} else if ace, err := toACEHostname(v.Str); err == nil {
						dnsNames = append(dnsNames, ace)
					} else {
						return valpkg.ErrorVal("Ungültiger Domainname '" + v.Str + "': " + err.Error())
					}
				}
			}
			if len(args) > 2 {
				outFile = args[2].Str
			}
		}

		var sb strings.Builder
		for i, d := range dnsNames {
			sb.WriteString(fmt.Sprintf("DNS.%d = %s\n", i+1, d))
		}
		for i, ip := range ipNames {
			sb.WriteString(fmt.Sprintf("IP.%d = %s\n", i+1, ip))
		}

		confTemplate := fmt.Sprintf(`[ req ]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[ req_distinguished_name ]
CN = %s

[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[ alt_names ]
%s`, cn, sb.String())

		finalPath := ""
		cwd, _ := os.Getwd()

		if outFile == "" {
			finalPath = filepath.Join(cwd, cn+".conf")
		} else {
			absOut, _ := filepath.Abs(outFile)
			fi, err := os.Stat(absOut)

			if (err == nil && fi.IsDir()) || strings.HasSuffix(outFile, "/") || strings.HasSuffix(outFile, "\\") {
				finalPath = filepath.Join(absOut, cn+".conf")
			} else {
				finalPath = absOut
			}
		}

		if _, err := os.Stat(finalPath); err == nil {
			fmt.Printf("\x1b[33mDatei existiert bereits:\x1b[0m %s\n", filepath.Base(finalPath))
			fmt.Print("Überschreiben? (j/n): ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "j" && strings.ToLower(response) != "y" {
				fmt.Println("Vorgang abgebrochen.")
				return valpkg.BoolVal(false)
			}
		}

		dir := filepath.Dir(finalPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return valpkg.ErrorVal("Ordner-Fehler: " + err.Error())
		}
		if err := os.WriteFile(finalPath, []byte(confTemplate), 0644); err != nil {
			return valpkg.ErrorVal("Schreibfehler: " + err.Error())
		}

		fmt.Printf("\x1b[32m✔ Konfiguration erfolgreich gespeichert:\x1b[0m %s\n", finalPath)
		return valpkg.BoolVal(true)
	})

	// ---------------- cert.CreateCSRConf ----------------
	Register(ns+"CreateCSRConf", "cert", "confPath, keyPath, outCSR", "Erstellt eine CSR auf Basis einer OpenSSL-Konfigurationsdatei (DNS/IP-SANs, inkl. Punycode-Konvertierung).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("usage: cert.CreateCSRConf(confPath, keyPath, outCSR)")
		}

		confP, _ := absPathVal(args[0].Str)
		keyP, _ := absPathVal(args[1].Str)
		outP, _ := absPathVal(args[2].Str)

		if err := ensureDir(outP); err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		confData, err := os.ReadFile(confP)
		if err != nil {
			return valpkg.ErrorVal("Config-Ladefehler: " + err.Error())
		}

		priv, err := loadPrivateKey(keyP)
		if err != nil {
			return valpkg.ErrorVal("Key-Ladefehler: " + err.Error())
		}

		cn, dnsNames, ips := parseVbxConf(string(confData))
		if cn == "" {
			return valpkg.ErrorVal("Kein CN in der Konfigurationsdatei gefunden")
		}

		csrTemplate := x509.CertificateRequest{
			Subject:     pkix.Name{CommonName: cn},
			DNSNames:    dnsNames,
			IPAddresses: ips,
		}

		csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, priv)
		if err != nil {
			return valpkg.ErrorVal("CSR-Fehler: " + err.Error())
		}

		pemBlock := &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes}
		err = os.WriteFile(outP, pem.EncodeToMemory(pemBlock), 0644)

		return valpkg.BoolVal(err == nil)
	})
}

func publicKey(priv crypto.PrivateKey) interface{} {
	switch key := priv.(type) {
	case *rsa.PrivateKey:
		return &key.PublicKey
	case *ecdsa.PrivateKey:
		return &key.PublicKey
	default:
		return nil
	}
}

// ---------------- Helper-Funktionen ----------------

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

func loadAllCerts(certPath string) ([]*x509.Certificate, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			certs = append(certs, cert)
		}
	}

	if len(certs) == 0 {
		return nil, errors.New("keine Zertifikate in der Datei gefunden")
	}
	return certs, nil
}

func loadPrivateKey(keyPath string) (crypto.PrivateKey, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("kein gültiger PEM-Key")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)

	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)

	case "PRIVATE KEY": // PKCS#8
		return x509.ParsePKCS8PrivateKey(block.Bytes)

	default:
		return nil, fmt.Errorf("unbekannter Key-Typ: %s", block.Type)
	}
}

func newSerial() *big.Int {
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	return serial
}

func parseAndValidateCSR(data []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("CSR ist kein PEM")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, err
	}

	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("ungültige CSR-Signatur: %w", err)
	}

	return csr, nil
}

// generatePrivateKey erzeugt einen RSA- oder ECDSA-Key. Geteilte Kernlogik für
// cert.GenerateKey und cert.CreateFromConf.
func generatePrivateKey(algo string, bits int) (crypto.PrivateKey, error) {
	switch strings.ToLower(algo) {
	case "rsa":
		if bits < 4096 {
			fmt.Printf("\x1b[33mHinweis:\x1b[0m RSA-Schlüssellänge %d Bit ist zu gering, wird auf 4096 Bit angehoben.\n", bits)
			bits = 4096
		}
		return rsa.GenerateKey(rand.Reader, bits)

	case "ecdsa":
		var curve elliptic.Curve
		switch bits {
		case 256:
			curve = elliptic.P256()
		case 384:
			curve = elliptic.P384()
		case 521:
			curve = elliptic.P521()
		default:
			curve = elliptic.P384()
		}
		return ecdsa.GenerateKey(curve, rand.Reader)

	default:
		return nil, fmt.Errorf("nicht unterstützter Algorithmus: %s", algo)
	}
}

// parseVbxConf parst CN, DNS.x und IP.x aus einer von cert.CreateConf erzeugten
// Datei. Domainnamen werden nach ACE/Punycode konvertiert; ungültige Einträge
// werden stillschweigend übersprungen (Datei kann von Hand editiert worden sein).
// Geteilt zwischen cert.CreateCSRConf und cert.CreateFromConf.
func parseVbxConf(confStr string) (cn string, dnsNames []string, ips []net.IP) {
	seen := make(map[string]bool)
	lines := strings.Split(confStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "CN ="):
			cn = strings.TrimSpace(strings.TrimPrefix(line, "CN ="))
		case strings.Contains(line, "DNS.") && strings.Contains(line, "="):
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				if ace, err := toACEHostname(strings.TrimSpace(parts[1])); err == nil {
					if !seen["dns:"+ace] {
						seen["dns:"+ace] = true
						dnsNames = append(dnsNames, ace)
					}
				}
			}
		case strings.Contains(line, "IP.") && strings.Contains(line, "="):
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				if ip := net.ParseIP(strings.TrimSpace(parts[1])); ip != nil {
					ips = append(ips, ip)
				}
			}
		}
	}
	return
}

// toACEHostname wandelt einen (ggf. Unicode-)Domainnamen ins ASCII-Compatible-Encoding
// (Punycode) um, wie es für X.509-SAN-Einträge zwingend erforderlich ist. Ein führendes
// "*."  (Wildcard-Zertifikat) wird abgetrennt, der Rest konvertiert und danach wieder
// vorangestellt, da "*" kein gültiges IDNA-Label-Zeichen ist. IPs werden hier nicht
// behandelt, nur reine Hostnamen.
func toACEHostname(name string) (string, error) {
	wildcard := false
	rest := name
	if strings.HasPrefix(name, "*.") {
		wildcard = true
		rest = strings.TrimPrefix(name, "*.")
	}

	ace, err := idna.ToASCII(rest)
	if err != nil {
		return "", fmt.Errorf("ungültiger Domainname '%s': %w", name, err)
	}

	if wildcard {
		return "*." + ace, nil
	}
	return ace, nil
}

// buildSANs baut aus Subject/CN und einer kommagetrennten SAN-Liste die finalen
// DNS-/IP-SAN-Einträge: CN wird immer als erster Eintrag aufgenommen (Browser/
// Clients validieren nur SANs, nicht mehr den CN), Domainnamen werden nach
// ACE/Punycode konvertiert, und Duplikate werden übersprungen. Geteilt zwischen
// cert.CreateCSR und cert.CreateSelfSigned.
func buildSANs(subject string, sansCSV string) (dns []string, ips []net.IP, err error) {
	seen := make(map[string]bool)

	add := func(raw string) error {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil
		}
		if ip := net.ParseIP(raw); ip != nil {
			key := "ip:" + ip.String()
			if seen[key] {
				return nil
			}
			seen[key] = true
			ips = append(ips, ip)
			return nil
		}
		ace, err := toACEHostname(raw)
		if err != nil {
			return err
		}
		key := "dns:" + ace
		if seen[key] {
			return nil
		}
		seen[key] = true
		dns = append(dns, ace)
		return nil
	}

	if err := add(subject); err != nil {
		return nil, nil, err
	}
	if sansCSV != "" {
		for _, s := range strings.Split(sansCSV, ",") {
			if err := add(s); err != nil {
				return nil, nil, err
			}
		}
	}
	return dns, ips, nil
}

func init() {
	initCertFunctions()
}
