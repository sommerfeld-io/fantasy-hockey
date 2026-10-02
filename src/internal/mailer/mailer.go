// Package mailer sends outbound email over SMTP. It is the only package
// that imports net/smtp; internal/auth calls it to deliver login codes.
package mailer

import (
	"fmt"
	"net"
	"net/smtp"
	"regexp"
)

// defaultFrom is used when SMTP_USERNAME is empty, matching a local dev
// capture tool that accepts any sender.
const defaultFrom = "no-reply@fantasy-hockey.local"

// Sender emails body under subject to to. It is a func type, not an
// interface, so tests can substitute a one-line closure fake (AD-3).
type Sender func(to, subject, body string) error

// sendMailFunc matches net/smtp.SendMail's signature so tests can inject a
// fake implementation without opening a real network connection.
type sendMailFunc func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

// NewSMTPSender builds a Sender that delivers mail via net/smtp. host, port,
// username, and password are all read by the caller from env vars and may
// all be empty - the app must still start with none of them set (AD-12).
// An empty username skips SMTP AUTH entirely, matching a no-auth local
// capture server; a non-empty username always authenticates via
// smtp.PlainAuth.
func NewSMTPSender(host, port, username, password string) Sender {
	return newSMTPSender(host, port, username, password, smtp.SendMail)
}

func newSMTPSender(host, port, username, password string, send sendMailFunc) Sender {
	return func(to, subject, body string) error {
		if host == "" {
			return fmt.Errorf("mailer: SMTP_HOST is not set")
		}
		if port == "" {
			return fmt.Errorf("mailer: SMTP_PORT is not set")
		}

		var auth smtp.Auth
		from := defaultFrom
		if username != "" {
			auth = smtp.PlainAuth("", username, password, host)
			from = username
		}

		msg := fmt.Appendf(nil, "From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", from, to, subject, body)
		if err := send(net.JoinHostPort(host, port), auth, from, []string{to}, msg); err != nil {
			return &redactedError{cause: err, text: "mailer: send mail: " + redactRecipient(err.Error(), to)}
		}
		return nil
	}
}

// redactedRecipient replaces the recipient address in returned errors, so a
// server reply that echoes it never reaches a log line.
const redactedRecipient = "[recipient]"

// redactRecipient replaces every occurrence of to in text, ignoring case,
// since a server may echo the address in a different case.
func redactRecipient(text, to string) string {
	if to == "" {
		return text
	}
	return regexp.MustCompile(`(?i)`+regexp.QuoteMeta(to)).ReplaceAllString(text, redactedRecipient)
}

// redactedError carries the redacted text but still unwraps to the original
// cause so errors.Is and errors.As keep working.
type redactedError struct {
	cause error
	text  string
}

func (e *redactedError) Error() string { return e.text }

func (e *redactedError) Unwrap() error { return e.cause }
