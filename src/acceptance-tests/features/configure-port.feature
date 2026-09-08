Feature: Configure the Port
  As an operator,
  I want to configure the port the application listens on,
  so that I can avoid port conflicts when running the application.

  Scenario: Starting without a port override
    When the application starts with no arguments
    Then it listens on port 8080
    And it logs the port it is listening on

  Scenario: Starting with a --port override
    When the application starts with arguments "--port 19091"
    Then it listens on port 19091
    And it logs the port it is listening on

  Scenario: Starting with a -p override
    When the application starts with arguments "-p 19092"
    Then it listens on port 19092
    And it logs the port it is listening on
