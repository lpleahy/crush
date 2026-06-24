// Package vim implements a minimal modal (vim-style) editing layer for
// the message composer.
//
// It treats the bubbles textarea as a string sink rather than owning a
// separate buffer: on each normal-mode command the engine reads the
// current value and cursor, computes the new value and cursor on an
// in-memory [][]rune, and writes them back via SetValue + a cursor
// seek. The textarea therefore remains the single source of truth for
// content, so insert-mode typing needs no special handling — the engine
// simply passes those keys through.
//
// The core (motions and edits) is pure and unit-tested against a fake
// Textarea; the only textarea-specific concern is the cursor seek, which
// works around the textarea exposing no absolute (row,col) setter.
package vim

import "strings"

// Mode is the editing mode of the composer.
type Mode int

const (
	// ModeNormal is command mode: keys move the cursor and edit.
	ModeNormal Mode = iota
	// ModeInsert is insert mode: keys are typed into the textarea.
	ModeInsert
	// ModeVisual is charwise visual mode: the selection runs from the
	// anchor to the cursor and an operator applies to it.
	ModeVisual
	// ModeVisualLine is linewise visual mode: the selection spans whole
	// lines from the anchor's row to the cursor's row.
	ModeVisualLine
)

// String returns the uppercase mode name, e.g. for a "-- NORMAL --"
// indicator.
func (m Mode) String() string {
	switch m {
	case ModeInsert:
		return "INSERT"
	case ModeVisual:
		return "VISUAL"
	case ModeVisualLine:
		return "V-LINE"
	default:
		return "NORMAL"
	}
}

// Textarea is the minimal surface the vim engine needs from the bubbles
// textarea. *textarea.Model satisfies it.
type Textarea interface {
	Value() string
	SetValue(string)
	Line() int   // logical (not visual) row of the cursor
	Column() int // rune column of the cursor in its logical line
	MoveToBegin()
	CursorDown()
	SetCursorColumn(int)
}

// Engine implements modal editing on top of a Textarea. It holds only
// modal state (mode, a pending count/operator, and undo/redo stacks); the
// textarea owns the buffer content.
type Engine struct {
	mode    Mode
	count   int  // numeric prefix accumulating now; 0 means none
	pending rune // pending operator ('d' delete, 'c' change); 0 means none
	opCount int  // count captured when the operator was pressed
	gPrefix bool // saw a leading 'g', waiting for the second key
	findOp  rune // pending f/F/t/T awaiting its target char; 0 means none
	textObj rune // pending 'i'/'a' awaiting the text-object char; 0 means none
	replace bool // pending 'r' awaiting the replacement char
	// lastFindOp/lastFindChar remember the most recent f/F/t/T for ; and ,.
	// They persist across commands (reset() leaves them alone).
	lastFindOp   rune
	lastFindChar rune
	reg          register // the unnamed register (yank/delete target for p/P)
	// sysClip stages the text of the most recent yank/delete to be mirrored
	// to the OS clipboard (set/cleared via ConsumeClipboard), matching a vim
	// 'unnamedplus' setup where every yank/delete updates the system register.
	sysClip      string
	sysClipReady bool
	// visualRow/visualCol anchor the visual-mode selection; the other end
	// is the live cursor. Valid only while mode is ModeVisual/ModeVisualLine.
	visualRow int
	visualCol int
	// last visual selection, remembered on exit for gv (reselect).
	hasLastVisual                bool
	lastVisualMode               Mode
	lastVisAncRow, lastVisAncCol int
	lastVisCurRow, lastVisCurCol int
	undo                         []undoState
	redo                         []undoState
	// pendingInsert is the buffer state captured when entering insert
	// mode. It's committed to the undo stack on the way back to normal
	// mode, and only if the buffer actually changed — so a whole insert
	// session (the o/O line-open plus any typing) is a single undo step,
	// and a no-op i<esc> doesn't create one. nil outside insert mode.
	pendingInsert *undoState
	// indentUnit is the string one >> inserts; indentWidth is the column
	// count one shift represents (for <<). Defaults to two spaces.
	indentUnit  string
	indentWidth int
	// Dot-repeat (the "." command) bookkeeping. dotKeys accumulates the
	// normal/visual keys of the in-progress command; dotStartVal is the
	// buffer when it began; dotInsert* capture an insert session's typed
	// text by diff. lastChange holds the most recent completed change, and
	// dotReplaying guards against recording during a replay.
	dotKeys         []string
	dotStartVal     string
	dotInsertActive bool
	dotInsertStart  string
	lastChange      *changeRecord
	dotReplaying    bool
}

// changeRecord is a replayable last-change for the "." command: the
// normal/visual keys that produced it, plus the inserted text (for commands
// that ended in insert mode).
type changeRecord struct {
	keys   []string
	text   string
	insert bool
}

type undoState struct {
	value string
	row   int
	col   int
}

// register holds yanked or deleted text for p/P. linewise text is whole
// lines (joined by "\n", no trailing newline) pasted as new lines; charwise
// text is inserted inline.
type register struct {
	text     string
	linewise bool
}

// setRegister stores text in the unnamed register (the source for p/P) and
// stages it to be mirrored to the OS clipboard, matching a vim 'unnamedplus'
// setup where every yank/delete updates the system register. Linewise text
// gets a trailing newline on the clipboard, as vim does.
func (e *Engine) setRegister(text string, linewise bool) {
	e.reg = register{text: text, linewise: linewise}
	e.sysClip = text
	if linewise {
		e.sysClip += "\n"
	}
	e.sysClipReady = true
}

// ConsumeClipboard returns text staged for the system clipboard by the most
// recent yank/delete (clearing it), or ok=false if none is pending. The TUI
// pushes the returned text to the OS clipboard (OSC 52 + native copy).
func (e *Engine) ConsumeClipboard() (string, bool) {
	if !e.sysClipReady {
		return "", false
	}
	e.sysClipReady = false
	return e.sysClip, true
}

// New returns an Engine in normal mode, with the default indent (2 spaces).
func New() *Engine {
	return &Engine{mode: ModeNormal, indentUnit: "  ", indentWidth: 2}
}

// SetIndent configures the unit one shift (>> / << / visual > <) applies:
// unit is the string inserted by >>, width the column count one shift
// represents (used to strip leading whitespace on <<).
func (e *Engine) SetIndent(unit string, width int) {
	if width < 1 {
		width = 1
	}
	if unit == "" {
		unit = strings.Repeat(" ", width)
	}
	e.indentUnit, e.indentWidth = unit, width
}

// Mode reports the current editing mode.
func (e *Engine) Mode() Mode { return e.mode }

// Insert reports whether the engine is in insert mode, in which case the
// caller should let the textarea handle keys natively.
func (e *Engine) Insert() bool { return e.mode == ModeInsert }

// Visual reports whether the engine is in either visual mode (charwise or
// linewise), in which case the caller should render the selection overlay.
func (e *Engine) Visual() bool {
	return e.mode == ModeVisual || e.mode == ModeVisualLine
}

// VisualLine reports whether the engine is in linewise visual mode (V),
// where the selection spans whole lines.
func (e *Engine) VisualLine() bool { return e.mode == ModeVisualLine }

// VisualAnchor returns the (row, col) where the visual selection was
// anchored (its fixed end; the live cursor is the other end). Meaningful
// only while Visual() is true.
func (e *Engine) VisualAnchor() (int, int) { return e.visualRow, e.visualCol }

// VisualSpan returns the selection bounds normalized to reading order
// (start <= end) given the live cursor at (curRow, curCol). For linewise
// mode the columns aren't meaningful — callers highlight whole rows.
func (e *Engine) VisualSpan(curRow, curCol int) (sr, sc, er, ec int) {
	sr, sc = e.visualRow, e.visualCol
	er, ec = curRow, curCol
	if sr > er || (sr == er && sc > ec) {
		sr, sc, er, ec = er, ec, sr, sc
	}
	return
}

// RowSpan is the half-open logical column range [StartCol, EndCol) selected
// on a given logical Row. EndCol is exclusive.
type RowSpan struct {
	Row      int
	StartCol int
	EndCol   int
}

// VisualRowSpans returns, per logical row, the column range the selection
// covers given the live cursor at (curRow, curCol) and each line's rune
// length (lineLen[r]). Charwise selections are inclusive of the cursor cell;
// interior rows of a multi-line charwise selection (and every row of a
// linewise selection) span the whole line. Each span is in LOGICAL columns —
// the renderer adds the prompt offset — so continuation-line prompts are
// never covered. Rows with an empty range are omitted.
func (e *Engine) VisualRowSpans(curRow, curCol int, lineLen []int) []RowSpan {
	sr, sc, er, ec := e.VisualSpan(curRow, curCol)
	linewise := e.mode == ModeVisualLine
	spans := make([]RowSpan, 0, er-sr+1)
	for r := sr; r <= er; r++ {
		ll := 0
		if r >= 0 && r < len(lineLen) {
			ll = lineLen[r]
		}
		c0, c1 := 0, ll
		if !linewise {
			if r == sr {
				c0 = sc
			}
			if r == er {
				c1 = ec + 1 // inclusive of the character under the cursor
			}
		}
		c0 = clamp(c0, 0, ll)
		c1 = clamp(c1, 0, ll)
		if c1 > c0 {
			spans = append(spans, RowSpan{Row: r, StartCol: c0, EndCol: c1})
		}
	}
	return spans
}

// HandleKey processes one key (a bubbletea key string such as "h", "esc",
// "ctrl+w"). It returns true if the engine consumed the key, in which
// case the caller must NOT forward it to the textarea.
//
// In insert mode only "esc"/"ctrl+[" are consumed (to leave insert
// mode); every other key passes through to the textarea. In normal mode
// every key is consumed so the textarea never edits behind the engine's
// back.
func (e *Engine) HandleKey(ta Textarea, key string) bool {
	// "." repeats the last change (only in normal mode with nothing pending).
	if key == "." && !e.dotReplaying &&
		e.mode == ModeNormal && e.pending == 0 && e.findOp == 0 &&
		e.textObj == 0 && !e.gPrefix && !e.replace {
		e.count, e.opCount = 0, 0
		e.repeatChange(ta)
		return true
	}
	if e.dotReplaying {
		return e.dispatch(ta, key)
	}
	// Record keys of the in-progress command for "." (see dotTrack).
	if len(e.dotKeys) == 0 && e.mode != ModeInsert {
		e.dotStartVal = ta.Value()
	}
	e.dotKeys = append(e.dotKeys, key)
	consumed := e.dispatch(ta, key)
	e.dotTrack(ta)
	return consumed
}

// dispatch routes a key to the mode-specific handler (the engine's core,
// independent of dot-repeat recording).
func (e *Engine) dispatch(ta Textarea, key string) bool {
	switch e.mode {
	case ModeInsert:
		if key == "esc" || key == "ctrl+[" {
			e.toNormal(ta)
			return true
		}
		return false
	case ModeVisual, ModeVisualLine:
		e.handleVisual(ta, key)
		return true
	default:
		e.handleNormal(ta, key)
		return true
	}
}

// dotIdle reports whether the engine has settled back to normal mode with no
// command in progress (no operator/count/find/text-object/replace pending).
func (e *Engine) dotIdle() bool {
	return e.mode == ModeNormal && e.pending == 0 && e.opCount == 0 &&
		e.count == 0 && e.findOp == 0 && e.textObj == 0 && !e.gPrefix && !e.replace
}

// dotTrack runs after each dispatched key while recording. It tracks an
// insert session's start (to diff the typed text) and, when the command
// settles in normal mode, saves it as the last change if it edited the
// buffer.
func (e *Engine) dotTrack(ta Textarea) {
	if e.mode == ModeInsert {
		if !e.dotInsertActive {
			e.dotInsertActive = true
			e.dotInsertStart = ta.Value()
		}
		return
	}
	if !e.dotIdle() {
		return // operator/visual/count/find still pending
	}
	keys := e.dotKeys
	inserted := ""
	insertCmd := e.dotInsertActive
	if insertCmd {
		inserted = diffInserted(e.dotInsertStart, ta.Value())
		if n := len(keys); n > 0 && (keys[n-1] == "esc" || keys[n-1] == "ctrl+[") {
			keys = keys[:n-1] // drop the esc that ended insert
		}
	}
	if ta.Value() != e.dotStartVal && isDotChange(keys) {
		e.lastChange = &changeRecord{keys: append([]string(nil), keys...), text: inserted, insert: insertCmd}
	}
	e.dotKeys = nil
	e.dotInsertActive = false
	e.dotInsertStart = ""
}

// repeatChange replays the last recorded change at the cursor.
func (e *Engine) repeatChange(ta Textarea) {
	if e.lastChange == nil {
		return
	}
	rec := e.lastChange
	e.dotReplaying = true
	for _, k := range rec.keys {
		e.dispatch(ta, k)
	}
	if rec.insert && e.mode == ModeInsert {
		e.insertTextRaw(ta, rec.text)
		e.toNormal(ta)
	}
	e.dotReplaying = false
}

// insertTextRaw splices text into the buffer at the cursor and leaves the
// cursor just past it (used when replaying an insert change for ".").
func (e *Engine) insertTextRaw(ta Textarea, text string) {
	if text == "" {
		return
	}
	// Use the raw cursor column (insert mode sits at len, one past the last
	// char), not read()'s normal-mode clamp to len-1.
	lines := toRunes(ta.Value())
	row := clamp(ta.Line(), 0, len(lines)-1)
	col := clamp(ta.Column(), 0, len(lines[row]))
	flat, starts := flatten(lines)
	at := clamp(starts[row]+col, 0, len(flat))
	ins := []rune(text)
	out := append(append(append([]rune{}, flat[:at]...), ins...), flat[at:]...)
	next := toRunes(string(out))
	nr, nc := flatIdxToPos(out, at+len(ins))
	ta.SetValue(string(out))
	moveCursorRaw(ta, next, nr, nc)
}

// isDotChange reports whether a settled command is a repeatable change.
// Undo/redo and visual-mode commands are excluded.
func isDotChange(keys []string) bool {
	if len(keys) == 0 {
		return false
	}
	switch keys[0] {
	case "u", "ctrl+r", "v", "V":
		return false
	}
	return true
}

// diffInserted returns the run of text inserted between two buffer versions
// (the middle once a common prefix and suffix are removed).
func diffInserted(before, after string) string {
	b, a := []rune(before), []rune(after)
	i := 0
	for i < len(b) && i < len(a) && b[i] == a[i] {
		i++
	}
	j := 0
	for j < len(b)-i && j < len(a)-i && b[len(b)-1-j] == a[len(a)-1-j] {
		j++
	}
	return string(a[i : len(a)-j])
}

// ConsumesNormal reports whether a key should be routed to the engine in
// normal mode. Single printable runes, the arrow keys, and esc are vim's;
// everything else (enter, tab, ctrl/alt chords) is left to the host app so
// its shortcuts keep working.
func ConsumesNormal(key string) bool {
	switch key {
	case "left", "right", "up", "down", "esc", "ctrl+r":
		return true
	}
	return len([]rune(key)) == 1
}

func (e *Engine) reset() {
	e.count = 0
	e.opCount = 0
	e.pending = 0
	e.gPrefix = false
	e.findOp = 0
	e.textObj = 0
	e.replace = false
}

// caseOpFor maps the second key of gu/gU/g~ to its case-operator marker.
func caseOpFor(key string) rune {
	switch key {
	case "u":
		return opLower
	case "U":
		return opUpper
	case "~":
		return opToggle
	}
	return 0
}

// effCount is the effective repeat count for a motion under the current
// operator: the product of the pre-operator and post-operator counts
// (e.g. 2d3w = 6 words), each at least 1. Matches vim.
func (e *Engine) effCount() int {
	op := e.opCount
	if op < 1 {
		op = 1
	}
	post := e.count
	if post < 1 {
		post = 1
	}
	return op * post
}

func (e *Engine) handleNormal(ta Textarea, key string) {
	// A replace is pending (r): the next key is the replacement char.
	if e.replace {
		e.replace = false
		e.applyReplace(ta, key)
		e.reset()
		return
	}

	// A find target is pending (f/F/t/T): the next key is its literal char.
	if e.findOp != 0 {
		op := e.findOp
		e.findOp = 0
		e.applyFind(ta, op, key)
		e.reset()
		return
	}

	// A text object is pending (i/a + object char), only while an operator
	// is pending.
	if e.textObj != 0 {
		ia := e.textObj
		e.textObj = 0
		e.applyTextObject(ta, ia, key)
		e.reset()
		return
	}

	// Second key of a 'g' sequence (gg / dgg / cgg, gJ, g_, ge/gE).
	if e.gPrefix {
		e.gPrefix = false
		switch key {
		case "g":
			e.lineMotion(ta, 0)
		case "J":
			e.joinLines(ta, e.effCount(), false) // gJ: join without a space
		case "_":
			lines, row, col := read(ta)
			t := clamp(row+e.effCount()-1, 0, len(lines)-1)
			if e.pending == 0 {
				moveCursor(ta, lines, t, lastNonBlank(lines[t]))
			} else {
				e.opCharRange(ta, lines, row, col, t, lastNonBlank(lines[t]), true)
			}
		case "e":
			e.opCharwise(ta, motionWordEndBack, true)
		case "E":
			e.opCharwise(ta, motionWORDEndBack, true)
		case "u", "U", "~":
			// gu/gU/g~: start a case operator, wait for a motion/object.
			e.pending = caseOpFor(key)
			e.opCount = e.count
			e.count = 0
			return
		case "v": // gv: reselect the last visual selection
			e.reselectVisual(ta)
			e.reset()
			return
		}
		e.reset()
		return
	}

	// Numeric count prefix (accumulates before AND after an operator). "0"
	// only counts as a digit when a count is already in progress.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		e.count = e.count*10 + int(key[0]-'0')
		return
	}
	if key == "0" && e.count > 0 {
		e.count *= 10
		return
	}

	// While an operator is pending, i/a introduce a text object (not
	// insert) and a repeated operator (dd / cc / guu / gUU / g~~) is linewise.
	if e.pending != 0 {
		switch key {
		case "i", "a":
			e.textObj = rune(key[0])
			return
		case "d", "c", "y":
			if rune(key[0]) == e.pending {
				r := rowOf(ta)
				e.opLinewise(ta, r, r+e.effCount()-1)
			}
			e.reset()
			return
		case "u", "U", "~":
			if caseOpFor(key) == e.pending { // guu/gUU/g~~: linewise case
				r := rowOf(ta)
				e.opLinewise(ta, r, r+e.effCount()-1)
				e.reset()
				return
			}
		case ">", "<":
			if (key == ">" && e.pending == opIndent) || (key == "<" && e.pending == opDedent) {
				r := rowOf(ta) // >> / << : shift N lines
				e.opLinewise(ta, r, r+e.effCount()-1)
			}
			e.reset()
			return
		}
	}

	switch key {
	case "i", "a", "I", "A", "o", "O":
		e.enterInsert(ta, key)
		e.reset()
		return

	// Visual mode: anchor the selection at the cursor.
	case "v":
		e.enterVisual(ta, ModeVisual)
		return
	case "V":
		e.enterVisual(ta, ModeVisualLine)
		return

	// Operators: wait for a motion / text object.
	case "d", "c", "y":
		e.pending = rune(key[0])
		e.opCount = e.count
		e.count = 0
		return
	case ">", "<": // indent / dedent operator (linewise)
		if key == ">" {
			e.pending = opIndent
		} else {
			e.pending = opDedent
		}
		e.opCount = e.count
		e.count = 0
		return

	// Charwise motions: move, or operate over the range when pending.
	case "h", "left":
		e.opCharwise(ta, motionLeft, false)
	case "l", "right", " ":
		e.opCharwise(ta, motionRight, false)
	case "0":
		e.opCharwise(ta, motionLineStart, false)
	case "^":
		e.opCharwise(ta, motionFirstNonBlank, false)
	case "$":
		e.opCharwise(ta, motionLineEnd, true)
	case "b":
		e.opCharwise(ta, motionWordBack, false)
	case "e":
		e.opCharwise(ta, motionWordEnd, true)
	case "w":
		switch {
		case e.pending == 'c':
			e.opCharwise(ta, motionWordEnd, true) // cw behaves like ce
		case e.pending == 'd' || e.pending == 'y' || isCaseOp(e.pending):
			e.opWord(ta, e.effCount()) // dw/yw/guw: EOL-clamped, never joins lines
		default:
			e.move(ta, motionWordFwd, e.effCount())
		}
	case "W":
		switch {
		case e.pending == 'c':
			e.opCharwise(ta, motionWORDEnd, true) // cW behaves like cE
		case e.pending == 'd' || e.pending == 'y' || isCaseOp(e.pending):
			e.opWordCls(ta, e.effCount(), bigClassOf) // dW/yW/guW: EOL-clamped
		default:
			e.move(ta, motionWORDFwd, e.effCount())
		}
	case "B":
		e.opCharwise(ta, motionWORDBack, false)
	case "E":
		e.opCharwise(ta, motionWORDEnd, true)
	case "f", "F", "t", "T":
		e.findOp = rune(key[0])
		return // wait for the target char
	case ";":
		e.repeatFind(ta, false)
	case ",":
		e.repeatFind(ta, true)
	case "%":
		e.matchBracketMotion(ta)
	case "{":
		e.opCharwise(ta, motionParaBack, false)
	case "}":
		e.opCharwise(ta, motionParaFwd, false)
	case "|":
		lines, row, col := read(ta)
		target := clamp(e.effCount()-1, 0, len(lines[row]))
		if e.pending == 0 {
			moveCursor(ta, lines, row, target)
		} else {
			e.opCharRange(ta, lines, row, col, row, target, false)
		}

	// Linewise motions: move, or operate linewise when pending.
	case "j", "down":
		if e.pending != 0 {
			r := rowOf(ta)
			e.opLinewise(ta, r, r+e.effCount())
		} else {
			e.move(ta, motionDown, e.effCount())
		}
	case "k", "up":
		if e.pending != 0 {
			r := rowOf(ta)
			e.opLinewise(ta, r-e.effCount(), r)
		} else {
			e.move(ta, motionUp, e.effCount())
		}
	case "G":
		if e.pending != 0 {
			r := rowOf(ta)
			e.opLinewise(ta, r, lastRowOf(ta))
		} else {
			lines, _, col := read(ta)
			moveCursor(ta, lines, len(lines)-1, col)
		}
	case "g":
		e.gPrefix = true
		return // keep operator + count
	case "_":
		if e.pending != 0 {
			r := rowOf(ta)
			e.opLinewise(ta, r, r+e.effCount()-1)
		} else {
			lines, row, _ := read(ta)
			t := clamp(row+e.effCount()-1, 0, len(lines)-1)
			moveCursor(ta, lines, t, firstNonBlank(lines[t]))
		}

	// Edits that take no motion.
	case "x":
		e.deleteChars(ta, e.effCount())
	case "X":
		e.pending = 'd'
		e.opCharwise(ta, motionLeft, false) // delete chars before the cursor
	case "D":
		e.deleteToLineEnd(ta)
	case "C": // change to end of line (c$)
		e.pending = 'c'
		e.opCharwise(ta, motionLineEnd, true)
	case "s": // substitute char(s): delete then insert
		e.substituteChars(ta, e.effCount())
	case "S": // substitute line(s) == cc
		e.pending = 'c'
		r := rowOf(ta)
		e.opLinewise(ta, r, r+e.effCount()-1)
	case "Y": // yank line(s) == yy
		e.pending = 'y'
		r := rowOf(ta)
		e.opLinewise(ta, r, r+e.effCount()-1)
	case "r":
		e.replace = true
		return // wait for the replacement char
	case "~":
		e.toggleCaseChars(ta, e.effCount())
	case "J":
		e.joinLines(ta, e.effCount(), true)
	case "p":
		e.paste(ta, true)
	case "P":
		e.paste(ta, false)
	case "u":
		e.undoOnce(ta)
	case "ctrl+r":
		e.redoOnce(ta)
	}

	e.reset()
}

// enterVisual anchors a selection at the current cursor and switches to the
// given visual mode.
func (e *Engine) enterVisual(ta Textarea, mode Mode) {
	_, row, col := read(ta)
	e.reset()
	e.visualRow, e.visualCol = row, col
	e.mode = mode
}

// visualCount is the effective repeat for a visual-mode motion (operator
// counts don't apply in visual mode).
func (e *Engine) visualCount() int {
	if e.count < 1 {
		return 1
	}
	return e.count
}

// handleVisual processes a key in visual/visual-line mode: motions move the
// cursor (extending the selection from the anchor), text objects reset both
// ends to the object's bounds, and an operator (d/x, c/s, y) applies to the
// selection and exits visual mode.
func (e *Engine) handleVisual(ta Textarea, key string) {
	// Pending r: the next key replaces every char of the selection.
	if e.replace {
		e.replace = false
		e.visualReplace(ta, key)
		e.reset()
		return
	}

	// Pending f/F/t/T target: move the cursor to it (extends the selection).
	if e.findOp != 0 {
		op := e.findOp
		e.findOp = 0
		if r := []rune(key); len(r) == 1 {
			lines, row, col := read(ta)
			if t := findChar(lines[row], col, op, r[0]); t >= 0 {
				moveCursor(ta, lines, row, t)
			}
		}
		e.reset()
		return
	}

	// Pending text object (i/a + object char): select it.
	if e.textObj != 0 {
		ia := e.textObj
		e.textObj = 0
		e.visualTextObject(ta, ia, key)
		e.reset()
		return
	}

	// Second key of a 'g' sequence (gg, g_, ge/gE).
	if e.gPrefix {
		e.gPrefix = false
		switch key {
		case "g":
			lines, _, col := read(ta)
			moveCursor(ta, lines, 0, col)
		case "_":
			lines, row, _ := read(ta)
			moveCursor(ta, lines, row, lastNonBlank(lines[row]))
		case "e":
			e.move(ta, motionWordEndBack, e.visualCount())
		case "E":
			e.move(ta, motionWORDEndBack, e.visualCount())
		}
		e.reset()
		return
	}

	// Numeric count prefix.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		e.count = e.count*10 + int(key[0]-'0')
		return
	}
	if key == "0" && e.count > 0 {
		e.count *= 10
		return
	}

	switch key {
	case "esc", "ctrl+[":
		_, row, col := read(ta)
		e.rememberVisual(row, col)
		e.mode = ModeNormal
	case "v": // toggle charwise off, or switch V -> v
		if e.mode == ModeVisual {
			_, row, col := read(ta)
			e.rememberVisual(row, col)
			e.mode = ModeNormal
		} else {
			e.mode = ModeVisual
		}
	case "V": // toggle linewise off, or switch v -> V
		if e.mode == ModeVisualLine {
			_, row, col := read(ta)
			e.rememberVisual(row, col)
			e.mode = ModeNormal
		} else {
			e.mode = ModeVisualLine
		}
	case "o": // swap the cursor to the other end of the selection
		lines, row, col := read(ta)
		ar, ac := e.visualRow, e.visualCol
		e.visualRow, e.visualCol = row, col
		moveCursor(ta, lines, ar, ac)
	case "i", "a":
		e.textObj = rune(key[0])
		return
	case "J": // join the selected lines
		e.visualJoin(ta)
		return
	case ">": // indent the selected lines
		e.visualIndent(ta, opIndent)
		return
	case "<": // dedent the selected lines
		e.visualIndent(ta, opDedent)
		return
	case "r": // replace every char of the selection
		e.replace = true
		return
	case "p", "P": // paste over the selection
		e.visualPaste(ta)
		return

	// Operators apply to the selection, then leave visual mode.
	case "d", "x":
		e.applyVisualOp(ta, 'd')
		return
	case "c", "s":
		e.applyVisualOp(ta, 'c')
		return
	case "y":
		e.applyVisualOp(ta, 'y')
		return
	case "u": // lowercase the selection
		e.applyVisualOp(ta, opLower)
		return
	case "U": // uppercase the selection
		e.applyVisualOp(ta, opUpper)
		return
	case "~": // toggle case of the selection
		e.applyVisualOp(ta, opToggle)
		return

	// Motions (no operator pending here, so these only move the cursor).
	case "h", "left":
		e.move(ta, motionLeft, e.visualCount())
	case "l", "right", " ":
		e.move(ta, motionRight, e.visualCount())
	case "0":
		lines, row, _ := read(ta)
		moveCursor(ta, lines, row, 0)
	case "^":
		lines, row, _ := read(ta)
		moveCursor(ta, lines, row, firstNonBlank(lines[row]))
	case "$":
		lines, row, _ := read(ta)
		moveCursor(ta, lines, row, len(lines[row]))
	case "b":
		e.move(ta, motionWordBack, e.visualCount())
	case "e":
		e.move(ta, motionWordEnd, e.visualCount())
	case "w":
		e.move(ta, motionWordFwd, e.visualCount())
	case "W":
		e.move(ta, motionWORDFwd, e.visualCount())
	case "B":
		e.move(ta, motionWORDBack, e.visualCount())
	case "E":
		e.move(ta, motionWORDEnd, e.visualCount())
	case "f", "F", "t", "T":
		e.findOp = rune(key[0])
		return // wait for the target char
	case ";":
		e.repeatFind(ta, false)
	case ",":
		e.repeatFind(ta, true)
	case "%":
		e.matchBracketMotion(ta)
	case "{":
		e.move(ta, motionParaBack, e.visualCount())
	case "}":
		e.move(ta, motionParaFwd, e.visualCount())
	case "|":
		lines, row, _ := read(ta)
		moveCursor(ta, lines, row, clamp(e.visualCount()-1, 0, len(lines[row])))
	case "j", "down":
		e.move(ta, motionDown, e.visualCount())
	case "k", "up":
		e.move(ta, motionUp, e.visualCount())
	case "G":
		lines, _, col := read(ta)
		moveCursor(ta, lines, len(lines)-1, col)
	case "g":
		e.gPrefix = true
		return
	case "_":
		lines, row, _ := read(ta)
		t := clamp(row+e.visualCount()-1, 0, len(lines)-1)
		moveCursor(ta, lines, t, firstNonBlank(lines[t]))
	}

	e.reset()
}

// applyVisualOp applies operator op ('d', 'c', or 'y') to the current
// selection (anchor..cursor) and leaves visual mode. 'd'/'y' return to
// normal mode; 'c' enters insert (committed as one undo step).
func (e *Engine) applyVisualOp(ta Textarea, op rune) {
	lines, row, col := read(ta)
	e.rememberVisual(row, col)
	e.pending = op
	if e.mode == ModeVisualLine {
		lo, hi := e.visualRow, row
		if lo > hi {
			lo, hi = hi, lo
		}
		e.opLinewise(ta, lo, hi)
		if op == 'y' { // yank leaves the buffer; rest the cursor on the top line
			ls, _, _ := read(ta)
			lo = clamp(lo, 0, len(ls)-1)
			moveCursor(ta, ls, lo, firstNonBlank(ls[lo]))
		}
	} else {
		// Charwise selections are inclusive of the character under the cursor.
		e.opCharRange(ta, lines, e.visualRow, e.visualCol, row, col, true)
	}
	if op != 'c' { // opCharRange/opLinewise already switched to insert for 'c'
		e.mode = ModeNormal
	}
	e.reset()
}

// rememberVisual records the current selection (anchor + given cursor +
// mode) so gv can reselect it after visual mode is left.
func (e *Engine) rememberVisual(curRow, curCol int) {
	e.hasLastVisual = true
	e.lastVisualMode = e.mode
	e.lastVisAncRow, e.lastVisAncCol = e.visualRow, e.visualCol
	e.lastVisCurRow, e.lastVisCurCol = curRow, curCol
}

// reselectVisual restores the last remembered selection (gv).
func (e *Engine) reselectVisual(ta Textarea) {
	if !e.hasLastVisual {
		return
	}
	lines := toRunes(ta.Value())
	e.mode = e.lastVisualMode
	e.visualRow = clamp(e.lastVisAncRow, 0, len(lines)-1)
	e.visualCol = e.lastVisAncCol
	moveCursor(ta, lines, e.lastVisCurRow, e.lastVisCurCol)
}

// visualIndent shifts the selected lines by visualCount levels (the
// visual-mode > / <). Unlike most visual operators it KEEPS the selection
// (like vim's common `>gv` remap) so the shift can be repeated without
// reselecting.
func (e *Engine) visualIndent(ta Textarea, op rune) {
	lines, row, col := read(ta)
	lo := min(e.visualRow, row)
	hi := max(e.visualRow, row)
	e.pending = op
	e.applyIndentLines(ta, lines, lo, hi, e.visualCount())
	e.pending = 0
	// Restore both ends of the selection (applyIndentLines moved the cursor
	// to the first line) so the highlight stays put for a repeat >.
	next := toRunes(ta.Value())
	e.visualRow = clamp(e.visualRow, 0, len(next)-1)
	e.visualCol = clamp(e.visualCol, 0, len(next[e.visualRow]))
	r := clamp(row, 0, len(next)-1)
	moveCursor(ta, next, r, clamp(col, 0, len(next[r])))
	e.count = 0
}

// visualJoin joins all selected lines into one (the visual-mode J).
func (e *Engine) visualJoin(ta Textarea) {
	lines, row, _ := read(ta)
	e.rememberVisual(row, e.visualCol)
	lo := min(e.visualRow, row)
	hi := max(e.visualRow, row)
	moveCursor(ta, lines, lo, 0)
	e.joinLines(ta, hi-lo+1, true)
	e.mode = ModeNormal
}

// visualReplace replaces every character of the selection with the given
// char (the visual-mode r{char}); newlines are preserved.
func (e *Engine) visualReplace(ta Textarea, key string) {
	rs := []rune(key)
	lines, row, col := read(ta)
	e.rememberVisual(row, col)
	if len(rs) != 1 { // esc cancels, just leave visual mode
		e.mode = ModeNormal
		return
	}
	ch := rs[0]
	if e.mode == ModeVisualLine {
		lo := min(e.visualRow, row)
		hi := max(e.visualRow, row)
		e.snapshot(ta)
		for r := lo; r <= hi; r++ {
			for c := range lines[r] {
				lines[r][c] = ch
			}
		}
		writeContent(ta, lines, lo, 0)
		e.mode = ModeNormal
		return
	}
	flat, i, j := flatRange(lines, e.visualRow, e.visualCol, row, col, true)
	if i >= j {
		e.mode = ModeNormal
		return
	}
	e.snapshot(ta)
	out := append([]rune{}, flat...)
	for k := i; k < j; k++ {
		if out[k] != '\n' {
			out[k] = ch
		}
	}
	next := toRunes(string(out))
	nr, nc := flatIdxToPos(out, i)
	ta.SetValue(string(out))
	moveCursor(ta, next, nr, nc)
	e.mode = ModeNormal
}

// visualPaste replaces the selection with the register contents (the
// visual-mode p/P), leaving the cursor at the start of the inserted text.
func (e *Engine) visualPaste(ta Textarea) {
	lines, row, col := read(ta)
	e.rememberVisual(row, col)
	if e.reg.text == "" {
		e.mode = ModeNormal
		return
	}
	if e.mode == ModeVisualLine {
		lo := min(e.visualRow, row)
		hi := max(e.visualRow, row)
		e.snapshot(ta)
		next := append([][]rune{}, lines[:lo]...)
		if e.reg.linewise {
			for _, ln := range toRunes(e.reg.text) {
				next = append(next, append([]rune{}, ln...))
			}
		} else {
			next = append(next, []rune(e.reg.text))
		}
		next = append(next, lines[hi+1:]...)
		if len(next) == 0 {
			next = [][]rune{{}}
		}
		r := clamp(lo, 0, len(next)-1)
		writeContent(ta, next, r, firstNonBlank(next[r]))
		e.mode = ModeNormal
		return
	}
	flat, i, j := flatRange(lines, e.visualRow, e.visualCol, row, col, true)
	e.snapshot(ta)
	ins := []rune(e.reg.text)
	out := append([]rune{}, flat[:i]...)
	out = append(out, ins...)
	out = append(out, flat[j:]...)
	next := toRunes(string(out))
	nr, nc := flatIdxToPos(out, i)
	ta.SetValue(string(out))
	moveCursor(ta, next, nr, nc)
	e.mode = ModeNormal
}

// visualTextObject selects a text object (word/WORD, quotes, brackets) by
// anchoring at its start and moving the cursor to its end (may span lines).
func (e *Engine) visualTextObject(ta Textarea, ia rune, objKey string) {
	lines, row, col := read(ta)
	r1, c1, r2, c2, ok := textObjectRange(lines, row, col, ia, objKey)
	if !ok {
		return
	}
	e.visualRow, e.visualCol = r1, c1
	moveCursor(ta, lines, r2, c2)
}

// opCharwise applies the operator over a charwise motion's range, or just
// moves the cursor when no operator is pending. inclusive includes the
// motion's endpoint in the deletion.
func (e *Engine) opCharwise(ta Textarea, fn motionFunc, inclusive bool) {
	n := e.effCount()
	lines, row, col := read(ta)
	tRow, tCol := row, col
	for range n {
		tRow, tCol = fn(lines, tRow, tCol)
	}
	if e.pending == 0 {
		moveCursor(ta, lines, tRow, tCol)
		return
	}
	e.opCharRange(ta, lines, row, col, tRow, tCol, inclusive)
}

// opLinewise applies the operator over whole lines [lo..hi] (order
// independent). 'd' removes the lines; 'c' replaces them with one empty
// line and enters insert.
func (e *Engine) opLinewise(ta Textarea, lo, hi int) {
	lines := toRunes(ta.Value())
	lo = clamp(lo, 0, len(lines)-1)
	hi = clamp(hi, 0, len(lines)-1)
	if lo > hi {
		lo, hi = hi, lo
	}
	if isIndentOp(e.pending) {
		e.applyIndentLines(ta, lines, lo, hi, 1)
		return
	}
	if isCaseOp(e.pending) {
		e.applyCaseLines(ta, lines, lo, hi)
		return
	}
	e.setRegister(fromRunes(lines[lo:hi+1]), true)
	if e.pending == 'y' {
		return // yank: register set, buffer and cursor unchanged
	}
	if e.pending == 'c' {
		e.pendingInsert = &undoState{value: ta.Value(), row: ta.Line(), col: ta.Column()}
		next := make([][]rune, 0, len(lines))
		next = append(next, lines[:lo]...)
		next = append(next, []rune{})
		next = append(next, lines[hi+1:]...)
		ta.SetValue(fromRunes(next))
		moveCursorRaw(ta, next, lo, 0)
		e.mode = ModeInsert
		return
	}
	e.snapshot(ta)
	next := append(append([][]rune{}, lines[:lo]...), lines[hi+1:]...)
	if len(next) == 0 {
		next = [][]rune{{}}
	}
	row := min(lo, len(next)-1)
	writeContent(ta, next, row, firstNonBlank(next[row]))
}

// applyOp writes the post-delete buffer and finishes the operator: 'd'
// snapshots and rests the cursor on a character; 'c' records the pre-change
// state as one undo unit and enters insert at the deletion point.
func (e *Engine) applyOp(ta Textarea, next [][]rune, row, col int) {
	if e.pending == 'c' {
		e.pendingInsert = &undoState{value: ta.Value(), row: ta.Line(), col: ta.Column()}
		ta.SetValue(fromRunes(next))
		moveCursorRaw(ta, next, row, col)
		e.mode = ModeInsert
		return
	}
	e.snapshot(ta)
	ta.SetValue(fromRunes(next))
	moveCursor(ta, next, row, col)
}

// applyReplace implements "r{char}" / "{n}r{char}": replace n characters
// from the cursor with the given char, leaving the cursor on the last one.
// If fewer than n characters remain on the line, it is a no-op (like vim).
func (e *Engine) applyReplace(ta Textarea, key string) {
	rs := []rune(key)
	if len(rs) != 1 { // esc or a non-literal key cancels
		return
	}
	ch := rs[0]
	lines, row, col := read(ta)
	n := e.effCount()
	line := lines[row]
	if col+n > len(line) {
		return
	}
	e.snapshot(ta)
	out := append([]rune{}, line...)
	for i := col; i < col+n; i++ {
		out[i] = ch
	}
	lines[row] = out
	writeContent(ta, lines, row, clampNormalCol(out, col+n-1))
}

// substituteChars implements "s" / "{n}s": delete n characters from the
// cursor and enter insert (as one undo step), like "cl".
func (e *Engine) substituteChars(ta Textarea, n int) {
	lines, row, col := read(ta)
	end := min(col+n, len(lines[row]))
	if end <= col { // empty line: just enter insert
		e.enterInsert(ta, "i")
		return
	}
	e.pending = 'c'
	e.opCharRange(ta, lines, row, col, row, end-1, true)
}

// toggleCaseChars implements "~" / "{n}~": flip the case of n characters
// from the cursor and advance past them.
func (e *Engine) toggleCaseChars(ta Textarea, n int) {
	lines, row, col := read(ta)
	line := lines[row]
	if col >= len(line) {
		return
	}
	end := min(col+n, len(line))
	e.snapshot(ta)
	out := append([]rune{}, line...)
	for i := col; i < end; i++ {
		out[i] = toggleCase(out[i])
	}
	lines[row] = out
	writeContent(ta, lines, row, clampNormalCol(out, end))
}

// joinLines implements "J" (withSpace) and "gJ" (no space): merge the line
// below into the current one, count times. With a space, leading blanks of
// the joined line are collapsed to a single separating space (unless the
// current line is empty or already ends in a space).
func (e *Engine) joinLines(ta Textarea, count int, withSpace bool) {
	lines, row, _ := read(ta)
	joins := max(1, count-1) // J and 2J both join 2 lines (one join)
	if row+1 >= len(lines) {
		return
	}
	e.snapshot(ta)
	cursorCol := 0
	for range joins {
		if row+1 >= len(lines) {
			break
		}
		cur := append([]rune{}, lines[row]...)
		nxt := lines[row+1]
		cursorCol = len(cur)
		if withSpace {
			k := 0
			for k < len(nxt) && isSpace(nxt[k]) {
				k++
			}
			nxt = nxt[k:]
			if len(cur) > 0 && len(nxt) > 0 && !isSpace(cur[len(cur)-1]) {
				cur = append(cur, ' ')
			}
		}
		lines[row] = append(cur, nxt...)
		lines = append(lines[:row+1], lines[row+2:]...)
	}
	writeContent(ta, lines, row, clampNormalCol(lines[row], cursorCol))
}

// paste inserts the register after (p) or before (P) the cursor, honoring a
// count. A linewise register opens new line(s) below/above; a charwise one
// is spliced inline, leaving the cursor on the last pasted character.
func (e *Engine) paste(ta Textarea, after bool) {
	if e.reg.text == "" {
		return
	}
	n := e.effCount()
	lines, row, col := read(ta)
	e.snapshot(ta)
	if e.reg.linewise {
		block := toRunes(e.reg.text)
		rep := make([][]rune, 0, len(block)*n)
		for range n {
			for _, ln := range block {
				rep = append(rep, append([]rune{}, ln...))
			}
		}
		at := row
		if after {
			at = row + 1
		}
		at = clamp(at, 0, len(lines))
		next := make([][]rune, 0, len(lines)+len(rep))
		next = append(next, lines[:at]...)
		next = append(next, rep...)
		next = append(next, lines[at:]...)
		writeContent(ta, next, at, firstNonBlank(next[at]))
		return
	}
	flat, starts := flatten(lines)
	insertCol := col
	if after && len(lines[row]) > 0 {
		insertCol = min(col+1, len(lines[row]))
	}
	ii := starts[row] + insertCol
	ins := []rune(strings.Repeat(e.reg.text, n))
	out := make([]rune, 0, len(flat)+len(ins))
	out = append(out, flat[:ii]...)
	out = append(out, ins...)
	out = append(out, flat[ii:]...)
	next := toRunes(string(out))
	end := max(ii, ii+len(ins)-1)
	nr, nc := flatIdxToPos(out, end)
	ta.SetValue(string(out))
	moveCursor(ta, next, nr, nc)
}

// lineMotion handles a linewise move/operate to targetRow (used by gg).
func (e *Engine) lineMotion(ta Textarea, targetRow int) {
	lines, row, col := read(ta)
	if e.pending != 0 {
		e.opLinewise(ta, row, targetRow)
		return
	}
	moveCursor(ta, lines, targetRow, col)
}

// applyFind resolves an f/F/t/T motion to the target char, then moves or
// (if an operator is pending) deletes/changes up to it.
func (e *Engine) applyFind(ta Textarea, op rune, key string) {
	r := []rune(key)
	if len(r) != 1 {
		return // not a literal char (esc, etc.): cancel
	}
	e.lastFindOp, e.lastFindChar = op, r[0] // remembered for ; and ,
	lines, row, col := read(ta)
	target := findChar(lines[row], col, op, r[0])
	if target < 0 {
		return // not found: no-op
	}
	if e.pending == 0 {
		moveCursor(ta, lines, row, target)
		return
	}
	inclusive := op == 'f' || op == 't'
	e.opCharRange(ta, lines, row, col, row, target, inclusive)
}

// repeatFind implements ";" (reverse=false) and "," (reverse=true): repeat
// the last f/F/t/T. For t/T the search starts one column over so ";" doesn't
// stick on an already-adjacent target.
func (e *Engine) repeatFind(ta Textarea, reverse bool) {
	if e.lastFindOp == 0 {
		return
	}
	op := e.lastFindOp
	if reverse {
		op = reverseFindOp(op)
	}
	lines, row, col := read(ta)
	from := col
	switch op {
	case 't':
		from = col + 1
	case 'T':
		from = col - 1
	}
	target := findChar(lines[row], from, op, e.lastFindChar)
	if target < 0 {
		return
	}
	if e.pending == 0 {
		moveCursor(ta, lines, row, target)
		return
	}
	inclusive := op == 'f' || op == 't'
	e.opCharRange(ta, lines, row, col, row, target, inclusive)
}

func reverseFindOp(op rune) rune {
	switch op {
	case 'f':
		return 'F'
	case 'F':
		return 'f'
	case 't':
		return 'T'
	case 'T':
		return 't'
	}
	return op
}

// matchBracketMotion implements "%": from the first bracket at/after the
// cursor on the line, jump to its match (which may be on another line).
func (e *Engine) matchBracketMotion(ta Textarea) {
	lines, row, col := read(ta)
	line := lines[row]
	bi := -1
	for i := col; i < len(line); i++ {
		if strings.ContainsRune("()[]{}", line[i]) {
			bi = i
			break
		}
	}
	if bi < 0 {
		return
	}
	var open, close rune
	switch line[bi] {
	case '(', ')':
		open, close = '(', ')'
	case '[', ']':
		open, close = '[', ']'
	case '{', '}':
		open, close = '{', '}'
	}
	flat, starts := flatten(lines)
	oi, ci, ok := matchBracketPair(flat, starts[row]+bi, open, close)
	if !ok {
		return
	}
	target := ci
	if strings.ContainsRune(")]}", line[bi]) {
		target = oi
	}
	tr, tc := flatIdxToPos(flat, target)
	if e.pending == 0 {
		moveCursor(ta, lines, tr, tc)
		return
	}
	e.opCharRange(ta, lines, row, col, tr, tc, true) // % is inclusive when operating
}

// applyTextObject resolves an i/a text object (word/WORD, quotes, brackets)
// and applies the pending operator to its range.
func (e *Engine) applyTextObject(ta Textarea, ia rune, objKey string) {
	lines, row, col := read(ta)
	r1, c1, r2, c2, ok := textObjectRange(lines, row, col, ia, objKey)
	if !ok {
		return
	}
	e.opCharRange(ta, lines, r1, c1, r2, c2, true)
}

func rowOf(ta Textarea) int {
	_, row, _ := read(ta)
	return row
}

func lastRowOf(ta Textarea) int {
	return len(toRunes(ta.Value())) - 1
}

// charEdit describes a charwise range under an operator: the text of the
// range (for the register), the buffer after deleting it, the cursor
// position in that new buffer (for d/c), and the range start in the
// original buffer (for y). ok is false for an empty range.
type charEdit struct {
	text     string
	next     [][]rune
	curRow   int
	curCol   int
	startRow int
	startCol int
	ok       bool
}

// computeCharEdit resolves the flat range between (r1,c1) and (r2,c2)
// (order independent; inclusive includes the higher endpoint).
func computeCharEdit(lines [][]rune, r1, c1, r2, c2 int, inclusive bool) charEdit {
	flat, starts := flatten(lines)
	idx := func(r, c int) int {
		r = clamp(r, 0, len(lines)-1)
		c = clamp(c, 0, len(lines[r]))
		return starts[r] + c
	}
	i, j := idx(r1, c1), idx(r2, c2)
	if i > j {
		i, j = j, i
	}
	if inclusive {
		j++
	}
	if j > len(flat) {
		j = len(flat)
	}
	if i < 0 {
		i = 0
	}
	if i >= j {
		return charEdit{}
	}
	out := make([]rune, 0, len(flat)-(j-i))
	out = append(out, flat[:i]...)
	out = append(out, flat[j:]...)
	cr, cc := flatIdxToPos(out, i)
	sr, sc := flatIdxToPos(flat, i)
	return charEdit{
		text:   string(flat[i:j]),
		next:   toRunes(string(out)),
		curRow: cr, curCol: cc,
		startRow: sr, startCol: sc,
		ok: true,
	}
}

// flatIdxToPos converts a flat-buffer index (newlines counted) into a
// (row, col) position.
func flatIdxToPos(flat []rune, idx int) (int, int) {
	idx = clamp(idx, 0, len(flat))
	row, col := 0, 0
	for k := 0; k < idx; k++ {
		if flat[k] == '\n' {
			row, col = row+1, 0
		} else {
			col++
		}
	}
	return row, col
}

// flatRange resolves a charwise range to flat-buffer indices [i, j) (order
// independent; inclusive includes the higher endpoint).
func flatRange(lines [][]rune, r1, c1, r2, c2 int, inclusive bool) (flat []rune, i, j int) {
	var starts []int
	flat, starts = flatten(lines)
	idx := func(r, c int) int {
		r = clamp(r, 0, len(lines)-1)
		c = clamp(c, 0, len(lines[r]))
		return starts[r] + c
	}
	i, j = idx(r1, c1), idx(r2, c2)
	if i > j {
		i, j = j, i
	}
	if inclusive {
		j++
	}
	if j > len(flat) {
		j = len(flat)
	}
	if i < 0 {
		i = 0
	}
	return flat, i, j
}

// opCharRange applies the pending operator to a charwise range: case
// operators transform it in place; 'y' copies it and moves to the start;
// 'd'/'c' yank then delete (and 'c' enters insert).
func (e *Engine) opCharRange(ta Textarea, lines [][]rune, r1, c1, r2, c2 int, inclusive bool) {
	if isIndentOp(e.pending) { // > / < promote any motion to linewise
		e.applyIndentLines(ta, lines, min(r1, r2), max(r1, r2), 1)
		return
	}
	if isCaseOp(e.pending) {
		e.applyCaseRange(ta, lines, r1, c1, r2, c2, inclusive)
		return
	}
	ce := computeCharEdit(lines, r1, c1, r2, c2, inclusive)
	if !ce.ok {
		return
	}
	e.setRegister(ce.text, false)
	if e.pending == 'y' {
		moveCursor(ta, lines, ce.startRow, ce.startCol)
		return
	}
	e.applyOp(ta, ce.next, ce.curRow, ce.curCol)
}

// applyCaseRange transforms the case of a charwise range in place (gu/gU/g~
// with a motion or text object), leaving the cursor at the range start.
func (e *Engine) applyCaseRange(ta Textarea, lines [][]rune, r1, c1, r2, c2 int, inclusive bool) {
	flat, i, j := flatRange(lines, r1, c1, r2, c2, inclusive)
	if i >= j {
		return
	}
	e.snapshot(ta)
	out := append([]rune{}, flat...)
	for k := i; k < j; k++ {
		out[k] = transformCase(out[k], e.pending)
	}
	next := toRunes(string(out))
	nr, nc := flatIdxToPos(out, i)
	ta.SetValue(string(out))
	moveCursor(ta, next, nr, nc)
}

// applyIndentLines indents (opIndent) or dedents (opDedent) whole lines
// [lo..hi] by `levels` shifts, leaving the cursor on the first line's first
// non-blank char.
func (e *Engine) applyIndentLines(ta Textarea, lines [][]rune, lo, hi, levels int) {
	lo = clamp(lo, 0, len(lines)-1)
	hi = clamp(hi, 0, len(lines)-1)
	if lo > hi {
		lo, hi = hi, lo
	}
	if levels < 1 {
		levels = 1
	}
	e.snapshot(ta)
	for r := lo; r <= hi; r++ {
		if e.pending == opIndent {
			lines[r] = indentLine(lines[r], e.indentUnit, levels)
		} else {
			lines[r] = dedentLine(lines[r], e.indentWidth, levels)
		}
	}
	writeContent(ta, lines, lo, firstNonBlank(lines[lo]))
}

// applyCaseLines transforms the case of whole lines [lo..hi] in place (the
// linewise case ops guu/gUU/g~~ and visual u/U/~ on a V selection).
func (e *Engine) applyCaseLines(ta Textarea, lines [][]rune, lo, hi int) {
	lo = clamp(lo, 0, len(lines)-1)
	hi = clamp(hi, 0, len(lines)-1)
	if lo > hi {
		lo, hi = hi, lo
	}
	e.snapshot(ta)
	for r := lo; r <= hi; r++ {
		for c := range lines[r] {
			lines[r][c] = transformCase(lines[r][c], e.pending)
		}
	}
	writeContent(ta, lines, lo, firstNonBlank(lines[lo]))
}

// enterInsert switches to insert mode, first repositioning the cursor
// per the variant (a appends, I/A jump to line ends, o/O open lines).
func (e *Engine) enterInsert(ta Textarea, key string) {
	// Snapshot the pre-insert buffer; the whole session (including the
	// o/O line-open below) commits as one undo step on the way back to
	// normal mode (see toNormal).
	e.pendingInsert = &undoState{value: ta.Value(), row: ta.Line(), col: ta.Column()}
	lines, row, col := read(ta)
	switch key {
	case "a":
		col = min(col+1, len(lines[row]))
	case "A":
		col = len(lines[row])
	case "I":
		col = firstNonBlank(lines[row])
	case "o":
		lines = insertLine(lines, row+1, nil)
		row, col = row+1, 0
		writeContent(ta, lines, row, col)
	case "O":
		lines = insertLine(lines, row, nil)
		col = 0
		writeContent(ta, lines, row, col)
	}
	if key != "o" && key != "O" {
		moveCursorRaw(ta, lines, row, col)
	}
	e.mode = ModeInsert
}

// toNormal leaves insert mode. The whole insert session commits as one
// undo step (only if the buffer changed, so a no-op i<esc> is free), then
// — as in vim — the cursor moves left one so it rests on a character
// rather than past the end of the line.
func (e *Engine) toNormal(ta Textarea) {
	e.mode = ModeNormal
	e.reset()
	if e.pendingInsert != nil {
		if ta.Value() != e.pendingInsert.value {
			e.pushUndo(*e.pendingInsert)
		}
		e.pendingInsert = nil
	}
	if col := ta.Column(); col > 0 {
		ta.SetCursorColumn(col - 1)
	}
}

// move applies a charwise/linewise motion n times and repositions the
// cursor (no content change).
func (e *Engine) move(ta Textarea, fn motionFunc, n int) {
	lines, row, col := read(ta)
	for range n {
		row, col = fn(lines, row, col)
	}
	moveCursor(ta, lines, row, col)
}

// --- edits (content-changing; each snapshots for undo) ---

func (e *Engine) deleteChars(ta Textarea, n int) {
	lines, row, col := read(ta)
	line := lines[row]
	if len(line) == 0 {
		return
	}
	end := min(col+n, len(line))
	if end <= col {
		return
	}
	e.setRegister(string(line[col:end]), false)
	e.snapshot(ta)
	lines[row] = append(append([]rune{}, line[:col]...), line[end:]...)
	col = clampNormalCol(lines[row], col)
	writeContent(ta, lines, row, col)
}

func (e *Engine) deleteToLineEnd(ta Textarea) {
	lines, row, col := read(ta)
	if col >= len(lines[row]) {
		return
	}
	e.setRegister(string(lines[row][col:]), false)
	e.snapshot(ta)
	lines[row] = append([]rune{}, lines[row][:col]...)
	writeContent(ta, lines, row, clampNormalCol(lines[row], col))
}

// opWord implements the word motion under an operator ("dw"/"yw"): it
// spans from the cursor up to the start of the next word, but never past
// the end of the current line (matching vim's behavior of not joining
// lines on dw). The range is routed through opCharRange so 'd' and 'y'
// share the same logic (yank then maybe delete).
func (e *Engine) opWord(ta Textarea, n int) { e.opWordCls(ta, n, classOf) }

// opWordCls is opWord parameterized by character class (classOf for dw/yw,
// bigClassOf for dW/yW).
func (e *Engine) opWordCls(ta Textarea, n int, cls func(rune) int) {
	lines, row, col := read(ta)
	line := lines[row]
	if col >= len(line) {
		return
	}
	target := col
	for range n {
		target = wordEndExclusiveOnLineCls(line, target, cls)
	}
	target = min(target, len(line))
	if target <= col {
		return
	}
	e.opCharRange(ta, lines, row, col, row, target, false)
}

// undoOnce reverts the most recent change, saving the current state so
// redoOnce can re-apply it.
func (e *Engine) undoOnce(ta Textarea) {
	if len(e.undo) == 0 {
		return
	}
	e.redo = append(e.redo, e.currentState(ta))
	last := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.applyState(ta, last)
}

// redoOnce re-applies the most recently undone change.
func (e *Engine) redoOnce(ta Textarea) {
	if len(e.redo) == 0 {
		return
	}
	e.undo = append(e.undo, e.currentState(ta))
	last := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.applyState(ta, last)
}

// BreakUndo ends the current insert-session undo block and starts a fresh
// one at the present buffer state, so a discrete insert-mode action — a
// paste, say — becomes its own undo step instead of merging into the rest
// of the session (where one `u` would wipe the whole thing). Call it just
// BEFORE applying the action, while the buffer still holds the pre-action
// content. No-op outside insert mode.
func (e *Engine) BreakUndo(ta Textarea) {
	if e.mode != ModeInsert {
		return
	}
	if e.pendingInsert != nil && ta.Value() != e.pendingInsert.value {
		e.pushUndo(*e.pendingInsert)
	}
	e.pendingInsert = &undoState{value: ta.Value(), row: ta.Line(), col: ta.Column()}
}

// snapshot records the current buffer as an undo step (called before an
// edit). Any pending redo history is discarded, since a fresh change
// makes the redo stack unreachable — matching vim.
func (e *Engine) snapshot(ta Textarea) {
	e.pushUndo(e.currentState(ta))
}

func (e *Engine) pushUndo(s undoState) {
	e.undo = append(e.undo, s)
	e.redo = nil
}

func (e *Engine) currentState(ta Textarea) undoState {
	return undoState{value: ta.Value(), row: ta.Line(), col: ta.Column()}
}

// applyState restores a snapshot's content and cursor (clamped to a valid
// normal-mode position).
func (e *Engine) applyState(ta Textarea, s undoState) {
	ta.SetValue(s.value)
	moveCursor(ta, toRunes(s.value), s.row, s.col)
}

// --- textarea read/write plumbing ---

// read returns the buffer as rune-lines plus the cursor clamped to a
// valid normal-mode position (on a character, not past end of line).
func read(ta Textarea) (lines [][]rune, row, col int) {
	lines = toRunes(ta.Value())
	row = clamp(ta.Line(), 0, len(lines)-1)
	col = clamp(ta.Column(), 0, max(0, len(lines[row])-1))
	return lines, row, col
}

func writeContent(ta Textarea, lines [][]rune, row, col int) {
	ta.SetValue(fromRunes(lines))
	moveCursorRaw(ta, lines, row, col)
}

// moveCursor repositions the cursor, clamping the column to normal-mode
// range (on a character).
func moveCursor(ta Textarea, lines [][]rune, row, col int) {
	row = clamp(row, 0, len(lines)-1)
	moveCursorRaw(ta, lines, row, clampNormalCol(lines[row], col))
}

// moveCursorRaw seeks the cursor to (row, col). Because the textarea has
// no absolute (row,col) setter and CursorDown moves by visual line, it
// drives CursorDown until the logical row matches, then sets the column.
func moveCursorRaw(ta Textarea, lines [][]rune, row, col int) {
	row = clamp(row, 0, len(lines)-1)
	col = clamp(col, 0, len(lines[row]))
	ta.MoveToBegin()
	for ta.Line() < row {
		pr, pc := ta.Line(), ta.Column()
		ta.CursorDown()
		if ta.Line() == pr && ta.Column() == pc {
			break // stuck (shouldn't happen for a valid row)
		}
	}
	ta.SetCursorColumn(col)
}

func toRunes(s string) [][]rune {
	parts := strings.Split(s, "\n")
	lines := make([][]rune, len(parts))
	for i, p := range parts {
		lines[i] = []rune(p)
	}
	return lines
}

func fromRunes(lines [][]rune) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

func insertLine(lines [][]rune, at int, content []rune) [][]rune {
	at = clamp(at, 0, len(lines))
	out := make([][]rune, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, content)
	out = append(out, lines[at:]...)
	return out
}

func clampNormalCol(line []rune, col int) int {
	return clamp(col, 0, max(0, len(line)-1))
}

func firstNonBlank(line []rune) int {
	for i, r := range line {
		if !isSpace(r) {
			return i
		}
	}
	return 0
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
