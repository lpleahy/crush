package hooks

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestAggregation(t *testing.T) {
	t.Parallel()

	t.Run("empty results", func(t *testing.T) {
		t.Parallel()
		agg := aggregate(nil, "{}")
		require.Equal(t, DecisionNone, agg.Decision)
		require.Empty(t, agg.Reason)
		require.Empty(t, agg.Context)
		require.False(t, agg.Halt)
	})

	t.Run("single allow", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow},
		}, "{}")
		require.Equal(t, DecisionAllow, agg.Decision)
	})

	t.Run("deny wins over allow", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, Context: "ctx1"},
			{Decision: DecisionDeny, Reason: "blocked"},
		}, "{}")
		require.Equal(t, DecisionDeny, agg.Decision)
		require.Equal(t, "blocked", agg.Reason)
		require.Equal(t, "ctx1", agg.Context)
	})

	t.Run("multiple deny reasons concatenated", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionDeny, Reason: "reason1"},
			{Decision: DecisionDeny, Reason: "reason2"},
		}, "{}")
		require.Equal(t, DecisionDeny, agg.Decision)
		require.Equal(t, "reason1\nreason2", agg.Reason)
	})

	t.Run("context concatenated from all hooks", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, Context: "ctx-a"},
			{Decision: DecisionNone, Context: "ctx-b"},
		}, "{}")
		require.Equal(t, DecisionAllow, agg.Decision)
		require.Equal(t, "ctx-a\nctx-b", agg.Context)
	})

	t.Run("allow wins over none", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionNone},
			{Decision: DecisionAllow},
		}, "{}")
		require.Equal(t, DecisionAllow, agg.Decision)
	})

	t.Run("halt is sticky across results", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow},
			{Halt: true, Reason: "stop now"},
		}, "{}")
		require.True(t, agg.Halt)
		require.Contains(t, agg.Reason, "stop now")
	})

	t.Run("halt with deny only records reason once", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionDeny, Halt: true, Reason: "stop"},
		}, "{}")
		require.True(t, agg.Halt)
		require.Equal(t, DecisionDeny, agg.Decision)
		require.Equal(t, "stop", agg.Reason)
	})
}

func TestParseStdout(t *testing.T) {
	t.Parallel()

	t.Run("empty stdout", func(t *testing.T) {
		t.Parallel()
		r := parseStdout("")
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("valid allow", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":"some context"}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, "some context", r.Context)
	})

	t.Run("valid deny", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"deny","reason":"not allowed"}`)
		require.Equal(t, DecisionDeny, r.Decision)
		require.Equal(t, "not allowed", r.Reason)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{bad json}`)
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("unknown decision", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"maybe"}`)
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("version 1 accepted", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"version":1,"decision":"allow"}`)
		require.Equal(t, DecisionAllow, r.Decision)
	})

	t.Run("unknown higher version still parses", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"version":99,"decision":"deny","reason":"future"}`)
		require.Equal(t, DecisionDeny, r.Decision)
		require.Equal(t, "future", r.Reason)
	})

	t.Run("halt true without decision", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"halt":true,"reason":"turn over"}`)
		require.True(t, r.Halt)
		require.Equal(t, "turn over", r.Reason)
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("context string form", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":"one note"}`)
		require.Equal(t, "one note", r.Context)
	})

	t.Run("context array form", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":["first","second"]}`)
		require.Equal(t, "first\nsecond", r.Context)
	})

	t.Run("context array drops empty entries", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":["","keep",""]}`)
		require.Equal(t, "keep", r.Context)
	})

	t.Run("context null becomes empty", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":null}`)
		require.Empty(t, r.Context)
	})
}

func TestBuildEnv(t *testing.T) {
	t.Parallel()

	env := BuildEnv(EventPreToolUse, "bash", "sess-1", "/work", "/project", `{"command":"ls","file_path":"/tmp/f.txt"}`)

	envMap := make(map[string]string)
	for _, e := range env {
		parts := splitFirst(e, "=")
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	require.Equal(t, EventPreToolUse, envMap["CRUSH_EVENT"])
	require.Equal(t, "bash", envMap["CRUSH_TOOL_NAME"])
	require.Equal(t, "sess-1", envMap["CRUSH_SESSION_ID"])
	require.Equal(t, "/work", envMap["CRUSH_CWD"])
	require.Equal(t, "/project", envMap["CRUSH_PROJECT_DIR"])
	require.Equal(t, "ls", envMap["CRUSH_TOOL_INPUT_COMMAND"])
	require.Equal(t, "/tmp/f.txt", envMap["CRUSH_TOOL_INPUT_FILE_PATH"])

	// Shared Crush markers must be present so hook-authored scripts can
	// detect they're running under Crush the same way bash-tool-invoked
	// scripts can.
	require.Equal(t, "1", envMap["CRUSH"])
	require.Equal(t, "crush", envMap["AGENT"])
	require.Equal(t, "crush", envMap["AI_AGENT"])
}

func splitFirst(s, sep string) []string {
	before, after, found := strings.Cut(s, sep)
	if !found {
		return []string{s}
	}
	return []string{before, after}
}

func TestBuildPayload(t *testing.T) {
	t.Parallel()
	payload := BuildPayload(EventPreToolUse, "sess-1", "/work", "bash", `{"command":"ls"}`)
	s := string(payload)
	require.Contains(t, s, `"event":"`+EventPreToolUse+`"`)
	require.Contains(t, s, `"tool_name":"bash"`)
	// tool_input should be an object, not a string.
	require.Contains(t, s, `"tool_input":{"command":"ls"}`)
}

func TestBuildPayloadOpts_LifecycleFields(t *testing.T) {
	t.Parallel()
	payload := BuildPayloadOpts(EventStop, "sess-1", "/work", "", "", PayloadOpts{
		TurnID:     "turn-9",
		ProjectDir: "/project",
		Provider:   "chatgpt",
		Model:      "gpt-5.5",
		Status:     "cancelled",
		Error:      "boom",
	})
	s := string(payload)
	require.Contains(t, s, `"event":"Stop"`)
	require.Contains(t, s, `"turn_id":"turn-9"`)
	require.Contains(t, s, `"provider":"chatgpt"`)
	require.Contains(t, s, `"model":"gpt-5.5"`)
	require.Contains(t, s, `"status":"cancelled"`)
	require.Contains(t, s, `"error":"boom"`)
	// A notification event without a tool must omit tool_name/tool_input.
	require.NotContains(t, s, `"tool_name"`)
	require.NotContains(t, s, `"tool_input"`)
}

func TestBuildEnvOpts_LifecycleVars(t *testing.T) {
	t.Parallel()
	env := BuildEnvOpts(EventModelRequestStart, "", "sess-1", "/work", "/project", "", PayloadOpts{
		TurnID:   "turn-9",
		Provider: "chatgpt",
		Model:    "gpt-5.5",
		Status:   "success",
	})
	envMap := make(map[string]string)
	for _, e := range env {
		parts := splitFirst(e, "=")
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	require.Equal(t, "ModelRequestStart", envMap["CRUSH_EVENT"])
	require.Equal(t, "turn-9", envMap["CRUSH_TURN_ID"])
	require.Equal(t, "chatgpt", envMap["CRUSH_PROVIDER"])
	require.Equal(t, "gpt-5.5", envMap["CRUSH_MODEL"])
	require.Equal(t, "success", envMap["CRUSH_STATUS"])
	// No tool for this event — CRUSH_TOOL_NAME must be absent.
	_, hasTool := envMap["CRUSH_TOOL_NAME"]
	require.False(t, hasTool, "CRUSH_TOOL_NAME should be omitted for non-tool events")
}

// TestIsNotificationEvent_AssistantMessage asserts the AssistantMessage
// event is notification-only (fire-and-forget): its decision/halt return
// values must be ignored so a hook on assistant output can never block or
// rewrite the turn.
func TestIsNotificationEvent_AssistantMessage(t *testing.T) {
	t.Parallel()
	require.True(t, IsNotificationEvent(EventAssistantMessage))
}

func TestBuildEnvOpts_AssistantMessageVars(t *testing.T) {
	t.Parallel()
	env := BuildEnvOpts(EventAssistantMessage, "", "sess-1", "/work", "/project", "", PayloadOpts{
		Provider:          "chatgpt",
		Model:             "gpt-5.5",
		MessageText:       "hello world",
		MessageTokenCount: 42,
		FinishReason:      "end_turn",
	})
	envMap := make(map[string]string)
	for _, e := range env {
		parts := splitFirst(e, "=")
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	require.Equal(t, "AssistantMessage", envMap["CRUSH_EVENT"])
	require.Equal(t, "chatgpt", envMap["CRUSH_PROVIDER"])
	require.Equal(t, "gpt-5.5", envMap["CRUSH_MODEL"])
	require.Equal(t, "hello world", envMap["CRUSH_ASSISTANT_MESSAGE_TEXT"])
	require.Equal(t, "end_turn", envMap["CRUSH_ASSISTANT_MESSAGE_FINISH_REASON"])
	require.Equal(t, "42", envMap["CRUSH_ASSISTANT_MESSAGE_TOKEN_COUNT"])
	// No tool for this event — CRUSH_TOOL_NAME must be absent.
	_, hasTool := envMap["CRUSH_TOOL_NAME"]
	require.False(t, hasTool, "CRUSH_TOOL_NAME should be omitted for AssistantMessage")
}

// TestBuildEnvOpts_AssistantMessageTextTruncated asserts the env var copy
// of the assistant text is clamped to ~4KB while the JSON payload keeps
// the full text. A long assistant turn must not blow past OS env limits.
func TestBuildEnvOpts_AssistantMessageTextTruncated(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 10*1024) // 10KB, well over the 4KB cap.
	env := BuildEnvOpts(EventAssistantMessage, "", "sess-1", "/work", "/project", "", PayloadOpts{
		MessageText: long,
	})
	var got string
	var found bool
	for _, e := range env {
		if parts := splitFirst(e, "="); len(parts) == 2 && parts[0] == "CRUSH_ASSISTANT_MESSAGE_TEXT" {
			got = parts[1]
			found = true
		}
	}
	require.True(t, found, "CRUSH_ASSISTANT_MESSAGE_TEXT should be present")
	// Truncated: shorter than the input and within the cap plus the
	// multi-byte ellipsis marker.
	require.Less(t, len(got), len(long))
	require.LessOrEqual(t, len(got), maxEnvTextBytes+len("…"))
	require.True(t, strings.HasSuffix(got, "…"), "truncated text should end with an ellipsis")

	// The full text survives untruncated in the JSON payload.
	payload := BuildPayloadOpts(EventAssistantMessage, "sess-1", "/work", "", "", PayloadOpts{
		MessageText: long,
	})
	require.Contains(t, string(payload), long)
}

// TestBuildEnvOpts_AssistantMessageVarsOmittedWhenEmpty asserts the
// AssistantMessage env vars follow the existing omit-when-empty
// convention so events that don't carry them stay clean.
func TestBuildEnvOpts_AssistantMessageVarsOmittedWhenEmpty(t *testing.T) {
	t.Parallel()
	env := BuildEnvOpts(EventStop, "", "sess-1", "/work", "/project", "", PayloadOpts{
		Status: "success",
	})
	for _, e := range env {
		require.NotContains(t, e, "CRUSH_ASSISTANT_MESSAGE_")
	}
}

func TestBuildPayloadOpts_AssistantMessageFields(t *testing.T) {
	t.Parallel()
	payload := BuildPayloadOpts(EventAssistantMessage, "sess-1", "/work", "", "", PayloadOpts{
		Provider:          "chatgpt",
		Model:             "gpt-5.5",
		MessageText:       "the full assistant reply",
		MessageTokenCount: 7,
		FinishReason:      "end_turn",
	})
	s := string(payload)
	require.Contains(t, s, `"event":"AssistantMessage"`)
	require.Contains(t, s, `"message_text":"the full assistant reply"`)
	require.Contains(t, s, `"message_token_count":7`)
	require.Contains(t, s, `"finish_reason":"end_turn"`)
	// A notification event without a tool must omit tool_name/tool_input.
	require.NotContains(t, s, `"tool_name"`)
	require.NotContains(t, s, `"tool_input"`)
}

// TestFireNotification_EndToEnd drives a real hook script through the
// package-level helper the app layer uses for SessionStart/SessionEnd,
// asserting the script sees the payload on stdin and the env vars.
func TestFireNotification_EndToEnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out := filepath.Join(dir, "captured.txt")
	// Capture both the stdin JSON and a couple of env vars.
	cmd := `cat > "` + out + `"; printf "\nEVENT=%s STATUS=%s\n" "$CRUSH_EVENT" "$CRUSH_STATUS" >> "` + out + `"`

	cfgs := []config.HookConfig{{Command: cmd}}
	FireNotification(context.Background(), cfgs, EventStop, "sess-1", dir, PayloadOpts{
		TurnID: "turn-1",
		Status: "success",
	})

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	got := string(data)
	require.Contains(t, got, `"event":"Stop"`)
	require.Contains(t, got, `"turn_id":"turn-1"`)
	require.Contains(t, got, `"status":"success"`)
	require.Contains(t, got, "EVENT=Stop STATUS=success")
}

func TestFireNotification_NoHooksIsNoop(t *testing.T) {
	t.Parallel()
	// Must not panic or error with an empty config.
	FireNotification(context.Background(), nil, EventStop, "s", t.TempDir(), PayloadOpts{})
}

func TestRunnerExitCode0Allow(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo '{"decision":"allow","context":"ok"}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Decision)
	require.Equal(t, "ok", result.Context)
}

func TestRunnerExitCode2Deny(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo "forbidden" >&2; exit 2`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionDeny, result.Decision)
	require.False(t, result.Halt)
	require.Equal(t, "forbidden", result.Reason)
}

func TestRunnerExitCode49Halt(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo "stop the turn" >&2; exit 49`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.True(t, result.Halt)
	require.Equal(t, DecisionDeny, result.Decision)
	require.Equal(t, "stop the turn", result.Reason)
}

func TestRunnerHaltViaJSON(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo '{"halt":true,"reason":"via json"}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.True(t, result.Halt)
	require.Equal(t, "via json", result.Reason)
}

func TestRunnerExitCodeOtherNonBlocking(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `exit 1`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionNone, result.Decision)
}

func TestRunnerTimeout(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `sleep 10`,
		Timeout: 1,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	start := time.Now()
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.Equal(t, DecisionNone, result.Decision)
	require.Less(t, elapsed, 5*time.Second)
}

func TestRunnerDeduplication(t *testing.T) {
	t.Parallel()
	// Two hooks with the same command should only run once.
	hookCfg := config.HookConfig{
		Command: `echo '{"decision":"allow"}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg, hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Decision)
}

func TestRunnerNoMatchingHooks(t *testing.T) {
	t.Parallel()
	// Hooks are empty.
	r := NewRunner(nil, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionNone, result.Decision)
}

// validatedHooks builds hook configs and runs ValidateHooks to compile
// matcher regexes, mirroring the real config-load path.
func validatedHooks(t *testing.T, hooks []config.HookConfig) []config.HookConfig {
	t.Helper()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			EventPreToolUse: hooks,
		},
	}
	require.NoError(t, cfg.ValidateHooks())
	return cfg.Hooks[EventPreToolUse]
}

func TestRunnerMatcherFiltering(t *testing.T) {
	t.Parallel()

	t.Run("compiled regex matches", func(t *testing.T) {
		t.Parallel()
		hooks := validatedHooks(t, []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"blocked"}'`, Matcher: "^bash$"},
		})
		r := NewRunner(hooks, t.TempDir(), t.TempDir())
		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionDeny, result.Decision)
	})

	t.Run("compiled regex does not match", func(t *testing.T) {
		t.Parallel()
		hooks := validatedHooks(t, []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"blocked"}'`, Matcher: "^edit$"},
		})
		r := NewRunner(hooks, t.TempDir(), t.TempDir())
		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionNone, result.Decision)
	})

	t.Run("no matcher matches everything", func(t *testing.T) {
		t.Parallel()
		hooks := validatedHooks(t, []config.HookConfig{
			{Command: `echo '{"decision":"allow"}'`},
		})
		r := NewRunner(hooks, t.TempDir(), t.TempDir())
		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionAllow, result.Decision)
	})

	t.Run("partial regex match", func(t *testing.T) {
		t.Parallel()
		hooks := validatedHooks(t, []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"mcp blocked"}'`, Matcher: "^mcp_"},
		})
		r := NewRunner(hooks, t.TempDir(), t.TempDir())

		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "mcp_github_get_me", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionDeny, result.Decision)

		result, err = r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionNone, result.Decision)
	})

	// Runner must compile matchers itself; it cannot rely on
	// ValidateHooks having run first. This is the guarantee that prevents
	// the reload-drops-matcher class of bug.
	t.Run("runner compiles matcher without ValidateHooks", func(t *testing.T) {
		t.Parallel()
		raw := []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"blocked"}'`, Matcher: "^bash$"},
		}
		r := NewRunner(raw, t.TempDir(), t.TempDir())

		deny, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionDeny, deny.Decision)

		noop, err := r.Run(context.Background(), EventPreToolUse, "sess", "view", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionNone, noop.Decision)
	})

	// A matcher that fails to compile at Runner construction must not
	// degrade to match-everything; the hook is dropped instead.
	t.Run("runner skips hooks with invalid matcher", func(t *testing.T) {
		t.Parallel()
		raw := []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"should not fire"}'`, Matcher: "[invalid"},
		}
		r := NewRunner(raw, t.TempDir(), t.TempDir())

		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionNone, result.Decision)
		require.Empty(t, r.Hooks())
	})
}

func TestValidateHooksInvalidRegex(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			EventPreToolUse: {
				{Command: "true", Matcher: "[invalid"},
			},
		},
	}
	err := cfg.ValidateHooks()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid matcher regex")
}

func TestValidateHooksEmptyCommand(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			EventPreToolUse: {
				{Command: ""},
			},
		},
	}
	err := cfg.ValidateHooks()
	require.Error(t, err)
	require.Contains(t, err.Error(), "command is required")
}

func TestValidateHooksNormalizesEventNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"canonical", "PreToolUse"},
		{"lowercase", "pretooluse"},
		{"snake_case", "pre_tool_use"},
		{"upper_snake", "PRE_TOOL_USE"},
		{"mixed_case", "preToolUse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.Config{
				Hooks: map[string][]config.HookConfig{
					tt.input: {
						{Command: "true"},
					},
				},
			}
			require.NoError(t, cfg.ValidateHooks())
			require.Len(t, cfg.Hooks[EventPreToolUse], 1)
		})
	}
}

func TestRunnerHookNameUsesDisplayName(t *testing.T) {
	t.Parallel()

	t.Run("name field is used when set", func(t *testing.T) {
		t.Parallel()
		hookCfg := config.HookConfig{
			Name:    "my-hook",
			Command: `echo '{"decision":"allow"}'`,
		}
		r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionAllow, result.Decision)
		require.Len(t, result.Hooks, 1)
		require.Equal(t, "my-hook", result.Hooks[0].Name)
	})

	t.Run("command is used when name is empty", func(t *testing.T) {
		t.Parallel()
		hookCfg := config.HookConfig{
			Command: `echo '{"decision":"allow"}'`,
		}
		r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionAllow, result.Decision)
		require.Len(t, result.Hooks, 1)
		require.Equal(t, `echo '{"decision":"allow"}'`, result.Hooks[0].Name)
	})
}

func TestRunnerParallelExecution(t *testing.T) {
	t.Parallel()
	// Two hooks: one allows, one denies. Deny should win.
	hooks := []config.HookConfig{
		{Command: `echo '{"decision":"allow","context":"hook1"}'`},
		{Command: `echo '{"decision":"deny","reason":"nope"}' ; exit 0`},
	}
	r := NewRunner(hooks, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionDeny, result.Decision)
	require.Equal(t, "nope", result.Reason)
}

func TestRunnerEnvVarsPropagated(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `printf '{"decision":"allow","context":"%s"}' "$CRUSH_TOOL_NAME"`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Decision)
	require.Equal(t, "bash", result.Context)
}

func TestParseStdoutUpdatedInput(t *testing.T) {
	t.Parallel()

	t.Run("nested object", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","updated_input":{"command":"rtk cat foo.go"}}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, `{"command":"rtk cat foo.go"}`, r.UpdatedInput)
	})

	t.Run("stringified backward compat", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","updated_input":"{\"command\":\"rtk cat foo.go\"}"}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, `{"command":"rtk cat foo.go"}`, r.UpdatedInput)
	})

	t.Run("no updated_input", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow"}`)
		require.Empty(t, r.UpdatedInput)
	})
}

func TestAggregationUpdatedInput(t *testing.T) {
	t.Parallel()

	t.Run("patches merge in config order with later overriding", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{"command":"first","keep":"me"}`},
			{Decision: DecisionAllow, UpdatedInput: `{"command":"second"}`},
		}, `{"command":"orig","timeout":60}`)
		require.Equal(t, DecisionAllow, agg.Decision)
		// command overridden by second patch; keep preserved from first
		// patch; timeout preserved from original input.
		require.JSONEq(
			t,
			`{"command":"second","keep":"me","timeout":60}`,
			agg.UpdatedInput,
		)
	})

	t.Run("shallow: nested objects are replaced wholesale", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{"env":{"FOO":"bar"}}`},
		}, `{"env":{"BAZ":"qux"},"command":"ls"}`)
		// "env" is replaced entirely; "command" preserved.
		require.JSONEq(
			t,
			`{"env":{"FOO":"bar"},"command":"ls"}`,
			agg.UpdatedInput,
		)
	})

	t.Run("deny still reports merged input (caller ignores it)", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{"command":"rewritten"}`},
			{Decision: DecisionDeny, Reason: "blocked"},
		}, `{"command":"orig"}`)
		require.Equal(t, DecisionDeny, agg.Decision)
	})

	t.Run("no patches leaves updated_input empty", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow},
			{Decision: DecisionNone},
		}, `{"command":"orig"}`)
		require.Empty(t, agg.UpdatedInput)
	})

	t.Run("invalid patch is ignored", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `"not-an-object"`},
			{Decision: DecisionAllow, UpdatedInput: `{"command":"good"}`},
		}, `{"command":"orig"}`)
		require.JSONEq(t, `{"command":"good"}`, agg.UpdatedInput)
	})

	t.Run("malformed patch JSON is ignored and merge continues", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{broken json`},
			{Decision: DecisionAllow, UpdatedInput: `{"command":"good"}`},
		}, `{"command":"orig"}`)
		require.JSONEq(t, `{"command":"good"}`, agg.UpdatedInput)
	})

	t.Run("non-object tool_input rejects all patches", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{"command":"rewrite"}`},
		}, `"just-a-string"`)
		require.Empty(t, agg.UpdatedInput)
	})

	t.Run("null updated_input is a no-op", func(t *testing.T) {
		t.Parallel()
		// parseStdout converts null updated_input to "", so aggregate
		// never sees a patch — the merged input is empty and the
		// original tool_input is used unchanged.
		r := parseStdout(`{"decision":"allow","updated_input":null}`)
		require.Empty(t, r.UpdatedInput)
		agg := aggregate([]HookResult{r}, `{"command":"orig"}`)
		require.Empty(t, agg.UpdatedInput)
	})
}

// TestRunnerAbandonRaceSafety verifies that if a hook's shell execution
// does not yield to ctx cancellation within abandonGrace, runOne returns
// promptly and never touches the shared stdout/stderr buffers again —
// even while the abandoned goroutine continues to write to them.
//
// The substitute shell executor ignores ctx entirely, writes to Stdout
// both before and after the abandon deadline, and only then returns.
// Under -race this catches any code path in runOne that reads those
// buffers after returning the DecisionNone abandon result.
func TestRunnerAbandonRaceSafety(t *testing.T) {
	origRunShell := runShell
	t.Cleanup(func() { runShell = origRunShell })

	// Synchronize shutdown with the abandoned goroutine so the test
	// exits cleanly even under -race.
	var wg sync.WaitGroup
	release := make(chan struct{})
	t.Cleanup(func() {
		close(release)
		wg.Wait()
	})

	runShell = func(_ context.Context, opts shell.RunOptions) error {
		wg.Add(1)
		defer wg.Done()
		// Write before the caller observes ctx.Done(); the caller will
		// not read the buffer while we still own it.
		_, _ = io.WriteString(opts.Stdout, "before\n")
		// Hold past ctx deadline + abandonGrace so the caller takes
		// the abandon branch, then continue writing. If the caller
		// reads these buffers after abandoning, -race will flag it.
		select {
		case <-time.After(5 * time.Second):
		case <-release:
		}
		_, _ = io.WriteString(opts.Stdout, "after\n")
		return nil
	}

	hookCfg := config.HookConfig{
		Command: "# irrelevant; runShell is stubbed",
		Timeout: 1,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())

	start := time.Now()
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, DecisionNone, result.Decision)
	// Abandon must happen at ~timeout + abandonGrace. Allow generous
	// slack so CI noise doesn't flake the test.
	require.Less(t, elapsed, 3500*time.Millisecond,
		"runOne should return within timeout+abandonGrace+slack")
}

func TestRunnerUpdatedInput(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo '{"decision":"allow","updated_input":{"command":"echo rewritten"}}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{"command":"echo original","timeout":60}`)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Decision)
	require.JSONEq(
		t,
		`{"command":"echo rewritten","timeout":60}`,
		result.UpdatedInput,
	)
}

func TestParseStdoutClaudeCodeFormat(t *testing.T) {
	t.Parallel()

	t.Run("allow with reason", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"hookSpecificOutput":{"permissionDecision":"allow","permissionDecisionReason":"RTK auto-rewrite"}}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, "RTK auto-rewrite", r.Reason)
	})

	t.Run("allow with updatedInput", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"hookSpecificOutput":{"permissionDecision":"allow","updatedInput":{"command":"rtk cat foo.go"}}}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, `{"command":"rtk cat foo.go"}`, r.UpdatedInput)
	})

	t.Run("deny", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"not allowed"}}`)
		require.Equal(t, DecisionDeny, r.Decision)
		require.Equal(t, "not allowed", r.Reason)
	})

	t.Run("no permissionDecision", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"hookSpecificOutput":{}}`)
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("crush format still works", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":"hello"}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, "hello", r.Context)
	})
}

// envToMap parses a CRUSH-style env slice ("KEY=value") into a map,
// splitting on the first '=' so values may themselves contain '='.
func envToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		if k, v, ok := strings.Cut(e, "="); ok {
			m[k] = v
		}
	}
	return m
}

// TestIsNotificationEvent_AllEvents is the decision that governs which
// events can block/halt/rewrite a turn. Every event except PreToolUse is
// notification-only (its decision/halt is ignored); PreToolUse is the
// sole event whose decision is acted on. A regression here would either
// let a notification hook block the turn or silently disarm PreToolUse.
func TestIsNotificationEvent_AllEvents(t *testing.T) {
	t.Parallel()

	notification := []string{
		EventSessionStart,
		EventUserPromptSubmit,
		EventModelRequestStart,
		EventModelRequestStop,
		EventPermissionRequest,
		EventPostToolUse,
		EventPostToolUseFailure,
		EventAssistantMessage,
		EventStop,
		EventSessionEnd,
	}
	for _, ev := range notification {
		require.Truef(t, IsNotificationEvent(ev),
			"%s must be notification-only (decision/halt ignored)", ev)
	}

	// PreToolUse is the only event whose decision can block/halt/rewrite.
	require.False(t, IsNotificationEvent(EventPreToolUse),
		"PreToolUse must NOT be notification-only — its decision is acted on")

	// An unknown event name is not in the notification set; callers treat
	// it as non-firing rather than as a blocking event.
	require.False(t, IsNotificationEvent("NotARealEvent"))

	// The notification set must be exactly the ten non-PreToolUse events.
	require.Len(t, notificationEvents, len(notification),
		"notificationEvents drifted from the enumerated set")
}

// TestBuildPayloadOpts_PostToolUseFields covers the PostToolUse payload:
// the outcome rides status/error, tool_output is carried, and the
// duration surfaces. PostToolUse fires with a tool name/input, unlike the
// tool-less notification events.
func TestBuildPayloadOpts_PostToolUseFields(t *testing.T) {
	t.Parallel()
	payload := BuildPayloadOpts(EventPostToolUse, "sess-1", "/work", "bash", `{"command":"ls"}`, PayloadOpts{
		Status:     "success",
		ToolOutput: "file listing",
		DurationMs: 1234,
	})
	s := string(payload)
	require.Contains(t, s, `"event":"PostToolUse"`)
	require.Contains(t, s, `"tool_name":"bash"`)
	require.Contains(t, s, `"tool_input":{"command":"ls"}`)
	require.Contains(t, s, `"status":"success"`)
	require.Contains(t, s, `"tool_output":"file listing"`)
	require.Contains(t, s, `"duration_ms":1234`)
	// No error on a success outcome.
	require.NotContains(t, s, `"error"`)
}

// TestBuildPayloadOpts_PostToolUseFailureFields asserts the failure
// outcome carries the error and omits tool_output (only successes set it).
func TestBuildPayloadOpts_PostToolUseFailureFields(t *testing.T) {
	t.Parallel()
	payload := BuildPayloadOpts(EventPostToolUseFailure, "sess-1", "/work", "bash", `{"command":"ls"}`, PayloadOpts{
		Status: "error",
		Error:  "exit status 1",
	})
	s := string(payload)
	require.Contains(t, s, `"event":"PostToolUseFailure"`)
	require.Contains(t, s, `"status":"error"`)
	require.Contains(t, s, `"error":"exit status 1"`)
	// A failure carries no tool output.
	require.NotContains(t, s, `"tool_output"`)
}

// TestBuildPayloadOpts_PermissionRequestFields covers the
// PermissionRequest payload, which carries the pending tool's name/input
// plus the permission kind and message.
func TestBuildPayloadOpts_PermissionRequestFields(t *testing.T) {
	t.Parallel()
	payload := BuildPayloadOpts(EventPermissionRequest, "sess-1", "/work", "bash", `{"command":"rm -rf /"}`, PayloadOpts{
		PermissionKind:    "execute",
		PermissionMessage: "needs approval",
	})
	s := string(payload)
	require.Contains(t, s, `"event":"PermissionRequest"`)
	require.Contains(t, s, `"tool_name":"bash"`)
	require.Contains(t, s, `"permission_kind":"execute"`)
	require.Contains(t, s, `"permission_message":"needs approval"`)
}

// TestBuildPayloadOpts_SessionEndReason covers the SessionEnd payload,
// whose distinguishing field is the reason (e.g. "exit").
func TestBuildPayloadOpts_SessionEndReason(t *testing.T) {
	t.Parallel()
	payload := BuildPayloadOpts(EventSessionEnd, "sess-1", "/work", "", "", PayloadOpts{
		Reason: "exit",
	})
	s := string(payload)
	require.Contains(t, s, `"event":"SessionEnd"`)
	require.Contains(t, s, `"reason":"exit"`)
	require.NotContains(t, s, `"tool_name"`)
}

// TestBuildPayloadOpts_OmitsEmptyOptionalFields locks down the
// omit-when-empty JSON convention: a bare notification event with no
// optional fields must emit only the always-present keys.
func TestBuildPayloadOpts_OmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()
	payload := BuildPayloadOpts(EventSessionStart, "sess-1", "/work", "", "", PayloadOpts{})
	s := string(payload)
	// Always present.
	require.Contains(t, s, `"event":"SessionStart"`)
	require.Contains(t, s, `"session_id":"sess-1"`)
	require.Contains(t, s, `"cwd":"/work"`)
	// Every optional field must be omitted.
	for _, key := range []string{
		"turn_id", "project_dir", "tool_name", "tool_input", "tool_output",
		"duration_ms", "provider", "model", "permission_kind",
		"permission_message", "message_kind", "status", "error", "reason",
		"message_text", "message_token_count", "finish_reason",
	} {
		require.NotContainsf(t, s, `"`+key+`"`, "optional field %q should be omitted when empty", key)
	}
}

// TestBuildPayloadOpts_InvalidToolInputFallsBackToEmptyObject asserts
// that a syntactically invalid tool_input is replaced with {} rather than
// corrupting the payload — the hook still receives valid JSON.
func TestBuildPayloadOpts_InvalidToolInputFallsBackToEmptyObject(t *testing.T) {
	t.Parallel()
	payload := BuildPayloadOpts(EventPreToolUse, "sess-1", "/work", "bash", `{not valid json`, PayloadOpts{})
	s := string(payload)
	require.Contains(t, s, `"tool_input":{}`)
	require.True(t, json.Valid(payload), "payload must remain valid JSON")
}

// TestBuildEnvOpts_PermissionAndMessageKindVars covers the two
// permission-related env vars and message_kind, which the lifecycle-var
// test above did not exercise.
func TestBuildEnvOpts_PermissionAndMessageKindVars(t *testing.T) {
	t.Parallel()
	env := BuildEnvOpts(EventPermissionRequest, "bash", "sess-1", "/work", "/project", "", PayloadOpts{
		PermissionKind:    "execute",
		PermissionMessage: "needs approval",
		MessageKind:       "tool_call",
	})
	m := envToMap(env)
	require.Equal(t, "execute", m["CRUSH_PERMISSION_KIND"])
	require.Equal(t, "needs approval", m["CRUSH_PERMISSION_MESSAGE"])
	require.Equal(t, "tool_call", m["CRUSH_MESSAGE_KIND"])
}

// TestBuildEnvOpts_NoEnvVarForErrorReasonOutputDuration documents the
// deliberate asymmetry between the JSON payload and the env block: error,
// reason, tool_output and duration ride the JSON payload only — there is
// no CRUSH_ERROR / CRUSH_REASON / CRUSH_TOOL_OUTPUT / CRUSH_DURATION_MS
// env var. A hook reads these from stdin, not the environment.
func TestBuildEnvOpts_NoEnvVarForErrorReasonOutputDuration(t *testing.T) {
	t.Parallel()
	env := BuildEnvOpts(EventPostToolUseFailure, "bash", "sess-1", "/work", "/project", "", PayloadOpts{
		Status:     "error",
		Error:      "boom",
		Reason:     "blocked by hook",
		ToolOutput: "partial output",
		DurationMs: 99,
	})
	m := envToMap(env)
	// Status does get an env var.
	require.Equal(t, "error", m["CRUSH_STATUS"])
	// These deliberately do not.
	for _, key := range []string{
		"CRUSH_ERROR", "CRUSH_REASON", "CRUSH_TOOL_OUTPUT", "CRUSH_DURATION_MS", "CRUSH_DURATION",
	} {
		_, present := m[key]
		require.Falsef(t, present, "%s should not be exported to the env (payload-only field)", key)
	}
}

// TestBuildEnvOpts_ToolInputCommandAndFilePathOnlyWhenPresent asserts the
// tool-input-derived env vars are extracted only when the corresponding
// JSON key exists, and are omitted otherwise.
func TestBuildEnvOpts_ToolInputCommandAndFilePathOnlyWhenPresent(t *testing.T) {
	t.Parallel()

	t.Run("command only", func(t *testing.T) {
		t.Parallel()
		env := BuildEnvOpts(EventPreToolUse, "bash", "s", "/w", "/p", `{"command":"ls -la"}`, PayloadOpts{})
		m := envToMap(env)
		require.Equal(t, "ls -la", m["CRUSH_TOOL_INPUT_COMMAND"])
		_, hasFP := m["CRUSH_TOOL_INPUT_FILE_PATH"]
		require.False(t, hasFP)
	})

	t.Run("file_path only", func(t *testing.T) {
		t.Parallel()
		env := BuildEnvOpts(EventPreToolUse, "edit", "s", "/w", "/p", `{"file_path":"/tmp/x.go"}`, PayloadOpts{})
		m := envToMap(env)
		require.Equal(t, "/tmp/x.go", m["CRUSH_TOOL_INPUT_FILE_PATH"])
		_, hasCmd := m["CRUSH_TOOL_INPUT_COMMAND"]
		require.False(t, hasCmd)
	})

	t.Run("neither when tool input has no recognized keys", func(t *testing.T) {
		t.Parallel()
		env := BuildEnvOpts(EventPreToolUse, "view", "s", "/w", "/p", `{"offset":10}`, PayloadOpts{})
		m := envToMap(env)
		_, hasCmd := m["CRUSH_TOOL_INPUT_COMMAND"]
		_, hasFP := m["CRUSH_TOOL_INPUT_FILE_PATH"]
		require.False(t, hasCmd)
		require.False(t, hasFP)
	})
}

// TestTruncateEnvText covers the rune-boundary truncation directly,
// including the exact-boundary no-op and the multi-byte-safe cut that the
// 4KB-of-'a' end-to-end test cannot reach (it never lands mid-rune).
func TestTruncateEnvText(t *testing.T) {
	t.Parallel()

	t.Run("under the cap is returned unchanged", func(t *testing.T) {
		t.Parallel()
		s := strings.Repeat("a", maxEnvTextBytes-1)
		require.Equal(t, s, truncateEnvText(s))
	})

	t.Run("exactly at the cap is returned unchanged", func(t *testing.T) {
		t.Parallel()
		s := strings.Repeat("a", maxEnvTextBytes)
		require.Equal(t, s, truncateEnvText(s))
	})

	t.Run("over the cap is truncated with an ellipsis", func(t *testing.T) {
		t.Parallel()
		s := strings.Repeat("a", maxEnvTextBytes+10)
		got := truncateEnvText(s)
		require.True(t, strings.HasSuffix(got, "…"))
		require.LessOrEqual(t, len(got), maxEnvTextBytes+len("…"))
	})

	t.Run("never splits a multi-byte rune at the cut point", func(t *testing.T) {
		t.Parallel()
		// '世' is 3 bytes. Pad with single-byte runes so the cut lands
		// inside a multi-byte rune, forcing the boundary walk-back.
		s := strings.Repeat("a", maxEnvTextBytes-1) + strings.Repeat("世", 10)
		got := truncateEnvText(s)
		// Trim the trailing ellipsis; the remainder must be valid UTF-8
		// with no partial trailing rune.
		body := strings.TrimSuffix(got, "…")
		require.True(t, utf8.ValidString(body), "truncated body must not contain a partial rune")
	})
}
