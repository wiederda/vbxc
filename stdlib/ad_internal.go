// ------------------------
// stdlib_ad_internal.go
// ------------------------

package stdlib

import (
	"fmt"
	"net"
	"os"
	"strings"
	valpkg "vbxc/value"

	"github.com/go-ldap/ldap/v3"
)

// adConfig hält die Verbindungsparameter für eine LDAP-Session.
type adConfig struct {
	Server string
	Port   int
	User   string
	Pass   string
	BaseDN string
}

// adConnect baut eine LDAP-Verbindung auf und führt einen Simple Bind durch.
// Credentials werden aus Umgebungsvariablen gelesen (AD_USER, AD_PASS).
// Der Server wird via adResolveServer() ermittelt.
func adConnect() (*ldap.Conn, *adConfig, error) {
	cfg, err := adBuildConfig()
	if err != nil {
		return nil, nil, err
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
	conn, err := ldap.DialURL("ldap://" + addr)
	if err != nil {
		// Fallback: LDAPS auf 636
		conn, err = ldap.DialURL("ldaps://" + addr)
		if err != nil {
			return nil, nil, fmt.Errorf("LDAP-Verbindung fehlgeschlagen (%s): %w", addr, err)
		}
	}

	if err := conn.Bind(cfg.User, cfg.Pass); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("LDAP-Bind fehlgeschlagen (user=%s): %w", cfg.User, err)
	}

	return conn, cfg, nil
}

// adBuildConfig liest alle nötigen Parameter aus Umgebungsvariablen
// und ermittelt den Server via DNS oder Umgebungsvariable.
func adBuildConfig() (*adConfig, error) {
	user := os.Getenv("AD_USER")
	pass := os.Getenv("AD_PASS")

	if user == "" || pass == "" {
		return nil, fmt.Errorf("AD_USER und AD_PASS Umgebungsvariablen müssen gesetzt sein")
	}

	server, err := adResolveServer()
	if err != nil {
		return nil, err
	}

	domain := adDomainFromUser(user)
	baseDN := adDomainToBaseDN(domain)

	return &adConfig{
		Server: server,
		Port:   389,
		User:   user,
		Pass:   pass,
		BaseDN: baseDN,
	}, nil
}

// adResolveServer ermittelt den LDAP-Server in folgender Reihenfolge:
//  1. Umgebungsvariable AD_SERVER
//  2. Umgebungsvariable USERDNSDOMAIN (→ DNS-SRV Lookup)
//  3. Umgebungsvariable LOGONSERVER (Windows, \\SERVER → SERVER)
//  4. Fehler
func adResolveServer() (string, error) {
	// 1. Explizit gesetzt
	if s := os.Getenv("AD_SERVER"); s != "" {
		return s, nil
	}

	// 2. Domain via DNS-SRV
	if domain := os.Getenv("USERDNSDOMAIN"); domain != "" {
		if host, err := adSRVLookup(domain); err == nil {
			return host, nil
		}
		// Fallback: Domain direkt als Server nutzen
		return domain, nil
	}

	// 3. LOGONSERVER (Windows: "\\DC01" → "DC01")
	if ls := os.Getenv("LOGONSERVER"); ls != "" {
		return strings.TrimLeft(ls, `\`), nil
	}

	return "", fmt.Errorf("kein AD-Server ermittelbar: AD_SERVER, USERDNSDOMAIN oder LOGONSERVER setzen")
}

// adSRVLookup sucht den LDAP-Server via DNS-SRV Record _ldap._tcp.<domain>.
func adSRVLookup(domain string) (string, error) {
	_, addrs, err := net.LookupSRV("ldap", "tcp", domain)
	if err != nil || len(addrs) == 0 {
		return "", fmt.Errorf("DNS-SRV Lookup fehlgeschlagen für %s: %w", domain, err)
	}
	// Erster Eintrag, trailing dot entfernen
	return strings.TrimSuffix(addrs[0].Target, "."), nil
}

// adDomainFromUser extrahiert die Domain aus "DOMAIN\user" oder "user@domain.com".
func adDomainFromUser(user string) string {
	if idx := strings.Index(user, `\`); idx > 0 {
		return user[:idx]
	}
	if idx := strings.Index(user, "@"); idx > 0 {
		return user[idx+1:]
	}
	if domain := os.Getenv("USERDNSDOMAIN"); domain != "" {
		return domain
	}
	return ""
}

// adDomainToBaseDN konvertiert "firma.local" → "DC=firma,DC=local".
func adDomainToBaseDN(domain string) string {
	if domain == "" {
		return ""
	}
	parts := strings.Split(strings.ToLower(domain), ".")
	dcs := make([]string, len(parts))
	for i, p := range parts {
		dcs[i] = "DC=" + p
	}
	return strings.Join(dcs, ",")
}

// adSearch führt eine LDAP-Suche durch und gibt die Ergebnisse zurück.
// filter: LDAP-Filterausdruck, z.B. "(sAMAccountName=jdoe)"
// attrs:  Gewünschte Attribute, leer = alle
func adSearch(conn *ldap.Conn, baseDN, filter string, attrs []string) ([]*ldap.Entry, error) {
	req := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,     // SizeLimit (0 = unbegrenzt)
		0,     // TimeLimit
		false, // TypesOnly
		filter,
		attrs,
		nil,
	)

	result, err := conn.Search(req)
	if err != nil {
		return nil, fmt.Errorf("LDAP-Suche fehlgeschlagen (filter=%s): %w", filter, err)
	}

	return result.Entries, nil
}

// adEntryToMap konvertiert einen LDAP-Entry in eine VBMini-Map (valpkg.KindMap).
// Attribute mit mehreren Werten werden als valpkg.KindArr gespeichert.
func adEntryToMap(entry *ldap.Entry) valpkg.Value {
	m := make(map[string]valpkg.Value)

	m["dn"] = valpkg.StrVal(entry.DN)

	for _, attr := range entry.Attributes {
		name := attr.Name
		vals := attr.Values

		switch len(vals) {
		case 0:
			m[name] = valpkg.StrVal("")
		case 1:
			m[name] = valpkg.StrVal(vals[0])
		default:
			arr := make([]valpkg.Value, len(vals))
			for i, v := range vals {
				arr[i] = valpkg.StrVal(v)
			}
			m[name] = valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
		}
	}

	return valpkg.Value{Kind: valpkg.KindMap, Map: m}
}

// adEscapeFilter escaped Sonderzeichen in LDAP-Filterwerten (RFC 4515).
func adEscapeFilter(s string) string {
	return ldap.EscapeFilter(s)
}

// adDefaultUserAttrs sind die Standard-Attribute die bei GetUser zurückgegeben werden.
var adDefaultUserAttrs = []string{
	"sAMAccountName",
	"displayName",
	"givenName",
	"sn",
	"mail",
	"telephoneNumber",
	"department",
	"title",
	"memberOf",
	"userAccountControl",
	"whenCreated",
	"lastLogon",
	"distinguishedName",
}

// adDefaultGroupAttrs sind die Standard-Attribute die bei GetGroup zurückgegeben werden.
var adDefaultGroupAttrs = []string{
	"sAMAccountName",
	"cn",
	"description",
	"member",
	"memberOf",
	"distinguishedName",
	"groupType",
	"whenCreated",
}
