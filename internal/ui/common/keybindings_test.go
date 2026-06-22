package common

import (
	"slices"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
)

// cfgWithKeybindings builds a Config whose only populated field is the
// keybinding override map, so Binding's override path can be driven
// without loading a real config.
func cfgWithKeybindings(kb map[string]map[string][]string) *config.Config {
	return &config.Config{
		Options: &config.Options{
			TUI: &config.TUIOptions{Keybindings: kb},
		},
	}
}

// TestBinding_NilConfigUsesCatalogDefaults: a nil cfg must yield the
// catalog defaults, with the curated help shortcut preserved.
func TestBinding_NilConfigUsesCatalogDefaults(t *testing.T) {
	t.Parallel()

	// chat "copy" binds [c, y, C, Y] but its curated shortcut is "c/y".
	b := Binding(nil, config.KeybindingGroupChat, config.KeybindActionChatCopy)
	if got := b.Keys(); !slices.Equal(got, []string{"c", "y", "C", "Y"}) {
		t.Errorf("keys = %v, want catalog defaults [c y C Y]", got)
	}
	if h := b.Help(); h.Key != "c/y" {
		t.Errorf("help key = %q, want curated shortcut c/y", h.Key)
	} else if h.Desc != "copy" {
		t.Errorf("help desc = %q, want copy", h.Desc)
	}
}

// TestBinding_UnknownGroupActionYieldsEmpty: a group/action absent from
// the catalog must produce an empty, disabled binding rather than
// panicking. This also exercises LookupKeybinding's not-found path.
func TestBinding_UnknownGroupActionYieldsEmpty(t *testing.T) {
	t.Parallel()

	b := Binding(nil, "no_such_group", "no_such_action")
	if got := b.Keys(); len(got) != 0 {
		t.Errorf("unknown binding keys = %v, want none", got)
	}
	if b.Enabled() {
		t.Error("unknown binding should be disabled (no keys)")
	}
	if h := b.Help(); h.Key != "" || h.Desc != "" {
		t.Errorf("unknown binding help = %+v, want empty", h)
	}
}

// TestBinding_EmptyHelpLabelHasNoHelp: a catalog entry with HelpLabel ""
// (e.g. editor history navigation) must bind its keys but expose no help
// metadata, even when overridden.
func TestBinding_EmptyHelpLabelHasNoHelp(t *testing.T) {
	t.Parallel()

	// editor/history_prev has Defaults [up] and an empty HelpLabel.
	b := Binding(nil, config.KeybindingGroupEditor, config.KeybindActionEditorHistoryPrev)
	if got := b.Keys(); !slices.Equal(got, []string{"up"}) {
		t.Errorf("keys = %v, want [up]", got)
	}
	if h := b.Help(); h.Key != "" || h.Desc != "" {
		t.Errorf("no-help binding help = %+v, want empty", h)
	}

	// Same with an override: still no help, new keys.
	b = Binding(
		cfgWithKeybindings(map[string]map[string][]string{
			config.KeybindingGroupEditor: {config.KeybindActionEditorHistoryPrev: {"ctrl+up"}},
		}),
		config.KeybindingGroupEditor, config.KeybindActionEditorHistoryPrev,
	)
	if got := b.Keys(); !slices.Equal(got, []string{"ctrl+up"}) {
		t.Errorf("overridden no-help keys = %v, want [ctrl+up]", got)
	}
	if h := b.Help(); h.Key != "" || h.Desc != "" {
		t.Errorf("overridden no-help binding help = %+v, want empty", h)
	}
}

// TestBinding_OverrideHelpShortcutBecomesFirstKey: when the keys are
// overridden (differ from the defaults), the footer must show the new
// primary key in place of the curated shortcut; the label is preserved.
func TestBinding_OverrideHelpShortcutBecomesFirstKey(t *testing.T) {
	t.Parallel()

	b := Binding(
		cfgWithKeybindings(map[string]map[string][]string{
			config.KeybindingGroupGlobal: {config.KeybindActionModels: {"alt+m", "alt+l"}},
		}),
		config.KeybindingGroupGlobal, config.KeybindActionModels,
	)
	if got := b.Keys(); !slices.Equal(got, []string{"alt+m", "alt+l"}) {
		t.Errorf("override keys = %v, want [alt+m alt+l]", got)
	}
	h := b.Help()
	if h.Key != "alt+m" {
		t.Errorf("help key = %q, want first overridden key alt+m", h.Key)
	}
	if h.Desc != "models" {
		t.Errorf("help desc = %q, want preserved label models", h.Desc)
	}
}

// TestBinding_OverrideEqualToDefaultsKeepsCuratedShortcut: an override
// that happens to equal the defaults must NOT swap the curated shortcut
// for keys[0] (slices.Equal short-circuits the swap).
func TestBinding_OverrideEqualToDefaultsKeepsCuratedShortcut(t *testing.T) {
	t.Parallel()

	// Re-state the exact defaults [c, y, C, Y] as the override.
	b := Binding(
		cfgWithKeybindings(map[string]map[string][]string{
			config.KeybindingGroupChat: {config.KeybindActionChatCopy: {"c", "y", "C", "Y"}},
		}),
		config.KeybindingGroupChat, config.KeybindActionChatCopy,
	)
	if h := b.Help(); h.Key != "c/y" {
		t.Errorf("help key = %q, want curated c/y (override == defaults)", h.Key)
	}
}

// TestBinding_SingleDefaultOverridePicksNewKey covers the common single
// curated shortcut case (commands: default ctrl+p, shortcut ctrl+p).
func TestBinding_SingleDefaultOverridePicksNewKey(t *testing.T) {
	t.Parallel()

	b := Binding(
		cfgWithKeybindings(map[string]map[string][]string{
			config.KeybindingGroupGlobal: {config.KeybindActionCommands: {"ctrl+/"}},
		}),
		config.KeybindingGroupGlobal, config.KeybindActionCommands,
	)
	if got := b.Keys(); !slices.Equal(got, []string{"ctrl+/"}) {
		t.Errorf("override keys = %v, want [ctrl+/]", got)
	}
	if h := b.Help(); h.Key != "ctrl+/" {
		t.Errorf("help key = %q, want ctrl+/", h.Key)
	}
}
