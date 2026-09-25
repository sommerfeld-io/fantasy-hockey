package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// TestGetPredictSheetShouldRenderTheStubPageForAKnownSet covers a Prediction
// Set id that isn't one of pickableSheetKinds - every id but "cup"/
// "presidents"/"playoffcup"/divisionsSetID/awardsSetID/seriesSetIDs keeps
// rendering the unchanged stub (Boundaries & Constraints). Story 3.3 made
// every remaining Epic 3 id ("r1"/"r2"/"cf"/"scf") real, so no genuinely
// future id remains to repoint this test onto (Story 2.3/2.4/2.6/3.3's own
// precedent) - "mystery-set" is a synthetic id that will never be one of
// pickableSheetKinds, standing in for whatever kind gets built next.
func TestGetPredictSheetShouldRenderTheStubPageForAKnownSet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	seed := fmt.Sprintf(`    - id: mystery-set
      title: Mystery set
      subtitle: Not built yet
      deadline_utc: %q
      phase: playoffs
      upcoming: false
`, deadline.Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict/mystery-set", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Mystery set") {
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
// non-pickable id whose deadline has passed - it still gets the static
// stub, never the cup/presidents/divisions/awards closed read-only banner.
func TestGetPredictSheetShouldRenderTheStubPageForAClosedSet(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	seed := fmt.Sprintf(`    - id: mystery-set
      title: Mystery set
      subtitle: Not built yet
      deadline_utc: %q
      phase: playoffs
      upcoming: false
`, deadline.Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict/mystery-set", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Mystery set") {
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

// TestPostPredictSheetShouldSaveAValidPickForPlayoffsCupAndShowSubmittedOnReload
// proves the shared pick-entry mechanic end-to-end for "playoffcup" too, not
// only "cup"/"presidents": save, redirect, a reopened sheet pre-filled with
// "Update predictions", and the Predict list showing Submitted for that row.
func TestPostPredictSheetShouldSaveAValidPickForPlayoffsCupAndShowSubmittedOnReload(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, playoffsCupPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postSheet(t, handler, store.KindPlayoffsCup, "VGK")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/predict" {
		t.Errorf("expected a redirect to %q, got %q", "/predict", rec.Header().Get("Location"))
	}

	prediction, ok := st.FindPrediction("basti", store.KindPlayoffsCup)
	if !ok {
		t.Fatal("expected a saved Prediction row for playoffcup")
	}
	if prediction.TeamID != "VGK" {
		t.Errorf("expected team id %q, got %q", "VGK", prediction.TeamID)
	}

	reopenReq := httptest.NewRequest("GET", "/predict/playoffcup", nil)
	reopenReq.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	reopenRec := httptest.NewRecorder()
	handler.ServeHTTP(reopenRec, reopenReq)

	reopenBody := reopenRec.Body.String()
	if !strings.Contains(reopenBody, `<form class="pick-form" method="post" action="/predict/playoffcup">`) {
		t.Errorf("expected the playoffcup sheet to submit to /predict/playoffcup, got %q", reopenBody)
	}
	if !strings.Contains(reopenBody, `<option value="VGK" selected>Vegas Golden Knights</option>`) {
		t.Errorf("expected the saved playoffcup pick preselected on reload, got %q", reopenBody)
	}
	if !strings.Contains(reopenBody, `>Update predictions</button>`) {
		t.Errorf("expected the button to read \"Update predictions\" on reload, got %q", reopenBody)
	}

	predictReq := httptest.NewRequest("GET", "/predict", nil)
	predictReq.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	predictRec := httptest.NewRecorder()
	handler.ServeHTTP(predictRec, predictReq)

	if !strings.Contains(predictRec.Body.String(), `<a id="predict-row-playoffcup" href="/predict/playoffcup" class="set-row set-row--submitted">`) {
		t.Errorf("expected the playoffcup row to show Submitted on the Predict list, got %q", predictRec.Body.String())
	}
}

// TestPostPredictSheetShouldNotOverwriteTheSeasonCupPickWhenSavingPlayoffsCup
// proves the Intent's independence requirement: saving a playoffcup pick
// never touches the player's already-saved season-opening cup pick, since
// the two are distinct (PlayerID, Kind) rows.
func TestPostPredictSheetShouldNotOverwriteTheSeasonCupPickWhenSavingPlayoffsCup(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline)+playoffsCupPredictionSetSeed(deadline))
	if err := st.SavePrediction("basti", store.KindCupChampion, "TOR", time.Now().UTC()); err != nil {
		t.Fatalf("seed SavePrediction: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	rec := postSheet(t, handler, store.KindPlayoffsCup, "VGK")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}

	playoffsCupPick, ok := st.FindPrediction("basti", store.KindPlayoffsCup)
	if !ok {
		t.Fatal("expected a saved playoffcup Prediction row")
	}
	if playoffsCupPick.TeamID != "VGK" {
		t.Errorf("expected the playoffcup pick to be %q, got %q", "VGK", playoffsCupPick.TeamID)
	}

	seasonCupPick, ok := st.FindPrediction("basti", store.KindCupChampion)
	if !ok {
		t.Fatal("expected the season-opening cup Prediction row to still exist")
	}
	if seasonCupPick.TeamID != "TOR" {
		t.Errorf("expected the season-opening cup pick to remain %q, got %q", "TOR", seasonCupPick.TeamID)
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
	st, dir := newSeededStore(t, seed)
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

// TestPostPredictSheetShouldReturn404ForANonPickableSetID covers a
// Prediction Set id that isn't one of pickableSheetKinds - Story 3.3 made
// every remaining Epic 3 id real (mirroring the stub-page tests' own
// repointing above), so "mystery-set" stands in as a synthetic id that will
// never be one of pickableSheetKinds.
func TestPostPredictSheetShouldReturn404ForANonPickableSetID(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	seed := fmt.Sprintf(`    - id: mystery-set
      title: Mystery set
      subtitle: Not built yet
      deadline_utc: %q
      phase: playoffs
      upcoming: false
`, deadline.Format(time.RFC3339))
	handler := NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret)

	rec := postSheet(t, handler, "mystery-set", "TOR")

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

// markUpcoming flips a single-set prediction_sets seed from "upcoming: false"
// to "upcoming: true", so every per-kind seed helper gets an Upcoming
// variant without a second copy of its YAML.
func markUpcoming(seed string) string {
	return strings.Replace(seed, "upcoming: false", "upcoming: true", 1)
}

// getSheet requests GET /predict/{id} as the seeded player.
func getSheet(t *testing.T, handler http.Handler, id string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/predict/"+id, nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// unknownSheetBody is the body GET /predict/{id} returns for an id matching
// no Prediction Set - the generic not-found response an Upcoming set must be
// indistinguishable from.
func unknownSheetBody(t *testing.T) string {
	t.Helper()
	return getSheet(t, NewServer(newTestStore(t), noopSender, testSecret), "does-not-exist").Body.String()
}

func TestGetPredictSheetShouldReturn404ForAnUpcomingSet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, markUpcoming(cupPredictionSetSeed(deadline)))
	handler := NewServer(st, noopSender, testSecret)

	rec := getSheet(t, handler, "cup")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for an upcoming set, got %d", http.StatusNotFound, rec.Code)
	}
	if rec.Body.String() != unknownSheetBody(t) {
		t.Errorf("expected the generic not-found body, got %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Cup champion") {
		t.Errorf("expected no set details in the response, got %q", rec.Body.String())
	}
}

func TestPostPredictSheetShouldReturn404ForAnUpcomingSetAndSaveNothing(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, markUpcoming(cupPredictionSetSeed(deadline)))
	handler := NewServer(st, noopSender, testSecret)

	rec := postSheet(t, handler, "cup", "TOR")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for an upcoming set, got %d", http.StatusNotFound, rec.Code)
	}
	if rec.Body.String() != unknownSheetBody(t) {
		t.Errorf("expected the generic not-found body, got %q", rec.Body.String())
	}
	if _, ok := st.FindPrediction("basti", store.KindCupChampion); ok {
		t.Error("expected nothing to be saved for an upcoming set")
	}
}

// TestPredictSheetShouldReturn404NotForbiddenForAnUpcomingSetPastItsDeadline
// covers Upcoming winning over Closed, matching predictStatus's own
// precedence: an upcoming set rejects the same way whatever its deadline.
func TestPredictSheetShouldReturn404NotForbiddenForAnUpcomingSetPastItsDeadline(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, markUpcoming(cupPredictionSetSeed(deadline)))
	handler := NewServer(st, noopSender, testSecret)

	getRec := getSheet(t, handler, "cup")
	postRec := postSheet(t, handler, "cup", "TOR")

	if getRec.Code != http.StatusNotFound {
		t.Errorf("expected GET status %d, got %d", http.StatusNotFound, getRec.Code)
	}
	if postRec.Code != http.StatusNotFound {
		t.Errorf("expected POST status %d (not %d), got %d", http.StatusNotFound, http.StatusForbidden, postRec.Code)
	}
	if _, ok := st.FindPrediction("basti", store.KindCupChampion); ok {
		t.Error("expected nothing to be saved for an upcoming set")
	}
}

// upcomingGateCases is every pickable sheet kind, each with its own store
// fixture, a valid submission and a "was anything saved" probe - so the
// Upcoming gate (and its non-upcoming counterpart) is proven for every kind,
// not just cup.
var upcomingGateCases = []struct {
	id       string
	newStore func(t *testing.T, seed string) *store.Store
	seed     func(deadline time.Time) string
	post     func(t *testing.T, handler http.Handler) *httptest.ResponseRecorder
	saved    func(st *store.Store) bool
}{
	{
		id:       store.KindCupChampion,
		newStore: newTestStoreWithPredictionSetsAndTeams,
		seed:     cupPredictionSetSeed,
		post: func(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
			return postSheet(t, handler, store.KindCupChampion, "TOR")
		},
		saved: func(st *store.Store) bool {
			_, ok := st.FindPrediction("basti", store.KindCupChampion)
			return ok
		},
	},
	{
		id:       store.KindPresidentsTrophy,
		newStore: newTestStoreWithPredictionSetsAndTeams,
		seed:     presidentsPredictionSetSeed,
		post: func(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
			return postSheet(t, handler, store.KindPresidentsTrophy, "TOR")
		},
		saved: func(st *store.Store) bool {
			_, ok := st.FindPrediction("basti", store.KindPresidentsTrophy)
			return ok
		},
	},
	{
		id:       store.KindPlayoffsCup,
		newStore: newTestStoreWithPredictionSetsAndTeams,
		seed:     playoffsCupPredictionSetSeed,
		post: func(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
			return postSheet(t, handler, store.KindPlayoffsCup, "TOR")
		},
		saved: func(st *store.Store) bool {
			_, ok := st.FindPrediction("basti", store.KindPlayoffsCup)
			return ok
		},
	},
	{
		id:       divisionsSetID,
		newStore: newTestStoreWithDivisionTeams,
		seed:     divisionsPredictionSetSeed,
		post: func(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
			return postDivisionsForm(t, handler, validDivisionPicks(), validDivisionWinners())
		},
		saved: func(st *store.Store) bool {
			_, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
			return ok
		},
	},
	{
		id:       awardsSetID,
		newStore: newTestStoreWithAwardsRoster,
		seed:     awardsPredictionSetSeed,
		post: func(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
			return postAwardsForm(t, handler, validAwardFinalistsForm())
		},
		saved: func(st *store.Store) bool {
			_, ok := st.FindAwardFinalists("basti", store.AwardHart)
			return ok
		},
	},
}

func TestPredictSheetShouldReturn404ForEveryUpcomingPickableKindAndSaveNothing(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	for _, tc := range upcomingGateCases {
		t.Run(tc.id, func(t *testing.T) {
			st := tc.newStore(t, markUpcoming(tc.seed(deadline)))
			handler := NewServer(st, noopSender, testSecret)

			getRec := getSheet(t, handler, tc.id)
			postRec := tc.post(t, handler)

			if getRec.Code != http.StatusNotFound {
				t.Errorf("expected GET status %d, got %d", http.StatusNotFound, getRec.Code)
			}
			if postRec.Code != http.StatusNotFound {
				t.Errorf("expected POST status %d, got %d", http.StatusNotFound, postRec.Code)
			}
			if tc.saved(st) {
				t.Error("expected nothing to be saved for an upcoming set")
			}
		})
	}
}

func TestPredictSheetShouldStillServeAndSaveEveryNonUpcomingOpenPickableKind(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	for _, tc := range upcomingGateCases {
		t.Run(tc.id, func(t *testing.T) {
			st := tc.newStore(t, tc.seed(deadline))
			handler := NewServer(st, noopSender, testSecret)

			getRec := getSheet(t, handler, tc.id)
			postRec := tc.post(t, handler)

			if getRec.Code != http.StatusOK {
				t.Errorf("expected GET status %d, got %d", http.StatusOK, getRec.Code)
			}
			if postRec.Code != http.StatusFound {
				t.Errorf("expected POST status %d, got %d", http.StatusFound, postRec.Code)
			}
			if !tc.saved(st) {
				t.Error("expected the valid submission to be saved for a non-upcoming set")
			}
		})
	}
}

// TestGetPredictSheetShouldReturn404ForAnUpcomingStubSet proves the gate
// applies to every set kind, not only the pickable ones.
func TestGetPredictSheetShouldReturn404ForAnUpcomingStubSet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	seed := fmt.Sprintf(`    - id: r1
      title: Playoff round 1
      subtitle: 8 series — winner & length
      deadline_utc: %q
      phase: playoffs
      upcoming: true
`, deadline.Format(time.RFC3339))

	rec := getSheet(t, NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret), "r1")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for an upcoming stub set, got %d", http.StatusNotFound, rec.Code)
	}
}

// TestGetPredictSheetShouldReturn404ForARoundGatedSetWithNoMatchupsRecorded
// covers Story 3.2's direct-URL gate: findOpenablePredictionSet must call
// the same effectiveUpcoming helper the Predict list uses, so "cf" - a
// roundGatedSetIDs id with no entry under playoff_matchups - answers 404
// even though its own hand-maintained upcoming flag is false (which would
// otherwise open it).
func TestGetPredictSheetShouldReturn404ForARoundGatedSetWithNoMatchupsRecorded(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	seed := fmt.Sprintf(`    - id: cf
      title: Conference finals
      subtitle: Set once round 2 ends
      deadline_utc: %q
      phase: playoffs
      upcoming: false
`, deadline.Format(time.RFC3339))

	rec := getSheet(t, NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret), "cf")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for a round-gated set with no recorded matchups, got %d", http.StatusNotFound, rec.Code)
	}
	if rec.Body.String() != unknownSheetBody(t) {
		t.Errorf("expected the generic not-found body, got %q", rec.Body.String())
	}
}

// TestGetPredictSheetShouldServeEveryRoundGatedSetOnceAMatchupIsRecorded is
// the counterpart, exercised for all three roundGatedSetIDs (a typo in
// store.Round2SetID/store.ConferenceFinalsSetID/store.StanleyCupFinalSetID's
// literal value would otherwise go uncaught): once a matchup is recorded,
// the direct URL
// opens the set's series sheet, even though its own upcoming flag is true -
// the state today's real fantasy-hockey.yml actually seeds "r2"/"cf"/"scf"
// at.
func TestGetPredictSheetShouldServeEveryRoundGatedSetOnceAMatchupIsRecorded(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	tests := []struct{ id, title string }{
		{id: store.Round2SetID, title: "Round 2"},
		{id: store.ConferenceFinalsSetID, title: "Conference finals"},
		{id: store.StanleyCupFinalSetID, title: "Stanley Cup final"},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			seed := fmt.Sprintf(`    - id: %s
      title: %s
      subtitle: Set once the prior round ends
      deadline_utc: %q
      phase: playoffs
      upcoming: true
`, tc.id, tc.title, deadline.Format(time.RFC3339))
			matchups := fmt.Sprintf("    %s:\n        - key: s1\n          a: FLA\n          b: TOR\n", tc.id)

			rec := getSheet(t, NewServer(newTestStoreWithPredictionSetsAndMatchups(t, seed, matchups), noopSender, testSecret), tc.id)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d for %q with a recorded matchup despite upcoming: true, got %d", http.StatusOK, tc.id, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.title) {
				t.Errorf("expected the sheet title once unlocked, got %q", rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `data-series="s1"`) {
				t.Errorf("expected the recorded matchup's series card once unlocked, got %q", rec.Body.String())
			}
		})
	}
}

// TestSheetShouldReturn404ForAGatedRoundWhenOnlyAnotherRoundHasMatchups
// covers the direct-URL side of independent unlocking: with matchups
// recorded only under "r2", both GET and POST /predict/cf answer the generic
// 404, and nothing is saved.
func TestSheetShouldReturn404ForAGatedRoundWhenOnlyAnotherRoundHasMatchups(t *testing.T) {
	seed := gatedRoundsSeed(time.Now().UTC().Add(5 * 24 * time.Hour))
	st := newTestStoreWithPredictionSetsAndMatchups(t, seed, r2OnlyMatchupsYAML)
	handler := NewServer(st, noopSender, testSecret)

	rec := getSheet(t, handler, store.ConferenceFinalsSetID)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected GET status %d for cf with matchups only under r2, got %d", http.StatusNotFound, rec.Code)
	}
	if rec.Body.String() != unknownSheetBody(t) {
		t.Errorf("expected the generic not-found body, got %q", rec.Body.String())
	}

	rec = postSeriesForm(t, handler, store.ConferenceFinalsSetID, map[string]seriesSubmission{
		"s1": {TeamID: "FLA", Games: "6"},
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected POST status %d for cf with matchups only under r2, got %d", http.StatusNotFound, rec.Code)
	}
	if _, ok := st.FindSeriesPick("basti", store.JoinSeriesKey(store.ConferenceFinalsSetID, "s1")); ok {
		t.Error("expected nothing saved for a still-locked cf")
	}
}

// TestPostPredictSheetShouldReturn404ForAnUpcomingSetBeforeParsingTheForm
// proves the Upcoming gate runs before form parsing: an oversized body that
// would otherwise fail ParseForm (500) still gets the generic 404.
func TestPostPredictSheetShouldReturn404ForAnUpcomingSetBeforeParsingTheForm(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithPredictionSetsAndTeams(t, markUpcoming(cupPredictionSetSeed(deadline)))
	handler := NewServer(st, noopSender, testSecret)
	oversized := "team_id=TOR&pad=" + strings.Repeat("x", maxSheetFormBytes+1)

	req := httptest.NewRequest("POST", "/predict/cup", strings.NewReader(oversized))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d before form parsing, got %d", http.StatusNotFound, rec.Code)
	}
	if _, ok := st.FindPrediction("basti", store.KindCupChampion); ok {
		t.Error("expected nothing to be saved for an upcoming set")
	}
}
