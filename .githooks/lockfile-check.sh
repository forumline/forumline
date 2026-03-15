#!/usr/bin/env bash
set -euo pipefail

ROOT_LOCK="pnpm-lock.yaml"
STANDALONE_DIRS=()

missing=()

workspace_pkgs=(
  "package.json"
  "forumline-identity-and-federation-web/package.json"
  "published-npm-packages/protocol/package.json"
  "published-npm-packages/server-sdk/package.json"
  "published-npm-packages/central-services-client/package.json"
  "published-npm-packages/core/package.json"
)

for pkg in "${workspace_pkgs[@]}"; do
  if git diff --cached --name-only | grep -qx "$pkg"; then
    if git diff --cached "$pkg" | grep -qE '^\+.*"(version|@forumline/)'; then
      if ! git diff --cached --name-only | grep -qx "$ROOT_LOCK"; then
        missing+=("$ROOT_LOCK (required by staged changes in $pkg)")
        break
      fi
    fi
  fi
done

for dir in "${STANDALONE_DIRS[@]}"; do
  pkg="$dir/package.json"
  lock="$dir/pnpm-lock.yaml"

  if git diff --cached --name-only | grep -qx "$pkg"; then
    if git diff --cached "$pkg" | grep -qE '^\+.*"(version|dependencies)'; then
      if ! git diff --cached --name-only | grep -qx "$lock"; then
        missing+=("$lock (required by staged changes in $pkg)")
      fi
    fi
  fi
done

if [ ${#missing[@]} -gt 0 ]; then
  echo "ERROR: Lockfile(s) not staged but required:"
  for m in "${missing[@]}"; do
    echo "  - $m"
  done
  echo ""
  echo "Run 'pnpm install' in the affected directories,"
  echo "then stage the lockfile(s) and retry."
  exit 1
fi
