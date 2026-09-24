package acceptance_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// playoffsCupPickSecret signs session cookies for this scenario's server - it
// only needs to be non-empty and stable within one scenario run.
const playoffsCupPickSecret = "playoffs-cup-pick-test-secret"

// playoffsCupPickPlayerID/Name is the one seeded player every scenario in
// this feature signs in as.
const (
	playoffsCupPickPlayerID   = "basti"
	playoffsCupPickPlayerName = "Basti"
)

// playoffsCupPickTeamNames names the sample teams this feature's Background
// declares by id - a small fixture, not the real file's full 32-team roster
// (mirrors cup_and_presidents_picks_steps_test.go's own rationale).
var playoffsCupPickTeamNames = map[string]string{
	"TOR": "Toronto Maple Leafs",
	"VGK": "Vegas Golden Knights",
}

// playoffsCupPickSetFixture is one Given-declared Prediction Set (either the
// "playoffcup" set itself or, for the independence scenario, the
// season-opening "cup" set), waiting to be written into the scenario's data
// file the first time a step needs a running server.
type playoffsCupPickSetFixture struct {
	id, title, subtitle, phase string
	deadline                   time.Time
	upcoming                   bool
}

// playoffsCupPickTeamFixture is one Given-declared canonical team.
type playoffsCupPickTeamFixture struct {
	id, name, division string
}

// playoffsCupPickPriorPick is a pick the player already made, saved via
// st.SavePrediction once the store exists (savePriorPicks), before the
// scenario's first request.
type playoffsCupPickPriorPick struct {
	kind, teamID string
}

// playoffsCupPickScenarioState holds the fixtures and results for one
// playoffs-cup-pick scenario. A fresh instance is created per scenario so
// state never leaks between runs. The store and server are built lazily
// (lazyFixture.ensureReady) on the first request, since Given steps keep
// appending fixtures beforehand.
type playoffsCupPickScenarioState struct {
	lazyFixture
	sets  []playoffsCupPickSetFixture
	teams []playoffsCupPickTeamFixture
	picks []playoffsCupPickPriorPick
}

func newPlayoffsCupPickScenarioState() *playoffsCupPickScenarioState {
	s := &playoffsCupPickScenarioState{}
	s.lazyFixture = newLazyFixture("playoffs-cup-pick", playoffsCupPickPlayerID, playoffsCupPickPlayerName,
		playoffsCupPickSecret, s.seedBody, s.savePriorPicks)
	return s
}

func (s *playoffsCupPickScenarioState) theSignedInPlayerIs(name string) error {
	if strings.ToLower(name) != playoffsCupPickPlayerID {
		return fmt.Errorf("no fixture for player %q; only %q is seeded", name, playoffsCupPickPlayerName)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) theCanonicalTeamListIncludes(id, division string) error {
	name, ok := playoffsCupPickTeamNames[id]
	if !ok {
		return fmt.Errorf("no known display name for team id %q", id)
	}
	s.teams = append(s.teams, playoffsCupPickTeamFixture{id: id, name: name, division: division})
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsCupPickHasADeadline(deadlinePhrase string) error {
	return s.addSet(store.KindPlayoffsCup, "Playoffs Cup pick", "Re-pick the Stanley Cup winner", "playoffs", deadlinePhrase, false)
}

func (s *playoffsCupPickScenarioState) thePlayoffsCupPickIsUpcomingWithADeadline(deadlinePhrase string) error {
	return s.addSet(store.KindPlayoffsCup, "Playoffs Cup pick", "Re-pick the Stanley Cup winner", "playoffs", deadlinePhrase, true)
}

func (s *playoffsCupPickScenarioState) theSeasonCupPickHasADeadline(deadlinePhrase string) error {
	return s.addSet(store.KindCupChampion, "Cup champion", "Your Stanley Cup winner", "before_season", deadlinePhrase, false)
}

// addSet appends one Prediction Set fixture with the given deadline phrase,
// resolved relative to now, and Upcoming flag.
func (s *playoffsCupPickScenarioState) addSet(id, title, subtitle, phase, deadlinePhrase string, upcoming bool) error {
	deadline, err := parseRelativeDeadline(deadlinePhrase, time.Now().UTC())
	if err != nil {
		return err
	}
	s.sets = append(s.sets, playoffsCupPickSetFixture{id: id, title: title, subtitle: subtitle, phase: phase, deadline: deadline, upcoming: upcoming})
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayerAlreadyPickedForThePlayoffsCupPick(teamID string) error {
	s.picks = append(s.picks, playoffsCupPickPriorPick{kind: store.KindPlayoffsCup, teamID: teamID})
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayerAlreadyPickedForTheSeasonCupPick(teamID string) error {
	s.picks = append(s.picks, playoffsCupPickPriorPick{kind: store.KindCupChampion, teamID: teamID})
	return nil
}

// seedBody renders every Given-declared Prediction Set and team as the data
// file's prediction_sets and teams sections.
func (s *playoffsCupPickScenarioState) seedBody() (string, error) {
	var yamlSets strings.Builder
	for _, set := range s.sets {
		fmt.Fprintf(&yamlSets, "    - id: %q\n      title: %q\n      subtitle: %q\n      deadline_utc: %q\n      phase: %q\n      upcoming: %t\n",
			set.id, set.title, set.subtitle, set.deadline.Format(time.RFC3339), set.phase, set.upcoming)
	}

	var yamlTeams strings.Builder
	for _, team := range s.teams {
		fmt.Fprintf(&yamlTeams, "    - id: %q\n      name: %q\n      conference: %q\n      division: %q\n",
			team.id, team.name, "N/A", team.division)
	}

	return "prediction_sets:\n" + yamlSets.String() + "teams:\n" + yamlTeams.String(), nil
}

// savePriorPicks saves every Given-declared prior pick directly through the
// store, before the scenario's first request.
func (s *playoffsCupPickScenarioState) savePriorPicks(st *store.Store) error {
	for _, pick := range s.picks {
		if err := st.SavePrediction(playoffsCupPickPlayerID, pick.kind, pick.teamID, time.Now().UTC()); err != nil {
			return fmt.Errorf("seed prior pick: %w", err)
		}
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayerOpensThePlayoffsCupPickSheet() error {
	return s.do(http.MethodGet, "/predict/"+store.KindPlayoffsCup, "")
}

func (s *playoffsCupPickScenarioState) thePlayerSubmitsTeamForThePlayoffsCupPick(teamID string) error {
	return s.do(http.MethodPost, "/predict/"+store.KindPlayoffsCup, "team_id="+teamID)
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickSheetShowsATeamDropdownGroupedByDivision() error {
	if !strings.Contains(s.lastBody, `<select name="team_id"`) || !strings.Contains(s.lastBody, "<optgroup label=") {
		return fmt.Errorf("expected a team dropdown grouped by division, got %q", s.lastBody)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickSheetShowsNoTeamPreselected() error {
	if !strings.Contains(s.lastBody, `<option value="" disabled selected>`) {
		return fmt.Errorf("expected no team preselected, got %q", s.lastBody)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickSheetShowsTheButtonText(text string) error {
	want := ">" + text + "</button>"
	if !strings.Contains(s.lastBody, want) {
		return fmt.Errorf("expected the button text %q, got %q", text, s.lastBody)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickSheetShowsTheTeamPreselected(teamID string) error {
	want := `value="` + teamID + `" selected>`
	if !strings.Contains(s.lastBody, want) {
		return fmt.Errorf("expected team %q preselected, got %q", teamID, s.lastBody)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickSheetShows(text string) error {
	if !strings.Contains(s.lastBody, text) {
		return fmt.Errorf("expected the pick sheet to show %q, got %q", text, s.lastBody)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickSheetShowsAReadOnlyBanner() error {
	if !strings.Contains(s.lastBody, "closed-banner") {
		return fmt.Errorf("expected a read-only banner, got %q", s.lastBody)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickSheetShowsNoSubmitButton() error {
	if strings.Contains(s.lastBody, "<button") {
		return fmt.Errorf("expected no submit button, got %q", s.lastBody)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickResponseShowsTheGenericNotFoundBody() error {
	if strings.TrimSpace(s.lastBody) != genericNotFoundBody {
		return fmt.Errorf("expected the generic not-found body %q, got %q", genericNotFoundBody, s.lastBody)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickResponseRedirectsTo(location string) error {
	if s.lastStatus != http.StatusFound {
		return fmt.Errorf("expected status %d, got %d", http.StatusFound, s.lastStatus)
	}
	if s.lastLocation != location {
		return fmt.Errorf("expected a redirect to %q, got %q", location, s.lastLocation)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayoffsPickResponseStatusIs(want int) error {
	if s.lastStatus != want {
		return fmt.Errorf("expected status %d, got %d", want, s.lastStatus)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayersSavedPlayoffsCupPickIs(teamID string) error {
	prediction, ok := s.st.FindPrediction(playoffsCupPickPlayerID, store.KindPlayoffsCup)
	if !ok {
		return fmt.Errorf("expected a saved Playoffs Cup pick, found none")
	}
	if prediction.TeamID != teamID {
		return fmt.Errorf("expected the saved Playoffs Cup pick to be %q, got %q", teamID, prediction.TeamID)
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayerHasNoSavedPlayoffsCupPick() error {
	if _, ok := s.st.FindPrediction(playoffsCupPickPlayerID, store.KindPlayoffsCup); ok {
		return fmt.Errorf("expected no saved Playoffs Cup pick, found one")
	}
	return nil
}

func (s *playoffsCupPickScenarioState) thePlayersSavedSeasonCupPickIsStill(teamID string) error {
	prediction, ok := s.st.FindPrediction(playoffsCupPickPlayerID, store.KindCupChampion)
	if !ok {
		return fmt.Errorf("expected the season-opening Cup champion pick to still be saved, found none")
	}
	if prediction.TeamID != teamID {
		return fmt.Errorf("expected the season-opening Cup champion pick to still be %q, got %q", teamID, prediction.TeamID)
	}
	return nil
}

// setRowFragment isolates the single set row for id (the <a>/<div> with
// id="predict-row-{id}" up to its closing tag) so an assertion about one row
// can't accidentally match text belonging to a different row on the same
// page.
func (s *playoffsCupPickScenarioState) setRowFragment(id string) (string, error) {
	marker := `id="predict-row-` + id + `"`
	start := strings.Index(s.lastBody, marker)
	if start == -1 {
		return "", fmt.Errorf("expected a set row for %q, got %q", id, s.lastBody)
	}
	rest := s.lastBody[start:]
	end := strings.Index(rest, "</a>")
	if divEnd := strings.Index(rest, "</div>"); end == -1 || (divEnd != -1 && divEnd < end) {
		end = divEnd
	}
	if end == -1 {
		return "", fmt.Errorf("could not find the end of the set row for %q", id)
	}
	return rest[:end], nil
}

func (s *playoffsCupPickScenarioState) thePredictScreenShowsThePlayoffsCupSetRowWithStatus(status string) error {
	if err := s.do(http.MethodGet, "/predict", ""); err != nil {
		return err
	}
	row, err := s.setRowFragment(store.KindPlayoffsCup)
	if err != nil {
		return err
	}
	want := ">" + status + "</span>"
	if !strings.Contains(row, want) {
		return fmt.Errorf("expected the Playoffs Cup set row to show status %q, got %q", status, row)
	}
	return nil
}

// InitializePlayoffsCupPickScenario registers the playoffs-cup-pick step
// definitions with GoDog.
func InitializePlayoffsCupPickScenario(ctx *godog.ScenarioContext) {
	s := newPlayoffsCupPickScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the signed-in player for the playoffs Cup pick is "([^"]*)"$`, s.theSignedInPlayerIs)
	ctx.Step(`^the canonical team list for the playoffs Cup pick includes "([^"]*)" in the "([^"]*)" division$`, s.theCanonicalTeamListIncludes)
	ctx.Step(`^the Playoffs Cup pick has a deadline "([^"]*)"$`, s.thePlayoffsCupPickHasADeadline)
	ctx.Step(`^the Playoffs Cup pick is upcoming with a deadline "([^"]*)"$`, s.thePlayoffsCupPickIsUpcomingWithADeadline)
	ctx.Step(`^the season-opening Cup champion pick has a deadline "([^"]*)"$`, s.theSeasonCupPickHasADeadline)
	ctx.Step(`^the player already picked "([^"]*)" for the Playoffs Cup pick$`, s.thePlayerAlreadyPickedForThePlayoffsCupPick)
	ctx.Step(`^the player already picked "([^"]*)" for the season-opening Cup champion pick$`, s.thePlayerAlreadyPickedForTheSeasonCupPick)
	ctx.Step(`^the player opens the Playoffs Cup pick sheet$`, s.thePlayerOpensThePlayoffsCupPickSheet)
	ctx.Step(`^the player submits team "([^"]*)" for the Playoffs Cup pick$`, s.thePlayerSubmitsTeamForThePlayoffsCupPick)
	ctx.Step(`^the playoffs pick sheet shows a team dropdown grouped by division$`, s.thePlayoffsPickSheetShowsATeamDropdownGroupedByDivision)
	ctx.Step(`^the playoffs pick sheet shows no team preselected$`, s.thePlayoffsPickSheetShowsNoTeamPreselected)
	ctx.Step(`^the playoffs pick sheet shows the button text "([^"]*)"$`, s.thePlayoffsPickSheetShowsTheButtonText)
	ctx.Step(`^the playoffs pick sheet shows the team "([^"]*)" preselected$`, s.thePlayoffsPickSheetShowsTheTeamPreselected)
	ctx.Step(`^the playoffs pick sheet shows "([^"]*)"$`, s.thePlayoffsPickSheetShows)
	ctx.Step(`^the playoffs pick sheet shows a read-only banner$`, s.thePlayoffsPickSheetShowsAReadOnlyBanner)
	ctx.Step(`^the playoffs pick sheet shows no submit button$`, s.thePlayoffsPickSheetShowsNoSubmitButton)
	ctx.Step(`^the playoffs pick response redirects to "([^"]*)"$`, s.thePlayoffsPickResponseRedirectsTo)
	ctx.Step(`^the playoffs pick response status is (\d+)$`, s.thePlayoffsPickResponseStatusIs)
	ctx.Step(`^the playoffs pick response shows the generic not-found body$`, s.thePlayoffsPickResponseShowsTheGenericNotFoundBody)
	ctx.Step(`^the player's saved Playoffs Cup pick is "([^"]*)"$`, s.thePlayersSavedPlayoffsCupPickIs)
	ctx.Step(`^the player has no saved Playoffs Cup pick$`, s.thePlayerHasNoSavedPlayoffsCupPick)
	ctx.Step(`^the player's saved season-opening Cup champion pick is still "([^"]*)"$`, s.thePlayersSavedSeasonCupPickIsStill)
	ctx.Step(`^the Predict screen shows the playoffs Cup set row with status "([^"]*)"$`, s.thePredictScreenShowsThePlayoffsCupSetRowWithStatus)
}
