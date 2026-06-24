package model

import (
	"reflect"
	"testing"
)

func TestOccurrencesInRendered_BasicAndMultiple(t *testing.T) {
	// Two hits on line 0, one on line 2. Line 1 has no hit.
	rendered := "foo bar foo\nnothing here\nfoo end"
	got := occurrencesInRendered(rendered, "foo")
	want := []occurrence{
		{line: 0, startCol: 0, endCol: 3},
		{line: 0, startCol: 8, endCol: 11},
		{line: 2, startCol: 0, endCol: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("occurrences = %+v, want %+v", got, want)
	}
}

func TestOccurrencesInRendered_CaseInsensitive(t *testing.T) {
	got := occurrencesInRendered("The Refactor refactor REFACTOR", "refactor")
	if len(got) != 3 {
		t.Fatalf("expected 3 case-insensitive hits, got %d: %+v", len(got), got)
	}
	if got[0].startCol != 4 {
		t.Errorf("first hit startCol = %d, want 4", got[0].startCol)
	}
}

func TestOccurrencesInRendered_StripsANSI(t *testing.T) {
	// A bold "match" preceded by 5 visible chars: columns must reflect
	// the visible text, not the ANSI bytes.
	rendered := "abcd \x1b[1mmatch\x1b[0m tail"
	got := occurrencesInRendered(rendered, "match")
	if len(got) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(got))
	}
	if got[0].startCol != 5 || got[0].endCol != 10 {
		t.Errorf("hit cols = (%d,%d), want (5,10)", got[0].startCol, got[0].endCol)
	}
}

func TestOccurrencesInRendered_EmptyQuery(t *testing.T) {
	if got := occurrencesInRendered("anything", "   "); got != nil {
		t.Errorf("blank query should yield no occurrences, got %+v", got)
	}
}

func TestOccurrencesInRendered_NoMatch(t *testing.T) {
	if got := occurrencesInRendered("hello world", "xyz"); got != nil {
		t.Errorf("no-match should yield nil, got %+v", got)
	}
}

func TestOccurrencesInRendered_Overlapping(t *testing.T) {
	// "aaaa" searching "aa" -> non-overlapping: positions 0 and 2.
	got := occurrencesInRendered("aaaa", "aa")
	want := []occurrence{
		{line: 0, startCol: 0, endCol: 2},
		{line: 0, startCol: 2, endCol: 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOccurrencesInRendered_WideCharsBeforeMatch(t *testing.T) {
	// Two fullwidth CJK chars (display width 2 each) precede an ASCII
	// match. The start column must be the *display* width (4), not the
	// byte offset (6, since each rune is 3 UTF-8 bytes).
	got := occurrencesInRendered("世界hello", "hello")
	want := []occurrence{
		{line: 0, startCol: 4, endCol: 9},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOccurrencesInRendered_WideCharIsTheMatch(t *testing.T) {
	// The query itself is fullwidth CJK: the match spans two display
	// columns even though the needle is six UTF-8 bytes.
	got := occurrencesInRendered("ab世界cd", "世界")
	want := []occurrence{
		{line: 0, startCol: 2, endCol: 6},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOccurrencesInRendered_WideCharsAcrossLines(t *testing.T) {
	// Per-line scanning with wide chars: a hit on line 0 after one wide
	// char (col 2) and a hit on line 1 with no preceding wide char.
	got := occurrencesInRendered("世foo\nfoo世", "foo")
	want := []occurrence{
		{line: 0, startCol: 2, endCol: 5},
		{line: 1, startCol: 0, endCol: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOccurrencesInRendered_CaseInsensitiveMultipleColumns(t *testing.T) {
	// Mixed-case repeats on one line: verify both that every variant is
	// found and that the reported columns line up with the visible text.
	got := occurrencesInRendered("Foo foo FOO", "foo")
	want := []occurrence{
		{line: 0, startCol: 0, endCol: 3},
		{line: 0, startCol: 4, endCol: 7},
		{line: 0, startCol: 8, endCol: 11},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOccurrencesInRendered_ByteLengthDriftGuard(t *testing.T) {
	// Exercises the defensive guard for lower/plain byte-length drift:
	// some Unicode characters lowercase to a *different* UTF-8 byte
	// length. Here plain is the uppercase Ⱥ (U+023A, 2 bytes) which
	// lowercases to ⱥ (3 bytes), so a lowercase-space match index can
	// point past the end of the original plain string. The function must
	// skip the occurrence rather than slice out of range and panic.
	got := occurrencesInRendered("Ⱥ", "ⱥ")
	if got != nil {
		t.Errorf("byte-drift match should be skipped by the guard, got %+v", got)
	}
}

func TestOccurrencesInRendered_AnsiStyledWideAndCase(t *testing.T) {
	// Combine all three concerns at once: an ANSI-styled, mixed-case
	// match that sits after a wide char. Columns must reflect the
	// stripped, display-width text.
	rendered := "世\x1b[1mMaTcH\x1b[0m"
	got := occurrencesInRendered(rendered, "match")
	want := []occurrence{
		{line: 0, startCol: 2, endCol: 7},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
