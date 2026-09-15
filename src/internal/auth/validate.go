package auth

import (
	"fmt"
	"log/slog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// ValidateLoginCode hashes the submitted code and asks st to consume a
// matching, unused, unexpired LoginCode row. A wrong, expired, or
// already-used code all produce the identical ok=false, err=nil outcome -
// ValidateLoginCode never distinguishes which, so a caller can't either. Only
// a failure to persist the consumed row's used_at surfaces as an error. A
// match is logged server-side; a rejection is not, since it never
// distinguishes wrong/expired/used and would add log noise without a way to
// act on it.
func ValidateLoginCode(st *store.Store, code string) (playerID string, ok bool, err error) {
	playerID, ok, err = st.ConsumeLoginCode(hashCode(code), clock.NowTime())
	if err != nil {
		return "", false, fmt.Errorf("auth: validate login code: %w", err)
	}
	if ok {
		slog.Info("validate login code", "player_id", playerID)
	}
	return playerID, ok, nil
}
