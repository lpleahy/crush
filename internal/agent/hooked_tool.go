package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/tidwall/sjson"
)

// postHookFire fires PostToolUse or PostToolUseFailure for a completed
// tool call. Implementations build their own runner from config (per
// event) and ignore Decision/Halt — these events are notification-only.
type postHookFire func(ctx context.Context, eventName, toolName, toolInput string, opts hooks.PayloadOpts)

// hookedTool wraps a fantasy.AgentTool to run PreToolUse hooks before
// delegating, and PostToolUse / PostToolUseFailure after.
type hookedTool struct {
	inner    fantasy.AgentTool
	runner   *hooks.Runner
	postFire postHookFire
}

func newHookedTool(inner fantasy.AgentTool, runner *hooks.Runner, postFire postHookFire) *hookedTool {
	return &hookedTool{inner: inner, runner: runner, postFire: postFire}
}

// wrapToolsWithHooks returns a tool slice with each entry wrapped in a
// hookedTool. Returns the original slice unchanged when no hooks are
// configured or when isSubAgent is true — sub-agents never fire hooks,
// the top-level invocation of the sub-agent tool itself is wrapped on
// the caller's side.
func wrapToolsWithHooks(tools []fantasy.AgentTool, runner *hooks.Runner, postFire postHookFire, isSubAgent bool) []fantasy.AgentTool {
	if isSubAgent {
		return tools
	}
	if runner == nil && postFire == nil {
		return tools
	}
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = newHookedTool(tool, runner, postFire)
	}
	return out
}

func (h *hookedTool) Info() fantasy.ToolInfo {
	return h.inner.Info()
}

func (h *hookedTool) ProviderOptions() fantasy.ProviderOptions {
	return h.inner.ProviderOptions()
}

func (h *hookedTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	h.inner.SetProviderOptions(opts)
}

func (h *hookedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := tools.GetSessionFromContext(ctx)
	start := time.Now()

	var result hooks.AggregateResult
	if h.runner != nil {
		var err error
		result, err = h.runner.Run(ctx, hooks.EventPreToolUse, sessionID, call.Name, call.Input)
		if err != nil {
			slog.Warn("Hook execution error, proceeding with tool call",
				"tool", call.Name, "error", err)
		}
	}

	if result.Decision == hooks.DecisionDeny || result.Halt {
		reason := fmt.Sprintf("Tool call blocked by hook. Reason: %s", result.Reason)
		if result.Halt {
			reason = fmt.Sprintf("Turn halted by hook. Reason: %s", result.Reason)
		}
		resp := fantasy.NewTextErrorResponse(reason)
		// Halt ends the whole turn; a plain deny only blocks this tool
		// call so the model can see the error and try something else.
		resp.StopTurn = result.Halt
		resp.Metadata = hookMetadataJSON(result)
		// A denied/halted call is still a tool-call outcome from the
		// hook system's perspective — fire PostToolUseFailure so
		// external indicators see the terminal transition.
		h.firePostHook(ctx, hooks.EventPostToolUseFailure, call, "denied", reason, "", start)
		return resp, nil
	}

	if result.UpdatedInput != "" {
		call.Input = result.UpdatedInput
	}

	// An explicit allow from a hook pre-approves the permission prompt for
	// this tool call. Deny is already handled above; silence falls through
	// to the normal permission flow.
	if result.Decision == hooks.DecisionAllow {
		ctx = permission.WithHookApproval(ctx, call.ID)
	}

	resp, runErr := h.inner.Run(ctx, call)

	if runErr != nil {
		h.firePostHook(ctx, hooks.EventPostToolUseFailure, call, "error", runErr.Error(), "", start)
		return resp, runErr
	}

	if result.Context != "" {
		if resp.Content != "" {
			resp.Content += "\n"
		}
		resp.Content += result.Context
	}

	resp.Metadata = mergeHookMetadata(resp.Metadata, result)
	h.firePostHook(ctx, hooks.EventPostToolUse, call, "success", "", resp.Content, start)
	return resp, nil
}

// firePostHook dispatches a PostToolUse or PostToolUseFailure event if
// the coordinator wired a postFire callback. The outcome rides the
// shared status/error fields (status, error, CRUSH_STATUS) — the same
// contract Stop and ModelRequestStop use — so a consumer reads one
// field name across every outcome-bearing event. tool_output is only
// set for successful calls.
func (h *hookedTool) firePostHook(ctx context.Context, eventName string, call fantasy.ToolCall, status, errMsg, output string, start time.Time) {
	if h.postFire == nil {
		return
	}
	opts := hooks.PayloadOpts{
		Status:     status,
		Error:      errMsg,
		DurationMs: time.Since(start).Milliseconds(),
	}
	if output != "" {
		opts.ToolOutput = output
	}
	h.postFire(ctx, eventName, call.Name, call.Input, opts)
}

// buildHookMetadata creates a HookMetadata from an AggregateResult.
func buildHookMetadata(result hooks.AggregateResult) hooks.HookMetadata {
	return hooks.HookMetadata{
		HookCount:    result.HookCount,
		Decision:     result.Decision.String(),
		Halt:         result.Halt,
		Reason:       result.Reason,
		InputRewrite: result.UpdatedInput != "",
		Hooks:        result.Hooks,
	}
}

// hookMetadataJSON builds a JSON string containing only the hook metadata.
func hookMetadataJSON(result hooks.AggregateResult) string {
	meta := buildHookMetadata(result)
	data, err := json.Marshal(meta)
	if err != nil {
		return ""
	}
	return `{"hook":` + string(data) + `}`
}

// mergeHookMetadata injects hook metadata into existing tool metadata.
func mergeHookMetadata(existing string, result hooks.AggregateResult) string {
	if result.HookCount == 0 {
		return existing
	}
	meta := buildHookMetadata(result)
	data, err := json.Marshal(meta)
	if err != nil {
		return existing
	}
	if existing == "" {
		existing = "{}"
	}
	merged, err := sjson.SetRaw(existing, "hook", string(data))
	if err != nil {
		return existing
	}
	return merged
}
