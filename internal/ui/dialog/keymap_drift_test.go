package dialog

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
)

// modelSideKeybindingGroups are the catalog groups whose consumption is
// cross-checked from the model package (model.KeyMap and the completions
// KeyMap) rather than here. The dialog drift test owns every other group.
// Together the two suites cover the whole catalog: TestNoUnownedKeybindingGroups
// in each package fails if a new group is claimed by neither.
var modelSideKeybindingGroups = map[string]bool{
	config.KeybindingGroupGlobal:      true,
	config.KeybindingGroupEditor:      true,
	config.KeybindingGroupChat:        true,
	config.KeybindingGroupInitialize:  true,
	config.KeybindingGroupCompletions: true,
}

const kbSentinelPrefix = "__kbtest__"

// sentinelKeybindings overrides every catalog entry to a single unique key
// "__kbtest__<group>.<action>", so a binding built via common.Binding can be
// mapped back to the exact catalog entry it was sourced from by reading its
// keys. (Help labels are irrelevant here; only the keys carry the marker.)
func sentinelKeybindings() map[string]map[string][]string {
	kb := map[string]map[string][]string{}
	for _, d := range config.KeybindingCatalog {
		if kb[d.Group] == nil {
			kb[d.Group] = map[string][]string{}
		}
		kb[d.Group][d.Action] = []string{kbSentinelPrefix + d.Group + "." + d.Action}
	}
	return kb
}

// sentinelCommon returns a *common.Common backed by a workspace that serves a
// config whose keybindings are the sentinel overrides, so constructed dialogs
// build their keymaps from recognizable keys.
func sentinelCommon() *common.Common {
	s := styles.CharmtonePantera()
	cfg := &config.Config{
		Options: &config.Options{
			TUI: &config.TUIOptions{Keybindings: sentinelKeybindings()},
		},
	}
	return &common.Common{
		Workspace: stubWorkspace{cfg: cfg},
		Styles:    &s,
	}
}

// stubWorkspace embeds the full Workspace interface (so it satisfies the type)
// but only implements the handful of methods the dialog constructors touch
// while wiring keymaps: Config (for the overrides), ListSessions (NewSessions),
// and WorkingDir (NewFilePicker). Any other method is a programming error in
// this test and will nil-panic, which is what we want.
type stubWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w stubWorkspace) Config() *config.Config { return w.cfg }

func (w stubWorkspace) ListSessions(context.Context) ([]session.Session, error) {
	return nil, nil
}

func (w stubWorkspace) WorkingDir() string { return "." }

// consumedPair decodes the catalog (group, action) a sentinel binding was
// sourced from. It requires the binding to carry exactly the one sentinel key
// (no more, no less); anything else means the field was not built from a
// single catalog entry via common.Binding.
func consumedPair(t *testing.T, name string, b key.Binding) (group, action string, ok bool) {
	t.Helper()
	keys := b.Keys()
	if len(keys) != 1 || !strings.HasPrefix(keys[0], kbSentinelPrefix) {
		t.Errorf("%s did not resolve to a single catalog-sourced key (keys=%v); "+
			"is it built via common.Binding from one catalog entry?", name, keys)
		return "", "", false
	}
	ga := strings.TrimPrefix(keys[0], kbSentinelPrefix)
	dot := strings.IndexByte(ga, '.')
	if dot < 0 {
		t.Errorf("%s sentinel %q is malformed", name, keys[0])
		return "", "", false
	}
	return ga[:dot], ga[dot+1:], true
}

// fields walks every key.Binding field of a keymap struct (by reflection) and
// returns the count, so a test that lists fields explicitly can assert it
// hasn't missed a newly added one.
func bindingFieldCount(v any) int {
	rv := reflect.Indirect(reflect.ValueOf(v))
	bt := reflect.TypeOf(key.Binding{})
	n := 0
	for i := 0; i < rv.NumField(); i++ {
		if rv.Type().Field(i).Type == bt {
			n++
		}
	}
	return n
}

// TestDialogKeybindings_BidirectionalDrift is the dialog/completions half of
// the catalog<->UI integrity guarantee. It asserts, for every group it owns:
//
//   - no unsourced field: each keymap binding resolves to exactly one catalog
//     entry (the sentinel round-trip), so a field built from the wrong/no
//     group/action is caught;
//   - no unlisted field: the explicit field list per keymap matches the
//     struct's reflective key.Binding field count, so a newly added field
//     can't dodge the cross-check;
//   - no orphan: every catalog entry in an owned group is consumed by at least
//     one field, so a typo'd or removed-from-UI catalog action fails the test
//     instead of passing silently.
func TestDialogKeybindings_BidirectionalDrift(t *testing.T) {
	t.Parallel()

	com := sentinelCommon()

	// consumed[group][action] = number of fields that source it.
	consumed := map[string]map[string]int{}
	record := func(group, action string) {
		if consumed[group] == nil {
			consumed[group] = map[string]int{}
		}
		consumed[group][action]++
	}

	// collect lists a named set of keymap fields, asserts each is
	// catalog-sourced, records the consumed pairs, and checks that the
	// explicit field list is exhaustive for the owning struct.
	collect := func(structName string, structVal any, fields map[string]key.Binding) {
		for name, b := range fields {
			if g, a, ok := consumedPair(t, structName+"."+name, b); ok {
				record(g, a)
			}
		}
		if got := bindingFieldCount(structVal); got != len(fields) {
			t.Errorf("%s has %d key.Binding fields but the drift test lists %d; "+
				"add the new field(s) to the cross-check", structName, got, len(fields))
		}
	}

	// permissions: keymap built by a standalone func (no construction needed).
	pkm := defaultPermissionsKeyMap(com.Config())
	collect("permissions", pkm, map[string]key.Binding{
		"Left": pkm.Left, "Right": pkm.Right, "Tab": pkm.Tab, "Select": pkm.Select,
		"Allow": pkm.Allow, "AllowSession": pkm.AllowSession, "Deny": pkm.Deny,
		"Close": pkm.Close, "ToggleDiffMode": pkm.ToggleDiffMode,
		"ToggleFullscreen": pkm.ToggleFullscreen, "ScrollUp": pkm.ScrollUp,
		"ScrollDown": pkm.ScrollDown, "ScrollLeft": pkm.ScrollLeft,
		"ScrollRight": pkm.ScrollRight, "Choose": pkm.Choose, "Scroll": pkm.Scroll,
	})

	// models (keymap built by a standalone func; NewModels itself fetches
	// providers and isn't needed to exercise the keymap wiring).
	mkm := defaultModelsKeyMap(com)
	collect("models", mkm, map[string]key.Binding{
		"Tab": mkm.Tab, "UpDown": mkm.UpDown, "Select": mkm.Select,
		"Edit": mkm.Edit, "Next": mkm.Next, "Previous": mkm.Previous,
		"Close": mkm.Close,
	})

	// sessions
	if s, err := NewSessions(com, ""); err != nil {
		t.Fatalf("NewSessions: %v", err)
	} else {
		collect("sessions", s.keyMap, map[string]key.Binding{
			"Select": s.keyMap.Select, "Next": s.keyMap.Next, "Previous": s.keyMap.Previous,
			"UpDown": s.keyMap.UpDown, "Delete": s.keyMap.Delete, "Rename": s.keyMap.Rename,
			"ConfirmRename": s.keyMap.ConfirmRename, "CancelRename": s.keyMap.CancelRename,
			"ConfirmDelete": s.keyMap.ConfirmDelete, "CancelDelete": s.keyMap.CancelDelete,
			"Close": s.keyMap.Close,
		})
	}

	// commands
	if c, err := NewCommands(com, "", false, false, false, nil, nil); err != nil {
		t.Fatalf("NewCommands: %v", err)
	} else {
		collect("commands", c.keyMap, map[string]key.Binding{
			"Select": c.keyMap.Select, "UpDown": c.keyMap.UpDown, "Next": c.keyMap.Next,
			"Previous": c.keyMap.Previous, "Tab": c.keyMap.Tab, "ShiftTab": c.keyMap.ShiftTab,
			"Close": c.keyMap.Close,
		})
	}

	// filepicker
	if fp, _ := NewFilePicker(com); fp == nil {
		t.Fatal("NewFilePicker returned nil")
	} else {
		collect("filepicker", fp.km, map[string]key.Binding{
			"Select": fp.km.Select, "Down": fp.km.Down, "Up": fp.km.Up,
			"Forward": fp.km.Forward, "Backward": fp.km.Backward,
			"Navigate": fp.km.Navigate, "Close": fp.km.Close,
		})
	}

	// arguments
	if a := NewArguments(com, "", "", nil, nil); a == nil {
		t.Fatal("NewArguments returned nil")
	} else {
		collect("arguments", a.keyMap, map[string]key.Binding{
			"Confirm": a.keyMap.Confirm, "Next": a.keyMap.Next,
			"Previous": a.keyMap.Previous, "Close": a.keyMap.Close,
		})
	}

	// reasoning (keymap built by a standalone func; NewReasoning needs a full
	// agent/model config to populate items, which is irrelevant to the keys).
	rkm := defaultReasoningKeyMap(com)
	collect("reasoning", rkm, map[string]key.Binding{
		"Select": rkm.Select, "Next": rkm.Next,
		"Previous": rkm.Previous, "UpDown": rkm.UpDown, "Close": rkm.Close,
	})

	// notifications
	n := NewNotifications(com)
	collect("notifications", n.keyMap, map[string]key.Binding{
		"Select": n.keyMap.Select, "Next": n.keyMap.Next,
		"Previous": n.keyMap.Previous, "UpDown": n.keyMap.UpDown, "Close": n.keyMap.Close,
	})

	// quit
	q := NewQuit(com)
	collect("quit", q.keyMap, map[string]key.Binding{
		"LeftRight": q.keyMap.LeftRight, "EnterSpace": q.keyMap.EnterSpace,
		"Yes": q.keyMap.Yes, "No": q.keyMap.No, "Tab": q.keyMap.Tab,
		"Quit": q.keyMap.Quit, "Close": q.keyMap.Close,
	})

	// oauth (construction only wires the keymap; the device-flow call is an
	// Init cmd, not run here).
	if o, _ := NewOAuthCopilot(com, false, catwalk.Provider{}, config.SelectedModel{}, config.SelectedModelTypeLarge); o == nil {
		t.Fatal("NewOAuthCopilot returned nil")
	} else {
		collect("oauth", o.keyMap, map[string]key.Binding{
			"Copy": o.keyMap.Copy, "Submit": o.keyMap.Submit, "Close": o.keyMap.Close,
		})
	}

	// api_key
	if ak, _ := NewAPIKeyInput(com, false, catwalk.Provider{}, config.SelectedModel{}, config.SelectedModelTypeLarge); ak == nil {
		t.Fatal("NewAPIKeyInput returned nil")
	} else {
		collect("api_key", ak.keyMap, map[string]key.Binding{
			"Submit": ak.keyMap.Submit, "Close": ak.keyMap.Close,
		})
	}

	// Orphan check: every catalog entry in a dialog-owned group must be
	// consumed by at least one field above.
	for _, d := range config.KeybindingCatalog {
		if modelSideKeybindingGroups[d.Group] {
			continue
		}
		if consumed[d.Group][d.Action] == 0 {
			t.Errorf("orphan catalog entry %s.%s: no dialog/completions keymap field "+
				"sources it (removed from the UI, or a typo?)", d.Group, d.Action)
		}
	}
}

// TestNoUnownedKeybindingGroups makes the two-suite partition airtight: every
// catalog group is either owned by the model-side suite or covered by the
// dialog drift test above. A brand-new group claimed by neither fails here.
func TestNoUnownedKeybindingGroups(t *testing.T) {
	t.Parallel()

	com := sentinelCommon()
	consumed := map[string]bool{}

	// Re-derive which groups the dialog suite actually consumes by collecting
	// the groups of every constructed dialog's bindings. (Cheap subset: any
	// non-model-side catalog group must appear among consumed dialog groups,
	// which TestDialogKeybindings_BidirectionalDrift already verifies via the
	// orphan check; here we only assert no group falls outside both suites.)
	pkm := defaultPermissionsKeyMap(com.Config())
	for _, b := range []key.Binding{pkm.Left, pkm.Close} {
		if g, _, ok := consumedPair(t, "permissions", b); ok {
			consumed[g] = true
		}
	}

	for _, d := range config.KeybindingCatalog {
		owned := modelSideKeybindingGroups[d.Group]
		if owned {
			continue
		}
		// Non-model-side groups are owned by the dialog suite; assert the
		// group name is a recognized dialog/completions group constant so a
		// stray group can't silently slip in unchecked.
		if !knownDialogGroup(d.Group) {
			t.Errorf("catalog group %q is owned by neither the model-side suite "+
				"(global/editor/chat/initialize/completions) nor a known dialog group; "+
				"add it to one of the drift suites", d.Group)
		}
	}
}

// knownDialogGroup reports whether g is one of the dialog/completions groups
// the dialog drift suite cross-checks.
func knownDialogGroup(g string) bool {
	switch g {
	case config.KeybindingGroupDialog,
		config.KeybindingGroupModels,
		config.KeybindingGroupSessions,
		config.KeybindingGroupCommands,
		config.KeybindingGroupFilePicker,
		config.KeybindingGroupArguments,
		config.KeybindingGroupPermissions,
		config.KeybindingGroupReasoning,
		config.KeybindingGroupNotifications,
		config.KeybindingGroupQuit,
		config.KeybindingGroupOAuth,
		config.KeybindingGroupAPIKey:
		return true
	}
	return false
}
