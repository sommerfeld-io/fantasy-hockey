Feature: Cup Champion and Presidents' Trophy Picks
  As a logged-in player,
  I want to pick my Stanley Cup champion and Presidents' Trophy winner from the season's teams,
  so that I'm scored against the actual result once the season plays out.

  Background:
    Given the signed-in player for cup and presidents picks is "Basti"
    And the canonical team list includes "TOR" in the "Atlantic" division
    And the canonical team list includes "VGK" in the "Pacific" division

  Scenario: Opening an Open cup set with no prior pick shows an empty, submittable form
    Given the "cup" Prediction Set has a deadline "in 5 days"
    When the player opens the cup-picks Prediction Set "cup"
    Then the pick sheet shows a team dropdown grouped by division
    And the pick sheet shows no team preselected
    And the pick sheet shows the button text "Submit predictions"

  Scenario: Submitting a valid pick saves it and returns to Predict with the set Submitted
    Given the "cup" Prediction Set has a deadline "in 5 days"
    When the player submits team "TOR" for the Prediction Set "cup"
    Then the pick response redirects to "/predict"
    And the player's saved pick for "cup" is "TOR"
    And the Predict screen shows the set row for "cup" with status "Submitted"

  Scenario: Reopening a Submitted set pre-fills the pick and reads "Update predictions"
    Given the "cup" Prediction Set has a deadline "in 5 days"
    And the player already picked "TOR" for "cup"
    When the player opens the cup-picks Prediction Set "cup"
    Then the pick sheet shows the team "TOR" preselected
    And the pick sheet shows the button text "Update predictions"

  Scenario: Submitting a valid pick for the Presidents' Trophy set saves it and returns to Predict with the set Submitted
    Given the "presidents" Prediction Set has a deadline "in 5 days"
    When the player submits team "VGK" for the Prediction Set "presidents"
    Then the pick response redirects to "/predict"
    And the player's saved pick for "presidents" is "VGK"
    And the Predict screen shows the set row for "presidents" with status "Submitted"

  Scenario: Resubmitting updates the existing pick rather than creating a second one
    Given the "cup" Prediction Set has a deadline "in 5 days"
    And the player already picked "TOR" for "cup"
    When the player submits team "VGK" for the Prediction Set "cup"
    Then the pick response redirects to "/predict"
    And the player's saved pick for "cup" is "VGK"

  Scenario: Submitting an unknown team id is rejected and nothing is saved
    Given the "cup" Prediction Set has a deadline "in 5 days"
    When the player submits team "ZZZ" for the Prediction Set "cup"
    Then the pick response status is 200
    And the pick sheet shows "Pick a team before submitting."
    And the player has no saved pick for "cup"

  Scenario: Submitting after the deadline is rejected with no override
    Given the "cup" Prediction Set has a deadline "1 day ago"
    When the player submits team "TOR" for the Prediction Set "cup"
    Then the pick response status is 403
    And the player has no saved pick for "cup"

  Scenario: Opening a closed set shows a read-only banner with no action bar
    Given the "cup" Prediction Set has a deadline "1 day ago"
    And the player already picked "TOR" for "cup"
    When the player opens the cup-picks Prediction Set "cup"
    Then the pick sheet shows a read-only banner
    And the pick sheet shows the team "TOR" preselected
    And the pick sheet shows no submit button

  Scenario: A Prediction Set id that isn't cup, presidents, divisions, or awards still shows the static stub
    Given a stub Prediction Set "playoffcup" titled "Playoffs Cup pick" with a deadline "in 5 days"
    When the player opens the cup-picks Prediction Set "playoffcup"
    Then the pick sheet shows "Not available yet."
    And the pick sheet shows no team dropdown

  Scenario: An unknown Prediction Set id gets a generic not-found response on submit
    When the player submits team "TOR" for the Prediction Set "does-not-exist"
    Then the pick response status is 404

  Scenario: Never opening a pick never force-creates a Prediction row
    Given the "cup" Prediction Set has a deadline "in 5 days"
    When the player opens the cup-picks Prediction Set "cup"
    Then the player has no saved pick for "cup"
