Feature: Audit Log Lines
  As the person running the pool,
  I want one log line per login event and prediction save,
  so that I can see who logged in and who did what and when.

  Scenario: Requesting a code for a known email writes one audit line naming the player
    Given a server for the audit log with an open cup set
    When the seeded player requests a login code for the audit log
    Then exactly one audit line has event "login_code_requested" and player id "basti"

  Scenario: Requesting a code for an unknown email writes no audit line
    Given a server for the audit log with an open cup set
    When a visitor asks for a login code for "nobody@example.com" for the audit log
    Then the audit log request was answered with status 200
    And no audit line is written

  Scenario: A correct login code writes one success line naming the player
    Given a server for the audit log with an open cup set
    When the seeded player requests a login code for the audit log
    And the seeded player submits the issued login code for the audit log
    Then exactly one audit line has event "login_succeeded" and player id "basti"

  Scenario: A wrong login code writes one failure line without a player id
    Given a server for the audit log with an open cup set
    When a visitor submits the wrong login code for the audit log
    Then exactly one audit line has event "login_failed" and no player id

  Scenario: Logging out with a valid session writes one audit line naming the player
    Given a server for the audit log with an open cup set
    When the signed-in player logs out for the audit log
    Then exactly one audit line has event "logout" and player id "basti"

  Scenario: Logging out without a session writes no audit line
    Given a server for the audit log with an open cup set
    When a visitor without a session logs out for the audit log
    Then the audit log request was answered with status 302
    And no audit line is written

  Scenario: Saving a cup pick writes one line with player, kind and set
    Given a server for the audit log with an open cup set
    When the signed-in player saves the cup pick "TOR" for the audit log
    Then exactly one audit line has event "prediction_saved", player id "basti", kind "cup" and set "cup"

  Scenario: A save rejected after the deadline writes no save line
    Given a server for the audit log with a closed cup set
    When the signed-in player saves the cup pick "TOR" for the audit log
    Then the audit log request was answered with status 403
    And no audit line is written

  Scenario: No audit line carries an email address or a login code
    Given a server for the audit log with an open cup set
    When the seeded player requests a login code for the audit log
    And the seeded player submits the issued login code for the audit log
    And the signed-in player saves the cup pick "TOR" for the audit log
    Then no audit line contains an email address or the issued login code
