package openai

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

// BrowserOpener opens a URL in the user's default browser. login.go
// wires pkg/browser into this in production; tests can inject a
// recording or HTTP-following implementation.
type BrowserOpener interface {
	Open(url string) error
}

// AuthorizeOptions controls how Authorize runs the flow.
type AuthorizeOptions struct {
	// Browser, if set, is called with the authorize URL after the
	// loopback listener binds. The call runs in a goroutine; Authorize
	// does not wait for Open to return.
	Browser BrowserOpener

	// OnReady is called once with the authorize URL after the loopback
	// listener binds. Use this to print the URL for paste-back even
	// when Browser is set, so SSH/headless users have a fallback.
	OnReady func(authorizeURL string)
}

// AuthorizeResult is the successful return of Authorize.
type AuthorizeResult struct {
	Code     string
	Verifier string
}

// successPageHTML is intentionally minimal: system fonts, no
// hardcoded colors, prefers-color-scheme for dark mode. The browser
// already has to render *something* (the redirect lands here, not on
// auth.openai.com), but we don't want our page to look louder than
// what the user came from.
const successPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Sign-in complete</title>
<style>
  body { font-family: system-ui, -apple-system, sans-serif;
         max-width: 32em; margin: 4em auto; padding: 0 1em;
         line-height: 1.5; }
  h1 { font-weight: 500; font-size: 1.25rem; margin: 0 0 0.5em; }
  p  { margin: 0; color: #666; }
  @media (prefers-color-scheme: dark) {
    body { background: #1a1a1a; color: #e0e0e0; }
    p { color: #999; }
  }
</style>
</head>
<body>
<h1>Sign-in complete</h1>
<p>You can close this tab and return to crush.</p>
</body>
</html>`

// Authorize runs the loopback PKCE flow against c.AuthorizeURL. It
// binds the loopback listener (port from c.RedirectURI), builds the
// authorize URL, optionally opens a browser, and blocks until the OAuth
// callback fires or ctx is cancelled.
//
// The returned Code + Verifier are passed to TokenExchange to mint the
// access and refresh tokens.
func (c *Client) Authorize(ctx context.Context, opts AuthorizeOptions) (*AuthorizeResult, error) {
	verifier, err := NewVerifier()
	if err != nil {
		return nil, fmt.Errorf("generate verifier: %w", err)
	}
	challenge := Challenge(verifier)

	state, err := NewVerifier()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	parsed, err := url.Parse(c.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("parse redirect_uri: %w", err)
	}
	listenAddr := parsed.Host
	if _, _, err := net.SplitHostPort(listenAddr); err != nil {
		// Host has no explicit port; default to Codex's registered port.
		listenAddr = net.JoinHostPort(listenAddr, strconv.Itoa(DefaultRedirectPort))
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("bind loopback listener: %w", err)
	}
	defer listener.Close()

	authorizeURL := c.buildAuthorizeURL(challenge, state)

	if opts.OnReady != nil {
		opts.OnReady(authorizeURL)
	}
	if opts.Browser != nil {
		go func() {
			_ = opts.Browser.Open(authorizeURL)
		}()
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(parsed.Path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errStr := q.Get("error"); errStr != "" {
			errCh <- fmt.Errorf("authorize: %s: %s", errStr, q.Get("error_description"))
			http.Error(w, "Sign-in failed: "+errStr, http.StatusBadRequest)
			return
		}
		if q.Get("state") != state {
			errCh <- errors.New("authorize: state mismatch")
			http.Error(w, "State mismatch — possible CSRF.", http.StatusBadRequest)
			return
		}
		code := q.Get("code")
		if code == "" {
			errCh <- errors.New("authorize: callback missing code")
			http.Error(w, "Callback missing code.", http.StatusBadRequest)
			return
		}
		codeCh <- code
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(successPageHTML))
	})

	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(listener)
	}()
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case code := <-codeCh:
		return &AuthorizeResult{Code: code, Verifier: verifier}, nil
	}
}

// Authorize wraps DefaultClient.Authorize for callers that don't need a
// custom Client.
func Authorize(ctx context.Context, opts AuthorizeOptions) (*AuthorizeResult, error) {
	return DefaultClient.Authorize(ctx, opts)
}

func (c *Client) buildAuthorizeURL(challenge, state string) string {
	q := url.Values{
		"response_type":              {"code"},
		"client_id":                  {c.ClientID},
		"redirect_uri":               {c.RedirectURI},
		"scope":                      {c.Scope},
		"code_challenge":             {challenge},
		"code_challenge_method":      {"S256"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
		"originator":                 {c.Originator},
		"state":                      {state},
	}
	return c.AuthorizeURL + "?" + q.Encode()
}
