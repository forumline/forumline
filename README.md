# Forumline

A modern community platform combining threaded forums, real-time chat, and voice rooms — with cross-forum federation via a central identity service.

**Live demo**: [demo.forumline.net](https://demo.forumline.net)
**Forumline App**: [app.forumline.net](https://app.forumline.net)

## Why

Traditional forums lack real-time interaction. Chat apps lack structure. Forumline combines both with voice rooms and a federation layer that lets independent forum instances share identity and direct messaging.

## Stack

- **Forum Frontend** — Vanilla JS, Vite, TailwindCSS
- **Forumline App Frontend** — Vanilla TS, Vite
- **Backend** — Go API servers (Chi router), Postgres 17, Zitadel (self-hosted OIDC auth)
- **Realtime** — SSE via Postgres LISTEN/NOTIFY
- **Voice** — LiveKit
- **Storage** — Cloudflare R2 (avatars/images)
- **Native** — iOS (Swift/WKWebView), Android (Kotlin/WebView), macOS, Windows, Linux
- **Deploy** — Self-hosted Proxmox LXCs, Docker Compose, Cloudflare Tunnel, Dagger CI/CD

## Monorepo Layout

| Directory | Description |
|-----------|-------------|
| `packages/protocol/` | Federation types (zero-dependency) |
| `packages/server-sdk/` | Protocol endpoint handler factories |
| `packages/shared-go/` | Shared Go infrastructure (db, auth, SSE, middleware) |
| `services/forum/` | Forum server — Go backend + vanilla JS frontend |
| `services/forumline-api/` | Forumline Go API server (`cmd/forumline/`) |
| `services/forumline-web/` | Forumline app — identity & federation registry (Vite + vanilla TS) |
| `services/hosted/` | Multi-tenant hosted forum platform |
| `services/website/` | Static website (forumline.net) |
| `apps/` | Native apps (iOS, Android, macOS, Windows, Linux) |
| `deploy/` | Dockerfiles, compose configs, Terraform |
| `tools/` | Cloudflare status worker |

## Quick Start

```bash
# Install (links workspace packages)
pnpm install

# Start local Postgres + Zitadel
cd services/zitadel && docker compose up -d

# Run the forum backend
cd services/forum && go run .

# Run the forum frontend
cd services/forum && pnpm dev

# Run the forumline backend
cd services/forumline-api && go run ./cmd/forumline/

# Run the forumline frontend
cd services/forumline-web && pnpm dev
```

Both apps require a `.env.local` — see `.env.example` in each directory.

## Scripts

```bash
pnpm build          # Build all packages (via Turbo)
pnpm dev:app        # Run central services dev server
pnpm lint           # ESLint
pnpm format         # Prettier
```

## Deployment

All services are self-hosted on Proxmox LXCs with Docker Compose, exposed via Cloudflare Tunnel. CI/CD pipelines are defined in Dagger (`ci/main.go`) and triggered by thin GitHub Actions wrappers on push to `main`. Run any pipeline locally with `dagger call <function> --source .`.

## License

All rights reserved — Forumline is not yet ready for public use. An open-source license will be added when it is. See [LICENSE](LICENSE).
