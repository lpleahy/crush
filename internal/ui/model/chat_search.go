package model

import (
	"strings"

	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/x/ansi"
)

// SearchMatch is one occurrence of a query within a chat block, with
// precomputed highlight coordinates. Line is the index of the rendered
// line within the item; StartCol/EndCol are display-width columns that
// include the MessageLeftPaddingTotal offset (the convention
// SetHighlight expects — it subtracts the offset internally).
type SearchMatch struct {
	ItemIndex int
	Line      int
	StartCol  int
	EndCol    int
}

// occurrence is the padding-free, item-free result of scanning a single
// block's rendered text. Kept separate from SearchMatch so the scanning
// logic is a pure function (string in, structs out) that's trivial to
// test without constructing a Chat.
type occurrence struct {
	line     int
	startCol int
	endCol   int
}

// occurrencesInRendered returns every case-insensitive occurrence of
// query in rendered (a block's RawRender output), per rendered line,
// left to right. Columns are display-width and content-relative (no
// padding offset). ANSI styling is stripped before matching so columns
// line up with what the user sees.
func occurrencesInRendered(rendered, query string) []occurrence {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	needle := strings.ToLower(q)
	var out []occurrence
	for ly, line := range strings.Split(rendered, "\n") {
		plain := ansi.Strip(line)
		lower := strings.ToLower(plain)
		from := 0
		for from <= len(lower) {
			rel := strings.Index(lower[from:], needle)
			if rel < 0 {
				break
			}
			abs := from + rel
			// Guard against lower/plain byte-length drift (rare; some
			// Unicode lowercases to a different byte length). Skip the
			// occurrence rather than slice out of range.
			if abs+len(q) > len(plain) {
				break
			}
			startCol := ansi.StringWidth(plain[:abs])
			endCol := startCol + ansi.StringWidth(plain[abs:abs+len(q)])
			out = append(out, occurrence{line: ly, startCol: startCol, endCol: endCol})
			from = abs + len(needle)
		}
	}
	return out
}

// FindMatches returns every occurrence of query across all selectable
// chat blocks, in display order (oldest block first, left-to-right
// within a block). Each carries its block index and highlight
// coordinates so navigation can apply the highlight without re-scanning.
func (m *Chat) FindMatches(query string) []SearchMatch {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	width := m.list.Width()
	if width <= 0 {
		width = 80
	}
	offset := chat.MessageLeftPaddingTotal

	var matches []SearchMatch
	for i := 0; i < m.list.Len(); i++ {
		if !m.isSelectable(i) {
			continue
		}
		rr, ok := m.list.ItemAt(i).(list.RawRenderable)
		if !ok {
			continue
		}
		for _, oc := range occurrencesInRendered(rr.RawRender(width), query) {
			matches = append(matches, SearchMatch{
				ItemIndex: i,
				Line:      oc.line,
				StartCol:  oc.startCol + offset,
				EndCol:    oc.endCol + offset,
			})
		}
	}
	return matches
}

// SetSearchMatches records every match (grouped by item index) so all of
// them are highlighted dim at once; the active one is highlighted bright
// via ApplyMatch. Pass nil/empty to clear just the dim layer.
func (m *Chat) SetSearchMatches(matches []SearchMatch) {
	if len(matches) == 0 {
		m.searchMatchRanges = nil
		return
	}
	grouped := make(map[int][]chat.SearchRange, len(matches))
	for _, mt := range matches {
		grouped[mt.ItemIndex] = append(grouped[mt.ItemIndex], chat.SearchRange{
			Line:     mt.Line,
			StartCol: mt.StartCol,
			EndCol:   mt.EndCol,
		})
	}
	m.searchMatchRanges = grouped
}

// ApplyMatch selects the match's block, records its highlight (applied
// by applyHighlightRange), and centers it in the viewport.
func (m *Chat) ApplyMatch(mt SearchMatch) {
	m.SetSelected(mt.ItemIndex)
	m.searchHiItem = mt.ItemIndex
	m.searchHiLine = mt.Line
	m.searchHiStartCol = mt.StartCol
	m.searchHiEndCol = mt.EndCol
	m.centerSelected()
}

// ClearSearchHighlight removes the active highlight and every dim match.
func (m *Chat) ClearSearchHighlight() {
	m.searchHiItem = -1
	m.searchMatchRanges = nil
}

// MatchViewportPos returns the position of the current search match
// relative to the chat viewport's top-left corner: col is the
// display column (already including the message left padding, since
// that's how the highlight column is stored) and row is the viewport
// row of the matched line. ok is false when there's no active match or
// its block isn't laid out. The match may still be scrolled out of
// view — callers should range-check row against the viewport height.
func (m *Chat) MatchViewportPos() (col, row int, ok bool) {
	if m.searchHiItem < 0 {
		return 0, 0, false
	}
	itemY, ok := m.list.ItemViewportY(m.searchHiItem)
	if !ok {
		return 0, 0, false
	}
	return m.searchHiStartCol, itemY + m.searchHiLine, true
}

// centerSelected scrolls so the selected item is near the vertical
// middle of the viewport. ScrollToIndex top-aligns the item; we then
// scroll up by ~half a viewport so it lands centered. Best-effort:
// items taller than the viewport simply top-align.
func (m *Chat) centerSelected() {
	sel := m.list.Selected()
	if sel < 0 {
		return
	}
	m.list.ScrollToIndex(sel)
	if h := m.Height(); h > 1 {
		m.list.ScrollBy(-(h / 2))
	}
	m.follow = m.AtBottom()
}
