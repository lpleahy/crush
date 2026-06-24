package list

import (
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/stringext"
	uv "github.com/charmbracelet/ultraviolet"
)

// DefaultHighlighter is the default highlighter function that applies inverse style.
var DefaultHighlighter Highlighter = func(x, y int, c *uv.Cell) *uv.Cell {
	if c == nil {
		return c
	}
	c.Style.Attrs |= uv.AttrReverse
	return c
}

// Highlighter represents a function that defines how to highlight text.
type Highlighter func(x, y int, c *uv.Cell) *uv.Cell

// HighlightContent returns the content with highlighted regions based on the specified parameters.
func HighlightContent(content string, area image.Rectangle, startLine, startCol, endLine, endCol int) string {
	var sb strings.Builder
	pos := image.Pt(-1, -1)
	HighlightBuffer(content, area, startLine, startCol, endLine, endCol, func(x, y int, c *uv.Cell) *uv.Cell {
		pos.X = x
		if pos.Y == -1 {
			pos.Y = y
		} else if y > pos.Y {
			sb.WriteString(strings.Repeat("\n", y-pos.Y))
			pos.Y = y
		}
		sb.WriteString(c.Content)
		return c
	})
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	return sb.String()
}

// Highlight highlights a region of text within the given content and region.
func Highlight(content string, area image.Rectangle, startLine, startCol, endLine, endCol int, highlighter Highlighter) string {
	buf := HighlightBuffer(content, area, startLine, startCol, endLine, endCol, highlighter)
	if buf == nil {
		return content
	}
	return buf.Render()
}

// HighlightBuffer highlights a region of text within the given content and
// region, returning a [uv.ScreenBuffer].
func HighlightBuffer(content string, area image.Rectangle, startLine, startCol, endLine, endCol int, highlighter Highlighter) *uv.ScreenBuffer {
	content = stringext.NormalizeSpace(content)

	if startLine < 0 || startCol < 0 {
		return nil
	}

	buf := drawBuffer(content, area)
	applyHighlightSpan(buf, area, startLine, startCol, endLine, endCol, highlighter)
	return buf
}

// HighlightSpan is one region to highlight, with its own highlighter, as
// passed to [HighlightRanges].
type HighlightSpan struct {
	StartLine, StartCol, EndLine, EndCol int
	Highlighter                          Highlighter
}

// HighlightRanges highlights several regions of content in a single pass.
// Each span is drawn over the same buffer in order, so later spans win on
// overlapping cells (e.g. an active match drawn on top of a dim one).
// Spans with a negative StartLine/StartCol are skipped. When no span
// applies, the original content is returned unchanged.
func HighlightRanges(content string, area image.Rectangle, spans []HighlightSpan) string {
	buf := drawBuffer(stringext.NormalizeSpace(content), area)
	applied := false
	for _, sp := range spans {
		if sp.StartLine < 0 || sp.StartCol < 0 {
			continue
		}
		applyHighlightSpan(buf, area, sp.StartLine, sp.StartCol, sp.EndLine, sp.EndCol, sp.Highlighter)
		applied = true
	}
	if !applied {
		return content
	}
	return buf.Render()
}

// drawBuffer renders content into a fresh screen buffer sized to area.
func drawBuffer(content string, area image.Rectangle) *uv.ScreenBuffer {
	buf := uv.NewScreenBuffer(area.Dx(), area.Dy())
	uv.NewStyledString(content).Draw(&buf, area)
	return &buf
}

// applyHighlightSpan applies highlighter to the cells of buf within the
// given range. -1 for endLine/endCol means "to the end of content". Only
// cells that hold content (not trailing blanks) are styled, matching the
// single-range behavior. Out-of-range starts are a no-op.
func applyHighlightSpan(buf *uv.ScreenBuffer, area image.Rectangle, startLine, startCol, endLine, endCol int, highlighter Highlighter) {
	if startLine < 0 || startCol < 0 {
		return
	}
	if highlighter == nil {
		highlighter = DefaultHighlighter
	}

	width, height := area.Dx(), area.Dy()

	// Treat -1 as "end of content"
	if endLine < 0 {
		endLine = height - 1
	}
	if endCol < 0 {
		endCol = width
	}

	for y := startLine; y <= endLine && y < height; y++ {
		if y >= buf.Height() {
			break
		}

		line := buf.Line(y)

		// Determine column range for this line
		colStart := 0
		if y == startLine {
			colStart = min(startCol, len(line))
		}

		colEnd := len(line)
		if y == endLine {
			colEnd = min(endCol, len(line))
		}

		// Track last non-empty position as we go
		lastContentX := -1

		// Single pass: check content and track last non-empty position
		for x := colStart; x < colEnd; x++ {
			cell := line.At(x)
			if cell == nil {
				continue
			}

			// Update last content position if non-empty
			if cell.Content != "" && cell.Content != " " {
				lastContentX = x
			}
		}

		// Only apply highlight up to last content position
		highlightEnd := colEnd
		if lastContentX >= 0 {
			highlightEnd = lastContentX + 1
		} else if lastContentX == -1 {
			highlightEnd = colStart // No content on this line
		}

		// Apply highlight style only to cells with content
		for x := colStart; x < highlightEnd; x++ {
			if !image.Pt(x, y).In(area) {
				continue
			}
			cell := line.At(x)
			if cell != nil {
				highlighter(x, y, cell)
			}
		}
	}
}

// ToHighlighter converts a [lipgloss.Style] to a [Highlighter].
func ToHighlighter(lgStyle lipgloss.Style) Highlighter {
	return func(_ int, _ int, c *uv.Cell) *uv.Cell {
		if c != nil {
			c.Style = ToStyle(lgStyle)
		}
		return c
	}
}

// ToStyle converts an inline [lipgloss.Style] to a [uv.Style].
func ToStyle(lgStyle lipgloss.Style) uv.Style {
	var uvStyle uv.Style

	// Colors are already color.Color
	uvStyle.Fg = lgStyle.GetForeground()
	uvStyle.Bg = lgStyle.GetBackground()

	// Build attributes using bitwise OR
	var attrs uint8

	if lgStyle.GetBold() {
		attrs |= uv.AttrBold
	}

	if lgStyle.GetItalic() {
		attrs |= uv.AttrItalic
	}

	if lgStyle.GetUnderline() {
		uvStyle.Underline = uv.UnderlineSingle
	}

	if lgStyle.GetStrikethrough() {
		attrs |= uv.AttrStrikethrough
	}

	if lgStyle.GetFaint() {
		attrs |= uv.AttrFaint
	}

	if lgStyle.GetBlink() {
		attrs |= uv.AttrBlink
	}

	if lgStyle.GetReverse() {
		attrs |= uv.AttrReverse
	}

	uvStyle.Attrs = attrs

	return uvStyle
}

// AdjustArea adjusts the given area rectangle by subtracting margins, borders,
// and padding from the style.
func AdjustArea(area image.Rectangle, style lipgloss.Style) image.Rectangle {
	topMargin, rightMargin, bottomMargin, leftMargin := style.GetMargin()
	topBorder, rightBorder, bottomBorder, leftBorder := style.GetBorderTopSize(),
		style.GetBorderRightSize(),
		style.GetBorderBottomSize(),
		style.GetBorderLeftSize()
	topPadding, rightPadding, bottomPadding, leftPadding := style.GetPadding()

	return image.Rectangle{
		Min: image.Point{
			X: area.Min.X + leftMargin + leftBorder + leftPadding,
			Y: area.Min.Y + topMargin + topBorder + topPadding,
		},
		Max: image.Point{
			X: area.Max.X - (rightMargin + rightBorder + rightPadding),
			Y: area.Max.Y - (bottomMargin + bottomBorder + bottomPadding),
		},
	}
}
