package model

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// chatSearchState holds the in-conversation block search. The zero
// value is inactive. matches are individual occurrences across all
// blocks (every hit, not one per block), in display order; cur indexes
// into matches. The live query text lives on UI.searchInput (the
// inline bar's text field), not here.
//
// The bar has two phases while active:
//   - editing:  typing the query (the field is focused). Printable keys
//     edit the query and re-run the search incrementally; ↑/↓ browse
//     matches live; enter confirms.
//   - navigating (editing == false): the query is locked in and the
//     field is blurred, so bare n/N (and ↑/↓) browse matches without
//     typing into the query. enter then drops into native copy mode;
//     esc closes; "/" jumps back to editing.
//
// Splitting the phases is what lets vim-style n/N work: while editing,
// n and N are just query characters; only after confirming can they
// mean "next/previous match".
type chatSearchState struct {
	active  bool
	editing bool
	matches []SearchMatch
	cur     int
	// composer is set when the bar searches the composer draft instead of
	// the conversation output; cmatches/ccur are its results.
	composer bool
	cmatches []composerMatch
	ccur     int
}

func (s chatSearchState) hasMatches() bool {
	if s.composer {
		return len(s.cmatches) > 0
	}
	return len(s.matches) > 0
}

// searchQuery returns the live query from the inline search bar.
func (m *UI) searchQuery() string { return m.searchInput.Value() }

// searchBarCounts returns the (current, total) match count for the active
// bar — composer-draft or conversation-output.
func (m *UI) searchBarCounts() (cur, total int) {
	if m.search.composer {
		return m.search.ccur, len(m.search.cmatches)
	}
	return m.search.cur, len(m.search.matches)
}

// searchBarSuffix is the inline status text (counter + hints) for whichever
// search bar is active.
func (m *UI) searchBarSuffix() string {
	c, t := m.searchBarCounts()
	return chatSearchBarSuffix(c, t, m.searchQuery(), m.search.editing)
}

// openChatSearch enters search mode in the editing phase with an empty
// query and focuses the inline search bar, which then captures all keys
// until esc/enter (see handleChatSearchKey).
func (m *UI) openChatSearch() {
	m.search = chatSearchState{active: true, editing: true}
	m.searchInput.Reset()
	m.searchInput.Focus()
	// Focus the conversation so the block each match lives in renders as
	// selected while navigating. Without this, opening search from the
	// composer (where the chat stays blurred) would highlight the matched
	// text but never show the surrounding block as selected.
	m.chat.Focus()
	// Carve out the bar row and resize the chat to match.
	m.updateLayoutAndSize()
}

// handleChatSearchKey routes every key while the inline search bar is
// open, dispatching to the editing- or navigation-phase handler.
func (m *UI) handleChatSearchKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.search.editing {
		return m.handleChatSearchEditKey(msg)
	}
	return m.handleChatSearchNavKey(msg)
}

// handleChatSearchEditKey handles keys while typing the query. Special
// keys are handled explicitly; everything else is fed to the text field,
// and its new value re-runs the incremental search.
func (m *UI) handleChatSearchEditKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "ctrl+c", "tab":
		// Cancel — leave the conversation scrolled where it is, drop the
		// match highlight, blur and clear the bar.
		m.endChatSearch()
		return nil
	case "enter":
		// Confirm the query and switch to the navigation phase: keep the
		// bar and matches, blur the field so bare n/N browse instead of
		// typing. With nothing matched there's nothing to browse, so stay
		// in editing.
		if !m.search.hasMatches() {
			return nil
		}
		m.search.editing = false
		m.searchInput.Blur()
		return nil
	case "up":
		// Browse without leaving the field: up = older.
		return m.chatSearchStep(-1)
	case "down":
		return m.chatSearchStep(1)
	}

	// Everything else (printable text, backspace, left/right, etc.) edits
	// the query field. Re-run the search only when the value changed.
	prev := m.searchInput.Value()
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	if m.searchInput.Value() != prev {
		m.runChatSearch()
	}
	return cmd
}

// handleChatSearchNavKey handles keys after the query is confirmed. The
// field is blurred, so bare n/N (vim-style, wrapping) and the arrows
// browse the matches; enter drops into native copy mode; esc/tab close;
// "/" returns to editing the query.
func (m *UI) handleChatSearchNavKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "n", "down":
		// Forward / newer (toward the bottom), wrapping like vim's n.
		return m.chatSearchStep(1)
	case "N", "up":
		// Back / older (toward the top), wrapping like vim's N.
		return m.chatSearchStep(-1)
	case "enter":
		// Drop into native copy mode with the cursor on the current match,
		// where the selection can be extended and yanked.
		if !m.search.hasMatches() {
			return nil
		}
		m.chat.EnterCopyModeAtMatch(m.search.matches[m.search.cur])
		// Close the search bar; copy mode now owns the keys.
		m.search.active = false
		m.search.editing = false
		m.updateLayoutAndSize()
		return nil
	case "esc", "ctrl+c", "tab":
		m.endChatSearch()
		return nil
	case "/":
		// Refine: jump back to editing the same query.
		m.search.editing = true
		m.searchInput.Focus()
		return nil
	}
	return nil
}

// runChatSearch recomputes matches for the current query and jumps to
// the most recent occurrence (the common case when looking back through
// history). n/N then iterate through every occurrence from there.
func (m *UI) runChatSearch() {
	m.search.matches = m.chat.FindMatches(m.searchQuery())
	if m.search.hasMatches() {
		m.search.cur = len(m.search.matches) - 1  // newest occurrence first
		m.chat.SetSearchMatches(m.search.matches) // every hit, dim
		m.chat.ApplyMatch(m.search.matches[m.search.cur])
	} else {
		m.search.cur = 0
		m.chat.ClearSearchHighlight()
	}
}

// chatSearchStep moves dir occurrences (negative = older/up, positive =
// newer/down), wrapping around the ends (… → oldest → newest → …) like
// vim's n/N, and centers the new match. A no-op when there's a single
// match (or none), since cur can't change.
func (m *UI) chatSearchStep(dir int) tea.Cmd {
	if !m.search.hasMatches() {
		return nil
	}
	n := len(m.search.matches)
	next := ((m.search.cur+dir)%n + n) % n
	if next == m.search.cur {
		return nil // single match; nothing to do
	}
	m.search.cur = next
	m.chat.ApplyMatch(m.search.matches[m.search.cur])
	return nil
}

// endChatSearch clears the active search, its matches, the match
// highlight, and the inline bar's text/focus. Called both from the bar
// itself (esc/Tab) and from exit paths that aren't the bar (Tab back to
// the editor, esc while a confirmed match is showing, sending a
// message).
func (m *UI) endChatSearch() {
	if !m.search.active && !m.search.hasMatches() {
		return
	}
	wasActive := m.search.active
	m.search = chatSearchState{}
	m.searchInput.Reset()
	m.searchInput.Blur()
	m.chat.ClearSearchHighlight()
	// Restore the conversation's focus to match the app focus — we may
	// have focused it just for the search (e.g. opened from the composer).
	if m.focus == uiFocusMain {
		m.chat.Focus()
	} else {
		m.chat.Blur()
	}
	if wasActive {
		// Reclaim the bar row for the chat.
		m.updateLayoutAndSize()
	}
}

// searchInputWidth is the width budget for the text field inside the
// inline bar: the bar width minus the inline counter/hints that share
// the row. Best-effort — at least 1 column so the field stays usable on
// narrow terminals.
func (m *UI) searchInputWidth() int {
	w := m.layout.searchBar.Dx()
	if w <= 0 {
		w = m.width
	}
	suffix := m.searchBarSuffix()
	return max(1, w-ansi.StringWidth(suffix))
}

// chatSearchSummary is the short match-counter + hints description shared
// by the inline bar. Kept as a method for the existing unit test.
func (m *UI) chatSearchSummary() string {
	if !m.search.hasMatches() {
		return "(no matches)"
	}
	return m.searchBarSuffix()
}

// chatSearchBarSuffix builds the trailing status text shown inline in
// the search bar, to the right of the query field: the match counter
// and phase-appropriate hints. Pure (no UI state) so it is unit-testable
// without a terminal.
//
//   - no query yet           -> ""
//   - query, zero matches    -> "  no matches"
//   - editing, total matches -> "  i+1/total  ·  ↑↓ browse  ·  enter → n/N"
//   - navigating             -> "  i+1/total  ·  n/N browse  ·  enter → select  ·  esc"
//
// editing selects the phase hint; while navigating, enter drops into copy
// mode at the current match.
func chatSearchBarSuffix(cur, total int, query string, editing bool) string {
	if strings.TrimSpace(query) == "" {
		return ""
	}
	if total <= 0 {
		return "  no matches"
	}
	s := fmt.Sprintf("  %d/%d", cur+1, total)
	if editing {
		// ↑/↓ browse live while typing; enter unlocks bare n/N.
		return s + "  ·  ↑↓ browse  ·  enter → n/N"
	}
	// n/N (wrapping) browse; enter drops into copy mode at the match.
	return s + "  ·  n/N browse  ·  enter → select  ·  esc"
}
