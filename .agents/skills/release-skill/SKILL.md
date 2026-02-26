---
name: dev-tagged-release-ci
description: Configure and maintain a dev-version plus tag-triggered GitHub release workflow for Go CLIs. Use when a repository should keep internal/version/VERSION on a development value, release by pushing vX.Y.Z tags, inject release versions at link time, and publish GitHub releases from RELEASE_NOTES.md sections.
---

# Dev-Tagged Release CI

Implement this flow:

1. Keep `internal/version/VERSION` on a dev value (for example `0.0.0-dev`).
2. Trigger releases from pushed tags like `v0.0.7`.
3. Build binaries with release version injected at link time.
4. Publish GitHub release body from `RELEASE_NOTES.md` section `## <version> - <date>`.
5. Keep PR guard focused on dev-version policy, not per-PR version bumping.

## Required Repository Changes

- **Release workflow**: Use a tag-push workflow in `.github/workflows/release.yml`.
- **Build flags**: Use `go build -ldflags "-X <module>/internal/version.Value=<version>"`.
- **Release notes extraction**: Extract matching section from `RELEASE_NOTES.md`; fail if missing/empty.
- **Version guard**: Ensure `internal/version/VERSION` remains a `*-dev*` semver and do not require version bump per code PR.

## Implementation Notes

- Derive `version` from tag: `version="${GITHUB_REF_NAME#v}"`.
- Validate tags with semver regex: `^vMAJOR.MINOR.PATCH(-...)?(+...)?$`.
- Set prerelease automatically when version contains `-`.
- Do not couple release tag validation to `internal/version/VERSION`; release binaries should use tag-derived version.

## Verification

Run these checks before concluding:

- `go test ./...`
- Confirm `internal/version/VERSION` is still a dev value.
- Confirm `.github/workflows/release.yml` is tag-triggered and references `RELEASE_NOTES.md` extraction.

## Reference

For concrete patterns and snippets, read `references/patterns.md`.
