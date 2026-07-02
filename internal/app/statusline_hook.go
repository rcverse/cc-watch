package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/richardchen/cc-watch/internal/ratelimit"
)

const (
	statuslineStdinCap    = 1 << 20 // defensive upper bound; never truncate a real payload
	statuslineTimeout     = 5 * time.Second
	statuslineWrapMarker  = " statusline -- "
	statuslineFallbackTTL = 300 // matches the parser's own "unknown tier" TTL convention
)

// StatuslineRunner spawns an argv-only subprocess for the wrapped statusline
// command, feeding it stdin and relaying its stderr live to stderr. It
// returns the subprocess's captured stdout and a non-nil error on any
// nonzero exit, spawn failure, or timeout -- even when partial stdout was
// still produced.
type StatuslineRunner func(ctx context.Context, stdin []byte, stderr io.Writer, name string, args []string) ([]byte, error)

func runStatuslineCommand(ctx context.Context, stdin []byte, stderr io.Writer, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	return stdout.Bytes(), err
}

// runStatusline is invisible plumbing riding along the user's real
// statusline -- it always exits 0 so a transient hiccup here never visibly
// breaks their setup.
func runStatusline(cmd Command, deps Dependencies, stdin io.Reader, stdout, stderr io.Writer) int {
	if cmd.CheckConfig {
		return runStatuslineCheck(deps, stdout)
	}

	raw, _ := io.ReadAll(io.LimitReader(stdin, statuslineStdinCap))
	suffix := statuslineSuffix(deps, raw)

	var wrapped []byte
	cleanExit := true
	if len(cmd.WrappedCommand) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), statuslineTimeout)
		defer cancel()
		runner := deps.RunStatuslineCommand
		if runner == nil {
			runner = runStatuslineCommand
		}
		out, err := runner(ctx, raw, stderr, cmd.WrappedCommand[0], cmd.WrappedCommand[1:])
		wrapped = out
		cleanExit = err == nil
	}

	if !cleanExit {
		// A wrapped-command hiccup must never produce a garbled combined
		// line -- relay whatever it produced and stop, without cc-watch's
		// own segment.
		stdout.Write(wrapped)
		return 0
	}

	trimmed := bytes.TrimSuffix(wrapped, []byte("\n"))
	stdout.Write(trimmed)
	if suffix != "" {
		if len(trimmed) > 0 {
			stdout.Write([]byte(" "))
		}
		stdout.Write([]byte(suffix))
	}
	return 0
}

type statuslinePayload struct {
	TranscriptPath string `json:"transcript_path"`
	RateLimits     struct {
		FiveHour struct {
			UsedPercentage *float64 `json:"used_percentage"`
			ResetsAt       *int64   `json:"resets_at"`
		} `json:"five_hour"`
	} `json:"rate_limits"`
}

// statuslineSuffix parses the hook payload, updates persisted rate-limit
// state, and returns the display segment to append -- or "" when
// rate_limits wasn't present this turn. Any failure to load/save state or
// resolve the session's cache tier degrades to showing the raw payload
// numbers rather than failing the whole readout.
func statuslineSuffix(deps Dependencies, raw []byte) string {
	var payload statuslinePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	fiveHour := payload.RateLimits.FiveHour
	if fiveHour.UsedPercentage == nil || fiveHour.ResetsAt == nil {
		return ""
	}
	usedPct := *fiveHour.UsedPercentage
	resetsAt := time.Unix(*fiveHour.ResetsAt, 0).UTC()
	pct := fmt.Sprintf("%.0f", usedPct)

	home, err := deps.HomeDir()
	if err != nil {
		return "5h " + pct + "%"
	}
	state, err := ratelimit.Load(home)
	if err != nil {
		return "5h " + pct + "%"
	}

	now := deps.Now()
	state.AddReading(now, ratelimit.Reading{UsedPct: usedPct, ResetsAt: resetsAt})
	ttlSeconds := statuslineTTLSeconds(deps, &state, payload.TranscriptPath)
	_ = ratelimit.Save(home, state) // best-effort; a save failure must not break the statusline

	pctPerMessage, momentumOK := ratelimit.Momentum(state.History)
	timeToReset := resetsAt.Sub(now).Seconds()
	projection, ok := ratelimit.Project(usedPct, pctPerMessage, momentumOK, timeToReset, ttlSeconds)
	if !ok {
		return "5h " + pct + "%"
	}
	if projection.AtRisk {
		return colorAtRisk("! 5h " + pct + "% cap before reset")
	}
	return fmt.Sprintf("5h %s%% ~%d msg left", pct, projection.MessagesLeft)
}

// statuslineTTLSeconds resolves the current session's cache-tier TTL,
// caching it per transcript path so a per-turn hook invocation doesn't pay
// the whole-file scan cost on every turn.
func statuslineTTLSeconds(deps Dependencies, state *ratelimit.State, transcriptPath string) int {
	if transcriptPath == "" {
		return statuslineFallbackTTL
	}
	if cached, ok := state.TierCache[transcriptPath]; ok && cached.Known {
		return cached.TTLSeconds
	}
	ttl := statuslineFallbackTTL
	if parsed, err := deps.ParseFile(transcriptPath); err == nil {
		ttl = parsed.CacheWindow.TTLSeconds
		if parsed.CacheWindow.Known {
			state.TierCache[transcriptPath] = ratelimit.TierInfo{TTLSeconds: ttl, Known: true}
		}
	}
	return ttl
}

func colorAtRisk(s string) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return "\x1b[1;31m" + s + "\x1b[0m"
}

// runStatuslineCheck is a read-only diagnostic: it reads
// ~/.claude/settings.json's statusLine.command and reports the snippet
// needed to reach the next wiring state. It never writes settings.json.
func runStatuslineCheck(deps Dependencies, stdout io.Writer) int {
	home, err := deps.HomeDir()
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 0
	}

	current, err := readStatuslineCommand(home)
	if err != nil {
		fmt.Fprintln(stdout, "Could not read ~/.claude/settings.json: "+err.Error())
		return 0
	}
	if current == "" {
		fmt.Fprintln(stdout, "Not configured. Add to ~/.claude/settings.json:")
		fmt.Fprintln(stdout, "  "+statuslineSettingsSnippet("cc-watch statusline"))
		return 0
	}

	if idx := strings.Index(current, statuslineWrapMarker); idx >= 0 {
		original := strings.TrimSpace(current[idx+len(statuslineWrapMarker):])
		fmt.Fprintln(stdout, "Configured, cc-watch-wrapped:")
		fmt.Fprintln(stdout, "  current:   "+statuslineSettingsSnippet(current))
		fmt.Fprintln(stdout, "  revert to: "+statuslineSettingsSnippet(original))
		return 0
	}

	// Anchored on the actual "cc-watch statusline" invocation token, not a
	// bare "statusline" substring -- a real, unrelated statusline tool can
	// coincidentally contain that substring in its own name (e.g. the
	// illustrative "ccstatusline"), which must never be misreported as an
	// ambiguous cc-watch wrapping state.
	if strings.Contains(current, "cc-watch statusline") {
		fmt.Fprintln(stdout, "Couldn't confidently determine wrapping state. Current value:")
		fmt.Fprintln(stdout, "  "+statuslineSettingsSnippet(current))
		return 0
	}

	wrapped := "cc-watch statusline -- " + current
	if needsShell(current) {
		wrapped = "cc-watch statusline -- sh -c " + shellQuote(current)
	}
	fmt.Fprintln(stdout, "Configured, not cc-watch-wrapped:")
	fmt.Fprintln(stdout, "  current:    "+statuslineSettingsSnippet(current))
	fmt.Fprintln(stdout, "  add cc-watch: "+statuslineSettingsSnippet(wrapped))
	return 0
}

func readStatuslineCommand(home string) (string, error) {
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var settings struct {
		StatusLine struct {
			Command string `json:"command"`
		} `json:"statusLine"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return "", err
	}
	return settings.StatusLine.Command, nil
}

func statuslineSettingsSnippet(command string) string {
	escaped, err := json.Marshal(command)
	if err != nil {
		escaped = []byte(`""`)
	}
	return fmt.Sprintf(`"statusLine": {"type": "command", "command": %s}`, escaped)
}

func needsShell(command string) bool {
	return strings.ContainsAny(command, "|&;<>()$`\\\n") ||
		strings.Contains(command, ">") ||
		strings.Contains(command, "<") ||
		strings.Contains(command, "=")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
