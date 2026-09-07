package acceptance_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// fakeLoginStore is an in-memory implementation of auth.Store. It lets the
// GoDog scenarios exercise the real auth/web code end-to-end without a
// PostgreSQL connection, since task go:build's Docker-stage tests have no
// network path to a sibling database container.
type fakeLoginStore struct {
	mu           sync.Mutex
	participants map[string]store.Participant
	codes        []store.LoginCode
}

func newFakeLoginStore() *fakeLoginStore {
	return &fakeLoginStore{participants: map[string]store.Participant{}}
}

func (f *fakeLoginStore) seed(name, email string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.participants[email] = store.Participant{ID: uuid.NewString(), Name: name, Email: email}
}

func (f *fakeLoginStore) ParticipantByEmail(_ context.Context, email string) (store.Participant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.participants[email]
	if !ok {
		return store.Participant{}, store.ErrParticipantNotFound
	}
	return p, nil
}

func (f *fakeLoginStore) InsertLoginCode(_ context.Context, code store.LoginCode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes = append(f.codes, code)
	return nil
}

func (f *fakeLoginStore) codesFor(participantID string) []store.LoginCode {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.LoginCode
	for _, c := range f.codes {
		if c.ParticipantID == participantID {
			out = append(out, c)
		}
	}
	return out
}

// fakeLoginMailer is an in-memory implementation of auth.Mailer that records
// every call instead of sending real email.
type fakeLoginMailer struct {
	mu   sync.Mutex
	sent []string // recipient addresses
}

func (f *fakeLoginMailer) Send(_ context.Context, to, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, to)
	return nil
}

func (f *fakeLoginMailer) sentTo(email string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, to := range f.sent {
		if to == email {
			return true
		}
	}
	return false
}

// loginScenarioState holds the fixtures and results for one
// request-login-code scenario. A fresh instance is created per scenario so
// state never leaks between runs.
type loginScenarioState struct {
	store    *fakeLoginStore
	mailer   *fakeLoginMailer
	server   *httptest.Server
	response *http.Response
	body     string
}

func newLoginScenarioState() *loginScenarioState {
	st := newFakeLoginStore()
	ml := &fakeLoginMailer{}
	svc := auth.NewService(st, ml)
	handler := web.NewServer(svc)

	return &loginScenarioState{
		store:  st,
		mailer: ml,
		server: httptest.NewServer(handler),
	}
}

func (s *loginScenarioState) close() {
	s.server.Close()
}

func (s *loginScenarioState) aParticipantIsSeededWithEmail(name, email string) error {
	s.store.seed(name, email)
	return nil
}

func (s *loginScenarioState) aVisitorOpensTheLoginPage() error {
	resp, err := http.Get(s.server.URL + "/login")
	if err != nil {
		return fmt.Errorf("get /login: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	s.response = resp
	s.body = string(body)
	return nil
}

func (s *loginScenarioState) theLoginPageShowsAnEmptyEmailField() error {
	if s.response == nil {
		return fmt.Errorf("no login page request has been made yet")
	}
	if s.response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP 200, got %d", s.response.StatusCode)
	}
	if !strings.Contains(s.body, `name="email"`) {
		return fmt.Errorf("expected the login page to contain an email field, got %q", s.body)
	}
	if strings.Contains(s.body, `value=`) {
		return fmt.Errorf("expected the email field to have no pre-filled value, got %q", s.body)
	}
	return nil
}

func (s *loginScenarioState) requestLoginCode(email string) error {
	resp, err := http.PostForm(s.server.URL+"/login", url.Values{"email": {email}})
	if err != nil {
		return fmt.Errorf("post /login: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	s.response = resp
	s.body = string(body)
	return nil
}

func (s *loginScenarioState) aVisitorHasAlreadyRequestedALoginCodeFor(email string) error {
	return s.requestLoginCode(email)
}

func (s *loginScenarioState) aVisitorRequestsALoginCodeFor(email string) error {
	return s.requestLoginCode(email)
}

func (s *loginScenarioState) aVisitorRequestsALoginCodeForAgain(email string) error {
	return s.requestLoginCode(email)
}

func (s *loginScenarioState) theResponseShowsTheGenericConfirmation(message string) error {
	if s.response == nil {
		return fmt.Errorf("no login-code request has been made yet")
	}
	if s.response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP 200, got %d", s.response.StatusCode)
	}
	if !strings.Contains(s.body, message) {
		return fmt.Errorf("expected response body to contain %q, got %q", message, s.body)
	}
	return nil
}

func (s *loginScenarioState) aLoginCodeIsPersistedFor(email string) error {
	p, err := s.store.ParticipantByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("expected %q to be a seeded participant: %w", email, err)
	}
	if len(s.store.codesFor(p.ID)) == 0 {
		return fmt.Errorf("expected at least one login code for %q", email)
	}
	return nil
}

func (s *loginScenarioState) noLoginCodeIsPersistedFor(email string) error {
	if _, err := s.store.ParticipantByEmail(context.Background(), email); err == nil {
		return fmt.Errorf("did not expect %q to be a seeded participant", email)
	}
	if len(s.store.codes) != 0 {
		return fmt.Errorf("expected no login codes to be persisted at all, found %d", len(s.store.codes))
	}
	return nil
}

func (s *loginScenarioState) aLoginCodeEmailIsSentTo(email string) error {
	if !s.mailer.sentTo(email) {
		return fmt.Errorf("expected an email to have been sent to %q", email)
	}
	return nil
}

func (s *loginScenarioState) noLoginCodeEmailIsSent() error {
	if len(s.mailer.sent) != 0 {
		return fmt.Errorf("expected no emails to be sent, got %d", len(s.mailer.sent))
	}
	return nil
}

func (s *loginScenarioState) nLoginCodesArePersistedFor(n int, email string) error {
	p, err := s.store.ParticipantByEmail(context.Background(), email)
	if err != nil {
		return err
	}
	got := len(s.store.codesFor(p.ID))
	if got != n {
		return fmt.Errorf("expected %d login codes for %q, got %d", n, email, got)
	}
	return nil
}

func (s *loginScenarioState) theFirstLoginCodeForIsStillUnused(email string) error {
	p, err := s.store.ParticipantByEmail(context.Background(), email)
	if err != nil {
		return err
	}
	codes := s.store.codesFor(p.ID)
	if len(codes) == 0 {
		return fmt.Errorf("expected at least one login code for %q", email)
	}
	if codes[0].UsedAt != nil {
		return fmt.Errorf("expected the first login code for %q to still be unused", email)
	}
	return nil
}

// InitializeLoginScenario registers the request-login-code step definitions
// with GoDog.
func InitializeLoginScenario(ctx *godog.ScenarioContext) {
	s := newLoginScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a Participant "([^"]*)" is seeded with email "([^"]*)"$`, s.aParticipantIsSeededWithEmail)
	ctx.Step(`^a visitor opens the login page$`, s.aVisitorOpensTheLoginPage)
	ctx.Step(`^the login page shows an empty email field$`, s.theLoginPageShowsAnEmptyEmailField)
	ctx.Step(`^a visitor has already requested a login code for "([^"]*)"$`, s.aVisitorHasAlreadyRequestedALoginCodeFor)
	ctx.Step(`^a visitor requests a login code for "([^"]*)"$`, s.aVisitorRequestsALoginCodeFor)
	ctx.Step(`^a visitor requests a login code for "([^"]*)" again$`, s.aVisitorRequestsALoginCodeForAgain)
	ctx.Step(`^the response shows the generic confirmation "([^"]*)"$`, s.theResponseShowsTheGenericConfirmation)
	ctx.Step(`^a login code is persisted for "([^"]*)"$`, s.aLoginCodeIsPersistedFor)
	ctx.Step(`^no login code is persisted for "([^"]*)"$`, s.noLoginCodeIsPersistedFor)
	ctx.Step(`^a login code email is sent to "([^"]*)"$`, s.aLoginCodeEmailIsSentTo)
	ctx.Step(`^no login code email is sent$`, s.noLoginCodeEmailIsSent)
	ctx.Step(`^(\d+) login codes are persisted for "([^"]*)"$`, s.nLoginCodesArePersistedFor)
	ctx.Step(`^the first login code for "([^"]*)" is still unused$`, s.theFirstLoginCodeForIsStillUnused)
}
