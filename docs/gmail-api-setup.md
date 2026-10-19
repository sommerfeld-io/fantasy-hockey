# Gmail API Setup

How to let the app send login-code emails through the Gmail API instead of SMTP. This is for production only: development and tests stay on SMTP and Mailpit, and the dev compose and `.env` carry no `GMAIL_*` variable.

Two things to know before you start:

- An API key cannot send mail. The Gmail API needs OAuth 2.0, so an API key on your Cloud project is not used and is not a setting.
- A consent screen left in Testing mode expires the refresh token after 7 days for an external account, and logins then stop working. Fix it by publishing the app to production, or by using a Workspace-internal app.

A video walkthrough is available at [YouTube walkthrough of the Gmail API setup](https://youtu.be/LgcNxu79rwo). It is a helpful extra, but it may not match Google's current screens.

## Steps

1. Create a Google Cloud project, or pick an existing one, in the [Google Cloud console](https://console.cloud.google.com/).
2. Enable the Gmail API for that project under APIs and Services, Library.
3. Configure the OAuth consent screen:
    - Choose the user type: Internal for a Workspace account (no further publishing is needed and the refresh token does not expire after 7 days), otherwise External.
    - Add the `https://www.googleapis.com/auth/gmail.send` scope.
    - For External, add the account that will send the emails as a test user while the app is in Testing mode, so you can authorize it in step 5.
    - For External, publish the app to production once the flow works, so the refresh token does not expire after 7 days. After publishing, the test user is no longer needed.
4. Create an OAuth client ID under APIs and Services, Credentials:
    - The application type must be Web application.
    - Add `https://developers.google.com/oauthplayground` as an authorized redirect URI.
    - Note the client ID and the client secret.
5. Get a refresh token with the [OAuth 2.0 Playground](https://developers.google.com/oauthplayground), which needs no code in this repo:
    - Open the settings (gear icon) and tick "Use your own OAuth credentials", then enter the client ID and client secret.
    - In step 1, enter the scope `https://www.googleapis.com/auth/gmail.send` and authorize with the sending account.
    - Google may show an "unverified app" warning because `gmail.send` is a sensitive scope. For your own account and your own app, choose Advanced and continue.
    - In step 2, exchange the authorization code for tokens and copy the refresh token. In the gear settings, check that the access type is set to offline; without it Google returns no refresh token.
6. Set the variables on the production host, with `MAIL_TYPE=API`:

```bash
MAIL_TYPE=API
GMAIL_CLIENT_ID=<client-id>
GMAIL_CLIENT_SECRET=<client-secret>
GMAIL_REFRESH_TOKEN=<refresh-token>
GMAIL_SENDER=<sending-gmail-address>
```

Gmail applies daily sending limits to the sending account. A small pool sending only login codes stays far below them, but a burst of failed or repeated login attempts counts against the same account.

## Settings

| Variable              | Purpose                                                                                                                                           |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `MAIL_TYPE`           | Set to `API` (case-insensitive) to use the Gmail API instead of SMTP                                                                              |
| `GMAIL_CLIENT_ID`     | OAuth client ID from step 4                                                                                                                       |
| `GMAIL_CLIENT_SECRET` | OAuth client secret from step 4                                                                                                                   |
| `GMAIL_REFRESH_TOKEN` | Refresh token from step 5                                                                                                                         |
| `GMAIL_SENDER`        | Gmail address of the sending account, used as `From`; it must be the account that authorized the refresh token, or a verified Send-as alias of it |

Never commit these values. Keep them in the host's secret store, and see the Configuration table in the [README](../README.md#configuration) for all mail settings. When `MAIL_TYPE` is `API`, the `SMTP_*` variables are ignored with a startup warning. If any of the four `GMAIL_*` variables is missing, the app still starts and logs one startup warning naming the unset variables (never their values), but each send fails with an error naming the first missing variable.

## Troubleshooting

- Startup warning "Gmail API settings are missing" or login codes that never arrive: one or more of the four `GMAIL_*` variables is unset on the host. The warning names them, never their values. Set them and restart the app.
- `invalid_grant` in the logs: the refresh token has expired or been revoked (often the 7-day Testing-mode expiry). Publish the app, revoke the app's access at [Google Account permissions](https://myaccount.google.com/permissions), and repeat step 5.
- No refresh token in step 5: Google only returns one on first consent. Revoke the app's access at the same page and repeat step 5.
- Mail arrives with a different sender, or is rejected: `GMAIL_SENDER` is not the authorizing account or one of its verified Send-as aliases.
