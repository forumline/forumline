#!/usr/bin/env bash
# Setup script for CI Docker LXC
#
# Run on a fresh Debian/Ubuntu LXC (Proxmox CT).
# This LXC hosts the Docker daemon that Dagger uses as its engine.
#
# Usage:
#   1. Create LXC on Proxmox (Debian 12, 2 cores, 4GB RAM, 32GB disk)
#   2. SSH in and run: bash setup.sh
#   3. On the self-hosted runner (Proxmox host), set:
#        export DOCKER_HOST=tcp://<this-lxc-ip>:2375
#      (add to /etc/environment or the runner's .env for persistence)
#   4. Install Dagger CLI on the runner:
#        curl -fsSL https://dl.dagger.io/dagger/install.sh | sh
#
# Security: Docker TCP is bound to 0.0.0.0 but only reachable on
# the Proxmox bridge (LAN). No public exposure.

set -euo pipefail

echo "=== Installing Docker CE ==="
apt-get update
apt-get install -y ca-certificates curl gnupg

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  > /etc/apt/sources.list.d/docker.list

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

echo "=== Configuring Docker to listen on TCP ==="
mkdir -p /etc/docker
cat > /etc/docker/daemon.json <<'EOF'
{
  "hosts": ["unix:///var/run/docker.sock", "tcp://0.0.0.0:2375"],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
EOF

# systemd override: remove -H fd:// so daemon.json hosts take effect
mkdir -p /etc/systemd/system/docker.service.d
cat > /etc/systemd/system/docker.service.d/override.conf <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd
EOF

systemctl daemon-reload
systemctl enable docker
systemctl restart docker

echo "=== Docker ready ==="
docker version
echo ""
echo "TCP endpoint: tcp://$(hostname -I | awk '{print $1}'):2375"
echo ""
echo "Next steps:"
echo "  1. On the Proxmox host (self-hosted runner), run:"
echo "     curl -fsSL https://dl.dagger.io/dagger/install.sh | sh"
echo "  2. Add to runner environment:"
echo "     echo 'DOCKER_HOST=tcp://$(hostname -I | awk '{print $1}'):2375' >> /etc/environment"
echo "  3. Verify: DOCKER_HOST=tcp://$(hostname -I | awk '{print $1}'):2375 dagger version"
