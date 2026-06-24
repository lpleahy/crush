# Fork maintenance

This fork carries local changes on top of Charm's upstream `crush` and a
sibling `catwalk` fork. Keep the two repositories synchronized deliberately:
Catwalk owns provider metadata, and Crush consumes it through the parent
`go.work` workspace.

## Repository layout

Expected local layout:

```text
~/code/pi/
├── go.work
├── catwalk/
└── crush/
```

`go.work` should include both modules so local Crush builds use the sibling
Catwalk checkout:

```text
use (
	./catwalk
	./crush
)
```

## Preflight

Run this in each repository before rebasing:

```sh
git status --short
git branch --show-current
git remote -v
git log --oneline --decorate -10
```

Do not start a rebase with unrelated uncommitted changes. Commit, stash, or
move them aside first.

## Sync remotes

Run in each repository:

```sh
git fetch --all --prune
```

The expected remotes are:

- `origin`: personal fork
- `upstream`: Charm upstream

## Create a safety branch

Before rewriting history, create a local recovery point:

```sh
git branch backup/pre-rebase-$(date +%Y%m%d-%H%M%S)
```

Keep the backup branch until tests pass and the rebased fork has been pushed.

## Rebase order

Rebase Catwalk first, then Crush.

Why:

- Catwalk contains provider metadata and generated model lists.
- Crush imports Catwalk.
- The parent `go.work` makes Crush tests/builds use the sibling Catwalk
  checkout, so a stale Catwalk rebase can make Crush failures misleading.

## Rebase Catwalk

```sh
cd ~/code/pi/catwalk
git switch main
git rebase --rebase-merges upstream/main
```

Use `--rebase-merges` because the fork may contain intentional merge commits
that document integration work. If the branch has been kept linear, a plain
`git rebase upstream/main` is also acceptable.

After resolving any conflicts:

```sh
go test ./...
go build ./...
```

If the `task` command is available, prefer the repository targets:

```sh
task fmt
task test
task lint
```

## Rebase Crush

```sh
cd ~/code/pi/crush
git switch main
git rebase --rebase-merges upstream/main
```

After resolving any conflicts:

```sh
go test ./...
go build ./...
```

If `task` is available:

```sh
task fmt
task test
task lint
```

## Cross-repo verification

From the parent workspace:

```sh
cd ~/code/pi
go test ./catwalk/... ./crush/...
```

Then run a manual smoke check:

1. Launch Crush from the rebased checkout.
2. Confirm ChatGPT still appears in the model picker.
3. Select ChatGPT for both large and small models if using subscription-only
   operation.
4. Send a normal prompt.
5. Run an `agentic_fetch` query.

If local Catwalk metadata is needed before upstream provider data catches up,
launch Crush with:

```sh
CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1 crush
```

## Review the resulting fork delta

For each repository:

```sh
git log --oneline --decorate upstream/main..HEAD
git diff upstream/main...HEAD
```

If you created a backup branch, compare the rebased series against it:

```sh
git range-diff upstream/main backup/pre-rebase-YYYYMMDD-HHMMSS main
```

The fork delta should contain only intentional local changes.

## Push policy

A rebase rewrites history. Push only after review and approval:

```sh
git push --force-with-lease origin main
```

Use `--force-with-lease`, not `--force`, so the push fails if someone else
updated the fork while you were rebasing.

## When to merge instead of rebase

Prefer rebase for maintaining a clean fork patch stack. Consider a merge only
when preserving an already-shared integration history matters more than a clean
linear relationship to upstream. If you merge, still run the same tests and
cross-repo verification.
