# Dev-Version + Tag Release Patterns

## 1) Release Workflow Trigger and Validation

Use tag push trigger:

```yaml
on:
  push:
    tags:
      - "v*.*.*"
```

Validate tag and derive version:

```bash
tag="${GITHUB_REF_NAME}"
regex='^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'
[[ "$tag" =~ $regex ]] || exit 1
version="${tag#v}"
```

## 2) Inject Release Version Into Binary

```bash
ldflags="-X github.com/<org>/<repo>/internal/version.Value=${version}"
go build -trimpath -ldflags "$ldflags" -o dist/ezgit .
```

## 3) Extract Release Notes From RELEASE_NOTES.md

Expected heading format:

```text
## 0.0.7 - 2026-02-26
```

Extraction pattern:

```bash
awk -v version="$version" '
  $0 ~ "^## " version " - " {in_section=1; next}
  $0 ~ /^## / && in_section {exit}
  in_section {print}
' RELEASE_NOTES.md > dist/RELEASE_BODY.md

grep -q '[^[:space:]]' dist/RELEASE_BODY.md
```

## 4) Publish GitHub Release

```yaml
- uses: softprops/action-gh-release@v2
  with:
    tag_name: ${{ needs.prepare.outputs.tag }}
    name: ${{ needs.prepare.outputs.tag }}
    prerelease: ${{ needs.prepare.outputs.prerelease }}
    body_path: dist/RELEASE_BODY.md
```

## 5) PR Guard Policy

Keep this policy in PR CI:

- `internal/version/VERSION` must exist.
- Version value must be valid semver and contain `-dev`.
- Do not fail code changes for not bumping version.

Avoid this policy in PR CI:

- Requiring version increase for every code PR.
- Requiring release notes update for every code PR.
- Enforcing tag == VERSION file.
