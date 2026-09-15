package web

import (
	"crypto/sha256"
	"encoding/hex"
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
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
	path := filepath.Join(dir, "fantasy-hockey.yml")
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return st
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
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
		t.Errorf("expected fantasy-hockey.yml to stay untouched, got diff:\nbefore:\n%s\nafter:\n%s", before, after)
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

			phases := buildPredictPhases(st, now)

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

func TestGetPredictSheetShouldRenderTheStubPageForAKnownSet(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	seed := fmt.Sprintf(`    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: %q
      phase: before_season
      upcoming: false
`, deadline.Format(time.RFC3339))

	req := httptest.NewRequest("GET", "/predict/cup", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithPredictionSets(t, seed), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Cup champion") {
		t.Errorf("expected the sheet title, got %q", body)
	}
	if !strings.Contains(body, "in 5 days") {
		t.Errorf("expected the formatted countdown, got %q", body)
	}
	if !strings.Contains(body, "Not available yet.") {
		t.Errorf("expected the static stub body, got %q", body)
	}
	if strings.Contains(body, `<button`) || strings.Contains(body, `<form`) {
		t.Errorf("expected no pick-entry form or action bar on the stub page, got %q", body)
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
	path := filepath.Join(dir, "fantasy-hockey.yml")
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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on reuse, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), renderedGenericCodeErrorText) {
		t.Errorf("expected a reused code to show the generic error, got %q", rec.Body.String())
	}
}

func TestPostLoginCodeShouldShowTheGenericErrorAndRetainTheCodeOnAWrongCode(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())

	rec := postCode(t, NewServer(st, noopSender, testSecret), "000000")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for a missing code field, got %d", rec.Code)
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
