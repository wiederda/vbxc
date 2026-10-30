package stdlib

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type AccountState struct {
	Directory string `json:"directory"`
	URL       string `json:"account_url"`
	Email     string `json:"email"`
}

func LoadOrCreateAccount(
	ctx context.Context,
	client *ACMEClient,
	keyPath string,
	statePath string,
	email string,
	acceptTOS bool,
) (*AccountState, error) {

	keyPath = filepath.Clean(keyPath)
	statePath = filepath.Clean(statePath)

	key, err := LoadOrCreateAccountKey(keyPath)
	if err != nil {
		return nil, err
	}

	client.AccountKey = key

	fmt.Println("Lade ACME Account Status...")
	fmt.Println("Account Status:", statePath)

	state, stateErr := LoadAccountState(statePath)

	fmt.Println("ACME Account Status geladen.")

	if stateErr == nil {
		fmt.Println("Account Status gefunden.")
		fmt.Println("Directory:", state.Directory)
		fmt.Println("Account URL:", state.URL)

		if state.Directory == client.DirectoryURL && state.URL != "" {
			fmt.Println("Vorhandenes ACME Account wird verwendet.")
			return state, nil
		}
	} else {
		fmt.Println("Kein gültiger Account Status:", stateErr)
	}

	fmt.Println("Suche ACME Account beim Server...")

	accountURL, err := client.FindExistingAccount(ctx)
	if err == nil && accountURL != "" {
		state := &AccountState{
			Directory: client.DirectoryURL,
			URL:       accountURL,
			Email:     email,
		}

		if err := SaveAccountState(statePath, state); err != nil {
			return nil, err
		}

		return state, nil
	}

	if !acceptTOS {
		return nil, errors.New("ACME Terms of Service wurden nicht akzeptiert")
	}

	fmt.Println("Kein vorhandenes ACME Account gefunden.")
	fmt.Println("Erstelle neues ACME Account...")

	accountURL, err = client.CreateAccount(ctx, email)
	if err != nil {
		return nil, err
	}

	state = &AccountState{
		Directory: client.DirectoryURL,
		URL:       accountURL,
		Email:     email,
	}

	if err := SaveAccountState(statePath, state); err != nil {
		return nil, err
	}

	return state, nil
}

func LoadOrCreateAccountKey(path string) (*rsa.PrivateKey, error) {
	fmt.Println("ACME Account Key:", path)

	data, err := os.ReadFile(path)

	if err == nil {
		fmt.Println("Vorhandenen ACME Account Key laden...")
		fmt.Printf("Account Key Größe: %d Bytes\n", len(data))

		fmt.Println("ParseAccountPrivateKey wird aufgerufen...")
		key, err := ParseAccountPrivateKey(data)
		fmt.Println("ParseAccountPrivateKey ist zurück.")

		if err != nil {
			return nil, fmt.Errorf(
				"ACME Account Key konnte nicht gelesen werden: %w",
				err,
			)
		}

		return key, nil
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf(
			"ACME Account Key konnte nicht gelesen werden: %w",
			err,
		)
	}

	fmt.Println("Erzeuge neuen ACME Account Key...")
	fmt.Println("Generiere RSA-4096 Key...")

	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf(
			"Account Key konnte nicht erzeugt werden: %w",
			err,
		)
	}

	fmt.Println("ACME Account Key wurde erzeugt.")

	data, err = MarshalAccountPrivateKey(key)
	if err != nil {
		return nil, err
	}

	if err := EnsureParentDirectory(path); err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, fmt.Errorf(
			"Account Key konnte nicht gespeichert werden: %w",
			err,
		)
	}

	fmt.Println("ACME Account Key wurde gespeichert.")

	return key, nil
}

func ParseAccountPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	fmt.Println("Parse ACME Account Key...")

	block, _ := pem.Decode(data)

	if block == nil {
		return nil, errors.New(
			"ACME Account Key enthält keinen gültigen PEM-Block",
		)
	}

	fmt.Printf("PEM-Typ: %s, Größe: %d Bytes\n",
		block.Type,
		len(block.Bytes),
	)

	switch block.Type {
	case "RSA PRIVATE KEY":
		fmt.Println("Parse PKCS#1...")

		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf(
				"RSA Account Key konnte nicht gelesen werden: %w",
				err,
			)
		}

		fmt.Println("PKCS#1 erfolgreich gelesen.")
		return key, nil

	case "PRIVATE KEY":
		fmt.Println("Parse PKCS#8...")

		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf(
				"PKCS#8 Account Key konnte nicht gelesen werden: %w",
				err,
			)
		}

		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New(
				"ACME Account Key ist kein RSA Key",
			)
		}

		fmt.Println("PKCS#8 erfolgreich gelesen.")
		return rsaKey, nil

	default:
		return nil, fmt.Errorf(
			"unbekannter ACME Account Key PEM-Typ: %s",
			block.Type,
		)
	}
}

func MarshalAccountPrivateKey(key *rsa.PrivateKey) ([]byte, error) {
	if key == nil {
		return nil, errors.New(
			"ACME Account Key ist nil",
		)
	}

	der := x509.MarshalPKCS1PrivateKey(key)

	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: der,
	}), nil
}

func LoadAccountState(path string) (*AccountState, error) {
	fmt.Println("LoadAccountState: Start")
	fmt.Println("LoadAccountState: Datei:", path)

	fmt.Println("LoadAccountState: os.ReadFile...")
	data, err := os.ReadFile(path)
	fmt.Println("LoadAccountState: os.ReadFile fertig")

	if err != nil {
		return nil, err
	}

	fmt.Printf("LoadAccountState: %d Bytes gelesen\n", len(data))

	fmt.Println("LoadAccountState: json.Unmarshal...")
	var state AccountState

	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf(
			"Account Statusdatei ist ungültig: %w",
			err,
		)
	}

	fmt.Println("LoadAccountState: json.Unmarshal fertig")

	return &state, nil
}

func SaveAccountState(path string, state *AccountState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf(
			"Account Status konnte nicht erzeugt werden: %w",
			err,
		)
	}

	if err := EnsureParentDirectory(path); err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf(
			"Account Status konnte nicht gespeichert werden: %w",
			err,
		)
	}

	return nil
}
