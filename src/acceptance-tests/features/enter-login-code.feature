Feature: Enter Login Code and Establish Session
  As a player who requested a login code,
  I want to submit it,
  so that I can establish a session and get into the app.

  Background:
    Given a player "Basti" with email "basti@example.com" has requested a login code

  Scenario: A valid, unused, unexpired code establishes a session
    When the player submits their login code
    Then the login-code response redirects to "/"
    And a session cookie is set
    And the login code is marked used

  Scenario: A wrong code shows a generic error
    When the player submits the login code "000000"
    Then the login-code response status is 200
    And the login-code response shows the generic code error
    And the submitted code "000000" is retained on the screen
    And no session cookie is set

  Scenario: An expired code shows the identical generic error
    Given the login code was issued 11 minutes ago
    When the player submits their login code
    Then the login-code response status is 200
    And the login-code response shows the generic code error
    And the submitted code "123456" is retained on the screen
    And no session cookie is set

  Scenario: An already-used code shows the identical generic error
    Given the player has already submitted their login code once
    When the player submits their login code
    Then the login-code response status is 200
    And the login-code response shows the generic code error
    And the submitted code "123456" is retained on the screen
    And no session cookie is set
