#!/usr/bin/env bash

set -euo pipefail

VERSION_FILE="internal/version/VERSION"
tag="${1:-}"

if [[ -z "$tag" ]]; then
  echo "::error::Missing tag argument. Usage: scripts/check-tag-version.sh vX.Y.Z"
  exit 1
fi

if [[ ! -f "$VERSION_FILE" ]]; then
  echo "::error::Missing version file '$VERSION_FILE'."
  exit 1
fi

version="$(tr -d '[:space:]' < "$VERSION_FILE")"
expected_tag="v${version}"

if [[ "$tag" != "$expected_tag" ]]; then
  echo "::error::Tag '$tag' does not match '$VERSION_FILE' (${expected_tag})."
  exit 1
fi

echo "Tag matches version file: ${expected_tag}"
