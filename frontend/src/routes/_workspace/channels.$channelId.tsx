import { createFileRoute } from '@tanstack/react-router'
import { useMessages, useSendMessage, useToggleReaction, useTogglePin, useMarkRead, useChannels } from '../../hooks/useMessaging'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useAuthStore } from '../../stores/auth.store'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { driveApi } from '../../api/drive'
import { useState, useEffect, useCallback } from 'react'

import { ErrorState } from '../../components/ErrorState'
import { ChatEditor, ChannelInfoPanel, VirtualizedMessageList, ThreadPanel } from '../../components/chat'
import { EmojiPicker } from '../../components/chat/EmojiPicker'
import { Search, Pin, Users, FolderOpen, MoreVertical, ArrowLeft } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export const Route = createFileRoute('/_workspace/channels/$channelId')({ component: ChannelChatView })

function ChannelChatView() {
  const { channelId } = Route.useParams()
  const { data, isLoading, error, refetch } = useMessages(channelId)
  const { data: wsData } = useWorkspaces()
  const wsParam = new URLSearchParams(window.location.search).get('ws')
  const wsId = (wsParam && wsData?.workspaces?.find(w => w.id === wsParam)?.id) || wsData?.workspaces?.[0]?.id || ''
  const { data: channelsData } = useChannels(wsId)
  const currentChannel = channelsData?.channels?.find(c => c.id === channelId)
  const channelName = currentChannel?.name || 'Channel'
  const send = useSendMessage(channelId)
  const toggleReaction = useToggleReaction(channelId)
  const togglePin = useTogglePin(channelId)
  const markRead = useMarkRead(channelId)
  const typingUsers = useWebSocketStore(s => s.typingUsers)
  const sendTyping = useWebSocketStore(s => s.sendTyping)
  const sendSubscribe = useWebSocketStore(s => s.sendSubscribe)
  const sendUnsubscribe = useWebSocketStore(s => s.sendUnsubscribe)
  const [threadId, setThreadId] = useState<string | null>(null)
  const [showInfoPanel, setShowInfoPanel] = useState(false)
  const [infoPanelTab, setInfoPanelTab] = useState('members')
  const [emojiTarget, setEmojiTarget] = useState<string | null>(null)
  const msgs = [...(data?.messages || [])].reverse()
  const currentUserId = useAuthStore(s => s.user?.id ?? '')

  // Subscribe to WS channel for real-time message delivery.
  useEffect(() => {
    sendSubscribe(channelId)
    return () => sendUnsubscribe(channelId)
  }, [channelId, sendSubscribe, sendUnsubscribe])

  // Auto mark-as-read (debounced)
  useEffect(() => {
    if (msgs.length === 0) return
    const lastMsg = msgs[msgs.length - 1]
    const timer = setTimeout(() => markRead.mutate(lastMsg.id), 1000)
    return () => clearTimeout(timer)
  }, [msgs.length, channelId])

  const handleSend = useCallback((content: string, mentions: string[]) => {
    if (!content.trim()) return
    send.mutate({ content, linkedEntity: undefined })
  }, [send])

  const handleFileUpload = useCallback(async (file: File) => {
    if (!wsId) return
    const driveData = await driveApi.channelDrive(wsId, channelId)
    const rootFolder = driveData?.items?.find(i => i.item_type === 'folder')
    const parentId = rootFolder?.id || undefined
    const created = await driveApi.createFile(wsId, file.name, file.type || 'application/octet-stream', file.size, parentId)
    await driveApi.uploadToStorage(created.upload_url, file)
    await driveApi.confirmFile(created.file_id)
    send.mutate({ content: `📎 ${file.name}`, linkedEntity: { type: 'drive_file', id: created.file_id } })
  }, [wsId, channelId, send])

  const openInfoTab = (tab: string) => {
    setInfoPanelTab(tab)
    setShowInfoPanel(true)
    setThreadId(null)
  }

  if (error) return <ErrorState title="Failed to load messages" message={error.message} onRetry={() => refetch()} />

  return (
    <div className="flex flex-1 min-h-0 overflow-hidden">
      {/* Main chat column */}
      <div className={`flex-1 flex flex-col min-w-0 bg-surface-container-lowest ${threadId || showInfoPanel ? 'border-r border-outline-variant' : ''}`}>
        <ChannelHeader
          channelName={channelName}
          memberCount={currentChannel?.member_count}
          topic={currentChannel?.topic}
          showInfoPanel={showInfoPanel}
          infoPanelTab={infoPanelTab}
          onOpenInfoTab={openInfoTab}
        />

        <VirtualizedMessageList
          messages={msgs}
          isLoading={isLoading}
          currentUserId={currentUserId}
          onReply={(id) => setThreadId(id)}
          onReact={(id) => setEmojiTarget(id)}
          onPin={(m) => togglePin.mutate({ messageId: m.id, isPinned: !!m.is_pinned })}
          onToggleReaction={(msgId, emoji, hasReacted) =>
            toggleReaction.mutate({ messageId: msgId, emoji, hasReacted })
          }
        />

        {/* Emoji picker overlay */}
        {emojiTarget && (
          <div className="absolute bottom-24 left-1/2 -translate-x-1/2 z-50">
            <EmojiPicker
              onSelect={(emoji) => {
                toggleReaction.mutate({ messageId: emojiTarget, emoji, hasReacted: false })
                setEmojiTarget(null)
              }}
              onClose={() => setEmojiTarget(null)}
            />
          </div>
        )}

        {/* Typing indicator — animated dots */}
        {typingUsers[channelId] && typingUsers[channelId].length > 0 && (
          <div className="px-4 md:px-6 py-1.5 flex items-center gap-2 text-xs text-on-surface-variant animate-fade-in">
            <span className="flex items-center gap-0.5">
              <span className="w-1.5 h-1.5 rounded-full bg-on-surface-variant animate-bounce [animation-delay:0ms]" />
              <span className="w-1.5 h-1.5 rounded-full bg-on-surface-variant animate-bounce [animation-delay:150ms]" />
              <span className="w-1.5 h-1.5 rounded-full bg-on-surface-variant animate-bounce [animation-delay:300ms]" />
            </span>
            <span>{formatTypingIndicator(typingUsers[channelId])}</span>
          </div>
        )}

        <ChatEditor
          channelId={channelId}
          onSend={handleSend}
          onTyping={() => sendTyping(channelId)}
          onFileUpload={handleFileUpload}
          isPending={send.isPending}
          error={send.error?.message ?? null}
        />
      </div>

      {/* Side panels */}
      {threadId && <ThreadPanel messageId={threadId} channelId={channelId} onClose={() => setThreadId(null)} />}
      {showInfoPanel && !threadId && (
        <ChannelInfoPanel
          channelId={channelId}
          channelName={channelName}
          wsId={wsId}
          onClose={() => setShowInfoPanel(false)}
          initialTab={infoPanelTab}
        />
      )}
    </div>
  )
}

/** Channel header with title, member count, action buttons, and tabs. */
function ChannelHeader({
  channelName,
  memberCount,
  topic,
  showInfoPanel,
  infoPanelTab,
  onOpenInfoTab,
}: {
  channelName: string
  memberCount?: number
  topic?: string
  showInfoPanel: boolean
  infoPanelTab: string
  onOpenInfoTab: (tab: string) => void
}) {
  return (
    <div className="bg-surface-container-lowest/80 backdrop-blur-md border-b border-outline-variant/20 shrink-0 sticky top-0 z-10">
      {/* Row 1: Identity */}
      <div className="flex items-center justify-between h-12 md:h-[72px] px-3 md:px-6">
        <div className="flex items-center gap-3 md:gap-4">
          <button
            onClick={() => window.dispatchEvent(new CustomEvent('open-mobile-list'))}
            className="lg:hidden min-h-9 min-w-9 flex items-center justify-center rounded-lg
              bg-transparent border-none cursor-pointer text-on-surface-variant hover:text-on-surface
              hover:bg-surface-container transition-colors"
          >
            <ArrowLeft size={20} />
          </button>
          <div className="w-8 h-8 md:w-12 md:h-12 rounded-lg md:rounded-xl bg-primary text-on-primary flex items-center justify-center font-bold shadow-sm">
            <span className="text-sm md:text-xl">#</span>
          </div>
          <div className="flex flex-col">
            <h1 className="text-sm md:text-h3 text-on-surface mb-0 md:mb-0.5">{channelName}</h1>
            {(memberCount != null || topic) && (
              <div className="flex items-center gap-2 text-on-surface-variant text-xs hidden md:flex">
                {memberCount != null && memberCount > 0 && (
                  <span className="flex items-center gap-1"><Users size={14} /> {memberCount} Members</span>
                )}
                {topic && (<><span>•</span><span>{topic}</span></>)}
              </div>
            )}
            {memberCount != null && memberCount > 0 && (
              <span className="text-micro text-on-surface-variant md:hidden">{memberCount} members</span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <HeaderButton icon={Users} label="Members" active={showInfoPanel && infoPanelTab === 'members'} onClick={() => onOpenInfoTab('members')} />
          <HeaderButton icon={Search} label="Search" active={showInfoPanel && infoPanelTab === 'search'} onClick={() => onOpenInfoTab('search')} />
          <button
            onClick={() => {}}
            title="More"
            className="p-2 rounded-lg border-none bg-transparent cursor-pointer text-on-surface-variant hover:bg-surface-container transition-colors"
          >
            <MoreVertical size={20} />
          </button>
        </div>
      </div>
      {/* Row 2: Tabs — desktop only */}
      <div className="hidden md:flex items-center gap-0.5 h-8 px-6 overflow-x-auto scrollbar-none">
        <button className="inline-flex items-center gap-1 py-1.5 px-3.5 rounded-[20px] text-[13px] font-medium leading-[1.4] border border-transparent cursor-pointer transition-all duration-fast bg-primary text-on-primary">Chat</button>
        <button
          className={`inline-flex items-center gap-1 py-1.5 px-3.5 rounded-[20px] text-[13px] font-medium leading-[1.4] border border-transparent cursor-pointer transition-all duration-fast hover:bg-surface-container-high hover:text-on-surface ${showInfoPanel && infoPanelTab === 'pins' ? 'bg-primary text-on-primary' : 'bg-transparent text-on-surface-variant'}`}
          onClick={() => onOpenInfoTab('pins')}
        >
          <Pin size={12} /> Pinned
        </button>
        <button
          className={`inline-flex items-center gap-1 py-1.5 px-3.5 rounded-[20px] text-[13px] font-medium leading-[1.4] border border-transparent cursor-pointer transition-all duration-fast hover:bg-surface-container-high hover:text-on-surface ${showInfoPanel && infoPanelTab === 'files' ? 'bg-primary text-on-primary' : 'bg-transparent text-on-surface-variant'}`}
          onClick={() => onOpenInfoTab('files')}
        >
          <FolderOpen size={12} /> Files
        </button>
      </div>
    </div>
  )
}

/** Header action button — Stitch: p-2 rounded-lg hover:bg-surface-container */
function HeaderButton({ icon: Icon, label, active, onClick }: { icon: LucideIcon; label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      title={label}
      className={`p-2 rounded-lg border-none cursor-pointer transition-colors
        ${active
          ? 'bg-surface-container text-on-surface'
          : 'bg-transparent text-on-surface-variant hover:bg-surface-container'}`}
    >
      <Icon size={20} />
    </button>
  )
}

/** Format typing indicator text for 1, 2, or 3+ users. */
function formatTypingIndicator(users: string[]): string {
  if (users.length === 1) return `${users[0]} is typing...`
  if (users.length === 2) return `${users[0]} and ${users[1]} are typing...`
  return `${users[0]} and ${users.length - 1} others are typing...`
}
