package keepalive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/richardchen/cc-watch/internal/config"
	"github.com/richardchen/cc-watch/internal/session"
)

func TestTimingDefaultsUseTTLSpecificTriggerWindows(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	oneHour := EvaluateTiming(sessionWithTTL(now, session.Tier1Hour, 3600, true), now, cfg)
	if oneHour.EffectiveTriggerSeconds != 300 {
		t.Fatalf("1h trigger = %d, want 300", oneHour.EffectiveTriggerSeconds)
	}
	if oneHour.EffectiveCountdownSeconds != 30 {
		t.Fatalf("1h countdown = %d, want 30", oneHour.EffectiveCountdownSeconds)
	}
	if !oneHour.AutoSendAllowed {
		t.Fatalf("1h AutoSendAllowed = false, want true: %#v", oneHour)
	}

	fiveMinute := EvaluateTiming(sessionWithTTL(now, session.Tier5Minute, 300, true), now, cfg)
	if fiveMinute.EffectiveTriggerSeconds != 60 {
		t.Fatalf("5m trigger = %d, want 60", fiveMinute.EffectiveTriggerSeconds)
	}
	if fiveMinute.EffectiveCountdownSeconds != 30 {
		t.Fatalf("5m countdown = %d, want 30", fiveMinute.EffectiveCountdownSeconds)
	}
	if !fiveMinute.AutoSendAllowed {
		t.Fatalf("5m AutoSendAllowed = false, want true: %#v", fiveMinute)
	}
}

func TestTimingUnknownTTLUsesConservativeFiveMinuteHeuristic(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	result := EvaluateTiming(sessionWithTTL(now, session.TierUnknown, 0, false), now, cfg)

	if result.EffectiveTTLSeconds != 300 {
		t.Fatalf("unknown effective TTL = %d, want 300", result.EffectiveTTLSeconds)
	}
	if result.EffectiveTriggerSeconds != 60 {
		t.Fatalf("unknown trigger = %d, want conservative 60", result.EffectiveTriggerSeconds)
	}
	if !result.UsesConservativeTTL {
		t.Fatalf("UsesConservativeTTL = false, want true")
	}
}

func TestTimingClampsCountdownToPreserveSafetyMargin(t *testing.T) {
	cfg := config.Default().KeepAlive
	cfg.CountdownSeconds = 90
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	result := EvaluateTiming(sessionWithTTL(now, session.Tier5Minute, 300, true), now, cfg)

	if result.EffectiveCountdownSeconds != 30 {
		t.Fatalf("countdown = %d, want clamped 30", result.EffectiveCountdownSeconds)
	}
	if result.SafetyClamped == false {
		t.Fatalf("SafetyClamped = false, want true")
	}
	if !result.AutoSendAllowed {
		t.Fatalf("AutoSendAllowed = false, want true after clamp: %#v", result)
	}
}

func TestTimingDisablesAutoSendWhenSafetyMarginCannotBePreserved(t *testing.T) {
	cfg := config.Default().KeepAlive
	cfg.TriggerBeforeExpiryMinutes = 1
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	last := now.Add(-4*time.Minute - 40*time.Second)
	s := session.Session{
		SessionID:     "unsafe",
		LastMessageAt: &last,
		CacheWindow: session.CacheWindow{
			Tier:       session.Tier5Minute,
			TTLSeconds: 300,
			Known:      true,
		},
	}

	result := EvaluateTiming(s, now, cfg)

	if result.EffectiveTriggerSeconds != 20 {
		t.Fatalf("trigger = %d, want current remaining 20", result.EffectiveTriggerSeconds)
	}
	if result.AutoSendAllowed {
		t.Fatalf("AutoSendAllowed = true, want false: %#v", result)
	}
	if result.EffectiveCountdownSeconds != 0 {
		t.Fatalf("countdown = %d, want 0 when auto-send disabled", result.EffectiveCountdownSeconds)
	}
}

func TestStateMachineTransitionsThroughRequiredStates(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	s := activeSession("stateful", now, time.Hour, 10*time.Minute)
	manager := NewManager(cfg)

	if got := manager.State("stateful").State; got != StateOff {
		t.Fatalf("initial state = %q, want %q", got, StateOff)
	}
	if actions := manager.Enable(s, now); len(actions) != 0 {
		t.Fatalf("enable outside trigger actions = %#v, want none", actions)
	}
	if got := manager.State("stateful").State; got != StateMonitoringIdle {
		t.Fatalf("state = %q, want %q", got, StateMonitoringIdle)
	}

	actions := manager.Check(now.Add(5*time.Minute), []session.Session{activeSession("stateful", now.Add(5*time.Minute), time.Hour, 5*time.Minute)})
	if len(actions) != 1 || actions[0].Kind != ActionCountdownStarted {
		t.Fatalf("threshold actions = %#v, want countdown start", actions)
	}
	countdownToken := actions[0].InstanceToken
	if got := manager.State("stateful").State; got != StateCountdown {
		t.Fatalf("state = %q, want %q", got, StateCountdown)
	}

	actions = manager.CountdownElapsed("stateful", countdownToken, now.Add(5*time.Minute+30*time.Second))
	if len(actions) != 1 || actions[0].Kind != ActionStartRunner {
		t.Fatalf("countdown actions = %#v, want runner start", actions)
	}
	sendToken := actions[0].InstanceToken
	st := manager.State("stateful")
	if st.State != StateSending || st.ScopeUsed != 1 {
		t.Fatalf("state after send start = %#v, want sending with one scope used", st)
	}

	manager.MarkSendStarted("stateful", sendToken, now.Add(5*time.Minute+31*time.Second))
	if got := manager.State("stateful").State; got != StateConfirming {
		t.Fatalf("state = %q, want %q", got, StateConfirming)
	}
	manager.MarkSuccess("stateful", sendToken, now.Add(5*time.Minute+40*time.Second))
	if got := manager.State("stateful").State; got != StateSuccess {
		t.Fatalf("state = %q, want %q", got, StateSuccess)
	}
	manager.Acknowledge("stateful")
	if got := manager.State("stateful").State; got != StateScopeComplete {
		t.Fatalf("state = %q, want %q", got, StateScopeComplete)
	}
	manager.Disable("stateful")
	if got := manager.State("stateful").State; got != StateOff {
		t.Fatalf("state = %q, want %q", got, StateOff)
	}
}

func TestStateMachineImmediateManualAndFailureStates(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	inside := activeSession("inside", now, time.Hour, 5*time.Minute)
	manager := NewManager(cfg)

	actions := manager.Enable(inside, now)
	if len(actions) != 1 || actions[0].Kind != ActionCountdownStarted {
		t.Fatalf("enable inside trigger actions = %#v, want countdown", actions)
	}

	cfg.AutoSend = false
	manual := NewManager(cfg)
	actions = manual.Enable(activeSession("manual", now, time.Hour, 5*time.Minute), now)
	if len(actions) != 1 || actions[0].Kind != ActionManualPromptShown {
		t.Fatalf("manual enable actions = %#v, want manual prompt", actions)
	}
	if got := manual.State("manual").State; got != StateManualReady {
		t.Fatalf("manual state = %q, want %q", got, StateManualReady)
	}
	manual.Dismiss("manual", actions[0].InstanceToken)
	if got := manual.State("manual").State; got != StateCancelledInstance {
		t.Fatalf("dismiss state = %q, want %q", got, StateCancelledInstance)
	}

	for _, tc := range []struct {
		name string
		fail func(*Manager, string, int64)
		want State
	}{
		{name: "no claude", fail: func(m *Manager, id string, token int64) { m.MarkNoClaude(id, token, "claude command not found") }, want: StateErrorNoClaude},
		{name: "subprocess", fail: func(m *Manager, id string, token int64) { m.MarkSubprocessFailure(id, token, "exit status 1", false) }, want: StateErrorSubprocess},
		{name: "timeout", fail: func(m *Manager, id string, token int64) { m.MarkConfirmationTimeout(id, token) }, want: StateErrorTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewManager(config.Default().KeepAlive)
			start := m.Enable(activeSession(tc.name, now, time.Hour, 5*time.Minute), now)[0]
			send := m.SendNow(tc.name, start.InstanceToken, now)
			if len(send) != 1 {
				t.Fatalf("SendNow actions = %#v, want runner start", send)
			}
			if tc.want == StateErrorTimeout {
				m.MarkSendStarted(tc.name, send[0].InstanceToken, now)
			}
			tc.fail(m, tc.name, send[0].InstanceToken)
			if got := m.State(tc.name).State; got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
			if m.State(tc.name).AutoSend {
				t.Fatalf("AutoSend stayed enabled after hard failure: %#v", m.State(tc.name))
			}
		})
	}
}

func TestStateMachineIsEdgeTriggeredScopedAndPerSession(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	manager := NewManager(cfg)

	manager.Enable(activeSession("a", now, time.Hour, 10*time.Minute), now)
	manager.Enable(activeSession("b", now, time.Hour, 10*time.Minute), now)
	if actions := manager.Check(now.Add(time.Minute), []session.Session{
		activeSession("a", now.Add(time.Minute), time.Hour, 9*time.Minute),
		activeSession("b", now.Add(time.Minute), time.Hour, 9*time.Minute),
	}); len(actions) != 0 {
		t.Fatalf("outside trigger actions = %#v, want none", actions)
	}

	actions := manager.Check(now.Add(5*time.Minute), []session.Session{
		activeSession("a", now.Add(5*time.Minute), time.Hour, 5*time.Minute),
		activeSession("b", now.Add(5*time.Minute), time.Hour, 5*time.Minute),
	})
	if len(actions) != 2 {
		t.Fatalf("threshold actions = %#v, want two countdowns", actions)
	}
	if repeat := manager.Check(now.Add(5*time.Minute+time.Second), []session.Session{
		activeSession("a", now.Add(5*time.Minute+time.Second), time.Hour, 5*time.Minute-time.Second),
		activeSession("b", now.Add(5*time.Minute+time.Second), time.Hour, 5*time.Minute-time.Second),
	}); len(repeat) != 0 {
		t.Fatalf("repeated tick actions = %#v, want edge-triggered silence", repeat)
	}

	sendA := manager.SendNow("a", actions[0].InstanceToken, now)
	if len(sendA) != 1 {
		t.Fatalf("SendNow(a) actions = %#v, want runner start", sendA)
	}
	if repeatedSend := manager.SendNow("a", actions[0].InstanceToken, now); len(repeatedSend) != 0 {
		t.Fatalf("single-flight repeated send actions = %#v, want none", repeatedSend)
	}
	if manager.State("a").ScopeUsed != 1 || manager.State("b").ScopeUsed != 0 {
		t.Fatalf("scope not isolated: a=%#v b=%#v", manager.State("a"), manager.State("b"))
	}

	manager.MarkSubprocessFailure("a", sendA[0].InstanceToken, "limit exceeded", true)
	if got := manager.State("a").State; got != StateErrorSubprocess {
		t.Fatalf("a state = %q, want subprocess error", got)
	}
	if !manager.State("a").RateLimited {
		t.Fatalf("a RateLimited = false, want true")
	}
	if got := manager.State("b").State; got != StateCountdown {
		t.Fatalf("b state = %q, want unchanged countdown", got)
	}

	manager.Refresh(activeSession("b", now.Add(10*time.Minute), time.Hour, 50*time.Minute), now.Add(10*time.Minute))
	if got := manager.State("b").State; got != StateMonitoringIdle {
		t.Fatalf("refresh above trigger state = %q, want monitoring idle", got)
	}
	rearmed := manager.Check(now.Add(55*time.Minute), []session.Session{
		activeSession("b", now.Add(55*time.Minute), time.Hour, 5*time.Minute),
	})
	if len(rearmed) != 1 || rearmed[0].Kind != ActionCountdownStarted {
		t.Fatalf("rearmed actions = %#v, want one countdown after refresh/new window", rearmed)
	}
}

func TestStateMachineIgnoresStaleTokenEventsAfterCancellationAndRefresh(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	manager := NewManager(cfg)

	start := manager.Enable(activeSession("stale", now, time.Hour, 5*time.Minute), now)[0]
	manager.Cancel("stale", start.InstanceToken)
	if actions := manager.CountdownElapsed("stale", start.InstanceToken, now.Add(30*time.Second)); len(actions) != 0 {
		t.Fatalf("stale countdown actions after cancel = %#v, want none", actions)
	}
	if got := manager.State("stale").State; got != StateCancelledInstance {
		t.Fatalf("state after stale countdown = %q, want cancelled", got)
	}

	manager.Refresh(activeSession("stale", now.Add(time.Minute), time.Hour, 50*time.Minute), now.Add(time.Minute))
	oldToken := manager.State("stale").InstanceToken
	manager.Refresh(activeSession("stale", now.Add(2*time.Minute), time.Hour, 5*time.Minute), now.Add(2*time.Minute))
	if actions := manager.CountdownElapsed("stale", oldToken, now.Add(2*time.Minute+30*time.Second)); len(actions) != 0 {
		t.Fatalf("stale countdown actions after refresh edge reset = %#v, want none", actions)
	}

	next := manager.State("stale").InstanceToken
	send := manager.SendNow("stale", next, now)
	manager.MarkSendStarted("stale", send[0].InstanceToken, now)
	manager.Cancel("stale", send[0].InstanceToken)
	manager.MarkSuccess("stale", send[0].InstanceToken, now.Add(time.Second))
	if got := manager.State("stale").State; got != StateCancelledInstance {
		t.Fatalf("stale confirmation changed state to %q, want cancelled", got)
	}
}

func TestStateMachineDoesNotRearmWithinSameTriggerWindowWhenScopeRemains(t *testing.T) {
	cfg := config.Default().KeepAlive
	cfg.Scope.MaxSends = 2
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	manager := NewManager(cfg)

	start := manager.Enable(activeSession("rearm", now, time.Hour, 5*time.Minute), now)[0]
	send := manager.SendNow("rearm", start.InstanceToken, now)[0]
	manager.MarkSendStarted("rearm", send.InstanceToken, now)
	manager.MarkSuccess("rearm", send.InstanceToken, now.Add(time.Second))
	manager.Acknowledge("rearm")
	if got := manager.State("rearm").State; got != StateMonitoringIdle {
		t.Fatalf("state = %q, want monitoring idle while waiting for edge reset", got)
	}

	actions := manager.Check(now.Add(2*time.Second), []session.Session{
		activeSession("rearm", now.Add(2*time.Second), time.Hour, 5*time.Minute-2*time.Second),
	})
	if len(actions) != 0 {
		t.Fatalf("same-window check actions = %#v, want no re-arm", actions)
	}
	actions = manager.Refresh(activeSession("rearm", now.Add(3*time.Second), time.Hour, 5*time.Minute-3*time.Second), now.Add(3*time.Second))
	if len(actions) != 0 {
		t.Fatalf("same-window refresh actions = %#v, want no re-arm", actions)
	}

	manager.Refresh(activeSession("rearm", now.Add(time.Minute), time.Hour, 50*time.Minute), now.Add(time.Minute))
	actions = manager.Check(now.Add(46*time.Minute), []session.Session{
		activeSession("rearm", now.Add(46*time.Minute), time.Hour, 5*time.Minute),
	})
	if len(actions) != 1 || actions[0].Kind != ActionCountdownStarted {
		t.Fatalf("new-window actions = %#v, want re-armed countdown", actions)
	}
}

func TestCountdownElapsedDoesNotSendWhenAutoSendTurnsOffOrSafetyDeadlineIsMissed(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	autoOff := NewManager(cfg)
	start := autoOff.Enable(activeSession("auto-off", now, time.Hour, 5*time.Minute), now)[0]
	autoOff.SetAutoSend("auto-off", false)
	if actions := autoOff.CountdownElapsed("auto-off", start.InstanceToken, now.Add(30*time.Second)); len(actions) != 1 || actions[0].Kind != ActionManualPromptShown {
		t.Fatalf("auto-off countdown actions = %#v, want manual prompt and no runner", actions)
	}
	state := autoOff.State("auto-off")
	if state.State != StateManualReady || state.ScopeUsed != 0 {
		t.Fatalf("auto-off state = %#v, want manual_ready without scope use", state)
	}

	// A 5m cache with 60s remaining expires at now+60; the hard send deadline
	// sits 10s before that (now+50). A countdown that only elapses past the
	// deadline -- e.g. after the machine slept -- bails to manual.
	delayed := NewManager(cfg)
	start = delayed.Enable(activeSession("delayed", now, 5*time.Minute, time.Minute), now)[0]
	if actions := delayed.CountdownElapsed("delayed", start.InstanceToken, now.Add(51*time.Second)); len(actions) != 1 || actions[0].Kind != ActionManualPromptShown {
		t.Fatalf("delayed countdown actions = %#v, want manual prompt and no runner", actions)
	}
	state = delayed.State("delayed")
	if state.State != StateManualReady || state.ScopeUsed != 0 || !state.SafetyDisabled {
		t.Fatalf("delayed state = %#v, want safety-disabled manual_ready without scope use", state)
	}
}

func TestAutoSendFiresForShortCacheDespiteCountdownDrift(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	m := NewManager(cfg)

	// A 5-minute cache entering its trigger window with 60s remaining: the
	// countdown is 30s and the naive safety deadline sits exactly 30s out, so
	// the tick-counted countdown (which always takes >=30 real seconds) elapses
	// a beat late and must still auto-send, not silently bail to manual.
	actions := m.Enable(activeSession("short", now, 5*time.Minute, 60*time.Second), now)
	if len(actions) != 1 || actions[0].Kind != ActionCountdownStarted {
		t.Fatalf("Enable actions = %#v, want a countdown", actions)
	}
	token := actions[0].InstanceToken

	elapse := now.Add(time.Duration(cfg.CountdownSeconds+2) * time.Second)
	got := m.CountdownElapsed("short", token, elapse)
	if len(got) != 1 || got[0].Kind != ActionStartRunner {
		t.Fatalf("countdown elapsed actions = %#v, want auto-send (start_runner)", got)
	}
}

func TestFailureAcknowledgeMovesExhaustedScopeToScopeComplete(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	manager := NewManager(cfg)

	start := manager.Enable(activeSession("fail-done", now, time.Hour, 5*time.Minute), now)[0]
	send := manager.SendNow("fail-done", start.InstanceToken, now)[0]
	manager.MarkSubprocessFailure("fail-done", send.InstanceToken, "exit status 1", false)
	if got := manager.State("fail-done").State; got != StateErrorSubprocess {
		t.Fatalf("state = %q, want subprocess error before acknowledge", got)
	}
	manager.Acknowledge("fail-done")
	if got := manager.State("fail-done").State; got != StateScopeComplete {
		t.Fatalf("state = %q, want scope complete after exhausted failure acknowledge", got)
	}
}

func TestReEnableAfterScopeExhaustedEmitsScopeCompleteAction(t *testing.T) {
	cfg := config.Default().KeepAlive
	cfg.Scope.MaxSends = 1
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	manager := NewManager(cfg)

	s := activeSession("re-enable", now, time.Hour, 5*time.Minute)
	start := manager.Enable(s, now)[0]
	send := manager.SendNow("re-enable", start.InstanceToken, now)[0]
	manager.MarkSendStarted("re-enable", send.InstanceToken, now)
	manager.MarkSuccess("re-enable", send.InstanceToken, now)
	manager.Acknowledge("re-enable")
	if got := manager.State("re-enable").State; got != StateScopeComplete {
		t.Fatalf("state = %q, want scope complete before disable", got)
	}

	manager.Disable("re-enable")
	actions := manager.Enable(s, now)
	if len(actions) != 1 || actions[0].Kind != ActionScopeComplete {
		t.Fatalf("re-enable after exhausted scope actions = %#v, want single ActionScopeComplete", actions)
	}
	if got := manager.State("re-enable").State; got != StateScopeComplete {
		t.Fatalf("state = %q, want scope complete after re-enable", got)
	}
}

func TestRunnerAvailabilityFailuresAreVisibleAndBounded(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	armed := NewManager(cfg)
	armed.Enable(activeSession("armed", now, time.Hour, 10*time.Minute), now)
	err := armed.CheckAvailability("armed", fakeClaudeRunner{availableErr: errors.New("claude command not found")})
	if err == nil {
		t.Fatal("CheckAvailability returned nil, want unavailable error")
	}
	state := armed.State("armed")
	if state.State != StateErrorNoClaude || state.ScopeUsed != 0 || state.AutoSend {
		t.Fatalf("armed unavailable state = %#v, want no-claude without consuming scope and auto-send stopped", state)
	}

	beforeSend := NewManager(cfg)
	start := beforeSend.Enable(activeSession("before-send", now, time.Hour, 5*time.Minute), now)[0]
	send := beforeSend.CountdownElapsed("before-send", start.InstanceToken, now.Add(30*time.Second))[0]
	result := beforeSend.Run(context.Background(), send, fakeClaudeRunner{availableErr: errors.New("claude command not found")}, now)
	if result.Err == nil {
		t.Fatal("Run returned nil error, want unavailable error")
	}
	state = beforeSend.State("before-send")
	if state.State != StateErrorNoClaude || state.ScopeUsed != 1 || state.AutoSend {
		t.Fatalf("send-time unavailable state = %#v, want no-claude with one attempted scope and auto-send stopped", state)
	}
}

func TestRunnerSubprocessFailuresAndLimitErrorsStopAutoSend(t *testing.T) {
	cfg := config.Default().KeepAlive
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name   string
		result RunResult
		want   string
	}{
		{name: "non-zero", result: RunResult{ExitCode: 1, Stderr: "boom", Err: errors.New("exit status 1")}, want: "boom"},
		{name: "limit", result: RunResult{ExitCode: 1, Stderr: "Claude usage limit reached", Limit: true, Err: ErrClaudeLimit}, want: "Claude usage limit reached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := NewManager(cfg)
			start := manager.Enable(activeSession(tc.name, now, time.Hour, 5*time.Minute), now)[0]
			send := manager.SendNow(tc.name, start.InstanceToken, now)[0]

			result := manager.Run(context.Background(), send, fakeClaudeRunner{sendResult: tc.result}, now)
			if result.Err == nil {
				t.Fatal("Run returned nil error, want subprocess failure")
			}
			state := manager.State(tc.name)
			if state.State != StateErrorSubprocess || state.AutoSend {
				t.Fatalf("state = %#v, want subprocess failure with auto-send stopped", state)
			}
			if !strings.Contains(state.LastFailure, tc.want) {
				t.Fatalf("LastFailure = %q, want %q", state.LastFailure, tc.want)
			}
		})
	}
}

func TestConfirmationRequiresTargetSessionLineAfterSendAttempt(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.jsonl")
	other := filepath.Join(dir, "other.jsonl")
	sendAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	writeJSONL(t, other, `{"timestamp":"2026-06-05T12:00:05Z","message":{"role":"assistant"}}`+"\n")
	result, err := ConfirmJSONL(target, sendAt)
	if err != nil {
		t.Fatalf("ConfirmJSONL missing target returned error: %v", err)
	}
	if result.Confirmed {
		t.Fatalf("confirmed from non-target file: %#v", result)
	}

	writeJSONL(t, target, `{"timestamp":"2026-06-05T11:59:59Z","message":{"role":"assistant"}}`+"\n")
	result, err = ConfirmJSONL(target, sendAt)
	if err != nil {
		t.Fatalf("ConfirmJSONL before-send returned error: %v", err)
	}
	if result.Confirmed {
		t.Fatalf("confirmed from line before send attempt: %#v", result)
	}

	appendJSONL(t, target, `{"timestamp":"2026-06-05T12:00:01Z","message":{"role":"assistant"}}`+"\n")
	result, err = ConfirmJSONL(target, sendAt)
	if err != nil {
		t.Fatalf("ConfirmJSONL returned error: %v", err)
	}
	if !result.Confirmed || !result.ConfirmedAt.Equal(sendAt.Add(time.Second)) {
		t.Fatalf("confirmation = %#v, want target line after send", result)
	}
}

func TestConfirmationIgnoresTargetLinesThatExistedBeforeSendAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "target.jsonl")
	sendAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	writeJSONL(t, path, `{"timestamp":"2026-06-05T12:00:05Z","message":{"role":"assistant"}}`+"\n")

	target := NewConfirmationTarget(path, sendAt)
	result, err := target.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if result.Confirmed {
		t.Fatalf("confirmed from line that existed before send attempt: %#v", result)
	}

	appendJSONL(t, path, `{"timestamp":"2026-06-05T12:00:06Z","message":{"role":"assistant"}}`+"\n")
	result, err = target.Check()
	if err != nil {
		t.Fatalf("Check after append returned error: %v", err)
	}
	if !result.Confirmed || !result.ConfirmedAt.Equal(sendAt.Add(6*time.Second)) {
		t.Fatalf("confirmation = %#v, want appended target line after send", result)
	}
}

func TestConfirmationWaitTimeoutPath(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := WaitForConfirmation(ctx, func() (ConfirmationResult, error) {
		return ConfirmationResult{}, nil
	})
	if !errors.Is(err, ErrConfirmationTimeout) {
		t.Fatalf("WaitForConfirmation error = %v, want %v", err, ErrConfirmationTimeout)
	}
}

func TestManualFallbackCommandUsesSafeDisplayQuoting(t *testing.T) {
	fallback := ManualFallbackCommand("abc123", `don't "; rm -rf / #`, "/tmp/my dir")

	wantArgs := []string{"claude", "-r", "abc123", "-p", `don't "; rm -rf / #`}
	if strings.Join(fallback.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("Args = %#v, want %#v", fallback.Args, wantArgs)
	}
	wantDisplay := `cd '/tmp/my dir' && claude -r abc123 -p 'don'\''t "; rm -rf / #'`
	if fallback.Display != wantDisplay {
		t.Fatalf("Display = %q, want %q", fallback.Display, wantDisplay)
	}
}

func TestSubprocessRunnerLimitOnlyOnFailedExit(t *testing.T) {
	run := func(stdout, stderr string, exitCode int) RunResult {
		r := SubprocessRunner{
			LookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
			Command: func(_ context.Context, _, _ string, _ ...string) (string, string, int, error) {
				var err error
				if exitCode != 0 {
					err = errors.New("exit")
				}
				return stdout, stderr, exitCode, err
			},
		}
		return r.Send(context.Background(), RunRequest{SessionID: "s1", Message: "hi"})
	}

	// Success whose reply mentions "usage" must not be read as a limit.
	if got := run("Sure, your token usage looks fine.", "", 0); got.Limit || got.Err != nil {
		t.Fatalf("success with 'usage' in reply: Limit=%v Err=%v, want false/nil", got.Limit, got.Err)
	}
	// A real limit arrives on a failed exit.
	if got := run("", "Claude usage limit reached", 1); !got.Limit || !errors.Is(got.Err, ErrClaudeLimit) {
		t.Fatalf("failed with limit stderr: Limit=%v Err=%v, want true/ErrClaudeLimit", got.Limit, got.Err)
	}
}

func TestSubprocessRunnerRunsInSessionDir(t *testing.T) {
	var gotDir, gotName string
	var gotArgs []string
	r := SubprocessRunner{
		LookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
		Command: func(_ context.Context, dir, name string, args ...string) (string, string, int, error) {
			gotDir, gotName, gotArgs = dir, name, args
			return "", "", 0, nil
		},
	}
	r.Send(context.Background(), RunRequest{SessionID: "s1", Message: "hi", Dir: "/proj/dir"})
	if gotDir != "/proj/dir" {
		t.Fatalf("dir = %q, want /proj/dir", gotDir)
	}
	wantArgv := []string{"claude", "-r", "s1", "-p", "hi"}
	if strings.Join(append([]string{gotName}, gotArgs...), "\x00") != strings.Join(wantArgv, "\x00") {
		t.Fatalf("argv = %#v, want %#v", append([]string{gotName}, gotArgs...), wantArgv)
	}
}

func sessionWithTTL(now time.Time, tier session.CacheTier, ttlSeconds int, known bool) session.Session {
	last := now.Add(-time.Duration(ttlSeconds)*time.Second + time.Duration(minIntForTest(300, ttlSeconds))*time.Second)
	if !known {
		last = now.Add(-4 * time.Minute)
	}
	return session.Session{
		SessionID:     string(tier),
		LastMessageAt: &last,
		CacheWindow: session.CacheWindow{
			Tier:       tier,
			TTLSeconds: ttlSeconds,
			Known:      known,
		},
	}
}

func activeSession(id string, now time.Time, ttl time.Duration, remaining time.Duration) session.Session {
	last := now.Add(-(ttl - remaining))
	return session.Session{
		SessionID:     id,
		ShortID:       id,
		Project:       "project-" + id,
		LastMessageAt: &last,
		CacheWindow: session.CacheWindow{
			Tier:       session.Tier1Hour,
			TTLSeconds: int(ttl.Seconds()),
			Known:      true,
		},
	}
}

type fakeClaudeRunner struct {
	availableErr error
	sendResult   RunResult
}

func (f fakeClaudeRunner) Available() error {
	return f.availableErr
}

func (f fakeClaudeRunner) Send(context.Context, RunRequest) RunResult {
	return f.sendResult
}

func writeJSONL(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func appendJSONL(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
}

func minIntForTest(a, b int) int {
	if b == 0 || a < b {
		return a
	}
	return b
}
