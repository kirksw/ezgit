# AGENTS.md
Guidance for coding agents operating in this repository.

## Repository Overview
- Language: Go (`go 1.24.2`)
- Module: `github.com/kirksw/ezgit`
- Entry point: `main.go` -> `cmd.Execute()`
- CLI: `cobra`
- TUI: `bubbletea`, `bubbles`, `lipgloss`
- Main folders:
  - `cmd/` command handlers
  - `internal/cache` cache + TTL
  - `internal/config` config parsing + token resolution
  - `internal/git` clone/convert/worktree helpers
  - `internal/github` API client/types
  - `internal/ui` TUI/fuzzy selection
  - `internal/version` embedded version value

## Build / Run / Test Commands

### Build
```bash
go build .
```

### Run locally
```bash
go run . --help
```

### Run with nix
```bash
nix run .# -- --help
```

### Run all tests
```bash
go test ./...
```

### Run a single test (most important)
```bash
go test ./cmd -run TestRefreshReposIncrementallySkipsFetchWhenCacheFresh
```

### Run a single test in another package
```bash
go test ./internal/ui -run TestFeaturePromptVisibleRangeClampsNearEdges
```

### Run a specific subtest
```bash
go test ./cmd -run 'TestExtractRepoFullName/ssh_url'
```

### Helpful test flags
```bash
go test ./cmd -run TestName -v
go test ./cmd -run TestName -count=1
```

### Focused package runs
```bash
go test ./cmd
go test ./internal/cache
go test ./internal/git
go test ./internal/ui
```

### Formatting / lint baseline
- No dedicated lint tool is configured in this repo today.
- Baseline quality gate: `gofmt` clean + `go test ./...` passes.
- Format touched files:
```bash
gofmt -w <changed-files>
```
- Optional broad format pass:
```bash
gofmt -w ./cmd ./internal
```

### Local hooks
```bash
./scripts/install-hooks.sh
```
- Pre-push hook runs:
```bash
scripts/check-version-bump.sh origin/main
```

## CI / Release Behavior
- PR workflow: `.github/workflows/version-guard.yml`
- Guard script: `scripts/check-version-bump.sh`
- Current guard intent:
  - `internal/version/VERSION` must be valid semver
  - value must contain `-dev`
  - no per-PR version bump required
- Release workflow: `.github/workflows/release.yml`
- Release trigger: push tag `vX.Y.Z`
- Release build injects version at link time:
  - `-ldflags "-X github.com/kirksw/ezgit/internal/version.Value=<version>"`
- Release notes body is extracted from `RELEASE_NOTES.md` heading:
  - `## <version> - <date>`

## Code Style Guidelines

### Imports and organization
- Use standard Go import grouping:
  1) stdlib
  2) blank line
  3) external/internal packages
- Keep command definition and command execution in same `cmd/*.go` file when practical.
- Keep helper funcs near callers unless reused broadly.

### Formatting and comments
- Always leave files `gofmt`-clean.
- Prefer simple control flow and guard clauses over deep nesting.
- Keep comments minimal; add them only for non-obvious behavior.
- Keep CLI output concise and user-facing.

### Types and naming
- Exported symbols: `PascalCase`.
- Internal symbols: `camelCase`.
- Use descriptive names (`defaultBranch`, `selectedWorktree`, `metadataPath`).
- Boolean names should read naturally (`isInteractive`, `hasWorktrees`, `cancelled`).
- Introduce constants for repeated literal values and menu options.

### Error handling
- Return errors; avoid panic in normal flows.
- Wrap errors with operation context via `%w`.
- Preferred pattern:
```go
return fmt.Errorf("failed to load config: %w", err)
```
- For non-fatal operations, follow existing style: print warning and continue.

### CLI behavior conventions
- Prefer `RunE` over `Run` for commands that can fail.
- Preserve existing interactive cancel semantics (`esc`, `ctrl+c`).
- Keep non-interactive paths deterministic; do not block on prompts.
- Reuse existing helpers before adding duplicate command-specific logic.

### Paths and external commands
- Use `filepath.Join`, `filepath.Clean`, and `filepath.Abs` for filesystem paths.
- Normalize user input with `strings.TrimSpace` where needed.
- Wrap external command failures (`git`, `gh`, `zoxide`, `bash`) with clear context.
- Avoid destructive operations unless explicitly required.

### Testing conventions
- Prefer table-driven tests for parser/validator logic.
- Add regression tests for bug fixes in touched package.
- Cover success + failure branches.
- Keep test names behavior-focused (`TestXDoesY`).

## Repository-Specific Policy Notes
- Keep `internal/version/VERSION` as a dev value during normal development (for example `0.0.0-dev`).
- Do not bump version on every commit.
- Release version is derived from pushed release tag.
- When release behavior changes, keep `README.md` and `RELEASE_NOTES.md` aligned.

## Project Agent Skills
- Store project-local agent skills at `.agents/skills/<name>/SKILL.md`.
- Keep a discoverable index at `.agents/skills/INDEX.md`.
- Current project skill:
  - `.agents/skills/release-skill/SKILL.md`

## Cursor / Copilot Rules Status
At the time this file was created, these files were not present:
- `.cursor/rules/`
- `.cursorrules`
- `.github/copilot-instructions.md`
If they are added later, treat them as higher-priority repo guidance and update this file.
