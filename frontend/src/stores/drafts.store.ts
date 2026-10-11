import { create } from 'zustand'
import type { ChatTask, Message } from '../api/messaging'
import { isImageAttachment } from '../components/chat/AttachmentGrid'
import { chatCardKeys } from '../hooks/keys/chat-cards'
import { keys } from '../hooks/keys'
import { queryClient } from '../lib/query-client'
import { messageText } from '../lib/message-model'
import type { PollView } from '../api/chat-cards'

/** What a quote shows of a message that has no text of its own: "3 ảnh", a file name, "Bình chọn: …". */
export interface QuoteSummary { icon: 'image' | 'file' | 'card'; label: string }

/** The message a composer is about to quote, as the strip shows it. */
export interface PendingQuote {
  id: string
  sender_id: string
  sender_name?: string
  /** The opening of the text; empty when the message has none (then `summary` says what it holds). */
  text: string
  summary?: QuoteSummary
  /** Sent on the message it was taken from: used to tell "deleted" from "not loaded". */
  created_at?: unknown
  /** The quoted message was deleted while the quote was pending: the next message goes without it. */
  deleted?: boolean
}

interface Draft { html: string; quote?: PendingQuote }

interface DraftsState {
  /** Per composer (see `scopeOf`): the typed text and the pending quote. Kept for the session, not on disk. */
  drafts: Record<string, Draft>
  /** Composers whose box is currently an editor of a sent message (phones). */
  editing: Record<string, true>
  setHtml: (scope: string, html: string) => void
  setQuote: (scope: string, quote: PendingQuote | null) => void
  markQuoteDeleted: (scope: string, messageId: string) => void
  /** The id to send as `quoted_message_id`: the pending quote, unless it is gone. */
  pendingQuoteId: (scope: string) => string | undefined
  setEditing: (scope: string, on: boolean) => void
  reset: () => void
}

/** Where a message is composed: the thread of a reply, otherwise the conversation. */
export const scopeOf = (m: Pick<Message, 'channel_id' | 'parent_message_id'>): string =>
  m.parent_message_id ? `thread:${m.parent_message_id}` : m.channel_id

export const threadScope = (rootId: string) => `thread:${rootId}`

const EXCERPT = 160
const CARD_LABEL: Record<string, string> = {
  poll: 'Bình chọn', task: 'Công việc', document: 'Tài liệu', approval: 'Đề nghị phê duyệt',
}

function summaryOf(m: Message, cardTitle?: string): QuoteSummary | undefined {
  const files = m.attachments ?? []
  if (files.length > 0) {
    const images = files.filter(isImageAttachment).length
    const others = files.length - images
    if (others === 0) return { icon: 'image', label: `${images} ảnh` }
    if (images === 0) return { icon: 'file', label: others === 1 ? files[0]!.file_name : `${others} tệp` }
    return { icon: 'file', label: `${images} ảnh, ${others} tệp` }
  }
  const card = m.linked_entity_type ? CARD_LABEL[m.linked_entity_type] : undefined
  if (card) return { icon: 'card', label: cardTitle ? `${card}: ${cardTitle}` : card }
  return undefined
}

/**
 * A message as a pending quote. The text is quoted when there is any;
 * otherwise a one-line summary of the files or the card stands in for it.
 */
export function quoteFromMessage(m: Message, opts: { cardTitle?: string } = {}): PendingQuote {
  // A poll or task message carries a fallback line ("📊 Poll: …"): the card, not that line, is what is quoted.
  const fallback = m.linked_entity_type === 'poll' || m.linked_entity_type === 'task'
  const plain = fallback ? '' : messageText(m.content ?? '').replace(/\s+/g, ' ').trim()
  const text = plain.length > EXCERPT ? `${plain.slice(0, EXCERPT).trimEnd()}…` : plain
  return {
    id: m.id,
    sender_id: m.sender_id,
    sender_name: m.sender_name,
    text,
    summary: text ? undefined : summaryOf(m, opts.cardTitle),
    created_at: m.created_at,
  }
}

const keep = (d: Draft): Draft | undefined => (d.html || d.quote ? d : undefined)

function put(drafts: Record<string, Draft>, scope: string, next: Draft | undefined): Record<string, Draft> {
  const out = { ...drafts }
  if (next) out[scope] = next
  else delete out[scope]
  return out
}

export const useDraftsStore = create<DraftsState>()((set, get) => ({
  drafts: {},
  editing: {},
  setHtml: (scope, html) =>
    set((s) => {
      const cur = s.drafts[scope]
      if ((cur?.html ?? '') === html) return s
      return { drafts: put(s.drafts, scope, keep({ ...cur, html })) }
    }),
  setQuote: (scope, quote) =>
    set((s) => {
      const cur = s.drafts[scope] ?? { html: '' }
      const { quote: _old, ...rest } = cur
      return { drafts: put(s.drafts, scope, keep(quote ? { ...rest, quote } : rest)) }
    }),
  markQuoteDeleted: (scope, messageId) =>
    set((s) => {
      const cur = s.drafts[scope]
      if (!cur?.quote || cur.quote.id !== messageId || cur.quote.deleted) return s
      return { drafts: { ...s.drafts, [scope]: { ...cur, quote: { ...cur.quote, deleted: true } } } }
    }),
  pendingQuoteId: (scope) => {
    const q = get().drafts[scope]?.quote
    return q && !q.deleted ? q.id : undefined
  },
  setEditing: (scope, on) =>
    set((s) => {
      if (!!s.editing[scope] === on) return s
      const next = { ...s.editing }
      if (on) next[scope] = true
      else delete next[scope]
      return { editing: next }
    }),
  reset: () => set({ drafts: {}, editing: {} }),
}))

/** The question or title of a poll or task card, when it is already loaded: a quote of a card names it. */
function cardTitleOf(m: Message): string | undefined {
  if (!m.linked_entity_id) return undefined
  if (m.linked_entity_type === 'poll') return queryClient.getQueryData<PollView>(chatCardKeys.poll(m.linked_entity_id))?.question
  if (m.linked_entity_type === 'task') {
    for (const [, data] of queryClient.getQueriesData<{ tasks: ChatTask[] }>({ queryKey: keys.messaging.tasksOf(m.channel_id) })) {
      const found = data?.tasks.find((t) => t.id === m.linked_entity_id)
      if (found) return found.title
    }
  }
  return undefined
}

/** "Trích dẫn khi trả lời": the message becomes the pending quote of the composer it is answered in. */
export function quoteMessage(m: Message): void {
  useDraftsStore.getState().setQuote(scopeOf(m), quoteFromMessage(m, { cardTitle: cardTitleOf(m) }))
}
