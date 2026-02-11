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

merge_base="$(git merge-base "$BASE_REF" HEAD)"
changed_files="$(git diff --name-only "$merge_base"...HEAD)"

if [[ -z "$changed_files" ]]; then
  echo "No changes detected against '$BASE_REF'."
  exit 0
fi

version_changed=false
release_notes_changed=false
code_changed=false

while IFS= read -r file; do
  [[ -z "$file" ]] && continue

  case "$file" in
    "$VERSION_FILE")
      version_changed=true
      ;;
    "$RELEASE_NOTES_FILE")
      release_notes_changed=true
      ;;
    ".github/"* | ".githooks/"* | "docs/"* | "scripts/"* | "README.md" | ".gitignore" | *.md | *_test.go)
      ;;
    *)
      code_changed=true
      ;;
  esac
done <<< "$changed_files"

if $version_changed; then
  if ! $release_notes_changed; then
    echo "::error::Version changed but '$RELEASE_NOTES_FILE' was not updated."
    exit 1
  fi

  if ! grep -Eq "^## ${current_version} - " "$RELEASE_NOTES_FILE"; then
    echo "::error::'$RELEASE_NOTES_FILE' must include a heading for version '${current_version}'."
    exit 1
  fi

  base_version="$(git show "${BASE_REF}:${VERSION_FILE}" 2>/dev/null | tr -d '[:space:]' || true)"
  if [[ -n "$base_version" ]]; then
    if [[ "$base_version" == "$current_version" ]]; then
      echo "::error::Version file changed but version stayed '${current_version}'."
      exit 1
    fi

    highest_version="$(printf '%s\n%s\n' "$base_version" "$current_version" | sort -V | tail -n 1)"
    if [[ "$highest_version" != "$current_version" ]]; then
      echo "::error::Version must increase (base: '$base_version', current: '$current_version')."
      exit 1
    fi
  fi
fi

if $code_changed && ! $version_changed; then
  echo "::error::Code changes detected but '$VERSION_FILE' was not updated."
  exit 1
fi

echo "Version guard passed."
