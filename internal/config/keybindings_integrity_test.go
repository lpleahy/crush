package config

import "testing"

// TestKeybindingCatalog_NoDuplicatesAndWellFormed guards the catalog
// itself: every entry must have a non-empty group, action, and default
// keys, and no (group, action) pair may appear twice. A duplicate would
// make LookupKeybinding return whichever entry comes first, silently
// shadowing the other.
func TestKeybindingCatalog_NoDuplicatesAndWellFormed(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, d := range KeybindingCatalog {
		if d.Group == "" || d.Action == "" {
			t.Errorf("catalog entry with empty group/action: %+v", d)
			continue
		}
		if len(d.Defaults) == 0 {
			t.Errorf("%s/%s has no default keys", d.Group, d.Action)
		}
		k := d.Group + "." + d.Action
		if seen[k] {
			t.Errorf("duplicate catalog entry: %s", k)
		}
		seen[k] = true
	}
}

// TestLookupKeybinding_RoundTrips verifies every catalog entry is
// resolvable by LookupKeybinding (i.e. the lookup the UI relies on finds
// every declared binding).
func TestLookupKeybinding_RoundTrips(t *testing.T) {
	t.Parallel()

	for _, d := range KeybindingCatalog {
		got, ok := LookupKeybinding(d.Group, d.Action)
		if !ok {
			t.Errorf("LookupKeybinding(%q, %q) not found", d.Group, d.Action)
			continue
		}
		if got.Action != d.Action || got.Group != d.Group {
			t.Errorf("LookupKeybinding(%q, %q) returned %q/%q", d.Group, d.Action, got.Group, got.Action)
		}
	}
}
