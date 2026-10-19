package mailer

import (
	"fmt"
	"strings"
	"time"
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

	// GmailTokenURL and GmailSendURL are never read from the environment.
	// Empty means Google's real URLs; tests point them at httptest servers.
	GmailTokenURL string
	GmailSendURL  string
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

// New returns the Sender selected by s.MailType: SMTP (also when unset or
// empty) or API. Any other value is an error, with no fallback to SMTP.
func New(s Settings) (Sender, error) {
	switch selectedType(s) {
	case typeSMTP:
		return NewSMTPSender(s.SMTPHost, s.SMTPPort, s.SMTPUsername, s.SMTPAppPassword), nil
	case typeAPI:
		return newGmailSender(s, gmailTimeout, time.Now), nil
	default:
		return nil, fmt.Errorf("mailer: invalid %s %q: allowed values are %s and %s",
			EnvMailType, strings.TrimSpace(s.MailType), typeSMTP, typeAPI)
	}
}

// IgnoredVars names the set variables that belong to the unselected
// transport, in a stable order. It returns names only, never values, and
// nothing for an invalid MAIL_TYPE since startup fails there anyway.
func IgnoredVars(s Settings) []string {
	var candidates []namedValue
	switch selectedType(s) {
	case typeSMTP:
		candidates = gmailVars(s)
	case typeAPI:
		candidates = []namedValue{
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

// MissingVars names the unset required variables of the selected transport,
// in a stable order: SMTP_HOST and SMTP_PORT for SMTP (the username and
// password stay optional), the four GMAIL_* variables for API. It returns
// nothing for an invalid MAIL_TYPE. Names only, never values.
func MissingVars(s Settings) []string {
	var required []namedValue
	switch selectedType(s) {
	case typeSMTP:
		required = []namedValue{{EnvSMTPHost, s.SMTPHost}, {EnvSMTPPort, s.SMTPPort}}
	case typeAPI:
		required = gmailVars(s)
	}
	var names []string
	for _, v := range required {
		if v.value == "" {
			names = append(names, v.name)
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

// namedValue pairs a setting's environment variable name with its value.
type namedValue struct{ name, value string }

// gmailVars lists the four Gmail settings in the order that errors and
// warnings report them. It is the one list behind the send-time check and
// the startup warning, so the two cannot drift apart.
func gmailVars(s Settings) []namedValue {
	return []namedValue{
		{EnvGmailClientID, s.GmailClientID},
		{EnvGmailClientSecret, s.GmailClientSecret},
		{EnvGmailRefreshToken, s.GmailRefreshToken},
		{EnvGmailSender, s.GmailSender},
	}
}
