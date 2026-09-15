Feature: Log Out
  As a logged-in player,
  I want to log out,
  so that my session ends and no one else can use it from my browser.

  Scenario: Logging out clears the session and returns to the login page
    Given a player has an active session
    When the player logs out
    Then the logout response redirects to "/login"
    And the logout response clears the session cookie

  Scenario: Logging out is idempotent when the player has no session
    Given the player has no session cookie
    When the player logs out
    Then the logout response redirects to "/login"
    And the logout response clears the session cookie

  Scenario: Logging out is idempotent for an idle-expired session
    Given the player's session was issued 31 minutes ago
    When the player logs out
    Then the logout response redirects to "/login"
    And the logout response clears the session cookie

  Scenario: Logging out is idempotent for a tampered session cookie
    Given the player's session cookie has been tampered with
    When the player logs out
    Then the logout response redirects to "/login"
    And the logout response clears the session cookie

  Scenario: Logging out genuinely empties the browser's cookie jar
    Given a player has an active session
    When the player logs out
    Then the browser's cookie jar no longer holds a session cookie

  Scenario: The very next request after logging out is treated as unauthenticated
    Given a player has an active session
    When the player logs out
    And the player makes their next request to a protected route
    Then that request is redirected to "/login"

  Scenario: The very next request to Predict after logging out is treated as unauthenticated
    Given a player has an active session
    When the player logs out
    And the player makes their next request to "/predict"
    Then that request is redirected to "/login"

  Scenario: The very next request to Leaderboard after logging out is treated as unauthenticated
    Given a player has an active session
    When the player logs out
    And the player makes their next request to "/leaderboard"
    Then that request is redirected to "/login"

  Scenario: The very next request to Compare after logging out is treated as unauthenticated
    Given a player has an active session
    When the player logs out
    And the player makes their next request to "/compare"
    Then that request is redirected to "/login"
