package acceptance_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// divisionPicksSecret signs session cookies for this scenario's server - it
// only needs to be non-empty and stable within one scenario run.
const divisionPicksSecret = "division-picks-test-secret"

// divisionPicksPlayerID/Name is the one seeded player every scenario in this
// feature signs in as.
const (
	divisionPicksPlayerID   = "basti"
	divisionPicksPlayerName = "Basti"
)

// divisionPicksTeamsByDivision is the full 32-team roster this feature's
// Background declares - real NHL team ids grouped by division, matching the
// reference App.jsx's own DIVISIONS constant. A small 1-team-per-division
// sample (like cup_and_presidents_picks_steps_test.go's own fixture) can't
// exercise this form's own caps, which depend on a division actually being
// able to reach 5 and a conference actually being able to reach 8.
var divisionPicksTeamsByDivision = map[string][]string{
	"Atlantic":     {"BOS", "BUF", "DET", "FLA", "MTL", "OTT", "TBL", "TOR"},
	"Metropolitan": {"CAR", "CBJ", "NJD", "NYI", "NYR", "PHI", "PIT", "WSH"},
	"Central":      {"CHI", "COL", "DAL", "MIN", "NSH", "STL", "UTA", "WPG"},
	"Pacific":      {"ANA", "CGY", "EDM", "LAK", "SJS", "SEA", "VAN", "VGK"},
}

// divisionPicksConferenceByDivision names each division's own conference,
// matching each seeded team's own Team.Conference field - used only to seed
// the fixture; production code never hardcodes this mapping (see
// internal/web's groupDivisionsByConference).
var divisionPicksConferenceByDivision = map[string]string{
	"Atlantic":     "Eastern",
	"Metropolitan": "Eastern",
	"Central":      "Western",
	"Pacific":      "Western",
}

// validDivisionPicksFixture is one valid, 4/4-split submission across both
// conferences (Atlantic+Metropolitan == 8, Central+Pacific == 8).
func validDivisionPicksFixture() map[string][]string {
	return map[string][]string{
		"Atlantic":     {"BOS", "BUF", "DET", "FLA"},
		"Metropolitan": {"CAR", "CBJ", "NJD", "NYI"},
		"Central":      {"CHI", "COL", "DAL", "MIN"},
		"Pacific":      {"ANA", "CGY", "EDM", "LAK"},
	}
}

// validDivisionWinnersFixture is one winner pick per division, each
// belonging to that division's own roster.
func validDivisionWinnersFixture() map[string]string {
	return map[string]string{
		"Atlantic":     "BOS",
		"Metropolitan": "CAR",
		"Central":      "CHI",
		"Pacific":      "ANA",
	}
}

// divisionPlayoffTeamsFieldNameFixture/divisionWinnerFieldNameFixture mirror
// internal/web's own (unexported) divisionPlayoffTeamsFieldName/
// divisionWinnerFieldName naming convention, so this fixture posts to the
// exact same form field names the real sheet renders.
func divisionPlayoffTeamsFieldNameFixture(division string) string {
	return "playoff_teams_" + strings.ToLower(division)
}

func divisionWinnerFieldNameFixture(division string) string {
	return "winner_" + strings.ToLower(division)
}

// divisionPicksSetFixture is the Given-declared "divisions" Prediction Set,
// waiting to be written into the scenario's data file the first time a step
// needs a running server.
type divisionPicksSetFixture struct {
	deadline time.Time
}

// divisionPicksScenarioState holds the fixtures and results for one
// division-picks scenario. A fresh instance is created per scenario so state
// never leaks between runs. The store and server are built lazily
// (lazyFixture.ensureReady) on the first request, since Given steps keep
// appending fixtures beforehand.
type divisionPicksScenarioState struct {
	lazyFixture
	set         *divisionPicksSetFixture
	priorSubmit bool
}

func newDivisionPicksScenarioState() *divisionPicksScenarioState {
	s := &divisionPicksScenarioState{}
	s.lazyFixture = newLazyFixture("division-picks", divisionPicksPlayerID, divisionPicksPlayerName,
		divisionPicksSecret, s.seedBody, s.savePriorSubmission)
	return s
}

func (s *divisionPicksScenarioState) theSignedInPlayerIs(name string) error {
	if strings.ToLower(name) != divisionPicksPlayerID {
		return fmt.Errorf("no fixture for player %q; only %q is seeded", name, divisionPicksPlayerName)
	}
	return nil
}

// theCanonicalDivisionTeamListIncludesTheFullRoster is a no-op: ensureReady
// always seeds divisionPicksTeamsByDivision's full 32-team roster regardless
// - this step exists only so the Background reads clearly as a Given.
func (s *divisionPicksScenarioState) theCanonicalDivisionTeamListIncludesTheFullRoster() error {
	return nil
}

func (s *divisionPicksScenarioState) theDivisionsSetHasADeadline(deadlinePhrase string) error {
	deadline, err := parseRelativeDeadline(deadlinePhrase, time.Now().UTC())
	if err != nil {
		return err
	}
	s.set = &divisionPicksSetFixture{deadline: deadline}
	return nil
}

func (s *divisionPicksScenarioState) thePlayerAlreadySubmittedAValid88SplitWithAllFourWinners() error {
	s.priorSubmit = true
	return nil
}

// seedBody renders the Given-declared "divisions" Prediction Set and the
// full team roster as the data file's prediction_sets and teams sections.
func (s *divisionPicksScenarioState) seedBody() (string, error) {
	if s.set == nil {
		return "", fmt.Errorf("no divisions Prediction Set deadline declared")
	}

	var yamlTeams strings.Builder
	for _, division := range store.Divisions() {
		teamIDs, ok := divisionPicksTeamsByDivision[division]
		if !ok {
			return "", fmt.Errorf("no fixture teams for division %q - divisionPicksTeamsByDivision has drifted from store.Divisions()", division)
		}
		conference, ok := divisionPicksConferenceByDivision[division]
		if !ok {
			return "", fmt.Errorf("no fixture conference for division %q - divisionPicksConferenceByDivision has drifted from store.Divisions()", division)
		}
		for _, id := range teamIDs {
			fmt.Fprintf(&yamlTeams, "    - id: %q\n      name: %q\n      conference: %q\n      division: %q\n",
				id, id+" Team", conference, division)
		}
	}

	return fmt.Sprintf("prediction_sets:\n    - id: divisions\n      title: \"Division picks\"\n      subtitle: \"Playoff teams & division winners\"\n      deadline_utc: %q\n      phase: before_season\n      upcoming: false\nteams:\n%s",
		s.set.deadline.Format(time.RFC3339), yamlTeams.String()), nil
}

// savePriorSubmission saves the Background's prior submission (if one was
// declared) directly through the store, before the scenario's first request.
func (s *divisionPicksScenarioState) savePriorSubmission(st *store.Store) error {
	if !s.priorSubmit {
		return nil
	}
	if err := st.SaveDivisionPicks(divisionPicksPlayerID, validDivisionPicksFixture(), validDivisionWinnersFixture(), time.Now().UTC()); err != nil {
		return fmt.Errorf("seed prior division picks: %w", err)
	}
	return nil
}

func (s *divisionPicksScenarioState) thePlayerOpensTheDivisionsPredictionSet() error {
	return s.do(http.MethodGet, "/predict/divisions", "")
}

// postDivisionsForm encodes playoffTeams/winners into the same field names
// the real divisions sheet renders and POSTs them.
func (s *divisionPicksScenarioState) postDivisionsForm(playoffTeams map[string][]string, winners map[string]string) error {
	values := url.Values{}
	for division, teamIDs := range playoffTeams {
		field := divisionPlayoffTeamsFieldNameFixture(division)
		for _, id := range teamIDs {
			values.Add(field, id)
		}
	}
	for division, teamID := range winners {
		values.Set(divisionWinnerFieldNameFixture(division), teamID)
	}
	return s.do(http.MethodPost, "/predict/divisions", values.Encode())
}

func (s *divisionPicksScenarioState) thePlayerSubmitsAValid88SplitWithAllFourWinners() error {
	return s.postDivisionsForm(validDivisionPicksFixture(), validDivisionWinnersFixture())
}

// thePlayerSubmitsAValid53SplitWithAllFourWinners covers the other half of
// "either a 4/4 split, or 5 in one division and 3 in the other" - every
// other submission in this feature exercises a 4/4 split, so this proves
// the 5-team boundary (exactly the per-division cap) is accepted.
func (s *divisionPicksScenarioState) thePlayerSubmitsAValid53SplitWithAllFourWinners() error {
	picks := validDivisionPicksFixture()
	picks["Atlantic"] = append(picks["Atlantic"], "MTL") // 5 teams (at the cap).
	picks["Metropolitan"] = picks["Metropolitan"][:3]    // 3 teams, so Eastern still nets exactly 8.
	return s.postDivisionsForm(picks, validDivisionWinnersFixture())
}

func (s *divisionPicksScenarioState) thePlayerSubmitsASplitWhereTheEasternConferenceTotals7() error {
	picks := validDivisionPicksFixture()
	picks["Atlantic"] = picks["Atlantic"][:3] // Eastern (Atlantic+Metropolitan) now totals 3+4=7.
	return s.postDivisionsForm(picks, validDivisionWinnersFixture())
}

func (s *divisionPicksScenarioState) thePlayerSubmitsASplitWhereAtlanticHolds6TeamsAndMetropolitanHolds2() error {
	picks := validDivisionPicksFixture()
	picks["Atlantic"] = append(picks["Atlantic"], "MTL", "OTT") // 6, over its own 5-team cap.
	picks["Metropolitan"] = picks["Metropolitan"][:2]           // 2, so Eastern still nets exactly 8.
	return s.postDivisionsForm(picks, validDivisionWinnersFixture())
}

func (s *divisionPicksScenarioState) thePlayerSubmitsASplitWhereAMetropolitanTeamIsCheckedUnderAtlantic() error {
	picks := validDivisionPicksFixture()
	picks["Atlantic"] = append(picks["Atlantic"][:3], "WSH") // WSH belongs to Metropolitan, not Atlantic.
	return s.postDivisionsForm(picks, validDivisionWinnersFixture())
}

func (s *divisionPicksScenarioState) thePlayerSubmitsAValid88SplitWithThePacificWinnerLeftBlank() error {
	winners := validDivisionWinnersFixture()
	winners["Pacific"] = ""
	return s.postDivisionsForm(validDivisionPicksFixture(), winners)
}

func (s *divisionPicksScenarioState) thePlayerSubmitsAValid88SplitWithAMetropolitanTeamAsTheAtlanticWinner() error {
	winners := validDivisionWinnersFixture()
	winners["Atlantic"] = "WSH" // WSH belongs to Metropolitan, not Atlantic.
	return s.postDivisionsForm(validDivisionPicksFixture(), winners)
}

func (s *divisionPicksScenarioState) theDivisionsSheetShowsFourDivisionChipGroups() error {
	for _, division := range store.Divisions() {
		if !strings.Contains(s.lastBody, `data-division="`+division+`"`) {
			return fmt.Errorf("expected a chip group for %q, got %q", division, s.lastBody)
		}
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionsSheetShowsFourDivisionWinnerSelects() error {
	for _, division := range store.Divisions() {
		if !strings.Contains(s.lastBody, `for="winner-`+division+`"`) {
			return fmt.Errorf("expected a winner select for %q, got %q", division, s.lastBody)
		}
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionsSheetShowsNoTeamPreselected() error {
	if strings.Contains(s.lastBody, "checked") {
		return fmt.Errorf("expected no chip preselected, got %q", s.lastBody)
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionsSheetShowsTheButtonText(text string) error {
	want := ">" + text + "</button>"
	if !strings.Contains(s.lastBody, want) {
		return fmt.Errorf("expected the button text %q, got %q", text, s.lastBody)
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionsSheetShowsTheSubmitButtonDisabled() error {
	if !strings.Contains(s.lastBody, `id="divisions-submit" disabled>`) {
		return fmt.Errorf("expected the submit button to render disabled, got %q", s.lastBody)
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionsSheetShowsTheSubmitButtonEnabled() error {
	if strings.Contains(s.lastBody, `id="divisions-submit" disabled>`) {
		return fmt.Errorf("expected the submit button to render enabled, got %q", s.lastBody)
	}
	return nil
}

// divisionChipGroupFragment isolates division's own chip-group markup (its
// data-division attribute through the next division or the conference's own
// count-indicator, whichever comes first), so a caller can assert a chip
// preselected within that division without matching a same-named team
// rendered under a sibling division's group.
func (s *divisionPicksScenarioState) divisionChipGroupFragment(division string) (string, error) {
	marker := `data-division="` + division + `"`
	start := strings.Index(s.lastBody, marker)
	if start == -1 {
		return "", fmt.Errorf("expected a chip group for %q, got %q", division, s.lastBody)
	}
	rest := s.lastBody[start+len(marker):]

	end := len(rest)
	for _, boundary := range []string{`data-division="`, `data-role="conference-count"`} {
		if idx := strings.Index(rest, boundary); idx != -1 && idx < end {
			end = idx
		}
	}
	return rest[:end], nil
}

func (s *divisionPicksScenarioState) theDivisionsSheetShowsTheTeamPreselectedForDivision(teamID, division string) error {
	fragment, err := s.divisionChipGroupFragment(division)
	if err != nil {
		return err
	}
	want := `value="` + teamID + `" checked`
	if !strings.Contains(fragment, want) {
		return fmt.Errorf("expected team %q preselected within %q's own chip group, got %q", teamID, division, fragment)
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionsSheetShows(text string) error {
	if !strings.Contains(s.lastBody, text) {
		return fmt.Errorf("expected the divisions sheet to show %q, got %q", text, s.lastBody)
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionsSheetShowsAReadOnlyBanner() error {
	if !strings.Contains(s.lastBody, "closed-banner") {
		return fmt.Errorf("expected a read-only banner, got %q", s.lastBody)
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionsSheetShowsNoSubmitButton() error {
	if strings.Contains(s.lastBody, `id="divisions-submit"`) {
		return fmt.Errorf("expected no submit button, got %q", s.lastBody)
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionPickResponseRedirectsTo(location string) error {
	if s.lastStatus != http.StatusFound {
		return fmt.Errorf("expected status %d, got %d: %s", http.StatusFound, s.lastStatus, s.lastBody)
	}
	if s.lastLocation != location {
		return fmt.Errorf("expected a redirect to %q, got %q", location, s.lastLocation)
	}
	return nil
}

func (s *divisionPicksScenarioState) theDivisionPickResponseStatusIs(want int) error {
	if s.lastStatus != want {
		return fmt.Errorf("expected status %d, got %d", want, s.lastStatus)
	}
	return nil
}

func (s *divisionPicksScenarioState) thePlayersSavedPlayoffTeamsForAre(division, csv string) error {
	prediction, ok := s.st.FindDivisionPlayoffTeams(divisionPicksPlayerID, division)
	if !ok {
		return fmt.Errorf("expected a saved playoff-teams row for %q, found none", division)
	}
	want := strings.Split(csv, ",")
	if len(prediction.TeamIDs) != len(want) {
		return fmt.Errorf("expected team ids %v for %q, got %v", want, division, prediction.TeamIDs)
	}
	for i, id := range want {
		if prediction.TeamIDs[i] != id {
			return fmt.Errorf("expected team ids %v for %q, got %v", want, division, prediction.TeamIDs)
		}
	}
	return nil
}

func (s *divisionPicksScenarioState) thePlayersSavedWinnerForIs(division, teamID string) error {
	prediction, ok := s.st.FindDivisionWinner(divisionPicksPlayerID, division)
	if !ok {
		return fmt.Errorf("expected a saved winner for %q, found none", division)
	}
	if prediction.TeamID != teamID {
		return fmt.Errorf("expected the saved winner for %q to be %q, got %q", division, teamID, prediction.TeamID)
	}
	return nil
}

func (s *divisionPicksScenarioState) thePlayerHasNoSavedPlayoffTeamsFor(division string) error {
	if _, ok := s.st.FindDivisionPlayoffTeams(divisionPicksPlayerID, division); ok {
		return fmt.Errorf("expected no saved playoff teams for %q, found some", division)
	}
	return nil
}

func (s *divisionPicksScenarioState) thePlayerHasNoSavedWinnerFor(division string) error {
	if _, ok := s.st.FindDivisionWinner(divisionPicksPlayerID, division); ok {
		return fmt.Errorf("expected no saved winner for %q, found one", division)
	}
	return nil
}

// thePredictScreenMarksAs asserts the Predict list's set row for id shows
// status - a distinctly-worded step from cup_and_presidents_picks_steps_
// test.go's own "the Predict screen shows the set row for ... with status
// ..." (same assertion, different phrasing) so the two features' step
// regexes can't collide when both are registered on the same
// godog.ScenarioContext.
func (s *divisionPicksScenarioState) thePredictScreenMarksAs(id, status string) error {
	if err := s.do(http.MethodGet, "/predict", ""); err != nil {
		return err
	}
	marker := `id="predict-row-` + id + `"`
	start := strings.Index(s.lastBody, marker)
	if start == -1 {
		return fmt.Errorf("expected a set row for %q, got %q", id, s.lastBody)
	}
	rest := s.lastBody[start:]
	end := strings.Index(rest, "</a>")
	if end == -1 {
		return fmt.Errorf("could not find the end of the set row for %q", id)
	}
	row := rest[:end]
	want := ">" + status + "</span>"
	if !strings.Contains(row, want) {
		return fmt.Errorf("expected the set row for %q to show status %q, got %q", id, status, row)
	}
	return nil
}

// InitializeDivisionPicksScenario registers the division-picks step
// definitions with GoDog.
func InitializeDivisionPicksScenario(ctx *godog.ScenarioContext) {
	s := newDivisionPicksScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the signed-in player for division picks is "([^"]*)"$`, s.theSignedInPlayerIs)
	ctx.Step(`^the canonical division team list includes the full 32-team roster$`, s.theCanonicalDivisionTeamListIncludesTheFullRoster)
	ctx.Step(`^the divisions Prediction Set has a deadline "([^"]*)"$`, s.theDivisionsSetHasADeadline)
	ctx.Step(`^the player already submitted a valid 8/8 division split with all four winners$`, s.thePlayerAlreadySubmittedAValid88SplitWithAllFourWinners)
	ctx.Step(`^the player opens the divisions Prediction Set$`, s.thePlayerOpensTheDivisionsPredictionSet)
	ctx.Step(`^the player submits a valid 8/8 division split with all four winners$`, s.thePlayerSubmitsAValid88SplitWithAllFourWinners)
	ctx.Step(`^the player submits a valid 5/3 division split with all four winners$`, s.thePlayerSubmitsAValid53SplitWithAllFourWinners)
	ctx.Step(`^the player submits a division split where the Eastern conference totals 7$`, s.thePlayerSubmitsASplitWhereTheEasternConferenceTotals7)
	ctx.Step(`^the player submits a division split where Atlantic holds 6 teams and Metropolitan holds 2$`, s.thePlayerSubmitsASplitWhereAtlanticHolds6TeamsAndMetropolitanHolds2)
	ctx.Step(`^the player submits a division split where a Metropolitan team is checked under Atlantic$`, s.thePlayerSubmitsASplitWhereAMetropolitanTeamIsCheckedUnderAtlantic)
	ctx.Step(`^the player submits a valid 8/8 division split with the Pacific winner left blank$`, s.thePlayerSubmitsAValid88SplitWithThePacificWinnerLeftBlank)
	ctx.Step(`^the player submits a valid 8/8 division split with a Metropolitan team as the Atlantic winner$`, s.thePlayerSubmitsAValid88SplitWithAMetropolitanTeamAsTheAtlanticWinner)
	ctx.Step(`^the divisions sheet shows four division chip groups$`, s.theDivisionsSheetShowsFourDivisionChipGroups)
	ctx.Step(`^the divisions sheet shows four division winner selects$`, s.theDivisionsSheetShowsFourDivisionWinnerSelects)
	ctx.Step(`^the divisions sheet shows no team preselected$`, s.theDivisionsSheetShowsNoTeamPreselected)
	ctx.Step(`^the divisions sheet shows the button text "([^"]*)"$`, s.theDivisionsSheetShowsTheButtonText)
	ctx.Step(`^the divisions sheet shows the submit button disabled$`, s.theDivisionsSheetShowsTheSubmitButtonDisabled)
	ctx.Step(`^the divisions sheet shows the submit button enabled$`, s.theDivisionsSheetShowsTheSubmitButtonEnabled)
	ctx.Step(`^the divisions sheet shows the team "([^"]*)" preselected for "([^"]*)"$`, s.theDivisionsSheetShowsTheTeamPreselectedForDivision)
	ctx.Step(`^the divisions sheet shows "([^"]*)"$`, s.theDivisionsSheetShows)
	ctx.Step(`^the divisions sheet shows a read-only banner$`, s.theDivisionsSheetShowsAReadOnlyBanner)
	ctx.Step(`^the divisions sheet shows no submit button$`, s.theDivisionsSheetShowsNoSubmitButton)
	ctx.Step(`^the division pick response redirects to "([^"]*)"$`, s.theDivisionPickResponseRedirectsTo)
	ctx.Step(`^the division pick response status is (\d+)$`, s.theDivisionPickResponseStatusIs)
	ctx.Step(`^the player's saved playoff teams for "([^"]*)" are "([^"]*)"$`, s.thePlayersSavedPlayoffTeamsForAre)
	ctx.Step(`^the player's saved winner for "([^"]*)" is "([^"]*)"$`, s.thePlayersSavedWinnerForIs)
	ctx.Step(`^the player has no saved playoff teams for "([^"]*)"$`, s.thePlayerHasNoSavedPlayoffTeamsFor)
	ctx.Step(`^the player has no saved winner for "([^"]*)"$`, s.thePlayerHasNoSavedWinnerFor)
	ctx.Step(`^the Predict screen marks "([^"]*)" as "([^"]*)"$`, s.thePredictScreenMarksAs)
}
