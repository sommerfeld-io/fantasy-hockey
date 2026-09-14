Feature: Request a Login Code
  As a player,
  I want to request a login code by email,
  so that I can log in without a password, and without anything revealing whether my email is part of the pool.

  Background:
    Given a player "Basti" with email "basti@example.com" is registered

  Scenario: A matching email gets a persisted, emailed code
    When a visitor requests a login code for "basti@example.com"
    Then the response status is 200
    And a login code is persisted whose hash matches the emailed code
    And the email is sent to "basti@example.com"

  Scenario: A non-matching email produces an identical response and no side effects
    When a visitor requests a login code for "basti@example.com"
    And a visitor requests a login code for "unknown@example.com"
    Then the two responses are identical
    And only 1 email was sent in total

  Scenario: Requesting a second code leaves the first one untouched
    When a visitor requests a login code for "basti@example.com"
    And a visitor requests a login code for "basti@example.com"
    Then 2 login codes are persisted
    And the first login code is unchanged

  Scenario: A mailer failure does not affect the response
    Given sending email is configured to fail
    When a visitor requests a login code for "basti@example.com"
    Then the response status is 200
    And an error was logged
