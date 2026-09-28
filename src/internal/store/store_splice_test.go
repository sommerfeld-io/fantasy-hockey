package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSavingAPredictionShouldPreserveTheHandRecordedResults(t *testing.T) {
	st, path := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	if err := st.SavePrediction("basti", KindCupChampion, "FLA", time.Now()); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	assertFixtureResultsRecorded(t, reopened)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	for _, injected := range []string{`position: ""`, `display_name: ""`} {
		if strings.Contains(string(raw), injected) {
			t.Errorf("expected a save not to inject %s, got:\n%s", injected, raw)
		}
	}
}

// assertFixtureResultsRecorded checks that st holds every one of the five
// result kinds resultsFixtureResults records, with no result problems.
func assertFixtureResultsRecorded(t *testing.T, st *Store) {
	t.Helper()
	checks := []struct {
		name string
		ok   func() bool
	}{
		{"StanleyCupWinner = FLA", func() bool { return st.StanleyCupWinner() == "FLA" }},
		{"PresidentsTrophyWinner = TOR", func() bool { return st.PresidentsTrophyWinner() == "TOR" }},
		{"DivisionResult(Atlantic) = [FLA TOR], FLA", func() bool {
			playoffs, winner := st.DivisionResult("Atlantic")
			return slices.Equal(playoffs, []string{"FLA", "TOR"}) && winner == "FLA"
		}},
		{"SeriesResult(r1.s1) = FLA in 5", func() bool {
			winner, games, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s1"))
			return ok && winner == "FLA" && games == "5"
		}},
		{"SeriesResult(cf.s1) = BOS in 7", func() bool {
			winner, games, ok := st.SeriesResult(JoinSeriesKey(ConferenceFinalsSetID, "s1"))
			return ok && winner == "BOS" && games == "7"
		}},
		{"RecordedAwardFinalists(hart) has 4 slugs", func() bool { return len(st.RecordedAwardFinalists(AwardHart)) == 4 }},
		{"ResultProblems is empty", func() bool { return len(st.ResultProblems()) == 0 }},
	}
	for _, c := range checks {
		if !c.ok() {
			t.Errorf("after a save, expected %s", c.name)
		}
	}
}

func TestSavingAPredictionShouldNotAddAResultsSectionToAFileWithoutOne(t *testing.T) {
	st, path := newSeededStore(t, resultsFixtureBase)

	if err := st.SavePrediction("basti", KindCupChampion, "FLA", time.Now()); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}

	for _, section := range []string{"results:", "award_finalists:"} {
		if strings.Contains(string(raw), section) {
			t.Errorf("expected no %q section to be written, got:\n%s", section, raw)
		}
	}
}

func TestSavingAPredictionShouldLeaveAToleratedShapeErrorByteForByteUnchanged(t *testing.T) {
	seed := resultsFixtureBase + "results:\n    team_marks:\n        atlantic:\n            playoffs: FLA\n            division_winner: FLA\n"
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := New(path)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := st.SavePrediction("basti", KindCupChampion, "FLA", time.Now()); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	wantSection := "results:\n    team_marks:\n        atlantic:\n            playoffs: FLA\n            division_winner: FLA\n"
	if !strings.Contains(string(raw), wantSection) {
		t.Errorf("expected the tolerated results: section to survive a save byte-for-byte, got:\n%s", raw)
	}
}

func TestSavingAPredictionShouldPreserveCommentsFlowStyleAndUnusualKeyOrderInHandMaintainedSections(t *testing.T) {
	seed := resultsFixtureBase + `results:
    team_marks:
        atlantic: {playoffs: [FLA, TOR], division_winner: FLA} # division call
    stanley_cup_winner: FLA
award_finalists:
    hart:
        - {display_name: Connor McDavid, slug: mcdavid-connor}
`
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := New(path)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := st.SavePrediction("basti", KindCupChampion, "FLA", time.Now()); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	got := string(raw)
	// The whole results:/award_finalists: block, byte-for-byte, except the
	// one property the Design Notes explicitly exempt: a block sequence's
	// indentation (award_finalists.hart's list item shifts from 8 to 6
	// spaces here) - empirically confirmed this is the *only* difference
	// from the original seed for this fixture. Everything else (comments,
	// flow style, key order, quoting) must match exactly, not just appear
	// somewhere in the file - a bug that duplicated, reordered, or inserted
	// content elsewhere in this section would fail this check even if the
	// individual fragments below still happened to appear.
	wantBlock := `results:
    team_marks:
        atlantic: {playoffs: [FLA, TOR], division_winner: FLA} # division call
    stanley_cup_winner: FLA
award_finalists:
    hart:
      - {display_name: Connor McDavid, slug: mcdavid-connor}
`
	if !strings.Contains(got, wantBlock) {
		t.Errorf("expected the results:/award_finalists: block to match exactly (aside from the documented indentation exception), got:\n%s", got)
	}
}

// --- pure splice/rollback mechanics: spliceNamedValueLocked and New's raw-
// node handling, independent of what results:/award_finalists: hold ---

func TestWriteLockedShouldRollBackAnAppendedRawNodeWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	seed := "season: \"2026-27\"\nplayers:\n    - id: basti\n      name: Basti\n      email: basti@example.com\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	// This seed has neither a login_codes: nor a predictions: key, so the
	// coming write has to append both - the code path a bootstrapped file
	// (which always has both already) never exercises.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	if err := st.CreateLoginCode("basti", "hash-1", time.Now()); err == nil {
		t.Fatal("expected CreateLoginCode to return an error when the write fails")
	}

	st.mu.RLock()
	_, loginCodesFound := mappingValue(topLevelMapping(st.raw), "login_codes")
	_, predictionsFound := mappingValue(topLevelMapping(st.raw), "predictions")
	st.mu.RUnlock()
	if loginCodesFound || predictionsFound {
		t.Error("expected both appended nodes to be rolled back from s.raw after a failed write")
	}
}

// TestWriteLockedShouldRollBackAReplacedRawNodeWhenTheWriteFails is the
// should-not counterpart of the append-branch rollback test above, for
// spliceNamedValueLocked's other branch: a bootstrapped store already has
// login_codes:/predictions: keys present, so every write after the first
// replaces their value node rather than appending one - the branch every
// real, long-running store actually exercises on every write. A failed
// write must restore the previously-committed node, not just remove it.
func TestWriteLockedShouldRollBackAReplacedRawNodeWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	// One successful write first, so login_codes:/predictions: are already
	// present with real content - the coming failed write has to replace,
	// not append.
	if err := st.CreateLoginCode("basti", "hash-1", time.Now()); err != nil {
		t.Fatalf("first CreateLoginCode returned error: %v", err)
	}

	st.mu.RLock()
	loginCodesNode, _ := mappingValue(topLevelMapping(st.raw), "login_codes")
	committedLen := len(loginCodesNode.Content)
	st.mu.RUnlock()
	if committedLen != 1 {
		t.Fatalf("expected 1 committed login code node before the failed write, got %d", committedLen)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	if err := st.CreateLoginCode("basti", "hash-2", time.Now()); err == nil {
		t.Fatal("expected the second CreateLoginCode to return an error when the write fails")
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	rolledBackNode, ok := mappingValue(topLevelMapping(st.raw), "login_codes")
	if !ok {
		t.Fatal("expected the login_codes node to still be present after a failed replace")
	}
	if len(rolledBackNode.Content) != committedLen {
		t.Errorf("expected the login_codes node to be rolled back to its previously-committed content (%d entries), got %d", committedLen, len(rolledBackNode.Content))
	}
}

func TestNewShouldRoundTripACleanFileEndToEnd(t *testing.T) {
	st, path := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	if err := st.SavePrediction("basti", KindCupChampion, "FLA", time.Now()); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	reopened, err := New(path)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	assertFixtureResultsRecorded(t, reopened)
	if prediction, ok := reopened.FindPrediction("basti", KindCupChampion); !ok || prediction.TeamID != "FLA" {
		t.Errorf("FindPrediction(basti, cup) = %+v, %v, want TeamID FLA, true", prediction, ok)
	}
}

// TestNewShouldAllowWritesAfterLoadingAnEmptyExistingFile is a regression
// test for a review finding (edge-case-hunter, spec-7-4): an existing file
// that's empty, whitespace-only, or comment-only parses to a *yaml.Node with
// no content at all (Kind 0) - New used to accept that silently, but the
// very next write then failed with "yaml: cannot encode node with unknown
// kind 0" since writeLocked has nothing to splice into. New now rebuilds raw
// from the (zero-valued) typed doc in that case, matching the bootstrap
// path, so a write right after loading such a file still succeeds.
func TestNewShouldAllowWritesAfterLoadingAnEmptyExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	if err := os.WriteFile(path, []byte("# just a comment, no content\n"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := st.CreateLoginCode("basti", "hash-1", time.Now().UTC()); err != nil {
		t.Fatalf("expected a write after loading an empty file to succeed, got error: %v", err)
	}
}

// TestNewShouldNotSilentlyDiscardWritesAfterLoadingABareNullFile is a
// regression test for a review finding (edge-case-hunter, spec-7-4,
// reproduced directly): a file whose only content is a bare YAML null
// scalar parses to a real, non-mapping top-level node (unlike an empty
// file's Kind-0 node, so the earlier empty-file fix alone didn't cover it).
// Before this fix, New and the next write both succeeded with no error, but
// the write was silently discarded - the file was left containing only
// "null", losing the very data the write was supposed to persist.
func TestNewShouldNotSilentlyDiscardWritesAfterLoadingABareNullFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	if err := os.WriteFile(path, []byte("null\n"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := st.CreateLoginCode("basti", "hash-1", time.Now().UTC()); err != nil {
		t.Fatalf("CreateLoginCode() returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	if !strings.Contains(string(raw), "hash-1") {
		t.Errorf("expected the write to actually persist rather than being silently discarded, got:\n%s", raw)
	}
}
