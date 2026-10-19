package mailer

import (
	"errors"
	"fmt"
	"strings"
)

// Environment variable names for the mail settings. Each has one home here so
// main.go, the tests and the error messages cannot drift apart.
const (
	EnvMailType          = "MAIL_TYPE"
	EnvSMTPHost          = "SMTP_HOST"
	EnvSMTPPort          = "SMTP_PORT"
	EnvSMTPUsername      = "SMTP_USERNAME"
	EnvSMTPAppPassword   = "SMTP_APP_PASSWORD"
	EnvGmailClientID     = "GMAIL_CLIENT_ID"
	EnvGmailClientSecret = "GMAIL_CLIENT_SECRET"
	EnvGmailRefreshToken = "GMAIL_REFRESH_TOKEN"
	EnvGmailSender       = "GMAIL_SENDER"
)

// Allowed MAIL_TYPE values, compared case-insensitively.
const (
	typeSMTP = "SMTP"
	typeAPI  = "API"
)

// Settings holds every mail-related environment value. All may be empty: the
// app must start with no mail settings at all.
type Settings struct {
	MailType          string
	SMTPHost          string
	SMTPPort          string
	SMTPUsername      string
	SMTPAppPassword   string
	GmailClientID     string
	GmailClientSecret string
	GmailRefreshToken string
	GmailSender       string
}

// SettingsFromEnv reads the mail settings through getenv (os.Getenv in
// production), so the environment is read in one place only.
func SettingsFromEnv(getenv func(string) string) Settings {
	return Settings{
		MailType:          getenv(EnvMailType),
		SMTPHost:          getenv(EnvSMTPHost),
		SMTPPort:          getenv(EnvSMTPPort),
		SMTPUsername:      getenv(EnvSMTPUsername),
		SMTPAppPassword:   getenv(EnvSMTPAppPassword),
		GmailClientID:     getenv(EnvGmailClientID),
		GmailClientSecret: getenv(EnvGmailClientSecret),
		GmailRefreshToken: getenv(EnvGmailRefreshToken),
		GmailSender:       getenv(EnvGmailSender),
	}
}

// errAPINotImplemented is returned by the API sender until the Gmail API
// transport exists (Story 10.2).
var errAPINotImplemented = errors.New("mailer: MAIL_TYPE=API is not implemented yet")

// New returns the Sender selected by s.MailType: SMTP (also when unset or
// empty) or API. Any other value is an error, with no fallback to SMTP.
func New(s Settings) (Sender, error) {
	switch selectedType(s) {
	case typeSMTP:
		return NewSMTPSender(s.SMTPHost, s.SMTPPort, s.SMTPUsername, s.SMTPAppPassword), nil
	case typeAPI:
		return func(_, _, _ string) error { return errAPINotImplemented }, nil
	default:
		return nil, fmt.Errorf("mailer: invalid %s %q: allowed values are %s and %s",
			EnvMailType, strings.TrimSpace(s.MailType), typeSMTP, typeAPI)
	}
}

// IgnoredVars names the set variables that belong to the unselected
// transport, in a stable order. It returns names only, never values, and
// nothing for an invalid MAIL_TYPE since startup fails there anyway.
func IgnoredVars(s Settings) []string {
	var candidates []struct{ name, value string }
	switch selectedType(s) {
	case typeSMTP:
		candidates = []struct{ name, value string }{
			{EnvGmailClientID, s.GmailClientID},
			{EnvGmailClientSecret, s.GmailClientSecret},
			{EnvGmailRefreshToken, s.GmailRefreshToken},
			{EnvGmailSender, s.GmailSender},
		}
	case typeAPI:
		candidates = []struct{ name, value string }{
			{EnvSMTPHost, s.SMTPHost},
			{EnvSMTPPort, s.SMTPPort},
			{EnvSMTPUsername, s.SMTPUsername},
			{EnvSMTPAppPassword, s.SMTPAppPassword},
		}
	}

	var names []string
	for _, c := range candidates {
		if c.value != "" {
			names = append(names, c.name)
		}
	}
	return names
}

// selectedType normalizes MAIL_TYPE: empty means SMTP, valid values are
// upper-cased, and anything else is returned as-is for New to reject.
func selectedType(s Settings) string {
	v := strings.ToUpper(strings.TrimSpace(s.MailType))
	if v == "" {
		return typeSMTP
	}
	return v
}
