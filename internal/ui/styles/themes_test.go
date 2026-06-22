package styles

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestThemeUsesCustomTheme(t *testing.T) {
	t.Parallel()

	theme := Theme("custom", "", config.Themes{
		"custom": {
			Primary: "#112233",
			BgBase:  "#010203",
		},
	})

	require.Equal(t, "#010203", *hex(theme.Background))
	require.Equal(t, "#112233", *theme.Markdown.H1.BackgroundColor)
}

func TestThemeCustomLookupNormalizesName(t *testing.T) {
	t.Parallel()

	theme := Theme("  Custom Theme  ", "", config.Themes{
		"custom theme": {
			BgBase: "#102030",
		},
	})

	require.Equal(t, "#102030", *hex(theme.Background))
}

func TestThemeCustomThemeExtendsBuiltIn(t *testing.T) {
	t.Parallel()

	theme := Theme("custom", "", config.Themes{
		"custom": {
			Extends: ThemeTokyoNightStorm,
			BgBase:  "#102030",
		},
	})

	require.Equal(t, "#102030", *hex(theme.Background))
	require.Equal(t, "#7aa2f7", *hex(theme.WorkingGradFromColor))
}

func TestThemeUnknownCustomFallsBackToProviderTheme(t *testing.T) {
	t.Parallel()

	theme := Theme("missing", "hyper", config.Themes{
		"custom": {
			BgBase: "#102030",
		},
	})

	require.Equal(t, *hex(HypercrushObsidiana().Background), *hex(theme.Background))
}

func TestBuiltinThemesHaveCompleteANSIPalette(t *testing.T) {
	t.Parallel()

	// Every built-in theme must populate all 16 ANSI palette slots. A nil
	// entry makes quickStyle's RemapANSI16 emit malformed escapes and mangles
	// raw 16-color terminal output (e.g. bang-mode shell commands).
	cases := map[string]Styles{
		"CharmtonePantera":    CharmtonePantera(),
		"HypercrushObsidiana": HypercrushObsidiana(),
		"TokyoNightStorm":     TokyoNightStorm(),
	}
	for name, s := range cases {
		for i, c := range s.ANSI {
			require.NotNilf(t, c, "%s: s.ANSI[%d] is nil", name, i)
		}
	}
}

func TestHypercrushInheritsCharmtoneBangShellOverrides(t *testing.T) {
	t.Parallel()

	// Upstream's HypercrushObsidiana inherited Charmtone's bang-prompt and
	// shell-bar overrides; the refactor to a bare quickStyle dropped them.
	// Guard that they're restored by matching CharmtonePantera.
	hyper := HypercrushObsidiana()
	charm := CharmtonePantera()

	require.Equal(t,
		charm.Editor.PromptBangIconFocused.Render("!"),
		hyper.Editor.PromptBangIconFocused.Render("!"))
	require.Equal(t,
		charm.Editor.PromptBangDotsBlurred.Render("."),
		hyper.Editor.PromptBangDotsBlurred.Render("."))
	require.Equal(t,
		charm.Messages.ShellBarFocused.Render("x"),
		hyper.Messages.ShellBarFocused.Render("x"))
	require.Equal(t,
		charm.Messages.ShellPrompt.Render("$"),
		hyper.Messages.ShellPrompt.Render("$"))
}

func TestTokyoNightStormBuiltinRichColors(t *testing.T) {
	t.Parallel()

	// Guards the refactor: the built-in keeps its TokyoNight-specific
	// markdown/syntax colors (the post-processing layered on quickStyle).
	s := TokyoNightStorm()
	require.Equal(t, "#89ddff", *s.Markdown.Link.Color)                              // blue5 (no semantic role)
	require.Equal(t, "#7dcfff", *s.Markdown.LinkText.Color)                          // cyan == accent
	require.Equal(t, "#bb9af7", *s.Markdown.Image.Color)                             // magenta == secondary
	require.Equal(t, "#7aa2f7", *s.Markdown.CodeBlock.Chroma.KeywordNamespace.Color) // blue == primary
	require.Equal(t, "#9ece6a", *s.Markdown.CodeBlock.Chroma.LiteralString.Color)    // green == success
	require.Contains(t, s.Diff.InsertLine.Code.Render("x"), "48;2")                  // diff add background present
}

func TestCustomThemeExtendsTokyoNightInheritsRichColors(t *testing.T) {
	t.Parallel()

	// A custom theme that extends tokyonight-storm but overrides nothing
	// must render identically to the built-in for the rich post-processing
	// colors — not just the semantic palette.
	builtin := TokyoNightStorm()
	custom := Theme("lilo", "", config.Themes{
		"lilo": {Extends: ThemeTokyoNightStorm},
	})

	require.Equal(t, *builtin.Markdown.Link.Color, *custom.Markdown.Link.Color)
	require.Equal(t,
		*builtin.Markdown.CodeBlock.Chroma.KeywordNamespace.Color,
		*custom.Markdown.CodeBlock.Chroma.KeywordNamespace.Color)
	require.Equal(t,
		builtin.Diff.InsertLine.Code.Render("x"),
		custom.Diff.InsertLine.Code.Render("x"))
	require.Equal(t,
		builtin.Diff.DeleteLine.Code.Render("x"),
		custom.Diff.DeleteLine.Code.Render("x"))
}

func TestCustomThemeExtendsTokyoNightHonorsPrimaryOverride(t *testing.T) {
	t.Parallel()

	const customPrimary = "#abcdef"
	theme := Theme("lilo", "", config.Themes{
		"lilo": {Extends: ThemeTokyoNightStorm, Primary: customPrimary},
	})

	// The override flows into quickStyle-derived styles...
	require.Equal(t, customPrimary, *hex(theme.WorkingGradFromColor))
	// ...and into the TokyoNight syntax colors that map to the primary role.
	require.Equal(t, customPrimary, *theme.Markdown.CodeBlock.Chroma.KeywordNamespace.Color)
	require.Equal(t, customPrimary, *theme.Markdown.CodeBlock.Chroma.NameFunction.Color)
	// TokyoNight-specific shades with no semantic role stay put.
	require.Equal(t, "#89ddff", *theme.Markdown.Link.Color)
}

func TestCustomThemeExtendsCharmtoneSkipsTokyoNightOverrides(t *testing.T) {
	t.Parallel()

	// Extending a base without rich post-processing must match that base,
	// not pick up TokyoNight's link color.
	base := CharmtonePantera()
	custom := Theme("lilo", "", config.Themes{
		"lilo": {Extends: ThemeCharmtonePantera},
	})
	require.Equal(t, base.Markdown.Link.Color, custom.Markdown.Link.Color)
	if custom.Markdown.Link.Color != nil {
		require.NotEqual(t, "#89ddff", *custom.Markdown.Link.Color)
	}
}

func TestThemeCustomOverridesBuiltInName(t *testing.T) {
	t.Parallel()

	theme := Theme(ThemeTokyoNightStorm, "", config.Themes{
		ThemeTokyoNightStorm: {
			BgBase: "#102030",
		},
	})

	require.Equal(t, "#102030", *hex(theme.Background))
}

func TestTransparentRemovesNonDiffBackgrounds(t *testing.T) {
	t.Parallel()

	theme := Transparent(TokyoNightStorm())

	require.Nil(t, theme.Background)
	require.Nil(t, theme.Markdown.H1.BackgroundColor)
	require.Nil(t, theme.Markdown.Code.BackgroundColor)
	require.Nil(t, theme.Markdown.CodeBlock.Chroma.Background.BackgroundColor)
	require.Nil(t, theme.QuietMarkdown.Document.BackgroundColor)
	require.Nil(t, theme.Tool.ContentCodeBg)
	require.NotContains(t, theme.Tool.ContentCodeLine.Render("x"), "48;2")
	require.NotContains(t, theme.Dialog.ContentPanel.Render("x"), "48;2")
	require.Nil(t, theme.Dialog.Permissions.ParamsBg)
	require.Nil(t, theme.Dialog.OAuth.UserCodeBg)
}

func TestTransparentKeepsDiffInsertDeleteBackgrounds(t *testing.T) {
	t.Parallel()

	theme := Transparent(TokyoNightStorm())

	require.Contains(t, theme.Diff.InsertLine.Code.Render("x"), "48;2")
	require.Contains(t, theme.Diff.DeleteLine.Code.Render("x"), "48;2")
	require.NotContains(t, theme.Diff.EqualLine.Code.Render("x"), "48;2")
	require.NotContains(t, theme.Diff.Filename.Code.Render("x"), "48;2")
}
