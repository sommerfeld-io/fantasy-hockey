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

// loadCanonicalNHLPlayerListPlayerID/Name mirror
// load_canonical_team_list_steps_test.go's own seeded fixture player.
const (
	loadCanonicalNHLPlayerListPlayerID   = "basti"
	loadCanonicalNHLPlayerListPlayerName = "Basti"
)

// loadCanonicalNHLPlayerListSeed is a full fantasy-hockey.yml document: one
// seeded player plus a small representative sample of NHL Players spanning
// all three positions - deliberately not an exhaustive roster, so this
// fixture can't silently drift out of sync with fantasy-hockey.yml's own
// nhl_players: section as that list is hand-edited over the season. This
// scenario only proves a well-formed nhl_players section doesn't break
// store.New's parsing or the Predict screen's rendering - it never asserts
// on the players themselves, since no UI reads Store.NHLPlayers() yet
// (Story 2.6 wires it into the awards autocomplete).
const loadCanonicalNHLPlayerListSeed = `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
nhl_players:
    - slug: mcdavid-connor
      display_name: Connor McDavid
      position: skater
    - slug: hughes-quinn
      display_name: Quinn Hughes
      position: defenseman
    - slug: hellebuyck-connor
      display_name: Connor Hellebuyck
      position: goalie
`

// loadCanonicalNHLPlayerListScenarioState holds the fixtures and last
// response for one scenario. A fresh instance is created per scenario so
// state never leaks between runs.
type loadCanonicalNHLPlayerListScenarioState struct {
	dataFile      string
	server        *httptest.Server
	sessionCookie *http.Cookie
	lastStatus    int
}

func newLoadCanonicalNHLPlayerListScenarioState() *loadCanonicalNHLPlayerListScenarioState {
	st, dataFile := newSeededStore("load-canonical-nhl-player-list", loadCanonicalNHLPlayerListSeed)
	return &loadCanonicalNHLPlayerListScenarioState{
		dataFile: dataFile,
		server:   httptest.NewServer(web.NewServer(st, noopSender, testSessionSecret)),
	}
}

func (s *loadCanonicalNHLPlayerListScenarioState) close() {
	s.server.Close()
	removeScenarioDataFile(s.dataFile)
}

// aDataFileWhoseNHLPlayersSectionHoldsASampleOfPositions is a no-op: the
// scenario's server is already wired against loadCanonicalNHLPlayerListSeed
// by newLoadCanonicalNHLPlayerListScenarioState. The step exists so the
// Gherkin reads as a Given on the data file's shape, matching how other
// features phrase their own seeded fixtures as Givens.
func (s *loadCanonicalNHLPlayerListScenarioState) aDataFileWhoseNHLPlayersSectionHoldsASampleOfPositions() error {
	return nil
}

// theSeededPlayerIsLoggedInForThisScenario is deliberately worded (and
// regexed) differently from load_canonical_team_list_steps_test.go's own
// "the seeded player ... has a valid session" and app_shell_steps_test.go's
// "the player ... is signed in" so none of the identically-shaped steps
// registered in the same GoDog suite ever collide on the same step text.
func (s *loadCanonicalNHLPlayerListScenarioState) theSeededPlayerIsLoggedInForThisScenario(name string) error {
	if err := requireSeededPlayer(name, loadCanonicalNHLPlayerListPlayerName); err != nil {
		return err
	}
	s.sessionCookie = auth.IssueSessionCookie(loadCanonicalNHLPlayerListPlayerID, testSessionSecret)
	return nil
}

func (s *loadCanonicalNHLPlayerListScenarioState) theLoggedInPlayerOpensThePredictDestination() error {
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

// thePredictScreenStillLoadsWithStatus asserts on the generic Predict
// destination's response status set by
// theLoggedInPlayerOpensThePredictDestination - this scenario re-requests
// the same /predict route load-canonical-team-list.feature already covers
// (there's no NHL-player-list-specific endpoint), only proving the seeded
// nhl_players: section didn't break its rendering.
func (s *loadCanonicalNHLPlayerListScenarioState) thePredictScreenStillLoadsWithStatus(want int) error {
	if s.lastStatus != want {
		return fmt.Errorf("expected status %d, got %d", want, s.lastStatus)
	}
	return nil
}

// InitializeLoadCanonicalNHLPlayerListScenario registers the load-canonical-
// nhl-player-list step definitions with GoDog.
func InitializeLoadCanonicalNHLPlayerListScenario(ctx *godog.ScenarioContext) {
	s := newLoadCanonicalNHLPlayerListScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a data file whose nhl_players section holds a sample of skaters, defensemen, and goalies$`, s.aDataFileWhoseNHLPlayersSectionHoldsASampleOfPositions)
	ctx.Step(`^the seeded player "([^"]*)" is logged in for this scenario$`, s.theSeededPlayerIsLoggedInForThisScenario)
	ctx.Step(`^the logged-in player opens the Predict destination$`, s.theLoggedInPlayerOpensThePredictDestination)
	ctx.Step(`^the Predict screen still loads successfully with status (\d+)$`, s.thePredictScreenStillLoadsWithStatus)
}
