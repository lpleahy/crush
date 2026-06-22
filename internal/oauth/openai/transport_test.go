package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransport_AddsRequiredHeaders(t *testing.T) {
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	body := bytes.NewReader([]byte(`{"model":"gpt-5.2-codex","input":[]}`))
	req, err := http.NewRequest("POST", srv.URL, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if captured.Header.Get("session-id") == "" {
		t.Error("session-id missing")
	}
	if captured.Header.Get("thread-id") == "" {
		t.Error("thread-id missing")
	}
	if captured.Header.Get("thread-id") != captured.Header.Get("x-client-request-id") {
		t.Errorf("thread-id (%q) should equal x-client-request-id (%q)",
			captured.Header.Get("thread-id"), captured.Header.Get("x-client-request-id"))
	}
}

func TestTransport_AdaptsJSONBody(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	body := bytes.NewReader([]byte(`{"model":"gpt-5.2-codex","input":[]}`))
	req, _ := http.NewRequest("POST", srv.URL, body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	var m map[string]any
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if m["store"] != false {
		t.Errorf("store = %v, want false", m["store"])
	}
	if _, ok := m["prompt_cache_key"]; !ok {
		t.Error("prompt_cache_key missing from adapted body")
	}
	if _, ok := m["client_metadata"]; !ok {
		t.Error("client_metadata missing from adapted body")
	}
}

func TestTransport_NonJSONPassthrough(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	req, _ := http.NewRequest("POST", srv.URL, strings.NewReader("not json"))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if string(gotBody) != "not json" {
		t.Errorf("body mutated, got %q", string(gotBody))
	}
}

func TestTransport_StableIDsAcrossRequests(t *testing.T) {
	sessionIDs := map[string]int{}
	threadIDs := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionIDs[r.Header.Get("session-id")]++
		threadIDs[r.Header.Get("thread-id")]++
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", srv.URL, nil)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		resp.Body.Close()
	}

	if len(sessionIDs) != 1 {
		t.Errorf("session-id changed across requests; saw %d distinct", len(sessionIDs))
	}
	if len(threadIDs) != 1 {
		t.Errorf("thread-id changed across requests; saw %d distinct", len(threadIDs))
	}
}

// TestTransport_FallbackIDsSharedAcrossClients verifies that
// requests without a session tag fall back to a single shared
// session/thread pair, regardless of how many transports the
// caller constructs. This is the "ad-hoc / test" path; production
// inference always tags ctx via WithSession.
func TestTransport_FallbackIDsSharedAcrossClients(t *testing.T) {
	sessionIDs := map[string]struct{}{}
	threadIDs := map[string]struct{}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionIDs[r.Header.Get("session-id")] = struct{}{}
		threadIDs[r.Header.Get("thread-id")] = struct{}{}
	}))
	defer srv.Close()

	for i := 0; i < 3; i++ {
		c := NewHTTPClient(false)
		req, _ := http.NewRequest("GET", srv.URL, nil)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		resp.Body.Close()
	}

	if len(sessionIDs) != 1 {
		t.Errorf("fallback session-id should be shared; saw %d distinct", len(sessionIDs))
	}
	if len(threadIDs) != 1 {
		t.Errorf("fallback thread-id should be shared; saw %d distinct", len(threadIDs))
	}
}

// TestTransport_DistinctIDsPerSession verifies the core
// per-conversation guarantee: two different session.IDs tagged via
// WithSession get different session-id / thread-id pairs.
func TestTransport_DistinctIDsPerSession(t *testing.T) {
	requests := []map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, map[string]string{
			"session": r.Header.Get("session-id"),
			"thread":  r.Header.Get("thread-id"),
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	for _, sid := range []string{"sess-A", "sess-B"} {
		ctx := WithSession(context.Background(), sid)
		req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		resp.Body.Close()
	}

	if len(requests) != 2 {
		t.Fatalf("want 2 requests, got %d", len(requests))
	}
	if requests[0]["session"] == requests[1]["session"] {
		t.Errorf("session-id should differ across sessions, both got %q", requests[0]["session"])
	}
	if requests[0]["thread"] == requests[1]["thread"] {
		t.Errorf("thread-id should differ across sessions, both got %q", requests[0]["thread"])
	}
}

// TestTransport_SameIDsForSameSessionAcrossClients verifies that the
// per-session map keys on session.ID across transports: if the same
// session.ID is asked about via multiple NewHTTPClient instances
// (e.g. provider rebuilds within one session — model swap, token
// refresh), they all return the same session-id/thread-id pair so
// the backend's prompt_cache_key stays stable.
//
// Note: sub-agents do NOT exercise this path. runSubAgent at
// coordinator.go:1220-1221 creates a fresh child session via
// CreateAgentToolSessionID, so each sub-agent has its own
// session.ID by design and therefore its own cache scope.
func TestTransport_SameIDsForSameSessionAcrossClients(t *testing.T) {
	requests := []map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, map[string]string{
			"session": r.Header.Get("session-id"),
			"thread":  r.Header.Get("thread-id"),
		})
	}))
	defer srv.Close()

	const sharedSession = "shared-session-xyz"
	for i := 0; i < 3; i++ {
		c := NewHTTPClient(false)
		ctx := WithSession(context.Background(), sharedSession)
		req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		resp.Body.Close()
	}

	if len(requests) != 3 {
		t.Fatalf("want 3 requests, got %d", len(requests))
	}
	sid, tid := requests[0]["session"], requests[0]["thread"]
	for i := 1; i < 3; i++ {
		if requests[i]["session"] != sid {
			t.Errorf("request %d session-id = %q, want %q", i, requests[i]["session"], sid)
		}
		if requests[i]["thread"] != tid {
			t.Errorf("request %d thread-id = %q, want %q", i, requests[i]["thread"], tid)
		}
	}
}

// TestTransport_UserAgentReReadsEnv covers reviewer feedback #3:
// CRUSH_CODEX_CLI_VERSION must take effect on the next request after
// it's set, without requiring SetupChatGPT to re-run.
func TestTransport_UserAgentReReadsEnv(t *testing.T) {
	var userAgents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgents = append(userAgents, r.Header.Get("User-Agent"))
	}))
	defer srv.Close()

	c := NewHTTPClient(false)

	// First request: env unset → DefaultCodexCliVersion.
	t.Setenv("CRUSH_CODEX_CLI_VERSION", "")
	req1, _ := http.NewRequest("GET", srv.URL, nil)
	resp1, _ := c.Do(req1)
	resp1.Body.Close()

	// Bump env mid-flight; subsequent request should reflect it.
	t.Setenv("CRUSH_CODEX_CLI_VERSION", "9.9.9-test")
	req2, _ := http.NewRequest("GET", srv.URL, nil)
	resp2, _ := c.Do(req2)
	resp2.Body.Close()

	if !strings.Contains(userAgents[0], DefaultCodexCliVersion) {
		t.Errorf("first UA should contain %q, got %q", DefaultCodexCliVersion, userAgents[0])
	}
	if !strings.Contains(userAgents[1], "9.9.9-test") {
		t.Errorf("second UA should contain 9.9.9-test, got %q", userAgents[1])
	}
}

func TestLogChatGPTBlock_PreservesBody(t *testing.T) {
	// 403 with a geo-error body — logChatGPTBlock sniffs it but must
	// leave the body readable for upstream callers.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"unsupported_country_region_territory","message":"nope"}}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Contains(body, []byte("unsupported_country_region_territory")) {
		t.Errorf("body was consumed or altered; got %q", body)
	}
}

func TestLogChatGPTBlock_NonForbidden(t *testing.T) {
	// 200 with a body containing the geo string still must not log;
	// just smoke-test that the sniff doesn't happen for non-403.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`unsupported_country_region_territory`))
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "unsupported_country_region_territory" {
		t.Errorf("body altered on 200: %q", body)
	}
}

// A JSON-typed body that is valid JSON but not a JSON object (here a
// top-level array) can't be adapted into a Codex request; RoundTrip
// must pass it through unchanged rather than error or corrupt it.
func TestTransport_JSONNonObjectPassthrough(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	const raw = `[1,2,3]`
	req, _ := http.NewRequest("POST", srv.URL, strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if string(gotBody) != raw {
		t.Errorf("non-object JSON body should pass through unchanged, got %q", string(gotBody))
	}
}

// A 403 carrying Cloudflare's cf-mitigated: challenge header takes the
// bot-mitigation branch of logChatGPTBlock, which returns early without
// touching the body. Verify the response (and its body) survive intact.
func TestLogChatGPTBlock_CloudflareChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("cf-mitigated", "challenge")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("blocked by cloudflare"))
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "blocked by cloudflare" {
		t.Errorf("body altered by cf-challenge branch, got %q", string(body))
	}
}

// A 403 with no body (http.NoBody) must not panic in logChatGPTBlock —
// it returns early once it sees there's nothing to sniff.
func TestLogChatGPTBlock_ForbiddenNoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		// no body written
	}))
	defer srv.Close()

	c := NewHTTPClient(false)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

// Smoke test for the debug transport path (NewHTTPClient(true) →
// chatgptTransport.next() returns the logging transport). Exercises
// the otherwise-uncovered debug branch end to end.
func TestTransport_DebugRoundTrip(t *testing.T) {
	var seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	c := NewHTTPClient(true)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do (debug): %v", err)
	}
	resp.Body.Close()

	if !strings.Contains(seenUA, "codex_cli_rs/") {
		t.Errorf("debug transport should still set User-Agent, got %q", seenUA)
	}
}

func TestNewUUID(t *testing.T) {
	s, err := newUUID()
	if err != nil {
		t.Fatalf("newUUID: %v", err)
	}
	if got, want := len(s), 36; got != want {
		t.Errorf("len(UUID) = %d, want %d", got, want)
	}
	if s[14] != '4' {
		t.Errorf("UUID v4 marker missing at index 14: %q", s)
	}
	s2, _ := newUUID()
	if s == s2 {
		t.Error("two UUIDs identical")
	}
}
