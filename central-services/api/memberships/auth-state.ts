import type { VercelRequest, VercelResponse } from '@vercel/node'
import { getHubSupabase, getAuthenticatedUser, handleCors } from '../_lib/supabase.js'

export default async function handler(req: VercelRequest, res: VercelResponse) {
  if (handleCors(req, res)) return

  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' })
  }

  const user = await getAuthenticatedUser(req, res)
  if (!user) return

  const { forum_domain, authed } = req.body as { forum_domain?: string; authed?: boolean }
  if (!forum_domain || typeof authed !== 'boolean') {
    return res.status(400).json({ error: 'Missing forum_domain or authed' })
  }

  const supabase = getHubSupabase()

  // Look up the forum ID from the domain
  const { data: forum, error: forumError } = await supabase
    .from('forumline_forums')
    .select('id')
    .eq('domain', forum_domain)
    .single()

  if (forumError || !forum) {
    return res.status(404).json({ error: 'Forum not found' })
  }

  // Update the membership's forum_authed_at
  const { error } = await supabase
    .from('forumline_memberships')
    .update({ forum_authed_at: authed ? new Date().toISOString() : null })
    .eq('user_id', user.id)
    .eq('forum_id', forum.id)

  if (error) {
    return res.status(500).json({ error: 'Failed to update auth state' })
  }

  return res.status(200).json({ ok: true })
}
