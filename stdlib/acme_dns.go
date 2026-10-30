package stdlib

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

func DNS01FQDN(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")

	return "_acme-challenge." + domain
}

func FindDNS01Challenge(authz *Authorization) (*Challenge, error) {
	for _, challenge := range authz.Challenges {
		if challenge.Type == "dns-01" {
			return &challenge, nil
		}
	}

	return nil, errors.New("Authorization enthält keine DNS-01 Challenge")
}

func (c *ACMEClient) DNS01Value(token string) (string, error) {
	if c.AccountKey == nil {
		return "", errors.New("ACME Account Key wurde nicht geladen")
	}

	thumbprint, err := JWKThumbprint(&c.AccountKey.PublicKey)
	if err != nil {
		return "", err
	}

	return DNS01Value(token, thumbprint), nil
}

func DNS01Value(token, thumbprint string) string {
	keyAuthorization := token + "." + thumbprint

	hash := sha256.Sum256([]byte(keyAuthorization))

	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func PrintDNSChallenge(domain, value string) {
	fmt.Println("DNS Name :", DNS01FQDN(domain))
	fmt.Println("TXT Wert :", value)
}

func PrintDNSProviderList() {
	fmt.Println("DNS Provider")
	fmt.Println("============")
	fmt.Println()

	for _, name := range dnsProviderNames {
		fmt.Println(name)
	}
}

func PrintDNSProviderDetails(provider DNSProvider) {
	config := provider.Config()

	fmt.Println("DNS Provider:", provider.Name())
	fmt.Println("Beschreibung:", provider.Description())
	fmt.Println()

	fmt.Println("Authentifizierung:")
	fmt.Println("  ", config.Authentication)
	fmt.Println()

	if len(config.Required) > 0 {
		fmt.Println("Erforderlich:")

		for _, field := range config.Required {
			fmt.Printf("  %-12s %s\n", field.Name, field.Description)
		}

		fmt.Println()
	}

	if len(config.Optional) > 0 {
		fmt.Println("Optional:")

		for _, field := range config.Optional {
			fmt.Printf("  %-12s %s\n", field.Name, field.Description)
		}

		fmt.Println()
	}

	fmt.Println("Unterstützt:")
	fmt.Printf("  DNS-01          %s\n", yesNo(true))
	fmt.Printf("  TXT setzen      %s\n", yesNo(config.SupportsTXT))
	fmt.Printf("  TXT löschen     %s\n", yesNo(config.SupportsDelete))
}

func yesNo(value bool) string {
	if value {
		return "ja"
	}

	return "nein"
}

func ResolveACMEDirectory(directory string) string {
	switch strings.ToLower(strings.TrimSpace(directory)) {
	case "letsencrypt-stage", "letsencrypt-staging":
		return LEStaging
	case "letsencrypt-prod", "letsencrypt-production":
		return LEProduction
	default:
		return directory
	}
}
