package vim

import (
	"strings"
	"testing"
)

// fakeTA emulates the bubbles textarea's logical-line cursor behavior
// (no soft-wrap, so logical == visual), which is all the engine relies
// on. SetValue leaves the cursor at the end, like the real textarea.
type fakeTA struct {
	lines    [][]rune
	row, col int
}

func newFakeTA(s string) *fakeTA {
	f := &fakeTA{}
	f.SetValue(s)
	return f
}

func (f *fakeTA) at(row, col int) { f.row, f.col = row, col }
func (f *fakeTA) Value() string   { return fromRunes(f.lines) }
func (f *fakeTA) Line() int       { return f.row }
func (f *fakeTA) Column() int     { return f.col }
func (f *fakeTA) MoveToBegin()    { f.row, f.col = 0, 0 }

func (f *fakeTA) SetValue(s string) {
	f.lines = toRunes(s)
	f.row = len(f.lines) - 1
	f.col = len(f.lines[f.row])
}

func (f *fakeTA) CursorDown() {
	if f.row < len(f.lines)-1 {
		f.row++
		if f.col > len(f.lines[f.row]) {
			f.col = len(f.lines[f.row])
		}
	}
}

func (f *fakeTA) SetCursorColumn(c int) {
	f.col = clamp(c, 0, len(f.lines[f.row]))
}

func run(initial string, row, col int, keys ...string) (*fakeTA, *Engine) {
	ta := newFakeTA(initial)
	ta.at(row, col)
	e := New()
	for _, k := range keys {
		e.HandleKey(ta, k)
	}
	return ta, e
}

func TestEngine_MotionsAndEdits(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
		wantInsert       bool
	}{
		// motions
		{"l moves right", "hello", 0, 0, []string{"l"}, "hello", 0, 1, false},
		{"l clamps at last char", "hi", 0, 1, []string{"l"}, "hi", 0, 1, false},
		{"h moves left", "hello", 0, 3, []string{"h"}, "hello", 0, 2, false},
		{"h clamps at col 0", "hi", 0, 0, []string{"h"}, "hi", 0, 0, false},
		{"3l counts", "hello", 0, 0, []string{"3", "l"}, "hello", 0, 3, false},
		{"count resets after use", "hello", 0, 0, []string{"3", "l", "l"}, "hello", 0, 4, false},
		{"0 to line start", "hello", 0, 3, []string{"0"}, "hello", 0, 0, false},
		{"$ to line end", "hello", 0, 1, []string{"$"}, "hello", 0, 4, false},
		{"j down keeps col", "abc\ndef", 0, 1, []string{"j"}, "abc\ndef", 1, 1, false},
		{"j clamps col on short line", "abc\nd", 0, 2, []string{"j"}, "abc\nd", 1, 0, false},
		{"k up", "abc\ndef", 1, 2, []string{"k"}, "abc\ndef", 0, 2, false},
		{"gg to top", "a\nb\nc", 2, 0, []string{"g", "g"}, "a\nb\nc", 0, 0, false},
		{"G to bottom", "a\nb\nc", 0, 0, []string{"G"}, "a\nb\nc", 2, 0, false},
		{"w next word", "foo bar baz", 0, 0, []string{"w"}, "foo bar baz", 0, 4, false},
		{"w stops on punct", "foo.bar", 0, 0, []string{"w"}, "foo.bar", 0, 3, false},
		{"w crosses lines", "foo\nbar", 0, 2, []string{"w"}, "foo\nbar", 1, 0, false},
		{"b prev word", "foo bar", 0, 4, []string{"b"}, "foo bar", 0, 0, false},
		{"e word end", "foo bar", 0, 0, []string{"e"}, "foo bar", 0, 2, false},
		{"2w counts", "a b c d", 0, 0, []string{"2", "w"}, "a b c d", 0, 4, false},

		// edits
		{"x deletes char", "abc", 0, 1, []string{"x"}, "ac", 0, 1, false},
		{"x at end clamps cursor", "abc", 0, 2, []string{"x"}, "ab", 0, 1, false},
		{"3x deletes three", "abcdef", 0, 1, []string{"3", "x"}, "aef", 0, 1, false},
		{"D to line end", "hello", 0, 2, []string{"D"}, "he", 0, 1, false},
		{"dd deletes line", "a\nb\nc", 1, 0, []string{"d", "d"}, "a\nc", 1, 0, false},
		{"dd last line moves up", "a\nb", 1, 0, []string{"d", "d"}, "a", 0, 0, false},
		{"dd only line empties", "abc", 0, 0, []string{"d", "d"}, "", 0, 0, false},
		{"2dd deletes two", "a\nb\nc\nd", 0, 0, []string{"2", "d", "d"}, "c\nd", 0, 0, false},
		{"dw deletes word+space", "foo bar", 0, 0, []string{"d", "w"}, "bar", 0, 0, false},
		{"dw last word to eol", "foo bar", 0, 4, []string{"d", "w"}, "foo ", 0, 3, false},

		// insert-mode entry positions (cursor only; content unchanged for i/a/I/A)
		{"i keeps cursor", "abc", 0, 1, []string{"i"}, "abc", 0, 1, true},
		{"a appends", "abc", 0, 1, []string{"a"}, "abc", 0, 2, true},
		{"A to line end", "abc", 0, 0, []string{"A"}, "abc", 0, 3, true},
		{"I to first non-blank", "  abc", 0, 4, []string{"I"}, "  abc", 0, 2, true},
		{"o opens line below", "abc", 0, 1, []string{"o"}, "abc\n", 1, 0, true},
		{"O opens line above", "abc", 0, 0, []string{"O"}, "\nabc", 0, 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
			if e.Insert() != tc.wantInsert {
				t.Errorf("insert = %v, want %v", e.Insert(), tc.wantInsert)
			}
		})
	}
}

func TestEngine_EscapeLeavesInsertAndMovesLeft(t *testing.T) {
	t.Parallel()
	ta, e := run("abc", 0, 1, "a") // insert after col 1 -> cursor at col 2, insert mode
	if !e.Insert() || ta.Column() != 2 {
		t.Fatalf("after 'a': insert=%v col=%d, want insert col 2", e.Insert(), ta.Column())
	}
	e.HandleKey(ta, "esc")
	if e.Insert() {
		t.Error("esc should leave insert mode")
	}
	if ta.Column() != 1 {
		t.Errorf("esc should move cursor left to 1, got %d", ta.Column())
	}
}

func TestEngine_InsertModePassesKeysThrough(t *testing.T) {
	t.Parallel()
	ta := newFakeTA("abc")
	ta.at(0, 1)
	e := New()
	e.HandleKey(ta, "i") // enter insert
	if handled := e.HandleKey(ta, "z"); handled {
		t.Error("insert-mode 'z' should pass through (handled=false)")
	}
	if handled := e.HandleKey(ta, "esc"); !handled {
		t.Error("insert-mode 'esc' should be consumed (handled=true)")
	}
}

func TestEngine_NormalModeConsumesKeys(t *testing.T) {
	t.Parallel()
	ta := newFakeTA("abc")
	ta.at(0, 0)
	e := New()
	for _, k := range []string{"l", "x", "j", "w", "g"} {
		if !e.HandleKey(ta, k) {
			t.Errorf("normal-mode %q should be consumed", k)
		}
	}
}

func TestEngine_Undo(t *testing.T) {
	t.Parallel()

	ta, e := run("abc", 0, 1, "x") // -> "ac"
	if ta.Value() != "ac" {
		t.Fatalf("after x: %q", ta.Value())
	}
	e.HandleKey(ta, "u")
	if ta.Value() != "abc" {
		t.Errorf("after undo: %q, want abc", ta.Value())
	}
	if ta.Line() != 0 || ta.Column() != 1 {
		t.Errorf("undo cursor = (%d,%d), want (0,1)", ta.Line(), ta.Column())
	}
	// Undo with empty stack is a no-op.
	e.HandleKey(ta, "u")
	if ta.Value() != "abc" {
		t.Errorf("extra undo changed value: %q", ta.Value())
	}
}

func TestEngine_UndoStacksMultipleEdits(t *testing.T) {
	t.Parallel()
	ta, e := run("abcdef", 0, 0, "x", "x") // -> "cdef"
	if ta.Value() != "cdef" {
		t.Fatalf("after xx: %q", ta.Value())
	}
	e.HandleKey(ta, "u")
	if ta.Value() != "bcdef" {
		t.Errorf("after 1 undo: %q, want bcdef", ta.Value())
	}
	e.HandleKey(ta, "u")
	if ta.Value() != "abcdef" {
		t.Errorf("after 2 undo: %q, want abcdef", ta.Value())
	}
}

func TestEngine_Redo(t *testing.T) {
	t.Parallel()

	ta, e := run("abc", 0, 1, "x") // -> "ac"
	e.HandleKey(ta, "u")
	if ta.Value() != "abc" {
		t.Fatalf("after undo: %q, want abc", ta.Value())
	}
	e.HandleKey(ta, "ctrl+r")
	if ta.Value() != "ac" {
		t.Errorf("after redo: %q, want ac", ta.Value())
	}
	// Redo with empty stack is a no-op.
	e.HandleKey(ta, "ctrl+r")
	if ta.Value() != "ac" {
		t.Errorf("extra redo changed value: %q", ta.Value())
	}
}

func TestEngine_RedoClearedByNewEdit(t *testing.T) {
	t.Parallel()

	ta, e := run("abcdef", 0, 0, "x") // -> "bcdef"
	e.HandleKey(ta, "u")              // -> "abcdef", redo holds "bcdef"
	if ta.Value() != "abcdef" {
		t.Fatalf("after undo: %q", ta.Value())
	}
	e.HandleKey(ta, "x") // a fresh edit must drop the redo history
	if ta.Value() != "bcdef" {
		t.Fatalf("after new edit: %q", ta.Value())
	}
	e.HandleKey(ta, "ctrl+r") // redo is now empty -> no-op
	if ta.Value() != "bcdef" {
		t.Errorf("redo after a new edit should be a no-op, got %q", ta.Value())
	}
}

// TestEngine_UndoRedoInsertSession treats a whole insert session as one
// undo step. The fake textarea doesn't apply pass-through insert keys, so
// the typed text is simulated with SetValue between 'i' and esc.
func TestEngine_UndoRedoInsertSession(t *testing.T) {
	t.Parallel()

	ta, e := run("abc", 0, 0, "i") // enter insert at col 0
	if !e.Insert() {
		t.Fatal("'i' should enter insert mode")
	}
	ta.SetValue("XYabc") // simulate typing "XY"
	e.HandleKey(ta, "esc")
	if e.Insert() {
		t.Fatal("esc should leave insert mode")
	}
	if ta.Value() != "XYabc" {
		t.Fatalf("after insert: %q", ta.Value())
	}
	e.HandleKey(ta, "u") // one step undoes the whole session
	if ta.Value() != "abc" {
		t.Errorf("undo insert session = %q, want abc", ta.Value())
	}
	e.HandleKey(ta, "ctrl+r")
	if ta.Value() != "XYabc" {
		t.Errorf("redo insert session = %q, want XYabc", ta.Value())
	}
}

// TestEngine_NoopInsertCreatesNoUndoStep verifies that entering and
// leaving insert mode without changing the buffer doesn't push an undo
// step — so a following 'u' undoes the prior real edit, not the no-op.
func TestEngine_NoopInsertCreatesNoUndoStep(t *testing.T) {
	t.Parallel()

	ta, e := run("abc", 0, 1, "x") // -> "ac"
	if ta.Value() != "ac" {
		t.Fatalf("after x: %q", ta.Value())
	}
	e.HandleKey(ta, "i")
	e.HandleKey(ta, "esc") // no typing -> no undo step
	e.HandleKey(ta, "u")
	if ta.Value() != "abc" {
		t.Errorf("undo after no-op insert = %q, want abc (the x), not a no-op", ta.Value())
	}
}

// TestEngine_UndoRedoOpenLine covers o/O: opening a line is part of the
// insert session, undone/redone as one step.
func TestEngine_UndoRedoOpenLine(t *testing.T) {
	t.Parallel()

	ta, e := run("abc", 0, 0, "o") // -> "abc\n", insert mode
	if ta.Value() != "abc\n" {
		t.Fatalf("after o: %q", ta.Value())
	}
	e.HandleKey(ta, "esc")
	e.HandleKey(ta, "u")
	if ta.Value() != "abc" {
		t.Errorf("undo o = %q, want abc", ta.Value())
	}
	e.HandleKey(ta, "ctrl+r")
	if ta.Value() != "abc\n" {
		t.Errorf("redo o = %q, want abc\\n", ta.Value())
	}
}

// TestEngine_BreakUndoSeparatesPastes covers the fix for two pastes in
// one insert session: BreakUndo (called before each paste) makes each its
// own undo step, so `u` removes only the most recent paste — matching the
// behavior of pasting across two separate insert sessions. The fake
// textarea doesn't apply pass-through keys, so pastes are simulated with
// SetValue after the pre-paste BreakUndo.
func TestEngine_BreakUndoSeparatesPastes(t *testing.T) {
	t.Parallel()

	ta, e := run("", 0, 0, "i") // insert mode, empty buffer
	if !e.Insert() {
		t.Fatal("'i' should enter insert mode")
	}

	e.BreakUndo(ta) // before paste A
	ta.SetValue("AAA")
	e.BreakUndo(ta) // before paste B
	ta.SetValue("AAABBB")

	e.HandleKey(ta, "esc")
	if ta.Value() != "AAABBB" {
		t.Fatalf("after two pastes: %q", ta.Value())
	}

	// First undo removes only the most recent paste.
	e.HandleKey(ta, "u")
	if ta.Value() != "AAA" {
		t.Errorf("after 1 undo: %q, want AAA (only paste B removed)", ta.Value())
	}
	// Second undo removes the first paste.
	e.HandleKey(ta, "u")
	if ta.Value() != "" {
		t.Errorf("after 2 undo: %q, want empty", ta.Value())
	}
	// Redo replays them one paste at a time.
	e.HandleKey(ta, "ctrl+r")
	if ta.Value() != "AAA" {
		t.Errorf("after redo: %q, want AAA", ta.Value())
	}
}

// TestEngine_BreakUndoNoopOutsideInsert ensures BreakUndo does nothing in
// normal mode (no stray undo steps from a normal-mode paste path).
func TestEngine_BreakUndoNoopOutsideInsert(t *testing.T) {
	t.Parallel()

	ta, e := run("abc", 0, 0) // normal mode, no keys
	e.BreakUndo(ta)
	e.HandleKey(ta, "u")
	if ta.Value() != "abc" {
		t.Errorf("BreakUndo in normal mode should not create an undo step, got %q", ta.Value())
	}
}

// TestEngine_OperatorMotionsAndObjects covers the generalized
// operator-pending engine: d/c composed with motions, f/F/t/T, and the
// iw/aw text objects, plus the standalone first-non-blank and find
// motions. The change operator (c) leaves the engine in insert mode.
func TestEngine_OperatorMotionsAndObjects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
		wantInsert       bool
	}{
		// d + charwise motions
		{"d0", "foo bar", 0, 5, []string{"d", "0"}, "ar", 0, 0, false},
		{"d$", "hello", 0, 2, []string{"d", "$"}, "he", 0, 1, false},
		{"dh", "abc", 0, 2, []string{"d", "h"}, "ac", 0, 1, false},
		{"dl", "abc", 0, 1, []string{"d", "l"}, "ac", 0, 1, false},
		{"db", "foo bar", 0, 6, []string{"d", "b"}, "foo r", 0, 4, false},
		{"de", "foo bar", 0, 0, []string{"d", "e"}, " bar", 0, 0, false},
		{"d^", "  foo", 0, 4, []string{"d", "^"}, "  o", 0, 2, false},

		// d + linewise motions
		{"dj", "a\nb\nc", 0, 0, []string{"d", "j"}, "c", 0, 0, false},
		{"dk", "a\nb\nc", 2, 0, []string{"d", "k"}, "a", 0, 0, false},
		{"dgg", "a\nb\nc", 2, 0, []string{"d", "g", "g"}, "", 0, 0, false},
		{"dG", "a\nb\nc", 1, 0, []string{"d", "G"}, "a", 0, 0, false},
		{"d_", "a\nb\nc", 0, 0, []string{"d", "_"}, "b\nc", 0, 0, false},

		// counts: d2j and 2dd both delete the right number of lines
		{"d2j", "a\nb\nc\nd", 0, 0, []string{"d", "2", "j"}, "d", 0, 0, false},
		{"2dd", "a\nb\nc\nd", 0, 0, []string{"2", "d", "d"}, "c\nd", 0, 0, false},

		// d + f/t/F/T
		{"df;", "foo;bar", 0, 0, []string{"d", "f", ";"}, "bar", 0, 0, false},
		{"dt;", "foo;bar", 0, 0, []string{"d", "t", ";"}, ";bar", 0, 0, false},
		{"dF.", "abc.def", 0, 6, []string{"d", "F", "."}, "abcf", 0, 3, false},
		{"dT.", "abc.def", 0, 6, []string{"d", "T", "."}, "abc.f", 0, 4, false},

		// text objects
		{"diw", "foo bar baz", 0, 5, []string{"d", "i", "w"}, "foo  baz", 0, 4, false},
		{"daw", "foo bar baz", 0, 5, []string{"d", "a", "w"}, "foo baz", 0, 4, false},

		// standalone motions
		{"^ first non-blank", "  foo", 0, 4, []string{"^"}, "  foo", 0, 2, false},
		{"_ first non-blank", "  abc", 0, 4, []string{"_"}, "  abc", 0, 2, false},
		{"f forward", "foo;bar", 0, 0, []string{"f", ";"}, "foo;bar", 0, 3, false},
		{"t forward", "foo;bar", 0, 0, []string{"t", ";"}, "foo;bar", 0, 2, false},
		{"F backward", "abc.def", 0, 6, []string{"F", "."}, "abc.def", 0, 3, false},
		{"T backward", "abc.def", 0, 6, []string{"T", "."}, "abc.def", 0, 4, false},

		// change operator (enters insert mode)
		{"cw acts like ce", "foo bar", 0, 0, []string{"c", "w"}, " bar", 0, 0, true},
		{"c$", "hello", 0, 3, []string{"c", "$"}, "hel", 0, 3, true},
		{"ciw", "foo bar baz", 0, 5, []string{"c", "i", "w"}, "foo  baz", 0, 4, true},
		{"cc clears the line", "abc", 0, 0, []string{"c", "c"}, "", 0, 0, true},
		{"2cc clears two lines", "a\nb\nc", 0, 0, []string{"2", "c", "c"}, "\nc", 0, 0, true},
		{"ct;", "foo;bar", 0, 0, []string{"c", "t", ";"}, ";bar", 0, 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
			if e.Insert() != tc.wantInsert {
				t.Errorf("insert = %v, want %v", e.Insert(), tc.wantInsert)
			}
		})
	}
}

// TestEngine_ChangeIsOneUndo verifies a change (c) — the delete plus the
// typed replacement — is a single undo step.
func TestEngine_ChangeIsOneUndo(t *testing.T) {
	t.Parallel()

	ta, e := run("foo bar", 0, 0, "c", "w") // delete "foo", insert mode
	if !e.Insert() || ta.Value() != " bar" {
		t.Fatalf("after cw: insert=%v value=%q", e.Insert(), ta.Value())
	}
	ta.SetValue("hello bar") // simulate typing "hello"
	e.HandleKey(ta, "esc")
	if ta.Value() != "hello bar" {
		t.Fatalf("after typing: %q", ta.Value())
	}
	e.HandleKey(ta, "u") // one undo restores the whole pre-change buffer
	if ta.Value() != "foo bar" {
		t.Errorf("undo cw = %q, want foo bar", ta.Value())
	}
}

// TestEngine_YankAndPaste covers the y operator (charwise, linewise, and
// text objects), the x/D/dw "yank-on-delete" registers, and p/P paste.
func TestEngine_YankAndPaste(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
	}{
		// linewise yank + paste
		{"yyp dup line below", "a\nb\nc", 0, 0, []string{"y", "y", "p"}, "a\na\nb\nc", 1, 0},
		{"yyP dup line above", "a\nb\nc", 1, 0, []string{"y", "y", "P"}, "a\nb\nb\nc", 1, 0},
		{"2yyp yanks two lines", "a\nb\nc", 0, 0, []string{"2", "y", "y", "p"}, "a\na\nb\nb\nc", 1, 0},
		{"ddp moves line down", "a\nb\nc", 0, 0, []string{"d", "d", "p"}, "b\na\nc", 1, 0},

		// charwise yank + paste
		{"y$ then p", "abc", 0, 0, []string{"y", "$", "p"}, "aabcbc", 0, 3},
		{"yl then P", "abc", 0, 0, []string{"y", "l", "P"}, "aabc", 0, 0},
		{"yiw then p", "foo bar baz", 0, 5, []string{"y", "i", "w", "p"}, "foo bbarar baz", 0, 7},
		{"yw then p", "foo bar", 0, 0, []string{"y", "w", "p"}, "ffoo oo bar", 0, 4},

		// delete also yanks (register reused by p/P)
		{"xp transposes", "ab", 0, 0, []string{"x", "p"}, "ba", 0, 1},
		{"Dp re-pastes tail", "hello", 0, 2, []string{"D", "p"}, "hello", 0, 4},
		{"dwp", "foo bar", 0, 0, []string{"d", "w", "p"}, "bfoo ar", 0, 4},

		// yank leaves the buffer untouched, cursor at range start
		{"yiw no-op buffer", "foo bar baz", 0, 5, []string{"y", "i", "w"}, "foo bar baz", 0, 4},
		{"yy no-op buffer", "a\nb\nc", 1, 0, []string{"y", "y"}, "a\nb\nc", 1, 0},

		// nothing yanked yet: p/P are no-ops
		{"p with empty register", "abc", 0, 1, []string{"p"}, "abc", 0, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
			if e.Insert() {
				t.Errorf("insert = true, want false")
			}
		})
	}
}

// TestEngine_WordMotionsAndObjects covers WORD motions (W/B/E) and the
// text objects beyond iw/aw: iW/aW, quotes, and brackets.
func TestEngine_WordMotionsAndObjects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
		wantInsert       bool
	}{
		// WORD motions ignore punctuation; contrast with w/e.
		{"W skips punctuation", "foo.bar baz", 0, 0, []string{"W"}, "foo.bar baz", 0, 8, false},
		{"w stops at punctuation", "foo.bar baz", 0, 0, []string{"w"}, "foo.bar baz", 0, 3, false},
		{"E to WORD end", "foo.bar baz", 0, 0, []string{"E"}, "foo.bar baz", 0, 6, false},
		{"B back a WORD", "foo.bar baz", 0, 8, []string{"B"}, "foo.bar baz", 0, 0, false},

		// WORD operators
		{"dW deletes WORD+space", "foo.bar baz", 0, 0, []string{"d", "W"}, "baz", 0, 0, false},
		{"dE to WORD end", "foo.bar baz", 0, 0, []string{"d", "E"}, " baz", 0, 0, false},
		{"cW acts like cE", "foo.bar baz", 0, 0, []string{"c", "W"}, " baz", 0, 0, true},

		// WORD vs word text objects
		{"diW whole WORD", "foo.bar baz", 0, 2, []string{"d", "i", "W"}, " baz", 0, 0, false},
		{"diw just word run", "foo.bar baz", 0, 0, []string{"d", "i", "w"}, ".bar baz", 0, 0, false},
		{"daW WORD+space", "foo.bar baz", 0, 2, []string{"d", "a", "W"}, "baz", 0, 0, false},

		// quote text objects
		{"di-double-quote", `a "bc" d`, 0, 3, []string{"d", "i", `"`}, `a "" d`, 0, 3, false},
		{"da-double-quote", `a "bc" d`, 0, 3, []string{"d", "a", `"`}, "a d", 0, 2, false},
		{"ci-double-quote", `a "bc" d`, 0, 3, []string{"c", "i", `"`}, `a "" d`, 0, 3, true},
		{"di-single-quote", `x 'ab' y`, 0, 4, []string{"d", "i", "'"}, `x '' y`, 0, 3, false},

		// bracket text objects (incl. b/B aliases and cursor-on-close)
		{"di-paren", "foo(bar)baz", 0, 4, []string{"d", "i", "("}, "foo()baz", 0, 4, false},
		{"da-paren", "foo(bar)baz", 0, 4, []string{"d", "a", "("}, "foobaz", 0, 3, false},
		{"di-b-alias", "foo(bar)baz", 0, 4, []string{"d", "i", "b"}, "foo()baz", 0, 4, false},
		{"di-on-close-paren", "foo(bar)baz", 0, 7, []string{"d", "i", ")"}, "foo()baz", 0, 4, false},
		{"di-brace", "x{ab}y", 0, 2, []string{"d", "i", "{"}, "x{}y", 0, 2, false},
		{"di-bracket", "x[ab]y", 0, 2, []string{"d", "i", "["}, "x[]y", 0, 2, false},
		{"yi-paren fills register", "foo(bar)baz", 0, 4, []string{"y", "i", "(", "$", "p"}, "foo(bar)bazbar", 0, 13, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
			if e.Insert() != tc.wantInsert {
				t.Errorf("insert = %v, want %v", e.Insert(), tc.wantInsert)
			}
		})
	}
}

// TestEngine_EditShortcuts covers C/S/Y/s/X/r/~/J/gJ.
func TestEngine_EditShortcuts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
		wantInsert       bool
	}{
		// C / S / Y
		{"C change to eol", "hello", 0, 2, []string{"C"}, "he", 0, 2, true},
		{"S clears line + insert", "abc\ndef", 0, 1, []string{"S"}, "\ndef", 0, 0, true},
		{"2S clears two lines", "a\nb\nc", 0, 0, []string{"2", "S"}, "\nc", 0, 0, true},
		{"Y then p dup line", "a\nb", 0, 0, []string{"Y", "p"}, "a\na\nb", 1, 0, false},

		// s / X
		{"s substitute char", "abc", 0, 1, []string{"s"}, "ac", 0, 1, true},
		{"2s substitute two", "abcd", 0, 1, []string{"2", "s"}, "ad", 0, 1, true},
		{"X delete before", "abc", 0, 2, []string{"X"}, "ac", 0, 1, false},
		{"2X delete two before", "abcd", 0, 3, []string{"2", "X"}, "ad", 0, 1, false},
		{"X at col0 no-op", "abc", 0, 0, []string{"X"}, "abc", 0, 0, false},

		// r
		{"r replaces char", "abc", 0, 1, []string{"r", "z"}, "azc", 0, 1, false},
		{"3r replaces three", "abcde", 0, 1, []string{"3", "r", "x"}, "axxxe", 0, 3, false},
		{"r count past eol no-op", "ab", 0, 0, []string{"3", "r", "x"}, "ab", 0, 0, false},
		{"r esc cancels", "abc", 0, 1, []string{"r", "esc"}, "abc", 0, 1, false},

		// ~
		{"~ toggles + advances", "abc", 0, 0, []string{"~"}, "Abc", 0, 1, false},
		{"~ upper to lower", "ABC", 0, 0, []string{"~"}, "aBC", 0, 1, false},
		{"2~ toggles two", "abc", 0, 0, []string{"2", "~"}, "ABc", 0, 2, false},
		{"~ skips non-letter", "a.b", 0, 1, []string{"~"}, "a.b", 0, 2, false},

		// J / gJ
		{"J joins with space", "foo\nbar", 0, 0, []string{"J"}, "foo bar", 0, 3, false},
		{"J collapses leading blanks", "foo\n   bar", 0, 0, []string{"J"}, "foo bar", 0, 3, false},
		{"gJ joins without space", "foo\nbar", 0, 0, []string{"g", "J"}, "foobar", 0, 3, false},
		{"3J joins three lines", "a\nb\nc\nd", 0, 0, []string{"3", "J"}, "a b c\nd", 0, 3, false},
		{"J on last line no-op", "abc", 0, 0, []string{"J"}, "abc", 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
			if e.Insert() != tc.wantInsert {
				t.Errorf("insert = %v, want %v", e.Insert(), tc.wantInsert)
			}
		})
	}
}

// TestEngine_MoreMotions covers ; , % { } g_ | ge gE.
func TestEngine_MoreMotions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
	}{
		// ; and , repeat the last f/t
		{"; repeats f", "a.b.c.d", 0, 0, []string{"f", ".", ";"}, "a.b.c.d", 0, 3},
		{", reverses f", "a.b.c.d", 0, 0, []string{"f", ".", ";", ","}, "a.b.c.d", 0, 1},
		{"; repeats t without sticking", "a.b.c", 0, 0, []string{"t", ".", ";"}, "a.b.c", 0, 2},

		// % matches brackets (and operates)
		{"% to matching close", "a(bc)d", 0, 0, []string{"%"}, "a(bc)d", 0, 4},
		{"% to matching open", "a(bc)d", 0, 4, []string{"%"}, "a(bc)d", 0, 1},
		{"d% deletes the pair", "a(bc)d", 0, 1, []string{"d", "%"}, "ad", 0, 1},

		// paragraph motions
		{"} to next blank", "a\n\nb", 0, 0, []string{"}"}, "a\n\nb", 1, 0},
		{"{ to prev blank", "a\n\nb", 2, 0, []string{"{"}, "a\n\nb", 1, 0},

		// g_ last non-blank, ge/gE backward word end
		{"g_ last non-blank", "  foo  ", 0, 0, []string{"g", "_"}, "  foo  ", 0, 4},
		{"ge prev word end", "foo bar", 0, 5, []string{"g", "e"}, "foo bar", 0, 2},
		{"gE prev WORD end", "foo.bar baz", 0, 8, []string{"g", "E"}, "foo.bar baz", 0, 6},

		// | to column
		{"3| to column 3", "hello", 0, 0, []string{"3", "|"}, "hello", 0, 2},
		{"| to column 1", "hello", 0, 4, []string{"|"}, "hello", 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, _ := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
		})
	}
}

// TestEngine_CaseOperators covers gu/gU/g~ with motions, text objects, the
// doubled linewise forms, and the visual u/U/~.
func TestEngine_CaseOperators(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
		wantMode         Mode
	}{
		// gu / gU / g~ with a word motion
		{"guw lowercases word", "FOO BAR", 0, 0, []string{"g", "u", "w"}, "foo BAR", 0, 0, ModeNormal},
		{"gUw uppercases word", "foo bar", 0, 0, []string{"g", "U", "w"}, "FOO bar", 0, 0, ModeNormal},
		{"g~w toggles word", "Foo bar", 0, 0, []string{"g", "~", "w"}, "fOO bar", 0, 0, ModeNormal},

		// to end of line and text objects
		{"gU$ to eol", "foo bar", 0, 4, []string{"g", "U", "$"}, "foo BAR", 0, 4, ModeNormal},
		{"gUiW WORD object", "foo.bar baz", 0, 2, []string{"g", "U", "i", "W"}, "FOO.BAR baz", 0, 0, ModeNormal},
		{"guiw word object", "FOO bar", 0, 1, []string{"g", "u", "i", "w"}, "foo bar", 0, 0, ModeNormal},

		// doubled linewise forms
		{"guu lowercases line", "FOO\nBAR", 0, 0, []string{"g", "u", "u"}, "foo\nBAR", 0, 0, ModeNormal},
		{"gUU uppercases line", "foo\nbar", 0, 0, []string{"g", "U", "U"}, "FOO\nbar", 0, 0, ModeNormal},
		{"g~~ toggles line", "Foo\nbar", 0, 0, []string{"g", "~", "~"}, "fOO\nbar", 0, 0, ModeNormal},

		// visual case ops
		{"visual U on selection", "foo bar", 0, 0, []string{"v", "e", "U"}, "FOO bar", 0, 0, ModeNormal},
		{"viw U", "foo bar baz", 0, 5, []string{"v", "i", "w", "U"}, "foo BAR baz", 0, 4, ModeNormal},
		{"V u lowercases line", "FOO\nBAR", 0, 0, []string{"V", "u"}, "foo\nBAR", 0, 0, ModeNormal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
			if e.Mode() != tc.wantMode {
				t.Errorf("mode = %v, want %v", e.Mode(), tc.wantMode)
			}
		})
	}
}

// TestEngine_Indent covers >> / << with counts and motions (default 2-space
// indent), plus a custom tab indent via SetIndent.
func TestEngine_Indent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
	}{
		{">> indents the line", "foo", 0, 0, []string{">", ">"}, "  foo", 0, 2},
		{"<< dedents the line", "  foo", 0, 0, []string{"<", "<"}, "foo", 0, 0},
		{"2>> indents two lines", "a\nb\nc", 0, 0, []string{"2", ">", ">"}, "  a\n  b\nc", 0, 2},
		{">j indents current+next", "a\nb\nc", 0, 0, []string{">", "j"}, "  a\n  b\nc", 0, 2},
		{">} indents to paragraph", "a\nb\n\nc", 0, 0, []string{">", "}"}, "  a\n  b\n\nc", 0, 2},
		{">G indents to end", "a\nb\nc", 0, 0, []string{">", "G"}, "  a\n  b\n  c", 0, 2},
		{"<< removes a leading tab", "\tfoo", 0, 0, []string{"<", "<"}, "foo", 0, 0},
		{">> leaves empty line alone", "", 0, 0, []string{">", ">"}, "", 0, 0},
		{"<< on unindented line no-op", "foo", 0, 0, []string{"<", "<"}, "foo", 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, _ := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
		})
	}
}

// TestEngine_IndentConfigurable checks SetIndent (tabs / custom width).
func TestEngine_IndentConfigurable(t *testing.T) {
	t.Parallel()

	ta := newFakeTA("foo")
	ta.at(0, 0)
	e := New()
	e.SetIndent("\t", 4) // tab indent, 4-col dedent
	e.HandleKey(ta, ">")
	e.HandleKey(ta, ">")
	if ta.Value() != "\tfoo" {
		t.Fatalf(">> with tab indent = %q, want %q", ta.Value(), "\tfoo")
	}
	e.HandleKey(ta, "<")
	e.HandleKey(ta, "<")
	if ta.Value() != "foo" {
		t.Errorf("<< with tab indent = %q, want %q", ta.Value(), "foo")
	}

	// 4-space width.
	ta2 := newFakeTA("x")
	ta2.at(0, 0)
	e2 := New()
	e2.SetIndent("    ", 4)
	e2.HandleKey(ta2, ">")
	e2.HandleKey(ta2, ">")
	if ta2.Value() != "    x" {
		t.Errorf(">> with 4 spaces = %q, want %q", ta2.Value(), "    x")
	}
}

// TestEngine_DotRepeat covers "." repeating the last change for non-insert
// commands (operators, edits, indents).
// TestEngine_AuditConfirmations locks in behaviors an audit flagged as
// possibly-buggy, confirming they are in fact correct: t/T are inclusive
// when operated on (findChar already lands before the char), visual replace
// preserves newlines, dot repeats the count, dedent handles mixed tabs and
// spaces, and bracket objects resolve from a cursor on the closing bracket.
func TestEngine_AuditConfirmations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
	}{
		{"dt deletes up to (not incl) target", "abc.", 0, 0, []string{"d", "t", "."}, ".", 0, 0},
		{"visual replace preserves newline", "a\nb", 0, 0, []string{"v", "j", "r", "x"}, "x\nx", 0, 0},
		{"dot repeats the count (2dw)", "a b c d e f", 0, 0, []string{"2", "d", "w", "."}, "e f", 0, 0},
		{"dedent strips a leading tab before spaces", "\t  foo", 0, 0, []string{"<", "<"}, "  foo", 0, 2},
		{"di{ from cursor on the close brace", "{\nx\n}", 2, 0, []string{"d", "i", "{"}, "{}", 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
			if e.Insert() || e.Visual() {
				t.Errorf("mode = %v, want NORMAL", e.Mode())
			}
		})
	}
}

func TestEngine_DotRepeat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
	}{
		{"x then .", "abcde", 0, 0, []string{"x", "."}, "cde", 0, 0},
		{"x . .", "aaaaa", 0, 0, []string{"x", ".", "."}, "aa", 0, 0},
		{"dw then .", "foo bar baz", 0, 0, []string{"d", "w", "."}, "baz", 0, 0},
		{"dd then .", "a\nb\nc", 0, 0, []string{"d", "d", "."}, "c", 0, 0},
		{"daw then . on next word", "foo bar baz", 0, 0, []string{"d", "a", "w", "."}, "baz", 0, 0},
		{">> then .", "a\nb", 0, 0, []string{">", ">", "."}, "    a\nb", 0, 4},
		{"~ then .", "abc", 0, 0, []string{"~", "."}, "ABc", 0, 2},
		{"r then l then .", "aaa", 0, 0, []string{"r", "x", "l", "."}, "xxa", 0, 1},
		{"J then .", "a\nb\nc\nd", 0, 0, []string{"J", "."}, "a b c\nd", 0, 3},
		{"p then .", "ab", 0, 0, []string{"x", "p", "."}, "baa", 0, 2},
		{". with no prior change", "abc", 0, 1, []string{"."}, "abc", 0, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, _ := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
		})
	}
}

// TestEngine_DotRepeatInsert covers "." repeating an insert change (cw + typed
// text). Typing is simulated via SetValue, as the textarea would do.
func TestEngine_DotRepeatInsert(t *testing.T) {
	t.Parallel()

	// cw "foo" -> type "X" -> esc, recorded as change {c w, "X"}.
	ta, e := run("foo bar", 0, 0, "c", "w")
	ta.SetValue("X bar") // textarea inserts the typed "X"
	e.HandleKey(ta, "esc")
	if ta.Value() != "X bar" {
		t.Fatalf("after cw+X: %q", ta.Value())
	}
	// Move onto "bar" and repeat the change there.
	e.HandleKey(ta, "0")
	e.HandleKey(ta, "w")
	e.HandleKey(ta, ".")
	if ta.Value() != "X X" {
		t.Errorf("cw then . = %q, want %q", ta.Value(), "X X")
	}

	// A plain insert (i + text + esc) is also repeatable.
	ta2, e2 := run("xy", 0, 0, "i")
	ta2.SetValue("Axy")      // typed "A"
	e2.HandleKey(ta2, "esc") // change {i, "A"}, cursor on the inserted A (col0)
	e2.HandleKey(ta2, "$")   // to end of line
	e2.HandleKey(ta2, ".")   // insert "A" before the last char again
	if ta2.Value() != "AxAy" {
		t.Errorf("i then . = %q, want %q", ta2.Value(), "AxAy")
	}
}

// TestEngine_NoopTargets verifies that a missing text-object/find target is
// a clean no-op AND leaves no stuck operator state (a follow-up command must
// still work).
func TestEngine_NoopTargets(t *testing.T) {
	t.Parallel()

	noop := []struct {
		name     string
		initial  string
		row, col int
		keys     []string
	}{
		{"di( no brackets", "no brackets here", 0, 3, []string{"d", "i", "("}},
		{"di-quote no quotes", "no quotes", 0, 2, []string{"d", "i", `"`}},
		{"dt; no semicolon", "abc def", 0, 0, []string{"d", "t", ";"}},
		{"df. no dot", "abc", 0, 0, []string{"d", "f", "."}},
		{"diw on empty line", "", 0, 0, []string{"d", "i", "w"}},
		{"r esc cancels cleanly", "abc", 0, 1, []string{"r", "esc"}},
	}
	for _, tc := range noop {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if ta.Value() != tc.initial {
				t.Errorf("value changed to %q, want unchanged %q", ta.Value(), tc.initial)
			}
			if e.Insert() || e.Visual() {
				t.Errorf("mode = %v, want NORMAL", e.Mode())
			}
			// State must not be stuck: a fresh edit must take effect.
			before := ta.Value()
			e.HandleKey(ta, "x")
			if ta.Value() == before && len(before) > tc.col {
				t.Errorf("follow-up x had no effect (stuck operator state?)")
			}
		})
	}
}

// TestEngine_VisualCtrlBracketExits confirms ctrl+[ leaves visual mode at the
// engine level (the UI routes it via the esc guard, not ConsumesNormal).
func TestEngine_VisualCtrlBracketExits(t *testing.T) {
	t.Parallel()
	_, e := run("hello", 0, 0, "v", "l", "ctrl+[")
	if e.Visual() || e.Mode() != ModeNormal {
		t.Errorf("after ctrl+[ mode = %v, want NORMAL", e.Mode())
	}
}

// TestEngine_MultilineBracketObject checks i{ across lines.
func TestEngine_MultilineBracketObject(t *testing.T) {
	t.Parallel()
	ta, _ := run("{\n  x\n}", 1, 2, "d", "i", "{")
	if got := ta.Value(); got != "{}" {
		t.Errorf("di{ across lines = %q, want %q", got, "{}")
	}
}

// TestEngine_VisualMode covers v/V entry, motions extending the selection,
// text objects (viw/vaw), and the operators d/x, c/s, y on the selection.
func TestEngine_VisualMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
		wantMode         Mode
	}{
		// charwise selection + delete
		{"v ll d", "hello", 0, 0, []string{"v", "l", "l", "d"}, "lo", 0, 0, ModeNormal},
		{"v 2l d", "hello", 0, 0, []string{"v", "2", "l", "d"}, "lo", 0, 0, ModeNormal},
		{"v e d (word)", "foo bar", 0, 0, []string{"v", "e", "d"}, " bar", 0, 0, ModeNormal},
		{"v $ d (to eol)", "hello", 0, 0, []string{"v", "$", "d"}, "", 0, 0, ModeNormal},
		{"v f. d (find)", "ab.cd", 0, 0, []string{"v", "f", ".", "d"}, "cd", 0, 0, ModeNormal},

		// backwards selection still deletes the right span
		{"v h h d backwards", "hello", 0, 3, []string{"v", "h", "h", "d"}, "ho", 0, 1, ModeNormal},

		// text objects
		{"viw d", "foo bar baz", 0, 5, []string{"v", "i", "w", "d"}, "foo  baz", 0, 4, ModeNormal},
		{"vaw d", "foo bar baz", 0, 5, []string{"v", "a", "w", "d"}, "foo baz", 0, 4, ModeNormal},
		{"viW d (WORD)", "foo.bar baz", 0, 2, []string{"v", "i", "W", "d"}, " baz", 0, 0, ModeNormal},
		{"vi-quote d", `a "bc" d`, 0, 3, []string{"v", "i", `"`, "d"}, `a "" d`, 0, 3, ModeNormal},
		{"vi-paren d", "foo(bar)baz", 0, 4, []string{"v", "i", "(", "d"}, "foo()baz", 0, 4, ModeNormal},
		{"viw y leaves buffer", "foo bar baz", 0, 5, []string{"v", "i", "w", "y"}, "foo bar baz", 0, 4, ModeNormal},
		{"viw c enters insert", "foo bar baz", 0, 5, []string{"v", "i", "w", "c"}, "foo  baz", 0, 4, ModeInsert},
		{"viw s enters insert", "foo bar baz", 0, 5, []string{"v", "i", "w", "s"}, "foo  baz", 0, 4, ModeInsert},
		{"viw x deletes", "foo bar baz", 0, 5, []string{"v", "i", "w", "x"}, "foo  baz", 0, 4, ModeNormal},

		// linewise selection
		{"V d one line", "a\nb\nc", 1, 0, []string{"V", "d"}, "a\nc", 1, 0, ModeNormal},
		{"Vj d two lines", "a\nb\nc\nd", 0, 0, []string{"V", "j", "d"}, "c\nd", 0, 0, ModeNormal},
		{"Vk d upward", "a\nb\nc", 2, 0, []string{"V", "k", "d"}, "a", 0, 0, ModeNormal},
		{"V y rests on top", "a\nb\nc", 1, 0, []string{"V", "y"}, "a\nb\nc", 1, 0, ModeNormal},
		{"V c clears + insert", "a\nb\nc", 1, 0, []string{"V", "c"}, "a\n\nc", 1, 0, ModeInsert},

		// yank then paste round-trips through the register
		{"v e y then P", "foo bar", 0, 0, []string{"v", "e", "y", "P"}, "foofoo bar", 0, 2, ModeNormal},

		// exiting visual mode without operating
		{"esc cancels", "hello", 0, 0, []string{"v", "l", "esc"}, "hello", 0, 1, ModeNormal},
		{"v v toggles off", "hello", 0, 0, []string{"v", "v"}, "hello", 0, 0, ModeNormal},
		{"v V switches to line", "a\nb", 0, 0, []string{"v", "V", "d"}, "b", 0, 0, ModeNormal},

		// structural ops: o swap ends, J join, r replace, p paste-over, gv
		{"o swaps active end", "hello", 0, 2, []string{"v", "l", "o", "h", "d"}, "ho", 0, 1, ModeNormal},
		{"o keeps selection", "hello", 0, 0, []string{"v", "l", "l", "o", "d"}, "lo", 0, 0, ModeNormal},
		{"V J joins lines", "a\nb\nc", 0, 0, []string{"V", "j", "J"}, "a b\nc", 0, 1, ModeNormal},
		{"r replaces selection", "hello", 0, 0, []string{"v", "l", "l", "r", "x"}, "xxxlo", 0, 0, ModeNormal},
		{"p pastes over selection", "foo bar", 0, 0, []string{"y", "i", "w", "w", "v", "i", "w", "p"}, "foo foo", 0, 4, ModeNormal},
		{"gv reselects", "hello", 0, 0, []string{"v", "l", "l", "esc", "g", "v", "d"}, "lo", 0, 0, ModeNormal},

		// indent in visual mode — selection is KEPT so > can repeat
		{"V j > indents + keeps selection", "a\nb\nc", 0, 0, []string{"V", "j", ">"}, "  a\n  b\nc", 1, 0, ModeVisualLine},
		{"V 2> indents two levels", "a", 0, 0, []string{"V", "2", ">"}, "    a", 0, 0, ModeVisualLine},
		{"V < dedents + keeps selection", "    a\n    b", 0, 0, []string{"V", "j", "<"}, "  a\n  b", 1, 0, ModeVisualLine},
		{"V > > repeats on same selection", "a", 0, 0, []string{"V", ">", ">"}, "    a", 0, 0, ModeVisualLine},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
			if e.Mode() != tc.wantMode {
				t.Errorf("mode = %v, want %v", e.Mode(), tc.wantMode)
			}
		})
	}
}

// TestEngine_VisualState checks the rendering-facing getters: Visual,
// VisualLine, the anchor, and the normalized span.
func TestEngine_VisualState(t *testing.T) {
	t.Parallel()

	ta, e := run("hello world", 0, 2, "v", "l", "l", "l")
	if !e.Visual() || e.VisualLine() {
		t.Fatalf("Visual=%v VisualLine=%v, want true/false", e.Visual(), e.VisualLine())
	}
	if ar, ac := e.VisualAnchor(); ar != 0 || ac != 2 {
		t.Errorf("anchor = (%d,%d), want (0,2)", ar, ac)
	}
	sr, sc, er, ec := e.VisualSpan(ta.Line(), ta.Column())
	if sr != 0 || sc != 2 || er != 0 || ec != 5 {
		t.Errorf("span = (%d,%d)-(%d,%d), want (0,2)-(0,5)", sr, sc, er, ec)
	}

	// Anchor before cursor is normalized when the cursor is earlier.
	ta2, e2 := run("hello", 0, 4, "v", "h", "h")
	sr, sc, er, ec = e2.VisualSpan(ta2.Line(), ta2.Column())
	if sr != 0 || sc != 2 || er != 0 || ec != 4 {
		t.Errorf("backwards span = (%d,%d)-(%d,%d), want (0,2)-(0,4)", sr, sc, er, ec)
	}

	_, e3 := run("a\nb", 0, 0, "V")
	if !e3.Visual() || !e3.VisualLine() {
		t.Errorf("V: Visual=%v VisualLine=%v, want true/true", e3.Visual(), e3.VisualLine())
	}
}

// TestEngine_VisualRowSpans checks the per-row column ranges the renderer
// uses to paint the selection — the logic that must NOT include the prompt
// gutter or be off by one.
func TestEngine_VisualRowSpans(t *testing.T) {
	t.Parallel()

	lens := func(s string) []int {
		parts := strings.Split(s, "\n")
		out := make([]int, len(parts))
		for i, p := range parts {
			out[i] = len([]rune(p))
		}
		return out
	}

	t.Run("viw covers exactly the word", func(t *testing.T) {
		ta, e := run("foo bar baz", 0, 5, "v", "i", "w")
		got := e.VisualRowSpans(ta.Line(), ta.Column(), lens("foo bar baz"))
		want := []RowSpan{{Row: 0, StartCol: 4, EndCol: 7}} // cols 4,5,6 = "bar"
		if !equalSpans(got, want) {
			t.Errorf("viw spans = %+v, want %+v", got, want)
		}
	})

	t.Run("viw then w extends by one word start", func(t *testing.T) {
		ta, e := run("foo bar baz", 0, 5, "v", "i", "w", "w")
		got := e.VisualRowSpans(ta.Line(), ta.Column(), lens("foo bar baz"))
		want := []RowSpan{{Row: 0, StartCol: 4, EndCol: 9}} // "bar b"
		if !equalSpans(got, want) {
			t.Errorf("viw+w spans = %+v, want %+v", got, want)
		}
	})

	t.Run("viw then b collapses to one char", func(t *testing.T) {
		ta, e := run("foo bar baz", 0, 5, "v", "i", "w", "b")
		got := e.VisualRowSpans(ta.Line(), ta.Column(), lens("foo bar baz"))
		want := []RowSpan{{Row: 0, StartCol: 4, EndCol: 5}} // just "b"
		if !equalSpans(got, want) {
			t.Errorf("viw+b spans = %+v, want %+v", got, want)
		}
	})

	t.Run("charwise across lines: interior is whole line, no prompt", func(t *testing.T) {
		ta, e := run("abc\ndef\nghi", 0, 1, "v", "j", "j")
		got := e.VisualRowSpans(ta.Line(), ta.Column(), lens("abc\ndef\nghi"))
		want := []RowSpan{
			{Row: 0, StartCol: 1, EndCol: 3}, // "bc" to EOL
			{Row: 1, StartCol: 0, EndCol: 3}, // whole "def"
			{Row: 2, StartCol: 0, EndCol: 2}, // "gh" up to cursor
		}
		if !equalSpans(got, want) {
			t.Errorf("multiline spans = %+v, want %+v", got, want)
		}
	})

	t.Run("linewise spans whole lines", func(t *testing.T) {
		ta, e := run("abc\ndef", 0, 1, "V", "j")
		got := e.VisualRowSpans(ta.Line(), ta.Column(), lens("abc\ndef"))
		want := []RowSpan{{Row: 0, StartCol: 0, EndCol: 3}, {Row: 1, StartCol: 0, EndCol: 3}}
		if !equalSpans(got, want) {
			t.Errorf("linewise spans = %+v, want %+v", got, want)
		}
	})

	t.Run("linewise omits empty interior line", func(t *testing.T) {
		ta, e := run("abc\n\ndef", 0, 0, "V", "j", "j")
		got := e.VisualRowSpans(ta.Line(), ta.Column(), lens("abc\n\ndef"))
		want := []RowSpan{{Row: 0, StartCol: 0, EndCol: 3}, {Row: 2, StartCol: 0, EndCol: 3}}
		if !equalSpans(got, want) {
			t.Errorf("empty-line spans = %+v, want %+v", got, want)
		}
	})
}

func equalSpans(a, b []RowSpan) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestEngine_SystemClipboard checks that yanks and deletes stage text for
// the OS clipboard (matching vim 'unnamedplus'), with a trailing newline on
// linewise copies, and that plain motions stage nothing.
func TestEngine_SystemClipboard(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		initial  string
		row, col int
		keys     []string
		wantText string
		wantOK   bool
	}{
		{"yiw yanks charwise", "foo bar baz", 0, 5, []string{"y", "i", "w"}, "bar", true},
		{"yy yanks linewise with newline", "a\nb", 0, 0, []string{"y", "y"}, "a\n", true},
		{"x delete syncs", "abc", 0, 0, []string{"x"}, "a", true},
		{"dd delete syncs linewise", "a\nb", 0, 0, []string{"d", "d"}, "a\n", true},
		{"visual yank syncs", "foo bar", 0, 0, []string{"v", "e", "y"}, "foo", true},
		{"visual line yank syncs", "a\nb\nc", 0, 0, []string{"V", "j", "y"}, "a\nb\n", true},
		{"plain motion stages nothing", "abc", 0, 0, []string{"l", "l"}, "", false},
		{"case op stages nothing", "FOO", 0, 0, []string{"g", "u", "w"}, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			got, ok := e.ConsumeClipboard()
			if ok != tc.wantOK || got != tc.wantText {
				t.Errorf("ConsumeClipboard = (%q, %v), want (%q, %v)", got, ok, tc.wantText, tc.wantOK)
			}
			// A second consume always reports nothing pending.
			if _, ok := e.ConsumeClipboard(); ok {
				t.Errorf("second ConsumeClipboard reported pending text")
			}
		})
	}
}

func TestMode_String(t *testing.T) {
	t.Parallel()
	if ModeVisual.String() != "VISUAL" || ModeVisualLine.String() != "V-LINE" {
		t.Errorf("visual mode strings = %q/%q", ModeVisual.String(), ModeVisualLine.String())
	}
	if ModeNormal.String() != "NORMAL" || ModeInsert.String() != "INSERT" {
		t.Errorf("mode strings = %q/%q", ModeNormal.String(), ModeInsert.String())
	}
}

func TestConsumesNormal(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"h", "j", "x", "0", "$", "w", "i", "u", "left", "right", "up", "down", "esc", "ctrl+r"} {
		if !ConsumesNormal(k) {
			t.Errorf("ConsumesNormal(%q) = false, want true", k)
		}
	}
	// Multi-rune non-special keys, control/alt chords, and the empty string
	// must pass through to the host app.
	for _, k := range []string{"enter", "tab", "ctrl+p", "ctrl+m", "ctrl+c", "shift+enter", "alt+m", "ctrl+w", "pgup", "home", "backspace", ""} {
		if ConsumesNormal(k) {
			t.Errorf("ConsumesNormal(%q) = true, want false (should pass through)", k)
		}
	}
}

// TestEngine_EdgeCases covers boundary branches not exercised by the main
// motions/edits table: leading-"0" count accumulation, edits on empty
// lines, word motions at buffer/line boundaries and crossing lines with
// counts, "dw" with counts clamping at end-of-line, "O" at row 0, and "I"
// on indented/all-blank lines.
func TestEngine_EdgeCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		initial          string
		row, col         int
		keys             []string
		wantVal          string
		wantRow, wantCol int
		wantInsert       bool
	}{
		// count accumulation: a "0" after a non-zero digit is a digit, not
		// the line-start motion. "1" then "0" -> count 10.
		{"10l count via leading 0", "abcdefghijklmno", 0, 0, []string{"1", "0", "l"}, "abcdefghijklmno", 0, 10, false},
		{"20w clamps at buffer end", "a b c d e f g h i j k l m", 0, 0, []string{"2", "0", "w"}, "a b c d e f g h i j k l m", 0, 24, false},

		// edits on an empty line are no-ops (and never panic).
		{"x on empty line", "\nabc", 0, 0, []string{"x"}, "\nabc", 0, 0, false},
		{"x on empty buffer", "", 0, 0, []string{"x"}, "", 0, 0, false},
		{"D on empty line", "\nabc", 0, 0, []string{"D"}, "\nabc", 0, 0, false},
		{"dw on empty line", "\nabc", 0, 0, []string{"d", "w"}, "\nabc", 0, 0, false},
		{"dw on empty buffer", "", 0, 0, []string{"d", "w"}, "", 0, 0, false},

		// word motions at buffer boundaries stay put.
		{"w on last char stays", "foobar", 0, 5, []string{"w"}, "foobar", 0, 5, false},
		{"e on last char stays", "foo", 0, 2, []string{"e"}, "foo", 0, 2, false},
		{"b on first char stays", "abc", 0, 0, []string{"b"}, "abc", 0, 0, false},

		// e variants: from inside a run of spaces, and when only trailing
		// spaces remain to the end of the buffer.
		{"e from within spaces", "a   bcd", 0, 1, []string{"e"}, "a   bcd", 0, 6, false},
		{"e over only trailing spaces", "ab   ", 0, 1, []string{"e"}, "ab   ", 0, 4, false},

		// word motions crossing logical lines, including with counts.
		{"w crosses to next line", "ab\ncd", 0, 1, []string{"w"}, "ab\ncd", 1, 0, false},
		{"b crosses up a line", "ab\ncd", 1, 0, []string{"b"}, "ab\ncd", 0, 0, false},
		{"e crosses to next line", "ab\ncd", 0, 1, []string{"e"}, "ab\ncd", 1, 1, false},
		{"2w across lines", "a b\nc d", 0, 0, []string{"2", "w"}, "a b\nc d", 1, 0, false},
		{"3b across lines", "a b\nc d", 1, 2, []string{"3", "b"}, "a b\nc d", 0, 0, false},

		// dw with a count, and a count that runs past the end of the line:
		// dw never joins lines, so it clamps at end-of-line.
		{"2dw deletes two words", "foo bar baz qux", 0, 0, []string{"2", "d", "w"}, "baz qux", 0, 0, false},
		{"3dw clamps at line end", "foo bar", 0, 0, []string{"3", "d", "w"}, "", 0, 0, false},

		// O on the first row opens a line above it (row stays 0).
		{"O at row 0", "abc", 0, 0, []string{"O"}, "\nabc", 0, 0, true},
		// I on an indented line lands on the first non-blank column...
		{"I on indented line", "    word", 0, 6, []string{"I"}, "    word", 0, 4, true},
		// ...and on an all-blank line falls back to column 0.
		{"I on all-blank line", "    ", 0, 2, []string{"I"}, "    ", 0, 0, true},

		// "gg" ignores any pending count and goes to the first line; a "g"
		// followed by a non-"g" key cancels the prefix without moving.
		{"5gg ignores count, goes top", "a\nb\nc\nd", 0, 0, []string{"5", "g", "g"}, "a\nb\nc\nd", 0, 0, false},
		{"g then non-g cancels", "abc", 0, 1, []string{"g", "x"}, "abc", 0, 1, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta, e := run(tc.initial, tc.row, tc.col, tc.keys...)
			if got := ta.Value(); got != tc.wantVal {
				t.Errorf("value = %q, want %q", got, tc.wantVal)
			}
			if ta.Line() != tc.wantRow || ta.Column() != tc.wantCol {
				t.Errorf("cursor = (%d,%d), want (%d,%d)", ta.Line(), ta.Column(), tc.wantRow, tc.wantCol)
			}
			if e.Insert() != tc.wantInsert {
				t.Errorf("insert = %v, want %v", e.Insert(), tc.wantInsert)
			}
		})
	}
}

func TestEngine_Mode(t *testing.T) {
	t.Parallel()
	ta := newFakeTA("abc")
	ta.at(0, 0)
	e := New()
	if e.Mode() != ModeNormal {
		t.Errorf("new engine Mode() = %v, want ModeNormal", e.Mode())
	}
	e.HandleKey(ta, "i")
	if e.Mode() != ModeInsert {
		t.Errorf("after 'i' Mode() = %v, want ModeInsert", e.Mode())
	}
	e.HandleKey(ta, "esc")
	if e.Mode() != ModeNormal {
		t.Errorf("after 'esc' Mode() = %v, want ModeNormal", e.Mode())
	}
}

// TestClamp covers the helper directly, including the lower-bound branch
// and the hi<lo normalization that the engine's always-non-negative
// callers never reach.
func TestClamp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v, lo, hi, want int
	}{
		{5, 0, 10, 5},   // in range
		{-5, 0, 10, 0},  // below lower bound
		{15, 0, 10, 10}, // above upper bound
		{0, 0, 10, 0},   // at lower bound
		{10, 0, 10, 10}, // at upper bound
		{3, 5, 2, 5},    // hi < lo: hi normalized up to lo, then v clamped to lo
	}
	for _, c := range cases {
		if got := clamp(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clamp(%d,%d,%d) = %d, want %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}
