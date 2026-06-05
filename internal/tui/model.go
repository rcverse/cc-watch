package tui

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-cache/internal/notify"
	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
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

type KeepAliveStatus string

const (
	KeepAliveInactive   KeepAliveStatus = ""
	KeepAliveCountdown  KeepAliveStatus = "countdown"
	KeepAliveConfirming KeepAliveStatus = "confirming"
	KeepAliveFailure    KeepAliveStatus = "failure"
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
	Discover                     func()
	Parse                        func()
	Refresh                      func()
	RefreshSessions              func(source refresh.Source, generation int) []session.Session
	RefreshSnapshot              func(source refresh.Source, generation int) RefreshSnapshot
	NotifyEvent                  func(event notify.Event) notify.Result
	ResetNotificationSuppression func()
}

type RefreshSnapshot struct {
	Sessions   []session.Session
	Refresh    RefreshViewState
	HasRefresh bool
}

type NotificationStatus struct {
	Event        notify.Event
	Notification notify.Notification
	Result       notify.Result
}

type Options struct {
	Now                time.Time
	Width              int
	Dependencies       Dependencies
	Sessions           []session.Session
	Countdowns         map[string]int
	ReminderEnabled    map[string]bool
	ReminderThresholds []int
	KeepAliveEnabled   map[string]bool
	RefreshGeneration  int
	SelectedID         string
	AmbiguousID        string
	StartMode          StartMode
	KeepAliveStatus    KeepAliveStatus
	Refresh            RefreshViewState
}

type Model struct {
	now                  time.Time
	width                int
	deps                 Dependencies
	route                Route
	sessions             []session.Session
	countdowns           map[string]int
	reminderEnabled      map[string]bool
	reminderThresholds   []int
	reminderFired        map[string]map[int]bool
	keepAliveEnabled     map[string]bool
	refreshGeneration    int
	watcherEvents        []WatcherEventMsg
	notificationStatuses []NotificationStatus
	helpOpen             bool
	focusIndex           int
	selectedIndex        int
	selectedID           string
	ambiguousID          string
	keepAliveStatus      KeepAliveStatus
	lastAction           string
	refresh              RefreshViewState
	lastRefreshSource    refresh.Source
	lastBypassedDebounce bool
}

var rootFocusActions = []string{"session", "reminder", "keepalive", "refresh", "help", "quit"}
var emptyFocusActions = []string{"refresh", "help", "quit"}

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
	width := options.Width
	if width <= 0 {
		width = 80
	}
	sessions := cloneSessions(options.Sessions)
	selectedIndex := selectedIndexFor(sessions, options.SelectedID)
	return Model{
		width:              width,
		now:                now,
		deps:               options.Dependencies,
		route:              routeFromOptions(options),
		sessions:           sessions,
		countdowns:         countdowns,
		reminderEnabled:    reminders,
		reminderThresholds: thresholds,
		reminderFired:      map[string]map[int]bool{},
		keepAliveEnabled:   keepAlives,
		refreshGeneration:  options.RefreshGeneration,
		selectedIndex:      selectedIndex,
		selectedID:         options.SelectedID,
		ambiguousID:        options.AmbiguousID,
		keepAliveStatus:    options.KeepAliveStatus,
		refresh:            defaultRefresh(options.Refresh),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
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

func (m Model) HelpOpen() bool {
	return m.helpOpen
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
	return m.keepAliveEnabled[sessionID]
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
