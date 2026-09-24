package web

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

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
