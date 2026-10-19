Feature: Choose the Mail Transport
  As the person running the pool,
  I want to choose with MAIL_TYPE how login codes are emailed,
  so that I can pick SMTP or the Gmail API and get a clear error if I mistype the setting.

  Background:
    Given a fake SMTP server is running
    And the SMTP settings point at the fake SMTP server
    And a pool player "Basti" with email "basti@example.com"

  Scenario: Without MAIL_TYPE the login code goes out over SMTP
    When the app starts
    And the pool player asks for a login code
    Then the fake SMTP server received exactly 1 message addressed to "basti@example.com"
    And the message contains a 6-digit login code
    And the app did not warn about ignored mail settings

  Scenario Outline: An explicit SMTP value sends over SMTP in any spelling
    Given the mail setting "MAIL_TYPE" is "<value>"
    When the app starts
    And the pool player asks for a login code
    Then the fake SMTP server received exactly 1 message addressed to "basti@example.com"

    Examples:
      | value |
      | SMTP  |
      | smtp  |

  Scenario: API opens no SMTP connection and warns about the ignored SMTP settings
    Given the mail setting "MAIL_TYPE" is "API"
    When the app starts
    And the pool player asks for a login code
    Then the fake SMTP server received no connection
    And the app warned about ignoring "SMTP_HOST" and "SMTP_PORT"
    And the warning does not contain the SMTP host value

  Scenario: Gmail settings are ignored with a warning when SMTP is selected
    Given the mail setting "GMAIL_CLIENT_ID" is "some-client-id"
    When the app starts
    Then the app warned about ignoring "GMAIL_CLIENT_ID"
    And the warning does not contain the value "some-client-id"

  Scenario: An invalid MAIL_TYPE stops startup
    Given the mail setting "MAIL_TYPE" is "pigeon"
    When the app starts
    Then startup fails with an error naming "MAIL_TYPE", "SMTP" and "API"
    And the fake SMTP server received no connection

  Scenario: API sends through the fake Gmail server and not over SMTP
    Given a fake Gmail server is running
    And the Gmail settings point at the fake Gmail server
    And the mail setting "MAIL_TYPE" is "API"
    When the app starts
    And the pool player asks for a login code
    Then the fake Gmail server received exactly 1 send request addressed to "basti@example.com"
    And the decoded message contains a 6-digit login code that logs the pool player in
    And the fake SMTP server received no connection

  Scenario: API with a missing Gmail setting fails the send without any request
    Given a fake Gmail server is running
    And the Gmail settings point at the fake Gmail server
    And the mail setting "GMAIL_CLIENT_SECRET" is ""
    And the mail setting "MAIL_TYPE" is "API"
    When the app starts
    And the pool player asks for a login code
    Then the mail send failed with an error naming "GMAIL_CLIENT_SECRET"
    And the fake Gmail server received no request
    And the fake SMTP server received no connection

  Scenario: API with a missing Gmail setting warns at startup and still starts
    Given the mail setting "MAIL_TYPE" is "API"
    And the mail setting "GMAIL_CLIENT_SECRET" is "some-client-secret"
    When the app starts
    Then the app started without error
    And the app warned about the missing Gmail settings "GMAIL_CLIENT_ID", "GMAIL_REFRESH_TOKEN" and "GMAIL_SENDER"
    And the missing-settings warning does not contain the value "some-client-secret"

  Scenario: SMTP gives no missing Gmail settings warning
    When the app starts
    Then the app did not warn about missing Gmail settings
