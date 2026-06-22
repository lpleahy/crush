package openai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRefreshTokenFromDisk(t *testing.T) {
	const validAuth = `{
		"auth_mode": "chatgpt",
		"OPENAI_API_KEY": "sk-test",
		"last_refresh": "2026-06-14T00:00:00Z",
		"tokens": {
			"id_token": "id-tok",
			"access_token": "access-tok",
			"refresh_token": "refresh-tok",
			"account_id": "acct-123"
		}
	}`

	tests := []struct {
		name string
		// content is written to auth.json. When writeFile is false the
		// file is not created, exercising the missing-file path.
		content     string
		writeFile   bool
		wantOK      bool
		wantAccess  string
		wantRefresh string
	}{
		{
			name:        "valid file",
			content:     validAuth,
			writeFile:   true,
			wantOK:      true,
			wantAccess:  "access-tok",
			wantRefresh: "refresh-tok",
		},
		{
			name:      "missing tokens object",
			content:   `{"auth_mode":"chatgpt","OPENAI_API_KEY":"sk-test"}`,
			writeFile: true,
			wantOK:    false,
		},
		{
			name:      "malformed JSON",
			content:   `{"tokens": {`,
			writeFile: true,
			wantOK:    false,
		},
		{
			name:      "missing file",
			writeFile: false,
			wantOK:    false,
		},
		{
			name:      "empty tokens",
			content:   `{"tokens": {"access_token": "", "refresh_token": ""}}`,
			writeFile: true,
			wantOK:    false,
		},
		{
			name:      "access token present but refresh empty",
			content:   `{"tokens": {"access_token": "access-tok", "refresh_token": ""}}`,
			writeFile: true,
			wantOK:    false,
		},
		{
			name:      "refresh token present but access empty",
			content:   `{"tokens": {"access_token": "", "refresh_token": "refresh-tok"}}`,
			writeFile: true,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.writeFile {
				if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(tt.content), 0o600); err != nil {
					t.Fatalf("write auth.json: %v", err)
				}
			}
			t.Setenv("CODEX_HOME", dir)

			token, ok := RefreshTokenFromDisk()
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				if token != nil {
					t.Fatalf("token = %+v, want nil on miss", token)
				}
				return
			}
			if token == nil {
				t.Fatal("token is nil despite ok = true")
			}
			if token.AccessToken != tt.wantAccess {
				t.Errorf("AccessToken = %q, want %q", token.AccessToken, tt.wantAccess)
			}
			if token.RefreshToken != tt.wantRefresh {
				t.Errorf("RefreshToken = %q, want %q", token.RefreshToken, tt.wantRefresh)
			}
			// Codex stores no expiry, so the token must be treated as
			// expired to force a validating refresh.
			if !token.IsExpired() {
				t.Errorf("imported token should be expired (no stored expiry), but IsExpired() = false")
			}
		})
	}
}

// TestRefreshTokenFromDisk_HomeFallback verifies the ~/.codex/auth.json
// fallback path when CODEX_HOME is unset, by pointing HOME at a temp dir.
func TestRefreshTokenFromDisk_HomeFallback(t *testing.T) {
	dir := t.TempDir()
	codexDir := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatalf("mkdir .codex: %v", err)
	}
	content := `{"tokens": {"access_token": "home-access", "refresh_token": "home-refresh", "account_id": "a"}}`
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}

	t.Setenv("CODEX_HOME", "")
	t.Setenv("HOME", dir)
	// os.UserHomeDir consults USERPROFILE on Windows; keep it aligned so
	// the test is portable.
	t.Setenv("USERPROFILE", dir)

	token, ok := RefreshTokenFromDisk()
	if !ok {
		t.Fatal("expected ok = true from ~/.codex/auth.json fallback")
	}
	if token.AccessToken != "home-access" {
		t.Errorf("AccessToken = %q, want home-access", token.AccessToken)
	}
	if token.RefreshToken != "home-refresh" {
		t.Errorf("RefreshToken = %q, want home-refresh", token.RefreshToken)
	}
}
