package version

import (
	_ "embed"
	"strings"
)

// Value is the semantic version read from the VERSION file.
//
//go:embed VERSION
var raw string

var Value = strings.TrimSpace(raw)
