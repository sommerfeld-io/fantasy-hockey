Feature: Login and Action Counters
  As the person running the pool,
  I want counters for logins and prediction saves on the metrics endpoint,
  so that I can see how often people log in and save predictions.

  Scenario: A fresh server exposes every login and save series at zero
    Given a server for the action counters with an open cup set
    When the metrics are read
    Then the login counter "login_code_requested" reads 0
    And the login counter "login_succeeded" reads 0
    And the login counter "login_failed" reads 0
    And the login counter "logout" reads 0
    And the save counter "cup" reads 0
    And the save counter "presidents" reads 0
    And the save counter "playoffcup" reads 0
    And the save counter "division_playoff_teams" reads 0
    And the save counter "division_winner" reads 0
    And the save counter "award" reads 0
    And the save counter "series" reads 0

  Scenario: A wrong login code raises only the failure counter
    Given a server for the action counters with an open cup set
    When a visitor submits the wrong login code
    And the metrics are read
    Then the login counter "login_failed" reads 1
    And the login counter "login_succeeded" reads 0

  Scenario: A correct login code raises only the success counter
    Given a server for the action counters with an open cup set
    When the seeded player requests a login code
    And the seeded player submits the issued login code
    And the metrics are read
    Then the login counter "login_succeeded" reads 1
    And the login counter "login_failed" reads 0

  Scenario: Requesting a code for a known email is counted
    Given a server for the action counters with an open cup set
    When the seeded player requests a login code
    And the metrics are read
    Then the login counter "login_code_requested" reads 1

  Scenario: Requesting a code for an unknown email is not counted
    Given a server for the action counters with an open cup set
    When a visitor asks for a login code for "nobody@example.com"
    And the metrics are read
    Then the login counter "login_code_requested" reads 0

  Scenario: Logging out with a valid session is counted
    Given a server for the action counters with an open cup set
    When the signed-in player logs out
    And the metrics are read
    Then the login counter "logout" reads 1

  Scenario: Logging out without a session is not counted
    Given a server for the action counters with an open cup set
    When a visitor without a session logs out
    And the metrics are read
    Then the login counter "logout" reads 0

  Scenario: Saving a cup pick is counted under its kind
    Given a server for the action counters with an open cup set
    When the signed-in player saves the cup pick "TOR"
    And the metrics are read
    Then the save counter "cup" reads 1
    And the save counter "presidents" reads 0

  Scenario: A save rejected after the deadline is not counted
    Given a server for the action counters with a closed cup set
    When the signed-in player saves the cup pick "TOR"
    And the metrics are read
    Then the save counter "cup" reads 0

  Scenario: No counter series carries a player, email, code or set id
    Given a server for the action counters with an open cup set
    When the seeded player requests a login code
    And the seeded player submits the issued login code
    And the signed-in player saves the cup pick "TOR"
    And the metrics are read
    Then the login and save counter series are labelled only by event or kind
