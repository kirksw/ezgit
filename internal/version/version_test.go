package version

import "testing"

func TestResolveValuePrefersInjectedVersion(t *testing.T) {
	got := resolveValue("0.0.8", "0.0.0-dev")
	if got != "0.0.8" {
		t.Fatalf("resolveValue()=%q, want %q", got, "0.0.8")
	}
}

func TestResolveValueFallsBackToEmbeddedVersion(t *testing.T) {
	got := resolveValue("", "0.0.0-dev\n")
	if got != "0.0.0-dev" {
		t.Fatalf("resolveValue()=%q, want %q", got, "0.0.0-dev")
	}
}
