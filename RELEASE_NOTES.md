# Release Notes

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
