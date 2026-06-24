# Smoke checklist — integrated `main`

Three layers verify the all-features build. The first two are automated; the
third is a manual pass for the interactive TUI pieces a script can't honestly
drive (they need a real TTY + a human eye).

1. **Go integration smoke** — `go test ./internal/smoke/...`
   Kitchen-sink config parses; keybindings/themes/vim/hooks/chatgpt all resolve.
2. **Binary CLI smoke** — `scripts/smoke.sh`
   Real binary: version, `keybindings` (all groups), `schema` (every knob),
   `login/logout` (chatgpt), kitchen-sink config loads + override effective.
3. **Manual TUI checklist** — below.

## Manual setup

Drop this `crush.json` in a scratch dir and launch `crush` there:

```json
{
  "options": { "tui": {
    "vim_mode": true, "theme": "midnight", "transparent": true,
    "keybindings": { "chat": { "half_page_down": ["ctrl+d"] } }
  } },
  "themes": { "midnight": { "extends": "tokyonight-storm", "primary": "#7aa2f7", "bg_base": "#1a1b26" } },
  "hooks": { "Stop": [ { "command": "echo stopped >> /tmp/crush-hook.log" } ] }
}
```

## Checklist

### Startup
- [ ] `crush` launches without error using the kitchen-sink config above.
- [ ] The TokyoNight-derived **midnight** theme renders (blue accent, dark bg).
- [ ] With **transparent: true**, the terminal background shows through (set a
      translucent/colored terminal bg to confirm).

### Vim mode (composer)
- [ ] Focus the composer — it starts in **NORMAL** (block cursor).
- [ ] `i` / `a` / `A` / `I` / `o` / `O` enter **INSERT** (bar cursor); typing works.
- [ ] `Esc` returns to NORMAL (block cursor); cursor steps left one.
- [ ] Motions: `h j k l`, `w b e` and `W B E` (WORD), `0 ^ _ $ g_`, `gg`/`G`,
      `f t F T` with `;`/`,` repeat, `{`/`}` paragraphs, `%` matching bracket,
      `ge`/`gE`, `|` column; counts like `3w`, `5l`.
- [ ] Edits: `x` `X`, `dd` `dw`/`dW`, `D` `C`, `S`/`Y`, `s`, `r<c>`, `~`,
      `J`/`gJ` (join); `u` undoes, `Ctrl+R` redoes.
- [ ] Operator + motion: `d`/`c`/`y` compose with every motion — `d0`, `d$`,
      `db`, `de`, `dW`, `d}`, `d%`, `dj`/`dk`/`dgg`/`dG` (linewise),
      `df<c>`/`dt<c>`. `c<motion>` deletes then enters insert (one undo);
      `cw`≈`ce`, `cc` clears the line.
- [ ] Text objects (with `d`/`c`/`y`/`v`): `iw`/`aw`, `iW`/`aW`, quotes
      `i"`/`a"` `i'` `` i` ``, brackets `i(`/`a(`/`ib`, `i{`/`iB`, `i[`, `i<`
      (and `)`/`}`/`]` aliases, cursor-on-close, multi-line `{ }`).
- [ ] Case ops: `gu`/`gU`/`g~` + motion/object (`guw`, `gUiW`, `gU$`), doubled
      `guu`/`gUU`/`g~~`; they don't touch the clipboard.
- [ ] Indent: `>>`/`<<` shift the current line (default 2 spaces; `3>>` for 3
      lines), `>j`/`>}`/`>G` shift the motion's lines, visual `>`/`<` shift the
      selection (`2>` = two levels). `options.tui.vim_indent` `{style, width}`
      changes the unit (e.g. `{"style":"tabs"}`).
- [ ] Undo covers a whole insert session: `i`, type some text, `Esc`, then `u`
      removes all of it in one step; `Ctrl+R` puts it back. (In NORMAL, `Ctrl+R`
      is redo; attachment delete-mode `Ctrl+R` still works in INSERT.)
- [ ] App chords still work in NORMAL: `Enter` sends, `Tab` switches focus,
      `Ctrl+P` / `Ctrl+.` open the command palette, `Ctrl+M` the model picker
      (i.e. vim doesn't swallow them).
- [ ] In NORMAL, `/` opens chat search directly (no need to enter insert first);
      in INSERT, `/` types a literal slash.

### Vim visual mode (composer)
- [ ] `v` enters charwise **VISUAL** (block cursor); motions (`h j k l`, `w b e`,
      `0 ^ $ _`, `gg`/`G`, `f`/`t`/`F`/`T`, counts) extend the selection from the
      anchor and the selected text renders highlighted (TextSelection style).
- [ ] `V` enters linewise **V-LINE**; `j`/`k` extend whole-line selection.
- [ ] Text objects select: `viw`/`vaw`/`viW`, `vi"`, `vi(`/`vi{` etc. (and
      `c`/`d`/`y` after them behave like `ciw`/`diw`/`yiw`).
- [ ] Operators on the selection then return to NORMAL: `d`/`x` delete, `y` yanks
      (then `p`/`P` pastes it), `c`/`s` delete and enter INSERT as one undo step;
      `u`/`U`/`~` change case, `J` joins, `r<c>` replaces, `o` swaps the active
      end, `p`/`P` paste over the selection.
- [ ] `gv` (in NORMAL) reselects the last visual selection.
- [ ] `v`/`V` toggle visual off; `Esc` cancels the selection. Esc in visual mode
      returns to NORMAL **first** — it does not cancel a running task.
- [ ] A multi-line selection highlights only the text — the `  >`/`:::` prompt
      gutter on each line is NOT painted over.
- [ ] Yank/delete (`y`, `d`, `x`, `c`, …) also copies to the **system clipboard**
      (OSC 52 + native), matching vim `unnamedplus`: after `yiw` you can paste the
      word into another app. Linewise copies carry a trailing newline.

### Search (focus-aware `/` + one-key output search)
- [ ] In the **composer** in vim NORMAL, `/` searches the **draft**: type a query
      and the composer cursor jumps to the match; `Enter` then `n`/`N` cycle; `Esc`
      leaves the cursor on the match.
- [ ] **`Alt+S`** (overridable `global.search_output`; `ctrl+/` alias) opens the
      **conversation/output** search from anywhere without `Tab`-ing to the chat.
- [ ] The black-box cursor bug is gone: opening `/` in vim mode shows a blinking
      **bar** cursor in the search field, not a solid block.

### Chat (output) search (two-phase: edit → navigate)
- [ ] `/` opens the inline search bar (a `/`-prefixed field, not the status line)
      from the chat list (after `Tab`), or `Alt+S` from anywhere. It opens in the
      **editing** phase.
- [ ] Editing: typing highlights **every** match at once (dim), with the active
      one brighter (TextSelection), and centers the active block live; `↑`/`↓`
      browse while you type; bare `n`/`N` are just query text. The bar hint reads
      `↑↓ browse · enter → n/N`.
- [ ] The block each match lives in shows as **selected** as you cycle — and this
      works whether search was opened from the conversation OR from the composer
      (the regression: composer-opened search used to highlight text but not
      select the block).
- [ ] `Enter` confirms → **navigation** phase: the bar stays, the field blurs,
      and now bare `n`/`N` (and `↑`/`↓`) browse without typing. Hint reads
      `n/N browse … · esc`.
- [ ] In navigation, `n`/`N` step through matches and **wrap** around the ends
      vim-style (forward from the newest → oldest; backward from the oldest →
      newest); no dead-end at the ends.
- [ ] In navigation, `/` jumps back to editing the query; `Esc`/`Tab` close the
      bar and clear the highlight.
- [ ] In navigation, `Enter` drops into **native copy mode** with the cursor on
      the matched word (no tmux). The bar hint reads `… · enter → select · esc`.

### Native copy mode (vim selection over the output, no tmux)
- [ ] With the chat focused (`Tab`), `v`/`V` enters copy mode — a cursor over the
      output. `h j k l`, `w b e`, `0 ^ $`, `gg`/`G` move it and the view **scrolls
      past the screen edges** to follow.
- [ ] `v` charwise / `V` linewise select; the highlight covers **only the text**,
      never the `▌` sidebar or block borders. `o` swaps the active end.
- [ ] `y` copies the selection to the system clipboard (OSC 52 + native) and the
      paste in another terminal is **clean** — a multi-line command has no sidebar,
      borders, or decorations. `Esc` leaves the selection, then copy mode.
- [ ] `/` + `Enter` from search drops the copy-mode cursor onto the match.

### Keybindings overrides
- [ ] The overridden `chat.half_page_down` = `Ctrl+D` works in the chat list.
- [ ] `crush keybindings` (CLI) shows it in the EFFECTIVE column.

### Dialogs
- [ ] Model picker opens (`Ctrl+M` or your override) and switches models.
- [ ] Sessions list, command palette (`Ctrl+P` or `Ctrl+.`), and a permission
      prompt render and respond to their keys.

### ChatGPT auth + model
- [ ] `crush login chatgpt` runs the browser (PKCE) flow; `--no-browser` runs the
      device-code flow; the modal in-TUI sign-in works.
- [ ] After login, a ChatGPT (gpt-5.x) model is selectable and responds.

### Hooks
- [ ] After a turn ends, the `Stop` hook fired (check `/tmp/crush-hook.log`).
