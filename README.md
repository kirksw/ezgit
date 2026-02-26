# ezgit

[![CI](https://github.com/kirksw/ezgit/actions/workflows/ci.yml/badge.svg)](https://github.com/kirksw/ezgit/actions/workflows/ci.yml)

CLI for cloning, opening, converting, and caching GitHub repositories with worktree-focused workflows.

## Table of Contents

- [Install](#install)
- [Quick Start](#quick-start)
- [Commands](#commands)
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
ezgit                             # fuzzy-pick repo, ensure local, then open
ezgit facebook/react              # ensure repo exists locally, then open
ezgit facebook/react my-feature   # ensure named worktree exists, then open
ezgit --no-open facebook/react    # ensure only (do not run open_command)
```

## Commands

### `ezgit [owner/repo] [worktree-name]`

Primary command. It ensures local state, then runs `open_command` unless `--no-open` is set.

- `ezgit` (no args): fuzzy picker over cached repos, then ensure+open.
- `ezgit owner/repo`: ensure repo exists (regular clone), then open.
- `ezgit owner/repo worktree`: ensure worktree exists (worktree layout), then open.

No-arg picker shortcuts:

- `tab`: toggle repo scope `all -> local -> opened`.
- `left/right`: switch focus between repo list and worktree pane.
- `enter` in repo pane: open repo root.
- `enter` in worktree pane: open selected worktree (repo root is intentionally hidden there).
- `enter` on `+ Create new worktree`: inline create mode (`name[:base]`) and create+open on confirm.
- `esc` / `ctrl+c`: cancel.

Flags: `--no-open`, `-b` branch, `--depth` shallow clone depth, `-q` quiet, `--key-path` SSH key, `-d` destination directory, `--feature`, `--feature-base`.

Worktree mode is now implicit:

- positional worktree name is provided (`ezgit owner/repo worktree`), or
- in interactive no-arg flow, more than one worktree is selected.

### `ezgit convert [path]`

Convert a local repository to bare metadata in `.git` + worktrees. Without a path, opens a fuzzy picker over local repos.

Flags: `-w` create worktree for specific branch(es), `--all-worktrees` create worktree for all branches, `--no-worktrees` skip worktree creation, `--key-path` SSH key.

### `ezgit cache <subcommand>`

Cache operations: `refresh`, `list`, `search`, `invalidate`.

Flags (on `cache`): `--force` full refresh regardless of TTL, `--ttl` custom TTL duration (e.g. `24h`).

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
open_command = "sesh connect \"$absPath\""
shallow_prompt_threshold_kb = 204800
```

GitHub auth resolution order:

1. `gh auth token`
2. `[github].token` in config
3. `GITHUB_TOKEN` environment variable

## Open Command Templates

`open_command` runs through `bash -lc` when `ezgit` opens a resolved repo/worktree path and can use:

- `$org`
- `$repo`
- `$worktree`
- `$orgRepo`
- `$repoPath`
- `$repoFullName`
- `$absPath`

Examples:

```toml
[git]
# sesh expects an absolute path
open_command = "sesh connect \"$absPath\""

# tmux: create or attach session named org/repo[/worktree]
open_command = "tmux new-session -A -s \"$repoPath\" -c \"$absPath\""
```

## Cache Behavior

- Cache refresh respects TTL by default and skips remote fetches while cache is fresh.
- `--force` performs a full refresh regardless of TTL.
- Use `ezgit cache refresh --ttl <duration>` to set a custom TTL for that refresh run.
- `ezgit` (no args) and picker-based flows trigger automatic cache refresh attempts before listing repos.

## Zoxide Integration

Repository ensure/clone/convert flows register paths with `zoxide` using `zoxide add`.

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
