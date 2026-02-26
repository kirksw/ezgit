# ezgit

CLI for cloning, opening, converting, and caching GitHub repositories with worktree-focused workflows.

## Install

```bash
go install github.com/kirksw/ezgit@latest
```

Or with Nix:

```bash
nix profile install github:kirksw/ezgit
```

## Config

Config lookup order:
1. `--config /path/to/config.toml`
2. `./config.toml`
3. `~/.config/ezgit/config.toml`
4. `~/.ezgit.toml`

Minimal example:

```toml
[organizations]
orgs = ["facebook", "google"]

[repos]
private = ["my-org/private-repo"]

[git]
clone_dir = "~/git/github.com"
worktree = false
sesh_open = false
open_command = "sesh connect \"$repoPath\""
shallow_prompt_threshold_kb = 204800
```

GitHub auth resolution order:
1. `gh auth token`
2. `[github].token` in config
3. `GITHUB_TOKEN` environment variable

## Commands

- `ezgit clone [owner/repo]`: clone repos, supports worktree mode and shallow clone.
- `ezgit add <owner/repo> <worktree-name>`: add a feature worktree to an existing worktree-layout repo.
- `ezgit open`: open local repos using `open_command`.
- `ezgit connect [session]`: connect to tmux session (fuzzy-select if omitted).
- `ezgit convert [path]`: convert local repo to bare metadata in `.git` + worktrees.
- `ezgit tui`: unified one-page TUI for clone/open/connect.
- `ezgit cache refresh|list|search|invalidate`: cache operations.

## Unified TUI (`ezgit tui`)

Single searchable list page with mode switching:

- `tab` / `left` / `right`: switch mode (`clone`, `open`, `connect`) and replace list contents.
- `enter`: run selected item in current mode.
- `ctrl+w`: toggle clone worktree mode (clone mode only).
- `ctrl+c`: convert selected local repo (open mode only).
- `esc` / `ctrl+c`: cancel.

## Existing Clone Behavior

When cloning into a destination that already exists:

- Regular clone target already exists: clone is skipped and treated as success.
- Worktree clone target already in worktree layout: clone is skipped and worktree creation continues.
- Worktree clone target is a regular (non-worktree) clone: interactive prompt offers:
  - open existing repo
  - auto-convert to worktree layout
  - cancel

When cloning via the fuzzy picker (`ezgit clone` with no args), this case defaults to opening the existing repo so the flow stays non-blocking.

In non-interactive mode, this last case returns a clear error with guidance.

## Open Command Templates

`open_command` runs through `bash -lc` and can use:

- `$org`
- `$repo`
- `$worktree`
- `$orgRepo`
- `$repoPath`
- `$repoFullName`
- `$absPath`

Example:

```toml
[git]
open_command = "sesh connect \"$repoPath\""
```

## Worktree Layout

Worktree-mode clone/convert layout:

```text
<repo>/.git/      # bare metadata
<repo>/main/      # default branch worktree
<repo>/review/    # review worktree
<repo>/<feature>/ # optional feature worktree
```

## Zoxide Integration

`clone`, `add`, and conversion paths register repository/worktree paths with `zoxide` using `zoxide add`.

- Adds the repo root path.
- Adds worktree paths (`main`, `review`, and feature worktrees when present).
- Best-effort only: failures do not fail the command.

## Cache Behavior

- Cache refresh respects TTL by default and skips remote fetches while cache is fresh.
- `--force` performs a full refresh regardless of TTL.
- Use `ezgit cache refresh --ttl <duration>` to set a custom TTL for that refresh run.
- `clone`, `open`, and `tui` trigger automatic cache refresh attempts before listing repos.

## Development

```bash
go test ./...
nix run .# -- --help
```

Enable local git hooks (recommended):

```bash
./scripts/install-hooks.sh
```

## Versioning

- `internal/version/VERSION` stays on a development value (for example `0.0.0-dev`) on normal branch work.
- `ezgit version` reads that value for local/dev builds.
- Releases are triggered by pushing a semantic-version tag (for example `v0.0.7`).
- Release builds inject the tag version into binaries at link time.
- GitHub release body is sourced from the matching `RELEASE_NOTES.md` section (`## <version> - <date>`), and the workflow fails if the section is missing.

## License

MIT
