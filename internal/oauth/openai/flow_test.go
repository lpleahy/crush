package openai

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

// callbackPoster is a BrowserOpener that ignores the authorize URL and
// instead issues a single GET straight at the loopback callback with a
// caller-supplied query string. It lets tests drive the callback
// handler's error branches directly.
type callbackPoster struct {
	callbackBase string // e.g. http://127.0.0.1:PORT/auth/callback
	query        string // raw query, without leading '?'
}

func (p callbackPoster) Open(string) error {
	resp, err := http.Get(p.callbackBase + "?" + p.query)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

// reserveLoopback grabs a free 127.0.0.1 port and returns a Client wired
// to use it as the redirect, plus the callback base URL. The port is
// released so Authorize can bind it.
func reserveLoopback(t *testing.T) (*Client, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().(*net.TCPAddr)
	_ = l.Close()
	c := NewClient()
	c.RedirectURI = (&url.URL{Scheme: "http", Host: addr.String(), Path: "/auth/callback"}).String()
	c.AuthorizeURL = "http://example.invalid/oauth/authorize"
	return c, c.RedirectURI
}

func TestAuthorize_OnReadyCalled(t *testing.T) {
	c, callbackBase := reserveLoopback(t)

	var readyURL string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Drive a valid callback once OnReady hands us the state-bearing URL.
	br := stateEchoBrowser{callbackBase: callbackBase}
	_, err := c.Authorize(ctx, AuthorizeOptions{
		Browser: br,
		OnReady: func(u string) { readyURL = u },
	})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if readyURL == "" {
		t.Fatal("OnReady was not called with the authorize URL")
	}
	if !strings.Contains(readyURL, "code_challenge=") {
		t.Errorf("OnReady URL missing code_challenge: %q", readyURL)
	}
}

// stateEchoBrowser parses the state out of the authorize URL and feeds
// a valid code+state back to the callback, completing the flow.
type stateEchoBrowser struct{ callbackBase string }

func (b stateEchoBrowser) Open(authURL string) error {
	u, err := url.Parse(authURL)
	if err != nil {
		return err
	}
	state := u.Query().Get("state")
	resp, err := http.Get(b.callbackBase + "?code=ok-code&state=" + url.QueryEscape(state))
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

func TestAuthorize_ErrorParam(t *testing.T) {
	c, callbackBase := reserveLoopback(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	br := callbackPoster{callbackBase: callbackBase, query: "error=access_denied&error_description=user+said+no"}
	_, err := c.Authorize(ctx, AuthorizeOptions{Browser: br})
	if err == nil {
		t.Fatal("expected error when callback carries error param, got nil")
	}
	if !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("error should mention access_denied, got %q", err.Error())
	}
}

func TestAuthorize_MissingCode(t *testing.T) {
	c, callbackBase := reserveLoopback(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Correct state but no code → "callback missing code". We need the
	// real state, so echo it back with code omitted.
	br := stateOnlyBrowser{callbackBase: callbackBase}
	_, err := c.Authorize(ctx, AuthorizeOptions{Browser: br})
	if err == nil {
		t.Fatal("expected error when callback omits code, got nil")
	}
	if !strings.Contains(err.Error(), "missing code") {
		t.Errorf("error should mention missing code, got %q", err.Error())
	}
}

// stateOnlyBrowser returns the correct state but no code, exercising the
// "callback missing code" branch (which is reached only after the state
// check passes).
type stateOnlyBrowser struct{ callbackBase string }

func (b stateOnlyBrowser) Open(authURL string) error {
	u, err := url.Parse(authURL)
	if err != nil {
		return err
	}
	state := u.Query().Get("state")
	resp, err := http.Get(b.callbackBase + "?state=" + url.QueryEscape(state))
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

func TestAuthorize_BindError(t *testing.T) {
	// Occupy a port, then point the redirect at it so Authorize's
	// net.Listen fails — covering the bind-error return.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)

	c := NewClient()
	c.RedirectURI = (&url.URL{Scheme: "http", Host: addr.String(), Path: "/auth/callback"}).String()
	c.AuthorizeURL = "http://example.invalid/oauth/authorize"

	_, err = c.Authorize(context.Background(), AuthorizeOptions{})
	if err == nil {
		t.Fatal("expected bind error when port is already in use, got nil")
	}
	if !strings.Contains(err.Error(), "bind loopback listener") {
		t.Errorf("error should mention bind failure, got %q", err.Error())
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
