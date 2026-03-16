#!/usr/bin/env bash
# Deploy a service to production via direct LAN SSH.
# Called by GitHub Actions deploy workflows.
#
# Usage: ci/deploy.sh <service>
#
# Services: forumline, hosted, website, logs, auth, livekit, logs-docker
#
# Secrets are read from deploy/secrets.kdbx via ci/secrets.sh.
# The master password comes from KEEPASS_PASSWORD env var (CI)
# or macOS Keychain (local dev).

set -euo pipefail

SERVICE="${1:?Usage: ci/deploy.sh <service>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Configure Docker syslog logging on all service LXCs ---
if [ "$SERVICE" = "logs-docker" ]; then
  echo "=== Configuring Docker syslog logging on all LXCs ==="
  DAEMON_JSON="$REPO_ROOT/deploy/compose/logs/daemon.json"
  declare -A SYSLOG_HOSTS=(
    [forumline-prod]="192.168.1.99"
    [hosted-prod]="192.168.1.107"
    [livekit-prod]="192.168.1.111"
    [auth-prod]="192.168.1.110"
  )
  for LXC_NAME in "${!SYSLOG_HOSTS[@]}"; do
    LXC_IP="${SYSLOG_HOSTS[$LXC_NAME]}"
    echo "Configuring $LXC_NAME ($LXC_IP)..."
    scp "$DAEMON_JSON" "root@$LXC_IP:/etc/docker/daemon.json"
    ssh "root@$LXC_IP" "systemctl restart docker"
    echo "$LXC_NAME: Docker restarted with syslog driver"
  done
  echo "=== All LXCs configured ==="
  exit 0
fi

# --- Website deploys to Cloudflare Pages (no LXC, no SSH) ---
if [ "$SERVICE" = "website" ]; then
  echo "=== Deploying website to Cloudflare Pages ==="
  CLOUDFLARE_API_TOKEN=$("$SCRIPT_DIR/secrets.sh" terraform | grep CLOUDFLARE_PAGES_TOKEN | cut -d= -f2-)
  CLOUDFLARE_ACCOUNT_ID="b4cf6ac20ef4cd693cd7a81113b8d031"
  export CLOUDFLARE_API_TOKEN CLOUDFLARE_ACCOUNT_ID
  npx wrangler pages deploy "$REPO_ROOT/services/website" \
    --project-name=forumline-website --branch=main
  echo "=== website deployed ==="
  exit 0
fi

declare -A HOSTS=(
  [forumline]="forumline-prod"
  [hosted]="hosted-prod"
  [logs]="logs-prod"
  [auth]="auth-prod"
  [livekit]="livekit-prod"
)

declare -A PATHS=(
  [forumline]="/opt/forumline"
  [hosted]="/opt/hosted"
  [logs]="/opt/logs"
  [auth]="/opt/auth"
  [livekit]="/opt/livekit"
)

# Map service name to KeePass group (services without secrets have no group)
declare -A SECRET_GROUPS=(
  [forumline]="forumline-prod"
  [hosted]="hosted-prod"
  [auth]="auth-prod"
  [livekit]="livekit-prod"
)

HOST="${HOSTS[$SERVICE]:?Unknown service: $SERVICE}"
REMOTE="${PATHS[$SERVICE]}"

echo "=== Deploying $SERVICE to $HOST ==="

# Generate and upload .env from KeePass secrets
if [ -n "${SECRET_GROUPS[$SERVICE]:-}" ]; then
  echo "Generating .env from secrets.kdbx..."
  "$SCRIPT_DIR/secrets.sh" "${SECRET_GROUPS[$SERVICE]}" /tmp/service.env
  scp /tmp/service.env "$HOST:$REMOTE/.env"
  rm -f /tmp/service.env
fi

# Upload docker-compose.yml
echo "Uploading docker-compose.yml..."
scp "deploy/compose/$SERVICE/docker-compose.yml" "$HOST:$REMOTE/docker-compose.yml"

# Upload extra config files
if [ "$SERVICE" = "livekit" ]; then
  echo "Uploading livekit.yaml..."
  scp deploy/compose/livekit/livekit.yaml "$HOST:$REMOTE/livekit.yaml"
fi

# Pull latest code (skip for infrastructure-only LXCs — no repo)
if [ "$SERVICE" != "logs" ] && [ "$SERVICE" != "auth" ] && [ "$SERVICE" != "livekit" ]; then
  echo "Pulling latest code..."
  ssh "$HOST" "cd $REMOTE/repo && git fetch origin main && git reset --hard origin/main"
fi

# Run migrations for forumline
if [ "$SERVICE" = "forumline" ]; then
  echo "Running migrations..."
  ssh "$HOST" "cd $REMOTE && for f in repo/services/forumline-api/migrations/*.sql; do echo \"Applying: \$f\" && docker compose exec -T postgres psql -U postgres -d postgres < \"\$f\"; done"
fi

# Rebuild and restart
if [ "$SERVICE" = "auth" ] || [ "$SERVICE" = "logs" ] || [ "$SERVICE" = "livekit" ]; then
  echo "Pulling and restarting..."
  ssh "$HOST" "cd $REMOTE && docker compose pull && docker compose up -d --wait && docker compose ps"
else
  echo "Building and restarting..."
  ssh "$HOST" "cd $REMOTE && docker compose up -d --build $SERVICE && docker compose ps"
fi

echo "=== $SERVICE deployed ==="
