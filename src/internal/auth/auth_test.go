package auth_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	yaml "go.yaml.in/yaml/v3"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// captureLogs swaps slog's default logger for one writing to a buffer this
// test can inspect, restoring the original default when the test ends.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

// persistedDocument mirrors internal/store's on-disk shape closely enough
// for tests to assert on what actually landed on disk, without store
// exposing a login-code reader of its own (AD-29 - store is method-only,
// and reading back login codes isn't a need any real caller has).
type persistedDocument struct {
	Season  string `yaml:"season"`
	Players []struct {
		ID    string `yaml:"id"`
		Name  string `yaml:"name"`
		Email string `yaml:"email"`
	} `yaml:"players"`
	LoginCodes []struct {
		ID       string  `yaml:"id"`
		PlayerID string  `yaml:"player_id"`
		CodeHash string  `yaml:"code_hash"`
		IssuedAt string  `yaml:"issued_at"`
		UsedAt   *string `yaml:"used_at"`
	} `yaml:"login_codes"`
}

func readPersisted(t *testing.T, path string) persistedDocument {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc persistedDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return doc
}

// newSeededStore writes a data file seeded with one player and loads it
// through store.New, returning the store and the file's path.
func newSeededStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fantasy-hockey.yml")
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
`
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return st, path
}

// fakeSender records every call and always succeeds. It is safe for
// concurrent use since RequestLoginCode invokes send from its own goroutine.
type fakeSender struct {
	mu    sync.Mutex
	calls []struct{ to, subject, body string }
}

func (f *fakeSender) send(to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, struct{ to, subject, body string }{to, subject, body})
	return nil
}

// callCount returns how many calls have been recorded so far, safe for
// concurrent use.
func (f *fakeSender) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// call returns a copy of the i-th recorded call, safe for concurrent use.
func (f *fakeSender) call(i int) struct{ to, subject, body string } {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[i]
}

// waitForSendCalls polls fake until it has recorded n calls, since
// RequestLoginCode sends asynchronously (in its own goroutine) so a match
// and a no-match return equally fast.
func waitForSendCalls(t *testing.T, fake *fakeSender, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fake.callCount() == n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d send call(s), got %d", n, fake.callCount())
}

func TestRequestLoginCodeShouldPersistAndEmailACodeOnAMatch(t *testing.T) {
	st, path := newSeededStore(t)
	fake := &fakeSender{}

	if err := auth.RequestLoginCode(st, fake.send, "basti@example.com"); err != nil {
		t.Fatalf("RequestLoginCode returned error: %v", err)
	}

	waitForSendCalls(t, fake, 1)
	call := fake.call(0)
	if call.to != "basti@example.com" {
		t.Errorf("expected the email to go to %q, got %q", "basti@example.com", call.to)
	}

	doc := readPersisted(t, path)
	if len(doc.LoginCodes) != 1 {
		t.Fatalf("expected 1 persisted login code, got %d", len(doc.LoginCodes))
	}
	row := doc.LoginCodes[0]
	if row.PlayerID != "basti" {
		t.Errorf("expected player_id %q, got %q", "basti", row.PlayerID)
	}
	if row.UsedAt != nil {
		t.Errorf("expected used_at to be nil, got %v", row.UsedAt)
	}
	if row.IssuedAt == "" {
		t.Error("expected issued_at to be set")
	}

	if !codeHashMatchesBody(row.CodeHash, call.body) {
		t.Errorf("expected code_hash %q to match the emailed code in body %q", row.CodeHash, call.body)
	}
}

// codeHashMatchesBody extracts the 6-digit code from the email body sent by
// auth.RequestLoginCode and checks it hashes to hash.
func codeHashMatchesBody(hash, body string) bool {
	const codeLength = 6
	for i := 0; i+codeLength <= len(body); i++ {
		candidate := body[i : i+codeLength]
		allDigits := true
		for _, r := range candidate {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if !allDigits {
			continue
		}
		sum := sha256.Sum256([]byte(candidate))
		if hex.EncodeToString(sum[:]) == hash {
			return true
		}
	}
	return false
}

func TestRequestLoginCodeShouldDoNothingOnNoMatch(t *testing.T) {
	st, path := newSeededStore(t)
	fake := &fakeSender{}

	if err := auth.RequestLoginCode(st, fake.send, "unknown@example.com"); err != nil {
		t.Fatalf("RequestLoginCode returned error: %v", err)
	}

	if fake.callCount() != 0 {
		t.Fatalf("expected the sender never to be invoked, got %d calls", fake.callCount())
	}

	doc := readPersisted(t, path)
	if len(doc.LoginCodes) != 0 {
		t.Fatalf("expected no persisted login code, got %d", len(doc.LoginCodes))
	}
}

func TestRequestLoginCodeShouldLogWhenACodeIsSent(t *testing.T) {
	st, _ := newSeededStore(t)
	fake := &fakeSender{}
	logs := captureLogs(t)

	if err := auth.RequestLoginCode(st, fake.send, "basti@example.com"); err != nil {
		t.Fatalf("RequestLoginCode returned error: %v", err)
	}

	waitForSendCalls(t, fake, 1)
	waitForLogContaining(t, logs, "send login code")
	if !strings.Contains(logs.String(), "player_id=basti") {
		t.Errorf("expected the log line to include player_id=basti, got %q", logs.String())
	}
}

func TestRequestLoginCodeShouldNotLogOnNoMatch(t *testing.T) {
	st, _ := newSeededStore(t)
	fake := &fakeSender{}
	logs := captureLogs(t)

	if err := auth.RequestLoginCode(st, fake.send, "unknown@example.com"); err != nil {
		t.Fatalf("RequestLoginCode returned error: %v", err)
	}

	if logs.Len() != 0 {
		t.Errorf("expected no log output on a no-match request, got %q", logs.String())
	}
}

// waitForLogContaining polls buf until it contains substr, since
// RequestLoginCode logs from its own goroutine after sending.
func waitForLogContaining(t *testing.T, buf *bytes.Buffer, substr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), substr) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for log output containing %q, got %q", substr, buf.String())
}

func TestRequestLoginCodeShouldLeaveAnEarlierRowUntouchedOnARepeatRequest(t *testing.T) {
	st, path := newSeededStore(t)
	fake := &fakeSender{}

	if err := auth.RequestLoginCode(st, fake.send, "basti@example.com"); err != nil {
		t.Fatalf("first RequestLoginCode returned error: %v", err)
	}
	first := readPersisted(t, path).LoginCodes[0]

	if err := auth.RequestLoginCode(st, fake.send, "basti@example.com"); err != nil {
		t.Fatalf("second RequestLoginCode returned error: %v", err)
	}

	doc := readPersisted(t, path)
	if len(doc.LoginCodes) != 2 {
		t.Fatalf("expected 2 persisted login codes, got %d", len(doc.LoginCodes))
	}
	if !loginCodeRowsEqual(doc.LoginCodes[0], first) {
		t.Errorf("expected the first row to stay untouched, got %+v, was %+v", doc.LoginCodes[0], first)
	}
}

// loginCodeRowsEqual compares two persisted login-code rows field by field.
// A plain struct == would compare UsedAt (a *string) by pointer identity,
// not value, which two independent yaml.Unmarshal calls would never share
// even for equal content.
func loginCodeRowsEqual(a, b struct {
	ID       string  `yaml:"id"`
	PlayerID string  `yaml:"player_id"`
	CodeHash string  `yaml:"code_hash"`
	IssuedAt string  `yaml:"issued_at"`
	UsedAt   *string `yaml:"used_at"`
}) bool {
	if a.ID != b.ID || a.PlayerID != b.PlayerID || a.CodeHash != b.CodeHash || a.IssuedAt != b.IssuedAt {
		return false
	}
	switch {
	case a.UsedAt == nil && b.UsedAt == nil:
		return true
	case a.UsedAt == nil || b.UsedAt == nil:
		return false
	default:
		return *a.UsedAt == *b.UsedAt
	}
}

func TestRequestLoginCodeShouldReturnNilWhenSendingFails(t *testing.T) {
	st, path := newSeededStore(t)
	sendErr := errors.New("smtp: connection refused")
	failingSend := func(_, _, _ string) error { return sendErr }

	if err := auth.RequestLoginCode(st, failingSend, "basti@example.com"); err != nil {
		t.Fatalf("expected RequestLoginCode to swallow the send error, got %v", err)
	}

	doc := readPersisted(t, path)
	if len(doc.LoginCodes) != 1 {
		t.Fatalf("expected the login code to still be persisted, got %d rows", len(doc.LoginCodes))
	}
}

func TestRequestLoginCodeShouldReturnAnErrorWhenTheStoreWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fantasy-hockey.yml")
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
`
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	// Remove the directory out from under the store so the atomic
	// write-and-rename that CreateLoginCode performs fails, simulating a
	// disk write failure. Unlike a read-only permission bit, this fails
	// even when the test process runs as root (e.g. inside a container
	// build), which otherwise bypasses ordinary permission checks.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	fake := &fakeSender{}
	if err := auth.RequestLoginCode(st, fake.send, "basti@example.com"); err == nil {
		t.Fatal("expected an error when the store write fails, got nil")
	}
	if fake.callCount() != 0 {
		t.Errorf("expected the sender not to be invoked when the store write fails, got %d calls", fake.callCount())
	}
}

func TestRequestLoginCodeShouldDoNothingForAMalformedOrEmptyEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"missing @", "not-an-email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, path := newSeededStore(t)
			fake := &fakeSender{}

			if err := auth.RequestLoginCode(st, fake.send, tt.email); err != nil {
				t.Fatalf("RequestLoginCode returned error: %v", err)
			}

			if fake.callCount() != 0 {
				t.Fatalf("expected the sender never to be invoked, got %d calls", fake.callCount())
			}

			doc := readPersisted(t, path)
			if len(doc.LoginCodes) != 0 {
				t.Fatalf("expected no persisted login code, got %d", len(doc.LoginCodes))
			}
		})
	}
}
