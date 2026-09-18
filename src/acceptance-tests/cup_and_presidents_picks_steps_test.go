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
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
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

// cupAndPresidentsTeamNames names the sample teams this feature's Background
// declares by id - a small fixture, not the real file's full 32-team roster
// (mirrors load_canonical_team_list_steps_test.go's own rationale).
var cupAndPresidentsTeamNames = map[string]string{
	"TOR": "Toronto Maple Leafs",
	"VGK": "Vegas Golden Knights",
}

// cupAndPresidentsPredictionSetFixture is one Given-declared Prediction Set,
// waiting to be written into the scenario's data file the first time a step
// needs a running server.
type cupAndPresidentsPredictionSetFixture struct {
	id, title, subtitle, phase string
	deadline                   time.Time
}

// cupAndPresidentsTeamFixture is one Given-declared canonical team.
type cupAndPresidentsTeamFixture struct {
	id, name, division string
}

// cupAndPresidentsPickFixture is a pick the player already made, saved via
// st.SavePrediction once the store exists (ensureReady), before the
// scenario's first request.
type cupAndPresidentsPickFixture struct {
	kind, teamID string
}

// cupAndPresidentsPicksScenarioState holds the fixtures and results for one
// cup-and-presidents-picks scenario. A fresh instance is created per
// scenario so state never leaks between runs. The store and server are
// built lazily (ensureReady) on the first request, since Given steps keep
// appending fixtures beforehand.
type cupAndPresidentsPicksScenarioState struct {
	dataFile     string
	sets         []cupAndPresidentsPredictionSetFixture
	teams        []cupAndPresidentsTeamFixture
	picks        []cupAndPresidentsPickFixture
	st           *store.Store
	server       *httptest.Server
	lastStatus   int
	lastBody     string
	lastLocation string
}

func newCupAndPresidentsPicksScenarioState() *cupAndPresidentsPicksScenarioState {
	dir, err := os.MkdirTemp("", "fantasy-hockey-cup-and-presidents-picks-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	return &cupAndPresidentsPicksScenarioState{dataFile: filepath.Join(dir, store.DataFileName)}
}

func (s *cupAndPresidentsPicksScenarioState) close() {
	if s.server != nil {
		s.server.Close()
	}
	_ = os.RemoveAll(filepath.Dir(s.dataFile)) // best-effort cleanup of the scenario's temp dir
}

func (s *cupAndPresidentsPicksScenarioState) theSignedInPlayerIs(name string) error {
	if strings.ToLower(name) != cupAndPresidentsPicksPlayerID {
		return fmt.Errorf("no fixture for player %q; only %q is seeded", name, cupAndPresidentsPicksPlayerName)
	}
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) theCanonicalTeamListIncludes(id, division string) error {
	name, ok := cupAndPresidentsTeamNames[id]
	if !ok {
		return fmt.Errorf("no known display name for team id %q", id)
	}
	s.teams = append(s.teams, cupAndPresidentsTeamFixture{id: id, name: name, division: division})
	return nil
}

func (s *cupAndPresidentsPicksScenarioState) theSetHasADeadline(id, deadlinePhrase string) error {
	base, ok := knownCupAndPresidentsPredictionSetBases[id]
	if !ok {
		return fmt.Errorf("no known base fixture for Prediction Set %q", id)
	}
	deadline, err := parseRelativeDeadline(deadlinePhrase, time.Now().UTC())
	if err != nil {
		return err
	}
	s.sets = append(s.sets, cupAndPresidentsPredictionSetFixture{
		id: id, title: base.title, subtitle: base.subtitle, phase: base.phase, deadline: deadline,
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

// ensureReady lazily persists every Given-declared Prediction Set/team,
// starts the real production web.NewServer handler around it, then saves
// every Given-declared prior pick directly through the store - the first
// time a step needs to make an HTTP call.
func (s *cupAndPresidentsPicksScenarioState) ensureReady() error {
	if s.server != nil {
		return nil
	}

	var yamlSets strings.Builder
	for _, set := range s.sets {
		fmt.Fprintf(&yamlSets, "    - id: %q\n      title: %q\n      subtitle: %q\n      deadline_utc: %q\n      phase: %q\n      upcoming: false\n",
			set.id, set.title, set.subtitle, set.deadline.Format(time.RFC3339), set.phase)
	}

	var yamlTeams strings.Builder
	for _, team := range s.teams {
		fmt.Fprintf(&yamlTeams, "    - id: %q\n      name: %q\n      conference: %q\n      division: %q\n",
			team.id, team.name, "N/A", team.division)
	}

	seed := fmt.Sprintf("season: \"2026-27\"\nplayers:\n    - id: %s\n      name: %s\n      email: basti@example.com\nprediction_sets:\n%steams:\n%s",
		cupAndPresidentsPicksPlayerID, cupAndPresidentsPicksPlayerName, yamlSets.String(), yamlTeams.String())
	if err := os.WriteFile(s.dataFile, []byte(seed), 0o600); err != nil {
		return fmt.Errorf("seed data file: %w", err)
	}

	st, err := store.New(s.dataFile)
	if err != nil {
		return fmt.Errorf("store.New: %w", err)
	}
	s.st = st

	for _, pick := range s.picks {
		if err := st.SavePrediction(cupAndPresidentsPicksPlayerID, pick.kind, pick.teamID, time.Now().UTC()); err != nil {
			return fmt.Errorf("seed prior pick: %w", err)
		}
	}

	s.server = httptest.NewServer(web.NewServer(st, noopSender, cupAndPresidentsPicksSecret))
	return nil
}

// do requests method+path carrying the signed-in player's session cookie,
// with an optional urlencoded form body, without following any redirect, and
// records the result.
func (s *cupAndPresidentsPicksScenarioState) do(method, path, body string) error {
	if err := s.ensureReady(); err != nil {
		return err
	}

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, s.server.URL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.AddCookie(auth.IssueSessionCookie(cupAndPresidentsPicksPlayerID, cupAndPresidentsPicksSecret))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	s.lastStatus = resp.StatusCode
	s.lastLocation = resp.Header.Get("Location")
	s.lastBody = string(respBody)
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

// setRowFragment isolates the single set row for id (the <a>/<div> with
// id="predict-row-{id}" up to its closing tag) so an assertion about one row
// can't accidentally match text belonging to a different row on the same
// page.
func (s *cupAndPresidentsPicksScenarioState) setRowFragment(id string) (string, error) {
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
	ctx.Step(`^the player's saved pick for "([^"]*)" is "([^"]*)"$`, s.thePlayersSavedPickForIs)
	ctx.Step(`^the player has no saved pick for "([^"]*)"$`, s.thePlayerHasNoSavedPickFor)
	ctx.Step(`^the Predict screen shows the set row for "([^"]*)" with status "([^"]*)"$`, s.thePredictScreenShowsTheSetRowWithStatus)
}
