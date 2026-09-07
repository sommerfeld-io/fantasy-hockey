// Package mailer sends email via Gmail SMTP (smtp.gmail.com:587, STARTTLS)
// using the standard library's net/smtp.
package mailer

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

const (
	gmailHost = "smtp.gmail.com"
	gmailPort = "587"
)

// Mailer sends email through a Gmail account authenticated with an App
// Password (SMTP_USERNAME/SMTP_APP_PASSWORD), not the account's own password.
type Mailer struct {
	username string
	password string
	addr     string
}

// New creates a Mailer that authenticates as username (a Gmail address) using
// password (a Gmail App Password) and sends through smtp.gmail.com:587.
func New(username, password string) *Mailer {
	return newWithAddr(username, password, net.JoinHostPort(gmailHost, gmailPort))
}

// newWithAddr is the same as New but lets tests point at a local SMTP
// listener instead of the real Gmail host.
func newWithAddr(username, password, addr string) *Mailer {
	return &Mailer{username: username, password: password, addr: addr}
}

// Send delivers a plain-text email to "to". smtp.SendMail negotiates STARTTLS
// automatically when the server advertises it, which smtp.gmail.com always
// does on port 587.
func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	msg := buildMessage(m.username, to, subject, body)
	auth := smtp.PlainAuth("", m.username, m.password, gmailHost)

	errCh := make(chan error, 1)
	go func() {
		errCh <- smtp.SendMail(m.addr, auth, m.username, []string{to}, msg)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("mailer: send email: %w", err)
		}
		return nil
	}
}

// stripCRLF removes carriage-return and line-feed characters so a header
// value can never inject an extra header or recipient into the raw message.
// Current callers only ever pass trusted, internally-generated values, but
// Send is a general-purpose primitive other packages may call with less
// trusted input later.
func stripCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	return strings.ReplaceAll(s, "\n", "")
}

// buildMessage assembles a minimal RFC 5322 message with From/To/Subject
// headers and a plain-text body.
func buildMessage(from, to, subject, body string) []byte {
	from, to, subject = stripCRLF(from), stripCRLF(to), stripCRLF(subject)
	return []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", from, to, subject, body))
}
