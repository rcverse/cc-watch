package ratelimit

import "math"

// Projection is the estimated safety margin for the current session's
// KeepAlive pings before the account-wide 5-hour window resets.
type Projection struct {
	MessagesLeft int
	AtRisk       bool
}

// Project estimates whether the account will run out of budget before the
// current session needs enough KeepAlive pings to survive to reset.
// messagesLeft is a conservative lower bound on true KeepAlive-only
// capacity, since pctPerMessage is calibrated from whatever mix of
// messages actually happened -- ordinary messages cost more than a short
// KeepAlive ping on average. ok is false whenever momentum or the TTL/reset
// inputs aren't known -- callers must never fabricate a number in that
// case.
func Project(usedPct, pctPerMessage float64, momentumOK bool, timeToResetSeconds float64, ttlSeconds int) (Projection, bool) {
	if !momentumOK || ttlSeconds <= 0 || timeToResetSeconds < 0 {
		return Projection{}, false
	}
	messagesLeft := int(math.Floor((100 - usedPct) / pctPerMessage))
	pingsNeeded := int(math.Ceil(timeToResetSeconds / float64(ttlSeconds)))
	return Projection{
		MessagesLeft: messagesLeft,
		AtRisk:       messagesLeft <= pingsNeeded,
	}, true
}
