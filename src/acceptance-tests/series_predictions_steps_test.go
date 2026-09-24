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

// seriesPredictionsSecret signs session cookies for this scenario's server -
// it only needs to be non-empty and stable within one scenario run.
const seriesPredictionsSecret = "series-predictions-test-secret"

// seriesPredictionsPlayerID/Name is the one seeded player every scenario in
// this feature signs in as.
const (
	seriesPredictionsPlayerID   = "basti"
	seriesPredictionsPlayerName = "Basti"
)

// seriesPredictionsSetID/seriesPredictionsSetTitle map this feature's own
// "round 1"/"Stanley Cup Final" Gherkin wording onto the real Prediction Set
// id/title it seeds - this feature only ever exercises those two rounds
// (the other two, "r2"/"cf", share the identical series sheet and are
// already covered by round-unlocking.feature's own matchup-gating
// scenarios).
var seriesPredictionsSetID = map[string]string{
	"round 1":           "r1",
	"Stanley Cup Final": "scf",
}

var seriesPredictionsSetTitle = map[string]string{
	"r1":  "Playoff round 1",
	"scf": "Stanley Cup Final",
}

// seriesPredictionsTeams is the fixed 16-team roster (8 Eastern, 8 Western)
// every scenario in this feature seeds, matching this file's own r1
// matchups fixture - a small representative-of-the-real-roster sample,
// division_picks_steps_test.go's own precedent.
var seriesPredictionsTeams = []struct{ id, name, conference string }{
	{"BOS", "Boston Bruins", "Eastern"},
	{"TOR", "Toronto Maple Leafs", "Eastern"},
	{"TBL", "Tampa Bay Lightning", "Eastern"},
	{"FLA", "Florida Panthers", "Eastern"},
	{"CAR", "Carolina Hurricanes", "Eastern"},
	{"NYR", "New York Rangers", "Eastern"},
	{"WSH", "Washington Capitals", "Eastern"},
	{"NJD", "New Jersey Devils", "Eastern"},
	{"COL", "Colorado Avalanche", "Western"},
	{"DAL", "Dallas Stars", "Western"},
	{"EDM", "Edmonton Oilers", "Western"},
	{"VGK", "Vegas Golden Knights", "Western"},
	{"WPG", "Winnipeg Jets", "Western"},
	{"LAK", "Los Angeles Kings", "Western"},
	{"NSH", "Nashville Predators", "Western"},
	{"MIN", "Minnesota Wild", "Western"},
}

// seriesPredictionsR1Matchups is round 1's own fixed 8-series fixture, 4
// Eastern + 4 Western, matching seriesPredictionsTeams' own roster.
var seriesPredictionsR1Matchups = []struct{ key, teamA, teamB string }{
	{"e1", "BOS", "TOR"},
	{"e2", "TBL", "FLA"},
	{"e3", "CAR", "NYR"},
	{"e4", "WSH", "NJD"},
	{"w1", "COL", "DAL"},
	{"w2", "EDM", "VGK"},
	{"w3", "WPG", "LAK"},
	{"w4", "NSH", "MIN"},
}

// seriesPredictionsSetFixture is one Given-declared Prediction Set fixture,
// waiting to be written into the scenario's data file the first time a step
// needs a running server.
type seriesPredictionsSetFixture struct {
	id, title string
	deadline  time.Time
}

// seriesPredictionsPriorPick is a "the player already picked..." Background
// fixture, applied directly through the store before the scenario's first
// request (savePriorPicks).
type seriesPredictionsPriorPick struct {
	key, teamID, games string
}

// seriesPredictionsScenarioState holds the fixtures and results for one
// series-predictions scenario. A fresh instance is created per scenario so
// state never leaks between runs. The store and server are built lazily
// (lazyFixture.ensureReady) on the first request, since Given steps keep
// appending fixtures beforehand.
type seriesPredictionsScenarioState struct {
	lazyFixture
	sets       []seriesPredictionsSetFixture
	matchupSet string
	priorPicks []seriesPredictionsPriorPick
}

func newSeriesPredictionsScenarioState() *seriesPredictionsScenarioState {
	s := &seriesPredictionsScenarioState{}
	s.lazyFixture = newLazyFixture("series-predictions", seriesPredictionsPlayerID, seriesPredictionsPlayerName,
		seriesPredictionsSecret, s.seedBody, s.savePriorPicks)
	return s
}

func (s *seriesPredictionsScenarioState) theSignedInPlayerIs(name string) error {
	if strings.ToLower(name) != seriesPredictionsPlayerID {
		return fmt.Errorf("no fixture for player %q; only %q is seeded", name, seriesPredictionsPlayerName)
	}
	return nil
}

// theCanonicalTeamListIncludesTeamsFromBothConferences is a no-op:
// ensureReady always seeds seriesPredictionsTeams' full roster regardless -
// this step exists only so the Background reads clearly as a Given
// (theCanonicalDivisionTeamListIncludesTheFullRoster's own precedent).
func (s *seriesPredictionsScenarioState) theCanonicalTeamListIncludesTeamsFromBothConferences() error {
	return nil
}

func (s *seriesPredictionsScenarioState) theRoundPredictionSetHasADeadline(roundName, deadlinePhrase string) error {
	id, ok := seriesPredictionsSetID[roundName]
	if !ok {
		return fmt.Errorf("no fixture for round %q", roundName)
	}
	deadline, err := parseRelativeDeadline(deadlinePhrase, time.Now().UTC())
	if err != nil {
		return err
	}
	s.sets = append(s.sets, seriesPredictionsSetFixture{id: id, title: seriesPredictionsSetTitle[id], deadline: deadline})
	return nil
}

func (s *seriesPredictionsScenarioState) round1HasItsEightSeriesMatchupsRecorded() error {
	s.matchupSet = "r1"
	return nil
}

func (s *seriesPredictionsScenarioState) theStanleyCupFinalHasItsOneSeriesMatchupRecorded() error {
	s.matchupSet = "scf"
	return nil
}

// seedBody renders every Given-declared Prediction Set, the fixed
// seriesPredictionsTeams roster, and - once matchupSet names which round's
// matchups were declared - that round's own fixture (round 1's 8 series, or
// the Final's single cross-conference one) as the data file's
// prediction_sets/teams/playoff_matchups sections.
func (s *seriesPredictionsScenarioState) seedBody() (string, error) {
	var yamlSets strings.Builder
	for _, set := range s.sets {
		fmt.Fprintf(&yamlSets, "    - id: %s\n      title: %q\n      subtitle: \"\"\n      deadline_utc: %q\n      phase: playoffs\n      upcoming: false\n",
			set.id, set.title, set.deadline.Format(time.RFC3339))
	}

	var yamlTeams strings.Builder
	for _, team := range seriesPredictionsTeams {
		fmt.Fprintf(&yamlTeams, "    - id: %s\n      name: %q\n      conference: %s\n      division: \"\"\n", team.id, team.name, team.conference)
	}

	body := "prediction_sets:\n" + yamlSets.String() + "teams:\n" + yamlTeams.String()

	switch s.matchupSet {
	case "r1":
		var yamlMatchups strings.Builder
		yamlMatchups.WriteString("playoff_matchups:\n    r1:\n")
		for _, m := range seriesPredictionsR1Matchups {
			fmt.Fprintf(&yamlMatchups, "        - key: %s\n          a: %s\n          b: %s\n", m.key, m.teamA, m.teamB)
		}
		body += yamlMatchups.String()
	case "scf":
		body += "playoff_matchups:\n    scf:\n        - key: f1\n          a: BOS\n          b: COL\n"
	}

	return body, nil
}

func (s *seriesPredictionsScenarioState) thePlayerAlreadyPickedInGamesForSeries(teamID, games, key string) error {
	s.priorPicks = append(s.priorPicks, seriesPredictionsPriorPick{key: key, teamID: teamID, games: games})
	return nil
}

// savePriorPicks saves every Background-declared "already picked" fixture
// directly through the store, before the scenario's first request -
// division_picks_steps_test.go's own savePriorSubmission precedent.
func (s *seriesPredictionsScenarioState) savePriorPicks(st *store.Store) error {
	for _, pick := range s.priorPicks {
		if err := st.SaveSeriesPick(seriesPredictionsPlayerID, "r1."+pick.key, pick.teamID, pick.games, time.Now().UTC()); err != nil {
			return fmt.Errorf("seed prior series pick: %w", err)
		}
	}
	return nil
}

func (s *seriesPredictionsScenarioState) thePlayerOpensTheRoundPredictionSet(roundName string) error {
	id, ok := seriesPredictionsSetID[roundName]
	if !ok {
		return fmt.Errorf("no fixture for round %q", roundName)
	}
	return s.do(http.MethodGet, "/predict/"+id, "")
}

func (s *seriesPredictionsScenarioState) thePlayerPicksInGamesForSeries(teamID, games, key string) error {
	values := url.Values{}
	values.Set("series_team_"+key, teamID)
	values.Set("series_games_"+key, games)
	return s.do(http.MethodPost, "/predict/r1", values.Encode())
}

func (s *seriesPredictionsScenarioState) thePlayerSubmitsOnlyTheTeamForSeriesAndAValidPickOfInGamesForSeries(halfTeamID, halfKey, validTeamID, validGames, validKey string) error {
	values := url.Values{}
	values.Set("series_team_"+halfKey, halfTeamID)
	values.Set("series_team_"+validKey, validTeamID)
	values.Set("series_games_"+validKey, validGames)
	return s.do(http.MethodPost, "/predict/r1", values.Encode())
}

func (s *seriesPredictionsScenarioState) thePlayerSubmitsTheForeignTeamInGamesForSeriesAndAValidPickOfInGamesForSeries(foreignTeamID, foreignGames, foreignKey, validTeamID, validGames, validKey string) error {
	values := url.Values{}
	values.Set("series_team_"+foreignKey, foreignTeamID)
	values.Set("series_games_"+foreignKey, foreignGames)
	values.Set("series_team_"+validKey, validTeamID)
	values.Set("series_games_"+validKey, validGames)
	return s.do(http.MethodPost, "/predict/r1", values.Encode())
}

func (s *seriesPredictionsScenarioState) theSeriesSheetShowsNSeriesCards(want int) error {
	got := strings.Count(s.lastBody, `data-series="`)
	if got != want {
		return fmt.Errorf("expected %d series cards, got %d in %q", want, got, s.lastBody)
	}
	return nil
}

// seriesGroupFragment isolates the group named label (from its own
// <legend>...</legend> through the next <fieldset> or the end of the body) -
// divisionChipGroupFragment's own isolation precedent, so a card-count
// assertion can't accidentally match a different group's own cards.
func (s *seriesPredictionsScenarioState) seriesGroupFragment(label string) (string, error) {
	marker := ">" + label + "</legend>"
	start := strings.Index(s.lastBody, marker)
	if start == -1 {
		return "", fmt.Errorf("expected a group labeled %q, got %q", label, s.lastBody)
	}
	rest := s.lastBody[start+len(marker):]
	if end := strings.Index(rest, "<fieldset"); end != -1 {
		return rest[:end], nil
	}
	return rest, nil
}

func (s *seriesPredictionsScenarioState) theSeriesSheetShowsNSeriesCardsUnder(want int, label string) error {
	fragment, err := s.seriesGroupFragment(label)
	if err != nil {
		return err
	}
	if got := strings.Count(fragment, `data-series="`); got != want {
		return fmt.Errorf("expected %d series cards under %q, got %d in %q", want, label, got, fragment)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) theSeriesSheetShowsAHeading(label string) error {
	if !strings.Contains(s.lastBody, ">"+label+"</legend>") {
		return fmt.Errorf("expected a %q heading, got %q", label, s.lastBody)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) theSeriesSheetShowsNoConferenceHeading() error {
	if strings.Contains(s.lastBody, "Eastern Conference") || strings.Contains(s.lastBody, "Western Conference") {
		return fmt.Errorf("expected no conference heading, got %q", s.lastBody)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) theSeriesPickResponseRedirectsTo(location string) error {
	if s.lastStatus != http.StatusFound {
		return fmt.Errorf("expected status %d, got %d: %s", http.StatusFound, s.lastStatus, s.lastBody)
	}
	if s.lastLocation != location {
		return fmt.Errorf("expected a redirect to %q, got %q", location, s.lastLocation)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) theSeriesPickResponseStatusIs(want int) error {
	if s.lastStatus != want {
		return fmt.Errorf("expected status %d, got %d: %s", want, s.lastStatus, s.lastBody)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) thePlayersSavedSeriesPickForIs(key, teamID, games string) error {
	prediction, ok := s.st.FindSeriesPick(seriesPredictionsPlayerID, "r1."+key)
	if !ok {
		return fmt.Errorf("expected a saved series pick for %q, found none", key)
	}
	if prediction.TeamID != teamID || prediction.Games != games {
		return fmt.Errorf("expected the saved pick for %q to be %q in %q games, got %q in %q games", key, teamID, games, prediction.TeamID, prediction.Games)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) thePlayerHasNoSavedSeriesPickFor(key string) error {
	if _, ok := s.st.FindSeriesPick(seriesPredictionsPlayerID, "r1."+key); ok {
		return fmt.Errorf("expected no saved series pick for %q, found one", key)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) thePredictScreenMarksRound1As(status string) error {
	if err := s.do(http.MethodGet, "/predict", ""); err != nil {
		return err
	}
	marker := `id="predict-row-r1"`
	start := strings.Index(s.lastBody, marker)
	if start == -1 {
		return fmt.Errorf("expected a set row for %q, got %q", "r1", s.lastBody)
	}
	rest := s.lastBody[start:]
	end := strings.Index(rest, "</a>")
	if end == -1 {
		return fmt.Errorf("could not find the end of the set row for %q", "r1")
	}
	row := rest[:end]
	want := ">" + status + "</span>"
	if !strings.Contains(row, want) {
		return fmt.Errorf("expected the round 1 row to show status %q, got %q", status, row)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) theSeriesSheetShowsAnInlineErrorOnSeries(key string) error {
	marker := `data-series="` + key + `"`
	start := strings.Index(s.lastBody, marker)
	if start == -1 {
		return fmt.Errorf("expected a series card for %q, got %q", key, s.lastBody)
	}
	rest := s.lastBody[start+len(marker):]
	end := len(rest)
	if next := strings.Index(rest, `data-series="`); next != -1 {
		end = next
	}
	card := rest[:end]
	if !strings.Contains(card, "error-text") {
		return fmt.Errorf("expected series %q to show an inline error, got %q", key, card)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) theSeriesSheetShowsAReadOnlyBanner() error {
	if !strings.Contains(s.lastBody, "closed-banner") {
		return fmt.Errorf("expected a read-only banner, got %q", s.lastBody)
	}
	return nil
}

func (s *seriesPredictionsScenarioState) theSeriesSheetShowsNoSubmitButton() error {
	if strings.Contains(s.lastBody, `<button type="submit">`) {
		return fmt.Errorf("expected no submit button, got %q", s.lastBody)
	}
	return nil
}

// InitializeSeriesPredictionsScenario registers the series-predictions step
// definitions with GoDog.
func InitializeSeriesPredictionsScenario(ctx *godog.ScenarioContext) {
	s := newSeriesPredictionsScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the signed-in player for series predictions is "([^"]*)"$`, s.theSignedInPlayerIs)
	ctx.Step(`^the canonical team list for series predictions includes teams from both conferences$`, s.theCanonicalTeamListIncludesTeamsFromBothConferences)
	ctx.Step(`^the (round 1|Stanley Cup Final) Prediction Set has a deadline "([^"]*)"$`, s.theRoundPredictionSetHasADeadline)
	ctx.Step(`^round 1 has its 8 series matchups recorded$`, s.round1HasItsEightSeriesMatchupsRecorded)
	ctx.Step(`^the Stanley Cup Final has its 1 series matchup recorded$`, s.theStanleyCupFinalHasItsOneSeriesMatchupRecorded)
	ctx.Step(`^the player already picked "([^"]*)" in "([^"]*)" games for series "([^"]*)"$`, s.thePlayerAlreadyPickedInGamesForSeries)
	ctx.Step(`^the player opens the (round 1|Stanley Cup Final) Prediction Set$`, s.thePlayerOpensTheRoundPredictionSet)
	ctx.Step(`^the player picks "([^"]*)" in "([^"]*)" games for series "([^"]*)"$`, s.thePlayerPicksInGamesForSeries)
	ctx.Step(`^the player submits only the team "([^"]*)" for series "([^"]*)" and a valid pick of "([^"]*)" in "([^"]*)" games for series "([^"]*)"$`, s.thePlayerSubmitsOnlyTheTeamForSeriesAndAValidPickOfInGamesForSeries)
	ctx.Step(`^the player submits the foreign team "([^"]*)" in "([^"]*)" games for series "([^"]*)" and a valid pick of "([^"]*)" in "([^"]*)" games for series "([^"]*)"$`, s.thePlayerSubmitsTheForeignTeamInGamesForSeriesAndAValidPickOfInGamesForSeries)
	ctx.Step(`^the series sheet shows (\d+) series cards?$`, s.theSeriesSheetShowsNSeriesCards)
	ctx.Step(`^the series sheet shows (\d+) series cards under "([^"]*)"$`, s.theSeriesSheetShowsNSeriesCardsUnder)
	ctx.Step(`^the series sheet shows a "([^"]*)" heading$`, s.theSeriesSheetShowsAHeading)
	ctx.Step(`^the series sheet shows no conference heading$`, s.theSeriesSheetShowsNoConferenceHeading)
	ctx.Step(`^the series pick response redirects to "([^"]*)"$`, s.theSeriesPickResponseRedirectsTo)
	ctx.Step(`^the series pick response status is (\d+)$`, s.theSeriesPickResponseStatusIs)
	ctx.Step(`^the player's saved series pick for "([^"]*)" is "([^"]*)" in "([^"]*)" games$`, s.thePlayersSavedSeriesPickForIs)
	ctx.Step(`^the player has no saved series pick for "([^"]*)"$`, s.thePlayerHasNoSavedSeriesPickFor)
	ctx.Step(`^the Predict screen marks round 1 as "([^"]*)"$`, s.thePredictScreenMarksRound1As)
	ctx.Step(`^the series sheet shows an inline error on series "([^"]*)"$`, s.theSeriesSheetShowsAnInlineErrorOnSeries)
	ctx.Step(`^the series sheet shows a read-only banner$`, s.theSeriesSheetShowsAReadOnlyBanner)
	ctx.Step(`^the series sheet shows no submit button$`, s.theSeriesSheetShowsNoSubmitButton)
}
