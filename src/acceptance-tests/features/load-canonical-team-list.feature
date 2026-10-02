Feature: Load Season's Canonical Team List
  As the app,
  I want fantasy-hockey.yml's hand-maintained teams section to load without disrupting startup,
  so that later stories (Cup champion/Presidents' Trophy, Division picks) have real team data to build their pick lists on.

  Scenario: A well-formed teams section doesn't break app startup or the Predict screen
    Given a data file whose teams section holds a sample of the season's canonical teams
    And the seeded player "Basti" has a valid session
    When the player requests the Predict destination
    Then the request succeeds with status 200
