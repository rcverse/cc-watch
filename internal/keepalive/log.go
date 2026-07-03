package keepalive

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogDir is the directory holding keepalive.log, set once at TUI startup by the
// app (to ~/.config/cc-watch, matching config.json and ratelimit.json). It's
// empty by default -- the zero value disables logging entirely, which keeps
// tests from writing into a developer's real forensic log.
//
// KeepAlive is the only durable record of what a send actually did: in-memory
// state (LastFailure etc.) is overwritten on the next refresh/instance, the TUI
// owns stdout, and send/confirm happen in async goroutines. Without this trail
// a failure in the wild is unreconstructable. KeepAlive is rare (a few sends
// per cache window), so an always-on append-only log stays tiny.
//
// It writes only to LogDir/keepalive.log and reads only os.Getwd() -- it never
// touches ~/.claude. A failure to open the log is silently a no-op: logging
// must never break a send.
var LogDir string

// maxLogBytes bounds the log: when it exceeds this at startup, the old file is
// rotated to keepalive.log.1 (one generation kept), so total size stays under
// ~2 MiB. KeepAlive is rare, so this holds many thousands of sends.
// ponytail: single-backup rotate, checked once per launch (cc-watch is a
// bounded TUI session, not a daemon); within-session growth is negligible.
const maxLogBytes = 1 << 20

var (
	logOnce sync.Once
	logger  *slog.Logger
)

func kaLogger() *slog.Logger {
	logOnce.Do(func() {
		if LogDir == "" {
			return
		}
		if err := os.MkdirAll(LogDir, 0o755); err != nil {
			return
		}
		path := filepath.Join(LogDir, "keepalive.log")
		if info, err := os.Stat(path); err == nil && info.Size() >= maxLogBytes {
			_ = os.Rename(path, path+".1") // keep one generation; ignore failure
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		logger = slog.New(slog.NewJSONHandler(f, nil))
	})
	return logger
}

// LogSend records one send attempt: the exact cwd claude ran in (the field that
// catches a resume launched from the wrong project directory), exit code,
// duration, classification, and truncated stdout/stderr.
func LogSend(action Action, execution RunnerExecution) {
	l := kaLogger()
	if l == nil {
		return
	}
	dir := action.Dir
	if dir == "" {
		if wd, err := os.Getwd(); err == nil {
			dir = wd + " (inherited)" // no session cwd resolved -- claude ran here
		}
	}
	r := execution.Result
	l.Info("keepalive_send",
		"session", action.SessionID,
		"token", action.InstanceToken,
		"cwd", dir,
		"message", action.Message,
		"exit", r.ExitCode,
		"duration_ms", durationMS(r.StartedAt),
		"classification", sendClassification(execution),
		"stdout", truncate(r.Stdout, 1024),
		"stderr", truncate(r.Stderr, 1024),
	)
}

// LogConfirm records the confirmation outcome: which transcript was watched,
// from what offset/after-time, and whether a new entry appeared or it timed out.
func LogConfirm(sessionID string, token int64, target ConfirmationTarget, res ConfirmationResult, err error) {
	l := kaLogger()
	if l == nil {
		return
	}
	outcome := "confirmed"
	switch {
	case err != nil && !errors.Is(err, ErrConfirmationTimeout):
		outcome = "error"
	case !res.Confirmed:
		outcome = "timeout"
	}
	l.Info("keepalive_confirm",
		"session", sessionID,
		"token", token,
		"path", target.Path,
		"after", target.After,
		"offset", target.Offset,
		"outcome", outcome,
		"confirmed_at", res.ConfirmedAt,
		"error", errString(err),
	)
}

func sendClassification(e RunnerExecution) string {
	if e.ClaudeUnavailable {
		return "claude_unavailable"
	}
	r := e.Result
	if r.Limit {
		return "claude_limit"
	}
	if r.Err != nil || r.ExitCode != 0 {
		return "subprocess_failed"
	}
	return "sent_pending_confirm"
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "...(truncated)"
	}
	return s
}

func durationMS(start time.Time) int64 {
	if start.IsZero() {
		return 0
	}
	return time.Since(start).Milliseconds()
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
