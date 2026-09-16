// Package store owns all reads and writes to the single fantasy-hockey.yml
// data file. It is the only package that touches that file: every other
// package reaches it through Store's exported methods.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	yaml "go.yaml.in/yaml/v3"
)

// loginCodeValidity is how long a LoginCode row stays eligible for
// ConsumeLoginCode after it was issued (PRD FR-2).
const loginCodeValidity = 10 * time.Minute

// defaultSeason seeds a brand-new data file. It names the current season
// only; no player data is ever invented here (AD-23) - a human hand-edits
// the file afterward to add real players.
const defaultSeason = "2026-27"

// Player is a person taking part in the pool. The player list is
// hand-maintained directly in fantasy-hockey.yml; internal/store never
// writes it.
type Player struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// LoginCode is one issued one-time login code. It is a tracked, persisted
// entity: a new request always appends a new row, and an existing row is
// never mutated or removed. CodeHash is the sha256 hex digest of the code;
// the plaintext code is never persisted.
type LoginCode struct {
	ID       string  `yaml:"id"`
	PlayerID string  `yaml:"player_id"`
	CodeHash string  `yaml:"code_hash"`
	IssuedAt string  `yaml:"issued_at"`
	UsedAt   *string `yaml:"used_at"`
}

// PredictionSet is one Prediction Set a player can eventually submit picks
// for (e.g. "Cup champion", "Playoff round 1"). Like Player, the list is
// hand-maintained directly in fantasy-hockey.yml; internal/store never
// writes it. DeadlineUTC is RFC3339 in UTC (Boundaries & Constraints of
// Story 2.1) - converting it for display is internal/clock's job, not
// store's. Phase groups sets into the Predict screen's sections ("before
// the season" or "playoffs"); Upcoming is a human-maintained flag, not
// computed - a later story may replace it with round-unlocking logic, but
// this one only reads it.
type PredictionSet struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Subtitle    string `yaml:"subtitle"`
	DeadlineUTC string `yaml:"deadline_utc"`
	Phase       string `yaml:"phase"`
	Upcoming    bool   `yaml:"upcoming"`
}

// Prediction Kind values, matching the Prediction Set's own id (AD-17/AD-24/
// AD-28) - one identifier vocabulary, no separate mapping table between a
// Prediction's kind and the Prediction Set it belongs to.
const (
	KindCupChampion      = "cup"
	KindPresidentsTrophy = "presidents"
)

// Prediction is one player's saved pick for one Prediction Set kind (e.g.
// their Stanley Cup champion pick). It's an application-created row (AD-17):
// FindPrediction/SavePrediction are the only ways to read or write it, and a
// resubmission updates the existing (PlayerID, Kind) row in place rather
// than appending a duplicate (FR-9). TeamID is a Team.ID (e.g. "TOR"), the
// value scoring later compares on. SubmittedAt is RFC3339 in UTC, matching
// PredictionSet.DeadlineUTC's convention.
type Prediction struct {
	ID          string `yaml:"id"`
	PlayerID    string `yaml:"player_id"`
	Kind        string `yaml:"kind"`
	TeamID      string `yaml:"team_id"`
	SubmittedAt string `yaml:"submitted_at"`
}

// Team is one of the season's 32 NHL teams. Like Player and PredictionSet,
// the list is hand-maintained directly in fantasy-hockey.yml;
// internal/store never writes it (AD-23). ID is the team's standard
// 3-letter abbreviation (e.g. "TOR"), never a UUID (AD-17) - it's the value
// every downstream reader (a saved Prediction, internal/scoring) compares
// on, never Name.
type Team struct {
	ID         string `yaml:"id"`
	Name       string `yaml:"name"`
	Conference string `yaml:"conference"`
	Division   string `yaml:"division"`
}

// document mirrors fantasy-hockey.yml's on-disk shape.
type document struct {
	Season         string          `yaml:"season"`
	Players        []Player        `yaml:"players"`
	LoginCodes     []LoginCode     `yaml:"login_codes"`
	PredictionSets []PredictionSet `yaml:"prediction_sets"`
	Teams          []Team          `yaml:"teams"`
	Predictions    []Prediction    `yaml:"predictions"`
}

// Store is the in-memory representation of fantasy-hockey.yml, guarded by a
// mutex so every read and write is synchronized (AD-29). Both the mutex and
// the document stay unexported; callers only ever go through Store's
// exported methods.
type Store struct {
	mu   sync.RWMutex
	path string
	doc  document
}

// New loads path into memory. If path doesn't exist, it bootstraps a new
// file there with an empty players list and the current default season,
// per AD-25/AD-26.
func New(path string) (*Store, error) {
	st := &Store{path: path}

	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		st.doc = document{Season: defaultSeason, Players: []Player{}}
		if err := st.writeLocked(); err != nil {
			return nil, fmt.Errorf("store: bootstrap %s: %w", path, err)
		}
		return st, nil
	case err != nil:
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(raw, &st.doc); err != nil {
		return nil, fmt.Errorf("store: parse %s: %w", path, err)
	}
	return st, nil
}

// FindPlayerByEmail returns the player whose email matches, if any. The
// comparison trims surrounding whitespace and ignores case on both sides, so
// a hand-maintained YAML entry or a submitted email that differs only in
// case or stray whitespace still matches. An empty (or whitespace-only)
// email never matches, even against a hand-maintained row that itself has a
// blank Email field.
func (s *Store) FindPlayerByEmail(email string) (Player, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	email = strings.TrimSpace(email)
	if email == "" {
		return Player{}, false
	}
	for _, p := range s.doc.Players {
		if strings.EqualFold(strings.TrimSpace(p.Email), email) {
			return p, true
		}
	}
	return Player{}, false
}

// FindPlayerByID returns the player whose ID matches id, if any - the app
// shell's header uses this to resolve the session's player id (AD-17 - the
// id is an opaque slug, so this is an exact match, unlike FindPlayerByEmail's
// case/whitespace-insensitive comparison) into a display name.
func (s *Store) FindPlayerByID(id string) (Player, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.doc.Players {
		if p.ID == id {
			return p, true
		}
	}
	return Player{}, false
}

// Season returns the store's current season label (e.g. "2026-27"), as
// hand-maintained in fantasy-hockey.yml.
func (s *Store) Season() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.doc.Season
}

// PredictionSets returns every Prediction Set, as hand-maintained in
// fantasy-hockey.yml (Season's read-only pattern: no write method exists,
// since this story only ever reads the list). The returned slice is a copy,
// so a caller mutating it can't reach back into the store's own state.
func (s *Store) PredictionSets() []PredictionSet {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sets := make([]PredictionSet, len(s.doc.PredictionSets))
	copy(sets, s.doc.PredictionSets)
	return sets
}

// Teams returns every one of the season's canonical NHL teams, as
// hand-maintained in fantasy-hockey.yml (Season's read-only pattern: no
// write method exists, since this story only ever reads the list). The
// returned slice is a copy, so a caller mutating it can't reach back into
// the store's own state.
func (s *Store) Teams() []Team {
	s.mu.RLock()
	defer s.mu.RUnlock()

	teams := make([]Team, len(s.doc.Teams))
	copy(teams, s.doc.Teams)
	return teams
}

// CreateLoginCode appends a new LoginCode row for playerID and persists it.
// It never mutates or removes any existing row. If the write fails, the
// appended row is rolled back from memory so a caller told the write failed
// can't later have that row silently persisted by an unrelated successful
// write.
func (s *Store) CreateLoginCode(playerID, codeHash, issuedAt string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.doc.LoginCodes = append(s.doc.LoginCodes, LoginCode{
		ID:       uuid.NewString(),
		PlayerID: playerID,
		CodeHash: codeHash,
		IssuedAt: issuedAt,
	})

	if err := s.writeLocked(); err != nil {
		s.doc.LoginCodes = s.doc.LoginCodes[:len(s.doc.LoginCodes)-1]
		return fmt.Errorf("store: persist login code: %w", err)
	}
	return nil
}

// ConsumeLoginCode looks for an unused LoginCode row matching codeHash whose
// issued_at is neither more than 10 minutes in the past nor in the future
// (guarding against clock skew or a hand-edited row). On a match, it marks
// that row's used_at (using now, RFC3339) and persists the change,
// returning the row's player ID. A wrong, expired, future-dated, or
// already-used code all produce the identical ok=false, err=nil outcome
// (never distinguishing which) so a caller can't tell them apart. Only a
// row exactly matching hash+unused+within-window is ever touched; every
// other row is left exactly as is. If the write fails, the mark is rolled
// back from memory so a caller told the write failed can't later have it
// silently persisted by an unrelated successful write.
func (s *Store) ConsumeLoginCode(codeHash string, now time.Time) (playerID string, ok bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.doc.LoginCodes {
		row := &s.doc.LoginCodes[i]
		if row.CodeHash != codeHash || row.UsedAt != nil {
			continue
		}

		issuedAt, parseErr := time.Parse(time.RFC3339, row.IssuedAt)
		if parseErr != nil || issuedAt.After(now) || now.Sub(issuedAt) > loginCodeValidity {
			continue
		}

		usedAt := now.UTC().Format(time.RFC3339)
		row.UsedAt = &usedAt

		if err := s.writeLocked(); err != nil {
			row.UsedAt = nil
			return "", false, fmt.Errorf("store: persist consumed login code: %w", err)
		}
		return row.PlayerID, true, nil
	}

	return "", false, nil
}

// FindPrediction returns playerID's saved Prediction row for kind, if any.
func (s *Store) FindPrediction(playerID, kind string) (Prediction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.doc.Predictions {
		if p.PlayerID == playerID && p.Kind == kind {
			return p, true
		}
	}
	return Prediction{}, false
}

// SavePrediction saves playerID's pick of teamID for kind, persisting the
// change. An existing (playerID, kind) row is updated in place - a
// resubmission never appends a duplicate (FR-9); otherwise a new row is
// appended with a generated id, mirroring CreateLoginCode's append pattern.
// If the write fails, the change is rolled back from memory (restoring the
// row's old TeamID/SubmittedAt on an update, or truncating the appended row)
// so a caller told the write failed can't later have it silently persisted
// by an unrelated successful write.
func (s *Store) SavePrediction(playerID, kind, teamID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	submittedAt := now.UTC().Format(time.RFC3339)

	for i := range s.doc.Predictions {
		row := &s.doc.Predictions[i]
		if row.PlayerID != playerID || row.Kind != kind {
			continue
		}

		oldTeamID, oldSubmittedAt := row.TeamID, row.SubmittedAt
		row.TeamID = teamID
		row.SubmittedAt = submittedAt

		if err := s.writeLocked(); err != nil {
			row.TeamID, row.SubmittedAt = oldTeamID, oldSubmittedAt
			return fmt.Errorf("store: persist prediction: %w", err)
		}
		return nil
	}

	s.doc.Predictions = append(s.doc.Predictions, Prediction{
		ID:          uuid.NewString(),
		PlayerID:    playerID,
		Kind:        kind,
		TeamID:      teamID,
		SubmittedAt: submittedAt,
	})

	if err := s.writeLocked(); err != nil {
		s.doc.Predictions = s.doc.Predictions[:len(s.doc.Predictions)-1]
		return fmt.Errorf("store: persist prediction: %w", err)
	}
	return nil
}

// writeLocked serializes the in-memory document and atomically replaces the
// file on disk by writing to a temporary file in the same directory and
// renaming it over the original (AD-27). Callers must hold s.mu for
// writing.
func (s *Store) writeLocked() error {
	out, err := yaml.Marshal(s.doc)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".fantasy-hockey-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
