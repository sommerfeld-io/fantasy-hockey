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
	"regexp"
	"slices"
	"sort"
	"strconv"
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
// the file afterward to add real players.
const DefaultSeason = "2026-27"

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

// results mirrors fantasy-hockey.yml's hand-maintained results: section -
// the real-world outcomes internal/scoring compares picks against. Like
// Team and PlayoffMatchup, no code path ever changes it; every save splices
// only login_codes/predictions into Store.raw and leaves this section's own
// parsed nodes untouched, so its comments, flow style, quoting and key order
// all survive a save unchanged (spec-7-4). TeamMarks is keyed by lowercase
// division name (e.g. "atlantic") and Series by round name ("round1"..
// "round4", see resultRounds) and then by PlayoffMatchup.Key.
type results struct {
	TeamMarks        map[string]divisionMarks            `yaml:"team_marks,omitempty"`
	PresidentsTrophy string                              `yaml:"presidents_trophy,omitempty"`
	StanleyCupWinner string                              `yaml:"stanley_cup_winner,omitempty"`
	Series           map[string]map[string]seriesOutcome `yaml:"series,omitempty"`
}

// divisionMarks is one division's recorded playoff teams and winner.
type divisionMarks struct {
	Playoffs       []string `yaml:"playoffs"`
	DivisionWinner string   `yaml:"division_winner,omitempty"`
}

// seriesOutcome is one recorded series result. Games is a string ("4".."7")
// like Prediction.Games; an unquoted hand-edited `games: 5` loads as "5".
type seriesOutcome struct {
	Winner string `yaml:"winner,omitempty"`
	Games  string `yaml:"games,omitempty"`
}

// document mirrors fantasy-hockey.yml's on-disk shape.
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
// per AD-25/AD-26.
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
		if err := st.writeLocked("bootstrap data file", time.Now().UTC()); err != nil {
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
		// the type mismatch didn't touch, including sibling fields in the
		// same struct, is exactly as if this error never happened.
		st.toleratedShapeErrors = typeErr.Errors
	}

	// An empty, whitespace-only, or comment-only file parses to a Node with
	// no content at all (Kind 0). Rebuild raw from the (here, zero-valued)
	// typed doc, matching New's bootstrap branch, so the next write doesn't
	// fail on a file that loaded successfully.
	if st.raw.Kind == 0 {
		if err := st.raw.Encode(st.doc); err != nil {
			return fmt.Errorf("store: parse %s: %w", path, err)
		}
	}
	return nil
}

// lenientTopLevelKeys are the two top-level sections whose shape errors AC2
// tolerates instead of aborting startup, confined by line range - never a
// general "the app never fails to parse" policy (Boundaries & Constraints).
var lenientTopLevelKeys = []string{"results", "award_finalists"}

// typeErrorLinePattern matches a *yaml.TypeError message's leading line
// number (e.g. "line 7: cannot unmarshal ..." - Design Notes' "stable,
// long-documented line %d: message format").
var typeErrorLinePattern = regexp.MustCompile(`^line (\d+):`)

// parseErrorLine extracts the line number msg (one entry of a
// *yaml.TypeError's Errors) leads with, if it's in the expected format.
func parseErrorLine(msg string) (int, bool) {
	m := typeErrorLinePattern.FindStringSubmatch(msg)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// typeErrorConfinedToLenientSections reports whether every line typeErr
// names falls within results:'s or award_finalists:'s own line range in
// raw's top-level mapping (AC2) - a message whose line can't be parsed, or
// that falls outside both ranges (including when neither section is
// present in raw at all), fails the check, so New returns the original
// error unchanged.
func typeErrorConfinedToLenientSections(typeErr *yaml.TypeError, raw *yaml.Node) bool {
	mapping := topLevelMapping(raw)

	var ranges [][2]int
	for _, key := range lenientTopLevelKeys {
		if node, ok := mappingValue(mapping, key); ok {
			ranges = append(ranges, [2]int{node.Line, maxLine(node)})
		}
	}

	for _, msg := range typeErr.Errors {
		line, ok := parseErrorLine(msg)
		if !ok {
			return false
		}
		if !lineWithinAnyRange(line, ranges) {
			return false
		}
	}
	return true
}

// lineWithinAnyRange reports whether line falls within any of ranges
// (inclusive [start, end] pairs).
func lineWithinAnyRange(line int, ranges [][2]int) bool {
	for _, r := range ranges {
		if line >= r[0] && line <= r[1] {
			return true
		}
	}
	return false
}

// maxLine returns the greatest Line value anywhere in n's own subtree
// (itself and every descendant) - a mapping or sequence node's own Line
// only marks where its content starts, so this is needed to find where it
// ends, bounding a section's full line range.
func maxLine(n *yaml.Node) int {
	max := n.Line
	for _, c := range n.Content {
		if l := maxLine(c); l > max {
			max = l
		}
	}
	return max
}

// topLevelMapping returns n's top-level mapping node: the DocumentNode's
// first child when n was produced by unmarshaling a full file's bytes, or n
// itself when n was produced by Node.Encode (which never wraps its result
// in a DocumentNode, unlike Unmarshal).
func topLevelMapping(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

// mappingValue returns key's value node within mapping's Content, and
// whether key was present at all. mapping being nil or not a MappingNode
// (a shape error already reported elsewhere) reports key absent.
func mappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], true
		}
	}
	return nil, false
}

// mappingKeys returns every key in mapping's Content, in the file's own
// order - already deterministic (unlike a Go map's iteration order), since
// it comes straight from the parsed document. mapping being nil or not a
// MappingNode returns nil.
func mappingKeys(mapping *yaml.Node) []string {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(mapping.Content)/2)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keys = append(keys, mapping.Content[i].Value)
	}
	return keys
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

		if err := s.writeLocked("save prediction", now); err != nil {
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

	if err := s.writeLocked("save prediction", now); err != nil {
		s.doc.Predictions = s.doc.Predictions[:len(s.doc.Predictions)-1]
		return fmt.Errorf("store: persist prediction: %w", err)
	}
	return nil
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
// silently persisted by an unrelated successful write.
func (s *Store) SaveSeriesPick(playerID, seriesKey, teamID, games string, now time.Time) error {
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
			return fmt.Errorf("store: persist series pick: %w", err)
		}
		return nil
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
		return fmt.Errorf("store: persist series pick: %w", err)
	}
	return nil
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
// underlying slice reallocates.
func (s *Store) SaveDivisionPicks(playerID string, playoffTeams map[string][]string, winners map[string]string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := make([]Prediction, len(s.doc.Predictions))
	copy(snapshot, s.doc.Predictions)

	submittedAt := now.UTC().Format(time.RFC3339)

	for division, teamIDs := range playoffTeams {
		s.upsertDivisionPredictionLocked(playerID, KindDivisionPlayoffTeams, division, teamIDs, "", submittedAt)
	}
	for division, teamID := range winners {
		if teamID == "" {
			continue
		}
		s.upsertDivisionPredictionLocked(playerID, KindDivisionWinner, division, nil, teamID, submittedAt)
	}

	if err := s.writeLocked("save division picks", now); err != nil {
		s.doc.Predictions = snapshot
		return fmt.Errorf("store: persist division picks: %w", err)
	}
	return nil
}

// upsertDivisionPredictionLocked updates the existing (playerID, kind,
// division) row's TeamIDs/TeamID/SubmittedAt in place, or appends a new row
// with a generated id - shared by SaveDivisionPicks' two upsert loops.
// Callers must hold s.mu for writing.
func (s *Store) upsertDivisionPredictionLocked(playerID, kind, division string, teamIDs []string, teamID, submittedAt string) {
	for i := range s.doc.Predictions {
		row := &s.doc.Predictions[i]
		if row.PlayerID == playerID && row.Kind == kind && row.Division == division {
			row.TeamIDs = teamIDs
			row.TeamID = teamID
			row.SubmittedAt = submittedAt
			return
		}
	}

	s.doc.Predictions = append(s.doc.Predictions, Prediction{
		ID:          uuid.NewString(),
		PlayerID:    playerID,
		Kind:        kind,
		Division:    division,
		TeamIDs:     teamIDs,
		TeamID:      teamID,
		SubmittedAt: submittedAt,
	})
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
// underlying slice reallocates).
func (s *Store) SaveAwardPicks(playerID string, finalists map[string][]string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := make([]Prediction, len(s.doc.Predictions))
	copy(snapshot, s.doc.Predictions)

	submittedAt := now.UTC().Format(time.RFC3339)

	for award, slugs := range finalists {
		if !awardHasAllFinalistSlugs(slugs) {
			continue
		}
		s.upsertAwardPredictionLocked(playerID, award, slugs, submittedAt)
	}

	if err := s.writeLocked("save award picks", now); err != nil {
		s.doc.Predictions = snapshot
		return fmt.Errorf("store: persist award picks: %w", err)
	}
	return nil
}

// upsertAwardPredictionLocked updates the existing (playerID, KindAward,
// award) row's FinalistSlugs/SubmittedAt in place, or appends a new row
// with a generated id - shared by SaveAwardPicks' own upsert loop, mirroring
// upsertDivisionPredictionLocked. Callers must hold s.mu for writing.
func (s *Store) upsertAwardPredictionLocked(playerID, award string, slugs []string, submittedAt string) {
	finalistSlugs := append([]string(nil), slugs...) // own copy: never alias the caller's slice.

	for i := range s.doc.Predictions {
		row := &s.doc.Predictions[i]
		if row.PlayerID == playerID && row.Kind == KindAward && row.Award == award {
			row.FinalistSlugs = finalistSlugs
			row.SubmittedAt = submittedAt
			return
		}
	}

	s.doc.Predictions = append(s.doc.Predictions, Prediction{
		ID:            uuid.NewString(),
		PlayerID:      playerID,
		Kind:          KindAward,
		Award:         award,
		FinalistSlugs: finalistSlugs,
		SubmittedAt:   submittedAt,
	})
}

// cleanupLoginCodes returns the subset of rows that are still eligible to be
// consumed as of now: a row is dropped when it has already been used
// (UsedAt != nil) or when its IssuedAt parses and is more than
// loginCodeValidity in now's past. A row whose IssuedAt fails to parse is
// always kept - the code can't confirm it's expired, so it never destroys
// that data (spec-7-2's Boundaries & Constraints). Order of kept rows is
// preserved. This is the single home for the opposite-direction check
// ConsumeLoginCode's own scan already performs inline; the two are
// intentionally not shared, since ConsumeLoginCode also matches on
// CodeHash/future-dating, a different job from pruning.
func cleanupLoginCodes(rows []LoginCode, now time.Time) []LoginCode {
	cleaned := make([]LoginCode, 0, len(rows))
	for _, row := range rows {
		if row.UsedAt != nil {
			continue
		}
		if issuedAt, err := time.Parse(time.RFC3339, row.IssuedAt); err == nil && now.Sub(issuedAt) > loginCodeValidity {
			continue
		}
		cleaned = append(cleaned, row)
	}
	return cleaned
}

// spliceNamedValueLocked replaces key's value node within mapping's Content
// with value, appending a new "key: value" pair at mapping's end when key
// isn't present yet (a hand-seeded fixture, or a pre-spec-7-4 file, that
// omits login_codes: or predictions: entirely still needs a write to
// succeed). It returns a restore closure that undoes exactly this splice -
// reassigning the old value node back in place, or truncating the appended
// pair - so writeLocked can roll every splice back in reverse order if a
// later step fails. Callers must hold s.mu for writing.
func spliceNamedValueLocked(mapping *yaml.Node, key string, value *yaml.Node) (restore func()) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			old := mapping.Content[i+1]
			mapping.Content[i+1] = value
			return func() { mapping.Content[i+1] = old }
		}
	}

	var keyNode yaml.Node
	keyNode.SetString(key)
	mapping.Content = append(mapping.Content, &keyNode, value)
	return func() { mapping.Content = mapping.Content[:len(mapping.Content)-2] }
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
// into, never reconstructed from doc's typed fields (spec-7-4, AD-23). Only
// once the rename succeeds are s.doc.LoginCodes and the two spliced nodes
// committed permanently; on any failure both splices are undone first, so
// s.raw is left exactly as it was before this call. On success it also logs
// exactly one slog.Info line naming reason, the caller's own literal label
// for why this write happened (e.g. "create login code") - the sole place
// any Store write is logged, so every mutating method's call site stays a
// one-line addition instead of duplicating a log call at every site
// (spec-7-1's Design Notes). reason is always a fixed, non-identifying
// literal, never a login code or player email. Callers must hold s.mu for
// writing.
func (s *Store) writeLocked(reason string, now time.Time) error {
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
	slog.Info("store write", "reason", reason)
	return nil
}

// resultRounds maps the results.series round names a human records by hand
// to the round Prediction Set ids every KindSeries SeriesKey starts with.
var resultRounds = map[string]string{
	"round1": Round1SetID,
	"round2": Round2SetID,
	"round3": ConferenceFinalsSetID,
	"round4": StanleyCupFinalSetID,
}

// awards is the fixed award vocabulary award_finalists may be keyed by.
var awards = []string{AwardHart, AwardNorris, AwardVezina, AwardArtRoss, AwardRocketRichard}

// minSeriesGames and maxSeriesGames bound a best-of-seven series' length.
const (
	minSeriesGames = 4
	maxSeriesGames = 7
)

// resultDivisionKey is the lowercase team_marks key for a Divisions() name.
func resultDivisionKey(division string) string {
	return strings.ToLower(division)
}

// resultRoundForSetID reverses resultRounds.
func resultRoundForSetID(setID string) (string, bool) {
	for round, id := range resultRounds {
		if id == setID {
			return round, true
		}
	}
	return "", false
}

// validSeriesGames reports whether games is a whole number from
// minSeriesGames to maxSeriesGames.
func validSeriesGames(games string) bool {
	_, ok := parseSeriesGames(games)
	return ok
}

// parseSeriesGames parses games and returns it in canonical form ("5" for a
// hand-edited "05" or "+5"), the form Prediction.Games is compared in.
func parseSeriesGames(games string) (string, bool) {
	n, err := strconv.Atoi(games)
	if err != nil || n < minSeriesGames || n > maxSeriesGames {
		return "", false
	}
	return strconv.Itoa(n), true
}

// sortedKeys returns m's keys in order, so ResultProblems is deterministic.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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

// DivisionResult returns the recorded playoff teams and winner for
// division, a capitalised Divisions() name. Unknown team abbreviations are
// left out (ResultProblems reports them), and nothing is returned for a
// division that isn't recorded yet.
func (s *Store) DivisionResult(division string) (playoffs []string, winner string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !slices.Contains(divisions, division) {
		return nil, ""
	}
	marks := s.doc.Results.TeamMarks[resultDivisionKey(division)]
	for _, team := range marks.Playoffs {
		if s.teamInDivisionLocked(team, division) {
			playoffs = append(playoffs, team)
		}
	}
	if !s.teamInDivisionLocked(marks.DivisionWinner, division) {
		return playoffs, ""
	}
	return playoffs, marks.DivisionWinner
}

// PresidentsTrophyWinner returns the recorded Presidents' Trophy team, or ""
// when none (or an unknown team) is recorded.
func (s *Store) PresidentsTrophyWinner() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.knownTeamOrEmptyLocked(s.doc.Results.PresidentsTrophy)
}

// StanleyCupWinner returns the recorded Stanley Cup champion, or "" when
// none (or an unknown team) is recorded.
func (s *Store) StanleyCupWinner() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.knownTeamOrEmptyLocked(s.doc.Results.StanleyCupWinner)
}

// SeriesResult returns the recorded winner and game count for seriesKey, the
// prediction-side key built by JoinSeriesKey (e.g. "r1.s1"). ok is false
// when the series isn't fully recorded yet or its entry is malformed (see
// ResultProblems), so a caller scores it as nothing.
func (s *Store) SeriesResult(seriesKey string) (winner, games string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	setID, key, ok := SplitSeriesKey(seriesKey)
	if !ok {
		return "", "", false
	}
	round, ok := resultRoundForSetID(setID)
	if !ok {
		return "", "", false
	}
	outcome, ok := s.doc.Results.Series[round][key]
	if !ok || !s.seriesOutcomeUsableLocked(setID, key, outcome) {
		return "", "", false
	}
	games, _ = parseSeriesGames(outcome.Games)
	return outcome.Winner, games, true
}

// RecordedAwardFinalists returns the slugs recorded as award's finalists -
// more than AwardFinalistCount when a tie expands the set. Unknown slugs and
// unknown awards are left out (ResultProblems reports them).
func (s *Store) RecordedAwardFinalists(award string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !slices.Contains(awards, award) {
		return nil
	}
	var slugs []string
	for _, f := range s.doc.AwardFinalists[award] {
		if s.knownSlugLocked(f.Slug) {
			slugs = append(slugs, f.Slug)
		}
	}
	return slugs
}

// ResultProblems lists every malformed entry in the hand-maintained results
// and award_finalists sections: an unknown team abbreviation, finalist
// slug, division, award or round, a series key with no matching
// playoff_matchups entry, games outside 4-7, a series winner that is not
// one of its matchup's two teams, a team_marks team from another division,
// a misspelled or unknown fixed-shape key (AC1), or a value New tolerated
// because its wrong shape was confined to results:/award_finalists: (AC2).
// The store only reports them (the read methods above ignore each bad
// entry); main.go logs them.
func (s *Store) ResultProblems() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var problems []string
	problems = append(problems, s.toleratedShapeErrorProblemsLocked()...)
	problems = append(problems, s.unknownKeyProblemsLocked()...)
	problems = append(problems, s.teamMarkProblemsLocked()...)
	problems = append(problems, s.teamProblemLocked("results.presidents_trophy", s.doc.Results.PresidentsTrophy)...)
	problems = append(problems, s.teamProblemLocked("results.stanley_cup_winner", s.doc.Results.StanleyCupWinner)...)
	problems = append(problems, s.seriesProblemsLocked()...)
	problems = append(problems, s.awardProblemsLocked()...)
	return problems
}

// toleratedShapeErrorProblemsLocked formats every *yaml.TypeError message
// New tolerated at load (AC2) into the same "problem" shape ResultProblems'
// other helpers return, so main.go's per-problem warning loop covers it
// too. Callers must hold s.mu for reading.
func (s *Store) toleratedShapeErrorProblemsLocked() []string {
	if len(s.toleratedShapeErrors) == 0 {
		return nil
	}
	problems := make([]string, len(s.toleratedShapeErrors))
	for i, msg := range s.toleratedShapeErrors {
		problems[i] = "results/award_finalists: " + msg
	}
	return problems
}

// resultsTopLevelKeys/divisionMarksKeys/seriesOutcomeKeys/
// awardFinalistEntryKeys are the fixed-shape struct fields AC1's
// unknown-key detection checks - the ones a typed struct silently drops
// instead of erroring on (Design Notes). unknownKeyProblemsLocked never
// re-checks the map-keyed vocabularies (division/round/award names)
// teamMarkProblemsLocked/seriesProblemsLocked/awardProblemsLocked already
// validate - a Go map, unlike a struct, keeps every key it's given.
var (
	resultsTopLevelKeys    = []string{"team_marks", "presidents_trophy", "stanley_cup_winner", "series"}
	divisionMarksKeys      = []string{"playoffs", "division_winner"}
	seriesOutcomeKeys      = []string{"winner", "games"}
	awardFinalistEntryKeys = []string{"slug", "display_name"}
)

// unknownMappingKeys reports every key in node (a mapping) not in known, at
// path "prefix.key" - node being nil or not a MappingNode (a shape error
// already reported elsewhere, or a section simply not present) reports
// nothing.
func unknownMappingKeys(prefix string, node *yaml.Node, known []string) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	var problems []string
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if !slices.Contains(known, key) {
			problems = append(problems, fmt.Sprintf("%s.%s: unknown key", prefix, key))
		}
	}
	return problems
}

// unknownKeyProblemsLocked reports a misspelled or unknown key anywhere in
// results:/award_finalists:'s fixed-shape struct fields - the one class of
// mistake yaml.Unmarshal never errors on (a misspelled field is simply
// never populated, Design Notes), so it's the one class ResultProblems'
// other helpers can't catch on their own (AC1). Callers must hold s.mu for
// reading.
func (s *Store) unknownKeyProblemsLocked() []string {
	if s.raw == nil {
		return nil
	}
	mapping := topLevelMapping(s.raw)

	var problems []string
	if resultsNode, ok := mappingValue(mapping, "results"); ok {
		problems = append(problems, unknownMappingKeys("results", resultsNode, resultsTopLevelKeys)...)
		problems = append(problems, unknownTeamMarksKeys(resultsNode)...)
		problems = append(problems, unknownSeriesOutcomeKeys(resultsNode)...)
	}
	if finalists, ok := mappingValue(mapping, "award_finalists"); ok {
		problems = append(problems, unknownAwardFinalistEntryKeys(finalists)...)
	}
	return problems
}

// unknownTeamMarksKeys reports an unknown key inside any of resultsNode's
// team_marks.<division> entries - shared by unknownKeyProblemsLocked to keep
// its own cyclomatic complexity in check.
func unknownTeamMarksKeys(resultsNode *yaml.Node) []string {
	teamMarks, ok := mappingValue(resultsNode, "team_marks")
	if !ok {
		return nil
	}
	var problems []string
	for _, division := range mappingKeys(teamMarks) {
		marks, _ := mappingValue(teamMarks, division)
		problems = append(problems, unknownMappingKeys("results.team_marks."+division, marks, divisionMarksKeys)...)
	}
	return problems
}

// unknownSeriesOutcomeKeys reports an unknown key inside any of resultsNode's
// series.<round>.<key> entries - shared by unknownKeyProblemsLocked to keep
// its own cyclomatic complexity in check.
func unknownSeriesOutcomeKeys(resultsNode *yaml.Node) []string {
	rounds, ok := mappingValue(resultsNode, "series")
	if !ok {
		return nil
	}
	var problems []string
	for _, round := range mappingKeys(rounds) {
		matchups, _ := mappingValue(rounds, round)
		for _, key := range mappingKeys(matchups) {
			outcome, _ := mappingValue(matchups, key)
			problems = append(problems, unknownMappingKeys(fmt.Sprintf("results.series.%s.%s", round, key), outcome, seriesOutcomeKeys)...)
		}
	}
	return problems
}

// unknownAwardFinalistEntryKeys reports an unknown key inside any of
// finalists' award_finalists.<award>[i] entries - shared by
// unknownKeyProblemsLocked to keep its own cyclomatic complexity in check.
func unknownAwardFinalistEntryKeys(finalists *yaml.Node) []string {
	var problems []string
	for _, award := range mappingKeys(finalists) {
		list, _ := mappingValue(finalists, award)
		if list == nil || list.Kind != yaml.SequenceNode {
			continue
		}
		for i, entry := range list.Content {
			problems = append(problems, unknownMappingKeys(fmt.Sprintf("award_finalists.%s[%d]", award, i), entry, awardFinalistEntryKeys)...)
		}
	}
	return problems
}

func (s *Store) teamMarkProblemsLocked() []string {
	var problems []string
	known := make([]string, len(divisions))
	for i, d := range divisions {
		known[i] = resultDivisionKey(d)
	}
	for _, division := range sortedKeys(s.doc.Results.TeamMarks) {
		path := "results.team_marks." + division
		if !slices.Contains(known, division) {
			problems = append(problems, fmt.Sprintf("%s: unknown division %q (keys are lowercase: %s)", path, division, strings.Join(known, ", ")))
			continue
		}
		name := divisions[slices.Index(known, division)]
		marks := s.doc.Results.TeamMarks[division]
		for _, team := range marks.Playoffs {
			problems = append(problems, s.divisionTeamProblemLocked(path+".playoffs", team, name)...)
		}
		problems = append(problems, s.divisionTeamProblemLocked(path+".division_winner", marks.DivisionWinner, name)...)
	}
	return problems
}

func (s *Store) seriesProblemsLocked() []string {
	var problems []string
	for _, round := range sortedKeys(s.doc.Results.Series) {
		setID, ok := resultRounds[round]
		if !ok {
			problems = append(problems, fmt.Sprintf("results.series.%s: unknown round (valid rounds: %s)", round, strings.Join(sortedKeys(resultRounds), ", ")))
			continue
		}
		outcomes := s.doc.Results.Series[round]
		for _, key := range sortedKeys(outcomes) {
			problems = append(problems, s.seriesOutcomeProblemsLocked(round, setID, key, outcomes[key])...)
		}
	}
	return problems
}

func (s *Store) seriesOutcomeProblemsLocked(round, setID, key string, outcome seriesOutcome) []string {
	path := fmt.Sprintf("results.series.%s.%s", round, key)
	var problems []string
	matchup, ok := s.findMatchupLocked(setID, key)
	if !ok {
		problems = append(problems, fmt.Sprintf("%s: no playoff_matchups.%s entry with key %q", path, setID, key))
	}
	winnerProblems := s.teamProblemLocked(path+".winner", outcome.Winner)
	problems = append(problems, winnerProblems...)
	if ok && outcome.Winner != "" && len(winnerProblems) == 0 && !matchupHasTeam(matchup, outcome.Winner) {
		problems = append(problems, fmt.Sprintf("%s: winner %q is not in the %s vs %s matchup", path+".winner", outcome.Winner, matchup.TeamA, matchup.TeamB))
	}
	if outcome.Games != "" && !validSeriesGames(outcome.Games) {
		problems = append(problems, fmt.Sprintf("%s: games %q outside %d-%d", path, outcome.Games, minSeriesGames, maxSeriesGames))
	}
	return problems
}

func (s *Store) awardProblemsLocked() []string {
	var problems []string
	for _, award := range sortedKeys(s.doc.AwardFinalists) {
		path := "award_finalists." + award
		if !slices.Contains(awards, award) {
			problems = append(problems, fmt.Sprintf("%s: unknown award %q", path, award))
			continue
		}
		for _, f := range s.doc.AwardFinalists[award] {
			if !s.knownSlugLocked(f.Slug) {
				problems = append(problems, fmt.Sprintf("%s: unknown finalist slug %q", path, f.Slug))
			}
		}
	}
	return problems
}

// teamProblemLocked reports team at path when it is set but not a canonical
// team; an empty (not yet recorded) value is never a problem.
func (s *Store) teamProblemLocked(path, team string) []string {
	if team == "" || s.knownTeamLocked(team) {
		return nil
	}
	return []string{fmt.Sprintf("%s: unknown team %q", path, team)}
}

// seriesOutcomeUsableLocked reports whether outcome is fully recorded and
// well-formed, so it can be scored.
func (s *Store) seriesOutcomeUsableLocked(setID, key string, outcome seriesOutcome) bool {
	matchup, ok := s.findMatchupLocked(setID, key)
	return ok && s.knownTeamLocked(outcome.Winner) && matchupHasTeam(matchup, outcome.Winner) && validSeriesGames(outcome.Games)
}

func (s *Store) findMatchupLocked(setID, key string) (PlayoffMatchup, bool) {
	i := slices.IndexFunc(s.doc.PlayoffMatchups[setID], func(m PlayoffMatchup) bool { return m.Key == key })
	if i < 0 {
		return PlayoffMatchup{}, false
	}
	return s.doc.PlayoffMatchups[setID][i], true
}

// matchupHasTeam reports whether team is one of m's two sides.
func matchupHasTeam(m PlayoffMatchup, team string) bool {
	return team == m.TeamA || team == m.TeamB
}

// divisionTeamProblemLocked is teamProblemLocked for a team_marks entry,
// additionally reporting a known team that plays in another division.
func (s *Store) divisionTeamProblemLocked(path, team, division string) []string {
	if problems := s.teamProblemLocked(path, team); problems != nil || team == "" {
		return problems
	}
	if s.teamInDivisionLocked(team, division) {
		return nil
	}
	return []string{fmt.Sprintf("%s: team %q is not in the %s division", path, team, division)}
}

// teamInDivisionLocked reports whether id is a canonical team of division.
func (s *Store) teamInDivisionLocked(id, division string) bool {
	return slices.ContainsFunc(s.doc.Teams, func(t Team) bool { return t.ID == id && t.Division == division })
}

func (s *Store) knownTeamLocked(id string) bool {
	return slices.ContainsFunc(s.doc.Teams, func(t Team) bool { return t.ID == id })
}

func (s *Store) knownTeamOrEmptyLocked(id string) string {
	if !s.knownTeamLocked(id) {
		return ""
	}
	return id
}

func (s *Store) knownSlugLocked(slug string) bool {
	return slices.ContainsFunc(s.doc.NHLPlayers, func(p AwardFinalist) bool { return p.Slug == slug })
}
