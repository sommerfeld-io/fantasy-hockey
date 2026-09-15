package acceptance_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
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

// relativeDeadlinePattern parses the Gherkin-friendly deadline phrases this
// feature's Given steps accept ("in 5 days", "in 3 hours", "1 day ago"),
// keeping every scenario's deadline relative to the moment it runs rather
// than a hardcoded date that would eventually go stale.
var relativeDeadlinePattern = regexp.MustCompile(`^(?:in (\d+) (hour|hours|day|days)|(\d+) (hour|hours|day|days) ago)$`)

// parseRelativeDeadline resolves phrase against now into an absolute UTC
// deadline.
func parseRelativeDeadline(phrase string, now time.Time) (time.Time, error) {
	m := relativeDeadlinePattern.FindStringSubmatch(phrase)
	if m == nil {
		return time.Time{}, fmt.Errorf("unrecognized relative deadline %q", phrase)
	}

	var amount, unit string
	sign := 1
	if m[1] != "" {
		amount, unit = m[1], m[2]
	} else {
		amount, unit = m[3], m[4]
		sign = -1
	}

	n, err := strconv.Atoi(amount)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse amount %q: %w", amount, err)
	}

	step := time.Hour
	if strings.HasPrefix(unit, "day") {
		step = 24 * time.Hour
	}

	return now.Add(time.Duration(sign*n) * step), nil
}

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
// (ensureReady) on the first request, since Given steps keep appending
// fixtures beforehand.
type browsePredictionSetsScenarioState struct {
	dataFile           string
	sets               []predictionSetFixture
	server             *httptest.Server
	fileBeforeRequests []byte
	lastStatus         int
	lastBody           string
	lastLocation       string
}

func newBrowsePredictionSetsScenarioState() *browsePredictionSetsScenarioState {
	dir, err := os.MkdirTemp("", "fantasy-hockey-browse-prediction-sets-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	return &browsePredictionSetsScenarioState{dataFile: filepath.Join(dir, "fantasy-hockey.yml")}
}

func (s *browsePredictionSetsScenarioState) close() {
	if s.server != nil {
		s.server.Close()
	}
	_ = os.RemoveAll(filepath.Dir(s.dataFile)) // best-effort cleanup of the scenario's temp dir
}

// theFixturePlayerIs validates name against the one player fixture every
// scenario in this feature seeds and signs requests in as (see get). It
// performs no sign-in action itself - the session cookie get() attaches is
// unconditional and doesn't depend on this step having run.
func (s *browsePredictionSetsScenarioState) theFixturePlayerIs(name string) error {
	if strings.ToLower(name) != browsePredictionSetsPlayerID {
		return fmt.Errorf("no fixture for player %q; only %q is seeded", name, browsePredictionSetsPlayerName)
	}
	return nil
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

// ensureReady lazily persists every Given-declared Prediction Set and starts
// the real production web.NewServer handler around it, the first time a step
// needs to make an HTTP call.
func (s *browsePredictionSetsScenarioState) ensureReady() error {
	if s.server != nil {
		return nil
	}

	var yamlSets strings.Builder
	for _, set := range s.sets {
		fmt.Fprintf(&yamlSets, "    - id: %q\n      title: %q\n      subtitle: %q\n      deadline_utc: %q\n      phase: %q\n      upcoming: %t\n",
			set.id, set.title, set.subtitle, set.deadline.Format(time.RFC3339), set.phase, set.upcoming)
	}

	seed := fmt.Sprintf("season: \"2026-27\"\nplayers:\n    - id: %s\n      name: %s\n      email: basti@example.com\nprediction_sets:\n%s",
		browsePredictionSetsPlayerID, browsePredictionSetsPlayerName, yamlSets.String())
	if err := os.WriteFile(s.dataFile, []byte(seed), 0o600); err != nil {
		return fmt.Errorf("seed data file: %w", err)
	}

	raw, err := os.ReadFile(s.dataFile)
	if err != nil {
		return fmt.Errorf("snapshot data file: %w", err)
	}
	s.fileBeforeRequests = raw

	st, err := store.New(s.dataFile)
	if err != nil {
		return fmt.Errorf("store.New: %w", err)
	}

	s.server = httptest.NewServer(web.NewServer(st, noopSender, browsePredictionSetsSecret))
	return nil
}

// get requests path carrying the signed-in player's session cookie, without
// following any redirect, and records the result.
func (s *browsePredictionSetsScenarioState) get(path string) error {
	if err := s.ensureReady(); err != nil {
		return err
	}

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequest(http.MethodGet, s.server.URL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.AddCookie(auth.IssueSessionCookie(browsePredictionSetsPlayerID, browsePredictionSetsSecret))

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
	return nil
}

func (s *browsePredictionSetsScenarioState) thePlayerOpensThePredictScreen() error {
	return s.get("/predict")
}

func (s *browsePredictionSetsScenarioState) thePlayerOpensThePredictionSet(id string) error {
	return s.get("/predict/" + id)
}

func (s *browsePredictionSetsScenarioState) thePredictScreenShowsTheSection(label string) error {
	if !strings.Contains(s.lastBody, label) {
		return fmt.Errorf("expected the Predict screen to show the %q section, got %q", label, s.lastBody)
	}
	return nil
}

// setRowFragment isolates the single set row for id (the <a>/<div> with
// id="predict-row-{id}" up to its closing tag) so an assertion about one row
// can't accidentally match text belonging to a different row on the same
// page.
func (s *browsePredictionSetsScenarioState) setRowFragment(id string) (string, error) {
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
		return fmt.Errorf("expected fantasy-hockey.yml to stay untouched, got diff:\nbefore:\n%s\nafter:\n%s", s.fileBeforeRequests, after)
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

	ctx.Step(`^the fixture player is "([^"]*)"$`, s.theFixturePlayerIs)
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
