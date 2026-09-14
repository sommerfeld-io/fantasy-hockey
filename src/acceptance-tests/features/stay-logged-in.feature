Feature: Stay Logged In With Sliding Session Timeout
  As a logged-in player,
  I want my session to slide forward while I'm active,
  so that I stay logged in without being forced to re-authenticate.

  Scenario: A session issued less than 30 minutes ago slides forward
    Given a player is logged in with a session issued 5 minutes ago
    When the player requests a protected route
    Then the protected-route response carries a re-issued session cookie for the same player

  Scenario: A session issued more than 30 minutes ago is redirected to login
    Given a player is logged in with a session issued 31 minutes ago
    When the player requests a protected route
    Then the protected-route response redirects to "/login"

  Scenario: No session cookie at all is redirected to login
    Given no session cookie is present
    When the player requests a protected route
    Then the protected-route response redirects to "/login"

  Scenario: A tampered session cookie is redirected to login
    Given a player is logged in with a tampered session cookie
    When the player requests a protected route
    Then the protected-route response redirects to "/login"

  Scenario: A session cookie with an empty player id is redirected to login
    Given a player is logged in with a session cookie carrying an empty player id
    When the player requests a protected route
    Then the protected-route response redirects to "/login"
