package acceptance_test

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/scoring"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// automaticScoringPlayerID/Name is the one seeded player every scenario in
// this feature scores.
const (
	automaticScoringPlayerID   = "basti"
	automaticScoringPlayerName = "Basti"
)

// automaticScoringSeries is one recorded (or picked) series outcome.
type automaticScoringSeries struct {
	round  seedRound
	key    string
	winner string
	games  string
}

// automaticScoringScenarioState accumulates one scenario's picks and
// results, writes them as seeded YAML on the first scoring run, and records
// each run's points.
type automaticScoringScenarioState struct {
	teamDiv     map[string]string
	slugs       []string
	predictions []seedPrediction
	pickSeries  []automaticScoringSeries
	resSeries   []automaticScoringSeries
	divisions   map[string]*seedDivisionMarks
	finalists   map[string][]string
	presidents  string
	cupWinner   string

	dataFile      string
	st            *store.Store
	points        []scoring.Points
	fileUntouched bool
}

func newAutomaticScoringScenarioState() *automaticScoringScenarioState {
	return &automaticScoringScenarioState{
		teamDiv:       map[string]string{},
		divisions:     map[string]*seedDivisionMarks{},
		finalists:     map[string][]string{},
		fileUntouched: true,
	}
}

// addTeam registers id as a canonical team in division (Atlantic unless the
// scenario says otherwise), so the store never flags it as unknown.
func (s *automaticScoringScenarioState) addTeam(id, division string) {
	if _, ok := s.teamDiv[id]; ok {
		return
	}
	if division == "" {
		division = store.Divisions()[0]
	}
	s.teamDiv[id] = division
}

func (s *automaticScoringScenarioState) addSlugs(slugs []string) {
	for _, slug := range slugs {
		if !slices.Contains(s.slugs, slug) {
			s.slugs = append(s.slugs, slug)
		}
	}
}

// addPrediction appends one of the scoring player's prediction rows from its
// kind-specific flow-mapping fields.
func (s *automaticScoringScenarioState) addPrediction(fields string) {
	s.predictions = append(s.predictions, seedPrediction{playerID: automaticScoringPlayerID, fields: fields})
}

func (s *automaticScoringScenarioState) division(name string) *seedDivisionMarks {
	d, ok := s.divisions[name]
	if !ok {
		d = &seedDivisionMarks{}
		s.divisions[name] = d
	}
	return d
}

func splitList(list string) []string {
	return strings.Split(list, ",")
}

func (s *automaticScoringScenarioState) theScoringPlayerIs(name string) error {
	return requireSeededPlayer(name, automaticScoringPlayerName)
}

func (s *automaticScoringScenarioState) pickedPlayoffTeams(list, division string) error {
	teams := splitList(list)
	for _, t := range teams {
		s.addTeam(t, division)
	}
	s.addPrediction(fmt.Sprintf("kind: %s, division: %s, team_ids: [%s]",
		store.KindDivisionPlayoffTeams, division, strings.Join(teams, ", ")))
	return nil
}

func (s *automaticScoringScenarioState) pickedDivisionWinner(team, division string) error {
	s.addTeam(team, division)
	s.addPrediction(fmt.Sprintf("kind: %s, division: %s, team_id: %s",
		store.KindDivisionWinner, division, team))
	return nil
}

func (s *automaticScoringScenarioState) pickedTeamFor(team, kind string) error {
	s.addTeam(team, "")
	s.addPrediction(fmt.Sprintf("kind: %s, team_id: %s", kind, team))
	return nil
}

func (s *automaticScoringScenarioState) pickedFinalists(list, award string) error {
	slugs := splitList(list)
	s.addSlugs(slugs)
	s.addPrediction(fmt.Sprintf("kind: %s, award: %s, finalist_slugs: [%s]",
		store.KindAward, award, strings.Join(slugs, ", ")))
	return nil
}

func (s *automaticScoringScenarioState) series(team, games, key, roundName string) (automaticScoringSeries, error) {
	round, ok := seedRounds[roundName]
	if !ok {
		return automaticScoringSeries{}, fmt.Errorf("unknown round %q", roundName)
	}
	s.addTeam(team, "")
	return automaticScoringSeries{round: round, key: key, winner: team, games: games}, nil
}

func (s *automaticScoringScenarioState) pickedSeries(team, games, key, roundName string) error {
	pick, err := s.series(team, games, key, roundName)
	if err != nil {
		return err
	}
	s.pickSeries = append(s.pickSeries, pick)
	s.addPrediction(fmt.Sprintf("kind: %s, series_key: %s, team_id: %s, games: %q",
		store.KindSeries, store.JoinSeriesKey(pick.round.setID, key), team, games))
	return nil
}

func (s *automaticScoringScenarioState) recordedSeries(key, roundName, team, games string) error {
	result, err := s.series(team, games, key, roundName)
	if err != nil {
		return err
	}
	s.resSeries = append(s.resSeries, result)
	return nil
}

func (s *automaticScoringScenarioState) recordedPlayoffTeams(division, list string) error {
	teams := splitList(list)
	for _, t := range teams {
		s.addTeam(t, division)
	}
	s.division(division).playoffs = teams
	return nil
}

func (s *automaticScoringScenarioState) recordedDivisionWinner(division, team string) error {
	s.addTeam(team, division)
	s.division(division).winner = team
	return nil
}

func (s *automaticScoringScenarioState) recordedFinalists(award, list string) error {
	slugs := splitList(list)
	s.addSlugs(slugs)
	s.finalists[award] = slugs
	return nil
}

func (s *automaticScoringScenarioState) recordedStanleyCupWinner(team string) error {
	s.addTeam(team, "")
	s.cupWinner = team
	return nil
}

func (s *automaticScoringScenarioState) recordedPresidentsTrophyWinner(team string) error {
	s.addTeam(team, "")
	s.presidents = team
	return nil
}

// pickedEverythingCorrectly seeds a full season: four playoff teams per
// division (winner listed first), all five awards, both trophies, the
// playoffs Cup pick and every series of every round, each pick matching its
// recorded result exactly.
func (s *automaticScoringScenarioState) pickedEverythingCorrectly() error {
	champion := fmt.Sprintf("%c01", store.Divisions()[0][0])
	return runAll(
		s.pickedAllDivisionsCorrectly,
		s.pickedAllAwardsCorrectly,
		func() error { return s.pickedAllTrophiesCorrectly(champion) },
		func() error { return s.pickedAllSeriesCorrectly(champion) },
	)
}

// runAll runs each step in order and stops at the first error.
func runAll(steps ...func() error) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// pickedAllDivisionsCorrectly picks and records four playoff teams per
// division, the first of them as the division winner.
func (s *automaticScoringScenarioState) pickedAllDivisionsCorrectly() error {
	for _, division := range store.Divisions() {
		teams := make([]string, 4)
		for i := range teams {
			teams[i] = fmt.Sprintf("%c%02d", division[0], i+1)
		}
		list := strings.Join(teams, ",")
		if err := runAll(
			func() error { return s.pickedPlayoffTeams(list, division) },
			func() error { return s.pickedDivisionWinner(teams[0], division) },
			func() error { return s.recordedPlayoffTeams(division, list) },
			func() error { return s.recordedDivisionWinner(division, teams[0]) },
		); err != nil {
			return err
		}
	}
	return nil
}

// pickedAllAwardsCorrectly picks and records the same finalist trio for
// every award.
func (s *automaticScoringScenarioState) pickedAllAwardsCorrectly() error {
	for _, award := range []string{store.AwardHart, store.AwardNorris, store.AwardVezina, store.AwardArtRoss, store.AwardRocketRichard} {
		trio := fmt.Sprintf("%s-a,%s-b,%s-c", award, award, award)
		if err := runAll(
			func() error { return s.pickedFinalists(trio, award) },
			func() error { return s.recordedFinalists(award, trio) },
		); err != nil {
			return err
		}
	}
	return nil
}

// pickedAllTrophiesCorrectly picks champion for the Cup, Presidents' Trophy
// and playoffs Cup picks and records it as both trophy winners.
func (s *automaticScoringScenarioState) pickedAllTrophiesCorrectly(champion string) error {
	for _, kind := range []string{store.KindCupChampion, store.KindPresidentsTrophy, store.KindPlayoffsCup} {
		if err := s.pickedTeamFor(champion, kind); err != nil {
			return err
		}
	}
	return runAll(
		func() error { return s.recordedStanleyCupWinner(champion) },
		func() error { return s.recordedPresidentsTrophyWinner(champion) },
	)
}

// pickedAllSeriesCorrectly picks and records champion in 5 games for every
// series of every round.
func (s *automaticScoringScenarioState) pickedAllSeriesCorrectly(champion string) error {
	seriesPerRound := map[string]int{"round 1": 8, "round 2": 4, "round 3": 2, "round 4": 1}
	for _, roundName := range slices.Sorted(maps.Keys(seriesPerRound)) {
		count := seriesPerRound[roundName]
		for i := 1; i <= count; i++ {
			key := fmt.Sprintf("s%d", i)
			if err := runAll(
				func() error { return s.pickedSeries(champion, "5", key, roundName) },
				func() error { return s.recordedSeries(key, roundName, champion, "5") },
			); err != nil {
				return err
			}
		}
	}
	return nil
}

// seed renders every accumulated fixture as one fantasy-hockey.yml document.
func (s *automaticScoringScenarioState) seed() string {
	var b strings.Builder
	b.WriteString(seedHeader(automaticScoringPlayerID, automaticScoringPlayerName))
	writeSeedTeams(&b, s.teamDiv)
	writeSeedNHLPlayers(&b, s.slugs)
	writeSeedMatchups(&b, s.matchups())
	writeSeedPredictions(&b, s.predictions)
	writeSeedResults(&b, seedResults{
		teamMarks:  s.divisions,
		presidents: s.presidents,
		cupWinner:  s.cupWinner,
		series:     s.seriesResults(),
	})
	writeSeedFinalists(&b, s.finalists)
	return b.String()
}

// matchups declares a playoff_matchups entry for every series that is
// picked or recorded, with every team picked or recorded for it as one of
// its two sides, so no series key or winner is flagged as a problem.
func (s *automaticScoringScenarioState) matchups() map[string][]seedMatchup {
	keysBySet := map[string][]string{}
	teamsByKey := map[string][]string{}
	for _, series := range slices.Concat(s.pickSeries, s.resSeries) {
		seriesKey := store.JoinSeriesKey(series.round.setID, series.key)
		if _, seen := teamsByKey[seriesKey]; !seen {
			keysBySet[series.round.setID] = append(keysBySet[series.round.setID], series.key)
		}
		if !slices.Contains(teamsByKey[seriesKey], series.winner) {
			teamsByKey[seriesKey] = append(teamsByKey[seriesKey], series.winner)
		}
	}
	bySet := map[string][]seedMatchup{}
	for setID, keys := range keysBySet {
		for _, key := range keys {
			teams := teamsByKey[store.JoinSeriesKey(setID, key)]
			bySet[setID] = append(bySet[setID], seedMatchup{key: key, a: teams[0], b: teams[len(teams)-1]})
		}
	}
	return bySet
}

// seriesResults groups the recorded series by results-side round name.
func (s *automaticScoringScenarioState) seriesResults() map[string][]seedSeriesResult {
	byRound := map[string][]seedSeriesResult{}
	for _, r := range s.resSeries {
		byRound[r.round.resultRound] = append(byRound[r.round.resultRound], seedSeriesResult{key: r.key, winner: r.winner, games: r.games})
	}
	return byRound
}

// ensureStore writes the seed and opens the store once per scenario.
func (s *automaticScoringScenarioState) ensureStore() error {
	if s.st != nil {
		return nil
	}
	s.dataFile = newScenarioDataFile("automatic-scoring")
	st, err := writeSeededStore(s.dataFile, s.seed())
	if err != nil {
		return err
	}
	s.st = st
	return nil
}

// scoreOnce runs scoring for playerID and checks the data file's bytes are
// identical before and after.
func (s *automaticScoringScenarioState) scoreOnce(playerID string) error {
	before, err := os.ReadFile(s.dataFile)
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	s.points = append(s.points, scoring.PlayerPoints(s.st, playerID))
	after, err := os.ReadFile(s.dataFile)
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	if !bytes.Equal(before, after) {
		s.fileUntouched = false
	}
	return nil
}

func (s *automaticScoringScenarioState) scoringRunsFor(name string) error {
	if err := s.ensureStore(); err != nil {
		return err
	}
	playerID := name
	if name == automaticScoringPlayerName {
		playerID = automaticScoringPlayerID
	}
	return s.scoreOnce(playerID)
}

// scoringRunsBeforeAndAfterACupEdit scores, then hand-edits the recorded
// Stanley Cup winner and restarts the store, then scores again. Results are
// loaded only at startup, so a hand edit takes effect after a restart.
func (s *automaticScoringScenarioState) scoringRunsBeforeAndAfterACupEdit(name, team string) error {
	if err := s.scoringRunsFor(name); err != nil {
		return err
	}
	if err := s.recordedStanleyCupWinner(team); err != nil {
		return err
	}
	st, err := writeSeededStore(s.dataFile, s.seed())
	if err != nil {
		return err
	}
	s.st = st
	return s.scoreOnce(automaticScoringPlayerID)
}

func (s *automaticScoringScenarioState) theScoringResultIs(regular, playoff int) error {
	if len(s.points) == 0 {
		return fmt.Errorf("scoring never ran")
	}
	got := s.points[len(s.points)-1]
	if got.Regular != regular || got.Playoff != playoff {
		return fmt.Errorf("expected %d Regular / %d Playoff points, got %d / %d", regular, playoff, got.Regular, got.Playoff)
	}
	if got.Total() != regular+playoff {
		return fmt.Errorf("expected Total %d, got %d", regular+playoff, got.Total())
	}
	return nil
}

func (s *automaticScoringScenarioState) theResultsBeforeAndAfterTheEditAre(before, after int) error {
	if len(s.points) != 2 {
		return fmt.Errorf("expected exactly 2 scoring runs, got %d", len(s.points))
	}
	if s.points[0].Regular != before || s.points[1].Regular != after {
		return fmt.Errorf("expected Regular %d then %d, got %d then %d", before, after, s.points[0].Regular, s.points[1].Regular)
	}
	return nil
}

func (s *automaticScoringScenarioState) theDataFileIsUnchangedByScoring() error {
	if !s.fileUntouched {
		return fmt.Errorf("expected scoring to leave the data file's bytes unchanged")
	}
	return nil
}

func (s *automaticScoringScenarioState) reportsAResultProblemMentioning(text string) error {
	for _, p := range s.st.ResultProblems() {
		if strings.Contains(p, text) {
			return nil
		}
	}
	return fmt.Errorf("expected a result problem mentioning %q, got %v", text, s.st.ResultProblems())
}

func (s *automaticScoringScenarioState) reportsNoResultProblems() error {
	if problems := s.st.ResultProblems(); len(problems) != 0 {
		return fmt.Errorf("expected no result problems, got %v", problems)
	}
	return nil
}

// InitializeAutomaticScoringScenario registers automatic-scoring.feature's
// steps. Scoring has no HTTP surface, so steps call scoring.PlayerPoints on
// a store seeded from YAML directly.
func InitializeAutomaticScoringScenario(ctx *godog.ScenarioContext) {
	s := newAutomaticScoringScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		if s.dataFile != "" {
			removeScenarioDataFile(s.dataFile)
		}
		return gctx, nil
	})

	ctx.Step(`^the scoring player is "([^"]*)"$`, s.theScoringPlayerIs)
	ctx.Step(`^the scoring player picked "([^"]*)" as the "([^"]*)" playoff teams$`, s.pickedPlayoffTeams)
	ctx.Step(`^the scoring player picked "([^"]*)" as the "([^"]*)" division winner$`, s.pickedDivisionWinner)
	ctx.Step(`^the scoring player picked "([^"]*)" for the "([^"]*)" pick$`, s.pickedTeamFor)
	ctx.Step(`^the scoring player picked "([^"]*)" as the "([^"]*)" finalists$`, s.pickedFinalists)
	ctx.Step(`^the scoring player picked "([^"]*)" in (\d+) games for series "([^"]*)" in "([^"]*)"$`, s.pickedSeries)
	ctx.Step(`^the scoring player picked everything correctly for a fully recorded season$`, s.pickedEverythingCorrectly)
	ctx.Step(`^the recorded "([^"]*)" playoff teams are "([^"]*)"$`, s.recordedPlayoffTeams)
	ctx.Step(`^the recorded "([^"]*)" division winner is "([^"]*)"$`, s.recordedDivisionWinner)
	ctx.Step(`^the recorded "([^"]*)" finalists are "([^"]*)"$`, s.recordedFinalists)
	ctx.Step(`^the recorded Stanley Cup winner is "([^"]*)"$`, s.recordedStanleyCupWinner)
	ctx.Step(`^the recorded Presidents' Trophy winner is "([^"]*)"$`, s.recordedPresidentsTrophyWinner)
	ctx.Step(`^the recorded series "([^"]*)" in "([^"]*)" was won by "([^"]*)" in (\d+) games$`, s.recordedSeries)
	ctx.Step(`^scoring runs for "([^"]*)"$`, s.scoringRunsFor)
	ctx.Step(`^scoring runs for "([^"]*)" before and after the recorded Stanley Cup winner is changed by hand to "([^"]*)" and the app restarts$`, s.scoringRunsBeforeAndAfterACupEdit)
	ctx.Step(`^the scoring result is (\d+) Regular points and (\d+) Playoff points$`, s.theScoringResultIs)
	ctx.Step(`^the scoring results before and after the edit are (\d+) and (\d+) Regular points$`, s.theResultsBeforeAndAfterTheEditAre)
	ctx.Step(`^the scoring data file is unchanged by scoring$`, s.theDataFileIsUnchangedByScoring)
	ctx.Step(`^the scoring data file reports a result problem mentioning "([^"]*)"$`, s.reportsAResultProblemMentioning)
	ctx.Step(`^the scoring data file reports no result problems$`, s.reportsNoResultProblems)
}
