package openai

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

const authClaimKey = "https://api.openai.com/auth"

type authClaim struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
}

// ChatGPTAccountID extracts chatgpt_account_id from the JWT access
// token's "https://api.openai.com/auth" claim. The signature is not
// verified — OpenAI signs the token and we received it from
// auth.openai.com over TLS, so we trust it.
func ChatGPTAccountID(accessToken string) (string, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return "", errors.New("access token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some issuers pad with '=' despite RFC 7519.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return "", err
		}
	}
	var claims map[string]json.RawMessage
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	raw, ok := claims[authClaimKey]
	if !ok {
		return "", errors.New("access token missing https://api.openai.com/auth claim")
	}
	var ac authClaim
	if err := json.Unmarshal(raw, &ac); err != nil {
		return "", err
	}
	if ac.ChatGPTAccountID == "" {
		return "", errors.New("access token has empty chatgpt_account_id")
	}
	return ac.ChatGPTAccountID, nil
}
