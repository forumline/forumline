# Deployment Guide

All CI/CD pipelines are defined in **Dagger** (`ci/main.go`). GitHub Actions workflows are thin wrappers that trigger on push to `main` and pass secrets to Dagger.

Run any pipeline locally: `dagger call <function> --source .`

## Production URLs

- **Website**: https://forumline.net
- **Forum Demo**: https://demo.forumline.net
- **Forumline App**: https://app.forumline.net
- **Auth (Zitadel)**: https://auth.forumline.net

## Architecture

```
GitHub push/PR → webhook → Woodpecker CI (single LXC, ~130MB idle)
                              ├── lint/test: runs directly on host (local backend)
                              └── deploy: SSH to service LXCs (direct LAN)
```

## CI/CD Pipelines (Woodpecker)

Pipelines are defined in `.woodpecker/` as YAML files. The Woodpecker agent runs commands directly on the host (local backend — no containers).

| Pipeline | Trigger | Description |
|----------|---------|-------------|
| `lint` | push, PR | Run lefthook checks (Go lint, tests, ESLint, gitleaks) |
| `deploy-forumline` | `services/forumline-api/**`, `services/forumline-web/**`, `packages/**` | Deploy Forumline app |
| `deploy-hosted` | `services/hosted/**`, `services/forum/**` | Deploy hosted forum platform |
| `deploy-website` | `services/website/**` | Deploy static website |
| `deploy-logs` | `deploy/compose/logs/**` | Deploy central VictoriaLogs |
| `deploy-auth` | `deploy/compose/auth/**` | Deploy Zitadel auth |
| `deploy-logs-agents` | `deploy/compose/logs-agent/**` | Deploy Vector agents to all LXCs |
| `publish-packages` | `packages/**` | Publish TS packages to GitHub Packages |
| `split-repos` | `packages/shared-go/**`, `services/forum/**` | Split forum subtree to read-only repo |
| `terraform-plan` | PR touching `deploy/terraform/` | Run OpenTofu plan |
| `terraform-apply` | manual | Run OpenTofu apply |

Deploy logic lives in `ci/deploy.sh` — a shared script that handles sops decrypt, scp, SSH rebuild per service.

## Required Woodpecker Secrets

Add these in the Woodpecker UI (repo settings > secrets):

- `sops_age_key` — age key for decrypting .env.enc files
- `github_packages_token` — GitHub token with packages:write scope
- `split_repo_token` — GitHub token for forum-server read-only repo
- `tf_state_r2_access_key_id` — R2 access key for OpenTofu state backend
- `tf_state_r2_secret_access_key` — R2 secret key for OpenTofu state backend
- `tf_cloudflare_api_token` — Cloudflare API token for tunnel/access management
- `tf_state_encryption_passphrase` — passphrase for OpenTofu state encryption

SSH deploy key is installed directly on the Woodpecker agent host (not a CI secret).

## Woodpecker CI Setup

Single LXC runs both the Woodpecker server (Docker) and agent (local backend).

1. Create a Proxmox LXC (Debian 12, 2 cores, 4GB RAM, 32GB disk) with nesting:
   ```bash
   pct set <CTID> -features nesting=1
   ```
2. Run the setup script: `sudo bash ci/setup-woodpecker.sh`
3. Create GitHub OAuth App (Homepage: `https://ci.forumline.net`, Callback: `https://ci.forumline.net/authorize`)
4. Add Cloudflare Tunnel route: `ci.forumline.net` → `http://localhost:8000`
5. Create `/opt/woodpecker/.env`, start server + agent (see script output)
6. Log in, activate the repo, add secrets

## Cloudflare Tunnel (Terraform)

Tunnel ingress and Zero Trust Access policies managed via OpenTofu in `deploy/terraform/`. Config lives in Cloudflare (remotely-managed). `cloudflared` runs with `--token` on `forum-prod` (CT 100) — no local config file.

**Managed resources:** tunnel ingress rules, Access applications for SSH endpoints, short-lived SSH CA certificates, service token for GitHub Actions deploys, developer email allow policies.

**Changing tunnel routes:**

```bash
cd deploy/terraform
AWS_ACCESS_KEY_ID=$(security find-generic-password -a access-key-id -s cloudflare-r2-terraform-state -w) \
AWS_SECRET_ACCESS_KEY=$(security find-generic-password -a secret-access-key -s cloudflare-r2-terraform-state -w) \
TF_VAR_cloudflare_api_token=$(security find-generic-password -a api-token -s cloudflare-tunnel-terraform -w) \
TF_VAR_state_encryption_passphrase=$(security find-generic-password -a passphrase -s tofu-state-encryption -w) \
tofu plan -var-file=prod.tfvars    # review changes
tofu apply -var-file=prod.tfvars   # apply — takes effect immediately, no restart
```

**State**: stored in Cloudflare R2 (`forumline-terraform-state` bucket), encrypted client-side via AES-GCM before upload.

**Rule ordering**: specific hostnames MUST come before `*.forumline.net` wildcard, or SSH routes break.

**Do NOT run `tofu destroy`** — `prevent_destroy` blocks it, but don't try to work around it.

## LXC Setup

Each service runs on a Proxmox LXC with Docker, SSH access via Cloudflare Tunnel, and a public Cloudflare Tunnel route for the service.

### Website LXC (one-time setup)

1. Create a Proxmox LXC (Debian/Ubuntu, 512MB RAM is plenty)
2. Install Docker:
   ```bash
   curl -fsSL https://get.docker.com | sh
   ```
3. Set up the deploy directory and clone the repo:
   ```bash
   mkdir -p /opt/website
   git clone https://github.com/forumline/forumline.git /opt/website/repo
   ```
4. Add the deploy SSH public key to `/root/.ssh/authorized_keys`
5. Add tunnel routes in `deploy/terraform/tunnel.tf` and apply (see above)
6. Test the deploy:
   ```bash
   cd /opt/website/repo && git pull origin main
   cp deploy/compose/website/docker-compose.yml /opt/website/docker-compose.yml
   cd /opt/website && docker compose up -d --build website
   ```

### Forum / Forumline LXCs

Same pattern — see existing LXC configs. Each uses `/opt/<service>/repo` and `/opt/<service>/docker-compose.yml`.

## Local Development

```bash
pnpm install                                          # from root — sets up workspaces
cd services/zitadel && docker compose up -d           # start local Zitadel + Postgres
cd services/forum && go run .                         # start forum backend
cd services/forum && pnpm dev                         # start forum frontend
cd services/forumline-web && VITE_BACKEND=local pnpm dev  # start forumline frontend
```

Create `services/forum/.env.local` and `services/forumline-api/.env.local` with the required env vars.
