package refresh

type Decision struct {
	ShouldRefresh bool
	Generation    int
	DebounceToken int
}

type Coordinator struct {
	pendingDebounce   bool
	debounceToken     int
	currentGeneration int
}

func NewCoordinator(initialGeneration int) *Coordinator {
	return &Coordinator{currentGeneration: initialGeneration}
}

func (c *Coordinator) OnWatcherEvent() int {
	c.pendingDebounce = true
	c.debounceToken++
	return c.debounceToken
}

func (c *Coordinator) OnDebounceElapsed(token int) Decision {
	if !c.pendingDebounce || token != c.debounceToken {
		return Decision{}
	}
	c.pendingDebounce = false
	return c.Refresh()
}

func (c *Coordinator) Refresh() Decision {
	c.currentGeneration++
	return Decision{
		ShouldRefresh: true,
		Generation:    c.currentGeneration,
	}
}
