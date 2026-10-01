package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/observe"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

func TestPostAwardsSheetShouldWriteOneAuditLinePerPersistedRow(t *testing.T) {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour)
	st := newTestStoreWithAwardsRoster(t, awardsPredictionSetSeed(deadline))
	handler := NewServer(st, noopSender, testSecret, observe.New())
	lines := captureAuditLogs(t)

	rec := postAwardsForm(t, handler, validAwardFinalistsForm())
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}

	got := lines()
	if len(got) != 5 {
		t.Fatalf("expected 5 audit lines, got %d: %q", len(got), got)
	}
	for _, line := range got {
		for _, want := range []string{"event=prediction_saved", "player_id=basti", "kind=" + store.KindAward} {
			if !strings.Contains(line, want) {
				t.Errorf("expected %q in %q", want, line)
			}
		}
	}
}

func TestPostSheetShouldNotWriteAnAuditLineForARejectedSave(t *testing.T) {
	deadline := time.Now().UTC().Add(-time.Hour)
	handler := NewServer(newTestStoreWithPredictionSetsAndTeams(t, cupPredictionSetSeed(deadline)), noopSender, testSecret, observe.New())
	lines := captureAuditLogs(t)

	rec := postSheet(t, handler, "cup", "TOR")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if got := lines(); len(got) != 0 {
		t.Errorf("expected no audit line, got %q", got)
	}
}
