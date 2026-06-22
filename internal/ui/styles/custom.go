package styles

import (
	"image/color"
	"log/slog"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
)

// hexColorPattern matches the 6-digit #RRGGBB form custom theme colors must
// use. lipgloss.Color silently yields no-color for anything else (e.g. a
// color name like "red" or a 3-digit "#fff"), so we warn at parse time.
var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func customTheme(name, providerID string, themes config.Themes) (Styles, bool) {
	if len(themes) == 0 {
		return Styles{}, false
	}

	cfg, ok := lookupCustomTheme(name, themes)
	if !ok {
		return Styles{}, false
	}

	opts := applyCustomPalette(builtinThemeOptions(cfg.Extends, providerID), cfg)
	s := quickStyle(opts)
	// Extending tokyonight-storm must also inherit its rich markdown/syntax/
	// diff post-processing, not just the semantic palette — otherwise a
	// custom theme would silently drop those overrides.
	if extendsTokyoNightStorm(cfg.Extends) {
		s = applyTokyoNightStormOverrides(s, opts)
	}
	return s, true
}

func lookupCustomTheme(name string, themes config.Themes) (config.ThemeConfig, bool) {
	normalized := normalizeThemeName(name)
	for id, cfg := range themes {
		if normalizeThemeName(id) == normalized {
			return cfg, true
		}
	}
	return config.ThemeConfig{}, false
}

func applyCustomPalette(base quickStyleOpts, cfg config.ThemeConfig) quickStyleOpts {
	base.primary = colorOr(base.primary, cfg.Primary)
	base.secondary = colorOr(base.secondary, cfg.Secondary)
	base.accent = colorOr(base.accent, cfg.Accent)
	base.keyword = colorOr(base.keyword, cfg.Keyword)
	base.fgBase = colorOr(base.fgBase, cfg.FgBase)
	base.fgSubtle = colorOr(base.fgSubtle, cfg.FgSubtle)
	base.fgMoreSubtle = colorOr(base.fgMoreSubtle, cfg.FgMoreSubtle)
	base.fgMostSubtle = colorOr(base.fgMostSubtle, cfg.FgMostSubtle)
	base.onPrimary = colorOr(base.onPrimary, cfg.OnPrimary)
	base.bgBase = colorOr(base.bgBase, cfg.BgBase)
	base.bgLeastVisible = colorOr(base.bgLeastVisible, cfg.BgLeastVisible)
	base.bgLessVisible = colorOr(base.bgLessVisible, cfg.BgLessVisible)
	base.bgMostVisible = colorOr(base.bgMostVisible, cfg.BgMostVisible)
	base.separator = colorOr(base.separator, cfg.Separator)
	base.destructive = colorOr(base.destructive, cfg.Destructive)
	base.error = colorOr(base.error, cfg.Error)
	base.warning = colorOr(base.warning, cfg.Warning)
	base.warningSubtle = colorOr(base.warningSubtle, cfg.WarningSubtle)
	base.denied = colorOr(base.denied, cfg.Denied)
	base.busy = colorOr(base.busy, cfg.Busy)
	base.info = colorOr(base.info, cfg.Info)
	base.infoMoreSubtle = colorOr(base.infoMoreSubtle, cfg.InfoMoreSubtle)
	base.infoMostSubtle = colorOr(base.infoMostSubtle, cfg.InfoMostSubtle)
	base.success = colorOr(base.success, cfg.Success)
	base.successMoreSubtle = colorOr(base.successMoreSubtle, cfg.SuccessMoreSubtle)
	base.successMostSubtle = colorOr(base.successMostSubtle, cfg.SuccessMostSubtle)
	return base
}

func colorOr(fallback color.Color, value string) color.Color {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if !hexColorPattern.MatchString(value) {
		// lipgloss.Color would silently return no-color here, leaving the
		// theme with an invisible role. Warn and keep the base color instead.
		slog.Warn("Invalid custom theme color, expected #RRGGBB; using base color",
			"value", value)
		return fallback
	}
	return lipgloss.Color(value)
}
