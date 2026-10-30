package stdlib

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Directory struct {
	NewNonce   string `json:"newNonce"`
	NewAccount string `json:"newAccount"`
	NewOrder   string `json:"newOrder"`
}

type ACMEClient struct {
	HTTPClient   *http.Client
	DirectoryURL string
	Directory    Directory

	AccountKey *rsa.PrivateKey
	AccountURL string

	Nonce string
}

type Order struct {
	URL            string       `json:"-"`
	Status         string       `json:"status"`
	Identifiers    []Identifier `json:"identifiers"`
	Authorizations []string     `json:"authorizations"`
	FinalizeURL    string       `json:"finalize"`
	CertificateURL string       `json:"certificate"`
}

type Identifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ACMEError struct {
	Type   string `json:"type"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}

type Authorization struct {
	URL        string      `json:"-"`
	Status     string      `json:"status"`
	Identifier Identifier  `json:"identifier"`
	Challenges []Challenge `json:"challenges"`
}

type Challenge struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Status string `json:"status"`
	Token  string `json:"token"`
}

func NewACMEClient(
	ctx context.Context,
	directoryURL string,
	accountKeyPath string,
) (*ACMEClient, error) {

	return &ACMEClient{
		HTTPClient:   &http.Client{Timeout: 60 * time.Second},
		DirectoryURL: strings.TrimRight(directoryURL, "/"),
	}, nil
}

func ParseACMEError(body []byte) string {
	var acmeErr ACMEError

	if err := json.Unmarshal(body, &acmeErr); err != nil {
		return strings.TrimSpace(string(body))
	}

	if acmeErr.Detail != "" {
		return acmeErr.Detail
	}

	if acmeErr.Type != "" {
		return acmeErr.Type
	}

	return strings.TrimSpace(string(body))
}

func (c *ACMEClient) LoadDirectory(ctx context.Context) error {
	resp, err := c.HTTPClient.Get(c.DirectoryURL)
	if err != nil {
		return fmt.Errorf(
			"ACME Directory konnte nicht geladen werden: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"ACME Directory liefert HTTP %d",
			resp.StatusCode,
		)
	}

	if err := json.NewDecoder(resp.Body).Decode(&c.Directory); err != nil {
		return fmt.Errorf(
			"ACME Directory ist ungültig: %w",
			err,
		)
	}

	if c.Directory.NewNonce == "" ||
		c.Directory.NewAccount == "" ||
		c.Directory.NewOrder == "" {

		return errors.New(
			"ACME Directory enthält nicht alle benötigten Endpunkte",
		)
	}

	return c.GetNonce(ctx)
}

func (c *ACMEClient) GetNonce(ctx context.Context) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodHead,
		c.Directory.NewNonce,
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"ACME Nonce konnte nicht abgefragt werden: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"ACME Nonce liefert HTTP %d",
			resp.StatusCode,
		)
	}

	c.Nonce = resp.Header.Get("Replay-Nonce")

	if c.Nonce == "" {
		return errors.New(
			"ACME Server hat keine Replay-Nonce geliefert",
		)
	}

	return nil
}

func (c *ACMEClient) FindExistingAccount(
	ctx context.Context,
) (string, error) {

	fmt.Println("Suche vorhandenes ACME Account...")

	payload := map[string]any{
		"onlyReturnExisting": true,
	}

	fmt.Println("Sende ACME Account Suche...")

	resp, err := c.postJWS(
		ctx,
		c.Directory.NewAccount,
		payload,
		false,
	)

	fmt.Println("ACME Account Suche Antwort erhalten.")

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		location := resp.Header.Get("Location")

		if location == "" {
			return "", errors.New(
				"ACME Account Antwort enthält keine Location",
			)
		}

		return location, nil
	}

	if resp.StatusCode == http.StatusBadRequest ||
		resp.StatusCode == http.StatusNotFound {

		detail := ParseACMEError(body)

		if detail != "" {
			fmt.Println(
				"ACME Account nicht vorhanden:",
				detail,
			)
		} else {
			fmt.Println(
				"ACME Account nicht vorhanden.",
			)
		}

		return "", errors.New(
			"kein bestehendes ACME Account gefunden",
		)
	}

	return "", fmt.Errorf(
		"ACME Account Suche fehlgeschlagen: HTTP %d: %s",
		resp.StatusCode,
		ParseACMEError(body),
	)
}

func (c *ACMEClient) CreateAccount(
	ctx context.Context,
	email string,
) (string, error) {

	payload := map[string]any{
		"contact": []string{
			"mailto:" + email,
		},
		"termsOfServiceAgreed": true,
	}

	resp, err := c.postJWS(
		ctx,
		c.Directory.NewAccount,
		payload,
		false,
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated &&
		resp.StatusCode != http.StatusOK {

		return "", fmt.Errorf(
			"ACME Account konnte nicht erstellt werden: HTTP %d: %s",
			resp.StatusCode,
			ParseACMEError(body),
		)
	}

	location := resp.Header.Get("Location")

	if location == "" {
		return "", errors.New(
			"ACME Account Antwort enthält keine Location",
		)
	}

	return location, nil
}

func (c *ACMEClient) CreateOrder(
	ctx context.Context,
	domains []string,
) (*Order, error) {

	identifiers := make(
		[]Identifier,
		0,
		len(domains),
	)

	for _, domain := range domains {
		identifiers = append(
			identifiers,
			Identifier{
				Type:  "dns",
				Value: domain,
			},
		)
	}

	payload := map[string]any{
		"identifiers": identifiers,
	}

	resp, err := c.postJWS(
		ctx,
		c.Directory.NewOrder,
		payload,
		true,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			"ACME Order konnte nicht erstellt werden: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var order Order

	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, err
	}

	order.URL = resp.Header.Get("Location")

	if order.URL == "" {
		return nil, errors.New(
			"ACME Order enthält keine Location",
		)
	}

	return &order, nil
}

func (c *ACMEClient) GetAuthorization(
	ctx context.Context,
	url string,
) (*Authorization, error) {

	resp, err := c.signedGET(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			"Authorization konnte nicht geladen werden: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var authz Authorization

	if err := json.NewDecoder(resp.Body).Decode(&authz); err != nil {
		return nil, err
	}

	authz.URL = url

	return &authz, nil
}

func (c *ACMEClient) AcceptChallenge(
	ctx context.Context,
	url string,
) error {

	resp, err := c.postJWS(
		ctx,
		url,
		map[string]any{},
		true,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf(
			"Challenge konnte nicht aktiviert werden: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	return nil
}

func (c *ACMEClient) WaitAuthorization(
	ctx context.Context,
	url string,
) error {

	for {
		authz, err := c.GetAuthorization(ctx, url)
		if err != nil {
			return err
		}

		fmt.Println(
			"Authorization Status:",
			authz.Status,
		)

		switch authz.Status {
		case "valid":
			return nil

		case "invalid":
			return errors.New(
				"ACME Authorization wurde abgelehnt",
			)

		case "pending", "processing":
			time.Sleep(3 * time.Second)

		default:
			return fmt.Errorf(
				"unbekannter Authorization Status: %s",
				authz.Status,
			)
		}
	}
}

func (c *ACMEClient) FinalizeOrder(
	ctx context.Context,
	url string,
	csrDER []byte,
) error {

	payload := map[string]any{
		"csr": base64.RawURLEncoding.EncodeToString(csrDER),
	}

	resp, err := c.postJWS(
		ctx,
		url,
		payload,
		true,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf(
			"Order konnte nicht finalisiert werden: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	return nil
}

func (c *ACMEClient) WaitOrder(
	ctx context.Context,
	url string,
) (*Order, error) {

	for {
		order, err := c.GetOrder(ctx, url)
		if err != nil {
			return nil, err
		}

		fmt.Println(
			"Order Status:",
			order.Status,
		)

		switch order.Status {
		case "valid":
			return order, nil

		case "invalid":
			return nil, errors.New(
				"ACME Order wurde ungültig",
			)

		case "pending", "processing":
			time.Sleep(3 * time.Second)

		default:
			return nil, fmt.Errorf(
				"unbekannter Order Status: %s",
				order.Status,
			)
		}
	}
}

func (c *ACMEClient) GetOrder(
	ctx context.Context,
	url string,
) (*Order, error) {

	resp, err := c.signedGET(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			"Order konnte nicht geladen werden: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var order Order

	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, err
	}

	order.URL = url

	return &order, nil
}

func (c *ACMEClient) FetchCertificate(
	ctx context.Context,
	url string,
) ([]byte, error) {

	resp, err := c.signedGET(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			"Zertifikat konnte nicht geladen werden: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	return io.ReadAll(resp.Body)
}

func (c *ACMEClient) signedGET(
	ctx context.Context,
	url string,
) (*http.Response, error) {

	payload := []byte{}

	return c.postJWSWithPayload(
		ctx,
		url,
		payload,
		true,
	)
}

func (c *ACMEClient) postJWS(
	ctx context.Context,
	url string,
	payload any,
	useAccount bool,
) (*http.Response, error) {

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return c.postJWSWithPayload(
		ctx,
		url,
		data,
		useAccount,
	)
}

func (c *ACMEClient) postJWSWithPayload(
	ctx context.Context,
	url string,
	payload []byte,
	useAccount bool,
) (*http.Response, error) {

	for attempt := 0; attempt < 2; attempt++ {

		if c.Nonce == "" {
			if err := c.GetNonce(ctx); err != nil {
				return nil, err
			}
		}

		protected := map[string]any{
			"alg":   "RS256",
			"nonce": c.Nonce,
			"url":   url,
		}

		if useAccount {
			if c.AccountURL == "" {
				return nil, errors.New(
					"ACME Account URL fehlt",
				)
			}

			protected["kid"] = c.AccountURL
		} else {
			jwk, err := PublicJWK(
				&c.AccountKey.PublicKey,
			)
			if err != nil {
				return nil, err
			}

			protected["jwk"] = jwk
		}

		protectedJSON, err := json.Marshal(protected)
		if err != nil {
			return nil, err
		}

		protected64 :=
			base64.RawURLEncoding.EncodeToString(
				protectedJSON,
			)

		payload64 :=
			base64.RawURLEncoding.EncodeToString(
				payload,
			)

		signingInput :=
			protected64 + "." + payload64

		hash := sha256.Sum256(
			[]byte(signingInput),
		)

		signature, err := rsa.SignPKCS1v15(
			rand.Reader,
			c.AccountKey,
			crypto.SHA256,
			hash[:],
		)
		if err != nil {
			return nil, err
		}

		body := map[string]string{
			"protected": protected64,
			"payload":   payload64,
			"signature": base64.RawURLEncoding.EncodeToString(
				signature,
			),
		}

		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			url,
			strings.NewReader(string(bodyJSON)),
		)
		if err != nil {
			return nil, err
		}

		req.Header.Set(
			"Content-Type",
			"application/jose+json",
		)
		req.Header.Set(
			"Accept",
			"application/json",
		)

		fmt.Println("ACME HTTP POST:", url)
		fmt.Println("ACME HTTP Request wird gesendet...")

		resp, err := c.HTTPClient.Do(req)

		fmt.Println("ACME HTTP Request ist zurück.")

		if err != nil {
			return nil, fmt.Errorf(
				"ACME HTTP Request fehlgeschlagen: %w",
				err,
			)
		}

		fmt.Println(
			"ACME HTTP Status:",
			resp.Status,
		)

		if nonce := resp.Header.Get("Replay-Nonce"); nonce != "" {
			c.Nonce = nonce
		}

		// Nur ein 400 mit dem ACME-Fehler
		// "badNonce" darf mit einer neuen Nonce
		// wiederholt werden.
		if resp.StatusCode == http.StatusBadRequest &&
			attempt == 0 {

			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()

			if readErr != nil {
				return nil, fmt.Errorf(
					"ACME Fehlerantwort konnte nicht gelesen werden: %w",
					readErr,
				)
			}

			var acmeErr ACMEError

			if err := json.Unmarshal(
				body,
				&acmeErr,
			); err == nil &&
				acmeErr.Type ==
					"urn:ietf:params:acme:error:badNonce" {

				fmt.Println(
					"ACME Nonce ungültig, wiederhole Request...",
				)

				if c.Nonce == "" {
					if err := c.GetNonce(ctx); err != nil {
						return nil, err
					}
				}

				continue
			}

			// Kein badNonce:
			// Die 400-Antwort gehört an den Aufrufer zurück.
			resp.Body = io.NopCloser(
				strings.NewReader(string(body)),
			)

			return resp, nil
		}

		return resp, nil
	}

	return nil, errors.New(
		"ACME Request konnte wegen ungültiger Nonce nicht abgeschlossen werden",
	)
}

func PublicJWK(
	key *rsa.PublicKey,
) (map[string]string, error) {

	return map[string]string{
		"kty": "RSA",
		"n": base64.RawURLEncoding.EncodeToString(
			key.N.Bytes(),
		),
		"e": base64.RawURLEncoding.EncodeToString(
			intToBytes(key.E),
		),
	}, nil
}

func JWKThumbprint(
	key *rsa.PublicKey,
) (string, error) {

	n := base64.RawURLEncoding.EncodeToString(
		key.N.Bytes(),
	)

	e := base64.RawURLEncoding.EncodeToString(
		intToBytes(key.E),
	)

	// RFC 7638: exakt diese Mitglieder
	// und lexikografische Reihenfolge.
	canonical :=
		`{"e":"` +
			e +
			`","kty":"RSA","n":"` +
			n +
			`"}`

	hash := sha256.Sum256(
		[]byte(canonical),
	)

	return base64.RawURLEncoding.EncodeToString(
		hash[:],
	), nil
}

func intToBytes(v int) []byte {
	if v == 0 {
		return []byte{0}
	}

	var result []byte

	for v > 0 {
		result = append(
			[]byte{byte(v & 0xff)},
			result...,
		)

		v >>= 8
	}

	return result
}

func WriteCertificate(
	path string,
	data []byte,
) error {

	if err := EnsureParentDirectory(path); err != nil {
		return err
	}

	if err := os.WriteFile(
		path,
		data,
		0644,
	); err != nil {
		return fmt.Errorf(
			"Zertifikat konnte nicht gespeichert werden: %w",
			err,
		)
	}

	return nil
}
