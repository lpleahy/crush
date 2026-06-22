package openai

import (
	"context"
	"testing"
)

// An empty sessionID is a documented no-op: WithSession must return the
// original context unchanged so callers fall back to the process-level
// scope rather than tagging a meaningless empty session.
func TestWithSession_EmptyIsNoOp(t *testing.T) {
	ctx := context.Background()
	got := WithSession(ctx, "")
	if got != ctx {
		t.Error("WithSession(ctx, \"\") should return the original context unchanged")
	}
	if sessionFromContext(got) != "" {
		t.Errorf("sessionFromContext after empty WithSession = %q, want empty", sessionFromContext(got))
	}
}

func TestWithSession_RoundTrip(t *testing.T) {
	ctx := WithSession(context.Background(), "sess-xyz")
	if got := sessionFromContext(ctx); got != "sess-xyz" {
		t.Errorf("sessionFromContext = %q, want sess-xyz", got)
	}
}

// A context never tagged via WithSession yields the empty scope.
func TestSessionFromContext_Untagged(t *testing.T) {
	if got := sessionFromContext(context.Background()); got != "" {
		t.Errorf("sessionFromContext(untagged) = %q, want empty", got)
	}
}
