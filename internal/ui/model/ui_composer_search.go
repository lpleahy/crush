package model

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// composerMatch is one occurrence of the query in the composer draft.
type composerMatch struct {
	line int
	col  int // rune column
}

// composerMatches finds every (case-insensitive) occurrence of query in the
// composer text, in reading order.
func composerMatches(text, query string) []composerMatch {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out []composerMatch
	for li, line := range strings.Split(text, "\n") {
		lower := strings.ToLower(line)
		from := 0
		for {
			rel := strings.Index(lower[from:], q)
			if rel < 0 {
				break
			}
			abs := from + rel
			out = append(out, composerMatch{line: li, col: len([]rune(line[:abs]))})
			from = abs + len(q)
		}
	}
	return out
}

// openComposerSearch opens the inline bar to search the composer draft
// (the focus-aware "/" in the composer, vim-style). The conversation/output
// search stays on the chat focus and the search_output hotkey.
func (m *UI) openComposerSearch() {
	m.search = chatSearchState{active: true, editing: true, composer: true}
	m.searchInput.Reset()
	m.searchInput.Focus()
	m.updateLayoutAndSize()
}

// handleComposerSearchKey routes keys while the composer search bar is open.
func (m *UI) handleComposerSearchKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.search.editing {
		switch msg.String() {
		case "esc", "ctrl+c", "tab":
			m.endChatSearch()
			return nil
		case "enter":
			if !m.search.hasMatches() {
				return nil
			}
			m.search.editing = false
			m.searchInput.Blur()
			return nil
		case "up":
			m.composerSearchStep(-1)
			return nil
		case "down":
			m.composerSearchStep(1)
			return nil
		}
		prev := m.searchInput.Value()
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != prev {
			m.runComposerSearch()
		}
		return cmd
	}

	// Navigation phase: bare n/N step, enter/esc close (cursor stays on the
	// match so editing resumes there).
	switch msg.String() {
	case "n", "down":
		m.composerSearchStep(1)
	case "N", "up":
		m.composerSearchStep(-1)
	case "enter", "esc", "ctrl+c", "tab":
		m.endChatSearch()
	case "/":
		m.search.editing = true
		m.searchInput.Focus()
	}
	return nil
}

// runComposerSearch recomputes matches and jumps the cursor to the first one.
func (m *UI) runComposerSearch() {
	m.search.cmatches = composerMatches(m.textarea.Value(), m.searchQuery())
	m.search.ccur = 0
	if m.search.hasMatches() {
		m.applyComposerMatch()
	}
}

// composerSearchStep moves dir matches (wrapping) and jumps the cursor.
func (m *UI) composerSearchStep(dir int) {
	n := len(m.search.cmatches)
	if n == 0 {
		return
	}
	m.search.ccur = ((m.search.ccur+dir)%n + n) % n
	m.applyComposerMatch()
}

// applyComposerMatch moves the composer cursor onto the current match.
func (m *UI) applyComposerMatch() {
	if m.search.ccur < 0 || m.search.ccur >= len(m.search.cmatches) {
		return
	}
	mt := m.search.cmatches[m.search.ccur]
	m.moveComposerCursor(mt.line, mt.col)
}

// moveComposerCursor seeks the composer cursor to (row, col).
func (m *UI) moveComposerCursor(row, col int) {
	m.textarea.MoveToBegin()
	for m.textarea.Line() < row {
		pr, pc := m.textarea.Line(), m.textarea.Column()
		m.textarea.CursorDown()
		if m.textarea.Line() == pr && m.textarea.Column() == pc {
			break // stuck (shouldn't happen for a valid row)
		}
	}
	m.textarea.SetCursorColumn(col)
}
