package openai

import (
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
)

func TestStaticHeaders_WithValidAccountID(t *testing.T) {
	jwt := makeJWT(t, map[string]any{
		authClaimKey: map[string]string{"chatgpt_account_id": "acct-static"},
	})
	tok := &oauth.Token{AccessToken: jwt}

	h := StaticHeaders(tok)

	if h["originator"] != DefaultOriginator {
		t.Errorf("originator = %q, want %q", h["originator"], DefaultOriginator)
	}
	if h["Accept"] != "text/event-stream" {
		t.Errorf("Accept = %q, want text/event-stream", h["Accept"])
	}
	if h["ChatGPT-Account-ID"] != "acct-static" {
		t.Errorf("ChatGPT-Account-ID = %q, want acct-static", h["ChatGPT-Account-ID"])
	}
}

// When the access token is not a parseable JWT, StaticHeaders omits the
// ChatGPT-Account-ID header rather than emitting a broken one — the
// backend then surfaces a clear error instead of crush masking it.
func TestStaticHeaders_OmitsAccountIDOnBadToken(t *testing.T) {
	tok := &oauth.Token{AccessToken: "not-a-jwt"}

	h := StaticHeaders(tok)

	if _, ok := h["ChatGPT-Account-ID"]; ok {
		t.Errorf("ChatGPT-Account-ID should be absent for unparseable token, got %q", h["ChatGPT-Account-ID"])
	}
	// The static, token-independent headers are still present.
	if h["originator"] != DefaultOriginator {
		t.Errorf("originator = %q, want %q", h["originator"], DefaultOriginator)
	}
	if h["Accept"] != "text/event-stream" {
		t.Errorf("Accept = %q, want text/event-stream", h["Accept"])
	}
}

func TestHeaders_FullSet(t *testing.T) {
	jwt := makeJWT(t, map[string]any{
		authClaimKey: map[string]string{"chatgpt_account_id": "acct-full"},
	})
	tok := &oauth.Token{AccessToken: jwt}

	h := Headers(tok, "sess-123", "thread-456")

	if got, want := h["Authorization"], "Bearer "+jwt; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
	if h["session-id"] != "sess-123" {
		t.Errorf("session-id = %q, want sess-123", h["session-id"])
	}
	if h["thread-id"] != "thread-456" {
		t.Errorf("thread-id = %q, want thread-456", h["thread-id"])
	}
	// x-client-request-id mirrors thread-id (see Headers doc / RoundTrip).
	if h["x-client-request-id"] != "thread-456" {
		t.Errorf("x-client-request-id = %q, want thread-456 (mirror of thread-id)", h["x-client-request-id"])
	}
	// Inherited from StaticHeaders.
	if h["ChatGPT-Account-ID"] != "acct-full" {
		t.Errorf("ChatGPT-Account-ID = %q, want acct-full", h["ChatGPT-Account-ID"])
	}
	if h["originator"] != DefaultOriginator {
		t.Errorf("originator = %q, want %q", h["originator"], DefaultOriginator)
	}
	if h["Accept"] != "text/event-stream" {
		t.Errorf("Accept = %q, want text/event-stream", h["Accept"])
	}
	// User-Agent is set per-call and embeds the codex_cli_rs version.
	if !strings.Contains(h["User-Agent"], "codex_cli_rs/") {
		t.Errorf("User-Agent = %q, want codex_cli_rs/ prefix", h["User-Agent"])
	}
}

func TestUserAgent_EmbedsVersionAndPlatform(t *testing.T) {
	// Force a known version so the assertion is deterministic.
	t.Setenv("CRUSH_CODEX_CLI_VERSION", "1.2.3-uatest")

	ua := UserAgent()

	if !strings.Contains(ua, "codex_cli_rs/1.2.3-uatest") {
		t.Errorf("UserAgent() = %q, want codex_cli_rs/1.2.3-uatest", ua)
	}
	if !strings.Contains(ua, runtime.GOOS) {
		t.Errorf("UserAgent() = %q, want GOOS %q", ua, runtime.GOOS)
	}
	if !strings.Contains(ua, runtime.GOARCH) {
		t.Errorf("UserAgent() = %q, want GOARCH %q", ua, runtime.GOARCH)
	}
	if !strings.Contains(ua, "crush/") {
		t.Errorf("UserAgent() = %q, want crush/ segment", ua)
	}
}
