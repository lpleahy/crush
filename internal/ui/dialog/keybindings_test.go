package dialog

import (
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

func dialogTestCommon() *common.Common {
	s := styles.CharmtonePantera()
	return &common.Common{Styles: &s}
}

// TestDialogKeymaps_NonEmpty guards the Style-B dialog call sites: every
// binding built via common.Binding must resolve to real keys, so a
// mistyped catalog group/action surfaces as a test failure instead of a
// silently dead key. Constructors that need a fully wired app are
// best-effort (recover-guarded); whichever build in a bare test
// environment get verified. (permissions is covered by permissions_test.go.)
func TestDialogKeymaps_NonEmpty(t *testing.T) {
	t.Parallel()

	com := dialogTestCommon()
	check := func(name string, b key.Binding) {
		if len(b.Keys()) == 0 {
			t.Errorf("%s resolved to no keys", name)
		}
	}

	// The shared close binding underpins most dialogs; verify it and its
	// nil-safe path (a config-less Common must still yield the defaults).
	check("dialog.close", closeBinding(com))
	check("dialog.close(nil)", closeBinding(nil))

	// Always-constructible dialogs.
	q := NewQuit(com)
	check("quit.LeftRight", q.keyMap.LeftRight)
	check("quit.EnterSpace", q.keyMap.EnterSpace)
	check("quit.Yes", q.keyMap.Yes)
	check("quit.No", q.keyMap.No)
	check("quit.Tab", q.keyMap.Tab)
	check("quit.Quit", q.keyMap.Quit)
	check("quit.Close", q.keyMap.Close)

	n := NewNotifications(com)
	check("notifications.Select", n.keyMap.Select)
	check("notifications.Next", n.keyMap.Next)
	check("notifications.Previous", n.keyMap.Previous)
	check("notifications.UpDown", n.keyMap.UpDown)
	check("notifications.Close", n.keyMap.Close)

	// Best-effort for dialogs whose constructors may need more app wiring.
	safe := func(fn func()) {
		defer func() { _ = recover() }()
		fn()
	}
	safe(func() {
		if m, err := NewModels(com, false); err == nil && m != nil {
			check("models.Tab", m.keyMap.Tab)
			check("models.Select", m.keyMap.Select)
			check("models.Close", m.keyMap.Close)
		}
	})
	safe(func() {
		if s, err := NewSessions(com, ""); err == nil && s != nil {
			check("sessions.Select", s.keyMap.Select)
			check("sessions.Delete", s.keyMap.Delete)
			check("sessions.Close", s.keyMap.Close)
		}
	})
	safe(func() {
		if r, err := NewReasoning(com); err == nil && r != nil {
			check("reasoning.Select", r.keyMap.Select)
			check("reasoning.UpDown", r.keyMap.UpDown)
			check("reasoning.Close", r.keyMap.Close)
		}
	})
	safe(func() {
		if fp, _ := NewFilePicker(com); fp != nil {
			check("filepicker.Select", fp.km.Select)
			check("filepicker.Navigate", fp.km.Navigate)
			check("filepicker.Close", fp.km.Close)
		}
	})
	safe(func() {
		if c, err := NewCommands(com, "", false, false, false, nil, nil); err == nil && c != nil {
			check("commands.Select", c.keyMap.Select)
			check("commands.Tab", c.keyMap.Tab)
			check("commands.ShiftTab", c.keyMap.ShiftTab)
		}
	})
	safe(func() {
		if a := NewArguments(com, "", "", nil, nil); a != nil {
			check("arguments.Confirm", a.keyMap.Confirm)
			check("arguments.Next", a.keyMap.Next)
			check("arguments.Close", a.keyMap.Close)
		}
	})
}
