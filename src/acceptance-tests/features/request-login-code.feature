Feature: Request Login Code
  As a Participant,
  I want to request a one-time login code by submitting my email,
  so that I can log in without a password.

  Background:
    Given a Participant "Basti" is seeded with email "basti@example.com"

  Scenario: Opening the login page for the first time
    When a visitor opens the login page
    Then the login page shows an empty email field

  Scenario: Requesting a login code with a registered email
    When a visitor requests a login code for "basti@example.com"
    Then the response shows the generic confirmation "Check the entered email address."
    And a login code is persisted for "basti@example.com"
    And a login code email is sent to "basti@example.com"

  Scenario: Requesting a login code with an unregistered email
    When a visitor requests a login code for "stranger@example.com"
    Then the response shows the generic confirmation "Check the entered email address."
    And no login code is persisted for "stranger@example.com"
    And no login code email is sent

  Scenario: Requesting a second login code before the first expires
    Given a visitor has already requested a login code for "basti@example.com"
    When a visitor requests a login code for "basti@example.com" again
    Then 2 login codes are persisted for "basti@example.com"
    And the first login code for "basti@example.com" is still unused
