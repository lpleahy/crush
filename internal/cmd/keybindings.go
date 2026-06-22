package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/spf13/cobra"
)

var keybindingsCmd = &cobra.Command{
	Use:   "keybindings",
	Short: "List overridable keybindings and their current keys",
	Long: `List the keybindings you can override from crush.json under
options.tui.keybindings, showing each action's default keys and the
effective keys after any override in your config.

Every group is overridable, not just global. Bindings are grouped by
area: global (app-level chords), editor (the message composer), chat
(the messages list), initialize (the onboarding prompt), and the overlay
dialogs and completions (models, sessions, commands, filepicker,
arguments, permissions, reasoning, notifications, quit, oauth, api_key,
dialog, completions). Run this command to see the full list.

Override example:

  {
    "options": {
      "tui": {
        "keybindings": {
          "global": {
            "models": ["ctrl+m"],
            "commands": ["ctrl+/", "ctrl+p"]
          },
          "chat": {
            "half_page_down": ["ctrl+d"]
          }
        }
      }
    }
  }

Keys use bubbletea spelling: "ctrl+l", "shift+tab", "alt+m", "enter".`,
	Aliases: []string{"keys", "keymap"},
	Example: `
# List overridable keybindings and effective keys
crush keybindings
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		dataDir, _ := cmd.Flags().GetString("data-dir")

		// Best-effort load so we can show effective (post-override)
		// keys. If config can't load, fall back to defaults only.
		var cfg *config.Config
		if store, err := config.Load(cwd, dataDir, false); err == nil {
			cfg = store.Config()
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "GROUP\tACTION\tDEFAULT\tEFFECTIVE\tDESCRIPTION")
		for _, kb := range config.KeybindingCatalog {
			effective := kb.Defaults
			if cfg != nil {
				effective = cfg.ResolveKeybinding(kb.Group, kb.Action, kb.Defaults...)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				kb.Group, kb.Action,
				strings.Join(kb.Defaults, ", "),
				strings.Join(effective, ", "),
				kb.Description,
			)
		}
		return w.Flush()
	},
}
