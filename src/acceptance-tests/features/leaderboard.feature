Feature: Leaderboard
  As a player in the pool,
  I want every player ranked by total points on the Leaderboard,
  so that I can see at a glance who is winning.

  Background:
    Given the pool players are "Basti", "Sadl" and "Tobbi"
    And the recorded Stanley Cup winner for the leaderboard is "FLA"
    And the recorded Presidents' Trophy winner for the leaderboard is "TOR"

  Scenario: A clear leader is ranked first and is the only one in gold
    Given "Basti" picked "FLA" for the "cup" pick
    And "Basti" picked "TOR" for the "presidents" pick
    And "Basti" picked "FLA" for the "playoffcup" pick
    And "Sadl" picked "FLA" for the "cup" pick
    And "Sadl" picked "TOR" for the "presidents" pick
    And "Tobbi" picked "FLA" for the "cup" pick
    When "Basti" opens the Leaderboard
    Then the Leaderboard shows these rows in order:
      | rank | player | regular | playoff | total | gold |
      | 1    | Basti  | 40      | 20      | 60    | yes  |
      | 2    | Sadl   | 40      | 0       | 40    | no   |
      | 3    | Tobbi  | 20      | 0       | 20    | no   |

  Scenario: Players tied below the top share a rank and are listed alphabetically
    Given "Basti" picked "FLA" for the "cup" pick
    And "Basti" picked "TOR" for the "presidents" pick
    And "Basti" picked "FLA" for the "playoffcup" pick
    And "Tobbi" picked "FLA" for the "cup" pick
    And "Tobbi" picked "FLA" for the "playoffcup" pick
    And "Sadl" picked "FLA" for the "cup" pick
    And "Sadl" picked "TOR" for the "presidents" pick
    When "Basti" opens the Leaderboard
    Then the Leaderboard shows these rows in order:
      | rank | player | regular | playoff | total | gold |
      | 1    | Basti  | 40      | 20      | 60    | yes  |
      | 2    | Sadl   | 40      | 0       | 40    | no   |
      | 2    | Tobbi  | 20      | 20      | 40    | no   |

  Scenario: Players tied at the top are all leaders and the next rank is skipped
    Given "Basti" picked "FLA" for the "cup" pick
    And "Basti" picked "TOR" for the "presidents" pick
    And "Sadl" picked "FLA" for the "cup" pick
    And "Sadl" picked "FLA" for the "playoffcup" pick
    And "Tobbi" picked "FLA" for the "cup" pick
    When "Basti" opens the Leaderboard
    Then the Leaderboard shows these rows in order:
      | rank | player | regular | playoff | total | gold |
      | 1    | Basti  | 40      | 0       | 40    | yes  |
      | 1    | Sadl   | 20      | 20      | 40    | yes  |
      | 3    | Tobbi  | 20      | 0       | 20    | no   |

  Scenario: Before anything scores everyone shares rank 1 and nobody is gold
    Given "Basti" picked "TOR" for the "cup" pick
    When "Basti" opens the Leaderboard
    Then the Leaderboard shows these rows in order:
      | rank | player | regular | playoff | total | gold |
      | 1    | Basti  | 0       | 0       | 0     | no   |
      | 1    | Sadl   | 0       | 0       | 0     | no   |
      | 1    | Tobbi  | 0       | 0       | 0     | no   |

  Scenario: A player with no picks is still listed with zero points
    Given "Basti" picked "FLA" for the "cup" pick
    And "Sadl" picked "FLA" for the "playoffcup" pick
    When "Basti" opens the Leaderboard
    Then the Leaderboard shows these rows in order:
      | rank | player | regular | playoff | total | gold |
      | 1    | Basti  | 20      | 0       | 20    | yes  |
      | 1    | Sadl   | 0       | 20      | 20    | yes  |
      | 3    | Tobbi  | 0       | 0       | 0     | no   |

  Scenario: A hand-recorded result shows up after the app restarts
    Given "Basti" picked "FLA" for the "cup" pick
    And "Sadl" picked "TOR" for the "cup" pick
    When "Basti" opens the Leaderboard
    And the recorded Stanley Cup winner is changed by hand to "TOR" and the app restarts
    And "Basti" opens the Leaderboard
    Then the Leaderboard shows these rows in order:
      | rank | player | regular | playoff | total | gold |
      | 1    | Sadl   | 20      | 0       | 20    | yes  |
      | 2    | Basti  | 0       | 0       | 0     | no   |
      | 2    | Tobbi  | 0       | 0       | 0     | no   |

  Scenario: A pick saved while the app runs shows up on the next request
    Given "Basti" picked "FLA" for the "cup" pick
    When "Basti" opens the Leaderboard
    And "Tobbi" saves "FLA" for the "playoffcup" pick while the app runs
    And "Basti" opens the Leaderboard
    Then the Leaderboard shows these rows in order:
      | rank | player | regular | playoff | total | gold |
      | 1    | Basti  | 20      | 0       | 20    | yes  |
      | 1    | Tobbi  | 0       | 20      | 20    | yes  |
      | 3    | Sadl   | 0       | 0       | 0     | no   |

  Scenario: The signed-in player's own row looks like every other row
    Given "Sadl" picked "FLA" for the "cup" pick
    And "Tobbi" picked "FLA" for the "cup" pick
    When "Sadl" opens the Leaderboard
    Then no Leaderboard row is marked "(you)"
    And the Leaderboard row for "Sadl" is marked up exactly like the row for "Tobbi"

  Scenario: The Leaderboard explains how it is ranked
    When "Basti" opens the Leaderboard
    Then the Leaderboard shows the caption "Ranked by total points."
    And the Leaderboard shows the footer "Regular-season picks feed the Regular column, playoff-round picks feed Playoff, and Total is what decides the standings."
    And the Leaderboard no longer shows the coming-soon placeholder
