# Signing in with ChatGPT

Crush can talk to the ChatGPT backend using your ChatGPT
Plus/Pro/Team/Enterprise subscription, without an `OPENAI_API_KEY`. This uses
the same public OAuth flow as OpenAI's official Codex CLI.

> [!NOTE]
> Access to the ChatGPT backend is governed by your OpenAI subscription's
> Terms of Use and Usage Policies; you are responsible for compliance, and
> OpenAI may rate-limit, revoke, or suspend access at any time. This feature is
> for personal use with your own subscription — for commercial or production
> workloads, use an OpenAI Platform API key (`OPENAI_API_KEY`). Crush is not
> affiliated with or endorsed by OpenAI.

## Logging in

```sh
crush login chatgpt
```

By default Crush opens your browser to sign in with OpenAI. After you
authorize, the browser redirects to a local callback
(`http://localhost:1455/auth/callback`) that Crush is listening on, and the
sign-in completes. The authorize URL is also printed to the terminal in case
the browser doesn't open on its own.

If you're already signed in, the command is a no-op. Use `crush login -f
chatgpt` to force re-authentication.

### Headless / SSH (device-code flow)

On a machine where Crush can't open a local browser — for example over SSH —
pass `--no-browser` to use the device-code flow instead:

```sh
crush login --no-browser chatgpt
```

Crush also auto-detects SSH sessions (via `SSH_CONNECTION` / `SSH_CLIENT`) and
switches to this flow on its own. Crush prints a short user code (and copies it
to your clipboard) and asks you to open `https://auth.openai.com/codex/device`
on any device with a browser and enter the code there. Crush polls in the
background and finishes once you've authorized. The code expires after 15
minutes.

If your workspace administrator has disabled device login, Crush reports this
and you can use the browser flow instead (run without `--no-browser`).

### From the TUI

If you pick a ChatGPT model in the model picker without being signed in, Crush
opens a sign-in modal showing the same device code and verification URL. Press
`c` to copy the code, or `enter` to copy the code and open the verification
page in your browser.

## Logging out

```sh
crush logout chatgpt
```

This makes a best-effort call to revoke the token server-side and then removes
the stored credentials. Revocation failure (e.g. no network) does not block
logout — you are signed out locally regardless. Use `-f` to skip the
confirmation prompt.

## Where credentials live

On successful login, Crush stores the OAuth token (access token, refresh token,
and expiry) in the `chatgpt` provider entry of your global Crush config
(`$CRUSH_GLOBAL_CONFIG/crush.json`, or `~/.config/crush/crush.json` by
default). Access tokens are short-lived (~5 minutes) and Crush refreshes them
automatically using the stored refresh token; rotated tokens are written back
to disk. The `ChatGPT-Account-ID` request header is derived from the access
token at request time, so it is not stored separately.

The access token is a JWT. Crush decodes its payload **without verifying the
signature** to read the account ID claim — this is intentional and safe: the
token is issued by OpenAI and received directly from `auth.openai.com` over
TLS, so its contents are trusted.

## Environment variables

| Variable                  | Purpose                                                    |
| ------------------------- | ---------------------------------------------------------- |
| `CRUSH_CODEX_CLI_VERSION` | Codex CLI version Crush mimics in the User-Agent and the `client_version` query param sent to `/models`. Defaults to a pinned value. |

The default `CRUSH_CODEX_CLI_VERSION` is pinned in the source. The ChatGPT
backend gates each model on a `minimal_client_version`: if the pinned version
drifts below the minimum for a live model, that model can **silently disappear
from the available model list** and inference may return "not supported".
Override this variable to a newer Codex CLI version to recover those models
without rebuilding Crush. If requests start failing with a Cloudflare challenge
or a region-restriction error, bumping this variable or falling back to
`OPENAI_API_KEY` is the suggested workaround.
