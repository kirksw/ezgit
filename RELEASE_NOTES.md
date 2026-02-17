# Release Notes

## 0.0.5 - 2026-02-17

Worktree command flow improvements:
- Added `ezgit add <repo> <worktreename>` for creating a worktree in an existing clone.
- Updated `ezgit clone <repo> <worktreename>` to run non-interactively: it creates default worktrees plus the specified worktree, or just adds the worktree when already cloned.
- Updated `ezgit open <repo> <worktree-name>` to clone/add as needed and then attach via `sesh connect`.

## 0.0.3 - 2026-02-11

Versioning and release safety improvements:
- Added a PR version guard workflow that enforces version bumps for code changes.
- Added a release tag/version consistency check to prevent mismatched releases.
- Added a local pre-push hook option to catch missing version bumps before push.

## 0.0.1 - 2026-02-09

Initial release of `ezgit`.

Highlights:
- Repository clone, open, convert, cache, connect, and tui command flows.
- Worktree-first workflows for clone/open/convert.
- Configurable open command with path/repo placeholders.
- Incremental cache refresh behavior.
- Nix flake support for build and run.
