package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const recentMessageLimit = 20

func ParseFile(path string) (Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, err
	}
	defer file.Close()

	var modTime time.Time
	if info, statErr := file.Stat(); statErr == nil {
		modTime = info.ModTime()
	}

	return ParseReader(file, path, modTime)
}

func ParseReader(r io.Reader, path string, fileModifiedAt time.Time) (Session, error) {
	parser := lineParser{
		session: Session{
			SessionID:      stem(path),
			ShortID:        shortID(stem(path)),
			Project:        projectName(path),
			JSONLPath:      path,
			FileModifiedAt: fileModifiedAt,
		},
		totals: map[string]int{},
	}

	reader := bufio.NewReader(r)
	var readErr error
	for {
		raw, err := reader.ReadString('\n')
		if raw != "" {
			parser.line++
			parser.parseLine(strings.TrimSpace(raw))
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		readErr = err
		parser.warn(WarningReadError, err.Error())
		break
	}

	parser.finish()
	return parser.session, readErr
}

type lineParser struct {
	session        Session
	totals         map[string]int
	timestamps     []time.Time
	recentMessages []MessageWindow
	userTexts      []string
	line           int
}

func (p *lineParser) parseLine(raw string) {
	if raw == "" {
		return
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		p.warn(WarningMalformedJSON, err.Error())
		return
	}

	ts, hasTimestamp := p.parseTimestamp(obj)
	p.parseUsage(obj)
	p.parseMessageWindow(obj, ts, hasTimestamp)
	p.parseUserMessage(obj)
	p.parseCwd(obj)
}

// parseCwd records the absolute working directory the session runs in, carried
// on message entries. KeepAlive needs it to run `claude --resume` from the same
// directory -- resume is scoped to the project dir, so a wrong cwd fails with
// "No conversation found". Last non-empty wins, so a mid-session /cd is honored.
func (p *lineParser) parseCwd(obj map[string]any) {
	if cwd, ok := obj["cwd"].(string); ok && cwd != "" {
		p.session.Cwd = cwd
	}
}

func (p *lineParser) parseTimestamp(obj map[string]any) (time.Time, bool) {
	rawTS, ok := obj["timestamp"].(string)
	if !ok || rawTS == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, rawTS)
	if err != nil {
		p.warn(WarningMalformedTimestamp, err.Error())
		return time.Time{}, false
	}
	p.timestamps = append(p.timestamps, ts)
	return ts, true
}

func (p *lineParser) parseUsage(obj map[string]any) {
	usage := mapValue(obj["usage"])
	if len(usage) == 0 {
		if message := mapValue(obj["message"]); message != nil {
			usage = mapValue(message["usage"])
		}
	}
	if usage == nil {
		return
	}

	for key, value := range usage {
		switch typed := value.(type) {
		case float64:
			p.totals[key] += int(typed)
		case map[string]any:
			for nestedKey, nestedValue := range typed {
				if number, ok := nestedValue.(float64); ok {
					p.totals[nestedKey] += int(number)
				}
			}
		}
	}
}

func (p *lineParser) parseUserMessage(obj map[string]any) {
	message := mapValue(obj["message"])
	if message == nil || message["role"] != "user" {
		return
	}

	text := contentText(message["content"])
	if displayUserText(text) {
		p.userTexts = append(p.userTexts, text)
	}
}

func (p *lineParser) parseMessageWindow(obj map[string]any, ts time.Time, hasTimestamp bool) {
	if !hasTimestamp {
		return
	}
	message := mapValue(obj["message"])
	if message == nil {
		return
	}
	role, _ := message["role"].(string)
	if role != "user" {
		return
	}
	text := contentText(message["content"])
	if !displayUserText(text) {
		return
	}
	p.recentMessages = append(p.recentMessages, MessageWindow{
		At:      ts,
		Role:    role,
		Excerpt: text,
	})
	if len(p.recentMessages) > recentMessageLimit {
		p.recentMessages = p.recentMessages[len(p.recentMessages)-recentMessageLimit:]
	}
}

func displayUserText(text string) bool {
	return text != "" && !strings.HasPrefix(text, "<local-command-") && !strings.HasPrefix(text, "<command-name>")
}

func (p *lineParser) finish() {
	sort.Slice(p.timestamps, func(i, j int) bool {
		return p.timestamps[i].Before(p.timestamps[j])
	})
	sort.Slice(p.recentMessages, func(i, j int) bool {
		return p.recentMessages[i].At.Before(p.recentMessages[j].At)
	})
	p.session.RecentMessages = append([]MessageWindow(nil), p.recentMessages...)

	if len(p.timestamps) > 0 {
		started := p.timestamps[0]
		last := p.timestamps[len(p.timestamps)-1]
		duration := int(last.Sub(started).Seconds())
		p.session.StartedAt = &started
		p.session.EndedAt = &last
		p.session.DurationSeconds = &duration
		p.session.LastMessageAt = &last
	}

	p.session.CacheWindow = cacheWindow(p.totals)
	p.session.TokenStats = tokenStats(p.totals)
	p.session.Gaps, p.session.ResetCount = gaps(p.timestamps, p.session.CacheWindow.TTLSeconds)

	if len(p.userTexts) > 0 {
		p.session.Messages.FirstUserExcerpt = p.userTexts[0]
		p.session.Messages.LastUserExcerpt = p.userTexts[len(p.userTexts)-1]
	}
}

func (p *lineParser) warn(code WarningCode, message string) {
	p.session.Warnings = append(p.session.Warnings, ParseWarning{
		Code:    code,
		Line:    p.line,
		Message: message,
	})
}

func cacheWindow(totals map[string]int) CacheWindow {
	if totals["ephemeral_1h_input_tokens"] > 0 {
		return CacheWindow{
			Tier:       Tier1Hour,
			Label:      "1h",
			TTLSeconds: 3600,
			Known:      true,
			Evidence:   []string{"ephemeral_1h_input_tokens"},
		}
	}
	if totals["ephemeral_5m_input_tokens"] > 0 {
		return CacheWindow{
			Tier:       Tier5Minute,
			Label:      "5m",
			TTLSeconds: 300,
			Known:      true,
			Evidence:   []string{"ephemeral_5m_input_tokens"},
		}
	}
	return CacheWindow{
		Tier:       TierUnknown,
		Label:      "TTL ?",
		TTLSeconds: 300,
		Known:      false,
	}
}

func tokenStats(totals map[string]int) TokenStats {
	stats := TokenStats{
		CacheWrites:  totals["cache_creation_input_tokens"],
		CacheReads:   totals["cache_read_input_tokens"],
		OutputTokens: totals["output_tokens"],
	}
	denominator := stats.CacheReads + stats.CacheWrites
	if denominator > 0 {
		stats.HitRate = float64(stats.CacheReads) / float64(denominator) * 100
	}
	return stats
}

func gaps(timestamps []time.Time, ttlSeconds int) ([]Gap, int) {
	var gaps []Gap
	resetCount := 0
	for i := 1; i < len(timestamps); i++ {
		seconds := timestamps[i].Sub(timestamps[i-1]).Seconds()
		if seconds <= 60 {
			continue
		}
		gap := Gap{
			Seconds: seconds,
			From:    timestamps[i-1],
			To:      timestamps[i],
			Reset:   seconds > float64(ttlSeconds),
		}
		if gap.Reset {
			resetCount++
		}
		gaps = append(gaps, gap)
	}
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].Seconds > gaps[j].Seconds
	})
	return gaps, resetCount
}

func contentText(content any) string {
	switch typed := content.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		for _, block := range typed {
			blockMap := mapValue(block)
			if blockMap == nil || blockMap["type"] != "text" {
				continue
			}
			if text, ok := blockMap["text"].(string); ok {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func mapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

func stem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func shortID(sessionID string) string {
	if len(sessionID) <= 8 {
		return sessionID
	}
	return sessionID[:8]
}

func projectName(path string) string {
	parent := filepath.Base(filepath.Dir(path))
	return strings.TrimPrefix(parent, "-")
}
