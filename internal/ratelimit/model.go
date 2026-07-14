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
	UsedPct  float64
	ResetsAt time.Time
}

type State struct {
	History         []HistoryPoint
	SevenDayHistory []HistoryPoint
	CacheSnapshots  map[string]CacheSnapshot
}

type CacheSnapshot struct {
	TTLSeconds     int        `json:"ttl_seconds"`
	Known          bool       `json:"known"`
	CacheAnchorAt  *time.Time `json:"cache_anchor_at,omitempty"`
	FileModifiedAt time.Time  `json:"file_modified_at"`
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
	if state.CacheSnapshots == nil {
		state.CacheSnapshots = map[string]CacheSnapshot{}
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
	return State{CacheSnapshots: map[string]CacheSnapshot{}}
}

// AddReading appends a new reading to the rolling history, keeping only the
// most recent maxHistory points. A reset-window rollover (resets_at differs
// from the last stored reading) clears prior history first -- momentum
// across a reset boundary is meaningless.
func (s *State) AddReading(reading Reading) {
	addReading(&s.History, reading)
}

func (s *State) AddSevenDayReading(reading Reading) {
	addReading(&s.SevenDayHistory, reading)
}

func addReading(history *[]HistoryPoint, reading Reading) {
	points := *history
	if len(points) > 0 && !points[len(points)-1].ResetsAt.Equal(reading.ResetsAt) {
		points = nil
	}
	points = append(points, HistoryPoint{
		UsedPct:  reading.UsedPct,
		ResetsAt: reading.ResetsAt,
	})
	if len(points) > maxHistory {
		points = points[len(points)-maxHistory:]
	}
	*history = points
}
