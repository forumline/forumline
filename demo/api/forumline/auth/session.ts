import type { VercelRequest, VercelResponse } from '@vercel/node'
import { getForumlineServer } from '../../_lib/forumline-server.js'
import { adaptRequest, adaptResponse } from '../../_lib/vercel-adapter.js'

export default async function handler(req: VercelRequest, res: VercelResponse) {
  // DELETE = clear all Forumline cookies (used by disconnect flow)
  if (req.method === 'DELETE') {
    res.setHeader('Set-Cookie', [
      'forumline_identity=; Path=/; HttpOnly; SameSite=Lax; Secure; Max-Age=0',
      'forumline_user_id=; Path=/; HttpOnly; SameSite=Lax; Secure; Max-Age=0',
      'hub_access_token=; Path=/; HttpOnly; SameSite=Lax; Secure; Max-Age=0',
    ])
    return res.status(200).json({ ok: true })
  }

  const server = getForumlineServer()
  return server.sessionHandler()(adaptRequest(req), adaptResponse(res))
}
