package web

import (
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

// playoffsCupPredictionSetSeed is one "playoffcup" Prediction Set's raw
// prediction_sets YAML block with deadline as its deadline_utc - Story 3.1's
// pickable id, sharing cup/presidents' own pick-entry sheet mechanic.
func playoffsCupPredictionSetSeed(deadline time.Time) string {
	return fmt.Sprintf(`    - id: playoffcup
      title: Playoffs Cup pick
      subtitle: Re-pick the Stanley Cup winner
      deadline_utc: %q
      phase: playoffs
      upcoming: false
`, deadline.Format(time.RFC3339))
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
