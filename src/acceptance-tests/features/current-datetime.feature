Feature: Print Current Date and Time
  As an operator running the fantasy-hockey application,
  I want the application to report the current date and time,
  so that I can confirm the application produces valid, observable output.

  Scenario: Requesting the current date and time
    When the application provides the current date and time
    Then the value should be a valid RFC 3339 timestamp
    And the value should represent a moment close to the actual current time
