package openai

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/version"
)

// NewHTTPClient returns an *http.Client whose transport shapes outgoing
// requests for chatgpt.com/backend-api/codex. session-id, thread-id,
// and x-client-request-id are scoped per logical conversation, looked
// up via WithSession on the request context. Requests without a
// tagged session fall back to a process-level scope.
//
// JSON request bodies go through AdaptRequestBody so the backend's
// required mutations (store=false, instructions extraction,
// encrypted reasoning replay, prompt_cache_key, client_metadata) are
// applied transparently.
func NewHTTPClient(debug bool) *http.Client {
	return &http.Client{Transport: newChatGPTTransport(debug)}
}

type chatgptTransport struct {
	debug bool
}

// Per-session IDs shared across all transports in the process so
// requests with the same session.ID — even when issued through
// different transports built per provider construction — get the
// same prompt_cache_key. Fallback IDs cover requests without a
// tagged session (tests, ad-hoc calls).
//
// The map grows with distinct sessions over a process lifetime
// (~80B per entry); not worth eviction logic for realistic CLI use.
var (
	fallbackSessionID = sync.OnceValue(mustNewUUID)
	fallbackThreadID  = sync.OnceValue(mustNewUUID)

	convIDsMu sync.Mutex
	convIDs   = map[string]conversationIDs{}
)

type conversationIDs struct {
	sessionID string
	threadID  string
}

func mustNewUUID() string {
	id, err := newUUID()
	if err != nil {
		panic("openai: generate UUID: " + err.Error())
	}
	return id
}

func newChatGPTTransport(debug bool) *chatgptTransport {
	return &chatgptTransport{debug: debug}
}

// idsForContext returns the conversation IDs for the session tagged
// on ctx, creating them on first lookup. Without a tagged session,
// returns a process-level fallback.
func idsForContext(ctx context.Context) conversationIDs {
	scope := sessionFromContext(ctx)
	if scope == "" {
		return conversationIDs{
			sessionID: fallbackSessionID(),
			threadID:  fallbackThreadID(),
		}
	}
	convIDsMu.Lock()
	defer convIDsMu.Unlock()
	if existing, ok := convIDs[scope]; ok {
		return existing
	}
	ids := conversationIDs{
		sessionID: mustNewUUID(),
		threadID:  mustNewUUID(),
	}
	convIDs[scope] = ids
	return ids
}

func (t *chatgptTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	ids := idsForContext(req.Context())
	req.Header.Set("session-id", ids.sessionID)
	req.Header.Set("thread-id", ids.threadID)
	req.Header.Set("x-client-request-id", ids.threadID)
	// Set User-Agent here (not via SetupChatGPT's ExtraHeaders) so
	// CRUSH_CODEX_CLI_VERSION is re-read on every request and users
	// can self-rescue without restarting crush.
	req.Header.Set("User-Agent", UserAgent())

	if isJSONBody(req) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("openai transport: read body: %w", err)
		}
		req.Body.Close()
		adapted, err := AdaptRequestBody(body, AdaptOptions{
			PromptCacheKey: ids.threadID,
			ClientName:     "crush",
			ClientVersion:  version.Version,
		})
		if err != nil {
			// Body wasn't a JSON object we can adapt; pass through unchanged.
			adapted = body
		}
		req.Body = io.NopCloser(bytes.NewReader(adapted))
		req.ContentLength = int64(len(adapted))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(adapted)), nil
		}
	}

	resp, err := t.next().RoundTrip(req)
	if err != nil {
		return nil, err
	}
	logChatGPTBlock(resp)
	return resp, nil
}

// logChatGPTBlock emits a slog.Warn for the two known-bad ChatGPT
// backend rejections so a user grepping crush.log can quickly diagnose
// without parsing the raw JSON error fantasy bubbles up. The response
// body is preserved (sniffed and rewrapped) so the upstream caller can
// still parse it normally.
func logChatGPTBlock(resp *http.Response) {
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		return
	}
	if resp.Header.Get("cf-mitigated") == "challenge" {
		slog.Warn("ChatGPT backend blocked by Cloudflare bot mitigation",
			"hint", "request fingerprint may not match Codex CLI; try setting CRUSH_CODEX_CLI_VERSION to a newer version or fall back to OPENAI_API_KEY")
		return
	}
	if resp.Body == nil || resp.Body == http.NoBody {
		return
	}
	orig := resp.Body
	peek, err := io.ReadAll(io.LimitReader(orig, 1<<14))
	if err != nil {
		return
	}
	resp.Body = combinedReadCloser{
		Reader: io.MultiReader(bytes.NewReader(peek), orig),
		Closer: orig,
	}
	if bytes.Contains(peek, []byte("unsupported_country_region_territory")) {
		slog.Warn("ChatGPT backend rejected request due to geo restriction",
			"hint", "your country/region is not supported for Codex backend access; use OPENAI_API_KEY instead")
	}
}

type combinedReadCloser struct {
	io.Reader
	io.Closer
}

func (t *chatgptTransport) next() http.RoundTripper {
	if t.debug {
		return log.NewHTTPClient().Transport
	}
	return http.DefaultTransport
}

func isJSONBody(req *http.Request) bool {
	if req.Body == nil || req.Body == http.NoBody {
		return false
	}
	ct := req.Header.Get("Content-Type")
	return ct == "application/json" || ct == "application/json; charset=utf-8"
}

// newUUID returns an RFC 4122 v4 UUID. crypto/rand is the only source
// of randomness; on a working OS it never fails.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
