# ezgit Agents

This document describes the agents used in the development of ezgit, a Git repository management CLI tool written in Go.

## Overview

ezgit uses three specialized agents that work together to deliver high-quality features:

- **Planning Agent** - Architectural decisions and feature planning
- **Coding Agent** - Implementation and code generation
- **QA Agent** - Testing, review, and security validation

## Planning Agent

**Role:** Handles architectural decisions, feature planning, and project coordination

### Responsibilities

- Design interfaces and data structures before coding begins
- Review code for adherence to architectural decisions
- Ensure documentation and testing completeness after implementation
- Provide guidance on "why" behind decisions

### Key Questions

When planning, the agent considers:

- What's the idiomatic Go way to handle this?
- What edge cases should we consider for this git operation?
- How can we make this code more testable?
- Should this be in `internal/` or `pkg/`?
- What interface design would make this more flexible?

### Design Principles

- Keep functions small and focused (< 50 lines when possible)
- Prefer composition over inheritance
- Use interfaces for testability
- Follow standard Go conventions (gofmt, golint)
- Minimize external dependencies

## Coding Agent

**Role:** Implements features, writes code, and handles development tasks

### Responsibilities

- Write idiomatic Go code following project conventions
- Implement features based on Planning Agent guidance
- Generate code patterns (CLI boilerplate, table-driven tests, interfaces)
- Update documentation for new features
- Refactor code for improved maintainability

### Code Style & Standards

- Follow standard Go conventions (gofmt, golint)
- Use meaningful variable and function names
- Keep functions small and focused (< 50 lines when possible)
- Write idiomatic Go code
- Use error wrapping with `fmt.Errorf` and `%w`
- Return errors, don't panic (except in truly exceptional cases)

### Error Handling

- Return descriptive errors at each layer
- Wrap errors with context: `fmt.Errorf("failed to fetch repo: %w", err)`
- Distinguish between user errors and system errors
- Exit codes: 0 (success), 1 (user error), 2 (system error)

### Feature Development Workflow

1. Start with interface definitions
2. Implement core logic in `internal/` packages
3. Add CLI commands in `cmd/`
4. Write unit tests alongside code
5. Update README.md with new commands
6. Add integration tests if needed

### Documentation Requirements

Every public function should have godoc comments:

```go
// FunctionName does X and returns Y.
// It returns an error if Z occurs.
func FunctionName(param Type) (ReturnType, error) {
    // implementation
}
```

## QA Agent

**Role:** Ensures code quality through testing, review, and security validation

### Responsibilities

- Write comprehensive tests for all business logic
- Review code for bugs, security issues, and best practices
- Validate testing coverage (>80% target)
- Check for race conditions and performance issues

### Testing Strategy

- Unit tests for all business logic
- Mock git operations using interfaces
- Integration tests with real git repositories in temp directories
- Test error conditions and edge cases
- Aim for >80% code coverage

### Testing Best Practices

- Use table-driven test patterns
- Use test fixtures in `testdata/` directories
- Implement proper setup/teardown for git repositories
- Mock external dependencies
- Test both success and failure paths

### Code Review Checklist

- ✅ Check for proper error handling
- ✅ Identify potential race conditions
- ✅ Suggest idiomatic Go improvements
- ✅ Flag security concerns (command injection, path traversal)
- ✅ Verify adherence to coding standards

### Security Considerations

- Sanitize user input before passing to git commands
- Avoid shell injection vulnerabilities
- Validate file paths to prevent traversal attacks
- Don't log sensitive information (tokens, passwords)
- Use `os/exec` with argument arrays, not shell strings

### Testing Commands

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/git

# Run with race detector
go test -race ./...
```

## Agent Workflow

### Typical Feature Development

1. **Planning Phase** - Planning Agent designs the feature
   - Define interfaces and data structures
   - Consider edge cases and error handling
   - Plan testing strategy

2. **Implementation Phase** - Coding Agent builds the feature
   - Implement core logic in `internal/` packages
   - Add CLI commands in `cmd/`
   - Write unit tests alongside code

3. **QA Phase** - QA Agent validates quality
   - Review code for bugs and security issues
   - Run tests and check coverage
   - Suggest improvements

### Coordination

The agents work collaboratively:

- Planning provides the "why" and architecture
- Coding provides the "what" and implementation
- QA ensures the "how" meets quality standards

## Project Structure

```
.
├── cmd/              # CLI command implementations
├── internal/         # Private application code
│   ├── git/         # Git operations wrapper
│   ├── config/      # Configuration management
│   ├── cache/       # Caching layer for GitHub API
│   ├── github/      # GitHub API client
│   └── utils/       # Utility functions
├── nix/             # Nix flake and launchd service
├── docs/agents/     # Agent session logs
├── main.go          # Application entry point
└── go.mod           # Go modules file
```

## Dependencies Management

- Minimize external dependencies
- Prefer standard library when possible
- Document why each dependency is needed
- Keep `go.mod` clean and updated
- Use `go mod tidy` regularly

### Current Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/BurntSushi/toml` - TOML configuration parser
- `github.com/inconshreveable/mousetrap` - Windows compatibility
- `github.com/spf13/pflag` - CLI flag parsing

## Build & Release

- Semantic versioning (v0.1.0, v1.0.0, etc.)
- Cross-compilation for Linux, macOS, Windows
- Nix flake for reproducible builds
- Launchd service integration (macOS)
- Changelog for each version

## Agent Sessions

Agent session logs are stored in `docs/agents/{yyyy-mm-dd}/` for historical reference and analysis.

## Common Patterns

### Command Structure

```go
type Command struct {
    Name        string
    Description string
    Flags       []Flag
    Action      func(*Context) error
}
```

### Git Wrapper Interface

```go
type GitRepo interface {
    Clone(url, path string) error
    Pull(path string) error
    Status(path string) (*Status, error)
    // ... other operations
}
```

### Configuration Structure

```go
type Config struct {
    Organizations OrganizationConfig `toml:"organizations"`
    Repos         RepoConfig         `toml:"repos"`
    GitHub        GitHubConfig       `toml:"github"`
    Git           GitConfig          `toml:"git"`
}

type GitConfig struct {
    CloneDir string `toml:"clone_dir"`
    Worktree bool   `toml:"worktree"`
    SeshOpen bool   `toml:"sesh_open"`
}
```

## Future Considerations

### Agent Improvements

- Enhanced coordination between agents
- Better context sharing across sessions
- Automated test generation
- Performance benchmarking integration

## Contributing

When contributing to ezgit:

1. Understand the agent roles and workflow
2. Follow the code style and standards defined by the Coding Agent
3. Ensure all tests pass (QA Agent requirements)
4. Consider architectural implications (Planning Agent guidance)
5. Document your changes (Documentation requirements)

## License

MIT - See LICENSE file for details.
