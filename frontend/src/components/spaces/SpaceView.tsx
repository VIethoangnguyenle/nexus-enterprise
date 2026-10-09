import { useCallback, useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence } from 'motion/react'
import { ArrowLeft, Info, Search, UserPlus, CircleAlert, Lock } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useConversations } from '../../hooks/useConversations'
import { usePeople } from '../../hooks/usePeople'
import {
  useMarkRead, useMessages, useSendMessage, useTasks, useTogglePin, useToggleReaction, useChannelMembers,
} from '../../hooks/useMessaging'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { messagingApi, type Message } from '../../api/messaging'
import { driveApi } from '../../api/drive'
import { statusOf, explain } from '../../lib/errors'
import { dmPartner, type Conversation } from '../../lib/conversations'
import { displayName } from '../../lib/people'
import { Avatar, Button, Heading, IconButton, Pressable, TabBar, toast } from '../primitives'
import { ChatEditor } from '../chat/ChatEditor'
import { EmojiPicker } from '../chat/EmojiPicker'
import { ThreadPanel } from '../chat/ThreadPanel'
import { ConversationIcon } from './ConversationIcon'
import { TopicStream } from './TopicStream'
import { MembersPanel } from './MembersPanel'
import { SpaceFiles } from './SpaceFiles'
import { SpaceTasks } from './SpaceTasks'
import { SpaceSearchPanel } from './SpaceSearchPanel'
import { SpaceInfoPanel } from './SpaceInfoPanel'
import { EmptyState } from './EmptyState'

type Tab = 'chat' | 'files' | 'tasks'
type Panel =
  | { kind: 'thread'; id: string }
  | { kind: 'members'; adding?: boolean }
  | { kind: 'search' }
  | { kind: 'info' }

/**
 * A space or DM (design/mockups/spaces.html §2): header, tabs Trò chuyện / Tệp /
 * Công việc, the topic stream with its composer, and one right panel at a
 * time (thread, members, search, info). Esc closes the panel.
 */
export function SpaceView({ channelId }: { channelId: string }) {
  const { workspaceId } = useActiveWorkspace()
  const { conversations, isLoading: listLoading } = useConversations(workspaceId)
  const people = usePeople(workspaceId)
  const me = useAuthStore((s) => s.user)
  const [tab, setTab] = useState<Tab>('chat')
  const [panel, setPanel] = useState<Panel | null>(null)
  const [emojiTarget, setEmojiTarget] = useState<string | null>(null)

  // Title, kind and counts come from the lists already loaded for the
  // navigator; a channel opened by link before they arrive is fetched alone.
  const listed = conversations.find((c) => c.id === channelId)
  const single = useQuery({
    queryKey: ['channel', channelId],
    queryFn: () => messagingApi.getChannel(channelId),
    enabled: !listLoading && !listed,
  })
  const conv: Conversation | undefined = useMemo(() => {
    if (listed) return listed
    const ch = single.data
    if (!ch) return undefined
    const dm = ch.channel_type === 'dm'
    const p = dm ? dmPartner(ch, { username: me?.username }, people) : undefined
    return {
      id: ch.id, kind: dm ? 'dm' : 'space', title: p?.title ?? ch.name, partner: p?.partner,
      hueKey: p?.partner?.userId || ch.id, unread: 0, memberCount: ch.member_count ?? 0, lastActivity: 0,
    }
  }, [listed, single.data, me?.username, people])

  const messagesQ = useMessages(channelId)
  const send = useSendMessage(channelId)
  const toggleReaction = useToggleReaction(channelId)
  const togglePin = useTogglePin(channelId)
  const markRead = useMarkRead(channelId)
  const tasksQ = useTasks(channelId)
  const membersQ = useChannelMembers(channelId)
  const typingUsers = useWebSocketStore((s) => s.typingUsers[channelId])
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const sendTyping = useWebSocketStore((s) => s.sendTyping)
  const sendSubscribe = useWebSocketStore((s) => s.sendSubscribe)
  const sendUnsubscribe = useWebSocketStore((s) => s.sendUnsubscribe)

  const messages = useMemo(() => [...(messagesQ.data?.messages ?? [])].reverse(), [messagesQ.data])

  useEffect(() => {
    sendSubscribe(channelId)
    return () => sendUnsubscribe(channelId)
  }, [channelId, sendSubscribe, sendUnsubscribe])

  // Mark read a second after the newest persisted message settles. The
  // optimistic placeholder (`temp-…`) is not a row in messages, and
  // read_receipts has a foreign key onto that table, so it is skipped.
  const lastPersisted = [...messages].reverse().find(
    (m) => m.id && !(m as { _optimistic?: boolean })._optimistic && !m.id.startsWith('temp-'),
  )?.id
  useEffect(() => {
    if (!lastPersisted) return
    const timer = setTimeout(() => markRead.mutate(lastPersisted), 1000)
    return () => clearTimeout(timer)
    // markRead is a fresh object every render; the message id is the trigger.
  }, [lastPersisted, channelId])

  // Esc closes the side panel (dialogs and menus stop the event before it gets here).
  useEffect(() => {
    if (!panel) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) setPanel(null)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [panel])

  const handleSend = useCallback(
    (content: string) => {
      if (!content.trim()) return
      send.mutate({ content }, { onError: (err) => toast.error(explain(err, 'gửi tin nhắn')) })
    },
    [send],
  )

  const handleFileUpload = useCallback(
    async (file: File) => {
      if (!workspaceId) return
      const drive = await driveApi.channelDrive(workspaceId, channelId)
      const folder = drive?.items?.find((i) => i.item_type === 'folder')
      const created = await driveApi.createFile(workspaceId, file.name, file.type || 'application/octet-stream', file.size, folder?.id)
      await driveApi.uploadToStorage(created.upload_url, file)
      await driveApi.confirmFile(created.file_id)
      send.mutate({ content: `📎 ${file.name}`, linkedEntity: { type: 'drive_file', id: created.file_id } })
    },
    [workspaceId, channelId, send],
  )

  const status = statusOf(messagesQ.error)
  if (messagesQ.error && (status === 403 || status === 404)) {
    return (
      <div className="flex-1 grid place-items-center bg-base">
        <EmptyState
          icon={<Lock size={24} strokeWidth={1.75} />}
          text={status === 403 ? 'Bạn không ở trong cuộc trò chuyện này. Nhờ một thành viên thêm bạn vào.' : 'Cuộc trò chuyện này không còn nữa.'}
        />
      </div>
    )
  }

  const kind = conv?.kind ?? 'space'
  const title = conv?.title ?? ''
  const memberCount = conv?.memberCount ?? membersQ.data?.members?.length ?? 0
  const openTasks = (tasksQ.data?.tasks ?? []).filter((t) => t.status !== 'done').length
  const onlineMembers = (membersQ.data?.members ?? []).filter((mm) => mm.user_id !== me?.id && onlineUsers[mm.user_id])
  const typingNames = (typingUsers ?? []).map((u) => displayName(people, undefined, u))
  const idPrefix = `space-${channelId}`

  return (
    <div className="relative flex flex-1 min-h-0 min-w-0">
      <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label={title || 'Cuộc trò chuyện'}>
        <header className="flex items-center gap-3 px-5 pt-3.5 pb-1.5 min-w-0">
          <IconButton
            className="lg:hidden"
            aria-label="Về danh sách trò chuyện"
            onClick={() => window.dispatchEvent(new CustomEvent('open-mobile-list'))}
          >
            <ArrowLeft size={18} strokeWidth={1.75} />
          </IconButton>
          {conv ? <ConversationIcon conversation={conv} size={40} /> : <div className="skeleton w-10 h-10 rounded-surface" />}
          <div className="grid gap-px min-w-0">
            {conv ? (
              <Heading as="h1" look="panel" className="truncate">{title}</Heading>
            ) : (
              <div className="skeleton h-5 w-40 rounded-sm" />
            )}
            {kind === 'space' ? (
              <Pressable
                onClick={() => setPanel({ kind: 'members' })}
                className="justify-self-start text-small text-ink-muted hover:text-ink rounded-sm tnum"
              >
                {memberCount} thành viên
              </Pressable>
            ) : (
              <span className="text-small text-ink-muted truncate">{conv?.partner?.role || 'Tin nhắn trực tiếp'}</span>
            )}
          </div>
          <div className="ml-auto flex items-center gap-1.5 shrink-0">
            {onlineMembers.length > 0 && (
              <span className="hidden md:flex items-center mr-1.5" aria-label={`${onlineMembers.length} người đang trực tuyến`}>
                {onlineMembers.slice(0, 3).map((mm, i) => (
                  <Avatar
                    key={mm.user_id}
                    name={displayName(people, mm.user_id, mm.username)}
                    hueKey={mm.user_id}
                    size={24}
                    className={`ring-2 ring-base animate-fade-in ${i > 0 ? '-ml-1.5' : ''}`}
                  />
                ))}
                {onlineMembers.length > 3 && (
                  <span className="ml-1 text-xs text-ink-muted tnum">+{onlineMembers.length - 3}</span>
                )}
              </span>
            )}
            {kind === 'space' && (
              <Button variant="soft" size="sm" onClick={() => setPanel({ kind: 'members', adding: true })}>
                <UserPlus size={16} strokeWidth={1.75} aria-hidden="true" />
                <span className="max-sm:sr-only">Thêm người</span>
              </Button>
            )}
            <IconButton
              aria-label={kind === 'space' ? 'Tìm trong nhóm' : 'Tìm trong cuộc trò chuyện'}
              title="Tìm kiếm"
              active={panel?.kind === 'search'}
              onClick={() => setPanel(panel?.kind === 'search' ? null : { kind: 'search' })}
            >
              <Search size={18} strokeWidth={1.75} />
            </IconButton>
            <IconButton
              aria-label={kind === 'space' ? 'Thông tin nhóm' : 'Thông tin'}
              title="Thông tin"
              active={panel?.kind === 'info'}
              onClick={() => setPanel(panel?.kind === 'info' ? null : { kind: 'info' })}
            >
              <Info size={18} strokeWidth={1.75} />
            </IconButton>
          </div>
        </header>

        <TabBar
          label="Nội dung"
          idPrefix={idPrefix}
          value={tab}
          onChange={(t) => setTab(t as Tab)}
          className="px-5 pt-1"
          tabs={[
            { id: 'chat', label: 'Trò chuyện' },
            { id: 'files', label: 'Tệp' },
            {
              id: 'tasks',
              label: 'Công việc',
              badge: openTasks > 0 ? (
                <span className="inline-flex items-center h-5 px-1.5 rounded-full bg-sunk text-ink-muted text-xs font-semibold tnum">
                  {openTasks}
                </span>
              ) : undefined,
            },
          ]}
        />
        <div className="h-px bg-line mx-5" />

        <div
          role="tabpanel"
          id={`${idPrefix}-panel-${tab}`}
          aria-labelledby={`${idPrefix}-tab-${tab}`}
          className="flex-1 flex flex-col min-h-0 animate-fade-in"
          key={tab}
        >
          {tab === 'chat' && (
            <>
              {messagesQ.isError ? (
                <div className="flex-1 grid place-items-center">
                  <EmptyState
                    icon={<CircleAlert size={24} strokeWidth={1.75} />}
                    text="Chưa tải được tin nhắn."
                    action={<Button variant="soft" size="sm" onClick={() => messagesQ.refetch()}>Thử lại</Button>}
                  />
                </div>
              ) : (
                <TopicStream
                  channelId={channelId}
                  messages={messages}
                  ready={!messagesQ.isLoading}
                  me={me?.id}
                  people={people}
                  activeThreadId={panel?.kind === 'thread' ? panel.id : null}
                  onOpenThread={(id) => setPanel({ kind: 'thread', id })}
                  onReact={(id) => setEmojiTarget(id)}
                  onToggleReaction={(id, emoji, hasReacted) => toggleReaction.mutate({ messageId: id, emoji, hasReacted })}
                  onPin={(m: Message) => togglePin.mutate({ messageId: m.id, isPinned: !!m.is_pinned })}
                />
              )}

              {emojiTarget && (
                <div className="absolute bottom-28 left-1/2 -translate-x-1/2 z-dropdown">
                  <EmojiPicker
                    onSelect={(emoji) => {
                      toggleReaction.mutate({ messageId: emojiTarget, emoji, hasReacted: false })
                      setEmojiTarget(null)
                    }}
                    onClose={() => setEmojiTarget(null)}
                  />
                </div>
              )}

              <div className="h-6 px-6 flex items-center text-xs text-ink-muted" aria-live="polite">
                {typingNames.length > 0 && <TypingLine names={typingNames} />}
              </div>

              <ChatEditor
                channelId={channelId}
                people={people}
                placeholder={title ? `Tin nhắn cho ${title}` : 'Viết tin nhắn'}
                hint={kind === 'space' ? 'Mỗi tin gửi ở đây mở một chủ đề mới' : undefined}
                onSend={handleSend}
                onTyping={() => sendTyping(channelId)}
                onFileUpload={handleFileUpload}
                isPending={send.isPending}
              />
            </>
          )}
          {tab === 'files' && (
            <div className="flex-1 min-h-0 overflow-y-auto">
              <SpaceFiles workspaceId={workspaceId} channelId={channelId} people={people} onGoToChat={() => setTab('chat')} />
            </div>
          )}
          {tab === 'tasks' && (
            <div className="flex-1 min-h-0 overflow-y-auto">
              <SpaceTasks channelId={channelId} people={people} />
            </div>
          )}
        </div>
      </section>

      <AnimatePresence mode="wait">
        {panel?.kind === 'thread' && (
          <ThreadPanel
            key="thread"
            channelId={channelId}
            messageId={panel.id}
            spaceName={title}
            people={people}
            me={me?.id}
            onClose={() => setPanel(null)}
          />
        )}
        {panel?.kind === 'members' && (
          <MembersPanel
            key="members"
            channelId={channelId}
            kind={kind}
            spaceName={title}
            people={people}
            me={me?.id}
            startAdding={panel.adding}
            onClose={() => setPanel(null)}
          />
        )}
        {panel?.kind === 'search' && (
          <SpaceSearchPanel
            key="search"
            channelId={channelId}
            title={title}
            people={people}
            onOpenTopic={(id) => {
              setTab('chat')
              setPanel({ kind: 'thread', id })
            }}
            onClose={() => setPanel(null)}
          />
        )}
        {panel?.kind === 'info' && conv && (
          <SpaceInfoPanel
            key="info"
            channelId={channelId}
            kind={kind}
            title={title}
            memberCount={memberCount}
            partner={conv.partner}
            people={people}
            onOpenTopic={(id) => {
              setTab('chat')
              setPanel({ kind: 'thread', id })
            }}
            onClose={() => setPanel(null)}
          />
        )}
      </AnimatePresence>
    </div>
  )
}

function TypingLine({ names }: { names: string[] }) {
  const who =
    names.length === 1 ? names[0] : names.length === 2 ? `${names[0]} và ${names[1]}` : `${names[0]} và ${names.length - 1} người khác`
  return (
    <span className="inline-flex items-center gap-2">
      <span className="inline-flex gap-0.5" aria-hidden="true">
        <span className="w-1 h-1 rounded-full bg-ink-muted animate-pulse" />
        <span className="w-1 h-1 rounded-full bg-ink-muted animate-pulse [animation-delay:160ms]" />
        <span className="w-1 h-1 rounded-full bg-ink-muted animate-pulse [animation-delay:320ms]" />
      </span>
      {who} đang nhập…
    </span>
  )
}
