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
- `ezgit open`: open local repos using `open_command`.
- `ezgit connect [session]`: connect to tmux session (fuzzy-select if omitted).
- `ezgit convert [path]`: convert local repo to bare metadata in `.git` + worktrees.
- `ezgit tui`: unified one-page TUI for clone/open/connect.
- `ezgit cache refresh|list|search|invalidate`: cache operations.

## Unified TUI (`ezgit tui`)

Single searchable list page with mode switching:

- `tab` / `left` / `right`: switch mode (`clone`, `open`, `connect`) and replace list contents.
- `enter`: run selected item in current mode.
- `w`: toggle clone worktree mode (clone mode only).
- `c`: convert selected local repo (open mode only).
- `esc` / `ctrl+c`: cancel.

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

## Cache Behavior

- Cache refresh is incremental by default.
- `--force` performs a full refresh.
- `clone`, `open`, and `tui` trigger automatic cache refresh attempts before listing repos.

## Development

```bash
go test ./...
nix run .# -- --help
```

## License

MIT
