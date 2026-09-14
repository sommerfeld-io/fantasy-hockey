package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

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
