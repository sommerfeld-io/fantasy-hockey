Feature: Playoffs Cup Pick
  As a logged-in player,
  I want to make a second Stanley Cup champion pick once the playoff field is set,
  so that I get a better-informed shot at the Cup, without disturbing my season-opening pick.

  Background:
    Given the signed-in player for the playoffs Cup pick is "Basti"
    And the canonical team list for the playoffs Cup pick includes "TOR" in the "Atlantic" division
    And the canonical team list for the playoffs Cup pick includes "VGK" in the "Pacific" division

  Scenario: Opening an Open Playoffs Cup pick with no prior pick shows an empty, submittable form
    Given the Playoffs Cup pick has a deadline "in 5 days"
    When the player opens the Playoffs Cup pick sheet
    Then the playoffs pick sheet shows a team dropdown grouped by division
    And the playoffs pick sheet shows no team preselected
    And the playoffs pick sheet shows the button text "Submit predictions"

  Scenario: Submitting a valid Playoffs Cup pick saves it and returns to Predict with the set Submitted
    Given the Playoffs Cup pick has a deadline "in 5 days"
    When the player submits team "TOR" for the Playoffs Cup pick
    Then the playoffs pick response redirects to "/predict"
    And the player's saved Playoffs Cup pick is "TOR"
    And the Predict screen shows the playoffs Cup set row with status "Submitted"

  Scenario: Reopening a Submitted Playoffs Cup pick pre-fills the pick and reads "Update predictions"
    Given the Playoffs Cup pick has a deadline "in 5 days"
    And the player already picked "TOR" for the Playoffs Cup pick
    When the player opens the Playoffs Cup pick sheet
    Then the playoffs pick sheet shows the team "TOR" preselected
    And the playoffs pick sheet shows the button text "Update predictions"

  Scenario: Resubmitting a Playoffs Cup pick updates the existing pick rather than creating a second one
    Given the Playoffs Cup pick has a deadline "in 5 days"
    And the player already picked "TOR" for the Playoffs Cup pick
    When the player submits team "VGK" for the Playoffs Cup pick
    Then the playoffs pick response redirects to "/predict"
    And the player's saved Playoffs Cup pick is "VGK"

  Scenario: Opening a closed Playoffs Cup pick shows a read-only banner with no action bar
    Given the Playoffs Cup pick has a deadline "1 day ago"
    And the player already picked "TOR" for the Playoffs Cup pick
    When the player opens the Playoffs Cup pick sheet
    Then the playoffs pick sheet shows a read-only banner
    And the playoffs pick sheet shows the team "TOR" preselected
    And the playoffs pick sheet shows no submit button

  Scenario: An Upcoming Playoffs Cup pick cannot be opened by direct URL
    Given the Playoffs Cup pick is upcoming with a deadline "in 5 days"
    When the player opens the Playoffs Cup pick sheet
    Then the playoffs pick response status is 404
    And the playoffs pick response shows the generic not-found body

  Scenario: An Upcoming Playoffs Cup pick cannot be submitted by direct POST, nothing is saved
    Given the Playoffs Cup pick is upcoming with a deadline "in 5 days"
    When the player submits team "TOR" for the Playoffs Cup pick
    Then the playoffs pick response status is 404
    And the playoffs pick response shows the generic not-found body
    And the player has no saved Playoffs Cup pick

  Scenario: Submitting an unknown team id for the Playoffs Cup pick is rejected and nothing is saved
    Given the Playoffs Cup pick has a deadline "in 5 days"
    When the player submits team "ZZZ" for the Playoffs Cup pick
    Then the playoffs pick response status is 200
    And the playoffs pick sheet shows "Pick a team before submitting."
    And the player has no saved Playoffs Cup pick

  Scenario: Submitting a Playoffs Cup pick after the deadline is rejected with no override
    Given the Playoffs Cup pick has a deadline "1 day ago"
    When the player submits team "TOR" for the Playoffs Cup pick
    Then the playoffs pick response status is 403
    And the player has no saved Playoffs Cup pick

  Scenario: The Playoffs Cup pick is stored independently from the season-opening Cup champion pick
    Given the season-opening Cup champion pick has a deadline "in 5 days"
    And the Playoffs Cup pick has a deadline "in 5 days"
    And the player already picked "TOR" for the season-opening Cup champion pick
    When the player submits team "VGK" for the Playoffs Cup pick
    Then the player's saved Playoffs Cup pick is "VGK"
    And the player's saved season-opening Cup champion pick is still "TOR"
