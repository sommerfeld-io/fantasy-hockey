package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

func TestNewShouldBootstrapCreateAMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fantasy-hockey.yml")

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
	dir := t.TempDir()
	path := filepath.Join(dir, "fantasy-hockey.yml")
	seed := `season: "2025-26"
players:
    - id: basti
      name: Basti
      email: basti@example.com
`
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	st, err := New(path)
	if err != nil {
		t.Fatalf("New(%q) returned error: %v", path, err)
	}

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
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
	st, err := New(filepath.Join(dir, "fantasy-hockey.yml"))
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
	st, err := New(filepath.Join(dir, "fantasy-hockey.yml"))
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
	dir := t.TempDir()
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

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
	dir := t.TempDir()
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	sets := st.PredictionSets()
	sets[0].Title = "Tampered"

	again := st.PredictionSets()
	if again[0].Title != "Cup champion" {
		t.Errorf("expected the store's own copy to stay untouched, got title %q", again[0].Title)
	}
}

func TestTeamsShouldReturnTheSeededList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

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
	dir := t.TempDir()
	path := filepath.Join(dir, "fantasy-hockey.yml")
	seed := `season: "2026-27"
players: []
login_codes: []
teams:
    - id: TOR
      name: Toronto Maple Leafs
      conference: Eastern
      division: Atlantic
`
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	teams := st.Teams()
	teams[0].Name = "Tampered"

	again := st.Teams()
	if again[0].Name != "Toronto Maple Leafs" {
		t.Errorf("expected the store's own copy to stay untouched, got name %q", again[0].Name)
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
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
	path := filepath.Join(dir, "fantasy-hockey.yml")
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

	cup, ok := st.FindPrediction("basti", KindCupChampion)
	if !ok || cup.TeamID != "TOR" {
		t.Errorf("expected the cup pick to stay %q, got %+v (ok=%v)", "TOR", cup, ok)
	}
	presidents, ok := st.FindPrediction("basti", KindPresidentsTrophy)
	if !ok || presidents.TeamID != "VGK" {
		t.Errorf("expected the presidents pick to be %q, got %+v (ok=%v)", "VGK", presidents, ok)
	}
}

func TestSavePredictionShouldPersistToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
	path := filepath.Join(dir, "fantasy-hockey.yml")
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
