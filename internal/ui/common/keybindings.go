package common

import (
	"slices"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
)

// Binding builds a key.Binding for the given keybinding group+action,
// applying any override from options.tui.keybindings.<group>.<action>.
//
// Defaults and help metadata come from config.KeybindingCatalog (the
// single source of truth shared with the `crush keybindings` command).
// The help label is preserved; the help shortcut reflects the (possibly
// overridden) primary key so the footer shows what's actually bound. A
// nil cfg yields the catalog defaults.
//
// An unknown group/action yields an empty binding — call sites use the
// config.Keybind* constants, so this should never happen at runtime; it
// just keeps a typo from panicking.
func Binding(cfg *config.Config, group, action string) key.Binding {
	d, ok := config.LookupKeybinding(group, action)
	if !ok {
		return key.NewBinding()
	}
	keys := cfg.ResolveKeybinding(group, action, d.Defaults...)
	if d.HelpLabel == "" {
		return key.NewBinding(key.WithKeys(keys...))
	}
	shortcut := d.HelpShortcut
	if !slices.Equal(keys, d.Defaults) && len(keys) > 0 {
		shortcut = keys[0]
	}
	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(shortcut, d.HelpLabel),
	)
}
