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

	"github.com/rcverse/cc-watch/internal/config"
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
	display := effectiveStatuslineConfig(deps, cmd)
	values := statuslineValues(deps, raw, display)

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
	stdout.Write([]byte(statusline.Render(string(trimmed), display, values)))
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

// statuslineValues parses the hook payload and produces independent values for
// the configured usage, warning, and cache elements. Cache timing is allowed
// to render even when a payload has no rate-limit readings.
func statuslineValues(deps Dependencies, raw []byte, display config.StatuslineConfig) map[string]string {
	values := map[string]string{}
	var payload statuslinePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return values
	}
	fiveHour, ok := parseStatuslineReading(payload.RateLimits.FiveHour)
	sevenDay, hasSevenDay := parseStatuslineReading(payload.RateLimits.SevenDay)
	if ok {
		values[config.StatuslineElementUsage] = statuslineUsageText(fiveHour, sevenDay, hasSevenDay, 0, false, display.Usage.Format)
	}

	home, err := deps.HomeDir()
	if err != nil {
		return values
	}
	state, err := ratelimit.Load(home)
	if err != nil {
		return values
	}

	now := deps.Now()
	if ok {
		state.AddReading(ratelimit.Reading{UsedPct: fiveHour.UsedPct, ResetsAt: fiveHour.ResetsAt})
	}
	if hasSevenDay {
		state.AddSevenDayReading(ratelimit.Reading{UsedPct: sevenDay.UsedPct, ResetsAt: sevenDay.ResetsAt})
	}
	snapshot := statuslineCacheSnapshot(deps, &state, payload.TranscriptPath)
	_ = ratelimit.Save(home, state) // best-effort; a save failure must not break the statusline

	if ok {
		pctPerMessage, momentumOK := ratelimit.Momentum(state.History)
		projection, hasProjection := ratelimit.Project(fiveHour.UsedPct, pctPerMessage, momentumOK, fiveHour.ResetsAt.Sub(now).Seconds(), snapshot.TTLSeconds)
		messagesLeft := 0
		if hasProjection {
			messagesLeft = projection.MessagesLeft
		}
		atRisk := hasProjection && projection.AtRisk
		if hasSevenDay {
			weekPctPerMessage, weekMomentumOK := ratelimit.Momentum(state.SevenDayHistory)
			weekProjection, weekOK := ratelimit.Project(sevenDay.UsedPct, weekPctPerMessage, weekMomentumOK, sevenDay.ResetsAt.Sub(now).Seconds(), snapshot.TTLSeconds)
			atRisk = atRisk || (weekOK && weekProjection.AtRisk)
		}
		if display.Warning.Enabled && (display.Warning.Format == config.StatuslineWarningFormatVerbose || atRisk) {
			values[config.StatuslineElementWarning] = statuslineWarningText(atRisk)
		}
		values[config.StatuslineElementUsage] = statuslineUsageText(fiveHour, sevenDay, hasSevenDay, messagesLeft, hasProjection && !atRisk, display.Usage.Format)
	}
	if display.Cache.Enabled {
		values[config.StatuslineElementCache] = statuslineCacheText(snapshot, now, display.Cache.Format)
	}
	return values
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

func statuslineUsageText(fiveHour, sevenDay statuslineReading, hasSevenDay bool, messagesLeft int, hasMessagesLeft bool, format string) string {
	if format == config.StatuslineFormatCompact {
		text := fmt.Sprintf("%.0f%%", fiveHour.UsedPct)
		if hasSevenDay {
			text += fmt.Sprintf("/%.0f%%", sevenDay.UsedPct)
		}
		return text
	}
	text := fmt.Sprintf("⏱ %.0f%% (5h)", fiveHour.UsedPct)
	if hasSevenDay {
		text += fmt.Sprintf(" / %.0f%% (7d)", sevenDay.UsedPct)
	}
	text += " used"
	if hasMessagesLeft {
		text += fmt.Sprintf(" · ✉ ~%d msgs", messagesLeft)
	}
	return text
}

func statuslineWarningText(atRisk bool) string {
	if atRisk {
		return colorAtRisk("⚠ KeepAlive at risk")
	}
	return "✓ KA OK"
}

func statuslineCacheText(snapshot ratelimit.CacheSnapshot, now time.Time, format string) string {
	if !snapshot.Known || snapshot.CacheAnchorAt == nil || snapshot.TTLSeconds <= 0 {
		return ""
	}
	expiresAt := snapshot.CacheAnchorAt.Add(time.Duration(snapshot.TTLSeconds) * time.Second)
	if now.Before(expiresAt) {
		remaining := ceilSeconds(expiresAt.Sub(now))
		if format == config.StatuslineFormatCompact {
			return formatStatuslineDuration(remaining)
		}
		return fmt.Sprintf("⌛ %s left · %s cache", formatStatuslineDuration(remaining), formatCacheTTL(snapshot.TTLSeconds))
	}
	expired := ceilSeconds(now.Sub(expiresAt))
	if format == config.StatuslineFormatCompact {
		return "expired " + formatStatuslineDuration(expired)
	}
	return fmt.Sprintf("⌛ expired %s · %s cache", formatStatuslineDuration(expired), formatCacheTTL(snapshot.TTLSeconds))
}

func ceilSeconds(duration time.Duration) int {
	seconds := int(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}

func formatStatuslineDuration(seconds int) string {
	if seconds >= 3600 {
		return fmt.Sprintf("%dh%02dm%02ds", seconds/3600, (seconds%3600)/60, seconds%60)
	}
	if seconds >= 60 {
		return fmt.Sprintf("%dm%02ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%ds", seconds)
}

func formatCacheTTL(seconds int) string {
	if seconds >= 3600 && seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds >= 60 && seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return formatStatuslineDuration(seconds)
}

func effectiveStatuslineConfig(deps Dependencies, cmd Command) config.StatuslineConfig {
	result := config.Default().Statusline
	if deps.HomeDir != nil {
		if home, err := deps.HomeDir(); err == nil {
			if loaded, err := config.Load(home); err == nil {
				result = loaded.Statusline
			}
		}
	}
	if cmd.StatuslineLayout != "" {
		result.Usage.Layout = cmd.StatuslineLayout
	}
	if cmd.StatuslineFormat != "" {
		result.Usage.Format = cmd.StatuslineFormat
	}
	return config.NormalizeStatusline(result)
}

// statuslineCacheSnapshot resolves the current session's cache timing. The
// parsed snapshot is reused while the transcript's mtime is unchanged, so a
// one-second Claude refresh interval does not rescan JSONL every second.
func statuslineCacheSnapshot(deps Dependencies, state *ratelimit.State, transcriptPath string) ratelimit.CacheSnapshot {
	if transcriptPath == "" {
		return ratelimit.CacheSnapshot{TTLSeconds: statuslineFallbackTTL}
	}
	if state.CacheSnapshots == nil {
		state.CacheSnapshots = map[string]ratelimit.CacheSnapshot{}
	}
	if cached, ok := state.CacheSnapshots[transcriptPath]; ok {
		if cached.Known && cached.CacheAnchorAt == nil {
			delete(state.CacheSnapshots, transcriptPath)
		} else if info, err := os.Stat(transcriptPath); err != nil || info.ModTime().Equal(cached.FileModifiedAt) {
			return cached
		}
	}
	if parsed, err := deps.ParseFile(transcriptPath); err == nil {
		snapshot := ratelimit.CacheSnapshot{
			TTLSeconds:     parsed.CacheWindow.TTLSeconds,
			Known:          parsed.CacheWindow.Known && parsed.CacheAnchorAt != nil,
			CacheAnchorAt:  parsed.CacheAnchorAt,
			FileModifiedAt: parsed.FileModifiedAt,
		}
		state.CacheSnapshots[transcriptPath] = snapshot
		return snapshot
	}
	return ratelimit.CacheSnapshot{TTLSeconds: statuslineFallbackTTL}
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
	ccWatch := statusline.RuntimeCommand(ccWatchCommand())

	switch status.State {
	case statusline.StateNotInstalled:
		fmt.Fprintln(stdout, "Claude Code statusLine is not configured.")
		fmt.Fprintln(stdout, "To enable cc-watch:")
		fmt.Fprintln(stdout, "  "+statusline.SettingsSnippet(ccWatch))
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

	wrapped := ccWatch + " -- " + status.Command
	if statusline.NeedsShell(status.Command) {
		wrapped = ccWatch + " -- sh -c " + statusline.ShellQuote(status.Command)
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
