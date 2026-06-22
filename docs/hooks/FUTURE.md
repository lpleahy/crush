# Hooks: Future Work

This document tracks planned features and design notes for hooks that are not
yet implemented. Nothing here is part of the current contract. Treat it as a
scratchpad for what's next, not as documentation of current behavior.

> [!NOTE]
> This document was largely LLM-generated.

## `context_files`

**Status:** planned, not implemented.

### Motivation

Today, a hook that wants to inject reference material into the agent's context
has exactly one knob: `context` (string or array of strings). Whatever the hook
puts there is concatenated into what the model sees. That's fine for short notes
("current branch: main", "scrubbed secrets") but it scales badly:

- Dumping a whole `README.md` or `package.json` into `context` burns tokens on
  every tool call where the hook fires.
- The model sees the file contents even if it doesn't need them.
- Large files can push the turn past the context window.

`context_files` is the lazy alternative: the hook returns **paths**, not
contents. Crush tells the agent the files exist and are relevant, and the agent
decides whether to open them with its existing `view` tool.

### Proposed shape

Additive envelope field. Accepts a list of strings:

```jsonc
{
  "decision": "allow",
  "context": "Scrubbed one secret",
  "context_files": ["README.md", "docs/ARCHITECTURE.md"],
}
```

Paths are resolved relative to `CRUSH_CWD`. Non-existent paths are dropped with
a debug log (don't fail the hook over a missing file).

### How the agent sees it

Crush appends a short note to the turn's context along the lines of:

```
## Referenced files
- README.md
- docs/ARCHITECTURE.md
```

No file contents are inlined. The agent opens them with `view` if it decides
they're relevant. This keeps cost proportional to need.

### Aggregation

Matches the existing rules for lists:

- Concatenates across matching hooks in config order.
- Deduplicates paths (same file referenced by two hooks → listed once).
- Dropped entirely if the final decision is `deny` or `halt`.

### Backwards compatibility

Purely additive. Hooks that don't emit `context_files` are unaffected. Existing
envelopes keep working unchanged. No version bump required.

### Open questions

- Should `context_files` paths be constrained to `CRUSH_PROJECT_DIR`? Probably
  yes, to avoid hooks smuggling in arbitrary filesystem reads.
- Do we want a per-file line range (`"README.md:1-40"`) or keep it dead simple
  (whole-file references only)? Start simple; add ranges only if asked for.
- Should we annotate "why this file is relevant" per entry? An object form
  (`{"path": "...", "reason": "..."}`) would allow that but complicates the
  schema. Defer until there's a real user need.

## Sub-agent opt-in

**Status:** not implemented.

### Background

Today hooks fire **only** on the top-level agent's tool calls. Sub-agents
(`agent` task tool, `agentic_fetch`, future delegated loops) run without hook
interception so a single delegated turn doesn't trigger the user's hook N times.

The outer sub-agent tool call itself is hooked, so blanket policy like "never
spawn sub-agents" or "rewrite prompts sent to the task agent" still works from
the coder's side. The sub-agent's inner loop is the part that's exempt.

### Why users might want the escape hatch

- Audit logging of every tool call, including delegated ones.
- Redaction hooks that want to apply uniformly regardless of who called the
  tool.
- Policy that cares about the _tool_ not the _caller_: "never fetch from this
  domain, even in `agentic_fetch`."

Until someone actually asks, don't ship this. YAGNI.

### Proposed shape

Additive, per-hook. Zero-value matches current default (skip sub-agents):

```jsonc
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^bash$",
        "command": "./hooks/audit.sh",
        "include_sub_agents": true, // default false
      },
    ],
  },
}
```

Implementation changes where `wrapToolsWithHooks` decides to skip. Instead of a
single `isSubAgent` bailout, the runner filters per-hook matches by the hook's
`include_sub_agents` flag. Hooks that opt in get wrapped into sub-agent tool
slices too; everything else stays skipped.

### Backwards Compatibility

Purely additive. Hooks that don't set `include_sub_agents` get the default
(`false` = skip sub-agents). No wire format change, no version bump. The initial
transition from "hooks fire everywhere" to "hooks skip sub-agents by default"
was a one-time behavior change; adding the opt-in is pure addition.

### Side benefit: payload awareness

Extend the stdin payload with `"is_sub_agent": true|false` so hook scripts that
opt in can branch on caller type ("audit top-level and sub-agent calls
differently"). Also purely additive — hooks that don't read the field are
unaffected.

### Open questions

- Per-hook flag (above) vs a global `hooks.include_sub_agents` default? A global
  toggle is simpler but coarse-grained; per-hook is more flexible and
  composable. Start per-hook; a global default can be layered on later with
  explicit precedence ("per-hook overrides global").
- Does an opt-in hook see hooks from _nested_ sub-agents too (a sub-agent that
  itself calls a sub-agent)? Probably yes — once you've opted in you want the
  full tree. But call it out explicitly in docs so users aren't surprised by N²
  explosions on pathological configs.

## `AssistantMessage` event

**Status:** once-per-message variant implemented; per-token streaming variant
not implemented.

The lifecycle event set (see [README.md](./README.md#events)) covers turn and
tool boundaries. `AssistantMessage` now fires **once per completed assistant
message** — after the model finishes a turn, on its terminal step (a step that
ends with tool calls is a continuation and is skipped). The payload carries the
full assistant `text`, a best-effort `message_token_count`, and the
`finish_reason`; the env-var copy of the text (`CRUSH_ASSISTANT_MESSAGE_TEXT`)
is truncated to ~4KB while the JSON `message_text` stays intact. Like the other
lifecycle events it is notification-only.

### Why the once-per-message variant first

Unlike the other lifecycle events, which fire at most a handful of times per
turn, assistant text streams token-by-token. A naive `AssistantMessage` that
fired per delta would spawn hundreds of subprocesses per turn, so per-token
firing is intentionally untenable. Firing once on the finalized message is
cheap and matches the "the assistant produced a message" use cases: stream a
running transcript to an external log, update a "last activity" timestamp, drive
a richer status display than the binary running/done.

### Remaining future work: streaming `partial` variant

An optional `message_kind: "partial"` behind a config flag, debounced to at most
~1/sec, for users who explicitly want intra-turn streaming progress and accept
the cost. The env var `CRUSH_MESSAGE_KIND` is reserved for this distinction
(`partial`/`final`); the current once-per-message firing does not set it. Until
a concrete use case needs intra-turn assistant text, the finalized-message
event plus the binary running→done signal from `UserPromptSubmit`/`Stop` is
enough.

## `UserPromptSubmit` control variant (rewrite / deny)

**Status:** notification variant implemented; control variant not implemented.

`UserPromptSubmit` now fires as a **notification-only** event (great for status
indicators). What's described below — the ability to **rewrite** the prompt
(`updated_prompt`) or **deny** the submission before it reaches the LLM —
remains future work. The notification variant ignores any decision the hook
returns.

### Motivation

The notification variant tells external tooling a turn started. The control
variant would let hooks inject context, rewrite prompts, or gate on content
without the mutation complexity of `PostToolUse` (output scrubbing, error
coercion, size limits — all rabbit holes).

### Use cases

- Prepend project context the user didn't think to include ("current branch:
  `feat/x`; last commit: `<sha> <title>`").
- Point at reference files via `context_files` (when that lands) so the agent
  knows where to look without being force-fed contents.
- Redact secrets out of the prompt before it leaves the machine.
- Refuse prompts matching a policy ("don't send anything mentioning
  `production.env`") — with `deny` and a reason the user sees.
- Expand shorthand (`@TODO` → "please address the TODO in …").

### Proposed shape

Stdin payload extends the common envelope with the prompt:

```jsonc
{
  "event": "UserPromptSubmit",
  "session_id": "…",
  "cwd": "/home/user/project",
  "prompt": "fix the login flow",
  "attachments": ["screenshot.png"],
}
```

Output envelope reuses common fields plus one new per-event field,
`updated_prompt`:

```jsonc
{
  "decision": "allow", // optional; deny blocks the submission entirely
  "reason": "includes a production secret", // shown to the user when denying
  "context": "Current branch: feat/login",
  "updated_prompt": "fix the login flow\n\n(from @TODO on line 42)",
}
```

`updated_prompt` is a **full replacement** — not a merge patch — because a
prompt is a single string with no natural key structure. If multiple hooks emit
`updated_prompt`, later hooks in config order win.

### Aggregation

Reuses the universal rules:

- `halt` is sticky. Halts the whole turn before the LLM is called.
- `context` concatenates in config order.
- `updated_prompt`: last writer wins.
- `decision: "deny"` blocks the submission. The user sees `reason`; the turn
  never reaches the LLM.

### Differences from `PreToolUse`

- No `updated_input`: there are no tool inputs at this point.
- No permission-prompt bypass: there's no permission prompt for a user prompt.
- `decision: "allow"` is functionally identical to silence. It exists only for
  symmetry with `PreToolUse` and to give hook authors a consistent vocabulary.
  (Could be argued both ways — consider dropping it here.)
- Fires on every user submission, including follow-ups in the same session.
  Hooks should be fast; no subprocess-per-keystroke scenarios but the per-turn
  overhead is real.

### Implementation sketch

- New event constant `EventUserPromptSubmit` in `internal/hooks/hooks.go`.
- `Runner.Run` already takes an event name; no interface change.
- A new call site in `sessionAgent.Run` (or the coordinator's Run path) that
  fires hooks after creating the user message but before the first LLM call. If
  the aggregate decision is `deny` or `halt`, abort the turn and surface
  `reason` to the user.
- If hooks return `context`, prepend it to the prompt seen by the LLM (or attach
  as a system-message-level note — decide based on how the prompt is threaded
  through fantasy).
- If hooks return `updated_prompt`, replace the prompt body before the first LLM
  call. The message row in the DB should still store the _original_ prompt so
  the user sees what they typed; only the outbound version is rewritten. (Or:
  store both, show the original, send the rewritten — mirror how `updated_input`
  is handled today.)

### Open questions

- Store original vs rewritten prompt? Probably both, with UI showing original
  and a subtle indicator that a hook modified it.
- Do hooks fire on queued prompts too, or only when actually dispatched? If the
  user queues three prompts and the hook blocks the second, what happens to the
  third? Simplest rule: fire when dispatched; denial skips to the next queued
  prompt with a visible note.
- What about the `/commands` prefix? Does `UserPromptSubmit` fire for slash
  commands, or are those intercepted earlier? Probably earlier — hooks see only
  freeform prompts that would actually reach the LLM.

## Cross-platform shell (Windows support)

**Status:** implemented. See the [Execution model](README.md#execution-model)
section in `README.md` for the current behavior and contract.
