package hooks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/tidwall/gjson"
)

// SupportedOutputVersion is the highest envelope version this build
// understands. Hooks may omit `version` entirely (treated as 1) or pin
// an older version. Unknown higher versions are still parsed but logged.
const SupportedOutputVersion = 1

// maxEnvTextBytes bounds the size of the CRUSH_ASSISTANT_MESSAGE_TEXT
// env var (~4KB). The full message text is always available untruncated
// in the JSON payload (message_text); this cap only protects the
// environment block from oversized assistant turns.
const maxEnvTextBytes = 4 * 1024

// truncateEnvText clamps s to maxEnvTextBytes on a UTF-8 rune boundary so
// the env var never carries a partial multi-byte sequence. Truncated
// output is marked with a trailing ellipsis.
func truncateEnvText(s string) string {
	if len(s) <= maxEnvTextBytes {
		return s
	}
	cut := maxEnvTextBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// Payload is the JSON structure piped to hook commands via stdin.
// ToolInput is emitted as a parsed JSON object for compatibility with
// Claude Code hooks (which expect tool_input to be an object, not a
// string). All non-tool fields are omitted from the JSON when empty so
// payloads stay compact for events that don't carry them.
type Payload struct {
	Event             string          `json:"event"`
	SessionID         string          `json:"session_id"`
	TurnID            string          `json:"turn_id,omitempty"`
	CWD               string          `json:"cwd"`
	ProjectDir        string          `json:"project_dir,omitempty"`
	ToolName          string          `json:"tool_name,omitempty"`
	ToolInput         json.RawMessage `json:"tool_input,omitempty"`
	ToolOutput        string          `json:"tool_output,omitempty"`
	DurationMs        int64           `json:"duration_ms,omitempty"`
	Provider          string          `json:"provider,omitempty"`
	Model             string          `json:"model,omitempty"`
	PermissionKind    string          `json:"permission_kind,omitempty"`
	PermissionMessage string          `json:"permission_message,omitempty"`
	MessageKind       string          `json:"message_kind,omitempty"`
	Status            string          `json:"status,omitempty"`
	Error             string          `json:"error,omitempty"`
	Reason            string          `json:"reason,omitempty"`
	// Assistant-message fields, carried by the AssistantMessage event,
	// which fires once per completed assistant turn. MessageText holds
	// the full assistant text (untruncated in the JSON payload);
	// CRUSH_ASSISTANT_MESSAGE_TEXT is truncated for the env var.
	// MessageTokenCount is a best-effort output-token estimate, and
	// FinishReason is the model's finish reason for the turn.
	MessageText       string `json:"message_text,omitempty"`
	MessageTokenCount int64  `json:"message_token_count,omitempty"`
	FinishReason      string `json:"finish_reason,omitempty"`
}

// PayloadOpts holds optional event-specific fields. All fields are
// optional; only the ones relevant to a given event need to be set.
type PayloadOpts struct {
	TurnID            string
	ProjectDir        string
	ToolOutput        string
	DurationMs        int64
	Provider          string
	Model             string
	PermissionKind    string
	PermissionMessage string
	MessageKind       string
	// Status/Error carry the outcome for every outcome-bearing event
	// (Stop, ModelRequestStop, PostToolUse, PostToolUseFailure). Status
	// also surfaces as CRUSH_STATUS.
	Status string
	Error  string
	Reason string
	// Assistant-message fields populate the AssistantMessage event.
	// MessageText is the full assistant text (the JSON payload keeps it
	// intact; the CRUSH_ASSISTANT_MESSAGE_TEXT env var is truncated).
	// MessageTokenCount is a best-effort output-token estimate and
	// FinishReason is the model's finish reason for the completed turn.
	MessageText       string
	MessageTokenCount int64
	FinishReason      string
}

// BuildPayload constructs the JSON stdin payload for a tool-shaped
// event (currently PreToolUse). For richer events use BuildPayloadOpts.
func BuildPayload(eventName, sessionID, cwd, toolName, toolInputJSON string) []byte {
	return BuildPayloadOpts(eventName, sessionID, cwd, toolName, toolInputJSON, PayloadOpts{})
}

// BuildPayloadOpts is BuildPayload with optional fields for lifecycle
// events that carry provider/model/permission/error context.
func BuildPayloadOpts(eventName, sessionID, cwd, toolName, toolInputJSON string, opts PayloadOpts) []byte {
	p := Payload{
		Event:             eventName,
		SessionID:         sessionID,
		TurnID:            opts.TurnID,
		CWD:               cwd,
		ProjectDir:        opts.ProjectDir,
		ToolName:          toolName,
		ToolOutput:        opts.ToolOutput,
		DurationMs:        opts.DurationMs,
		Provider:          opts.Provider,
		Model:             opts.Model,
		PermissionKind:    opts.PermissionKind,
		PermissionMessage: opts.PermissionMessage,
		MessageKind:       opts.MessageKind,
		Status:            opts.Status,
		Error:             opts.Error,
		Reason:            opts.Reason,
		MessageText:       opts.MessageText,
		MessageTokenCount: opts.MessageTokenCount,
		FinishReason:      opts.FinishReason,
	}
	if toolInputJSON != "" {
		raw := json.RawMessage(toolInputJSON)
		if !json.Valid(raw) {
			raw = json.RawMessage("{}")
		}
		p.ToolInput = raw
	}
	data, err := json.Marshal(p)
	if err != nil {
		return []byte("{}")
	}
	return data
}

// BuildEnv constructs the environment variable slice for a hook command.
// It includes all current process env vars plus hook-specific ones.
func BuildEnv(eventName, toolName, sessionID, cwd, projectDir, toolInputJSON string) []string {
	return BuildEnvOpts(eventName, toolName, sessionID, cwd, projectDir, toolInputJSON, PayloadOpts{})
}

// BuildEnvOpts is BuildEnv with optional fields for lifecycle events.
func BuildEnvOpts(eventName, toolName, sessionID, cwd, projectDir, toolInputJSON string, opts PayloadOpts) []string {
	env := os.Environ()
	env = append(env, shell.CrushEnvMarkers()...)
	env = append(
		env,
		fmt.Sprintf("CRUSH_EVENT=%s", eventName),
		fmt.Sprintf("CRUSH_SESSION_ID=%s", sessionID),
		fmt.Sprintf("CRUSH_CWD=%s", cwd),
		fmt.Sprintf("CRUSH_PROJECT_DIR=%s", projectDir),
	)
	if toolName != "" {
		env = append(env, fmt.Sprintf("CRUSH_TOOL_NAME=%s", toolName))
	}
	if opts.TurnID != "" {
		env = append(env, fmt.Sprintf("CRUSH_TURN_ID=%s", opts.TurnID))
	}
	if opts.Provider != "" {
		env = append(env, fmt.Sprintf("CRUSH_PROVIDER=%s", opts.Provider))
	}
	if opts.Model != "" {
		env = append(env, fmt.Sprintf("CRUSH_MODEL=%s", opts.Model))
	}
	if opts.PermissionKind != "" {
		env = append(env, fmt.Sprintf("CRUSH_PERMISSION_KIND=%s", opts.PermissionKind))
	}
	if opts.PermissionMessage != "" {
		env = append(env, fmt.Sprintf("CRUSH_PERMISSION_MESSAGE=%s", opts.PermissionMessage))
	}
	if opts.Status != "" {
		env = append(env, fmt.Sprintf("CRUSH_STATUS=%s", opts.Status))
	}
	if opts.MessageKind != "" {
		env = append(env, fmt.Sprintf("CRUSH_MESSAGE_KIND=%s", opts.MessageKind))
	}
	if opts.FinishReason != "" {
		env = append(env, fmt.Sprintf("CRUSH_ASSISTANT_MESSAGE_FINISH_REASON=%s", opts.FinishReason))
	}
	if opts.MessageTokenCount != 0 {
		env = append(env, fmt.Sprintf("CRUSH_ASSISTANT_MESSAGE_TOKEN_COUNT=%d", opts.MessageTokenCount))
	}
	if opts.MessageText != "" {
		// The env var is truncated to keep argv/env size bounded — a
		// long assistant message could otherwise blow past OS env limits.
		// The full text is preserved in the JSON payload's message_text.
		env = append(env, fmt.Sprintf("CRUSH_ASSISTANT_MESSAGE_TEXT=%s", truncateEnvText(opts.MessageText)))
	}

	// Extract tool-specific env vars from the JSON input.
	if toolInputJSON != "" {
		if cmd := gjson.Get(toolInputJSON, "command"); cmd.Exists() {
			env = append(env, fmt.Sprintf("CRUSH_TOOL_INPUT_COMMAND=%s", cmd.String()))
		}
		if fp := gjson.Get(toolInputJSON, "file_path"); fp.Exists() {
			env = append(env, fmt.Sprintf("CRUSH_TOOL_INPUT_FILE_PATH=%s", fp.String()))
		}
	}

	return env
}

// parseStdout parses the JSON output from a hook command's stdout.
// Supports both Crush format and Claude Code format (hookSpecificOutput).
func parseStdout(stdout string) HookResult {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return HookResult{Decision: DecisionNone}
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return HookResult{Decision: DecisionNone}
	}

	// Claude Code compat: if hookSpecificOutput is present, parse that.
	if hso, ok := raw["hookSpecificOutput"]; ok {
		return parseClaudeCodeOutput(hso)
	}

	var parsed struct {
		Version      int             `json:"version"`
		Decision     string          `json:"decision"`
		Halt         bool            `json:"halt"`
		Reason       string          `json:"reason"`
		Context      json.RawMessage `json:"context"`
		UpdatedInput json.RawMessage `json:"updated_input"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		return HookResult{Decision: DecisionNone}
	}

	if parsed.Version > SupportedOutputVersion {
		slog.Debug(
			"Hook output declared a newer envelope version than this build supports",
			"version", parsed.Version,
			"supported", SupportedOutputVersion,
		)
	}

	result := HookResult{
		Halt:    parsed.Halt,
		Reason:  parsed.Reason,
		Context: parseContext(parsed.Context),
	}
	result.Decision = parseDecision(parsed.Decision)
	result.UpdatedInput = rawToString(parsed.UpdatedInput)
	return result
}

// parseContext accepts either a single string or an array of strings and
// returns a newline-joined value with empty entries dropped.
func parseContext(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// String form.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return ""
	}
	// Array form.
	if raw[0] == '[' {
		var items []string
		if err := json.Unmarshal(raw, &items); err != nil {
			return ""
		}
		out := items[:0]
		for _, s := range items {
			if s != "" {
				out = append(out, s)
			}
		}
		return strings.Join(out, "\n")
	}
	return ""
}

// parseClaudeCodeOutput handles the Claude Code hook output format:
// {"hookSpecificOutput": {"permissionDecision": "allow", ...}}
func parseClaudeCodeOutput(data json.RawMessage) HookResult {
	var hso struct {
		PermissionDecision       string          `json:"permissionDecision"`
		PermissionDecisionReason string          `json:"permissionDecisionReason"`
		UpdatedInput             json.RawMessage `json:"updatedInput"`
		AdditionalContext        string          `json:"additionalContext"`
	}
	if err := json.Unmarshal(data, &hso); err != nil {
		return HookResult{Decision: DecisionNone}
	}

	result := HookResult{
		Decision: parseDecision(hso.PermissionDecision),
		Reason:   hso.PermissionDecisionReason,
		Context:  hso.AdditionalContext,
	}

	// Marshal updatedInput back to a string for our opaque format.
	if len(hso.UpdatedInput) > 0 && string(hso.UpdatedInput) != "null" {
		result.UpdatedInput = string(hso.UpdatedInput)
	}

	return result
}

// rawToString converts a json.RawMessage to a string suitable for use
// as opaque tool input. It accepts both a JSON object (nested) and a
// JSON string (stringified, for backward compatibility).
func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// If it's a JSON string, unwrap it.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	// Otherwise it's an object/array — use as-is.
	return string(raw)
}

func parseDecision(s string) Decision {
	switch strings.ToLower(s) {
	case "allow":
		return DecisionAllow
	case "deny":
		return DecisionDeny
	default:
		return DecisionNone
	}
}
