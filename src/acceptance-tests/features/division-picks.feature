Feature: Division Picks - Playoff Teams and Division Winners
  As a logged-in player,
  I want to choose which teams make the playoffs per division and pick one winner per division,
  so that my field reflects both conferences.

  Background:
    Given the signed-in player for division picks is "Basti"
    And the canonical division team list includes the full 32-team roster

  Scenario: Opening an Open divisions set with no prior picks shows an empty, submittable form
    Given the divisions Prediction Set has a deadline "in 5 days"
    When the player opens the divisions Prediction Set
    Then the divisions sheet shows four division chip groups
    And the divisions sheet shows four division winner selects
    And the divisions sheet shows no team preselected
    And the divisions sheet shows the button text "Submit predictions"
    And the divisions sheet shows the submit button disabled

  Scenario: Submitting a valid 8/8 split with all four winners saves everything and returns to Predict Submitted
    Given the divisions Prediction Set has a deadline "in 5 days"
    When the player submits a valid 8/8 division split with all four winners
    Then the division pick response redirects to "/predict"
    And the player's saved playoff teams for "Atlantic" are "BOS,BUF,DET,FLA"
    And the player's saved winner for "Atlantic" is "BOS"
    And the Predict screen marks "divisions" as "Submitted"

  Scenario: A 5/3 split within a conference is accepted
    Given the divisions Prediction Set has a deadline "in 5 days"
    When the player submits a valid 5/3 division split with all four winners
    Then the division pick response redirects to "/predict"
    And the player's saved playoff teams for "Atlantic" are "BOS,BUF,DET,FLA,MTL"
    And the player's saved playoff teams for "Metropolitan" are "CAR,CBJ,NJD"

  Scenario: A conference total that isn't exactly 8 is rejected and nothing is saved
    Given the divisions Prediction Set has a deadline "in 5 days"
    When the player submits a division split where the Eastern conference totals 7
    Then the division pick response status is 200
    And the divisions sheet shows "8 teams per conference"
    And the player has no saved playoff teams for "Atlantic"

  Scenario: A single division holding more than 5 teams is rejected even if its conference totals 8
    Given the divisions Prediction Set has a deadline "in 5 days"
    When the player submits a division split where Atlantic holds 6 teams and Metropolitan holds 2
    Then the division pick response status is 200
    And the divisions sheet shows "8 teams per conference"
    And the player has no saved playoff teams for "Atlantic"

  Scenario: A team id foreign to the division it was submitted under is rejected and nothing is saved
    Given the divisions Prediction Set has a deadline "in 5 days"
    When the player submits a division split where a Metropolitan team is checked under Atlantic
    Then the division pick response status is 200
    And the divisions sheet shows "8 teams per conference"
    And the player has no saved playoff teams for "Atlantic"

  Scenario: Leaving one division winner blank saves the rest and skips only that one
    Given the divisions Prediction Set has a deadline "in 5 days"
    When the player submits a valid 8/8 division split with the Pacific winner left blank
    Then the division pick response redirects to "/predict"
    And the player's saved winner for "Atlantic" is "BOS"
    And the player has no saved winner for "Pacific"

  Scenario: An unknown division winner id is rejected and nothing is saved
    Given the divisions Prediction Set has a deadline "in 5 days"
    When the player submits a valid 8/8 division split with a Metropolitan team as the Atlantic winner
    Then the division pick response status is 200
    And the player has no saved playoff teams for "Atlantic"
    And the player has no saved winner for "Metropolitan"

  Scenario: Submitting division picks after the deadline is rejected with no override
    Given the divisions Prediction Set has a deadline "1 day ago"
    When the player submits a valid 8/8 division split with all four winners
    Then the division pick response status is 403
    And the player has no saved playoff teams for "Atlantic"

  Scenario: Reopening a submitted, still-open divisions set pre-fills every pick and reads Update predictions
    Given the divisions Prediction Set has a deadline "in 5 days"
    And the player already submitted a valid 8/8 division split with all four winners
    When the player opens the divisions Prediction Set
    Then the divisions sheet shows the team "BOS" preselected for "Atlantic"
    And the divisions sheet shows the button text "Update predictions"
    And the divisions sheet shows the submit button enabled

  Scenario: Opening a closed divisions set shows a read-only banner with no action bar
    Given the divisions Prediction Set has a deadline "1 day ago"
    And the player already submitted a valid 8/8 division split with all four winners
    When the player opens the divisions Prediction Set
    Then the divisions sheet shows a read-only banner
    And the divisions sheet shows the team "BOS" preselected for "Atlantic"
    And the divisions sheet shows no submit button
