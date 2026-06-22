package styles

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCustomThemeMalformedColorFallsBackAndWarns(t *testing.T) {
	// Not parallel: swaps the process-global slog default to capture output.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	// "red" (a color name) and "#fff" (3-digit) are both invalid: lipgloss
	// would silently swallow them, leaving an invisible role. Each must fall
	// back to the base color and emit a warning.
	base := CharmtonePantera()
	theme := Theme("custom", "", config.Themes{
		"custom": {
			Primary: "red",
			BgBase:  "#fff",
		},
	})

	require.Equal(t, base.Background, theme.Background,
		"invalid bg_base must fall back to the base theme's background")
	require.Equal(t,
		base.Markdown.CodeBlock.Chroma.KeywordNamespace.Color,
		theme.Markdown.CodeBlock.Chroma.KeywordNamespace.Color,
		"invalid primary must fall back to the base theme's primary")

	logs := buf.String()
	require.Equal(t, 2, strings.Count(logs, "Invalid custom theme color"),
		"expected a warning per malformed color, got:\n%s", logs)
	require.Contains(t, logs, "value=red")
	require.Contains(t, logs, "value=#fff")
}

func TestCustomThemeValidColorNoWarning(t *testing.T) {
	// Not parallel: swaps the process-global slog default to capture output.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	theme := Theme("custom", "", config.Themes{
		"custom": {BgBase: "#010203"},
	})

	require.Equal(t, "#010203", *hex(theme.Background))
	require.NotContains(t, buf.String(), "Invalid custom theme color")
}
