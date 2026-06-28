package refresh

import (
	"time"

	"github.com/richardchen/cc-cache/internal/session"
)

type Source string

const (
	SourceFsnotify Source = "fsnotify"
	SourceSafety   Source = "safety"
	SourceManual   Source = "manual"
)

type Parser func(source string) []session.Session

type Options struct {
	Debounce          time.Duration
	SafetyInterval    time.Duration
	Parser            Parser
	ProjectsDir       string
	InitialNow        time.Time
	InitialSessions   []session.Session
	InitialGeneration int
}

type Decision struct {
	ShouldRefresh    bool
	Source           Source
	Generation       int
	BypassedDebounce bool
	DebounceToken    int
}

type Result struct {
	Generation int
	Sessions   []session.Session
}

type Coordinator struct {
	debounce          time.Duration
	safetyInterval    time.Duration
	parser            Parser
	projectsDir       string
	now               time.Time
	sessions          []session.Session
	pendingDebounce   bool
	debounceToken     int
	currentGeneration int
	appliedGeneration int
}

func NewCoordinator(options Options) *Coordinator {
	return &Coordinator{
		debounce:          options.Debounce,
		safetyInterval:    options.SafetyInterval,
		parser:            options.Parser,
		projectsDir:       options.ProjectsDir,
		now:               options.InitialNow,
		sessions:          cloneSessions(options.InitialSessions),
		currentGeneration: options.InitialGeneration,
		appliedGeneration: options.InitialGeneration,
	}
}

func (c *Coordinator) OnWatcherEvents(events []NormalizedEvent) Decision {
	if len(events) == 0 {
		return Decision{}
	}
	c.pendingDebounce = true
	c.debounceToken++
	return Decision{Source: SourceFsnotify, DebounceToken: c.debounceToken}
}

func (c *Coordinator) OnDebounceElapsed(now time.Time, token int) Decision {
	c.now = now
	if !c.pendingDebounce || token != c.debounceToken {
		return Decision{}
	}
	c.pendingDebounce = false
	return c.NextRefresh(SourceFsnotify)
}

func (c *Coordinator) OnSafetyTick(now time.Time) Decision {
	c.now = now
	return c.NextRefresh(SourceSafety)
}

func (c *Coordinator) OnManualRefresh() Decision {
	decision := c.NextRefresh(SourceManual)
	decision.BypassedDebounce = true
	return decision
}

func (c *Coordinator) NextRefresh(source Source) Decision {
	c.currentGeneration++
	return Decision{
		ShouldRefresh: true,
		Source:        source,
		Generation:    c.currentGeneration,
	}
}

func (c *Coordinator) ApplyResult(result Result) {
	if result.Generation != c.currentGeneration {
		return
	}
	c.appliedGeneration = result.Generation
	c.sessions = cloneSessions(result.Sessions)
}

func (c *Coordinator) Run(decision Decision) Result {
	if !decision.ShouldRefresh || c.parser == nil {
		return Result{Generation: decision.Generation}
	}
	return Result{
		Generation: decision.Generation,
		Sessions:   c.parser(string(decision.Source)),
	}
}

func (c *Coordinator) Sessions() []session.Session {
	return cloneSessions(c.sessions)
}

func (c *Coordinator) PendingDebounceCount() int {
	if c.pendingDebounce {
		return 1
	}
	return 0
}

func cloneSessions(sessions []session.Session) []session.Session {
	cloned := make([]session.Session, len(sessions))
	for i, s := range sessions {
		cloned[i] = s
		cloned[i].CacheWindow.Evidence = append([]string(nil), s.CacheWindow.Evidence...)
		cloned[i].Gaps = append([]session.Gap(nil), s.Gaps...)
		cloned[i].Warnings = append([]session.ParseWarning(nil), s.Warnings...)
	}
	return cloned
}
