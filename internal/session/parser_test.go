package session

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseAllFeaturesFixture(t *testing.T) {
	s, err := ParseFile(filepath.Join("testdata", "all-features.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if s.SessionID != "all-features" {
		t.Fatalf("SessionID = %q, want all-features", s.SessionID)
	}
	if s.CacheWindow.Tier != Tier1Hour {
		t.Fatalf("Tier = %q, want %q", s.CacheWindow.Tier, Tier1Hour)
	}
	if s.CacheWindow.TTLSeconds != 3600 || !s.CacheWindow.Known {
		t.Fatalf("CacheWindow = %+v, want known 3600s", s.CacheWindow)
	}
	if !contains(s.CacheWindow.Evidence, "ephemeral_1h_input_tokens") {
		t.Fatalf("Evidence = %v, want 1h evidence", s.CacheWindow.Evidence)
	}

	if s.TokenStats.CacheWrites != 130 {
		t.Fatalf("CacheWrites = %d, want 130", s.TokenStats.CacheWrites)
	}
	if s.TokenStats.CacheReads != 200 {
		t.Fatalf("CacheReads = %d, want 200", s.TokenStats.CacheReads)
	}
	if s.TokenStats.OutputTokens != 25 {
		t.Fatalf("OutputTokens = %d, want 25", s.TokenStats.OutputTokens)
	}
	if s.TokenStats.HitRate != 60.60606060606061 {
		t.Fatalf("HitRate = %.14f, want 60.60606060606061", s.TokenStats.HitRate)
	}
	if s.StartedAt == nil || s.EndedAt == nil {
		t.Fatalf("StartedAt/EndedAt = %v/%v, want timestamps", s.StartedAt, s.EndedAt)
	}
	if got := s.StartedAt.Format(time.RFC3339); got != "2026-06-03T00:00:00Z" {
		t.Fatalf("StartedAt = %s, want first timestamp", got)
	}
	if got := s.EndedAt.Format(time.RFC3339); got != "2026-06-03T01:12:40Z" {
		t.Fatalf("EndedAt = %s, want last timestamp", got)
	}
	if s.DurationSeconds == nil || *s.DurationSeconds != 4360 {
		t.Fatalf("DurationSeconds = %v, want 4360", s.DurationSeconds)
	}

	if got := s.Messages.FirstUserExcerpt; got != "first synthetic prompt" {
		t.Fatalf("FirstUserExcerpt = %q", got)
	}
	if got := s.Messages.LastUserExcerpt; got != "last synthetic prompt from list block" {
		t.Fatalf("LastUserExcerpt = %q", got)
	}

	if len(s.Gaps) != 2 {
		t.Fatalf("len(Gaps) = %d, want 2", len(s.Gaps))
	}
	if s.ResetCount != 1 {
		t.Fatalf("ResetCount = %d, want 1", s.ResetCount)
	}
	if !s.Gaps[0].Reset || s.Gaps[0].Seconds != 4210 {
		t.Fatalf("largest gap = %+v, want 4210s reset", s.Gaps[0])
	}

	assertWarning(t, s.Warnings, WarningMalformedJSON)
	assertWarning(t, s.Warnings, WarningMalformedTimestamp)

	status := s.StatusAt(time.Date(2026, 6, 3, 1, 42, 40, 0, time.UTC))
	if status.State != StatusActive {
		t.Fatalf("status.State = %q, want active", status.State)
	}
	if status.RemainingSeconds == nil || *status.RemainingSeconds != 1800 {
		t.Fatalf("RemainingSeconds = %v, want 1800", status.RemainingSeconds)
	}
	if status.ExpiredSeconds != nil {
		t.Fatalf("ExpiredSeconds = %v, want nil", status.ExpiredSeconds)
	}
}

func TestParseUnknownTTLUsesConservativeResetHeuristic(t *testing.T) {
	s, err := ParseFile(filepath.Join("testdata", "unknown-ttl-reset.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if s.CacheWindow.Tier != TierUnknown {
		t.Fatalf("Tier = %q, want unknown", s.CacheWindow.Tier)
	}
	if s.CacheWindow.Known {
		t.Fatal("CacheWindow.Known = true, want false")
	}
	if s.CacheWindow.TTLSeconds != 300 {
		t.Fatalf("TTLSeconds = %d, want conservative 300", s.CacheWindow.TTLSeconds)
	}
	if s.ResetCount != 1 {
		t.Fatalf("ResetCount = %d, want 1", s.ResetCount)
	}
	status := s.StatusAt(time.Date(2026, 6, 3, 0, 7, 0, 0, time.UTC))
	if status.State != StatusUnknown {
		t.Fatalf("status.State = %q, want unknown for unknown TTL display", status.State)
	}
}

func TestParseTimestamplessFile(t *testing.T) {
	s, err := ParseFile(filepath.Join("testdata", "timestampless.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if s.LastMessageAt != nil {
		t.Fatalf("LastMessageAt = %v, want nil", s.LastMessageAt)
	}
	status := s.StatusAt(time.Date(2026, 6, 3, 1, 0, 0, 0, time.UTC))
	if status.State != StatusUnknown {
		t.Fatalf("status.State = %q, want unknown", status.State)
	}
	if s.TokenStats.CacheWrites != 7 || s.TokenStats.CacheReads != 3 || s.TokenStats.OutputTokens != 1 {
		t.Fatalf("TokenStats = %+v", s.TokenStats)
	}
}

func TestParseReaderKeepsRecentMessageWindows(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		`{"timestamp":"2026-06-03T00:00:00Z","message":{"role":"user","content":"<local-command-caveat>Caveat: hidden"}}`,
		`{"timestamp":"2026-06-03T00:00:30Z","message":{"role":"user","content":"<local-command-stdout>Set model to Opus</local-command-stdout>"}}`,
		`{"timestamp":"2026-06-03T00:00:00Z","message":{"role":"user","content":"first prompt"},"usage":{"ephemeral_1h_input_tokens":1}}`,
		`{"timestamp":"2026-06-03T00:01:00Z","message":{"role":"assistant","content":"first answer"}}`,
		`{"timestamp":"2026-06-03T00:01:30Z","message":{"role":"user","content":"<command-name>/reload-skills</command-name>"}}`,
		`{"timestamp":"2026-06-03T00:02:00Z","message":{"role":"user","content":[{"type":"text","text":"second prompt from block"},{"type":"tool_use","name":"ignored"}]}}`,
	}, "\n"))

	s, err := ParseReader(input, "recent.jsonl", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.RecentMessages) != 2 {
		t.Fatalf("len(RecentMessages) = %d, want 2", len(s.RecentMessages))
	}
	if s.RecentMessages[0].Role != "user" || s.RecentMessages[0].Excerpt != "first prompt" {
		t.Fatalf("first recent message = %#v", s.RecentMessages[0])
	}
	if s.RecentMessages[1].Role != "user" || s.RecentMessages[1].Excerpt != "second prompt from block" {
		t.Fatalf("last recent message = %#v", s.RecentMessages[1])
	}
	if s.Messages.FirstUserExcerpt != "first prompt" {
		t.Fatalf("FirstUserExcerpt = %q, want first prompt", s.Messages.FirstUserExcerpt)
	}
	if s.Messages.LastUserExcerpt != "second prompt from block" {
		t.Fatalf("LastUserExcerpt = %q, want second prompt from block", s.Messages.LastUserExcerpt)
	}
}

func TestParseReaderCapsRecentMessageWindows(t *testing.T) {
	var lines []string
	for i := 0; i < recentMessageLimit+1; i++ {
		lines = append(lines, `{"timestamp":"2026-06-03T00:`+fmt.Sprintf("%02d", i)+`:00Z","message":{"role":"user","content":"prompt `+fmt.Sprint(i)+`"}}`)
	}
	s, err := ParseReader(strings.NewReader(strings.Join(lines, "\n")), "recent-cap.jsonl", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.RecentMessages) != recentMessageLimit {
		t.Fatalf("len(RecentMessages) = %d, want %d", len(s.RecentMessages), recentMessageLimit)
	}
	if s.RecentMessages[0].Excerpt != "prompt 1" {
		t.Fatalf("first retained excerpt = %q, want prompt 1", s.RecentMessages[0].Excerpt)
	}
}

func TestParseLongJSONLLine(t *testing.T) {
	s, err := ParseFile(filepath.Join("testdata", "long-line.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if len(s.Messages.FirstUserExcerpt) != 70000 {
		t.Fatalf("long excerpt length = %d, want 70000", len(s.Messages.FirstUserExcerpt))
	}
	if s.CacheWindow.Tier != Tier5Minute {
		t.Fatalf("Tier = %q, want 5m", s.CacheWindow.Tier)
	}
}

func TestParseShortAndMalformedFilenameStems(t *testing.T) {
	short, err := ParseFile(filepath.Join("testdata", "short.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile(short) returned error: %v", err)
	}
	if short.SessionID != "short" || short.ShortID != "short" {
		t.Fatalf("short IDs = %q/%q, want short/short", short.SessionID, short.ShortID)
	}

	malformed, err := ParseFile(filepath.Join("testdata", "bad-stem!.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile(bad-stem) returned error: %v", err)
	}
	if malformed.SessionID != "bad-stem!" {
		t.Fatalf("malformed SessionID = %q, want bad-stem!", malformed.SessionID)
	}
	if malformed.ShortID != "bad-stem" {
		t.Fatalf("malformed ShortID = %q, want first 8 characters", malformed.ShortID)
	}
}

func TestParseReaderReportsReadErrorAfterValidLines(t *testing.T) {
	readerErr := errors.New("synthetic read failure")
	reader := &errorAfterValidLineReader{
		line: `{"timestamp":"2026-06-03T00:00:00Z","usage":{"cache_creation_input_tokens":1},"message":{"role":"user","content":"valid before error"}}` + "\n",
		err:  readerErr,
	}

	s, err := ParseReader(reader, "read-error.jsonl", time.Time{})
	if !errors.Is(err, readerErr) {
		t.Fatalf("ParseReader error = %v, want reader error", err)
	}
	if s.TokenStats.CacheWrites != 1 {
		t.Fatalf("CacheWrites = %d, want parsed valid line before read error", s.TokenStats.CacheWrites)
	}
	assertWarning(t, s.Warnings, WarningReadError)
}

func TestParseMalformedTimestampWarningDoesNotDropUsage(t *testing.T) {
	s, err := ParseReader(strings.NewReader(`{"timestamp":"bad","usage":{"cache_read_input_tokens":9},"message":{"role":"user","content":"bad timestamp still counts"}}`+"\n"), "bad-time.jsonl", time.Time{})
	if err != nil {
		t.Fatalf("ParseReader returned error: %v", err)
	}

	if s.TokenStats.CacheReads != 9 {
		t.Fatalf("CacheReads = %d, want 9", s.TokenStats.CacheReads)
	}
	if s.LastMessageAt != nil {
		t.Fatalf("LastMessageAt = %v, want nil", s.LastMessageAt)
	}
	assertWarning(t, s.Warnings, WarningMalformedTimestamp)
}

func TestParseCapturesLastNonEmptyCwd(t *testing.T) {
	lines := `{"cwd":"/Users/x/proj","timestamp":"2026-07-03T10:00:00Z"}` + "\n" +
		`{"timestamp":"2026-07-03T10:00:01Z"}` + "\n" +
		`{"cwd":"/Users/x/proj/sub","timestamp":"2026-07-03T10:00:02Z"}` + "\n"
	s, err := ParseReader(strings.NewReader(lines), "cwd.jsonl", time.Time{})
	if err != nil {
		t.Fatalf("ParseReader returned error: %v", err)
	}
	if s.Cwd != "/Users/x/proj/sub" {
		t.Fatalf("Cwd = %q, want /Users/x/proj/sub", s.Cwd)
	}
}

func TestParseFallsBackToNestedUsageWhenTopLevelUsageIsEmpty(t *testing.T) {
	s, err := ParseReader(strings.NewReader(`{"timestamp":"2026-06-03T00:00:00Z","usage":{},"message":{"role":"assistant","usage":{"cache_read_input_tokens":11,"output_tokens":4}}}`+"\n"), "empty-top-usage.jsonl", time.Time{})
	if err != nil {
		t.Fatalf("ParseReader returned error: %v", err)
	}

	if s.TokenStats.CacheReads != 11 {
		t.Fatalf("CacheReads = %d, want nested usage fallback", s.TokenStats.CacheReads)
	}
	if s.TokenStats.OutputTokens != 4 {
		t.Fatalf("OutputTokens = %d, want nested usage fallback", s.TokenStats.OutputTokens)
	}
}

type errorAfterValidLineReader struct {
	line string
	err  error
	sent bool
}

func (r *errorAfterValidLineReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, r.err
	}
	r.sent = true
	return copy(p, r.line), nil
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func assertWarning(t *testing.T, warnings []ParseWarning, code WarningCode) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Code == code {
			return
		}
	}
	t.Fatalf("warnings = %+v, want code %q", warnings, code)
}

var _ io.Reader = (*errorAfterValidLineReader)(nil)
