package ratelimit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const maxHistory = 4

// Reading is one turn's account-wide 5-hour rate-limit numbers, as reported
// by the statusLine hook payload.
type Reading struct {
	UsedPct  float64
	ResetsAt time.Time
}

// HistoryPoint is one persisted reading, kept to compute momentum across
// turns.
type HistoryPoint struct {
	CapturedAt time.Time
	UsedPct    float64
	ResetsAt   time.Time
}

// TierInfo is the cached cache-tier TTL for one session's transcript, so a
// per-turn hook invocation doesn't have to re-scan the whole transcript.
type TierInfo struct {
	TTLSeconds int
	Known      bool
}

type State struct {
	History   []HistoryPoint
	TierCache map[string]TierInfo
}

func StatePath(home string) string {
	return filepath.Join(home, ".config", "cc-watch", "ratelimit.json")
}

// Load reads persisted rate-limit state. A missing or corrupt file is not an
// error -- the state self-heals from the next reading, so callers get a
// fresh, empty state instead of a fatal error in either case.
func Load(home string) (State, error) {
	path := StatePath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return freshState(), nil
		}
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return freshState(), nil
	}
	if state.TierCache == nil {
		state.TierCache = map[string]TierInfo{}
	}
	return state, nil
}

func Save(home string, state State) error {
	path := StatePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func freshState() State {
	return State{TierCache: map[string]TierInfo{}}
}

// AddReading appends a new reading to the rolling history, keeping only the
// most recent maxHistory points. A reset-window rollover (resets_at differs
// from the last stored reading) clears prior history first -- momentum
// across a reset boundary is meaningless.
func (s *State) AddReading(capturedAt time.Time, reading Reading) {
	if len(s.History) > 0 && !s.History[len(s.History)-1].ResetsAt.Equal(reading.ResetsAt) {
		s.History = nil
	}
	s.History = append(s.History, HistoryPoint{
		CapturedAt: capturedAt,
		UsedPct:    reading.UsedPct,
		ResetsAt:   reading.ResetsAt,
	})
	if len(s.History) > maxHistory {
		s.History = s.History[len(s.History)-maxHistory:]
	}
}
