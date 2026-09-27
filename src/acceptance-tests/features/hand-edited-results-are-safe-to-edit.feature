Feature: Hand-Edited Results Are Safe to Edit
  As the pool operator maintaining results and award finalists by hand,
  I want the app to leave my hand-edited results and award_finalists sections
  exactly as I wrote them whenever anyone else's pick is saved,
  so that my comments, flow style and key order in the data file are never lost.

  Scenario: Saving a pick through the web UI leaves a hand-edited results section untouched
    Given a data file with a hand-edited results and award_finalists section containing a comment, flow style, and an unusual key order
    When the player submits a valid pick through the web UI
    Then the persisted results and award_finalists sections are byte-for-byte unchanged
