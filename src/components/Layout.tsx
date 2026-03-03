import { Outlet } from 'react-router-dom'
import { useState, useCallback, useEffect } from 'react'
import Header from './Header'
import Sidebar from './Sidebar'
import MobileSidebar from './MobileSidebar'
import { supabase } from '../lib/supabase'
import { useAuth } from '../lib/auth'
import { useCachedData, cacheKeys } from '../lib/useCache'
import type { Category, ChatChannel, VoiceRoom } from '../types'

export default function Layout() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const { user } = useAuth()
  const [unreadDmCount, setUnreadDmCount] = useState(0)

  // Use cached data for static sidebar content - instant on tab switch!
  const { data: categories = [] } = useCachedData<Category[]>(
    cacheKeys.categories(),
    'categories',
    async () => {
      const { data } = await supabase.from('categories').select('*').order('sort_order')
      return data || []
    }
  )

  const { data: channels = [] } = useCachedData<ChatChannel[]>(
    cacheKeys.channels(),
    'channels',
    async () => {
      const { data } = await supabase.from('chat_channels').select('*').order('name')
      return data || []
    }
  )

  const { data: rooms = [] } = useCachedData<VoiceRoom[]>(
    cacheKeys.voiceRooms(),
    'voiceRooms',
    async () => {
      const { data } = await supabase.from('voice_rooms').select('*').order('name')
      return data || []
    }
  )

  useEffect(() => {
    if (!user) {
      setUnreadDmCount(0)
      return
    }

    const fetchUnread = async () => {
      const { count } = await supabase
        .from('direct_messages')
        .select('*', { count: 'exact', head: true })
        .eq('recipient_id', user.id)
        .eq('read', false)
      setUnreadDmCount(count ?? 0)
    }

    fetchUnread()

    // Subscribe to new DMs to update unread badge in real-time
    const sub = supabase
      .channel('layout-dm-unread')
      .on(
        'postgres_changes',
        { event: '*', schema: 'public', table: 'direct_messages', filter: `recipient_id=eq.${user.id}` },
        () => fetchUnread()
      )
      .subscribe()

    return () => { sub.unsubscribe() }
  }, [user])

  const handleMenuClick = useCallback(() => {
    setMobileMenuOpen(true)
  }, [])

  const handleMenuClose = useCallback(() => {
    setMobileMenuOpen(false)
  }, [])

  return (
    <div className="min-h-screen bg-slate-900">
      <Header onMenuClick={handleMenuClick} />

      {/* Mobile sidebar */}
      <MobileSidebar
        isOpen={mobileMenuOpen}
        onClose={handleMenuClose}
        categories={categories}
        channels={channels}
        rooms={rooms}
        unreadDmCount={unreadDmCount}
      />

      <div className="flex">
        <Sidebar
          categories={categories}
          channels={channels}
          rooms={rooms}
          unreadDmCount={unreadDmCount}
        />
        <main className="flex-1 p-4 sm:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
