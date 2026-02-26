# Release Notes

## 0.0.9 - 2026-02-26

Release version injection fix:
- Updated version resolution so link-time injected values (`-ldflags -X .../internal/version.Value=<version>`) take precedence over embedded `internal/version/VERSION`.
- Added regression tests for injected-vs-embedded version selection.

## 0.0.8 - 2026-02-26

Packaging/version output fix:
- Updated Nix flake build settings to inject a release version at link time for non-local flake builds, preventing release-tag builds from reporting `0.0.0-dev` in `ezgit version`.

## 0.0.7 - 2026-02-26

Unified command interface and worktree UX improvements:
- Collapsed primary flows into `ezgit [owner/repo] [worktree]` with `--no-open`, removing separate clone/open/add/connect/tui command surface from help.
- Removed config-driven default worktree mode so regular clones remain possible unless worktree intent is explicit.
- Added dynamic worktree planning UI for clone/convert to add or remove multiple custom worktrees inline.
- Updated docs and config examples for `open_command`, including fixed `sesh` usage with absolute path and a tmux session example using `org/repo[/worktree]` names.

## 0.0.6 - 2026-02-26

Clone/worktree reliability and cache behavior fixes:
- Fixed clone behavior when destination already exists, including interactive open-or-convert handling for existing non-worktree repos in worktree mode.
- Added zoxide path registration for repo roots and worktrees created by clone/add/convert flows.
- Fixed custom worktree branch selection UI to support long branch lists with visible scrolling/windowing.
- Fixed cache TTL handling so incremental refresh skips remote fetches while cache is still fresh.
- Updated README command and behavior docs to match current functionality.

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
