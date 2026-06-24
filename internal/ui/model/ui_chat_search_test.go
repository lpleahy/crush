package model

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
)

func TestChatSearchState_Step(t *testing.T) {
	// Pure index-arithmetic check on the wrap behavior used by n/N.
	step := func(cur, dir, n int) int {
		return ((cur+dir)%n + n) % n
	}

	cases := []struct {
		cur, dir, n, want int
	}{
		{0, 1, 3, 1},
		{2, 1, 3, 0},  // wrap forward
		{0, -1, 3, 2}, // wrap backward
		{1, -1, 3, 0},
		{0, 1, 1, 0}, // single match stays put
	}
	for _, c := range cases {
		if got := step(c.cur, c.dir, c.n); got != c.want {
			t.Errorf("step(cur=%d, dir=%d, n=%d) = %d, want %d", c.cur, c.dir, c.n, got, c.want)
		}
	}
}

func TestChatSearchState_HasMatches(t *testing.T) {
	if (chatSearchState{}).hasMatches() {
		t.Error("zero value should have no matches")
	}
	if !(chatSearchState{matches: []SearchMatch{{ItemIndex: 2}}}).hasMatches() {
		t.Error("non-empty matches should report true")
	}
}

func TestChatSearchSummary(t *testing.T) {
	m := &UI{}
	m.searchInput = textinput.New()
	m.search = chatSearchState{matches: nil}
	if got := m.chatSearchSummary(); got != "(no matches)" {
		t.Errorf("no matches summary = %q", got)
	}
	m.searchInput.SetValue("foo")
	m.search = chatSearchState{
		matches: []SearchMatch{{ItemIndex: 1}, {ItemIndex: 4}, {ItemIndex: 7}},
		cur:     1,
	}
	got := m.chatSearchSummary()
	if got == "" || got == "(no matches)" {
		t.Errorf("summary should describe position, got %q", got)
	}
}

func TestChatSearchBarSuffix(t *testing.T) {
	cases := []struct {
		name           string
		cur, total     int
		query          string
		editing        bool
		wantExact      string // when set, the full suffix must equal this
		wantContainsN  bool   // expect the "n/N" hint
		wantArrowsHint bool   // expect the "↑↓ browse" editing hint
		wantSelectHint bool   // expect the "enter → select" copy-mode hint
		wantEscHint    bool   // expect the navigation "esc" hint
		wantCounterStr string // substring the counter must contain (e.g. "2/3")
	}{
		{
			name: "empty query yields empty suffix",
			cur:  0, total: 0, query: "", editing: true,
			wantExact: "",
		},
		{
			name: "whitespace query yields empty suffix even with matches",
			cur:  0, total: 5, query: "   ", editing: true,
			wantExact: "",
		},
		{
			name: "query but no matches",
			cur:  0, total: 0, query: "foo", editing: true,
			wantExact: "  no matches",
		},
		{
			name: "editing: counter + arrows hint + enter→n/N, no select/esc hint",
			cur:  1, total: 3, query: "foo", editing: true,
			wantContainsN: true, wantArrowsHint: true, wantSelectHint: false, wantEscHint: false, wantCounterStr: "2/3",
		},
		{
			name: "navigating: counter + n/N + enter→select + esc",
			cur:  0, total: 12, query: "bar", editing: false,
			wantContainsN: true, wantArrowsHint: false, wantSelectHint: true, wantEscHint: true, wantCounterStr: "1/12",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := chatSearchBarSuffix(c.cur, c.total, c.query, c.editing)
			// Deterministic ("empty"/"no matches") cases assert the exact
			// string (including the empty string); match cases assert the
			// individual pieces.
			if c.wantCounterStr == "" && got != c.wantExact {
				t.Fatalf("suffix = %q, want %q", got, c.wantExact)
			}
			if c.wantCounterStr != "" && !strings.Contains(got, c.wantCounterStr) {
				t.Errorf("suffix %q missing counter %q", got, c.wantCounterStr)
			}
			if c.wantContainsN && !strings.Contains(got, "n/N") {
				t.Errorf("suffix %q missing n/N hint", got)
			}
			if got, want := strings.Contains(got, "↑↓ browse"), c.wantArrowsHint; got != want {
				t.Errorf("suffix arrows-hint = %v, want %v", got, want)
			}
			if gotSel := strings.Contains(got, "select"); gotSel != c.wantSelectHint {
				t.Errorf("suffix %q select-hint = %v, want %v", got, gotSel, c.wantSelectHint)
			}
			if gotEsc := strings.Contains(got, "esc"); gotEsc != c.wantEscHint {
				t.Errorf("suffix %q esc-hint = %v, want %v", got, gotEsc, c.wantEscHint)
			}
		})
	}
}
