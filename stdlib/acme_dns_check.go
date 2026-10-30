package stdlib

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

var defaultDNSResolvers = []string{
	"1.1.1.1:53",
	"8.8.8.8:53",
}

func ParseDNSResolvers(value string) []string {
	value = strings.TrimSpace(value)

	if value == "" {
		return append([]string(nil), defaultDNSResolvers...)
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		server := strings.TrimSpace(part)

		if server == "" {
			continue
		}

		// Bereits mit Port angegeben.
		if _, _, err := net.SplitHostPort(server); err == nil {
			result = append(result, server)
			continue
		}

		// IPv4 oder Hostname ohne Port.
		if net.ParseIP(server) != nil || !strings.Contains(server, ":") {
			result = append(result, net.JoinHostPort(server, "53"))
			continue
		}

		// IPv6 ohne Port.
		if ip := net.ParseIP(server); ip != nil {
			result = append(result, net.JoinHostPort(ip.String(), "53"))
			continue
		}

		result = append(result, server)
	}

	if len(result) == 0 {
		return append([]string(nil), defaultDNSResolvers...)
	}

	return result
}

func WaitForTXT(
	ctx context.Context,
	fqdn string,
	expected string,
	servers []string,
	timeout time.Duration,
	interval time.Duration,
) error {
	fqdn = strings.TrimSpace(fqdn)
	fqdn = strings.TrimSuffix(fqdn, ".")
	expected = strings.TrimSpace(expected)

	if fqdn == "" {
		return fmt.Errorf("DNS Name fehlt")
	}

	if expected == "" {
		return fmt.Errorf("erwarteter TXT-Wert fehlt")
	}

	if len(servers) == 0 {
		return fmt.Errorf("keine DNS Resolver angegeben")
	}

	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	if interval <= 0 {
		interval = 5 * time.Second
	}

	fmt.Println("Warte auf öffentliche DNS-Auflösung...")
	fmt.Println("  Name :", fqdn)
	fmt.Println("  Wert :", expected)
	fmt.Println("  DNS  :", strings.Join(servers, ", "))
	fmt.Println()

	deadline := time.Now().Add(timeout)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		for _, server := range servers {
			values, err := lookupTXTServer(ctx, fqdn, server)

			if err != nil {
				fmt.Printf("  %s: %v\n", server, err)
				continue
			}

			for _, value := range values {
				if value == expected {
					fmt.Println()
					fmt.Println("TXT-Eintrag ist öffentlich sichtbar.")
					fmt.Println("Resolver:", server)
					return nil
				}
			}

			fmt.Printf(
				"  %s: TXT-Wert noch nicht gefunden\n",
				server,
			)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf(
				"TXT-Eintrag wurde innerhalb von %s nicht öffentlich gefunden",
				timeout,
			)
		}

		timer := time.NewTimer(interval)

		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()

		case <-timer.C:
		}
	}
}

func lookupTXTServer(
	ctx context.Context,
	fqdn string,
	server string,
) ([]string, error) {
	resolver := &net.Resolver{
		PreferGo: true,

		Dial: func(
			ctx context.Context,
			network string,
			_ string,
		) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout: 5 * time.Second,
			}

			return dialer.DialContext(
				ctx,
				"udp",
				server,
			)
		},
	}

	lookupCtx, cancel := context.WithTimeout(
		ctx,
		10*time.Second,
	)
	defer cancel()

	return resolver.LookupTXT(lookupCtx, fqdn)
}
