---
name: ezgit
description: Manage GitHub repositories and isolated Git worktrees with ezgit. Use when an agent needs to discover cached repositories, inspect local clone state, clone a repository, create or prune worktrees, or resolve a repository/worktree path before starting work.
---

# ezgit

Use ezgit for repository discovery and worktree lifecycle operations.
Prefer scriptable subcommands over the interactive no-argument picker.

## Discover repositories

```bash
ezgit list orgs
ezgit list repos
ezgit list repos --local
ezgit describe owner/repo
ezgit list worktrees owner/repo
```

`ezgit describe` prints JSON with the repository path, clone state, layout, default branch, and worktrees.
Use it before assuming that a repository is cloned or uses worktree layout.

## Clone repositories

Create a regular clone:

```bash
ezgit clone owner/repo
```

Create bare metadata plus a worktree for the default branch:

```bash
ezgit clone --worktree owner/repo
# --bare is an alias for --worktree
ezgit clone --bare owner/repo
```

Non-interactive `--worktree` and `--bare` clones do not create a `review` worktree.
The interactive planner can select additional worktrees.

## Manage worktrees

Create a feature worktree from the repository default branch:

```bash
ezgit worktree add owner/repo feature-name
# short alias
ezgit wt add owner/repo feature-name
```

The compatibility command `ezgit add owner/repo feature-name` performs the same operation.

Preview worktrees whose newest file modification is older than 14 days:

```bash
ezgit worktree prune owner/repo
ezgit wt prune owner/repo
```

Remove the listed worktrees, including dirty worktrees, while keeping their branches:

```bash
ezgit wt prune owner/repo --apply
```

Use `--older-than 30d` to change the age threshold.
`main` and `master` are always protected.

List worktrees after changes:

```bash
ezgit list worktrees owner/repo
```

## Open or prepare a worktree

Ensure a named worktree exists and run the configured open command:

```bash
ezgit open owner/repo feature-name
```

Prepare without running the open command:

```bash
ezgit --no-open owner/repo feature-name
```

## Agent isolation

Run a worker agent with its working directory set to its dedicated worktree, not the bare repository metadata directory.
A working directory is not a security boundary: an agent can still modify accessible sibling paths unless the runtime sandbox or filesystem permissions prevent it.
Use the bare repository only as a coordination location when the coordinator does not edit files.

## Safety

- Inspect state before modifying it with `ezgit describe owner/repo`.
- Run `ezgit wt prune owner/repo` without `--apply` first and review every candidate.
- Treat `--apply` as destructive because it force-removes dirty worktrees.
- Do not assume the default branch is named `main`.
- Do not use the no-argument TUI in automation.
