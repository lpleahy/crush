package openai

import "testing"

func TestChallenge_RFC7636Fixture(t *testing.T) {
	// RFC 7636 Appendix B fixture.
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := Challenge(verifier); got != want {
		t.Errorf("Challenge(%q) = %q, want %q", verifier, got, want)
	}
}

func TestNewVerifier(t *testing.T) {
	v1, err := NewVerifier()
	if err != nil {
		t.Fatalf("NewVerifier() error: %v", err)
	}
	if got := len(v1); got != 64 {
		t.Errorf("len(NewVerifier()) = %d, want 64", got)
	}

	v2, err := NewVerifier()
	if err != nil {
		t.Fatalf("NewVerifier() error: %v", err)
	}
	if v1 == v2 {
		t.Error("two NewVerifier() calls returned the same value")
	}
}
