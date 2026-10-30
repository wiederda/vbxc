// ------------------------
// stdlib_ad.go
// ------------------------

package stdlib

import (
	"fmt"
	"strings"
	valpkg "vbxc/value"

	"github.com/go-ldap/ldap/v3"
)

func initADFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "ad."

	// ------------------------
	// ad.GetUser
	// ------------------------
	Register(ns+"GetUser", "ad", "samAccountName",
		"Gibt Attribute eines AD-Benutzers als Map zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 || args[0].Str == "" {
				return valpkg.ErrorVal("ad.GetUser(samAccountName)")
			}

			conn, cfg, err := adConnect()
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer conn.Close()

			filter := fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))",
				adEscapeFilter(args[0].Str))

			entries, err := adSearch(conn, cfg.BaseDN, filter, adDefaultUserAttrs)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			if len(entries) == 0 {
				return valpkg.ErrorVal("Benutzer nicht gefunden: " + args[0].Str)
			}

			return adEntryToMap(entries[0])
		})

	// ------------------------
	// ad.GetGroup
	// ------------------------
	Register(ns+"GetGroup", "ad", "groupName",
		"Gibt Attribute einer AD-Gruppe zurück (inkl. Mitgliederliste als Array).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 || args[0].Str == "" {
				return valpkg.ErrorVal("ad.GetGroup(groupName)")
			}

			conn, cfg, err := adConnect()
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer conn.Close()

			filter := fmt.Sprintf("(&(objectClass=group)(sAMAccountName=%s))",
				adEscapeFilter(args[0].Str))

			entries, err := adSearch(conn, cfg.BaseDN, filter, adDefaultGroupAttrs)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			if len(entries) == 0 {
				return valpkg.ErrorVal("Gruppe nicht gefunden: " + args[0].Str)
			}

			return adEntryToMap(entries[0])
		})

	// ------------------------
	// ad.GetMembers
	// ------------------------
	Register(ns+"GetMembers", "ad", "groupName",
		"Gibt alle Mitglieder einer AD-Gruppe als Array von sAMAccountNames zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 || args[0].Str == "" {
				return valpkg.ErrorVal("ad.GetMembers(groupName)")
			}

			conn, cfg, err := adConnect()
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer conn.Close()

			// 1. Gruppe suchen um DN zu ermitteln
			groupFilter := fmt.Sprintf("(&(objectClass=group)(sAMAccountName=%s))",
				adEscapeFilter(args[0].Str))

			groupEntries, err := adSearch(conn, cfg.BaseDN, groupFilter, []string{"distinguishedName"})
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			if len(groupEntries) == 0 {
				return valpkg.ErrorVal("Gruppe nicht gefunden: " + args[0].Str)
			}

			groupDN := groupEntries[0].GetAttributeValue("distinguishedName")

			// 2. Alle Benutzer suchen die Mitglied dieser Gruppe sind
			// memberOf=<groupDN> findet auch verschachtelte Mitglieder nicht –
			// für rekursive Auflösung: LDAP_MATCHING_RULE_IN_CHAIN (1.2.840.113556.1.4.1941)
			memberFilter := fmt.Sprintf("(&(objectClass=user)(memberOf:1.2.840.113556.1.4.1941:=%s))",
				adEscapeFilter(groupDN))

			members, err := adSearch(conn, cfg.BaseDN, memberFilter,
				[]string{"sAMAccountName", "displayName", "mail"})
			if err != nil {
				// Fallback: einfaches memberOf ohne rekursive Auflösung
				memberFilter = fmt.Sprintf("(&(objectClass=user)(memberOf=%s))",
					adEscapeFilter(groupDN))
				members, err = adSearch(conn, cfg.BaseDN, memberFilter,
					[]string{"sAMAccountName", "displayName", "mail"})
				if err != nil {
					return valpkg.ErrorVal(err.Error())
				}
			}

			arr := make([]valpkg.Value, len(members))
			for i, entry := range members {
				arr[i] = adEntryToMap(entry)
			}

			return valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
		})

	// ------------------------
	// ad.GetUserGroups
	// ------------------------
	Register(ns+"GetUserGroups", "ad", "samAccountName",
		"Gibt alle Gruppen zurück, in denen ein Benutzer Mitglied ist (rekursiv).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 || args[0].Str == "" {
				return valpkg.ErrorVal("ad.GetUserGroups(samAccountName)")
			}

			conn, cfg, err := adConnect()
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer conn.Close()

			// 1. Benutzer-DN ermitteln
			userFilter := fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))",
				adEscapeFilter(args[0].Str))

			userEntries, err := adSearch(conn, cfg.BaseDN, userFilter,
				[]string{"distinguishedName"})
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			if len(userEntries) == 0 {
				return valpkg.ErrorVal("Benutzer nicht gefunden: " + args[0].Str)
			}

			userDN := userEntries[0].GetAttributeValue("distinguishedName")

			// 2. Alle Gruppen suchen bei denen der Benutzer Mitglied ist (rekursiv)
			groupFilter := fmt.Sprintf("(&(objectClass=group)(member:1.2.840.113556.1.4.1941:=%s))",
				adEscapeFilter(userDN))

			groups, err := adSearch(conn, cfg.BaseDN, groupFilter,
				[]string{"sAMAccountName", "cn", "description", "distinguishedName"})
			if err != nil {
				// Fallback: direkte memberOf-Abfrage ohne rekursive Auflösung
				groupFilter = fmt.Sprintf("(&(objectClass=group)(member=%s))",
					adEscapeFilter(userDN))
				groups, err = adSearch(conn, cfg.BaseDN, groupFilter,
					[]string{"sAMAccountName", "cn", "description", "distinguishedName"})
				if err != nil {
					return valpkg.ErrorVal(err.Error())
				}
			}

			arr := make([]valpkg.Value, len(groups))
			for i, entry := range groups {
				arr[i] = adEntryToMap(entry)
			}

			return valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
		})

	// ------------------------
	// ad.Exists
	// ------------------------
	Register(ns+"Exists", "ad", "samAccountName",
		"Prüft ob eine Gruppe mit diesem sAMAccountName im AD existiert. Gibt true/false zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 || args[0].Str == "" {
				return valpkg.ErrorVal("ad.Exists(samAccountName)")
			}

			conn, cfg, err := adConnect()
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer conn.Close()

			filter := fmt.Sprintf("(&(objectClass=group)(sAMAccountName=%s))",
				adEscapeFilter(args[0].Str))

			entries, err := adSearch(conn, cfg.BaseDN, filter, []string{"sAMAccountName"})
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			return valpkg.BoolVal(len(entries) > 0)
		})

	// ------------------------
	// ad.UserExists
	// ------------------------
	Register(ns+"UserExists", "ad", "samAccountName",
		"Prüft ob ein Benutzer im AD existiert. Gibt 'user', 'ambiguous', '' oder ErrorVal zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 || args[0].Str == "" {
				return valpkg.ErrorVal("ad.UserExists(samAccountName)")
			}

			conn, cfg, err := adConnect()
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer conn.Close()

			filter := fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))",
				adEscapeFilter(args[0].Str))

			entries, err := adSearch(conn, cfg.BaseDN, filter, []string{"sAMAccountName"})
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			switch len(entries) {
			case 0:
				return valpkg.StrVal("")
			case 1:
				return valpkg.StrVal("user")
			default:
				return valpkg.StrVal("ambiguous")
			}
		})

	// ------------------------
	// ad.Search
	// ------------------------
	Register(ns+"Search", "ad", "filter [, attrs...]",
		"Führt eine rohe LDAP-Suche durch. Gibt ein Array von Maps zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 || args[0].Str == "" {
				return valpkg.ErrorVal("ad.Search(filter [, attr1, attr2, ...])")
			}

			conn, cfg, err := adConnect()
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer conn.Close()

			filter := args[0].Str

			// Sicherheits-Check: Filter muss mit ( beginnen
			if !strings.HasPrefix(strings.TrimSpace(filter), "(") {
				filter = "(" + filter + ")"
			}

			// Optionale Attribute aus weiteren Argumenten
			var attrs []string
			for _, a := range args[1:] {
				if a.Kind == valpkg.KindArr {
					for _, v := range a.Arr {
						attrs = append(attrs, v.Str)
					}
				} else if a.Str != "" {
					attrs = append(attrs, a.Str)
				}
			}

			entries, err := adSearch(conn, cfg.BaseDN, filter, attrs)
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}

			arr := make([]valpkg.Value, len(entries))
			for i, entry := range entries {
				arr[i] = adEntryToMap(entry)
			}

			return valpkg.Value{Kind: valpkg.KindArr, Arr: arr}
		})

	// ------------------------
	// ad.GetOU
	// ------------------------
	Register(ns+"GetOU", "ad", "ouName",
		"Gibt Informationen und Inhalt einer Organizational Unit zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 || args[0].Str == "" {
				return valpkg.ErrorVal("ad.GetOU(ouName)")
			}

			conn, cfg, err := adConnect()
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			defer conn.Close()

			filter := fmt.Sprintf("(&(objectClass=organizationalUnit)(ou=%s))",
				adEscapeFilter(args[0].Str))

			entries, err := adSearch(conn, cfg.BaseDN, filter,
				[]string{"ou", "description", "distinguishedName", "whenCreated"})
			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
			if len(entries) == 0 {
				return valpkg.ErrorVal("OU nicht gefunden: " + args[0].Str)
			}

			// Inhalt der OU laden (direkte Kinder)
			ouDN := entries[0].GetAttributeValue("distinguishedName")
			childReq := ldap.NewSearchRequest(
				ouDN,
				ldap.ScopeSingleLevel, // Nur direkte Kinder
				ldap.NeverDerefAliases,
				0, 0, false,
				"(objectClass=*)",
				[]string{"sAMAccountName", "cn", "objectClass", "distinguishedName"},
				nil,
			)

			childResult, err := conn.Search(childReq)
			if err == nil && len(childResult.Entries) > 0 {
				children := make([]valpkg.Value, len(childResult.Entries))
				for i, child := range childResult.Entries {
					children[i] = adEntryToMap(child)
				}
				ouMap := adEntryToMap(entries[0])
				ouMap.Map["children"] = valpkg.Value{Kind: valpkg.KindArr, Arr: children}
				return ouMap
			}

			return adEntryToMap(entries[0])
		})
}

func init() {
	initADFunctions()
}
