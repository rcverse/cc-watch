package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/richardchen/cc-watch/internal/ratelimit"
	"github.com/richardchen/cc-watch/internal/session"
)

func statuslinePayloadJSON(usedPct float64, resetsAt time.Time, transcriptPath string) string {
	return statuslinePayloadJSONWithWeek(usedPct, resetsAt, nil, nil, transcriptPath)
}

func statuslinePayloadJSONWithWeek(usedPct float64, resetsAt time.Time, weekUsedPct *float64, weekResetsAt *time.Time, transcriptPath string) string {
	rateLimits := map[string]any{
		"five_hour": map[string]any{
			"used_percentage": usedPct,
			"resets_at":       resetsAt.Unix(),
		},
	}
	if weekUsedPct != nil && weekResetsAt != nil {
		rateLimits["seven_day"] = map[string]any{
			"used_percentage": *weekUsedPct,
			"resets_at":       weekResetsAt.Unix(),
		}
	}
	data, err := json.Marshal(map[string]any{
		"transcript_path": transcriptPath,
		"rate_limits":     rateLimits,
	})
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestRunStatuslineBareModeNoRateLimitsProducesNoOutput(t *testing.T) {
	deps := fakeDeps(t)
	var stdout, stderr bytes.Buffer

	code := runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(`{}`), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunStatuslineBareModeUnknownMomentumShowsRawPercentage(t *testing.T) {
	deps := fakeDeps(t)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	deps.Now = func() time.Time { return now }
	payload := statuslinePayloadJSON(34, now.Add(3*time.Hour), "")
	var stdout, stderr bytes.Buffer

	code := runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(payload), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout.String() != "⏱ 34% (5h) used" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "⏱ 34% (5h) used")
	}
}

func TestRunStatuslineShowsWeeklyLimitWhenPresent(t *testing.T) {
	deps := fakeDeps(t)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	deps.Now = func() time.Time { return now }
	weekUsedPct := 41.0
	weekResetsAt := now.Add(4 * 24 * time.Hour)
	payload := statuslinePayloadJSONWithWeek(34, now.Add(3*time.Hour), &weekUsedPct, &weekResetsAt, "")
	var stdout bytes.Buffer

	runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(payload), &stdout, io.Discard)

	if stdout.String() != "⏱ 34% (5h) / 41% (7d) used" {
		t.Fatalf("stdout = %q, want weekly limit segment", stdout.String())
	}
}

func TestRunStatuslineMomentumSafeShowsMessagesLeft(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	parseCalls := 0
	deps.ParseFile = func(path string) (session.Session, error) {
		parseCalls++
		return session.Session{CacheWindow: session.CacheWindow{TTLSeconds: 3600, Known: true}}, nil
	}
	now1 := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	now2 := now1.Add(time.Minute)
	resetsAt := now1.Add(3 * time.Hour)

	deps.Now = func() time.Time { return now1 }
	runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(statuslinePayloadJSON(10, resetsAt, "/tmp/session.jsonl")), io.Discard, io.Discard)

	deps.Now = func() time.Time { return now2 }
	var stdout bytes.Buffer
	code := runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(statuslinePayloadJSON(15, resetsAt, "/tmp/session.jsonl")), &stdout, io.Discard)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// pctPerMessage=5, messagesLeft=floor(85/5)=17; pingsNeeded=ceil((10800-60)/3600)=3; safe.
	want := "⏱ 15% (5h) used · ✉ ~17 msgs"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if parseCalls != 1 {
		t.Fatalf("ParseFile calls = %d, want 1 (second turn should hit the cached tier)", parseCalls)
	}
}

func TestRunStatuslineMomentumAtRiskAppendsWarningAndColor(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	now1 := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	now2 := now1.Add(time.Minute)
	resetsAt := now1.Add(10 * time.Hour)

	deps.Now = func() time.Time { return now1 }
	runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(statuslinePayloadJSON(10, resetsAt, "")), io.Discard, io.Discard)

	t.Setenv("NO_COLOR", "")
	deps.Now = func() time.Time { return now2 }
	var stdout bytes.Buffer
	code := runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(statuslinePayloadJSON(15, resetsAt, "")), &stdout, io.Discard)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// pctPerMessage=5, messagesLeft=17; fallback TTL 300s, pingsNeeded=ceil(35940/300)=120; at risk.
	want := "\x1b[1;31m⏱ 15% (5h) used · ⚠ KeepAlive at risk\x1b[0m"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunStatuslineNoColorSuppressesColorCodes(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	now1 := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	now2 := now1.Add(time.Minute)
	resetsAt := now1.Add(10 * time.Hour)

	deps.Now = func() time.Time { return now1 }
	runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(statuslinePayloadJSON(10, resetsAt, "")), io.Discard, io.Discard)

	t.Setenv("NO_COLOR", "1")
	deps.Now = func() time.Time { return now2 }
	var stdout bytes.Buffer
	runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(statuslinePayloadJSON(15, resetsAt, "")), &stdout, io.Discard)

	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("stdout = %q, want no ANSI codes when NO_COLOR is set", stdout.String())
	}
	if !strings.Contains(stdout.String(), "KeepAlive at risk") {
		t.Fatalf("stdout = %q, want at-risk text", stdout.String())
	}
}

func TestRunStatuslineWeeklyLimitCanPutKeepAliveAtRisk(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	now1 := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	now2 := now1.Add(time.Minute)
	resetsAt := now1.Add(30 * time.Minute)
	weekResetsAt := now1.Add(7 * 24 * time.Hour)
	weekUsed1 := 90.0
	weekUsed2 := 95.0

	deps.Now = func() time.Time { return now1 }
	runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(statuslinePayloadJSONWithWeek(10, resetsAt, &weekUsed1, &weekResetsAt, "")), io.Discard, io.Discard)

	t.Setenv("NO_COLOR", "")
	deps.Now = func() time.Time { return now2 }
	var stdout bytes.Buffer
	runStatusline(Command{Mode: ModeStatusline}, deps.Dependencies, strings.NewReader(statuslinePayloadJSONWithWeek(15, resetsAt, &weekUsed2, &weekResetsAt, "")), &stdout, io.Discard)

	want := "\x1b[1;31m⏱ 15% (5h) / 95% (7d) used · ⚠ KeepAlive at risk\x1b[0m"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunStatuslineWrappedCommandSuccessTrimsNewlineAndAppendsSuffix(t *testing.T) {
	deps := fakeDeps(t)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	deps.Now = func() time.Time { return now }
	var capturedStdin []byte
	var capturedName string
	var capturedArgs []string
	deps.RunStatuslineCommand = func(ctx context.Context, stdin []byte, stderr io.Writer, name string, args []string) ([]byte, error) {
		capturedStdin = stdin
		capturedName = name
		capturedArgs = args
		stderr.Write([]byte("warn: from wrapped\n"))
		return []byte("base output\n"), nil
	}
	payload := statuslinePayloadJSON(34, now.Add(3*time.Hour), "")

	var stdout, stderr bytes.Buffer
	code := runStatusline(Command{Mode: ModeStatusline, WrappedCommand: []string{"ccstatusline", "--flag"}}, deps.Dependencies, strings.NewReader(payload), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if capturedName != "ccstatusline" || len(capturedArgs) != 1 || capturedArgs[0] != "--flag" {
		t.Fatalf("runner invoked with name=%q args=%#v, want argv-only ccstatusline --flag (never a shell)", capturedName, capturedArgs)
	}
	if string(capturedStdin) != payload {
		t.Fatalf("runner stdin = %q, want original untruncated payload", capturedStdin)
	}
	if stdout.String() != "base output | ⏱ 34% (5h) used" {
		t.Fatalf("stdout = %q, want trimmed wrapped output plus suffix", stdout.String())
	}
	if stderr.String() != "warn: from wrapped\n" {
		t.Fatalf("stderr = %q, want wrapped command's stderr relayed verbatim", stderr.String())
	}
}

func TestRunStatuslineWrappedCommandFailureRelaysPartialOutputWithoutSuffix(t *testing.T) {
	deps := fakeDeps(t)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	deps.Now = func() time.Time { return now }
	deps.RunStatuslineCommand = func(ctx context.Context, stdin []byte, stderr io.Writer, name string, args []string) ([]byte, error) {
		return []byte("partial"), errors.New("exit status 1")
	}
	payload := statuslinePayloadJSON(34, now.Add(3*time.Hour), "")

	var stdout, stderr bytes.Buffer
	code := runStatusline(Command{Mode: ModeStatusline, WrappedCommand: []string{"ccstatusline"}}, deps.Dependencies, strings.NewReader(payload), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (a wrapped-command hiccup must never break the user's statusline)", code)
	}
	if stdout.String() != "partial" {
		t.Fatalf("stdout = %q, want unmodified partial output, no cc-watch segment appended", stdout.String())
	}
}

func TestRunStatuslineWrappedCommandTimeoutRelaysPartialOutputWithoutSuffix(t *testing.T) {
	deps := fakeDeps(t)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	deps.Now = func() time.Time { return now }
	deps.RunStatuslineCommand = func(ctx context.Context, stdin []byte, stderr io.Writer, name string, args []string) ([]byte, error) {
		return []byte("partial-before-timeout"), context.DeadlineExceeded
	}
	payload := statuslinePayloadJSON(34, now.Add(3*time.Hour), "")

	var stdout bytes.Buffer
	code := runStatusline(Command{Mode: ModeStatusline, WrappedCommand: []string{"ccstatusline"}}, deps.Dependencies, strings.NewReader(payload), &stdout, io.Discard)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout.String() != "partial-before-timeout" {
		t.Fatalf("stdout = %q, want partial output relayed as-is on timeout", stdout.String())
	}
}

func TestRunStatuslineCheckNotConfigured(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }

	var stdout bytes.Buffer
	code := runStatuslineCheck(deps.Dependencies, &stdout)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "statusLine is not configured") {
		t.Fatalf("stdout = %q, want not-configured message", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"type": "command"`) || !strings.Contains(stdout.String(), `"command": "cc-watch statusline"`) {
		t.Fatalf("stdout = %q, want bare command snippet", stdout.String())
	}
	if !strings.Contains(stdout.String(), "No files were changed.") {
		t.Fatalf("stdout = %q, want read-only reminder", stdout.String())
	}
}

func TestRunStatuslineCheckConfiguredNotWrapped(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	writeSettings(t, home, `{"statusLine":{"command":"ccstatusline"}}`)

	var stdout bytes.Buffer
	code := runStatuslineCheck(deps.Dependencies, &stdout)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "cc-watch is not in the chain") {
		t.Fatalf("stdout = %q, want not-in-chain message", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"command": "cc-watch statusline -- ccstatusline"`) {
		t.Fatalf("stdout = %q, want wrapped snippet", stdout.String())
	}
	if !strings.Contains(stdout.String(), "To undo later:") || !strings.Contains(stdout.String(), "No files were changed.") {
		t.Fatalf("stdout = %q, want reversible read-only copy", stdout.String())
	}
}

func TestRunStatuslineCheckConfiguredShellCommandWrapsThroughSh(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	writeSettings(t, home, `{"statusLine":{"command":"echo 'hi' | sed s/i/o/"}}`)

	var stdout bytes.Buffer
	code := runStatuslineCheck(deps.Dependencies, &stdout)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := `"command": "cc-watch statusline -- sh -c 'echo '\\''hi'\\'' | sed s/i/o/'"`
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout = %q, want shell-preserving wrapped snippet containing %q", stdout.String(), want)
	}
}

func TestRunStatuslineCheckConfiguredWrappedWithAbsolutePath(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	writeSettings(t, home, `{"statusLine":{"command":"~/.local/bin/cc-watch statusline -- ccstatusline --theme dark"}}`)

	var stdout bytes.Buffer
	code := runStatuslineCheck(deps.Dependencies, &stdout)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "already includes cc-watch") {
		t.Fatalf("stdout = %q, want detection via absolute install path, not just a bare cc-watch prefix", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"command": "ccstatusline --theme dark"`) {
		t.Fatalf("stdout = %q, want revert-to original inner command", stdout.String())
	}
	if !strings.Contains(stdout.String(), "To undo:") || !strings.Contains(stdout.String(), "No files were changed.") {
		t.Fatalf("stdout = %q, want undo read-only copy", stdout.String())
	}
}

func TestRunStatuslineCheckAmbiguousWrappingReportsUncertainty(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	// Mentions cc-watch, but is not the runtime statusline command.
	writeSettings(t, home, `{"statusLine":{"command":"cc-watch statusline --check"}}`)

	var stdout bytes.Buffer
	code := runStatuslineCheck(deps.Dependencies, &stdout)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "wrapper shape is unclear") {
		t.Fatalf("stdout = %q, want uncertainty message for ambiguous statusline value", stdout.String())
	}
	if !strings.Contains(stdout.String(), "No files were changed.") {
		t.Fatalf("stdout = %q, want read-only reminder", stdout.String())
	}
}

func TestRunStatuslineCheckMalformedSettingsReportsDiagnostic(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	writeSettings(t, home, `{not-json`)

	var stdout bytes.Buffer
	code := runStatuslineCheck(deps.Dependencies, &stdout)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Could not read ~/.claude/settings.json") {
		t.Fatalf("stdout = %q, want settings diagnostic", stdout.String())
	}
}

func TestRunStatuslineCheckNeverWritesSettingsFile(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	path := filepath.Join(home, ".claude", "settings.json")
	writeSettings(t, home, `{"statusLine":{"command":"ccstatusline"}}`)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	runStatuslineCheck(deps.Dependencies, io.Discard)

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after check: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("settings.json changed after --check: before=%q after=%q", before, after)
	}
}

func writeSettings(t *testing.T, home, contents string) {
	t.Helper()
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestStatuslineDispatchReadsStdinAndExitsZero(t *testing.T) {
	deps := fakeDeps(t)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	deps.Now = func() time.Time { return now }
	deps.Stdin = strings.NewReader(statuslinePayloadJSON(34, now.Add(3*time.Hour), ""))

	var stdout, stderr bytes.Buffer
	code := RunWithDeps([]string{"statusline"}, &stdout, &stderr, deps.Dependencies)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stdout.String() != "⏱ 34% (5h) used" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "⏱ 34% (5h) used")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestStatuslineTTLRetriesAfterUnknownTier(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	calls := 0
	deps.ParseFile = func(path string) (session.Session, error) {
		calls++
		if calls == 1 {
			return session.Session{CacheWindow: session.CacheWindow{TTLSeconds: 300, Known: false}}, nil
		}
		return session.Session{CacheWindow: session.CacheWindow{TTLSeconds: 3600, Known: true}}, nil
	}
	state, err := ratelimit.Load(home)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := statuslineTTLSeconds(deps.Dependencies, &state, "/tmp/session.jsonl"); got != 300 {
		t.Fatalf("first TTL = %d, want fallback 300", got)
	}
	if got := statuslineTTLSeconds(deps.Dependencies, &state, "/tmp/session.jsonl"); got != 3600 {
		t.Fatalf("second TTL = %d, want known parsed TTL 3600", got)
	}
	if calls != 2 {
		t.Fatalf("ParseFile calls = %d, want retry after unknown tier", calls)
	}
}
