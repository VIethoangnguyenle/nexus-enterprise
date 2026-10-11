import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'motion/react'
import { Bell, BellRing, CircleAlert, Upload } from 'lucide-react'
import { useThread, useSendReply } from '../../hooks/useMessages'
import { useMessageActions } from '../../hooks/useMessageActions'
import { usePhone } from '../../hooks/usePhone'
import { useAttachmentUploads } from '../../hooks/useAttachmentUploads'
import { lastEditableOf, toEditorHtml } from '../../lib/message-model'
import { useFollowThread, useMarkThreadRead } from '../../hooks/useThreads'
import { useArrivals } from '../../hooks/useArrivals'
import { useMotionPresets } from '../../lib/motion'
import { personStyle } from '../../lib/person-hue'
import { explain } from '../../lib/errors'
import { buildStream, firstUnreadReplyId } from '../../lib/stream-model'
import type { PeopleDirectory } from '../../lib/people'
import type { Message } from '../../api/messaging'
import { Button, toast } from '../primitives'
import { ChatEditor } from './ChatEditor'
import { SidePanel } from '../spaces/SidePanel'
import { MessageBlock } from '../spaces/MessageBlock'
import { EmptyState } from '../spaces/EmptyState'
import { MessageMenu, type MessageMenuTarget } from '../spaces/MessageMenu'
import { MessageActionSheet } from '../spaces/MessageActionSheet'
import { MessageEditBox } from '../spaces/MessageEditBox'
import { DeleteMessageDialog } from '../spaces/DeleteMessageDialog'
import { keys } from '../../hooks/keys'
import { threadScope } from '../../stores/drafts.store'

type Msg = Message & { _optimistic?: boolean }

interface ThreadPanelProps {
  channelId: string
  workspaceId?: string
  /** The root message whose replies to show. */
  messageId: string
  /** Space or DM title, shown under "Thread". */
  spaceName: string
  people: PeopleDirectory
  me: string | undefined
  /** Set when the thread was opened from the thread list: a way back to it. */
  onBack?: () => void
  onClose: () => void
}

/** Where "Tin mới" lands below the top of the panel when the thread has unread replies. */
const DIVIDER_OFFSET = 48

/**
 * A thread at the side (DESIGN.md "Panel thread"): the root message, a divider
 * with the reply count, the replies grouped by sender, a "Tin mới" divider
 * before the first unread reply, and a reply box. Opening it marks the thread
 * read; the divider stays until the panel closes. Follow / unfollow is the
 * button in the header. Below 1024 it is a full-screen sheet.
 */
export function ThreadPanel({ channelId, workspaceId, messageId, spaceName, people, me, onBack, onClose }: ThreadPanelProps) {
  const uploads = useAttachmentUploads({ workspaceId: workspaceId ?? '', channelId })
  const uploadsRef = useRef(uploads)
  uploadsRef.current = uploads

  useEffect(() => {
    uploads.clear()
  }, [messageId])
  const [isDragging, setIsDragging] = useState(false)
  const motionPresets = useMotionPresets()
  const qc = useQueryClient()
  const { data, isLoading, error, refetch } = useThread(messageId)
  const send = useSendReply(channelId, messageId)
  const phone = usePhone()
  // A reply marked unread from inside the panel keeps the thread unread until the panel is left:
  // the read that follows an opening must not undo it at once.
  const heldUnread = useRef<string | null>(null)
  const lifecycle = useMessageActions(channelId, me, {
    onMarkedUnread: (m) => {
      if (m.parent_message_id === messageId || m.id === messageId) heldUnread.current = messageId
    },
  })
  const [menu, setMenu] = useState<MessageMenuTarget | null>(null)
  const [sheetFor, setSheetFor] = useState<Message | null>(null)
  const anchorRef = useRef<HTMLElement | null>(null)
  const follow = useFollowThread(channelId)
  const markRead = useMarkThreadRead(channelId)
  const bodyRef = useRef<HTMLDivElement>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  const all = (data?.messages ?? []) as Msg[]
  const fromList = qc
    .getQueryData<{ messages: Message[] }>(keys.messaging.messages(channelId))
    ?.messages.find((x) => x.id === messageId)
  const root = all.find((x) => x.id === messageId) ?? fromList
  const replies = useMemo(() => all.filter((x) => x.id !== messageId), [all, messageId])
  const following = !!root?.following
  const unreadNow = root?.thread_unread_count ?? 0

  // The divider is fixed from the moment the thread is first shown: reading moves the position, not the line.
  const [seen, setSeen] = useState<{ scope: string; resolved: boolean; unread: number }>({ scope: messageId, resolved: false, unread: 0 })
  if (seen.scope !== messageId) {
    setSeen({ scope: messageId, resolved: false, unread: 0 })
  } else if (!seen.resolved && !isLoading && root) {
    setSeen({ scope: messageId, resolved: true, unread: following ? unreadNow : 0 })
  }
  const unreadReplyId = seen.resolved ? firstUnreadReplyId(replies, seen.unread, me) : null
  const rows = useMemo(
    () => buildStream(replies, { me, unreadFromId: unreadReplyId }).filter((i) => i.kind !== 'day'),
    [replies, me, unreadReplyId],
  )

  const { fresh } = useArrivals(replies, {
    keyOf: (r) => r.id,
    authorOf: (r) => r.sender_id,
    me,
    ready: !isLoading,
    scope: messageId,
  })

  // Opening the thread reads it; so does a reply that lands while it is open.
  const markReadMutate = markRead.mutate
  useEffect(() => {
    if (seen.resolved && following && unreadNow > 0 && heldUnread.current !== messageId) markReadMutate(messageId)
  }, [seen.resolved, following, unreadNow, messageId, markReadMutate])

  // At the divider when there are unread replies, otherwise at the newest reply.
  const placed = useRef('')
  const placedCount = useRef(0)
  useLayoutEffect(() => {
    const body = bodyRef.current
    if (!body || !seen.resolved || placed.current === messageId) return
    placed.current = messageId
    placedCount.current = replies.length
    const d = body.querySelector<HTMLElement>('[data-unread-divider]')
    body.scrollTop = d ? Math.max(0, d.offsetTop - DIVIDER_OFFSET) : body.scrollHeight
  }, [seen.resolved, messageId, replies.length])
  // A reply that arrives after that moves the view to it.
  useEffect(() => {
    if (placed.current !== messageId || replies.length <= placedCount.current) return
    placedCount.current = replies.length
    bottomRef.current?.scrollIntoView?.({ block: 'end', behavior: motionPresets.reduced ? 'auto' : 'smooth' })
  }, [replies.length, messageId, motionPresets.reduced])

  // "Đã theo dõi thread" is said once, after a change the reader made.
  const [announce, setAnnounce] = useState('')
  const toggleFollow = () => {
    const next = !following
    follow.mutate(
      { rootId: messageId, follow: next },
      {
        onSuccess: () => setAnnounce(next ? 'Đã theo dõi thread' : 'Đã bỏ theo dõi thread'),
        onError: (err) => toast.error(explain(err, next ? 'theo dõi thread' : 'bỏ theo dõi thread')),
      },
    )
  }

  useEffect(() => {
    const el = (bodyRef.current?.closest('aside') ?? bodyRef.current) as HTMLElement | null
    if (!el) return
    const onDragOver = (e: DragEvent) => {
      if (e.dataTransfer?.types.includes('Files')) {
        e.preventDefault()
        setIsDragging(true)
      }
    }
    const onDragLeave = (e: DragEvent) => {
      if (!el.contains(e.relatedTarget as Node)) {
        setIsDragging(false)
      }
    }
    const onDrop = (e: DragEvent) => {
      e.preventDefault()
      setIsDragging(false)
      const files = Array.from(e.dataTransfer?.files ?? [])
      if (files.length) uploadsRef.current.add(files)
    }
    el.addEventListener('dragover', onDragOver)
    el.addEventListener('dragleave', onDragLeave)
    el.addEventListener('drop', onDrop)
    return () => {
      el.removeEventListener('dragover', onDragOver)
      el.removeEventListener('dragleave', onDragLeave)
      el.removeEventListener('drop', onDrop)
    }
  }, [])

  const count = replies.length
  const countLabel = isLoading ? 'Đang tải câu trả lời' : count ? `${count} trả lời` : 'Chưa có câu trả lời'
  return (
    <SidePanel
      label="Thread"
      title="Thread"
      sub={spaceName}
      closeLabel="thread"
      sheetBelow="lg"
      backOnSheet
      onBack={onBack}
      onClose={onClose}
      bodyRef={bodyRef}
      actions={
        root && (
          <Button
            variant={following ? 'tonal' : 'soft'}
            size="sm"
            aria-pressed={following}
            disabled={follow.isPending}
            onClick={toggleFollow}
            className="min-h-9 md:min-h-0"
          >
            {following
              ? <BellRing size={16} strokeWidth={1.75} aria-hidden="true" className="text-accent" />
              : <Bell size={16} strokeWidth={1.75} aria-hidden="true" />}
            {following ? 'Đang theo dõi' : 'Theo dõi'}
          </Button>
        )
      }
      footer={
        <>
          <p className="sr-only" role="status" aria-live="polite">{announce}</p>
          <ChatEditor
            channelId={channelId}
            people={people}
            tone="base"
            placeholder="Trả lời trong thread"
            hint="Enter để gửi"
            uploads={uploads}
            parentMessageId={messageId}
            draftKey={threadScope(messageId)}
            isPending={send.isPending}
            onSend={async (html, mentions, attachments) => {
              if (!html.trim() && (!attachments || attachments.length === 0)) return
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
                await send.mutateAsync({ content: html, mentions, attachments, optimisticAttachments })
              } catch (err) {
                toast.error(explain(err, 'gửi câu trả lời'))
                throw err
              }
            }}
            edit={phone && lifecycle.editing ? {
              initialHtml: lifecycle.editing.draft ?? toEditorHtml(lifecycle.editing.message),
              onSave: lifecycle.saveEdit,
              onCancel: lifecycle.cancelEdit,
            } : undefined}
            onEditLast={() => {
              const last = lastEditableOf(replies, me)
              if (last) lifecycle.startEdit(last)
            }}
          />
        </>
      }
    >
      <div className="relative min-h-full">
        {isDragging && (
          <div className="absolute inset-0 z-dropdown bg-base/80 backdrop-blur-xs border-2 border-dashed border-accent flex flex-col items-center justify-center pointer-events-none text-accent font-semibold text-sm gap-1">
            <Upload size={24} />
            <b>Thả tệp để đính kèm</b>
            <span className="text-xs text-ink-muted font-normal">Tối đa 20 tệp mỗi tin</span>
          </div>
        )}
        {error && !root ? (
          <EmptyState
            compact
            icon={<CircleAlert size={24} strokeWidth={1.75} />}
            text="Chưa tải được thread này."
            action={<Button variant="soft" size="sm" onClick={() => refetch()}>Thử lại</Button>}
          />
        ) : (
          <div className="grid content-start gap-0.5 pt-1">
            {root && (
              <MessageBlock
                message={root}
                people={people}
                idBase={`thread-root-${messageId}`}
                mine={root.sender_id === me}
                me={me}
                className="px-2"
              />
            )}
          <div
            role="separator"
            aria-label={countLabel}
            className="flex items-center gap-2.5 px-3 py-1.5 text-xs font-semibold text-ink-muted
              before:content-[''] before:h-px before:flex-1 before:bg-line after:content-[''] after:h-px after:flex-1 after:bg-line"
          >
            {countLabel}
          </div>
          <AnimatePresence initial={false}>
            {rows.map((item) => {
              if (item.kind === 'unread') {
                return (
                  <motion.div key={item.key} data-unread-divider initial={false} exit={{ opacity: 0 }}>
                    <div
                      role="separator"
                      aria-label="Tin mới"
                      className="flex items-center gap-3 px-3 pt-3 pb-1.5 text-xs font-semibold text-accent before:content-[''] before:h-px before:flex-1 before:bg-accent"
                    >
                      Tin mới
                    </div>
                  </motion.div>
                )
              }
              if (item.kind !== 'message') return null
              const r = item.message as Msg
              const a = fresh.get(item.key)
              const byOther = a?.source === 'other'
              return (
                <motion.div key={item.key} {...motionPresets.row}>
                  <div
                    data-realtime={a?.source}
                    data-message-id={r.id}
                    style={byOther ? personStyle(a!.author ?? r.sender_id) : undefined}
                    className={`rounded-surface ${byOther ? 'rt-wash' : ''}`}
                  >
                    <MessageBlock
                      message={r}
                      people={people}
                      idBase={`reply-${item.key}`}
                      mine={item.mine}
                      grouped={item.grouped}
                      me={me}
                      halo={byOther}
                      tag={byOther ? 'vừa trả lời' : undefined}
                      className="px-2"
                      pressed={sheetFor?.id === r.id}
                      onEdit={lifecycle.abilities(r).edit ? (m) => lifecycle.startEdit(m) : undefined}
                      onMore={(m, anchor) => {
                        anchorRef.current = anchor
                        setMenu({ message: m, up: false })
                      }}
                      onOpenSheet={(m) => setSheetFor(m)}
                      editor={lifecycle.editing?.message.id === r.id && !phone ? (
                        <MessageEditBox
                          initialHtml={lifecycle.editing.draft ?? toEditorHtml(r)}
                          onSave={lifecycle.saveEdit}
                          onCancel={lifecycle.cancelEdit}
                        />
                      ) : undefined}
                    />
                  </div>
                </motion.div>
              )
            })}
          </AnimatePresence>
          <div ref={bottomRef} />
        </div>
      )}
      </div>
      <MessageMenu target={menu} anchorRef={anchorRef} onClose={() => setMenu(null)} lifecycle={lifecycle} />
      <AnimatePresence>
        {sheetFor && (
          <MessageActionSheet
            key={sheetFor.id}
            message={sheetFor}
            people={people}
            me={me}
            onEdit={lifecycle.abilities(sheetFor).edit ? () => lifecycle.startEdit(sheetFor) : undefined}
            onMarkUnread={lifecycle.abilities(sheetFor).markUnread ? () => lifecycle.markUnread(sheetFor) : undefined}
            onDelete={lifecycle.abilities(sheetFor).remove ? () => lifecycle.askDelete(sheetFor) : undefined}
            onClose={() => setSheetFor(null)}
          />
        )}
      </AnimatePresence>
      <DeleteMessageDialog
        message={lifecycle.deleting}
        me={me}
        people={people}
        busy={lifecycle.deleteBusy}
        onConfirm={lifecycle.confirmDelete}
        onCancel={lifecycle.cancelDelete}
      />
    </SidePanel>
  )
}
