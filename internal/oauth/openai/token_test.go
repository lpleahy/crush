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
