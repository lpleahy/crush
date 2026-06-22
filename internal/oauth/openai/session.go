package openai

import "context"

type sessionContextKey struct{}

// WithSession returns ctx tagged with sessionID so the chatgpt
// transport (NewHTTPClient) can issue a stable session-id, thread-id,
// and prompt_cache_key per logical conversation. The coordinator
// applies this once per agent run (both top-level and sub-agent), so
// repeated turns within the same session share cache state on the
// ChatGPT backend while unrelated sessions don't.
//
// Callers that don't tag ctx fall back to a process-level scope —
// fine for ad-hoc requests but not appropriate for production
// inference paths where multiple unrelated conversations would
// otherwise collapse onto a single shared key.
//
// Empty sessionID is a no-op.
func WithSession(ctx context.Context, sessionID string) context.Context {
	if sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionContextKey{}, sessionID)
}

// sessionFromContext returns the sessionID previously tagged via
// WithSession, or "" if none.
func sessionFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(sessionContextKey{}).(string); ok {
		return v
	}
	return ""
}
