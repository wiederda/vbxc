package stdlib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ipv64APIURL = "https://ipv64.net/api"

type ipv64Response struct {
	Info      string `json:"info"`
	Status    string `json:"status"`
	AddRecord string `json:"add_record,omitempty"`
	DelRecord string `json:"del_record,omitempty"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

func IPv64SplitFQDN(fqdn string) (string, string, error) {
	fqdn = strings.TrimSpace(fqdn)
	fqdn = strings.TrimSuffix(fqdn, ".")

	const prefix = "_acme-challenge."

	if !strings.HasPrefix(strings.ToLower(fqdn), prefix) {
		return "", "", fmt.Errorf(
			"ungültiger DNS-01 FQDN für IPv64: %q",
			fqdn,
		)
	}

	domain := fqdn[len(prefix):]

	if domain == "" {
		return "", "", errors.New("IPv64 Domain fehlt")
	}

	return domain, "_acme-challenge", nil
}

func (p *IPv64Provider) addRecord(
	ctx context.Context,
	domain string,
	prefix string,
	recordType string,
	content string,
) error {
	if strings.TrimSpace(p.Token) == "" {
		return errors.New("IPv64 API Token fehlt")
	}

	values := url.Values{}
	values.Set("add_record", domain)
	values.Set("praefix", prefix)
	values.Set("type", recordType)
	values.Set("content", content)

	return p.request(ctx, http.MethodPost, values)
}

func (p *IPv64Provider) deleteRecord(
	ctx context.Context,
	domain string,
	prefix string,
	recordType string,
	content string,
) error {
	if strings.TrimSpace(p.Token) == "" {
		return errors.New("IPv64 API Token fehlt")
	}

	values := url.Values{}
	values.Set("del_record", domain)
	values.Set("praefix", prefix)
	values.Set("type", recordType)
	values.Set("content", content)

	return p.request(ctx, http.MethodDelete, values)
}

func (p *IPv64Provider) request(
	ctx context.Context,
	method string,
	values url.Values,
) error {
	req, err := http.NewRequestWithContext(
		ctx,
		method,
		ipv64APIURL,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return fmt.Errorf(
			"IPv64 HTTP-Anfrage konnte nicht erstellt werden: %w",
			err,
		)
	}

	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(
			"IPv64 API nicht erreichbar: %w",
			err,
		)
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf(
			"IPv64 Antwort konnte nicht gelesen werden: %w",
			err,
		)
	}

	responseText := strings.TrimSpace(string(responseBody))

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf(
			"IPv64 API Rate Limit erreicht (HTTP %d): %s",
			resp.StatusCode,
			responseText,
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"IPv64 API Fehler: HTTP %d: %s",
			resp.StatusCode,
			responseText,
		)
	}

	var response ipv64Response

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf(
			"IPv64 Antwort ist kein gültiges JSON: %w",
			err,
		)
	}

	if strings.EqualFold(response.Info, "success") {
		return nil
	}

	message := response.Error

	if message == "" {
		message = response.Message
	}

	if message == "" {
		message = response.Info
	}

	if message == "" {
		message = response.Status
	}

	if message == "" {
		message = responseText
	}

	return fmt.Errorf("IPv64 API Fehler: %s", message)
}
