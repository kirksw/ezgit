# ezgit - Easy GitHub Repository Management CLI

A powerful CLI tool for managing GitHub repositories with support for cloning, bare conversions, worktrees, organization caching, and fuzzy search UI.

## Features

- **Clone Repositories**: Clone repos using SSH with optional bare mode
- **Bare Convert**: Convert existing repos to bare and create worktrees for branches
- **Organization Caching**: Cache org repos for fast search and completion
- **SSH Key Management**: Automatic SSH key validation
- **Config File Support**: Define organizations and private repos in `config.toml`
- **Fuzzy Search UI**: Interactive TUI for selecting repositories
- **Worktree Selection**: Automatically prompts for worktree selection when opening repos with multiple worktrees
- **Sesh Integration**: Open repos in sesh with automatic cloning if needed

## Installation

```bash
go install github.com/kirksw/ezgit@latest
```

## Configuration

The CLI searches for `config.toml` in the following locations (in order):
1. Command-line flag: `--config /path/to/config.toml`
2. Current directory: `./config.toml`
3. User config directory: `~/.config/ezgit/config.toml`
4. Home directory: `~/.ezgit.toml`

Or specify a custom path with:
```bash
ezgit --config /path/to/config.toml cache refresh
```

### Example: Config in user directory

```bash
mkdir -p ~/.config/ezgit
cp config.toml.example ~/.config/ezgit/config.toml
edit ~/.config/ezgit/config.toml
```

### Example: Config in home directory

```bash
cp config.toml.example ~/.ezgit.toml
edit ~/.ezgit.toml
```

Create a `config.toml` file:

```toml
[organizations]
orgs = [
    "facebook",
    "google",
    "kubernetes",
]

[repos]
private = [
    "my-org/secret-repo",
]

[git]
clone_dir = "~/git/github.com"
worktree = false
sesh_open = false
```

## GitHub Authentication

The tool authenticates with GitHub using the following methods (in order of preference):

1. **GitHub CLI** - Uses `gh auth token` (recommended)
2. **Config file** - Set `[github] token = "ghp_your_token_here"`
3. **Environment variable** - Set `GITHUB_TOKEN=ghp_your_token_here`

### Recommended: GitHub CLI

Install and authenticate once:
```bash
# Install gh CLI
brew install gh  # macOS
# or visit https://cli.github.com/

# Authenticate
gh auth login
```

The tool will automatically use your gh token for cache operations.

### Alternative: Manual Token

Set your GitHub token in config or environment:
```bash
export GITHUB_TOKEN=ghp_your_token_here
```

## Usage

### Clone a Repository

Clone a public repo:
```bash
ezgit clone owner/repo
```

Clone a specific branch:
```bash
ezgit clone owner/repo -b develop
```

Create a bare repository:
```bash
ezgit clone owner/repo --bare
```

Shallow clone:
```bash
ezgit clone owner/repo --depth 1
```

### Open a Repository

Open a locally cloned repository in sesh:
```bash
ezgit open
```

- Automatically prompts for repository selection from cached repos
- If repository has multiple worktrees, prompts for worktree selection
- If repository is not locally cloned, automatically clones it first

### Fuzzy Search UI

When running `ezgit clone` or `ezgit open` without arguments, an interactive TUI is displayed:

**Main Page Controls:**
- `up/down`: Navigate through repositories
- `tab`: Toggle local filter
- `ctrl+s`: Open settings page
- `enter`: Clone/open selected repository
- `esc`: Cancel

**Settings Page Controls:**
- `up/down`: Navigate between options
- `space`: Toggle option (worktree mode, sesh open mode)
- `enter/ctrl+s`: Back to repositories
- `esc`: Cancel

**Worktree Selection Page (shown when opening a repo with multiple worktrees):**
- `up/down`: Navigate through available worktrees
- `enter`: Select worktree and open in sesh
- `esc`: Back to repositories

### Convert to Bare with Worktrees

Convert an existing repo to bare and create worktrees:
```bash
ezgit bare-convert ~/git/github.com/org/repo
```

Create worktrees for specific branches:
```bash
ezgit bare-convert ~/git/github.com/org/repo --worktree main --worktree develop
```

Create worktrees for all branches:
```bash
ezgit bare-convert ~/git/github.com/org/repo --all-worktrees
```

### Cache Management

Refresh cache for all organizations:
```bash
ezgit cache refresh
```

Refresh cache for specific org:
```bash
ezgit cache refresh facebook
```

List all cached organizations:
```bash
ezgit cache list
```

Search cached repositories:
```bash
ezgit cache search "react"
```

Invalidate cache:
```bash
ezgit cache invalidate
```

## Worktree Structure

When converting a repo to bare with worktrees:

```
~/git/github.com/org/repo/          # Bare repository
~/git/github.com/org/repo/main/     # Worktree for main branch
~/git/github.com/org/repo/develop/  # Worktree for develop branch
```

## Sesh Integration

When using `ezgit open` or selecting "sesh open mode" in the fuzzy search UI:

- If the repository is locally cloned:
  - If it has multiple worktrees, prompts for worktree selection
  - Opens the selected worktree in sesh (or main if no worktrees exist)
- If the repository is not locally cloned:
  - Automatically clones the repository first
  - Then opens it in sesh

## SSH Keys

The tool uses SSH for cloning. By default, it uses `~/.ssh/id_rsa`.

Specify a custom key:
```bash
ezgit clone owner/repo --key-path ~/.ssh/custom_key
```

## Clone Directory

Configure a default directory for cloned repositories in `config.toml`:

```toml
[git]
clone_dir = "~/git/github.com"
```

When configured, clones will default to this directory:
```bash
ezgit clone facebook/react
# Clones to ~/git/github.com/react
```

Override with `-d` flag:
```bash
ezgit clone facebook/react -d ~/projects/react
```

## Exit Codes

- `0`: Success
- `1`: User error (invalid input, repo not found, authentication)
- `2`: System error (git failure, network, cache corruption)

## Examples

Clone with specific destination:
```bash
ezgit clone facebook/react -d ~/projects/react
```

Convert repo and create worktrees for all branches:
```bash
ezgit bare-convert ~/git/github.com/facebook/react --all-worktrees
```

Search for repositories in cache:
```bash
ezgit cache search "react"
```

## Nix Installation

Install using Nix flakes:
```bash
nix profile install github:kirksw/ezgit
```

Or build from source:
```bash
nix build
# Binary available at ./result/bin/ezgit
```

### Launchd Service (macOS)

The package includes a launchd service for automatic cache refresh.

Install the service:
```bash
ezgit-service install
```

Check service status:
```bash
ezgit-service status
```

Manually trigger refresh:
```bash
ezgit-service refresh
```

Uninstall service:
```bash
ezgit-service uninstall
```

The service runs automatically every 24 hours, refreshing all configured organization caches.

## Development

Build locally:
```bash
go build -o ezgit .
```

Run tests:
```bash
go test ./...
```

## License

MIT
