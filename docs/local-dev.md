# Local development against a sibling catwalk

`crush` depends on `charm.land/catwalk` for provider metadata. When you're
hacking on both repos at once (adding a provider, debugging the embedded
list, etc.), pin the dependency at the workspace level so Go uses your
local checkout instead of the published module.

## Setup

Check both repos out side-by-side:

```
~/code/anywhere/
├── crush/
└── catwalk/
```

Create a `go.work` file in the parent directory:

```
go 1.26

use (
	./catwalk
	./crush
)
```

That's it. `go build` from inside either repo now resolves
`charm.land/catwalk` to the sibling checkout. The workspace file is
local-only — don't commit it to either repo.

## When you've added a new provider to catwalk

The default crush behavior is to fetch providers from
`https://catwalk.charm.land` at startup (`Providers()` in
`internal/config/provider.go`), which overrides the embedded list. If
the provider you're working on isn't merged upstream yet, disable that
fetch for local runs:

```
CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1 ./crush
```

`./crush update-providers embedded` writes the embedded list to the
cache, but the next launch will fetch over it unless you also disable
auto-update.

Once the catwalk PR lands and `catwalk.charm.land` knows about the new
provider, the env var is no longer needed.
