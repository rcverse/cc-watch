package tui

import (
	"context"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/config"
	"github.com/richardchen/cc-watch/internal/keepalive"
	"github.com/richardchen/cc-watch/internal/notify"
	"github.com/richardchen/cc-watch/internal/refresh"
	"github.com/richardchen/cc-watch/internal/session"
)

type Route string

const (
	RouteList      Route = "list"
	RouteWorkspace Route = "workspace"
	RouteAmbiguous Route = "ambiguous"
	RouteConfig    Route = "config"
)

type StartMode string

const (
	StartList   StartMode = "list"
	StartConfig StartMode = "config"
)

type EmptyState string

const (
	EmptyNone        EmptyState = ""
	EmptyLoading     EmptyState = "loading"
	EmptyProjectsDir EmptyState = "projects_dir_missing"
	EmptyNoSessions  EmptyState = "no_sessions"
)

type RefreshViewState struct {
	Watcher                  refresh.State
	ProjectsDir              string
	EmptyState               EmptyState
	NotificationDegraded     string
	ClaudeUnavailableMessage string
}

type Dependencies struct {
	RefreshSnapshot              func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot
	CheckClaudeAvailable         func() error
	KeepAliveRunner              keepalive.ClaudeRunner
	ConfirmKeepAlive             func(context.Context, keepalive.ConfirmationTarget) (keepalive.ConfirmationResult, error)
	SaveConfig                   func(config.Config) error
	NotifyEvent                  func(event notify.Event) notify.Result
	ResetNotificationSuppression func()
}

type RefreshSnapshot struct {
	Sessions     []session.Session
	Refresh      RefreshViewState
	HasRefresh   bool
	SelectedOnly bool
	SelectedID   string
}

type RefreshTiming struct {
	Debounce       time.Duration
	SafetyInterval time.Duration
}

type NotificationStatus struct {
	Event        notify.Event
	Notification notify.Notification
	Result       notify.Result
}

type Notice struct {
	Message   string
	Role      StyleRole
	ExpiresAt time.Time
}

type Options struct {
	Now                time.Time
	Width              int
	Height             int
	Dependencies       Dependencies
	Sessions           []session.Session
	Countdowns         map[string]int
	ReminderEnabled    map[string]bool
	ReminderThresholds []int
	KeepAliveEnabled   map[string]bool
	KeepAliveConfig    config.KeepAliveConfig
	KeepAliveManager   *keepalive.Manager
	KeepAliveStates    map[string]keepalive.SessionState
	RefreshGeneration  int
	RefreshTiming      RefreshTiming
	SelectedID         string
	AmbiguousID        string
	StartMode          StartMode
	Refresh            RefreshViewState
	StartDisplayTicker bool
	StartRefreshTicker bool
	LiveRefresh        tea.Cmd
	CloseLiveRefresh   func() error
	Config             config.Config
}

type Model struct {
	now                  time.Time
	width                int
	height               int
	deps                 Dependencies
	route                Route
	sessions             []session.Session
	countdowns           map[string]int
	reminderEnabled      map[string]bool
	reminderThresholds   []int
	reminderFired        map[string]map[int]bool
	keepAliveEnabled     map[string]bool
	keepAliveConfig      config.KeepAliveConfig
	keepAliveManager     *keepalive.Manager
	refreshGeneration    int
	refreshCoordinator   *refresh.Coordinator
	refreshTiming        RefreshTiming
	refreshDebounceToken int
	liveRefresh          tea.Cmd
	watcherEvents        []WatcherEventMsg
	notificationStatuses []NotificationStatus
	focusIndex           int
	selectedIndex        int
	selectedID           string
	ambiguousID          string
	lastAction           string
	notice               Notice
	refresh              RefreshViewState
	lastRefreshSource    refresh.Source
	lastBypassedDebounce bool
	detailsOffset        int
	sessionInfoExpanded  bool
	gapSortNewest        bool
	startDisplayTicker   bool
	startRefreshTicker   bool
	configReturnRoute    Route
	configOriginal       config.Config
	configDraft          config.Config
	configEditing        bool
	configEditingField   string
	configInput          string
	configInputFresh     bool
	configFieldErrors    map[string]string
	configResetConfirm   bool
	configSaveError      string
}

type focusItem struct {
	action string
}

var emptyFocusActions = []string{"refresh", "quit"}

func NewModel(options Options) Model {
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	countdowns := map[string]int{}
	for sessionID, seconds := range options.Countdowns {
		countdowns[sessionID] = seconds
	}
	reminders := cloneBoolMap(options.ReminderEnabled)
	thresholds := append([]int(nil), options.ReminderThresholds...)
	if len(thresholds) == 0 {
		thresholds = []int{20, 10}
	}
	keepAlives := cloneBoolMap(options.KeepAliveEnabled)
	keepAliveConfig := normalizeKeepAliveConfig(options.KeepAliveConfig)
	configDraft := normalizeConfig(options.Config)
	keepAliveManager := options.KeepAliveManager
	if keepAliveManager == nil {
		keepAliveManager = keepalive.NewManager(keepAliveConfig)
	}
	width := options.Width
	if width <= 0 {
		width = 80
	}
	height := options.Height
	if height <= 0 {
		height = 24
	}
	sessions := cloneSessions(options.Sessions)
	selectedIndex := selectedIndexFor(sessions, options.SelectedID)
	for _, state := range options.KeepAliveStates {
		keepAliveManager.SetState(state)
	}
	for _, s := range sessions {
		if keepAlives[s.SessionID] && s.StatusAt(now).State != session.StatusExpired && keepAliveManager.State(s.SessionID).State == keepalive.StateOff {
			keepAliveManager.Enable(s, now)
		}
	}
	refreshTiming := options.RefreshTiming
	if refreshTiming.Debounce <= 0 {
		refreshTiming.Debounce = 300 * time.Millisecond
	}
	if refreshTiming.SafetyInterval <= 0 {
		refreshTiming.SafetyInterval = 30 * time.Second
	}
	refreshCoordinator := refresh.NewCoordinator(refresh.Options{
		Debounce:          refreshTiming.Debounce,
		SafetyInterval:    refreshTiming.SafetyInterval,
		InitialNow:        now,
		InitialGeneration: options.RefreshGeneration,
	})
	model := Model{
		width:              width,
		height:             height,
		now:                now,
		deps:               options.Dependencies,
		route:              routeFromOptions(options),
		sessions:           sessions,
		countdowns:         countdowns,
		reminderEnabled:    reminders,
		reminderThresholds: thresholds,
		reminderFired:      map[string]map[int]bool{},
		keepAliveEnabled:   keepAlives,
		keepAliveConfig:    keepAliveConfig,
		keepAliveManager:   keepAliveManager,
		refreshGeneration:  options.RefreshGeneration,
		refreshCoordinator: refreshCoordinator,
		refreshTiming:      refreshTiming,
		liveRefresh:        options.LiveRefresh,
		selectedIndex:      selectedIndex,
		selectedID:         options.SelectedID,
		ambiguousID:        options.AmbiguousID,
		refresh:            defaultRefresh(options.Refresh),
		startDisplayTicker: options.StartDisplayTicker,
		startRefreshTicker: options.StartRefreshTicker,
		configOriginal:     configDraft,
		configDraft:        configDraft,
		configFieldErrors:  map[string]string{},
	}
	model.focusIndex = model.defaultFocusIndex()
	return model
}

func (m Model) Init() tea.Cmd {
	var commands []tea.Cmd
	if m.startDisplayTicker {
		commands = append(commands, displayTickCommand())
	}
	if m.startRefreshTicker {
		commands = append(commands, refreshTickCommand(m.refreshTiming.SafetyInterval))
	}
	if m.liveRefresh != nil {
		commands = append(commands, m.liveRefresh)
	}
	return tea.Batch(commands...)
}

func (m Model) Route() Route {
	return m.route
}

func (m Model) Sessions() []session.Session {
	return cloneSessions(m.sessions)
}

func (m Model) SessionStatuses() map[string]session.Status {
	statuses := make(map[string]session.Status, len(m.sessions))
	for _, s := range m.sessions {
		statuses[s.SessionID] = s.StatusAt(m.now)
	}
	return statuses
}

func (m Model) Countdown(sessionID string) int {
	return m.countdowns[sessionID]
}

func (m Model) WatcherEvents() []WatcherEventMsg {
	return append([]WatcherEventMsg(nil), m.watcherEvents...)
}

func (m Model) NotificationStatuses() []NotificationStatus {
	return append([]NotificationStatus(nil), m.notificationStatuses...)
}

func (m Model) FocusedAction() string {
	return m.focusedAction()
}

func (m Model) LastAction() string {
	return m.lastAction
}

func (m Model) RefreshGeneration() int {
	return m.refreshGeneration
}

func (m Model) LastRefreshSource() refresh.Source {
	return m.lastRefreshSource
}

func (m Model) LastRefreshBypassedDebounce() bool {
	return m.lastBypassedDebounce
}

func (m Model) RefreshDebounceToken() int {
	return m.refreshDebounceToken
}

func (m Model) SelectedSessionID() string {
	if m.selectedID != "" {
		return m.selectedID
	}
	selected := m.selectedSession()
	if selected == nil {
		return ""
	}
	return selected.SessionID
}

func (m Model) ReminderEnabled(sessionID string) bool {
	return m.reminderEnabled[sessionID]
}

func (m Model) KeepAliveEnabled(sessionID string) bool {
	if m.keepAliveEnabled[sessionID] {
		return true
	}
	state := m.KeepAliveState(sessionID)
	return state.State != "" && state.State != keepalive.StateOff
}

func (m Model) KeepAliveState(sessionID string) keepalive.SessionState {
	if m.keepAliveManager == nil {
		return keepalive.SessionState{SessionID: sessionID, State: keepalive.StateOff}
	}
	return m.keepAliveManager.State(sessionID)
}

func routeFromOptions(options Options) Route {
	if options.StartMode == StartConfig {
		return RouteConfig
	}
	if options.AmbiguousID != "" {
		return RouteAmbiguous
	}
	if options.SelectedID != "" {
		return RouteWorkspace
	}
	return RouteList
}

func defaultRefresh(state RefreshViewState) RefreshViewState {
	if state.Watcher.Status == "" {
		state.Watcher.Status = refresh.StatusOK
	}
	if !state.Watcher.SafetyRefreshActive {
		state.Watcher.SafetyRefreshActive = true
	}
	return state
}

func normalizeKeepAliveConfig(cfg config.KeepAliveConfig) config.KeepAliveConfig {
	defaults := config.Default().KeepAlive
	if cfg.TriggerBeforeExpiryMinutes == 0 && cfg.CountdownSeconds == 0 && cfg.Message == "" && cfg.Scope.Mode == "" && cfg.Scope.MaxSends == 0 {
		return defaults
	}
	if cfg.TriggerBeforeExpiryMinutes <= 0 {
		cfg.TriggerBeforeExpiryMinutes = defaults.TriggerBeforeExpiryMinutes
	}
	if cfg.CountdownSeconds <= 0 {
		cfg.CountdownSeconds = defaults.CountdownSeconds
	}
	if cfg.Message == "" {
		cfg.Message = defaults.Message
	}
	if cfg.Scope.Mode == "" {
		cfg.Scope.Mode = defaults.Scope.Mode
	}
	if cfg.Scope.MaxSends <= 0 {
		cfg.Scope.MaxSends = defaults.Scope.MaxSends
	}
	return cfg
}

func normalizeConfig(cfg config.Config) config.Config {
	defaults := config.Default()
	if len(cfg.ReminderThresholds) == 0 && cfg.KeepAlive.TriggerBeforeExpiryMinutes == 0 && cfg.KeepAlive.CountdownSeconds == 0 && cfg.KeepAlive.Message == "" && cfg.KeepAlive.Scope.Mode == "" && cfg.KeepAlive.Scope.MaxSends == 0 {
		return defaults
	}
	if len(cfg.ReminderThresholds) == 0 {
		cfg.ReminderThresholds = append([]int(nil), defaults.ReminderThresholds...)
	} else {
		cfg.ReminderThresholds = append([]int(nil), cfg.ReminderThresholds...)
	}
	cfg.KeepAlive = normalizeKeepAliveConfig(cfg.KeepAlive)
	return cfg
}

func cloneSessions(sessions []session.Session) []session.Session {
	cloned := make([]session.Session, len(sessions))
	for i, s := range sessions {
		cloned[i] = s
		cloned[i].CacheWindow.Evidence = append([]string(nil), s.CacheWindow.Evidence...)
		cloned[i].Gaps = append([]session.Gap(nil), s.Gaps...)
		cloned[i].Warnings = append([]session.ParseWarning(nil), s.Warnings...)
	}
	sort.SliceStable(cloned, func(i, j int) bool {
		return cloned[i].FileModifiedAt.After(cloned[j].FileModifiedAt)
	})
	return cloned
}

func cloneBoolMap(values map[string]bool) map[string]bool {
	cloned := map[string]bool{}
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func selectedIndexFor(sessions []session.Session, selectedID string) int {
	index, _ := findSessionIndex(sessions, selectedID)
	return index
}

func findSessionIndex(sessions []session.Session, selectedID string) (int, bool) {
	if selectedID == "" {
		return 0, false
	}
	for i, s := range sessions {
		if s.SessionID == selectedID || s.ShortID == selectedID {
			return i, true
		}
	}
	return 0, false
}

func findSessionByID(sessions []session.Session, selectedID string) (session.Session, bool) {
	if index, ok := findSessionIndex(sessions, selectedID); ok {
		return sessions[index], true
	}
	return session.Session{}, false
}
