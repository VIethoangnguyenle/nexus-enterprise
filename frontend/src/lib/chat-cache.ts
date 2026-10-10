import { queryClient } from './query-client'
import { keys } from '../hooks/keys'
import type {
  ChatMessage as WSChatMessage,
  PinEvent,
  ReactionEvent,
  TaskUpdateEvent,
} from '../generated/proto/messaging/ws'
import type { Message, ReactionGroup, ChatTask } from '../api/messaging'

type MessageList = { messages: Message[]; has_more: boolean }

/** Plain-text preview of a message body: tags stripped, whitespace folded, capped. */
export function previewText(content: string): string {
  return content.replace(/<[^>]+>/g, ' ').replace(/&nbsp;/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 120)
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
  }
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
