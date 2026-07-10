package ratelimit

import "testing"

func TestProjectUnknownWhenMomentumUnknown(t *testing.T) {
	_, ok := Project(50, 0, false, 3600, 300)
	if ok {
		t.Fatal("Project with unknown momentum = ok, want unknown")
	}
}

func TestProjectUnknownWhenTTLMissing(t *testing.T) {
	_, ok := Project(50, 5, true, 3600, 0)
	if ok {
		t.Fatal("Project with zero TTL = ok, want unknown")
	}
}

func TestProjectSafeWhenMessagesLeftExceedsPingsNeeded(t *testing.T) {
	// 50% remaining / 5% per message = 10 messages left.
	// 3600s to reset / 3600s TTL = 1 ping needed.
	proj, ok := Project(50, 5, true, 3600, 3600)
	if !ok {
		t.Fatal("Project = unknown, want ok")
	}
	if proj.MessagesLeft != 10 {
		t.Fatalf("MessagesLeft = %d, want 10", proj.MessagesLeft)
	}
	if proj.AtRisk {
		t.Fatal("AtRisk = true, want false")
	}
}

func TestProjectAtRiskWhenMessagesLeftAtOrBelowPingsNeeded(t *testing.T) {
	// 10% remaining / 5% per message = 2 messages left.
	// 7200s to reset / 3600s TTL = 2 pings needed -- at the boundary.
	proj, ok := Project(90, 5, true, 7200, 3600)
	if !ok {
		t.Fatal("Project = unknown, want ok")
	}
	if proj.MessagesLeft != 2 {
		t.Fatalf("MessagesLeft = %d, want 2", proj.MessagesLeft)
	}
	if !proj.AtRisk {
		t.Fatal("AtRisk = false, want true at the messagesLeft == pingsNeeded boundary")
	}
}
