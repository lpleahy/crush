package config

import (
	"slices"
	"testing"
)

func cfgWithKeybindings(kb map[string]map[string][]string) *Config {
	return &Config{
		Options: &Options{
			TUI: &TUIOptions{Keybindings: kb},
		},
	}
}

func TestResolveKeybinding_Default(t *testing.T) {
	c := cfgWithKeybindings(nil)
	got := c.ResolveKeybinding(KeybindingGroupGlobal, KeybindActionModels, "ctrl+m", "ctrl+l")
	want := []string{"ctrl+m", "ctrl+l"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want defaults %v", got, want)
	}
}

func TestResolveKeybinding_Override(t *testing.T) {
	c := cfgWithKeybindings(map[string]map[string][]string{
		KeybindingGroupGlobal: {KeybindActionModels: {"ctrl+m"}},
	})
	got := c.ResolveKeybinding(KeybindingGroupGlobal, KeybindActionModels, "ctrl+m", "ctrl+l")
	if !slices.Equal(got, []string{"ctrl+m"}) {
		t.Errorf("override not applied: got %v", got)
	}
}

func TestResolveKeybinding_EmptyOverrideFallsBack(t *testing.T) {
	c := cfgWithKeybindings(map[string]map[string][]string{
		KeybindingGroupGlobal: {KeybindActionModels: {}},
	})
	got := c.ResolveKeybinding(KeybindingGroupGlobal, KeybindActionModels, "ctrl+m", "ctrl+l")
	if !slices.Equal(got, []string{"ctrl+m", "ctrl+l"}) {
		t.Errorf("empty override should fall back to defaults, got %v", got)
	}
}

func TestResolveKeybinding_NilConfig(t *testing.T) {
	var c *Config
	got := c.ResolveKeybinding(KeybindingGroupGlobal, KeybindActionQuit, "ctrl+c")
	if !slices.Equal(got, []string{"ctrl+c"}) {
		t.Errorf("nil config should return defaults, got %v", got)
	}
}

func TestResolveKeybinding_UnknownGroupOrAction(t *testing.T) {
	c := cfgWithKeybindings(map[string]map[string][]string{
		"nope": {"whatever": {"x"}},
	})
	got := c.ResolveKeybinding(KeybindingGroupGlobal, KeybindActionModels, "ctrl+l")
	if !slices.Equal(got, []string{"ctrl+l"}) {
		t.Errorf("unknown group must not affect resolution, got %v", got)
	}
}

func TestValidateKeybindings(t *testing.T) {
	t.Run("all known is zero", func(t *testing.T) {
		c := cfgWithKeybindings(map[string]map[string][]string{
			KeybindingGroupGlobal: {
				KeybindActionModels:   {"ctrl+m"},
				KeybindActionCommands: {"ctrl+p"},
			},
		})
		if n := c.ValidateKeybindings(); n != 0 {
			t.Errorf("expected 0 unknown, got %d", n)
		}
	})

	t.Run("unknown action counted", func(t *testing.T) {
		c := cfgWithKeybindings(map[string]map[string][]string{
			KeybindingGroupGlobal: {"bogus_action": {"x"}},
		})
		if n := c.ValidateKeybindings(); n != 1 {
			t.Errorf("expected 1 unknown action, got %d", n)
		}
	})

	t.Run("unknown group counts its actions", func(t *testing.T) {
		c := cfgWithKeybindings(map[string]map[string][]string{
			"bogus_group": {"a": {"x"}, "b": {"y"}},
		})
		if n := c.ValidateKeybindings(); n != 2 {
			t.Errorf("expected 2 unknown (whole group), got %d", n)
		}
	})

	t.Run("nil is zero", func(t *testing.T) {
		c := cfgWithKeybindings(nil)
		if n := c.ValidateKeybindings(); n != 0 {
			t.Errorf("expected 0, got %d", n)
		}
	})

	// The nil-guard short-circuits (nil Config, nil Options, nil TUI)
	// must each return 0 without dereferencing.
	t.Run("nil receiver is zero", func(t *testing.T) {
		var c *Config
		if n := c.ValidateKeybindings(); n != 0 {
			t.Errorf("nil *Config: expected 0, got %d", n)
		}
	})

	t.Run("nil Options is zero", func(t *testing.T) {
		c := &Config{}
		if n := c.ValidateKeybindings(); n != 0 {
			t.Errorf("nil Options: expected 0, got %d", n)
		}
	})

	t.Run("nil TUI is zero", func(t *testing.T) {
		c := &Config{Options: &Options{}}
		if n := c.ValidateKeybindings(); n != 0 {
			t.Errorf("nil TUI: expected 0, got %d", n)
		}
	})

	t.Run("mixed known and unknown counts only unknown", func(t *testing.T) {
		c := cfgWithKeybindings(map[string]map[string][]string{
			KeybindingGroupGlobal: {
				KeybindActionModels: {"ctrl+m"}, // known
				"ghost":             {"x"},      // unknown action
			},
		})
		if n := c.ValidateKeybindings(); n != 1 {
			t.Errorf("expected 1 unknown action among knowns, got %d", n)
		}
	})
}

// TestLookupKeybinding_NotFound covers the descriptor-not-found path:
// an unknown group/action returns the zero descriptor and false.
func TestLookupKeybinding_NotFound(t *testing.T) {
	t.Parallel()

	d, ok := LookupKeybinding("no_such_group", "no_such_action")
	if ok {
		t.Errorf("expected not-found, got ok=true (%+v)", d)
	}
	if d.Group != "" || d.Action != "" || len(d.Defaults) != 0 {
		t.Errorf("expected zero descriptor, got %+v", d)
	}
}
