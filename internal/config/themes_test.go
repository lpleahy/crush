package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromBytesThemes(t *testing.T) {
	t.Parallel()

	cfg, err := loadFromBytes([][]byte{[]byte(`{
		"options": {
			"tui": {
				"theme": "custom"
			}
		},
		"themes": {
			"custom": {
				"extends": "tokyonight-storm",
				"primary": "#112233",
				"bg_base": "#010203",
				"success_most_subtle": "#abcdef"
			}
		}
	}`)})

	require.NoError(t, err)
	require.Equal(t, "custom", cfg.Options.TUI.Theme)
	require.Equal(t, "tokyonight-storm", cfg.Themes["custom"].Extends)
	require.Equal(t, "#112233", cfg.Themes["custom"].Primary)
	require.Equal(t, "#010203", cfg.Themes["custom"].BgBase)
	require.Equal(t, "#abcdef", cfg.Themes["custom"].SuccessMostSubtle)
}

func TestLoadFromBytesThemesMergeByName(t *testing.T) {
	t.Parallel()

	cfg, err := loadFromBytes([][]byte{
		[]byte(`{"themes":{"custom":{"primary":"#112233","bg_base":"#010203"}}}`),
		[]byte(`{"themes":{"custom":{"primary":"#445566"}}}`),
	})

	require.NoError(t, err)
	require.Equal(t, "#445566", cfg.Themes["custom"].Primary)
	require.Equal(t, "#010203", cfg.Themes["custom"].BgBase)
}
