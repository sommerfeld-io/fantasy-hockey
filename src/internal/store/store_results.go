package store

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

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

// toleratedShapeErrorProblemsLocked returns the section-tagged *yaml.TypeError
// messages New tolerated at load (AC2), already formatted (loadExisting) in
// the same "problem" shape ResultProblems' other helpers return, so main.go's
// per-problem warning loop covers it too. Callers must hold s.mu for reading.
func (s *Store) toleratedShapeErrorProblemsLocked() []string {
	return s.toleratedShapeErrors
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
