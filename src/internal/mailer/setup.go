package mailer

import (
	"log/slog"
	"strings"
)

// Startup warning messages logged by Setup. Each names variables, never
// their values.
const (
	WarnIgnoredSettings = "ignoring mail settings of the unselected transport"
	WarnMissingSettings = "Mail settings of the selected transport are missing, so login codes will not be sent until they are set"
)

// Setup builds the Sender for the transport s selects and logs, through
// logger, one warning naming the set variables of the unselected transport
// and one naming the missing required variables of the selected one. An
// invalid MAIL_TYPE is returned as an error and nothing is logged. logger
// must not be nil.
func Setup(s Settings, logger *slog.Logger) (Sender, error) {
	send, err := New(s)
	if err != nil {
		return nil, err
	}
	if ignored := IgnoredVars(s); len(ignored) > 0 {
		logger.Warn(WarnIgnoredSettings, "variables", strings.Join(ignored, ", "))
	}
	if missing := MissingVars(s); len(missing) > 0 {
		logger.Warn(WarnMissingSettings, "variables", strings.Join(missing, ", "))
	}
	return send, nil
}
