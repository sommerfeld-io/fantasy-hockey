package acceptance_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// roundUnlockingSecret signs session cookies for this scenario's server - it
// only needs to be non-empty and stable within one scenario run.
const roundUnlockingSecret = "round-unlocking-test-secret"

// roundUnlockingPlayerID/Name is the one seeded player every scenario in
// this feature signs in as.
const (
	roundUnlockingPlayerID   = "basti"
	roundUnlockingPlayerName = "Basti"
)

// roundUnlockingMatchupFixture is one Given-declared playoff_matchups entry,
// waiting to be written into the scenario's data file the first time a step
// needs a running server.
type roundUnlockingMatchupFixture struct {
	setID, teamA, teamB string
}

// roundUnlockingScenarioState holds the fixtures and results for one
// round-unlocking scenario. A fresh instance is created per scenario so
// state never leaks between runs. The store and server are built lazily
// (lazyFixture.ensureReady) on the first request, since Given steps keep
// appending fixtures beforehand. It reuses predictionSetFixture (defined
// alongside browse-prediction-sets' own steps in this same package) for its
// Prediction Set fixtures, since the shape is identical.
type roundUnlockingScenarioState struct {
	lazyFixture
	sets     []predictionSetFixture
	matchups []roundUnlockingMatchupFixture
}

func newRoundUnlockingScenarioState() *roundUnlockingScenarioState {
	s := &roundUnlockingScenarioState{}
	s.lazyFixture = newLazyFixture("round-unlocking", roundUnlockingPlayerID, roundUnlockingPlayerName,
		roundUnlockingSecret, s.seedBody, nil)
	return s
}

// aPredictionSetExists declares a Prediction Set fixture with upcoming
// always false - Story 3.2's own scenarios only ever need to prove a
// round-gated id ignores that flag, never that it honors an upcoming: true
// value (browse-prediction-sets.feature already covers the ordinary,
// non-round-gated Upcoming path).
func (s *roundUnlockingScenarioState) aPredictionSetExists(id, title, subtitle, phase, deadlinePhrase string) error {
	deadline, err := parseRelativeDeadline(deadlinePhrase, time.Now().UTC())
	if err != nil {
		return err
	}
	s.sets = append(s.sets, predictionSetFixture{
		id:       id,
		title:    title,
		subtitle: subtitle,
		phase:    phase,
		deadline: deadline,
		upcoming: false,
	})
	return nil
}

func (s *roundUnlockingScenarioState) aRecordedPlayoffMatchup(teamA, teamB, setID string) error {
	s.matchups = append(s.matchups, roundUnlockingMatchupFixture{setID: setID, teamA: teamA, teamB: teamB})
	return nil
}

// seedBody renders every Given-declared Prediction Set as the data file's
// prediction_sets section, followed by a playoff_matchups section grouping
// every Given-declared matchup by its Prediction Set id - omitted entirely
// when no matchup was declared, matching how fantasy-hockey.yml has no
// playoff_matchups key at all until a human first adds one (Design Notes).
func (s *roundUnlockingScenarioState) seedBody() (string, error) {
	var yamlSets strings.Builder
	for _, set := range s.sets {
		fmt.Fprintf(&yamlSets, "    - id: %q\n      title: %q\n      subtitle: %q\n      deadline_utc: %q\n      phase: %q\n      upcoming: %t\n",
			set.id, set.title, set.subtitle, set.deadline.Format(time.RFC3339), set.phase, set.upcoming)
	}
	body := "prediction_sets:\n" + yamlSets.String()

	if len(s.matchups) == 0 {
		return body, nil
	}

	bySetID := make(map[string][]roundUnlockingMatchupFixture)
	var order []string
	for _, m := range s.matchups {
		if _, seen := bySetID[m.setID]; !seen {
			order = append(order, m.setID)
		}
		bySetID[m.setID] = append(bySetID[m.setID], m)
	}

	var yamlMatchups strings.Builder
	yamlMatchups.WriteString("playoff_matchups:\n")
	for _, setID := range order {
		fmt.Fprintf(&yamlMatchups, "    %s:\n", setID)
		// key is generated as "s<n>" (1-based, per setID) rather than left
		// blank - Story 3.3's series sheet requires every matchup to carry a
		// non-empty Key, so this fixture must reflect that same valid shape.
		for i, m := range bySetID[setID] {
			fmt.Fprintf(&yamlMatchups, "        - key: %q\n          a: %q\n          b: %q\n", fmt.Sprintf("s%d", i+1), m.teamA, m.teamB)
		}
	}

	return body + yamlMatchups.String(), nil
}

func (s *roundUnlockingScenarioState) thePlayerOpensThePredictScreen() error {
	return s.do(http.MethodGet, "/predict", "")
}

func (s *roundUnlockingScenarioState) thePlayerOpensThePredictionSet(id string) error {
	return s.do(http.MethodGet, "/predict/"+id, "")
}

func (s *roundUnlockingScenarioState) theSetRowShowsTheStatus(id, status string) error {
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

func (s *roundUnlockingScenarioState) theSetRowIsActionable(id string) error {
	row, err := s.setRowFragment(id)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(row, `id="predict-row-`+id+`" href="/predict/`+id+`"`) {
		return fmt.Errorf("expected the set row for %q to be an <a> linking to /predict/%s, got %q", id, id, row)
	}
	return nil
}

func (s *roundUnlockingScenarioState) theSetRowIsNotActionable(id string) error {
	row, err := s.setRowFragment(id)
	if err != nil {
		return err
	}
	if strings.Contains(row, "href=") {
		return fmt.Errorf("expected the set row for %q not to be a link, got %q", id, row)
	}
	return nil
}

func (s *roundUnlockingScenarioState) theSheetResponseStatusIs(want int) error {
	if s.lastStatus != want {
		return fmt.Errorf("expected status %d, got %d", want, s.lastStatus)
	}
	return nil
}

func (s *roundUnlockingScenarioState) theSheetShowsTheTitle(title string) error {
	if !strings.Contains(s.lastBody, title) {
		return fmt.Errorf("expected the sheet to show title %q, got %q", title, s.lastBody)
	}
	return nil
}

// InitializeRoundUnlockingScenario registers the round-unlocking step
// definitions with GoDog.
func InitializeRoundUnlockingScenario(ctx *godog.ScenarioContext) {
	s := newRoundUnlockingScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the signed-in player for round unlocking is "([^"]*)"$`, s.theSignedInPlayerIs)
	ctx.Step(`^a round-unlocking Prediction Set "([^"]*)" titled "([^"]*)" with subtitle "([^"]*)" in phase "([^"]*)" with a deadline "([^"]*)"$`, s.aPredictionSetExists)
	ctx.Step(`^a recorded playoff matchup "([^"]*)" vs "([^"]*)" for "([^"]*)"$`, s.aRecordedPlayoffMatchup)
	ctx.Step(`^the player opens the round-unlocking Predict screen$`, s.thePlayerOpensThePredictScreen)
	ctx.Step(`^the player opens the round-unlocking Prediction Set "([^"]*)"$`, s.thePlayerOpensThePredictionSet)
	ctx.Step(`^the round-unlocking set row for "([^"]*)" shows the status "([^"]*)"$`, s.theSetRowShowsTheStatus)
	ctx.Step(`^the round-unlocking set row for "([^"]*)" is actionable$`, s.theSetRowIsActionable)
	ctx.Step(`^the round-unlocking set row for "([^"]*)" is not actionable$`, s.theSetRowIsNotActionable)
	ctx.Step(`^the round-unlocking sheet response status is (\d+)$`, s.theSheetResponseStatusIs)
	ctx.Step(`^the round-unlocking sheet shows the title "([^"]*)"$`, s.theSheetShowsTheTitle)
}
