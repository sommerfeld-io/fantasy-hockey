package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// r1SeriesTeamsYAML seeds 16 teams (8 Eastern, 8 Western) - enough to build
// r1's 8-matchup fixture below, 4 series per conference.
const r1SeriesTeamsYAML = `    - id: BOS
      name: Boston Bruins
      conference: Eastern
      division: Atlantic
    - id: TOR
      name: Toronto Maple Leafs
      conference: Eastern
      division: Atlantic
    - id: TBL
      name: Tampa Bay Lightning
      conference: Eastern
      division: Atlantic
    - id: FLA
      name: Florida Panthers
      conference: Eastern
      division: Atlantic
    - id: CAR
      name: Carolina Hurricanes
      conference: Eastern
      division: Metropolitan
    - id: NYR
      name: New York Rangers
      conference: Eastern
      division: Metropolitan
    - id: WSH
      name: Washington Capitals
      conference: Eastern
      division: Metropolitan
    - id: NJD
      name: New Jersey Devils
      conference: Eastern
      division: Metropolitan
    - id: COL
      name: Colorado Avalanche
      conference: Western
      division: Central
    - id: DAL
      name: Dallas Stars
      conference: Western
      division: Central
    - id: EDM
      name: Edmonton Oilers
      conference: Western
      division: Pacific
    - id: VGK
      name: Vegas Golden Knights
      conference: Western
      division: Pacific
    - id: WPG
      name: Winnipeg Jets
      conference: Western
      division: Central
    - id: LAK
      name: Los Angeles Kings
      conference: Western
      division: Pacific
    - id: NSH
      name: Nashville Predators
      conference: Western
      division: Central
    - id: MIN
      name: Minnesota Wild
      conference: Western
      division: Central
`

// r1MatchupsYAML seeds r1's 8 recorded series, 4 Eastern + 4 Western,
// matching r1SeriesTeamsYAML's own roster.
const r1MatchupsYAML = `    r1:
        - key: e1
          a: BOS
          b: TOR
        - key: e2
          a: TBL
          b: FLA
        - key: e3
          a: CAR
          b: NYR
        - key: e4
          a: WSH
          b: NJD
        - key: w1
          a: COL
          b: DAL
        - key: w2
          a: EDM
          b: VGK
        - key: w3
          a: WPG
          b: LAK
        - key: w4
          a: NSH
          b: MIN
`

// scfMatchupsYAML seeds scf's single, cross-conference matchup.
const scfMatchupsYAML = `    scf:
        - key: f1
          a: BOS
          b: COL
`

// r1PredictionSetSeed/scfPredictionSetSeed are r1's/scf's raw prediction_sets
// YAML block with deadline as its deadline_utc, mirroring
// divisionsPredictionSetSeed's own precedent.
func r1PredictionSetSeed(deadline time.Time) string {
	return fmt.Sprintf(`    - id: r1
      title: Playoff round 1
      subtitle: 8 series — winner & length
      deadline_utc: %q
      phase: playoffs
      upcoming: false
`, deadline.Format(time.RFC3339))
}

func scfPredictionSetSeed(deadline time.Time) string {
	return fmt.Sprintf(`    - id: scf
      title: Stanley Cup Final
      subtitle: The last series standing
      deadline_utc: %q
      phase: playoffs
      upcoming: false
`, deadline.Format(time.RFC3339))
}

// newTestStoreWithSeriesFixtureAndDir seeds a store with one player, the
// given raw prediction_sets/playoff_matchups YAML blocks, and
// r1SeriesTeamsYAML's own 16-team roster - the series pick-entry sheet's own
// fixture, returning the seeded temp directory too so a write-failure test
// can remove it out from under the store (newTestStoreWithDivisionTeamsAndDir's
// own precedent).
func newTestStoreWithSeriesFixtureAndDir(t *testing.T, predictionSetsYAML, matchupsYAML string) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
prediction_sets:
` + predictionSetsYAML + `teams:
` + r1SeriesTeamsYAML + `playoff_matchups:
` + matchupsYAML
	path := filepath.Join(dir, store.DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return st, dir
}

func newTestStoreWithSeriesFixture(t *testing.T, predictionSetsYAML, matchupsYAML string) *store.Store {
	t.Helper()
	st, _ := newTestStoreWithSeriesFixtureAndDir(t, predictionSetsYAML, matchupsYAML)
	return st
}

// getSeriesSheet GETs /predict/{id} as the signed-in test player and returns
// the recorded response.
func getSeriesSheet(t *testing.T, handler http.Handler, id string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/predict/"+id, nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// postSeriesForm POSTs picks (matchup key -> {team_id, games}) to
// /predict/{id} as the signed-in test player, following no redirect, and
// returns the recorded response. A field left out of a pick simply submits
// no value for it, matching what a real unchecked radio group sends.
func postSeriesForm(t *testing.T, handler http.Handler, id string, picks map[string]seriesSubmission) *httptest.ResponseRecorder {
	t.Helper()
	values := url.Values{}
	for key, sub := range picks {
		if sub.TeamID != "" {
			values.Set(seriesTeamFieldName(key), sub.TeamID)
		}
		if sub.Games != "" {
			values.Set(seriesGamesFieldName(key), sub.Games)
		}
	}

	req := httptest.NewRequest("POST", "/predict/"+id, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestGetR1SheetShouldGroupEightSeriesFourEasternFourWestern(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := getSeriesSheet(t, handler, "r1")

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if got := strings.Count(body, `data-series="`); got != 8 {
		t.Errorf("expected 8 series cards, got %d in %q", got, body)
	}
	if !strings.Contains(body, `>Eastern Conference</legend>`) {
		t.Errorf("expected an Eastern Conference group, got %q", body)
	}
	if !strings.Contains(body, `>Western Conference</legend>`) {
		t.Errorf("expected a Western Conference group, got %q", body)
	}
	for _, key := range []string{"e1", "e2", "e3", "e4", "w1", "w2", "w3", "w4"} {
		if !strings.Contains(body, `data-series="`+key+`"`) {
			t.Errorf("expected a series card for %q, got %q", key, body)
		}
	}
}

// seriesGroupFragment isolates the conference/final group named by label
// (from its own <legend> through the next <fieldset> or the end of the
// form), mirroring divisionChipGroupFragment's own isolation precedent -
// asserting a series card belongs to the right group, not just that it
// exists somewhere on the page.
func seriesGroupFragment(t *testing.T, body, label string) string {
	t.Helper()
	marker := ">" + label + "</legend>"
	start := strings.Index(body, marker)
	if start == -1 {
		t.Fatalf("expected a group labeled %q, got %q", label, body)
	}
	rest := body[start+len(marker):]
	if end := strings.Index(rest, "<fieldset"); end != -1 {
		return rest[:end]
	}
	return rest
}

func TestGetR1SheetShouldPutEasternSeriesUnderEasternAndWesternUnderWestern(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	body := getSeriesSheet(t, handler, "r1").Body.String()

	eastern := seriesGroupFragment(t, body, "Eastern Conference")
	for _, key := range []string{"e1", "e2", "e3", "e4"} {
		if !strings.Contains(eastern, `data-series="`+key+`"`) {
			t.Errorf("expected %q within the Eastern Conference group, got %q", key, eastern)
		}
	}
	for _, key := range []string{"w1", "w2", "w3", "w4"} {
		if strings.Contains(eastern, `data-series="`+key+`"`) {
			t.Errorf("expected %q NOT within the Eastern Conference group, got %q", key, eastern)
		}
	}
}

func TestGetScfSheetShouldGroupUnderStanleyCupFinalNotAConference(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, scfPredictionSetSeed(deadline), scfMatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := getSeriesSheet(t, handler, "scf")

	body := rec.Body.String()
	if got := strings.Count(body, `data-series="`); got != 1 {
		t.Errorf("expected exactly 1 series card, got %d in %q", got, body)
	}
	if !strings.Contains(body, `>Stanley Cup Final</legend>`) {
		t.Errorf("expected a Stanley Cup Final group, got %q", body)
	}
	if strings.Contains(body, "Eastern Conference") || strings.Contains(body, "Western Conference") {
		t.Errorf("expected no conference subheader for the Final, got %q", body)
	}
}

func TestGetR1SheetShouldShowNoTeamOrGamesPreselectedWhenNoPriorPick(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	body := getSeriesSheet(t, handler, "r1").Body.String()

	if strings.Contains(body, "checked") {
		t.Errorf("expected no button preselected, got %q", body)
	}
	if !strings.Contains(body, `>Submit predictions</button>`) {
		t.Errorf("expected the button to read \"Submit predictions\", got %q", body)
	}
}

// TestGetR1SheetShouldShowANoSeriesRecordedYetPlaceholderWhenNoMatchupsAreRecorded
// covers a series-pickable set with zero recorded matchups - "r1" is
// deliberately ungated by playoff_matchups (Story 3.2's Design Notes), so it
// can be Open, and therefore reachable, before a human enters its bracket.
func TestGetR1SheetShouldShowANoSeriesRecordedYetPlaceholderWhenNoMatchupsAreRecorded(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), "")
	handler := NewServer(st, noopSender, testSecret)

	body := getSeriesSheet(t, handler, "r1").Body.String()

	if !strings.Contains(body, "No series recorded yet.") {
		t.Errorf("expected the \"no series recorded yet\" placeholder, got %q", body)
	}
	if strings.Contains(body, "data-series=\"") {
		t.Errorf("expected no series cards, got %q", body)
	}
	if strings.Contains(body, `<button type="submit">`) {
		t.Errorf("expected no submit button when nothing can be picked yet, got %q", body)
	}
}

func TestGetR1SheetShouldNeverCreateARowMerelyByOpeningTheSheet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	getSeriesSheet(t, handler, "r1")

	if _, ok := st.FindSeriesPick("basti", "r1.e1"); ok {
		t.Error("expected no Prediction row to be force-created merely by opening the sheet")
	}
}

func TestPostR1SheetShouldSaveWinnerAndGamesTogetherAndRedirect(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "BOS", Games: "6"},
	})

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusFound, rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/predict" {
		t.Errorf("expected a redirect to %q, got %q", "/predict", rec.Header().Get("Location"))
	}
	got, ok := st.FindSeriesPick("basti", "r1.e1")
	if !ok {
		t.Fatal("expected a saved series pick, found none")
	}
	if got.TeamID != "BOS" || got.Games != "6" {
		t.Errorf("expected TeamID %q and Games %q saved together, got %q/%q", "BOS", "6", got.TeamID, got.Games)
	}
}

func TestPostR1SheetShouldUpdateAnExistingPickInPlaceOnResubmission(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	if err := st.SaveSeriesPick("basti", "r1.e1", "BOS", "6", time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveSeriesPick: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "TOR", Games: "7"},
	})

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusFound, rec.Code, rec.Body.String())
	}
	got, ok := st.FindSeriesPick("basti", "r1.e1")
	if !ok {
		t.Fatal("expected a saved series pick, found none")
	}
	if got.TeamID != "TOR" || got.Games != "7" {
		t.Errorf("expected the updated pick TOR/7, got %q/%q", got.TeamID, got.Games)
	}
}

func TestGetR1SheetShouldPreselectSavedPicksAndReadUpdatePredictions(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	if err := st.SaveSeriesPick("basti", "r1.e1", "BOS", "6", time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveSeriesPick: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	body := getSeriesSheet(t, handler, "r1").Body.String()

	if !strings.Contains(body, `value="BOS" aria-label="Boston Bruins" checked`) {
		t.Errorf("expected the saved winner preselected, got %q", body)
	}
	if !strings.Contains(body, `value="6" aria-label="6 games" checked`) {
		t.Errorf("expected the saved game count preselected, got %q", body)
	}
	if !strings.Contains(body, `>Update predictions</button>`) {
		t.Errorf("expected the button to read \"Update predictions\", got %q", body)
	}
}

func TestPostR1SheetShouldSilentlyDropAHalfFilledSeriesButSaveOthers(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "BOS"},             // half-filled: team only, no games.
		"e2": {Games: "6"},                // half-filled: games only, no team.
		"e3": {TeamID: "CAR", Games: "6"}, // complete and valid.
	})

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d for a submission with only half-filled/valid series, got %d: %s", http.StatusFound, rec.Code, rec.Body.String())
	}
	if _, ok := st.FindSeriesPick("basti", "r1.e1"); ok {
		t.Error("expected no saved pick for the half-filled e1 series")
	}
	if _, ok := st.FindSeriesPick("basti", "r1.e2"); ok {
		t.Error("expected no saved pick for the half-filled e2 series")
	}
	got, ok := st.FindSeriesPick("basti", "r1.e3")
	if !ok || got.TeamID != "CAR" || got.Games != "6" {
		t.Errorf("expected e3's valid pick CAR/6 to still save, got %+v (ok=%v)", got, ok)
	}
}

// assertSavedSeriesPick fails t unless playerID has a saved series pick for
// seriesKey matching wantTeamID/wantGames - factored out purely to keep its
// callers' own cyclomatic complexity low (gocyclo), mirroring
// internal/store's own assertSeriesPick precedent.
func assertSavedSeriesPick(t *testing.T, st *store.Store, playerID, seriesKey, wantTeamID, wantGames string) {
	t.Helper()
	got, ok := st.FindSeriesPick(playerID, seriesKey)
	if !ok {
		t.Errorf("expected a series pick for %q/%q, found none", playerID, seriesKey)
		return
	}
	if got.TeamID != wantTeamID || got.Games != wantGames {
		t.Errorf("expected %q/%q for %q/%q, got %q/%q", wantTeamID, wantGames, playerID, seriesKey, got.TeamID, got.Games)
	}
}

// TestPostR1SheetShouldLeaveAnExistingSeriesPickUnchangedWhenItsResubmissionIsHalfFilledOrInvalid
// covers the core data-safety guarantee behind "an incomplete/invalid series
// pick is simply not saved": that must also hold when a saved pick already
// exists for that same series - a half-filled or invalid resubmission must
// never clear or overwrite it, only leave it exactly as it was. e1 already
// has a saved pick and is resubmitted half-filled; e2 already has a saved
// pick and is resubmitted with a foreign team_id; e3 is a fresh, complete,
// valid submission alongside them and must still save.
func TestPostR1SheetShouldLeaveAnExistingSeriesPickUnchangedWhenItsResubmissionIsHalfFilledOrInvalid(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	if err := st.SaveSeriesPick("basti", "r1.e1", "BOS", "6", time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveSeriesPick for e1: %v", err)
	}
	if err := st.SaveSeriesPick("basti", "r1.e2", "TBL", "5", time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveSeriesPick for e2: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "TOR"},             // half-filled resubmission over an existing pick: no games.
		"e2": {TeamID: "COL", Games: "5"}, // invalid resubmission over an existing pick: COL is foreign to e2.
		"e3": {TeamID: "CAR", Games: "6"}, // fresh, complete, and valid.
	})

	if rec.Code != 200 {
		t.Fatalf("expected status 200 (e2's invalid resubmission rejects), got %d", rec.Code)
	}
	assertSavedSeriesPick(t, st, "basti", "r1.e1", "BOS", "6") // unchanged: half-filled resubmission.
	assertSavedSeriesPick(t, st, "basti", "r1.e2", "TBL", "5") // unchanged: invalid resubmission.
	assertSavedSeriesPick(t, st, "basti", "r1.e3", "CAR", "6") // fresh, complete, valid submission saves.
}

func TestPostR1SheetShouldRejectAForeignTeamIDOnlyForThatSeriesAndSaveOthers(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "COL", Games: "6"}, // COL belongs to w1, not e1 (BOS/TOR).
		"e2": {TeamID: "TBL", Games: "5"}, // complete and valid.
	})

	if rec.Code != 200 {
		t.Fatalf("expected status 200 for a rejected series, got %d", rec.Code)
	}
	body := rec.Body.String()
	e1 := strings.Index(body, `data-series="e1"`)
	e2 := strings.Index(body, `data-series="e2"`)
	if e1 == -1 || e2 == -1 {
		t.Fatalf("expected both series cards to render, got %q", body)
	}
	if !strings.Contains(body[e1:e2], "error-text") {
		t.Errorf("expected e1's own inline error caption, got %q", body[e1:e2])
	}
	if _, ok := st.FindSeriesPick("basti", "r1.e1"); ok {
		t.Error("expected no saved pick for the rejected e1 series")
	}
	got, ok := st.FindSeriesPick("basti", "r1.e2")
	if !ok || got.TeamID != "TBL" || got.Games != "5" {
		t.Errorf("expected e2's valid pick TBL/5 to still save, got %+v (ok=%v)", got, ok)
	}
}

func TestPostR1SheetShouldRejectAnOutOfRangeGamesCountOnlyForThatSeriesAndSaveOthers(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "BOS", Games: "3"}, // 3 is outside 4-7.
		"e2": {TeamID: "FLA", Games: "7"}, // complete and valid.
	})

	if rec.Code != 200 {
		t.Fatalf("expected status 200 for a rejected series, got %d", rec.Code)
	}
	assertSeriesCardHasInlineError(t, rec.Body.String(), "e1", "e2")
	if _, ok := st.FindSeriesPick("basti", "r1.e1"); ok {
		t.Error("expected no saved pick for the rejected e1 series")
	}
	got, ok := st.FindSeriesPick("basti", "r1.e2")
	if !ok || got.TeamID != "FLA" || got.Games != "7" {
		t.Errorf("expected e2's valid pick FLA/7 to still save, got %+v (ok=%v)", got, ok)
	}
}

// assertSeriesCardHasInlineError fails t unless the card for key (up to the
// next card, nextKey) carries its own inline error caption, and the nextKey
// card itself does not.
func assertSeriesCardHasInlineError(t *testing.T, body, key, nextKey string) {
	t.Helper()
	start := strings.Index(body, `data-series="`+key+`"`)
	next := strings.Index(body, `data-series="`+nextKey+`"`)
	if start == -1 || next == -1 || next < start {
		t.Fatalf("expected cards %q and %q to render in order, got %q", key, nextKey, body)
	}
	if !strings.Contains(body[start:next], "error-text") {
		t.Errorf("expected %q's own inline error caption, got %q", key, body[start:next])
	}
	end := strings.Index(body[next+1:], `data-series="`)
	nextCard := body[next:]
	if end != -1 {
		nextCard = body[next : next+1+end]
	}
	if strings.Contains(nextCard, "error-text") {
		t.Errorf("expected no inline error on the valid %q card, got %q", nextKey, nextCard)
	}
}

func TestPostR1SheetShouldRejectANonNumericGamesValueOnlyForThatSeriesAndSaveOthers(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "BOS", Games: "abc"}, // not a game count at all.
		"e2": {TeamID: "FLA", Games: "7"},   // complete and valid.
	})

	if rec.Code != 200 {
		t.Fatalf("expected status 200 for a rejected series, got %d", rec.Code)
	}
	assertSeriesCardHasInlineError(t, rec.Body.String(), "e1", "e2")
	if _, ok := st.FindSeriesPick("basti", "r1.e1"); ok {
		t.Error("expected no saved pick for the rejected e1 series")
	}
	assertSavedSeriesPick(t, st, "basti", "r1.e2", "FLA", "7")
}

// TestPostR1SheetShouldKeepOtherCardsSubmittedValuesWhenRejecting proves a
// rejected re-render shows every card's own submitted values - here a
// half-filled sibling (e3, winner only) keeps its checked winner button, so
// the player doesn't lose unsaved selections because another card (e1) was
// invalid - while a card left blank (e4) shows nothing checked.
func TestPostR1SheetShouldKeepOtherCardsSubmittedValuesWhenRejecting(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "COL", Games: "6"}, // COL is foreign to e1: rejects.
		"e3": {TeamID: "CAR"},             // half-filled: nothing saved, but kept on re-render.
	})

	if rec.Code != 200 {
		t.Fatalf("expected status 200 for a rejected series, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `value="CAR" aria-label="Carolina Hurricanes" checked`) {
		t.Errorf("expected e3's submitted winner CAR to stay checked on re-render, got %q", body)
	}
	if strings.Contains(body, `value="WSH" aria-label="Washington Capitals" checked`) {
		t.Errorf("expected the blank e4 card to show no checked winner, got %q", body)
	}
	if _, ok := st.FindSeriesPick("basti", "r1.e3"); ok {
		t.Error("expected no saved pick for the half-filled e3 series")
	}
}

func TestPostR1SheetShouldRejectAfterTheDeadlineWithNoOverride(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "BOS", Games: "6"},
	})

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if _, ok := st.FindSeriesPick("basti", "r1.e1"); ok {
		t.Error("expected nothing saved after the deadline")
	}
}

func TestGetR1SheetShouldRenderAReadOnlyBannerAndDisabledInputsWhenClosed(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	if err := st.SaveSeriesPick("basti", "r1.e1", "BOS", "6", time.Now().UTC().Add(-48*time.Hour)); err != nil {
		t.Fatalf("seed SaveSeriesPick: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	body := getSeriesSheet(t, handler, "r1").Body.String()

	if !strings.Contains(body, "closed-banner") {
		t.Errorf("expected the read-only banner, got %q", body)
	}
	if !strings.Contains(body, `value="BOS" aria-label="Boston Bruins" checked disabled`) {
		t.Errorf("expected the preselected winner button to render disabled, got %q", body)
	}
	if strings.Contains(body, `<button type="submit">`) {
		t.Errorf("expected no submit button on a closed set, got %q", body)
	}
}

func TestPredictShouldShowSubmittedStatusForR1OnceAPickIsSaved(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	if err := st.SaveSeriesPick("basti", "r1.e1", "BOS", "6", time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveSeriesPick: %v", err)
	}

	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `>Submitted</span>`) {
		t.Errorf("expected the r1 row to show Submitted once a series pick is saved, got %q", rec.Body.String())
	}
}

func TestPostR1SheetShouldReturn500WhenTheStoreWriteFails(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st, dir := newTestStoreWithSeriesFixtureAndDir(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "BOS", Games: "6"},
	})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d when the store write fails, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// gatedSeriesPredictionSetSeed is a round-gated series set's raw
// prediction_sets YAML block, seeded at upcoming: true exactly as today's
// real fantasy-hockey.yml seeds "r2"/"cf"/"scf" - only a recorded matchup
// unlocks it.
func gatedSeriesPredictionSetSeed(id, title string, deadline time.Time) string {
	return fmt.Sprintf(`    - id: %s
      title: %s
      subtitle: Set once the prior round ends
      deadline_utc: %q
      phase: playoffs
      upcoming: true
`, id, title, deadline.Format(time.RFC3339))
}

// gatedSeriesMatchupsYAML records one Eastern (e1) and one Western (w1)
// series under id.
func gatedSeriesMatchupsYAML(id string) string {
	return fmt.Sprintf(`    %s:
        - key: e1
          a: BOS
          b: TOR
        - key: w1
          a: COL
          b: DAL
`, id)
}

// TestGetGatedSeriesSheetShouldRenderSeriesCardsGroupedByConference proves
// "r2" and "cf" render as real series sheets once unlocked - keyed cards
// under both conference groups - not the single-team dropdown or the stub.
func TestGetGatedSeriesSheetShouldRenderSeriesCardsGroupedByConference(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	for _, id := range []string{round2SetID, conferenceFinalsSetID} {
		t.Run(id, func(t *testing.T) {
			st := newTestStoreWithSeriesFixture(t, gatedSeriesPredictionSetSeed(id, "Gated round", deadline), gatedSeriesMatchupsYAML(id))

			rec := getSeriesSheet(t, NewServer(st, noopSender, testSecret), id)

			if rec.Code != 200 {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}
			body := rec.Body.String()
			if got := strings.Count(body, `data-series="`); got != 2 {
				t.Errorf("expected 2 series cards, got %d in %q", got, body)
			}
			if !strings.Contains(seriesGroupFragment(t, body, "Eastern Conference"), `data-series="e1"`) {
				t.Errorf("expected e1 under the Eastern Conference group, got %q", body)
			}
			if !strings.Contains(seriesGroupFragment(t, body, "Western Conference"), `data-series="w1"`) {
				t.Errorf("expected w1 under the Western Conference group, got %q", body)
			}
			if strings.Contains(body, `name="team_id"`) {
				t.Errorf("expected no single-team dropdown on a series sheet, got %q", body)
			}
		})
	}
}

func TestPostGatedSeriesSheetShouldSaveEachSeriesUnderItsOwnSetID(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	for _, id := range []string{round2SetID, conferenceFinalsSetID, stanleyCupFinalSetID} {
		t.Run(id, func(t *testing.T) {
			st := newTestStoreWithSeriesFixture(t, gatedSeriesPredictionSetSeed(id, "Gated round", deadline), gatedSeriesMatchupsYAML(id))

			rec := postSeriesForm(t, NewServer(st, noopSender, testSecret), id, map[string]seriesSubmission{
				"e1": {TeamID: "TOR", Games: "7"},
			})

			if rec.Code != http.StatusFound {
				t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
			}
			assertSavedSeriesPick(t, st, "basti", joinSeriesKey(id, "e1"), "TOR", "7")
			if _, ok := st.FindSeriesPick("basti", joinSeriesKey(round1SetID, "e1")); ok {
				t.Errorf("expected nothing saved under r1 for a %q submission", id)
			}
		})
	}
}

// TestGetGatedSeriesSheetShouldRenderReadOnlyWhenUnlockedButPastItsDeadline
// pins newSheetData's own use of effectiveUpcoming: "cf" at upcoming: true
// with a recorded matchup is unlocked, so past its deadline it must render
// Closed (read-only banner, disabled inputs) - not an editable form whose
// submit would only fail with a 403.
func TestGetGatedSeriesSheetShouldRenderReadOnlyWhenUnlockedButPastItsDeadline(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, gatedSeriesPredictionSetSeed(conferenceFinalsSetID, "Conference finals", deadline), gatedSeriesMatchupsYAML(conferenceFinalsSetID))

	rec := getSeriesSheet(t, NewServer(st, noopSender, testSecret), conferenceFinalsSetID)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "closed-banner") {
		t.Errorf("expected the read-only banner, got %q", body)
	}
	if !strings.Contains(body, `value="BOS" aria-label="Boston Bruins"  disabled`) {
		t.Errorf("expected disabled winner buttons, got %q", body)
	}
	if strings.Contains(body, `<button type="submit">`) {
		t.Errorf("expected no submit button on a closed set, got %q", body)
	}
}

// TestJoinSeriesKeyShouldRoundTripThroughSplitSeriesKey proves
// joinSeriesKey/splitSeriesKey are true inverses of one another - the
// join/split helper pair the Code Map calls for, guarding the "r1.s1"-shaped
// key format against an accidental separator change ever silently breaking
// the round trip.
func TestJoinSeriesKeyShouldRoundTripThroughSplitSeriesKey(t *testing.T) {
	joined := joinSeriesKey("r1", "s1")
	if joined != "r1.s1" {
		t.Fatalf("expected joined key %q, got %q", "r1.s1", joined)
	}

	setID, key, ok := splitSeriesKey(joined)
	if !ok {
		t.Fatal("expected splitSeriesKey to report ok=true for a joined key")
	}
	if setID != "r1" || key != "s1" {
		t.Errorf("expected setID/key %q/%q, got %q/%q", "r1", "s1", setID, key)
	}
}

func TestSplitSeriesKeyShouldReportNotOkForAKeyWithNoSeparator(t *testing.T) {
	_, _, ok := splitSeriesKey("malformed")
	if ok {
		t.Error("expected ok=false for a key with no separator")
	}
}
