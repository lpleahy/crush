#!/usr/bin/env bash
# Smoke test for the integrated `crush` binary: exercises the CLI surface
# of every feature on `main`. Run from the crush repo root:  scripts/smoke.sh
# Pairs with the Go integration smoke in internal/smoke and the manual TUI
# checklist in docs/smoke-checklist.md.
set -uo pipefail

fail=0
pass() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }
has()  { # desc want cmd...
  local desc="$1" want="$2"; shift 2
  # Capture fully before matching: piping to `grep -q` would exit on the
  # first match, SIGPIPE the still-writing command, and (under pipefail)
  # report a false failure on large-output commands.
  local out; out="$("$@" 2>&1)"
  case "$out" in *"$want"*) pass "$desc" ;; *) bad "$desc (missing: $want)" ;; esac
}

echo "== build the integrated binary =="
CRUSH=$(mktemp -u /tmp/crush-smoke.XXXX)
go build -o "$CRUSH" . || { echo "BUILD FAILED"; exit 1; }
pass "binary builds"

echo "== CLI surface =="
has "version runs"        "."       "$CRUSH" --version
has "help runs"           "crush"   "$CRUSH" --help
has "login: chatgpt"      "chatgpt" "$CRUSH" login --help
has "logout: chatgpt"     "chatgpt" "$CRUSH" logout --help
has "keybindings cmd"     "GROUP"   "$CRUSH" keybindings

echo "== keybindings: every group present =="
groups=$("$CRUSH" keybindings 2>/dev/null | awk 'NR>1 && $1!="" {print $1}' | sort -u)
n=$(printf '%s\n' "$groups" | grep -c .)
{ [ "$n" -ge 17 ] && pass "lists $n groups (>=17)"; } || bad "only $n groups (<17)"
for g in global editor chat initialize dialog completions models sessions permissions reasoning notifications quit oauth api_key filepicker commands arguments; do
  printf '%s\n' "$groups" | grep -qx "$g" && pass "group: $g" || bad "missing group: $g"
done

echo "== schema: every feature knob =="
schema="$("$CRUSH" schema 2>/dev/null)"
for k in vim_mode cursor_blink '"theme"' '"themes"' '"transparent"' '"keybindings"' '"hooks"' '$schema'; do
  printf '%s' "$schema" | grep -qF -- "$k" && pass "schema has $k" || bad "schema missing $k"
done

echo "== keybindings: / opens commands from the composer, command palette on ctrl+p =="
# Search/copy-mode keys are intentionally decoupled from the catalog (direct
# bindings), so they don't appear in `crush keybindings`. The catalog routes
# editor "/" to commands and the global palette to ctrl+p.
kb="$("$CRUSH" keybindings 2>/dev/null)"
case "$kb" in *"editor"*"commands"*) pass "editor 'commands' action present (/ from composer)" ;; *) bad "editor 'commands' action missing" ;; esac
case "$kb" in *"commands"*"ctrl+p"*) pass "command palette on ctrl+p" ;; *) bad "command palette ctrl+p missing" ;; esac

echo "== kitchen-sink config loads + override is effective =="
tmp=$(mktemp -d)
cat > "$tmp/crush.json" <<'JSON'
{
  "options": { "tui": {
    "vim_mode": true, "theme": "midnight", "transparent": true,
    "keybindings": { "global": { "models": ["ctrl+m"] }, "chat": { "half_page_down": ["ctrl+d"] } }
  } },
  "themes": { "midnight": { "extends": "tokyonight-storm", "bg_base": "#1a1b26" } },
  "hooks": { "Stop": [ { "command": "true" } ] }
}
JSON
out=$(cd "$tmp" && CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1 "$CRUSH" keybindings 2>&1)
ec=$?
{ [ "$ec" -eq 0 ] && printf '%s' "$out" | grep -q GROUP; } && pass "binary runs with kitchen-sink config" || bad "binary errored with kitchen-sink config (exit $ec)"
printf '%s' "$out" | awk '$1=="chat" && $2=="half_page_down"' | grep -q "ctrl+d" \
  && pass "chat override (half_page_down=ctrl+d) is effective" \
  || bad "chat override not effective (config path?)"
rm -rf "$tmp" "$CRUSH"

echo
if [ "$fail" -eq 0 ]; then echo "SMOKE: PASS"; else echo "SMOKE: FAIL"; fi
exit "$fail"
