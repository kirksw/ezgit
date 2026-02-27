#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$repo_root"

echo "== Fuzzy startup preparation benchmark =="
go test ./cmd -run '^$' -bench BenchmarkFuzzyStartupPreparation -benchmem -count=1

echo
echo "== Worktree lookup benchmark =="
go test ./cmd -run '^$' -bench BenchmarkBuildLocalRepoWorktreeMap -benchmem -count=1

echo
echo "== Cache deserialization benchmark =="
go test ./internal/cache -run '^$' -bench BenchmarkGetAllRepos -benchmem -count=1

echo
echo "== Local repo detection benchmark =="
go test ./internal/utils -run '^$' -bench BenchmarkBuildLocalRepoMap -benchmem -count=1
