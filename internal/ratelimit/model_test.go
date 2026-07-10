package ratelimit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsFreshState(t *testing.T) {
	home := t.TempDir()

	state, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if state.TierCache == nil {
		t.Fatal("TierCache = nil, want initialized empty map")
	}
	if len(state.History) != 0 {
		t.Fatalf("History = %#v, want empty", state.History)
	}
}

func TestLoadInvalidJSONReturnsFreshStateNotError(t *testing.T) {
	home := t.TempDir()
	path := StatePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v, want self-healing nil", err)
	}
	if state.TierCache == nil {
		t.Fatal("TierCache = nil, want initialized empty map")
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	home := t.TempDir()
	resetsAt := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	state := freshState()
	state.AddReading(Reading{UsedPct: 10, ResetsAt: resetsAt})
	state.TierCache["/tmp/session.jsonl"] = 3600

	if err := Save(home, state); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	loaded, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.History) != 1 || loaded.History[0].UsedPct != 10 {
		t.Fatalf("History = %#v, want one reading with UsedPct 10", loaded.History)
	}
	if loaded.TierCache["/tmp/session.jsonl"] != 3600 {
		t.Fatalf("TierCache = %#v, want TTLSeconds 3600", loaded.TierCache)
	}
}

func TestAddReadingKeepsOnlyMostRecentHistory(t *testing.T) {
	resetsAt := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	state := freshState()
	for i := 0; i < 6; i++ {
		state.AddReading(Reading{
			UsedPct:  float64(i * 10),
			ResetsAt: resetsAt,
		})
	}

	if len(state.History) != maxHistory {
		t.Fatalf("len(History) = %d, want %d", len(state.History), maxHistory)
	}
	if state.History[len(state.History)-1].UsedPct != 50 {
		t.Fatalf("last UsedPct = %v, want 50 (most recent reading kept)", state.History[len(state.History)-1].UsedPct)
	}
}

func TestAddReadingClearsHistoryOnRollover(t *testing.T) {
	firstReset := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	secondReset := firstReset.Add(5 * time.Hour)
	state := freshState()
	state.AddReading(Reading{UsedPct: 90, ResetsAt: firstReset})
	state.AddReading(Reading{UsedPct: 95, ResetsAt: firstReset})

	state.AddReading(Reading{UsedPct: 5, ResetsAt: secondReset})

	if len(state.History) != 1 {
		t.Fatalf("len(History) = %d, want 1 (rollover should clear prior history)", len(state.History))
	}
	if state.History[0].UsedPct != 5 {
		t.Fatalf("History[0].UsedPct = %v, want 5", state.History[0].UsedPct)
	}
}
