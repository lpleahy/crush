package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
)

func TestSchemaNoBrokenRefs(t *testing.T) {
	t.Parallel()

	reflector := new(jsonschema.Reflector)
	bts, err := json.Marshal(reflector.Reflect(&config.Config{}))
	require.NoError(t, err)

	var schema struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(bts, &schema))
	require.NotEmpty(t, schema.Defs, "schema should have definitions")

	for name := range schema.Defs {
		require.NotContains(t, name, "/", "schema $def key %q contains '/' which breaks JSON Pointer $ref resolution", name)
	}
}

func TestSchemaProvidersHasAdditionalProperties(t *testing.T) {
	t.Parallel()

	reflector := new(jsonschema.Reflector)
	bts, err := json.Marshal(reflector.Reflect(&config.Config{}))
	require.NoError(t, err)

	var schema struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(bts, &schema))

	var cfg struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(schema.Defs["Config"], &cfg))

	providersRaw, ok := cfg.Properties["providers"]
	require.True(t, ok, "Config should have a providers property")

	var providers struct {
		Type                 string          `json:"type"`
		AdditionalProperties json.RawMessage `json:"additionalProperties"`
	}
	require.NoError(t, json.Unmarshal(providersRaw, &providers))
	require.Equal(t, "object", providers.Type)
	require.True(t, strings.Contains(string(providers.AdditionalProperties), "ProviderConfig"),
		"providers should use additionalProperties with a ProviderConfig ref, got: %s", string(providers.AdditionalProperties))
}

// TestSchemaIncludesLocalFeatures guards that the knobs this fork adds on
// top of upstream stay documented in `crush schema`. The published
// (online) schema lags behind these, which is exactly why the local
// schema exists; if a config refactor silently drops one from the
// reflected output, editors would flag a valid key as unknown again.
func TestSchemaIncludesLocalFeatures(t *testing.T) {
	t.Parallel()

	reflector := new(jsonschema.Reflector)
	bts, err := json.Marshal(reflector.Reflect(&config.Config{}))
	require.NoError(t, err)
	s := string(bts)

	for _, prop := range []string{
		"$schema",      // self-reference so editors can wire it up
		"vim_mode",     // options.tui.vim_mode
		"cursor_blink", // options.tui.cursor_blink
		"compact_mode", // options.tui.compact_mode
		"transparent",  // options.tui.transparent
		"keybindings",  // options.tui.keybindings
		"themes",       // custom theme definitions
		"hooks",        // lifecycle hooks
	} {
		require.Containsf(t, s, prop, "crush schema must document the %q knob", prop)
	}
}

// TestSchemaGoldenFileUpToDate pins the committed crush.schema.json (the
// local schema users point their editor at) to the reflector output, so
// it can't drift when config.Config changes. Regenerate on failure with:
//
//	go run . schema > crush.schema.json
func TestSchemaGoldenFileUpToDate(t *testing.T) {
	t.Parallel()

	reflector := new(jsonschema.Reflector)
	bts, err := json.MarshalIndent(reflector.Reflect(&config.Config{}), "", "  ")
	require.NoError(t, err)
	want := string(bts) + "\n" // schema.go prints with fmt.Println

	got, err := os.ReadFile("../../crush.schema.json")
	require.NoError(t, err, "crush.schema.json missing; generate with: go run . schema > crush.schema.json")
	require.Equal(t, want, string(got),
		"crush.schema.json is stale; regenerate with: go run . schema > crush.schema.json")
}
