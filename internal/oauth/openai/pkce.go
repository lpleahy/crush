package openai

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewVerifier returns a random PKCE code_verifier per RFC 7636 §4.1.
// The result is 64 characters of base64url-unpadded entropy.
func NewVerifier() (string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Challenge computes the S256 code_challenge for a verifier per
// RFC 7636 §4.2: BASE64URL-NO-PAD(SHA256(ASCII(verifier))).
func Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
