package mailer

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// setupWithEnv runs Setup on settings read from env and returns the sender,
// everything Setup logged and the error.
func setupWithEnv(env map[string]string) (Sender, string, error) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	send, err := Setup(SettingsFromEnv(func(k string) string { return env[k] }), logger)
	return send, logs.String(), err
}

func TestSetupShouldWarnOnceNamingIgnoredVariablesButNotTheirValues(t *testing.T) {
	env := map[string]string{EnvMailType: "API", EnvSMTPHost: "secret-host.example.com", EnvSMTPPort: "25",
		EnvGmailClientID: "a", EnvGmailClientSecret: "b", EnvGmailRefreshToken: "c", EnvGmailSender: "d"}

	_, out, err := setupWithEnv(env)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Count(out, "level=WARN"); got != 1 {
		t.Fatalf("expected exactly one warning, got %d: %s", got, out)
	}
	if !strings.Contains(out, WarnIgnoredSettings) {
		t.Errorf("expected the ignored-settings message, got %s", out)
	}
	if !strings.Contains(out, EnvSMTPHost) || !strings.Contains(out, EnvSMTPPort) {
		t.Errorf("expected the warning to name the ignored variables, got %s", out)
	}
	if strings.Contains(out, "secret-host.example.com") {
		t.Errorf("expected no value in the log, got %s", out)
	}
}

func TestSetupShouldNotWarnWhenNothingIsIgnoredOrMissing(t *testing.T) {
	_, out, err := setupWithEnv(map[string]string{EnvSMTPHost: "h", EnvSMTPPort: "25"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected no log output, got %s", out)
	}
}

func TestSetupShouldReturnAnErrorForAnInvalidMailTypeAndLogNothing(t *testing.T) {
	send, out, err := setupWithEnv(map[string]string{EnvMailType: "pigeon", EnvSMTPHost: "h"})

	if err == nil || !strings.Contains(err.Error(), EnvMailType) {
		t.Fatalf("expected an error naming MAIL_TYPE, got %v", err)
	}
	if send != nil || out != "" {
		t.Errorf("expected no sender and no log output, got sender %v and logs %q", send != nil, out)
	}
}

func TestSetupShouldWarnOnceNamingMissingGmailVariablesButNotValues(t *testing.T) {
	env := map[string]string{EnvMailType: "API", EnvGmailClientSecret: "very-secret-value",
		EnvGmailSender: "sender@example.com", EnvGmailRefreshToken: "tok-value"}

	send, out, err := setupWithEnv(env)

	if err != nil || send == nil {
		t.Fatalf("expected the app to start, got %v", err)
	}
	if got := strings.Count(out, "level=WARN"); got != 1 {
		t.Fatalf("expected exactly one warning, got %d: %s", got, out)
	}
	if !strings.Contains(out, WarnMissingSettings) || !strings.Contains(out, EnvGmailClientID) {
		t.Errorf("expected the missing-settings message naming GMAIL_CLIENT_ID, got %s", out)
	}
	for _, set := range []string{EnvGmailClientSecret, EnvGmailRefreshToken, EnvGmailSender} {
		if strings.Contains(out, set) {
			t.Errorf("expected the warning not to name the set variable %s, got %s", set, out)
		}
	}
	for _, leaked := range []string{"very-secret-value", "sender@example.com", "tok-value"} {
		if strings.Contains(out, leaked) {
			t.Errorf("expected no value %q in the log, got %s", leaked, out)
		}
	}
}

func TestSetupShouldNotWarnAboutMissingGmailVariablesWhenSMTPIsSelected(t *testing.T) {
	for _, mailType := range []string{"", "SMTP"} {
		_, out, err := setupWithEnv(map[string]string{EnvMailType: mailType, EnvSMTPHost: "h", EnvSMTPPort: "25"})

		if err != nil || out != "" {
			t.Fatalf("MAIL_TYPE %q: err %v, logs %q", mailType, err, out)
		}
	}
}

func TestSetupShouldWarnOnceNamingMissingSMTPHostAndPortWhenNothingIsSet(t *testing.T) {
	send, out, err := setupWithEnv(map[string]string{})

	if err != nil || send == nil {
		t.Fatalf("expected the app to start, got %v", err)
	}
	if got := strings.Count(out, "level=WARN"); got != 1 {
		t.Fatalf("expected exactly one warning, got %d: %s", got, out)
	}
	if !strings.Contains(out, EnvSMTPHost) || !strings.Contains(out, EnvSMTPPort) {
		t.Errorf("expected the warning to name SMTP_HOST and SMTP_PORT, got %s", out)
	}
	for _, name := range []string{EnvSMTPUsername, EnvSMTPAppPassword, "GMAIL_"} {
		if strings.Contains(out, name) {
			t.Errorf("expected the warning not to name %s, got %s", name, out)
		}
	}
}

func TestSetupShouldNameOnlyTheMissingSMTPVariable(t *testing.T) {
	_, out, err := setupWithEnv(map[string]string{EnvSMTPPort: "1025"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, EnvSMTPHost) || strings.Contains(out, EnvSMTPPort) || strings.Contains(out, "1025") {
		t.Errorf("expected the warning to name only SMTP_HOST, got %s", out)
	}
}

func TestSetupShouldNotWarnWhenAllGmailVariablesAreSet(t *testing.T) {
	env := map[string]string{EnvMailType: "API", EnvGmailClientID: "a", EnvGmailClientSecret: "b",
		EnvGmailRefreshToken: "c", EnvGmailSender: "d"}

	if _, out, err := setupWithEnv(env); err != nil || out != "" {
		t.Fatalf("err %v, logs %q", err, out)
	}
}

// TestSetupWarningMessagesShouldKeepTheirWording pins the text a person
// running the pool searches the log for; the acceptance steps reuse the
// constants, so only this test notices a rewording.
func TestSetupWarningMessagesShouldKeepTheirWording(t *testing.T) {
	if WarnIgnoredSettings != "ignoring mail settings of the unselected transport" {
		t.Errorf("WarnIgnoredSettings changed: %q", WarnIgnoredSettings)
	}
	if WarnMissingSettings != "Mail settings of the selected transport are missing, so login codes will not be sent until they are set" {
		t.Errorf("WarnMissingSettings changed: %q", WarnMissingSettings)
	}
}

func TestSetupShouldLogBothWarningsOnceWhenSettingsAreIgnoredAndMissing(t *testing.T) {
	env := map[string]string{EnvMailType: "API", EnvSMTPHost: "h", EnvGmailClientID: "a"}

	send, out, err := setupWithEnv(env)

	if err != nil || send == nil {
		t.Fatalf("expected the app to start, got %v", err)
	}
	for _, msg := range []string{WarnIgnoredSettings, WarnMissingSettings} {
		if got := strings.Count(out, msg); got != 1 {
			t.Errorf("expected %q exactly once, got %d: %s", msg, got, out)
		}
	}
	if got := strings.Count(out, "level=WARN"); got != 2 {
		t.Errorf("expected exactly two warnings, got %d: %s", got, out)
	}
}
