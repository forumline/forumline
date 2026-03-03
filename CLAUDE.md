# Forum Chat Voice

## Testing

**Always test through production**: https://forum-chat-voice.vercel.app

Do NOT use local dev server for testing. Use Playwright to interact with the production site.

## Deployment

Both projects deploy via GitHub Actions on push to main. Do NOT deploy via Vercel CLI or Vercel dashboard.

- **Forum** (forum-chat-voice.vercel.app): `.github/workflows/deploy-forum.yml` — triggers on all changes except `hub/`
- **Hub** (forumline-hub.vercel.app): `.github/workflows/deploy-hub.yml` — triggers only on `hub/` changes

Both use the `VERCEL_TOKEN` GitHub secret.

## Vercel
The Vercel CLI token is stored in macOS Keychain under `vercel-token`.

## Supabase
The Supabase personal access token is stored in macOS Keychain under `supabase-access-token`.

## Stack

- React 19 + Vite + TailwindCSS
- Supabase (auth, database, realtime)
- LiveKit (voice rooms)
- Deployed on Vercel
