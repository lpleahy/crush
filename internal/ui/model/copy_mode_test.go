package model

import "testing"

// feed sends a sequence of keys to copy mode, returning the last yanked text.
func (cm *copyMode) feed(keys ...string) string {
	var last string
	for _, k := range keys {
		if y, _ := cm.handleKey(k); y != "" {
			last = y
		}
	}
	return last
}

func TestCopyMode_BuildLines_StripsAnsiAndSeparatesBlocks(t *testing.T) {
	// Two blocks; the first carries ANSI styling that must be stripped for
	// the cursor/selection model but kept in the colored display lines.
	blocks := []string{"\x1b[31mred\x1b[0m line\nsecond", "other block"}
	colored, stripped, lineBlock := buildCopyLines(blocks)
	want := []string{"red line", "second", "", "other block"}
	if len(stripped) != len(want) {
		t.Fatalf("stripped = %#v, want %#v", stripped, want)
	}
	for i := range want {
		if stripped[i] != want[i] {
			t.Errorf("stripped %d = %q, want %q", i, stripped[i], want[i])
		}
	}
	if colored[0] != "\x1b[31mred\x1b[0m line" {
		t.Errorf("colored[0] lost its ANSI: %q", colored[0])
	}
	// block 0 spans lines 0-1, line 2 is the separator, block 1 is line 3.
	wantBlocks := []int{0, 0, -1, 1}
	for i, b := range wantBlocks {
		if lineBlock[i] != b {
			t.Errorf("lineBlock[%d] = %d, want %d", i, lineBlock[i], b)
		}
	}
}

func TestCopyMode_VisualYankCopiesCleanText(t *testing.T) {
	cm := newCopyMode([]string{"foo bar baz"})

	// viw on "bar" then yank -> clean "bar" to the clipboard.
	cm.feed("l", "l", "l", "l") // cursor onto "bar"
	got := cm.feed("v", "i", "w", "y")
	if got != "bar" {
		t.Errorf("viw yank = %q, want %q", got, "bar")
	}
	if cm.eng.Visual() {
		t.Errorf("still in visual after yank")
	}
}

func TestCopyMode_LinewiseYankAcrossLines(t *testing.T) {
	cm := newCopyMode([]string{"git clone x\n  cd x\n  make"})

	// V j j y selects the three lines and yanks them with a trailing newline.
	got := cm.feed("V", "j", "j", "y")
	want := "git clone x\n  cd x\n  make\n"
	if got != want {
		t.Errorf("linewise yank = %q, want %q", got, want)
	}
}

func TestCopyMode_TrailingPaddingTrimmed(t *testing.T) {
	// A code-block line padded with trailing spaces (the box background).
	cm := newCopyMode([]string{"  code()      "})
	cm.handleKey("$") // should land on ')' (rune 7), not the padded end
	if _, col := cm.cursor(); col != 7 {
		t.Errorf("$ landed at col %d, want 7 (last real char)", col)
	}
	if got := cm.feed("V", "y"); got != "  code()\n" {
		t.Errorf("yank = %q, want %q (trailing padding trimmed)", got, "  code()\n")
	}
}

func TestCopyMode_YankExits(t *testing.T) {
	cm := newCopyMode([]string{"foo bar"})
	cm.handleKey("v")
	cm.handleKey("e")
	txt, exit := cm.handleKey("y")
	if txt != "foo" || !exit {
		t.Errorf("yank = (%q, %v), want (\"foo\", true) so we hop back to the composer", txt, exit)
	}
}

func TestCopyMode_EditsAreInert(t *testing.T) {
	cm := newCopyMode([]string{"keep this"})
	before := cm.ta.Value()
	// Withheld edit keys must not mutate the read-only model.
	cm.feed("x", "d", "d", "d", "w", "p", "r", "z", "c", "w")
	if cm.ta.Value() != before {
		t.Errorf("model mutated by edits: %q -> %q", before, cm.ta.Value())
	}
	if cm.eng.Insert() {
		t.Errorf("stuck in insert mode after edit keys")
	}
}

func TestCopyMode_EscExitsVisualThenCopyMode(t *testing.T) {
	cm := newCopyMode([]string{"abc"})
	cm.feed("v", "l")
	if _, exit := cm.handleKey("esc"); exit {
		t.Errorf("first esc should leave visual, not copy mode")
	}
	if cm.eng.Visual() {
		t.Errorf("still visual after esc")
	}
	if _, exit := cm.handleKey("esc"); !exit {
		t.Errorf("second esc should exit copy mode")
	}
}

func TestCopyMode_DisplayCol(t *testing.T) {
	cm := newCopyMode([]string{"abc"})
	if got := cm.displayCol(0, 2); got != 2 {
		t.Errorf("displayCol ascii = %d, want 2", got)
	}
	// Wide runes count as 2 cells each; a negative column is clamped (no panic).
	wide := newCopyMode([]string{"世界x"})
	if got := wide.displayCol(0, 2); got != 4 {
		t.Errorf("displayCol wide = %d, want 4", got)
	}
	if got := wide.displayCol(0, -1); got != 0 {
		t.Errorf("displayCol negative = %d, want 0", got)
	}
}

func TestCopyMode_ViewScrollsToCursor(t *testing.T) {
	// 20 lines, viewport height 5: moving to the bottom scrolls past the edge.
	blockLines := make([]string, 20)
	for i := range blockLines {
		blockLines[i] = "line"
	}
	cm := newCopyMode([]string{joinLines(blockLines)})
	for range 19 {
		cm.handleKey("j")
	}
	_ = cm.view(40, 5)
	if cm.scroll == 0 {
		t.Errorf("viewport did not scroll to follow the cursor to the bottom")
	}
	curLine, _ := cm.cursor()
	if curLine < cm.scroll || curLine >= cm.scroll+5 {
		t.Errorf("cursor line %d not in viewport [%d,%d)", curLine, cm.scroll, cm.scroll+5)
	}
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
