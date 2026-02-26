#!/usr/bin/env bash

set -euo pipefail

BASE_REF="${1:-origin/main}"
VERSION_FILE="internal/version/VERSION"
RELEASE_NOTES_FILE="RELEASE_NOTES.md"

if ! git rev-parse --verify "$BASE_REF" >/dev/null 2>&1; then
  echo "::error::Base ref '$BASE_REF' was not found."
  exit 1
fi

if [[ ! -f "$VERSION_FILE" ]]; then
  echo "::error::Missing version file '$VERSION_FILE'."
  exit 1
fi

if [[ ! -f "$RELEASE_NOTES_FILE" ]]; then
  echo "::error::Missing release notes file '$RELEASE_NOTES_FILE'."
  exit 1
fi

current_version="$(tr -d '[:space:]' < "$VERSION_FILE")"
semver_regex='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'
if [[ ! "$current_version" =~ $semver_regex ]]; then
  echo "::error::Version '$current_version' in '$VERSION_FILE' is not valid semver."
  exit 1
fi

if [[ "$current_version" != *-dev* ]]; then
  echo "::error::Version '$current_version' must be a dev version (for example: 0.0.0-dev)."
  exit 1
fi

merge_base="$(git merge-base "$BASE_REF" HEAD)"
changed_files="$(git diff --name-only "$merge_base"...HEAD)"

if [[ -z "$changed_files" ]]; then
  echo "No changes detected against '$BASE_REF'."
  exit 0
fi

release_notes_changed=false

while IFS= read -r file; do
  [[ -z "$file" ]] && continue

  case "$file" in
    "$RELEASE_NOTES_FILE")
      release_notes_changed=true
      ;;
  esac
done <<< "$changed_files"

if $release_notes_changed; then
  if ! grep -Eq '^## [0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)? - ' "$RELEASE_NOTES_FILE"; then
    echo "::error::'$RELEASE_NOTES_FILE' must contain at least one semver heading (for example: '## 0.0.7 - 2026-02-26')."
    exit 1
  fi
fi

echo "Version guard passed (dev-version policy)."
