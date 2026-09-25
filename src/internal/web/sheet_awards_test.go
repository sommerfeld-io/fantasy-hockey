package web

import (
	"fmt"
	"html"
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

// awardsPredictionSetSeed is the "awards" Prediction Set's raw
// prediction_sets YAML block with deadline as its deadline_utc, for tests
// exercising the awards pick-entry sheet.
func awardsPredictionSetSeed(deadline time.Time) string {
	return fmt.Sprintf(`    - id: awards
      title: Player awards
      subtitle: Hart, Norris, Vezina, Art Ross, Rocket
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))
}

// awardsNHLPlayersYAML is a small representative nhl_players: fixture
// spanning all 3 positions, for tests exercising the awards sheet's
// position-scoped autocomplete embeds and server-side slug re-validation.
const awardsNHLPlayersYAML = `nhl_players:
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

// newTestStoreWithAwardsRoster seeds a store with one player, the given raw
// prediction_sets YAML block, and awardsNHLPlayersYAML's small
// representative nhl_players roster - the awards pick-entry sheet's own
// fixture, mirroring newTestStoreWithDivisionTeams' own precedent.
func newTestStoreWithAwardsRoster(t *testing.T, predictionSetsYAML string) *store.Store {
	t.Helper()
	st, _ := newTestStoreWithAwardsRosterAndDir(t, predictionSetsYAML)
	return st
}

// newTestStoreWithAwardsRosterAndDir is newTestStoreWithAwardsRoster's own
// variant that also returns the seeded temp directory, for tests that need
// to remove it out from under the store (e.g. to force a write failure) -
// mirrors newTestStoreWithDivisionTeamsAndDir's own reason for existing.
func newTestStoreWithAwardsRosterAndDir(t *testing.T, predictionSetsYAML string) (*store.Store, string) {
	t.Helper()
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
prediction_sets:
` + predictionSetsYAML + awardsNHLPlayersYAML
	return newSeededStore(t, seed)
}

// awardSlot is a small constructor for awardSlotSubmission, for tests that
// build a full 5-award submission by hand.
func awardSlot(text, slug string) awardSlotSubmission {
	return awardSlotSubmission{Text: text, Slug: slug}
}

// assertNoAwardFinalistsSaved fails t unless none of awardOrder's 5 awards
// have a saved FindAwardFinalists row for "basti" - a rejected submission's
// all-or-nothing guarantee applies across every sibling award, not just the
// one or two an individual test happens to spot-check.
func assertNoAwardFinalistsSaved(t *testing.T, st *store.Store) {
	t.Helper()
	for _, award := range awardOrder {
		if _, ok := st.FindAwardFinalists("basti", award); ok {
			t.Errorf("expected the whole submission to be all-or-nothing: %q must not be saved either", award)
		}
	}
}

// validAwardFinalistsForm is one valid, fully-resolved submission across
// all 5 awards, each finalist trio matching awardsNHLPlayersYAML's own
// roster and each award's own eligible position.
func validAwardFinalistsForm() map[string][awardFinalistCount]awardSlotSubmission {
	skaters := [awardFinalistCount]awardSlotSubmission{
		awardSlot("Connor McDavid", "mcdavid-connor"),
		awardSlot("Nathan MacKinnon", "mackinnon-nathan"),
		awardSlot("Nikita Kucherov", "kucherov-nikita"),
	}
	return map[string][awardFinalistCount]awardSlotSubmission{
		store.AwardHart:          skaters,
		store.AwardArtRoss:       skaters,
		store.AwardRocketRichard: skaters,
		store.AwardNorris: {
			awardSlot("Cale Makar", "makar-cale"),
			awardSlot("Quinn Hughes", "hughes-quinn"),
			awardSlot("Zach Werenski", "werenski-zach"),
		},
		store.AwardVezina: {
			awardSlot("Connor Hellebuyck", "hellebuyck-connor"),
			awardSlot("Igor Shesterkin", "shesterkin-igor"),
			awardSlot("Andrei Vasilevskiy", "vasilevskiy-andrei"),
		},
	}
}

// postAwardsForm POSTs slots (award -> its 3 (text, slug) pairs) to
// /predict/awards as the signed-in test player, following no redirect, and
// returns the recorded response. An award absent from slots simply submits
// 3 blank text/slug pairs for it, matching what an untouched trophy group
// sends.
func postAwardsForm(t *testing.T, handler http.Handler, slots map[string][awardFinalistCount]awardSlotSubmission) *httptest.ResponseRecorder {
	t.Helper()
	values := url.Values{}
	for _, award := range awardOrder {
		pairs := slots[award]
		for i := 0; i < awardFinalistCount; i++ {
			values.Set(awardFinalistTextFieldName(award, i), pairs[i].Text)
			values.Set(awardFinalistSlugFieldName(award, i), pairs[i].Slug)
		}
	}

	req := httptest.NewRequest("POST", "/predict/awards", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// getAwardsSheet GETs /predict/awards as the signed-in test player and
// returns the recorded response.
func getAwardsSheet(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/predict/awards", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestGetAwardsSheetShouldRenderFiveEmptyTrophyGroupsWithNoGreenChecks(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := getAwardsSheet(t, handler)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	// A single flat table (rather than one if-per-assertion) keeps this
	// test's own cyclomatic complexity under gocyclo's threshold.
	wantPresent := []string{
		`data-award="hart"`, `data-award="norris"`, `data-award="vezina"`, `data-award="art_ross"`, `data-award="rocket_richard"`,
		"Hart Trophy finalists", "Norris Trophy finalists", "Vezina Trophy finalists", "Art Ross finalists", "Rocket Richard finalists",
		`>Submit predictions</button>`,
		`<script type="application/json" id="award-options-skater">`, `"mcdavid-connor"`,
		`<script type="application/json" id="award-options-defenseman">`, `"makar-cale"`,
		`<script type="application/json" id="award-options-goalie">`, `"hellebuyck-connor"`,
		`<script src="/static/awards.js" defer></script>`,
	}
	for _, want := range wantPresent {
		if !strings.Contains(body, want) {
			t.Errorf("expected the awards sheet to contain %q, got %q", want, body)
		}
	}

	wantAbsent := []string{"award-group--filled", "Update predictions"}
	for _, unwanted := range wantAbsent {
		if strings.Contains(body, unwanted) {
			t.Errorf("expected the awards sheet not to contain %q, got %q", unwanted, body)
		}
	}

	// A bare Contains(body, `value=""`) would pass even if only one of the
	// 15 finalist text/hidden-slug pairs actually started empty - count
	// every occurrence instead: 5 awards * 3 slots * 2 fields (text + slug).
	if got := strings.Count(body, `value=""`); got != 30 {
		t.Errorf("expected all 15 finalist text/slug pairs to start empty (30 occurrences of value=\"\"), got %d in %q", got, body)
	}
}

func TestGetAwardsSheetShouldPreselectSavedPicksAndReadUpdatePredictions(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	if err := st.SaveAwardPicks("basti", map[string][]string{
		store.AwardHart: {"mcdavid-connor", "mackinnon-nathan", "kucherov-nikita"},
	}, time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveAwardPicks: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	rec := getAwardsSheet(t, handler)

	body := rec.Body.String()
	if !strings.Contains(body, `value="Connor McDavid"`) {
		t.Errorf("expected the saved Hart finalist's display name preselected, got %q", body)
	}
	if !strings.Contains(body, `value="mcdavid-connor"`) {
		t.Errorf("expected the saved Hart finalist's slug preselected, got %q", body)
	}
	if !strings.Contains(body, `class="award-group award-group--filled" data-award="hart"`) {
		t.Errorf("expected the Hart trophy group to show its green check, got %q", body)
	}
	if strings.Contains(body, `class="award-group award-group--filled" data-award="norris"`) {
		t.Errorf("expected the unfilled Norris trophy group to show no green check, got %q", body)
	}
	if !strings.Contains(body, `>Update predictions</button>`) {
		t.Errorf("expected the button to read \"Update predictions\", got %q", body)
	}
}

func TestGetAwardsSheetShouldRenderAReadOnlyBannerAndDisabledInputsWhenClosed(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	if err := st.SaveAwardPicks("basti", map[string][]string{
		store.AwardHart: {"mcdavid-connor", "mackinnon-nathan", "kucherov-nikita"},
	}, time.Now().UTC().Add(-48*time.Hour)); err != nil {
		t.Fatalf("seed SaveAwardPicks: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	rec := getAwardsSheet(t, handler)

	body := rec.Body.String()
	if !strings.Contains(body, `value="Connor McDavid" placeholder="Player name" aria-label="Hart Trophy finalists, slot 1" autocomplete="off" disabled>`) {
		t.Errorf("expected a preselected finalist input to render disabled, got %q", body)
	}
	// One slot-1 input per remaining (blank, unseeded) award - not just
	// Hart's own seeded one - so a per-award template bug wouldn't slip
	// past this test undetected.
	wantBlankDisabled := map[string]string{
		"Norris Trophy finalists":  "Defenseman name",
		"Vezina Trophy finalists":  "Goalie name",
		"Art Ross finalists":       "Player name",
		"Rocket Richard finalists": "Player name",
	}
	for award, placeholder := range wantBlankDisabled {
		if !strings.Contains(body, `value="" placeholder="`+placeholder+`" aria-label="`+award+`, slot 1" autocomplete="off" disabled>`) {
			t.Errorf("expected %s's blank slot 1 input to render disabled, got %q", award, body)
		}
	}
	if !strings.Contains(body, "closed-banner") {
		t.Errorf("expected the read-only banner, got %q", body)
	}
	if strings.Contains(body, `<button`) {
		t.Errorf("expected no action bar (submit button) on a closed set, got %q", body)
	}
}

func TestGetAwardsSheetShouldNeverCreateARowMerelyByOpeningTheSheet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	getAwardsSheet(t, handler)

	if _, ok := st.FindAwardFinalists("basti", store.AwardHart); ok {
		t.Error("expected no Prediction row to be force-created merely by opening the sheet")
	}
}

// TestPredictShouldShowOpenStatusForAwardsBeforeAnyPickIsSaved mirrors
// TestPredictShouldShowOpenStatusForDivisionsBeforeAnyPickIsSaved: spec-2-6's
// own Intent says awards' Submitted-state logic mirrors Divisions' "at least
// one part saved" convention, so the same pre-save verification applies.
func TestPredictShouldShowOpenStatusForAwardsBeforeAnyPickIsSaved(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))

	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()
	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `<a id="predict-row-awards" href="/predict/awards" class="set-row set-row--open">`) {
		t.Errorf("expected the awards row to show Open before any pick is saved, got %q", rec.Body.String())
	}
}

func TestPostAwardsSheetShouldSaveAllFiveAwardsAndRedirectToPredict(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postAwardsForm(t, handler, validAwardFinalistsForm())

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusFound, rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/predict" {
		t.Errorf("expected a redirect to %q, got %q", "/predict", rec.Header().Get("Location"))
	}

	for _, award := range awardOrder {
		got, ok := st.FindAwardFinalists("basti", award)
		if !ok || len(got.FinalistSlugs) != 3 {
			t.Errorf("expected a saved 3-slug row for %q, got %+v (ok=%v)", award, got, ok)
		}
	}

	predictReq := httptest.NewRequest("GET", "/predict", nil)
	predictReq.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	predictRec := httptest.NewRecorder()
	handler.ServeHTTP(predictRec, predictReq)
	if !strings.Contains(predictRec.Body.String(), `<a id="predict-row-awards" href="/predict/awards" class="set-row set-row--submitted">`) {
		t.Errorf("expected the awards row to show Submitted, got %q", predictRec.Body.String())
	}
}

func TestPostAwardsSheetShouldSaveOnlyTheCompleteAwardsAndLeaveOthersUnsaved(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	slots := validAwardFinalistsForm()
	delete(slots, store.AwardVezina)  // leave Vezina fully blank.
	delete(slots, store.AwardArtRoss) // leave Art Ross fully blank.
	delete(slots, store.AwardRocketRichard)

	rec := postAwardsForm(t, handler, slots)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusFound, rec.Code, rec.Body.String())
	}
	if _, ok := st.FindAwardFinalists("basti", store.AwardHart); !ok {
		t.Error("expected the complete Hart award to be saved")
	}
	if _, ok := st.FindAwardFinalists("basti", store.AwardNorris); !ok {
		t.Error("expected the complete Norris award to be saved")
	}
	if _, ok := st.FindAwardFinalists("basti", store.AwardVezina); ok {
		t.Error("expected the blank Vezina award to stay unsaved")
	}
}

// TestPostAwardsSheetShouldAcceptAndSkipAPartiallyFilledAward covers the
// spec's own boundary: "an award left partially or fully blank simply isn't
// saved this submission" - a 2-of-3-filled award must be treated exactly
// like a fully-blank one (accepted, not saved, no error), never rejected as
// incomplete and never partially persisted.
func TestPostAwardsSheetShouldAcceptAndSkipAPartiallyFilledAward(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	slots := validAwardFinalistsForm()
	hart := slots[store.AwardHart]
	hart[2] = awardSlot("", "") // slot 2 left truly blank - 2 of 3 filled.
	slots[store.AwardHart] = hart

	rec := postAwardsForm(t, handler, slots)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d (a partially-filled award must not block the rest of the submission), got %d: %s", http.StatusFound, rec.Code, rec.Body.String())
	}
	if _, ok := st.FindAwardFinalists("basti", store.AwardHart); ok {
		t.Error("expected the 2-of-3-filled Hart award to stay unsaved, same as a fully-blank award")
	}
	if _, ok := st.FindAwardFinalists("basti", store.AwardNorris); !ok {
		t.Error("expected the complete Norris award to still be saved")
	}
}

func TestPostAwardsSheetShouldRejectATypedButUnresolvedNameAndSaveNothing(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	slots := validAwardFinalistsForm()
	hart := slots[store.AwardHart]
	hart[1] = awardSlot("Not A Real Player", "") // typed text, no resolved slug.
	slots[store.AwardHart] = hart

	rec := postAwardsForm(t, handler, slots)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), invalidFinalistErrorText) {
		t.Errorf("expected the inline error caption, got %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `class="finalist-text error"`) {
		t.Errorf("expected the offending slot's goal-border class, got %q", rec.Body.String())
	}
	assertNoAwardFinalistsSaved(t, st)
}

// TestPostAwardsSheetShouldEscapeRejectedFinalistTextInTheRerenderedHTML
// covers the first place in this codebase a genuinely free-typed, arbitrary
// string (not a value drawn from a closed set like a team id) gets echoed
// straight back into a rendered HTML attribute (sheet.html's
// value="{{.Text}}"). html/template auto-escapes this by default, but
// nothing proved it before this test - a future refactor casting .Text to
// template.HTML (as this same diff already does deliberately for the JSON
// option blocks, via template.JS) could silently reintroduce an XSS path.
func TestPostAwardsSheetShouldEscapeRejectedFinalistTextInTheRerenderedHTML(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	const malicious = `"><script>alert(1)</script>`
	slots := validAwardFinalistsForm()
	hart := slots[store.AwardHart]
	hart[1] = awardSlot(malicious, "") // typed text, no resolved slug - rejected and echoed back.
	slots[store.AwardHart] = hart

	rec := postAwardsForm(t, handler, slots)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, malicious) {
		t.Errorf("expected the rejected finalist text to be HTML-escaped, found it raw/unescaped in %q", body)
	}
	if !strings.Contains(body, html.EscapeString(malicious)) {
		t.Errorf("expected the rejected finalist text to appear HTML-escaped, got %q", body)
	}
}

func TestPostAwardsSheetShouldRejectASlugBelongingToTheWrongPositionAndSaveNothing(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	slots := validAwardFinalistsForm()
	norris := slots[store.AwardNorris]
	norris[0] = awardSlot("Connor Hellebuyck", "hellebuyck-connor") // a goalie's slug submitted for Norris (needs a defenseman).
	slots[store.AwardNorris] = norris

	rec := postAwardsForm(t, handler, slots)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), invalidFinalistErrorText) {
		t.Errorf("expected the inline error caption, got %q", rec.Body.String())
	}
	assertNoAwardFinalistsSaved(t, st)
}

// TestPostAwardsSheetShouldRejectADuplicateSlugWithinOneAwardAndSaveNothing
// covers "pick 3 finalists" inherently meaning 3 distinct players: the same
// NHL Player's slug submitted twice within one award's own 3 slots must be
// rejected the same way an unresolved or wrong-position slug is, even
// though each individual slot's own slug resolves validly on its own.
func TestPostAwardsSheetShouldRejectADuplicateSlugWithinOneAwardAndSaveNothing(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	slots := validAwardFinalistsForm()
	hart := slots[store.AwardHart]
	hart[1] = awardSlot("Connor McDavid", "mcdavid-connor") // same slug as slot 0.
	slots[store.AwardHart] = hart

	rec := postAwardsForm(t, handler, slots)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), invalidFinalistErrorText) {
		t.Errorf("expected the inline error caption, got %q", rec.Body.String())
	}
	assertNoAwardFinalistsSaved(t, st)
}

// TestPostAwardsSheetShouldNotShowTheGreenCheckForAnAwardWithADuplicateSlug
// covers the rendered inconsistency a duplicate slug would otherwise cause:
// each individual slot resolves to a valid, correctly-positioned player, so
// naively checking per-slot validity alone would still show the award's
// green check even while its slots also show the rejection error styling.
func TestPostAwardsSheetShouldNotShowTheGreenCheckForAnAwardWithADuplicateSlug(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	slots := validAwardFinalistsForm()
	hart := slots[store.AwardHart]
	hart[1] = awardSlot("Connor McDavid", "mcdavid-connor") // same slug as slot 0.
	slots[store.AwardHart] = hart

	rec := postAwardsForm(t, handler, slots)

	if strings.Contains(rec.Body.String(), `class="award-group award-group--filled" data-award="hart"`) {
		t.Errorf("expected no green check for an award with a duplicate slug, got %q", rec.Body.String())
	}
}

// TestGetAwardsSheetShouldNotMarkAPreviouslySavedNowUnresolvedSlugAsInvalid
// covers a previously-saved, valid finalist slug that later stops resolving
// (e.g. the maintainer edits the nhl_players: roster mid-season, or removes
// a player) - opening the sheet must never show the rejection-only
// goal-border/caption styling, which is reserved for a rejected
// resubmission's own re-render.
func TestGetAwardsSheetShouldNotMarkAPreviouslySavedNowUnresolvedSlugAsInvalid(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	if err := st.SaveAwardPicks("basti", map[string][]string{
		store.AwardHart: {"mcdavid-connor", "retired-player-not-on-roster", "kucherov-nikita"},
	}, time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveAwardPicks: %v", err)
	}
	handler := NewServer(st, noopSender, testSecret)

	rec := getAwardsSheet(t, handler)

	body := rec.Body.String()
	if strings.Contains(body, `class="finalist-text error"`) {
		t.Errorf("expected no rejection-style error class on a plain saved-state render, got %q", body)
	}
	if strings.Contains(body, invalidFinalistErrorText) {
		t.Errorf("expected no rejection caption on a plain saved-state render, got %q", body)
	}
}

func TestPostAwardsSheetShouldRejectAfterTheDeadlineWithNoOverride(t *testing.T) {
	deadline := time.Now().UTC().Add(-24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	rec := postAwardsForm(t, handler, validAwardFinalistsForm())

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if _, ok := st.FindAwardFinalists("basti", store.AwardHart); ok {
		t.Error("expected nothing to be saved after the deadline")
	}
}

func TestPostAwardsSheetShouldUpdateAnExistingAwardInPlaceOnResubmission(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	if rec := postAwardsForm(t, handler, validAwardFinalistsForm()); rec.Code != http.StatusFound {
		t.Fatalf("expected the first submission to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	updated := validAwardFinalistsForm()
	updated[store.AwardHart] = [awardFinalistCount]awardSlotSubmission{
		awardSlot("Nikita Kucherov", "kucherov-nikita"),
		awardSlot("Nathan MacKinnon", "mackinnon-nathan"),
		awardSlot("Connor McDavid", "mcdavid-connor"),
	}
	rec := postAwardsForm(t, handler, updated)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected the resubmission to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	got, ok := st.FindAwardFinalists("basti", store.AwardHart)
	if !ok || len(got.FinalistSlugs) != 3 || got.FinalistSlugs[0] != "kucherov-nikita" {
		t.Errorf("expected the updated finalist slugs %v, got %v", []string{"kucherov-nikita", "mackinnon-nathan", "mcdavid-connor"}, got.FinalistSlugs)
	}
}

func TestPostAwardsSheetShouldReturn500WhenTheStoreWriteFails(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st, dir := newTestStoreWithAwardsRosterAndDir(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret)

	// Remove the directory out from under the store so SaveAwardPicks'
	// atomic write-and-rename fails, mirroring
	// TestPostDivisionsSheetShouldReturn500WhenTheStoreWriteFails's own
	// technique.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	rec := postAwardsForm(t, handler, validAwardFinalistsForm())

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d when the store write fails, got %d", http.StatusInternalServerError, rec.Code)
	}
}
