package mailer

import (
	"errors"
	"net/smtp"
	"strings"
	"testing"
)

// capturedCall records the arguments a fake sendMailFunc was invoked with.
type capturedCall struct {
	addr    string
	auth    smtp.Auth
	from    string
	to      []string
	message []byte
}

func TestNewSMTPSenderShouldErrorWhenHostIsUnset(t *testing.T) {
	var called bool
	fake := func(_ string, _ smtp.Auth, _ string, _ []string, _ []byte) error {
		called = true
		return nil
	}

	send := newSMTPSender("", "587", "", "", fake)
	err := send("player@example.com", "subject", "body")

	if err == nil {
		t.Fatal("expected an error when SMTP_HOST is unset, got nil")
	}
	if called {
		t.Error("expected the underlying send function not to be called when SMTP_HOST is unset")
	}
}

func TestNewSMTPSenderShouldErrorWhenPortIsUnset(t *testing.T) {
	var called bool
	fake := func(_ string, _ smtp.Auth, _ string, _ []string, _ []byte) error {
		called = true
		return nil
	}

	send := newSMTPSender("smtp.example.com", "", "", "", fake)
	err := send("player@example.com", "subject", "body")

	if err == nil {
		t.Fatal("expected an error when SMTP_PORT is unset, got nil")
	}
	if called {
		t.Error("expected the underlying send function not to be called when SMTP_PORT is unset")
	}
}

func TestNewSMTPSenderShouldSkipAuthWhenUsernameIsEmpty(t *testing.T) {
	var got capturedCall
	fake := func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		got = capturedCall{addr: addr, auth: auth, from: from, to: to, message: msg}
		return nil
	}

	send := newSMTPSender("smtp.example.com", "1025", "", "", fake)
	if err := send("player@example.com", "subject", "body"); err != nil {
		t.Fatalf("send returned error: %v", err)
	}

	if got.auth != nil {
		t.Errorf("expected no auth when username is empty, got %v", got.auth)
	}
	if got.addr != "smtp.example.com:1025" {
		t.Errorf("expected addr %q, got %q", "smtp.example.com:1025", got.addr)
	}
	if len(got.to) != 1 || got.to[0] != "player@example.com" {
		t.Errorf("expected to=[player@example.com], got %v", got.to)
	}
	if got.from != defaultFrom {
		t.Errorf("expected from %q, got %q", defaultFrom, got.from)
	}
}

func TestNewSMTPSenderShouldAuthenticateWhenUsernameIsSet(t *testing.T) {
	var got capturedCall
	fake := func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		got = capturedCall{addr: addr, auth: auth, from: from, to: to, message: msg}
		return nil
	}

	send := newSMTPSender("smtp.example.com", "587", "user@gmail.com", "app-password", fake)
	if err := send("player@example.com", "subject", "body"); err != nil {
		t.Fatalf("send returned error: %v", err)
	}

	if got.auth == nil {
		t.Error("expected AUTH to be used when username is set, got none")
	}
	if got.from != "user@gmail.com" {
		t.Errorf("expected from %q, got %q", "user@gmail.com", got.from)
	}
}

func TestNewSMTPSenderShouldWrapAnUnderlyingSendError(t *testing.T) {
	sendErr := errors.New("connection refused")
	fake := func(_ string, _ smtp.Auth, _ string, _ []string, _ []byte) error {
		return sendErr
	}

	send := newSMTPSender("smtp.example.com", "587", "", "", fake)
	err := send("player@example.com", "subject", "body")

	if err == nil || !errors.Is(err, sendErr) {
		t.Fatalf("expected the error to wrap %v, got %v", sendErr, err)
	}
}

func TestNewSMTPSenderShouldIncludeSubjectAndBodyInTheMessage(t *testing.T) {
	var got capturedCall
	fake := func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		got = capturedCall{addr: addr, auth: auth, from: from, to: to, message: msg}
		return nil
	}

	send := newSMTPSender("smtp.example.com", "1025", "", "", fake)
	if err := send("player@example.com", "Your login code", "Your login code is 123456."); err != nil {
		t.Fatalf("send returned error: %v", err)
	}

	body := string(got.message)
	if !strings.Contains(body, "Your login code") || !strings.Contains(body, "Your login code is 123456.") {
		t.Errorf("expected the message to contain the subject and body, got %q", body)
	}
}
