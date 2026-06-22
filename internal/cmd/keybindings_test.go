package cmd

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestKeybindingCatalog_CoversGlobalActions guards the canonical config
// catalog (now the single source of truth shared by this command and the
// TUI keymap) against the global action list: every global action must
// appear exactly once and every catalog entry must have default keys.
func TestKeybindingCatalog_CoversGlobalActions(t *testing.T) {
	t.Parallel()

	seen := map[string]int{}
	for _, kb := range config.KeybindingCatalog {
		require.NotEmptyf(t, kb.Defaults, "%s/%s has no default keys", kb.Group, kb.Action)
		if kb.Group == config.KeybindingGroupGlobal {
			seen[kb.Action]++
		}
	}

	for _, action := range config.GlobalKeybindActions {
		require.Equalf(t, 1, seen[action], "global action %q should appear exactly once in the catalog", action)
	}
	require.Len(t, seen, len(config.GlobalKeybindActions),
		"catalog global group and config action list have diverged")
}

func TestKeybindingsCmd_Registered(t *testing.T) {
	t.Parallel()
	require.Equal(t, "keybindings", keybindingsCmd.Use)
	require.Contains(t, keybindingsCmd.Aliases, "keys")
}
