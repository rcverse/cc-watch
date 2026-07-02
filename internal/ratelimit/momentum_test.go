package ratelimit

import "testing"

func TestMomentumRequiresAtLeastTwoReadings(t *testing.T) {
	_, ok := Momentum([]HistoryPoint{{UsedPct: 10}})
	if ok {
		t.Fatal("Momentum with one reading = ok, want unknown")
	}
	_, ok = Momentum(nil)
	if ok {
		t.Fatal("Momentum with no readings = ok, want unknown")
	}
}

func TestMomentumAveragesLastThreeDeltas(t *testing.T) {
	history := []HistoryPoint{
		{UsedPct: 0},
		{UsedPct: 40}, // delta 40, dropped -- only last 3 deltas count
		{UsedPct: 44}, // delta 4
		{UsedPct: 50}, // delta 6
		{UsedPct: 58}, // delta 8
	}
	pctPerMessage, ok := Momentum(history)
	if !ok {
		t.Fatal("Momentum = unknown, want ok")
	}
	want := (4.0 + 6.0 + 8.0) / 3.0
	if pctPerMessage != want {
		t.Fatalf("pctPerMessage = %v, want %v", pctPerMessage, want)
	}
}

func TestMomentumFloorReturnsUnknownOnIdleAccount(t *testing.T) {
	history := []HistoryPoint{
		{UsedPct: 34.0},
		{UsedPct: 34.1},
	}
	pctPerMessage, ok := Momentum(history)
	if ok {
		t.Fatalf("Momentum = %v, ok, want unknown for near-zero movement (idle account)", pctPerMessage)
	}
}

func TestMomentumSingleDelta(t *testing.T) {
	history := []HistoryPoint{
		{UsedPct: 10},
		{UsedPct: 15},
	}
	pctPerMessage, ok := Momentum(history)
	if !ok {
		t.Fatal("Momentum = unknown, want ok")
	}
	if pctPerMessage != 5 {
		t.Fatalf("pctPerMessage = %v, want 5", pctPerMessage)
	}
}
