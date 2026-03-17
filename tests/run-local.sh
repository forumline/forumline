#!/usr/bin/env bash
# Run e2e tests locally, sourcing test user passwords from secrets.kdbx.
# Usage:
#   ./tests/run-local.sh                    # all tests
#   ./tests/run-local.sh --project=smoke    # smoke only
#   ./tests/run-local.sh --project=app      # app e2e only

set -euo pipefail

MASTER="$(security find-generic-password -a master -s forumline-secrets -w)"

export TESTCALLER_PASSWORD
TESTCALLER_PASSWORD="$(printf '%s\n' "$MASTER" | keepassxc-cli show secrets.kdbx "dev/TESTCALLER_PASSWORD" -q -sa password)" # gitleaks:allow

export TESTUSER_DEBUG_PASSWORD
TESTUSER_DEBUG_PASSWORD="$(printf '%s\n' "$MASTER" | keepassxc-cli show secrets.kdbx "dev/TESTUSER_DEBUG_PASSWORD" -q -sa password)" # gitleaks:allow

exec pnpm test:e2e "$@"
