import type { Message, QuotedMessage } from '../api/messaging'

/**
 * What can be done with one message (docs/specs/chat-message-lifecycle): pure,
 * so the rules behind the menus are tested without a DOM. The server decides
 * in the end; this only keeps the menus from offering what it would refuse.
 */

type Pending = Message & { _optimistic?: boolean }

export type CardKind = 'poll' | 'task' | 'document' | 'approval'

/** The card a message carries, if any: the object it links to is shown in place of (or besides) its text. */
export function cardKindOf(m: Pick<Message, 'linked_entity_type' | 'linked_entity_id'>): CardKind | undefined {
  if (!m.linked_entity_id) return undefined
  const t = m.linked_entity_type
  return t === 'poll' || t === 'task' || t === 'document' || t === 'approval' ? t : undefined
}

export interface MessageAbilities {
  /** Its sender changes the text (cards and files are not text). */
  edit: boolean
  /** The sender, or whoever manages the channel. */
  remove: boolean
  /** Someone else's message: move the read position back to it. */
  markUnread: boolean
}

const NONE: MessageAbilities = { edit: false, remove: false, markUnread: false }

/** A line the product wrote into the conversation (a task created, a deletion logged), not something a person said. */
export const isSystemLine = (m: Message): boolean =>
  m.message_type === 'system' || m.linked_entity_type === 'task' || m.linked_entity_type === 'message_deleted'

export function messageAbilities(m: Pending, ctx: { me: string | undefined; canManage: boolean }): MessageAbilities {
  if (m._optimistic || m.id.startsWith('temp-') || m.deleted || isSystemLine(m)) return NONE
  const mine = !!ctx.me && m.sender_id === ctx.me
  return {
    edit: mine && !m.linked_entity_type,
    remove: mine || ctx.canManage,
    markUnread: !mine,
  }
}

/** The newest message of the reader's that they can still edit; `messages` is oldest first. */
export function lastEditableOf(messages: Pending[], me: string | undefined): Pending | undefined {
  if (!me) return undefined
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i]!
    if (messageAbilities(m, { me, canManage: false }).edit) return m
  }
  return undefined
}

/** A deleted message that stays in the stream: it is a root with a thread. */
export const isDeletedPlaceholder = (m: Message): boolean => !!m.deleted

/** The sentences of the delete confirmation: what happens, to whom, and what is kept. */
export function deleteConsequence(o: { mine: boolean; senderName: string; hasThread: boolean }): string[] {
  const lines = [
    o.mine
      ? 'Tin sẽ bị xoá với mọi người trong cuộc trò chuyện và không khôi phục được.'
      : `Tin của ${o.senderName} sẽ bị xoá với mọi người trong cuộc trò chuyện và không khôi phục được.`,
  ]
  if (!o.mine) lines.push('Nhật ký nhóm ghi lại bạn đã xoá tin này.')
  if (o.hasThread) lines.push('Thread của tin vẫn giữ lại.')
  return lines
}

export interface QuoteView { name: string; text: string; deleted: boolean }

export const DELETED_QUOTE = 'Tin nhắn đã bị xoá'

/** A quote as it is shown: the quoted person's name and the opening of their text, or that it is gone. */
export function quoteSummary(q: QuotedMessage, nameOf: (userId: string, fallback?: string) => string): QuoteView {
  const deleted = !!q.deleted
  return { name: nameOf(q.sender_id, q.sender_name), text: deleted ? DELETED_QUOTE : (q.text ?? ''), deleted }
}

const MARKUP = /<\/?[a-z][^>]*>/i

const escapeHtml = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

/** What the editor opens with when a message is edited: rich text as it is, an older plain or markdown body as paragraphs. */
export function toEditorHtml(m: Pick<Message, 'content' | 'content_format'>): string {
  if (m.content_format === 'html' || MARKUP.test(m.content)) return m.content
  return m.content.split('\n').map((line) => `<p>${escapeHtml(line)}</p>`).join('')
}

/** A message body as text, line breaks kept: for copying, previews and the editor's empty check. */
export function messageText(content: string): string {
  if (!MARKUP.test(content)) return content
  const doc = new DOMParser().parseFromString(
    content.replace(/<br\s*\/?>/gi, '\n').replace(/<\/(p|div|li|blockquote|pre)>/gi, '$&\n'),
    'text/html',
  )
  return (doc.body.textContent ?? '').replace(/\n{3,}/g, '\n\n').trim()
}
