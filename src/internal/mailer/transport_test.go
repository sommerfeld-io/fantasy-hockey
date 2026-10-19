package mailer

import (
	"reflect"
	"strings"
	"testing"
)

func TestSettingsFromEnvShouldReadEveryMailVariable(t *testing.T) {
	env := map[string]string{
		EnvMailType:          "API",
		EnvSMTPHost:          "smtp.example.com",
		EnvSMTPPort:          "587",
		EnvSMTPUsername:      "user",
		EnvSMTPAppPassword:   "pw",
		EnvGmailClientID:     "cid",
		EnvGmailClientSecret: "csecret",
		EnvGmailRefreshToken: "rtoken",
		EnvGmailSender:       "sender@example.com",
	}

	got := SettingsFromEnv(func(k string) string { return env[k] })

	want := Settings{
		MailType: "API", SMTPHost: "smtp.example.com", SMTPPort: "587", SMTPUsername: "user", SMTPAppPassword: "pw",
		GmailClientID: "cid", GmailClientSecret: "csecret", GmailRefreshToken: "rtoken", GmailSender: "sender@example.com",
	}
	if got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestNewShouldSelectSMTPWhenMailTypeIsUnsetOrEmptyOrAnySpelling(t *testing.T) {
	for _, mailType := range []string{"", "   ", "smtp", "SMTP", "Smtp", " smtp "} {
		t.Run("["+mailType+"]", func(t *testing.T) {
			send, err := New(Settings{MailType: mailType})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			sendErr := send("p@example.com", "s", "b")

			if sendErr == nil || !strings.Contains(sendErr.Error(), EnvSMTPHost) {
				t.Fatalf("expected the SMTP sender's missing-host error, got %v", sendErr)
			}
		})
	}
}

func TestNewShouldSelectAPIWithoutTouchingSMTP(t *testing.T) {
	for _, mailType := range []string{"api", "API", "Api"} {
		t.Run(mailType, func(t *testing.T) {
			send, err := New(Settings{MailType: mailType, SMTPHost: "127.0.0.1", SMTPPort: "1"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			sendErr := send("p@example.com", "s", "b")

			if sendErr == nil || !strings.Contains(sendErr.Error(), EnvGmailClientID) {
				t.Fatalf("expected a missing-%s error, got %v", EnvGmailClientID, sendErr)
			}
			if strings.Contains(sendErr.Error(), "send mail") {
				t.Fatalf("API sender must not use SMTP, got %v", sendErr)
			}
		})
	}
}

func TestNewShouldRejectAnInvalidMailTypeWithoutFallingBack(t *testing.T) {
	send, err := New(Settings{MailType: "pigeon", SMTPHost: "h", SMTPPort: "25"})

	if err == nil {
		t.Fatal("expected an error for an invalid MAIL_TYPE")
	}
	for _, want := range []string{EnvMailType, "SMTP", "API"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected the error to name %q, got %q", want, err.Error())
		}
	}
	if send != nil {
		t.Error("expected no sender on an invalid MAIL_TYPE")
	}
}

func TestIgnoredVarsShouldListSMTPVariablesWhenAPIIsSelected(t *testing.T) {
	got := IgnoredVars(Settings{MailType: "api", SMTPHost: "h", SMTPAppPassword: "secret", GmailSender: "x"})

	want := []string{EnvSMTPHost, EnvSMTPAppPassword}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestIgnoredVarsShouldListGmailVariablesWhenSMTPIsSelected(t *testing.T) {
	for _, mailType := range []string{"", "SMTP"} {
		got := IgnoredVars(Settings{MailType: mailType, SMTPHost: "h", GmailClientID: "a", GmailRefreshToken: "b"})

		want := []string{EnvGmailClientID, EnvGmailRefreshToken}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("MAIL_TYPE %q: expected %v, got %v", mailType, want, got)
		}
	}
}

func TestIgnoredVarsShouldBeEmptyWhenNothingIsIgnoredOrTypeIsInvalid(t *testing.T) {
	cases := map[string]Settings{
		"nothing set":         {},
		"only selected set":   {MailType: "api", GmailClientID: "a"},
		"invalid MAIL_TYPE":   {MailType: "pigeon", SMTPHost: "h", GmailSender: "g"},
		"smtp with smtp only": {SMTPHost: "h", SMTPPort: "25"},
	}
	for name, s := range cases {
		if got := IgnoredVars(s); len(got) != 0 {
			t.Errorf("%s: expected none, got %v", name, got)
		}
	}
}

func TestNewShouldSelectAPIWhenMailTypeHasSurroundingWhitespace(t *testing.T) {
	send, err := New(Settings{MailType: " api "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sendErr := send("p@example.com", "s", "b")

	if sendErr == nil || strings.Contains(sendErr.Error(), EnvSMTPHost) {
		t.Fatalf("expected the API sender, got %v", sendErr)
	}
}
