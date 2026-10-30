package stdlib

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
	valpkg "vbxc/value"
)

func initSmtpFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "smtp."

	Register(ns+"Send", "smtp", "mailObject", "SMTP (Port 25 / unverschlüsselt)", func(args []valpkg.Value) valpkg.Value {
		return smtpSend("Send", args, sendMailPlain)
	})

	Register(ns+"SendTLS", "smtp", "mailObject", "SMTP über TLS (Port 465)", func(args []valpkg.Value) valpkg.Value {
		return smtpSend("SendTLS", args, sendMailTLS)
	})

	Register(ns+"SendSTARTTLS", "smtp", "mailObject", "SMTP über STARTTLS (Port 587)", func(args []valpkg.Value) valpkg.Value {
		return smtpSend("SendSTARTTLS", args, sendMailSTARTTLS)
	})
}

//
// ---------------- SEND WRAPPER ----------------
//

func smtpSend(
	name string,
	args []valpkg.Value,
	fn func(string, string, string, string, string, string, string, string, string, string, string) error,
) valpkg.Value {

	if len(args) < 1 || args[0].Kind != valpkg.KindObj {
		return valpkg.ErrorVal(name + " erwartet ein Objekt")
	}

	if args[0].Obj == nil {
		return valpkg.ErrorVal("Ungültiges Objekt")
	}

	obj := args[0].Obj.Fields

	getStr := func(key string) string {
		if v, ok := obj[key]; ok && v.Kind == valpkg.KindStr {
			return strings.TrimSpace(v.Str)
		}
		return ""
	}

	err := fn(
		getStr("to"),
		getStr("cc"),
		getStr("bcc"),
		getStr("subject"),
		getStr("body"),
		getStr("server"),
		getStr("port"),
		getStr("user"),
		getStr("pass"),
		getStr("from"),
		getStr("html"),
	)

	if err != nil {
		return valpkg.ErrorVal(err.Error())
	}

	return valpkg.BoolVal(true)
}

//
// ---------------- HELPERS ----------------
//

func parsePort(portStr string, def int) int {
	if portStr == "" {
		return def
	}

	if p, err := strconv.Atoi(strings.TrimSpace(portStr)); err == nil {
		return p
	}

	return def
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	var result []string

	for _, p := range parts {
		p = strings.TrimSpace(p)

		if p != "" {
			result = append(result, p)
		}
	}

	return result
}

func buildMessage(to, cc, subject, body, from, html string) (string, []string, []string) {

	toList := splitList(to)
	ccList := splitList(cc)

	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
	}

	if len(ccList) > 0 {
		headers = append(headers, fmt.Sprintf("Cc: %s", strings.Join(ccList, ",")))
	}

	headers = append(headers,
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
	)

	if html != "" {
		headers = append(headers, `Content-Type: text/html; charset="utf-8"`)
	} else {
		headers = append(headers, `Content-Type: text/plain; charset="utf-8"`)
	}

	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + body

	return message, toList, ccList
}

func resolveFrom(from, user string) string {
	if from != "" {
		return from
	}

	if user != "" {
		return user
	}

	return "noreply@localhost"
}

func buildRecipients(toList, ccList, bccList []string) []string {
	all := append([]string{}, toList...)
	all = append(all, ccList...)
	all = append(all, bccList...)

	return all
}

//
// ---------------- SEND IMPLEMENTATIONS ----------------
//

func sendMailPlain(
	to,
	cc,
	bcc,
	subject,
	body,
	server,
	portStr,
	user,
	pass,
	from,
	html string,
) error {

	if to == "" {
		return fmt.Errorf("Empfänger fehlt")
	}

	if server == "" {
		return fmt.Errorf("SMTP-Server fehlt")
	}

	port := parsePort(portStr, 25)
	from = resolveFrom(from, user)

	msg, toList, ccList := buildMessage(
		to,
		cc,
		subject,
		body,
		from,
		html,
	)

	bccList := splitList(bcc)
	recipients := buildRecipients(toList, ccList, bccList)

	addr := fmt.Sprintf("%s:%d", server, port)

	var auth smtp.Auth

	if user != "" && pass != "" {
		auth = smtp.PlainAuth("", user, pass, server)
	}

	return smtp.SendMail(
		addr,
		auth,
		from,
		recipients,
		[]byte(msg),
	)
}

func sendMailTLS(
	to,
	cc,
	bcc,
	subject,
	body,
	server,
	portStr,
	user,
	pass,
	from,
	html string,
) error {

	if to == "" {
		return fmt.Errorf("Empfänger fehlt")
	}

	if server == "" {
		return fmt.Errorf("SMTP-Server fehlt")
	}

	port := parsePort(portStr, 465)
	from = resolveFrom(from, user)

	msg, toList, ccList := buildMessage(
		to,
		cc,
		subject,
		body,
		from,
		html,
	)

	bccList := splitList(bcc)
	recipients := buildRecipients(toList, ccList, bccList)

	addr := net.JoinHostPort(
		server,
		strconv.Itoa(port),
	)

	conn, err := net.DialTimeout(
		"tcp",
		addr,
		10*time.Second,
	)
	if err != nil {
		return err
	}

	tlsconfig := &tls.Config{
		ServerName: server,
		MinVersion: tls.VersionTLS12,
	}

	tlsConn := tls.Client(conn, tlsconfig)

	c, err := smtp.NewClient(tlsConn, server)
	if err != nil {
		return err
	}

	defer c.Quit()

	if user != "" && pass != "" {
		auth := smtp.PlainAuth(
			"",
			user,
			pass,
			server,
		)

		if err = c.Auth(auth); err != nil {
			return err
		}
	}

	if err = c.Mail(from); err != nil {
		return err
	}

	for _, r := range recipients {
		if err = c.Rcpt(r); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}

	return w.Close()
}

func sendMailSTARTTLS(
	to,
	cc,
	bcc,
	subject,
	body,
	server,
	portStr,
	user,
	pass,
	from,
	html string,
) error {

	if to == "" {
		return fmt.Errorf("Empfänger fehlt")
	}

	if server == "" {
		return fmt.Errorf("SMTP-Server fehlt")
	}

	port := parsePort(portStr, 587)
	from = resolveFrom(from, user)

	msg, toList, ccList := buildMessage(
		to,
		cc,
		subject,
		body,
		from,
		html,
	)

	bccList := splitList(bcc)
	recipients := buildRecipients(toList, ccList, bccList)

	addr := net.JoinHostPort(
		server,
		strconv.Itoa(port),
	)

	conn, err := net.DialTimeout(
		"tcp",
		addr,
		10*time.Second,
	)
	if err != nil {
		return err
	}

	c, err := smtp.NewClient(conn, server)
	if err != nil {
		return err
	}

	defer c.Quit()

	if err = c.Hello("localhost"); err != nil {
		return err
	}

	ok, _ := c.Extension("STARTTLS")
	if !ok {
		return fmt.Errorf("Server unterstützt kein STARTTLS")
	}

	tlsconfig := &tls.Config{
		ServerName: server,
		MinVersion: tls.VersionTLS12,
	}

	if err = c.StartTLS(tlsconfig); err != nil {
		return err
	}

	if err = c.Hello("localhost"); err != nil {
		return err
	}

	if user != "" && pass != "" {
		auth := smtp.PlainAuth(
			"",
			user,
			pass,
			server,
		)

		if err = c.Auth(auth); err != nil {
			return err
		}
	}

	if err = c.Mail(from); err != nil {
		return err
	}

	for _, r := range recipients {
		if err = c.Rcpt(r); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}

	return w.Close()
}

func init() { initSmtpFunctions() }
