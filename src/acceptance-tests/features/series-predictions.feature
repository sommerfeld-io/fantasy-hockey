Feature: Per-Round Series Predictions
  As a logged-in player,
  I want to pick the winner and game count for each recorded playoff series,
  so that my picks for round 1, round 2, the Conference Finals, and the Stanley Cup Final all count.

  Background:
    Given the signed-in player for series predictions is "Basti"
    And the canonical team list for series predictions includes teams from both conferences

  Scenario: Round 1's 8 recorded series render grouped 4 Eastern and 4 Western
    Given the round 1 Prediction Set has a deadline "in 5 days"
    And round 1 has its 8 series matchups recorded
    When the player opens the round 1 Prediction Set
    Then the series sheet shows 8 series cards
    And the series sheet shows 4 series cards under "Eastern Conference"
    And the series sheet shows 4 series cards under "Western Conference"

  Scenario: The Stanley Cup Final's one recorded series renders under its own heading, not a conference
    Given the Stanley Cup Final Prediction Set has a deadline "in 5 days"
    And the Stanley Cup Final has its 1 series matchup recorded
    When the player opens the Stanley Cup Final Prediction Set
    Then the series sheet shows 1 series card
    And the series sheet shows a "Stanley Cup Final" heading
    And the series sheet shows no conference heading

  Scenario: Submitting a winner and game count for one series saves it and marks round 1 Submitted
    Given the round 1 Prediction Set has a deadline "in 5 days"
    And round 1 has its 8 series matchups recorded
    When the player picks "BOS" in "6" games for series "e1"
    Then the series pick response redirects to "/predict"
    And the player's saved series pick for "e1" is "BOS" in "6" games
    And the Predict screen marks round 1 as "Submitted"

  Scenario: A half-filled series pick is silently skipped but another valid series in the same submission still saves
    Given the round 1 Prediction Set has a deadline "in 5 days"
    And round 1 has its 8 series matchups recorded
    When the player submits only the team "BOS" for series "e1" and a valid pick of "CAR" in "6" games for series "e3"
    Then the series pick response redirects to "/predict"
    And the player has no saved series pick for "e1"
    And the player's saved series pick for "e3" is "CAR" in "6" games

  Scenario: An invalid series pick is rejected on its own card but another valid series in the same submission still saves
    Given the round 1 Prediction Set has a deadline "in 5 days"
    And round 1 has its 8 series matchups recorded
    When the player submits the foreign team "COL" in "6" games for series "e1" and a valid pick of "TBL" in "5" games for series "e2"
    Then the series pick response status is 200
    And the series sheet shows an inline error on series "e1"
    And the player has no saved series pick for "e1"
    And the player's saved series pick for "e2" is "TBL" in "5" games

  Scenario: Opening a closed round shows a read-only banner with no action bar
    Given the round 1 Prediction Set has a deadline "1 day ago"
    And round 1 has its 8 series matchups recorded
    And the player already picked "BOS" in "6" games for series "e1"
    When the player opens the round 1 Prediction Set
    Then the series sheet shows a read-only banner
    And the series sheet shows no submit button
