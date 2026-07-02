package ratelimit

// epsilonPct is the safety floor below which observed movement is treated
// as "no meaningful signal" (e.g. an idle account) rather than a real,
// tiny per-message cost. Never return a large messagesLeft from noise.
const epsilonPct = 0.5

// Momentum estimates the account's rate-limit usage-percentage cost per
// message, from the last few consecutive reading-to-reading deltas. ok is
// false when there's not enough history or the observed movement is below
// the safety floor -- callers must treat that as "unknown", never as "zero
// cost" (which would look confidently, and wrongly, safe).
func Momentum(history []HistoryPoint) (pctPerMessage float64, ok bool) {
	if len(history) < 2 {
		return 0, false
	}
	deltas := len(history) - 1
	if deltas > 3 {
		deltas = 3
	}
	start := len(history) - 1 - deltas

	var sum float64
	for i := start; i < len(history)-1; i++ {
		sum += history[i+1].UsedPct - history[i].UsedPct
	}
	avg := sum / float64(deltas)
	if avg < epsilonPct {
		return 0, false
	}
	return avg, true
}
