package openai

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func makeJWT(t *testing.T, body map[string]any) string {
	t.Helper()
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return strings.Join([]string{
		base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`)),
		base64.RawURLEncoding.EncodeToString(bodyJSON),
		base64.RawURLEncoding.EncodeToString([]byte("signature")),
	}, ".")
}

func TestChatGPTAccountID_Success(t *testing.T) {
	jwt := makeJWT(t, map[string]any{
		authClaimKey: map[string]string{"chatgpt_account_id": "acct-test-123"},
	})
	got, err := ChatGPTAccountID(jwt)
	if err != nil {
		t.Fatalf("ChatGPTAccountID() error: %v", err)
	}
	if got != "acct-test-123" {
		t.Errorf("got %q, want acct-test-123", got)
	}
}

func TestChatGPTAccountID_NotAJWT(t *testing.T) {
	if _, err := ChatGPTAccountID("not.a"); err == nil {
		t.Fatal("expected error for non-JWT input, got nil")
	}
}

func TestChatGPTAccountID_MissingClaim(t *testing.T) {
	jwt := makeJWT(t, map[string]any{"sub": "user"})
	if _, err := ChatGPTAccountID(jwt); err == nil {
		t.Fatal("expected error when auth claim absent, got nil")
	}
}

func TestChatGPTAccountID_EmptyAccountID(t *testing.T) {
	jwt := makeJWT(t, map[string]any{
		authClaimKey: map[string]string{"chatgpt_account_id": ""},
	})
	if _, err := ChatGPTAccountID(jwt); err == nil {
		t.Fatal("expected error for empty account id, got nil")
	}
}

func TestChatGPTAccountID_MalformedBase64(t *testing.T) {
	if _, err := ChatGPTAccountID("header.!!!.signature"); err == nil {
		t.Fatal("expected error for malformed base64, got nil")
	}
}
