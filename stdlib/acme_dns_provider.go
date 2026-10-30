package stdlib

import (
	"context"
	"strings"
)

type DNSProviderConfig struct {
	Authentication string
	Required       []DNSProviderField
	Optional       []DNSProviderField
	SupportsTXT    bool
	SupportsDelete bool
}

type DNSProviderField struct {
	Name        string
	Description string
	Required    bool
}

type DNSProvider interface {
	Name() string
	Description() string
	Config() DNSProviderConfig

	Present(ctx context.Context, fqdn string, value string) error
	Cleanup(ctx context.Context, fqdn string, value string) error
}

type IPv64Provider struct {
	Token string
}

func NewIPv64Provider(token string) *IPv64Provider {
	return &IPv64Provider{
		Token: token,
	}
}

func (p *IPv64Provider) Name() string {
	return "ipv64"
}

func (p *IPv64Provider) Description() string {
	return "IPv64 DNS API"
}

func (p *IPv64Provider) Config() DNSProviderConfig {
	return DNSProviderConfig{
		Authentication: "API Token",
		Required: []DNSProviderField{
			{
				Name:        "token",
				Description: "API Token",
				Required:    true,
			},
		},
		SupportsTXT:    true,
		SupportsDelete: true,
	}
}

func (p *IPv64Provider) Present(ctx context.Context, fqdn string, value string) error {
	domain, prefix, err := IPv64SplitFQDN(fqdn)
	if err != nil {
		return err
	}

	return p.addRecord(ctx, domain, prefix, "TXT", value)
}

func (p *IPv64Provider) Cleanup(ctx context.Context, fqdn string, value string) error {
	domain, prefix, err := IPv64SplitFQDN(fqdn)
	if err != nil {
		return err
	}

	return p.deleteRecord(ctx, domain, prefix, "TXT", value)
}

var dnsProviderNames = []string{"ipv64"}

// GetDNSProvider erstellt eine neue DNSProvider-Instanz für den angegebenen
// Namen und konfiguriert sie mit den übergebenen Werten (z.B. config["token"]).
func GetDNSProvider(name string, config map[string]string) (DNSProvider, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ipv64":
		return NewIPv64Provider(config["token"]), true
	default:
		return nil, false
	}
}
