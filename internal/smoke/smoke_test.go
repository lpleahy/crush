// Package smoke holds integration smoke tests for the fully-integrated
// build: they exercise every feature's config + resolution together, so
// they only compile on the integration branch where all features are
// present. Per-feature depth lives in each feature's own unit tests; this
// is breadth — proof the features coexist and stay wired in one binary.
package smoke

import (
	"encoding/json"
	"fmt"
	"image/color"
	"slices"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/vim"
)

// kitchenSink configures every config-surfaced feature at once.
const kitchenSink = `{
  "options": {
    "tui": {
      "vim_mode": true,
      "theme": "midnight",
      "transparent": true,
      "keybindings": {
        "global": { "models": ["ctrl+m"] },
        "editor": { "newline": ["enter"] },
        "chat":   { "half_page_down": ["ctrl+d"] }
      }
    }
  },
  "themes": {
    "midnight": { "extends": "tokyonight-storm", "primary": "#7aa2f7", "bg_base": "#1a1b26" }
  },
  "hooks": {
    "PreToolUse": [ { "command": "echo pre" } ],
    "Stop": [ { "command": "echo stop" } ]
  }
}`

func parseKitchenSink(t *testing.T) *config.Config {
	t.Helper()
	var cfg config.Config
	if err := json.Unmarshal([]byte(kitchenSink), &cfg); err != nil {
		t.Fatalf("kitchen-sink config failed to parse: %v", err)
	}
	return &cfg
}

// TestSmoke_KitchenSinkConfigParses proves a single config exercising
// every feature unmarshals and populates each feature's fields.
func TestSmoke_KitchenSinkConfigParses(t *testing.T) {
	t.Parallel()
	cfg := parseKitchenSink(t)

	if cfg.Options == nil || cfg.Options.TUI == nil {
		t.Fatal("options.tui missing")
	}
	tui := cfg.Options.TUI
	if !tui.VimMode {
		t.Error("vim_mode not parsed")
	}
	if tui.Theme != "midnight" {
		t.Errorf("theme = %q, want midnight", tui.Theme)
	}
	if tui.Transparent == nil || !*tui.Transparent {
		t.Error("transparent not parsed")
	}
	if _, ok := cfg.Themes["midnight"]; !ok {
		t.Error("custom theme not parsed")
	}
	for group, action := range map[string]string{"global": "models", "editor": "newline", "chat": "half_page_down"} {
		if _, ok := tui.Keybindings[group][action]; !ok {
			t.Errorf("keybinding %s.%s not parsed", group, action)
		}
	}
	if len(cfg.Hooks["PreToolUse"]) == 0 || cfg.Hooks["PreToolUse"][0].Command != "echo pre" {
		t.Error("PreToolUse hook not parsed")
	}
	if len(cfg.Hooks["Stop"]) == 0 {
		t.Error("Stop hook not parsed")
	}
}

// TestSmoke_KeybindingsAllGroups proves overrides across multiple groups
// validate clean and resolve to the configured keys.
func TestSmoke_KeybindingsAllGroups(t *testing.T) {
	t.Parallel()
	cfg := parseKitchenSink(t)

	if n := cfg.ValidateKeybindings(); n != 0 {
		t.Errorf("ValidateKeybindings reported %d unknown entries, want 0", n)
	}
	cases := []struct {
		group, action, want string
	}{
		{config.KeybindingGroupGlobal, config.KeybindActionModels, "ctrl+m"},
		{config.KeybindingGroupEditor, "newline", "enter"},
		{config.KeybindingGroupChat, "half_page_down", "ctrl+d"},
	}
	for _, c := range cases {
		got := cfg.ResolveKeybinding(c.group, c.action, "DEFAULT")
		if !slices.Equal(got, []string{c.want}) {
			t.Errorf("ResolveKeybinding(%s,%s) = %v, want [%s]", c.group, c.action, got, c.want)
		}
	}
}

// TestSmoke_ThemesCustomAndTransparent proves built-in + custom themes
// resolve and transparency unsets the background.
func TestSmoke_ThemesCustomAndTransparent(t *testing.T) {
	t.Parallel()
	cfg := parseKitchenSink(t)

	// A built-in resolves with a real background.
	builtin := styles.Theme("tokyonight-storm", "", nil)
	if builtin.Background == nil {
		t.Error("built-in theme has nil background")
	}
	// The custom theme resolves and differs from the same name without the
	// custom definitions available (which falls back to a built-in/default).
	custom := styles.Theme("midnight", "openai", cfg.Themes)
	fallback := styles.Theme("midnight", "openai", nil)
	if colorKey(custom.Background) == colorKey(fallback.Background) {
		t.Error("custom theme background matches fallback — custom palette not applied")
	}
	// Transparency drops the background.
	if got := styles.Transparent(custom); got.Background != nil {
		t.Errorf("Transparent left a background: %v", got.Background)
	}
}

// TestSmoke_VimEngineWired proves the vim package is reachable and its
// host-routing/mode API behaves.
func TestSmoke_VimEngineWired(t *testing.T) {
	t.Parallel()
	e := vim.New()
	if e.Insert() {
		t.Error("vim engine should start in normal mode")
	}
	if !vim.ConsumesNormal("i") || !vim.ConsumesNormal("dd"[:1]) {
		t.Error("vim should consume printable normal-mode keys")
	}
	if vim.ConsumesNormal("enter") || vim.ConsumesNormal("ctrl+m") {
		t.Error("vim should let app chords pass through")
	}
}

// TestSmoke_AllHookEventsRecognized proves the lifecycle event set is
// wired with correct notification classification.
func TestSmoke_AllHookEventsRecognized(t *testing.T) {
	t.Parallel()
	notification := []string{
		hooks.EventAssistantMessage, hooks.EventStop,
	}
	for _, e := range notification {
		if !hooks.IsNotificationEvent(e) {
			t.Errorf("%s should be a notification event", e)
		}
	}
	if hooks.IsNotificationEvent(hooks.EventPreToolUse) {
		t.Error("PreToolUse must not be a notification event (it can block)")
	}
}

// TestSmoke_ChatGPTProviderRegistered proves the ChatGPT provider made it
// into catwalk's known set (the cross-repo integration point).
func TestSmoke_ChatGPTProviderRegistered(t *testing.T) {
	t.Parallel()
	if !slices.Contains(catwalk.KnownProviders(), catwalk.InferenceProviderChatGPT) {
		t.Error("ChatGPT provider not in catwalk.KnownProviders()")
	}
}

func colorKey(c color.Color) string {
	if c == nil {
		return "nil"
	}
	r, g, b, a := c.RGBA()
	return fmt.Sprintf("%04x%04x%04x%04x", r, g, b, a)
}
