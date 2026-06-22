package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

// Client makes OAuth requests against auth.openai.com. All URLs are
// overridable for tests. In production, use NewClient.
type Client struct {
	AuthorizeURL string
	TokenURL     string
	RevokeURL    string

	ClientID    string
	RedirectURI string
	Scope       string
	Originator  string

	HTTPClient *http.Client
}

// NewClient returns a Client configured against OpenAI's production
// endpoints with a 30-second HTTP timeout.
func NewClient() *Client {
	return &Client{
		AuthorizeURL: DefaultAuthorizeURL,
		TokenURL:     DefaultTokenURL,
		RevokeURL:    DefaultRevokeURL,
		ClientID:     DefaultClientID,
		RedirectURI:  DefaultRedirectURI,
		Scope:        DefaultScope,
		Originator:   DefaultOriginator,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// DefaultClient backs the free-function API (TokenExchange,
// RefreshToken, Authorize). Tests should construct their own Client.
var DefaultClient = NewClient()

// TokenExchange swaps an authorization code for an access + refresh
// token. Called once after the loopback callback fires.
func TokenExchange(ctx context.Context, code, verifier string) (*oauth.Token, error) {
	return DefaultClient.TokenExchange(ctx, code, verifier)
}

// RefreshToken exchanges a refresh_token for a new access_token. The
// config store calls this when the current access_token is near expiry.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	return DefaultClient.RefreshToken(ctx, refreshToken)
}

// RevokeToken invalidates a token server-side via RFC 7009. Used on
// logout so the refresh token can't be reused even if the local copy
// leaks. Best-effort — callers should not block logout on failure.
func RevokeToken(ctx context.Context, token string) error {
	return DefaultClient.RevokeToken(ctx, token)
}

func (c *Client) TokenExchange(ctx context.Context, code, verifier string) (*oauth.Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.ClientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {c.RedirectURI},
	}
	return c.postToken(ctx, form)
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.ClientID},
		"refresh_token": {refreshToken},
		"scope":         {c.Scope},
	}
	token, err := c.postToken(ctx, form)
	if err != nil {
		return nil, err
	}
	// OpenAI may omit refresh_token on refresh responses (rotation is
	// optional). Preserve the existing one so subsequent refreshes work.
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}

func (c *Client) RevokeToken(ctx context.Context, token string) error {
	form := url.Values{
		"token":     {token},
		"client_id": {c.ClientID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RevokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("revoke endpoint %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) postToken(ctx context.Context, form url.Values) (*oauth.Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var er struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &er)
		if er.Error != "" {
			return nil, fmt.Errorf("token endpoint %d: %s: %s", resp.StatusCode, er.Error, er.ErrorDescription)
		}
		return nil, fmt.Errorf("token endpoint %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}

	token := &oauth.Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresIn:    raw.ExpiresIn,
	}
	token.SetExpiresAt()
	return token, nil
}
