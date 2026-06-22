package model

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/completions"
)

// modelSideKeybindingGroups are the catalog groups whose catalog<->UI
// integrity is cross-checked here (from model.KeyMap and the completions
// KeyMap). Every other group is owned by the dialog drift suite
// (internal/ui/dialog). The two suites together cover the whole catalog; the
// no-unowned-group guards in each fail if a new group is claimed by neither.
var modelSideKeybindingGroups = map[string]bool{
	config.KeybindingGroupGlobal:      true,
	config.KeybindingGroupEditor:      true,
	config.KeybindingGroupChat:        true,
	config.KeybindingGroupInitialize:  true,
	config.KeybindingGroupCompletions: true,
}

const kbSentinelPrefix = "__kbtest__"

// sentinelConfig overrides every catalog entry to a single unique key
// "__kbtest__<group>.<action>", so a binding built via common.Binding can be
// mapped back to the exact catalog entry it was sourced from by its keys.
func sentinelConfig() *config.Config {
	kb := map[string]map[string][]string{}
	for _, d := range config.KeybindingCatalog {
		if kb[d.Group] == nil {
			kb[d.Group] = map[string][]string{}
		}
		kb[d.Group][d.Action] = []string{kbSentinelPrefix + d.Group + "." + d.Action}
	}
	return &config.Config{
		Options: &config.Options{TUI: &config.TUIOptions{Keybindings: kb}},
	}
}

// walkBindings visits every key.Binding in a (possibly nested) keymap struct,
// calling fn with a dotted path and the binding.
func walkBindings(v reflect.Value, path string, fn func(path string, b key.Binding)) {
	bindingType := reflect.TypeOf(key.Binding{})
	if v.Type() == bindingType {
		fn(path, v.Interface().(key.Binding))
		return
	}
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			walkBindings(v.Field(i), path+"."+v.Type().Field(i).Name, fn)
		}
	}
}

// TestModelKeybindings_BidirectionalDrift is the model/completions half of the
// catalog<->UI integrity guarantee (the dialog half lives in
// internal/ui/dialog). For every group it owns it asserts:
//
//   - no unsourced field: each binding in model.KeyMap and completions.KeyMap
//     resolves to exactly one catalog entry (the sentinel round-trip), so a
//     field built from the wrong/no group/action is caught;
//   - no orphan: every catalog entry in an owned group is consumed by at least
//     one field, so a typo'd or removed-from-UI catalog action fails instead of
//     passing silently.
func TestModelKeybindings_BidirectionalDrift(t *testing.T) {
	t.Parallel()

	cfg := sentinelConfig()
	consumed := map[string]map[string]int{}
	record := func(group, action string) {
		if consumed[group] == nil {
			consumed[group] = map[string]int{}
		}
		consumed[group][action]++
	}

	visit := func(path string, b key.Binding) {
		keys := b.Keys()
		if len(keys) != 1 || !strings.HasPrefix(keys[0], kbSentinelPrefix) {
			t.Errorf("%s did not resolve to a single catalog-sourced key (keys=%v); "+
				"is it built via common.Binding from one catalog entry?", path, keys)
			return
		}
		ga := strings.TrimPrefix(keys[0], kbSentinelPrefix)
		dot := strings.IndexByte(ga, '.')
		if dot < 0 {
			t.Errorf("%s sentinel %q is malformed", path, keys[0])
			return
		}
		record(ga[:dot], ga[dot+1:])
	}

	km := DefaultKeyMap(cfg)
	walkBindings(reflect.ValueOf(km), "KeyMap", visit)

	ckm := completions.DefaultKeyMap(cfg)
	walkBindings(reflect.ValueOf(ckm), "completions.KeyMap", visit)

	// Orphan check for the owned groups.
	for _, d := range config.KeybindingCatalog {
		if !modelSideKeybindingGroups[d.Group] {
			continue
		}
		if consumed[d.Group][d.Action] == 0 {
			t.Errorf("orphan catalog entry %s.%s: no model/completions keymap field "+
				"sources it (removed from the UI, or a typo?)", d.Group, d.Action)
		}
	}

	// Every group these keymaps consume must be one this suite owns; a
	// stray group here would mean a dialog group leaked into the model keymap.
	for group := range consumed {
		if !modelSideKeybindingGroups[group] {
			t.Errorf("model/completions keymaps consume group %q not owned by the "+
				"model-side drift suite; reconcile the suite ownership", group)
		}
	}
}
