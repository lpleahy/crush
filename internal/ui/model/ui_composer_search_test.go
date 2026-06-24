package model

import "testing"

func TestComposerMatches(t *testing.T) {
	text := "foo bar\nbaz Foo qux\nno match here"
	got := composerMatches(text, "foo")
	want := []composerMatch{
		{line: 0, col: 0}, // "foo" on line 0
		{line: 1, col: 4}, // "Foo" (case-insensitive) on line 1
	}
	if len(got) != len(want) {
		t.Fatalf("matches = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("match %d = %#v, want %#v", i, got[i], want[i])
		}
	}

	if composerMatches(text, "   ") != nil {
		t.Errorf("whitespace query should yield no matches")
	}
	if composerMatches(text, "zzz") != nil {
		t.Errorf("absent query should yield no matches")
	}
}

func TestComposerMatches_ColumnIsRuneIndex(t *testing.T) {
	// Wide runes before the match: the column is a rune index, not bytes.
	got := composerMatches("世界 hello", "hello")
	if len(got) != 1 || got[0].col != 3 {
		t.Errorf("matches = %#v, want one at rune col 3", got)
	}
}
