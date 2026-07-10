package keepalive

import (
	"time"

	"github.com/richardchen/cc-watch/internal/config"
	"github.com/richardchen/cc-watch/internal/session"
)

const (
	conservativeTTLSeconds = 300
	// safetyMarginSeconds sizes the countdown: it aims to finish this far
	// before expiry so the send has time to land.
	safetyMarginSeconds = 30
	// sendDeadlineMarginSeconds is the hard stop when the countdown actually
	// elapses -- deliberately smaller than safetyMarginSeconds. The countdown
	// is tick-counted, so it always takes a beat longer than its nominal
	// duration; if the deadline used the full 30s margin it would coincide
	// with the countdown's own end and any drift would silently pause sends for
	// 5-minute and unknown-tier caches. The smaller floor absorbs normal drift
	// while still refusing to send within seconds of expiry.
	sendDeadlineMarginSeconds = 10
)

type TimingDecision struct {
	EffectiveCountdownSeconds int
	InsideTrigger             bool
	SendAllowed               bool
}

func EvaluateTiming(s session.Session, now time.Time, cfg config.KeepAliveConfig) TimingDecision {
	ttl := effectiveTTL(s)
	remaining := ttl
	if s.LastMessageAt != nil {
		remaining = ttl - int(now.Sub(*s.LastMessageAt).Seconds())
	}
	if remaining < 0 {
		remaining = 0
	}

	trigger := min(cfg.TriggerBeforeExpiryMinutes*60, ttl/5)
	if trigger <= 0 {
		trigger = remaining
	}
	insideTrigger := remaining <= trigger
	sendWindow := trigger
	if insideTrigger {
		sendWindow = remaining
	}

	countdown, disabled := effectiveCountdown(cfg.CountdownSeconds, sendWindow)
	return TimingDecision{
		EffectiveCountdownSeconds: countdown,
		InsideTrigger:             insideTrigger,
		SendAllowed:               !disabled,
	}
}

func effectiveTTL(s session.Session) int {
	if s.CacheWindow.Known && s.CacheWindow.TTLSeconds > 0 {
		return s.CacheWindow.TTLSeconds
	}
	return conservativeTTLSeconds
}

func effectiveCountdown(configuredCountdown, sendWindow int) (int, bool) {
	if sendWindow <= safetyMarginSeconds {
		return 0, true
	}
	latestSafeCountdown := sendWindow - safetyMarginSeconds
	if configuredCountdown > latestSafeCountdown {
		return latestSafeCountdown, false
	}
	return configuredCountdown, false
}
