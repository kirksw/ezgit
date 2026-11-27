---
name: ezgit
description: Discover GitHub repos, clone repositories, and create isolated Git worktrees for coding agents.
---

# Using ezgit

Use `ezgit describe <repo>` for the local path, layout, and registered worktree names.
Use `ezgit list repos` to discover cached GitHub repositories and `ezgit list worktrees <repo>` to list linked checkouts.
Repo arguments can be `owner/repo` for GitHub, `host/owner/repo`, or a Git URL.

## Prepare checkouts

```bash
ezgit clone owner/repo
base_commit=$(git -C ~/git/github.com/owner/repo rev-parse HEAD)
ezgit wt add owner/repo task-agent --base "$base_commit" --json
```

If the root is customized, use the `path` from `ezgit describe <repo>` instead.
The default primary checkout is `~/git/<host>/<owner>/<repo>`.
New linked worktrees live at `~/git/.worktrees/<host>/<owner>/<repo>/<name>`.
Use the `path` returned in JSON as the worker's absolute working directory; `branch` and `base_commit` identify the new branch and its starting commit.
The name must be unique and contain no slashes.
A duplicate branch or path is an error, not permission to reuse another worker's checkout.
Without `--base`, ezgit uses a locally known default branch; it does not fetch.

For parallel writers, pin a base commit and create a worktree per agent.
Assign independent scopes, inspect their commits, then integrate and run combined verification before submitting from the parent agent.
Do not put a worker in the user's primary checkout or move an existing checkout to fit the default layout.

## Open and clean up

`ezgit open <repo> [worktree]` launches an existing checkout and does not create it.
Agents should use the absolute path directly rather than launching another session.
After integrating a worker's commits, use `git worktree remove <path>` for explicit cleanup.
Keep dirty and failed worktrees for inspection.
Check for ignored files before removing a worktree directly with Git.
`ezgit wt prune <repo>` previews stale linked checkouts; `--apply` skips worktrees with local changes, untracked files, or ignored files, and keeps branches.
