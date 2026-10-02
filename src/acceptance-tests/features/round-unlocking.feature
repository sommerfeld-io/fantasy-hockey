Feature: Round Unlocking Based on Recorded Matchups
  As a logged-in player,
  I want a playoff round to stay locked until its real matchups are known,
  so that I'm never shown a pickable round before its opponents exist.

  Background:
    Given the signed-in player for round unlocking is "Basti"

  Scenario: A round-gated set with no recorded matchups stays Upcoming despite its own upcoming flag being false
    Given a round-unlocking Prediction Set "cf" titled "Conference finals" with subtitle "Set once round 2 ends" in phase "playoffs" with a deadline "in 200 days"
    When the player opens the round-unlocking Predict screen
    Then the round-unlocking set row for "cf" shows the status "Upcoming"
    And the round-unlocking set row for "cf" is not actionable

  Scenario: A round-gated set with no recorded matchups returns 404 by direct URL
    Given a round-unlocking Prediction Set "cf" titled "Conference finals" with subtitle "Set once round 2 ends" in phase "playoffs" with a deadline "in 200 days"
    When the player opens the round-unlocking Prediction Set "cf"
    Then the round-unlocking sheet response status is 404

  Scenario: Recording a matchup unlocks the round on the Predict screen
    Given a round-unlocking Prediction Set "cf" titled "Conference finals" with subtitle "Set once round 2 ends" in phase "playoffs" with a deadline "in 5 days"
    And a recorded playoff matchup "FLA" vs "TOR" for "cf"
    When the player opens the round-unlocking Predict screen
    Then the round-unlocking set row for "cf" shows the status "Open"
    And the round-unlocking set row for "cf" is actionable

  Scenario: Recording a matchup unlocks the round for direct URL access
    Given a round-unlocking Prediction Set "cf" titled "Conference finals" with subtitle "Set once round 2 ends" in phase "playoffs" with a deadline "in 5 days"
    And a recorded playoff matchup "FLA" vs "TOR" for "cf"
    When the player opens the round-unlocking Prediction Set "cf"
    Then the round-unlocking sheet response status is 200
    And the round-unlocking sheet shows the title "Conference finals"

  Scenario: A recorded matchup still respects the round's own deadline
    Given a round-unlocking Prediction Set "cf" titled "Conference finals" with subtitle "Set once round 2 ends" in phase "playoffs" with a deadline "1 day ago"
    And a recorded playoff matchup "FLA" vs "TOR" for "cf"
    When the player opens the round-unlocking Predict screen
    Then the round-unlocking set row for "cf" shows the status "Closed"

  Scenario: Round 1 stays governed by its own upcoming flag when playoff_matchups is absent
    Given a round-unlocking Prediction Set "r1" titled "Playoff round 1" with subtitle "8 series — winner & length" in phase "playoffs" with a deadline "in 5 days"
    When the player opens the round-unlocking Predict screen
    Then the round-unlocking set row for "r1" shows the status "Open"
    And the round-unlocking set row for "r1" is actionable
