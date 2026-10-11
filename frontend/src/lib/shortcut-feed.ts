import type { FeedCursor, FeedMessage, FeedPage } from '../api/shortcuts'
import { formatDate, formatTime, toDate } from './format'
import { dayLabel, timestampToRfc3339 } from './stream-model'
import { sanitizeHtml } from './sanitize-html'

/**
 * What the mentions and starred feeds need to show and page: pure, so the rules
 * are tested without a DOM or a server.
 */

/** The cursor of the page after this one, or nothing when it was the last (or its time cannot be read). */
export function feedCursor(page: FeedPage, field: 'created_at' | 'starred_at'): FeedCursor | undefined {
  const last = page.messages[page.messages.length - 1]
  if (!page.has_more || !last) return undefined
  const before = timestampToRfc3339(last[field])
  return before ? { before, beforeId: last.id } : undefined
}

export interface DayGroup { label: string; items: FeedMessage[] }

/** Consecutive messages of one day under that day's divider label; the feed's order is kept. */
export function groupByDay(items: FeedMessage[], now: Date = new Date()): DayGroup[] {
  const groups: DayGroup[] = []
  for (const m of items) {
    const label = dayLabel(m.created_at, now)
    const last = groups[groups.length - 1]
    if (last && last.label === label) last.items.push(m)
    else groups.push({ label, items: [m] })
  }
  return groups
}

const pad = (n: number) => String(n).padStart(2, '0')

/** A starred card's time: `09:12`, `Hôm qua 17:20`, `03/10`, `31/12/2025`. */
export function cardTime(value: unknown, now: Date = new Date()): string {
  const d = toDate(value)
  if (!d) return ''
  const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
  const ago = Math.round((day(now) - day(d)) / 86_400_000)
  if (ago === 0) return formatTime(d)
  if (ago === 1) return `Hôm qua ${formatTime(d)}`
  return d.getFullYear() === now.getFullYear() ? `${pad(d.getDate())}/${pad(d.getMonth() + 1)}` : formatDate(d)
}

export interface Segment { text: string; mention?: 'self' | 'other' }

const BLOCKS = new Set(['P', 'DIV', 'BR', 'LI', 'UL', 'OL', 'BLOCKQUOTE', 'PRE', 'H1', 'H2', 'H3', 'H4'])

/**
 * A body as flat text runs for a three-line card: no markup, paragraphs folded
 * to a space, and the people mentioned marked so the reader's own name can be
 * washed. A mention in rich text is `span[data-mention=<user id>]`; an older
 * plain or markdown body carries no such marks.
 */
export function messageSegments(content: string, format: string | undefined, me: string | undefined): Segment[] {
  if (format !== 'html' && !content.startsWith('<')) {
    const text = content.replace(/\s+/g, ' ').trim()
    return text ? [{ text }] : []
  }
  const doc = new DOMParser().parseFromString(sanitizeHtml(content), 'text/html')
  const out: Segment[] = []
  const push = (text: string, mention?: Segment['mention']) => {
    if (!text) return
    const prev = out[out.length - 1]
    if (prev && !prev.mention && !mention) prev.text += text
    else out.push(mention ? { text, mention } : { text })
  }
  const walk = (node: Node) => {
    node.childNodes.forEach((child) => {
      if (child.nodeType === Node.TEXT_NODE) {
        push((child.textContent ?? '').replace(/\s+/g, ' '))
        return
      }
      if (!(child instanceof Element)) return
      const who = child.getAttribute('data-mention')
      if (who !== null) {
        push(child.textContent ?? '', me && who === me ? 'self' : 'other')
        return
      }
      if (BLOCKS.has(child.tagName)) push(' ')
      walk(child)
      if (BLOCKS.has(child.tagName)) push(' ')
    })
  }
  walk(doc.body)
  // Folded paragraphs leave runs of spaces and a space at each end.
  const folded = out.map((s) => (s.mention ? s : { text: s.text.replace(/ {2,}/g, ' ') }))
  const first = folded[0]
  if (first && !first.mention) first.text = first.text.trimStart()
  const last = folded[folded.length - 1]
  if (last && !last.mention) last.text = last.text.trimEnd()
  return folded.filter((s) => s.text !== '')
}

export interface OpenTarget {
  channelId: string
  /** The message to centre and flash. */
  messageId: string
  /** Set for a thread reply: the root whose thread panel opens and flashes the reply. */
  rootId: string | undefined
}

/** Where opening a feed item leads. */
export function openTarget(m: FeedMessage): OpenTarget {
  return { channelId: m.channel_id, messageId: m.id, rootId: m.parent_message_id || undefined }
}

/** Whether a message just received puts a new item in my mentions feed: someone else's, naming me. */
export function mentionsMe(mentions: readonly string[] | undefined, senderId: string, me: string | undefined): boolean {
  return !!me && senderId !== me && !!mentions?.includes(me)
}
