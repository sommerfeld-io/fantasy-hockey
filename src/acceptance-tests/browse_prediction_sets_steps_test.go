package acceptance_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// browsePredictionSetsSecret signs session cookies for this scenario's
// server - it only needs to be non-empty and stable within one scenario run.
const browsePredictionSetsSecret = "browse-prediction-sets-test-secret"

// browsePredictionSetsPlayerID/Name is the one seeded player every scenario
// in this feature signs in as.
const (
	browsePredictionSetsPlayerID   = "basti"
	browsePredictionSetsPlayerName = "Basti"
)

// predictionSetFixture is one Given-declared Prediction Set, waiting to be
// written into the scenario's data file the first time a step needs a
// running server.
type predictionSetFixture struct {
	id, title, subtitle, phase string
	deadline                   time.Time
	upcoming                   bool
}

// browsePredictionSetsScenarioState holds the fixtures and results for one
// browse-prediction-sets scenario. A fresh instance is created per scenario
// so state never leaks between runs. The store and server are built lazily
// (lazyFixture.ensureReady) on the first request, since Given steps keep
// appending fixtures beforehand.
type browsePredictionSetsScenarioState struct {
	lazyFixture
	sets               []predictionSetFixture
	fileBeforeRequests []byte
}

func newBrowsePredictionSetsScenarioState() *browsePredictionSetsScenarioState {
	s := &browsePredictionSetsScenarioState{}
	s.lazyFixture = newLazyFixture("browse-prediction-sets", browsePredictionSetsPlayerID, browsePredictionSetsPlayerName,
		browsePredictionSetsSecret, s.seedBody, s.snapshotDataFile)
	return s
}

func (s *browsePredictionSetsScenarioState) addPredictionSet(id, title, subtitle, phase, deadlinePhrase string, upcoming bool) error {
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
		upcoming: upcoming,
	})
	return nil
}

func (s *browsePredictionSetsScenarioState) aPredictionSetExists(id, title, subtitle, phase, deadlinePhrase string) error {
	return s.addPredictionSet(id, title, subtitle, phase, deadlinePhrase, false)
}

func (s *browsePredictionSetsScenarioState) anUpcomingPredictionSetExists(id, title, subtitle, phase, deadlinePhrase string) error {
	return s.addPredictionSet(id, title, subtitle, phase, deadlinePhrase, true)
}

// seedBody renders every Given-declared Prediction Set as the data file's
// prediction_sets section.
func (s *browsePredictionSetsScenarioState) seedBody() (string, error) {
	var yamlSets strings.Builder
	for _, set := range s.sets {
		fmt.Fprintf(&yamlSets, "    - id: %q\n      title: %q\n      subtitle: %q\n      deadline_utc: %q\n      phase: %q\n      upcoming: %t\n",
			set.id, set.title, set.subtitle, set.deadline.Format(time.RFC3339), set.phase, set.upcoming)
	}
	return "prediction_sets:\n" + yamlSets.String(), nil
}

// snapshotDataFile records the seeded data file's bytes before any request
// runs, so theDataFileIsUnchanged can prove no request wrote to it.
func (s *browsePredictionSetsScenarioState) snapshotDataFile(_ *store.Store) error {
	raw, err := os.ReadFile(s.dataFile)
	if err != nil {
		return fmt.Errorf("snapshot data file: %w", err)
	}
	s.fileBeforeRequests = raw
	return nil
}

func (s *browsePredictionSetsScenarioState) thePlayerOpensThePredictScreen() error {
	return s.do(http.MethodGet, "/predict", "")
}

func (s *browsePredictionSetsScenarioState) thePlayerOpensThePredictionSet(id string) error {
	return s.do(http.MethodGet, "/predict/"+id, "")
}

func (s *browsePredictionSetsScenarioState) thePredictScreenShowsTheSection(label string) error {
	if !strings.Contains(s.lastBody, label) {
		return fmt.Errorf("expected the Predict screen to show the %q section, got %q", label, s.lastBody)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theSetRowShowsTheTitle(id, title string) error {
	row, err := s.setRowFragment(id)
	if err != nil {
		return err
	}
	if !strings.Contains(row, title) {
		return fmt.Errorf("expected the set row for %q to show title %q, got %q", id, title, row)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theSetRowShowsTheSubtitle(id, subtitle string) error {
	row, err := s.setRowFragment(id)
	if err != nil {
		return err
	}
	if !strings.Contains(row, subtitle) {
		return fmt.Errorf("expected the set row for %q to show subtitle %q, got %q", id, subtitle, row)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theSetRowShowsTheStatus(id, status string) error {
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

func (s *browsePredictionSetsScenarioState) theSetRowShowsTheCountdown(id, countdown string) error {
	row, err := s.setRowFragment(id)
	if err != nil {
		return err
	}
	if !strings.Contains(row, countdown) {
		return fmt.Errorf("expected the set row for %q to show countdown %q, got %q", id, countdown, row)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theSetRowIsActionable(id string) error {
	row, err := s.setRowFragment(id)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(row, `id="predict-row-`+id+`" href="/predict/`+id+`"`) {
		return fmt.Errorf("expected the set row for %q to be an <a> linking to /predict/%s, got %q", id, id, row)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theSetRowIsNotActionable(id string) error {
	row, err := s.setRowFragment(id)
	if err != nil {
		return err
	}
	if strings.Contains(row, "href=") {
		return fmt.Errorf("expected the set row for %q not to be a link, got %q", id, row)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theSheetShowsTheTitle(title string) error {
	if !strings.Contains(s.lastBody, title) {
		return fmt.Errorf("expected the sheet to show title %q, got %q", title, s.lastBody)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theSheetShowsTheCountdown(countdown string) error {
	if !strings.Contains(s.lastBody, countdown) {
		return fmt.Errorf("expected the sheet to show countdown %q, got %q", countdown, s.lastBody)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theSheetShows(text string) error {
	if !strings.Contains(s.lastBody, text) {
		return fmt.Errorf("expected the sheet to show %q, got %q", text, s.lastBody)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theSheetResponseStatusIs(want int) error {
	if s.lastStatus != want {
		return fmt.Errorf("expected status %d, got %d", want, s.lastStatus)
	}
	return nil
}

func (s *browsePredictionSetsScenarioState) theDataFileIsUnchanged() error {
	after, err := os.ReadFile(s.dataFile)
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	if string(after) != string(s.fileBeforeRequests) {
		return fmt.Errorf("expected %s to stay untouched, got diff:\nbefore:\n%s\nafter:\n%s", store.DataFileName, s.fileBeforeRequests, after)
	}
	return nil
}

// InitializeBrowsePredictionSetsScenario registers the browse-prediction-
// sets step definitions with GoDog.
func InitializeBrowsePredictionSetsScenario(ctx *godog.ScenarioContext) {
	s := newBrowsePredictionSetsScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the fixture player is "([^"]*)"$`, s.theSignedInPlayerIs)
	ctx.Step(`^a Prediction Set "([^"]*)" titled "([^"]*)" with subtitle "([^"]*)" in phase "([^"]*)" with a deadline "([^"]*)"$`, s.aPredictionSetExists)
	ctx.Step(`^an upcoming Prediction Set "([^"]*)" titled "([^"]*)" with subtitle "([^"]*)" in phase "([^"]*)" with a deadline "([^"]*)"$`, s.anUpcomingPredictionSetExists)
	ctx.Step(`^the player opens the Predict screen$`, s.thePlayerOpensThePredictScreen)
	ctx.Step(`^the player opens the Prediction Set "([^"]*)"$`, s.thePlayerOpensThePredictionSet)
	ctx.Step(`^the Predict screen shows the "([^"]*)" section$`, s.thePredictScreenShowsTheSection)
	ctx.Step(`^the set row for "([^"]*)" shows the title "([^"]*)"$`, s.theSetRowShowsTheTitle)
	ctx.Step(`^the set row for "([^"]*)" shows the subtitle "([^"]*)"$`, s.theSetRowShowsTheSubtitle)
	ctx.Step(`^the set row for "([^"]*)" shows the status "([^"]*)"$`, s.theSetRowShowsTheStatus)
	ctx.Step(`^the set row for "([^"]*)" shows the countdown "([^"]*)"$`, s.theSetRowShowsTheCountdown)
	ctx.Step(`^the set row for "([^"]*)" is actionable$`, s.theSetRowIsActionable)
	ctx.Step(`^the set row for "([^"]*)" is not actionable$`, s.theSetRowIsNotActionable)
	ctx.Step(`^the sheet shows the title "([^"]*)"$`, s.theSheetShowsTheTitle)
	ctx.Step(`^the sheet shows the countdown "([^"]*)"$`, s.theSheetShowsTheCountdown)
	ctx.Step(`^the sheet shows "([^"]*)"$`, s.theSheetShows)
	ctx.Step(`^the sheet response status is (\d+)$`, s.theSheetResponseStatusIs)
	ctx.Step(`^the data file is unchanged$`, s.theDataFileIsUnchanged)
}
