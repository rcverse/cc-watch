package keepalive

import "testing"

func TestSendClassification(t *testing.T) {
	cases := []struct {
		name string
		exec RunnerExecution
		want string
	}{
		{"unavailable", RunnerExecution{ClaudeUnavailable: true}, "claude_unavailable"},
		{"limit", RunnerExecution{Result: RunResult{Limit: true}}, "claude_limit"},
		{"exit_nonzero", RunnerExecution{Result: RunResult{ExitCode: 1}}, "subprocess_failed"},
		{"err", RunnerExecution{Result: RunResult{Err: ErrSubprocess}}, "subprocess_failed"},
		{"ok", RunnerExecution{Result: RunResult{}}, "sent_pending_confirm"},
	}
	for _, tc := range cases {
		if got := sendClassification(tc.exec); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
