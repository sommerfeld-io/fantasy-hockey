package acceptance_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// stayLoggedInResponse is one recorded protected-route request result.
type stayLoggedInResponse struct {
	status   int
	location string
	cookies  []*http.Cookie
}

// stayLoggedInScenarioState holds the fixtures and results for one
// stay-logged-in scenario. A fresh instance is created per scenario so
// state never leaks between runs.
type stayLoggedInScenarioState struct {
	server        *httptest.Server
	sessionCookie *http.Cookie
	response      *stayLoggedInResponse
}

func newStayLoggedInScenarioState() *stayLoggedInScenarioState {
	return &stayLoggedInScenarioState{
		server: httptest.NewServer(web.NewServer(newTempStore(), noopSender, testSessionSecret)),
	}
}

func (s *stayLoggedInScenarioState) close() {
	s.server.Close()
}

func (s *stayLoggedInScenarioState) aPlayerIsLoggedInWithASessionIssuedMinutesAgo(minutes int) error {
	issuedAt := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	s.sessionCookie = signedSessionCookieForTest("basti", issuedAt)
	return nil
}

func (s *stayLoggedInScenarioState) noSessionCookieIsPresent() error {
	s.sessionCookie = nil
	return nil
}

func (s *stayLoggedInScenarioState) aPlayerIsLoggedInWithATamperedSessionCookie() error {
	s.sessionCookie = tamperSessionCookie(signedSessionCookieForTest("basti", time.Now().UTC()))
	return nil
}

func (s *stayLoggedInScenarioState) aPlayerIsLoggedInWithASessionCookieCarryingAnEmptyPlayerID() error {
	s.sessionCookie = signedSessionCookieForTest("", time.Now().UTC())
	return nil
}

// thePlayerRequestsAProtectedRoute requests the home page - today's only
// protected route - without following any redirect, so a step can inspect
// the 302 and its Set-Cookie header directly.
func (s *stayLoggedInScenarioState) thePlayerRequestsAProtectedRoute() error {
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}

	req, err := http.NewRequest(http.MethodGet, s.server.URL+"/", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if s.sessionCookie != nil {
		req.AddCookie(s.sessionCookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("get /: %w", err)
	}
	defer resp.Body.Close()

	s.response = &stayLoggedInResponse{
		status:   resp.StatusCode,
		location: resp.Header.Get("Location"),
		cookies:  resp.Cookies(),
	}
	return nil
}

func (s *stayLoggedInScenarioState) lastResponse() (*stayLoggedInResponse, error) {
	if s.response == nil {
		return nil, fmt.Errorf("no request has been made yet")
	}
	return s.response, nil
}

func (s *stayLoggedInScenarioState) protectedRouteResponseRedirectsTo(target string) error {
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
	if len(got.cookies) != 0 {
		return fmt.Errorf("expected no cookie to be set on a redirect, got %v", got.cookies)
	}
	return nil
}

func (s *stayLoggedInScenarioState) protectedRouteResponseCarriesAReIssuedSessionCookieForTheSamePlayer() error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	if got.status != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", got.status)
	}
	if len(got.cookies) != 1 {
		return fmt.Errorf("expected exactly 1 cookie to be re-issued, got %d", len(got.cookies))
	}

	c := got.cookies[0]
	if c.Name != auth.SessionCookieName {
		return fmt.Errorf("expected cookie name %q, got %q", auth.SessionCookieName, c.Name)
	}

	playerID, issuedAt, ok := auth.ParseSessionCookie(c, testSessionSecret)
	if !ok {
		return fmt.Errorf("expected the re-issued cookie to parse successfully")
	}
	if playerID != "basti" {
		return fmt.Errorf("expected player id %q, got %q", "basti", playerID)
	}
	if age := time.Since(issuedAt); age < -time.Minute || age > time.Minute {
		return fmt.Errorf("expected a freshly re-issued issued_at, got %v (%v old)", issuedAt, age)
	}
	return nil
}

// InitializeStayLoggedInScenario registers the stay-logged-in step
// definitions with GoDog.
func InitializeStayLoggedInScenario(ctx *godog.ScenarioContext) {
	s := newStayLoggedInScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a player is logged in with a session issued (\d+) minutes ago$`, s.aPlayerIsLoggedInWithASessionIssuedMinutesAgo)
	ctx.Step(`^no session cookie is present$`, s.noSessionCookieIsPresent)
	ctx.Step(`^a player is logged in with a tampered session cookie$`, s.aPlayerIsLoggedInWithATamperedSessionCookie)
	ctx.Step(`^a player is logged in with a session cookie carrying an empty player id$`, s.aPlayerIsLoggedInWithASessionCookieCarryingAnEmptyPlayerID)
	ctx.Step(`^the player requests a protected route$`, s.thePlayerRequestsAProtectedRoute)
	ctx.Step(`^the protected-route response redirects to "([^"]*)"$`, s.protectedRouteResponseRedirectsTo)
	ctx.Step(`^the protected-route response carries a re-issued session cookie for the same player$`,
		s.protectedRouteResponseCarriesAReIssuedSessionCookieForTheSamePlayer)
}
