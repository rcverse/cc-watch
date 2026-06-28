package jsonout

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
)

const SchemaVersion = 1

type Query struct {
	ID    string
	Limit int
}

type RefreshState struct {
	Mode                string
	WatcherStatus       string
	WatcherDegraded     bool
	WatcherMessages     []string
	SafetyRefreshActive bool
	LastRefreshAt       time.Time
}

type NotificationState struct {
	Status   string
	Degraded bool
	Recent   []string
}

type ReminderState struct {
	Available  bool
	Enabled    *bool
	Thresholds []int
	Fired      []ReminderEvent
}

type ReminderEvent struct {
	Kind             string
	SessionID        string
	ThresholdPercent int
	RemainingPercent float64
	OccurredAt       time.Time
}

type KeepAliveState struct {
	Available  bool
	Enabled    *bool
	AutoSend   *bool
	State      string
	Scope      *KeepAliveScope
	LastResult any
}

type KeepAliveScope struct {
	Mode     string `json:"mode"`
	MaxSends int    `json:"max_sends"`
}

type Error struct {
	Code    string
	Message string
	Query   string
}

type State struct {
	GeneratedAt    time.Time
	Query          Query
	ConfigWarnings []string
	Refresh        RefreshState
	Notifications  NotificationState
	Sessions       []session.Session
	Selected       *session.Session
	Reminder       map[string]ReminderState
	KeepAlive      map[string]KeepAliveState
	Error          *Error
}

func Marshal(state State) ([]byte, error) {
	if state.Error != nil && !allowedErrorCode(state.Error.Code) {
		return nil, fmt.Errorf("unsupported error code %q", state.Error.Code)
	}
	doc := buildDocument(state)
	return json.MarshalIndent(doc, "", "  ")
}

func AllowedErrorCodes() []string {
	return []string{
		"projects_dir_missing",
		"no_sessions_found",
		"session_not_found",
		"ambiguous_session_id",
		"parse_error",
		"config_error",
	}
}

func FromRefreshState(state refresh.State, lastRefreshAt time.Time) RefreshState {
	return RefreshState{
		Mode:                "snapshot",
		WatcherStatus:       string(state.Status),
		WatcherDegraded:     state.Status == refresh.StatusPartial || state.Status == refresh.StatusDegraded,
		WatcherMessages:     append([]string(nil), state.Messages...),
		SafetyRefreshActive: state.SafetyRefreshActive,
		LastRefreshAt:       lastRefreshAt,
	}
}

type document struct {
	SchemaVersion   int              `json:"schema_version"`
	GeneratedAt     string           `json:"generated_at"`
	Query           queryDocument    `json:"query"`
	Config          configDocument   `json:"config"`
	Refresh         refreshDocument  `json:"refresh"`
	Notifications   notifyDocument   `json:"notifications"`
	Sessions        []any            `json:"sessions"`
	SelectedSession *sessionDocument `json:"selected_session"`
	Error           *errorDocument   `json:"error"`
}

type queryDocument struct {
	ID    *string `json:"id"`
	Limit int     `json:"limit"`
}

type configDocument struct {
	Warnings []string `json:"warnings"`
}

type refreshDocument struct {
	Mode                string          `json:"mode"`
	Watcher             watcherDocument `json:"watcher"`
	SafetyRefreshActive bool            `json:"safety_refresh_active"`
	LastRefreshAt       string          `json:"last_refresh_at"`
}

type watcherDocument struct {
	Status   string   `json:"status"`
	Degraded bool     `json:"degraded"`
	Messages []string `json:"messages"`
}

type notifyDocument struct {
	Status   string   `json:"status"`
	Degraded bool     `json:"degraded"`
	Recent   []string `json:"recent"`
}

type sessionDocument struct {
	SessionID       string              `json:"session_id"`
	ShortID         string              `json:"short_id"`
	Project         string              `json:"project"`
	JSONLPath       string              `json:"jsonl_path"`
	FileModifiedAt  string              `json:"file_modified_at"`
	StartedAt       *string             `json:"started_at"`
	EndedAt         *string             `json:"ended_at"`
	DurationSeconds *int                `json:"duration_seconds"`
	CacheWindow     cacheWindowDocument `json:"cache_window"`
	Status          statusDocument      `json:"status"`
	Messages        messagesDocument    `json:"messages"`
	TokenStats      tokenStatsDocument  `json:"token_stats"`
	Gaps            gapsDocument        `json:"gaps"`
	Warnings        []warningDocument   `json:"warnings"`
	Reminder        reminderDocument    `json:"reminder"`
	KeepAlive       keepAliveDocument   `json:"keep_alive"`
}

type candidateDocument struct {
	SessionID string `json:"session_id"`
	ShortID   string `json:"short_id"`
	Project   string `json:"project"`
}

type cacheWindowDocument struct {
	Tier       string   `json:"tier"`
	Label      string   `json:"label"`
	TTLSeconds int      `json:"ttl_seconds"`
	Known      bool     `json:"known"`
	Evidence   []string `json:"evidence"`
}

type statusDocument struct {
	State            string   `json:"state"`
	LastMessageAt    *string  `json:"last_message_at"`
	RemainingSeconds *int     `json:"remaining_seconds"`
	ExpiredSeconds   *int     `json:"expired_seconds"`
	PercentElapsed   *float64 `json:"percent_elapsed"`
}

type messagesDocument struct {
	FirstUserExcerpt string `json:"first_user_excerpt"`
	LastUserExcerpt  string `json:"last_user_excerpt"`
}

type tokenStatsDocument struct {
	CacheWrites  int     `json:"cache_writes"`
	CacheReads   int     `json:"cache_reads"`
	OutputTokens int     `json:"output_tokens"`
	HitRate      float64 `json:"hit_rate"`
}

type gapsDocument struct {
	Count      int           `json:"count"`
	ResetCount int           `json:"reset_count"`
	Latest     []gapDocument `json:"latest"`
}

type gapDocument struct {
	Seconds float64 `json:"seconds"`
	From    string  `json:"from"`
	To      string  `json:"to"`
	Reset   bool    `json:"reset"`
}

type warningDocument struct {
	Code    string `json:"code"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type reminderDocument struct {
	Available  bool                    `json:"available"`
	Enabled    *bool                   `json:"enabled"`
	Thresholds []int                   `json:"thresholds"`
	Fired      []reminderEventDocument `json:"fired"`
}

type reminderEventDocument struct {
	Kind             string  `json:"kind"`
	ThresholdPercent int     `json:"threshold_percent"`
	RemainingPercent float64 `json:"remaining_percent"`
	OccurredAt       string  `json:"occurred_at"`
}

type keepAliveDocument struct {
	Available  bool            `json:"available"`
	Enabled    *bool           `json:"enabled"`
	AutoSend   *bool           `json:"auto_send"`
	State      string          `json:"state"`
	Scope      *KeepAliveScope `json:"scope"`
	LastResult any             `json:"last_result"`
}

type errorDocument struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Query   string `json:"query"`
}

func buildDocument(state State) document {
	generatedAt := state.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	sessions := make([]any, 0, len(state.Sessions))
	for _, s := range state.Sessions {
		if state.Error != nil {
			sessions = append(sessions, buildCandidate(s))
			continue
		}
		sessions = append(sessions, buildSession(s, generatedAt, state.Reminder, state.KeepAlive))
	}

	var selected *sessionDocument
	if state.Selected != nil {
		doc := buildSession(*state.Selected, generatedAt, state.Reminder, state.KeepAlive)
		selected = &doc
	}

	return document{
		SchemaVersion:   SchemaVersion,
		GeneratedAt:     formatTime(generatedAt),
		Query:           buildQuery(state.Query),
		Config:          configDocument{Warnings: stringSlice(state.ConfigWarnings)},
		Refresh:         buildRefresh(state.Refresh, generatedAt),
		Notifications:   buildNotifications(state.Notifications),
		Sessions:        sessions,
		SelectedSession: selected,
		Error:           buildError(state.Error),
	}
}

func buildQuery(query Query) queryDocument {
	doc := queryDocument{Limit: query.Limit}
	if doc.Limit == 0 {
		doc.Limit = 5
	}
	if query.ID != "" {
		doc.ID = &query.ID
	}
	return doc
}

func buildRefresh(state RefreshState, generatedAt time.Time) refreshDocument {
	if state.Mode == "" {
		state.Mode = "snapshot"
	}
	if state.WatcherStatus == "" {
		state.WatcherStatus = "not_started"
	}
	if state.LastRefreshAt.IsZero() {
		state.LastRefreshAt = generatedAt
	}
	return refreshDocument{
		Mode: state.Mode,
		Watcher: watcherDocument{
			Status:   state.WatcherStatus,
			Degraded: state.WatcherDegraded,
			Messages: stringSlice(state.WatcherMessages),
		},
		SafetyRefreshActive: state.SafetyRefreshActive,
		LastRefreshAt:       formatTime(state.LastRefreshAt),
	}
}

func buildNotifications(state NotificationState) notifyDocument {
	if state.Status == "" {
		state.Status = "not_started"
	}
	return notifyDocument{
		Status:   state.Status,
		Degraded: state.Degraded,
		Recent:   stringSlice(state.Recent),
	}
}

func buildSession(s session.Session, now time.Time, reminderStates map[string]ReminderState, keepAliveStates map[string]KeepAliveState) sessionDocument {
	return sessionDocument{
		SessionID:       s.SessionID,
		ShortID:         s.ShortID,
		Project:         s.Project,
		JSONLPath:       s.JSONLPath,
		FileModifiedAt:  formatTime(s.FileModifiedAt),
		StartedAt:       formatTimePtr(s.StartedAt),
		EndedAt:         formatTimePtr(s.EndedAt),
		DurationSeconds: s.DurationSeconds,
		CacheWindow: cacheWindowDocument{
			Tier:       string(s.CacheWindow.Tier),
			Label:      s.CacheWindow.Label,
			TTLSeconds: s.CacheWindow.TTLSeconds,
			Known:      s.CacheWindow.Known,
			Evidence:   stringSlice(s.CacheWindow.Evidence),
		},
		Status:     buildStatus(s.StatusAt(now)),
		Messages:   messagesDocument(s.Messages),
		TokenStats: tokenStatsDocument(s.TokenStats),
		Gaps:       buildGaps(s),
		Warnings:   buildWarnings(s.Warnings),
		Reminder:   buildReminder(reminderStates[s.SessionID]),
		KeepAlive:  buildKeepAlive(keepAliveStates[s.SessionID]),
	}
}

func buildCandidate(s session.Session) candidateDocument {
	return candidateDocument{
		SessionID: s.SessionID,
		ShortID:   s.ShortID,
		Project:   s.Project,
	}
}

func buildStatus(status session.Status) statusDocument {
	return statusDocument{
		State:            string(status.State),
		LastMessageAt:    formatTimePtr(status.LastMessageAt),
		RemainingSeconds: status.RemainingSeconds,
		ExpiredSeconds:   status.ExpiredSeconds,
		PercentElapsed:   status.PercentElapsed,
	}
}

func buildGaps(s session.Session) gapsDocument {
	gaps := make([]gapDocument, 0, len(s.Gaps))
	for _, gap := range s.Gaps {
		gaps = append(gaps, gapDocument{
			Seconds: gap.Seconds,
			From:    formatTime(gap.From),
			To:      formatTime(gap.To),
			Reset:   gap.Reset,
		})
	}
	return gapsDocument{
		Count:      len(s.Gaps),
		ResetCount: s.ResetCount,
		Latest:     gaps,
	}
}

func buildWarnings(warnings []session.ParseWarning) []warningDocument {
	docs := make([]warningDocument, 0, len(warnings))
	for _, warning := range warnings {
		docs = append(docs, warningDocument{
			Code:    string(warning.Code),
			Line:    warning.Line,
			Message: warning.Message,
		})
	}
	return docs
}

func buildReminder(state ReminderState) reminderDocument {
	thresholds := state.Thresholds
	if thresholds == nil {
		thresholds = []int{20, 10}
	}
	fired := make([]reminderEventDocument, 0, len(state.Fired))
	for _, event := range state.Fired {
		fired = append(fired, reminderEventDocument{
			Kind:             event.Kind,
			ThresholdPercent: event.ThresholdPercent,
			RemainingPercent: event.RemainingPercent,
			OccurredAt:       formatTime(event.OccurredAt),
		})
	}
	return reminderDocument{
		Available:  state.Available,
		Enabled:    state.Enabled,
		Thresholds: append([]int(nil), thresholds...),
		Fired:      fired,
	}
}

func buildKeepAlive(state KeepAliveState) keepAliveDocument {
	keepAliveState := state.State
	if keepAliveState == "" {
		keepAliveState = "unavailable"
	}
	return keepAliveDocument{
		Available:  state.Available,
		Enabled:    state.Enabled,
		AutoSend:   state.AutoSend,
		State:      keepAliveState,
		Scope:      state.Scope,
		LastResult: state.LastResult,
	}
}

func buildError(err *Error) *errorDocument {
	if err == nil {
		return nil
	}
	return &errorDocument{
		Code:    err.Code,
		Message: err.Message,
		Query:   err.Query,
	}
}

func allowedErrorCode(code string) bool {
	for _, allowed := range AllowedErrorCodes() {
		if code == allowed {
			return true
		}
	}
	return false
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := formatTime(*t)
	return &formatted
}

func stringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
