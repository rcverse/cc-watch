package keepalive

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

var ErrConfirmationTimeout = errors.New("keepalive confirmation timed out")

type ConfirmationResult struct {
	Confirmed   bool
	ConfirmedAt time.Time
}

type ConfirmationTarget struct {
	Path   string
	After  time.Time
	Offset int64
}

type FallbackCommand struct {
	Args    []string
	Display string
}

func NewConfirmationTarget(path string, after time.Time) ConfirmationTarget {
	var offset int64
	if stat, err := os.Stat(path); err == nil {
		offset = stat.Size()
	}
	return ConfirmationTarget{Path: path, After: after, Offset: offset}
}

func (t ConfirmationTarget) Check() (ConfirmationResult, error) {
	return confirmJSONLFromOffset(t.Path, t.After, t.Offset)
}

func ConfirmJSONL(path string, after time.Time) (ConfirmationResult, error) {
	return confirmJSONLFromOffset(path, after, 0)
}

func confirmJSONLFromOffset(path string, after time.Time, offset int64) (ConfirmationResult, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return ConfirmationResult{}, nil
	}
	if err != nil {
		return ConfirmationResult{}, err
	}
	defer file.Close()

	if offset > 0 {
		if _, err := file.Seek(offset, 0); err != nil {
			return ConfirmationResult{}, err
		}
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var entry struct {
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil || entry.Timestamp == "" {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			continue
		}
		if timestamp.After(after) {
			return ConfirmationResult{Confirmed: true, ConfirmedAt: timestamp}, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return ConfirmationResult{}, err
	}
	return ConfirmationResult{}, nil
}

func WaitForConfirmation(ctx context.Context, check func() (ConfirmationResult, error)) (ConfirmationResult, error) {
	for {
		result, err := check()
		if err != nil || result.Confirmed {
			return result, err
		}
		select {
		case <-ctx.Done():
			return ConfirmationResult{}, ErrConfirmationTimeout
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func ManualFallbackCommand(sessionID, message string) FallbackCommand {
	args := []string{"claude", "-r", sessionID, "-p", message}
	return FallbackCommand{
		Args:    append([]string(nil), args...),
		Display: shellJoin(args),
	}
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if isShellSafe(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

func isShellSafe(arg string) bool {
	for _, r := range arg {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '_', '-', '.', '/', ':':
			continue
		default:
			return false
		}
	}
	return true
}
