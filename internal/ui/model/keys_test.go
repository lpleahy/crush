package model

import (
	"slices"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
)

func cfgWithGlobal(overrides map[string][]string) *config.Config {
	tui := &config.TUIOptions{}
	if overrides != nil {
		tui.Keybindings = map[string]map[string][]string{
			config.KeybindingGroupGlobal: overrides,
		}
	}
	return &config.Config{Options: &config.Options{TUI: tui}}
}

func TestDefaultKeyMap_NilConfigUsesDefaults(t *testing.T) {
	km := DefaultKeyMap(nil)
	if got := km.Models.Keys(); !slices.Equal(got, []string{"ctrl+m", "ctrl+l"}) {
		t.Errorf("Models default keys = %v", got)
	}
	if got := km.Commands.Keys(); !slices.Equal(got, []string{"ctrl+p"}) {
		t.Errorf("Commands default keys = %v", got)
	}
}

func TestDefaultKeyMap_OverrideAppliesKeysAndHelp(t *testing.T) {
	km := DefaultKeyMap(cfgWithGlobal(map[string][]string{
		config.KeybindActionModels: {"alt+m"},
	}))

	if got := km.Models.Keys(); !slices.Equal(got, []string{"alt+m"}) {
		t.Errorf("Models override keys = %v, want [alt+m]", got)
	}
	// Help shortcut should reflect the new primary key; label preserved.
	h := km.Models.Help()
	if h.Key != "alt+m" {
		t.Errorf("Models help key = %q, want alt+m", h.Key)
	}
	if h.Desc != "models" {
		t.Errorf("Models help desc = %q, want models", h.Desc)
	}
}

func TestDefaultKeyMap_NonOverriddenKeepsCustomHelpShortcut(t *testing.T) {
	// Chat.Copy binds [c, y, C, Y] but advertises the custom "c/y"
	// shortcut. Without an override, that custom shortcut must be
	// preserved (not auto-derived from the first bound key).
	km := DefaultKeyMap(cfgWithGlobal(nil))
	if h := km.Chat.Copy.Help(); h.Key != "c/y" {
		t.Errorf("non-overridden Chat.Copy help key = %q, want c/y", h.Key)
	}
}

func TestDefaultKeyMap_EmptyOverrideFallsBack(t *testing.T) {
	km := DefaultKeyMap(cfgWithGlobal(map[string][]string{
		config.KeybindActionModels: {},
	}))
	if got := km.Models.Keys(); !slices.Equal(got, []string{"ctrl+m", "ctrl+l"}) {
		t.Errorf("empty override should fall back, got %v", got)
	}
}
