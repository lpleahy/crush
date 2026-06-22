package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient()
	c.TokenURL = srv.URL + "/oauth/token"
	return c
}

func TestTokenExchange_Success(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("code") != "test-code" {
			t.Errorf("code = %q", r.Form.Get("code"))
		}
		if r.Form.Get("code_verifier") != "test-verifier" {
			t.Errorf("code_verifier = %q", r.Form.Get("code_verifier"))
		}
		if r.Form.Get("client_id") != DefaultClientID {
			t.Errorf("client_id = %q", r.Form.Get("client_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "test-access",
			"refresh_token": "test-refresh",
			"id_token":      "test-id",
			"expires_in":    300,
			"token_type":    "Bearer",
		})
	})

	tok, err := c.TokenExchange(context.Background(), "test-code", "test-verifier")
	if err != nil {
		t.Fatalf("TokenExchange() error: %v", err)
	}
	if tok.AccessToken != "test-access" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
	if tok.RefreshToken != "test-refresh" {
		t.Errorf("RefreshToken = %q", tok.RefreshToken)
	}
	if tok.ExpiresIn != 300 {
		t.Errorf("ExpiresIn = %d", tok.ExpiresIn)
	}
	if tok.ExpiresAt <= time.Now().Unix() {
		t.Errorf("ExpiresAt should be in the future, got %d (now %d)", tok.ExpiresAt, time.Now().Unix())
	}
}

func TestTokenExchange_InvalidGrant(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "Authorization code expired",
		})
	})

	_, err := c.TokenExchange(context.Background(), "expired", "verifier")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error should mention invalid_grant, got %q", err.Error())
	}
}

func TestRefreshToken_PreservesRefreshIfOmitted(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		if got, want := r.Form.Get("scope"), DefaultScope; got != want {
			t.Errorf("scope = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			// refresh_token intentionally omitted
			"expires_in": 300,
			"token_type": "Bearer",
		})
	})

	tok, err := c.RefreshToken(context.Background(), "original-refresh")
	if err != nil {
		t.Fatalf("RefreshToken() error: %v", err)
	}
	if tok.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
	if tok.RefreshToken != "original-refresh" {
		t.Errorf("RefreshToken should be preserved when server omits it, got %q", tok.RefreshToken)
	}
}

func TestRefreshToken_PropagatesError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "refresh token revoked",
		})
	})

	if _, err := c.RefreshToken(context.Background(), "dead-refresh"); err == nil {
		t.Fatal("expected error to propagate from postToken, got nil")
	}
}

// Non-200 response whose body is not the standard {error,...} JSON
// envelope: postToken should fall back to embedding the raw body in
// the error rather than reporting an empty error code.
func TestPostToken_NonJSONErrorBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream exploded"))
	})

	_, err := c.TokenExchange(context.Background(), "code", "verifier")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upstream exploded") {
		t.Errorf("error should include raw body, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("error should include status code 502, got %q", err.Error())
	}
}

// A 200 response with a body that isn't valid JSON must surface a
// decode error, not silently yield a zero token.
func TestPostToken_MalformedSuccessBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{ this is not json"))
	})

	_, err := c.TokenExchange(context.Background(), "code", "verifier")
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decode token response") {
		t.Errorf("error should mention decode failure, got %q", err.Error())
	}
}

// A 200 response that omits access_token is invalid; postToken must
// reject it rather than hand back a token with an empty AccessToken.
func TestPostToken_MissingAccessToken(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"refresh_token": "rt",
			"expires_in":    300,
		})
	})

	_, err := c.TokenExchange(context.Background(), "code", "verifier")
	if err == nil {
		t.Fatal("expected error for missing access_token, got nil")
	}
	if !strings.Contains(err.Error(), "missing access_token") {
		t.Errorf("error should mention missing access_token, got %q", err.Error())
	}
}

func TestPostToken_NetworkError(t *testing.T) {
	c := NewClient()
	c.TokenURL = "http://127.0.0.1:1/oauth/token" // unreachable
	c.HTTPClient = &http.Client{Timeout: 200 * time.Millisecond}
	if _, err := c.TokenExchange(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("expected error from unreachable endpoint, got nil")
	}
}

func TestRevokeToken_Success(t *testing.T) {
	var seenToken, seenClientID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		seenToken = r.Form.Get("token")
		seenClientID = r.Form.Get("client_id")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := NewClient()
	c.RevokeURL = srv.URL + "/oauth/revoke"

	if err := c.RevokeToken(context.Background(), "rt-test"); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if seenToken != "rt-test" {
		t.Errorf("token = %q, want rt-test", seenToken)
	}
	if seenClientID != DefaultClientID {
		t.Errorf("client_id = %q, want %q", seenClientID, DefaultClientID)
	}
}

func TestRevokeToken_NetworkError(t *testing.T) {
	c := NewClient()
	c.RevokeURL = "http://127.0.0.1:1/oauth/revoke" // unreachable
	c.HTTPClient = &http.Client{Timeout: 200 * time.Millisecond}
	if err := c.RevokeToken(context.Background(), "rt"); err == nil {
		t.Fatal("expected error from unreachable revoke endpoint, got nil")
	}
}

func TestRevokeToken_ErrorBubbles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient()
	c.RevokeURL = srv.URL + "/oauth/revoke"

	err := c.RevokeToken(context.Background(), "rt-bad")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid_token") {
		t.Errorf("error should mention invalid_token, got %q", err.Error())
	}
}
