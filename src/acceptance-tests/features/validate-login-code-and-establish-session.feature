Feature: Validate Login Code and Establish Session
  As a Participant,
  I want to submit my emailed login code and stay signed in for a while,
  so that I can use the app without a password and without logging in on every request.

  Background:
    Given a Participant "Basti" is seeded with email "basti@example.com"

  Scenario: Submitting an email replaces that field with the code field
    When a visitor requests a login code for "basti@example.com"
    Then the login page shows a code field instead of an email field

  Scenario: A valid unused code authenticates the Participant
    Given a visitor requests a login code for "basti@example.com"
    When the visitor submits the emailed code for "basti@example.com"
    Then the visitor is authenticated

  Scenario: A wrong code shows the generic invalid-code message
    Given a visitor requests a login code for "basti@example.com"
    When the visitor submits the code "000000" for "basti@example.com"
    Then the response shows "Invalid code."
    And the visitor is not authenticated

  Scenario: An expired code shows the same generic invalid-code message
    Given a visitor requests a login code for "basti@example.com"
    And the login code for "basti@example.com" was issued 11 minutes ago
    When the visitor submits the emailed code for "basti@example.com"
    Then the response shows "Invalid code."
    And the visitor is not authenticated

  Scenario: An already-used code shows the same generic invalid-code message
    Given a visitor requests a login code for "basti@example.com"
    And the visitor submits the emailed code for "basti@example.com"
    And the visitor has no session cookie
    When the visitor submits the emailed code for "basti@example.com" again
    Then the response shows "Invalid code."
    And the visitor is not authenticated

  Scenario: A request within the sliding timeout re-issues the session
    Given a visitor holds a session for "basti@example.com" issued 25 minutes ago
    When the visitor visits the home page
    Then the visitor is authenticated
    And the visitor's session for "basti@example.com" was re-issued just now

  Scenario: A session times out after 30 minutes of no requests
    Given a visitor holds a session for "basti@example.com" issued 31 minutes ago
    When the visitor visits the home page
    Then the response shows "Session expired."
    And the visitor is not authenticated

  Scenario: A visitor who never had a session sees no session-expired message
    When the visitor visits the home page
    Then the response does not show "Session expired."
    And the visitor is not authenticated
