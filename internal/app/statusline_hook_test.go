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
	data, err := json.Marshal(map[string]any{
		"transcript_path": transcriptPath,
		"rate_limits": map[string]any{
			"five_hour": map[string]any{
				"used_percentage": usedPct,
				"resets_at":       resetsAt.Unix(),
			},
		},
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
	if stdout.String() != "5h 34%" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "5h 34%")
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
	want := "5h 15% ~17 msg left"
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
	want := "\x1b[1;31m! 5h 15% cap before reset\x1b[0m"
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
	if !strings.Contains(stdout.String(), "cap before reset") {
		t.Fatalf("stdout = %q, want at-risk text", stdout.String())
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
	if stdout.String() != "base output 5h 34%" {
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
	if !strings.Contains(stdout.String(), "Not configured") {
		t.Fatalf("stdout = %q, want 'Not configured'", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"type": "command"`) || !strings.Contains(stdout.String(), `"command": "cc-watch statusline"`) {
		t.Fatalf("stdout = %q, want bare command snippet", stdout.String())
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
	if !strings.Contains(stdout.String(), "not cc-watch-wrapped") {
		t.Fatalf("stdout = %q, want 'not cc-watch-wrapped'", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"command": "cc-watch statusline -- ccstatusline"`) {
		t.Fatalf("stdout = %q, want wrapped snippet", stdout.String())
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
	if !strings.Contains(stdout.String(), "cc-watch-wrapped") {
		t.Fatalf("stdout = %q, want detection via absolute install path, not just a bare cc-watch prefix", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"command": "ccstatusline --theme dark"`) {
		t.Fatalf("stdout = %q, want revert-to original inner command", stdout.String())
	}
}

func TestRunStatuslineCheckAmbiguousWrappingReportsUncertainty(t *testing.T) {
	deps := fakeDeps(t)
	home := t.TempDir()
	deps.HomeDir = func() (string, error) { return home, nil }
	// Already contains cc-watch's own subcommand invocation, but with no
	// recognizable " -- " wrap marker (e.g. already bare-configured) --
	// genuinely ambiguous, unlike an unrelated tool that merely shares the
	// "statusline" substring in its own name.
	writeSettings(t, home, `{"statusLine":{"command":"cc-watch statusline"}}`)

	var stdout bytes.Buffer
	code := runStatuslineCheck(deps.Dependencies, &stdout)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Couldn't confidently determine") {
		t.Fatalf("stdout = %q, want uncertainty message for ambiguous statusline value", stdout.String())
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
	if stdout.String() != "5h 34%" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "5h 34%")
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
