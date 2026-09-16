package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// renderedGenericCodeErrorText is genericCodeErrorText as html/template
// renders it into the response body (its apostrophe comes out
// HTML-escaped).
var renderedGenericCodeErrorText = html.EscapeString(genericCodeErrorText)

// testSecret signs session cookies for every test server built in this file.
const testSecret = "test-session-secret"

// noopSender never sends anything and never fails, for tests that don't
// care about the outgoing email itself.
func noopSender(_, _, _ string) error { return nil }

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
`
	path := filepath.Join(dir, store.DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return st
}

// newTestStoreWithPredictionSets seeds a store with one player and the
// given raw prediction_sets YAML block (indented as a top-level document
// entry), for tests exercising Predict's phase-grouped rendering.
func newTestStoreWithPredictionSets(t *testing.T, predictionSetsYAML string) *store.Store {
	t.Helper()
	dir := t.TempDir()
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
prediction_sets:
` + predictionSetsYAML
	path := filepath.Join(dir, store.DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return st
}

// newTestStoreWithPredictionSetsAndTeams seeds a store with one player, the
// given raw prediction_sets YAML block, and a small representative sample of
// teams spanning all four divisions - for tests exercising the cup/
// presidents pick-entry sheet's dropdown grouping.
func newTestStoreWithPredictionSetsAndTeams(t *testing.T, predictionSetsYAML string) *store.Store {
	t.Helper()
	dir := t.TempDir()
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
prediction_sets:
` + predictionSetsYAML + `teams:
    - id: TOR
      name: Toronto Maple Leafs
      conference: Eastern
      division: Atlantic
    - id: WSH
      name: Washington Capitals
      conference: Eastern
      division: Metropolitan
    - id: COL
      name: Colorado Avalanche
      conference: Western
      division: Central
    - id: VGK
      name: Vegas Golden Knights
      conference: Western
      division: Pacific
`
	path := filepath.Join(dir, store.DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return st
}

// cupPredictionSetSeed is one "cup" Prediction Set's raw prediction_sets
// YAML block with deadline as its deadline_utc, for tests exercising the
// cup/presidents pick-entry sheet.
func cupPredictionSetSeed(deadline time.Time) string {
	return fmt.Sprintf(`    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))
}

// presidentsPredictionSetSeed is one "presidents" Prediction Set's raw
// prediction_sets YAML block with deadline as its deadline_utc - the cup/
// presidents pick-entry sheet's other pickable id, so tests can prove the
// shared mechanic isn't only exercised through "cup".
func presidentsPredictionSetSeed(deadline time.Time) string {
	return fmt.Sprintf(`    - id: presidents
      title: Presidents' Trophy
      subtitle: Best regular-season record
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))
}

func TestPredictShouldRenderBothSectionsWithEverySetsFields(t *testing.T) {
	now := time.Now().UTC()
	seed := fmt.Sprintf(`    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: %q
      phase: before_season
      upcoming: false
    - id: scf
      title: Stanley Cup final
      subtitle: Set once the finalists are known
      deadline_utc: %q
      phase: playoffs
      upcoming: true
`, now.Add(5*24*time.Hour).Format(time.RFC3339), now.Add(200*24*time.Hour).Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Before the season", "Playoffs",
		"Cup champion", "Your Stanley Cup winner",
		"Stanley Cup final", "Set once the finalists are known",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected the Predict page to contain %q, got %q", want, body)
		}
	}

	// strings.Contains alone wouldn't catch a set rendering under the wrong
	// phase section, so also check each set's title falls between its own
	// section's header and the next one - a structural proof, not just a
	// presence check.
	assertMarkersInOrder(t, body,
		"<span>Before the season</span>", "Cup champion",
		"<span>Playoffs</span>", "Stanley Cup final")
}

// assertMarkersInOrder fails t unless every marker appears in body, each one
// strictly after the previous, in the given order.
func assertMarkersInOrder(t *testing.T, body string, markers ...string) {
	t.Helper()
	last := -1
	for _, marker := range markers {
		idx := strings.Index(body, marker)
		if idx == -1 {
			t.Fatalf("expected to find %q in %q", marker, body)
		}
		if idx <= last {
			t.Fatalf("expected %q to appear after the preceding marker, got %q", marker, body)
		}
		last = idx
	}
}

func TestPredictShouldShowAnOpenPillChevronAndLinkForASetWithinItsWindow(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	seed := fmt.Sprintf(`    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `class="status-pill status-pill--open">Open</span>`) {
		t.Errorf("expected an Open status pill, got %q", body)
	}
	if !strings.Contains(body, "in 5 days") {
		t.Errorf("expected the countdown to read \"in 5 days\", got %q", body)
	}
	if !strings.Contains(body, `<a id="predict-row-cup" href="/predict/cup" class="set-row set-row--open">`) {
		t.Errorf("expected an actionable row linking to /predict/cup, got %q", body)
	}
	if !strings.Contains(body, `<path d="M9 6l6 6-6 6"/>`) {
		t.Errorf("expected a chevron affordance on an actionable row, got %q", body)
	}
}

func TestPredictShouldShowAClosedPillAndCountdownForASetPastItsDeadline(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	seed := fmt.Sprintf(`    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `class="status-pill status-pill--closed">Closed</span>`) {
		t.Errorf("expected a Closed status pill, got %q", body)
	}
	if !strings.Contains(body, "closed") {
		t.Errorf("expected the countdown to read \"closed\", got %q", body)
	}
	// A Closed set is still actionable (AC: "an Open, Submitted, or Closed
	// set... shows a chevron and links").
	if !strings.Contains(body, `<a id="predict-row-cup" href="/predict/cup" class="set-row set-row--open">`) {
		t.Errorf("expected a Closed row to still be actionable, got %q", body)
	}
}

func TestPredictShouldDimAndLockAnUpcomingSetInsteadOfLinkingIt(t *testing.T) {
	seed := fmt.Sprintf(`    - id: cf
      title: Conference finals
      subtitle: Set once round 2 ends
      deadline_utc: %q
      phase: playoffs
      upcoming: true
`, time.Now().UTC().Add(200*24*time.Hour).Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `class="status-pill status-pill--upcoming">Upcoming</span>`) {
		t.Errorf("expected an Upcoming status pill, got %q", body)
	}
	if !strings.Contains(body, `<div id="predict-row-cf" class="set-row set-row--upcoming">`) {
		t.Errorf("expected a dimmed, non-link row for an Upcoming set, got %q", body)
	}
	if strings.Contains(body, `<a href="/predict/cf"`) {
		t.Errorf("expected an Upcoming set never to render as a link, got %q", body)
	}
	if !strings.Contains(body, `<rect x="5" y="11" width="14" height="9" rx="2"/>`) {
		t.Errorf("expected a lock affordance on an Upcoming row, got %q", body)
	}
}

func TestPredictShouldReadTodayAndTomorrowForNearDeadlines(t *testing.T) {
	now := time.Now().UTC()
	seed := fmt.Sprintf(`    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: %q
      phase: before_season
      upcoming: false
    - id: presidents
      title: Presidents' Trophy
      subtitle: Best regular-season record
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, now.Add(2*time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "&middot; today") {
		t.Errorf("expected a deadline later today to read \"today\", got %q", body)
	}
	if !strings.Contains(body, "&middot; tomorrow") {
		t.Errorf("expected a deadline tomorrow to read \"tomorrow\", got %q", body)
	}
}

func TestPredictShouldNeverWriteBackToTheStoreFile(t *testing.T) {
	dir := t.TempDir()
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
prediction_sets:
    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: "2026-10-06T17:00:00Z"
      phase: before_season
      upcoming: false
`
	path := filepath.Join(dir, store.DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	for _, p := range []string{"/predict", "/predict/cup", "/predict/does-not-exist"} {
		req := httptest.NewRequest("GET", p, nil)
		req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file after requests: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("expected %s to stay untouched, got diff:\nbefore:\n%s\nafter:\n%s", store.DataFileName, before, after)
	}
}

// TestBuildPredictPhasesShouldSkipMalformedRowsWhileKeepingValidSiblings
// covers buildPredictPhases' two skip-and-log paths (an unparseable
// deadline_utc, an unrecognized phase) directly, asserting each malformed
// row is dropped while a valid sibling row in the same seed still renders.
func TestBuildPredictPhasesShouldSkipMalformedRowsWhileKeepingValidSiblings(t *testing.T) {
	now := time.Now().UTC()
	validDeadline := now.Add(5 * 24 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name    string
		seed    string
		want    []string
		missing []string
	}{
		{
			name: "unparseable deadline_utc",
			seed: fmt.Sprintf(`    - id: bad-deadline
      title: Bad deadline
      subtitle: Should be skipped
      deadline_utc: "not-a-timestamp"
      phase: before_season
      upcoming: false
    - id: good
      title: Good set
      subtitle: Should render
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, validDeadline),
			want:    []string{"good"},
			missing: []string{"bad-deadline"},
		},
		{
			name: "unrecognized phase",
			seed: fmt.Sprintf(`    - id: bad-phase
      title: Bad phase
      subtitle: Should be skipped
      deadline_utc: %q
      phase: mystery
      upcoming: false
    - id: good
      title: Good set
      subtitle: Should render
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, validDeadline, validDeadline),
			want:    []string{"good"},
			missing: []string{"bad-phase"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newTestStoreWithPredictionSets(t, tt.seed)

			phases := buildPredictPhases(st, "basti", now)

			rendered := make(map[string]bool)
			for _, v := range append(phases.BeforeSeason, phases.Playoffs...) {
				rendered[v.ID] = true
			}
			for _, id := range tt.want {
				if !rendered[id] {
					t.Errorf("expected %q to still render, got rendered ids %v", id, rendered)
				}
			}
			for _, id := range tt.missing {
				if rendered[id] {
					t.Errorf("expected %q to be skipped, got rendered ids %v", id, rendered)
				}
			}
		})
	}
}

// TestNewTeamOptionsShouldConvertEveryTeamIntoTheSharedEmbedShape covers
// AD-19: every entry gets exactly {"id": "<abbr>", "label": "<name>"},
// preserving the input order, so 2.3/2.4's embedding sites can rely on it.
func TestNewTeamOptionsShouldConvertEveryTeamIntoTheSharedEmbedShape(t *testing.T) {
	teams := []store.Team{
		{ID: "TOR", Name: "Toronto Maple Leafs", Conference: "Eastern", Division: "Atlantic"},
		{ID: "VGK", Name: "Vegas Golden Knights", Conference: "Western", Division: "Pacific"},
	}

	options := newTeamOptions(teams)

	out, err := json.Marshal(options)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `[{"id":"TOR","label":"Toronto Maple Leafs"},{"id":"VGK","label":"Vegas Golden Knights"}]`
	if string(out) != want {
		t.Errorf("expected JSON %q, got %q", want, string(out))
	}
}

func TestNewTeamOptionsShouldReturnAnEmptySliceForAnEmptyInput(t *testing.T) {
	options := newTeamOptions(nil)

	if len(options) != 0 {
		t.Errorf("expected an empty slice, got %v", options)
	}
}

// TestGetPredictSheetShouldRenderTheStubPageForAKnownSet covers a Prediction
// Set id other than "cup"/"presidents" - Story 2.3 gives only those two ids a
// real pick-entry form; every other id keeps rendering the unchanged stub
// (Boundaries & Constraints).
func TestGetPredictSheetShouldRenderTheStubPageForAKnownSet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	seed := fmt.Sprintf(`    - id: divisions
      title: Division picks
      subtitle: Playoff teams & division winners
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict/divisions", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Division picks") {
		t.Errorf("expected the sheet title, got %q", body)
	}
	if !strings.Contains(body, "in 5 days") {
		t.Errorf("expected the formatted countdown, got %q", body)
	}
	if !strings.Contains(body, "Not available yet.") {
		t.Errorf("expected the static stub body, got %q", body)
	}
	if strings.Contains(body, `<button`) || strings.Contains(body, `<form`) || strings.Contains(body, `<select`) {
		t.Errorf("expected no pick-entry form or action bar on the stub page, got %q", body)
	}
}

// TestGetPredictSheetShouldRenderTheStubPageForAClosedSet covers a
// non-cup/presidents id whose deadline has passed - it still gets the static
// stub, never the cup/presidents closed read-only banner.
func TestGetPredictSheetShouldRenderTheStubPageForAClosedSet(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	seed := fmt.Sprintf(`    - id: divisions
      title: Division picks
      subtitle: Playoff teams & division winners
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict/divisions", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Division picks") {
		t.Errorf("expected the sheet title, got %q", body)
	}
	if !strings.Contains(body, "closed") {
		t.Errorf("expected the countdown to read \"closed\", got %q", body)
	}
	if strings.Contains(body, "closed-banner") {
		t.Errorf("expected no cup/presidents read-only banner on the stub page, got %q", body)
	}
}

func TestGetPredictSheetShouldReturn404ForASetWithAnUnparseableDeadline(t *testing.T) {
	seed := `    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: "not-a-timestamp"
      phase: before_season
      upcoming: false
`

	req := httptest.NewRequest("GET", "/predict/cup", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for a set with an unparseable deadline_utc, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetPredictSheetShouldRedirectToLoginWithNoSessionCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/predict/cup", nil)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertRedirectsToLoginWithNoCookie(t, rec)
}

func TestGetPredictSheetShouldReturn404ForAnUnknownSetID(t *testing.T) {
	req := httptest.NewRequest("GET", "/predict/does-not-exist", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for an unknown prediction set id, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetPredictSheetShouldRenderThePickFormWithNoTeamPreselectedWhenNoPriorPick(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))

	req := httptest.NewRequest("GET", "/predict/cup", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<select name="team_id" aria-label="Cup champion pick" required >`) {
		t.Errorf("expected an enabled, required, labeled team select, got %q", body)
	}
	if !strings.Contains(body, `<form class="pick-form" method="post" action="/predict/cup">`) {
		t.Errorf("expected the form to submit to /predict/cup, got %q", body)
	}
	if !strings.Contains(body, `<option value="" disabled selected>Choose a team</option>`) {
		t.Errorf("expected no team preselected, got %q", body)
	}
	if !strings.Contains(body, `<optgroup label="Atlantic">`) || !strings.Contains(body, `<option value="TOR" >Toronto Maple Leafs</option>`) {
		t.Errorf("expected teams grouped by division, got %q", body)
	}
	if !strings.Contains(body, `>Submit predictions</button>`) {
		t.Errorf("expected the button to read \"Submit predictions\", got %q", body)
	}
	if strings.Contains(body, "Update predictions") {
		t.Errorf("expected no \"Update predictions\" text with no prior pick, got %q", body)
	}
}

func TestGetPredictSheetShouldPreselectTheSavedPickAndReadUpdatePredictions(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))
	if err := st.SavePrediction("basti", store.KindCupChampion, "TOR", time.Now().UTC()); err != nil {
		t.Fatalf("seed SavePrediction: %v", err)
	}

	req := httptest.NewRequest("GET", "/predict/cup", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `<option value="TOR" selected>Toronto Maple Leafs</option>`) {
		t.Errorf("expected the saved pick preselected, got %q", body)
	}
	if !strings.Contains(body, `>Update predictions</button>`) {
		t.Errorf("expected the button to read \"Update predictions\", got %q", body)
	}
}

func TestGetPredictSheetShouldOrderOptgroupsAtlanticMetropolitanCentralPacific(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))

	req := httptest.NewRequest("GET", "/predict/cup", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	assertMarkersInOrder(t, rec.Body.String(),
		`<optgroup label="Atlantic">`, `<optgroup label="Metropolitan">`, `<optgroup label="Central">`, `<optgroup label="Pacific">`)
}

func TestGetPredictSheetShouldRenderAReadOnlyBannerAndDisabledSelectWhenClosed(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))
	if err := st.SavePrediction("basti", store.KindCupChampion, "TOR", time.Now().UTC().Add(-48*time.Hour)); err != nil {
		t.Fatalf("seed SavePrediction: %v", err)
	}

	req := httptest.NewRequest("GET", "/predict/cup", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `<select name="team_id" aria-label="Cup champion pick" required disabled>`) {
		t.Errorf("expected the select to render disabled and labeled, got %q", body)
	}
	if !strings.Contains(body, `<option value="TOR" selected>Toronto Maple Leafs</option>`) {
		t.Errorf("expected the saved pick still shown, got %q", body)
	}
	if !strings.Contains(body, "closed-banner") {
		t.Errorf("expected the read-only banner, got %q", body)
	}
	if strings.Contains(body, `<button`) {
		t.Errorf("expected no action bar (submit button) on a closed set, got %q", body)
	}
	if strings.Contains(body, "Editable until the deadline.") {
		t.Errorf("expected no hint caption on a closed set, got %q", body)
	}
}

func TestGetPredictSheetShouldNeverCreateAPredictionRowMerelyByOpeningTheSheet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))

	req := httptest.NewRequest("GET", "/predict/cup", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if _, ok := st.FindPrediction("basti", store.KindCupChampion); ok {
		t.Error("expected no Prediction row to be force-created merely by opening the sheet")
	}
}

// postSheet POSTs teamID to /predict/{id} as the signed-in test player,
// following no redirect, and returns the recorded response.
func postSheet(t *testing.T, handler http.Handler, id, teamID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/predict/"+id, strings.NewReader("team_id="+teamID))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestPostPredictSheetShouldSaveAValidPickAndRedirectToPredict(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postSheet(t, handler, "cup", "TOR")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/predict" {
		t.Errorf("expected a redirect to %q, got %q", "/predict", rec.Header().Get("Location"))
	}

	prediction, ok := st.FindPrediction("basti", store.KindCupChampion)
	if !ok {
		t.Fatal("expected a saved Prediction row")
	}
	if prediction.TeamID != "TOR" {
		t.Errorf("expected team id %q, got %q", "TOR", prediction.TeamID)
	}
}

// TestPostPredictSheetShouldSaveAValidPickForPresidentsAndShowSubmittedOnReload
// proves the shared pick-entry mechanic end-to-end for "presidents" too, not
// only "cup": save, redirect, a reopened sheet pre-filled with "Update
// predictions", and the Predict list showing Submitted for that row.
func TestPostPredictSheetShouldSaveAValidPickForPresidentsAndShowSubmittedOnReload(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, presidentsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postSheet(t, handler, "presidents", "VGK")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/predict" {
		t.Errorf("expected a redirect to %q, got %q", "/predict", rec.Header().Get("Location"))
	}

	prediction, ok := st.FindPrediction("basti", store.KindPresidentsTrophy)
	if !ok {
		t.Fatal("expected a saved Prediction row for presidents")
	}
	if prediction.TeamID != "VGK" {
		t.Errorf("expected team id %q, got %q", "VGK", prediction.TeamID)
	}

	reopenReq := httptest.NewRequest("GET", "/predict/presidents", nil)
	reopenReq.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	reopenRec := httptest.NewRecorder()
	handler.ServeHTTP(reopenRec, reopenReq)

	reopenBody := reopenRec.Body.String()
	if !strings.Contains(reopenBody, `<form class="pick-form" method="post" action="/predict/presidents">`) {
		t.Errorf("expected the presidents sheet to submit to /predict/presidents, not /predict/cup, got %q", reopenBody)
	}
	if !strings.Contains(reopenBody, `<option value="VGK" selected>Vegas Golden Knights</option>`) {
		t.Errorf("expected the saved presidents pick preselected on reload, got %q", reopenBody)
	}
	if !strings.Contains(reopenBody, `>Update predictions</button>`) {
		t.Errorf("expected the button to read \"Update predictions\" on reload, got %q", reopenBody)
	}

	predictReq := httptest.NewRequest("GET", "/predict", nil)
	predictReq.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	predictRec := httptest.NewRecorder()
	handler.ServeHTTP(predictRec, predictReq)

	if !strings.Contains(predictRec.Body.String(), `<a id="predict-row-presidents" href="/predict/presidents" class="set-row set-row--submitted">`) {
		t.Errorf("expected the presidents row to show Submitted on the Predict list, got %q", predictRec.Body.String())
	}
}

func TestPostPredictSheetShouldUpdateAnExistingPickInPlaceOnResubmission(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))
	if err := st.SavePrediction("basti", store.KindCupChampion, "TOR", time.Now().UTC()); err != nil {
		t.Fatalf("seed SavePrediction: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	rec := postSheet(t, handler, "cup", "VGK")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}

	prediction, ok := st.FindPrediction("basti", store.KindCupChampion)
	if !ok {
		t.Fatal("expected the Prediction row to still be found")
	}
	if prediction.TeamID != "VGK" {
		t.Errorf("expected the pick to be updated to %q, got %q", "VGK", prediction.TeamID)
	}
	// SavePrediction's own store-level tests cover the "updates in place,
	// never appends a duplicate" guarantee directly against the document;
	// this handler-level test only needs to prove the visible effect: the
	// one (playerID, kind) row now reflects the resubmitted pick.
}

func TestPostPredictSheetShouldRejectAMissingTeamIDAndSaveNothing(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postSheet(t, handler, "cup", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), invalidTeamErrorText) {
		t.Errorf("expected the inline error caption, got %q", rec.Body.String())
	}
	if _, ok := st.FindPrediction("basti", store.KindCupChampion); ok {
		t.Error("expected nothing to be saved for a missing team id")
	}
}

func TestPostPredictSheetShouldRejectAnUnknownTeamIDAndSaveNothing(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postSheet(t, handler, "cup", "ZZZ")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), invalidTeamErrorText) {
		t.Errorf("expected the inline error caption, got %q", rec.Body.String())
	}
	if _, ok := st.FindPrediction("basti", store.KindCupChampion); ok {
		t.Error("expected nothing to be saved for an unknown team id")
	}
}

func TestPostPredictSheetShouldRejectAfterTheDeadlineWithNoOverride(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postSheet(t, handler, "cup", "TOR")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if _, ok := st.FindPrediction("basti", store.KindCupChampion); ok {
		t.Error("expected nothing to be saved after the deadline")
	}
}

func TestPostPredictSheetShouldReturn404WhenThePickableIDHasNoMatchingSet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	handler := NewServer(newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline)), noopSender, testSecret)

	// "presidents" is a pickable kind, but this store only seeded "cup".
	rec := postSheet(t, handler, "presidents", "TOR")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestPostPredictSheetShouldReturn404ForASetWithAnUnparseableDeadline(t *testing.T) {
	seed := `    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: "not-a-timestamp"
      phase: before_season
      upcoming: false
`
	handler := NewServer(newTestStoreWithPredictionSetsAndTeams(t, seed), noopSender, testSecret)

	rec := postSheet(t, handler, "cup", "TOR")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for a set with an unparseable deadline_utc, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestPostPredictSheetShouldReturn500ForAMalformedRequestBody(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	handler := NewServer(newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline)), noopSender, testSecret)

	req := httptest.NewRequest("POST", "/predict/cup", strings.NewReader("team_id=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d for a malformed request body, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestPostPredictSheetShouldReturn500WhenTheStoreWriteFails(t *testing.T) {
	dir := t.TempDir()
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
prediction_sets:
` + cupPredictionSetSeed(deadline) + `teams:
    - id: TOR
      name: Toronto Maple Leafs
      conference: Eastern
      division: Atlantic
`
	path := filepath.Join(dir, store.DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	// Remove the directory out from under the store so SavePrediction's
	// atomic write-and-rename fails, simulating a disk write failure - the
	// same technique internal/store's own tests use, mirrored here to prove
	// handleSheetSubmit surfaces it as a 500 rather than silently redirecting.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	rec := postSheet(t, handler, "cup", "TOR")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d when the store write fails, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestPostPredictSheetShouldReturn404ForANonPickableSetID(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	seed := fmt.Sprintf(`    - id: divisions
      title: Division picks
      subtitle: Playoff teams & division winners
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))
	handler := NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret)

	rec := postSheet(t, handler, "divisions", "TOR")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestPostPredictSheetShouldReturn404ForAnUnknownSetID(t *testing.T) {
	handler := NewServer(newTestStore(t), noopSender, testSecret)

	rec := postSheet(t, handler, "does-not-exist", "TOR")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestPostPredictSheetShouldRedirectToLoginWithNoSessionCookie(t *testing.T) {
	req := httptest.NewRequest("POST", "/predict/cup", strings.NewReader("team_id=TOR"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertRedirectsToLoginWithNoCookie(t, rec)
}

func TestPredictShouldShowSubmittedStatusAndAccentAfterASavedPick(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline))
	if err := st.SavePrediction("basti", store.KindCupChampion, "TOR", time.Now().UTC()); err != nil {
		t.Fatalf("seed SavePrediction: %v", err)
	}

	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `class="status-pill status-pill--submitted">Submitted</span>`) {
		t.Errorf("expected a Submitted status pill, got %q", body)
	}
	if !strings.Contains(body, `<a id="predict-row-cup" href="/predict/cup" class="set-row set-row--submitted">`) {
		t.Errorf("expected the submitted accent border, got %q", body)
	}
}

func TestNewServerShouldReturnOKForTheHomePage(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

// shellRouteTests is every shell route this server registers, alongside the
// bottom-nav tab that should render active and the unique "Coming soon"
// fragment that route's content should show. "/" aliases to Predict.
var shellRouteTests = []struct {
	path       string
	activeTab  string
	navLabel   string
	messageFor string
}{
	{"/", "predict", "Predict", "Before the season"},
	{"/predict", "predict", "Predict", "Before the season"},
	{"/leaderboard", "leaderboard", "Leaderboard", "The leaderboard is coming soon."},
	{"/compare", "compare", "Compare", "Player comparison is coming soon."},
}

func TestNewServerShouldRenderTheShellForEveryDestination(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			if rec.Code != 200 {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}
			body := rec.Body.String()
			assertShellHeader(t, body)
			if !strings.Contains(body, tt.messageFor) {
				t.Errorf("expected the %q content, got %q", tt.messageFor, body)
			}
			assertActiveTab(t, body, tt.navLabel)
			assertInactiveTabs(t, body, tt.activeTab)
			assertLogoutControl(t, body)
		})
	}
}

// assertShellHeader checks the header shows the logged-in player's name and
// the formatted season label.
func assertShellHeader(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, "Basti") {
		t.Errorf("expected the header to show the player's name, got %q", body)
	}
	if !strings.Contains(body, "NHL 2026–27") {
		t.Errorf("expected the header to show the formatted season, got %q", body)
	}
}

// navHrefs maps each nav label to its route, so tests can tie a label to
// its own anchor's active/inactive state without depending on the icon
// markup rendered between the anchor's opening tag and its label.
var navHrefs = map[string]string{
	"Predict":     "/predict",
	"Leaderboard": "/leaderboard",
	"Compare":     "/compare",
}

// assertActiveTab checks navLabel's anchor renders as the active tab, in
// `ice` with aria-current="page" for assistive technology.
func assertActiveTab(t *testing.T, body, navLabel string) {
	t.Helper()
	activeOpenTag := `<a href="` + navHrefs[navLabel] + `" class="nav-item active" aria-current="page">`
	if !strings.Contains(body, activeOpenTag) {
		t.Errorf("expected %q's anchor to render active with aria-current=\"page\", got %q", navLabel, body)
	}
	if !strings.Contains(body, `<span class="nav-label">`+navLabel+`</span>`) {
		t.Errorf("expected %q to render as a nav label, got %q", navLabel, body)
	}
}

// assertInactiveTabs checks every tab other than activeTab renders as
// inactive (muted, no aria-current).
func assertInactiveTabs(t *testing.T, body, activeTab string) {
	t.Helper()
	for _, other := range shellRouteTests {
		if other.activeTab == activeTab {
			continue
		}
		inactiveOpenTag := `<a href="` + navHrefs[other.navLabel] + `" class="nav-item">`
		if !strings.Contains(body, inactiveOpenTag) {
			t.Errorf("expected %q's anchor to render inactive (muted, no aria-current), got %q", other.navLabel, body)
		}
	}
}

// assertLogoutControl checks the shell renders a visible logout button wired
// to POST /logout, so a regression that broke or removed it would be caught.
func assertLogoutControl(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, `<form class="logout-form" method="post" action="/logout">`) {
		t.Errorf("expected the rendered shell to contain the logout form wired to POST /logout, got %q", body)
	}
	if !strings.Contains(body, `<button type="submit" class="logout-btn">Log out</button>`) {
		t.Errorf("expected the rendered shell to contain the visible logout button, got %q", body)
	}
}

func TestNewServerShouldRedirectToLoginForEveryShellDestinationWithNoSessionCookie(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			assertRedirectsToLoginWithNoCookie(t, rec)
		})
	}
}

// TestNewServerShouldRedirectToLoginForEveryShellDestinationWithAnIdleExpiredSession
// extends the idle-expired-session coverage (already proven for GET /{$} by
// TestGetHomeShouldRedirectToLoginForAnIdleExpiredSession) to the 3 new shell
// routes, matching the I/O & Edge-Case Matrix's "any shell route" scope.
func TestNewServerShouldRedirectToLoginForEveryShellDestinationWithAnIdleExpiredSession(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.AddCookie(auth.IssueSessionCookieAt("basti", time.Now().UTC().Add(-31*time.Minute), testSecret))
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			assertRedirectsToLoginWithNoCookie(t, rec)
		})
	}
}

// TestNewServerShouldRedirectToLoginForEveryShellDestinationWithATamperedSessionCookie
// extends the tampered-cookie coverage to the 3 new shell routes.
func TestNewServerShouldRedirectToLoginForEveryShellDestinationWithATamperedSessionCookie(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			c := auth.IssueSessionCookie("basti", testSecret)
			last := c.Value[len(c.Value)-1]
			replacement := byte('0')
			if last == replacement {
				replacement = '1'
			}
			c.Value = c.Value[:len(c.Value)-1] + string(replacement)

			req := httptest.NewRequest("GET", tt.path, nil)
			req.AddCookie(c)
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			assertRedirectsToLoginWithNoCookie(t, rec)
		})
	}
}

// TestNewServerShouldRedirectToLoginForEveryShellDestinationWithAnEmptyPlayerIDSession
// extends the empty-player-id coverage to the 3 new shell routes.
func TestNewServerShouldRedirectToLoginForEveryShellDestinationWithAnEmptyPlayerIDSession(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.AddCookie(auth.IssueSessionCookieAt("", time.Now().UTC(), testSecret))
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			assertRedirectsToLoginWithNoCookie(t, rec)
		})
	}
}

func TestNewServerShouldDegradeTheHeaderWithoutPanickingForAStalePlayerID(t *testing.T) {
	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("a-deleted-player-id", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200 (no crash) for a stale player id, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Basti") {
		t.Errorf("expected no real player name to leak into the degraded header, got %q", body)
	}
	if !strings.Contains(body, `<span class="header-name">&mdash;</span>`) {
		t.Errorf("expected the header to degrade to a neutral placeholder, got %q", body)
	}
}

func TestNewServerShouldReturn404ForAnUnknownPath(t *testing.T) {
	req := httptest.NewRequest("GET", "/unknown", nil)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("expected status 404 for an unknown path, got %d", rec.Code)
	}
}

// assertRedirectsToLoginWithNoCookie asserts rec is a 302 to /login carrying
// no Set-Cookie header - a regression that both redirected and leaked/
// re-issued a cookie on this path would otherwise go unnoticed.
func assertRedirectsToLoginWithNoCookie(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/login" {
		t.Errorf("expected a redirect to %q, got %q", "/login", rec.Header().Get("Location"))
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("expected no cookie to be set on a redirect, got %v", cookies)
	}
}

func TestGetHomeShouldRedirectToLoginWithNoSessionCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertRedirectsToLoginWithNoCookie(t, rec)
}

func TestGetHomeShouldRedirectToLoginForAnIdleExpiredSession(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(auth.IssueSessionCookieAt("basti", time.Now().UTC().Add(-31*time.Minute), testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertRedirectsToLoginWithNoCookie(t, rec)
}

func TestGetHomeShouldRedirectToLoginForATamperedSessionCookie(t *testing.T) {
	c := auth.IssueSessionCookie("basti", testSecret)
	last := c.Value[len(c.Value)-1]
	replacement := byte('0')
	if last == replacement {
		replacement = '1'
	}
	c.Value = c.Value[:len(c.Value)-1] + string(replacement)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertRedirectsToLoginWithNoCookie(t, rec)
}

func TestGetHomeShouldRedirectToLoginForAnEmptyPlayerIDSession(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(auth.IssueSessionCookieAt("", time.Now().UTC(), testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertRedirectsToLoginWithNoCookie(t, rec)
}

func TestGetHomeShouldReIssueTheSessionCookieOnAValidRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	before := time.Now().UTC()
	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)
	after := time.Now().UTC()

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("expected Cache-Control %q on an authenticated response, got %q", "no-store", got)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie to be re-issued, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != auth.SessionCookieName {
		t.Errorf("expected cookie name %q, got %q", auth.SessionCookieName, c.Name)
	}

	playerID, issuedAt, ok := auth.ParseSessionCookie(c, testSecret)
	if !ok {
		t.Fatal("expected the re-issued cookie to parse successfully")
	}
	if playerID != "basti" {
		t.Errorf("expected player id %q, got %q", "basti", playerID)
	}
	if issuedAt.Before(before.Add(-time.Second)) || issuedAt.After(after.Add(time.Second)) {
		t.Errorf("expected a freshly re-issued issued_at, got %v", issuedAt)
	}
}

func TestGetLoginShouldRenderTheEmailEntryScreen(t *testing.T) {
	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Email address") {
		t.Errorf("expected the email-entry screen to be rendered, got %q", rec.Body.String())
	}
}

func TestGetLoginShouldRedirectToTheShellForAnAlreadyAuthenticatedSession(t *testing.T) {
	req := httptest.NewRequest("GET", "/login", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/" {
		t.Errorf("expected a redirect to %q, got %q", "/", rec.Header().Get("Location"))
	}
}

func TestGetLoginShouldRenderTheEmailEntryScreenForAnIdleExpiredSession(t *testing.T) {
	req := httptest.NewRequest("GET", "/login", nil)
	req.AddCookie(auth.IssueSessionCookieAt("basti", time.Now().UTC().Add(-31*time.Minute), testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Email address") {
		t.Errorf("expected the email-entry screen to be rendered, got %q", rec.Body.String())
	}
}

func TestPostLoginShouldRenderTheCodeEntryScreenOnAMatch(t *testing.T) {
	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=basti%40example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "6-digit code") {
		t.Errorf("expected the code-entry screen to be rendered, got %q", rec.Body.String())
	}
}

func TestPostLoginShouldRenderAnIdenticalBodyRegardlessOfAMatch(t *testing.T) {
	matchReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=basti%40example.com"))
	matchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	matchRec := httptest.NewRecorder()
	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(matchRec, matchReq)

	noMatchReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=unknown%40example.com"))
	noMatchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	noMatchRec := httptest.NewRecorder()
	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(noMatchRec, noMatchReq)

	if matchRec.Code != noMatchRec.Code {
		t.Fatalf("expected identical status codes, got %d and %d", matchRec.Code, noMatchRec.Code)
	}
	if matchRec.Body.String() != noMatchRec.Body.String() {
		t.Errorf("expected identical response bodies for match and no-match, got %q and %q",
			matchRec.Body.String(), noMatchRec.Body.String())
	}
}

func TestPostLoginShouldReturn500WhenTheStoreWriteFails(t *testing.T) {
	dir := t.TempDir()
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
`
	path := filepath.Join(dir, store.DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	// Remove the directory out from under the store so the atomic
	// write-and-rename that CreateLoginCode performs fails, simulating a
	// disk write failure. Unlike a read-only permission bit, this fails
	// even when the test process runs as root (e.g. inside a container
	// build), which otherwise bypasses ordinary permission checks.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=basti%40example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("expected status 500 when the store write fails, got %d", rec.Code)
	}
}

func TestPostLoginShouldReturn500WhenTheRequestBodyIsMalformed(t *testing.T) {
	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("expected status 500 for a malformed request body, got %d", rec.Code)
	}
}

// assertClearsSessionCookieAndRedirectsToLogin asserts rec is a 302 to
// /login carrying a Set-Cookie that instructs the browser to delete the
// session cookie immediately (empty value, negative Max-Age).
func assertClearsSessionCookieAndRedirectsToLogin(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/login" {
		t.Errorf("expected a redirect to %q, got %q", "/login", rec.Header().Get("Location"))
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie to be set, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != auth.SessionCookieName {
		t.Errorf("expected cookie name %q, got %q", auth.SessionCookieName, c.Name)
	}
	if c.Value != "" {
		t.Errorf("expected an empty cookie value, got %q", c.Value)
	}
	if c.MaxAge >= 0 {
		t.Errorf("expected a negative Max-Age so the browser deletes the cookie immediately, got %d", c.MaxAge)
	}

	// Assert the literal wire text too, not just the parsed MaxAge field:
	// Go's net/http renders any MaxAge<0 as "Max-Age=0" (RFC 6265's
	// immediate-deletion form), and this is the exact instruction that
	// actually reaches a browser or cookiejar - a regression here wouldn't
	// necessarily be caught by asserting the parsed struct field alone.
	if raw := rec.Header().Get("Set-Cookie"); !strings.Contains(raw, "Max-Age=0") {
		t.Errorf("expected the raw Set-Cookie header to contain %q, got %q", "Max-Age=0", raw)
	}
}

func TestPostLogoutShouldClearTheSessionCookieAndRedirectToLoginWithAValidSession(t *testing.T) {
	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertClearsSessionCookieAndRedirectsToLogin(t, rec)
}

func TestPostLogoutShouldClearTheSessionCookieAndRedirectToLoginWithNoSessionCookie(t *testing.T) {
	req := httptest.NewRequest("POST", "/logout", nil)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertClearsSessionCookieAndRedirectsToLogin(t, rec)
}

func TestPostLogoutShouldClearTheSessionCookieAndRedirectToLoginWithAnExpiredSessionCookie(t *testing.T) {
	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(auth.IssueSessionCookieAt("basti", time.Now().UTC().Add(-31*time.Minute), testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertClearsSessionCookieAndRedirectsToLogin(t, rec)
}

func TestPostLogoutShouldClearTheSessionCookieAndRedirectToLoginWithATamperedSessionCookie(t *testing.T) {
	c := auth.IssueSessionCookie("basti", testSecret)
	last := c.Value[len(c.Value)-1]
	replacement := byte('0')
	if last == replacement {
		replacement = '1'
	}
	c.Value = c.Value[:len(c.Value)-1] + string(replacement)

	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertClearsSessionCookieAndRedirectsToLogin(t, rec)
}

func TestGetLogoutShouldNotClearTheSessionOrRedirect(t *testing.T) {
	req := httptest.NewRequest("GET", "/logout", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code == http.StatusFound {
		t.Fatalf("expected GET /logout not to redirect like POST /logout does, got status %d", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("expected GET /logout not to touch the session cookie, got %v", rec.Result().Cookies())
	}
}

func TestGetStaticStylesheetShouldBeServed(t *testing.T) {
	req := httptest.NewRequest("GET", "/static/styles.css", nil)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--bg") {
		t.Errorf("expected the stylesheet body to contain design tokens, got %q", rec.Body.String())
	}
}

// newTestStoreWithLoginCode seeds a store with one player and one persisted
// LoginCode row for code, issued issuedAt ago.
func newTestStoreWithLoginCode(t *testing.T, code string, issuedAt time.Time) *store.Store {
	t.Helper()
	st := newTestStore(t)
	sum := sha256.Sum256([]byte(code))
	hash := hex.EncodeToString(sum[:])
	if err := st.CreateLoginCode("basti", hash, issuedAt.Format(time.RFC3339)); err != nil {
		t.Fatalf("CreateLoginCode: %v", err)
	}
	return st
}

func postCode(t *testing.T, handler http.Handler, code string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/login/code", strings.NewReader("code="+code))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestPostLoginCodeShouldRedirectAndSetASessionCookieOnAValidCode(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())

	rec := postCode(t, NewServer(st, noopSender, testSecret), "123456")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/" {
		t.Errorf("expected a redirect to %q, got %q", "/", rec.Header().Get("Location"))
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie to be set, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "session" {
		t.Errorf("expected cookie name %q, got %q", "session", c.Name)
	}
	if !c.HttpOnly {
		t.Error("expected the session cookie to be HttpOnly")
	}
	if c.Secure {
		t.Error("expected the session cookie not to be Secure (AD-14 - plain HTTP today)")
	}
	if strings.Contains(rec.Body.String(), renderedGenericCodeErrorText) {
		t.Error("expected no error text in the response on a successful login")
	}
	if strings.Contains(rec.Body.String(), `class="code-input error"`) {
		t.Error("expected no error class in the response on a successful login")
	}
}

func TestPostLoginCodeShouldMatchAWhitespacePaddedSubmittedCode(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())

	rec := postCode(t, NewServer(st, noopSender, testSecret), " 123456\n")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d for a whitespace-padded code, got %d", http.StatusFound, rec.Code)
	}
	if len(rec.Result().Cookies()) != 1 {
		t.Error("expected a session cookie to be set for a whitespace-padded code")
	}
}

func TestPostLoginCodeShouldMarkTheCodeUsed(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())
	handler := NewServer(st, noopSender, testSecret)

	postCode(t, handler, "123456")
	rec := postCode(t, handler, "123456")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 on reuse, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), renderedGenericCodeErrorText) {
		t.Errorf("expected a reused code to show the generic error, got %q", rec.Body.String())
	}
}

func TestPostLoginCodeShouldShowTheGenericErrorAndRetainTheCodeOnAWrongCode(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())

	rec := postCode(t, NewServer(st, noopSender, testSecret), "000000")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), renderedGenericCodeErrorText) {
		t.Errorf("expected the generic error, got %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `value="000000"`) {
		t.Errorf("expected the submitted code to be retained in the input, got %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `class="code-input error"`) {
		t.Errorf("expected the code input to carry the error class, got %q", rec.Body.String())
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("expected no cookie to be set for a wrong code")
	}
}

func TestPostLoginCodeShouldShowTheGenericErrorWhenTheCodeFieldIsMissing(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())

	req := httptest.NewRequest("POST", "/login/code", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for a missing code field, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), renderedGenericCodeErrorText) {
		t.Errorf("expected the generic error for a missing code field, got %q", rec.Body.String())
	}
}

func TestPostLoginCodeShouldShowTheIdenticalErrorForWrongExpiredAndUsedCodes(t *testing.T) {
	wrongStore := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())
	expiredStore := newTestStoreWithLoginCode(t, "123456", time.Now().UTC().Add(-11*time.Minute))
	usedStore := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())
	usedHandler := NewServer(usedStore, noopSender, testSecret)
	postCode(t, usedHandler, "123456")

	wrongRec := postCode(t, NewServer(wrongStore, noopSender, testSecret), "000000")
	expiredRec := postCode(t, NewServer(expiredStore, noopSender, testSecret), "123456")
	usedRec := postCode(t, usedHandler, "123456")

	if wrongRec.Code != expiredRec.Code || expiredRec.Code != usedRec.Code {
		t.Fatalf("expected identical status codes, got wrong=%d expired=%d used=%d", wrongRec.Code, expiredRec.Code, usedRec.Code)
	}
	for name, body := range map[string]string{"wrong": wrongRec.Body.String(), "expired": expiredRec.Body.String(), "used": usedRec.Body.String()} {
		if !strings.Contains(body, renderedGenericCodeErrorText) {
			t.Errorf("expected the %s case to show the generic error, got %q", name, body)
		}
		if !strings.Contains(body, `class="code-input error"`) {
			t.Errorf("expected the %s case to carry the error class, got %q", name, body)
		}
	}
	if !strings.Contains(expiredRec.Body.String(), `value="123456"`) {
		t.Errorf("expected the expired case to retain the submitted code, got %q", expiredRec.Body.String())
	}
	if !strings.Contains(usedRec.Body.String(), `value="123456"`) {
		t.Errorf("expected the used case to retain the submitted code, got %q", usedRec.Body.String())
	}
}

func TestPostLoginCodeShouldReturn500WhenTheRequestBodyIsMalformed(t *testing.T) {
	req := httptest.NewRequest("POST", "/login/code", strings.NewReader("code=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("expected status 500 for a malformed request body, got %d", rec.Code)
	}
}
