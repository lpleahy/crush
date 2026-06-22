# Keybindings

Crush's keybindings — every TUI chord, from the app-level model picker and
command palette to the editor, chat, and dialog keys — can be overridden from
`crush.json`. This is useful when a default collides with your terminal or
multiplexer (a common one: `ctrl+l`, which tmux's `vim-tmux-navigator` and many
terminals bind to "clear" or "pane navigation", shadowing Crush's model picker).

## Quick start

See what's overridable and what's currently bound:

```bash
crush keybindings
```

Override a binding in `crush.json` (global `~/.config/crush/crush.json` or a
project-level `crush.json`):

```json
{
  "options": {
    "tui": {
      "keybindings": {
        "global": {
          "models": ["ctrl+m"],
          "commands": ["ctrl+/", "ctrl+p"]
        }
      }
    }
  }
}
```

That rebinds the model picker to just `ctrl+m` (dropping the conflicting
`ctrl+l`) and gives the command palette two keys. Run `crush keybindings` again
to confirm the **EFFECTIVE** column reflects your changes.

## Shape

```
options.tui.keybindings.<group>.<action> = ["key", "key", ...]
```

- An action maps to a **list** of keys; any of them triggers it.
- A list with multiple keys binds them all (e.g. `["ctrl+m", "ctrl+l"]`).
- An **empty** list (`[]`) falls back to the default — handy for documenting
  intent without changing behavior.
- Unknown groups or actions are ignored with a warning in the log (so a typo is
  visible but never blocks startup).

## Groups and actions

**Every** group is overridable, not just `global`. Each keymap in the TUI is
built from the same catalog (`config.KeybindingCatalog`), so any
`<group>.<action>` below can be remapped from `crush.json`. The `global` group
holds the application-level chords most likely to collide with your
terminal/tmux, but the editor, chat, dialog, and onboarding groups respond to
overrides exactly the same way.

`crush keybindings` is the source of truth — it prints these groups with your
effective keys merged in. The tables below are a snapshot of the defaults,
generated from the same catalog; run `crush keybindings` for the live view.

### `global`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `quit` | `ctrl+c` | Quit crush |
| `help` | `ctrl+g` | Show more help |
| `commands` | `ctrl+p` | Open the command palette |
| `models` | `ctrl+m`, `ctrl+l` | Open the model picker |
| `suspend` | `ctrl+z` | Suspend crush |
| `sessions` | `ctrl+s` | Open the sessions list |
| `tab` | `tab` | Change focus |
| `toggle_yolo` | `ctrl+y` | Toggle yolo (auto-approve) mode |

### `editor`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `add_file` | `/` | Add a file to the prompt |
| `send_message` | `enter` | Send the message |
| `open_editor` | `ctrl+o` | Open the message in $EDITOR |
| `newline` | `shift+enter`, `ctrl+j` | Insert a newline |
| `add_image` | `ctrl+f` | Add an image to the prompt |
| `paste_image` | `ctrl+v` | Paste an image from the clipboard |
| `mention_file` | `@` | Mention a file with @ |
| `commands` | `/` | Open commands from an empty composer |
| `attachment_delete_mode` | `ctrl+r` | Enter attachment delete mode |
| `escape` | `esc`, `alt+esc` | Cancel attachment delete mode |
| `delete_all_attachments` | `r` | Delete all attachments |
| `history_prev` | `up` | Recall the previous prompt from history |
| `history_next` | `down` | Recall the next prompt from history |

### `chat`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `new_session` | `ctrl+n` | Start a new session |
| `add_attachment` | `ctrl+f` | Add an attachment |
| `cancel` | `esc`, `alt+esc` | Cancel the running task |
| `tab` | `tab` | Change focus |
| `details` | `ctrl+d` | Toggle message details |
| `toggle_pills` | `ctrl+t`, `ctrl+space` | Toggle the tasks panel |
| `pill_left` | `left` | Switch section left |
| `pill_right` | `right` | Switch section right |
| `down` | `down`, `ctrl+j`, `j` | Scroll down |
| `up` | `up`, `ctrl+k`, `k` | Scroll up |
| `up_down` | `up`, `down` | Scroll up/down |
| `up_one_item` | `shift+up`, `K` | Move up one item |
| `down_one_item` | `shift+down`, `J` | Move down one item |
| `up_down_one_item` | `shift+up`, `shift+down` | Scroll one item at a time |
| `page_down` | `pgdown`, ` `, `f` | Page down |
| `page_up` | `pgup`, `b` | Page up |
| `half_page_down` | `d` | Half page down |
| `half_page_up` | `u` | Half page up |
| `home` | `g`, `home` | Go to the top |
| `end` | `G`, `end` | Go to the bottom |
| `copy` | `c`, `y`, `C`, `Y` | Copy the selection |
| `clear_highlight` | `esc`, `alt+esc` | Clear the selection |
| `expand` | `space` | Expand or collapse the focused item |
| `scroll_left` | `shift+left`, `H` | Scroll the viewport left |
| `scroll_right` | `shift+right`, `L` | Scroll the viewport right |

### `initialize`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `yes` | `y`, `Y` | Confirm yes |
| `no` | `n`, `N`, `esc`, `alt+esc` | Decline |
| `switch` | `left`, `right`, `tab` | Switch the highlighted option |
| `enter` | `enter` | Select the highlighted option |

### `dialog`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `close` | `esc`, `alt+esc` | Close the dialog |

### `completions`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `down` | `down` | Move down in completions |
| `up` | `up` | Move up in completions |
| `select` | `enter`, `tab`, `ctrl+y` | Select the completion |
| `cancel` | `esc`, `alt+esc` | Dismiss completions |
| `down_insert` | `ctrl+n` | Insert next completion |
| `up_insert` | `ctrl+p` | Insert previous completion |

### `models`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `tab` | `tab`, `shift+tab` | Toggle large/small model |
| `select` | `enter`, `ctrl+y` | Confirm the model |
| `edit` | `ctrl+e` | Edit the model config |
| `up_down` | `up`, `down` | Move through the list |
| `next` | `down`, `ctrl+n` | Next item |
| `previous` | `up`, `ctrl+p` | Previous item |

### `sessions`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `select` | `enter`, `tab`, `ctrl+y` | Open the session |
| `next` | `down`, `ctrl+n` | Next item |
| `previous` | `up`, `ctrl+p` | Previous item |
| `up_down` | `up`, `down` | Move through the list |
| `delete` | `ctrl+x` | Delete the session |
| `rename` | `ctrl+r` | Rename the session |
| `confirm_rename` | `enter` | Confirm the rename |
| `cancel_rename` | `esc` | Cancel the rename |
| `confirm_delete` | `y` | Confirm the delete |
| `cancel_delete` | `n`, `esc` | Cancel the delete |

### `commands`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `select` | `enter`, `ctrl+y` | Run the command |
| `up_down` | `up`, `down` | Move through the list |
| `next` | `down` | Next item |
| `previous` | `up`, `ctrl+p` | Previous item |
| `tab` | `tab` | Switch selection |
| `shift_tab` | `shift+tab` | Switch selection (reverse) |

### `filepicker`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `select` | `enter` | Accept the file |
| `down` | `down`, `j` | Move down |
| `up` | `up`, `k` | Move up |
| `forward` | `right`, `l` | Enter the directory |
| `backward` | `left`, `h` | Go up a directory |
| `navigate` | `right`, `l`, `left`, `h`, `up`, `k`, `down`, `j` | Navigate the tree |
| `close` | `esc`, `alt+esc` | Close the file picker |

### `arguments`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `confirm` | `enter` | Confirm the arguments |
| `next` | `down`, `tab` | Next field |
| `previous` | `up`, `shift+tab` | Previous field |

### `permissions`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `left` | `left`, `h` | Previous option |
| `right` | `right`, `l` | Next option |
| `tab` | `tab` | Next option |
| `select` | `enter`, `ctrl+y` | Confirm the choice |
| `allow` | `a`, `A`, `ctrl+a` | Allow once |
| `allow_session` | `s`, `S`, `ctrl+s` | Allow for the session |
| `deny` | `d`, `D` | Deny |
| `toggle_diff_mode` | `t` | Toggle the diff view |
| `toggle_fullscreen` | `f` | Toggle fullscreen |
| `scroll_up` | `shift+up`, `K` | Scroll the diff up |
| `scroll_down` | `shift+down`, `J` | Scroll the diff down |
| `scroll_left` | `shift+left`, `H` | Scroll the diff left |
| `scroll_right` | `shift+right`, `L` | Scroll the diff right |
| `choose` | `left`, `right` | Choose an option |
| `scroll` | `shift+left`, `shift+down`, `shift+up`, `shift+right` | Scroll the diff |

### `reasoning`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `select` | `enter`, `ctrl+y` | Confirm the effort |
| `next` | `down`, `ctrl+n` | Next item |
| `previous` | `up`, `ctrl+p` | Previous item |
| `up_down` | `up`, `down` | Move through the list |

### `notifications`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `select` | `enter`, `ctrl+y` | Confirm the choice |
| `next` | `down`, `ctrl+n` | Next item |
| `previous` | `up`, `ctrl+p` | Previous item |
| `up_down` | `up`, `down` | Move through the list |

### `quit`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `left_right` | `left`, `right` | Switch options |
| `enter_space` | `enter`, ` ` | Confirm the choice |
| `yes` | `y`, `Y`, `ctrl+c` | Quit |
| `no` | `n`, `N` | Cancel quit |
| `tab` | `tab` | Switch options |
| `quit` | `ctrl+c` | Force quit |

### `oauth`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `copy` | `c` | Copy the device code |
| `submit` | `enter`, `ctrl+y` | Copy the code and open the URL |

### `api_key`

| Action | Default keys | What it does |
|--------|--------------|--------------|
| `submit` | `enter`, `ctrl+y` | Submit the API key |

## Key spelling

Keys use bubbletea's naming:

- Modifiers: `ctrl+`, `alt+`, `shift+` (combine, e.g. `ctrl+shift+k`).
- Named keys: `enter`, `tab`, `esc`, `space`, `up`, `down`, `left`, `right`,
  `home`, `end`, `pgup`, `pgdown`, `backspace`, `delete`.
- Plain characters: `a`, `/`, `?` (case-sensitive — `g` and `G` are distinct).

If a binding doesn't seem to take effect, the most common cause is the terminal
or multiplexer intercepting the chord before Crush sees it. `ctrl+l` under tmux
is the classic example — either rebind it in Crush (as above) or exempt Crush in
your tmux `vim-tmux-navigator` `is_vim` pattern.

## Precedence

Keybinding overrides follow the same config precedence as everything else:
project `.crush.json` / `crush.json` override the global
`~/.config/crush/crush.json`. So you can set a machine-wide default and tweak it
per project.
