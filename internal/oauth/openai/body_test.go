package openai

import (
	"encoding/json"
	"testing"
)

func TestAdaptRequestBody_AddsRequiredFields(t *testing.T) {
	in := []byte(`{"model":"gpt-5.2-codex","input":[]}`)
	out, err := AdaptRequestBody(in, AdaptOptions{
		PromptCacheKey: "cache-1",
		ClientName:     "crush",
		ClientVersion:  "0.1.0",
	})
	if err != nil {
		t.Fatalf("AdaptRequestBody() error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["store"] != false {
		t.Errorf("store = %v, want false", m["store"])
	}
	if m["prompt_cache_key"] != "cache-1" {
		t.Errorf("prompt_cache_key = %v, want cache-1", m["prompt_cache_key"])
	}
	incs, ok := m["include"].([]any)
	if !ok {
		t.Fatalf("include is %T, want []any", m["include"])
	}
	found := false
	for _, v := range incs {
		if v == "reasoning.encrypted_content" {
			found = true
		}
	}
	if !found {
		t.Errorf("include missing reasoning.encrypted_content: %v", incs)
	}
	cm, ok := m["client_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("client_metadata is %T, want map", m["client_metadata"])
	}
	if cm["client"] != "crush" || cm["client_version"] != "0.1.0" {
		t.Errorf("client_metadata = %v", cm)
	}
}

func TestAdaptRequestBody_OverridesStoreTrue(t *testing.T) {
	in := []byte(`{"model":"x","store":true,"input":[]}`)
	out, err := AdaptRequestBody(in, AdaptOptions{PromptCacheKey: "k", ClientName: "crush"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["store"] != false {
		t.Errorf("store should be forced false, got %v", m["store"])
	}
}

func TestAdaptRequestBody_PreservesExistingInclude(t *testing.T) {
	in := []byte(`{"include":["existing"]}`)
	out, err := AdaptRequestBody(in, AdaptOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	incs, _ := m["include"].([]any)
	if len(incs) != 2 {
		t.Errorf("include should have 2 entries, got %d: %v", len(incs), incs)
	}
}

func TestAdaptRequestBody_NoDuplicateInclude(t *testing.T) {
	in := []byte(`{"include":["reasoning.encrypted_content"]}`)
	out, err := AdaptRequestBody(in, AdaptOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	incs, _ := m["include"].([]any)
	if len(incs) != 1 {
		t.Errorf("include should not duplicate reasoning.encrypted_content, got %d: %v", len(incs), incs)
	}
}

func TestAdaptRequestBody_PreservesUnknownFields(t *testing.T) {
	in := []byte(`{"custom_field":"keep-me","model":"x"}`)
	out, err := AdaptRequestBody(in, AdaptOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["custom_field"] != "keep-me" {
		t.Errorf("custom_field lost: %v", m["custom_field"])
	}
}

func TestAdaptRequestBody_PreservesExistingPromptCacheKey(t *testing.T) {
	in := []byte(`{"prompt_cache_key":"existing"}`)
	out, err := AdaptRequestBody(in, AdaptOptions{PromptCacheKey: "new"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["prompt_cache_key"] != "existing" {
		t.Errorf("prompt_cache_key should not be overwritten, got %v", m["prompt_cache_key"])
	}
}

func TestAdaptRequestBody_InvalidJSON(t *testing.T) {
	if _, err := AdaptRequestBody([]byte(`not json`), AdaptOptions{}); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAdaptRequestBody_DefaultInstructionsWhenMissing(t *testing.T) {
	in := []byte(`{"model":"x","input":[]}`)
	out, err := AdaptRequestBody(in, AdaptOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["instructions"] != defaultInstructions {
		t.Errorf("instructions = %v, want %q", m["instructions"], defaultInstructions)
	}
}

func TestAdaptRequestBody_PreservesExplicitInstructions(t *testing.T) {
	in := []byte(`{"model":"x","instructions":"custom prompt","input":[]}`)
	out, err := AdaptRequestBody(in, AdaptOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["instructions"] != "custom prompt" {
		t.Errorf("instructions overwritten: %v", m["instructions"])
	}
}

func TestAdaptRequestBody_ExtractsSystemFromInput(t *testing.T) {
	in := []byte(`{
		"model":"x",
		"input":[
			{"role":"system","content":"you are crush"},
			{"role":"user","content":"hi"}
		]
	}`)
	out, err := AdaptRequestBody(in, AdaptOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["instructions"] != "you are crush" {
		t.Errorf("instructions = %v", m["instructions"])
	}
	input := m["input"].([]any)
	if len(input) != 1 {
		t.Fatalf("expected 1 remaining input item, got %d", len(input))
	}
	if input[0].(map[string]any)["role"] != "user" {
		t.Errorf("remaining input should be the user message, got %v", input[0])
	}
}

func TestAdaptRequestBody_ExtractsContentParts(t *testing.T) {
	in := []byte(`{
		"input":[
			{"role":"system","content":[{"type":"input_text","text":"alpha"},{"type":"input_text","text":" beta"}]},
			{"role":"user","content":"hi"}
		]
	}`)
	out, err := AdaptRequestBody(in, AdaptOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["instructions"] != "alpha beta" {
		t.Errorf("instructions = %v", m["instructions"])
	}
}

func TestAdaptRequestBody_StripsRejectedFields(t *testing.T) {
	in := []byte(`{
		"model":"gpt-5.4",
		"max_output_tokens":4096,
		"max_tokens":4096,
		"temperature":0.7,
		"top_p":1,
		"frequency_penalty":0,
		"presence_penalty":0,
		"response_format":{"type":"json_object"},
		"input":[]
	}`)
	out, err := AdaptRequestBody(in, AdaptOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	for _, f := range rejectedFields {
		if _, ok := m[f]; ok {
			t.Errorf("rejected field %q should be stripped, still present", f)
		}
	}
	if m["model"] != "gpt-5.4" {
		t.Errorf("model should be preserved, got %v", m["model"])
	}
}

func TestAdaptRequestBody_ConcatenatesMultipleSystemMessages(t *testing.T) {
	in := []byte(`{
		"input":[
			{"role":"system","content":"first"},
			{"role":"developer","content":"second"},
			{"role":"user","content":"hi"}
		]
	}`)
	out, err := AdaptRequestBody(in, AdaptOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["instructions"] != "first\n\nsecond" {
		t.Errorf("instructions = %q", m["instructions"])
	}
}
