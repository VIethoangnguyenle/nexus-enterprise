import { useState, useMemo } from 'react'
import { useNavigate, useParams } from '@tanstack/react-router'
import { useChannels, useUnreadCounts } from '../../hooks/useMessaging'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useUiStore } from '../../stores/ui.store'
import { ChatListItem } from './ChatListItem'
import { CreateChannelModal } from '../CreateChannelModal'
import { Avatar, IconButton } from '../primitives'
import { Search, Plus, SlidersHorizontal, ChevronDown, ChevronRight, Pin, Building2, User } from 'lucide-react'
import type { Channel } from '../../api/messaging'

interface ChatListProps {
  workspaceId: string
}

/** Grouped chat list matching Stitch design — PINNED / DEPARTMENTS / DIRECT MESSAGES sections. */
export function ChatList({ workspaceId }: ChatListProps) {
  const { data } = useChannels(workspaceId)
  const channels = data?.channels || []
  const { data: unreadData } = useUnreadCounts()
  const unreadMap = useMemo(() => {
    const map: Record<string, number> = {}
    if (unreadData?.channels) {
      for (const u of unreadData.channels) {
        map[u.channel_id] = u.unread_count
      }
    }
    return map
  }, [unreadData])

  const [searchQuery, setSearchQuery] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [collapsedSections, setCollapsedSections] = useState<Record<string, boolean>>({})

  const params = useParams({ strict: false }) as { channelId?: string }
  const activeChannelId = params.channelId
  const navigate = useNavigate()
  const lastMessages = useWebSocketStore((s) => s.lastMessages)
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const starredChannels = useUiStore((s) => s.starredChannels)

  // Filter by search
  const filtered = useMemo(() => {
    if (!searchQuery.trim()) return channels
    const q = searchQuery.toLowerCase()
    return channels.filter((ch) => ch.name.toLowerCase().includes(q))
  }, [channels, searchQuery])

  // Group channels into sections — always show CHANNELS and DIRECT MESSAGES
  const sections = useMemo(() => {
    const pinned: Channel[] = []
    const channels: Channel[] = []
    const dms: Channel[] = []

    for (const ch of filtered) {
      if (starredChannels.includes(ch.id)) {
        pinned.push(ch)
      } else if (ch.channel_type === 'dm') {
        dms.push(ch)
      } else {
        channels.push(ch)
      }
    }

    // Sort each section by most recent activity
    const sortByActivity = (a: Channel, b: Channel) => {
      const tsA = lastMessages[a.id]?.timestamp
      const tsB = lastMessages[b.id]?.timestamp
      if (!tsA && !tsB) return 0
      if (!tsA) return 1
      if (!tsB) return -1
      return new Date(tsB).getTime() - new Date(tsA).getTime()
    }

    pinned.sort(sortByActivity)
    channels.sort(sortByActivity)
    dms.sort(sortByActivity)

    const result = [
      ...(pinned.length > 0 ? [{ key: 'pinned', label: 'PINNED', channels: pinned, alwaysShow: false }] : []),
      { key: 'channels', label: 'DEPARTMENTS', channels, alwaysShow: true },
      { key: 'dms', label: 'DIRECT MESSAGES', channels: dms, alwaysShow: true },
    ]

    return result
  }, [filtered, starredChannels, lastMessages])

  const handleSelect = (channelId: string) => {
    navigate({ to: '/channels/$channelId', params: { channelId } })
  }

  const toggleSection = (key: string) => {
    setCollapsedSections((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header — Stitch: p-space-md, h2 text-h3, border-b border-outline-variant/20, filter button p-1.5 rounded-md */}
      <div className="p-4 flex items-center justify-between border-b border-outline-variant/20">
        <h2 className="text-h3 text-on-surface">Messages</h2>
        <div className="flex items-center gap-1">
          <IconButton
            onClick={() => {}}
            aria-label="Filter"
            title="Filter"
            size="sm"
          >
            <SlidersHorizontal size={20} />
          </IconButton>
          <IconButton
            onClick={() => setShowCreate(true)}
            aria-label="Create channel"
            title="Create channel"
            size="sm"
          >
            <Plus size={20} />
          </IconButton>
        </div>
      </div>

      {/* Search */}
      <div className="px-2 py-1.5">
        <div className="flex items-center gap-1.5 px-2 py-1 bg-surface-container rounded-sm
          border border-border/30 focus-within:border-primary/40 transition-colors duration-fast">
          <Search size={12} className="text-on-surface-variant flex-shrink-0" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search chats..."
            className="bg-transparent border-none outline-none text-caption text-on-surface
              placeholder:text-on-surface-variant w-full"
          />
        </div>
      </div>

      {/* Grouped channel list — Stitch: p-space-sm (8px) */}
      <div className="flex-1 overflow-y-auto p-2">
        {sections.map((section) => (
          <div key={section.key}>
            {/* Pinned section: horizontal scroll on mobile, collapsible list on desktop */}
            {section.key === 'pinned' && section.channels.length > 0 ? (
              <>
                {/* Mobile: horizontal avatar scroll */}
                <div className="lg:hidden">
                  <div className="px-3 py-1 flex items-center gap-2 text-on-surface-variant mb-1">
                    <Pin size={16} />
                    <span className="font-label-caps text-label-caps">PINNED</span>
                  </div>
                  <div className="flex gap-3 px-3 py-2 overflow-x-auto scrollbar-none">
                    {section.channels.map((ch) => {
                      const lastMsg = lastMessages[ch.id]
                      const preview = lastMsg ? lastMsg.content : undefined
                      return (
                        <PinnedAvatarItem
                          key={ch.id}
                          channel={ch}
                          preview={preview}
                          unreadCount={unreadMap[ch.id] || 0}
                          isActive={ch.id === activeChannelId}
                          onClick={handleSelect}
                        />
                      )
                    })}
                  </div>
                </div>

                {/* Desktop: standard collapsible section */}
                <div className="hidden lg:block">
                  <button
                    onClick={() => toggleSection(section.key)}
                    className="w-full flex items-center gap-1.5 px-3 py-1
                      text-on-surface-variant
                      hover:bg-surface-container transition-colors duration-fast
                      border-none bg-transparent cursor-pointer"
                  >
                    {collapsedSections[section.key] ? (
                      <ChevronRight size={16} className="flex-shrink-0" />
                    ) : (
                      <ChevronDown size={16} className="flex-shrink-0" />
                    )}
                    <span className="font-label-caps text-label-caps">{section.label}</span>
                  </button>
                  {!collapsedSections[section.key] &&
                    section.channels.map((ch) => {
                      const lastMsg = lastMessages[ch.id]
                      const preview = lastMsg
                        ? `${lastMsg.senderName ? lastMsg.senderName + ': ' : ''}${lastMsg.content}`
                        : undefined
                      const ts = lastMsg?.timestamp
                      const timeLabel = ts ? formatRelativeTime(new Date(ts)) : undefined
                      return (
                        <ChatListItem
                          key={ch.id}
                          id={ch.id}
                          name={ch.name}
                          channelType={ch.channel_type}
                          preview={preview}
                          timestamp={timeLabel}
                          unreadCount={unreadMap[ch.id] || 0}
                          isActive={ch.id === activeChannelId}
                          isStarred={starredChannels.includes(ch.id)}
                          isOnline={ch.channel_type === 'dm' ? isDmOnline(ch, onlineUsers) : undefined}
                          onClick={handleSelect}
                        />
                      )
                    })}
                </div>
              </>
            ) : (
              <>
                {/* Standard collapsible section */}
                <button
                  onClick={() => toggleSection(section.key)}
                  className="w-full flex items-center gap-2 px-3 py-1
                    text-on-surface-variant
                    hover:bg-surface-container transition-colors
                    border-none bg-transparent cursor-pointer"
                >
                  {collapsedSections[section.key] ? (
                    <ChevronRight size={16} className="flex-shrink-0" />
                  ) : (
                    <ChevronDown size={16} className="flex-shrink-0" />
                  )}
                  <span className="font-label-caps text-label-caps">{section.label}</span>
                </button>

                {/* Section items */}
                {!collapsedSections[section.key] &&
                  section.channels.map((ch) => {
                    const lastMsg = lastMessages[ch.id]
                    const preview = lastMsg
                      ? `${lastMsg.senderName ? lastMsg.senderName + ': ' : ''}${lastMsg.content}`
                      : undefined
                    const ts = lastMsg?.timestamp
                    const timeLabel = ts
                      ? formatRelativeTime(new Date(ts))
                      : undefined
                    return (
                      <ChatListItem
                        key={ch.id}
                        id={ch.id}
                        name={ch.name}
                        channelType={ch.channel_type}
                        preview={preview}
                        timestamp={timeLabel}
                        unreadCount={unreadMap[ch.id] || 0}
                        isActive={ch.id === activeChannelId}
                        isStarred={starredChannels.includes(ch.id)}
                        isOnline={ch.channel_type === 'dm' ? isDmOnline(ch, onlineUsers) : undefined}
                        onClick={handleSelect}
                      />
                    )
                  })}

                {/* Empty section hint */}
                {!collapsedSections[section.key] && section.channels.length === 0 && (
                  <div className="px-4 py-2">
                    <span className="text-micro text-on-surface-variant italic">
                      {section.key === 'dms' ? 'No conversations yet' : 'No channels yet'}
                    </span>
                  </div>
                )}
              </>
            )}
          </div>
        ))}
        {sections.length === 0 && (
          <div className="px-4 py-6 text-center">
            <span className="text-caption text-on-surface-variant">
              {searchQuery ? 'No matching chats' : 'No channels yet'}
            </span>
          </div>
        )}
      </div>

      {showCreate && <CreateChannelModal onClose={() => setShowCreate(false)} />}
    </div>
  )
}

/** Mobile pinned channel — circular avatar + name + preview text, matching Stitch horizontal scroll. */
function PinnedAvatarItem({
  channel,
  preview,
  unreadCount,
  isActive,
  onClick,
}: {
  channel: Channel
  preview?: string
  unreadCount: number
  isActive: boolean
  onClick: (id: string) => void
}) {
  const hasUnread = unreadCount > 0

  return (
    <button
      onClick={() => onClick(channel.id)}
      className={`flex flex-col items-center gap-1 min-w-[72px] max-w-[80px] py-1.5 px-1
        rounded-lg border-none bg-transparent cursor-pointer transition-colors
        ${isActive ? 'bg-primary-fixed' : 'hover:bg-surface-container'}`}
    >
      <div className="relative">
        <Avatar name={channel.name} size="lg" />
        {hasUnread && (
          <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1
            rounded-full bg-error text-white text-[9px] font-bold
            flex items-center justify-center">
            {unreadCount > 9 ? '+' : unreadCount}
          </span>
        )}
      </div>
      <span className="text-micro text-on-surface truncate w-full text-center leading-tight">
        {channel.name}
      </span>
      {preview && (
        <span className="text-[10px] text-on-surface-variant truncate w-full text-center leading-tight">
          {preview}
        </span>
      )}
    </button>
  )
}

/** Format timestamp as relative time (Just now, 10:42 AM, Yesterday, Mon, etc.) */
function formatRelativeTime(date: Date): string | undefined {
  if (isNaN(date.getTime())) return undefined
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMin = Math.floor(diffMs / 60000)
  const diffHour = Math.floor(diffMin / 60)
  const diffDay = Math.floor(diffHour / 24)

  if (diffMin < 1) return 'Just now'
  if (diffDay === 0) {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  if (diffDay === 1) return 'Yesterday'
  if (diffDay < 7) {
    return date.toLocaleDateString([], { weekday: 'short' })
  }
  return date.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

/** Check if the other participant in a DM channel is online.
 *  DM names are the other user's display name — match against onlineUsers values (usernames). */
function isDmOnline(channel: Channel, onlineUsers: Record<string, string>): boolean {
  const onlineNames = Object.values(onlineUsers)
  return onlineNames.some((name) => name === channel.name)
}
