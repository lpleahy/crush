package openai

import (
	"encoding/json"
	"slices"
	"strings"
)

const defaultInstructions = "You are a helpful coding assistant."

// AdaptOptions controls AdaptRequestBody's mutations.
type AdaptOptions struct {
	// PromptCacheKey is set on the request if absent. Stable per
	// logical conversation; the backend uses it for free-tier rate
	// limiting.
	PromptCacheKey string

	// ClientName and ClientVersion populate client_metadata if absent.
	ClientName    string
	ClientVersion string
}

// rejectedFields are top-level fields the standard OpenAI Responses
// API accepts but the ChatGPT backend rejects with "Unsupported
// parameter". We drop them before sending.
var rejectedFields = []string{
	"max_output_tokens",
	"max_tokens",
	"temperature",
	"top_p",
	"frequency_penalty",
	"presence_penalty",
	"response_format",
}

// AdaptRequestBody mutates a Responses API request body so it matches
// what chatgpt.com/backend-api/codex/responses expects. Standard
// api.openai.com/v1/responses does not need these tweaks.
//
// Mutations applied:
//   - store: false (backend rejects store=true; opposite of public default).
//   - include includes "reasoning.encrypted_content" so we can replay
//     encrypted reasoning across turns without re-billing.
//   - prompt_cache_key set if absent.
//   - client_metadata set if absent.
//   - instructions extracted from system/developer messages in input[]
//     (or defaulted) so the backend's "instructions required" check
//     passes. Backend ignores system-role items inside input.
//   - fields not in Codex's request schema (max_output_tokens,
//     temperature, etc.) are stripped — the backend rejects them.
//
// Unknown fields are preserved verbatim via json.RawMessage.
func AdaptRequestBody(body []byte, opts AdaptOptions) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]json.RawMessage{}
	}

	storeRaw, _ := json.Marshal(false)
	m["store"] = storeRaw

	var includes []string
	if data, ok := m["include"]; ok {
		_ = json.Unmarshal(data, &includes)
	}
	if !slices.Contains(includes, "reasoning.encrypted_content") {
		includes = append(includes, "reasoning.encrypted_content")
	}
	if data, err := json.Marshal(includes); err == nil {
		m["include"] = data
	}

	if _, ok := m["prompt_cache_key"]; !ok && opts.PromptCacheKey != "" {
		if data, err := json.Marshal(opts.PromptCacheKey); err == nil {
			m["prompt_cache_key"] = data
		}
	}

	if _, ok := m["client_metadata"]; !ok {
		meta := map[string]string{
			"client":         opts.ClientName,
			"client_version": opts.ClientVersion,
		}
		if data, err := json.Marshal(meta); err == nil {
			m["client_metadata"] = data
		}
	}

	adaptInstructions(m)

	for _, f := range rejectedFields {
		delete(m, f)
	}

	return json.Marshal(m)
}

// adaptInstructions ensures m["instructions"] is non-empty. If the
// request already provides instructions, they're left alone. Otherwise
// any system/developer role messages in input[] are concatenated into
// instructions and removed from input. Falls back to a default if no
// system context is found.
func adaptInstructions(m map[string]json.RawMessage) {
	if data, ok := m["instructions"]; ok {
		var s string
		if json.Unmarshal(data, &s) == nil && strings.TrimSpace(s) != "" {
			return
		}
	}

	var inputItems []json.RawMessage
	if data, ok := m["input"]; ok {
		_ = json.Unmarshal(data, &inputItems)
	}

	var sysParts []string
	keptInput := make([]json.RawMessage, 0, len(inputItems))
	for _, raw := range inputItems {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			keptInput = append(keptInput, raw)
			continue
		}
		var role string
		if r, ok := item["role"]; ok {
			_ = json.Unmarshal(r, &role)
		}
		if role != "system" && role != "developer" {
			keptInput = append(keptInput, raw)
			continue
		}
		if text := extractContentText(item["content"]); text != "" {
			sysParts = append(sysParts, text)
		}
	}

	instructions := strings.Join(sysParts, "\n\n")
	if instructions == "" {
		instructions = defaultInstructions
	}
	if data, err := json.Marshal(instructions); err == nil {
		m["instructions"] = data
	}

	if len(sysParts) > 0 {
		if data, err := json.Marshal(keptInput); err == nil {
			m["input"] = data
		}
	}
}

// extractContentText turns Responses API content (which can be a plain
// string, or an array of typed parts like {"type":"input_text","text":"..."})
// into a single string. Returns "" if no text is found.
func extractContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []map[string]any
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var texts []string
	for _, p := range parts {
		if t, ok := p["text"].(string); ok && t != "" {
			texts = append(texts, t)
		}
	}
	return strings.Join(texts, "")
}
