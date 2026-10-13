import { queryClient } from './query-client'
import { keys } from '../hooks/keys'
import type {
  ChatMessage as WSChatMessage,
  MessageDeletedEvent,
  MessageEditedEvent,
  PinEvent,
  ReactionEvent,
  TaskUpdateEvent,
} from '../generated/proto/messaging/ws'
import type { Message, ReactionGroup, ChatTask } from '../api/messaging'
import type { LastMessage } from './conversations'
import { useAuthStore } from '../stores/auth.store'

type MessageList = { messages: Message[]; has_more: boolean }

/** The message as the open screens hold it, if any. */
function cachedMessage(channelId: string, id: string, parentId: string | undefined): Message | undefined {
  const find = (list: { messages: Message[] } | undefined) => list?.messages.find((m) => m.id === id)
  return (!parentId ? find(queryClient.getQueryData<MessageList>(keys.messaging.messages(channelId))) : undefined)
    ?? find(queryClient.getQueryData<{ messages: Message[] }>(keys.messaging.thread(parentId || id)))
}

/** The "Được nhắc tới" pill and feed asked for again: what they count changed. */
function refreshMentions(): void {
  queryClient.invalidateQueries({ queryKey: keys.shortcuts.mentionsAll() })
  queryClient.invalidateQueries({ queryKey: keys.shortcuts.mentionsUnreadAll() })
}

/** Plain-text preview of a message body: tags stripped, whitespace folded, capped. */
export function previewText(content: string): string {
  return content.replace(/<[^>]+>/g, ' ').replace(/&nbsp;/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 120)
}

/**
 * What a list row remembers of a live message: its opening, who wrote it, whom it
 * names, whether it opens a thread, and what kind of thing it is. `parentId` is
 * the root of the thread a reply belongs to.
 */
export function lastMessageOf(
  msg: Pick<WSChatMessage, 'id' | 'content' | 'createdAt' | 'senderName' | 'senderId' | 'mentions' | 'attachments' | 'linkedEntityType'>,
  parentId?: string,
): LastMessage {
  const file = msg.attachments?.[0]
  const linked = msg.linkedEntityType
  const kind: LastMessage['kind'] = file
    ? 'attachment'
    : linked === 'poll' ? 'poll' : linked === 'task' ? 'task' : linked === 'document' || linked === 'approval' ? 'card' : 'text'
  return {
    id: msg.id,
    content: previewText(msg.content),
    timestamp: timestampToIso(msg.createdAt),
    senderName: msg.senderName || '',
    senderId: msg.senderId,
    parentId: parentId || undefined,
    mentions: msg.mentions?.length ? msg.mentions : undefined,
    kind,
    attachmentName: file?.fileName || undefined,
  }
}

/** Proto Timestamp (or nothing) to ISO; "now" when the server omitted it. */
export function timestampToIso(ts: WSChatMessage['createdAt']): string {
  if (ts && ts.seconds !== undefined) {
    const ms = Number(ts.seconds) * 1000 + Math.floor((ts.nanos ?? 0) / 1e6)
    if (Number.isFinite(ms) && ms > 0) return new Date(ms).toISOString()
  }
  return new Date().toISOString()
}

/** Convert a WebSocket ChatMessage (proto camelCase) to API Message (snake_case). */
export function convertChatMsgToMessage(chatMsg: WSChatMessage): Message {
  return {
    id: chatMsg.id,
    channel_id: chatMsg.channelId,
    sender_id: chatMsg.senderId,
    sender_name: chatMsg.senderName,
    content: chatMsg.content,
    content_format: chatMsg.contentFormat || 'markdown',
    created_at: chatMsg.createdAt ?? new Date().toISOString(),
    reply_count: chatMsg.replyCount || 0,
    parent_message_id: chatMsg.parentMessageId || '',
    mentions: chatMsg.mentions || [],
    linked_entity_type: chatMsg.linkedEntityType || '',
    linked_entity_id: chatMsg.linkedEntityId || '',
    reactions: [],
    is_pinned: false,
    message_type: chatMsg.messageType || undefined,
    quoted: chatMsg.quoted && chatMsg.quoted.id
      ? {
          id: chatMsg.quoted.id,
          sender_id: chatMsg.quoted.senderId,
          sender_name: chatMsg.quoted.senderName,
          text: chatMsg.quoted.text,
          deleted: chatMsg.quoted.deleted,
        }
      : undefined,
    attachments: (chatMsg.attachments || []).map((a) => ({
      drive_file_id: a.driveFileId,
      file_name: a.fileName,
      mime_type: a.mimeType,
      size_bytes: typeof a.sizeBytes === 'bigint' ? Number(a.sizeBytes) : Number(a.sizeBytes || 0),
      width: a.width || undefined,
      height: a.height || undefined,
    })),
  }
}

/** Applies `fn` to the message with this id in a cached list; the list itself is returned untouched when it is not there. */
function mapMessage(old: MessageList | undefined, id: string, fn: (m: Message) => Message): MessageList | undefined {
  if (!old || !(old.messages || []).some((m) => m.id === id)) return old
  return { ...old, messages: old.messages.map((m) => (m.id === id ? fn(m) : m)) }
}

/**
 * A message of the open conversation changed text: take the stored text and
 * the edit time into every cache that holds it. A root lives in its channel's
 * stream and at the head of its own thread; a reply only in its thread. It is
 * not a new message, so nothing else moves.
 */
export function applyMessageEdited(ev: MessageEditedEvent): void {
  const m = ev.message
  if (!m?.id) return
  // The pill counts mentions of me: an edit that adds or removes me moves it, whoever is looking.
  const me = useAuthStore.getState().user?.id
  const before = cachedMessage(m.channelId, m.id, m.parentMessageId)
  if (me && (m.mentions?.includes(me) || before?.mentions?.includes(me))) refreshMentions()
  patchMessage(m.channelId, m.id, m.parentMessageId, (cur) =>
    cur.deleted
      ? cur
      : {
          ...cur,
          content: m.content,
          content_format: m.contentFormat || cur.content_format,
          // The frame carries the whole list after the edit: an empty one means every mention was removed.
          mentions: m.mentions ?? cur.mentions,
          edited_at: ev.editedAt,
        })
}

/**
 * Applies `fn` to one message wherever it is cached: a root in its channel's
 * stream and at the head of its own thread, a reply (`parentId` set) only in
 * its thread. The thread lists are marked stale, since they show a root's first line.
 */
export function patchMessage(channelId: string, id: string, parentId: string | undefined, fn: (m: Message) => Message): void {
  if (!parentId) {
    queryClient.setQueryData(keys.messaging.messages(channelId), (old: MessageList | undefined) => mapMessage(old, id, fn))
  }
  queryClient.setQueryData(
    keys.messaging.thread(parentId || id),
    (old: { messages: Message[] } | undefined) => mapMessage(old as MessageList | undefined, id, fn),
  )
  if (!parentId) queryClient.invalidateQueries({ queryKey: keys.messaging.threadsListOf(channelId) })
}

/** What is left of a deleted message: no text, files, reactions or pin. */
const tombstone = (m: Message): Message => ({
  ...m, deleted: true, content: '', reactions: [], is_pinned: false, mentions: [], quoted: undefined,
})

/**
 * A message was deleted. A root that has replies stays as a placeholder so its
 * thread survives; any other message leaves the view. A deleted reply leaves
 * its thread and the root takes the reply count the server computed. Messages
 * that quote it now show the quote as deleted. Applying it twice changes
 * nothing: the sender's own request and its frame both arrive.
 */
export function applyMessageDeleted(ev: MessageDeletedEvent): void {
  const { messageId: id, channelId, parentMessageId: parent } = ev
  // The frame names no one: a message not held here may have named me, so only a known miss skips the refresh.
  const me = useAuthStore.getState().user?.id
  const known = cachedMessage(channelId, id, parent)
  if (me && (!known || known.mentions?.includes(me))) refreshMentions()
  const without = (old: MessageList | undefined): MessageList | undefined =>
    old && old.messages.some((m) => m.id === id) ? { ...old, messages: old.messages.filter((m) => m.id !== id) } : old

  if (parent) {
    queryClient.setQueryData(keys.messaging.thread(parent), (old: MessageList | undefined) => {
      const kept = without(old)
      return mapMessage(kept, parent, (root) => withReplyCount(root, ev.replyCount))
    })
    queryClient.setQueryData(keys.messaging.messages(channelId), (old: MessageList | undefined) =>
      mapMessage(old, parent, (root) => withReplyCount(root, ev.replyCount)))
  } else if (ev.keepPlaceholder) {
    queryClient.setQueryData(keys.messaging.messages(channelId), (old: MessageList | undefined) => mapMessage(old, id, tombstone))
    queryClient.setQueryData(keys.messaging.thread(id), (old: MessageList | undefined) => mapMessage(old, id, tombstone))
  } else {
    queryClient.setQueryData(keys.messaging.messages(channelId), without)
    queryClient.setQueryData(keys.messaging.thread(id), (old: MessageList | undefined) => mapMessage(old, id, tombstone))
  }

  const unquote = (m: Message): Message =>
    m.quoted?.id === id ? { ...m, quoted: { ...m.quoted, text: '', deleted: true } } : m
  const unquoteAll = (old: MessageList | undefined): MessageList | undefined =>
    old && old.messages.some((m) => m.quoted?.id === id) ? { ...old, messages: old.messages.map(unquote) } : old
  queryClient.setQueryData(keys.messaging.messages(channelId), unquoteAll)
  queryClient.setQueriesData({ queryKey: keys.messaging.threadsAll() }, unquoteAll)

  queryClient.invalidateQueries({ queryKey: keys.messaging.threadsListOf(channelId) })
  queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
}

/** The root's summary after a reply was deleted: the server's count, and nobody replied when it is zero. */
function withReplyCount(root: Message, count: number): Message {
  return count > 0
    ? { ...root, reply_count: count }
    : { ...root, reply_count: 0, last_reply_at: undefined, reply_participant_ids: [], thread_unread_count: 0 }
}

/** Inject a reaction change directly into the channel's messages cache. */
export function applyReaction(reaction: ReactionEvent): void {
  queryClient.setQueryData(
    keys.messaging.messages(reaction.channelId),
    (old: MessageList | undefined) => {
      if (!old) return old
      return {
        ...old,
        messages: (old.messages || []).map((m: Message) => {
          if (m.id !== reaction.messageId) return m
          const reactions = [...(m.reactions || [])]
          const idx = reactions.findIndex((r: ReactionGroup) => r.emoji === reaction.emoji)
          if (reaction.action === 'add') {
            const current = reactions[idx]
            if (current) {
              const group: ReactionGroup = { ...current }
              if (!group.user_ids.includes(reaction.userId)) {
                group.count += 1
                group.user_ids = [...group.user_ids, reaction.userId]
              }
              reactions[idx] = group
            } else {
              reactions.push({ emoji: reaction.emoji, count: 1, user_ids: [reaction.userId] })
            }
          } else {
            const current = reactions[idx]
            if (current) {
              const group: ReactionGroup = { ...current }
              group.user_ids = group.user_ids.filter((id: string) => id !== reaction.userId)
              group.count = group.user_ids.length
              if (group.count <= 0) {
                reactions.splice(idx, 1)
              } else {
                reactions[idx] = group
              }
            }
          }
          return { ...m, reactions }
        }),
      }
    },
  )
}

/** Flip is_pinned in the messages cache and refresh the pins list, which needs the server's pin metadata. */
export function applyPin(pin: PinEvent): void {
  const isPinned = pin.action === 'pin'
  queryClient.setQueryData(
    keys.messaging.messages(pin.channelId),
    (old: MessageList | undefined) => {
      if (!old) return old
      return {
        ...old,
        messages: (old.messages || []).map((m: Message) =>
          m.id === pin.messageId ? { ...m, is_pinned: isPinned } : m,
        ),
      }
    },
  )
  queryClient.invalidateQueries({ queryKey: keys.messaging.pins(pin.channelId) })
}

/**
 * Inject a task status/assignee change into every cached list of the
 * channel's tasks, whatever status filter each was fetched with.
 */
export function applyTaskUpdate(task: TaskUpdateEvent): void {
  queryClient.setQueriesData(
    { queryKey: keys.messaging.tasksOf(task.channelId) },
    (old: { tasks: ChatTask[] } | undefined) => {
      if (!old) return old
      return {
        ...old,
        tasks: old.tasks.map((t: ChatTask) =>
          t.id === task.taskId
            ? { ...t, status: task.status || t.status, assignee_id: task.assigneeId || t.assignee_id, title: task.title || t.title }
            : t,
        ),
      }
    },
  )
}
