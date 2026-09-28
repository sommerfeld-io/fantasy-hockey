package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// seedSubmittedAt stamps every seeded prediction row; no results test
// reads it.
const seedSubmittedAt = "2026-09-20T10:00:00Z"

// resultsFixtureBase is the canonical data every test in this package builds
// on, not just the results-focused ones here: three Atlantic teams and one
// Pacific team, four NHL players and one matchup in round 1 and in the
// conference finals. store_test.go and store_splice_test.go seed it too.
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
// section: games are unquoted, the way a human hand-edits them. Package-wide
// fixture, not results-only - store_test.go and store_splice_test.go seed it
// too.
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

// resultsFixtureMatchups is resultsFixtureBase's playoff_matchups section.
const resultsFixtureMatchups = "playoff_matchups:\n    r1:\n        - {key: s1, a: FLA, b: TOR}\n    cf:\n        - {key: s1, a: FLA, b: BOS}\n"

// newSeededStore writes seed to a temp data file and opens st on it, so
// New's own parsing of the seeded document is exercised. path is the data
// file's path (not its directory), for tests that inspect or reopen it.
// Package-wide helper - store_test.go and store_splice_test.go call it too,
// not just the results tests here.
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

// --- spec-7-4: hand-edited results are safe to edit ---

func TestResultProblemsShouldReportAnUnknownOrMisspelledKeyInsideResultsOrAwardFinalists(t *testing.T) {
	tests := []struct {
		name     string
		results  string
		wantPath string
	}{
		{
			name:     "unknown results top-level key",
			results:  "results:\n    stanley_cup_winer: FLA\n",
			wantPath: "results.stanley_cup_winer",
		},
		{
			name:     "unknown division marks key",
			results:  "results:\n    team_marks:\n        atlantic:\n            playoffs: [FLA]\n            division_champ: FLA\n",
			wantPath: "results.team_marks.atlantic.division_champ",
		},
		{
			name:     "unknown series outcome key",
			results:  "results:\n    series:\n        round1:\n            s1: {winner: FLA, gams: 5}\n",
			wantPath: "results.series.round1.s1.gams",
		},
		{
			name:     "unknown award finalist entry key",
			results:  "award_finalists:\n    hart:\n        - {slug: mcdavid-connor, dispaly_name: Connor McDavid}\n",
			wantPath: "award_finalists.hart[0].dispaly_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, _ := newSeededStore(t, resultsFixtureBase+tt.results)

			problems := st.ResultProblems()
			if len(problems) != 1 || !strings.Contains(problems[0], tt.wantPath) || !strings.Contains(problems[0], "unknown key") {
				t.Errorf("ResultProblems = %v, want exactly one naming %q as an unknown key", problems, tt.wantPath)
			}
		})
	}
}

func TestResultProblemsShouldNotFlagAKnownFixedShapeKeyAsUnknown(t *testing.T) {
	st, _ := newSeededStore(t, resultsFixtureBase+resultsFixtureResults)

	for _, problem := range st.ResultProblems() {
		if strings.Contains(problem, "unknown key") {
			t.Errorf("ResultProblems = %v, expected no false positive for a well-formed file", problem)
		}
	}
}

func TestNewShouldTolerateAWronglyShapedResultsEntryAndStartAnyway(t *testing.T) {
	seed := resultsFixtureBase + "results:\n    team_marks:\n        atlantic:\n            playoffs: FLA\n            division_winner: FLA\n"
	st, _ := newSeededStore(t, seed)

	if playoffs, winner := st.DivisionResult("Atlantic"); len(playoffs) != 0 || winner != "FLA" {
		t.Errorf("DivisionResult(Atlantic) = %v, %q, want no playoffs (tolerated as zero-valued) but the sibling division_winner field still populated", playoffs, winner)
	}

	problems := st.ResultProblems()
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "results: ") || !strings.Contains(problems[0], "line") {
		t.Errorf("ResultProblems = %v, want exactly one problem naming the tolerated shape error, prefixed with the specific section (review finding: it used to say \"results/award_finalists\" regardless of which section actually had the problem)", problems)
	}
}

func TestNewShouldTolerateAWronglyShapedAwardFinalistsEntryAndStartAnyway(t *testing.T) {
	seed := resultsFixtureBase + "award_finalists:\n    hart: FLA\n"
	st, _ := newSeededStore(t, seed)

	if got := st.RecordedAwardFinalists(AwardHart); len(got) != 0 {
		t.Errorf("RecordedAwardFinalists(hart) = %v, want none (tolerated as zero-valued)", got)
	}
	problems := st.ResultProblems()
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "award_finalists: ") || !strings.Contains(problems[0], "line") {
		t.Errorf("ResultProblems = %v, want exactly one problem naming the tolerated shape error, prefixed with the specific section", problems)
	}
}

// TestResultProblemsShouldWarnThatSiblingFieldsMayBeAffectedByADuplicateKey
// covers a review finding (epic-7 retrospective): a duplicate key at the
// same mapping level as other results: fields doesn't just zero the
// duplicated key -- yaml.Unmarshal abandons decoding every field at that
// same level, so presidents_trophy/stanley_cup_winner (team_marks'
// siblings, not its contents) silently come back empty too, with the
// tolerated-shape-error message naming only the duplicated key. The
// warning must say so, since AC2's "does not wipe what the human wrote"
// promise doesn't cover an unrelated sibling being wiped.
func TestResultProblemsShouldWarnThatSiblingFieldsMayBeAffectedByADuplicateKey(t *testing.T) {
	seed := resultsFixtureBase + "results:\n    team_marks:\n        atlantic:\n            playoffs: [FLA]\n    team_marks:\n        pacific:\n            playoffs: [EDM]\n    presidents_trophy: FLA\n    stanley_cup_winner: FLA\n"
	st, _ := newSeededStore(t, seed)

	if got := st.PresidentsTrophyWinner(); got != "" {
		t.Errorf("PresidentsTrophyWinner() = %q, want empty (zeroed as a side effect of the sibling team_marks duplicate key)", got)
	}
	if got := st.StanleyCupWinner(); got != "" {
		t.Errorf("StanleyCupWinner() = %q, want empty (zeroed as a side effect of the sibling team_marks duplicate key)", got)
	}

	problems := st.ResultProblems()
	if len(problems) != 1 || !strings.Contains(problems[0], "already defined") {
		t.Fatalf("ResultProblems = %v, want exactly one problem naming the duplicate key", problems)
	}
	if !strings.Contains(problems[0], "other fields in this section may also be unset") {
		t.Errorf("ResultProblems = %v, want the duplicate-key warning to note that sibling fields in the same section may also be affected", problems)
	}
}

// TestResultProblemsShouldNotAddTheDuplicateKeyNoteToAnOrdinaryTypeMismatch
// is the should-not counterpart: an ordinary type-mismatch tolerated error
// (which does NOT zero unrelated siblings, per Design Notes) must not carry
// the duplicate-key note -- it would be misleading noise on a message where
// siblings are, in fact, untouched.
func TestResultProblemsShouldNotAddTheDuplicateKeyNoteToAnOrdinaryTypeMismatch(t *testing.T) {
	seed := resultsFixtureBase + "results:\n    team_marks:\n        atlantic:\n            playoffs: FLA\n    presidents_trophy: FLA\n"
	st, _ := newSeededStore(t, seed)

	if got := st.PresidentsTrophyWinner(); got != "FLA" {
		t.Errorf("PresidentsTrophyWinner() = %q, want %q (sibling of the malformed playoffs field, untouched)", got, "FLA")
	}

	problems := st.ResultProblems()
	if len(problems) != 1 {
		t.Fatalf("ResultProblems = %v, want exactly one problem", problems)
	}
	if strings.Contains(problems[0], "other fields in this section may also be unset") {
		t.Errorf("ResultProblems = %v, want no duplicate-key note on an ordinary type-mismatch message", problems)
	}
}

func TestNewShouldStillFailForAShapeErrorOutsideResultsAndAwardFinalists(t *testing.T) {
	seed := "season: \"2026-27\"\nteams: FLA\nresults:\n    stanley_cup_winner: FLA\n"
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := New(path); err == nil {
		t.Fatal("expected New to fail for a shape error outside results:/award_finalists:, even alongside a well-formed results: section")
	}
}

func TestNewShouldStillFailForAShapeErrorMixingAToleratedAndAnUntoleratedLine(t *testing.T) {
	seed := "season: \"2026-27\"\nteams: FLA\nresults:\n    team_marks:\n        atlantic:\n            playoffs: FLA\n"
	dir := t.TempDir()
	path := filepath.Join(dir, DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := New(path); err == nil {
		t.Fatal("expected New to fail when even one malformed line falls outside results:/award_finalists:, despite another falling inside")
	}
}

// TestNewShouldTolerateShapeErrorsInBothResultsAndAwardFinalistsAtOnce
// exercises typeErrorConfinedToLenientSections' multi-range support with
// both of its ranges actually populated at once - every other tolerated-
// shape-error test only ever seeds one of the two sections.
func TestNewShouldTolerateShapeErrorsInBothResultsAndAwardFinalistsAtOnce(t *testing.T) {
	seed := resultsFixtureBase +
		"results:\n    team_marks:\n        atlantic:\n            playoffs: FLA\n" +
		"award_finalists:\n    hart: FLA\n"
	st, _ := newSeededStore(t, seed)

	if playoffs, _ := st.DivisionResult("Atlantic"); len(playoffs) != 0 {
		t.Errorf("DivisionResult(Atlantic) = %v, want no playoffs (tolerated as zero-valued)", playoffs)
	}
	if got := st.RecordedAwardFinalists(AwardHart); len(got) != 0 {
		t.Errorf("RecordedAwardFinalists(hart) = %v, want none (tolerated as zero-valued)", got)
	}

	problems := st.ResultProblems()
	if len(problems) != 2 {
		t.Fatalf("ResultProblems = %v, want exactly 2 problems (one per tolerated section)", problems)
	}
	// Each message must be tagged with its own specific section, not a
	// generic "results/award_finalists" label for both regardless of which
	// one actually had the problem (review finding) - the real test of the
	// section-disambiguation fix, since both ranges are populated here.
	var sawResults, sawAwardFinalists bool
	for _, p := range problems {
		sawResults = sawResults || strings.HasPrefix(p, "results: ")
		sawAwardFinalists = sawAwardFinalists || strings.HasPrefix(p, "award_finalists: ")
	}
	if !sawResults || !sawAwardFinalists {
		t.Errorf("ResultProblems = %v, want one problem prefixed \"results: \" and one prefixed \"award_finalists: \"", problems)
	}
}
