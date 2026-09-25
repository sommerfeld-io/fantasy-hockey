Feature: Automatic Scoring
  As a player in the pool,
  I want my Regular and Playoff points computed automatically from my picks and the recorded results,
  so that nobody has to score the pool by hand and a scoring error can't slip through.

  Background:
    Given the scoring player is "Basti"

  Scenario: No results recorded yet scores nothing
    Given the scoring player picked "FLA,TOR" as the "Atlantic" playoff teams
    And the scoring player picked "FLA" as the "Atlantic" division winner
    And the scoring player picked "FLA" for the "cup" pick
    And the scoring player picked "FLA" for the "playoffcup" pick
    And the scoring player picked "mcdavid-connor,mackinnon-nathan,kucherov-nikita" as the "hart" finalists
    When scoring runs for "Basti"
    Then the scoring result is 0 Regular points and 0 Playoff points

  Scenario: A correctly picked division winner earns 15, not 20
    Given the scoring player picked "FLA" as the "Atlantic" playoff teams
    And the scoring player picked "FLA" as the "Atlantic" division winner
    And the recorded "Atlantic" playoff teams are "FLA,TOR,BOS,TBL"
    And the recorded "Atlantic" division winner is "FLA"
    When scoring runs for "Basti"
    Then the scoring result is 15 Regular points and 0 Playoff points

  Scenario: A listed playoff team still earns 5 when the division winner pick is wrong
    Given the scoring player picked "TOR" as the "Atlantic" playoff teams
    And the scoring player picked "TOR" as the "Atlantic" division winner
    And the recorded "Atlantic" playoff teams are "FLA,TOR,BOS,TBL"
    And the recorded "Atlantic" division winner is "FLA"
    When scoring runs for "Basti"
    Then the scoring result is 5 Regular points and 0 Playoff points

  Scenario: A team that missed the playoffs earns nothing
    Given the scoring player picked "BUF" as the "Atlantic" playoff teams
    And the recorded "Atlantic" playoff teams are "FLA,TOR,BOS,TBL"
    And the recorded "Atlantic" division winner is "FLA"
    When scoring runs for "Basti"
    Then the scoring result is 0 Regular points and 0 Playoff points

  Scenario: A tie that expands an award's finalists still scores every matching pick
    Given the scoring player picked "mcdavid-connor,mackinnon-nathan,kucherov-nikita" as the "hart" finalists
    And the recorded "hart" finalists are "mcdavid-connor,mackinnon-nathan,matthews-auston,kucherov-nikita"
    When scoring runs for "Basti"
    Then the scoring result is 15 Regular points and 0 Playoff points

  Scenario: An empty award pick scores zero without failing
    Given the scoring player picked "mcdavid-connor,mackinnon-nathan,kucherov-nikita" as the "hart" finalists
    And the recorded "hart" finalists are "mcdavid-connor,draisaitl-leon,matthews-auston"
    And the recorded "norris" finalists are "makar-cale,hughes-quinn,werenski-zach"
    When scoring runs for "Basti"
    Then the scoring result is 5 Regular points and 0 Playoff points

  Scenario: The season-opening and playoffs Cup picks score into separate buckets
    Given the scoring player picked "FLA" for the "cup" pick
    And the scoring player picked "FLA" for the "playoffcup" pick
    And the scoring player picked "FLA" for the "presidents" pick
    And the recorded Stanley Cup winner is "FLA"
    And the recorded Presidents' Trophy winner is "TOR"
    When scoring runs for "Basti"
    Then the scoring result is 20 Regular points and 20 Playoff points

  Scenario: An exact series result earns the exact value, not exact plus winner
    Given the scoring player picked "FLA" in 5 games for series "s1" in "round 1"
    And the recorded series "s1" in "round 1" was won by "FLA" in 5 games
    When scoring runs for "Basti"
    Then the scoring result is 0 Regular points and 25 Playoff points

  Scenario: A correct winner with the wrong game count earns only the winner value
    Given the scoring player picked "FLA" in 6 games for series "s1" in "round 3"
    And the recorded series "s1" in "round 3" was won by "FLA" in 5 games
    When scoring runs for "Basti"
    Then the scoring result is 0 Regular points and 30 Playoff points

  Scenario: A wrong series winner earns nothing even with the right game count
    Given the scoring player picked "TOR" in 5 games for series "s1" in "round 1"
    And the recorded series "s1" in "round 1" was won by "FLA" in 5 games
    When scoring runs for "Basti"
    Then the scoring result is 0 Regular points and 0 Playoff points

  Scenario: An unknown player scores nothing
    Given the scoring player picked "FLA" for the "cup" pick
    And the recorded Stanley Cup winner is "FLA"
    When scoring runs for "ghost"
    Then the scoring result is 0 Regular points and 0 Playoff points

  Scenario: A player with every pick correct scores the maximum the point table allows
    Given the scoring player picked everything correctly for a fully recorded season
    When scoring runs for "Basti"
    Then the scoring result is 235 Regular points and 500 Playoff points

  Scenario: Scoring reflects a hand-edited result after a restart and never writes the data file
    Given the scoring player picked "FLA" for the "cup" pick
    And the recorded Stanley Cup winner is "TOR"
    When scoring runs for "Basti" before and after the recorded Stanley Cup winner is changed by hand to "FLA" and the app restarts
    Then the scoring results before and after the edit are 0 and 20 Regular points
    And the scoring data file is unchanged by scoring

  Scenario: A malformed series result is reported and scores nothing
    Given the scoring player picked "FLA" in 5 games for series "s1" in "round 1"
    And the recorded series "s1" in "round 1" was won by "FLA" in 9 games
    When scoring runs for "Basti"
    Then the scoring result is 0 Regular points and 0 Playoff points
    And the scoring data file reports a result problem mentioning "games"

  Scenario: A well-formed results section reports no problems
    Given the recorded series "s1" in "round 1" was won by "FLA" in 5 games
    And the recorded "hart" finalists are "mcdavid-connor,draisaitl-leon,matthews-auston"
    When scoring runs for "Basti"
    Then the scoring data file reports no result problems
