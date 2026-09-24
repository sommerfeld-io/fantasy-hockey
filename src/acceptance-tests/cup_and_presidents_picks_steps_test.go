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

// cupAndPresidentsPicksSecret signs session cookies for this scenario's
// server - it only needs to be non-empty and stable within one scenario run.
const cupAndPresidentsPicksSecret = "cup-and-presidents-picks-test-secret"

// cupAndPresidentsPicksPlayerID/Name is the one seeded player every scenario
// in this feature signs in as.
const (
	cupAndPresidentsPicksPlayerID   = "basti"
	cupAndPresidentsPicksPlayerName = "Basti"
)

// knownCupAndPresidentsPredictionSetBases names the title/subtitle/phase for
// the two real ids this story gives a pick-entry form - a scenario only has
// to say what deadline it wants (see theSetHasADeadline), not restate every
// other hand-maintained field each time.
var knownCupAndPresidentsPredictionSetBases = map[string]struct{ title, subtitle, phase string }{
	store.KindCupChampion:      {"Cup champion", "Your Stanley Cup winner", "before_season"},
	store.KindPresidentsTrophy: {"Presidents' Trophy", "Best regular-season record", "before_season"},
}

// cupAndPresidentsPredictionSetFixture is one Given-declared Prediction Set,
// waiting to be written into the scenario's data file the first time a step
// needs a running server.
type cupAndPresidentsPredictionSetFixture struct {
	id, title, subtitle, phase string
	deadline                   time.Time
	upcoming                   bool
}

// cupAndPresidentsTeamFixture is one Given-declared canonical team.
type cupAndPresidentsTeamFixture struct {
	id, name, division string
}

// cupAndPresidentsPickFixture is a pick the player already made, saved via
// st.SavePrediction once the store exists (savePriorPicks), before the
// scenario's first request.
type cupAndPresidentsPickFixture struct {
	kind, teamID string
}

// cupAndPresidentsPicksScenarioState holds the fixtures and results for one
// cup-and-presidents-picks scenario. A fresh instance is created per
// scenario so state never leaks between runs. The store and server are
// built lazily (lazyFixture.ensureReady) on the first request, since Given
// steps keep appending fixtures beforehand.
type cupAndPresidentsPicksScenarioState struct {
	lazyFixture
	sets  []cupAndPresidentsPredictionSetFixture
	teams []cupAndPresidentsTeamFixture
	picks []cupAndPresidentsPickFixture
}

func newCupAndPresidentsPicksScenarioState() *cupAndPresidentsPicksScenarioState {
	s := &cupAndPresidentsPicksScenarioState{}
	s.lazyFixture = newLazyFixture("cup-and-presidents-picks", cupAndPresidentsPicksPlayerID, cupAndPresidentsPicksPlayerName,
		cupAndPresidentsPicksSecret, s.seedBody, s.savePriorPicks)
	return s
}

func (s *cupAndPresidentsPicksScenarioState) theCanonicalTeamListIncludes(id, division string) error {
	name, ok := sampleTeamNames[id]
	if !ok {
		return fmt.Errorf("no known display name for team id %q", id)
	}
	s.teams = append(s.teams, cupAndPresidentsTeamFixture{id: id, name: name, division: division})
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) theSetHasADeadline(id, deadlinePhrase string) error {
	return s.addKnownSet(id, deadlinePhrase, false)
}

func (s *cupAndPresidentsPicksScenarioState) theSetIsUpcomingWithADeadline(id, deadlinePhrase string) error {
	return s.addKnownSet(id, deadlinePhrase, true)
}

// addKnownSet appends one of knownCupAndPresidentsPredictionSetBases' sets
// with the given deadline and Upcoming flag.
func (s *cupAndPresidentsPicksScenarioState) addKnownSet(id, deadlinePhrase string, upcoming bool) error {
	base, ok := knownCupAndPresidentsPredictionSetBases[id]
	if !ok {
		return fmt.Errorf("no known base fixture for Prediction Set %q", id)
	}
	deadline, err := parseRelativeDeadline(deadlinePhrase, time.Now().UTC())
	if err != nil {
		return err
	}
	s.sets = append(s.sets, cupAndPresidentsPredictionSetFixture{
		id: id, title: base.title, subtitle: base.subtitle, phase: base.phase, deadline: deadline, upcoming: upcoming,
	})
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) aStubPredictionSetExists(id, title, deadlinePhrase string) error {
	deadline, err := parseRelativeDeadline(deadlinePhrase, time.Now().UTC())
	if err != nil {
		return err
	}
	s.sets = append(s.sets, cupAndPresidentsPredictionSetFixture{
		id: id, title: title, subtitle: "N/A", phase: "before_season", deadline: deadline,
	})
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePlayerAlreadyPicked(teamID, kind string) error {
	s.picks = append(s.picks, cupAndPresidentsPickFixture{kind: kind, teamID: teamID})
	return nil
}

// seedBody renders every Given-declared Prediction Set and team as the data
// file's prediction_sets and teams sections.
func (s *cupAndPresidentsPicksScenarioState) seedBody() (string, error) {
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
func (s *cupAndPresidentsPicksScenarioState) savePriorPicks(st *store.Store) error {
	for _, pick := range s.picks {
		if err := st.SavePrediction(cupAndPresidentsPicksPlayerID, pick.kind, pick.teamID, time.Now().UTC()); err != nil {
			return fmt.Errorf("seed prior pick: %w", err)
		}
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePlayerOpensThePredictionSet(id string) error {
	return s.do(http.MethodGet, "/predict/"+id, "")
}

func (s *cupAndPresidentsPicksScenarioState) thePlayerSubmitsTeamForThePredictionSet(teamID, id string) error {
	return s.do(http.MethodPost, "/predict/"+id, "team_id="+teamID)
}

func (s *cupAndPresidentsPicksScenarioState) thePickSheetShowsATeamDropdownGroupedByDivision() error {
	if !strings.Contains(s.lastBody, `<select name="team_id"`) || !strings.Contains(s.lastBody, "<optgroup label=") {
		return fmt.Errorf("expected a team dropdown grouped by division, got %q", s.lastBody)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePickSheetShowsNoTeamPreselected() error {
	if !strings.Contains(s.lastBody, `<option value="" disabled selected>`) {
		return fmt.Errorf("expected no team preselected, got %q", s.lastBody)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePickSheetShowsTheButtonText(text string) error {
	want := ">" + text + "</button>"
	if !strings.Contains(s.lastBody, want) {
		return fmt.Errorf("expected the button text %q, got %q", text, s.lastBody)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePickSheetShowsTheTeamPreselected(teamID string) error {
	want := `value="` + teamID + `" selected>`
	if !strings.Contains(s.lastBody, want) {
		return fmt.Errorf("expected team %q preselected, got %q", teamID, s.lastBody)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePickSheetShowsAReadOnlyBanner() error {
	if !strings.Contains(s.lastBody, "closed-banner") {
		return fmt.Errorf("expected a read-only banner, got %q", s.lastBody)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePickSheetShowsNoSubmitButton() error {
	if strings.Contains(s.lastBody, "<button") {
		return fmt.Errorf("expected no submit button, got %q", s.lastBody)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePickSheetShowsNoTeamDropdown() error {
	if strings.Contains(s.lastBody, `<select name="team_id"`) {
		return fmt.Errorf("expected no team dropdown, got %q", s.lastBody)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePickSheetShows(text string) error {
	if !strings.Contains(s.lastBody, text) {
		return fmt.Errorf("expected the pick sheet to show %q, got %q", text, s.lastBody)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePickResponseRedirectsTo(location string) error {
	if s.lastStatus != http.StatusFound {
		return fmt.Errorf("expected status %d, got %d", http.StatusFound, s.lastStatus)
	}
	if s.lastLocation != location {
		return fmt.Errorf("expected a redirect to %q, got %q", location, s.lastLocation)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePickResponseStatusIs(want int) error {
	if s.lastStatus != want {
		return fmt.Errorf("expected status %d, got %d", want, s.lastStatus)
	}
	return nil
}

// genericNotFoundBody is net/http's own http.NotFound body - the same
// response an unknown path or Prediction Set id gets, so it reveals nothing
// about the set.
const genericNotFoundBody = "404 page not found"

func (s *cupAndPresidentsPicksScenarioState) thePickResponseShowsTheGenericNotFoundBody() error {
	if strings.TrimSpace(s.lastBody) != genericNotFoundBody {
		return fmt.Errorf("expected the generic not-found body %q, got %q", genericNotFoundBody, s.lastBody)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePlayersSavedPickForIs(kind, teamID string) error {
	prediction, ok := s.st.FindPrediction(cupAndPresidentsPicksPlayerID, kind)
	if !ok {
		return fmt.Errorf("expected a saved pick for %q, found none", kind)
	}
	if prediction.TeamID != teamID {
		return fmt.Errorf("expected the saved pick for %q to be %q, got %q", kind, teamID, prediction.TeamID)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePlayerHasNoSavedPickFor(kind string) error {
	if _, ok := s.st.FindPrediction(cupAndPresidentsPicksPlayerID, kind); ok {
		return fmt.Errorf("expected no saved pick for %q, found one", kind)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) thePredictScreenShowsTheSetRowWithStatus(id, status string) error {
	if err := s.do(http.MethodGet, "/predict", ""); err != nil {
		return err
	}
	row, err := s.setRowFragment(id)
	if err != nil {
		return err
	}
	want := ">" + status + "</span>"
	if !strings.Contains(row, want) {
		return fmt.Errorf("expected the set row for %q to show status %q, got %q", id, status, row)
	}
	return nil
}

// InitializeCupAndPresidentsPicksScenario registers the
// cup-and-presidents-picks step definitions with GoDog.
func InitializeCupAndPresidentsPicksScenario(ctx *godog.ScenarioContext) {
	s := newCupAndPresidentsPicksScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the signed-in player for cup and presidents picks is "([^"]*)"$`, s.theSignedInPlayerIs)
	ctx.Step(`^the canonical team list includes "([^"]*)" in the "([^"]*)" division$`, s.theCanonicalTeamListIncludes)
	ctx.Step(`^the "([^"]*)" Prediction Set has a deadline "([^"]*)"$`, s.theSetHasADeadline)
	ctx.Step(`^the "([^"]*)" Prediction Set is upcoming with a deadline "([^"]*)"$`, s.theSetIsUpcomingWithADeadline)
	ctx.Step(`^a stub Prediction Set "([^"]*)" titled "([^"]*)" with a deadline "([^"]*)"$`, s.aStubPredictionSetExists)
	ctx.Step(`^the player already picked "([^"]*)" for "([^"]*)"$`, s.thePlayerAlreadyPicked)
	ctx.Step(`^the player opens the cup-picks Prediction Set "([^"]*)"$`, s.thePlayerOpensThePredictionSet)
	ctx.Step(`^the player submits team "([^"]*)" for the Prediction Set "([^"]*)"$`, s.thePlayerSubmitsTeamForThePredictionSet)
	ctx.Step(`^the pick sheet shows a team dropdown grouped by division$`, s.thePickSheetShowsATeamDropdownGroupedByDivision)
	ctx.Step(`^the pick sheet shows no team preselected$`, s.thePickSheetShowsNoTeamPreselected)
	ctx.Step(`^the pick sheet shows the button text "([^"]*)"$`, s.thePickSheetShowsTheButtonText)
	ctx.Step(`^the pick sheet shows the team "([^"]*)" preselected$`, s.thePickSheetShowsTheTeamPreselected)
	ctx.Step(`^the pick sheet shows a read-only banner$`, s.thePickSheetShowsAReadOnlyBanner)
	ctx.Step(`^the pick sheet shows no submit button$`, s.thePickSheetShowsNoSubmitButton)
	ctx.Step(`^the pick sheet shows no team dropdown$`, s.thePickSheetShowsNoTeamDropdown)
	ctx.Step(`^the pick sheet shows "([^"]*)"$`, s.thePickSheetShows)
	ctx.Step(`^the pick response redirects to "([^"]*)"$`, s.thePickResponseRedirectsTo)
	ctx.Step(`^the pick response status is (\d+)$`, s.thePickResponseStatusIs)
	ctx.Step(`^the pick response shows the generic not-found body$`, s.thePickResponseShowsTheGenericNotFoundBody)
	ctx.Step(`^the player's saved pick for "([^"]*)" is "([^"]*)"$`, s.thePlayersSavedPickForIs)
	ctx.Step(`^the player has no saved pick for "([^"]*)"$`, s.thePlayerHasNoSavedPickFor)
	ctx.Step(`^the Predict screen shows the set row for "([^"]*)" with status "([^"]*)"$`, s.thePredictScreenShowsTheSetRowWithStatus)
}
