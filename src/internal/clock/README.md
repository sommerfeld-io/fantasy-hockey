# Package: `clock`

Provides the current date and time for the application.

`NowTime` returns the current date and time in UTC as a `time.Time`, for callers that need it - for example, `internal/web` rendering it on the home page.
