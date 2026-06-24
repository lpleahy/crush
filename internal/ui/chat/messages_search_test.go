package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestHighlightableItem_SearchMatches covers the dim "all matches" layer:
// columns arrive with the left inset included and are stored
// content-relative, the version bumps only on a real change, and
// renderHighlighted styles the matched spans while preserving the text.
func TestHighlightableItem_SearchMatches(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := defaultHighlighter(&sty, list.NewVersioned())

	off := MessageLeftPaddingTotal
	ranges := []SearchRange{
		{Line: 0, StartCol: off + 0, EndCol: off + 5},
		{Line: 0, StartCol: off + 6, EndCol: off + 11},
	}

	before := item.version.Version()
	item.SetSearchMatches(ranges)
	require.Greater(t, item.version.Version(), before, "SetSearchMatches must bump version on change")
	require.Equal(t, []matchRange{
		{line: 0, startCol: 0, endCol: 5}, // inset subtracted
		{line: 0, startCol: 6, endCol: 11},
	}, item.matches)

	// Identical ranges must not bump the version (no spurious re-render).
	v := item.version.Version()
	item.SetSearchMatches(ranges)
	require.Equal(t, v, item.version.Version(), "identical SetSearchMatches must not bump")

	// renderHighlighted styles the spans (output differs) but keeps text.
	out := item.renderHighlighted("hello world", 11, 1)
	require.NotEqual(t, "hello world", out, "matches should add styling")
	require.Equal(t, "hello world", strings.TrimRight(ansi.Strip(out), " \n"))

	// Clearing drops the dim layer; with no active highlight either, the
	// content renders untouched.
	item.SetSearchMatches(nil)
	require.Empty(t, item.matches)
	require.Equal(t, "hello world", item.renderHighlighted("hello world", 11, 1))
}

// TestHighlightableItem_MatchesCountAsHighlighted guards the cache-bypass
// fix: an item with only dim matches (no active/single highlight) must
// still report isHighlighted(), or Render would serve the prefix cache
// (built without highlights) and the matches would never be drawn — and
// a stale highlight could survive into the next search.
func TestHighlightableItem_MatchesCountAsHighlighted(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := defaultHighlighter(&sty, list.NewVersioned())

	if item.isHighlighted() {
		t.Fatal("a fresh item should not be highlighted")
	}
	item.SetSearchMatches([]SearchRange{{Line: 0, StartCol: 0, EndCol: 3}})
	if !item.isHighlighted() {
		t.Error("an item with search matches must report isHighlighted() so Render draws them instead of serving the cache")
	}
	item.SetSearchMatches(nil)
	if item.isHighlighted() {
		t.Error("clearing matches should report not-highlighted again")
	}
}
