package acceptance_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// logOutResponse is one recorded request result.
type logOutResponse struct {
	status   int
	location string
	cookies  []*http.Cookie
}

// logOutScenarioState holds the fixtures and results for one log-out
// scenario. A fresh instance is created per scenario so state never leaks
// between runs. client carries a real cookiejar, so a Set-Cookie with a
// negative Max-Age genuinely removes the cookie from later requests; the
// "cookie jar no longer holds a session cookie" step asserts this directly
// (not just that the follow-up request happens to redirect, which an empty-
// but-not-actually-removed cookie would also produce).
type logOutScenarioState struct {
	server   *httptest.Server
	client   *http.Client
	response *logOutResponse
}

func newLogOutScenarioState() *logOutScenarioState {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(fmt.Sprintf("create cookie jar: %v", err))
	}
	return &logOutScenarioState{
		server: httptest.NewServer(web.NewServer(newTempStore(), noopSender, testSessionSecret)),
		client: &http.Client{
			Jar:           jar,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *logOutScenarioState) close() {
	s.server.Close()
}

// serverURL parses the test server's own URL, used to seed the client's
// cookie jar directly (as if a previous login had already set a cookie)
// without making a real request.
func (s *logOutScenarioState) serverURL() *url.URL {
	u, err := url.Parse(s.server.URL)
	if err != nil {
		panic(fmt.Sprintf("parse server URL: %v", err))
	}
	return u
}

func (s *logOutScenarioState) addSessionCookie(c *http.Cookie) {
	s.client.Jar.SetCookies(s.serverURL(), []*http.Cookie{c})
}

func (s *logOutScenarioState) aPlayerHasAnActiveSession() error {
	s.addSessionCookie(auth.IssueSessionCookie("basti", testSessionSecret))
	return nil
}

func (s *logOutScenarioState) thePlayerHasNoSessionCookie() error {
	return nil
}

func (s *logOutScenarioState) thePlayersSessionWasIssuedMinutesAgo(minutes int) error {
	issuedAt := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	s.addSessionCookie(signedSessionCookieForTest("basti", issuedAt))
	return nil
}

func (s *logOutScenarioState) thePlayersSessionCookieHasBeenTamperedWith() error {
	s.addSessionCookie(tamperSessionCookie(auth.IssueSessionCookie("basti", testSessionSecret)))
	return nil
}

func (s *logOutScenarioState) theBrowsersCookieJarNoLongerHoldsASessionCookie() error {
	for _, c := range s.client.Jar.Cookies(s.serverURL()) {
		if c.Name == auth.SessionCookieName {
			return fmt.Errorf("expected the cookie jar to hold no session cookie after logout, found %v", c)
		}
	}
	return nil
}

func (s *logOutScenarioState) thePlayerLogsOut() error {
	resp, err := s.client.Post(s.server.URL+"/logout", "", nil)
	if err != nil {
		return fmt.Errorf("post /logout: %w", err)
	}
	defer resp.Body.Close()

	s.response = &logOutResponse{
		status:   resp.StatusCode,
		location: resp.Header.Get("Location"),
		cookies:  resp.Cookies(),
	}
	return nil
}

func (s *logOutScenarioState) thePlayerMakesTheirNextRequestToAProtectedRoute() error {
	resp, err := s.client.Get(s.server.URL + "/")
	if err != nil {
		return fmt.Errorf("get /: %w", err)
	}
	defer resp.Body.Close()

	s.response = &logOutResponse{
		status:   resp.StatusCode,
		location: resp.Header.Get("Location"),
		cookies:  resp.Cookies(),
	}
	return nil
}

func (s *logOutScenarioState) lastResponse() (*logOutResponse, error) {
	if s.response == nil {
		return nil, fmt.Errorf("no request has been made yet")
	}
	return s.response, nil
}

func (s *logOutScenarioState) redirectsTo(target string) error {
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

func (s *logOutScenarioState) theLogoutResponseRedirectsTo(target string) error {
	return s.redirectsTo(target)
}

func (s *logOutScenarioState) thatRequestIsRedirectedTo(target string) error {
	return s.redirectsTo(target)
}

func (s *logOutScenarioState) theLogoutResponseClearsTheSessionCookie() error {
	got, err := s.lastResponse()
	if err != nil {
		return err
	}
	if len(got.cookies) != 1 {
		return fmt.Errorf("expected exactly 1 cookie to be set, got %d", len(got.cookies))
	}
	c := got.cookies[0]
	if c.Name != auth.SessionCookieName {
		return fmt.Errorf("expected cookie name %q, got %q", auth.SessionCookieName, c.Name)
	}
	if c.Value != "" {
		return fmt.Errorf("expected an empty cookie value, got %q", c.Value)
	}
	if c.MaxAge >= 0 {
		return fmt.Errorf("expected a negative Max-Age so the browser deletes the cookie immediately, got %d", c.MaxAge)
	}
	return nil
}

// InitializeLogOutScenario registers the log-out step definitions with
// GoDog.
func InitializeLogOutScenario(ctx *godog.ScenarioContext) {
	s := newLogOutScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a player has an active session$`, s.aPlayerHasAnActiveSession)
	ctx.Step(`^the player has no session cookie$`, s.thePlayerHasNoSessionCookie)
	ctx.Step(`^the player's session was issued (\d+) minutes ago$`, s.thePlayersSessionWasIssuedMinutesAgo)
	ctx.Step(`^the player's session cookie has been tampered with$`, s.thePlayersSessionCookieHasBeenTamperedWith)
	ctx.Step(`^the player logs out$`, s.thePlayerLogsOut)
	ctx.Step(`^the player makes their next request to a protected route$`, s.thePlayerMakesTheirNextRequestToAProtectedRoute)
	ctx.Step(`^the logout response redirects to "([^"]*)"$`, s.theLogoutResponseRedirectsTo)
	ctx.Step(`^the logout response clears the session cookie$`, s.theLogoutResponseClearsTheSessionCookie)
	ctx.Step(`^the browser's cookie jar no longer holds a session cookie$`, s.theBrowsersCookieJarNoLongerHoldsASessionCookie)
	ctx.Step(`^that request is redirected to "([^"]*)"$`, s.thatRequestIsRedirectedTo)
}
