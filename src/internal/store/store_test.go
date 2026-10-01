package store

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

// seasonFormat is the YYYY-YY shape DefaultSeason is expected to follow -
// defined here, not in store.go, since nothing at runtime validates it. This
// is a format-only backstop for the Go constant alone, not a validation rule
// for a hand-edited season: field: spec-1-5's Design Notes already rejected
// adding format/fuzz validation for that value specifically, since it's a
// hand-maintained operator config value, not user input, and no other
// config value gets this kind of coverage - that decision stands unchanged.
var seasonFormat = regexp.MustCompile(`^\d{4}-\d{2}$`)

// captureLogs swaps slog's default logger for one writing to a buffer this
// test can inspect, restoring the original default when the test ends.
// Mirrors internal/auth/auth_test.go's own captureLogs - the two can't share
// one implementation across package boundaries, so this ~8-line helper is
// duplicated rather than imported.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

// assertExactlyOneInfoLine fails the test unless logs holds exactly one
// "store write" log line - the shape every successful mutating write must
// produce (spec-7-1's acceptance criteria), never zero and never more than
// one.
func assertExactlyOneInfoLine(t *testing.T, logs *bytes.Buffer) {
	t.Helper()
	if logs.Len() == 0 {
		t.Fatal("expected exactly one log line, got none")
	}
	lines := strings.Split(strings.TrimRight(logs.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one log line, got %d: %q", len(lines), logs.String())
	}
	if !strings.Contains(lines[0], `msg="store write"`) {
		t.Errorf("expected the log line's message to be %q, got %q", "store write", lines[0])
	}
}

// assertNoLogOutput fails the test unless logs is empty - the shape every
// no-op call or failed write must produce (spec-7-1's acceptance criteria).
func assertNoLogOutput(t *testing.T, logs *bytes.Buffer) {
	t.Helper()
	if logs.Len() != 0 {
		t.Errorf("expected no log output, got %q", logs.String())
	}
}

// TestDefaultSeasonShouldMatchTheExpectedFormat is a format-only regression
// test, deliberately not a value-equality check against DefaultSeason's own
// literal value (which would be tautological, proving nothing - epic-6
// retrospective, item 42). It's a partial backstop only: it can't detect a
// season that's merely stale (e.g. still "2026-27" a year later), since a
// stale value is just as well-formed as a current one - only a human
// bumping the constant closes that gap (docs/operator-guide.md's own
// warning row covers the operator side of it).
func TestDefaultSeasonShouldMatchTheExpectedFormat(t *testing.T) {
	if !seasonFormat.MatchString(DefaultSeason) {
		t.Errorf("expected DefaultSeason %q to match the YYYY-YY season format", DefaultSeason)
	}
}

// TestSeasonFormatShouldRejectMalformedValues is the should-not counterpart
// to TestDefaultSeasonShouldMatchTheExpectedFormat: proves seasonFormat
// itself still rejects an obviously wrong shape, so an accidental future
// loosening of the pattern (e.g. widening \d{2} to \d+) has regression
// coverage, not just a one-off manual check.
func TestSeasonFormatShouldRejectMalformedValues(t *testing.T) {
	for _, bad := range []string{"2026-2027", "26-27", "2026/27", "2026-27 ", ""} {
		if seasonFormat.MatchString(bad) {
			t.Errorf("expected seasonFormat to reject %q, but it matched", bad)
		}
	}
}

func TestNewShouldBootstrapCreateAMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)

	st, err := New(path)
	if err != nil {
		t.Fatalf("New(%q) returned error: %v", path, err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %q to be created, got error: %v", path, err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bootstrapped file: %v", err)
	}

	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal bootstrapped file: %v", err)
	}

	if doc.Season != DefaultSeason {
		t.Errorf("expected season %q, got %q", DefaultSeason, doc.Season)
	}
	if len(doc.Players) != 0 {
		t.Errorf("expected an empty players list, got %v", doc.Players)
	}
	if st == nil {
		t.Fatal("expected a non-nil Store")
	}
}

// TestNewShouldReturnAnOperatorLegibleErrorForANonexistentParentDirectory
// covers a real, plausible operator mistake - DATA_FILE/--data-file pointed
// at a path whose parent directory doesn't exist yet (a typo, a wrong
// volume mount) - proving New neither panics nor silently succeeds, and
// that the resulting error can actually be diagnosed (epic-6 retrospective,
// item 46). A single missing level is representative of the general case: a
// deeper missing chain (a/b/c/<file>) fails identically, since it's always
// writeLocked's os.CreateTemp(dir, ...) - which requires only its own
// immediate dir argument to exist, never creating it - that produces the
// ENOENT this test checks for, regardless of how many levels are missing
// above that.
func TestNewShouldReturnAnOperatorLegibleErrorForANonexistentParentDirectory(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "nonexistent-subdir")
	path := filepath.Join(subdir, DataFileName)

	st, err := New(path)

	if err == nil {
		t.Fatal("expected an error for a nonexistent parent directory, got nil")
	}
	if st != nil {
		t.Errorf("expected a nil Store on error, got %+v", st)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected the error to satisfy errors.Is(err, fs.ErrNotExist), got %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("expected the error to name the attempted path %q for an operator to diagnose, got %v", path, err)
	}
	if _, statErr := os.Stat(subdir); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("expected the failed attempt to leave no stray directory behind, got stat err %v", statErr)
	}
}

func TestNewShouldNotOverwriteAnExistingFile(t *testing.T) {
	seed := `season: "2025-26"
players:
    - id: basti
      name: Basti
      email: basti@example.com
`
	st, _ := newSeededStore(t, seed)

	player, ok := st.FindPlayerByEmail("basti@example.com")
	if !ok {
		t.Fatal("expected the seeded player to be loaded")
	}
	if player.ID != "basti" {
		t.Errorf("expected player id %q, got %q", "basti", player.ID)
	}
}

func TestNewShouldFailWhenTheFileIsNotValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	if err := os.WriteFile(path, []byte("not: valid: yaml: at all"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := New(path); err == nil {
		t.Fatal("expected an error for invalid YAML, got nil")
	}
}

// TestNewShouldLogTheBootstrapWrite covers a review finding (epic-7
// retrospective, item 56): the bootstrap log line originally logged only
// reason, leaving epic-6-retro-item-43's own path/season ask unclosed -
// this now asserts both fields are present too, alongside the existing
// distinct-reason check.
func TestNewShouldLogTheBootstrapWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	logs := captureLogs(t)

	if _, err := New(path); err != nil {
		t.Fatalf("New(%q) returned error: %v", path, err)
	}

	assertExactlyOneInfoLine(t, logs)
	line := logs.String()
	if !strings.Contains(line, `reason="bootstrap data file"`) {
		t.Errorf("expected the bootstrap write's log line to read distinctly from every other mutating call's line, got %q", line)
	}
	// Checks the raw path/season values appear, not "path="+path/
	// "season="+DefaultSeason concatenated - slog's TextHandler quotes a
	// value containing a space (e.g. a temp dir under a profile path with
	// one), which a bare concatenation check would miss (review finding,
	// verified empirically against the real TextHandler).
	if !strings.Contains(line, "path=") || !strings.Contains(line, path) {
		t.Errorf("expected the bootstrap write's log line to name the resolved path %q, got %q", path, line)
	}
	if !strings.Contains(line, "season=") || !strings.Contains(line, DefaultSeason) {
		t.Errorf("expected the bootstrap write's log line to name the bootstrapped season %q, got %q", DefaultSeason, line)
	}
}

func TestNewShouldNotLogWhenLoadingAnExistingFile(t *testing.T) {
	seed := `season: "2025-26"
players:
    - id: basti
      name: Basti
      email: basti@example.com
`
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	logs := captureLogs(t)

	if _, err := New(path); err != nil {
		t.Fatalf("New(%q) returned error: %v", path, err)
	}

	assertNoLogOutput(t, logs)
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := New(filepath.Join(dir, DataFileName))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	st.mu.Lock()
	st.doc.Players = append(st.doc.Players, Player{ID: "basti", Name: "Basti", Email: "basti@example.com"})
	st.mu.Unlock()

	return st
}

func TestFindPlayerByEmailShouldReturnThePlayerOnAMatch(t *testing.T) {
	st := newTestStore(t)

	player, ok := st.FindPlayerByEmail("basti@example.com")
	if !ok {
		t.Fatal("expected a match, got none")
	}
	if player.Name != "Basti" {
		t.Errorf("expected name %q, got %q", "Basti", player.Name)
	}
}

func TestFindPlayerByEmailShouldNotReturnAPlayerOnNoMatch(t *testing.T) {
	st := newTestStore(t)

	_, ok := st.FindPlayerByEmail("unknown@example.com")
	if ok {
		t.Fatal("expected no match for an unknown email, got one")
	}
}

func TestFindPlayerByEmailShouldMatchIgnoringCaseAndSurroundingWhitespace(t *testing.T) {
	st := newTestStore(t)

	player, ok := st.FindPlayerByEmail("  Basti@Example.COM  ")
	if !ok {
		t.Fatal("expected a case/whitespace-insensitive match, got none")
	}
	if player.ID != "basti" {
		t.Errorf("expected player id %q, got %q", "basti", player.ID)
	}
}

func TestFindPlayerByEmailShouldNotMatchAnEmptyEmailAgainstABlankPlayerRow(t *testing.T) {
	dir := t.TempDir()
	st, err := New(filepath.Join(dir, DataFileName))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	st.mu.Lock()
	st.doc.Players = append(st.doc.Players, Player{ID: "broken-row", Name: "Broken Row", Email: ""})
	st.mu.Unlock()

	_, ok := st.FindPlayerByEmail("")
	if ok {
		t.Fatal("expected an empty submitted email never to match, even against a blank Email row")
	}
	_, ok = st.FindPlayerByEmail("   ")
	if ok {
		t.Fatal("expected a whitespace-only submitted email never to match")
	}
}

func TestFindPlayerByIDShouldReturnThePlayerOnAMatch(t *testing.T) {
	st := newTestStore(t)

	player, ok := st.FindPlayerByID("basti")
	if !ok {
		t.Fatal("expected a match, got none")
	}
	if player.Name != "Basti" {
		t.Errorf("expected name %q, got %q", "Basti", player.Name)
	}
}

func TestFindPlayerByIDShouldNotReturnAPlayerOnNoMatch(t *testing.T) {
	st := newTestStore(t)

	_, ok := st.FindPlayerByID("unknown-id")
	if ok {
		t.Fatal("expected no match for an unknown player id, got one")
	}
}

func TestFindPlayerByIDShouldNotMatchOnCaseAlone(t *testing.T) {
	st := newTestStore(t)

	// Unlike FindPlayerByEmail, ids are opaque slugs (AD-17), not
	// case-insensitive addresses, so a differently-cased id must not match.
	_, ok := st.FindPlayerByID("BASTI")
	if ok {
		t.Fatal("expected an id differing only in case not to match")
	}
}

func TestSeasonShouldReturnTheSeededSeason(t *testing.T) {
	st := newTestStore(t)

	if got := st.Season(); got != DefaultSeason {
		t.Errorf("expected season %q, got %q", DefaultSeason, got)
	}
}

func TestPredictionSetsShouldReturnTheSeededList(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
prediction_sets:
    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: "2026-10-06T17:00:00Z"
      phase: before_season
      upcoming: false
    - id: cf
      title: Conference finals
      subtitle: Set once round 2 ends
      deadline_utc: "2027-05-14T16:00:00Z"
      phase: playoffs
      upcoming: true
`
	st, _ := newSeededStore(t, seed)

	sets := st.PredictionSets()
	if len(sets) != 2 {
		t.Fatalf("expected 2 prediction sets, got %d", len(sets))
	}
	if sets[0].ID != "cup" || sets[0].Phase != "before_season" || sets[0].Upcoming {
		t.Errorf("unexpected first prediction set: %+v", sets[0])
	}
	if sets[1].ID != "cf" || sets[1].Phase != "playoffs" || !sets[1].Upcoming {
		t.Errorf("unexpected second prediction set: %+v", sets[1])
	}
}

func TestPredictionSetsShouldReturnAnEmptyListWhenNoneAreSeeded(t *testing.T) {
	st := newTestStore(t)

	sets := st.PredictionSets()
	if len(sets) != 0 {
		t.Errorf("expected an empty list, got %v", sets)
	}
}

func TestPredictionSetsShouldReturnACopyThatCannotMutateTheStore(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
prediction_sets:
    - id: cup
      title: Cup champion
      subtitle: Your Stanley Cup winner
      deadline_utc: "2026-10-06T17:00:00Z"
      phase: before_season
      upcoming: false
`
	st, _ := newSeededStore(t, seed)

	sets := st.PredictionSets()
	sets[0].Title = "Tampered"

	again := st.PredictionSets()
	if again[0].Title != "Cup champion" {
		t.Errorf("expected the store's own copy to stay untouched, got title %q", again[0].Title)
	}
}

func TestTeamsShouldReturnTheSeededList(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
teams:
    - id: TOR
      name: Toronto Maple Leafs
      conference: Eastern
      division: Atlantic
    - id: VGK
      name: Vegas Golden Knights
      conference: Western
      division: Pacific
`
	st, _ := newSeededStore(t, seed)

	want := []Team{
		{ID: "TOR", Name: "Toronto Maple Leafs", Conference: "Eastern", Division: "Atlantic"},
		{ID: "VGK", Name: "Vegas Golden Knights", Conference: "Western", Division: "Pacific"},
	}
	teams := st.Teams()
	if len(teams) != len(want) {
		t.Fatalf("expected %d teams, got %d", len(want), len(teams))
	}
	for i, w := range want {
		if teams[i] != w {
			t.Errorf("unexpected team at index %d: got %+v, want %+v", i, teams[i], w)
		}
	}
}

func TestTeamsShouldReturnAnEmptyListWhenNoneAreSeeded(t *testing.T) {
	st := newTestStore(t)

	teams := st.Teams()
	if len(teams) != 0 {
		t.Errorf("expected an empty list, got %v", teams)
	}
}

func TestTeamsShouldReturnACopyThatCannotMutateTheStore(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
teams:
    - id: TOR
      name: Toronto Maple Leafs
      conference: Eastern
      division: Atlantic
`
	st, _ := newSeededStore(t, seed)

	teams := st.Teams()
	teams[0].Name = "Tampered"

	again := st.Teams()
	if again[0].Name != "Toronto Maple Leafs" {
		t.Errorf("expected the store's own copy to stay untouched, got name %q", again[0].Name)
	}
}

func TestNHLPlayersShouldReturnTheSeededList(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
nhl_players:
    - slug: mcdavid-connor
      display_name: Connor McDavid
      position: skater
    - slug: hellebuyck-connor
      display_name: Connor Hellebuyck
      position: goalie
`
	st, _ := newSeededStore(t, seed)

	want := []AwardFinalist{
		{Slug: "mcdavid-connor", DisplayName: "Connor McDavid", Position: PositionSkater},
		{Slug: "hellebuyck-connor", DisplayName: "Connor Hellebuyck", Position: PositionGoalie},
	}
	players := st.NHLPlayers()
	if len(players) != len(want) {
		t.Fatalf("expected %d NHL Players, got %d", len(want), len(players))
	}
	for i, w := range want {
		if players[i] != w {
			t.Errorf("unexpected NHL Player at index %d: got %+v, want %+v", i, players[i], w)
		}
	}
}

func TestNHLPlayersShouldReturnAnEmptyListWhenNoneAreSeeded(t *testing.T) {
	st := newTestStore(t)

	players := st.NHLPlayers()
	if len(players) != 0 {
		t.Errorf("expected an empty list, got %v", players)
	}
}

func TestNHLPlayersShouldReturnACopyThatCannotMutateTheStore(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
nhl_players:
    - slug: mcdavid-connor
      display_name: Connor McDavid
      position: skater
`
	st, _ := newSeededStore(t, seed)

	players := st.NHLPlayers()
	players[0].DisplayName = "Tampered"

	again := st.NHLPlayers()
	if again[0].DisplayName != "Connor McDavid" {
		t.Errorf("expected the store's own copy to stay untouched, got name %q", again[0].DisplayName)
	}
}

func TestNHLPlayersByPositionShouldReturnOnlyThatPositionsEntries(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
nhl_players:
    - slug: mcdavid-connor
      display_name: Connor McDavid
      position: skater
    - slug: hughes-quinn
      display_name: Quinn Hughes
      position: defenseman
    - slug: makar-cale
      display_name: Cale Makar
      position: defenseman
    - slug: hellebuyck-connor
      display_name: Connor Hellebuyck
      position: goalie
`
	st, _ := newSeededStore(t, seed)

	want := []AwardFinalist{
		{Slug: "hughes-quinn", DisplayName: "Quinn Hughes", Position: PositionDefenseman},
		{Slug: "makar-cale", DisplayName: "Cale Makar", Position: PositionDefenseman},
	}
	got := st.NHLPlayersByPosition(PositionDefenseman)
	if len(got) != len(want) {
		t.Fatalf("expected %d defensemen, got %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("unexpected NHL Player at index %d: got %+v, want %+v", i, got[i], w)
		}
	}
}

func TestNHLPlayersByPositionShouldReturnAnEmptyListWhenThatPositionHasNoMatches(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
nhl_players:
    - slug: mcdavid-connor
      display_name: Connor McDavid
      position: skater
`
	st, _ := newSeededStore(t, seed)

	got := st.NHLPlayersByPosition(PositionGoalie)
	if len(got) != 0 {
		t.Errorf("expected no goalies, got %v", got)
	}
}

func TestPlayoffMatchupsShouldReturnTheSeededListForAPresentKey(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
playoff_matchups:
    cf:
        - a: FLA
          b: TOR
        - a: EDM
          b: VGK
`
	st, _ := newSeededStore(t, seed)

	want := []PlayoffMatchup{
		{TeamA: "FLA", TeamB: "TOR"},
		{TeamA: "EDM", TeamB: "VGK"},
	}
	got := st.PlayoffMatchups("cf")
	if len(got) != len(want) {
		t.Fatalf("expected %d matchups, got %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("unexpected matchup at index %d: got %+v, want %+v", i, got[i], w)
		}
	}
}

func TestPlayoffMatchupsShouldReturnAnEmptyListForAnAbsentKey(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
playoff_matchups:
    cf:
        - a: FLA
          b: TOR
`
	st, _ := newSeededStore(t, seed)

	got := st.PlayoffMatchups("scf")
	if len(got) != 0 {
		t.Errorf("expected an empty list for an absent key, got %v", got)
	}
}

func TestPlayoffMatchupsShouldReturnAnEmptyListWhenNoSectionIsSeeded(t *testing.T) {
	st := newTestStore(t)

	got := st.PlayoffMatchups("cf")
	if len(got) != 0 {
		t.Errorf("expected an empty list when playoff_matchups is absent entirely, got %v", got)
	}
}

func TestPlayoffMatchupsShouldReturnAnEmptyListForAnEmptySeededList(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
playoff_matchups:
    cf: []
`
	st, _ := newSeededStore(t, seed)

	got := st.PlayoffMatchups("cf")
	if len(got) != 0 {
		t.Errorf("expected an empty list for an explicitly empty seeded list, got %v", got)
	}
}

func TestPlayoffMatchupsShouldReturnACopyThatCannotMutateTheStore(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
playoff_matchups:
    cf:
        - a: FLA
          b: TOR
`
	st, _ := newSeededStore(t, seed)

	matchups := st.PlayoffMatchups("cf")
	matchups[0].TeamA = "Tampered"

	again := st.PlayoffMatchups("cf")
	if again[0].TeamA != "FLA" {
		t.Errorf("expected the store's own copy to stay untouched, got team_a %q", again[0].TeamA)
	}
}

func TestCreateLoginCodeShouldAppendANewRow(t *testing.T) {
	st := newTestStore(t)

	if err := st.CreateLoginCode("basti", "hash-1", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("CreateLoginCode returned error: %v", err)
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.doc.LoginCodes) != 1 {
		t.Fatalf("expected 1 login code, got %d", len(st.doc.LoginCodes))
	}
	got := st.doc.LoginCodes[0]
	if got.PlayerID != "basti" || got.CodeHash != "hash-1" || got.IssuedAt != "2026-09-14T10:00:00Z" {
		t.Errorf("unexpected login code row: %+v", got)
	}
	if got.UsedAt != nil {
		t.Errorf("expected used_at to be nil, got %v", got.UsedAt)
	}
	if got.ID == "" {
		t.Error("expected a generated id, got empty string")
	}
}

func TestCreateLoginCodeShouldNotMutateAnEarlierRowOnARepeatRequest(t *testing.T) {
	st := newTestStore(t)

	if err := st.CreateLoginCode("basti", "hash-1", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("first CreateLoginCode returned error: %v", err)
	}
	if err := st.CreateLoginCode("basti", "hash-2", time.Date(2026, 9, 14, 10, 5, 0, 0, time.UTC)); err != nil {
		t.Fatalf("second CreateLoginCode returned error: %v", err)
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.doc.LoginCodes) != 2 {
		t.Fatalf("expected 2 login codes, got %d", len(st.doc.LoginCodes))
	}
	first := st.doc.LoginCodes[0]
	if first.CodeHash != "hash-1" || first.IssuedAt != "2026-09-14T10:00:00Z" {
		t.Errorf("expected the first row to stay untouched, got %+v", first)
	}
}

func TestCreateLoginCodeShouldPersistToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := st.CreateLoginCode("basti", "hash-1", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("CreateLoginCode returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal file: %v", err)
	}
	if len(doc.LoginCodes) != 1 || doc.LoginCodes[0].CodeHash != "hash-1" {
		t.Errorf("expected the persisted file to contain the new login code, got %+v", doc.LoginCodes)
	}
}

func TestCreateLoginCodeShouldLogOnASuccessfulWrite(t *testing.T) {
	st := newTestStore(t)
	logs := captureLogs(t)

	if err := st.CreateLoginCode("basti", "hash-1", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("CreateLoginCode returned error: %v", err)
	}

	assertExactlyOneInfoLine(t, logs)
	if !strings.Contains(logs.String(), `reason="create login code"`) {
		t.Errorf("expected reason=%q, got %q", "create login code", logs.String())
	}
	if strings.Contains(logs.String(), "hash-1") {
		t.Errorf("expected no raw code hash in the log line, got %q", logs.String())
	}
	// Should-not counterpart (review finding, epic-7-retro-item-56): only
	// New's bootstrap branch passes writeLocked's optional path/season
	// fields - every other call site, this one included, must stay exactly
	// as it was before that parameter existed.
	if strings.Contains(logs.String(), "path=") || strings.Contains(logs.String(), "season=") {
		t.Errorf("expected no path/season fields on a non-bootstrap write, got %q", logs.String())
	}
}

func TestCreateLoginCodeShouldRollBackTheAppendWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	// Remove the directory out from under the store so writeLocked's
	// create-temp-file step fails, simulating a disk write failure that
	// happens even when the test process runs as root (unlike a read-only
	// permission bit, which root bypasses).
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if err := st.CreateLoginCode("basti", "hash-1", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("expected CreateLoginCode to return an error when the write fails")
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.doc.LoginCodes) != 0 {
		t.Errorf("expected the failed append to be rolled back, got %d login code(s) still in memory", len(st.doc.LoginCodes))
	}
	assertNoLogOutput(t, logs)
}

// seedLoginCode appends a login code row directly into st's in-memory
// document (bypassing CreateLoginCode) so tests can construct rows with an
// arbitrary issuedAt/usedAt without waiting on the clock.
func seedLoginCode(t *testing.T, st *Store, row LoginCode) {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	st.doc.LoginCodes = append(st.doc.LoginCodes, row)
}

func TestConsumeLoginCodeShouldMatchAnUnusedUnexpiredCode(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)})

	playerID, ok, err := st.ConsumeLoginCode("hash-1", now)
	if err != nil {
		t.Fatalf("ConsumeLoginCode returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected a match, got none")
	}
	if playerID != "basti" {
		t.Errorf("expected player id %q, got %q", "basti", playerID)
	}

	// The consumed row is now used, so writeLocked's own cleanup prunes it
	// in this same write (spec-7-2) - its used_at getting set is proven
	// indirectly, by that specific row no longer being present.
	st.mu.RLock()
	defer st.mu.RUnlock()
	if i := slices.IndexFunc(st.doc.LoginCodes, func(row LoginCode) bool { return row.CodeHash == "hash-1" }); i >= 0 {
		t.Fatalf("expected the just-consumed row to be pruned by the same write, still found %+v", st.doc.LoginCodes[i])
	}
}

func TestConsumeLoginCodeShouldLogOnASuccessfulWrite(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)})
	logs := captureLogs(t)

	if _, ok, err := st.ConsumeLoginCode("hash-1", now); err != nil || !ok {
		t.Fatalf("ConsumeLoginCode returned ok=%v, err=%v, want ok=true, err=nil", ok, err)
	}

	assertExactlyOneInfoLine(t, logs)
	if !strings.Contains(logs.String(), `reason="consume login code"`) {
		t.Errorf("expected reason=%q, got %q", "consume login code", logs.String())
	}
	if strings.Contains(logs.String(), "hash-1") {
		t.Errorf("expected no raw code hash in the log line, got %q", logs.String())
	}
}

func TestConsumeLoginCodeShouldNotLogOnNoMatch(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)})
	logs := captureLogs(t)

	if _, ok, err := st.ConsumeLoginCode("wrong-hash", now); err != nil || ok {
		t.Fatalf("ConsumeLoginCode returned ok=%v, err=%v, want ok=false, err=nil", ok, err)
	}

	assertNoLogOutput(t, logs)
}

func TestConsumeLoginCodeShouldNotMatchAWrongHash(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)})

	_, ok, err := st.ConsumeLoginCode("wrong-hash", now)
	if err != nil {
		t.Fatalf("ConsumeLoginCode returned error: %v", err)
	}
	if ok {
		t.Fatal("expected no match for a wrong hash")
	}
}

func TestConsumeLoginCodeShouldNotMatchAnExpiredCode(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-11 * time.Minute).Format(time.RFC3339)})

	_, ok, err := st.ConsumeLoginCode("hash-1", now)
	if err != nil {
		t.Fatalf("ConsumeLoginCode returned error: %v", err)
	}
	if ok {
		t.Fatal("expected no match for an expired code")
	}
}

func TestConsumeLoginCodeShouldNotMatchAFutureDatedCode(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(5 * time.Minute).Format(time.RFC3339)})

	_, ok, err := st.ConsumeLoginCode("hash-1", now)
	if err != nil {
		t.Fatalf("ConsumeLoginCode returned error: %v", err)
	}
	if ok {
		t.Fatal("expected no match for a future-dated code")
	}
}

func TestConsumeLoginCodeShouldNotMatchAnAlreadyUsedCode(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	usedAt := now.Add(-1 * time.Minute).Format(time.RFC3339)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339), UsedAt: &usedAt})

	_, ok, err := st.ConsumeLoginCode("hash-1", now)
	if err != nil {
		t.Fatalf("ConsumeLoginCode returned error: %v", err)
	}
	if ok {
		t.Fatal("expected no match for an already-used code")
	}
}

func TestConsumeLoginCodeShouldNotTouchAnyOtherRow(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)})
	seedLoginCode(t, st, LoginCode{ID: "lc2", PlayerID: "other", CodeHash: "hash-2", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)})

	if _, ok, err := st.ConsumeLoginCode("hash-1", now); err != nil || !ok {
		t.Fatalf("ConsumeLoginCode(hash-1) = ok=%v, err=%v", ok, err)
	}

	// Cleanup prunes the just-consumed row in this same write, shifting the
	// second row down - find it by CodeHash rather than a fixed index
	// (spec-7-2).
	st.mu.RLock()
	defer st.mu.RUnlock()
	i := slices.IndexFunc(st.doc.LoginCodes, func(row LoginCode) bool { return row.CodeHash == "hash-2" })
	if i < 0 {
		t.Fatalf("expected the second row to still be present, got %+v", st.doc.LoginCodes)
	}
	if st.doc.LoginCodes[i].UsedAt != nil {
		t.Errorf("expected the second row to stay untouched, got used_at=%v", *st.doc.LoginCodes[i].UsedAt)
	}
}

func TestConsumeLoginCodeShouldRollBackTheMarkWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)})

	// Remove the directory out from under the store so writeLocked's
	// create-temp-file step fails, simulating a disk write failure that
	// happens even when the test process runs as root (unlike a read-only
	// permission bit, which root bypasses).
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if _, ok, err := st.ConsumeLoginCode("hash-1", now); err == nil || ok {
		t.Fatalf("expected ConsumeLoginCode to fail when the write fails, got ok=%v, err=%v", ok, err)
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.doc.LoginCodes[0].UsedAt != nil {
		t.Errorf("expected the failed mark to be rolled back, got used_at=%v", *st.doc.LoginCodes[0].UsedAt)
	}
	assertNoLogOutput(t, logs)
}

func TestCleanupLoginCodesShouldRemoveAnExpiredRow(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	rows := []LoginCode{
		{ID: "lc1", CodeHash: "hash-1", IssuedAt: now.Add(-11 * time.Minute).Format(time.RFC3339)},
	}

	got := cleanupLoginCodes(rows, now)

	if len(got) != 0 {
		t.Errorf("expected the expired row to be removed, got %+v", got)
	}
}

// TestCleanupLoginCodesShouldRemoveAFutureDatedRow is a regression test for a
// review finding (edge-case-hunter + blind-hunter, spec-7-2): ConsumeLoginCode
// always rejects a future-dated row (clock skew), so cleanupLoginCodes must
// prune it too, or it would accumulate in the file forever despite never
// being consumable.
func TestCleanupLoginCodesShouldRemoveAFutureDatedRow(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	rows := []LoginCode{
		{ID: "lc1", CodeHash: "hash-1", IssuedAt: now.Add(5 * time.Minute).Format(time.RFC3339)},
	}

	got := cleanupLoginCodes(rows, now)

	if len(got) != 0 {
		t.Errorf("expected the future-dated row to be removed, got %+v", got)
	}
}

func TestCleanupLoginCodesShouldRemoveAUsedRow(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	usedAt := now.Add(-1 * time.Minute).Format(time.RFC3339)
	rows := []LoginCode{
		{ID: "lc1", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339), UsedAt: &usedAt},
	}

	got := cleanupLoginCodes(rows, now)

	if len(got) != 0 {
		t.Errorf("expected the used row to be removed, got %+v", got)
	}
}

func TestCleanupLoginCodesShouldKeepAnUnexpiredUnusedRow(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	rows := []LoginCode{
		{ID: "lc1", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)},
	}

	got := cleanupLoginCodes(rows, now)

	if len(got) != 1 || got[0].ID != "lc1" {
		t.Errorf("expected the unexpired, unused row to be kept, got %+v", got)
	}
}

// TestCleanupLoginCodesShouldKeepARowExactlyAtTheValidityBoundary proves the
// comparison is strict (>), not >=: a row issued exactly loginCodeValidity
// ago is not yet expired, matching ConsumeLoginCode's own identical check.
func TestCleanupLoginCodesShouldKeepARowExactlyAtTheValidityBoundary(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	rows := []LoginCode{
		{ID: "lc1", CodeHash: "hash-1", IssuedAt: now.Add(-loginCodeValidity).Format(time.RFC3339)},
	}

	got := cleanupLoginCodes(rows, now)

	if len(got) != 1 || got[0].ID != "lc1" {
		t.Errorf("expected a row exactly at the validity boundary to be kept, got %+v", got)
	}
}

// TestCleanupLoginCodesShouldKeepARowWithUnparseableIssuedAt is the
// should-not counterpart of TestCleanupLoginCodesShouldRemoveAnExpiredRow -
// cleanupLoginCodes can't confirm a malformed issued_at is expired, so it
// must never destroy that row (spec-7-2's Boundaries & Constraints).
func TestCleanupLoginCodesShouldKeepARowWithUnparseableIssuedAt(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	rows := []LoginCode{
		{ID: "lc1", CodeHash: "hash-1", IssuedAt: "not-a-timestamp"},
	}

	got := cleanupLoginCodes(rows, now)

	if len(got) != 1 || got[0].ID != "lc1" {
		t.Errorf("expected a row with an unparseable issued_at to be kept rather than destroyed, got %+v", got)
	}
}

// TestCleanupLoginCodesShouldRemoveAUsedRowEvenWithAnUnparseableIssuedAt
// locks in the precedence between cleanupLoginCodes' two checks: a used row
// is dropped unconditionally, without ever needing to parse IssuedAt at all.
func TestCleanupLoginCodesShouldRemoveAUsedRowEvenWithAnUnparseableIssuedAt(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	usedAt := now.Add(-1 * time.Minute).Format(time.RFC3339)
	rows := []LoginCode{
		{ID: "lc1", CodeHash: "hash-1", IssuedAt: "not-a-timestamp", UsedAt: &usedAt},
	}

	got := cleanupLoginCodes(rows, now)

	if len(got) != 0 {
		t.Errorf("expected the used row to be removed regardless of its unparseable issued_at, got %+v", got)
	}
}

func TestCleanupLoginCodesShouldPreserveOrderOfKeptRows(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	usedAt := now.Add(-1 * time.Minute).Format(time.RFC3339)
	rows := []LoginCode{
		{ID: "lc1", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)},
		{ID: "lc2", CodeHash: "hash-2", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339), UsedAt: &usedAt},
		{ID: "lc3", CodeHash: "hash-3", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)},
	}

	got := cleanupLoginCodes(rows, now)

	if len(got) != 2 || got[0].ID != "lc1" || got[1].ID != "lc3" {
		t.Errorf("expected kept rows [lc1, lc3] in order, got %+v", got)
	}
}

// TestSavePredictionShouldPruneAStaleLoginCodeRowOnItsNextWrite proves
// cleanup runs from writeLocked itself, so an unrelated mutation (here,
// SavePrediction) also prunes a stale LoginCode row - spec-7-2's central
// intent, that cleanup piggybacks on any write for any reason.
func TestSavePredictionShouldPruneAStaleLoginCodeRowOnItsNextWrite(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-11 * time.Minute).Format(time.RFC3339)})

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	st.mu.RLock()
	if len(st.doc.LoginCodes) != 0 {
		t.Errorf("expected the stale login code row to be pruned by an unrelated write, got %+v", st.doc.LoginCodes)
	}
	path := st.path
	st.mu.RUnlock()

	// st.doc.LoginCodes above is the in-memory field, only ever assigned
	// after a successful rename (writeLocked's own doc comment) - it can't
	// by itself prove the persisted file was actually updated. Re-reading
	// through a fresh New() does. Only login_codes and predictions are safe
	// to verify this way (writeLocked splices both on every write); every
	// other section round-trips through st.raw's parsed nodes rather than
	// st.doc, so an in-memory-only seed (like newTestStore's Players append)
	// never reaches disk and a reread of it would always come back empty
	// regardless of what was actually written.
	reread, err := New(path)
	if err != nil {
		t.Fatalf("re-read New(%q) returned error: %v", path, err)
	}
	if len(reread.doc.LoginCodes) != 0 {
		t.Errorf("expected the pruned state to be persisted to disk, re-read got %+v", reread.doc.LoginCodes)
	}
	if len(reread.doc.Predictions) != 1 {
		t.Errorf("expected the SavePrediction write itself to be persisted to disk too, re-read got %+v", reread.doc.Predictions)
	}
}

// TestSavePredictionShouldPruneAUsedLoginCodeRowOnItsNextWrite is
// TestSavePredictionShouldPruneAStaleLoginCodeRowOnItsNextWrite's own
// used-row counterpart.
func TestSavePredictionShouldPruneAUsedLoginCodeRowOnItsNextWrite(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	usedAt := now.Add(-1 * time.Minute).Format(time.RFC3339)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339), UsedAt: &usedAt})

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.doc.LoginCodes) != 0 {
		t.Errorf("expected the used login code row to be pruned by an unrelated write, got %+v", st.doc.LoginCodes)
	}
}

// TestSavePredictionShouldNotPruneAnUnexpiredUnusedLoginCodeRowOnItsNextWrite
// is the should-not counterpart of both prune tests above: a row that's
// still genuinely redeemable must survive an unrelated write untouched.
func TestSavePredictionShouldNotPruneAnUnexpiredUnusedLoginCodeRowOnItsNextWrite(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-5 * time.Minute).Format(time.RFC3339)})

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.doc.LoginCodes) != 1 || st.doc.LoginCodes[0].ID != "lc1" {
		t.Errorf("expected the unexpired, unused login code row to survive an unrelated write untouched, got %+v", st.doc.LoginCodes)
	}
}

// TestConsumeLoginCodeShouldRejectAResubmissionOfACodeCleanedUpByAnEarlierWrite
// is a regression test, not a behavior change: ConsumeLoginCode's existing
// no-match linear scan already returns the generic ok=false, err=nil outcome
// for a hash it can't find, whether that's because the code was always
// wrong or because an earlier write's cleanup already removed its row
// (spec-7-2's Boundaries & Constraints).
func TestConsumeLoginCodeShouldRejectAResubmissionOfACodeCleanedUpByAnEarlierWrite(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-11 * time.Minute).Format(time.RFC3339)})

	// An unrelated write triggers cleanup, pruning the already-expired row
	// before it's ever submitted for consumption.
	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	playerID, ok, err := st.ConsumeLoginCode("hash-1", now)
	if err != nil {
		t.Fatalf("ConsumeLoginCode returned error: %v", err)
	}
	if ok {
		t.Fatal("expected a resubmission of a cleaned-up code to be rejected")
	}
	if playerID != "" {
		t.Errorf("expected an empty player id, got %q", playerID)
	}
}

func TestFindPredictionShouldNotReturnARowOnNoMatch(t *testing.T) {
	st := newTestStore(t)

	_, ok := st.FindPrediction("basti", KindCupChampion)
	if ok {
		t.Fatal("expected no match when no Prediction rows exist")
	}
}

// seedPrediction appends a Prediction row directly into st's in-memory
// document (bypassing SavePrediction) so tests can seed a row without
// exercising the write path under test.
func seedPrediction(t *testing.T, st *Store, row Prediction) {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	st.doc.Predictions = append(st.doc.Predictions, row)
}

func TestFindPredictionShouldReturnTheSeededRowOnAMatch(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindCupChampion, TeamID: "TOR", SubmittedAt: "2026-09-14T10:00:00Z"})

	got, ok := st.FindPrediction("basti", KindCupChampion)
	if !ok {
		t.Fatal("expected a match, got none")
	}
	if got.TeamID != "TOR" {
		t.Errorf("expected team id %q, got %q", "TOR", got.TeamID)
	}
}

func TestFindPredictionShouldNotMatchARowForADifferentKind(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindCupChampion, TeamID: "TOR", SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindPrediction("basti", KindPresidentsTrophy)
	if ok {
		t.Fatal("expected no match for a different kind, even for the same player")
	}
}

func TestFindPredictionShouldNotMatchARowForADifferentPlayer(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindCupChampion, TeamID: "TOR", SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindPrediction("someone-else", KindCupChampion)
	if ok {
		t.Fatal("expected no match for a different player, even for the same kind")
	}
}

func TestSavePredictionShouldAppendANewRowWhenNoneExists(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	got, ok := st.FindPrediction("basti", KindCupChampion)
	if !ok {
		t.Fatal("expected a Prediction row to have been saved")
	}
	if got.TeamID != "TOR" {
		t.Errorf("expected team id %q, got %q", "TOR", got.TeamID)
	}
	if got.PlayerID != "basti" || got.Kind != KindCupChampion {
		t.Errorf("unexpected prediction row: %+v", got)
	}
	if got.SubmittedAt != "2026-09-14T10:00:00Z" {
		t.Errorf("expected submitted_at %q, got %q", "2026-09-14T10:00:00Z", got.SubmittedAt)
	}
	if got.ID == "" {
		t.Error("expected a generated id, got empty string")
	}
}

func TestSavePredictionShouldUpdateAnExistingRowInPlaceOnResubmission(t *testing.T) {
	st := newTestStore(t)
	first := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", first); err != nil {
		t.Fatalf("first SavePrediction returned error: %v", err)
	}
	logs := captureLogs(t)
	if _, err := st.SavePrediction("basti", KindCupChampion, "VGK", second); err != nil {
		t.Fatalf("second SavePrediction returned error: %v", err)
	}

	assertExactlyOneInfoLine(t, logs)
	if !strings.Contains(logs.String(), `reason="save prediction"`) {
		t.Errorf("expected the update-in-place write to log reason=%q too, got %q", "save prediction", logs.String())
	}

	st.mu.RLock()
	rows := len(st.doc.Predictions)
	st.mu.RUnlock()
	if rows != 1 {
		t.Fatalf("expected the resubmission to update the existing row rather than append, got %d rows", rows)
	}

	got, ok := st.FindPrediction("basti", KindCupChampion)
	if !ok {
		t.Fatal("expected a match")
	}
	if got.TeamID != "VGK" {
		t.Errorf("expected the updated team id %q, got %q", "VGK", got.TeamID)
	}
	if got.SubmittedAt != second.Format(time.RFC3339) {
		t.Errorf("expected the updated submitted_at %q, got %q", second.Format(time.RFC3339), got.SubmittedAt)
	}
}

func TestSavePredictionShouldNotTouchARowForADifferentKindOrPlayer(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}
	if _, err := st.SavePrediction("basti", KindPresidentsTrophy, "VGK", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}
	if _, err := st.SavePrediction("other-player", KindCupChampion, "COL", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	cup, ok := st.FindPrediction("basti", KindCupChampion)
	if !ok || cup.TeamID != "TOR" {
		t.Errorf("expected the cup pick to stay %q, got %+v (ok=%v)", "TOR", cup, ok)
	}
	presidents, ok := st.FindPrediction("basti", KindPresidentsTrophy)
	if !ok || presidents.TeamID != "VGK" {
		t.Errorf("expected the presidents pick to be %q, got %+v (ok=%v)", "VGK", presidents, ok)
	}
	otherPlayerCup, ok := st.FindPrediction("other-player", KindCupChampion)
	if !ok || otherPlayerCup.TeamID != "COL" {
		t.Errorf("expected the other player's cup pick to be %q, got %+v (ok=%v)", "COL", otherPlayerCup, ok)
	}
}

func TestSavePredictionShouldPersistToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal file: %v", err)
	}
	if len(doc.Predictions) != 1 || doc.Predictions[0].TeamID != "TOR" {
		t.Errorf("expected the persisted file to contain the new prediction, got %+v", doc.Predictions)
	}
}

func TestSavePredictionShouldLogOnASuccessfulWrite(t *testing.T) {
	st := newTestStore(t)
	logs := captureLogs(t)

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}

	assertExactlyOneInfoLine(t, logs)
	if !strings.Contains(logs.String(), `reason="save prediction"`) {
		t.Errorf("expected reason=%q, got %q", "save prediction", logs.String())
	}
}

func TestSavePredictionShouldRollBackTheAppendWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", time.Now().UTC()); err == nil {
		t.Fatal("expected SavePrediction to return an error when the write fails")
	}

	if _, ok := st.FindPrediction("basti", KindCupChampion); ok {
		t.Error("expected the failed append to be rolled back, but a Prediction row was found")
	}
	assertNoLogOutput(t, logs)
}

// TestSavePredictionShouldNotCommitLoginCodeCleanupWhenTheWriteFails closes
// the "Write fails mid-cleanup" row of spec-7-2's I/O matrix: writeLocked
// only assigns s.doc.LoginCodes = cleaned after a successful rename, so a
// failed write must leave an already-stale row exactly as it was, not
// silently pruned - this is what makes CreateLoginCode's/ConsumeLoginCode's
// own narrower rollbacks stay correct (spec-7-2's Design Notes). This test
// forces the failure at the earliest possible step (os.CreateTemp, via a
// removed directory) rather than specifically at the rename call; no test
// anywhere in this file isolates a rename-specific failure from an earlier
// one, since the commit-only-after-rename guarantee is the same regardless
// of which step actually fails.
func TestSavePredictionShouldNotCommitLoginCodeCleanupWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	now := time.Now().UTC()
	seedLoginCode(t, st, LoginCode{ID: "lc1", PlayerID: "basti", CodeHash: "hash-1", IssuedAt: now.Add(-11 * time.Minute).Format(time.RFC3339)})

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err == nil {
		t.Fatal("expected SavePrediction to return an error when the write fails")
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.doc.LoginCodes) != 1 || st.doc.LoginCodes[0].ID != "lc1" {
		t.Errorf("expected the stale login code row to survive a failed write untouched, got %+v", st.doc.LoginCodes)
	}
}

func TestSavePredictionShouldRollBackTheUpdateWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if _, err := st.SavePrediction("basti", KindCupChampion, "TOR", time.Now().UTC()); err != nil {
		t.Fatalf("seed SavePrediction returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if _, err := st.SavePrediction("basti", KindCupChampion, "VGK", time.Now().UTC()); err == nil {
		t.Fatal("expected SavePrediction to return an error when the write fails")
	}

	got, ok := st.FindPrediction("basti", KindCupChampion)
	if !ok {
		t.Fatal("expected the original row to still be found")
	}
	if got.TeamID != "TOR" {
		t.Errorf("expected the failed update to be rolled back to %q, got %q", "TOR", got.TeamID)
	}
	assertNoLogOutput(t, logs)
}

func TestFindDivisionPlayoffTeamsShouldNotReturnARowOnNoMatch(t *testing.T) {
	st := newTestStore(t)

	_, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if ok {
		t.Fatal("expected no match when no division rows exist")
	}
}

func TestFindDivisionPlayoffTeamsShouldReturnTheSeededRowOnAMatch(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindDivisionPlayoffTeams, Division: "Atlantic", TeamIDs: []string{"TOR", "BOS"}, SubmittedAt: "2026-09-14T10:00:00Z"})

	got, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if !ok {
		t.Fatal("expected a match, got none")
	}
	if len(got.TeamIDs) != 2 || got.TeamIDs[0] != "TOR" || got.TeamIDs[1] != "BOS" {
		t.Errorf("expected team ids %v, got %v", []string{"TOR", "BOS"}, got.TeamIDs)
	}
}

func TestFindDivisionPlayoffTeamsShouldNotMatchARowForADifferentDivision(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindDivisionPlayoffTeams, Division: "Atlantic", TeamIDs: []string{"TOR"}, SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindDivisionPlayoffTeams("basti", "Metropolitan")
	if ok {
		t.Fatal("expected no match for a different division, even for the same player")
	}
}

func TestFindDivisionPlayoffTeamsShouldNotMatchARowForADifferentPlayer(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindDivisionPlayoffTeams, Division: "Atlantic", TeamIDs: []string{"TOR"}, SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindDivisionPlayoffTeams("someone-else", "Atlantic")
	if ok {
		t.Fatal("expected no match for a different player, even for the same division")
	}
}

func TestFindDivisionPlayoffTeamsShouldNotMatchADivisionWinnerRow(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindDivisionWinner, Division: "Atlantic", TeamID: "TOR", SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if ok {
		t.Fatal("expected no match for a differently-kinded row sharing the same player/division")
	}
}

func TestFindDivisionWinnerShouldNotReturnARowOnNoMatch(t *testing.T) {
	st := newTestStore(t)

	_, ok := st.FindDivisionWinner("basti", "Atlantic")
	if ok {
		t.Fatal("expected no match when no division rows exist")
	}
}

func TestFindDivisionWinnerShouldReturnTheSeededRowOnAMatch(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindDivisionWinner, Division: "Atlantic", TeamID: "TOR", SubmittedAt: "2026-09-14T10:00:00Z"})

	got, ok := st.FindDivisionWinner("basti", "Atlantic")
	if !ok {
		t.Fatal("expected a match, got none")
	}
	if got.TeamID != "TOR" {
		t.Errorf("expected team id %q, got %q", "TOR", got.TeamID)
	}
}

func TestFindDivisionWinnerShouldNotMatchARowForADifferentDivisionOrPlayer(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindDivisionWinner, Division: "Atlantic", TeamID: "TOR", SubmittedAt: "2026-09-14T10:00:00Z"})

	if _, ok := st.FindDivisionWinner("basti", "Metropolitan"); ok {
		t.Error("expected no match for a different division")
	}
	if _, ok := st.FindDivisionWinner("someone-else", "Atlantic"); ok {
		t.Error("expected no match for a different player")
	}
}

func TestSaveDivisionPicksShouldAppendNewRowsForEveryPresentDivision(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	playoffTeams := map[string][]string{
		"Atlantic":     {"TOR", "BOS"},
		"Metropolitan": {"WSH"},
	}
	winners := map[string]string{
		"Atlantic": "TOR",
		// Metropolitan winner intentionally omitted - no row should exist.
	}

	if _, err := st.SaveDivisionPicks("basti", playoffTeams, winners, now); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}

	atlantic, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if !ok || len(atlantic.TeamIDs) != 2 {
		t.Errorf("expected an Atlantic playoff-teams row with 2 team ids, got %+v (ok=%v)", atlantic, ok)
	}
	metro, ok := st.FindDivisionPlayoffTeams("basti", "Metropolitan")
	if !ok || len(metro.TeamIDs) != 1 {
		t.Errorf("expected a Metropolitan playoff-teams row with 1 team id, got %+v (ok=%v)", metro, ok)
	}
	winner, ok := st.FindDivisionWinner("basti", "Atlantic")
	if !ok || winner.TeamID != "TOR" {
		t.Errorf("expected the Atlantic winner row %q, got %+v (ok=%v)", "TOR", winner, ok)
	}
	if _, ok := st.FindDivisionWinner("basti", "Metropolitan"); ok {
		t.Error("expected no Metropolitan winner row for an empty winner pick")
	}
}

func TestSaveDivisionPicksShouldNotUpsertADivisionAbsentFromTheMap(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, nil, now); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}

	if _, ok := st.FindDivisionPlayoffTeams("basti", "Metropolitan"); ok {
		t.Error("expected no row for a division genuinely absent from the map")
	}
}

func TestSaveDivisionPicksShouldUpsertAPresentButEmptyDivisionWithAnEmptyPick(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {}}, nil, now); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}

	got, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if !ok {
		t.Fatal("expected a row to be upserted for a present-but-empty division")
	}
	if len(got.TeamIDs) != 0 {
		t.Errorf("expected an empty pick, got %v", got.TeamIDs)
	}
}

func TestSaveDivisionPicksShouldUpdateExistingRowsInPlaceOnResubmission(t *testing.T) {
	st := newTestStore(t)
	first := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)

	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, map[string]string{"Atlantic": "TOR"}, first); err != nil {
		t.Fatalf("first SaveDivisionPicks returned error: %v", err)
	}
	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"BOS", "TBL"}}, map[string]string{"Atlantic": "BOS"}, second); err != nil {
		t.Fatalf("second SaveDivisionPicks returned error: %v", err)
	}

	st.mu.RLock()
	rows := len(st.doc.Predictions)
	st.mu.RUnlock()
	if rows != 2 {
		t.Fatalf("expected the resubmission to update the existing 2 rows rather than append, got %d rows", rows)
	}

	teams, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if !ok || len(teams.TeamIDs) != 2 || teams.TeamIDs[0] != "BOS" || teams.TeamIDs[1] != "TBL" {
		t.Errorf("expected the updated team ids %v, got %+v (ok=%v)", []string{"BOS", "TBL"}, teams, ok)
	}
	winner, ok := st.FindDivisionWinner("basti", "Atlantic")
	if !ok || winner.TeamID != "BOS" {
		t.Errorf("expected the updated winner %q, got %+v (ok=%v)", "BOS", winner, ok)
	}
}

// assertSavedDivisionPlayoffTeams checks st's saved (playerID, division)
// playoff-teams row matches want exactly - factored out of its callers
// purely to keep their own cyclomatic complexity low (gocyclo).
func assertSavedDivisionPlayoffTeams(t *testing.T, st *Store, playerID, division string, want []string) {
	t.Helper()
	got, ok := st.FindDivisionPlayoffTeams(playerID, division)
	if !ok {
		t.Errorf("expected a playoff-teams row for %q/%q, found none", playerID, division)
		return
	}
	if !slices.Equal(got.TeamIDs, want) {
		t.Errorf("expected %v for %q/%q, got %v", want, playerID, division, got.TeamIDs)
	}
}

func TestSaveDivisionPicksShouldNotTouchARowForADifferentDivisionOrPlayer(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, nil, now); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}
	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Metropolitan": {"WSH"}}, nil, now); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}
	if _, err := st.SaveDivisionPicks("other-player", map[string][]string{"Atlantic": {"COL"}}, nil, now); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}

	assertSavedDivisionPlayoffTeams(t, st, "basti", "Atlantic", []string{"TOR"})
	assertSavedDivisionPlayoffTeams(t, st, "basti", "Metropolitan", []string{"WSH"})
	assertSavedDivisionPlayoffTeams(t, st, "other-player", "Atlantic", []string{"COL"})
}

func TestSaveDivisionPicksShouldPersistToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, map[string]string{"Atlantic": "TOR"}, time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal file: %v", err)
	}
	if len(doc.Predictions) != 2 {
		t.Fatalf("expected 2 persisted rows, got %+v", doc.Predictions)
	}
}

func TestSaveDivisionPicksShouldLogOnASuccessfulWrite(t *testing.T) {
	st := newTestStore(t)
	logs := captureLogs(t)

	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, map[string]string{"Atlantic": "TOR"}, time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}

	assertExactlyOneInfoLine(t, logs)
	if !strings.Contains(logs.String(), `reason="save division picks"`) {
		t.Errorf("expected reason=%q, got %q", "save division picks", logs.String())
	}
}

func TestSaveDivisionPicksShouldRollBackTheAppendsWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, map[string]string{"Atlantic": "TOR"}, time.Now().UTC()); err == nil {
		t.Fatal("expected SaveDivisionPicks to return an error when the write fails")
	}

	if _, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic"); ok {
		t.Error("expected the failed append to be rolled back, but a playoff-teams row was found")
	}
	if _, ok := st.FindDivisionWinner("basti", "Atlantic"); ok {
		t.Error("expected the failed append to be rolled back, but a winner row was found")
	}
	assertNoLogOutput(t, logs)
}

func TestSaveDivisionPicksShouldRollBackTheUpdatesWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, map[string]string{"Atlantic": "TOR"}, time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveDivisionPicks returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if _, err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"BOS"}}, map[string]string{"Atlantic": "BOS"}, time.Now().UTC()); err == nil {
		t.Fatal("expected SaveDivisionPicks to return an error when the write fails")
	}

	teams, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic")
	if !ok || len(teams.TeamIDs) != 1 || teams.TeamIDs[0] != "TOR" {
		t.Errorf("expected the failed update to be rolled back to %v, got %+v (ok=%v)", []string{"TOR"}, teams, ok)
	}
	winner, ok := st.FindDivisionWinner("basti", "Atlantic")
	if !ok || winner.TeamID != "TOR" {
		t.Errorf("expected the failed update to be rolled back to %q, got %+v (ok=%v)", "TOR", winner, ok)
	}
	assertNoLogOutput(t, logs)
}

func TestFindAwardFinalistsShouldNotReturnARowOnNoMatch(t *testing.T) {
	st := newTestStore(t)

	_, ok := st.FindAwardFinalists("basti", AwardHart)
	if ok {
		t.Fatal("expected no match when no award rows exist")
	}
}

func TestFindAwardFinalistsShouldReturnTheSeededRowOnAMatch(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindAward, Award: AwardHart, FinalistSlugs: []string{"mcdavid-connor", "mackinnon-nathan", "kucherov-nikita"}, SubmittedAt: "2026-09-14T10:00:00Z"})

	got, ok := st.FindAwardFinalists("basti", AwardHart)
	if !ok {
		t.Fatal("expected a match, got none")
	}
	want := []string{"mcdavid-connor", "mackinnon-nathan", "kucherov-nikita"}
	if !slices.Equal(got.FinalistSlugs, want) {
		t.Errorf("expected finalist slugs %v, got %v", want, got.FinalistSlugs)
	}
}

func TestFindAwardFinalistsShouldNotMatchARowForADifferentAward(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindAward, Award: AwardHart, FinalistSlugs: []string{"a", "b", "c"}, SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindAwardFinalists("basti", AwardNorris)
	if ok {
		t.Fatal("expected no match for a different award, even for the same player")
	}
}

func TestFindAwardFinalistsShouldNotMatchARowForADifferentPlayer(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindAward, Award: AwardHart, FinalistSlugs: []string{"a", "b", "c"}, SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindAwardFinalists("someone-else", AwardHart)
	if ok {
		t.Fatal("expected no match for a different player, even for the same award")
	}
}

func TestFindAwardFinalistsShouldNotMatchADifferentlyKindedRow(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindDivisionWinner, Award: AwardHart, TeamID: "TOR", SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindAwardFinalists("basti", AwardHart)
	if ok {
		t.Fatal("expected no match for a differently-kinded row, even sharing the same scoping key")
	}
}

func TestSaveAwardPicksShouldAppendNewRowsForEveryFullyFilledAward(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	finalists := map[string][]string{
		AwardHart:   {"mcdavid-connor", "mackinnon-nathan", "kucherov-nikita"},
		AwardNorris: {"makar-cale", "hughes-quinn", "werenski-zach"},
	}

	if _, err := st.SaveAwardPicks("basti", finalists, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}

	hart, ok := st.FindAwardFinalists("basti", AwardHart)
	if !ok || len(hart.FinalistSlugs) != 3 {
		t.Errorf("expected a Hart row with 3 finalist slugs, got %+v (ok=%v)", hart, ok)
	}
	norris, ok := st.FindAwardFinalists("basti", AwardNorris)
	if !ok || len(norris.FinalistSlugs) != 3 {
		t.Errorf("expected a Norris row with 3 finalist slugs, got %+v (ok=%v)", norris, ok)
	}
}

func TestSaveAwardPicksShouldNotUpsertAnAwardWithFewerThanThreeSlugs(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	finalists := map[string][]string{
		AwardHart: {"mcdavid-connor", "mackinnon-nathan"}, // only 2 of 3.
	}

	if _, err := st.SaveAwardPicks("basti", finalists, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}

	if _, ok := st.FindAwardFinalists("basti", AwardHart); ok {
		t.Error("expected no row for an award with fewer than 3 finalist slugs")
	}
}

func TestSaveAwardPicksShouldNotUpsertAnAwardWithAnEmptySlugAmongThree(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	finalists := map[string][]string{
		AwardHart: {"mcdavid-connor", "", "kucherov-nikita"},
	}

	if _, err := st.SaveAwardPicks("basti", finalists, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}

	if _, ok := st.FindAwardFinalists("basti", AwardHart); ok {
		t.Error("expected no row for an award with an empty slug among its three")
	}
}

func TestSaveAwardPicksShouldNotUpsertAnAwardAbsentFromTheMap(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}

	if _, ok := st.FindAwardFinalists("basti", AwardNorris); ok {
		t.Error("expected no row for an award genuinely absent from the map")
	}
}

func TestSaveAwardPicksShouldUpdateAnExistingRowInPlaceOnResubmission(t *testing.T) {
	st := newTestStore(t)
	first := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)

	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, first); err != nil {
		t.Fatalf("first SaveAwardPicks returned error: %v", err)
	}
	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"x", "y", "z"}}, second); err != nil {
		t.Fatalf("second SaveAwardPicks returned error: %v", err)
	}

	st.mu.RLock()
	rows := len(st.doc.Predictions)
	st.mu.RUnlock()
	if rows != 1 {
		t.Fatalf("expected the resubmission to update the existing row rather than append, got %d rows", rows)
	}

	got, ok := st.FindAwardFinalists("basti", AwardHart)
	if !ok || !slices.Equal(got.FinalistSlugs, []string{"x", "y", "z"}) {
		t.Errorf("expected the updated finalist slugs %v, got %+v (ok=%v)", []string{"x", "y", "z"}, got, ok)
	}
}

func TestSaveAwardPicksShouldNotTouchARowForADifferentAwardOrPlayer(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}
	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardNorris: {"d", "e", "f"}}, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}
	if _, err := st.SaveAwardPicks("other-player", map[string][]string{AwardHart: {"g", "h", "i"}}, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}

	bastiHart, ok := st.FindAwardFinalists("basti", AwardHart)
	if !ok || !slices.Equal(bastiHart.FinalistSlugs, []string{"a", "b", "c"}) {
		t.Errorf("expected basti's Hart slugs %v, got %+v (ok=%v)", []string{"a", "b", "c"}, bastiHart, ok)
	}
	bastiNorris, ok := st.FindAwardFinalists("basti", AwardNorris)
	if !ok || !slices.Equal(bastiNorris.FinalistSlugs, []string{"d", "e", "f"}) {
		t.Errorf("expected basti's Norris slugs %v, got %+v (ok=%v)", []string{"d", "e", "f"}, bastiNorris, ok)
	}
	otherHart, ok := st.FindAwardFinalists("other-player", AwardHart)
	if !ok || !slices.Equal(otherHart.FinalistSlugs, []string{"g", "h", "i"}) {
		t.Errorf("expected other-player's Hart slugs %v, got %+v (ok=%v)", []string{"g", "h", "i"}, otherHart, ok)
	}
}

func TestSaveAwardPicksShouldPersistToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal file: %v", err)
	}
	if len(doc.Predictions) != 1 {
		t.Fatalf("expected 1 persisted row, got %+v", doc.Predictions)
	}
}

func TestSaveAwardPicksShouldLogOnASuccessfulWrite(t *testing.T) {
	st := newTestStore(t)
	logs := captureLogs(t)

	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}

	assertExactlyOneInfoLine(t, logs)
	if !strings.Contains(logs.String(), `reason="save award picks"`) {
		t.Errorf("expected reason=%q, got %q", "save award picks", logs.String())
	}
}

func TestSaveAwardPicksShouldRollBackTheAppendsWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, time.Now().UTC()); err == nil {
		t.Fatal("expected SaveAwardPicks to return an error when the write fails")
	}

	if _, ok := st.FindAwardFinalists("basti", AwardHart); ok {
		t.Error("expected the failed append to be rolled back, but an award row was found")
	}
	assertNoLogOutput(t, logs)
}

func TestSaveAwardPicksShouldRollBackTheUpdateWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveAwardPicks returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if _, err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"x", "y", "z"}}, time.Now().UTC()); err == nil {
		t.Fatal("expected SaveAwardPicks to return an error when the write fails")
	}

	got, ok := st.FindAwardFinalists("basti", AwardHart)
	if !ok || !slices.Equal(got.FinalistSlugs, []string{"a", "b", "c"}) {
		t.Errorf("expected the failed update to be rolled back to %v, got %+v (ok=%v)", []string{"a", "b", "c"}, got, ok)
	}
	assertNoLogOutput(t, logs)
}

// TestPlayoffMatchupShouldRoundTripItsKeyThroughYAML proves PlayoffMatchup's
// hand-maintained Key field parses off fantasy-hockey.yml's key: entry
// alongside TeamA/TeamB, extending TestPlayoffMatchupsShouldReturnThe
// SeededListForAPresentKey's own seeded-list coverage to the new field.
func TestPlayoffMatchupShouldRoundTripItsKeyThroughYAML(t *testing.T) {
	seed := `season: "2026-27"
players: []
login_codes: []
playoff_matchups:
    r1:
        - key: s1
          a: FLA
          b: TOR
`
	st, _ := newSeededStore(t, seed)

	got := st.PlayoffMatchups("r1")
	if len(got) != 1 {
		t.Fatalf("expected 1 matchup, got %d: %+v", len(got), got)
	}
	if got[0].Key != "s1" {
		t.Errorf("expected key %q, got %q", "s1", got[0].Key)
	}
	if got[0].TeamA != "FLA" || got[0].TeamB != "TOR" {
		t.Errorf("expected teams FLA/TOR, got %+v", got[0])
	}
}

func TestFindSeriesPickShouldNotReturnARowOnNoMatch(t *testing.T) {
	st := newTestStore(t)

	_, ok := st.FindSeriesPick("basti", "r1.s1")
	if ok {
		t.Fatal("expected no match when no series rows exist")
	}
}

func TestFindSeriesPickShouldReturnTheSeededRowOnAMatch(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindSeries, SeriesKey: "r1.s1", TeamID: "TOR", Games: "6", SubmittedAt: "2026-09-14T10:00:00Z"})

	got, ok := st.FindSeriesPick("basti", "r1.s1")
	if !ok {
		t.Fatal("expected a match, got none")
	}
	if got.TeamID != "TOR" || got.Games != "6" {
		t.Errorf("expected team id %q and games %q, got %q/%q", "TOR", "6", got.TeamID, got.Games)
	}
}

func TestFindSeriesPickShouldNotMatchARowForADifferentSeriesKey(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindSeries, SeriesKey: "r1.s1", TeamID: "TOR", Games: "6", SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindSeriesPick("basti", "r1.s2")
	if ok {
		t.Fatal("expected no match for a different series key, even for the same player")
	}
}

func TestFindSeriesPickShouldNotMatchARowForADifferentPlayer(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindSeries, SeriesKey: "r1.s1", TeamID: "TOR", Games: "6", SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindSeriesPick("someone-else", "r1.s1")
	if ok {
		t.Fatal("expected no match for a different player, even for the same series key")
	}
}

func TestFindSeriesPickShouldNotMatchADifferentlyKindedRow(t *testing.T) {
	st := newTestStore(t)
	seedPrediction(t, st, Prediction{ID: "p1", PlayerID: "basti", Kind: KindCupChampion, TeamID: "TOR", SubmittedAt: "2026-09-14T10:00:00Z"})

	_, ok := st.FindSeriesPick("basti", "r1.s1")
	if ok {
		t.Fatal("expected no match for a differently-kinded row, even sharing the same player")
	}
}

func TestSaveSeriesPickShouldAppendANewRowWhenNoneExists(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if _, err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", now); err != nil {
		t.Fatalf("SaveSeriesPick returned error: %v", err)
	}

	got, ok := st.FindSeriesPick("basti", "r1.s1")
	if !ok {
		t.Fatal("expected a Prediction row to have been saved")
	}
	if got.TeamID != "TOR" || got.Games != "6" {
		t.Errorf("expected team id %q and games %q, got %q/%q", "TOR", "6", got.TeamID, got.Games)
	}
	if got.PlayerID != "basti" || got.Kind != KindSeries {
		t.Errorf("unexpected prediction row: %+v", got)
	}
	if got.SubmittedAt != "2026-09-14T10:00:00Z" {
		t.Errorf("expected submitted_at %q, got %q", "2026-09-14T10:00:00Z", got.SubmittedAt)
	}
	if got.ID == "" {
		t.Error("expected a generated id, got empty string")
	}
}

func TestSaveSeriesPickShouldUpdateAnExistingRowInPlaceOnResubmission(t *testing.T) {
	st := newTestStore(t)
	first := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)

	if _, err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", first); err != nil {
		t.Fatalf("first SaveSeriesPick returned error: %v", err)
	}
	logs := captureLogs(t)
	if _, err := st.SaveSeriesPick("basti", "r1.s1", "FLA", "7", second); err != nil {
		t.Fatalf("second SaveSeriesPick returned error: %v", err)
	}

	assertExactlyOneInfoLine(t, logs)
	if !strings.Contains(logs.String(), `reason="save series pick"`) {
		t.Errorf("expected the update-in-place write to log reason=%q too, got %q", "save series pick", logs.String())
	}

	st.mu.RLock()
	rows := len(st.doc.Predictions)
	st.mu.RUnlock()
	if rows != 1 {
		t.Fatalf("expected the resubmission to update the existing row rather than append, got %d rows", rows)
	}

	got, ok := st.FindSeriesPick("basti", "r1.s1")
	if !ok {
		t.Fatal("expected a match")
	}
	if got.TeamID != "FLA" || got.Games != "7" {
		t.Errorf("expected the updated team id %q and games %q, got %q/%q", "FLA", "7", got.TeamID, got.Games)
	}
	if got.SubmittedAt != second.Format(time.RFC3339) {
		t.Errorf("expected the updated submitted_at %q, got %q", second.Format(time.RFC3339), got.SubmittedAt)
	}
}

// assertSeriesPick fails t unless playerID has a saved series pick for
// seriesKey matching wantTeamID/wantGames - assertSavedDivisionPlayoffTeams'
// own single-assertion-helper precedent, factored out purely to keep its
// callers' own cyclomatic complexity low (gocyclo).
func assertSeriesPick(t *testing.T, st *Store, playerID, seriesKey, wantTeamID, wantGames string) {
	t.Helper()
	got, ok := st.FindSeriesPick(playerID, seriesKey)
	if !ok {
		t.Errorf("expected a series pick for %q/%q, found none", playerID, seriesKey)
		return
	}
	if got.TeamID != wantTeamID || got.Games != wantGames {
		t.Errorf("expected %q/%q for %q/%q, got %q/%q", wantTeamID, wantGames, playerID, seriesKey, got.TeamID, got.Games)
	}
}

func TestSaveSeriesPickShouldNotTouchARowForADifferentSeriesKeyOrPlayer(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if _, err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", now); err != nil {
		t.Fatalf("SaveSeriesPick returned error: %v", err)
	}
	if _, err := st.SaveSeriesPick("basti", "r1.s2", "EDM", "5", now); err != nil {
		t.Fatalf("SaveSeriesPick returned error: %v", err)
	}
	if _, err := st.SaveSeriesPick("other-player", "r1.s1", "BOS", "4", now); err != nil {
		t.Fatalf("SaveSeriesPick returned error: %v", err)
	}

	assertSeriesPick(t, st, "basti", "r1.s1", "TOR", "6")
	assertSeriesPick(t, st, "basti", "r1.s2", "EDM", "5")
	assertSeriesPick(t, st, "other-player", "r1.s1", "BOS", "4")
}

func TestSaveSeriesPickShouldPersistToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if _, err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveSeriesPick returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal file: %v", err)
	}
	if len(doc.Predictions) != 1 {
		t.Fatalf("expected 1 persisted prediction, got %d", len(doc.Predictions))
	}
	got := doc.Predictions[0]
	if got.Kind != KindSeries || got.SeriesKey != "r1.s1" || got.TeamID != "TOR" || got.Games != "6" {
		t.Errorf("expected the persisted file to contain the new series pick, got %+v", got)
	}
}

func TestSaveSeriesPickShouldLogOnASuccessfulWrite(t *testing.T) {
	st := newTestStore(t)
	logs := captureLogs(t)

	if _, err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveSeriesPick returned error: %v", err)
	}

	assertExactlyOneInfoLine(t, logs)
	if !strings.Contains(logs.String(), `reason="save series pick"`) {
		t.Errorf("expected reason=%q, got %q", "save series pick", logs.String())
	}
}

func TestSaveSeriesPickShouldRollBackTheAppendWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if _, err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", time.Now().UTC()); err == nil {
		t.Fatal("expected SaveSeriesPick to return an error when the write fails")
	}

	if _, ok := st.FindSeriesPick("basti", "r1.s1"); ok {
		t.Error("expected the failed append to be rolled back, but a Prediction row was found")
	}
	assertNoLogOutput(t, logs)
}

func TestSaveSeriesPickShouldRollBackTheUpdateWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if _, err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveSeriesPick returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	logs := captureLogs(t)

	if _, err := st.SaveSeriesPick("basti", "r1.s1", "FLA", "7", time.Now().UTC()); err == nil {
		t.Fatal("expected SaveSeriesPick to return an error when the write fails")
	}

	got, ok := st.FindSeriesPick("basti", "r1.s1")
	if !ok {
		t.Fatal("expected the original row to still be found")
	}
	if got.TeamID != "TOR" || got.Games != "6" {
		t.Errorf("expected the failed update to be rolled back to %q/%q, got %q/%q", "TOR", "6", got.TeamID, got.Games)
	}
	assertNoLogOutput(t, logs)
}

func TestStoreShouldBeSafeForConcurrentCreateLoginCode(t *testing.T) {
	st := newTestStore(t)
	captureLogs(t) // silence the "store write" line each concurrent write now emits (spec-7-1)

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := st.CreateLoginCode("basti", "hash", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
				t.Errorf("CreateLoginCode returned error: %v", err)
			}
		}()
	}
	wg.Wait()

	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.doc.LoginCodes) != n {
		t.Errorf("expected %d login codes, got %d", n, len(st.doc.LoginCodes))
	}
}

// TestJoinSeriesKeyShouldRoundTripThroughSplitSeriesKey proves
// JoinSeriesKey/SplitSeriesKey are true inverses of one another, guarding
// the persisted "r1.s1"-shaped series_key format against an accidental
// separator change ever silently breaking the round trip.
func TestJoinSeriesKeyShouldRoundTripThroughSplitSeriesKey(t *testing.T) {
	joined := JoinSeriesKey("r1", "s1")
	if joined != "r1.s1" {
		t.Fatalf("expected joined key %q, got %q", "r1.s1", joined)
	}

	setID, key, ok := SplitSeriesKey(joined)
	if !ok {
		t.Fatal("expected SplitSeriesKey to report ok=true for a joined key")
	}
	if setID != "r1" || key != "s1" {
		t.Errorf("expected setID/key %q/%q, got %q/%q", "r1", "s1", setID, key)
	}
}

func TestSplitSeriesKeyShouldReportNotOkForAKeyWithNoSeparator(t *testing.T) {
	_, _, ok := SplitSeriesKey("malformed")
	if ok {
		t.Error("expected ok=false for a key with no separator")
	}
}

func TestDivisionsShouldReturnTheFixedDisplayOrder(t *testing.T) {
	want := []string{"Atlantic", "Metropolitan", "Central", "Pacific"}
	if got := Divisions(); !slices.Equal(got, want) {
		t.Errorf("expected divisions %v, got %v", want, got)
	}
}

func TestDivisionsShouldNotBeMutatedByACaller(t *testing.T) {
	first := Divisions()
	first[0] = "Mutated"

	want := []string{"Atlantic", "Metropolitan", "Central", "Pacific"}
	if got := Divisions(); !slices.Equal(got, want) {
		t.Errorf("expected divisions %v to survive a caller's mutation, got %v", want, got)
	}
}

func TestPlayersShouldReturnEveryPlayer(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase)

	got := st.Players()
	if len(got) != 2 || got[0].ID != "basti" || got[1].ID != "kim" {
		t.Errorf("Players = %v, want basti and kim", got)
	}
}

func TestPredictionsForPlayerShouldReturnOnlyThatPlayersRows(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase)

	got := st.PredictionsForPlayer("basti")
	if len(got) != 1 || got[0].ID != "p1" {
		t.Errorf("PredictionsForPlayer(basti) = %v, want only p1", got)
	}
	if got := st.PredictionsForPlayer("ghost"); len(got) != 0 {
		t.Errorf("PredictionsForPlayer(ghost) = %v, want none", got)
	}
}

func TestSavePredictionShouldReturnThePersistedRow(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	rows, err := st.SavePrediction("basti", KindCupChampion, "TOR", now)
	if err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Kind != KindCupChampion || rows[0].TeamID != "TOR" || rows[0].PlayerID != "basti" {
		t.Errorf("expected the one saved cup row, got %+v", rows)
	}

	rows, err = st.SavePrediction("basti", KindCupChampion, "VGK", now)
	if err != nil {
		t.Fatalf("SavePrediction update returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].TeamID != "VGK" {
		t.Errorf("expected the updated row on resubmission, got %+v", rows)
	}
}

func TestSavePredictionShouldReturnNoRowWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	st, err := New(filepath.Join(dir, DataFileName))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	captureLogs(t)

	rows, err := st.SavePrediction("basti", KindCupChampion, "TOR", time.Now().UTC())
	if err == nil || len(rows) != 0 {
		t.Errorf("expected an error and no rows, got rows=%+v err=%v", rows, err)
	}
}

func TestSaveSeriesPickShouldReturnThePersistedRow(t *testing.T) {
	st := newTestStore(t)
	rows, err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", time.Now().UTC())
	if err != nil {
		t.Fatalf("SaveSeriesPick returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Kind != KindSeries || rows[0].SeriesKey != "r1.s1" {
		t.Errorf("expected the one saved series row, got %+v", rows)
	}
}

func TestSaveDivisionPicksShouldReturnOnlyRowsCarryingAPick(t *testing.T) {
	st := newTestStore(t)
	playoffTeams := map[string][]string{"Atlantic": {"TOR", "BOS"}, "Metropolitan": {}}
	winners := map[string]string{"Atlantic": "TOR", "Metropolitan": ""}

	rows, err := st.SaveDivisionPicks("basti", playoffTeams, winners, time.Now().UTC())
	if err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Kind]++
	}
	if len(rows) != 2 || counts[KindDivisionPlayoffTeams] != 1 || counts[KindDivisionWinner] != 1 {
		t.Errorf("expected one playoff-teams and one winner row, got %+v", rows)
	}
}

func TestSaveAwardPicksShouldReturnOnlyFullyFilledAwards(t *testing.T) {
	st := newTestStore(t)
	finalists := map[string][]string{
		AwardHart:   {"a", "b", "c"},
		AwardNorris: {"d", "e", "f"},
		AwardVezina: {"g", "h", ""},
	}

	rows, err := st.SaveAwardPicks("basti", finalists, time.Now().UTC())
	if err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 persisted award rows, got %+v", rows)
	}

	rows, err = st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "", ""}}, time.Now().UTC())
	if err != nil || len(rows) != 0 {
		t.Errorf("expected no rows for an incomplete award, got rows=%+v err=%v", rows, err)
	}
}

func TestKindsShouldListEveryPredictionKind(t *testing.T) {
	want := []string{
		KindCupChampion, KindPresidentsTrophy, KindPlayoffsCup,
		KindDivisionPlayoffTeams, KindDivisionWinner, KindAward, KindSeries,
	}
	if len(Kinds) != len(want) {
		t.Fatalf("Kinds = %v, want %v", Kinds, want)
	}
	for i := range want {
		if Kinds[i] != want[i] {
			t.Errorf("Kinds[%d] = %q, want %q", i, Kinds[i], want[i])
		}
	}
}
