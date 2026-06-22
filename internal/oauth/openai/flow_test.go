package openai

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestBuildAuthorizeURL(t *testing.T) {
	c := NewClient()
	raw := c.buildAuthorizeURL("CHALLENGE", "STATE")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()

	want := map[string]string{
		"response_type":              "code",
		"client_id":                  DefaultClientID,
		"redirect_uri":               DefaultRedirectURI,
		"scope":                      DefaultScope,
		"code_challenge":             "CHALLENGE",
		"code_challenge_method":      "S256",
		"id_token_add_organizations": "true",
		"codex_cli_simplified_flow":  "true",
		"originator":                 DefaultOriginator,
		"state":                      "STATE",
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Errorf("query[%s] = %q, want %q", k, got, v)
		}
	}
}

// browserFollower simulates the user's browser: it GETs the authorize
// URL, then follows the 302 to the loopback callback.
type browserFollower struct{}

func (browserFollower) Open(authURL string) error {
	noFollow := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := noFollow.Get(authURL)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		return nil
	}
	location := resp.Header.Get("Location")
	if location == "" {
		return nil
	}
	resp2, err := http.Get(location)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()
	return nil
}

func TestAuthorize_LoopbackRoundtrip(t *testing.T) {
	// Reserve a free port, then release it so Authorize can bind.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().(*net.TCPAddr)
	_ = l.Close()

	c := NewClient()
	c.RedirectURI = (&url.URL{Scheme: "http", Host: addr.String(), Path: "/auth/callback"}).String()

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		target := c.RedirectURI + "?code=test-code&state=" + url.QueryEscape(state)
		http.Redirect(w, r, target, http.StatusFound)
	}))
	defer authSrv.Close()
	c.AuthorizeURL = authSrv.URL + "/oauth/authorize"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.Authorize(ctx, AuthorizeOptions{Browser: browserFollower{}})
	if err != nil {
		t.Fatalf("Authorize() error: %v", err)
	}
	if result.Code != "test-code" {
		t.Errorf("Code = %q, want test-code", result.Code)
	}
	if result.Verifier == "" {
		t.Error("Verifier is empty")
	}
}

func TestAuthorize_StateMismatch(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().(*net.TCPAddr)
	_ = l.Close()

	c := NewClient()
	c.RedirectURI = (&url.URL{Scheme: "http", Host: addr.String(), Path: "/auth/callback"}).String()

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Replay a forged state instead of echoing back.
		target := c.RedirectURI + "?code=test-code&state=forged"
		http.Redirect(w, r, target, http.StatusFound)
	}))
	defer authSrv.Close()
	c.AuthorizeURL = authSrv.URL + "/oauth/authorize"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = c.Authorize(ctx, AuthorizeOptions{Browser: browserFollower{}})
	if err == nil {
		t.Fatal("expected state mismatch error, got nil")
	}
}

func TestAuthorize_ContextCancelled(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().(*net.TCPAddr)
	_ = l.Close()

	c := NewClient()
	c.RedirectURI = (&url.URL{Scheme: "http", Host: addr.String(), Path: "/auth/callback"}).String()
	c.AuthorizeURL = "http://example.invalid/oauth/authorize"

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = c.Authorize(ctx, AuthorizeOptions{})
	if err == nil {
		t.Fatal("expected context.Canceled, got nil")
	}
}
