import { useState, useCallback } from 'react'
import { useForum } from '@johnvondrashek/forumline-react'

export default function MobileForumList() {
  const { forums, activeForum, unreadCounts, switchForum, addForum } = useForum()
  const [showAddModal, setShowAddModal] = useState(false)
  const [addUrl, setAddUrl] = useState('')
  const [adding, setAdding] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)

  const handleAddForum = useCallback(async () => {
    if (!addUrl.trim()) return
    setAdding(true)
    setAddError(null)
    try {
      await addForum(addUrl.trim())
      setShowAddModal(false)
      setAddUrl('')
    } catch (err) {
      setAddError(String(err))
    } finally {
      setAdding(false)
    }
  }, [addUrl, addForum])

  const totalUnread = (domain: string) => {
    const counts = unreadCounts[domain]
    if (!counts) return 0
    return counts.notifications + counts.chat_mentions + counts.dms
  }

  return (
    <div className="md:hidden">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-slate-400">Your Forums</h2>
      <div className="space-y-2">
        {forums.map((forum) => {
          const unread = totalUnread(forum.domain)
          const isActive = activeForum?.domain === forum.domain
          return (
            <button
              key={forum.domain}
              onClick={() => switchForum(forum.domain)}
              className={`flex w-full items-center gap-3 rounded-xl p-3 transition-colors ${
                isActive
                  ? 'bg-indigo-600/20 ring-1 ring-indigo-500/50'
                  : 'bg-slate-800/50 hover:bg-slate-700/50'
              }`}
            >
              {forum.icon_url ? (
                <img
                  src={forum.icon_url.startsWith('/') ? `${forum.web_base}${forum.icon_url}` : forum.icon_url}
                  alt={forum.name}
                  className="h-10 w-10 rounded-lg object-cover"
                  onError={(e) => {
                    const target = e.target as HTMLImageElement
                    target.style.display = 'none'
                  }}
                />
              ) : (
                <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-slate-700 text-lg font-bold text-white">
                  {forum.name[0].toUpperCase()}
                </div>
              )}
              <div className="flex-1 text-left">
                <p className="font-medium text-white">{forum.name}</p>
                <p className="text-xs text-slate-400">{forum.domain}</p>
              </div>
              {unread > 0 && (
                <div className="flex h-5 min-w-[20px] items-center justify-center rounded-full bg-red-500 px-1.5 text-[10px] font-bold text-white">
                  {unread > 99 ? '99+' : unread}
                </div>
              )}
            </button>
          )
        })}
      </div>

      <button
        onClick={() => setShowAddModal(true)}
        className="mt-3 flex w-full items-center justify-center gap-2 rounded-xl border border-dashed border-slate-600 p-3 text-sm text-slate-400 transition-colors hover:border-green-500 hover:text-green-400"
      >
        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
        </svg>
        Add Forum
      </button>

      {showAddModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
          <div
            className="fixed inset-0 bg-black/60"
            onClick={() => { setShowAddModal(false); setAddError(null); setAddUrl('') }}
          />
          <div className="relative w-full max-w-md rounded-xl border border-slate-700 bg-slate-800 p-6 shadow-2xl">
            <h3 className="text-lg font-semibold text-white">Add a Forum</h3>
            <p className="mt-1 text-sm text-slate-400">
              Enter the URL of a Forumline-compatible forum
            </p>
            <input
              type="url"
              value={addUrl}
              onChange={(e) => setAddUrl(e.target.value)}
              placeholder="https://example-forum.com"
              className="mt-4 w-full rounded-lg border border-slate-600 bg-slate-700 px-4 py-2.5 text-white placeholder-slate-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleAddForum()
                if (e.key === 'Escape') { setShowAddModal(false); setAddError(null); setAddUrl('') }
              }}
            />
            {addError && (
              <p className="mt-2 text-sm text-red-400">{addError}</p>
            )}
            <div className="mt-4 flex justify-end gap-3">
              <button
                onClick={() => { setShowAddModal(false); setAddError(null); setAddUrl('') }}
                className="rounded-lg px-4 py-2 text-sm text-slate-400 hover:bg-slate-700 hover:text-white"
              >
                Cancel
              </button>
              <button
                onClick={handleAddForum}
                disabled={adding || !addUrl.trim()}
                className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
              >
                {adding ? 'Adding...' : 'Add Forum'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
