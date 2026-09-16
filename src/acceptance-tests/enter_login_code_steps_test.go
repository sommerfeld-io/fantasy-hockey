package acceptance_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"
	yaml "go.yaml.in/yaml/v3"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// enterLoginCodeSecret signs session cookies for this scenario's server -
// it only needs to be non-empty and stable within one scenario run.
const enterLoginCodeSecret = "enter-login-code-test-secret"

// enterLoginCodeGenericErrorFragment is a substring of the generic
// code-entry error that survives html/template's escaping unchanged (its
// apostrophe is rendered as "&#39;"), so scenarios can assert on it without
// caring about that escaping.
const enterLoginCodeGenericErrorFragment = "check it and try again."

// enterLoginCodeResponse is one recorded POST /login/code result.
type enterLoginCodeResponse struct {
	status   int
	body     string
	cookies  []*http.Cookie
	location string
}

// enterLoginCodeScenarioState holds the fixtures and results for one enter-
// login-code scenario. A fresh instance is created per scenario so state
// never leaks between runs. The store and server are built lazily on the
// first submission, so a "Given ... issued N minutes ago" step can still
// override issuedAt beforehand.
type enterLoginCodeScenarioState struct {
	dataFile  string
	playerID  string
	code      string
	issuedAt  time.Time
	server    *httptest.Server
	responses []enterLoginCodeResponse
}

func newEnterLoginCodeScenarioState() *enterLoginCodeScenarioState {
	dir, err := os.MkdirTemp("", "fantasy-hockey-enter-code-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	return &enterLoginCodeScenarioState{
		dataFile: filepath.Join(dir, store.DataFileName),
		code:     "123456",
		issuedAt: time.Now().UTC(),
	}
}

func (s *enterLoginCodeScenarioState) close() {
	if s.server != nil {
		s.server.Close()
	}
	_ = os.RemoveAll(filepath.Dir(s.dataFile)) // best-effort cleanup of the scenario's temp dir
}

// aPlayerHasRequestedALoginCode seeds the data file with one hand-maintained
// player. The login code row itself is only persisted once ensureReady runs,
// so a later "issued N minutes ago" step can still change s.issuedAt first.
func (s *enterLoginCodeScenarioState) aPlayerHasRequestedALoginCode(name, email string) error {
	s.playerID = strings.ToLower(name)
	seed := fmt.Sprintf("season: \"2026-27\"\nplayers:\n    - id: %s\n      name: %s\n      email: %s\n",
		s.playerID, name, email)
	if err := os.WriteFile(s.dataFile, []byte(seed), 0o600); err != nil {
		return fmt.Errorf("seed data file: %w", err)
	}
	return nil
}

// theLoginCodeWasIssuedMinutesAgo backdates the not-yet-persisted login
// code's issued_at, so ensureReady() then persists an already-expired row.
func (s *enterLoginCodeScenarioState) theLoginCodeWasIssuedMinutesAgo(minutes int) error {
	s.issuedAt = time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	return nil
}

// ensureReady lazily persists the login code row and starts the real
// production web.NewServer handler around it, the first time a step needs
// to make an HTTP call.
func (s *enterLoginCodeScenarioState) ensureReady() error {
	if s.server != nil {
		return nil
	}

	st, err := store.New(s.dataFile)
	if err != nil {
		return fmt.Errorf("store.New: %w", err)
	}

	sum := sha256.Sum256([]byte(s.code))
	hash := hex.EncodeToString(sum[:])
	if err := st.CreateLoginCode(s.playerID, hash, s.issuedAt.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("seed login code: %w", err)
	}

	send := func(_, _, _ string) error { return nil }
	s.server = httptest.NewServer(web.NewServer(st, send, enterLoginCodeSecret))
	return nil
}

// postLoginCode submits code to POST /login/code without following any
// redirect, so a step can inspect the 302 and its Set-Cookie header
// directly.
func (s *enterLoginCodeScenarioState) postLoginCode(code string) error {
	if err := s.ensureReady(); err != nil {
		return err
	}

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.PostForm(s.server.URL+"/login/code", url.Values{"code": {code}})
	if err != nil {
		return fmt.Errorf("post /login/code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	s.responses = append(s.responses, enterLoginCodeResponse{
		status:   resp.StatusCode,
		body:     string(body),
		cookies:  resp.Cookies(),
		location: resp.Header.Get("Location"),
	})
	return nil
}

func (s *enterLoginCodeScenarioState) thePlayerSubmitsTheirLoginCode() error {
	return s.postLoginCode(s.code)
}

func (s *enterLoginCodeScenarioState) thePlayerSubmitsTheLoginCode(code string) error {
	return s.postLoginCode(code)
}

func (s *enterLoginCodeScenarioState) thePlayerHasAlreadySubmittedTheirLoginCodeOnce() error {
	return s.postLoginCode(s.code)
}

func (s *enterLoginCodeScenarioState) lastResponse() (enterLoginCodeResponse, error) {
	if len(s.responses) == 0 {
		return enterLoginCodeResponse{}, fmt.Errorf("no requests have been made yet")
	}
	return s.responses[len(s.responses)-1], nil
}

func (s *enterLoginCodeScenarioState) loginCodeResponseRedirectsTo(target string) error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	if got.status != http.StatusFound {
		return fmt.Errorf("expected status %d, got %d", http.StatusFound, got.status)
	}
	if got.location != target {
		return fmt.Errorf("expected a redirect to %q, got %q", target, got.location)
	}
	return nil
}

func (s *enterLoginCodeScenarioState) loginCodeResponseStatusIs(want int) error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	if got.status != want {
		return fmt.Errorf("expected status %d, got %d", want, got.status)
	}
	return nil
}

func (s *enterLoginCodeScenarioState) loginCodeResponseShowsTheGenericCodeError() error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	if !strings.Contains(got.body, enterLoginCodeGenericErrorFragment) {
		return fmt.Errorf("expected the generic code error, got body %q", got.body)
	}
	return nil
}

func (s *enterLoginCodeScenarioState) submittedCodeIsRetainedOnScreen(code string) error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	want := fmt.Sprintf(`value="%s"`, code)
	if !strings.Contains(got.body, want) {
		return fmt.Errorf("expected the submitted code to be retained, got body %q", got.body)
	}
	return nil
}

func (s *enterLoginCodeScenarioState) aSessionCookieIsSet() error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	if len(got.cookies) != 1 {
		return fmt.Errorf("expected exactly 1 cookie to be set, got %d", len(got.cookies))
	}
	c := got.cookies[0]
	if c.Name != "session" {
		return fmt.Errorf("expected cookie name %q, got %q", "session", c.Name)
	}
	if !c.HttpOnly {
		return fmt.Errorf("expected the session cookie to be HttpOnly")
	}
	return nil
}

// theSessionCookieIdentifiesAsTheLoggedInPlayer chains the session cookie
// the last response set into a real follow-up request against a protected
// shell route, and asserts the rendered header shows name - proving the
// login-code (Story 1.2) -> session-consumption (Story 1.5) handoff itself,
// not just that some cookie was set. A regression that issued the cookie
// for the wrong or an empty player id would render the wrong name (or the
// stale-player-id placeholder) here, even though every other assertion on
// this response already passes.
func (s *enterLoginCodeScenarioState) theSessionCookieIdentifiesAsTheLoggedInPlayer(name string) error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	if len(got.cookies) != 1 {
		return fmt.Errorf("expected exactly 1 session cookie, got %d", len(got.cookies))
	}

	req, err := http.NewRequest(http.MethodGet, s.server.URL+"/predict", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.AddCookie(got.cookies[0])

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("get /predict: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if !strings.Contains(string(body), name) {
		return fmt.Errorf("expected the shell to identify %q as the logged-in player, got %q", name, string(body))
	}
	return nil
}

func (s *enterLoginCodeScenarioState) noSessionCookieIsSet() error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	if len(got.cookies) != 0 {
		return fmt.Errorf("expected no cookie to be set, got %v", got.cookies)
	}
	return nil
}

// enterLoginCodeDocument mirrors internal/store's on-disk shape closely
// enough to assert on what actually landed in fantasy-hockey.yml.
type enterLoginCodeDocument struct {
	LoginCodes []struct {
		CodeHash string  `yaml:"code_hash"`
		UsedAt   *string `yaml:"used_at"`
	} `yaml:"login_codes"`
}

func (s *enterLoginCodeScenarioState) theLoginCodeIsMarkedUsed() error {
	raw, err := os.ReadFile(s.dataFile)
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	var doc enterLoginCodeDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("unmarshal data file: %w", err)
	}

	sum := sha256.Sum256([]byte(s.code))
	hash := hex.EncodeToString(sum[:])
	for _, row := range doc.LoginCodes {
		if row.CodeHash == hash {
			if row.UsedAt == nil {
				return fmt.Errorf("expected the login code's used_at to be set, got nil")
			}
			return nil
		}
	}
	return fmt.Errorf("expected a persisted login code matching the emailed code, found %+v", doc.LoginCodes)
}

// InitializeEnterLoginCodeScenario registers the enter-login-code step
// definitions with GoDog.
func InitializeEnterLoginCodeScenario(ctx *godog.ScenarioContext) {
	s := newEnterLoginCodeScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a player "([^"]*)" with email "([^"]*)" has requested a login code$`, s.aPlayerHasRequestedALoginCode)
	ctx.Step(`^the login code was issued (\d+) minutes ago$`, s.theLoginCodeWasIssuedMinutesAgo)
	ctx.Step(`^the player submits their login code$`, s.thePlayerSubmitsTheirLoginCode)
	ctx.Step(`^the player submits the login code "([^"]*)"$`, s.thePlayerSubmitsTheLoginCode)
	ctx.Step(`^the player has already submitted their login code once$`, s.thePlayerHasAlreadySubmittedTheirLoginCodeOnce)
	ctx.Step(`^the login-code response redirects to "([^"]*)"$`, s.loginCodeResponseRedirectsTo)
	ctx.Step(`^the login-code response status is (\d+)$`, s.loginCodeResponseStatusIs)
	ctx.Step(`^the login-code response shows the generic code error$`, s.loginCodeResponseShowsTheGenericCodeError)
	ctx.Step(`^the submitted code "([^"]*)" is retained on the screen$`, s.submittedCodeIsRetainedOnScreen)
	ctx.Step(`^a session cookie is set$`, s.aSessionCookieIsSet)
	ctx.Step(`^the session cookie identifies "([^"]*)" as the logged-in player$`, s.theSessionCookieIdentifiesAsTheLoggedInPlayer)
	ctx.Step(`^no session cookie is set$`, s.noSessionCookieIsSet)
	ctx.Step(`^the login code is marked used$`, s.theLoginCodeIsMarkedUsed)
}
