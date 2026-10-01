package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/observe"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

func quietAuditLogs(t *testing.T) {
	t.Helper()
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
}

func newCountedServer(t *testing.T, st *store.Store) (http.Handler, *observe.Observer) {
	t.Helper()
	quietAuditLogs(t)
	ob := observe.New()
	return NewServer(st, noopSender, testSecret, ob), ob
}

func scrapeMetrics(t *testing.T, ob *observe.Observer) string {
	t.Helper()
	rec := httptest.NewRecorder()
	ob.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

func assertSeries(t *testing.T, out, name, label, value, want string) {
	t.Helper()
	line := name + "{" + label + `="` + value + `"} ` + want + "\n"
	if !strings.Contains(out, line) {
		t.Errorf("expected %q in the metrics output", strings.TrimSpace(line))
	}
}

func assertLogin(t *testing.T, ob *observe.Observer, event, want string) {
	t.Helper()
	assertSeries(t, scrapeMetrics(t, ob), "fantasy_hockey_login_events_total", "event", event, want)
}

func assertSaves(t *testing.T, ob *observe.Observer, kind, want string) {
	t.Helper()
	assertSeries(t, scrapeMetrics(t, ob), "fantasy_hockey_prediction_saves_total", "kind", kind, want)
}

func postForm(handler http.Handler, path string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestNewServerShouldPreRegisterEverySaveKindAtZero(t *testing.T) {
	_, ob := newCountedServer(t, newTestStore(t))
	for _, kind := range store.Kinds {
		assertSaves(t, ob, kind, "0")
	}
}

func TestPostLoginShouldCountACodeRequestOnlyForAKnownEmail(t *testing.T) {
	handler, ob := newCountedServer(t, newTestStore(t))

	postForm(handler, "/login", url.Values{"email": {"nobody@example.com"}}, nil)
	assertLogin(t, ob, observe.EventLoginCodeRequested, "0")

	postForm(handler, "/login", url.Values{"email": {"basti@example.com"}}, nil)
	assertLogin(t, ob, observe.EventLoginCodeRequested, "1")
}

func TestPostLoginCodeShouldCountAFailureForAWrongCodeAndNoSuccess(t *testing.T) {
	handler, ob := newCountedServer(t, newTestStore(t))

	rec := postForm(handler, "/login/code", url.Values{"code": {"000000"}}, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	assertLogin(t, ob, observe.EventLoginFailed, "1")
	assertLogin(t, ob, observe.EventLoginSucceeded, "0")
}

func TestPostLogoutShouldCountOnlyWithAValidSession(t *testing.T) {
	handler, ob := newCountedServer(t, newTestStore(t))

	postForm(handler, "/logout", url.Values{}, nil)
	postForm(handler, "/logout", url.Values{}, &http.Cookie{Name: auth.SessionCookieName, Value: "garbage"})
	assertLogin(t, ob, observe.EventLogout, "0")

	postForm(handler, "/logout", url.Values{}, auth.IssueSessionCookie("basti", testSecret))
	assertLogin(t, ob, observe.EventLogout, "1")
}

func TestPostSheetShouldCountACupSaveAndNotARejectedOne(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	handler, ob := newCountedServer(t, newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline)))

	postSheet(t, handler, "cup", "NOPE")
	assertSaves(t, ob, store.KindCupChampion, "0")

	postSheet(t, handler, "cup", "TOR")
	assertSaves(t, ob, store.KindCupChampion, "1")
}

func TestPostSheetShouldNotCountASaveAfterTheDeadline(t *testing.T) {
	deadline := time.Now().UTC().Add(-time.Hour)
	handler, ob := newCountedServer(t, newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline)))

	rec := postSheet(t, handler, "cup", "TOR")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	assertSaves(t, ob, store.KindCupChampion, "0")
}

func TestPostAwardsSheetShouldCountOneSavePerPersistedAward(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	handler, ob := newCountedServer(t, newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline)))

	postAwardsForm(t, handler, map[string][awardFinalistCount]awardSlotSubmission{})
	assertSaves(t, ob, store.KindAward, "0")

	rec := postAwardsForm(t, handler, validAwardFinalistsForm())
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	assertSaves(t, ob, store.KindAward, "5")
}

func TestPostDivisionsSheetShouldCountOneSavePerPersistedRow(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	handler, ob := newCountedServer(t, newTestStoreWithDivisionTeams(t, divisionsPredictionSetSeed(deadline)))

	rec := postDivisionsForm(t, handler, validDivisionPicks(), validDivisionWinners())
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	assertSaves(t, ob, store.KindDivisionPlayoffTeams, "4")
	assertSaves(t, ob, store.KindDivisionWinner, "4")
}

func TestPostSeriesSheetShouldCountPersistedSeriesEvenWhenAnotherIsRejected(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithSeriesFixture(t, r1PredictionSetSeed(deadline), r1MatchupsYAML)
	handler, ob := newCountedServer(t, st)

	rec := postSeriesForm(t, handler, "r1", map[string]seriesSubmission{
		"e1": {TeamID: "BOS", Games: "6"},
		"e2": {TeamID: "TBL", Games: "5"},
		"e3": {TeamID: "COL", Games: "5"}, // foreign to e3: rejected.
		"e4": {TeamID: "TOR"},             // half-filled: skipped.
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for the rejected-pick page, got %d", rec.Code)
	}
	assertSaves(t, ob, store.KindSeries, "2")
}
