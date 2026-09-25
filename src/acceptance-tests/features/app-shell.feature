Feature: Persistent App Shell With Player Identity and Navigation
  As a logged-in player,
  I want a persistent header and bottom navigation around Predict, Leaderboard, and Compare,
  so that I can move between destinations and always see who I'm logged in as.

  Background:
    Given the player "Basti" is signed in

  Scenario: The Predict destination shows the shell with Predict active
    When the player visits the Predict destination
    Then the shell shows the player's name "Basti"
    And the shell shows the season "NHL 2026–27"
    And the shell shows "Predict" as the active tab
    And the shell shows the Predict content

  Scenario: The Leaderboard destination shows the shell with Leaderboard active
    When the player visits the Leaderboard destination
    Then the shell shows "Leaderboard" as the active tab
    And the shell shows the Leaderboard content

  Scenario: The Compare destination shows the shell with Compare active
    When the player visits the Compare destination
    Then the shell shows "Compare" as the active tab
    And the shell shows the Compare placeholder content

  Scenario: The root path aliases to Predict
    When the player visits the root path
    Then the shell shows "Predict" as the active tab
    And the shell shows the Predict content

  Scenario: Navigating between destinations keeps showing the player's identity
    When the player visits the Predict destination
    And the player visits the Leaderboard destination
    And the player visits the Compare destination
    Then every visited destination showed the player's name "Basti"

  Scenario: Logging out through the shell's logout control
    When the player visits the Predict destination
    And the player uses the shell's logout control
    Then the player is redirected to "/login"
