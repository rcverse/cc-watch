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
	// with the countdown's own end and any drift would silently bail auto-send
	// to a manual prompt. That gap made auto-send effectively never fire for
	// 5-minute and unknown-tier caches. The smaller floor absorbs normal drift
	// while still refusing to auto-send within seconds of expiry.
	sendDeadlineMarginSeconds = 10
)

type TimingDecision struct {
	EffectiveTTLSeconds       int
	EffectiveTriggerSeconds   int
	EffectiveCountdownSeconds int
	RemainingSeconds          int
	InsideTrigger             bool
	AutoSendAllowed           bool
	SafetyClamped             bool
	UsesConservativeTTL       bool
}

func EvaluateTiming(s session.Session, now time.Time, cfg config.KeepAliveConfig) TimingDecision {
	ttl, conservative := effectiveTTL(s)
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

	countdown, disabled, clamped := effectiveCountdown(cfg.CountdownSeconds, sendWindow)
	return TimingDecision{
		EffectiveTTLSeconds:       ttl,
		EffectiveTriggerSeconds:   sendWindow,
		EffectiveCountdownSeconds: countdown,
		RemainingSeconds:          remaining,
		InsideTrigger:             insideTrigger,
		AutoSendAllowed:           !disabled,
		SafetyClamped:             clamped,
		UsesConservativeTTL:       conservative,
	}
}

func effectiveTTL(s session.Session) (int, bool) {
	if s.CacheWindow.Known && s.CacheWindow.TTLSeconds > 0 {
		return s.CacheWindow.TTLSeconds, false
	}
	return conservativeTTLSeconds, true
}

func effectiveCountdown(configuredCountdown, sendWindow int) (int, bool, bool) {
	if sendWindow <= safetyMarginSeconds {
		return 0, true, false
	}
	latestSafeCountdown := sendWindow - safetyMarginSeconds
	if configuredCountdown > latestSafeCountdown {
		return latestSafeCountdown, false, true
	}
	return configuredCountdown, false, false
}
