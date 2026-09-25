package scoring

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// scoringFixtureBase declares the canonical data every scoring test builds
// on: five Atlantic teams, four NHL players, one matchup "s1" in every round
// and the one player being scored.
const scoringFixtureBase = `season: "2026-27"
players:
    - {id: basti, name: Basti, email: basti@example.com}
teams:
    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}
    - {id: TOR, name: Toronto Maple Leafs, conference: Eastern, division: Atlantic}
    - {id: BOS, name: Boston Bruins, conference: Eastern, division: Atlantic}
    - {id: TBL, name: Tampa Bay Lightning, conference: Eastern, division: Atlantic}
    - {id: BUF, name: Buffalo Sabres, conference: Eastern, division: Atlantic}
nhl_players:
    - {slug: mcdavid-connor, display_name: Connor McDavid, position: skater}
    - {slug: mackinnon-nathan, display_name: Nathan MacKinnon, position: skater}
    - {slug: kucherov-nikita, display_name: Nikita Kucherov, position: skater}
    - {slug: matthews-auston, display_name: Auston Matthews, position: skater}
playoff_matchups:
    r1: [{key: s1, a: FLA, b: TOR}]
    r2: [{key: s1, a: FLA, b: TOR}]
    cf: [{key: s1, a: FLA, b: TOR}]
    scf: [{key: s1, a: FLA, b: TOR}]
`

// pick renders one of basti's Prediction rows from its kind-specific fields.
func pick(fields string) string {
	return "    - {player_id: basti, submitted_at: \"2026-09-20T10:00:00Z\", " + fields + "}\n"
}

// newScoringStore opens a store on scoringFixtureBase plus picks (Prediction
// rows) and results (raw results/award_finalists YAML), returning it with
// its data file's path.
func newScoringStore(t *testing.T, picks []string, results string) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), store.DataFileName)
	seed := scoringFixtureBase + "predictions:\n" + strings.Join(picks, "") + results
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New returned error: %v", err)
	}
	if problems := st.ResultProblems(); len(problems) != 0 {
		t.Fatalf("fixture has result problems: %v", problems)
	}
	return st, path
}

const atlanticFLAWon = "results:\n    team_marks:\n        atlantic: {playoffs: [FLA, TOR, BOS, TBL], division_winner: FLA}\n"

const hartWithATie = "award_finalists:\n    hart:\n" +
	"        - {slug: mcdavid-connor, display_name: Connor McDavid}\n" +
	"        - {slug: mackinnon-nathan, display_name: Nathan MacKinnon}\n" +
	"        - {slug: matthews-auston, display_name: Auston Matthews}\n" +
	"        - {slug: kucherov-nikita, display_name: Nikita Kucherov}\n"

var (
	hartTrio   = pick("kind: award, award: hart, finalist_slugs: [mcdavid-connor, mackinnon-nathan, kucherov-nikita]")
	cupFLA     = pick("kind: cup, team_id: FLA")
	playoffFLA = pick("kind: playoffcup, team_id: FLA")
)

func divisionList(teams string) string {
	return pick("kind: division_playoff_teams, division: Atlantic, team_ids: [" + teams + "]")
}

func divisionWinner(team string) string {
	return pick("kind: division_winner, division: Atlantic, team_id: " + team)
}

func seriesPick(setID, team, games string) string {
	return pick(fmt.Sprintf("kind: series, series_key: %s, team_id: %s, games: %q", store.JoinSeriesKey(setID, "s1"), team, games))
}

func seriesResult(round, team, games string) string {
	return fmt.Sprintf("results:\n    series:\n        %s:\n            s1: {winner: %s, games: %s}\n", round, team, games)
}

func TestPlayerPointsShouldScoreEachRule(t *testing.T) {
	tests := []struct {
		name    string
		picks   []string
		results string
		want    Points
	}{
		{"no results recorded yet scores nothing", []string{divisionList("FLA, TOR"), divisionWinner("FLA"), hartTrio, cupFLA, playoffFLA, seriesPick(store.Round1SetID, "FLA", "5")}, "", Points{}},
		{"a correct division winner in the list earns 15, not 20", []string{divisionList("FLA"), divisionWinner("FLA")}, atlanticFLAWon, Points{Regular: 15}},
		{"a correct division winner not in the list still earns 15", []string{divisionWinner("FLA")}, atlanticFLAWon, Points{Regular: 15}},
		{"a wrong winner pick that made the playoffs earns 5", []string{divisionList("TOR"), divisionWinner("TOR")}, atlanticFLAWon, Points{Regular: 5}},
		{"a wrong winner pick that is not in the list earns nothing", []string{divisionWinner("TOR")}, atlanticFLAWon, Points{}},
		{"each other listed playoff team earns 5", []string{divisionList("FLA, TOR, BOS"), divisionWinner("FLA")}, atlanticFLAWon, Points{Regular: 25}},
		{"a listed team that missed the playoffs earns nothing", []string{divisionList("BUF")}, atlanticFLAWon, Points{}},
		{"a listed team that won the division counts as making the playoffs", []string{divisionList("FLA"), divisionWinner("TOR")},
			"results:\n    team_marks:\n        atlantic: {playoffs: [TOR], division_winner: FLA}\n", Points{Regular: 5}},
		{"a listed team is not scored twice", []string{divisionList("TOR, TOR")}, atlanticFLAWon, Points{Regular: 5}},
		{"a tie expands the award finalists", []string{hartTrio}, hartWithATie, Points{Regular: 15}},
		{"each award pick found scores 5", []string{pick("kind: award, award: hart, finalist_slugs: [mcdavid-connor, draisaitl-leon, makar-cale]")}, hartWithATie, Points{Regular: 5}},
		{"award picks found in another award's finalists earn nothing", []string{pick("kind: award, award: norris, finalist_slugs: [mcdavid-connor, mackinnon-nathan, kucherov-nikita]")}, hartWithATie, Points{}},
		{"an empty award pick scores nothing", nil, hartWithATie, Points{}},
		{"the season-opening Cup pick scores 20 Regular", []string{cupFLA}, "results:\n    stanley_cup_winner: FLA\n", Points{Regular: 20}},
		{"the playoffs Cup pick scores 20 Playoff", []string{playoffFLA}, "results:\n    stanley_cup_winner: FLA\n", Points{Playoff: 20}},
		{"a wrong Cup pick scores nothing", []string{cupFLA, playoffFLA}, "results:\n    stanley_cup_winner: TOR\n", Points{}},
		{"the Presidents' Trophy pick scores 20", []string{pick("kind: presidents, team_id: TOR")}, "results:\n    presidents_trophy: TOR\n", Points{Regular: 20}},
		{"a wrong Presidents' Trophy pick scores nothing", []string{pick("kind: presidents, team_id: FLA")}, "results:\n    presidents_trophy: TOR\n", Points{}},
		{"a Cup pick is not scored against the Presidents' Trophy", []string{cupFLA}, "results:\n    presidents_trophy: FLA\n", Points{}},
		{"round 1 exact earns 25, not 40", []string{seriesPick(store.Round1SetID, "FLA", "5")}, seriesResult("round1", "FLA", "5"), Points{Playoff: 25}},
		{"round 1 winner only earns 15", []string{seriesPick(store.Round1SetID, "FLA", "6")}, seriesResult("round1", "FLA", "5"), Points{Playoff: 15}},
		{"round 2 exact earns 35", []string{seriesPick(store.Round2SetID, "FLA", "7")}, seriesResult("round2", "FLA", "7"), Points{Playoff: 35}},
		{"round 2 winner only earns 25", []string{seriesPick(store.Round2SetID, "FLA", "4")}, seriesResult("round2", "FLA", "7"), Points{Playoff: 25}},
		{"conference finals exact earns 45", []string{seriesPick(store.ConferenceFinalsSetID, "FLA", "5")}, seriesResult("round3", "FLA", "5"), Points{Playoff: 45}},
		{"conference finals winner only earns 30", []string{seriesPick(store.ConferenceFinalsSetID, "FLA", "6")}, seriesResult("round3", "FLA", "5"), Points{Playoff: 30}},
		{"Stanley Cup Final exact earns 50", []string{seriesPick(store.StanleyCupFinalSetID, "FLA", "6")}, seriesResult("round4", "FLA", "6"), Points{Playoff: 50}},
		{"Stanley Cup Final winner only earns 30", []string{seriesPick(store.StanleyCupFinalSetID, "FLA", "4")}, seriesResult("round4", "FLA", "6"), Points{Playoff: 30}},
		{"a wrong series winner with the right games earns nothing", []string{seriesPick(store.Round1SetID, "TOR", "5")}, seriesResult("round1", "FLA", "5"), Points{}},
		{"a series recorded for another round earns nothing", []string{seriesPick(store.Round2SetID, "FLA", "5")}, seriesResult("round1", "FLA", "5"), Points{}},
		{"a series row with a malformed key earns nothing", []string{pick("kind: series, series_key: s1, team_id: FLA, games: \"5\"")}, seriesResult("round1", "FLA", "5"), Points{}},
		{"a series row for an unknown round earns nothing", []string{pick("kind: series, series_key: r9.s1, team_id: FLA, games: \"5\"")}, seriesResult("round1", "FLA", "5"), Points{}},
		{"a hand-edited games: 05 still scores exact", []string{seriesPick(store.Round1SetID, "FLA", "5")}, seriesResult("round1", "FLA", "05"), Points{Playoff: 25}},
		{"empty cup, presidents and playoffs Cup picks score nothing", []string{pick(`kind: cup, team_id: ""`), pick(`kind: presidents, team_id: ""`), pick(`kind: playoffcup, team_id: ""`)}, "", Points{}},
		{"an award row with no finalist slugs scores nothing", []string{pick("kind: award, award: hart, finalist_slugs: []")}, hartWithATie, Points{}},
		{"a repeated award pick scores once", []string{pick("kind: award, award: hart, finalist_slugs: [mcdavid-connor, mcdavid-connor, mcdavid-connor]")}, hartWithATie, Points{Regular: 5}},
		{"a half-recorded series earns nothing", []string{seriesPick(store.Round1SetID, "FLA", "5")}, "results:\n    series:\n        round1:\n            s1: {winner: FLA}\n", Points{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, _ := newScoringStore(t, tt.picks, tt.results)

			got := PlayerPoints(st, "basti")

			if got != tt.want {
				t.Errorf("PlayerPoints = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPlayerPointsShouldScoreEveryAwardTheSameWay(t *testing.T) {
	for _, award := range []string{store.AwardHart, store.AwardNorris, store.AwardVezina, store.AwardArtRoss, store.AwardRocketRichard} {
		t.Run(award, func(t *testing.T) {
			picks := []string{pick("kind: award, award: " + award + ", finalist_slugs: [mcdavid-connor, mackinnon-nathan, kucherov-nikita]")}
			results := strings.Replace(hartWithATie, "    hart:", "    "+award+":", 1)
			st, _ := newScoringStore(t, picks, results)

			if got := PlayerPoints(st, "basti"); got != (Points{Regular: 15}) {
				t.Errorf("PlayerPoints = %+v, want 15 Regular", got)
			}
		})
	}
}

func TestPlayerPointsShouldScoreNothingForAPlayerNotInThePlayersList(t *testing.T) {
	seed := []string{strings.Replace(cupFLA, "player_id: basti", "player_id: ghost", 1)}
	st, _ := newScoringStore(t, seed, "results:\n    stanley_cup_winner: FLA\n")

	if got := PlayerPoints(st, "ghost"); got != (Points{}) {
		t.Errorf("PlayerPoints(ghost) = %+v, want zero", got)
	}
}

func TestPlayerPointsShouldNotScoreAnotherPlayersPicks(t *testing.T) {
	st, _ := newScoringStore(t, []string{cupFLA}, "results:\n    stanley_cup_winner: FLA\n")

	if got := PlayerPoints(st, "someone-else"); got != (Points{}) {
		t.Errorf("PlayerPoints(someone-else) = %+v, want zero", got)
	}
}

func TestPlayerPointsShouldReflectAPickSavedBetweenCalls(t *testing.T) {
	st, _ := newScoringStore(t, nil, "results:\n    stanley_cup_winner: FLA\n")
	before := PlayerPoints(st, "basti")

	if err := st.SavePrediction("basti", store.KindCupChampion, "FLA", time.Now()); err != nil {
		t.Fatalf("SavePrediction returned error: %v", err)
	}
	after := PlayerPoints(st, "basti")

	if before != (Points{}) || after != (Points{Regular: 20}) {
		t.Errorf("PlayerPoints before/after = %+v / %+v, want zero then 20 Regular", before, after)
	}
}

func TestPlayerPointsShouldNotWriteTheDataFile(t *testing.T) {
	st, path := newScoringStore(t, []string{cupFLA, hartTrio}, "results:\n    stanley_cup_winner: FLA\n"+hartWithATie)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}

	_ = PlayerPoints(st, "basti")

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("expected scoring to leave the data file's bytes unchanged")
	}
}

func TestTotalShouldAddRegularAndPlayoff(t *testing.T) {
	if got := (Points{Regular: 35, Playoff: 70}).Total(); got != 105 {
		t.Errorf("Total = %d, want 105", got)
	}
	if got := (Points{}).Total(); got != 0 {
		t.Errorf("Total of zero Points = %d, want 0", got)
	}
}
