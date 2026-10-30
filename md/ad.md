# 🏢 ad.* – Active Directory / LDAP-Funktionen

Dient zum Lesen von Benutzern, Gruppen und Organisationseinheiten aus einem Active Directory.
Plattformübergreifend (Windows, Linux, macOS) via `go-ldap/ldap`.
Credentials und Server werden aus Umgebungsvariablen gelesen (`AD_USER`, `AD_PASS`, optional `AD_SERVER`, `USERDNSDOMAIN`, `LOGONSERVER`).

---

## ad.GetUser(samAccountName)
- **Konkret:**
  Gibt Attribute eines AD-Benutzers als Map zurück.
  Standardattribute: `dn`, `sAMAccountName`, `displayName`, `givenName`, `sn`, `mail`, `telephoneNumber`, `department`, `title`, `memberOf`, `userAccountControl`, `whenCreated`, `lastLogon`, `distinguishedName`.
  Attribute mit mehreren Werten (z. B. `memberOf`) werden als `ArrVal` zurückgegeben.
- **Parameter:**
  - `samAccountName`: Login-Name des Benutzers.
- **Rückgabe:**
  `KindMap`, `ErrorVal` wenn nicht gefunden oder LDAP-Fehler.

---

## ad.GetGroup(groupName)
- **Konkret:**
  Gibt Attribute einer AD-Gruppe zurück.
  Standardattribute: `dn`, `sAMAccountName`, `cn`, `description`, `member`, `memberOf`, `distinguishedName`, `groupType`, `whenCreated`.
  `member` enthält die DNs aller direkten Mitglieder als `ArrVal`.
- **Parameter:**
  - `groupName`: `sAMAccountName` der Gruppe.
- **Rückgabe:**
  `KindMap`, `ErrorVal` wenn nicht gefunden oder LDAP-Fehler.

---

## ad.GetMembers(groupName)
- **Konkret:**
  Gibt alle Benutzer einer Gruppe als Array zurück.
  Löst Mitgliedschaft rekursiv auf via LDAP Matching Rule `1.2.840.113556.1.4.1941`.
  Fallback auf einfaches `memberOf` wenn der Server die Matching Rule nicht unterstützt.
- **Parameter:**
  - `groupName`: `sAMAccountName` der Gruppe.
- **Rückgabe:**
  `ArrVal`
  Array von Maps. Je Eintrag: `sAMAccountName`, `displayName`, `mail`, `dn`.
  `ErrorVal` wenn nicht gefunden oder LDAP-Fehler.

---

## ad.GetUserGroups(samAccountName)
- **Konkret:**
  Gibt alle Gruppen zurück, in denen ein Benutzer Mitglied ist (rekursiv, inkl. verschachtelter Gruppen).
  Fallback auf direkte Mitgliedschaft wenn rekursive Auflösung nicht unterstützt wird.
- **Parameter:**
  - `samAccountName`: Login-Name des Benutzers.
- **Rückgabe:**
  `ArrVal`
  Array von Maps. Je Eintrag: `sAMAccountName`, `cn`, `description`, `distinguishedName`.
  `ErrorVal` wenn nicht gefunden oder LDAP-Fehler.

---

## ad.UserExists(samAccountName)
- **Konkret:**
  Prüft ob ein Benutzer im AD existiert.
  Gibt `"ambiguous"` zurück wenn mehrere Objekte denselben `sAMAccountName` haben.
- **Parameter:**
  - `samAccountName`: Login-Name des Benutzers.
- **Rückgabe:**
  `StrVal`
  `"user"` = eindeutig gefunden, `"ambiguous"` = mehrere Treffer, `""` = nicht gefunden.
  `ErrorVal` bei LDAP-Fehler.

---

## ad.Exists(samAccountName)
- **Konkret:**
  Prüft ob eine Gruppe mit diesem Namen im AD existiert.
  Sucht ausschließlich nach `objectClass=group`.
- **Parameter:**
  - `samAccountName`: `sAMAccountName` der Gruppe.
- **Rückgabe:**
  `BoolVal`

---

## ad.GetOU(ouName)
- **Konkret:**
  Gibt Informationen zu einer Organizational Unit zurück inkl. aller direkten Kinder (nicht rekursiv).
  Kinder werden im Schlüssel `children` als `ArrVal` zurückgegeben.
- **Parameter:**
  - `ouName`: Name der OU.
- **Rückgabe:**
  `KindMap` mit `ou`, `description`, `distinguishedName`, `whenCreated`, `children`.
  `ErrorVal` wenn nicht gefunden oder LDAP-Fehler.

---

## ad.Search(filter [, attrs...])
- **Konkret:**
  Führt eine rohe LDAP-Suche mit eigenem Filter durch.
  Filter ohne führende `(` werden automatisch geklammert.
  Attribute können als einzelne Strings oder als `ArrVal` übergeben werden.
  Ohne Attributangabe werden alle verfügbaren Attribute zurückgegeben.
- **Parameter:**
  - `filter`: LDAP-Filterausdruck (z. B. `"(objectClass=user)"`).
  - `attrs...`: Optional. Gewünschte Attributnamen.
- **Rückgabe:**
  `ArrVal`
  Array von Maps, ein Eintrag pro gefundenem Objekt.
  `ErrorVal` bei ungültigem Filter oder LDAP-Fehler.