Feature: Compare Predictions Side by Side
  As a player in the pool,
  I want to pick a Prediction Set on the Compare tab and see everyone's picks for it side by side,
  so that I can see who called what.

  Background:
    Given the compare pool players are "Basti", "Sadl" and "Tobbi"
    And a compare Prediction Set "cup" titled "Cup champion" in phase "before_season" with a deadline "in 5 days"
    And a compare Prediction Set "presidents" titled "Presidents' Trophy" in phase "before_season" with a deadline "in 6 days"
    And a compare Prediction Set "divisions" titled "Division picks" in phase "before_season" with a deadline "in 7 days"
    And a compare Prediction Set "awards" titled "Player awards" in phase "before_season" with a deadline "in 8 days"
    And a compare Prediction Set "playoffcup" titled "Playoffs Cup pick" in phase "playoffs" with a deadline "in 200 days"
    And a compare Prediction Set "r1" titled "Playoff round 1" in phase "playoffs" with a deadline "in 203 days"
    And an upcoming compare Prediction Set "r2" titled "Playoff round 2" in phase "playoffs" with a deadline "in 215 days"

  Scenario: The selector groups every selectable set into two labelled rows
    When "Basti" opens Compare
    Then the Compare section header reads "Everyone's picks."
    And the Compare "Before the season" row shows the chips "Cup champion, Presidents' Trophy, Division picks, Player awards"
    And the Compare "Playoffs" row shows the chips "Playoffs Cup pick, Playoff round 1"

  Scenario: A gated round gets its chip once its matchups are recorded
    Given the round 2 matchups for compare are "s1: FLA vs TOR" and "s2: COL vs VGK"
    When "Basti" opens Compare
    Then the Compare "Playoffs" row shows the chips "Playoffs Cup pick, Playoff round 1, Playoff round 2"

  Scenario: The earliest before-the-season set is selected on first entry
    Given "Sadl" picked "FLA" for the compare "cup" pick
    When "Basti" opens Compare
    Then only the "Cup champion" chip is selected on Compare
    And the Compare deadline is the deadline of "cup"
    And the Compare table shows:
      | category           | Basti | Sadl     | Tobbi |
      | Stanley Cup winner | —     | Team FLA | —     |

  Scenario: The default is the earliest-deadline before-the-season set, not the first listed
    Given a compare Prediction Set "early" titled "Early bird" in phase "before_season" with a deadline "in 2 days"
    When "Basti" opens Compare
    Then only the "Early bird" chip is selected on Compare
    And the Compare deadline is the deadline of "early"

  Scenario: Selecting a set shows its deadline, one column per player and my column marked
    Given "Sadl" picked "TOR" for the compare "presidents" pick
    And "Tobbi" picked "FLA" for the compare "presidents" pick
    When "Basti" opens Compare
    And "Basti" selects the "Presidents' Trophy" chip on Compare
    Then only the "Presidents' Trophy" chip is selected on Compare
    And the Compare deadline is the deadline of "presidents"
    And the Compare table shows:
      | category           | Basti | Sadl     | Tobbi    |
      | Presidents' Trophy | —     | Team TOR | Team FLA |
    And only the Compare column for "Basti" is marked as the signed-in player's own

  Scenario: Another player sees my column marked as theirs, not mine
    When "Sadl" opens Compare
    Then only the Compare column for "Sadl" is marked as the signed-in player's own

  Scenario: Division picks get a playoff-teams row and a winner row per division
    Given "Sadl" picked "FLA, TOR, BOS, TBL" as the compare "Atlantic" playoff teams
    And "Sadl" picked "FLA" as the compare "Atlantic" division winner
    And "Tobbi" picked "COL" as the compare "Central" division winner
    When "Basti" opens Compare with the set "divisions"
    Then the Compare table shows:
      | category                     | Basti | Sadl               | Tobbi |
      | Atlantic — playoff teams     | —     | FLA, TOR, BOS, TBL | —     |
      | Atlantic — winner            | —     | FLA                | —     |
      | Metropolitan — playoff teams | —     | —                  | —     |
      | Metropolitan — winner        | —     | —                  | —     |
      | Central — playoff teams      | —     | —                  | —     |
      | Central — winner             | —     | —                  | COL   |
      | Pacific — playoff teams      | —     | —                  | —     |
      | Pacific — winner             | —     | —                  | —     |

  Scenario: Player awards get one row per award with finalist names
    Given "Basti" picked "mcdavid-connor, mackinnon-nathan, kucherov-nikita" as the compare "hart" finalists
    When "Basti" opens Compare with the set "awards"
    Then the Compare table shows:
      | category                 | Basti                                                                  | Sadl | Tobbi |
      | Hart Trophy finalists    | Player mcdavid-connor, Player mackinnon-nathan, Player kucherov-nikita | —    | —     |
      | Norris Trophy finalists  | —                                                                      | —    | —     |
      | Vezina Trophy finalists  | —                                                                      | —    | —     |
      | Art Ross finalists       | —                                                                      | —    | —     |
      | Rocket Richard finalists | —                                                                      | —    | —     |

  Scenario: A playoff round gets one row per recorded series
    Given the round 1 matchups for compare are "s1: FLA vs TOR" and "s2: COL vs VGK"
    And "Sadl" picked "FLA" in 5 games for compare round 1 series "s1"
    And "Tobbi" picked "VGK" in 7 games for compare round 1 series "s2"
    When "Basti" opens Compare with the set "r1"
    Then the Compare table shows:
      | category             | Basti | Sadl     | Tobbi    |
      | Eastern · FLA vs TOR | —     | FLA in 5 | —        |
      | Western · COL vs VGK | —     | —        | VGK in 7 |

  Scenario: The Playoffs Cup pick is labelled as a Stanley Cup winner row
    Given "Tobbi" picked "COL" for the compare "playoffcup" pick
    When "Basti" opens Compare with the set "playoffcup"
    Then only the "Playoffs Cup pick" chip is selected on Compare
    And the Compare table shows:
      | category           | Basti | Sadl | Tobbi    |
      | Stanley Cup winner | —     | —    | Team COL |

  Scenario: An unknown set id falls back to the default selection
    When "Basti" opens Compare with the set "nope"
    Then only the "Cup champion" chip is selected on Compare
    And the Compare deadline is the deadline of "cup"

  Scenario: A still-gated round's id falls back to the default selection
    When "Basti" opens Compare with the set "r2"
    Then only the "Cup champion" chip is selected on Compare
    And the Compare deadline is the deadline of "cup"

  Scenario: A set with an unparseable deadline is left out of the chips
    Given a compare Prediction Set "broken" titled "Broken set" in phase "before_season" with the raw deadline "not-a-date"
    When "Basti" opens Compare
    Then the Compare "Before the season" row shows the chips "Cup champion, Presidents' Trophy, Division picks, Player awards"

  Scenario: A stale session still renders Compare with no column marked as own
    When a player whose account no longer exists opens Compare
    Then the Compare section header reads "Everyone's picks."
    And no Compare column is marked as the signed-in player's own
