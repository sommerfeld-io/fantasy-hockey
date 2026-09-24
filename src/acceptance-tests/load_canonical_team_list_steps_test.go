package acceptance_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// loadCanonicalTeamListPlayerID/Name mirror app_shell_steps_test.go's seeded
// fixture player, so this scenario can reuse the same "the player ... is
// signed in" phrasing.
const (
	loadCanonicalTeamListPlayerID   = "basti"
	loadCanonicalTeamListPlayerName = "Basti"
)

// loadCanonicalTeamListSeed is a full fantasy-hockey.yml document: one
// seeded player plus a small representative sample of teams (one per
// division, spanning both conferences) - deliberately not the real file's
// full 32-team roster, so this fixture can't silently drift out of sync
// with fantasy-hockey.yml's own teams: section as that roster is
// hand-edited over time. This scenario only proves a well-formed teams
// section doesn't break store.New's parsing or the Predict screen's
// rendering - it never asserts on the teams themselves, since no UI reads
// Store.Teams() yet (Story 2.2's Boundaries: "renders nothing itself").
const loadCanonicalTeamListSeed = `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
teams:
    - id: TOR
      name: Toronto Maple Leafs
      conference: Eastern
      division: Atlantic
    - id: WSH
      name: Washington Capitals
      conference: Eastern
      division: Metropolitan
    - id: COL
      name: Colorado Avalanche
      conference: Western
      division: Central
    - id: VGK
      name: Vegas Golden Knights
      conference: Western
      division: Pacific
`

// loadCanonicalTeamListScenarioState holds the fixtures and last response
// for one scenario. A fresh instance is created per scenario so state never
// leaks between runs.
type loadCanonicalTeamListScenarioState struct {
	dataFile      string
	server        *httptest.Server
	sessionCookie *http.Cookie
	lastStatus    int
}

func newLoadCanonicalTeamListScenarioState() *loadCanonicalTeamListScenarioState {
	st, dataFile := newSeededStore("load-canonical-team-list", loadCanonicalTeamListSeed)
	return &loadCanonicalTeamListScenarioState{
		dataFile: dataFile,
		server:   httptest.NewServer(web.NewServer(st, noopSender, testSessionSecret)),
	}
}

func (s *loadCanonicalTeamListScenarioState) close() {
	s.server.Close()
	removeScenarioDataFile(s.dataFile)
}

// aDataFileWhoseTeamsSectionHoldsASampleOfCanonicalTeams is a no-op: the
// scenario's server is already wired against loadCanonicalTeamListSeed by
// newLoadCanonicalTeamListScenarioState. The step exists so the Gherkin
// reads as a Given on the data file's shape, matching how other features
// phrase their own seeded fixtures as Givens.
func (s *loadCanonicalTeamListScenarioState) aDataFileWhoseTeamsSectionHoldsASampleOfCanonicalTeams() error {
	return nil
}

// theSeededPlayerHasAValidSession is deliberately worded (and regexed)
// differently from app_shell_steps_test.go's "the player ... is signed in"
// so the two identically-shaped steps registered in the same GoDog suite
// never collide on the same step text.
func (s *loadCanonicalTeamListScenarioState) theSeededPlayerHasAValidSession(name string) error {
	if name != loadCanonicalTeamListPlayerName {
		return fmt.Errorf("no fixture for player %q; only %q is seeded", name, loadCanonicalTeamListPlayerName)
	}
	s.sessionCookie = auth.IssueSessionCookie(loadCanonicalTeamListPlayerID, testSessionSecret)
	return nil
}

func (s *loadCanonicalTeamListScenarioState) thePlayerRequestsThePredictDestination() error {
	req, err := http.NewRequest(http.MethodGet, s.server.URL+"/predict", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if s.sessionCookie != nil {
		req.AddCookie(s.sessionCookie)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("get /predict: %w", err)
	}
	defer resp.Body.Close()

	s.lastStatus = resp.StatusCode
	return nil
}

func (s *loadCanonicalTeamListScenarioState) theRequestSucceedsWithStatus(want int) error {
	if s.lastStatus != want {
		return fmt.Errorf("expected status %d, got %d", want, s.lastStatus)
	}
	return nil
}

// InitializeLoadCanonicalTeamListScenario registers the load-canonical-
// team-list step definitions with GoDog.
func InitializeLoadCanonicalTeamListScenario(ctx *godog.ScenarioContext) {
	s := newLoadCanonicalTeamListScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a data file whose teams section holds a sample of the season's canonical teams$`, s.aDataFileWhoseTeamsSectionHoldsASampleOfCanonicalTeams)
	ctx.Step(`^the seeded player "([^"]*)" has a valid session$`, s.theSeededPlayerHasAValidSession)
	ctx.Step(`^the player requests the Predict destination$`, s.thePlayerRequestsThePredictDestination)
	ctx.Step(`^the request succeeds with status (\d+)$`, s.theRequestSucceedsWithStatus)
}
