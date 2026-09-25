package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

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

	if doc.Season != "2026-27" {
		t.Errorf("expected season %q, got %q", "2026-27", doc.Season)
	}
	if len(doc.Players) != 0 {
		t.Errorf("expected an empty players list, got %v", doc.Players)
	}
	if st == nil {
		t.Fatal("expected a non-nil Store")
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

	if got := st.Season(); got != "2026-27" {
		t.Errorf("expected season %q, got %q", "2026-27", got)
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

	if err := st.CreateLoginCode("basti", "hash-1", "2026-09-14T10:00:00Z"); err != nil {
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

	if err := st.CreateLoginCode("basti", "hash-1", "2026-09-14T10:00:00Z"); err != nil {
		t.Fatalf("first CreateLoginCode returned error: %v", err)
	}
	if err := st.CreateLoginCode("basti", "hash-2", "2026-09-14T10:05:00Z"); err != nil {
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

	if err := st.CreateLoginCode("basti", "hash-1", "2026-09-14T10:00:00Z"); err != nil {
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

	if err := st.CreateLoginCode("basti", "hash-1", "2026-09-14T10:00:00Z"); err == nil {
		t.Fatal("expected CreateLoginCode to return an error when the write fails")
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.doc.LoginCodes) != 0 {
		t.Errorf("expected the failed append to be rolled back, got %d login code(s) still in memory", len(st.doc.LoginCodes))
	}
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

	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.doc.LoginCodes[0].UsedAt == nil {
		t.Fatal("expected used_at to be set")
	}
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

	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.doc.LoginCodes[1].UsedAt != nil {
		t.Errorf("expected the second row to stay untouched, got used_at=%v", *st.doc.LoginCodes[1].UsedAt)
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

	if _, ok, err := st.ConsumeLoginCode("hash-1", now); err == nil || ok {
		t.Fatalf("expected ConsumeLoginCode to fail when the write fails, got ok=%v, err=%v", ok, err)
	}

	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.doc.LoginCodes[0].UsedAt != nil {
		t.Errorf("expected the failed mark to be rolled back, got used_at=%v", *st.doc.LoginCodes[0].UsedAt)
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

	if err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err != nil {
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

	if err := st.SavePrediction("basti", KindCupChampion, "TOR", first); err != nil {
		t.Fatalf("first SavePrediction returned error: %v", err)
	}
	if err := st.SavePrediction("basti", KindCupChampion, "VGK", second); err != nil {
		t.Fatalf("second SavePrediction returned error: %v", err)
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

	if err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}
	if err := st.SavePrediction("basti", KindPresidentsTrophy, "VGK", now); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}
	if err := st.SavePrediction("other-player", KindCupChampion, "COL", now); err != nil {
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

	if err := st.SavePrediction("basti", KindCupChampion, "TOR", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
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

	if err := st.SavePrediction("basti", KindCupChampion, "TOR", time.Now().UTC()); err == nil {
		t.Fatal("expected SavePrediction to return an error when the write fails")
	}

	if _, ok := st.FindPrediction("basti", KindCupChampion); ok {
		t.Error("expected the failed append to be rolled back, but a Prediction row was found")
	}
}

func TestSavePredictionShouldRollBackTheUpdateWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := st.SavePrediction("basti", KindCupChampion, "TOR", time.Now().UTC()); err != nil {
		t.Fatalf("seed SavePrediction returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	if err := st.SavePrediction("basti", KindCupChampion, "VGK", time.Now().UTC()); err == nil {
		t.Fatal("expected SavePrediction to return an error when the write fails")
	}

	got, ok := st.FindPrediction("basti", KindCupChampion)
	if !ok {
		t.Fatal("expected the original row to still be found")
	}
	if got.TeamID != "TOR" {
		t.Errorf("expected the failed update to be rolled back to %q, got %q", "TOR", got.TeamID)
	}
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

	if err := st.SaveDivisionPicks("basti", playoffTeams, winners, now); err != nil {
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

	if err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, nil, now); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}

	if _, ok := st.FindDivisionPlayoffTeams("basti", "Metropolitan"); ok {
		t.Error("expected no row for a division genuinely absent from the map")
	}
}

func TestSaveDivisionPicksShouldUpsertAPresentButEmptyDivisionWithAnEmptyPick(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {}}, nil, now); err != nil {
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

	if err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, map[string]string{"Atlantic": "TOR"}, first); err != nil {
		t.Fatalf("first SaveDivisionPicks returned error: %v", err)
	}
	if err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"BOS", "TBL"}}, map[string]string{"Atlantic": "BOS"}, second); err != nil {
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

	if err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, nil, now); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}
	if err := st.SaveDivisionPicks("basti", map[string][]string{"Metropolitan": {"WSH"}}, nil, now); err != nil {
		t.Fatalf("SaveDivisionPicks returned error: %v", err)
	}
	if err := st.SaveDivisionPicks("other-player", map[string][]string{"Atlantic": {"COL"}}, nil, now); err != nil {
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

	if err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, map[string]string{"Atlantic": "TOR"}, time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
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

	if err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, map[string]string{"Atlantic": "TOR"}, time.Now().UTC()); err == nil {
		t.Fatal("expected SaveDivisionPicks to return an error when the write fails")
	}

	if _, ok := st.FindDivisionPlayoffTeams("basti", "Atlantic"); ok {
		t.Error("expected the failed append to be rolled back, but a playoff-teams row was found")
	}
	if _, ok := st.FindDivisionWinner("basti", "Atlantic"); ok {
		t.Error("expected the failed append to be rolled back, but a winner row was found")
	}
}

func TestSaveDivisionPicksShouldRollBackTheUpdatesWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"TOR"}}, map[string]string{"Atlantic": "TOR"}, time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveDivisionPicks returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	if err := st.SaveDivisionPicks("basti", map[string][]string{"Atlantic": {"BOS"}}, map[string]string{"Atlantic": "BOS"}, time.Now().UTC()); err == nil {
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

	if err := st.SaveAwardPicks("basti", finalists, now); err != nil {
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

	if err := st.SaveAwardPicks("basti", finalists, now); err != nil {
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

	if err := st.SaveAwardPicks("basti", finalists, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}

	if _, ok := st.FindAwardFinalists("basti", AwardHart); ok {
		t.Error("expected no row for an award with an empty slug among its three")
	}
}

func TestSaveAwardPicksShouldNotUpsertAnAwardAbsentFromTheMap(t *testing.T) {
	st := newTestStore(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	if err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, now); err != nil {
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

	if err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, first); err != nil {
		t.Fatalf("first SaveAwardPicks returned error: %v", err)
	}
	if err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"x", "y", "z"}}, second); err != nil {
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

	if err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}
	if err := st.SaveAwardPicks("basti", map[string][]string{AwardNorris: {"d", "e", "f"}}, now); err != nil {
		t.Fatalf("SaveAwardPicks returned error: %v", err)
	}
	if err := st.SaveAwardPicks("other-player", map[string][]string{AwardHart: {"g", "h", "i"}}, now); err != nil {
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

	if err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
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

	if err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, time.Now().UTC()); err == nil {
		t.Fatal("expected SaveAwardPicks to return an error when the write fails")
	}

	if _, ok := st.FindAwardFinalists("basti", AwardHart); ok {
		t.Error("expected the failed append to be rolled back, but an award row was found")
	}
}

func TestSaveAwardPicksShouldRollBackTheUpdateWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"a", "b", "c"}}, time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveAwardPicks returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	if err := st.SaveAwardPicks("basti", map[string][]string{AwardHart: {"x", "y", "z"}}, time.Now().UTC()); err == nil {
		t.Fatal("expected SaveAwardPicks to return an error when the write fails")
	}

	got, ok := st.FindAwardFinalists("basti", AwardHart)
	if !ok || !slices.Equal(got.FinalistSlugs, []string{"a", "b", "c"}) {
		t.Errorf("expected the failed update to be rolled back to %v, got %+v (ok=%v)", []string{"a", "b", "c"}, got, ok)
	}
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

	if err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", now); err != nil {
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

	if err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", first); err != nil {
		t.Fatalf("first SaveSeriesPick returned error: %v", err)
	}
	if err := st.SaveSeriesPick("basti", "r1.s1", "FLA", "7", second); err != nil {
		t.Fatalf("second SaveSeriesPick returned error: %v", err)
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

	if err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", now); err != nil {
		t.Fatalf("SaveSeriesPick returned error: %v", err)
	}
	if err := st.SaveSeriesPick("basti", "r1.s2", "EDM", "5", now); err != nil {
		t.Fatalf("SaveSeriesPick returned error: %v", err)
	}
	if err := st.SaveSeriesPick("other-player", "r1.s1", "BOS", "4", now); err != nil {
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

	if err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)); err != nil {
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

	if err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", time.Now().UTC()); err == nil {
		t.Fatal("expected SaveSeriesPick to return an error when the write fails")
	}

	if _, ok := st.FindSeriesPick("basti", "r1.s1"); ok {
		t.Error("expected the failed append to be rolled back, but a Prediction row was found")
	}
}

func TestSaveSeriesPickShouldRollBackTheUpdateWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := st.SaveSeriesPick("basti", "r1.s1", "TOR", "6", time.Now().UTC()); err != nil {
		t.Fatalf("seed SaveSeriesPick returned error: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	if err := st.SaveSeriesPick("basti", "r1.s1", "FLA", "7", time.Now().UTC()); err == nil {
		t.Fatal("expected SaveSeriesPick to return an error when the write fails")
	}

	got, ok := st.FindSeriesPick("basti", "r1.s1")
	if !ok {
		t.Fatal("expected the original row to still be found")
	}
	if got.TeamID != "TOR" || got.Games != "6" {
		t.Errorf("expected the failed update to be rolled back to %q/%q, got %q/%q", "TOR", "6", got.TeamID, got.Games)
	}
}

func TestStoreShouldBeSafeForConcurrentCreateLoginCode(t *testing.T) {
	st := newTestStore(t)

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := st.CreateLoginCode("basti", "hash", "2026-09-14T10:00:00Z"); err != nil {
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

// seedSubmittedAt stamps every seeded prediction row; no results test
// reads it.
const seedSubmittedAt = "2026-09-20T10:00:00Z"

// resultsFixtureBase is the canonical data every results test builds on:
// three Atlantic teams and one Pacific team, four NHL players and one matchup in round 1 and in
// the conference finals.
const resultsFixtureBase = `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
    - id: kim
      name: Kim
      email: kim@example.com
teams:
    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}
    - {id: TOR, name: Toronto Maple Leafs, conference: Eastern, division: Atlantic}
    - {id: BOS, name: Boston Bruins, conference: Eastern, division: Atlantic}
    - {id: EDM, name: Edmonton Oilers, conference: Western, division: Pacific}
nhl_players:
    - {slug: mcdavid-connor, display_name: Connor McDavid, position: skater}
    - {slug: mackinnon-nathan, display_name: Nathan MacKinnon, position: skater}
    - {slug: kucherov-nikita, display_name: Nikita Kucherov, position: skater}
    - {slug: matthews-auston, display_name: Auston Matthews, position: skater}
playoff_matchups:
    r1:
        - {key: s1, a: FLA, b: TOR}
    cf:
        - {key: s1, a: FLA, b: BOS}
predictions:
    - id: p1
      player_id: basti
      kind: division_playoff_teams
      division: Atlantic
      team_ids: [FLA, TOR]
      submitted_at: "` + seedSubmittedAt + `"
    - id: p2
      player_id: kim
      kind: cup
      team_id: TOR
      submitted_at: "` + seedSubmittedAt + `"
`

// resultsFixtureResults is a well-formed results and award_finalists
// section: games are unquoted, the way a human hand-edits them.
const resultsFixtureResults = `results:
    team_marks:
        atlantic:
            playoffs: [FLA, TOR]
            division_winner: FLA
    presidents_trophy: TOR
    stanley_cup_winner: FLA
    series:
        round1:
            s1: {winner: FLA, games: 5}
        round3:
            s1: {winner: BOS, games: 7}
award_finalists:
    hart:
        - {slug: mcdavid-connor, display_name: Connor McDavid}
        - {slug: mackinnon-nathan, display_name: Nathan MacKinnon}
        - {slug: kucherov-nikita, display_name: Nikita Kucherov}
        - {slug: matthews-auston, display_name: Auston Matthews}
`

// newSeededStore writes seed to a temp data file and opens st on it, so
// New's own parsing of the seeded document is exercised. path is the data
// file's path (not its directory), for tests that inspect or reopen it.
func newSeededStore(t *testing.T, seed string) (st *Store, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := New(path)
	if err != nil {
		t.Fatalf("New(%q) returned error: %v", path, err)
	}
	return st, path
}

func TestNewShouldLoadAFileWithNeitherResultsNorAwardFinalists(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase)

	if playoffs, winner := st.DivisionResult("Atlantic"); len(playoffs) != 0 || winner != "" {
		t.Errorf("DivisionResult = %v, %q, want nothing recorded", playoffs, winner)
	}
	if got := st.PresidentsTrophyWinner(); got != "" {
		t.Errorf("PresidentsTrophyWinner = %q, want empty", got)
	}
	if got := st.StanleyCupWinner(); got != "" {
		t.Errorf("StanleyCupWinner = %q, want empty", got)
	}
	if _, _, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s1")); ok {
		t.Error("SeriesResult ok = true, want false with no results recorded")
	}
	if got := st.RecordedAwardFinalists(AwardHart); len(got) != 0 {
		t.Errorf("RecordedAwardFinalists = %v, want empty", got)
	}
	if got := st.ResultProblems(); len(got) != 0 {
		t.Errorf("ResultProblems = %v, want none", got)
	}
}

func TestDivisionResultShouldReadTheLowercaseDivisionKeyForTheCapitalisedName(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	playoffs, winner := st.DivisionResult("Atlantic")
	if !slices.Equal(playoffs, []string{"FLA", "TOR"}) || winner != "FLA" {
		t.Errorf("DivisionResult(Atlantic) = %v, %q, want [FLA TOR], FLA", playoffs, winner)
	}
}

func TestDivisionResultShouldNotMatchTheLowercaseNameOrAnotherDivision(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	for _, division := range []string{"atlantic", "Metropolitan"} {
		if playoffs, winner := st.DivisionResult(division); len(playoffs) != 0 || winner != "" {
			t.Errorf("DivisionResult(%q) = %v, %q, want nothing", division, playoffs, winner)
		}
	}
}

func TestTrophyWinnersShouldReturnTheRecordedTeams(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	if got := st.PresidentsTrophyWinner(); got != "TOR" {
		t.Errorf("PresidentsTrophyWinner = %q, want TOR", got)
	}
	if got := st.StanleyCupWinner(); got != "FLA" {
		t.Errorf("StanleyCupWinner = %q, want FLA", got)
	}
}

func TestSeriesResultShouldMapRoundNamesToSetIDsAndLoadUnquotedGames(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	tests := []struct {
		seriesKey, wantWinner, wantGames string
	}{
		{JoinSeriesKey(Round1SetID, "s1"), "FLA", "5"},
		{JoinSeriesKey(ConferenceFinalsSetID, "s1"), "BOS", "7"},
	}
	for _, tt := range tests {
		winner, games, ok := st.SeriesResult(tt.seriesKey)
		if !ok || winner != tt.wantWinner || games != tt.wantGames {
			t.Errorf("SeriesResult(%q) = %q, %q, %v, want %q, %q, true", tt.seriesKey, winner, games, ok, tt.wantWinner, tt.wantGames)
		}
	}
}

func TestSeriesResultShouldNotReturnAnUnrecordedOrMalformedKey(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	for _, key := range []string{JoinSeriesKey(Round2SetID, "s1"), JoinSeriesKey(Round1SetID, "s2"), "round1.s1", "r1"} {
		if _, _, ok := st.SeriesResult(key); ok {
			t.Errorf("SeriesResult(%q) ok = true, want false", key)
		}
	}
}

func TestSeriesResultShouldNotReturnAHalfRecordedSeries(t *testing.T) {
	seed := resultsFixtureBase + "results:\n    series:\n        round1:\n            s1: {winner: FLA}\n"
	st, _ := newSeededStore(t, seed)

	if _, _, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s1")); ok {
		t.Error("SeriesResult ok = true for a series with no games recorded, want false")
	}
	if got := st.ResultProblems(); len(got) != 0 {
		t.Errorf("ResultProblems = %v, want none for a series still in progress", got)
	}
}

func TestRecordedAwardFinalistsShouldReturnEverySlugIncludingATie(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	want := []string{"mcdavid-connor", "mackinnon-nathan", "kucherov-nikita", "matthews-auston"}
	if got := st.RecordedAwardFinalists(AwardHart); !slices.Equal(got, want) {
		t.Errorf("RecordedAwardFinalists(hart) = %v, want %v", got, want)
	}
	if got := st.RecordedAwardFinalists(AwardNorris); len(got) != 0 {
		t.Errorf("RecordedAwardFinalists(norris) = %v, want empty", got)
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

func TestResultReadMethodsShouldReturnCopiesThatDoNotAliasTheStore(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	playoffs, _ := st.DivisionResult("Atlantic")
	playoffs[0] = "XXX"
	finalists := st.RecordedAwardFinalists(AwardHart)
	finalists[0] = "xxx"
	players := st.Players()
	players[0].ID = "xxx"
	predictions := st.PredictionsForPlayer("basti")
	predictions[0].TeamIDs[0] = "XXX"

	if again, _ := st.DivisionResult("Atlantic"); again[0] != "FLA" {
		t.Errorf("DivisionResult aliased internal state: got %v", again)
	}
	if again := st.RecordedAwardFinalists(AwardHart); again[0] != "mcdavid-connor" {
		t.Errorf("RecordedAwardFinalists aliased internal state: got %v", again)
	}
	if again := st.Players(); again[0].ID != "basti" {
		t.Errorf("Players aliased internal state: got %v", again)
	}
	if again := st.PredictionsForPlayer("basti"); again[0].TeamIDs[0] != "FLA" {
		t.Errorf("PredictionsForPlayer aliased internal state: got %v", again)
	}
}

func TestResultProblemsShouldReportEachMalformedEntryAndIgnoreIt(t *testing.T) {
	tests := []struct {
		name    string
		results string
		want    string
		check   func(*Store) bool
	}{
		{
			name:    "unknown playoff team",
			results: "results:\n    team_marks:\n        atlantic: {playoffs: [FLA, XXX], division_winner: FLA}\n",
			want:    "XXX",
			check:   func(st *Store) bool { p, _ := st.DivisionResult("Atlantic"); return slices.Equal(p, []string{"FLA"}) },
		},
		{
			name:    "unknown division winner",
			results: "results:\n    team_marks:\n        atlantic: {playoffs: [FLA], division_winner: XXX}\n",
			want:    "XXX",
			check:   func(st *Store) bool { _, w := st.DivisionResult("Atlantic"); return w == "" },
		},
		{
			name:    "unknown division",
			results: "results:\n    team_marks:\n        northeast: {playoffs: [FLA]}\n",
			want:    "northeast",
			check:   func(*Store) bool { return true },
		},
		{
			name:    "capitalised division key names the lowercase keys",
			results: "results:\n    team_marks:\n        Atlantic: {playoffs: [FLA]}\n",
			want:    "atlantic",
			check:   func(*Store) bool { return true },
		},
		{
			name:    "unknown round names the valid rounds",
			results: "results:\n    series:\n        round9:\n            s1: {winner: FLA, games: 5}\n",
			want:    "round1",
			check:   func(*Store) bool { return true },
		},
		{
			name:    "unknown presidents trophy team",
			results: "results:\n    presidents_trophy: XXX\n",
			want:    "XXX",
			check:   func(st *Store) bool { return st.PresidentsTrophyWinner() == "" },
		},
		{
			name:    "unknown stanley cup team",
			results: "results:\n    stanley_cup_winner: XXX\n",
			want:    "XXX",
			check:   func(st *Store) bool { return st.StanleyCupWinner() == "" },
		},
		{
			name:    "unknown round",
			results: "results:\n    series:\n        round9:\n            s1: {winner: FLA, games: 5}\n",
			want:    "round9",
			check:   func(*Store) bool { return true },
		},
		{
			name:    "series key without a matchup",
			results: "results:\n    series:\n        round1:\n            s7: {winner: FLA, games: 5}\n",
			want:    "s7",
			check:   func(st *Store) bool { _, _, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s7")); return !ok },
		},
		{
			name:    "unknown series winner",
			results: "results:\n    series:\n        round1:\n            s1: {winner: XXX, games: 5}\n",
			want:    "XXX",
			check:   func(st *Store) bool { _, _, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s1")); return !ok },
		},
		{
			name:    "games outside 4-7",
			results: "results:\n    series:\n        round1:\n            s1: {winner: FLA, games: 3}\n",
			want:    "games",
			check:   func(st *Store) bool { _, _, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s1")); return !ok },
		},
		{
			name:    "playoff team from another division",
			results: "results:\n    team_marks:\n        atlantic: {playoffs: [FLA, EDM], division_winner: FLA}\n",
			want:    "EDM",
			check:   func(st *Store) bool { p, _ := st.DivisionResult("Atlantic"); return slices.Equal(p, []string{"FLA"}) },
		},
		{
			name:    "division winner from another division",
			results: "results:\n    team_marks:\n        atlantic: {playoffs: [FLA], division_winner: EDM}\n",
			want:    "EDM",
			check:   func(st *Store) bool { _, w := st.DivisionResult("Atlantic"); return w == "" },
		},
		{
			name:    "series winner not in the matchup",
			results: "results:\n    series:\n        round1:\n            s1: {winner: BOS, games: 5}\n",
			want:    "BOS",
			check:   func(st *Store) bool { _, _, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s1")); return !ok },
		},
		{
			name:    "unknown award",
			results: "award_finalists:\n    selke:\n        - {slug: mcdavid-connor, display_name: Connor McDavid}\n",
			want:    "selke",
			check:   func(st *Store) bool { return len(st.RecordedAwardFinalists("selke")) == 0 },
		},
		{
			name:    "unknown finalist slug",
			results: "award_finalists:\n    hart:\n        - {slug: mcdavid-connor, display_name: Connor McDavid}\n        - {slug: nobody-here, display_name: Nobody}\n",
			want:    "nobody-here",
			check: func(st *Store) bool {
				return slices.Equal(st.RecordedAwardFinalists(AwardHart), []string{"mcdavid-connor"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, _ := newSeededStore(t, resultsFixtureBase+tt.results)

			problems := st.ResultProblems()
			if len(problems) != 1 || !strings.Contains(problems[0], tt.want) {
				t.Errorf("ResultProblems = %v, want exactly one mentioning %q", problems, tt.want)
			}
			if !tt.check(st) {
				t.Error("expected the malformed entry to be ignored by the read methods")
			}
		})
	}
}

func TestResultProblemsShouldReportNothingForAWellFormedFile(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	if got := st.ResultProblems(); len(got) != 0 {
		t.Errorf("ResultProblems = %v, want none", got)
	}
}

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

// resultsFixtureMatchups is resultsFixtureBase's playoff_matchups section.
const resultsFixtureMatchups = "playoff_matchups:\n    r1:\n        - {key: s1, a: FLA, b: TOR}\n    cf:\n        - {key: s1, a: FLA, b: BOS}\n"

// resultRoundNames is the results.series round name a human records for
// each round Prediction Set id - the store's own mapping stays unexported,
// so this test-side copy is what pins it.
var resultRoundNames = map[string]string{
	Round1SetID:           "round1",
	Round2SetID:           "round2",
	ConferenceFinalsSetID: "round3",
	StanleyCupFinalSetID:  "round4",
}

func TestSeriesResultShouldRoundTripEveryRoundSetID(t *testing.T) {
	if !strings.Contains(resultsFixtureBase, resultsFixtureMatchups) {
		t.Fatal("resultsFixtureMatchups no longer matches resultsFixtureBase's playoff_matchups section; update it")
	}
	for setID, round := range resultRoundNames {
		t.Run(setID, func(t *testing.T) {
			seed := strings.Replace(resultsFixtureBase, resultsFixtureMatchups, "playoff_matchups:\n    "+setID+":\n        - {key: x1, a: FLA, b: TOR}\n", 1) +
				"results:\n    series:\n        " + round + ":\n            x1: {winner: TOR, games: 6}\n"
			st, _ := newSeededStore(t, seed)

			winner, games, ok := st.SeriesResult(JoinSeriesKey(setID, "x1"))
			if !ok || winner != "TOR" || games != "6" {
				t.Errorf("SeriesResult(%s.x1) = %q, %q, %v, want TOR, 6, true", setID, winner, games, ok)
			}
			if problems := st.ResultProblems(); len(problems) != 0 {
				t.Errorf("ResultProblems = %v, want none", problems)
			}
		})
	}
}

func TestSeriesResultShouldNotReadARoundRecordedUnderAnotherRoundsName(t *testing.T) {
	seed := strings.Replace(resultsFixtureBase, "playoff_matchups:\n", "playoff_matchups:\n    r2:\n        - {key: s1, a: FLA, b: TOR}\n", 1) +
		"results:\n    series:\n        round2:\n            s1: {winner: FLA, games: 5}\n"
	st, _ := newSeededStore(t, seed)

	if problems := st.ResultProblems(); len(problems) != 0 {
		t.Fatalf("ResultProblems = %v, want none", problems)
	}
	if winner, games, ok := st.SeriesResult(JoinSeriesKey(Round2SetID, "s1")); !ok || winner != "FLA" || games != "5" {
		t.Errorf("SeriesResult(r2.s1) = %q, %q, %v, want FLA, 5, true", winner, games, ok)
	}
	if _, _, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s1")); ok {
		t.Error("SeriesResult(r1.s1) ok = true for a result recorded under round2, want false")
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

func TestSeriesResultShouldNormalizeAHandEditedGameCount(t *testing.T) {
	for _, games := range []string{"05", "+5"} {
		seed := resultsFixtureBase + "results:\n    series:\n        round1:\n            s1: {winner: FLA, games: " + games + "}\n"
		st, _ := newSeededStore(t, seed)

		_, got, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s1"))
		if !ok || got != "5" {
			t.Errorf("SeriesResult games for %q = %q, %v, want \"5\", true", games, got, ok)
		}
		if problems := st.ResultProblems(); len(problems) != 0 {
			t.Errorf("ResultProblems for %q = %v, want none", games, problems)
		}
	}
}

func TestResultProblemsShouldAcceptEitherMatchupTeamAsSeriesWinner(t *testing.T) {
	for _, winner := range []string{"FLA", "TOR"} {
		seed := resultsFixtureBase + "results:\n    series:\n        round1:\n            s1: {winner: " + winner + ", games: 5}\n"
		st, _ := newSeededStore(t, seed)

		if problems := st.ResultProblems(); len(problems) != 0 {
			t.Errorf("ResultProblems for winner %q = %v, want none", winner, problems)
		}
		if got, _, ok := st.SeriesResult(JoinSeriesKey(Round1SetID, "s1")); !ok || got != winner {
			t.Errorf("SeriesResult winner = %q, %v, want %q, true", got, ok, winner)
		}
	}
}

func TestResultProblemsShouldAcceptTeamMarksFromTheirOwnDivision(t *testing.T) {
	seed := resultsFixtureBase + "results:\n    team_marks:\n        atlantic: {playoffs: [FLA, TOR, BOS], division_winner: BOS}\n        pacific: {playoffs: [EDM], division_winner: EDM}\n"
	st, _ := newSeededStore(t, seed)

	if problems := st.ResultProblems(); len(problems) != 0 {
		t.Errorf("ResultProblems = %v, want none", problems)
	}
	if playoffs, winner := st.DivisionResult("Pacific"); !slices.Equal(playoffs, []string{"EDM"}) || winner != "EDM" {
		t.Errorf("DivisionResult(Pacific) = %v, %q, want [EDM], EDM", playoffs, winner)
	}
}
