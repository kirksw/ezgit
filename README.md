# ezgit

[![CI](https://github.com/kirksw/ezgit/actions/workflows/ci.yml/badge.svg)](https://github.com/kirksw/ezgit/actions/workflows/ci.yml)

CLI for cloning, opening, converting, and caching GitHub repositories with worktree-focused workflows.

## Table of Contents

- [Install](#install)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Unified TUI](#unified-tui-ezgit-tui)
- [Worktree Layout](#worktree-layout)
- [Config](#config)
- [Open Command Templates](#open-command-templates)
- [Cache Behavior](#cache-behavior)
- [Zoxide Integration](#zoxide-integration)
- [Development](#development)
- [Versioning](#versioning)
- [License](#license)

## Install

```bash
go install github.com/kirksw/ezgit@latest
```

Or with Nix:

```bash
nix profile install github:kirksw/ezgit
```

## Quick Start

```bash
ezgit clone facebook/react        # clone a repo
ezgit clone -w facebook/react     # clone with worktree layout
ezgit open                        # fuzzy-pick and open a local repo
ezgit tui                         # all-in-one interactive mode
```

## Commands

### `ezgit clone [owner/repo]`

Clone a GitHub repository. With no arguments, opens a fuzzy picker over cached repos.

Flags: `-w` worktree mode, `-b` branch, `--depth` shallow clone depth, `-q` quiet, `--key-path` SSH key, `-d` destination directory, `--feature` create an additional feature worktree, `--feature-base` base branch for `--feature`.

When the destination already exists:

- **Regular clone target exists**: clone is skipped (treated as success).
- **Worktree layout already present**: clone is skipped, worktree creation continues.
- **Existing non-worktree clone with `-w`**: interactive prompt offers to open the repo, auto-convert to worktree layout, or cancel. Via the fuzzy picker this defaults to opening the repo. In non-interactive mode a clear error with guidance is returned.

### `ezgit add <owner/repo> <worktree-name>`

Add a feature worktree to an existing worktree-layout repo.

### `ezgit open`

Open a locally cloned repository using the configured `open_command`. Launches a fuzzy picker over local repos.

### `ezgit connect [session]`

Connect to a tmux session. Fuzzy-selects if session name is omitted.

### `ezgit convert [path]`

Convert a local repository to bare metadata in `.git` + worktrees. Without a path, opens a fuzzy picker over local repos.

Flags: `-w` create worktree for specific branch(es), `--all-worktrees` create worktree for all branches, `--no-worktrees` skip worktree creation, `--key-path` SSH key.

### `ezgit tui`

Launch the unified interactive TUI (see [below](#unified-tui-ezgit-tui)).

### `ezgit cache <subcommand>`

Cache operations: `refresh`, `list`, `search`, `invalidate`.

Flags (on `cache`): `--force` full refresh regardless of TTL, `--ttl` custom TTL duration (e.g. `24h`).

## Unified TUI (`ezgit tui`)

Single searchable list page with mode switching:

- `tab` / `left` / `right`: switch mode (`clone`, `open`, `connect`) and replace list contents.
- `enter`: run selected item in current mode.
- `ctrl+w`: toggle clone worktree mode (clone mode only).
- `ctrl+c`: convert selected local repo (open mode only).
- `esc` / `ctrl+c`: cancel.

## Worktree Layout

Worktrees let you check out multiple branches simultaneously without stashing or switching — useful for code review, parallel feature work, and CI investigation.

Worktree-mode clone/convert layout:

```text
<repo>/.git/      # bare metadata
<repo>/main/      # default branch worktree
<repo>/review/    # review worktree
<repo>/<feature>/ # optional feature worktree
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

## Cache Behavior

- Cache refresh respects TTL by default and skips remote fetches while cache is fresh.
- `--force` performs a full refresh regardless of TTL.
- Use `ezgit cache refresh --ttl <duration>` to set a custom TTL for that refresh run.
- `clone`, `open`, and `tui` trigger automatic cache refresh attempts before listing repos.

## Zoxide Integration

`clone`, `add`, and conversion paths register repository/worktree paths with `zoxide` using `zoxide add`.

- Adds the repo root path.
- Adds worktree paths (`main`, `review`, and feature worktrees when present).
- Best-effort only: failures do not fail the command.

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
