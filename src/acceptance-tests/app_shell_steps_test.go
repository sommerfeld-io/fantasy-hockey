package acceptance_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// appShellPlayerID is the seeded player's id, matching the session cookie
// "the player \"Basti\" is signed in" issues.
const appShellPlayerID = "basti"

// appShellPlayerName is appShellPlayerID's seeded display name. Kept in
// sync with it so a scenario naming a different player fails loudly in
// thePlayerIsSignedIn rather than silently signing in as this fixture.
const appShellPlayerName = "Basti"

// navHrefsByTab maps each bottom-nav tab label to its route, so
// theShellShowsAsTheActiveTab can tie a tab name to its own anchor's
// active state without depending on the icon markup between the anchor's
// opening tag and its label.
var navHrefsByTab = map[string]string{
	"Predict":     "/predict",
	"Leaderboard": "/leaderboard",
	"Compare":     "/compare",
}

// newAppShellStore bootstraps a store seeded with one player ("Basti"),
// since (unlike newTempStore, which starts with an empty player list) the
// shell's header needs a real player to resolve the session's id against.
func newAppShellStore() *store.Store {
	dir, err := os.MkdirTemp("", "fantasy-hockey-app-shell-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
`
	path := filepath.Join(dir, "fantasy-hockey.yml")
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		panic(fmt.Sprintf("seed file: %v", err))
	}
	st, err := store.New(path)
	if err != nil {
		panic(fmt.Sprintf("store.New: %v", err))
	}
	return st
}

// appShellScenarioState holds the fixtures and results for one app-shell
// scenario. A fresh instance is created per scenario so state never leaks
// between runs.
type appShellScenarioState struct {
	server        *httptest.Server
	sessionCookie *http.Cookie
	lastStatus    int
	lastLocation  string
	lastBody      string
	visitedBodies []string
}

func newAppShellScenarioState() *appShellScenarioState {
	return &appShellScenarioState{
		server: httptest.NewServer(web.NewServer(newAppShellStore(), noopSender, testSessionSecret)),
	}
}

func (s *appShellScenarioState) close() {
	s.server.Close()
}

func (s *appShellScenarioState) thePlayerIsSignedIn(name string) error {
	if name != appShellPlayerName {
		return fmt.Errorf("no fixture for player %q; only %q is seeded", name, appShellPlayerName)
	}
	s.sessionCookie = auth.IssueSessionCookie(appShellPlayerID, testSessionSecret)
	return nil
}

// visit requests path carrying the scenario's session cookie, without
// following any redirect - a step can then inspect either a rendered shell
// page or a 302 (e.g. the logout control's response) directly.
func (s *appShellScenarioState) visit(path string) error {
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}

	req, err := http.NewRequest(http.MethodGet, s.server.URL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if s.sessionCookie != nil {
		req.AddCookie(s.sessionCookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	s.lastStatus = resp.StatusCode
	s.lastLocation = resp.Header.Get("Location")
	s.lastBody = string(body)
	s.visitedBodies = append(s.visitedBodies, s.lastBody)
	return nil
}

func (s *appShellScenarioState) thePlayerVisitsTheRootPath() error {
	return s.visit("/")
}

func (s *appShellScenarioState) thePlayerVisitsTheDestination(destination string) error {
	return s.visit("/" + strings.ToLower(destination))
}

func (s *appShellScenarioState) theShellShowsThePlayersName(name string) error {
	if !strings.Contains(s.lastBody, name) {
		return fmt.Errorf("expected the shell to show the player's name %q, got %q", name, s.lastBody)
	}
	return nil
}

func (s *appShellScenarioState) theShellShowsTheSeason(season string) error {
	if !strings.Contains(s.lastBody, season) {
		return fmt.Errorf("expected the shell to show the season %q, got %q", season, s.lastBody)
	}
	return nil
}

func (s *appShellScenarioState) theShellShowsAsTheActiveTab(tab string) error {
	href, ok := navHrefsByTab[tab]
	if !ok {
		return fmt.Errorf("no known nav href for tab %q", tab)
	}
	active := `<a href="` + href + `" class="nav-item active" aria-current="page">`
	if !strings.Contains(s.lastBody, active) {
		return fmt.Errorf("expected %q to render as the active tab, got %q", tab, s.lastBody)
	}
	return nil
}

// theShellShowsThePlaceholderContent asserts Leaderboard/Compare's static
// "Coming soon" content - Predict has no placeholder left to assert here
// since Story 2.1 replaced it with real content, covered by
// browse-prediction-sets.feature instead.
func (s *appShellScenarioState) theShellShowsThePlaceholderContent(tab string) error {
	fragments := map[string]string{
		"Leaderboard": "The leaderboard is coming soon.",
		"Compare":     "Player comparison is coming soon.",
	}
	fragment, ok := fragments[tab]
	if !ok {
		return fmt.Errorf("no known placeholder fragment for tab %q", tab)
	}
	if !strings.Contains(s.lastBody, fragment) {
		return fmt.Errorf("expected the %s placeholder content %q, got %q", tab, fragment, s.lastBody)
	}
	return nil
}

// theShellShowsThePredictContent is a smoke check that "/" and "/predict"
// serve the same real content (Story 2.1's phase-grouped Prediction Set
// lists), mirroring what shellRouteTests already checks at the unit level -
// not a full re-test of Story 2.1's own feature file.
func (s *appShellScenarioState) theShellShowsThePredictContent() error {
	if !strings.Contains(s.lastBody, "Before the season") {
		return fmt.Errorf("expected the shell to show the Predict content, got %q", s.lastBody)
	}
	return nil
}

func (s *appShellScenarioState) everyVisitedDestinationShowedThePlayersName(name string) error {
	if len(s.visitedBodies) == 0 {
		return fmt.Errorf("no destination has been visited yet")
	}
	for i, body := range s.visitedBodies {
		if !strings.Contains(body, name) {
			return fmt.Errorf("expected visit #%d to show the player's name %q, got %q", i+1, name, body)
		}
	}
	return nil
}

func (s *appShellScenarioState) thePlayerUsesTheShellsLogoutControl() error {
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}

	req, err := http.NewRequest(http.MethodPost, s.server.URL+"/logout", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if s.sessionCookie != nil {
		req.AddCookie(s.sessionCookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post /logout: %w", err)
	}
	defer resp.Body.Close()

	s.lastStatus = resp.StatusCode
	s.lastLocation = resp.Header.Get("Location")
	return nil
}

func (s *appShellScenarioState) thePlayerIsRedirectedTo(target string) error {
	if s.lastStatus != http.StatusFound {
		return fmt.Errorf("expected status %d, got %d", http.StatusFound, s.lastStatus)
	}
	if s.lastLocation != target {
		return fmt.Errorf("expected a redirect to %q, got %q", target, s.lastLocation)
	}
	return nil
}

// InitializeAppShellScenario registers the app-shell step definitions with
// GoDog.
func InitializeAppShellScenario(ctx *godog.ScenarioContext) {
	s := newAppShellScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the player "([^"]*)" is signed in$`, s.thePlayerIsSignedIn)
	ctx.Step(`^the player visits the root path$`, s.thePlayerVisitsTheRootPath)
	ctx.Step(`^the player visits the (Predict|Leaderboard|Compare) destination$`, s.thePlayerVisitsTheDestination)
	ctx.Step(`^the shell shows the player's name "([^"]*)"$`, s.theShellShowsThePlayersName)
	ctx.Step(`^the shell shows the season "([^"]*)"$`, s.theShellShowsTheSeason)
	ctx.Step(`^the shell shows "([^"]*)" as the active tab$`, s.theShellShowsAsTheActiveTab)
	ctx.Step(`^the shell shows the (Leaderboard|Compare) placeholder content$`, s.theShellShowsThePlaceholderContent)
	ctx.Step(`^the shell shows the Predict content$`, s.theShellShowsThePredictContent)
	ctx.Step(`^every visited destination showed the player's name "([^"]*)"$`, s.everyVisitedDestinationShowedThePlayersName)
	ctx.Step(`^the player uses the shell's logout control$`, s.thePlayerUsesTheShellsLogoutControl)
	ctx.Step(`^the player is redirected to "([^"]*)"$`, s.thePlayerIsRedirectedTo)
}
