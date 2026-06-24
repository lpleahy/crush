package model

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/list"
)

// searchKey builds a KeyPressMsg whose String() matches what
// handleChatSearchKey switches on: special names map to their key codes,
// anything else is treated as printable text.
func searchKey(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}

// focusableMessageItem is a test chat item that — unlike the bare
// testMessageItem in chat_draw_cache_test.go — also implements
// list.Focusable, so Chat.isSelectable reports it selectable and
// FindMatches will scan it. body is the RawRender text (what search
// scans); its line count also drives the list item height, which is
// what ItemViewportY / MatchViewportPos depend on.
type focusableMessageItem struct {
	id   string
	body string
}

func (m *focusableMessageItem) ID() string           { return m.id }
func (m *focusableMessageItem) Render(int) string    { return m.body }
func (m *focusableMessageItem) RawRender(int) string { return m.body }
func (m *focusableMessageItem) Version() uint64      { return 0 }
func (m *focusableMessageItem) Finished() bool       { return true }
func (m *focusableMessageItem) SetFocused(bool)      {}

var (
	_ chat.MessageItem = (*focusableMessageItem)(nil)
	_ list.Focusable   = (*focusableMessageItem)(nil)
)

// newSearchUI builds a UI with a sized chat populated with the given
// focusable items, ready for FindMatches / navigation assertions.
func newSearchUI(t *testing.T, items ...chat.MessageItem) *UI {
	t.Helper()
	u := newTestUI()
	// Mirror the production search field (ui.go New) so Focus()/Update
	// behave the same as at runtime — the zero value has no cursor blink
	// context and panics on Focus().
	u.searchInput = textinput.New()
	u.searchInput.SetVirtualCursor(false)
	u.searchInput.Prompt = "/"
	u.searchInput.SetStyles(u.com.Styles.TextInput)
	u.chat.SetMessages(items...)
	// A generous viewport and a fixed width so RawRender(width) is
	// deterministic and the list lays out every item.
	u.chat.SetSize(80, 100)
	return u
}

func TestFindMatches_AcrossSelectableBlocks(t *testing.T) {
	t.Parallel()

	// Three blocks; "foo" appears twice in block 0 (one line each),
	// once in block 2. Block 1 has no hit. Display order: block 0
	// left-to-right, then block 2.
	u := newSearchUI(t,
		&focusableMessageItem{id: "a", body: "foo bar\nbaz foo"},
		&focusableMessageItem{id: "b", body: "nothing here"},
		&focusableMessageItem{id: "c", body: "ending foo"},
	)

	matches := u.chat.FindMatches("foo")
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d: %+v", len(matches), matches)
	}

	// Columns include the MessageLeftPaddingTotal offset.
	off := chat.MessageLeftPaddingTotal
	want := []SearchMatch{
		{ItemIndex: 0, Line: 0, StartCol: 0 + off, EndCol: 3 + off},
		{ItemIndex: 0, Line: 1, StartCol: 4 + off, EndCol: 7 + off},
		{ItemIndex: 2, Line: 0, StartCol: 7 + off, EndCol: 10 + off},
	}
	for i, w := range want {
		if matches[i] != w {
			t.Errorf("match[%d] = %+v, want %+v", i, matches[i], w)
		}
	}
}

func TestFindMatches_EmptyQuery(t *testing.T) {
	t.Parallel()

	u := newSearchUI(t, &focusableMessageItem{id: "a", body: "foo"})
	if got := u.chat.FindMatches("   "); got != nil {
		t.Errorf("blank query should yield no matches, got %+v", got)
	}
}

func TestFindMatches_DefaultWidthWhenUnset(t *testing.T) {
	t.Parallel()

	// A chat whose list width is still 0 (never sized for width) must
	// fall back to width 80 rather than scan at width 0. The match is
	// still found at its content column.
	u := newTestUI()
	u.chat.SetMessages(&focusableMessageItem{id: "a", body: "find me"})
	// Intentionally do NOT call SetSize, so list.Width() == 0 and the
	// width<=0 fallback branch runs.
	if w := u.chat.list.Width(); w != 0 {
		t.Fatalf("precondition: expected unset width 0, got %d", w)
	}

	matches := u.chat.FindMatches("me")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match at default width, got %d", len(matches))
	}
	off := chat.MessageLeftPaddingTotal
	if matches[0].StartCol != 5+off || matches[0].EndCol != 7+off {
		t.Errorf("match cols = (%d,%d), want (%d,%d)",
			matches[0].StartCol, matches[0].EndCol, 5+off, 7+off)
	}
}

// nonRawFocusableItem is selectable (Focusable) but does NOT implement
// list.RawRenderable, so FindMatches must skip it at the type assertion.
type nonRawFocusableItem struct {
	id   string
	body string
}

func (m *nonRawFocusableItem) ID() string        { return m.id }
func (m *nonRawFocusableItem) Render(int) string { return m.body }
func (m *nonRawFocusableItem) Version() uint64   { return 0 }
func (m *nonRawFocusableItem) Finished() bool    { return true }
func (m *nonRawFocusableItem) SetFocused(bool)   {}

func TestFindMatches_SkipsNonRawRenderable(t *testing.T) {
	t.Parallel()

	// The list takes list.Item, so the non-RawRenderable focusable item
	// is a valid list entry; FindMatches must skip it (no RawRender) and
	// only return the focusable+RawRenderable block's hit.
	u := newTestUI()
	u.chat.list.SetItems(
		&nonRawFocusableItem{id: "noraw", body: "foo hidden"},
		&focusableMessageItem{id: "raw", body: "foo shown"},
	)
	u.chat.SetSize(80, 100)

	matches := u.chat.FindMatches("foo")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match (raw item only), got %d: %+v", len(matches), matches)
	}
	if matches[0].ItemIndex != 1 {
		t.Errorf("match should be in the RawRenderable item (index 1), got %d", matches[0].ItemIndex)
	}
}

func TestFindMatches_SkipsNonFocusableBlocks(t *testing.T) {
	t.Parallel()

	// testMessageItem (from chat_draw_cache_test.go) is NOT focusable,
	// so it is not selectable and FindMatches must skip it even though
	// it contains the query and is RawRenderable.
	u := newTestUI()
	u.chat.SetMessages(
		testMessageItem{id: "plain", text: "foo here"},
		&focusableMessageItem{id: "focus", body: "foo there"},
	)
	u.chat.SetSize(80, 100)

	matches := u.chat.FindMatches("foo")
	if len(matches) != 1 {
		t.Fatalf("expected only the focusable block to match, got %d: %+v", len(matches), matches)
	}
	if matches[0].ItemIndex != 1 {
		t.Errorf("expected match in item 1 (focusable), got item %d", matches[0].ItemIndex)
	}
}

func TestApplyMatch_And_ClearSearchHighlight(t *testing.T) {
	t.Parallel()

	u := newSearchUI(t,
		&focusableMessageItem{id: "a", body: "alpha"},
		&focusableMessageItem{id: "b", body: "bravo match"},
	)
	matches := u.chat.FindMatches("match")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	u.chat.ApplyMatch(matches[0])
	if u.chat.searchHiItem != matches[0].ItemIndex {
		t.Errorf("searchHiItem = %d, want %d", u.chat.searchHiItem, matches[0].ItemIndex)
	}
	if u.chat.searchHiLine != matches[0].Line ||
		u.chat.searchHiStartCol != matches[0].StartCol ||
		u.chat.searchHiEndCol != matches[0].EndCol {
		t.Errorf("highlight fields not recorded: line=%d start=%d end=%d",
			u.chat.searchHiLine, u.chat.searchHiStartCol, u.chat.searchHiEndCol)
	}
	if u.chat.list.Selected() != matches[0].ItemIndex {
		t.Errorf("ApplyMatch should select the block: selected=%d, want %d",
			u.chat.list.Selected(), matches[0].ItemIndex)
	}

	u.chat.ClearSearchHighlight()
	if u.chat.searchHiItem != -1 {
		t.Errorf("after clear, searchHiItem = %d, want -1", u.chat.searchHiItem)
	}
}

func TestMatchViewportPos_NoActiveMatch(t *testing.T) {
	t.Parallel()

	u := newSearchUI(t, &focusableMessageItem{id: "a", body: "alpha"})
	u.chat.ClearSearchHighlight() // searchHiItem = -1
	if _, _, ok := u.chat.MatchViewportPos(); ok {
		t.Error("MatchViewportPos should report ok=false with no active match")
	}
}

func TestMatchViewportPos_ItemNotLaidOut(t *testing.T) {
	t.Parallel()

	// An active highlight whose item index is out of range: ItemViewportY
	// returns ok=false, so MatchViewportPos must too (the "block isn't
	// laid out" path), even though searchHiItem >= 0.
	u := newSearchUI(t, &focusableMessageItem{id: "a", body: "alpha"})
	u.chat.searchHiItem = 99 // out of range for a 1-item list
	if _, _, ok := u.chat.MatchViewportPos(); ok {
		t.Error("MatchViewportPos should report ok=false when the item is out of range")
	}
}

func TestMatchViewportPos_RowIsItemTopPlusLine(t *testing.T) {
	t.Parallel()

	// Block 0 is 3 lines tall; the match is on line 1 of block 1.
	// With gap 1 (the chat list default) block 1's top is at row
	// 3 + gap(1) = 4 from the top of the list, so the match row is
	// 4 + 1 = 5 when nothing is scrolled off the top. The column is the
	// stored StartCol (padding included).
	u := newSearchUI(t,
		&focusableMessageItem{id: "a", body: "l0\nl1\nl2"},
		&focusableMessageItem{id: "b", body: "x0\nfoo here\nx2"},
	)
	// Pin to top so Offset() == 0 and the geometry is unambiguous.
	u.chat.list.ScrollToTop()

	matches := u.chat.FindMatches("foo")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d: %+v", len(matches), matches)
	}
	u.chat.ApplyMatch(matches[0])
	// ApplyMatch centers the view; re-pin to top for a deterministic row.
	u.chat.list.ScrollToTop()

	col, row, ok := u.chat.MatchViewportPos()
	if !ok {
		t.Fatal("expected ok=true for a laid-out match")
	}
	if col != matches[0].StartCol {
		t.Errorf("col = %d, want %d (stored StartCol)", col, matches[0].StartCol)
	}
	if want := 4 + matches[0].Line; row != want {
		t.Errorf("row = %d, want %d (block-1 top 4 + line %d)", row, want, matches[0].Line)
	}
}

func TestRunChatSearch_JumpsToNewestMatch(t *testing.T) {
	t.Parallel()

	// Three matches in display (oldest-first) order; runChatSearch must
	// land on the LAST one (newest) and highlight it.
	u := newSearchUI(t,
		&focusableMessageItem{id: "a", body: "foo one"},
		&focusableMessageItem{id: "b", body: "foo two"},
		&focusableMessageItem{id: "c", body: "foo three"},
	)
	u.searchInput.SetValue("foo")

	u.runChatSearch()

	if !u.search.hasMatches() {
		t.Fatal("expected matches after runChatSearch")
	}
	last := len(u.search.matches) - 1
	if u.search.cur != last {
		t.Errorf("cur = %d, want newest (%d)", u.search.cur, last)
	}
	// The highlighted block must be the newest match's block.
	if u.chat.searchHiItem != u.search.matches[last].ItemIndex {
		t.Errorf("highlighted item = %d, want newest match's item %d",
			u.chat.searchHiItem, u.search.matches[last].ItemIndex)
	}
}

func TestRunChatSearch_NoMatchesClearsHighlight(t *testing.T) {
	t.Parallel()

	u := newSearchUI(t, &focusableMessageItem{id: "a", body: "foo"})
	u.searchInput.SetValue("foo")
	u.runChatSearch()
	if !u.search.hasMatches() {
		t.Fatal("precondition: should have a match for foo")
	}

	// Change the query to something absent: matches drop and the
	// highlight is cleared, cur resets to 0.
	u.searchInput.SetValue("zzz")
	u.runChatSearch()
	if u.search.hasMatches() {
		t.Error("expected no matches for absent query")
	}
	if u.search.cur != 0 {
		t.Errorf("cur = %d, want 0 after no matches", u.search.cur)
	}
	if u.chat.searchHiItem != -1 {
		t.Errorf("searchHiItem = %d, want -1 (cleared)", u.chat.searchHiItem)
	}
}

func TestChatSearchStep_WrapsAround(t *testing.T) {
	t.Parallel()

	// Build a UI with several single-line matches across blocks so cur
	// can move 0..n-1 and ApplyMatch has valid indices.
	items := make([]chat.MessageItem, 0, 5)
	for i := range 5 {
		items = append(items, &focusableMessageItem{
			id:   "m" + strconv.Itoa(i),
			body: "foo " + strconv.Itoa(i),
		})
	}
	u := newSearchUI(t, items...)
	u.searchInput.SetValue("foo")
	u.runChatSearch()

	n := len(u.search.matches)
	if n != 5 {
		t.Fatalf("expected 5 matches, got %d", n)
	}
	last := n - 1

	// runChatSearch starts at the newest (last). Forward (newer) from the
	// last wraps around to the first, like vim's n.
	if u.search.cur != last {
		t.Fatalf("precondition: cur should start at last (%d), got %d", last, u.search.cur)
	}
	u.chatSearchStep(1)
	if u.search.cur != 0 {
		t.Errorf("down at the last match should wrap to 0, got %d", u.search.cur)
	}

	// Backward (older) from the first wraps around to the last.
	u.chatSearchStep(-1)
	if u.search.cur != last {
		t.Errorf("up at the first match should wrap to last (%d), got %d", last, u.search.cur)
	}

	// A single step in the middle moves by exactly one.
	u.chatSearchStep(-1)
	if u.search.cur != last-1 {
		t.Errorf("one step up from last: cur = %d, want %d", u.search.cur, last-1)
	}
}

func TestChatSearchStep_NoMatches(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.search = chatSearchState{} // no matches
	if cmd := u.chatSearchStep(1); cmd != nil {
		t.Error("chatSearchStep with no matches should return nil")
	}
	if u.search.cur != 0 {
		t.Errorf("cur should stay 0 with no matches, got %d", u.search.cur)
	}
}

func TestChatSearchStep_SingleMatchStaysPut(t *testing.T) {
	t.Parallel()

	u := newSearchUI(t, &focusableMessageItem{id: "a", body: "foo only"})
	u.searchInput.SetValue("foo")
	u.runChatSearch()
	if len(u.search.matches) != 1 {
		t.Fatalf("expected exactly 1 match, got %d", len(u.search.matches))
	}

	if cmd := u.chatSearchStep(1); cmd != nil {
		t.Error("down with a single match should be a no-op")
	}
	if cmd := u.chatSearchStep(-1); cmd != nil {
		t.Error("up with a single match should be a no-op")
	}
	if u.search.cur != 0 {
		t.Errorf("single match: cur = %d, want 0", u.search.cur)
	}
}

func TestCenterSelected_NoSelectionIsNoOp(t *testing.T) {
	t.Parallel()

	// With nothing selected (Selected() == -1) centerSelected must
	// return without touching scroll. Build a chat with no selectable
	// items so SetSelected leaves selection at -1.
	u := newTestUI()
	u.chat.SetMessages(
		testMessageItem{id: "p", text: "plain one"}, // not focusable
	)
	u.chat.SetSize(80, 10)
	u.chat.list.SetSelected(-1)
	if u.chat.list.Selected() != -1 {
		t.Fatalf("precondition: expected no selection, got %d", u.chat.list.Selected())
	}

	before := u.chat.list.Offset()
	u.chat.centerSelected() // must be a no-op
	if after := u.chat.list.Offset(); after != before {
		t.Errorf("centerSelected with no selection changed offset: %d -> %d", before, after)
	}
}

func TestCenterSelected_KeepsMatchInViewport(t *testing.T) {
	t.Parallel()

	// Many tall blocks so the selected block is far from the top; after
	// ApplyMatch (which calls centerSelected) the selected block's top
	// must be at or above the viewport's vertical midpoint and within
	// the viewport, i.e. the match is brought on-screen.
	items := make([]chat.MessageItem, 0, 40)
	for i := range 40 {
		lines := make([]string, 5)
		for j := range lines {
			lines[j] = "block" + strconv.Itoa(i) + "-line" + strconv.Itoa(j)
		}
		// Put the query only in block 30.
		if i == 30 {
			lines[2] = "needle here"
		}
		items = append(items, &focusableMessageItem{
			id:   "m" + strconv.Itoa(i),
			body: strings.Join(lines, "\n"),
		})
	}
	u := newSearchUI(t, items...)
	u.chat.SetSize(80, 20) // a viewport smaller than the content

	matches := u.chat.FindMatches("needle")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	u.chat.ApplyMatch(matches[0]) // selects + centers

	_, row, ok := u.chat.MatchViewportPos()
	if !ok {
		t.Fatal("expected the match to be laid out")
	}
	if row < 0 || row >= u.chat.Height() {
		t.Errorf("centered match row = %d, want within [0, %d)", row, u.chat.Height())
	}
}

// TestChatSearch_PhaseTransitions drives the inline bar through keys to
// verify the two-phase model that makes vim-style n/N usable: while
// editing, n is a query character; only after enter (the navigation
// phase) do bare n/N browse matches. esc closes; "/" re-edits.
func TestChatSearch_PhaseTransitions(t *testing.T) {
	t.Parallel()

	u := newSearchUI(t,
		&focusableMessageItem{id: "a", body: "foo one"},
		&focusableMessageItem{id: "b", body: "foo two"},
		&focusableMessageItem{id: "c", body: "foo three"},
	)
	u.openChatSearch()
	if !u.search.active || !u.search.editing {
		t.Fatalf("openChatSearch: active=%v editing=%v, want both true", u.search.active, u.search.editing)
	}

	// Editing: each printable key types into the query and re-runs search.
	for _, r := range "foo" {
		u.handleChatSearchKey(searchKey(string(r)))
	}
	if u.searchQuery() != "foo" {
		t.Fatalf("after typing, query = %q, want \"foo\"", u.searchQuery())
	}
	if len(u.search.matches) != 3 {
		t.Fatalf("expected 3 matches for foo, got %d", len(u.search.matches))
	}
	last := len(u.search.matches) - 1
	if u.search.cur != last {
		t.Fatalf("editing lands on newest match %d, got %d", last, u.search.cur)
	}

	// Editing: 'n' is a query character (the exact thing that made n/N
	// unusable), NOT a navigation key.
	u.handleChatSearchKey(searchKey("n"))
	if u.searchQuery() != "foon" {
		t.Errorf("editing: 'n' should type into the query, got %q", u.searchQuery())
	}
	// Reset the query cleanly and re-run for the navigation checks.
	u.searchInput.SetValue("foo")
	u.runChatSearch()

	// Enter confirms -> navigation phase: bar stays active, field blurs.
	u.handleChatSearchKey(searchKey("enter"))
	if u.search.editing {
		t.Error("enter should leave the editing phase")
	}
	if !u.search.active {
		t.Error("enter should keep the bar active for navigation")
	}

	// Navigation: bare 'N' steps to the older match and does NOT type.
	prev := u.search.cur
	u.handleChatSearchKey(searchKey("N"))
	if u.search.cur != prev-1 {
		t.Errorf("nav 'N': cur = %d, want older %d", u.search.cur, prev-1)
	}
	if u.searchQuery() != "foo" {
		t.Errorf("nav 'N' must not edit the query, got %q", u.searchQuery())
	}
	// 'n' steps to the newer match.
	prev = u.search.cur
	u.handleChatSearchKey(searchKey("n"))
	if u.search.cur != prev+1 {
		t.Errorf("nav 'n': cur = %d, want newer %d", u.search.cur, prev+1)
	}

	// "/" re-enters editing.
	u.handleChatSearchKey(searchKey("/"))
	if !u.search.editing {
		t.Error("'/' should return to the editing phase")
	}

	// esc closes the search entirely.
	u.handleChatSearchKey(searchKey("esc"))
	if u.search.active || u.search.hasMatches() {
		t.Errorf("esc should close search: active=%v matches=%d", u.search.active, len(u.search.matches))
	}
}

// TestChatSetSearchMatches_GroupsByItem checks that the dim "all matches"
// layer groups occurrences by their block (item) index, so each block
// gets exactly its own hits, and that an empty set clears the grouping.
func TestChatSetSearchMatches_GroupsByItem(t *testing.T) {
	t.Parallel()

	c := &Chat{}
	c.SetSearchMatches([]SearchMatch{
		{ItemIndex: 2, Line: 0, StartCol: 1, EndCol: 4},
		{ItemIndex: 2, Line: 1, StartCol: 0, EndCol: 3},
		{ItemIndex: 5, Line: 0, StartCol: 7, EndCol: 10},
	})
	if got := len(c.searchMatchRanges); got != 2 {
		t.Fatalf("grouped into %d blocks, want 2", got)
	}
	if got := len(c.searchMatchRanges[2]); got != 2 {
		t.Errorf("block 2 has %d ranges, want 2", got)
	}
	if got := len(c.searchMatchRanges[5]); got != 1 {
		t.Errorf("block 5 has %d ranges, want 1", got)
	}
	if want := (chat.SearchRange{Line: 1, StartCol: 0, EndCol: 3}); c.searchMatchRanges[2][1] != want {
		t.Errorf("block 2 second range = %+v, want %+v", c.searchMatchRanges[2][1], want)
	}

	c.SetSearchMatches(nil)
	if c.searchMatchRanges != nil {
		t.Errorf("empty SetSearchMatches should clear grouping, got %+v", c.searchMatchRanges)
	}
}

// TestChatSetSearchMatches_SelectsBlockForEachMatch verifies that
// applying each match selects (and highlights) the block that match lives
// in — the thing that lets n/N move the conversation's selection to each
// hit.
func TestChatSetSearchMatches_SelectsBlockForEachMatch(t *testing.T) {
	t.Parallel()

	u := newSearchUI(t,
		&focusableMessageItem{id: "a", body: "foo first"},
		&focusableMessageItem{id: "b", body: "nothing"},
		&focusableMessageItem{id: "c", body: "foo third"},
	)
	u.searchInput.SetValue("foo")
	u.runChatSearch()

	matches := u.search.matches
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches (blocks 0 and 2), got %d", len(matches))
	}
	// runChatSearch lands on the newest match; its block must be selected.
	if got, want := u.chat.list.Selected(), matches[len(matches)-1].ItemIndex; got != want {
		t.Errorf("after runChatSearch, selected block = %d, want match's block %d", got, want)
	}
	// Stepping to every match selects that match's block in turn.
	for i := range matches {
		u.search.cur = i
		u.chat.ApplyMatch(matches[i])
		if got := u.chat.list.Selected(); got != matches[i].ItemIndex {
			t.Errorf("match %d in block %d: selected block = %d", i, matches[i].ItemIndex, got)
		}
		if got := u.chat.searchHiItem; got != matches[i].ItemIndex {
			t.Errorf("match %d: searchHiItem = %d, want %d", i, got, matches[i].ItemIndex)
		}
	}
}

// TestChatSearch_FocusesConversation guards the block-selection fix:
// opening search focuses the conversation so the selected block renders
// (the selected-item visual only applies when the list is focused), even
// when search is started from the composer. Ending search restores focus
// to match the app focus.
func TestChatSearch_FocusesConversation(t *testing.T) {
	t.Parallel()

	u := newSearchUI(t,
		&focusableMessageItem{id: "a", body: "foo first"},
		&focusableMessageItem{id: "b", body: "foo second"},
	)

	// Searching from the composer: editor-focused, conversation blurred.
	u.focus = uiFocusEditor
	u.chat.Blur()
	if u.chat.list.Focused() {
		t.Fatal("precondition: chat should be blurred when editing")
	}

	u.openChatSearch()
	if !u.chat.list.Focused() {
		t.Error("openChatSearch must focus the conversation so the selected block shows")
	}

	// Ending from the editor restores the blurred conversation.
	u.endChatSearch()
	if u.chat.list.Focused() {
		t.Error("ending search from the editor should re-blur the conversation")
	}

	// Searching from the conversation keeps it focused after ending.
	u.focus = uiFocusMain
	u.openChatSearch()
	u.endChatSearch()
	if !u.chat.list.Focused() {
		t.Error("ending search from the conversation should keep it focused")
	}
}

// TestChatSearchNav_NKeysWrap checks that, in the navigation phase, bare
// n/N step through matches and wrap around the ends (vim-style) without
// dead-ending; enter is reserved for the native copy-mode hand-off.
func TestChatSearchNav_NKeysWrap(t *testing.T) {
	t.Parallel()

	u := newSearchUI(t,
		&focusableMessageItem{id: "a", body: "foo 1"},
		&focusableMessageItem{id: "b", body: "foo 2"},
		&focusableMessageItem{id: "c", body: "foo 3"},
	)
	u.openChatSearch()
	for _, r := range "foo" {
		u.handleChatSearchKey(searchKey(string(r)))
	}
	u.handleChatSearchKey(searchKey("enter")) // confirm -> navigation
	if u.search.editing {
		t.Fatal("expected navigation phase after confirm")
	}

	n := len(u.search.matches)
	if n != 3 {
		t.Fatalf("expected 3 matches, got %d", n)
	}
	last := n - 1 // cur starts here (newest)

	// 'n' (newer/forward) from the newest wraps around to the oldest.
	u.handleChatSearchKey(searchKey("n"))
	if u.search.cur != 0 {
		t.Errorf("nav 'n' from newest should wrap to 0, got %d", u.search.cur)
	}
	// 'N' (older/back) from the oldest wraps around to the newest.
	u.handleChatSearchKey(searchKey("N"))
	if u.search.cur != last {
		t.Errorf("nav 'N' from oldest should wrap to last (%d), got %d", last, u.search.cur)
	}
}
