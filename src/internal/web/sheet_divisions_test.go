package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// divisionTeamsByDivision is this file's small, representative-of-the-real-
// roster fixture for the divisions pick-entry sheet: all 32 real NHL teams,
// 8 per division, matching the App.jsx reference's own DIVISIONS constant -
// needed (unlike newTestStoreWithPredictionSetsAndTeams's 4-team fixture)
// because the divisions form's own validation gates depend on a division
// actually being able to reach 5 and a conference actually being able to
// reach 8.
var divisionTeamsByDivision = map[string][]string{
	"Atlantic":     {"BOS", "BUF", "DET", "FLA", "MTL", "OTT", "TBL", "TOR"},
	"Metropolitan": {"CAR", "CBJ", "NJD", "NYI", "NYR", "PHI", "PIT", "WSH"},
	"Central":      {"CHI", "COL", "DAL", "MIN", "NSH", "STL", "UTA", "WPG"},
	"Pacific":      {"ANA", "CGY", "EDM", "LAK", "SJS", "SEA", "VAN", "VGK"},
}

// newTestStoreWithDivisionTeams seeds a store with one player, the given raw
// prediction_sets YAML block, and the full 32-team roster
// (divisionTeamsByDivision) - the divisions pick-entry sheet's own fixture,
// distinct from newTestStoreWithPredictionSetsAndTeams's smaller one-team-
// per-division sample used by the cup/presidents sheet.
func newTestStoreWithDivisionTeams(t *testing.T, predictionSetsYAML string) *store.Store {
	t.Helper()
	st, _ := newTestStoreWithDivisionTeamsAndDir(t, predictionSetsYAML)
	return st
}

// newTestStoreWithDivisionTeamsAndDir is newTestStoreWithDivisionTeams' own
// variant that also returns the seeded temp directory, for tests that need
// to remove it out from under the store (e.g. to force a write failure) -
// this is the only place that seed-building logic lives, so a
// write-failure test never has to duplicate it just to keep the dir.
func newTestStoreWithDivisionTeamsAndDir(t *testing.T, predictionSetsYAML string) (*store.Store, string) {
	t.Helper()
	var yamlTeams strings.Builder
	for _, division := range store.Divisions() {
		conference := "Eastern"
		if division == "Central" || division == "Pacific" {
			conference = "Western"
		}
		teams, ok := divisionTeamsByDivision[division]
		if !ok {
			t.Fatalf("no fixture teams for division %q - divisionTeamsByDivision has drifted from store.Divisions()", division)
		}
		for _, id := range teams {
			fmt.Fprintf(&yamlTeams, "    - id: %s\n      name: %s Team\n      conference: %s\n      division: %s\n", id, id, conference, division)
		}
	}

	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
prediction_sets:
` + predictionSetsYAML + `teams:
` + yamlTeams.String()
	return newSeededStore(t, seed)
}

// divisionsPredictionSetSeed is the "divisions" Prediction Set's raw
// prediction_sets YAML block with deadline as its deadline_utc, for tests
// exercising the divisions pick-entry sheet.
func divisionsPredictionSetSeed(deadline time.Time) string {
	return fmt.Sprintf(`    - id: divisions
      title: Division picks
      subtitle: Playoff teams & division winners
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))
}

// validDivisionPicks is one valid, all-4/4-split submission across both
// conferences (Atlantic+Metropolitan == 8, Central+Pacific == 8), for tests
// that need a submission that passes every gate.
func validDivisionPicks() map[string][]string {
	return map[string][]string{
		"Atlantic":     {"BOS", "BUF", "DET", "FLA"},
		"Metropolitan": {"CAR", "CBJ", "NJD", "NYI"},
		"Central":      {"CHI", "COL", "DAL", "MIN"},
		"Pacific":      {"ANA", "CGY", "EDM", "LAK"},
	}
}

// validDivisionWinners is one winner pick per division, each belonging to
// that division's own roster.
func validDivisionWinners() map[string]string {
	return map[string]string{
		"Atlantic":     "BOS",
		"Metropolitan": "CAR",
		"Central":      "CHI",
		"Pacific":      "ANA",
	}
}

// postDivisionsForm POSTs playoffTeams/winners to /predict/divisions as the
// signed-in test player, following no redirect, and returns the recorded
// response. A division absent from playoffTeams/winners simply submits no
// checkbox/an empty winner for it, matching what a real unchecked
// checkbox-group/unset <select> sends.
func postDivisionsForm(t *testing.T, handler http.Handler, playoffTeams map[string][]string, winners map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	values := url.Values{}
	for division, teamIDs := range playoffTeams {
		field := divisionPlayoffTeamsFieldName(division)
		for _, id := range teamIDs {
			values.Add(field, id)
		}
	}
	for division, teamID := range winners {
		values.Set(divisionWinnerFieldName(division), teamID)
	}

	req := httptest.NewRequest("POST", "/predict/divisions", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// getDivisionsSheet GETs /predict/divisions as the signed-in test player and
// returns the recorded response.
func getDivisionsSheet(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/predict/divisions", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// assertDivisionsSheetHasChipGroupsAndWinnerSelects checks body renders a
// chip group and a winner select for every division in store.Divisions() -
// factored out of its one caller purely to keep that test's own cyclomatic
// complexity low (gocyclo).
func assertDivisionsSheetHasChipGroupsAndWinnerSelects(t *testing.T, body string) {
	t.Helper()
	for _, division := range store.Divisions() {
		if !strings.Contains(body, `data-division="`+division+`"`) {
			t.Errorf("expected a chip group for %q, got %q", division, body)
		}
		if !strings.Contains(body, `for="winner-`+division+`"`) {
			t.Errorf("expected a winner select for %q, got %q", division, body)
		}
	}
}

func TestGetDivisionsSheetShouldRenderFourChipGroupsAndWinnerSelectsWithNothingCheckedWhenNoPriorPick(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := getDivisionsSheet(t, handler)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	assertDivisionsSheetHasChipGroupsAndWinnerSelects(t, body)
	if strings.Contains(body, "checked") {
		t.Errorf("expected no chip preselected, got %q", body)
	}
	if got := strings.Count(body, `0/5`); got != 4 {
		t.Errorf("expected all 4 divisions' live n/5 counters to start at 0, got %d occurrences in %q", got, body)
	}
	if got := strings.Count(body, `0/8 selected`); got != 2 {
		t.Errorf("expected both conferences' live n/8 indicators to start at 0, got %d occurrences in %q", got, body)
	}
	if !strings.Contains(body, `>Submit predictions</button>`) {
		t.Errorf("expected the button to read \"Submit predictions\", got %q", body)
	}
	if !strings.Contains(body, `id="divisions-submit" disabled>`) {
		t.Errorf("expected the submit button to render disabled while incomplete, got %q", body)
	}
	if !strings.Contains(body, `<script src="/static/divisions.js" defer></script>`) {
		t.Errorf("expected the divisions.js script tag, got %q", body)
	}
}

func TestGetDivisionsSheetShouldPreselectSavedPicksAndReadUpdatePredictions(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	now := time.Now().UTC()
	if err := st.SaveDivisionPicks("basti", validDivisionPicks(), validDivisionWinners(), now); err != nil {
		t.Fatalf("seed SaveDivisionPicks: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	rec := getDivisionsSheet(t, handler)

	body := rec.Body.String()
	if !strings.Contains(body, `value="BOS" checked`) {
		t.Errorf("expected the saved Atlantic pick preselected, got %q", body)
	}
	if !strings.Contains(body, `value="BOS" selected>`) {
		t.Errorf("expected the saved Atlantic winner preselected, got %q", body)
	}
	if got := strings.Count(body, `4/5`); got != 4 {
		t.Errorf("expected all 4 divisions' live n/5 counters to reflect the saved 4-team picks, got %d occurrences in %q", got, body)
	}
	if got := strings.Count(body, `8/8 selected`); got != 2 {
		t.Errorf("expected both conferences' live n/8 indicators to read 8/8, got %d occurrences in %q", got, body)
	}
	if !strings.Contains(body, `>Update predictions</button>`) {
		t.Errorf("expected the button to read \"Update predictions\", got %q", body)
	}
	if strings.Contains(body, `id="divisions-submit" disabled>`) {
		t.Errorf("expected the submit button to render enabled once both conferences are valid, got %q", body)
	}
}

func TestGetDivisionsSheetShouldRenderAReadOnlyBannerAndDisabledInputsWhenClosed(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	if err := st.SaveDivisionPicks("basti", validDivisionPicks(), validDivisionWinners(), time.Now().UTC().Add(-48*time.Hour)); err != nil {
		t.Fatalf("seed SaveDivisionPicks: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	rec := getDivisionsSheet(t, handler)

	body := rec.Body.String()
	// One representative preselected chip and winner select per division -
	// not just Atlantic - so a per-division template bug wouldn't slip past
	// this test undetected.
	wantDisabled := map[string]string{
		"Atlantic":     "BOS",
		"Metropolitan": "CAR",
		"Central":      "CHI",
		"Pacific":      "ANA",
	}
	for division, teamID := range wantDisabled {
		if !strings.Contains(body, `value="`+teamID+`" checked disabled>`) {
			t.Errorf("expected %s's preselected chip %q to render disabled, got %q", division, teamID, body)
		}
		if !strings.Contains(body, `id="winner-`+division+`" name="winner_`+strings.ToLower(division)+`" aria-label="`+division+` winner" disabled>`) {
			t.Errorf("expected %s's winner select to render disabled, got %q", division, body)
		}
	}
	if !strings.Contains(body, "closed-banner") {
		t.Errorf("expected the read-only banner, got %q", body)
	}
	if strings.Contains(body, `id="divisions-submit"`) {
		t.Errorf("expected no submit button on a closed set, got %q", body)
	}
}

func TestGetDivisionsSheetShouldNeverCreateARowMerelyByOpeningTheSheet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	getDivisionsSheet(t, handler)

	if _, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic"); ok {
		t.Error("expected no Prediction row to be force-created merely by opening the sheet")
	}
}

func TestPredictShouldShowOpenStatusForDivisionsBeforeAnyPickIsSaved(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))

	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `<a id="predict-row-divisions" href="/predict/divisions" class="set-row set-row--open">`) {
		t.Errorf("expected the divisions row to show Open before any pick is saved, got %q", rec.Body.String())
	}
}

func TestPostDivisionsSheetShouldSaveAllPicksAndRedirectToPredict(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postDivisionsForm(t, handler, validDivisionPicks(), validDivisionWinners())

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/predict" {
		t.Errorf("expected a redirect to %q, got %q", "/predict", rec.Header().Get("Location"))
	}

	for division, teamIDs := range validDivisionPicks() {
		got, ok := st.FindDivisionPlayoffTeams("basti", division)
		if !ok {
			t.Fatalf("expected a saved playoff-teams row for %q", division)
		}
		if len(got.TeamIDs) != len(teamIDs) {
			t.Errorf("expected %d team ids for %q, got %v", len(teamIDs), division, got.TeamIDs)
		}
	}
	for division, teamID := range validDivisionWinners() {
		got, ok := st.FindDivisionWinner("basti", division)
		if !ok || got.TeamID != teamID {
			t.Errorf("expected the %q winner %q, got %+v (ok=%v)", division, teamID, got, ok)
		}
	}

	predictReq := httptest.NewRequest("GET", "/predict", nil)
	predictReq.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	predictRec := httptest.NewRecorder()
	handler.ServeHTTP(predictRec, predictReq)
	if !strings.Contains(predictRec.Body.String(), `<a id="predict-row-divisions" href="/predict/divisions" class="set-row set-row--submitted">`) {
		t.Errorf("expected the divisions row to show Submitted, got %q", predictRec.Body.String())
	}
}

// TestPostDivisionsSheetShouldAcceptA53SplitWithinAConference covers the
// other half of "either a 4/4 split, or 5 in one division and 3 in the
// other" - every other divisions test in this file only exercises a 4/4
// split, so this proves the 5-team boundary (exactly maxTeamsPerDivision)
// is accepted, not just rejected past it.
func TestPostDivisionsSheetShouldAcceptA53SplitWithinAConference(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	picks := validDivisionPicks()
	picks["Atlantic"] = append(picks["Atlantic"], "MTL") // 5 teams (at the cap).
	picks["Metropolitan"] = picks["Metropolitan"][:3]    // 3 teams, so Eastern still nets exactly 8.

	rec := postDivisionsForm(t, handler, picks, validDivisionWinners())

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d for a valid 5/3 split, got %d: %s", http.StatusFound, rec.Code, rec.Body.String())
	}
	atlantic, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if !ok || len(atlantic.TeamIDs) != 5 {
		t.Errorf("expected 5 saved Atlantic team ids, got %+v (ok=%v)", atlantic, ok)
	}
	metro, ok := st.FindDivisionPlayoffTeams("basti", "Metropolitan")
	if !ok || len(metro.TeamIDs) != 3 {
		t.Errorf("expected 3 saved Metropolitan team ids, got %+v (ok=%v)", metro, ok)
	}
}

func TestPostDivisionsSheetShouldRejectWhenAConferenceTotalIsNotEightAndSaveNothing(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	picks := validDivisionPicks()
	picks["Atlantic"] = picks["Atlantic"][:3] // Eastern conference now totals 7, not 8.

	rec := postDivisionsForm(t, handler, picks, validDivisionWinners())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), divisionsCountErrorText) {
		t.Errorf("expected the inline error caption, got %q", rec.Body.String())
	}
	if _, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic"); ok {
		t.Error("expected nothing to be saved for an invalid conference total")
	}
	if _, ok := st.FindDivisionPlayoffTeams("basti", "Central"); ok {
		t.Error("expected the whole submission to be all-or-nothing: a valid sibling conference must not be saved either")
	}
	if _, ok := st.FindDivisionWinner("basti", "Atlantic"); ok {
		t.Error("expected the whole submission to be all-or-nothing: the valid winner picks must not be saved either")
	}
}

func TestPostDivisionsSheetShouldRejectADivisionExceedingFiveEvenWhenItsConferenceTotalsEight(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	picks := validDivisionPicks()
	// Atlantic now has 6 (over its own 5-team cap); Metropolitan drops to 2 so
	// the Eastern conference total still nets out to exactly 8 - proving the
	// server enforces the per-division cap independently of the conference
	// total (the bug this spec's review loop 1 fixed).
	picks["Atlantic"] = append(picks["Atlantic"], "MTL", "OTT")
	picks["Metropolitan"] = picks["Metropolitan"][:2]

	rec := postDivisionsForm(t, handler, picks, validDivisionWinners())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), divisionsCountErrorText) {
		t.Errorf("expected the inline error caption, got %q", rec.Body.String())
	}
	if _, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic"); ok {
		t.Error("expected nothing to be saved when a division exceeds its 5-team cap")
	}
	if _, ok := st.FindDivisionWinner("basti", "Atlantic"); ok {
		t.Error("expected the whole submission to be all-or-nothing: the valid winner picks must not be saved either")
	}
}

func TestPostDivisionsSheetShouldTreatARepeatedTeamIDAsOneSelection(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	picks := map[string][]string{
		"Atlantic":     {"BOS", "BOS", "BUF", "DET"}, // 4 raw values, 3 unique.
		"Metropolitan": {"CAR", "CBJ", "NJD", "NYI", "NYR"},
		"Central":      {"CHI", "COL", "DAL", "MIN"},
		"Pacific":      {"ANA", "CGY", "EDM", "LAK"},
	}

	rec := postDivisionsForm(t, handler, picks, validDivisionWinners())

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d (a repeated id should count once, keeping Eastern at 3+5=8), got %d: %s", http.StatusFound, rec.Code, rec.Body.String())
	}

	atlantic, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if !ok {
		t.Fatal("expected a saved Atlantic row")
	}
	if len(atlantic.TeamIDs) != 3 {
		t.Errorf("expected the repeated id to be deduped to 3 team ids, got %v", atlantic.TeamIDs)
	}
}

func TestPostDivisionsSheetShouldRejectAForeignDivisionTeamIDAndNotInflateTheRenderedCount(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	picks := validDivisionPicks()
	picks["Atlantic"] = append(picks["Atlantic"], "WSH") // WSH belongs to Metropolitan, not Atlantic.

	rec := postDivisionsForm(t, handler, picks, validDivisionWinners())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), divisionsCountErrorText) {
		t.Errorf("expected the inline error caption, got %q", rec.Body.String())
	}
	if _, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic"); ok {
		t.Error("expected nothing to be saved for a foreign-division team id")
	}
	// The foreign id must not inflate Atlantic's re-rendered count: only its
	// own 4 valid teams belong to Atlantic's roster, not the WSH intruder.
	if !strings.Contains(rec.Body.String(), `4/5`) {
		t.Errorf("expected Atlantic's re-rendered count to stay 4/5, not counting the foreign id, got %q", rec.Body.String())
	}
	if _, ok := st.FindDivisionWinner("basti", "Atlantic"); ok {
		t.Error("expected the whole submission to be all-or-nothing: the valid winner picks must not be saved either")
	}
}

func TestPostDivisionsSheetShouldSaveNonEmptyWinnersAndSkipABlankOne(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	winners := validDivisionWinners()
	winners["Pacific"] = ""

	rec := postDivisionsForm(t, handler, validDivisionPicks(), winners)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusFound, rec.Code, rec.Body.String())
	}
	if _, ok := st.FindDivisionPlayoffTeams("basti", "Pacific"); !ok {
		t.Error("expected the Pacific playoff-teams row to still be saved")
	}
	if _, ok := st.FindDivisionWinner("basti", "Pacific"); ok {
		t.Error("expected no Pacific winner row for a blank winner pick")
	}
	if got, ok := st.FindDivisionWinner("basti", "Atlantic"); !ok || got.TeamID != "BOS" {
		t.Errorf("expected the Atlantic winner to still be saved, got %+v (ok=%v)", got, ok)
	}
}

func TestPostDivisionsSheetShouldRejectAnUnknownWinnerIDAndSaveNothing(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	winners := validDivisionWinners()
	winners["Atlantic"] = "WSH" // WSH belongs to Metropolitan, not Atlantic.

	rec := postDivisionsForm(t, handler, validDivisionPicks(), winners)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), divisionsWinnerErrorText) {
		t.Errorf("expected the inline error caption, got %q", rec.Body.String())
	}
	if _, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic"); ok {
		t.Error("expected the whole submission to be rejected (all-or-nothing), including the valid playoff-teams picks")
	}
	if _, ok := st.FindDivisionWinner("basti", "Metropolitan"); ok {
		t.Error("expected nothing to be saved for an invalid winner pick")
	}
}

func TestPostDivisionsSheetShouldRejectAfterTheDeadlineWithNoOverride(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postDivisionsForm(t, handler, validDivisionPicks(), validDivisionWinners())

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if _, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic"); ok {
		t.Error("expected nothing to be saved after the deadline")
	}
}

func TestPostDivisionsSheetShouldUpdateExistingPicksInPlaceOnResubmission(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	if rec := postDivisionsForm(t, handler, validDivisionPicks(), validDivisionWinners()); rec.Code != http.StatusFound {
		t.Fatalf("expected the first submission to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	updatedPicks := map[string][]string{
		"Atlantic":     {"MTL", "OTT", "TBL", "TOR"},
		"Metropolitan": validDivisionPicks()["Metropolitan"],
		"Central":      validDivisionPicks()["Central"],
		"Pacific":      validDivisionPicks()["Pacific"],
	}
	rec := postDivisionsForm(t, handler, updatedPicks, validDivisionWinners())
	if rec.Code != http.StatusFound {
		t.Fatalf("expected the resubmission to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	got, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if !ok {
		t.Fatal("expected the Atlantic row to still be found")
	}
	if len(got.TeamIDs) != 4 || got.TeamIDs[0] != "MTL" {
		t.Errorf("expected the updated team ids %v, got %v", updatedPicks["Atlantic"], got.TeamIDs)
	}
}

func TestPostDivisionsSheetShouldReturn500WhenTheStoreWriteFails(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st, dir := newTestStoreWithDivisionTeamsAndDir(t, divisionsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	// Remove the directory out from under the store so SaveDivisionPicks'
	// atomic write-and-rename fails, mirroring
	// TestPostPredictSheetShouldReturn500WhenTheStoreWriteFails's own
	// technique.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	rec := postDivisionsForm(t, handler, validDivisionPicks(), validDivisionWinners())

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d when the store write fails, got %d", http.StatusInternalServerError, rec.Code)
	}
}
