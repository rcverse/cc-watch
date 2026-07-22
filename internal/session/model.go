package session

import "time"

type CacheTier string
type CacheUnknownReason string

const (
	Tier1Hour   CacheTier = "1h"
	Tier5Minute CacheTier = "5m"
	TierUnknown CacheTier = "unknown"
)

const (
	CacheUnknownNoEvidence    CacheUnknownReason = "no_cache_evidence"
	CacheUnknownResponseError CacheUnknownReason = "response_error"
	CacheUnknownAmbiguousTier CacheUnknownReason = "ambiguous_tier"
	CacheUnknownAfterCompact  CacheUnknownReason = "after_compact"
	CacheUnknownAfterModel    CacheUnknownReason = "after_model"
	CacheUnknownAfterPlugins  CacheUnknownReason = "after_reload_plugins"
	CacheUnknownAfterEffort   CacheUnknownReason = "after_effort"
)

type CacheWindow struct {
	Tier       CacheTier
	Label      string
	TTLSeconds int
	Known      bool
}

type TokenStats struct {
	CacheWrites  int
	CacheReads   int
	OutputTokens int
	HitRate      float64
}

type Messages struct {
	FirstUserExcerpt string
	LastUserExcerpt  string
}

type MessageWindow struct {
	At      time.Time
	Excerpt string
}

type Gap struct {
	Seconds float64
	From    time.Time
	To      time.Time
	Reset   bool
}

type Session struct {
	SessionID            string
	ShortID              string
	Project              string
	Cwd                  string
	CurrentModel         string
	ModelsUsed           []string
	CurrentContextTokens int
	JSONLPath            string
	FileModifiedAt       time.Time
	CacheWindow          CacheWindow
	CacheUnknownReason   CacheUnknownReason
	DurationSeconds      *int
	// CacheAnchorAt is the latest confirmed cache-refreshing response.
	CacheAnchorAt  *time.Time
	Messages       Messages
	RecentMessages []MessageWindow
	TokenStats     TokenStats
	Gaps           []Gap
	ResetCount     int
	WarningCount   int
}

type StatusState string

const (
	StatusActive  StatusState = "active"
	StatusExpired StatusState = "expired"
	StatusUnknown StatusState = "unknown"
)

type Status struct {
	State            StatusState
	CacheAnchorAt    *time.Time
	RemainingSeconds *int
	ExpiredSeconds   *int
	PercentElapsed   *float64
}

func (s Session) StatusAt(now time.Time) Status {
	status := Status{
		State:         StatusUnknown,
		CacheAnchorAt: s.CacheAnchorAt,
	}
	if s.CacheAnchorAt == nil {
		return status
	}
	if !s.CacheWindow.Known || s.CacheWindow.TTLSeconds <= 0 {
		return status
	}

	elapsed := int(now.Sub(*s.CacheAnchorAt).Seconds())
	percent := float64(elapsed) / float64(s.CacheWindow.TTLSeconds) * 100
	if percent < 0 {
		percent = 0
	}
	status.PercentElapsed = &percent

	if elapsed >= s.CacheWindow.TTLSeconds {
		expired := elapsed - s.CacheWindow.TTLSeconds
		status.State = StatusExpired
		status.ExpiredSeconds = &expired
		return status
	}

	remaining := s.CacheWindow.TTLSeconds - elapsed
	status.State = StatusActive
	status.RemainingSeconds = &remaining
	return status
}
