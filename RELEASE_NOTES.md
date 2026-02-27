# Release Notes

## 0.0.12 - 2026-02-27

Warm-path performance and open-flow improvements:
- Reduced interactive open latency by removing redundant worktree checks and switching open-command shell execution from `bash -lc` to `bash -c`.
- Added in-memory `GetAllRepos` snapshot caching with file-signature validation and TTL-aware expiry, dramatically reducing repeated cache deserialization and allocations.
- Wired lazy worktree loading into `ezgit open` picker so worktrees resolve on demand for selected local repos.
- Pre-seeded default-branch lookup from already loaded repositories in fuzzy/open/root flows to avoid extra cache scans.
- Added regression coverage for cache snapshot invalidation/expiry, open worktree loader behavior, and default-branch lookup seeding.

## 0.0.11 - 2026-02-27

Startup performance and responsiveness improvements:
- Reduced no-arg startup latency by loading cached repositories immediately and running stale cache refresh in the background for picker-based flows.
- Added lazy worktree loading in the open picker so worktrees are resolved on demand for selected local repositories instead of eagerly scanning all local repos.
- Parallelized and streamlined startup prep (local repo detection, tmux/opened-state mapping, cache-refresh targets) and removed redundant default-branch/cache lookups.
- Added startup benchmark coverage and helper scripts to measure key hot paths (`fuzzy` prep, worktree discovery, cache deserialization, local repo detection), plus expanded regression tests for cache/default-branch/worktree-loading behavior.

## 0.0.10 - 2026-02-26

Picker and worktree UX refinements:
- Improved the no-arg `ezgit` picker with a richer split-pane layout and inline worktree creation from the right pane.
- Added `all/local/opened` scope cycling with clearer status badges (`[local]`, `[open]`) and tmux session matching improvements for open detection.
- Updated worktree list behavior to hide repo root, highlight opened worktrees, and keep keybind guidance in a bottom status box.
- Added regression tests for picker state, tmux session matching, and worktree inline parsing.

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
