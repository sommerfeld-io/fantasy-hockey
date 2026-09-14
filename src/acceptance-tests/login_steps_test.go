package acceptance_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	yaml "go.yaml.in/yaml/v3"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// sendWaitTimeout bounds how long a step waits for internal/auth's
// send-in-its-own-goroutine to land, since RequestLoginCode returns before
// that goroutine necessarily runs (matching auth_test.go's waitForSendCalls).
const sendWaitTimeout = 2 * time.Second

// loginDocument mirrors internal/store's on-disk shape closely enough for
// scenarios to assert on what actually landed in fantasy-hockey.yml.
type loginDocument struct {
	Season  string `yaml:"season"`
	Players []struct {
		ID    string `yaml:"id"`
		Name  string `yaml:"name"`
		Email string `yaml:"email"`
	} `yaml:"players"`
	LoginCodes []loginCodeRow `yaml:"login_codes"`
}

type loginCodeRow struct {
	ID       string  `yaml:"id"`
	PlayerID string  `yaml:"player_id"`
	CodeHash string  `yaml:"code_hash"`
	IssuedAt string  `yaml:"issued_at"`
	UsedAt   *string `yaml:"used_at"`
}

// loginResponse is one recorded POST /login result.
type loginResponse struct {
	status int
	body   string
}

// loginScenarioState holds the fixtures and results for one request-a-
// login-code scenario. A fresh instance is created per scenario so state
// never leaks between runs.
//
// mu guards every field the fake mailer.Sender writes, since
// internal/auth.RequestLoginCode invokes send from its own goroutine
// (asynchronously, so a match and a no-match return equally fast) - both
// that goroutine and a step definition's assertion can touch sentTo,
// sentCodes, and logs concurrently.
type loginScenarioState struct {
	dataFile     string
	server       *httptest.Server
	sendFails    bool
	mu           sync.Mutex
	sentTo       []string
	sentCodes    []string
	logs         *bytes.Buffer
	responses    []loginResponse
	firstCodeRow *loginCodeRow // snapshot of doc.LoginCodes[0] right after the first request
}

func newLoginScenarioState() *loginScenarioState {
	dir, err := os.MkdirTemp("", "fantasy-hockey-login-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	return &loginScenarioState{
		dataFile: filepath.Join(dir, "fantasy-hockey.yml"),
		logs:     &bytes.Buffer{},
	}
}

func (s *loginScenarioState) close() {
	if s.server != nil {
		s.server.Close()
	}
}

// aPlayerWithEmailIsRegistered seeds the data file with one hand-maintained
// player before the store is ever opened, so store.New loads it rather than
// bootstrapping an empty file.
func (s *loginScenarioState) aPlayerWithEmailIsRegistered(name, email string) error {
	seed := fmt.Sprintf("season: \"2026-27\"\nplayers:\n    - id: %s\n      name: %s\n      email: %s\n",
		strings.ToLower(name), name, email)
	if err := os.WriteFile(s.dataFile, []byte(seed), 0o600); err != nil {
		return fmt.Errorf("seed data file: %w", err)
	}
	return nil
}

func (s *loginScenarioState) sendingEmailIsConfiguredToFail() error {
	s.sendFails = true
	return nil
}

// startServer lazily builds the real production web.NewServer handler
// around a store.Store loaded from s.dataFile and a mailer.Sender under
// this scenario's control, so every step exercises the actual wiring.
func (s *loginScenarioState) startServer() error {
	if s.server != nil {
		return nil
	}

	st, err := store.New(s.dataFile)
	if err != nil {
		return fmt.Errorf("store.New: %w", err)
	}

	send := func(to, _, body string) error {
		if s.sendFails {
			return fmt.Errorf("smtp: connection refused")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.sentTo = append(s.sentTo, to)
		s.sentCodes = append(s.sentCodes, extractSixDigitCode(body))
		return nil
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(&syncWriter{mu: &s.mu, w: s.logs}, nil)))
	s.server = httptest.NewServer(web.NewServer(st, send))
	return nil
}

// syncWriter guards writes to w with mu, since the goroutine
// internal/auth.RequestLoginCode sends from can log an error concurrently
// with a step definition reading s.logs.
type syncWriter struct {
	mu *sync.Mutex
	w  *bytes.Buffer
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

// waitUntil polls cond (evaluated under s.mu) until it returns true or
// sendWaitTimeout elapses, since the fake mailer.Sender's side effects land
// asynchronously. Returns an error naming what never became true.
func (s *loginScenarioState) waitUntil(what string, cond func() bool) error {
	deadline := time.Now().Add(sendWaitTimeout)
	for {
		s.mu.Lock()
		ok := cond()
		s.mu.Unlock()
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// extractSixDigitCode pulls the emailed login code out of the mail body
// produced by internal/auth's body template.
func extractSixDigitCode(body string) string {
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
		if allDigits {
			return candidate
		}
	}
	return ""
}

func (s *loginScenarioState) aVisitorRequestsALoginCodeFor(email string) error {
	if err := s.startServer(); err != nil {
		return err
	}

	resp, err := http.PostForm(s.server.URL+"/login", url.Values{"email": {email}})
	if err != nil {
		return fmt.Errorf("post /login: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	s.responses = append(s.responses, loginResponse{status: resp.StatusCode, body: string(body)})

	if len(s.responses) == 1 {
		doc, err := s.readDoc()
		if err != nil {
			return err
		}
		if len(doc.LoginCodes) > 0 {
			row := doc.LoginCodes[0]
			s.firstCodeRow = &row
		}
	}
	return nil
}

func (s *loginScenarioState) readDoc() (loginDocument, error) {
	raw, err := os.ReadFile(s.dataFile)
	if err != nil {
		return loginDocument{}, fmt.Errorf("read data file: %w", err)
	}
	var doc loginDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return loginDocument{}, fmt.Errorf("unmarshal data file: %w", err)
	}
	return doc, nil
}

func (s *loginScenarioState) lastResponse() (loginResponse, error) {
	if len(s.responses) == 0 {
		return loginResponse{}, fmt.Errorf("no requests have been made yet")
	}
	return s.responses[len(s.responses)-1], nil
}

func (s *loginScenarioState) theResponseStatusIs(want int) error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	if got.status != want {
		return fmt.Errorf("expected status %d, got %d", want, got.status)
	}
	return nil
}

func (s *loginScenarioState) aLoginCodeIsPersistedWhoseHashMatchesTheEmailedCode() error {
	doc, err := s.readDoc()
	if err != nil {
		return err
	}
	if len(doc.LoginCodes) != 1 {
		return fmt.Errorf("expected 1 persisted login code, got %d", len(doc.LoginCodes))
	}
	if doc.LoginCodes[0].UsedAt != nil {
		return fmt.Errorf("expected used_at to be nil, got %v", doc.LoginCodes[0].UsedAt)
	}
	if doc.LoginCodes[0].IssuedAt == "" {
		return fmt.Errorf("expected issued_at to be set")
	}
	if err := s.waitUntil("an emailed code to be captured", func() bool { return len(s.sentCodes) == 1 && s.sentCodes[0] != "" }); err != nil {
		return err
	}

	s.mu.Lock()
	sentCode := s.sentCodes[0]
	s.mu.Unlock()

	sum := sha256.Sum256([]byte(sentCode))
	want := hex.EncodeToString(sum[:])
	if doc.LoginCodes[0].CodeHash != want {
		return fmt.Errorf("expected the persisted hash %q to match the emailed code's hash %q", doc.LoginCodes[0].CodeHash, want)
	}
	return nil
}

func (s *loginScenarioState) theEmailIsSentTo(email string) error {
	err := s.waitUntil(fmt.Sprintf("an email to %q", email), func() bool {
		for _, to := range s.sentTo {
			if to == email {
				return true
			}
		}
		return false
	})
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return fmt.Errorf("expected an email to %q, got calls to %v", email, s.sentTo)
	}
	return nil
}

func (s *loginScenarioState) theTwoResponsesAreIdentical() error {
	if len(s.responses) != 2 {
		return fmt.Errorf("expected 2 recorded responses, got %d", len(s.responses))
	}
	a, b := s.responses[0], s.responses[1]
	if a.status != b.status {
		return fmt.Errorf("expected identical status codes, got %d and %d", a.status, b.status)
	}
	if a.body != b.body {
		return fmt.Errorf("expected identical response bodies, got %q and %q", a.body, b.body)
	}
	return nil
}

func (s *loginScenarioState) onlyNEmailsWereSentInTotal(n int) error {
	// A non-matching request never calls send, so there's nothing async to
	// wait for on that side; only wait when more calls could still land.
	_ = s.waitUntil(fmt.Sprintf("%d total email(s) sent", n), func() bool { return len(s.sentTo) >= n })

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sentTo) != n {
		return fmt.Errorf("expected %d email(s) sent in total, got %d: %v", n, len(s.sentTo), s.sentTo)
	}
	return nil
}

func (s *loginScenarioState) nLoginCodesArePersisted(n int) error {
	doc, err := s.readDoc()
	if err != nil {
		return err
	}
	if len(doc.LoginCodes) != n {
		return fmt.Errorf("expected %d persisted login codes, got %d", n, len(doc.LoginCodes))
	}
	return nil
}

func (s *loginScenarioState) theFirstLoginCodeIsUnchanged() error {
	doc, err := s.readDoc()
	if err != nil {
		return err
	}
	if len(doc.LoginCodes) == 0 {
		return fmt.Errorf("expected at least 1 persisted login code, got none")
	}
	if s.firstCodeRow == nil {
		return fmt.Errorf("no snapshot of the first login code was captured")
	}
	if doc.LoginCodes[0] != *s.firstCodeRow {
		return fmt.Errorf("expected the first login code to stay untouched, got %+v, was %+v", doc.LoginCodes[0], *s.firstCodeRow)
	}
	return nil
}

func (s *loginScenarioState) anErrorWasLogged() error {
	err := s.waitUntil("an error to be logged", func() bool { return strings.Contains(s.logs.String(), "ERROR") })
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return fmt.Errorf("expected an error to be logged, got logs: %q", s.logs.String())
	}
	return nil
}

// InitializeLoginScenario registers the request-a-login-code step
// definitions with GoDog.
func InitializeLoginScenario(ctx *godog.ScenarioContext) {
	s := newLoginScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a player "([^"]*)" with email "([^"]*)" is registered$`, s.aPlayerWithEmailIsRegistered)
	ctx.Step(`^sending email is configured to fail$`, s.sendingEmailIsConfiguredToFail)
	ctx.Step(`^a visitor requests a login code for "([^"]*)"$`, s.aVisitorRequestsALoginCodeFor)
	ctx.Step(`^the response status is (\d+)$`, s.theResponseStatusIs)
	ctx.Step(`^a login code is persisted whose hash matches the emailed code$`, s.aLoginCodeIsPersistedWhoseHashMatchesTheEmailedCode)
	ctx.Step(`^the email is sent to "([^"]*)"$`, s.theEmailIsSentTo)
	ctx.Step(`^the two responses are identical$`, s.theTwoResponsesAreIdentical)
	ctx.Step(`^only (\d+) email was sent in total$`, s.onlyNEmailsWereSentInTotal)
	ctx.Step(`^(\d+) login codes are persisted$`, s.nLoginCodesArePersisted)
	ctx.Step(`^the first login code is unchanged$`, s.theFirstLoginCodeIsUnchanged)
	ctx.Step(`^an error was logged$`, s.anErrorWasLogged)
}
