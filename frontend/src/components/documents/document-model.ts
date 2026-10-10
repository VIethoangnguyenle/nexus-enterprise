import type { TextDocument, TextScope, TextStatus } from '../../api/documents'
import { formatTime } from '../../lib/format'
import type { Pill } from '../../lib/approval-model'
import { sanitizeHtml } from '../../lib/sanitize-html'

export const STATUSES: readonly TextStatus[] = ['draft', 'active', 'archived']

const STATUS_PILL: Record<TextStatus, Pill> = {
  draft: { label: 'Bản nháp', tone: 'wait' },
  active: { label: 'Đang dùng', tone: 'ok' },
  archived: { label: 'Lưu trữ', tone: 'idle' },
}

/** A status as a labelled pill; an unknown value reads as a draft rather than as a code. */
export const statusPill = (status: string): Pill => STATUS_PILL[status as TextStatus] ?? STATUS_PILL.draft

/** Which documents a view lists, and what the view is called. */
export type TextGroup = 'all' | 'drafts' | 'shared'

export const GROUPS: { id: TextGroup; label: string; title: string; scope: TextScope }[] = [
  { id: 'all', label: 'Tất cả văn bản', title: 'Tất cả văn bản', scope: 'all' },
  { id: 'drafts', label: 'Bản nháp của tôi', title: 'Bản nháp của tôi', scope: 'drafts' },
  { id: 'shared', label: 'Được chia sẻ', title: 'Văn bản được chia sẻ với tôi', scope: 'shared' },
]

export const groupOf = (id: string | undefined) => GROUPS.find((g) => g.id === id) ?? GROUPS[0]!

/** Case- and accent-insensitive title match is done by the caller; this is the title shown. */
export const titleOf = (d: Pick<TextDocument, 'title'>) => d.title.trim() || 'Văn bản chưa đặt tên'

// ---- saving ----

/** Where a document's saving stands. */
export type SaveState = 'saved' | 'dirty' | 'saving' | 'offline' | 'error' | 'conflict'

/**
 * The words next to the title. There is no Save button, so this line is how a
 * person knows their work is safe (mockup §3): "Đang lưu…", "Đã lưu 09:41",
 * "Chưa lưu, đang chờ mạng".
 */
export function saveStateLabel(state: SaveState, savedAt: Date | null): string {
  switch (state) {
    case 'saving': return 'Đang lưu…'
    case 'dirty': return 'Chưa lưu'
    case 'offline': return 'Chưa lưu, đang chờ mạng'
    case 'error': return 'Chưa lưu được'
    case 'conflict': return 'Chưa lưu: có bản mới hơn'
    case 'saved': return savedAt ? `Đã lưu ${formatTime(savedAt)}` : 'Đã lưu'
  }
}

// ---- comparing two versions ----

const BLOCKS = 'p, h1, h2, h3, h4, h5, h6, li, blockquote, pre'

/** A document's text, one entry per paragraph, heading or list item. Markup is not shown. */
export function textLines(html: string): string[] {
  const doc = new DOMParser().parseFromString(sanitizeHtml(html), 'text/html')
  const lines: string[] = []
  for (const el of Array.from(doc.body.querySelectorAll(BLOCKS))) {
    // A block that holds other blocks (a list item with a paragraph, a quote) is read through them.
    if (el.querySelector(BLOCKS)) continue
    const text = (el.textContent ?? '').replace(/\s+/g, ' ').trim()
    if (text) lines.push(text)
  }
  if (!lines.length) {
    const text = (doc.body.textContent ?? '').replace(/\s+/g, ' ').trim()
    if (text) lines.push(text)
  }
  return lines
}

export interface DiffLine { text: string; changed: boolean }

/** Past this many lines a side-by-side diff stops being useful, and costs too much to compute. */
const DIFF_MAX = 1500

/**
 * Marks which lines of each side have no counterpart in the other, by longest
 * common subsequence: a line that moved is not "changed", a line only one side
 * has is.
 */
export function diffLines(mine: string[], theirs: string[]): { mine: DiffLine[]; theirs: DiffLine[] } {
  if (mine.length > DIFF_MAX || theirs.length > DIFF_MAX) {
    return {
      mine: mine.map((text) => ({ text, changed: false })),
      theirs: theirs.map((text) => ({ text, changed: false })),
    }
  }
  const n = mine.length
  const m = theirs.length
  const lcs: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i]![j] = mine[i] === theirs[j] ? lcs[i + 1]![j + 1]! + 1 : Math.max(lcs[i + 1]![j]!, lcs[i]![j + 1]!)
    }
  }
  const a: DiffLine[] = []
  const b: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (mine[i] === theirs[j]) {
      a.push({ text: mine[i]!, changed: false })
      b.push({ text: theirs[j]!, changed: false })
      i++
      j++
    } else if (lcs[i + 1]![j]! >= lcs[i]![j + 1]!) {
      a.push({ text: mine[i++]!, changed: true })
    } else {
      b.push({ text: theirs[j++]!, changed: true })
    }
  }
  while (i < n) a.push({ text: mine[i++]!, changed: true })
  while (j < m) b.push({ text: theirs[j++]!, changed: true })
  return { mine: a, theirs: b }
}
