package openai

import (
	"fmt"
	"runtime"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/version"
)

// UserAgent returns the User-Agent for ChatGPT backend requests.
// Re-reads CRUSH_CODEX_CLI_VERSION on every call so a user can bump
// the version without restarting crush.
func UserAgent() string {
	return fmt.Sprintf("codex_cli_rs/%s (%s; %s) crush/%s",
		CodexCliVersion(), runtime.GOOS, runtime.GOARCH, version.Version)
}

// Headers returns the full HTTP header set required by the ChatGPT
// backend at chatgpt.com/backend-api/codex. sessionID and threadID
// should be stable for the lifetime of a logical conversation.
//
// chatgpt_account_id is parsed from the JWT access_token. If parsing
// fails the header is omitted; the backend then returns a clear error
// rather than us masking it.
//
// Most callers should use StaticHeaders + the chatgpt transport
// instead — this function exists primarily for tests and standalone
// integrations.
func Headers(token *oauth.Token, sessionID, threadID string) map[string]string {
	h := StaticHeaders(token)
	h["Authorization"] = "Bearer " + token.AccessToken
	h["User-Agent"] = UserAgent()
	h["session-id"] = sessionID
	h["thread-id"] = threadID
	h["x-client-request-id"] = threadID
	return h
}

// StaticHeaders returns the subset of ChatGPT backend headers that
// don't depend on per-request state. Use these in
// ProviderConfig.ExtraHeaders, refreshed each time the OAuth token
// rotates (since ChatGPT-Account-ID is derived from the JWT).
//
// User-Agent is set per-request by the chatgpt transport
// (NewHTTPClient) so CRUSH_CODEX_CLI_VERSION takes effect without
// re-running SetupChatGPT. session-id, thread-id, and
// x-client-request-id are also set per-request, derived from the
// session tagged on the request context via WithSession.
// Authorization is supplied by the openai-compat path from APIKey.
func StaticHeaders(token *oauth.Token) map[string]string {
	h := map[string]string{
		"originator": DefaultOriginator,
		"Accept":     "text/event-stream",
	}
	if id, err := ChatGPTAccountID(token.AccessToken); err == nil {
		h["ChatGPT-Account-ID"] = id
	}
	return h
}
