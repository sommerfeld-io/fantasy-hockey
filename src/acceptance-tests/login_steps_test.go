package acceptance_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// acceptanceSessionSecret signs session tokens for every scenario in this
// suite - equivalent to the SESSION_SECRET env var required in production.
const acceptanceSessionSecret = "acceptance-test-session-secret"

// sessionCookieName mirrors the (unexported) cookie name web.NewServer's
// handlers actually set on the wire - acceptance tests only observe the
// HTTP-level contract, never internal package details.
const sessionCookieName = "session"

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

func (f *fakeLoginStore) UnusedLoginCodesForParticipant(_ context.Context, participantID string, issuedAfter time.Time) ([]store.LoginCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.LoginCode
	for _, c := range f.codes {
		if c.ParticipantID == participantID && c.UsedAt == nil && c.IssuedAt.After(issuedAfter) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeLoginStore) MarkLoginCodeUsed(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.codes {
		if f.codes[i].ID == id {
			usedAt := time.Now().UTC()
			f.codes[i].UsedAt = &usedAt
		}
	}
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

// backdateLatestLoginCode sets the issued_at of the most recently inserted
// login code for participantID to ago in the past, letting scenarios
// simulate the 10-minute expiry window without waiting in real time.
func (f *fakeLoginStore) backdateLatestLoginCode(participantID string, ago time.Duration) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.codes) - 1; i >= 0; i-- {
		if f.codes[i].ParticipantID == participantID {
			f.codes[i].IssuedAt = time.Now().UTC().Add(-ago)
			return true
		}
	}
	return false
}

// sixDigitCodePattern extracts the plaintext login code from an emailed
// message body, mirroring internal/auth's own code shape.
var sixDigitCodePattern = regexp.MustCompile(`\b\d{6}\b`)

// fakeLoginMailer is an in-memory implementation of auth.Mailer that records
// every call instead of sending real email. It captures the full body (not
// just the recipient) so scenarios can extract the emailed code.
type fakeLoginMailer struct {
	mu   sync.Mutex
	sent []sentLoginEmail
}

type sentLoginEmail struct {
	to, body string
}

func (f *fakeLoginMailer) Send(_ context.Context, to, _, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentLoginEmail{to: to, body: body})
	return nil
}

func (f *fakeLoginMailer) sentTo(email string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sent {
		if s.to == email {
			return true
		}
	}
	return false
}

// lastCodeSentTo returns the plaintext code from the most recent email sent
// to email, so a scenario can submit the exact code a Participant would have
// received without the test ever touching the store's hashed copy.
func (f *fakeLoginMailer) lastCodeSentTo(email string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.sent) - 1; i >= 0; i-- {
		if f.sent[i].to == email {
			code := sixDigitCodePattern.FindString(f.sent[i].body)
			return code, code != ""
		}
	}
	return "", false
}

// loginScenarioState holds the fixtures and results for one login/session
// scenario. A fresh instance is created per scenario so state never leaks
// between runs. The httptest.Client carries a cookie jar so the session
// cookie set by one request persists to the next, exactly like a browser.
type loginScenarioState struct {
	store    *fakeLoginStore
	mailer   *fakeLoginMailer
	auth     *auth.Service
	server   *httptest.Server
	client   *http.Client
	response *http.Response
	body     string
}

func newLoginScenarioState() *loginScenarioState {
	st := newFakeLoginStore()
	ml := &fakeLoginMailer{}
	svc := auth.NewService(st, ml, acceptanceSessionSecret)
	handler := web.NewServer(svc)

	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(fmt.Errorf("create cookie jar: %w", err))
	}

	return &loginScenarioState{
		store:  st,
		mailer: ml,
		auth:   svc,
		server: httptest.NewServer(handler),
		client: &http.Client{Jar: jar},
	}
}

func (s *loginScenarioState) close() {
	s.server.Close()
}

func (s *loginScenarioState) aParticipantIsSeededWithEmail(name, email string) error {
	s.store.seed(name, email)
	return nil
}

func (s *loginScenarioState) recordResponse(resp *http.Response) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	s.response = resp
	s.body = string(body)
	return nil
}

func (s *loginScenarioState) aVisitorOpensTheLoginPage() error {
	resp, err := s.client.Get(s.server.URL + "/login")
	if err != nil {
		return fmt.Errorf("get /login: %w", err)
	}
	return s.recordResponse(resp)
}

func (s *loginScenarioState) theLoginPageShowsAnEmptyEmailField() error {
	if s.response == nil {
		return fmt.Errorf("no login page request has been made yet")
	}
	if s.response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP 200, got %d", s.response.StatusCode)
	}
	if !strings.Contains(s.body, `type="email"`) {
		return fmt.Errorf("expected the login page to contain an email field, got %q", s.body)
	}
	if strings.Contains(s.body, `value=`) {
		return fmt.Errorf("expected the email field to have no pre-filled value, got %q", s.body)
	}
	return nil
}

func (s *loginScenarioState) theLoginPageShowsACodeFieldInsteadOfAnEmailField() error {
	if s.response == nil {
		return fmt.Errorf("no login-code request has been made yet")
	}
	if !strings.Contains(s.body, `name="code"`) {
		return fmt.Errorf("expected the login page to show a code field, got %q", s.body)
	}
	if strings.Contains(s.body, `type="email"`) {
		return fmt.Errorf("expected the email field to no longer be shown, got %q", s.body)
	}
	return nil
}

func (s *loginScenarioState) requestLoginCode(email string) error {
	resp, err := s.client.PostForm(s.server.URL+"/login", url.Values{"email": {email}})
	if err != nil {
		return fmt.Errorf("post /login: %w", err)
	}
	return s.recordResponse(resp)
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

func (s *loginScenarioState) theResponseShows(message string) error {
	if s.response == nil {
		return fmt.Errorf("no request has been made yet")
	}
	if s.response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP 200, got %d", s.response.StatusCode)
	}
	if !strings.Contains(s.body, message) {
		return fmt.Errorf("expected response body to contain %q, got %q", message, s.body)
	}
	return nil
}

// theResponseDoesNotShow asserts message is absent from the last recorded
// response body - used to confirm sessionExpiredMessage is never shown when
// no session cookie was ever present (as opposed to a present-but-expired
// one, which does show it).
func (s *loginScenarioState) theResponseDoesNotShow(message string) error {
	if s.response == nil {
		return fmt.Errorf("no request has been made yet")
	}
	if strings.Contains(s.body, message) {
		return fmt.Errorf("expected response body not to contain %q, got %q", message, s.body)
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

// submitCode posts email/code to /login/code. The client follows a
// successful 303 redirect to / automatically, so on success s.response/body
// end up reflecting the authenticated home placeholder rather than the
// redirect itself.
func (s *loginScenarioState) submitCode(email, code string) error {
	resp, err := s.client.PostForm(s.server.URL+"/login/code", url.Values{"email": {email}, "code": {code}})
	if err != nil {
		return fmt.Errorf("post /login/code: %w", err)
	}
	return s.recordResponse(resp)
}

func (s *loginScenarioState) theVisitorSubmitsTheEmailedCodeFor(email string) error {
	code, ok := s.mailer.lastCodeSentTo(email)
	if !ok {
		return fmt.Errorf("no login code email was sent to %q", email)
	}
	return s.submitCode(email, code)
}

func (s *loginScenarioState) theVisitorSubmitsTheCodeFor(code, email string) error {
	return s.submitCode(email, code)
}

func (s *loginScenarioState) theLoginCodeForWasIssuedMinutesAgo(email string, minutes int) error {
	p, err := s.store.ParticipantByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("expected %q to be a seeded participant: %w", email, err)
	}
	if !s.store.backdateLatestLoginCode(p.ID, time.Duration(minutes)*time.Minute) {
		return fmt.Errorf("expected at least one login code for %q to backdate", email)
	}
	return nil
}

// aVisitorHoldsASessionForIssuedMinutesAgo mints a signed session cookie
// with an explicit past issued-at directly via auth.Service.EncodeSession,
// simulating an idle Participant without waiting real minutes.
func (s *loginScenarioState) aVisitorHoldsASessionForIssuedMinutesAgo(email string, minutesAgo int) error {
	p, err := s.store.ParticipantByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("expected %q to be a seeded participant: %w", email, err)
	}

	token := s.auth.EncodeSession(auth.Session{
		ParticipantID: p.ID,
		IssuedAt:      time.Now().UTC().Add(-time.Duration(minutesAgo) * time.Minute),
	})

	u, err := url.Parse(s.server.URL)
	if err != nil {
		return fmt.Errorf("parse server URL: %w", err)
	}
	s.client.Jar.SetCookies(u, []*http.Cookie{{Name: sessionCookieName, Value: token}})
	return nil
}

// theVisitorHasNoSessionCookie clears any session cookie held by the client
// jar - used to isolate a later request from a session established by an
// earlier, unrelated step in the same scenario (e.g. the successful first
// use of a code that a later step then attempts to reuse).
func (s *loginScenarioState) theVisitorHasNoSessionCookie() error {
	u, err := url.Parse(s.server.URL)
	if err != nil {
		return fmt.Errorf("parse server URL: %w", err)
	}
	s.client.Jar.SetCookies(u, []*http.Cookie{{Name: sessionCookieName, Value: "", MaxAge: -1}})
	return nil
}

func (s *loginScenarioState) theVisitorVisitsTheHomePage() error {
	resp, err := s.client.Get(s.server.URL + "/")
	if err != nil {
		return fmt.Errorf("get /: %w", err)
	}
	return s.recordResponse(resp)
}

func (s *loginScenarioState) theVisitorIsAuthenticated() error {
	resp, err := s.client.Get(s.server.URL + "/")
	if err != nil {
		return fmt.Errorf("get /: %w", err)
	}
	if err := s.recordResponse(resp); err != nil {
		return err
	}
	if s.response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP 200 from an authenticated request to /, got %d", s.response.StatusCode)
	}
	if !strings.Contains(s.body, "Signed in.") {
		return fmt.Errorf("expected the authenticated home placeholder, got %q", s.body)
	}
	return nil
}

func (s *loginScenarioState) theVisitorIsNotAuthenticated() error {
	resp, err := s.client.Get(s.server.URL + "/")
	if err != nil {
		return fmt.Errorf("get /: %w", err)
	}
	if err := s.recordResponse(resp); err != nil {
		return err
	}
	if s.response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP 200 from an unauthenticated request to /, got %d", s.response.StatusCode)
	}
	if strings.Contains(s.body, "Signed in.") {
		return fmt.Errorf("expected the visitor not to be authenticated, got %q", s.body)
	}
	return nil
}

func (s *loginScenarioState) theVisitorsSessionForWasReissuedJustNow(email string) error {
	if s.response == nil {
		return fmt.Errorf("no request has been made yet")
	}

	var cookie *http.Cookie
	for _, c := range s.response.Cookies() {
		if c.Name == sessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		return fmt.Errorf("expected the response to set a fresh session cookie")
	}

	sess, err := s.auth.DecodeSession(cookie.Value)
	if err != nil {
		return fmt.Errorf("decode reissued session: %w", err)
	}

	p, err := s.store.ParticipantByEmail(context.Background(), email)
	if err != nil {
		return err
	}
	if sess.ParticipantID != p.ID {
		return fmt.Errorf("expected the reissued session to belong to %q, got participant %q", email, sess.ParticipantID)
	}
	if age := time.Since(sess.IssuedAt); age > time.Minute {
		return fmt.Errorf("expected the reissued session's issued-at to be recent, got %v ago", age)
	}
	return nil
}

// InitializeLoginScenario registers the login-code request/validation and
// session step definitions with GoDog.
func InitializeLoginScenario(ctx *godog.ScenarioContext) {
	s := newLoginScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a Participant "([^"]*)" is seeded with email "([^"]*)"$`, s.aParticipantIsSeededWithEmail)
	ctx.Step(`^a visitor opens the login page$`, s.aVisitorOpensTheLoginPage)
	ctx.Step(`^the login page shows an empty email field$`, s.theLoginPageShowsAnEmptyEmailField)
	ctx.Step(`^the login page shows a code field instead of an email field$`, s.theLoginPageShowsACodeFieldInsteadOfAnEmailField)
	ctx.Step(`^a visitor has already requested a login code for "([^"]*)"$`, s.aVisitorHasAlreadyRequestedALoginCodeFor)
	ctx.Step(`^a visitor requests a login code for "([^"]*)"$`, s.aVisitorRequestsALoginCodeFor)
	ctx.Step(`^a visitor requests a login code for "([^"]*)" again$`, s.aVisitorRequestsALoginCodeForAgain)
	ctx.Step(`^the response shows the generic confirmation "([^"]*)"$`, s.theResponseShows)
	ctx.Step(`^the response shows "([^"]*)"$`, s.theResponseShows)
	ctx.Step(`^the response does not show "([^"]*)"$`, s.theResponseDoesNotShow)
	ctx.Step(`^a login code is persisted for "([^"]*)"$`, s.aLoginCodeIsPersistedFor)
	ctx.Step(`^no login code is persisted for "([^"]*)"$`, s.noLoginCodeIsPersistedFor)
	ctx.Step(`^a login code email is sent to "([^"]*)"$`, s.aLoginCodeEmailIsSentTo)
	ctx.Step(`^no login code email is sent$`, s.noLoginCodeEmailIsSent)
	ctx.Step(`^(\d+) login codes are persisted for "([^"]*)"$`, s.nLoginCodesArePersistedFor)
	ctx.Step(`^the first login code for "([^"]*)" is still unused$`, s.theFirstLoginCodeForIsStillUnused)

	ctx.Step(`^the visitor submits the emailed code for "([^"]*)" again$`, s.theVisitorSubmitsTheEmailedCodeFor)
	ctx.Step(`^the visitor submits the emailed code for "([^"]*)"$`, s.theVisitorSubmitsTheEmailedCodeFor)
	ctx.Step(`^the visitor submits the code "([^"]*)" for "([^"]*)"$`, s.theVisitorSubmitsTheCodeFor)
	ctx.Step(`^the login code for "([^"]*)" was issued (\d+) minutes? ago$`, func(email, minutes string) error {
		n, err := strconv.Atoi(minutes)
		if err != nil {
			return fmt.Errorf("parse minutes: %w", err)
		}
		return s.theLoginCodeForWasIssuedMinutesAgo(email, n)
	})
	ctx.Step(`^a visitor holds a session for "([^"]*)" issued (\d+) minutes? ago$`, func(email, minutes string) error {
		n, err := strconv.Atoi(minutes)
		if err != nil {
			return fmt.Errorf("parse minutes: %w", err)
		}
		return s.aVisitorHoldsASessionForIssuedMinutesAgo(email, n)
	})
	ctx.Step(`^the visitor has no session cookie$`, s.theVisitorHasNoSessionCookie)
	ctx.Step(`^the visitor visits the home page$`, s.theVisitorVisitsTheHomePage)
	ctx.Step(`^the visitor is authenticated$`, s.theVisitorIsAuthenticated)
	ctx.Step(`^the visitor is not authenticated$`, s.theVisitorIsNotAuthenticated)
	ctx.Step(`^the visitor's session for "([^"]*)" was re-issued just now$`, s.theVisitorsSessionForWasReissuedJustNow)
}
