Feature: Player Awards Finalists
  As a logged-in player,
  I want to pick 3 finalists for each of the 5 individual awards,
  so that a typo can never silently fail to score.

  Background:
    Given the signed-in player for award finalists is "Basti"
    And the canonical NHL Player list includes a sample of skaters, defensemen, and goalies

  Scenario: Opening an Open awards set with no prior picks shows five empty trophy groups
    Given the awards Prediction Set has a deadline "in 5 days"
    When the player opens the awards Prediction Set
    Then the awards sheet shows five trophy groups
    And the awards sheet shows no trophy group's green check
    And the awards sheet shows the button text "Submit predictions"

  Scenario: Submitting all 5 awards' finalists saves everything and returns to Predict Submitted
    Given the awards Prediction Set has a deadline "in 5 days"
    When the player submits valid finalists for all 5 awards
    Then the award pick response redirects to "/predict"
    And the player's saved finalists for "hart" are "mcdavid-connor,mackinnon-nathan,kucherov-nikita"
    And the Predict screen marks the awards set as "Submitted"

  Scenario: Leaving some awards blank saves only the complete ones
    Given the awards Prediction Set has a deadline "in 5 days"
    When the player submits valid finalists for only the Hart and Norris awards
    Then the award pick response redirects to "/predict"
    And the player's saved finalists for "hart" are "mcdavid-connor,mackinnon-nathan,kucherov-nikita"
    And the player has no saved finalists for "vezina"

  Scenario: A typed name that never resolves to a real NHL Player is rejected and nothing is saved
    Given the awards Prediction Set has a deadline "in 5 days"
    When the player submits valid finalists for all 5 awards but types an unresolved name for one Hart slot
    Then the award pick response status is 200
    And the awards sheet shows "Pick a name from the suggestions."
    And the player has no saved finalists for "hart"
    And the player has no saved finalists for "norris"

  Scenario: A resolved slug belonging to the wrong position is rejected and nothing is saved
    Given the awards Prediction Set has a deadline "in 5 days"
    When the player submits valid finalists for all 5 awards but submits a goalie's slug for a Hart slot
    Then the award pick response status is 200
    And the awards sheet shows "Pick a name from the suggestions."
    And the player has no saved finalists for "hart"

  Scenario: Submitting award picks after the deadline is rejected with no override
    Given the awards Prediction Set has a deadline "1 day ago"
    When the player submits valid finalists for all 5 awards
    Then the award pick response status is 403
    And the player has no saved finalists for "hart"

  Scenario: Reopening a submitted, still-open awards set pre-fills the completed award and reads Update predictions
    Given the awards Prediction Set has a deadline "in 5 days"
    And the player already submitted valid finalists for all 5 awards
    When the player opens the awards Prediction Set
    Then the awards sheet shows "Connor McDavid" preselected for "hart"
    And the awards sheet shows the trophy group's green check for "hart"
    And the awards sheet shows the button text "Update predictions"

  Scenario: Opening a closed awards set shows a read-only banner with no action bar
    Given the awards Prediction Set has a deadline "1 day ago"
    And the player already submitted valid finalists for all 5 awards
    When the player opens the awards Prediction Set
    Then the awards sheet shows a read-only banner
    And the awards sheet shows "Connor McDavid" preselected for "hart"
    And the awards sheet shows no submit button
