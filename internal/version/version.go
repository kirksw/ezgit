package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var raw string

// Value is the semantic version of the running binary.
//
// It can be overridden at link time with:
// -ldflags "-X github.com/kirksw/ezgit/internal/version.Value=<version>"
//
// If not injected, it falls back to the embedded VERSION file.
var Value string

func init() {
	Value = resolveValue(Value, raw)
}

func resolveValue(injected string, embedded string) string {
	injected = strings.TrimSpace(injected)
	if injected != "" {
		return injected
	}
	return strings.TrimSpace(embedded)
}
