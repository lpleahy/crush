package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// fakeTool records the context it was invoked with so tests can assert on
// values stamped onto it by the hookedTool decorator.
type fakeTool struct {
	name   string
	called bool
	gotCtx context.Context
	resp   fantasy.ToolResponse
	err    error
}

func (f *fakeTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: f.name}
}

func (f *fakeTool) Run(ctx context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
	f.called = true
	f.gotCtx = ctx
	return f.resp, f.err
}

func (f *fakeTool) ProviderOptions() fantasy.ProviderOptions     { return nil }
func (f *fakeTool) SetProviderOptions(_ fantasy.ProviderOptions) {}

// newRunner builds a hooks.Runner from a single HookConfig, running the
// config-loader path that compiles the matcher regex.
func newRunner(t *testing.T, cmd string) *hooks.Runner {
	t.Helper()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventPreToolUse: {{Command: cmd}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())
	return hooks.NewRunner(cfg.Hooks[hooks.EventPreToolUse], t.TempDir(), t.TempDir())
}

func TestHookedTool_AllowStampsHookApproval(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	runner := newRunner(t, `echo '{"decision":"allow"}'`)
	tool := newHookedTool(inner, runner, nil)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-1", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called, "inner tool should have run")

	// The inner tool's permission service can now treat call-1 as pre-approved.
	svc := permission.NewPermissionService(t.TempDir(), false, nil)
	granted, err := svc.Request(inner.gotCtx, permission.CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "call-1",
		ToolName:   "view",
		Action:     "read",
		Path:       t.TempDir(),
	})
	require.NoError(t, err)
	require.True(t, granted, "hook allow should bypass the permission prompt")
}

func TestHookedTool_SilentDoesNotStampApproval(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	runner := newRunner(t, `exit 0`) // no stdout, no decision
	tool := newHookedTool(inner, runner, nil)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-2", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called)

	// With no hook opinion, a fresh permission request has nothing stamped
	// and must fall through to the normal flow. We verify by checking that
	// the context does not look pre-approved for this call ID: sending a
	// request that no subscriber resolves will block until cancelled.
	svc := permission.NewPermissionService(t.TempDir(), false, nil)
	ctx, cancel := context.WithCancel(inner.gotCtx)
	cancel()
	granted, err := svc.Request(ctx, permission.CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "call-2",
		ToolName:   "view",
		Action:     "read",
		Path:       t.TempDir(),
	})
	require.Error(t, err, "no approval stamped => request should reach the prompt path")
	require.False(t, granted)
}

func TestHookedTool_DenySkipsInnerTool(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash"}
	runner := newRunner(t, `echo "blocked" >&2; exit 2`)
	tool := newHookedTool(inner, runner, nil)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-3", Name: "bash"})
	require.NoError(t, err)
	require.False(t, inner.called, "denied call must not reach the inner tool")
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "blocked")
}

// postHookCapture records the post-tool events a hookedTool fires.
type postHookEvent struct {
	event  string
	tool   string
	status string
}

func TestHookedTool_FiresPostToolUseOnSuccess(t *testing.T) {
	t.Parallel()

	var fired []postHookEvent
	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("done")}
	postFire := func(_ context.Context, event, toolName, _ string, opts hooks.PayloadOpts) {
		fired = append(fired, postHookEvent{event: event, tool: toolName, status: opts.Status})
	}
	tool := newHookedTool(inner, nil, postFire)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "view"})
	require.NoError(t, err)
	require.Len(t, fired, 1)
	require.Equal(t, hooks.EventPostToolUse, fired[0].event)
	require.Equal(t, "view", fired[0].tool)
	require.Equal(t, "success", fired[0].status)
}

func TestHookedTool_FiresFailureOnInnerError(t *testing.T) {
	t.Parallel()

	var fired []postHookEvent
	inner := &fakeTool{name: "bash", err: assertErr("boom")}
	postFire := func(_ context.Context, event, toolName, _ string, opts hooks.PayloadOpts) {
		fired = append(fired, postHookEvent{event: event, tool: toolName, status: opts.Status})
	}
	tool := newHookedTool(inner, nil, postFire)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "bash"})
	require.Error(t, err)
	require.Len(t, fired, 1)
	require.Equal(t, hooks.EventPostToolUseFailure, fired[0].event)
	require.Equal(t, "error", fired[0].status)
}

func TestHookedTool_FiresFailureOnHookDeny(t *testing.T) {
	t.Parallel()

	var fired []postHookEvent
	inner := &fakeTool{name: "bash"}
	runner := newRunner(t, `echo "nope" >&2; exit 2`)
	postFire := func(_ context.Context, event, toolName, _ string, opts hooks.PayloadOpts) {
		fired = append(fired, postHookEvent{event: event, tool: toolName, status: opts.Status})
	}
	tool := newHookedTool(inner, runner, postFire)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "bash"})
	require.NoError(t, err)
	require.False(t, inner.called)
	require.Len(t, fired, 1)
	require.Equal(t, hooks.EventPostToolUseFailure, fired[0].event)
	require.Equal(t, "denied", fired[0].status)
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

// fullPostHook captures the entire PayloadOpts a post-hook fired with so
// tests can assert the outcome -> status/error/output mapping, not just
// the status string.
type fullPostHook struct {
	event string
	tool  string
	input string
	opts  hooks.PayloadOpts
}

// TestHookedTool_PostHookSuccessCarriesOutputNoError asserts the success
// mapping: status "success", tool_output set to the response content, and
// no error. DurationMs must be populated (non-negative) on every outcome.
func TestHookedTool_PostHookSuccessCarriesOutputNoError(t *testing.T) {
	t.Parallel()

	var got fullPostHook
	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("the output")}
	postFire := func(_ context.Context, event, toolName, input string, opts hooks.PayloadOpts) {
		got = fullPostHook{event: event, tool: toolName, input: input, opts: opts}
	}
	tool := newHookedTool(inner, nil, postFire)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "view", Input: `{"file_path":"/x"}`})
	require.NoError(t, err)
	require.Equal(t, hooks.EventPostToolUse, got.event)
	require.Equal(t, "success", got.opts.Status)
	require.Equal(t, "the output", got.opts.ToolOutput, "success must carry the tool output")
	require.Empty(t, got.opts.Error, "success must not carry an error")
	require.Equal(t, `{"file_path":"/x"}`, got.input, "tool input is passed through to the post hook")
	require.GreaterOrEqual(t, got.opts.DurationMs, int64(0), "duration should be populated")
}

// TestHookedTool_PostHookErrorCarriesErrNoOutput asserts the inner-error
// mapping: status "error", the error message set, and no tool_output (a
// failed call produced none).
func TestHookedTool_PostHookErrorCarriesErrNoOutput(t *testing.T) {
	t.Parallel()

	var got fullPostHook
	inner := &fakeTool{name: "bash", err: assertErr("exit status 1")}
	postFire := func(_ context.Context, event, toolName, input string, opts hooks.PayloadOpts) {
		got = fullPostHook{event: event, tool: toolName, input: input, opts: opts}
	}
	tool := newHookedTool(inner, nil, postFire)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "bash"})
	require.Error(t, err)
	require.Equal(t, hooks.EventPostToolUseFailure, got.event)
	require.Equal(t, "error", got.opts.Status)
	require.Equal(t, "exit status 1", got.opts.Error, "error outcome must carry the error message")
	require.Empty(t, got.opts.ToolOutput, "a failed call carries no tool output")
}

// TestHookedTool_PostHookDeniedCarriesReasonNoOutput asserts the
// hook-deny mapping: status "denied", the deny reason rides the error
// field (the same field every outcome-bearing event uses), and there is
// no tool_output because the inner tool never ran.
func TestHookedTool_PostHookDeniedCarriesReasonNoOutput(t *testing.T) {
	t.Parallel()

	var got fullPostHook
	inner := &fakeTool{name: "bash"}
	runner := newRunner(t, `echo "policy violation" >&2; exit 2`)
	postFire := func(_ context.Context, event, toolName, input string, opts hooks.PayloadOpts) {
		got = fullPostHook{event: event, tool: toolName, input: input, opts: opts}
	}
	tool := newHookedTool(inner, runner, postFire)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "bash"})
	require.NoError(t, err)
	require.False(t, inner.called)
	require.Equal(t, hooks.EventPostToolUseFailure, got.event)
	require.Equal(t, "denied", got.opts.Status)
	require.Contains(t, got.opts.Error, "policy violation", "denied reason rides the error field")
	require.Empty(t, got.opts.ToolOutput, "a denied call never produced output")
	// A plain deny (exit 2) blocks only this tool call; the turn continues.
	require.False(t, resp.StopTurn, "a plain deny must not stop the turn")
	require.True(t, resp.IsError)
}

// TestHookedTool_HaltStopsTurnAndFiresFailure asserts the halt mapping
// (exit 49): the response sets StopTurn so the whole turn ends, and a
// PostToolUseFailure still fires with the "denied" outcome so external
// indicators see a terminal transition.
func TestHookedTool_HaltStopsTurnAndFiresFailure(t *testing.T) {
	t.Parallel()

	var got fullPostHook
	inner := &fakeTool{name: "bash"}
	runner := newRunner(t, `echo "halting the turn" >&2; exit 49`)
	postFire := func(_ context.Context, event, toolName, input string, opts hooks.PayloadOpts) {
		got = fullPostHook{event: event, tool: toolName, input: input, opts: opts}
	}
	tool := newHookedTool(inner, runner, postFire)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "bash"})
	require.NoError(t, err)
	require.False(t, inner.called, "a halted call must not reach the inner tool")
	require.True(t, resp.StopTurn, "halt must stop the turn")
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "halting the turn")
	require.Equal(t, hooks.EventPostToolUseFailure, got.event)
	require.Equal(t, "denied", got.opts.Status)
}

// TestHookedTool_NilPostFireIsNoop asserts firePostHook is a safe no-op
// when no postFire callback was wired (sub-agents, or a top-level agent
// without post hooks). The inner tool still runs and returns normally.
func TestHookedTool_NilPostFireIsNoop(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	tool := newHookedTool(inner, nil, nil)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called)
	require.Equal(t, "ok", resp.Content)
}

func TestWrapToolsWithHooks(t *testing.T) {
	t.Parallel()

	runner := newRunner(t, `exit 0`)
	inputs := []fantasy.AgentTool{&fakeTool{name: "a"}, &fakeTool{name: "b"}}

	t.Run("top-level agent wraps every tool", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, nil, false)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
	})

	t.Run("sub-agent skips the wrap", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, nil, true)
		require.Equal(t, inputs, out, "sub-agent tools should be returned unwrapped")
		for _, tool := range out {
			_, isHooked := tool.(*hookedTool)
			require.False(t, isHooked, "sub-agent tool should not be wrapped")
		}
	})

	t.Run("nil runner and nil postFire skips the wrap for both agent kinds", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, nil, false))
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, nil, true))
	})

	t.Run("postFire alone wraps top-level tools even with nil runner", func(t *testing.T) {
		t.Parallel()
		postFire := func(context.Context, string, string, string, hooks.PayloadOpts) {}
		out := wrapToolsWithHooks(inputs, nil, postFire, false)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
	})
}
