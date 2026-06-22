package openai

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/oauth"
)

// codexAuth mirrors the on-disk shape written by the Codex CLI to
// auth.json. Only the fields we consume are modeled; unknown keys are
// ignored by encoding/json.
type codexAuth struct {
	Tokens struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id"`
	} `json:"tokens"`
}

// RefreshTokenFromDisk looks for an existing Codex CLI credential and,
// if present and usable, returns it as an *oauth.Token. It mirrors the
// Copilot disk-import flow: a miss (no file, unreadable file, malformed
// JSON, or empty access/refresh token) returns (nil, false) without an
// error so callers can fall through to the interactive login.
//
// The credential file is $CODEX_HOME/auth.json when CODEX_HOME is set,
// otherwise ~/.codex/auth.json. The returned token has no expiry
// information (Codex does not persist expires_in/expires_at), so
// SetExpiresAt marks it expired, prompting the caller to refresh it
// before use — which doubles as validation.
func RefreshTokenFromDisk() (*oauth.Token, bool) {
	path := codexAuthPath()
	if path == "" {
		return nil, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var content codexAuth
	if err := json.Unmarshal(data, &content); err != nil {
		return nil, false
	}

	if content.Tokens.AccessToken == "" || content.Tokens.RefreshToken == "" {
		return nil, false
	}

	token := &oauth.Token{
		AccessToken:  content.Tokens.AccessToken,
		RefreshToken: content.Tokens.RefreshToken,
	}
	// Codex does not store an expiry; SetExpiresAt will mark the token as
	// immediately expired, which is what we want — the caller refreshes
	// to obtain a fresh access token and thereby validates the import.
	token.SetExpiresAt()
	return token, true
}

// codexAuthPath resolves the location of the Codex auth.json file.
// CODEX_HOME takes precedence; otherwise it falls back to ~/.codex. An
// empty string is returned only when no home directory can be
// determined and CODEX_HOME is unset.
func codexAuthPath() string {
	if home := os.Getenv("CODEX_HOME"); home != "" {
		return filepath.Join(home, "auth.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".codex", "auth.json")
}
