package main

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sommerfeld-io/fantasy-hockey/internal/server"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// testSessionSecret is set on every test that needs resolveConfig to
// succeed but doesn't itself exercise SESSION_SECRET's value.
const testSessionSecret = "test-session-secret"

// assertLogSilenceAndZeroProblems fails the test unless logs holds no
// level=WARN line and reports the AC4 count=0 summary line - the shape
// openStore's own Warn/Info calls produce for a file with zero malformed
// results. It only observes what openStore itself logs through the logger
// passed into it, not store.New's own bootstrap-write line (store.go's
// writeLocked logs that separately, via the package-level default slog
// logger, not the *slog.Logger a caller supplies). scenario names what the
// test seeded, for a legible failure message.
func assertLogSilenceAndZeroProblems(t *testing.T, logs *bytes.Buffer, scenario string) {
	t.Helper()
	if strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("expected no warnings for %s, got %q", scenario, logs.String())
	}
	if !strings.Contains(logs.String(), `msg="result problems found" count=0`) {
		t.Errorf("expected a summary line reporting count=0 (AC4) for %s, got %q", scenario, logs.String())
	}
}

func TestResolveConfigShouldApplyDefaultsWithNoArgsOrEnv(t *testing.T) {
	t.Setenv("DATA_FILE", "")
	t.Setenv("SESSION_SECRET", testSessionSecret)

	cfg, err := resolveConfig(nil)
	if err != nil {
		t.Fatalf("resolveConfig returned error: %v", err)
	}
	if cfg.port != server.DefaultPort {
		t.Errorf("expected default port %d, got %d", server.DefaultPort, cfg.port)
	}
	if cfg.dataFile != store.DataFileName {
		t.Errorf("expected default data file %q, got %q", store.DataFileName, cfg.dataFile)
	}
	if cfg.secret != testSessionSecret {
		t.Errorf("expected secret %q, got %q", testSessionSecret, cfg.secret)
	}
}

func TestResolveConfigShouldParsePortAndDataFileFlagsTogetherOnTheSharedFlagSet(t *testing.T) {
	t.Setenv("DATA_FILE", "")
	t.Setenv("SESSION_SECRET", testSessionSecret)

	tests := []struct {
		name     string
		args     []string
		wantPort int
	}{
		{"long form --port", []string{"--port=9090", "--data-file=/tmp/x.yml"}, 9090},
		{"shorthand -p", []string{"-p", "9091", "--data-file=/tmp/x.yml"}, 9091},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := resolveConfig(tt.args)
			if err != nil {
				t.Fatalf("resolveConfig returned error: %v", err)
			}
			if cfg.port != tt.wantPort {
				t.Errorf("expected port %d, got %d", tt.wantPort, cfg.port)
			}
			if cfg.dataFile != "/tmp/x.yml" {
				t.Errorf("expected data file %q, got %q", "/tmp/x.yml", cfg.dataFile)
			}
		})
	}
}

func TestResolveConfigShouldFallBackToDataFileEnvWhenNoFlagIsGiven(t *testing.T) {
	t.Setenv("DATA_FILE", "/tmp/env.yml")
	t.Setenv("SESSION_SECRET", testSessionSecret)

	cfg, err := resolveConfig(nil)
	if err != nil {
		t.Fatalf("resolveConfig returned error: %v", err)
	}
	if cfg.dataFile != "/tmp/env.yml" {
		t.Errorf("expected data file %q, got %q", "/tmp/env.yml", cfg.dataFile)
	}
}

func TestResolveConfigShouldReturnAnErrorForAnUnrecognizedFlag(t *testing.T) {
	t.Setenv("SESSION_SECRET", testSessionSecret)

	if _, err := resolveConfig([]string{"--not-a-real-flag"}); err == nil {
		t.Fatal("expected an error for an unrecognized flag, got nil")
	}
}

func TestResolveConfigShouldPreferDataFileFlagOverEnvWhenBothAreSet(t *testing.T) {
	t.Setenv("DATA_FILE", "/tmp/env.yml")
	t.Setenv("SESSION_SECRET", testSessionSecret)

	cfg, err := resolveConfig([]string{"--data-file=/tmp/flag.yml"})
	if err != nil {
		t.Fatalf("resolveConfig returned error: %v", err)
	}
	if cfg.dataFile != "/tmp/flag.yml" {
		t.Errorf("expected the --data-file flag to win over DATA_FILE, got %q", cfg.dataFile)
	}
}

func TestResolveConfigShouldReturnAnErrorWhenSessionSecretIsUnset(t *testing.T) {
	t.Setenv("SESSION_SECRET", "")

	if _, err := resolveConfig(nil); err == nil {
		t.Fatal("expected an error when SESSION_SECRET is unset, got nil")
	}
}

func TestResolveConfigShouldReturnAnErrorWhenSessionSecretIsWhitespaceOnly(t *testing.T) {
	t.Setenv("SESSION_SECRET", "   ")

	if _, err := resolveConfig(nil); err == nil {
		t.Fatal("expected an error when SESSION_SECRET is whitespace-only, got nil")
	}
}

func TestOpenStoreShouldWarnAboutAMalformedResultAndStillSucceed(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	seed := "season: \"2026-27\"\nresults:\n    presidents_trophy: YYY\n    stanley_cup_winner: XXX\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	st, err := openStore(path, logger)

	if err != nil || st == nil {
		t.Fatalf("openStore = %v, %v, want a store and no error", st, err)
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "XXX") {
		t.Errorf("expected a warning naming the bad entry, got %q", logs.String())
	}
	if got := strings.Count(logs.String(), "level=WARN"); got != 2 {
		t.Errorf("expected one warning per bad entry (2), got %d in %q", got, logs.String())
	}
	if !strings.Contains(logs.String(), `msg="result problems found" count=2`) {
		t.Errorf("expected a summary line reporting count=2 (AC4), got %q", logs.String())
	}
}

func TestOpenStoreShouldNotWarnForAWellFormedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	seed := "season: \"2026-27\"\nteams:\n    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}\nresults:\n    presidents_trophy: FLA\n    stanley_cup_winner: FLA\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	if _, err := openStore(path, logger); err != nil {
		t.Fatalf("openStore returned error: %v", err)
	}
	assertLogSilenceAndZeroProblems(t, &logs, "a well-formed file")
}

// TestOpenStoreShouldWarnAboutAToleratedShapeErrorFromResultProblems proves
// New's tolerated-shape-error category (AC2, spec-7-4) reaches openStore's
// WARN loop and AC4 count line, not just st.ResultProblems() directly
// (already covered in store_results_test.go) (epic-7 retrospective, item 52).
func TestOpenStoreShouldWarnAboutAToleratedShapeErrorFromResultProblems(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	// teams: FLA is required so division_winner: FLA validates - without it,
	// FLA is an unrecognized team abbreviation and teamMarkProblemsLocked
	// reports a second, unrelated problem, breaking this test's count=1
	// assertion below.
	seed := "season: \"2026-27\"\nteams:\n    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}\nresults:\n    team_marks:\n        atlantic:\n            playoffs: FLA\n            division_winner: FLA\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	st, err := openStore(path, logger)

	if err != nil || st == nil {
		t.Fatalf("openStore = %v, %v, want a store and no error", st, err)
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), `problem="results: `) || !strings.Contains(logs.String(), "line") {
		t.Errorf("expected a warning naming the tolerated shape error (results-section-prefixed, with a source line), got %q", logs.String())
	}
	if got := strings.Count(logs.String(), "level=WARN"); got != 1 {
		t.Errorf("expected exactly one warning for the one tolerated shape error, got %d in %q", got, logs.String())
	}
	if !strings.Contains(logs.String(), `msg="result problems found" count=1`) {
		t.Errorf("expected a summary line reporting count=1 (AC4), got %q", logs.String())
	}
}

// TestOpenStoreShouldWarnAboutAnUnknownKeyFromResultProblems proves New's
// unknown/misspelled-key category (AC1, spec-7-4) reaches openStore's WARN
// loop and AC4 count line, not just st.ResultProblems() directly (already
// covered in store_results_test.go) (epic-7 retrospective, item 52).
func TestOpenStoreShouldWarnAboutAnUnknownKeyFromResultProblems(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	seed := "season: \"2026-27\"\nresults:\n    stanley_cup_winer: FLA\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	st, err := openStore(path, logger)

	if err != nil || st == nil {
		t.Fatalf("openStore = %v, %v, want a store and no error", st, err)
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "results.stanley_cup_winer: unknown key") {
		t.Errorf("expected a warning naming the unknown key, got %q", logs.String())
	}
	if got := strings.Count(logs.String(), "level=WARN"); got != 1 {
		t.Errorf("expected exactly one warning for the one unknown key, got %d in %q", got, logs.String())
	}
	if !strings.Contains(logs.String(), `msg="result problems found" count=1`) {
		t.Errorf("expected a summary line reporting count=1 (AC4), got %q", logs.String())
	}
}

// TestOpenStoreShouldNotWarnAboutCorrectlyShapedTeamMarksOrKeyNames is the
// should-not counterpart to the two tests above: the correctly-shaped
// equivalent of each seed (a real list for team_marks.playoffs, the
// correctly-spelled stanley_cup_winner key) must not false-positive a
// warning through openStore's WARN loop.
func TestOpenStoreShouldNotWarnAboutCorrectlyShapedTeamMarksOrKeyNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	seed := "season: \"2026-27\"\nteams:\n    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}\nresults:\n    team_marks:\n        atlantic:\n            playoffs: [FLA]\n            division_winner: FLA\n    stanley_cup_winner: FLA\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	if _, err := openStore(path, logger); err != nil {
		t.Fatalf("openStore returned error: %v", err)
	}
	if strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("expected no warnings for correctly-shaped/spelled equivalents, got %q", logs.String())
	}
	if !strings.Contains(logs.String(), `msg="result problems found" count=0`) {
		t.Errorf("expected a summary line reporting count=0 (AC4), got %q", logs.String())
	}
}

func TestOpenStoreShouldReturnAnErrorForAnUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	if err := os.WriteFile(path, []byte("not: valid: yaml: at all"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := openStore(path, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))); err == nil {
		t.Fatal("expected an error for invalid YAML, got nil")
	}
}

// TestOpenStoreShouldReturnAnOperatorLegibleErrorForANonexistentParentDirectory
// proves store.New's own nonexistent-parent-directory error (see
// internal/store's own TestNewShouldReturnAnOperatorLegibleErrorForA...)
// survives openStore's error wrapping still naming the attempted path
// (epic-6 retrospective, item 46).
func TestOpenStoreShouldReturnAnOperatorLegibleErrorForANonexistentParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent-subdir", store.DataFileName)

	st, err := openStore(path, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	if err == nil {
		t.Fatal("expected an error for a nonexistent parent directory, got nil")
	}
	if st != nil {
		t.Errorf("expected a nil Store on error, got %v", st)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected the error to satisfy errors.Is(err, fs.ErrNotExist), got %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("expected the error to name the attempted path %q for an operator to diagnose, got %v", path, err)
	}
}

// TestOpenStoreShouldBootstrapAFreshSeasonWhenTheResolvedPathDoesNotExist
// proves openStore's season-rollover behavior for I/O matrix row 1: an
// already-resolved path that doesn't exist yet must bootstrap a clean
// season skeleton, not fail or inherit anything from elsewhere. It calls
// openStore directly with a constructed path - DATA_FILE/--data-file's own
// resolution into that path is TestResolveConfigShould*'s job, not this
// test's, and run() (which wires the two together) is exercised by neither.
func TestOpenStoreShouldBootstrapAFreshSeasonWhenTheResolvedPathDoesNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	st, err := openStore(path, logger)

	if err != nil || st == nil {
		t.Fatalf("openStore(%q) = %v, %v, want a store and no error", path, st, err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected %q to be created, got error: %v", path, statErr)
	}
	if got := st.Season(); got != store.DefaultSeason {
		t.Errorf("expected bootstrapped season %q, got %q", store.DefaultSeason, got)
	}
	if got := st.Players(); len(got) != 0 {
		t.Errorf("expected an empty players list, got %v", got)
	}
	assertLogSilenceAndZeroProblems(t, &logs, "a freshly bootstrapped file")
}

// TestOpenStoreShouldNeverTouchAnArchivedFileAtADifferentPath proves the
// second half of a season rollover through openStore directly (same scope
// note as TestOpenStoreShouldBootstrapAFreshSeasonWhenTheResolvedPathDoesNotExist
// above - resolveConfig/run() are not exercised here): given an archived
// prior-season file and a fresh path colocated in the same directory - a
// realistic rollover topology - openStore never reads from or writes to the
// archived file (I/O matrix row 2).
func TestOpenStoreShouldNeverTouchAnArchivedFileAtADifferentPath(t *testing.T) {
	dir := t.TempDir()
	archivedPath := filepath.Join(dir, "fantasy-hockey-2025-26.yml")
	archivedSeed := "season: \"2025-26\"\nplayers:\n    - id: basti\n      name: Basti\n      email: basti@example.com\nresults:\n    presidents_trophy: FLA\n    stanley_cup_winner: FLA\n"
	if err := os.WriteFile(archivedPath, []byte(archivedSeed), 0o600); err != nil {
		t.Fatalf("seed archived file: %v", err)
	}
	before, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatalf("read archived file before openStore: %v", err)
	}

	freshPath := filepath.Join(dir, store.DataFileName)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	st, err := openStore(freshPath, logger)
	if err != nil {
		t.Fatalf("openStore(%q) returned error: %v", freshPath, err)
	}
	if got := st.Season(); got != store.DefaultSeason {
		t.Errorf("expected bootstrapped season %q, got %q", store.DefaultSeason, got)
	}
	if got := st.Players(); len(got) != 0 {
		t.Errorf("expected an empty players list, got %v", got)
	}
	assertLogSilenceAndZeroProblems(t, &logs, "the freshly bootstrapped file alongside an archived one")

	after, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatalf("read archived file after openStore: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("expected archived file to be byte-for-byte unchanged, before=%q after=%q", before, after)
	}
}
