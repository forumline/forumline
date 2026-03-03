# Deployment Guide

This app is deployed on **Vercel** with **Supabase** as the backend.

## Production URLs

- **Frontend**: Deployed via Vercel (auto-deploys from main branch)
- **Database**: Supabase PostgreSQL
- **Voice**: LiveKit Cloud

## Environment Variables

### Required for Vercel

Set these in Vercel Dashboard → Settings → Environment Variables:

| Variable | Description |
|----------|-------------|
| `VITE_SUPABASE_URL` | Your Supabase project URL |
| `VITE_SUPABASE_ANON_KEY` | Supabase anonymous/public key |
| `VITE_SITE_URL` | Your production URL (e.g., `https://forum-chat-voice.vercel.app`) |
| `LIVEKIT_URL` | LiveKit server URL (wss://...) |
| `LIVEKIT_API_KEY` | LiveKit API key |
| `LIVEKIT_API_SECRET` | LiveKit API secret |

### Local Development

Create `.env.local`:

```bash
VITE_SUPABASE_URL=https://your-project.supabase.co
VITE_SUPABASE_ANON_KEY=your-anon-key
VITE_SITE_URL=http://localhost:3000
LIVEKIT_URL=wss://your-livekit.livekit.cloud
LIVEKIT_API_KEY=your-api-key
LIVEKIT_API_SECRET=your-api-secret
```

## Local Development

```bash
npm install
npm run dev
```

## Deployment

Vercel auto-deploys when you push to the main branch.

Manual deploy:
```bash
npm run build
vercel --prod
```

## Supabase Setup

1. Create a Supabase project at https://supabase.com
2. Run the schema in `supabase/schema.sql`
3. Enable Row Level Security (RLS)
4. Configure authentication providers (Email, GitHub)
5. Set the Site URL in Authentication → URL Configuration

### Auth Redirect URLs

In Supabase Dashboard → Authentication → URL Configuration:

- **Site URL**: `https://your-app.vercel.app`
- **Redirect URLs**:
  - `https://your-app.vercel.app`
  - `https://your-app.vercel.app/reset-password`
  - `http://localhost:3000` (for local dev)
  - `http://localhost:3000/reset-password` (for local dev)

## LiveKit Setup

1. Create a LiveKit Cloud account at https://livekit.io
2. Create a new project
3. Copy the API credentials to your environment variables
