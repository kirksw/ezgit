# ezgit

[![CI](https://github.com/kirksw/ezgit/actions/workflows/ci.yml/badge.svg)](https://github.com/kirksw/ezgit/actions/workflows/ci.yml)

Clone and open Git repositories, create linked worktrees, and discover repositories from a GitHub cache.

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
ezgit                                           # open the GitHub repo picker
ezgit lunarway/hubble-continuum                  # clone if needed, then open
ezgit wt add lunarway/hubble-continuum feature   # create a branch and linked checkout
ezgit open lunarway/hubble-continuum feature     # open the existing linked checkout
```

## Commands

Repository arguments accept `owner/repo` (GitHub shorthand), `host/owner/repo`, and HTTP(S), SSH, or SCP-style Git URLs.
The shorthand and host-qualified forms clone over SSH.
The host, owner, and repo identify the checkout; SSH and HTTPS URLs for that identity share the same path.
Non-default URL ports are not supported.
GitHub discovery and the picker use the GitHub cache, but direct commands do not require GitHub authentication for other hosts.

- `ezgit [repo] [worktree]`: no arguments opens the picker; a repo argument clones the primary checkout if needed and opens it; a named worktree must already exist.
  Use `--no-open` to prepare or validate the checkout without launching it.
- `ezgit clone <repo>`: create the primary checkout if it does not already exist.
  Use `--branch`, `--depth`, `--quiet`, or `--key-path` to control cloning.
- `ezgit open <repo> [worktree]`: run `open_command` for an existing checkout.
  It never clones, creates worktrees, or converts a repository.
- `ezgit describe <repo>`: return JSON with the host, local path, layout, and registered linked worktree names.
- `ezgit list orgs`, `ezgit list repos [--local]`, and `ezgit list worktrees <repo>`: write one name per line.
- `ezgit cache <subcommand>`: `refresh`, `list`, `search`, or `invalidate`.
  Use `--force` for a full refresh, or `--ttl` to override cache freshness for the operation.
- `ezgit skill`: print the embedded agent instructions.

In the picker, `tab` cycles repo scopes, `left`/`right` switches panes, `enter` opens the selected checkout, and `esc` or `ctrl+c` cancels.
Choose `+ new worktree` and enter `name[:base]` to explicitly create and open a linked checkout.

## Worktree Layout

The primary path is a normal checkout; linked worktrees share its Git metadata but keep files and branch checkouts separate:

```text
~/git/github.com/lunarway/hubble-continuum/
~/git/.worktrees/github.com/lunarway/hubble-continuum/feature/
```

`ezgit wt add <repo> <name>` (also `ezgit worktree add`) creates a new branch and linked worktree for an already cloned repository.
Use a single branch name without slashes.
Use `--base <ref>` to choose a local ref or commit; otherwise ezgit checks the local `origin/HEAD` ref and falls back to known local default-branch names.
It does not fetch for you.
If a branch or destination already exists, creation fails rather than sharing or overwriting another checkout.
Git determines the location of existing linked worktrees, including ones created by other tools.
When names collide, `ezgit list worktrees` prints their absolute paths; pass one of those paths to `ezgit open`.

### Parallel agents

Clone once and choose a base commit before starting parallel writers.
Give each writer a unique worktree and branch; pass the returned absolute `path` to the agent harness as its working directory.

```bash
base_commit=$(git -C ~/git/github.com/lunarway/hubble-continuum rev-parse HEAD)
ezgit wt add lunarway/hubble-continuum fix-code --base "$base_commit" --json
ezgit wt add lunarway/hubble-continuum fix-test --base "$base_commit" --json
```

Each command prints one JSON object with `path`, `branch`, and the resolved `base_commit`.
The parent agent integrates and verifies the worker commits before submitting.
After integration, use `git worktree remove <path>` for explicit cleanup and retain dirty checkouts for inspection.
`ezgit wt prune <repo>` previews linked checkouts inactive for 14 days by default; use `--older-than 30d` to change the threshold or `--apply` to remove candidates that Git permits removing without force.
It keeps branches and skips the primary checkout, locked worktrees, and `main` / `master`.
`--apply` also skips worktrees with local changes, untracked files, or ignored files.

### Existing installations

Existing clones and bare-layout worktrees are not moved or converted automatically.
With only `[git].clone_dir` set, GitHub repos continue to resolve under that directory.
Set `[git].root` to use the new host-aware layout for new clones; if both are set, `clone_dir` remains a GitHub fallback when the canonical path does not exist.
For the old default `clone_dir = "~/git/github.com"`, the new default already points to the same primary paths.
Use `wt add` instead of `convert`, `clone --worktree` / `--bare`, `clone <repo> <worktree>`, or `--feature` / `--feature-base`; these deprecated forms retain their old behavior where supported.
`clone --dest` is also deprecated; managed commands do not look up arbitrary destinations.

## Config

Config lookup order:

1. `--config /path/to/config.toml`
2. `~/.config/ezgit/config.toml`
3. `~/.ezgit.toml`

A `config.toml` in the working directory is not loaded automatically, because `open_command` runs as a shell command. Pass `--config ./config.toml` to use one you trust.

Minimal example:

```toml
[organizations]
orgs = ["facebook", "google"]

[repos]
private = ["my-org/private-repo"]

[git]
root = "~/git"
open_command = "sesh connect \"$absPath\""
shallow_prompt_threshold_kb = 204800
```

Clones use your normal ssh setup (agent, `~/.ssh/config`, default keys). Pass `--key-path` to `ezgit` or `ezgit clone` to use a specific key.

Names that could resolve outside the configured root, such as `../repo`, are rejected.

Prompts only appear when stdin is a terminal. With stdin redirected, including from `/dev/null`, commands fail with an error rather than wait for input.

GitHub auth resolution order:

1. `gh auth token`
2. `[github].token` in config
3. `GITHUB_TOKEN` environment variable

## Open Command Templates

`open_command` runs through `bash -c` when `ezgit` opens a resolved repo/worktree path and can use:

- `$host`
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

- Manual `ezgit cache refresh` always performs an incremental API refresh.
- `--force` performs a full refresh (fetch all pages) instead of incremental.
- Use `ezgit cache refresh --ttl <duration>` to set a custom TTL for that refresh run.
- `ezgit` (no args) and picker-based flows use cached repos immediately when available and refresh stale caches in the background (TTL-aware).

## Zoxide Integration

Cloning and adding worktrees register the checkout path with `zoxide add`.
This is best-effort; zoxide failures do not fail the command.

## Development

Run the validation suite used by CI and the landing policy:

```bash
test -z "$(gofmt -l ./cmd ./internal)"
go build .
go vet ./...
go test ./...
nix build .#
```

Completed changes are submitted through [git-land](https://github.com/kirksw/git-land):

```bash
nix run github:kirksw/git-land#land --json
```

The committed `land.yaml` policy creates a pull request, waits for CI, and merges automatically after all required checks pass.

```bash
go test ./...
nix run .# -- --help
```

Measure startup-related performance:

```bash
./scripts/measure-startup.sh
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
