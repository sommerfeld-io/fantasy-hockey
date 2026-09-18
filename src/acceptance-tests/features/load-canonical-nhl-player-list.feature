Feature: Load Season's Canonical NHL Player List
  As the app,
  I want fantasy-hockey.yml's hand-maintained nhl_players section to load without disrupting startup,
  so that Story 2.6's award-finalist autocomplete has real skater/defenseman/goalie data to build its pick lists on.

  Scenario: A well-formed nhl_players section doesn't break app startup or the Predict screen
    Given a data file whose nhl_players section holds a sample of skaters, defensemen, and goalies
    And the seeded player "Basti" is logged in for this scenario
    When the logged-in player opens the Predict destination
    Then the Predict screen still loads successfully with status 200
