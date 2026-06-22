package completions

import (
	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// KeyMap defines the key bindings for the completions component.
type KeyMap struct {
	Down,
	Up,
	Select,
	Cancel key.Binding
	DownInsert,
	UpInsert key.Binding
}

// DefaultKeyMap returns the default key bindings for completions.
func DefaultKeyMap(cfg *config.Config) KeyMap {
	return KeyMap{
		Down:       common.Binding(cfg, config.KeybindingGroupCompletions, "down"),
		Up:         common.Binding(cfg, config.KeybindingGroupCompletions, "up"),
		Select:     common.Binding(cfg, config.KeybindingGroupCompletions, "select"),
		Cancel:     common.Binding(cfg, config.KeybindingGroupCompletions, "cancel"),
		DownInsert: common.Binding(cfg, config.KeybindingGroupCompletions, "down_insert"),
		UpInsert:   common.Binding(cfg, config.KeybindingGroupCompletions, "up_insert"),
	}
}

// KeyBindings returns all key bindings as a slice.
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Down,
		k.Up,
		k.Select,
		k.Cancel,
	}
}

// FullHelp returns the full help for the key bindings.
func (k KeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp returns the short help for the key bindings.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
	}
}
