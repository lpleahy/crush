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

// Some issuers std-base64-encode the payload with '=' padding despite
// RFC 7519 mandating base64url-no-pad. ChatGPTAccountID must fall back
// to the padded std-URL decoder and still parse the claim.
func TestChatGPTAccountID_PaddedBase64Payload(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	bodyJSON, err := json.Marshal(map[string]any{
		authClaimKey: map[string]string{"chatgpt_account_id": "acct-padded"},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	// Use the padded std-URL encoding (RawURLEncoding's padded sibling).
	paddedPayload := base64.URLEncoding.EncodeToString(bodyJSON)
	if !strings.Contains(paddedPayload, "=") {
		t.Skip("payload happened to need no padding; fixture not exercising fallback")
	}
	jwt := strings.Join([]string{header, paddedPayload, "sig"}, ".")

	got, err := ChatGPTAccountID(jwt)
	if err != nil {
		t.Fatalf("ChatGPTAccountID() error: %v", err)
	}
	if got != "acct-padded" {
		t.Errorf("got %q, want acct-padded", got)
	}
}

// The payload decodes from base64 but is not a JSON object (here: a
// bare JSON array). The map unmarshal must fail and propagate an error.
func TestChatGPTAccountID_PayloadNotJSONObject(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`[1,2,3]`))
	jwt := strings.Join([]string{header, payload, "sig"}, ".")

	if _, err := ChatGPTAccountID(jwt); err == nil {
		t.Fatal("expected error when payload is not a JSON object, got nil")
	}
}

// The auth claim is present but is the wrong JSON type (a string rather
// than the expected {chatgpt_account_id: ...} object), so unmarshalling
// it into authClaim fails.
func TestChatGPTAccountID_AuthClaimWrongType(t *testing.T) {
	jwt := makeJWT(t, map[string]any{
		authClaimKey: "should-be-an-object-not-a-string",
	})
	if _, err := ChatGPTAccountID(jwt); err == nil {
		t.Fatal("expected error when auth claim has wrong type, got nil")
	}
}
