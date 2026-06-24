package model

import (
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/vim"
	"github.com/charmbracelet/x/ansi"
)

// copyGutter is the left margin copy mode draws before each line — matching
// the chat's 2-col "▌ " gutter width, but as plain spaces so the selection
// highlight never lands on a sidebar/border cell.
const copyGutter = 2

// copyMode is a vim-style selection layer over the conversation output: a
// cursor and visual selection driven by the vim engine over a plain-text
// (ANSI-stripped) model of the rendered blocks, so a yank copies the clean
// source text — never the sidebar, borders, or block decorations — to the
// system clipboard.
type copyMode struct {
	active bool
	ta     *copyTextarea
	eng    *vim.Engine
	// stripped is the per-line plain text the cursor/selection operate on;
	// colored is the same lines with their original ANSI styling for display.
	// Both are parallel and gutter-free (built from each block's RawRender).
	// lineBlock maps each line to its block index (-1 for a separator).
	stripped  []string
	colored   []string
	lineBlock []int
	// scroll is the first model line shown (the viewport top); the cursor
	// drives it so motions can run past the screen edges.
	scroll int
	// centerOnLine, when >= 0, recenters the view on that line on the next
	// render then clears (used on entry so the starting block is in view).
	centerOnLine int
	// styles for the selection overlay and the current-block border marker.
	selStyle    lipgloss.Style
	cursorStyle lipgloss.Style
	borderStyle lipgloss.Style
}

// copySafeKey reports whether a key should be forwarded to the engine in
// copy mode. In visual mode everything is forwarded (motions, text objects,
// y) — any edit is neutralized by the read-only model and the insert guard.
// In normal mode only cursor motions, counts, visual entry, and yank pass
// through, so i/a/o and edits can't start an insert or jiggle the cursor.
func copySafeKey(e *vim.Engine, key string) bool {
	if e.Visual() {
		return true
	}
	switch key {
	case "h", "j", "k", "l", "left", "right", "up", "down", " ",
		"w", "b", "e", "W", "B", "E", "0", "^", "$", "_", "|",
		"f", "F", "t", "T", ";", ",", "{", "}", "%", "g", "G",
		"v", "V", "y":
		return true
	}
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		return true
	}
	return false
}

// newCopyMode builds a copy-mode session from the blocks' rendered output
// (one string per block; each may contain newlines and ANSI styling). It
// starts with the cursor at the top.
func newCopyMode(blocks []string) *copyMode {
	colored, stripped, lineBlock := buildCopyLines(blocks)
	cm := &copyMode{
		active:       true,
		ta:           newCopyTextarea(strings.Join(stripped, "\n")),
		eng:          vim.New(),
		stripped:     stripped,
		colored:      colored,
		lineBlock:    lineBlock,
		centerOnLine: -1,
	}
	cm.ta.MoveToBegin()
	return cm
}

// buildCopyLines flattens the blocks' rendered output into parallel colored
// (ANSI-preserving) and stripped (plain) lines, with one blank line between
// blocks. lineBlock maps each line to its block index (-1 for a separator),
// so the renderer can mark the block the cursor is in.
func buildCopyLines(blocks []string) (colored, stripped []string, lineBlock []int) {
	for bi, b := range blocks {
		for _, ln := range strings.Split(b, "\n") {
			colored = append(colored, ln)
			// Trim trailing padding (code blocks pad lines with spaces to the
			// box width) from the cursor/selection model so $ lands on the
			// last real character and yanks don't carry trailing spaces. The
			// colored line keeps its padding so the background box still draws.
			stripped = append(stripped, strings.TrimRight(ansi.Strip(ln), " \t"))
			lineBlock = append(lineBlock, bi)
		}
		if bi < len(blocks)-1 {
			colored = append(colored, "")
			stripped = append(stripped, "")
			lineBlock = append(lineBlock, -1)
		}
	}
	if len(stripped) == 0 {
		colored, stripped, lineBlock = []string{""}, []string{""}, []int{-1}
	}
	return colored, stripped, lineBlock
}

// handleKey routes a key in copy mode. It returns any text that was yanked
// (to copy to the OS clipboard) and whether copy mode should exit.
func (cm *copyMode) handleKey(key string) (yanked string, exit bool) {
	if key == "esc" || key == "ctrl+[" {
		if cm.eng.Visual() {
			cm.eng.HandleKey(cm.ta, key) // leave visual, stay in copy mode
			return "", false
		}
		return "", true // leave copy mode
	}
	if !copySafeKey(cm.eng, key) {
		return "", false
	}
	cm.eng.HandleKey(cm.ta, key)
	if cm.eng.Insert() { // safety: copy mode never edits or inserts
		cm.eng.HandleKey(cm.ta, "esc")
	}
	if txt, ok := cm.eng.ConsumeClipboard(); ok {
		return txt, true // copied; exit copy mode (caller returns to the composer)
	}
	return "", false
}

// cursor returns the cursor's model line and rune column.
func (cm *copyMode) cursor() (line, col int) {
	return cm.ta.Line(), cm.ta.Column()
}

// displayCol converts a rune column on a model line to a display (cell)
// column, accounting for wide runes — used to place the cursor/selection
// overlay on the colored output.
func (cm *copyMode) displayCol(line, col int) int {
	if line < 0 || line >= len(cm.stripped) {
		return col
	}
	s := []rune(cm.stripped[line])
	if col < 0 {
		col = 0
	}
	if col > len(s) {
		col = len(s)
	}
	return ansi.StringWidth(string(s[:col]))
}

// moveToLine places the cursor at the start of model line n.
func (cm *copyMode) moveToLine(n int) { cm.moveToLineCol(n, 0) }

// moveToLineCol places the cursor at model line n, rune column col.
func (cm *copyMode) moveToLineCol(n, col int) {
	if n < 0 {
		n = 0
	}
	if n >= len(cm.stripped) {
		n = len(cm.stripped) - 1
	}
	cm.ta.MoveToBegin()
	for cm.ta.Line() < n {
		cm.ta.CursorDown()
	}
	if col < 0 {
		col = 0
	}
	cm.ta.SetCursorColumn(col)
}

// lineLens returns each model line's rune length (for VisualRowSpans).
func (cm *copyMode) lineLens() []int {
	ll := make([]int, len(cm.stripped))
	for i, s := range cm.stripped {
		ll[i] = len([]rune(s))
	}
	return ll
}

// followCursor scrolls the viewport so the cursor line stays visible.
func (cm *copyMode) followCursor(height int) {
	if height < 1 {
		height = 1
	}
	line, _ := cm.cursor()
	if line < cm.scroll {
		cm.scroll = line
	}
	if line >= cm.scroll+height {
		cm.scroll = line - height + 1
	}
	if cm.scroll < 0 {
		cm.scroll = 0
	}
}

// blockOf returns the block index of a model line (-1 for separators / out of
// range).
func (cm *copyMode) blockOf(line int) int {
	if line < 0 || line >= len(cm.lineBlock) {
		return -1
	}
	return cm.lineBlock[line]
}

// view renders the copy-mode viewport: height rows from cm.scroll, each as a
// gutter + colored content line. The block the cursor is in is marked with a
// "▌" border (like the focused chat block), and the visual selection is
// highlighted over the text (never the gutter).
func (cm *copyMode) view(width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	// Recenter once on entry so the starting block is in view rather than
	// scrolled to the bottom edge.
	if cm.centerOnLine >= 0 {
		maxScroll := max(0, len(cm.colored)-height)
		cm.scroll = min(max(0, cm.centerOnLine-height/2), maxScroll)
		cm.centerOnLine = -1
	}
	cm.followCursor(height)

	curLine, curCol := cm.cursor()
	curBlock := cm.blockOf(curLine)
	border := cm.borderStyle.Render("▌") + " " // marks the cursor's block
	plain := strings.Repeat(" ", copyGutter)

	rows := make([]string, height)
	for r := range height {
		ml := cm.scroll + r
		content := ""
		if ml >= 0 && ml < len(cm.colored) {
			content = cm.colored[ml]
		}
		gutter := plain
		if curBlock >= 0 && cm.blockOf(ml) == curBlock {
			gutter = border
		}
		rows[r] = gutter + content
	}
	out := strings.Join(rows, "\n")
	area := image.Rect(0, 0, width, height)

	if cm.eng.Visual() {
		hl := list.ToHighlighter(cm.selStyle)
		for _, sp := range cm.eng.VisualRowSpans(curLine, curCol, cm.lineLens()) {
			vr := sp.Row - cm.scroll
			if vr < 0 || vr >= height {
				continue
			}
			c0 := copyGutter + cm.displayCol(sp.Row, sp.StartCol)
			c1 := copyGutter + cm.displayCol(sp.Row, sp.EndCol)
			out = list.Highlight(out, area, vr, c0, vr, c1, hl)
		}
	}

	return out
}

// cursorScreenPos returns the cursor's (col, row) within the rendered view
// (relative to its top-left), using the scroll set by the last view() call.
// The caller offsets it by the chat area origin to place the terminal cursor.
func (cm *copyMode) cursorScreenPos() (col, row int) {
	line, c := cm.cursor()
	return copyGutter + cm.displayCol(line, c), line - cm.scroll
}

// copyTextarea is a read-only [vim.Textarea] over a rune buffer. The engine
// drives the cursor through it; SetValue is a no-op so withheld edit
// commands can never mutate the model.
type copyTextarea struct {
	lines    [][]rune
	row, col int
}

func newCopyTextarea(text string) *copyTextarea {
	t := &copyTextarea{}
	t.set(text)
	return t
}

func (t *copyTextarea) set(s string) {
	parts := strings.Split(s, "\n")
	t.lines = make([][]rune, len(parts))
	for i, p := range parts {
		t.lines[i] = []rune(p)
	}
	if t.row >= len(t.lines) {
		t.row = len(t.lines) - 1
	}
	if t.col > len(t.lines[t.row]) {
		t.col = len(t.lines[t.row])
	}
}

func (t *copyTextarea) Value() string {
	out := make([]string, len(t.lines))
	for i, l := range t.lines {
		out[i] = string(l)
	}
	return strings.Join(out, "\n")
}

func (t *copyTextarea) SetValue(string) {} // read-only

func (t *copyTextarea) Line() int    { return t.row }
func (t *copyTextarea) Column() int  { return t.col }
func (t *copyTextarea) MoveToBegin() { t.row, t.col = 0, 0 }

func (t *copyTextarea) CursorDown() {
	if t.row < len(t.lines)-1 {
		t.row++
		if t.col > len(t.lines[t.row]) {
			t.col = len(t.lines[t.row])
		}
	}
}

func (t *copyTextarea) SetCursorColumn(c int) {
	if c < 0 {
		c = 0
	}
	if c > len(t.lines[t.row]) {
		c = len(t.lines[t.row])
	}
	t.col = c
}
