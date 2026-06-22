package completions

import (
	"testing"

	"charm.land/bubbles/v2/key"
)

// TestDefaultKeyMap_AllNonEmpty guards the completions call sites: every
// binding must resolve to at least one key. An empty one would mean a
// mistyped catalog group/action in completions/keys.go.
func TestDefaultKeyMap_AllNonEmpty(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap(nil)
	for name, b := range map[string]key.Binding{
		"Down":       km.Down,
		"Up":         km.Up,
		"Select":     km.Select,
		"Cancel":     km.Cancel,
		"DownInsert": km.DownInsert,
		"UpInsert":   km.UpInsert,
	} {
		if len(b.Keys()) == 0 {
			t.Errorf("completions binding %q has no keys", name)
		}
	}
}
