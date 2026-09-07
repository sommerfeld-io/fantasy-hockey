# Package: `clock`

Provides the current date and time for the application.

`NowTime` returns the current date and time in UTC as a `time.Time`, for callers that need to store or compare timestamps (for example, session issuance in `internal/auth`).
