package acceptance_test

import (
	"bytes"
	"context"
	"fmt"
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

// automaticScoringSubmittedAt stamps every seeded prediction row; scoring
// never reads it.
const automaticScoringSubmittedAt = "2026-09-20T10:00:00Z"

// automaticScoringRound is one playoff round as the feature names it
// ("round 1"), with the prediction-side set id and the results-side round
// name it maps to.
type automaticScoringRound struct {
	setID, resultRound string
}

// automaticScoringRounds maps the feature's round wording to the ids the
// seeded YAML needs on each side.
var automaticScoringRounds = map[string]automaticScoringRound{
	"round 1": {store.Round1SetID, "round1"},
	"round 2": {store.Round2SetID, "round2"},
	"round 3": {store.ConferenceFinalsSetID, "round3"},
	"round 4": {store.StanleyCupFinalSetID, "round4"},
}

// automaticScoringSeries is one recorded (or picked) series outcome.
type automaticScoringSeries struct {
	round  automaticScoringRound
	key    string
	winner string
	games  string
}

// automaticScoringDivision is one division's recorded team marks.
type automaticScoringDivision struct {
	playoffs []string
	winner   string
}

// automaticScoringScenarioState accumulates one scenario's picks and
// results, writes them as seeded YAML on the first scoring run, and records
// each run's points.
type automaticScoringScenarioState struct {
	teams       []string
	teamDiv     map[string]string
	slugs       []string
	predictions []string
	pickSeries  []automaticScoringSeries
	resSeries   []automaticScoringSeries
	divisions   map[string]*automaticScoringDivision
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
		divisions:     map[string]*automaticScoringDivision{},
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
	s.teams = append(s.teams, id)
	s.teamDiv[id] = division
}

func (s *automaticScoringScenarioState) addSlugs(slugs []string) {
	for _, slug := range slugs {
		if !slices.Contains(s.slugs, slug) {
			s.slugs = append(s.slugs, slug)
		}
	}
}

func (s *automaticScoringScenarioState) addPrediction(fields string) {
	id := fmt.Sprintf("p%d", len(s.predictions)+1)
	s.predictions = append(s.predictions, fmt.Sprintf(
		"    - id: %s\n      player_id: %s\n      submitted_at: %q\n%s", id, automaticScoringPlayerID, automaticScoringSubmittedAt, fields))
}

func (s *automaticScoringScenarioState) division(name string) *automaticScoringDivision {
	d, ok := s.divisions[name]
	if !ok {
		d = &automaticScoringDivision{}
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
	s.addPrediction(fmt.Sprintf("      kind: %s\n      division: %s\n      team_ids: [%s]\n",
		store.KindDivisionPlayoffTeams, division, strings.Join(teams, ", ")))
	return nil
}

func (s *automaticScoringScenarioState) pickedDivisionWinner(team, division string) error {
	s.addTeam(team, division)
	s.addPrediction(fmt.Sprintf("      kind: %s\n      division: %s\n      team_id: %s\n",
		store.KindDivisionWinner, division, team))
	return nil
}

func (s *automaticScoringScenarioState) pickedTeamFor(team, kind string) error {
	s.addTeam(team, "")
	s.addPrediction(fmt.Sprintf("      kind: %s\n      team_id: %s\n", kind, team))
	return nil
}

func (s *automaticScoringScenarioState) pickedFinalists(list, award string) error {
	slugs := splitList(list)
	s.addSlugs(slugs)
	s.addPrediction(fmt.Sprintf("      kind: %s\n      award: %s\n      finalist_slugs: [%s]\n",
		store.KindAward, award, strings.Join(slugs, ", ")))
	return nil
}

func (s *automaticScoringScenarioState) series(team, games, key, roundName string) (automaticScoringSeries, error) {
	round, ok := automaticScoringRounds[roundName]
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
	s.addPrediction(fmt.Sprintf("      kind: %s\n      series_key: %s\n      team_id: %s\n      games: %q\n",
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
	for roundName, count := range seriesPerRound {
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
	s.writeTeams(&b)
	s.writeNHLPlayers(&b)
	s.writeMatchups(&b)
	b.WriteString("predictions:\n")
	for _, p := range s.predictions {
		b.WriteString(p)
	}
	s.writeResults(&b)
	s.writeFinalists(&b)
	return b.String()
}

func (s *automaticScoringScenarioState) writeTeams(b *strings.Builder) {
	b.WriteString("teams:\n")
	for _, id := range s.teams {
		fmt.Fprintf(b, "    - id: %s\n      name: Team %s\n      conference: Eastern\n      division: %s\n", id, id, s.teamDiv[id])
	}
}

func (s *automaticScoringScenarioState) writeNHLPlayers(b *strings.Builder) {
	b.WriteString("nhl_players:\n")
	for _, slug := range s.slugs {
		fmt.Fprintf(b, "    - slug: %s\n      display_name: Player %s\n      position: skater\n", slug, slug)
	}
}

// writeMatchups declares a playoff_matchups entry for every series that is
// picked or recorded, with every team picked or recorded for it as one of
// its two sides, so no series key or winner is flagged as a problem.
func (s *automaticScoringScenarioState) writeMatchups(b *strings.Builder) {
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
	if len(keysBySet) == 0 {
		return
	}
	b.WriteString("playoff_matchups:\n")
	for setID, keys := range keysBySet {
		fmt.Fprintf(b, "    %s:\n", setID)
		for _, key := range keys {
			teams := teamsByKey[store.JoinSeriesKey(setID, key)]
			fmt.Fprintf(b, "        - key: %s\n          a: %s\n          b: %s\n", key, teams[0], teams[len(teams)-1])
		}
	}
}

func (s *automaticScoringScenarioState) writeResults(b *strings.Builder) {
	if len(s.divisions) == 0 && s.presidents == "" && s.cupWinner == "" && len(s.resSeries) == 0 {
		return
	}
	b.WriteString("results:\n")
	if len(s.divisions) > 0 {
		b.WriteString("    team_marks:\n")
		for name, d := range s.divisions {
			fmt.Fprintf(b, "        %s:\n            playoffs: [%s]\n", strings.ToLower(name), strings.Join(d.playoffs, ", "))
			if d.winner != "" {
				fmt.Fprintf(b, "            division_winner: %s\n", d.winner)
			}
		}
	}
	if s.presidents != "" {
		fmt.Fprintf(b, "    presidents_trophy: %s\n", s.presidents)
	}
	if s.cupWinner != "" {
		fmt.Fprintf(b, "    stanley_cup_winner: %s\n", s.cupWinner)
	}
	s.writeSeriesResults(b)
}

// writeSeriesResults writes games unquoted, the way a human hand-edits it.
func (s *automaticScoringScenarioState) writeSeriesResults(b *strings.Builder) {
	if len(s.resSeries) == 0 {
		return
	}
	byRound := map[string][]automaticScoringSeries{}
	for _, r := range s.resSeries {
		byRound[r.round.resultRound] = append(byRound[r.round.resultRound], r)
	}
	b.WriteString("    series:\n")
	for round, results := range byRound {
		fmt.Fprintf(b, "        %s:\n", round)
		for _, r := range results {
			fmt.Fprintf(b, "            %s: {winner: %s, games: %s}\n", r.key, r.winner, r.games)
		}
	}
}

func (s *automaticScoringScenarioState) writeFinalists(b *strings.Builder) {
	if len(s.finalists) == 0 {
		return
	}
	b.WriteString("award_finalists:\n")
	for award, slugs := range s.finalists {
		fmt.Fprintf(b, "    %s:\n", award)
		for _, slug := range slugs {
			fmt.Fprintf(b, "        - slug: %s\n          display_name: Player %s\n", slug, slug)
		}
	}
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
