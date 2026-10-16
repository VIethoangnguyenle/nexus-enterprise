import { useMutation } from '@tanstack/react-query'
import { messagingApi, type EditMessageInput, type Message } from '../api/messaging'
import { mentionsFromHtml } from '../components/chat/chat-html'
import { applyMessageDeleted, patchMessage } from '../lib/chat-cache'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

type MessageList = { messages: Message[] }

/**
 * The mentions an edit leaves the message with. Rich text carries them as spans, so the edited
 * text is the whole truth. A message written before rich text names people in plain text: no
 * span says so, and reading only spans would drop every one of them, so its stored list stays
 * and only a person picked while editing is added.
 */
function mentionsAfterEdit(message: Message, content: string): string[] {
  const found = mentionsFromHtml(content)
  if (message.content_format === 'html') return found
  return [...new Set([...(message.mentions ?? []), ...found])]
}

/** The message a reply belongs to, as the channel's stream holds it. */
const rootOf = (channelId: string, rootId: string) =>
  queryClient.getQueryData<MessageList>(keys.messaging.messages(channelId))?.messages.find((m) => m.id === rootId)

/**
 * Changes the text of your own message. The new text shows at once; the
 * server's answer (filtered, with the edit time) replaces it, and a refusal
 * puts the old text back. Silent: the caller says what went wrong.
 */
export function useEditMessage(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ message, content }: { message: Message; content: string }) => {
      // Always the full list: leaving it out would keep the old mentions, and the server tells
      // who the edit added by comparing the two lists.
      const request: EditMessageInput = { content, content_format: 'html', mentions: mentionsAfterEdit(message, content) }
      return messagingApi.editMessage(message.id, request)
    },
    onMutate: ({ message, content }) => {
      patchMessage(channelId, message.id, message.parent_message_id, (m) => ({
        ...m, content, content_format: 'html', mentions: mentionsAfterEdit(message, content), edited_at: new Date().toISOString(),
      }))
      return { before: message }
    },
    onSuccess: (stored, { message }) => {
      patchMessage(channelId, message.id, message.parent_message_id, (m) => ({
        ...m, content: stored.content, content_format: stored.content_format ?? m.content_format, edited_at: stored.edited_at ?? m.edited_at,
      }))
    },
    onError: (_err, { message }, ctx) => {
      if (!ctx) return
      patchMessage(channelId, message.id, message.parent_message_id, (m) => ({
        ...m, content: ctx.before.content, content_format: ctx.before.content_format, mentions: ctx.before.mentions, edited_at: ctx.before.edited_at,
      }))
    },
  })
}

/**
 * Deletes a message. Nothing moves until the server agrees; then the stream
 * shows what the server's own frame will show everyone: a root with replies
 * stays as a placeholder, any other message leaves.
 */
export function useDeleteMessage(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (message: Message) => messagingApi.deleteMessage(message.id),
    onSuccess: (_ok, message) => {
      const parent = message.parent_message_id ?? ''
      // The server's own frame usually beats this answer and has already applied
      // the delete with the true count; doing it again would count the reply twice.
      if (parent) {
        const thread = queryClient.getQueryData<MessageList>(keys.messaging.thread(parent))
        if (thread && !thread.messages.some((m) => m.id === message.id)) return
      }
      applyMessageDeleted({
        messageId: message.id,
        channelId,
        parentMessageId: parent,
        keepPlaceholder: !parent && (message.reply_count ?? 0) > 0,
        replyCount: parent ? Math.max(0, (rootOf(channelId, parent)?.reply_count ?? 1) - 1) : 0,
      })
    },
  })
}

/** Moves your read position back to just before a message, then lets the counts and thread summaries catch up. */
export function useMarkUnread(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (message: Message) => messagingApi.markMessageUnread(message.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
      void queryClient.invalidateQueries({ queryKey: keys.messaging.messages(channelId) })
      void queryClient.invalidateQueries({ queryKey: keys.messaging.threadsListOf(channelId) })
    },
  })
}
