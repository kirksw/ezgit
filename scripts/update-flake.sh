#!/usr/bin/env bash

set -euo pipefail

FAKE_HASH="sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FLAKE_FILE="${ROOT_DIR}/flake.nix"
BUILD_TARGET="."
VERIFY=true
ALLOW_UNTRACKED=false

usage() {
  cat <<'EOF'
Usage: scripts/update-flake.sh [--skip-verify] [--allow-untracked] [build-target]

Examples:
  scripts/update-flake.sh
  scripts/update-flake.sh .#packages.aarch64-darwin.default
  scripts/update-flake.sh --skip-verify
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-verify)
      VERIFY=false
      shift
      ;;
    --allow-untracked)
      ALLOW_UNTRACKED=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      BUILD_TARGET="$1"
      shift
      ;;
  esac
done

if ! command -v nix >/dev/null 2>&1; then
  echo "error: nix is required but not installed" >&2
  exit 1
fi

if [[ ! -f "${FLAKE_FILE}" ]]; then
  echo "error: flake.nix not found at ${FLAKE_FILE}" >&2
  exit 1
fi

if command -v git >/dev/null 2>&1 && [[ "${VERIFY}" == "true" ]] && [[ "${ALLOW_UNTRACKED}" != "true" ]]; then
  untracked_files="$(git -C "${ROOT_DIR}" ls-files --others --exclude-standard)"
  if [[ -n "${untracked_files}" ]]; then
    echo "error: untracked files detected; cleanSource excludes these from nix builds." >&2
    echo "Either add/commit the files, or rerun with --allow-untracked / --skip-verify." >&2
    printf '%s\n' "${untracked_files}" >&2
    exit 1
  fi
fi

extract_vendor_hash() {
  awk '
    /^[[:space:]]*vendorHash[[:space:]]*=/ {
      if (match($0, /"[^"]+"/)) {
        hash = substr($0, RSTART + 1, RLENGTH - 2)
        print hash
        exit
      }
    }
  ' "${FLAKE_FILE}"
}

replace_vendor_hash() {
  local hash="$1"
  local tmp_file
  tmp_file="$(mktemp)"

  awk -v hash="${hash}" '
    BEGIN { replaced = 0 }
    {
      if ($0 ~ /^[[:space:]]*vendorHash[[:space:]]*=/) {
        sub(/"[^"]+"/, "\"" hash "\"")
        replaced = 1
      }
      print
    }
    END {
      if (!replaced) {
        exit 2
      }
    }
  ' "${FLAKE_FILE}" > "${tmp_file}"

  mv "${tmp_file}" "${FLAKE_FILE}"
}

current_hash="$(extract_vendor_hash)"
if [[ -z "${current_hash}" ]]; then
  echo "error: could not find quoted vendorHash in ${FLAKE_FILE}" >&2
  exit 1
fi

backup_file="$(mktemp)"
cp "${FLAKE_FILE}" "${backup_file}"
restore_on_exit=true

cleanup() {
  if [[ "${restore_on_exit}" == "true" ]]; then
    cp "${backup_file}" "${FLAKE_FILE}"
  fi
  rm -f "${backup_file}"
}
trap cleanup EXIT

echo "Current vendorHash: ${current_hash}"
echo "Setting temporary fake hash and building ${BUILD_TARGET}..."
replace_vendor_hash "${FAKE_HASH}"

set +e
build_output="$(cd "${ROOT_DIR}" && nix build "${BUILD_TARGET}" 2>&1)"
build_status=$?
set -e

if [[ ${build_status} -eq 0 ]]; then
  echo "error: nix build unexpectedly succeeded with fake hash; aborting" >&2
  echo "${build_output}" >&2
  exit 1
fi

new_hash="$(printf '%s\n' "${build_output}" | awk '
  /got:[[:space:]]*sha256[-:]/ {
    for (i = 1; i <= NF; i++) {
      if ($i ~ /^sha256[-:]/) {
        hash = $i
      }
    }
  }
  END { print hash }
')"

if [[ -z "${new_hash}" ]]; then
  echo "error: failed to extract the new hash from nix build output" >&2
  echo "${build_output}" >&2
  exit 1
fi

if [[ "${new_hash}" == sha256:* ]]; then
  new_hash="$(nix hash convert "${new_hash}")"
fi

echo "Updating vendorHash to: ${new_hash}"
replace_vendor_hash "${new_hash}"

if [[ "${VERIFY}" == "true" ]]; then
  echo "Verifying with nix build ${BUILD_TARGET}..."
  cd "${ROOT_DIR}"
  nix build "${BUILD_TARGET}" >/dev/null
fi

restore_on_exit=false
echo "flake.nix updated successfully."
