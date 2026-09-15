Feature: Browse Prediction Sets by Phase and Status
  As a logged-in player,
  I want to see every Prediction Set grouped by phase, with its status and deadline,
  so that I know what's open, closed, or still upcoming before I try to pick it.

  Background:
    Given the fixture player is "Basti"

  Scenario: The Predict screen shows both phase-grouped sections with every set's fields
    Given a Prediction Set "cup" titled "Cup champion" with subtitle "Your Stanley Cup winner" in phase "before_season" with a deadline "in 21 days"
    And an upcoming Prediction Set "scf" titled "Stanley Cup final" with subtitle "Set once the finalists are known" in phase "playoffs" with a deadline "in 200 days"
    When the player opens the Predict screen
    Then the Predict screen shows the "Before the season" section
    And the Predict screen shows the "Playoffs" section
    And the set row for "cup" shows the title "Cup champion"
    And the set row for "cup" shows the subtitle "Your Stanley Cup winner"
    And the set row for "cup" shows the status "Open"
    And the set row for "scf" shows the title "Stanley Cup final"
    And the set row for "scf" shows the subtitle "Set once the finalists are known"
    And the set row for "scf" shows the status "Upcoming"

  Scenario: A set within its window shows an Open pill, a chevron, and a days-out countdown
    Given a Prediction Set "cup" titled "Cup champion" with subtitle "Your Stanley Cup winner" in phase "before_season" with a deadline "in 5 days"
    When the player opens the Predict screen
    Then the set row for "cup" shows the status "Open"
    And the set row for "cup" shows the countdown "in 5 days"
    And the set row for "cup" is actionable

  Scenario: A set past its deadline shows a Closed pill and stays actionable
    Given a Prediction Set "cup" titled "Cup champion" with subtitle "Your Stanley Cup winner" in phase "before_season" with a deadline "1 day ago"
    When the player opens the Predict screen
    Then the set row for "cup" shows the status "Closed"
    And the set row for "cup" shows the countdown "closed"
    And the set row for "cup" is actionable

  Scenario: A deadline later today reads "today"
    Given a Prediction Set "cup" titled "Cup champion" with subtitle "Your Stanley Cup winner" in phase "before_season" with a deadline "in 3 hours"
    When the player opens the Predict screen
    Then the set row for "cup" shows the countdown "today"

  Scenario: A deadline tomorrow reads "tomorrow"
    Given a Prediction Set "cup" titled "Cup champion" with subtitle "Your Stanley Cup winner" in phase "before_season" with a deadline "in 24 hours"
    When the player opens the Predict screen
    Then the set row for "cup" shows the countdown "tomorrow"

  Scenario: An Upcoming set is dimmed, locked, and not a link
    Given an upcoming Prediction Set "cf" titled "Conference finals" with subtitle "Set once round 2 ends" in phase "playoffs" with a deadline "in 200 days"
    When the player opens the Predict screen
    Then the set row for "cf" shows the status "Upcoming"
    And the set row for "cf" is not actionable

  Scenario: Tapping an actionable row opens its stub page
    Given a Prediction Set "cup" titled "Cup champion" with subtitle "Your Stanley Cup winner" in phase "before_season" with a deadline "in 5 days"
    When the player opens the Prediction Set "cup"
    Then the sheet shows the title "Cup champion"
    And the sheet shows the countdown "in 5 days"
    And the sheet shows "Not available yet."

  Scenario: An unknown Prediction Set id gets a generic not-found response
    When the player opens the Prediction Set "does-not-exist"
    Then the sheet response status is 404

  Scenario: Browsing Predict never writes back to the data file
    Given a Prediction Set "cup" titled "Cup champion" with subtitle "Your Stanley Cup winner" in phase "before_season" with a deadline "in 5 days"
    When the player opens the Predict screen
    And the player opens the Prediction Set "cup"
    And the player opens the Prediction Set "does-not-exist"
    Then the data file is unchanged
