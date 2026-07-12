package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/rcverse/cc-watch/internal/ratelimit"
	"github.com/rcverse/cc-watch/internal/statusline"
)

const (
	statuslineStdinCap    = 1 << 20 // defensive upper bound; never truncate a real payload
	statuslineTimeout     = 5 * time.Second
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
			stdout.Write([]byte(" | "))
		}
		stdout.Write([]byte(suffix))
	}
	return 0
}

type statuslinePayload struct {
	TranscriptPath string `json:"transcript_path"`
	RateLimits     struct {
		FiveHour statuslineLimitWindow `json:"five_hour"`
		SevenDay statuslineLimitWindow `json:"seven_day"`
	} `json:"rate_limits"`
}

type statuslineLimitWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *int64   `json:"resets_at"`
}

type statuslineReading struct {
	UsedPct  float64
	ResetsAt time.Time
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
	fiveHour, ok := parseStatuslineReading(payload.RateLimits.FiveHour)
	if !ok {
		return ""
	}
	sevenDay, hasSevenDay := parseStatuslineReading(payload.RateLimits.SevenDay)

	home, err := deps.HomeDir()
	if err != nil {
		return statuslineText(fiveHour, sevenDay, hasSevenDay, 0, false, false)
	}
	state, err := ratelimit.Load(home)
	if err != nil {
		return statuslineText(fiveHour, sevenDay, hasSevenDay, 0, false, false)
	}

	now := deps.Now()
	state.AddReading(ratelimit.Reading{UsedPct: fiveHour.UsedPct, ResetsAt: fiveHour.ResetsAt})
	if hasSevenDay {
		state.AddSevenDayReading(ratelimit.Reading{UsedPct: sevenDay.UsedPct, ResetsAt: sevenDay.ResetsAt})
	}
	ttlSeconds := statuslineTTLSeconds(deps, &state, payload.TranscriptPath)
	_ = ratelimit.Save(home, state) // best-effort; a save failure must not break the statusline

	pctPerMessage, momentumOK := ratelimit.Momentum(state.History)
	projection, hasProjection := ratelimit.Project(fiveHour.UsedPct, pctPerMessage, momentumOK, fiveHour.ResetsAt.Sub(now).Seconds(), ttlSeconds)
	atRisk := hasProjection && projection.AtRisk
	if hasSevenDay {
		weekPctPerMessage, weekMomentumOK := ratelimit.Momentum(state.SevenDayHistory)
		weekProjection, ok := ratelimit.Project(sevenDay.UsedPct, weekPctPerMessage, weekMomentumOK, sevenDay.ResetsAt.Sub(now).Seconds(), ttlSeconds)
		atRisk = atRisk || (ok && weekProjection.AtRisk)
	}
	return statuslineText(fiveHour, sevenDay, hasSevenDay, projection.MessagesLeft, hasProjection, atRisk)
}

func parseStatuslineReading(window statuslineLimitWindow) (statuslineReading, bool) {
	if window.UsedPercentage == nil || window.ResetsAt == nil {
		return statuslineReading{}, false
	}
	return statuslineReading{
		UsedPct:  *window.UsedPercentage,
		ResetsAt: time.Unix(*window.ResetsAt, 0).UTC(),
	}, true
}

func statuslineText(fiveHour, sevenDay statuslineReading, hasSevenDay bool, messagesLeft int, hasMessagesLeft, atRisk bool) string {
	text := fmt.Sprintf("⏱ %.0f%% (5h)", fiveHour.UsedPct)
	if hasSevenDay {
		text += fmt.Sprintf(" / %.0f%% (7d)", sevenDay.UsedPct)
	}
	text += " used"
	if hasMessagesLeft && !atRisk {
		text += fmt.Sprintf(" · ✉ ~%d msgs", messagesLeft)
	}
	if atRisk {
		text += " · ⚠ KeepAlive at risk"
		return colorAtRisk(text)
	}
	return text
}

// statuslineTTLSeconds resolves the current session's cache-tier TTL,
// caching it per transcript path so a per-turn hook invocation doesn't pay
// the whole-file scan cost on every turn.
func statuslineTTLSeconds(deps Dependencies, state *ratelimit.State, transcriptPath string) int {
	if transcriptPath == "" {
		return statuslineFallbackTTL
	}
	if cached, ok := state.TierCache[transcriptPath]; ok {
		return cached
	}
	ttl := statuslineFallbackTTL
	if parsed, err := deps.ParseFile(transcriptPath); err == nil {
		ttl = parsed.CacheWindow.TTLSeconds
		if parsed.CacheWindow.Known {
			state.TierCache[transcriptPath] = ttl
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

	status, err := statusline.Inspect(home)
	if err != nil {
		fmt.Fprintln(stdout, "Could not read ~/.claude/settings.json: "+err.Error())
		return 0
	}

	switch status.State {
	case statusline.StateNotInstalled:
		fmt.Fprintln(stdout, "Claude Code statusLine is not configured.")
		fmt.Fprintln(stdout, "To enable cc-watch:")
		fmt.Fprintln(stdout, "  "+statusline.SettingsSnippet("cc-watch statusline"))
		fmt.Fprintln(stdout, "To undo later: remove the statusLine block.")
		fmt.Fprintln(stdout, "No files were changed.")
		return 0
	case statusline.StateInstalled:
		fmt.Fprintln(stdout, "Claude Code statusLine already includes cc-watch.")
		fmt.Fprintln(stdout, "Current:")
		fmt.Fprintln(stdout, "  "+statusline.SettingsSnippet(status.Command))
		if status.PreviousCommand == "" {
			fmt.Fprintln(stdout, "To undo: remove the statusLine block.")
		} else {
			fmt.Fprintln(stdout, "To undo:")
			fmt.Fprintln(stdout, "  "+statusline.SettingsSnippet(status.PreviousCommand))
		}
		fmt.Fprintln(stdout, "No files were changed.")
		return 0
	case statusline.StateManualReview:
		fmt.Fprintln(stdout, "cc-watch appears in statusLine, but the wrapper shape is unclear.")
		fmt.Fprintln(stdout, "Current:")
		fmt.Fprintln(stdout, "  "+statusline.SettingsSnippet(status.Command))
		fmt.Fprintln(stdout, "No files were changed.")
		return 0
	}

	wrapped := "cc-watch statusline -- " + status.Command
	if statusline.NeedsShell(status.Command) {
		wrapped = "cc-watch statusline -- sh -c " + statusline.ShellQuote(status.Command)
	}
	fmt.Fprintln(stdout, "Claude Code statusLine is set, but cc-watch is not in the chain.")
	fmt.Fprintln(stdout, "Current:")
	fmt.Fprintln(stdout, "  "+statusline.SettingsSnippet(status.Command))
	fmt.Fprintln(stdout, "To enable cc-watch:")
	fmt.Fprintln(stdout, "  "+statusline.SettingsSnippet(wrapped))
	fmt.Fprintln(stdout, "To undo later:")
	fmt.Fprintln(stdout, "  "+statusline.SettingsSnippet(status.Command))
	fmt.Fprintln(stdout, "No files were changed.")
	return 0
}
