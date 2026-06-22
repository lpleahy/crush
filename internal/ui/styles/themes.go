package styles

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/x/exp/charmtone"
)

const (
	ThemeAuto                = "auto"
	ThemeCharmtonePantera    = "charmtone-pantera"
	ThemeHypercrushObsidiana = "hypercrush-obsidiana"
	ThemeTokyoNightStorm     = "tokyonight-storm"
)

// Theme returns the Styles associated with a configured theme name. Empty,
// "auto", and unknown names fall back to the provider-aware default.
func Theme(themeName, providerID string, themes config.Themes) Styles {
	if s, ok := customTheme(themeName, providerID, themes); ok {
		return s
	}
	return builtinTheme(themeName, providerID)
}

// ThemeForProvider returns the Styles associated with the given provider ID.
// Unknown or empty provider IDs yield the default Charmtone Pantera theme.
func ThemeForProvider(providerID string) Styles {
	return providerTheme(providerID)
}

func builtinTheme(themeName, providerID string) Styles {
	switch normalizeThemeName(themeName) {
	case "", ThemeAuto:
		return providerTheme(providerID)
	case ThemeCharmtonePantera, "charmtone", "pantera":
		return CharmtonePantera()
	case ThemeHypercrushObsidiana, "hyper", "obsidiana":
		return HypercrushObsidiana()
	case ThemeTokyoNightStorm, "tokyonight", "tokyonight_storm", "tokyo-night-storm", "tokyo-night":
		return TokyoNightStorm()
	default:
		return providerTheme(providerID)
	}
}

func builtinThemeOptions(themeName, providerID string) quickStyleOpts {
	switch normalizeThemeName(themeName) {
	case "", ThemeAuto:
		return providerThemeOptions(providerID)
	case ThemeCharmtonePantera, "charmtone", "pantera":
		return charmtonePanteraOptions()
	case ThemeHypercrushObsidiana, "hyper", "obsidiana":
		return hypercrushObsidianaOptions()
	case ThemeTokyoNightStorm, "tokyonight", "tokyonight_storm", "tokyo-night-storm", "tokyo-night":
		return tokyoNightStormOptions()
	default:
		return providerThemeOptions(providerID)
	}
}

func providerTheme(providerID string) Styles {
	return quickStyle(providerThemeOptions(providerID))
}

func providerThemeOptions(providerID string) quickStyleOpts {
	switch providerID {
	case "hyper":
		return hypercrushObsidianaOptions()
	default:
		return charmtonePanteraOptions()
	}
}

func normalizeThemeName(themeName string) string {
	return strings.ToLower(strings.TrimSpace(themeName))
}

// CharmtonePantera returns the Charmtone dark theme. It's the default style
// for the UI.
func CharmtonePantera() Styles {
	s := quickStyle(charmtonePanteraOptions())

	// Bang ! prompt overrides - use Salt/Hazy/Larple colors.
	s.Editor.PromptBangIconFocused = s.Editor.PromptBangIconFocused.
		Foreground(charmtone.Salt).
		Background(charmtone.Hazy)
	s.Editor.PromptBangDotsFocused = s.Editor.PromptBangDotsFocused.
		Foreground(charmtone.Hazy)
	s.Editor.PromptBangDotsBlurred = s.Editor.PromptBangDotsBlurred.
		Foreground(charmtone.Larple)

	// Shell bar/prompt overrides - use Charple/Iron/Hazy colors.
	s.Messages.ShellBarFocused = s.Messages.ShellBarFocused.
		BorderForeground(charmtone.Charple)
	s.Messages.ShellBarBlurred = s.Messages.ShellBarBlurred.
		BorderForeground(charmtone.Iron)
	s.Messages.ShellPrompt = s.Messages.ShellPrompt.
		Foreground(charmtone.Hazy)
	s.Messages.ShellPromptBlurred = s.Messages.ShellPromptBlurred.
		Foreground(charmtone.Hazy)

	return s
}

func charmtonePanteraOptions() quickStyleOpts {
	return quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Bok,
		keyword:   charmtone.Blush,

		fgBase:       charmtone.Sash,
		fgMoreSubtle: charmtone.Squid,
		fgSubtle:     charmtone.Smoke,
		fgMostSubtle: charmtone.Oyster,

		onPrimary: charmtone.Butter,

		bgBase:         charmtone.Pepper,
		bgLeastVisible: charmtone.BBQ,
		bgLessVisible:  charmtone.Char,
		bgMostVisible:  charmtone.Iron,

		separator: charmtone.Char,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Zest,
		warning:           charmtone.Mustard,
		denied:            charmtone.Tang,
		busy:              charmtone.Citron,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Julep,
		successMoreSubtle: charmtone.Bok,
		successMostSubtle: charmtone.Guac,

		// ANSI 16-color palette for remapping raw terminal output
		// (e.g. bang-mode shell commands) onto legible Charmtone colors.
		ansiBlack:   charmtone.BBQ,
		ansiRed:     charmtone.Coral,
		ansiGreen:   charmtone.Guac,
		ansiYellow:  charmtone.Mustard,
		ansiBlue:    charmtone.Charple,
		ansiMagenta: charmtone.Dolly,
		ansiCyan:    charmtone.Malibu,
		ansiWhite:   charmtone.Smoke,

		ansiBrightBlack:   charmtone.Iron,
		ansiBrightRed:     charmtone.Tuna,
		ansiBrightGreen:   charmtone.Julep,
		ansiBrightYellow:  charmtone.Zest,
		ansiBrightBlue:    charmtone.Guppy,
		ansiBrightMagenta: charmtone.Blush,
		ansiBrightCyan:    charmtone.Sardine,
		ansiBrightWhite:   charmtone.Salt,
	}
}

// HypercrushObsidiana returns the Hypercrush dark theme.
func HypercrushObsidiana() Styles {
	s := quickStyle(hypercrushObsidianaOptions())

	// Bang ! prompt overrides - use Salt/Hazy/Larple colors. Upstream's
	// HypercrushObsidiana inherited these via CharmtonePantera; the refactor
	// to a bare quickStyle dropped them, so restore them here.
	s.Editor.PromptBangIconFocused = s.Editor.PromptBangIconFocused.
		Foreground(charmtone.Salt).
		Background(charmtone.Hazy)
	s.Editor.PromptBangDotsFocused = s.Editor.PromptBangDotsFocused.
		Foreground(charmtone.Hazy)
	s.Editor.PromptBangDotsBlurred = s.Editor.PromptBangDotsBlurred.
		Foreground(charmtone.Larple)

	// Shell bar/prompt overrides - use Charple/Iron/Hazy colors.
	s.Messages.ShellBarFocused = s.Messages.ShellBarFocused.
		BorderForeground(charmtone.Charple)
	s.Messages.ShellBarBlurred = s.Messages.ShellBarBlurred.
		BorderForeground(charmtone.Iron)
	s.Messages.ShellPrompt = s.Messages.ShellPrompt.
		Foreground(charmtone.Hazy)
	s.Messages.ShellPromptBlurred = s.Messages.ShellPromptBlurred.
		Foreground(charmtone.Hazy)

	return s
}

func hypercrushObsidianaOptions() quickStyleOpts {
	return quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Bok,
		keyword:   charmtone.Blush,

		fgBase:       charmtone.Sash,
		fgMoreSubtle: charmtone.Squid,
		fgSubtle:     charmtone.Smoke,
		fgMostSubtle: charmtone.Oyster,

		onPrimary: charmtone.Butter,

		bgBase:         charmtone.Pepper,
		bgLeastVisible: charmtone.BBQ,
		bgLessVisible:  charmtone.Char,
		bgMostVisible:  charmtone.Iron,

		separator: charmtone.Char,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Zest,
		warning:           charmtone.Mustard,
		denied:            charmtone.Tang,
		busy:              charmtone.Citron,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Julep,
		successMoreSubtle: charmtone.Bok,
		successMostSubtle: charmtone.Guac,

		// ANSI 16-color palette. Hypercrush reuses Charmtone's palette, as
		// upstream's HypercrushObsidiana inherited it via CharmtonePantera.
		// Without these, quickStyle would build s.ANSI with nil entries and
		// RemapANSI16 would emit malformed escapes for raw 16-color terminal
		// output (e.g. bang-mode shell commands).
		ansiBlack:   charmtone.BBQ,
		ansiRed:     charmtone.Coral,
		ansiGreen:   charmtone.Guac,
		ansiYellow:  charmtone.Mustard,
		ansiBlue:    charmtone.Charple,
		ansiMagenta: charmtone.Dolly,
		ansiCyan:    charmtone.Malibu,
		ansiWhite:   charmtone.Smoke,

		ansiBrightBlack:   charmtone.Iron,
		ansiBrightRed:     charmtone.Tuna,
		ansiBrightGreen:   charmtone.Julep,
		ansiBrightYellow:  charmtone.Zest,
		ansiBrightBlue:    charmtone.Guppy,
		ansiBrightMagenta: charmtone.Blush,
		ansiBrightCyan:    charmtone.Sardine,
		ansiBrightWhite:   charmtone.Salt,
	}
}

// TokyoNightStorm returns a dark theme based on the upstream TokyoNight Storm
// palette.
func TokyoNightStorm() Styles {
	o := tokyoNightStormOptions()
	return applyTokyoNightStormOverrides(quickStyle(o), o)
}

// extendsTokyoNightStorm reports whether name is a TokyoNight Storm alias,
// so a custom theme that extends it gets the same rich post-processing as
// the built-in. The alias set mirrors builtinThemeOptions.
func extendsTokyoNightStorm(name string) bool {
	switch normalizeThemeName(name) {
	case ThemeTokyoNightStorm, "tokyonight", "tokyonight_storm", "tokyo-night-storm", "tokyo-night":
		return true
	}
	return false
}

// applyTokyoNightStormOverrides layers TokyoNight Storm's rich markdown,
// syntax, diff, and hypercredit colors onto a Styles built from o — the
// colors quickStyle keeps explicit so the theme stays inside the palette.
//
// Colors that map to a semantic palette role are taken from o, so a custom
// theme extending tokyonight-storm honors those overrides (e.g. a custom
// primary flows into the namespace/function syntax colors). The remaining
// TokyoNight-specific shades with no semantic role (link/operator blue,
// keyword-type blue, the diff backgrounds) come from the fixed palette. For
// the built-in theme o == tokyoNightStormOptions(), where every mapped role
// already equals its palette color, so the result is unchanged.
func applyTokyoNightStormOverrides(s Styles, o quickStyleOpts) Styles {
	c := tokyoNightStormPalette

	s.Markdown.Link.Color = hex(c.blue5)
	s.Markdown.LinkText.Color = hex(o.accent)
	s.Markdown.Image.Color = hex(o.secondary)
	s.Markdown.CodeBlock.Chroma.CommentPreproc.Color = hex(o.accent)
	s.Markdown.CodeBlock.Chroma.Keyword.Color = hex(o.secondary)
	s.Markdown.CodeBlock.Chroma.KeywordReserved.Color = hex(o.keyword)
	s.Markdown.CodeBlock.Chroma.KeywordNamespace.Color = hex(o.primary)
	s.Markdown.CodeBlock.Chroma.KeywordType.Color = hex(c.blue1)
	s.Markdown.CodeBlock.Chroma.Operator.Color = hex(c.blue5)
	s.Markdown.CodeBlock.Chroma.Punctuation.Color = hex(o.fgMoreSubtle)
	s.Markdown.CodeBlock.Chroma.NameBuiltin.Color = hex(o.accent)
	s.Markdown.CodeBlock.Chroma.NameTag.Color = hex(o.successMostSubtle)
	s.Markdown.CodeBlock.Chroma.NameAttribute.Color = hex(o.warningSubtle)
	s.Markdown.CodeBlock.Chroma.NameClass.Color = hex(o.primary)
	s.Markdown.CodeBlock.Chroma.NameDecorator.Color = hex(o.warningSubtle)
	s.Markdown.CodeBlock.Chroma.NameFunction.Color = hex(o.primary)
	s.Markdown.CodeBlock.Chroma.LiteralNumber.Color = hex(o.warning)
	s.Markdown.CodeBlock.Chroma.LiteralString.Color = hex(o.success)
	s.Markdown.CodeBlock.Chroma.LiteralStringEscape.Color = hex(o.accent)
	s.Markdown.CodeBlock.Chroma.GenericDeleted.Color = hex(o.error)
	s.Markdown.CodeBlock.Chroma.GenericInserted.Color = hex(o.success)

	s.Diff.InsertLine.LineNumber = lipgloss.NewStyle().Foreground(o.successMostSubtle).Background(c.gitAddBg)
	s.Diff.InsertLine.Symbol = lipgloss.NewStyle().Foreground(o.successMostSubtle).Background(c.gitAddBg)
	s.Diff.InsertLine.Code = lipgloss.NewStyle().Background(c.gitAddBg)
	s.Diff.DeleteLine.LineNumber = lipgloss.NewStyle().Foreground(o.error).Background(c.gitDeleteBg)
	s.Diff.DeleteLine.Symbol = lipgloss.NewStyle().Foreground(o.error).Background(c.gitDeleteBg)
	s.Diff.DeleteLine.Code = lipgloss.NewStyle().Background(c.gitDeleteBg)

	s.Header.HypercreditIcon = lipgloss.NewStyle().Foreground(o.warningSubtle)
	s.ModelInfo.HypercreditIcon = lipgloss.NewStyle().Foreground(o.warningSubtle)

	return s
}

func tokyoNightStormOptions() quickStyleOpts {
	c := tokyoNightStormPalette
	return quickStyleOpts{
		primary:   c.blue,
		secondary: c.magenta,
		accent:    c.cyan,
		keyword:   c.purple,

		fgBase:       c.fg,
		fgSubtle:     c.fgDark,
		fgMoreSubtle: c.dark5,
		fgMostSubtle: c.comment,

		onPrimary: c.bgDark1,

		bgBase:         c.bg,
		bgLeastVisible: c.bgDark,
		bgLessVisible:  c.bgHighlight,
		bgMostVisible:  c.fgGutter,

		separator: c.fgGutter,

		destructive:       c.red1,
		error:             c.red,
		warningSubtle:     c.yellow,
		warning:           c.orange,
		denied:            c.red1,
		busy:              c.yellow,
		info:              c.blue,
		infoMoreSubtle:    c.blue0,
		infoMostSubtle:    c.blue7,
		success:           c.green,
		successMoreSubtle: c.green2,
		successMostSubtle: c.green1,

		// ANSI 16-color palette from TokyoNight Storm's published terminal
		// palette. Without these, quickStyle would build s.ANSI with nil
		// entries and RemapANSI16 would emit malformed escapes for raw
		// 16-color terminal output (e.g. bang-mode shell commands).
		ansiBlack:   c.black,
		ansiRed:     c.red,
		ansiGreen:   c.green,
		ansiYellow:  c.yellow,
		ansiBlue:    c.blue,
		ansiMagenta: c.magenta,
		ansiCyan:    c.cyan,
		ansiWhite:   c.fgDark,

		ansiBrightBlack:   c.terminalBlack,
		ansiBrightRed:     c.red,
		ansiBrightGreen:   c.green,
		ansiBrightYellow:  c.yellow,
		ansiBrightBlue:    c.blue,
		ansiBrightMagenta: c.magenta,
		ansiBrightCyan:    c.cyan,
		ansiBrightWhite:   c.fg,
	}
}

var tokyoNightStormPalette = struct {
	black         color.Color
	terminalBlack color.Color
	bg            color.Color
	bgDark        color.Color
	bgDark1       color.Color
	bgHighlight   color.Color
	blue          color.Color
	blue0         color.Color
	blue1         color.Color
	blue5         color.Color
	blue7         color.Color
	comment       color.Color
	cyan          color.Color
	dark5         color.Color
	fg            color.Color
	fgDark        color.Color
	fgGutter      color.Color
	green         color.Color
	green1        color.Color
	green2        color.Color
	magenta       color.Color
	orange        color.Color
	purple        color.Color
	red           color.Color
	red1          color.Color
	yellow        color.Color
	gitAddBg      color.Color
	gitDeleteBg   color.Color
}{
	// Terminal ANSI black / bright-black, from TokyoNight's published
	// terminal palette (not used elsewhere in the UI palette).
	black:         lipgloss.Color("#1d202f"),
	terminalBlack: lipgloss.Color("#414868"),
	bg:            lipgloss.Color("#24283b"),
	bgDark:        lipgloss.Color("#1f2335"),
	bgDark1:       lipgloss.Color("#1b1e2d"),
	bgHighlight:   lipgloss.Color("#292e42"),
	blue:          lipgloss.Color("#7aa2f7"),
	blue0:         lipgloss.Color("#3d59a1"),
	blue1:         lipgloss.Color("#2ac3de"),
	blue5:         lipgloss.Color("#89ddff"),
	blue7:         lipgloss.Color("#394b70"),
	comment:       lipgloss.Color("#565f89"),
	cyan:          lipgloss.Color("#7dcfff"),
	dark5:         lipgloss.Color("#737aa2"),
	fg:            lipgloss.Color("#c0caf5"),
	fgDark:        lipgloss.Color("#a9b1d6"),
	fgGutter:      lipgloss.Color("#3b4261"),
	green:         lipgloss.Color("#9ece6a"),
	green1:        lipgloss.Color("#73daca"),
	green2:        lipgloss.Color("#41a6b5"),
	magenta:       lipgloss.Color("#bb9af7"),
	orange:        lipgloss.Color("#ff9e64"),
	purple:        lipgloss.Color("#9d7cd8"),
	red:           lipgloss.Color("#f7768e"),
	red1:          lipgloss.Color("#db4b4b"),
	yellow:        lipgloss.Color("#e0af68"),
	gitAddBg:      lipgloss.Color("#1d3b3a"),
	gitDeleteBg:   lipgloss.Color("#3b2634"),
}
