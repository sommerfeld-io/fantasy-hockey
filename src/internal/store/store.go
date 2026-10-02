// Package store owns all reads and writes to the single fantasy-hockey.yml
// data file. It is the only package that touches that file: every other
// package reaches it through Store's exported methods.
package store

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	yaml "go.yaml.in/yaml/v3"
)

// DataFileName is the canonical name of the single data file this app reads
// and writes. Every reference to that filename anywhere in this module -
// production code and tests alike - goes through this constant instead of
// the literal string, so a future rename only requires editing this one
// line. Gherkin `.feature` files are the one sanctioned exception, since
// they describe behavior in prose, not Go code.
const DataFileName = "fantasy-hockey.yml"

// loginCodeValidity is how long a LoginCode row stays eligible for
// ConsumeLoginCode after it was issued (PRD FR-2).
const loginCodeValidity = 10 * time.Minute

// DefaultSeason seeds a brand-new data file. It names the current season
// only; no player data is ever invented here (AD-23) - a human hand-edits
// the file afterward to add real players. It's baked into the binary at
// build time: bump it and rebuild/redeploy before the app is ever started
// against a genuinely new season's fresh path, or the bootstrap silently
// labels that file with a stale season (epic-6 retrospective, item 42;
// docs/operator-guide.md carries the operator-facing warning).
//
// Separately: application code outside internal/store must never branch on
// this constant - e.g. if season == store.DefaultSeason - to build
// season-aware behavior. Store.Season() is the one supported way to read a
// loaded store's current season; comparing it against DefaultSeason (or any
// other season) starts building the cross-season-aware logic epic-6
// explicitly forbids in v1: no in-app season-selector, cross-season query,
// or history/Hall-of-Fame view (epic-6 retrospective, item 47). Test code
// comparing st.Season()/doc.Season against DefaultSeason to verify bootstrap
// behavior (e.g. main_test.go, store_test.go) is the sanctioned exception -
// this rule targets production/application logic, not test assertions.
const DefaultSeason = "2026-27"

// Player is a person taking part in the pool. The player list is
// hand-maintained directly in fantasy-hockey.yml; internal/store never
// writes it.
type Player struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// LoginCode is one issued one-time login code. A new request always appends
// a new row; an existing row is only ever mutated to record its use
// (ConsumeLoginCode), and is removed once it's no longer usable - expired,
// used, or future-dated (cleanupLoginCodes, spec-7-2). CodeHash is the
// sha256 hex digest of the code; the plaintext code is never persisted.
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
// the season" or "playoffs"); Upcoming is a human-maintained flag - for the
// Round2SetID/ConferenceFinalsSetID/StanleyCupFinalSetID round ids it is no
// longer read directly (internal/web's effectiveUpcoming computes their
// effective value from PlayoffMatchups instead, per Story 3.2); for every
// other id it remains authoritative exactly as before.
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

	// KindPlayoffsCup is Story 3.1's second, independently-stored Cup pick,
	// made once the playoff field is set - a distinct (PlayerID, Kind) row
	// from KindCupChampion's, so neither ever overwrites the other.
	KindPlayoffsCup = "playoffcup"

	// KindDivisionPlayoffTeams and KindDivisionWinner are Story 2.4's two
	// division-scoped Kind values: unlike KindCupChampion/
	// KindPresidentsTrophy, neither matches a Prediction Set id directly
	// (both belong to the single "divisions" set) - a row of either kind is
	// instead keyed by (PlayerID, Kind, Division), never by Kind alone.
	KindDivisionPlayoffTeams = "division_playoff_teams"
	KindDivisionWinner       = "division_winner"

	// KindAward is Story 2.6's award-scoped Kind value: like
	// KindDivisionPlayoffTeams/KindDivisionWinner, it doesn't match a
	// Prediction Set id directly (all five awards belong to the single
	// "awards" set) - a row of this kind is instead keyed by (PlayerID,
	// Kind, Award), never by Kind alone.
	KindAward = "award"

	// KindSeries is Story 3.3's per-series Kind value, shared by all four
	// round Prediction Sets (Round1SetID/Round2SetID/ConferenceFinalsSetID/
	// StanleyCupFinalSetID) - like KindAward, it doesn't match any one of
	// those Prediction Set ids directly. A row of
	// this kind is instead keyed by (PlayerID, Kind, SeriesKey), never by
	// (PlayerID, Kind) alone - SeriesKey itself already encodes which
	// Prediction Set the row belongs to.
	KindSeries = "series"
)

// Kinds lists every Prediction Kind value, so observe can pre-register one
// save series per kind (AD-36).
var Kinds = []string{
	KindCupChampion,
	KindPresidentsTrophy,
	KindPlayoffsCup,
	KindDivisionPlayoffTeams,
	KindDivisionWinner,
	KindAward,
	KindSeries,
}

// Award values, matching the PRD's own five individual-award names
// (FR-17/AD-28) - the fixed key every KindAward Prediction row is scoped by,
// mirroring Division's own scoping of KindDivisionPlayoffTeams/
// KindDivisionWinner rows.
const (
	AwardHart          = "hart"
	AwardNorris        = "norris"
	AwardVezina        = "vezina"
	AwardArtRoss       = "art_ross"
	AwardRocketRichard = "rocket_richard"
)

// Playoff round Prediction Set ids, matching fantasy-hockey.yml's
// prediction_sets[].id values for the four playoff rounds - the Prediction
// Sets every KindSeries row belongs to (Story 3.3), and the first half of
// every SeriesKey (see JoinSeriesKey).
const (
	// Round1SetID is the first playoff round's Prediction Set id.
	Round1SetID = "r1"
	// Round2SetID is the second playoff round's Prediction Set id.
	Round2SetID = "r2"
	// ConferenceFinalsSetID is the conference finals' Prediction Set id.
	ConferenceFinalsSetID = "cf"
	// StanleyCupFinalSetID is the Stanley Cup Final's Prediction Set id.
	StanleyCupFinalSetID = "scf"
)

// seriesKeySeparator joins a Prediction Set id (e.g. "r1") and a
// PlayoffMatchup's own hand-maintained Key (e.g. "s1") into one
// Prediction.SeriesKey (e.g. "r1.s1") - the one separator every
// JoinSeriesKey/SplitSeriesKey call uses, so the persisted series_key
// format can't drift (magic-value rule).
const seriesKeySeparator = "."

// JoinSeriesKey builds the full SeriesKey a KindSeries Prediction row is
// scoped by, from setID (the Prediction Set id, e.g. Round1SetID) and key (a
// PlayoffMatchup's own hand-maintained Key): "<setID>.<key>", e.g. "r1.s1".
func JoinSeriesKey(setID, key string) string {
	return setID + seriesKeySeparator + key
}

// SplitSeriesKey reverses JoinSeriesKey, returning the Prediction Set id and
// matchup key it was built from. ok is false when seriesKey doesn't contain
// the separator at all - never produced by JoinSeriesKey itself, but guards
// a caller against a hand-edited or otherwise malformed row.
func SplitSeriesKey(seriesKey string) (setID, key string, ok bool) {
	return strings.Cut(seriesKey, seriesKeySeparator)
}

// divisions is the fixed division vocabulary and display order behind
// Divisions - kept unexported so no importer can mutate the shared slice.
var divisions = []string{"Atlantic", "Metropolitan", "Central", "Pacific"}

// Divisions returns the four Team.Division values in their fixed display
// order (Atlantic, Metropolitan, Central, Pacific) - the one vocabulary
// every division-scoped Prediction row (KindDivisionPlayoffTeams/
// KindDivisionWinner) and every division grouping is keyed by. Each call
// returns a fresh copy, so a caller editing it never affects the next call.
func Divisions() []string {
	return slices.Clone(divisions)
}

// Prediction is one player's saved pick for one Prediction Set kind (e.g.
// their Stanley Cup champion pick). It's an application-created row (AD-17):
// FindPrediction/SavePrediction are the only ways to read or write a
// KindCupChampion/KindPresidentsTrophy row, and a resubmission updates the
// existing (PlayerID, Kind) row in place rather than appending a duplicate
// (FR-9). TeamID is a Team.ID (e.g. "TOR"), the value scoring later compares
// on. SubmittedAt is RFC3339 in UTC, matching PredictionSet.DeadlineUTC's
// convention. Division is set on both a KindDivisionPlayoffTeams and a
// KindDivisionWinner row (FindDivisionPlayoffTeams/FindDivisionWinner/
// SaveDivisionPicks); TeamIDs is set only on a KindDivisionPlayoffTeams row -
// a KindDivisionWinner row instead uses TeamID, the same field the cup/
// presidents rows use. Division and TeamIDs both stay zero-valued/omitted on
// every cup/presidents row, where a row is keyed by (PlayerID, Kind) alone.
// Award and FinalistSlugs are set only on a KindAward row
// (FindAwardFinalists/SaveAwardPicks): Award is one of the AwardHart/
// AwardNorris/AwardVezina/AwardArtRoss/AwardRocketRichard keys, and
// FinalistSlugs holds exactly 3 ordered NHL Player (AwardFinalist) slugs -
// SaveAwardPicks never upserts a row with fewer. SeriesKey and Games are set
// only on a KindSeries row (FindSeriesPick/SaveSeriesPick): SeriesKey is the
// full key a PlayoffMatchup is scoped by, built by JoinSeriesKey from the
// owning Prediction Set id and the matchup's own hand-maintained Key - and
// Games is one of "4"/"5"/"6"/"7", kept a string for consistency with
// every other Prediction field even though it's numeric. All of Division/
// TeamIDs/Award/FinalistSlugs/SeriesKey/Games stay zero-valued/omitted on
// every row they don't apply to.
type Prediction struct {
	ID            string   `yaml:"id"`
	PlayerID      string   `yaml:"player_id"`
	Kind          string   `yaml:"kind"`
	TeamID        string   `yaml:"team_id"`
	SubmittedAt   string   `yaml:"submitted_at"`
	Division      string   `yaml:"division,omitempty"`
	TeamIDs       []string `yaml:"team_ids,omitempty"`
	Award         string   `yaml:"award,omitempty"`
	FinalistSlugs []string `yaml:"finalist_slugs,omitempty"`
	SeriesKey     string   `yaml:"series_key,omitempty"`
	Games         string   `yaml:"games,omitempty"`
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

// Position values for an AwardFinalist, matching the PRD's own wording
// (FR-33/AD-24). Like Prediction.Kind, Position stays a plain string
// constant rather than a dedicated type - this codebase never validates
// hand-maintained enum-like fields (a hand-edited value outside these three
// is simply never returned by NHLPlayersByPosition, never rejected).
const (
	PositionSkater     = "skater"
	PositionDefenseman = "defenseman"
	PositionGoalie     = "goalie"
)

// AwardFinalist is one NHL player eligible as a Story 2.6 award-finalist
// pick (Hart/Art Ross/Rocket Richard need skaters, Norris needs
// defensemen, Vezina needs goalies). Like Team, the list is hand-maintained
// directly in fantasy-hockey.yml's nhl_players: section; internal/store
// never writes it (AD-23). Slug is the value every downstream reader (a
// saved Prediction's finalist pick, the embed's submitted id) ever compares
// on, never DisplayName (AD-17) - hand-picked once by the maintainer and
// never regenerated.
type AwardFinalist struct {
	Slug        string `yaml:"slug"`
	DisplayName string `yaml:"display_name"`
	Position    string `yaml:"position,omitempty"`
}

// PlayoffMatchup is one recorded playoff series matchup, keyed by Prediction
// Set id (e.g. Round2SetID, ConferenceFinalsSetID, StanleyCupFinalSetID)
// under fantasy-hockey.yml's
// playoff_matchups: section. Like Team and AwardFinalist, it's
// hand-maintained directly in the data file; internal/store never writes it
// (AD-23). Its presence for a round-gated Prediction Set id is what
// internal/web's effectiveUpcoming treats as "that round's matchups are
// known" (Story 3.2, FR-20) - TeamA/TeamB's values themselves are never read
// by that story, only whether at least one matchup entry exists for the id.
// Key is Story 3.3's hand-maintained per-series identity (e.g. "s1") - a
// stable, human-chosen id rather than one derived from the entry's position
// in the list, so a human reordering playoff_matchups by hand can never
// silently reassign an already-saved series pick to a different matchup. It
// is joined with the Prediction Set id to form a KindSeries Prediction row's
// full SeriesKey.
type PlayoffMatchup struct {
	Key   string `yaml:"key"`
	TeamA string `yaml:"a"`
	TeamB string `yaml:"b"`
}

// document mirrors fantasy-hockey.yml's on-disk shape (see also
// writeLocked's own doc comment and README.md's "Design notes" - all three
// describe the same splice list; a third spliced field needs updating
// wherever this list is written down). Only LoginCodes and Predictions are
// ever re-derived from s.doc on a write - writeLocked splices a cleaned
// copy of LoginCodes (cleanupLoginCodes' pruned subset, not a verbatim
// echo) and s.doc.Predictions into s.raw's own parsed node tree and
// encodes that (store_splice.go's spliceNamedValueLocked, spec-7-4). Every
// other field here is read once at load (or, for a brand-new file, never
// read at all - simply zero-valued before the first encode) and, from
// then on, exists in s.doc only as a read-side convenience: mutating it
// has no effect on disk.
//
// A future app-writable field follows the existing SavePrediction pattern
// (mutate s.doc, call writeLocked), but that alone silently fails to
// persist: it also needs (1) its own spliceNamedValueLocked call inside
// writeLocked, and (2) that call's restore closure wired into
// restoreSplices in the correct position, or a write failure for the new
// field leaves s.raw mutated instead of rolled back. Skipping either one
// compiles, mutates s.doc, and reports success while the change never
// reaches the file (epic-7 retrospective, item 51). Add test coverage for
// both the splice and its rollback, mirroring store_splice_test.go's
// existing coverage of the two current splices.
type document struct {
	Season          string                      `yaml:"season"`
	Players         []Player                    `yaml:"players"`
	LoginCodes      []LoginCode                 `yaml:"login_codes"`
	PredictionSets  []PredictionSet             `yaml:"prediction_sets"`
	Teams           []Team                      `yaml:"teams"`
	NHLPlayers      []AwardFinalist             `yaml:"nhl_players"`
	PlayoffMatchups map[string][]PlayoffMatchup `yaml:"playoff_matchups"`
	Predictions     []Prediction                `yaml:"predictions"`
	Results         results                     `yaml:"results,omitempty"`
	AwardFinalists  map[string][]AwardFinalist  `yaml:"award_finalists,omitempty"`
}

// Store is the in-memory representation of fantasy-hockey.yml, guarded by a
// mutex so every read and write is synchronized (AD-29). The mutex and both
// representations of the document stay unexported; callers only ever go
// through Store's exported methods.
type Store struct {
	mu   sync.RWMutex
	path string
	doc  document

	// raw is the same document as doc, parsed as a *yaml.Node tree instead
	// of typed Go values. writeLocked splices freshly-encoded login_codes/
	// predictions nodes into this same tree and encodes that (spec-7-4) -
	// every other hand-maintained section (players, prediction_sets, teams,
	// nhl_players, playoff_matchups, results, award_finalists) round-trips
	// through the exact node objects it was parsed into, so a save never
	// reconstructs their content from doc's typed fields. New populates it
	// for both an existing file and a freshly bootstrapped one, so every
	// write from then on has a tree to splice into.
	raw *yaml.Node

	// toleratedShapeErrors holds the *yaml.TypeError messages New tolerated
	// because every line they name fell inside results:'s or
	// award_finalists:'s own line range (AC2) instead of aborting startup.
	// ResultProblems formats and appends them alongside every other
	// per-problem warning main.go already logs.
	toleratedShapeErrors []string
}

// New loads path into memory. If path doesn't exist, it bootstraps a new
// file there with an empty players list and the current default season,
// per AD-25/AD-26 - logging one distinct slog.Info line naming path and
// season (writeLocked's own doc comment; epic-6-retro-item-43,
// epic-7-retro-item-56), unlike every other write, which logs only reason.
//
// A shape error confined to results:/award_finalists: (e.g. a scalar where
// a list belongs) no longer aborts startup: New keeps going with those
// fields left zero-valued, and ResultProblems reports the mistake instead
// (AC2). A shape error anywhere else (players:, teams:, etc.) still fails
// New exactly as before.
func New(path string) (*Store, error) {
	st := &Store{path: path}

	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		st.doc = document{Season: DefaultSeason, Players: []Player{}}
		st.raw = &yaml.Node{}
		if err := st.raw.Encode(st.doc); err != nil {
			return nil, fmt.Errorf("store: bootstrap %s: %w", path, err)
		}
		if err := st.writeLocked("bootstrap data file", time.Now().UTC(), "path", path, "season", st.doc.Season); err != nil {
			return nil, fmt.Errorf("store: bootstrap %s: %w", path, err)
		}
		return st, nil
	case err != nil:
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	}

	if err := st.loadExisting(path, raw); err != nil {
		return nil, err
	}
	return st, nil
}

// loadExisting parses raw (path's current on-disk bytes) into st.doc and
// st.raw, tolerating a *yaml.TypeError confined to results:/award_finalists:
// (AC2) and an empty/whitespace/comment-only file (Kind 0, which
// writeLocked's encoder can't encode later) - split out of New to keep its
// own cyclomatic complexity within gocyclo's gate.
func (st *Store) loadExisting(path string, raw []byte) error {
	var rawNode yaml.Node
	if err := yaml.Unmarshal(raw, &rawNode); err != nil {
		return fmt.Errorf("store: parse %s: %w", path, err)
	}
	st.raw = &rawNode

	if err := yaml.Unmarshal(raw, &st.doc); err != nil {
		var typeErr *yaml.TypeError
		if !errors.As(err, &typeErr) || !typeErrorConfinedToLenientSections(typeErr, st.raw) {
			return fmt.Errorf("store: parse %s: %w", path, err)
		}
		// st.doc is still partially populated (Design Notes) - every field
		// an ordinary type mismatch didn't touch, including sibling fields
		// in the same struct, is exactly as if this error never happened.
		// A duplicate-key error is different (epic-7 retro finding,
		// confirmed empirically): yaml.Unmarshal abandons every field at
		// the SAME mapping level as the duplicate, not just the duplicated
		// key itself, so an unrelated sibling (e.g. presidents_trophy next
		// to a duplicated team_marks) silently zeroes too. Tag each message
		// with the specific section it came from (review finding), rather
		// than a generic "results/award_finalists" label for every message
		// regardless of which one actually had the problem, and append a
		// note when the message is a duplicate-key error so the operator
		// knows to check nearby fields too.
		ranges := lenientRanges(st.raw)
		for _, msg := range typeErr.Errors {
			line, _ := parseErrorLine(msg) // already validated above
			formatted := sectionForLine(line, ranges) + ": " + msg
			if isDuplicateKeyError(msg) {
				formatted += " (other fields in this section may also be unset)"
			}
			st.toleratedShapeErrors = append(st.toleratedShapeErrors, formatted)
		}
	}

	// An empty/whitespace/comment-only file parses to a Node with no
	// content at all (Kind 0); a file whose only content is a bare scalar
	// (e.g. a lone "null") parses to a real but non-mapping top-level node.
	// Both are "successfully loaded" as far as yaml.Unmarshal is concerned
	// (a null document leaves st.doc at its zero value, no error), but
	// neither gives writeLocked's splice a mapping to splice into -
	// reproduced empirically: without this check, New succeeds, the next
	// write ALSO succeeds silently, and the file is left containing just
	// "null" - login_codes/predictions written that same call are silently
	// discarded, not merely rejected. Rebuild raw from the (here,
	// zero-valued) typed doc whenever the top level isn't a real mapping,
	// matching New's bootstrap branch, so the next write always has a
	// mapping to splice into instead of silently losing data.
	if mapping := topLevelMapping(st.raw); mapping == nil || mapping.Kind != yaml.MappingNode {
		if err := st.raw.Encode(st.doc); err != nil {
			return fmt.Errorf("store: parse %s: %w", path, err)
		}
	}
	return nil
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

// NHLPlayers returns every one of the season's canonical NHL Players
// (AwardFinalist), as hand-maintained in fantasy-hockey.yml (Teams's own
// read-only pattern: no write method exists, since this story only ever
// reads the list). The returned slice is a copy, so a caller mutating it
// can't reach back into the store's own state.
func (s *Store) NHLPlayers() []AwardFinalist {
	s.mu.RLock()
	defer s.mu.RUnlock()

	players := make([]AwardFinalist, len(s.doc.NHLPlayers))
	copy(players, s.doc.NHLPlayers)
	return players
}

// NHLPlayersByPosition returns only the NHL Players whose Position matches,
// preserving the seeded order - the one parameterized filter Story 2.6's
// award-finalist autocomplete scopes its skater/defenseman/goalie
// suggestions with, rather than three separate per-position methods.
func (s *Store) NHLPlayersByPosition(position string) []AwardFinalist {
	s.mu.RLock()
	defer s.mu.RUnlock()

	players := make([]AwardFinalist, 0, len(s.doc.NHLPlayers))
	for _, p := range s.doc.NHLPlayers {
		if p.Position == position {
			players = append(players, p)
		}
	}
	return players
}

// PlayoffMatchups returns every recorded PlayoffMatchup for setID, as
// hand-maintained in fantasy-hockey.yml's playoff_matchups: section (Teams's
// own read-only pattern: no write method exists, since this story only ever
// reads the list). A setID with no key present, or one whose list is empty,
// both return an empty slice, never nil - Story 3.2's effectiveUpcoming only
// cares about the length. The returned slice is a copy, so a caller mutating
// it can't reach back into the store's own state.
func (s *Store) PlayoffMatchups(setID string) []PlayoffMatchup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matchups := make([]PlayoffMatchup, len(s.doc.PlayoffMatchups[setID]))
	copy(matchups, s.doc.PlayoffMatchups[setID])
	return matchups
}

// CreateLoginCode appends a new LoginCode row for playerID and persists it.
// It never mutates any existing row itself, though the write it triggers may
// still prune other rows that are already expired or used (writeLocked's
// opportunistic cleanup, spec-7-2). now is used both as the row's own
// issued_at (RFC3339, UTC) and as the write's cleanup instant. If the write
// fails, the appended row is rolled back from memory so a caller told the
// write failed can't later have that row silently persisted by an unrelated
// successful write.
func (s *Store) CreateLoginCode(playerID, codeHash string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	issuedAt := now.UTC().Format(time.RFC3339)
	s.doc.LoginCodes = append(s.doc.LoginCodes, LoginCode{
		ID:       uuid.NewString(),
		PlayerID: playerID,
		CodeHash: codeHash,
		IssuedAt: issuedAt,
	})

	if err := s.writeLocked("create login code", now); err != nil {
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

		if err := s.writeLocked("consume login code", now); err != nil {
			row.UsedAt = nil
			return "", false, fmt.Errorf("store: persist consumed login code: %w", err)
		}
		// row still points into the pre-write backing array, which
		// writeLocked's successful cleanup replaces (s.doc.LoginCodes =
		// cleaned) but never mutates in place - cleanupLoginCodes always
		// builds cleaned as a fresh copy, so this read stays valid. A future
		// cleanupLoginCodes rewritten to compact rows.doc.LoginCodes in
		// place instead of copying would silently break this.
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
// by an unrelated successful write. It returns the one persisted row (a copy),
// or nil when the write failed (AD-35).
func (s *Store) SavePrediction(playerID, kind, teamID string, now time.Time) ([]Prediction, error) {
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

		if err := s.writeLocked("save prediction", now); err != nil {
			row.TeamID, row.SubmittedAt = oldTeamID, oldSubmittedAt
			return nil, fmt.Errorf("store: persist prediction: %w", err)
		}
		return []Prediction{*row}, nil
	}

	s.doc.Predictions = append(s.doc.Predictions, Prediction{
		ID:          uuid.NewString(),
		PlayerID:    playerID,
		Kind:        kind,
		TeamID:      teamID,
		SubmittedAt: submittedAt,
	})

	if err := s.writeLocked("save prediction", now); err != nil {
		s.doc.Predictions = s.doc.Predictions[:len(s.doc.Predictions)-1]
		return nil, fmt.Errorf("store: persist prediction: %w", err)
	}
	return []Prediction{s.doc.Predictions[len(s.doc.Predictions)-1]}, nil
}

// FindSeriesPick returns playerID's saved winner-and-games pick for
// seriesKey, if any (Kind == KindSeries) - a row is matched by (PlayerID,
// Kind, SeriesKey), mirroring findDivisionPrediction's own extra-key match.
func (s *Store) FindSeriesPick(playerID, seriesKey string) (Prediction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.doc.Predictions {
		if p.PlayerID == playerID && p.Kind == KindSeries && p.SeriesKey == seriesKey {
			return p, true
		}
	}
	return Prediction{}, false
}

// SaveSeriesPick saves playerID's winner (teamID) and game-count (games)
// pick for seriesKey together as one row, persisting the change -
// SavePrediction's own single-row upsert-by-(PlayerID, Kind) pattern,
// generalized to the extra SeriesKey every KindSeries row is scoped by. An
// existing (playerID, KindSeries, seriesKey) row is updated in place - a
// resubmission never appends a duplicate - otherwise a new row is appended
// with a generated id. Store does not validate teamID/games itself
// (internal/web re-validates both server-side before ever calling this,
// matching every other pick kind). If the write fails, the change is rolled
// back from memory so a caller told the write failed can't later have it
// silently persisted by an unrelated successful write. It returns the one
// persisted row (a copy), or nil when the write failed (AD-35).
func (s *Store) SaveSeriesPick(playerID, seriesKey, teamID, games string, now time.Time) ([]Prediction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	submittedAt := now.UTC().Format(time.RFC3339)

	for i := range s.doc.Predictions {
		row := &s.doc.Predictions[i]
		if row.PlayerID != playerID || row.Kind != KindSeries || row.SeriesKey != seriesKey {
			continue
		}

		oldTeamID, oldGames, oldSubmittedAt := row.TeamID, row.Games, row.SubmittedAt
		row.TeamID = teamID
		row.Games = games
		row.SubmittedAt = submittedAt

		if err := s.writeLocked("save series pick", now); err != nil {
			row.TeamID, row.Games, row.SubmittedAt = oldTeamID, oldGames, oldSubmittedAt
			return nil, fmt.Errorf("store: persist series pick: %w", err)
		}
		return []Prediction{*row}, nil
	}

	s.doc.Predictions = append(s.doc.Predictions, Prediction{
		ID:          uuid.NewString(),
		PlayerID:    playerID,
		Kind:        KindSeries,
		SeriesKey:   seriesKey,
		TeamID:      teamID,
		Games:       games,
		SubmittedAt: submittedAt,
	})

	if err := s.writeLocked("save series pick", now); err != nil {
		s.doc.Predictions = s.doc.Predictions[:len(s.doc.Predictions)-1]
		return nil, fmt.Errorf("store: persist series pick: %w", err)
	}
	return []Prediction{s.doc.Predictions[len(s.doc.Predictions)-1]}, nil
}

// FindDivisionPlayoffTeams returns playerID's saved playoff-teams pick for
// division, if any (Kind == KindDivisionPlayoffTeams).
func (s *Store) FindDivisionPlayoffTeams(playerID, division string) (Prediction, bool) {
	return s.findDivisionPrediction(playerID, KindDivisionPlayoffTeams, division)
}

// FindDivisionWinner returns playerID's saved division-winner pick for
// division, if any (Kind == KindDivisionWinner).
func (s *Store) FindDivisionWinner(playerID, division string) (Prediction, bool) {
	return s.findDivisionPrediction(playerID, KindDivisionWinner, division)
}

// findDivisionPrediction returns playerID's row matching kind and division,
// if any - shared by FindDivisionPlayoffTeams/FindDivisionWinner so the
// (PlayerID, Kind, Division) match can't drift between the two.
func (s *Store) findDivisionPrediction(playerID, kind, division string) (Prediction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.doc.Predictions {
		if p.PlayerID == playerID && p.Kind == kind && p.Division == division {
			return p, true
		}
	}
	return Prediction{}, false
}

// SaveDivisionPicks saves every division present in playoffTeams and winners
// for playerID, persisting all of it through exactly one atomic write
// (epic-2-context.md's "a save triggers exactly one atomic write-and-
// rename"). Each division present in playoffTeams gets one
// KindDivisionPlayoffTeams row carrying that division's TeamIDs, whether or
// not the slice itself is empty - a division genuinely absent from the map
// gets no upsert at all. Each division present in winners with a non-empty
// TeamID gets one KindDivisionWinner row; an empty or absent winner leaves
// that division's winner row untouched rather than force-creating or
// clearing it (FR-11 - no row is force-created for an empty pick). An
// existing (playerID, Kind, Division) row is updated in place, mirroring
// SavePrediction's own upsert shape, generalized to a batch. If the single
// write fails, every row touched by this call is rolled back by restoring a
// snapshot of the whole Predictions slice taken before any mutation - a
// whole-document snapshot rather than per-row pointer restoration (which
// SavePrediction uses for its one row), since holding row pointers across
// this call's own interleaved appends could invalidate them once the
// underlying slice reallocates. It returns the rows it persisted that carry a
// pick (a playoff-teams row with an empty team list is written but not
// returned), in no particular order; nil when the write failed (AD-35).
func (s *Store) SaveDivisionPicks(playerID string, playoffTeams map[string][]string, winners map[string]string, now time.Time) ([]Prediction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := make([]Prediction, len(s.doc.Predictions))
	copy(snapshot, s.doc.Predictions)

	submittedAt := now.UTC().Format(time.RFC3339)

	var saved []Prediction
	for division, teamIDs := range playoffTeams {
		row := s.upsertDivisionPredictionLocked(playerID, KindDivisionPlayoffTeams, division, teamIDs, "", submittedAt)
		if len(teamIDs) > 0 {
			saved = append(saved, row)
		}
	}
	for division, teamID := range winners {
		if teamID == "" {
			continue
		}
		saved = append(saved, s.upsertDivisionPredictionLocked(playerID, KindDivisionWinner, division, nil, teamID, submittedAt))
	}

	if err := s.writeLocked("save division picks", now); err != nil {
		s.doc.Predictions = snapshot
		return nil, fmt.Errorf("store: persist division picks: %w", err)
	}
	return saved, nil
}

// upsertDivisionPredictionLocked updates the existing (playerID, kind,
// division) row's TeamIDs/TeamID/SubmittedAt in place, or appends a new row
// with a generated id - shared by SaveDivisionPicks' two upsert loops. It
// returns a copy of the row as persisted in memory.
// Callers must hold s.mu for writing.
func (s *Store) upsertDivisionPredictionLocked(playerID, kind, division string, teamIDs []string, teamID, submittedAt string) Prediction {
	for i := range s.doc.Predictions {
		row := &s.doc.Predictions[i]
		if row.PlayerID == playerID && row.Kind == kind && row.Division == division {
			row.TeamIDs = teamIDs
			row.TeamID = teamID
			row.SubmittedAt = submittedAt
			return *row
		}
	}

	row := Prediction{
		ID:          uuid.NewString(),
		PlayerID:    playerID,
		Kind:        kind,
		Division:    division,
		TeamIDs:     teamIDs,
		TeamID:      teamID,
		SubmittedAt: submittedAt,
	}
	s.doc.Predictions = append(s.doc.Predictions, row)
	return row
}

// AwardFinalistCount is the exact number of finalist slugs a KindAward row
// must carry (PRD FR-17/AD-28's "3 finalists per award") - SaveAwardPicks
// never upserts a row with fewer. Exported so internal/web references this
// one home instead of declaring its own copy of the same PRD-fixed number.
const AwardFinalistCount = 3

// FindAwardFinalists returns playerID's saved finalist-trio pick for award,
// if any (Kind == KindAward), mirroring FindDivisionPlayoffTeams/
// FindDivisionWinner's own (PlayerID, Kind, scoping-key) match.
func (s *Store) FindAwardFinalists(playerID, award string) (Prediction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.doc.Predictions {
		if p.PlayerID == playerID && p.Kind == KindAward && p.Award == award {
			return p, true
		}
	}
	return Prediction{}, false
}

// awardHasAllFinalistSlugs reports whether slugs holds exactly
// AwardFinalistCount non-empty entries - SaveAwardPicks' own gate for
// upserting an award's row at all (never a force-created partial row).
func awardHasAllFinalistSlugs(slugs []string) bool {
	if len(slugs) != AwardFinalistCount {
		return false
	}
	for _, slug := range slugs {
		if slug == "" {
			return false
		}
	}
	return true
}

// SaveAwardPicks saves every award present in finalists whose slice holds
// all AwardFinalistCount non-empty slugs, persisting all of it through
// exactly one atomic write (SaveDivisionPicks' own batched-write
// precedent). An award present with fewer than AwardFinalistCount non-empty
// slugs gets no upsert at all - a 1-or-2-filled award is treated
// identically to a fully-blank one (FR-11, AD-28's framing of the row as
// "the trio," not three independent slots). An existing (playerID,
// KindAward, Award) row is updated in place, mirroring
// upsertDivisionPredictionLocked's own upsert shape. If the single write
// fails, every row touched by this call is rolled back by restoring a
// whole-document snapshot taken before any mutation, the same rollback
// shape SaveDivisionPicks uses for the same reason (this call's own
// interleaved appends could invalidate held row pointers once the
// underlying slice reallocates). It returns the rows it persisted, in no
// particular order; nil when nothing qualified or the write failed (AD-35).
func (s *Store) SaveAwardPicks(playerID string, finalists map[string][]string, now time.Time) ([]Prediction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := make([]Prediction, len(s.doc.Predictions))
	copy(snapshot, s.doc.Predictions)

	submittedAt := now.UTC().Format(time.RFC3339)

	var saved []Prediction
	for award, slugs := range finalists {
		if !awardHasAllFinalistSlugs(slugs) {
			continue
		}
		saved = append(saved, s.upsertAwardPredictionLocked(playerID, award, slugs, submittedAt))
	}

	if err := s.writeLocked("save award picks", now); err != nil {
		s.doc.Predictions = snapshot
		return nil, fmt.Errorf("store: persist award picks: %w", err)
	}
	return saved, nil
}

// upsertAwardPredictionLocked updates the existing (playerID, KindAward,
// award) row's FinalistSlugs/SubmittedAt in place, or appends a new row
// with a generated id - shared by SaveAwardPicks' own upsert loop, mirroring
// upsertDivisionPredictionLocked. Callers must hold s.mu for writing.
func (s *Store) upsertAwardPredictionLocked(playerID, award string, slugs []string, submittedAt string) Prediction {
	finalistSlugs := append([]string(nil), slugs...) // own copy: never alias the caller's slice.

	for i := range s.doc.Predictions {
		row := &s.doc.Predictions[i]
		if row.PlayerID == playerID && row.Kind == KindAward && row.Award == award {
			row.FinalistSlugs = finalistSlugs
			row.SubmittedAt = submittedAt
			return *row
		}
	}

	row := Prediction{
		ID:            uuid.NewString(),
		PlayerID:      playerID,
		Kind:          KindAward,
		Award:         award,
		FinalistSlugs: finalistSlugs,
		SubmittedAt:   submittedAt,
	}
	s.doc.Predictions = append(s.doc.Predictions, row)
	return row
}

// cleanupLoginCodes returns the subset of rows that are still eligible to be
// consumed as of now: a row is dropped when it has already been used
// (UsedAt != nil), or when its IssuedAt parses and is either more than
// loginCodeValidity in now's past or after now (a future-dated row from
// clock skew - ConsumeLoginCode always rejects one of these too, so keeping
// it forever would contradict this story's own goal). A row whose IssuedAt
// fails to parse is always kept - the code can't confirm it's expired, so it
// never destroys that data (spec-7-2's Boundaries & Constraints). Order of
// kept rows is preserved. ConsumeLoginCode's own inline scan checks the same
// three conditions (used, expired, future-dated) plus a CodeHash match; the
// two are intentionally not shared implementations, since matching-and-
// consuming one specific row is a different job from pruning every row.
func cleanupLoginCodes(rows []LoginCode, now time.Time) []LoginCode {
	cleaned := make([]LoginCode, 0, len(rows))
	for _, row := range rows {
		if row.UsedAt != nil {
			continue
		}
		if issuedAt, err := time.Parse(time.RFC3339, row.IssuedAt); err == nil && (issuedAt.After(now) || now.Sub(issuedAt) > loginCodeValidity) {
			continue
		}
		cleaned = append(cleaned, row)
	}
	return cleaned
}

// writeLocked atomically replaces the file on disk by writing to a
// temporary file in the same directory and renaming it over the original
// (AD-27). Before encoding, it computes cleanupLoginCodes(s.doc.LoginCodes,
// now) and encodes that cleaned slice - a fresh node, never s.doc.LoginCodes
// itself - so a failed write leaves s.doc.LoginCodes exactly as the calling
// method's own mutation left it (its existing rollback logic stays correct,
// untouched - spec-7-2's Design Notes). It splices that node and a fresh
// s.doc.Predictions node into s.raw's own top-level mapping and encodes
// s.raw itself - every other hand-maintained section (players,
// prediction_sets, teams, nhl_players, playoff_matchups, results,
// award_finalists) round-trips through the exact node objects it was parsed
// into, never reconstructed from doc's typed fields (spec-7-4, AD-23) -
// document's own doc comment states what a new app-writable field needs
// beyond a struct tag to actually reach disk. Only
// once the rename succeeds are s.doc.LoginCodes and the two spliced nodes
// committed permanently; on any failure both splices are undone first, so
// s.raw is left exactly as it was before this call. On success it also logs
// exactly one slog.Info line naming reason, the caller's own literal label
// for why this write happened (e.g. "create login code") - the sole place
// any Store write is logged, so every mutating method's call site stays a
// one-line addition instead of duplicating a log call at every site
// (spec-7-1's Design Notes). reason is always a fixed, non-identifying
// literal, never a login code or player email. extra is appended to the
// logged fields as-is, key-value pairs same as slog.Info's own variadic
// args - only New's bootstrap branch passes any (path, season -
// epic-6-retro-item-43, epic-7-retro-item-56), every other call site
// passes none, keeping their log line exactly as before. Like reason,
// nothing passed through extra may be a raw login code, player email, or
// other identifying/secret value - the same hash-code-never-logged rule
// applies. Callers must hold s.mu for writing.
func (s *Store) writeLocked(reason string, now time.Time, extra ...any) error {
	cleaned := cleanupLoginCodes(s.doc.LoginCodes, now)

	var loginCodesNode, predictionsNode yaml.Node
	// Encode only fails for a type yaml can't represent at all - never the
	// case for []LoginCode/[]Prediction - so these branches are defensive
	// and intentionally untested (spec-7-3's I/O matrix).
	if err := loginCodesNode.Encode(cleaned); err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := predictionsNode.Encode(s.doc.Predictions); err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	mapping := topLevelMapping(s.raw)
	restoreLoginCodes := spliceNamedValueLocked(mapping, "login_codes", &loginCodesNode)
	restorePredictions := spliceNamedValueLocked(mapping, "predictions", &predictionsNode)
	restoreSplices := func() {
		// Undo in reverse order: each restore closure trims from
		// mapping.Content's current end when it originally appended,
		// so undoing the later splice first keeps that trim correct.
		restorePredictions()
		restoreLoginCodes()
	}

	// A plain yaml.Marshal indents a sequence nested inside a mapping
	// that's itself inside a list (e.g. Prediction.TeamIDs) by a smaller
	// increment than a top-level sequence gets, which yamllint's
	// indentation rule (extends: default) rejects as inconsistent.
	// CompactSeqIndent applies the same relative increment to every
	// sequence regardless of nesting depth, producing yamllint-compliant
	// output (spec-7-3's Design Notes).
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.CompactSeqIndent()
	// Encode/Close only fail if the emitter's own write handler errors
	// (impossible: &buf is a *bytes.Buffer, whose Write always returns a
	// nil error), if the emitter's internal event-ordering invariants are
	// violated (impossible: Encode always drives it through its own single,
	// correctly-ordered document-start/content/document-end sequence, never
	// a hand-built or externally-driven event stream), or - Encode only,
	// since Close never re-traverses content - if s.raw itself holds a
	// value the emitter can't serialize: invalid UTF-8 scalar data, a
	// Node.Kind it doesn't recognize, or badly-tagged !!binary data. s.raw
	// is always one of three things here: freshly parsed from valid YAML
	// (loadExisting), built by Node.Encode(st.doc) at bootstrap (New), or
	// has just had the two splices above written into it - in every case
	// already proven encodable by an earlier successful Encode() call. Both
	// branches are defensive and intentionally untested, like the two
	// typed-slice Encode calls above (spec-7-3's I/O matrix).
	if err := enc.Encode(s.raw); err != nil {
		restoreSplices()
		return fmt.Errorf("marshal: %w", err)
	}
	if err := enc.Close(); err != nil {
		restoreSplices()
		return fmt.Errorf("marshal: %w", err)
	}
	out := buf.Bytes()

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".fantasy-hockey-*.tmp")
	if err != nil {
		restoreSplices()
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		restoreSplices()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		restoreSplices()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		restoreSplices()
		return fmt.Errorf("rename temp file: %w", err)
	}

	s.doc.LoginCodes = cleaned
	slog.Info("store write", append([]any{"reason", reason}, extra...)...)
	return nil
}

// Players returns every hand-maintained player. The returned slice is a
// copy, so a caller mutating it can't reach back into the store's state.
func (s *Store) Players() []Player {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return slices.Clone(s.doc.Players)
}

// PredictionsForPlayer returns every Prediction row saved by playerID, of
// every kind. Each row's TeamIDs/FinalistSlugs are copied too, so a caller
// mutating the result can't reach back into the store's state.
func (s *Store) PredictionsForPlayer(playerID string) []Prediction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []Prediction
	for _, p := range s.doc.Predictions {
		if p.PlayerID != playerID {
			continue
		}
		p.TeamIDs = slices.Clone(p.TeamIDs)
		p.FinalistSlugs = slices.Clone(p.FinalistSlugs)
		rows = append(rows, p)
	}
	return rows
}
