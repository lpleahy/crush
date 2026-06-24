package list

import (
	"image"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// TestHighlightRanges checks the multi-span highlighter that powers
// showing every search match at once: spans are applied in order over a
// single buffer, so later spans win on overlap, and an all-skipped span
// list returns the original content. A content-rewriting highlighter
// makes "which cells got styled" directly assertable.
func TestHighlightRanges(t *testing.T) {
	area := image.Rect(0, 0, 11, 1)
	star := func(_, _ int, c *uv.Cell) *uv.Cell {
		if c != nil {
			c.Content = "*"
		}
		return c
	}
	hash := func(_, _ int, c *uv.Cell) *uv.Cell {
		if c != nil {
			c.Content = "#"
		}
		return c
	}
	strip := func(s string) string { return strings.TrimRight(ansi.Strip(s), " \n") }

	t.Run("two non-overlapping ranges both apply", func(t *testing.T) {
		out := HighlightRanges("hello world", area, []HighlightSpan{
			{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5, Highlighter: star},
			{StartLine: 0, StartCol: 6, EndLine: 0, EndCol: 11, Highlighter: star},
		})
		if got := strip(out); got != "***** *****" {
			t.Errorf("got %q, want %q", got, "***** *****")
		}
	})

	t.Run("later span wins on overlap", func(t *testing.T) {
		out := HighlightRanges("hello world", area, []HighlightSpan{
			{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 11, Highlighter: star},
			{StartLine: 0, StartCol: 3, EndLine: 0, EndCol: 6, Highlighter: hash},
		})
		if got := strip(out); got != "***###*****" {
			t.Errorf("got %q, want %q", got, "***###*****")
		}
	})

	t.Run("negative spans skipped -> original content", func(t *testing.T) {
		out := HighlightRanges("hello world", area, []HighlightSpan{
			{StartLine: -1, StartCol: -1, EndLine: 0, EndCol: 5, Highlighter: star},
		})
		if out != "hello world" {
			t.Errorf("all-skipped spans should return original, got %q", out)
		}
	})
}
