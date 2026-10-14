import { useNavigate } from '@tanstack/react-router'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence } from 'motion/react'
import { ArrowLeft, Info, MessagesSquare, Search, UserPlus, CircleAlert, Lock, Upload } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useConversations } from '../../hooks/useConversations'
import { usePeople } from '../../hooks/usePeople'
import { usePhone } from '../../hooks/usePhone'
import { useMessageActions } from '../../hooks/useMessageActions'
import { useFocusMessage } from '../../hooks/useFocusMessage'
import { useDelayedFlag } from '../../hooks/useDelayedFlag'
import { useNotificationLevel } from '../../hooks/useNotificationLevel'
import { useStarConversation } from '../../hooks/useShortcuts'
import { lastEditableOf, toEditorHtml } from '../../lib/message-model'
import { useMarkRead, useChannelMembers, useUnreadCounts } from '../../hooks/useMessaging'
import { useFollowThread, useLoadOlderMessages, useThreadsList } from '../../hooks/useThreads'
import { useMessages, useSendMessage, useThread, useTogglePin, useToggleReaction } from '../../hooks/useMessages'
import { useTasks } from '../../hooks/useChannelTasks'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { messagingApi, type Message } from '../../api/messaging'
import { useAttachmentUploads } from '../../hooks/useAttachmentUploads'
import { statusOf, explain } from '../../lib/errors'
import { dmPartner, type Conversation } from '../../lib/conversations'
import { avatarOf, displayName } from '../../lib/people'
import { Avatar, Button, IconButton, Pressable, Spinner, TabBar, toast } from '../primitives'
import { ChatEditor } from '../chat/ChatEditor'
import { EmojiPicker } from '../chat/EmojiPicker'
import { ThreadPanel } from '../chat/ThreadPanel'
import { ConversationIcon } from './ConversationIcon'
import { ConversationTitleMenu } from './ConversationTitleMenu'
import { ConversationStream } from './ConversationStream'
import { ThreadsPanel } from './ThreadsPanel'
import { MembersPanel } from './MembersPanel'
import { SpaceFiles } from './SpaceFiles'
import { SpaceTasks } from './SpaceTasks'
import { SpaceSearchPanel } from './SpaceSearchPanel'
import { SpaceInfoPanel } from './SpaceInfoPanel'
import { SelfMentionStyle } from './SelfMentionStyle'
import { EmptyState } from './EmptyState'
import { CallHeaderActions } from '../calls/CallHeaderActions'
import { keys } from '../../hooks/keys'

type Tab = 'chat' | 'files' | 'tasks'
type Panel =
  | { kind: 'thread'; id: string; /** Opened from the thread list: the panel can go back to it. */ fromList?: boolean }
  | { kind: 'threads' }
  | { kind: 'members'; adding?: boolean }
  | { kind: 'search' }
  | { kind: 'info' }

/**
 * A space or DM (design/mockups/spaces.html §2–§4): header, tabs Trò chuyện / Tệp /
 * Công việc, the in-line stream with its composer, and one right panel at a
 * time (thread, thread list, members, search, info). Esc closes the panel.
 *
 * A link can name where to land: `initialThreadId` opens that thread's panel,
 * `focusMessageId` brings a message to the middle and outlines it for 2.4 s, in
 * the panel when a thread is named and in the stream otherwise.
 */
export function SpaceView({ channelId, initialThreadId, focusMessageId }: {
  channelId: string
  initialThreadId?: string
  focusMessageId?: string
}) {
  const navigate = useNavigate()
  const { workspaceId } = useActiveWorkspace()
  const { conversations, isLoading: listLoading } = useConversations(workspaceId)
  const people = usePeople(workspaceId)
  const me = useAuthStore((s) => s.user)
  const [tab, setTab] = useState<Tab>('chat')
  const [panel, setPanel] = useState<Panel | null>(() => (initialThreadId ? { kind: 'thread', id: initialThreadId } : null))
  const viewRef = useRef<HTMLDivElement>(null)
  const [emojiTarget, setEmojiTarget] = useState<string | null>(null)

  // Title, kind and counts come from the lists already loaded for the
  // navigator; a channel opened by link before they arrive is fetched alone.
  const listed = conversations.find((c) => c.id === channelId)
  const single = useQuery({
    queryKey: keys.messaging.channel(channelId),
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
      starred: !!ch.starred,
    }
  }, [listed, single.data, me?.username, people])

  const messagesQ = useMessages(channelId)
  const phone = usePhone()
  // The level and the star ride on the unread list the navigator already holds, so no request of their own.
  const notify = useNotificationLevel(channelId, conv?.title ?? '', { fetchLevel: false })
  const star = useStarConversation()
  const savingLate = useDelayedFlag(notify.isSaving, 300)
  const lifecycle = useMessageActions(channelId, me?.id)
  const send = useSendMessage(channelId)
  const toggleReaction = useToggleReaction(channelId)
  const togglePin = useTogglePin(channelId)
  const markRead = useMarkRead(channelId)
  const follow = useFollowThread(channelId)
  const older = useLoadOlderMessages(channelId)
  const unreadQ = useUnreadCounts()
  const unreadThreadsQ = useThreadsList(channelId, 'unread')
  const tasksQ = useTasks(channelId)
  const membersQ = useChannelMembers(channelId)
  const typingUsers = useWebSocketStore((s) => s.typingUsers[channelId])
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const sendTyping = useWebSocketStore((s) => s.sendTyping)
  const sendSubscribe = useWebSocketStore((s) => s.sendSubscribe)
  const sendUnsubscribe = useWebSocketStore((s) => s.sendUnsubscribe)

  // The cache is newest first; the stream reads oldest first. Replies live in their thread.
  const messages = useMemo(
    () => [...(messagesQ.data?.messages ?? [])].reverse().filter((m) => !m.parent_message_id),
    [messagesQ.data],
  )
  const channelUnread = (unreadQ.data?.channels ?? []).find((c) => c.channel_id === channelId)
  const threadsWithNews = unreadThreadsQ.data?.pages.flatMap((p) => p.messages).length ?? 0
  const [atEnd, setAtEnd] = useState(true)

  // A link into this conversation: the thread it names opens, and the stream is the tab that shows the message.
  useEffect(() => {
    if (initialThreadId) setPanel((p) => (p?.kind === 'thread' && p.id === initialThreadId ? p : { kind: 'thread', id: initialThreadId }))
    if (initialThreadId || focusMessageId) setTab('chat')
  }, [initialThreadId, focusMessageId])
  const hasMoreOlder = !!messagesQ.data?.has_more
  const loadOlder = older.loadOlder
  const olderBusy = older.loading || older.failed
  const askOlder = useCallback(() => {
    if (!hasMoreOlder || olderBusy) return false
    void loadOlder()
    return true
  }, [hasMoreOlder, olderBusy, loadOlder])
  // Where the message may be has loaded: the thread, or every page of the stream.
  const threadQ = useThread(initialThreadId ?? '')
  const focusSettled = initialThreadId
    ? threadQ.isSuccess || threadQ.isError
    : (messagesQ.isSuccess && !hasMoreOlder && !older.loading) || messagesQ.isError
  useFocusMessage(viewRef, focusMessageId, initialThreadId ? 'panel' : 'stream', askOlder, {
    settled: focusSettled,
    onMissing: () => toast.info('Không tìm thấy tin nhắn này. Có thể tin đã bị xoá hoặc bạn không còn xem được.'),
  }, () => {
    // The link is served: leaving it in the address would make the same link a no-op the next time.
    if (!initialThreadId && !focusMessageId) return
    void navigate({
      to: '/channels/$channelId',
      params: { channelId },
      search: (prev: { ws?: string }) => ({ ws: prev.ws }),
      replace: true,
    })
  })

  useEffect(() => {
    sendSubscribe(channelId)
    return () => sendUnsubscribe(channelId)
  }, [channelId, sendSubscribe, sendUnsubscribe])

  // This conversation is on screen: no push about it while the page is visible.
  const viewChannel = useWebSocketStore((s) => s.viewChannel)
  useEffect(() => viewChannel(channelId), [channelId, viewChannel])

  // Mark read a second after the newest persisted message settles, once the
  // reader is at the end of the stream: a long unread run opened at "Tin mới"
  // stays unread until they get through it. The optimistic placeholder
  // (`temp-…`) is not a row in messages, and read_receipts has a foreign key
  // onto that table, so it is skipped.
  const lastPersisted = [...messages].reverse().find(
    (m) => m.id && !(m as { _optimistic?: boolean })._optimistic && !m.id.startsWith('temp-'),
  )?.id
  useEffect(() => {
    if (!lastPersisted || !atEnd || tab !== 'chat') return
    const timer = setTimeout(() => markRead.mutate(lastPersisted), 1000)
    return () => clearTimeout(timer)
    // markRead is a fresh object every render; the message id and position are the trigger.
  }, [lastPersisted, channelId, atEnd, tab])

  // Esc closes the side panel (dialogs and menus stop the event before it gets here).
  useEffect(() => {
    if (!panel) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) setPanel(null)
    }
    // On `window`, so it runs after the dialogs' and menus' `document` handlers.
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [panel])

  const uploads = useAttachmentUploads({ workspaceId, channelId })
  const [isDragging, setIsDragging] = useState(false)

  const handleSend = useCallback(
    async (
      content: string,
      mentions: string[],
      attachments?: { drive_file_id: string; width?: number; height?: number }[],
    ) => {
      if (!content.trim() && (!attachments || attachments.length === 0)) return
      const optimisticAttachments = uploads.items
        .filter((it) => it.status === 'done' && it.fileId)
        .map((it) => ({
          drive_file_id: it.fileId!,
          file_name: it.file.name,
          mime_type: it.file.type,
          size_bytes: it.file.size,
          width: it.width,
          height: it.height,
          localUrl: it.file.type.startsWith('image/') ? URL.createObjectURL(it.file) : undefined,
        }))
      try {
        await send.mutateAsync({ content, mentions, attachments, optimisticAttachments })
      } catch (err) {
        toast.error(explain(err, 'gửi tin nhắn'))
        throw err
      }
    },
    [send, uploads.items],
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
  // The unread list is the source of the level and the star; the hook's own answer covers a conversation it does not list yet.
  const level = conv?.level ?? notify.level
  const starred = !!conv?.starred

  return (
    <div ref={viewRef} className="relative flex flex-1 min-h-0 min-w-0">
      <SelfMentionStyle userId={me?.id} />
      <section
        className="flex-1 flex flex-col min-w-0 min-h-0 bg-base relative"
        aria-label={title || 'Cuộc trò chuyện'}
        onDragOver={(e) => {
          if (e.dataTransfer.types.includes('Files')) {
            e.preventDefault()
            setIsDragging(true)
          }
        }}
        onDragLeave={(e) => {
          if (!e.currentTarget.contains(e.relatedTarget as Node)) {
            setIsDragging(false)
          }
        }}
        onDrop={(e) => {
          e.preventDefault()
          setIsDragging(false)
          const files = Array.from(e.dataTransfer.files)
          if (files.length) uploads.add(files)
        }}
      >
        {isDragging && (
          <div className="absolute inset-0 z-dropdown bg-base/80 backdrop-blur-xs border-2 border-dashed border-accent flex flex-col items-center justify-center pointer-events-none text-accent font-semibold text-sm gap-1">
            <Upload size={24} />
            <b>Thả tệp để đính kèm</b>
            <span className="text-xs text-ink-muted font-normal">Tối đa 20 tệp mỗi tin</span>
          </div>
        )}
        <header className="flex items-center gap-3 px-4 md:px-5 pt-3.5 pb-1.5 min-w-0">
          <IconButton
            className="lg:hidden"
            aria-label="Về danh sách trò chuyện"
            onClick={() => void navigate({ to: '/channels' })}
          >
            <ArrowLeft size={18} strokeWidth={1.75} />
          </IconButton>
          {/* A phone header is back + name only (mockup "Trên điện thoại"): the room goes to the name and the call buttons. */}
          <span className="max-sm:hidden inline-flex shrink-0">
            {conv ? <ConversationIcon conversation={conv} size={40} /> : <div className="skeleton w-10 h-10 rounded-surface" />}
          </span>
          <div className="grid gap-px min-w-0 flex-1">
            {conv ? (
              <ConversationTitleMenu
                conversation={conv}
                level={level}
                starred={starred}
                phone={phone}
                onChooseLevel={notify.setLevel}
                onToggleStar={() => void star({ channelId, name: title, starred: !starred })}
                onOpenInfo={() => setPanel({ kind: 'info' })}
              />
            ) : (
              <div className="skeleton h-5 w-40 rounded-sm" />
            )}
            {kind === 'space' ? (
              <Pressable
                onClick={() => setPanel({ kind: 'members' })}
                className="justify-self-start text-small text-ink-muted hover:text-ink rounded-sm tnum"
              >
                {memberCount} thành viên
                {savingLate && <Saving />}
              </Pressable>
            ) : (
              <span className="text-small text-ink-muted truncate">
                {conv?.partner?.role || 'Tin nhắn trực tiếp'}
                {savingLate && <Saving />}
              </span>
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
                    src={avatarOf(people, mm.user_id, mm.username)}
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
              <Button variant="soft" size="sm" className="max-md:hidden" onClick={() => setPanel({ kind: 'members', adding: true })}>
                <UserPlus size={16} strokeWidth={1.75} aria-hidden="true" />
                <span className="max-sm:sr-only">Thêm người</span>
              </Button>
            )}
            <CallHeaderActions channelId={channelId} kind={kind === 'dm' ? 'dm' : 'space'} />
            <IconButton
              aria-label={kind === 'space' ? 'Tìm trong nhóm' : 'Tìm trong cuộc trò chuyện'}
              title="Tìm kiếm"
              data-module-search
              // A conversation has no phone top bar (it steps aside here), so its
              // own header carries search at every width.
              active={panel?.kind === 'search'}
              onClick={() => setPanel(panel?.kind === 'search' ? null : { kind: 'search' })}
            >
              <Search size={18} strokeWidth={1.75} />
            </IconButton>
            <IconButton
              aria-label={threadsWithNews > 0 ? `Thread, ${threadsWithNews} thread có tin mới` : 'Thread'}
              title="Thread"
              active={panel?.kind === 'threads'}
              aria-pressed={panel?.kind === 'threads'}
              onClick={() => setPanel(panel?.kind === 'threads' ? null : { kind: 'threads' })}
              className="relative"
            >
              <MessagesSquare size={18} strokeWidth={1.75} />
              {threadsWithNews > 0 && (
                <span
                  aria-hidden="true"
                  className="absolute -top-0.5 -right-1 inline-flex items-center justify-center min-w-4 h-4 px-1 rounded-full
                    bg-accent text-on-accent text-2xs font-semibold leading-none tnum ring-2 ring-base"
                >
                  {threadsWithNews > 9 ? '9+' : threadsWithNews}
                </span>
              )}
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
                <ConversationStream
                  channelId={channelId}
                  label={title ? `Tin nhắn trong ${title}` : 'Tin nhắn'}
                  messages={messages}
                  ready={!messagesQ.isLoading}
                  hasMore={!!messagesQ.data?.has_more}
                  loadingOlder={older.loading}
                  olderFailed={older.failed}
                  onLoadOlder={older.loadOlder}
                  me={me?.id}
                  people={people}
                  unread={{
                    ready: unreadQ.isSuccess || unreadQ.isError,
                    lastReadId: channelUnread?.last_read_message_id ?? '',
                    count: channelUnread?.unread_count ?? 0,
                  }}
                  activeThreadId={panel?.kind === 'thread' ? panel.id : null}
                  onOpenThread={(id) => setPanel({ kind: 'thread', id })}
                  onReact={(id) => setEmojiTarget(id)}
                  onToggleReaction={(id, emoji, hasReacted) => toggleReaction.mutate({ messageId: id, emoji, hasReacted })}
                  onPin={(m: Message) => togglePin.mutate({ messageId: m.id, isPinned: !!m.is_pinned })}
                  onFollow={(m, on) =>
                    follow.mutate(
                      { rootId: m.id, follow: on },
                      { onError: (err) => toast.error(explain(err, on ? 'theo dõi thread' : 'bỏ theo dõi thread')) },
                    )
                  }
                  onAtEndChange={setAtEnd}
                  lifecycle={lifecycle}
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

              <div className="relative">
                {/* Floats over the end of the stream so typing never shifts the layout. */}
                <div className="absolute bottom-full left-6 pb-1 text-xs text-ink-muted" aria-live="polite">
                  {typingNames.length > 0 && <TypingLine names={typingNames} />}
                </div>
              <ChatEditor
                channelId={channelId}
                people={people}
                placeholder={title ? `Tin nhắn cho ${title}` : 'Viết tin nhắn'}
                hint="Enter để gửi · Shift+Enter xuống dòng"
                onSend={handleSend}
                onTyping={() => sendTyping(channelId)}
                uploads={uploads}
                isPending={send.isPending}
                // On a phone a message is edited in the composer; on a wide screen, where it stands.
                edit={phone && lifecycle.editing ? {
                  initialHtml: lifecycle.editing.draft ?? toEditorHtml(lifecycle.editing.message),
                  onSave: lifecycle.saveEdit,
                  onCancel: lifecycle.cancelEdit,
                } : undefined}
                onEditLast={() => {
                  const last = lastEditableOf(messages, me?.id)
                  if (last) lifecycle.startEdit(last)
                }}
              />
              </div>
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
            key={`thread-${panel.id}`}
            channelId={channelId}
            workspaceId={workspaceId}
            messageId={panel.id}
            spaceName={title}
            people={people}
            me={me?.id}
            onBack={panel.fromList ? () => setPanel({ kind: 'threads' }) : undefined}
            onClose={() => setPanel(null)}
          />
        )}
        {panel?.kind === 'threads' && (
          <ThreadsPanel
            key="threads"
            channelId={channelId}
            spaceName={title}
            people={people}
            onOpenThread={(id) => setPanel({ kind: 'thread', id, fromList: true })}
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
            notification={phone ? { level, saving: savingLate, onChoose: notify.setLevel } : undefined}
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

/** "· Đang lưu…" after the sub line, once a save has taken longer than 300ms. */
function Saving() {
  return (
    <span className="inline-flex items-center gap-1.5 ml-1.5" role="status">
      · <Spinner size="sm" />Đang lưu…
    </span>
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
