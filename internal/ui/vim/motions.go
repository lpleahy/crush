package vim

import (
	"strings"
	"unicode"
)

// motionFunc maps a cursor position to a new one on the given buffer. All
// motions are charwise and clamp to valid normal-mode positions.
type motionFunc func(lines [][]rune, row, col int) (int, int)

func motionLeft(lines [][]rune, row, col int) (int, int) {
	if col > 0 {
		col--
	}
	return row, col
}

func motionRight(lines [][]rune, row, col int) (int, int) {
	if last := max(0, len(lines[row])-1); col < last {
		col++
	}
	return row, col
}

func motionDown(lines [][]rune, row, col int) (int, int) {
	if row < len(lines)-1 {
		row++
	}
	return row, clampNormalCol(lines[row], col)
}

func motionUp(lines [][]rune, row, col int) (int, int) {
	if row > 0 {
		row--
	}
	return row, clampNormalCol(lines[row], col)
}

func motionLineStart(lines [][]rune, row, col int) (int, int) {
	return row, 0
}

// motionFirstNonBlank implements "^" / "_": the first non-whitespace
// column of the line (column 0 on an all-blank line).
func motionFirstNonBlank(lines [][]rune, row, col int) (int, int) {
	return row, firstNonBlank(lines[row])
}

func motionLineEnd(lines [][]rune, row, col int) (int, int) {
	return row, max(0, len(lines[row])-1)
}

// findChar implements the f/F/t/T motions on a single line. op is one of
// 'f' (forward, land on ch), 't' (forward, land before ch), 'F' (backward,
// land on ch), 'T' (backward, land after ch). Returns the target column or
// -1 if ch isn't found in range. f/t are line-local, like vim.
func findChar(line []rune, col int, op, ch rune) int {
	switch op {
	case 'f':
		for i := col + 1; i < len(line); i++ {
			if line[i] == ch {
				return i
			}
		}
	case 't':
		for i := col + 1; i < len(line); i++ {
			if line[i] == ch {
				return i - 1
			}
		}
	case 'F':
		for i := col - 1; i >= 0; i-- {
			if line[i] == ch {
				return i
			}
		}
	case 'T':
		for i := col - 1; i >= 0; i-- {
			if line[i] == ch {
				return i + 1
			}
		}
	}
	return -1
}

// innerWord returns the inclusive [start,end] columns of the "iw" text
// object: the maximal run of the class (word/punct/space) under the
// cursor. ok is false on an empty line.
func innerWord(line []rune, col int) (start, end int, ok bool) {
	return innerWordCls(line, col, classOf)
}

// innerWordCls is innerWord with a pluggable character classer (classOf for
// iw, bigClassOf for iW).
func innerWordCls(line []rune, col int, cls func(rune) int) (start, end int, ok bool) {
	if len(line) == 0 {
		return 0, 0, false
	}
	col = clamp(col, 0, len(line)-1)
	c := cls(line[col])
	start, end = col, col
	for start > 0 && cls(line[start-1]) == c {
		start--
	}
	for end < len(line)-1 && cls(line[end+1]) == c {
		end++
	}
	return start, end, true
}

// aWord returns the inclusive [start,end] columns of the "aw" text object:
// the inner word plus its trailing whitespace, or — if there is none
// (word at end of line) — its leading whitespace.
func aWord(line []rune, col int) (start, end int, ok bool) {
	return aWordCls(line, col, classOf)
}

// aWordCls is aWord with a pluggable character classer (classOf for aw,
// bigClassOf for aW).
func aWordCls(line []rune, col int, cls func(rune) int) (start, end int, ok bool) {
	start, end, ok = innerWordCls(line, col, cls)
	if !ok {
		return 0, 0, false
	}
	if end < len(line)-1 && classOf(line[end+1]) == clsSpace {
		for end < len(line)-1 && classOf(line[end+1]) == clsSpace {
			end++
		}
	} else {
		for start > 0 && classOf(line[start-1]) == clsSpace {
			start--
		}
	}
	return start, end, true
}

// quoteObject resolves a quote text object (i"/a" and ' / ` variants) on a
// single line. For "i" it is the text between the surrounding pair; for "a"
// it includes the quotes plus trailing whitespace (or leading if none).
func quoteObject(line []rune, col int, q rune, ia rune) (start, end int, ok bool) {
	var qs []int
	for i, r := range line {
		if r == q {
			qs = append(qs, i)
		}
	}
	if len(qs) < 2 {
		return 0, 0, false
	}
	for p := 0; p+1 < len(qs); p += 2 {
		o, c := qs[p], qs[p+1]
		if col > c {
			continue // cursor past this pair; try the next
		}
		if ia == 'i' {
			if o+1 > c-1 {
				return 0, 0, false // empty "" — nothing inside
			}
			return o + 1, c - 1, true
		}
		s, e := o, c
		if te := e; te+1 < len(line) && isSpace(line[te+1]) {
			for te+1 < len(line) && isSpace(line[te+1]) {
				te++
			}
			e = te
		} else {
			for s > 0 && isSpace(line[s-1]) {
				s--
			}
		}
		return s, e, true
	}
	return 0, 0, false
}

// matchBracketPair finds the bracket pair (open/close) enclosing — or under —
// the flat-buffer position pos, returning the open and close indices. It
// nests correctly and spans lines (the buffer is flat).
func matchBracketPair(flat []rune, pos int, open, close rune) (oi, ci int, ok bool) {
	n := len(flat)
	if n == 0 {
		return 0, 0, false
	}
	pos = clamp(pos, 0, n-1)
	switch flat[pos] {
	case open:
		oi = pos
	case close:
		depth := 0 // matching open is to the left
		oi = -1
		for i := pos - 1; i >= 0; i-- {
			if flat[i] == close {
				depth++
			} else if flat[i] == open {
				if depth == 0 {
					oi = i
					break
				}
				depth--
			}
		}
		if oi < 0 {
			return 0, 0, false
		}
		return oi, pos, true
	default:
		depth := 0 // enclosing open is to the left
		oi = -1
		for i := pos; i >= 0; i-- {
			if flat[i] == close {
				depth++
			} else if flat[i] == open {
				if depth == 0 {
					oi = i
					break
				}
				depth--
			}
		}
		if oi < 0 {
			return 0, 0, false
		}
	}
	depth := 0 // matching close to the right of oi
	for i := oi + 1; i < n; i++ {
		if flat[i] == open {
			depth++
		} else if flat[i] == close {
			if depth == 0 {
				return oi, i, true
			}
			depth--
		}
	}
	return 0, 0, false
}

// motionWordFwd implements "w": to the start of the next word. Newlines
// count as whitespace, so it crosses lines.
func motionWordFwd(lines [][]rune, row, col int) (int, int) {
	return wordFwdCls(lines, row, col, classOf)
}

// motionWORDFwd implements "W": like w but words are whitespace-delimited.
func motionWORDFwd(lines [][]rune, row, col int) (int, int) {
	return wordFwdCls(lines, row, col, bigClassOf)
}

func wordFwdCls(lines [][]rune, row, col int, cls func(rune) int) (int, int) {
	flat, starts := flatten(lines)
	n := len(flat)
	i := starts[row] + col
	if i >= n {
		return row, col
	}
	if c := cls(flat[i]); c != clsSpace {
		for i < n && cls(flat[i]) == c {
			i++
		}
	}
	for i < n && cls(flat[i]) == clsSpace {
		i++
	}
	if i >= n {
		i = n - 1
	}
	return flatToPos(lines, starts, i)
}

// motionWordEnd implements "e": to the end of the next word.
func motionWordEnd(lines [][]rune, row, col int) (int, int) {
	return wordEndCls(lines, row, col, classOf)
}

// motionWORDEnd implements "E": end of the next whitespace-delimited word.
func motionWORDEnd(lines [][]rune, row, col int) (int, int) {
	return wordEndCls(lines, row, col, bigClassOf)
}

func wordEndCls(lines [][]rune, row, col int, cls func(rune) int) (int, int) {
	flat, starts := flatten(lines)
	n := len(flat)
	i := starts[row] + col
	if i >= n-1 {
		return row, col
	}
	i++
	for i < n && cls(flat[i]) == clsSpace {
		i++
	}
	if i >= n {
		return flatToPos(lines, starts, n-1)
	}
	c := cls(flat[i])
	for i+1 < n && cls(flat[i+1]) == c {
		i++
	}
	return flatToPos(lines, starts, i)
}

// motionWordBack implements "b": to the start of the current or previous
// word.
func motionWordBack(lines [][]rune, row, col int) (int, int) {
	return wordBackCls(lines, row, col, classOf)
}

// motionWORDBack implements "B": start of the current/previous
// whitespace-delimited word.
func motionWORDBack(lines [][]rune, row, col int) (int, int) {
	return wordBackCls(lines, row, col, bigClassOf)
}

func wordBackCls(lines [][]rune, row, col int, cls func(rune) int) (int, int) {
	flat, starts := flatten(lines)
	i := starts[row] + col
	if i <= 0 {
		return row, col
	}
	i--
	for i > 0 && cls(flat[i]) == clsSpace {
		i--
	}
	c := cls(flat[i])
	for i > 0 && cls(flat[i-1]) == c {
		i--
	}
	return flatToPos(lines, starts, i)
}

// textObjectRange resolves an i/a text object (ia is 'i' or 'a') named by
// obj — "w"/"W" (word/WORD), quotes ("\""/"'"/"`"), or brackets
// ("("/")"/"b", "{"/"}"/"B", "["/"]", "<"/">") — to an inclusive charwise
// range (r1,c1)-(r2,c2). ok=false if the object isn't found.
func textObjectRange(lines [][]rune, row, col int, ia rune, obj string) (r1, c1, r2, c2 int, ok bool) {
	r := []rune(obj)
	if len(r) != 1 {
		return
	}
	switch obj {
	case "w":
		s, e, k := innerOrAWord(lines[row], col, ia, classOf)
		return row, s, row, e, k
	case "W":
		s, e, k := innerOrAWord(lines[row], col, ia, bigClassOf)
		return row, s, row, e, k
	case "\"", "'", "`":
		s, e, k := quoteObject(lines[row], col, r[0], ia)
		return row, s, row, e, k
	case "(", ")", "b":
		return bracketRange(lines, row, col, '(', ')', ia)
	case "{", "}", "B":
		return bracketRange(lines, row, col, '{', '}', ia)
	case "[", "]":
		return bracketRange(lines, row, col, '[', ']', ia)
	case "<", ">":
		return bracketRange(lines, row, col, '<', '>', ia)
	}
	return
}

func innerOrAWord(line []rune, col int, ia rune, cls func(rune) int) (start, end int, ok bool) {
	if ia == 'i' {
		return innerWordCls(line, col, cls)
	}
	return aWordCls(line, col, cls)
}

// bracketRange resolves an i/a bracket object to (row,col) coordinates via
// the flat buffer (so it nests and spans lines). "i" excludes the brackets;
// "a" includes them. An empty pair (e.g. di( on "()") returns ok=false.
func bracketRange(lines [][]rune, row, col int, open, close rune, ia rune) (r1, c1, r2, c2 int, ok bool) {
	flat, starts := flatten(lines)
	oi, ci, found := matchBracketPair(flat, starts[row]+col, open, close)
	if !found {
		return 0, 0, 0, 0, false
	}
	a, b := oi, ci
	if ia == 'i' {
		a, b = oi+1, ci-1
		if a > b {
			return 0, 0, 0, 0, false
		}
	}
	r1, c1 = flatIdxToPos(flat, a)
	r2, c2 = flatIdxToPos(flat, b)
	return r1, c1, r2, c2, true
}

// wordEndExclusiveOnLine returns the column where "dw" stops, starting
// from col on a single line: past the current word run and any trailing
// spaces, clamped to the end of the line (dw never joins lines).
func wordEndExclusiveOnLine(line []rune, col int) int {
	return wordEndExclusiveOnLineCls(line, col, classOf)
}

func wordEndExclusiveOnLineCls(line []rune, col int, cls func(rune) int) int {
	n := len(line)
	if col >= n {
		return n
	}
	if c := cls(line[col]); c != clsSpace {
		for col < n && cls(line[col]) == c {
			col++
		}
	}
	for col < n && cls(line[col]) == clsSpace {
		col++
	}
	return col
}

// flatten joins the rune-lines into one flat slice (newlines included)
// and returns the flat start index of each line.
func flatten(lines [][]rune) (flat []rune, starts []int) {
	starts = make([]int, len(lines))
	idx := 0
	for r, line := range lines {
		starts[r] = idx
		flat = append(flat, line...)
		idx += len(line)
		if r < len(lines)-1 {
			flat = append(flat, '\n')
			idx++
		}
	}
	return flat, starts
}

// flatToPos converts a flat index back to (row, col), clamping to a
// character position within a line.
func flatToPos(lines [][]rune, starts []int, fi int) (int, int) {
	for r := len(lines) - 1; r >= 0; r-- {
		if fi >= starts[r] {
			return r, clampNormalCol(lines[r], fi-starts[r])
		}
	}
	return 0, 0
}

const (
	clsSpace = iota
	clsWord
	clsPunct
)

func classOf(r rune) int {
	switch {
	case isSpace(r):
		return clsSpace
	case isWordRune(r):
		return clsWord
	default:
		return clsPunct
	}
}

// bigClassOf classes runes for WORD motions (W/B/E, iW/aW): a WORD is any
// run of non-whitespace, so only space vs non-space matters.
func bigClassOf(r rune) int {
	if isSpace(r) {
		return clsSpace
	}
	return clsWord
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isBlankLine reports whether a line is empty or all whitespace.
func isBlankLine(line []rune) bool {
	for _, r := range line {
		if !isSpace(r) {
			return false
		}
	}
	return true
}

// motionParaFwd implements "}": to the next blank line (or the last line).
func motionParaFwd(lines [][]rune, row, col int) (int, int) {
	for r := row + 1; r < len(lines); r++ {
		if isBlankLine(lines[r]) {
			return r, 0
		}
	}
	return len(lines) - 1, max(0, len(lines[len(lines)-1]))
}

// motionParaBack implements "{": to the previous blank line (or the first).
func motionParaBack(lines [][]rune, row, col int) (int, int) {
	for r := row - 1; r >= 0; r-- {
		if isBlankLine(lines[r]) {
			return r, 0
		}
	}
	return 0, 0
}

// lastNonBlank returns the column of the last non-whitespace char (for g_),
// or 0 on a blank line.
func lastNonBlank(line []rune) int {
	for i := len(line) - 1; i >= 0; i-- {
		if !isSpace(line[i]) {
			return i
		}
	}
	return 0
}

// motionWordEndBack implements "ge": back to the end of the previous word.
func motionWordEndBack(lines [][]rune, row, col int) (int, int) {
	return wordEndBackCls(lines, row, col, classOf)
}

// motionWORDEndBack implements "gE": back to the end of the previous WORD.
func motionWORDEndBack(lines [][]rune, row, col int) (int, int) {
	return wordEndBackCls(lines, row, col, bigClassOf)
}

func wordEndBackCls(lines [][]rune, row, col int, cls func(rune) int) (int, int) {
	flat, starts := flatten(lines)
	n := len(flat)
	for i := starts[row] + col - 1; i >= 0; i-- {
		if cls(flat[i]) != clsSpace && (i+1 >= n || cls(flat[i+1]) != cls(flat[i])) {
			return flatToPos(lines, starts, i)
		}
	}
	return row, col
}

// toggleCase flips a letter's case; non-letters are returned unchanged.
func toggleCase(r rune) rune {
	switch {
	case unicode.IsUpper(r):
		return unicode.ToLower(r)
	case unicode.IsLower(r):
		return unicode.ToUpper(r)
	default:
		return r
	}
}

// Case-operator markers (used as Engine.pending values for gu/gU/g~ and the
// visual u/U/~). They're outside the printable ASCII operators (d/c/y) so
// they never collide with a real key.
const (
	opLower  rune = '\x01' // gu
	opUpper  rune = '\x02' // gU
	opToggle rune = '\x03' // g~
)

func isCaseOp(op rune) bool {
	return op == opLower || op == opUpper || op == opToggle
}

// Indent-operator markers (Engine.pending values for >> / << / visual > <).
const (
	opIndent rune = '\x04' // >
	opDedent rune = '\x05' // <
)

func isIndentOp(op rune) bool {
	return op == opIndent || op == opDedent
}

// indentLine prepends `levels` indent units to a line. Empty lines are left
// alone (vim doesn't indent them).
func indentLine(line []rune, unit string, levels int) []rune {
	if len(line) == 0 || levels < 1 {
		return line
	}
	return append([]rune(strings.Repeat(unit, levels)), line...)
}

// dedentLine removes up to width*levels columns of leading whitespace, a tab
// counting as width columns.
func dedentLine(line []rune, width, levels int) []rune {
	remove := width * levels
	removed, i := 0, 0
	for i < len(line) && removed < remove {
		switch line[i] {
		case ' ':
			removed++
		case '\t':
			removed += width
		default:
			return append([]rune{}, line[i:]...)
		}
		i++
	}
	return append([]rune{}, line[i:]...)
}

// transformCase applies a case operator to a rune.
func transformCase(r rune, op rune) rune {
	switch op {
	case opLower:
		return unicode.ToLower(r)
	case opUpper:
		return unicode.ToUpper(r)
	case opToggle:
		return toggleCase(r)
	}
	return r
}
