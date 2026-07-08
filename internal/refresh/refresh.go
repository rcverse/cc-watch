package refresh

import (
	"time"
)

type Source string

const (
	SourceFsnotify Source = "fsnotify"
	SourceSafety   Source = "safety"
	SourceManual   Source = "manual"
)

type Options struct {
	Debounce          time.Duration
	SafetyInterval    time.Duration
	InitialNow        time.Time
	InitialGeneration int
}

type Decision struct {
	ShouldRefresh    bool
	Source           Source
	Generation       int
	BypassedDebounce bool
	DebounceToken    int
}

type Coordinator struct {
	debounce          time.Duration
	safetyInterval    time.Duration
	now               time.Time
	pendingDebounce   bool
	debounceToken     int
	currentGeneration int
}

func NewCoordinator(options Options) *Coordinator {
	return &Coordinator{
		debounce:          options.Debounce,
		safetyInterval:    options.SafetyInterval,
		now:               options.InitialNow,
		currentGeneration: options.InitialGeneration,
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

func (c *Coordinator) PendingDebounceCount() int {
	if c.pendingDebounce {
		return 1
	}
	return 0
}
