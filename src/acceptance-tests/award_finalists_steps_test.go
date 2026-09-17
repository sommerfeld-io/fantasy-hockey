package acceptance_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// awardFinalistsSecret signs session cookies for this scenario's server - it
// only needs to be non-empty and stable within one scenario run.
const awardFinalistsSecret = "award-finalists-test-secret"

// awardFinalistsPlayerID/Name is the one seeded player every scenario in
// this feature signs in as.
const (
	awardFinalistsPlayerID   = "basti"
	awardFinalistsPlayerName = "Basti"
)

// awardFinalistsOrder mirrors internal/web's own awardOrder - this feature
// exercises the real form, so its fixture must cover every award in the
// same fixed order.
var awardFinalistsOrder = []string{"hart", "norris", "vezina", "art_ross", "rocket_richard"}

// awardFinalistsNHLPlayersYAML is a small representative nhl_players:
// fixture spanning all 3 positions - enough skaters/defensemen/goalies to
// fill every award's own eligible position (Hart/Art Ross/Rocket Richard
// reuse the same 3 skaters, matching how this feature's own valid
// submission fixture does).
const awardFinalistsNHLPlayersYAML = `nhl_players:
    - slug: mcdavid-connor
      display_name: Connor McDavid
      position: skater
    - slug: mackinnon-nathan
      display_name: Nathan MacKinnon
      position: skater
    - slug: kucherov-nikita
      display_name: Nikita Kucherov
      position: skater
    - slug: makar-cale
      display_name: Cale Makar
      position: defenseman
    - slug: hughes-quinn
      display_name: Quinn Hughes
      position: defenseman
    - slug: werenski-zach
      display_name: Zach Werenski
      position: defenseman
    - slug: hellebuyck-connor
      display_name: Connor Hellebuyck
      position: goalie
    - slug: shesterkin-igor
      display_name: Igor Shesterkin
      position: goalie
    - slug: vasilevskiy-andrei
      display_name: Andrei Vasilevskiy
      position: goalie
`

// awardFinalistSlotFixture is one finalist slot's raw (text, slug) pair, as
// this fixture posts it to the real form.
type awardFinalistSlotFixture struct {
	text string
	slug string
}

// validAwardFinalistsFixture is one valid, fully-resolved submission across
// all 5 awards, each finalist trio matching
// awardFinalistsNHLPlayersYAML's own roster and each award's own eligible
// position.
func validAwardFinalistsFixture() map[string][3]awardFinalistSlotFixture {
	skaters := [3]awardFinalistSlotFixture{
		{text: "Connor McDavid", slug: "mcdavid-connor"},
		{text: "Nathan MacKinnon", slug: "mackinnon-nathan"},
		{text: "Nikita Kucherov", slug: "kucherov-nikita"},
	}
	return map[string][3]awardFinalistSlotFixture{
		"hart":           skaters,
		"art_ross":       skaters,
		"rocket_richard": skaters,
		"norris": {
			{text: "Cale Makar", slug: "makar-cale"},
			{text: "Quinn Hughes", slug: "hughes-quinn"},
			{text: "Zach Werenski", slug: "werenski-zach"},
		},
		"vezina": {
			{text: "Connor Hellebuyck", slug: "hellebuyck-connor"},
			{text: "Igor Shesterkin", slug: "shesterkin-igor"},
			{text: "Andrei Vasilevskiy", slug: "vasilevskiy-andrei"},
		},
	}
}

// awardFinalistTextFieldNameFixture/awardFinalistSlugFieldNameFixture
// mirror internal/web's own (unexported) awardFinalistTextFieldName/
// awardFinalistSlugFieldName naming convention, so this fixture posts to
// the exact same form field names the real sheet renders.
func awardFinalistTextFieldNameFixture(award string, slot int) string {
	return fmt.Sprintf("award_%s_text_%d", award, slot)
}

func awardFinalistSlugFieldNameFixture(award string, slot int) string {
	return fmt.Sprintf("award_%s_slug_%d", award, slot)
}

// awardFinalistsSetFixture is the Given-declared "awards" Prediction Set,
// waiting to be written into the scenario's data file the first time a step
// needs a running server.
type awardFinalistsSetFixture struct {
	deadline time.Time
}

// awardFinalistsScenarioState holds the fixtures and results for one
// award-finalists scenario. A fresh instance is created per scenario so
// state never leaks between runs. The store and server are built lazily
// (ensureReady) on the first request, since Given steps keep appending
// fixtures beforehand.
type awardFinalistsScenarioState struct {
	dataFile     string
	set          *awardFinalistsSetFixture
	priorSubmit  bool
	st           *store.Store
	server       *httptest.Server
	lastStatus   int
	lastBody     string
	lastLocation string
}

func newAwardFinalistsScenarioState() *awardFinalistsScenarioState {
	dir, err := os.MkdirTemp("", "fantasy-hockey-award-finalists-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	return &awardFinalistsScenarioState{dataFile: filepath.Join(dir, store.DataFileName)}
}

func (s *awardFinalistsScenarioState) close() {
	if s.server != nil {
		s.server.Close()
	}
	_ = os.RemoveAll(filepath.Dir(s.dataFile)) // best-effort cleanup of the scenario's temp dir
}

func (s *awardFinalistsScenarioState) theSignedInPlayerIs(name string) error {
	if strings.ToLower(name) != awardFinalistsPlayerID {
		return fmt.Errorf("no fixture for player %q; only %q is seeded", name, awardFinalistsPlayerName)
	}
	return nil
}

// theCanonicalNHLPlayerListIncludesASample is a no-op: ensureReady always
// seeds awardFinalistsNHLPlayersYAML's roster regardless - this step exists
// only so the Background reads clearly as a Given.
func (s *awardFinalistsScenarioState) theCanonicalNHLPlayerListIncludesASample() error {
	return nil
}

func (s *awardFinalistsScenarioState) theAwardsSetHasADeadline(deadlinePhrase string) error {
	deadline, err := parseRelativeDeadline(deadlinePhrase, time.Now().UTC())
	if err != nil {
		return err
	}
	s.set = &awardFinalistsSetFixture{deadline: deadline}
	return nil
}

func (s *awardFinalistsScenarioState) thePlayerAlreadySubmittedValidFinalistsForAll5Awards() error {
	s.priorSubmit = true
	return nil
}

// ensureReady lazily persists the Given-declared "awards" Prediction Set
// and the NHL Player roster, starts the real production web.NewServer
// handler around it, then - if the Background declared a prior submission -
// saves it directly through the store, the first time a step needs to make
// an HTTP call.
func (s *awardFinalistsScenarioState) ensureReady() error {
	if s.server != nil {
		return nil
	}
	if s.set == nil {
		return fmt.Errorf("no awards Prediction Set deadline declared")
	}

	seed := fmt.Sprintf("season: \"2026-27\"\nplayers:\n    - id: %s\n      name: %s\n      email: basti@example.com\nprediction_sets:\n    - id: awards\n      title: \"Player awards\"\n      subtitle: \"Hart, Norris, Vezina, Art Ross, Rocket\"\n      deadline_utc: %q\n      phase: before_season\n      upcoming: false\n%s",
		awardFinalistsPlayerID, awardFinalistsPlayerName, s.set.deadline.Format(time.RFC3339), awardFinalistsNHLPlayersYAML)
	if err := os.WriteFile(s.dataFile, []byte(seed), 0o600); err != nil {
		return fmt.Errorf("seed data file: %w", err)
	}

	st, err := store.New(s.dataFile)
	if err != nil {
		return fmt.Errorf("store.New: %w", err)
	}
	s.st = st

	if s.priorSubmit {
		finalists := make(map[string][]string, len(awardFinalistsOrder))
		for award, slots := range validAwardFinalistsFixture() {
			slugs := make([]string, 0, len(slots))
			for _, slot := range slots {
				slugs = append(slugs, slot.slug)
			}
			finalists[award] = slugs
		}
		if err := st.SaveAwardPicks(awardFinalistsPlayerID, finalists, time.Now().UTC()); err != nil {
			return fmt.Errorf("seed prior award picks: %w", err)
		}
	}

	s.server = httptest.NewServer(web.NewServer(st, noopSender, awardFinalistsSecret))
	return nil
}

// do requests method+path carrying the signed-in player's session cookie,
// with an optional urlencoded form body, without following any redirect,
// and records the result.
func (s *awardFinalistsScenarioState) do(method, path, body string) error {
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
	req.AddCookie(auth.IssueSessionCookie(awardFinalistsPlayerID, awardFinalistsSecret))

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

func (s *awardFinalistsScenarioState) thePlayerOpensTheAwardsPredictionSet() error {
	return s.do(http.MethodGet, "/predict/awards", "")
}

// postAwardsForm encodes slots into the same field names the real awards
// sheet renders and POSTs them. An award absent from slots simply submits 3
// blank text/slug pairs for it, matching what an untouched trophy group
// sends.
func (s *awardFinalistsScenarioState) postAwardsForm(slots map[string][3]awardFinalistSlotFixture) error {
	values := url.Values{}
	for _, award := range awardFinalistsOrder {
		pairs := slots[award]
		for i, slot := range pairs {
			values.Set(awardFinalistTextFieldNameFixture(award, i), slot.text)
			values.Set(awardFinalistSlugFieldNameFixture(award, i), slot.slug)
		}
	}
	return s.do(http.MethodPost, "/predict/awards", values.Encode())
}

func (s *awardFinalistsScenarioState) thePlayerSubmitsValidFinalistsForAll5Awards() error {
	return s.postAwardsForm(validAwardFinalistsFixture())
}

func (s *awardFinalistsScenarioState) thePlayerSubmitsValidFinalistsForOnlyTheHartAndNorrisAwards() error {
	full := validAwardFinalistsFixture()
	slots := map[string][3]awardFinalistSlotFixture{
		"hart":   full["hart"],
		"norris": full["norris"],
	}
	return s.postAwardsForm(slots)
}

func (s *awardFinalistsScenarioState) thePlayerSubmitsValidFinalistsForAll5AwardsButTypesAnUnresolvedNameForOneHartSlot() error {
	slots := validAwardFinalistsFixture()
	hart := slots["hart"]
	hart[1] = awardFinalistSlotFixture{text: "Not A Real Player", slug: ""}
	slots["hart"] = hart
	return s.postAwardsForm(slots)
}

func (s *awardFinalistsScenarioState) thePlayerSubmitsValidFinalistsForAll5AwardsButSubmitsAGoaliesSlugForAHartSlot() error {
	slots := validAwardFinalistsFixture()
	hart := slots["hart"]
	hart[0] = awardFinalistSlotFixture{text: "Connor Hellebuyck", slug: "hellebuyck-connor"} // a goalie's slug, Hart needs a skater.
	slots["hart"] = hart
	return s.postAwardsForm(slots)
}

func (s *awardFinalistsScenarioState) theAwardsSheetShowsFiveTrophyGroups() error {
	for _, award := range awardFinalistsOrder {
		if !strings.Contains(s.lastBody, `data-award="`+award+`"`) {
			return fmt.Errorf("expected a trophy group for %q, got %q", award, s.lastBody)
		}
	}
	return nil
}

func (s *awardFinalistsScenarioState) theAwardsSheetShowsNoTrophyGroupsGreenCheck() error {
	if strings.Contains(s.lastBody, "award-group--filled") {
		return fmt.Errorf("expected no trophy group's green check, got %q", s.lastBody)
	}
	return nil
}

func (s *awardFinalistsScenarioState) theAwardsSheetShowsTheButtonText(text string) error {
	want := ">" + text + "</button>"
	if !strings.Contains(s.lastBody, want) {
		return fmt.Errorf("expected the button text %q, got %q", text, s.lastBody)
	}
	return nil
}

// awardGroupFragment isolates award's own trophy-group markup (its
// data-award attribute through the next award's own data-award, or the
// position-options embeds, whichever comes first), so a caller can assert
// a value preselected within that group without matching a same-valued
// input rendered under a sibling award's own group - mirrors
// division_picks_steps_test.go's own divisionChipGroupFragment.
func (s *awardFinalistsScenarioState) awardGroupFragment(award string) (string, error) {
	marker := `data-award="` + award + `"`
	start := strings.Index(s.lastBody, marker)
	if start == -1 {
		return "", fmt.Errorf("expected a trophy group for %q, got %q", award, s.lastBody)
	}
	rest := s.lastBody[start+len(marker):]

	end := len(rest)
	for _, boundary := range []string{`data-award="`, `<script type="application/json"`} {
		if idx := strings.Index(rest, boundary); idx != -1 && idx < end {
			end = idx
		}
	}
	return rest[:end], nil
}

func (s *awardFinalistsScenarioState) theAwardsSheetShowsPreselectedFor(name, award string) error {
	fragment, err := s.awardGroupFragment(award)
	if err != nil {
		return err
	}
	want := `value="` + name + `"`
	if !strings.Contains(fragment, want) {
		return fmt.Errorf("expected %q preselected within %q's own trophy group, got %q", name, award, fragment)
	}
	return nil
}

func (s *awardFinalistsScenarioState) theAwardsSheetShowsTheTrophyGroupsGreenCheckFor(award string) error {
	want := `class="award-group award-group--filled" data-award="` + award + `"`
	if !strings.Contains(s.lastBody, want) {
		return fmt.Errorf("expected the trophy group for %q to show its green check, got %q", award, s.lastBody)
	}
	return nil
}

func (s *awardFinalistsScenarioState) theAwardsSheetShows(text string) error {
	if !strings.Contains(s.lastBody, text) {
		return fmt.Errorf("expected the awards sheet to show %q, got %q", text, s.lastBody)
	}
	return nil
}

func (s *awardFinalistsScenarioState) theAwardsSheetShowsAReadOnlyBanner() error {
	if !strings.Contains(s.lastBody, "closed-banner") {
		return fmt.Errorf("expected a read-only banner, got %q", s.lastBody)
	}
	return nil
}

func (s *awardFinalistsScenarioState) theAwardsSheetShowsNoSubmitButton() error {
	if strings.Contains(s.lastBody, `<button`) {
		return fmt.Errorf("expected no submit button, got %q", s.lastBody)
	}
	return nil
}

func (s *awardFinalistsScenarioState) theAwardPickResponseRedirectsTo(location string) error {
	if s.lastStatus != http.StatusFound {
		return fmt.Errorf("expected status %d, got %d: %s", http.StatusFound, s.lastStatus, s.lastBody)
	}
	if s.lastLocation != location {
		return fmt.Errorf("expected a redirect to %q, got %q", location, s.lastLocation)
	}
	return nil
}

func (s *awardFinalistsScenarioState) theAwardPickResponseStatusIs(want int) error {
	if s.lastStatus != want {
		return fmt.Errorf("expected status %d, got %d", want, s.lastStatus)
	}
	return nil
}

func (s *awardFinalistsScenarioState) thePlayersSavedFinalistsForAre(award, csv string) error {
	prediction, ok := s.st.FindAwardFinalists(awardFinalistsPlayerID, award)
	if !ok {
		return fmt.Errorf("expected a saved finalists row for %q, found none", award)
	}
	want := strings.Split(csv, ",")
	if len(prediction.FinalistSlugs) != len(want) {
		return fmt.Errorf("expected finalist slugs %v for %q, got %v", want, award, prediction.FinalistSlugs)
	}
	for i, slug := range want {
		if prediction.FinalistSlugs[i] != slug {
			return fmt.Errorf("expected finalist slugs %v for %q, got %v", want, award, prediction.FinalistSlugs)
		}
	}
	return nil
}

func (s *awardFinalistsScenarioState) thePlayerHasNoSavedFinalistsFor(award string) error {
	if _, ok := s.st.FindAwardFinalists(awardFinalistsPlayerID, award); ok {
		return fmt.Errorf("expected no saved finalists for %q, found some", award)
	}
	return nil
}

// thePredictScreenMarksTheAwardsSetAs asserts the Predict list's set row for
// "awards" shows status - deliberately worded (and regexed) differently
// from division_picks_steps_test.go's own "the Predict screen marks ... as
// ..." step (same assertion, a fixed id instead of a parameter) so the two
// features' step regexes can't collide when both are registered on the
// same godog.ScenarioContext.
func (s *awardFinalistsScenarioState) thePredictScreenMarksTheAwardsSetAs(status string) error {
	if err := s.do(http.MethodGet, "/predict", ""); err != nil {
		return err
	}
	marker := `id="predict-row-awards"`
	start := strings.Index(s.lastBody, marker)
	if start == -1 {
		return fmt.Errorf("expected a set row for %q, got %q", "awards", s.lastBody)
	}
	rest := s.lastBody[start:]
	end := strings.Index(rest, "</a>")
	if end == -1 {
		return fmt.Errorf("could not find the end of the set row for %q", "awards")
	}
	row := rest[:end]
	want := ">" + status + "</span>"
	if !strings.Contains(row, want) {
		return fmt.Errorf("expected the set row for %q to show status %q, got %q", "awards", status, row)
	}
	return nil
}

// InitializeAwardFinalistsScenario registers the award-finalists step
// definitions with GoDog.
func InitializeAwardFinalistsScenario(ctx *godog.ScenarioContext) {
	s := newAwardFinalistsScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the signed-in player for award finalists is "([^"]*)"$`, s.theSignedInPlayerIs)
	ctx.Step(`^the canonical NHL Player list includes a sample of skaters, defensemen, and goalies$`, s.theCanonicalNHLPlayerListIncludesASample)
	ctx.Step(`^the awards Prediction Set has a deadline "([^"]*)"$`, s.theAwardsSetHasADeadline)
	ctx.Step(`^the player already submitted valid finalists for all 5 awards$`, s.thePlayerAlreadySubmittedValidFinalistsForAll5Awards)
	ctx.Step(`^the player opens the awards Prediction Set$`, s.thePlayerOpensTheAwardsPredictionSet)
	ctx.Step(`^the player submits valid finalists for all 5 awards$`, s.thePlayerSubmitsValidFinalistsForAll5Awards)
	ctx.Step(`^the player submits valid finalists for only the Hart and Norris awards$`, s.thePlayerSubmitsValidFinalistsForOnlyTheHartAndNorrisAwards)
	ctx.Step(`^the player submits valid finalists for all 5 awards but types an unresolved name for one Hart slot$`, s.thePlayerSubmitsValidFinalistsForAll5AwardsButTypesAnUnresolvedNameForOneHartSlot)
	ctx.Step(`^the player submits valid finalists for all 5 awards but submits a goalie's slug for a Hart slot$`, s.thePlayerSubmitsValidFinalistsForAll5AwardsButSubmitsAGoaliesSlugForAHartSlot)
	ctx.Step(`^the awards sheet shows five trophy groups$`, s.theAwardsSheetShowsFiveTrophyGroups)
	ctx.Step(`^the awards sheet shows no trophy group's green check$`, s.theAwardsSheetShowsNoTrophyGroupsGreenCheck)
	ctx.Step(`^the awards sheet shows the button text "([^"]*)"$`, s.theAwardsSheetShowsTheButtonText)
	ctx.Step(`^the awards sheet shows "([^"]*)" preselected for "([^"]*)"$`, s.theAwardsSheetShowsPreselectedFor)
	ctx.Step(`^the awards sheet shows the trophy group's green check for "([^"]*)"$`, s.theAwardsSheetShowsTheTrophyGroupsGreenCheckFor)
	ctx.Step(`^the awards sheet shows "([^"]*)"$`, s.theAwardsSheetShows)
	ctx.Step(`^the awards sheet shows a read-only banner$`, s.theAwardsSheetShowsAReadOnlyBanner)
	ctx.Step(`^the awards sheet shows no submit button$`, s.theAwardsSheetShowsNoSubmitButton)
	ctx.Step(`^the award pick response redirects to "([^"]*)"$`, s.theAwardPickResponseRedirectsTo)
	ctx.Step(`^the award pick response status is (\d+)$`, s.theAwardPickResponseStatusIs)
	ctx.Step(`^the player's saved finalists for "([^"]*)" are "([^"]*)"$`, s.thePlayersSavedFinalistsForAre)
	ctx.Step(`^the player has no saved finalists for "([^"]*)"$`, s.thePlayerHasNoSavedFinalistsFor)
	ctx.Step(`^the Predict screen marks the awards set as "([^"]*)"$`, s.thePredictScreenMarksTheAwardsSetAs)
}
