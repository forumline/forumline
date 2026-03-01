import { Link, useLocation } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { supabase, isConfigured } from '../lib/supabase'
import type { Category } from '../types'

// Demo categories when Supabase is not configured
const demoCategories: Category[] = [
  { id: '1', name: 'General', slug: 'general', description: 'General discussion', sort_order: 0, created_at: '' },
  { id: '2', name: 'Announcements', slug: 'announcements', description: 'Official announcements', sort_order: 1, created_at: '' },
  { id: '3', name: 'Help & Support', slug: 'help', description: 'Get help from the community', sort_order: 2, created_at: '' },
  { id: '4', name: 'Showcase', slug: 'showcase', description: 'Show off your projects', sort_order: 3, created_at: '' },
]

export default function Sidebar() {
  const location = useLocation()
  const [categories, setCategories] = useState<Category[]>(demoCategories)

  useEffect(() => {
    if (!isConfigured) return

    const fetchCategories = async () => {
      const { data } = await supabase
        .from('categories')
        .select('*')
        .order('sort_order')
      if (data) setCategories(data)
    }

    fetchCategories()
  }, [])

  return (
    <aside className="sticky top-14 hidden h-[calc(100vh-3.5rem)] w-64 shrink-0 border-r border-slate-700 bg-slate-800/50 lg:block">
      <nav className="p-4">
        <div className="mb-4">
          <Link
            to="/"
            className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
              location.pathname === '/'
                ? 'bg-indigo-600 text-white'
                : 'text-slate-300 hover:bg-slate-700 hover:text-white'
            }`}
          >
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
            </svg>
            Home
          </Link>
        </div>

        <div className="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
          Categories
        </div>
        <div className="space-y-1">
          {categories.map((category) => {
            const isActive = location.pathname === `/c/${category.slug}`
            return (
              <Link
                key={category.id}
                to={`/c/${category.slug}`}
                className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors ${
                  isActive
                    ? 'bg-slate-700 text-white'
                    : 'text-slate-400 hover:bg-slate-700/50 hover:text-slate-200'
                }`}
              >
                <span className="h-2 w-2 rounded-full bg-indigo-400" />
                {category.name}
              </Link>
            )
          })}
        </div>

        {/* Chat Channels */}
        <div className="mt-6 mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
          Chat Channels
        </div>
        <div className="space-y-1">
          {['general', 'random', 'introductions', 'help'].map((channel) => {
            const isActive = location.pathname === `/chat/${channel}`
            return (
              <Link
                key={channel}
                to={`/chat/${channel}`}
                className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors ${
                  isActive
                    ? 'bg-slate-700 text-white'
                    : 'text-slate-400 hover:bg-slate-700/50 hover:text-slate-200'
                }`}
              >
                <span className="text-green-400">#</span>
                {channel}
              </Link>
            )
          })}
        </div>

        {/* Voice Rooms */}
        <div className="mt-6 mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
          Voice Rooms
        </div>
        <div className="space-y-1">
          {['lounge', 'gaming', 'music', 'study'].map((room) => {
            const isActive = location.pathname === `/voice/${room}`
            return (
              <Link
                key={room}
                to={`/voice/${room}`}
                className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors ${
                  isActive
                    ? 'bg-slate-700 text-white'
                    : 'text-slate-400 hover:bg-slate-700/50 hover:text-slate-200'
                }`}
              >
                <svg className="h-4 w-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15.414a5 5 0 001.414 1.414m2.828-9.9a9 9 0 0112.728 0" />
                </svg>
                {room.charAt(0).toUpperCase() + room.slice(1)}
              </Link>
            )
          })}
        </div>

        {/* Direct Messages */}
        <div className="mt-6 mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
          Direct Messages
        </div>
        <div className="space-y-1">
          <Link
            to="/dm"
            className={`flex items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors ${
              location.pathname.startsWith('/dm')
                ? 'bg-slate-700 text-white'
                : 'text-slate-400 hover:bg-slate-700/50 hover:text-slate-200'
            }`}
          >
            <div className="flex items-center gap-2">
              <svg className="h-4 w-4 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
              </svg>
              Messages
            </div>
            <span className="flex h-5 min-w-[1.25rem] items-center justify-center rounded-full bg-indigo-500 px-1.5 text-xs font-medium text-white">
              2
            </span>
          </Link>
        </div>
      </nav>
    </aside>
  )
}
