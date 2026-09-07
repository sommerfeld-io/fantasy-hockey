package mailer

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

// unreachableAddr returns a local address nothing is listening on, by
// opening then immediately closing a listener.
func unreachableAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a local port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}
	return addr
}

func TestSendShouldReturnErrorWhenContextIsAlreadyCancelled(t *testing.T) {
	m := newWithAddr("user@example.com", "app-password", unreachableAddr(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Send(ctx, "to@example.com", "subject", "body")

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestSendShouldWrapConnectionErrorsWhenServerUnreachable(t *testing.T) {
	m := newWithAddr("user@example.com", "app-password", unreachableAddr(t))

	err := m.Send(context.Background(), "to@example.com", "subject", "body")

	if err == nil {
		t.Fatal("expected an error when the SMTP server is unreachable")
	}
	if !strings.Contains(err.Error(), "mailer: send email") {
		t.Errorf("expected the error to be wrapped with context, got %q", err.Error())
	}
}

func TestBuildMessageShouldIncludeAllHeaderFieldsAndBody(t *testing.T) {
	msg := string(buildMessage("from@example.com", "to@example.com", "Your code", "123456"))

	for _, want := range []string{"From: from@example.com", "To: to@example.com", "Subject: Your code", "123456"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected message to contain %q, got %q", want, msg)
		}
	}
}

func TestBuildMessageShouldNotIncludeAnUnrelatedSubject(t *testing.T) {
	msg := string(buildMessage("from@example.com", "to@example.com", "Your code", "123456"))

	if strings.Contains(msg, "Unrelated subject") {
		t.Errorf("expected message not to contain an unrelated subject, got %q", msg)
	}
}

func TestNewShouldTargetTheGmailSMTPHostAndPort(t *testing.T) {
	m := New("user@example.com", "app-password")

	if m.addr != "smtp.gmail.com:587" {
		t.Errorf("expected New to target smtp.gmail.com:587, got %q", m.addr)
	}
}
