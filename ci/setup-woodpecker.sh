#!/usr/bin/env bash
# Setup script for a Woodpecker CI LXC — server + local backend agent on one machine.
#
# Replaces the old GitHub Actions runner + CI Docker LXC setup.
# The server receives GitHub webhooks and the agent runs commands directly on the host.
#
# Prerequisites:
#   - Proxmox LXC with nesting enabled (for Docker):
#       pct set <CTID> -features nesting=1
#       pct restart <CTID>
#   - Debian 12, 2+ cores, 4GB RAM, 32GB+ disk
#   - Cloudflare Tunnel route: ci.forumline.net → http://localhost:8000
#   - GitHub OAuth App:
#       Homepage URL: https://ci.forumline.net
#       Callback URL: https://ci.forumline.net/authorize
#
# Usage: sudo bash ci/setup-woodpecker.sh
#
# After this script completes:
#   1. Create /opt/woodpecker/.env with GitHub OAuth credentials (see output)
#   2. Start the server: cd /opt/woodpecker && docker compose up -d
#   3. Start the agent: systemctl start woodpecker-agent
#   4. Copy deploy SSH key to /home/woodpecker/.ssh/id_deploy
#   5. Log in at https://ci.forumline.net and activate the repo
#   6. Add secrets via Woodpecker UI or CLI

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root: sudo bash $0"
  exit 1
fi

# ── System packages ──────────────────────────────────────────────

echo "=== Installing system packages ==="
apt-get update
apt-get install -y curl git build-essential jq unzip openssh-client ca-certificates gnupg

# ── Docker (for Woodpecker server only) ──────────────────────────

echo "=== Installing Docker CE ==="
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  > /etc/apt/sources.list.d/docker.list

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

systemctl enable docker
systemctl start docker

# ── Build tools (for local backend agent) ────────────────────────

echo "=== Installing Go 1.26 ==="
curl -fsSL "https://go.dev/dl/go1.26.0.linux-amd64.tar.gz" -o /tmp/go.tar.gz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tar.gz
rm /tmp/go.tar.gz

echo "=== Installing Node.js 22 ==="
curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
apt-get install -y nodejs

echo "=== Installing pnpm ==="
corepack enable
corepack prepare pnpm@10.6.5 --activate

echo "=== Installing golangci-lint ==="
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b /usr/local/bin v2.11.2

echo "=== Installing gitleaks ==="
curl -fsSL https://github.com/gitleaks/gitleaks/releases/download/v8.21.2/gitleaks_8.21.2_linux_x64.tar.gz | tar -xz -C /usr/local/bin gitleaks

echo "=== Installing sops + age ==="
curl -fsSL "https://dl.filippo.io/age/v1.2.0?for=linux/amd64" -o /tmp/age.tar.gz
tar -xzf /tmp/age.tar.gz -C /tmp && mv /tmp/age/age /usr/local/bin/ && rm -rf /tmp/age*
curl -fsSL "https://github.com/getsops/sops/releases/download/v3.9.4/sops-v3.9.4.linux.amd64" -o /usr/local/bin/sops
chmod +x /usr/local/bin/sops

echo "=== Installing OpenTofu ==="
curl -fsSL https://get.opentofu.org/install-opentofu.sh | sh -s -- --install-method deb

echo "=== Installing splitsh-lite ==="
curl -fsSL https://github.com/splitsh/lite/releases/download/v1.0.1/lite_linux_amd64.tar.gz -o /tmp/splitsh.tar.gz
tar -xzf /tmp/splitsh.tar.gz -C /tmp && mv /tmp/splitsh-lite /usr/local/bin/ && rm -f /tmp/splitsh.tar.gz

# ── Woodpecker agent user ────────────────────────────────────────

echo "=== Creating woodpecker user ==="
id woodpecker &>/dev/null || useradd -m -s /bin/bash woodpecker

# ── Woodpecker agent (local backend) ─────────────────────────────

echo "=== Installing Woodpecker agent ==="
WP_VERSION=$(curl -s https://api.github.com/repos/woodpecker-ci/woodpecker/releases/latest | jq -r .tag_name | sed 's/^v//')
curl -fsSL "https://github.com/woodpecker-ci/woodpecker/releases/download/v${WP_VERSION}/woodpecker-agent_${WP_VERSION}_amd64.deb" -o /tmp/wp-agent.deb
dpkg -i /tmp/wp-agent.deb
rm /tmp/wp-agent.deb

# Generate agent secret
AGENT_SECRET=$(openssl rand -hex 32)

cat > /etc/woodpecker/woodpecker-agent.env <<EOF
WOODPECKER_SERVER=localhost:9000
WOODPECKER_AGENT_SECRET=${AGENT_SECRET}
WOODPECKER_BACKEND=local
WOODPECKER_MAX_WORKFLOWS=2
WOODPECKER_LOG_LEVEL=info
WOODPECKER_HEALTHCHECK_ADDR=:3001
EOF

# Agent needs tools in PATH
mkdir -p /etc/systemd/system/woodpecker-agent.service.d
cat > /etc/systemd/system/woodpecker-agent.service.d/override.conf <<'EOF'
[Service]
User=woodpecker
Environment=PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin
EOF

systemctl daemon-reload
systemctl enable woodpecker-agent

# ── SSH for deploy ───────────────────────────────────────────────

echo "=== Setting up SSH for LAN access ==="
mkdir -p /home/woodpecker/.ssh
chmod 700 /home/woodpecker/.ssh
cat > /home/woodpecker/.ssh/config <<'SSHEOF'
Host forumline-prod
  HostName 192.168.1.99
  User root
  IdentityFile ~/.ssh/id_deploy
  StrictHostKeyChecking no

Host hosted-prod
  HostName 192.168.1.107
  User root
  IdentityFile ~/.ssh/id_deploy
  StrictHostKeyChecking no

Host website-prod
  HostName 192.168.1.106
  User root
  IdentityFile ~/.ssh/id_deploy
  StrictHostKeyChecking no

Host logs-prod
  HostName 192.168.1.108
  User root
  IdentityFile ~/.ssh/id_deploy
  StrictHostKeyChecking no

Host forum-prod
  HostName 192.168.1.23
  User root
  IdentityFile ~/.ssh/id_deploy
  StrictHostKeyChecking no

Host auth-prod
  HostName 192.168.1.110
  User root
  IdentityFile ~/.ssh/id_deploy
  StrictHostKeyChecking no
SSHEOF
chmod 600 /home/woodpecker/.ssh/config
chown -R woodpecker:woodpecker /home/woodpecker/.ssh

# ── Woodpecker server compose ────────────────────────────────────

echo "=== Setting up Woodpecker server ==="
mkdir -p /opt/woodpecker
cp "$(dirname "$0")/../deploy/compose/woodpecker/docker-compose.yml" /opt/woodpecker/docker-compose.yml 2>/dev/null || true

echo ""
echo "=== Setup complete ==="
echo ""
echo "Agent secret (save this): ${AGENT_SECRET}"
echo ""
echo "Next steps:"
echo ""
echo "  1. Create /opt/woodpecker/.env:"
echo "     WOODPECKER_HOST=https://ci.forumline.net"
echo "     WOODPECKER_AGENT_SECRET=${AGENT_SECRET}"
echo "     WOODPECKER_ADMIN=<your-github-username>"
echo "     WOODPECKER_OPEN=false"
echo "     WOODPECKER_GITHUB=true"
echo "     WOODPECKER_GITHUB_CLIENT=<oauth-client-id>"
echo "     WOODPECKER_GITHUB_SECRET=<oauth-client-secret>"
echo ""
echo "  2. Start the server:"
echo "     cd /opt/woodpecker && docker compose up -d"
echo ""
echo "  3. Start the agent:"
echo "     systemctl start woodpecker-agent"
echo ""
echo "  4. Copy deploy SSH key:"
echo "     scp id_deploy root@<this-host>:/home/woodpecker/.ssh/id_deploy"
echo "     chown woodpecker:woodpecker /home/woodpecker/.ssh/id_deploy"
echo "     chmod 600 /home/woodpecker/.ssh/id_deploy"
echo ""
echo "  5. Add Cloudflare Tunnel route: ci.forumline.net → http://localhost:8000"
echo ""
echo "  6. Log in at https://ci.forumline.net, activate the repo, add secrets"
