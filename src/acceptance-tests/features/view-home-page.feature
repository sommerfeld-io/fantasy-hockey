Feature: View Home Page
  As a visitor,
  I want to open the home page,
  so that I can see the application is running and the current date and time.

  Scenario: Opening the home page
    When a visitor opens the home page
    Then the response shows "Fantasy Hockey"
    And the response shows the current date and time
