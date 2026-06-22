// Package openai implements the ChatGPT-account OAuth flow used by
// crush to sign users in with their ChatGPT subscription, modeled after
// OpenAI's official Codex CLI. The flow is PKCE + loopback (port 1455);
// auth.openai.com does not support device code.
//
// After Authorize completes, TokenExchange mints access + refresh
// tokens. Downstream requests to chatgpt.com/backend-api/codex use
// Headers() to attach Authorization, ChatGPT-Account-ID (parsed from
// the JWT), originator, session-id, and thread-id. AdaptRequestBody
// reshapes a standard Responses API body to what the ChatGPT backend
// expects (store=false, encrypted reasoning replay, prompt cache key).
package openai

import "os"

const (
	DefaultAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	DefaultTokenURL     = "https://auth.openai.com/oauth/token"
	DefaultRevokeURL    = "https://auth.openai.com/oauth/revoke"

	DefaultClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	DefaultRedirectURI = "http://localhost:1455/auth/callback"
	DefaultScope       = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	DefaultOriginator  = "codex_cli_rs"

	DefaultRedirectPort = 1455

	// DefaultCodexCliVersion is the codex_cli_rs version we mimic in
	// the User-Agent and the client_version query param on /models.
	// The backend gates each model on minimal_client_version — if our
	// tag drifts below the highest live model's minimum, that model
	// silently disappears from /models and inference returns "not
	// supported". Override at runtime via CRUSH_CODEX_CLI_VERSION.
	DefaultCodexCliVersion = "0.130.0"

	// Device flow URLs (non-RFC-8628; Codex's own shape).
	// Verification URL is where the user visits in their browser to
	// enter the code; OpenAI renders the styled "Signed in to Codex"
	// page on success. Redirect URI is what gets sent in the final
	// token exchange (the auth server already redirected there
	// internally — we just have to match what was bound to the code).
	DefaultDeviceUserCodeURL     = "https://auth.openai.com/api/accounts/deviceauth/usercode"
	DefaultDeviceTokenURL        = "https://auth.openai.com/api/accounts/deviceauth/token"
	DefaultDeviceVerificationURL = "https://auth.openai.com/codex/device"
	DefaultDeviceRedirectURI     = "https://auth.openai.com/deviceauth/callback"
)

// CodexCliVersion returns the codex_cli_rs version string to mimic.
// Reads CRUSH_CODEX_CLI_VERSION at call time so users can bump the
// version when a new model ships without rebuilding crush.
func CodexCliVersion() string {
	if v := os.Getenv("CRUSH_CODEX_CLI_VERSION"); v != "" {
		return v
	}
	return DefaultCodexCliVersion
}
