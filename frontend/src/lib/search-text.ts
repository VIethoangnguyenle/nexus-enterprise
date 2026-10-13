/**
 * Text matching for the Tìm trong chat screen: case and accent insensitive
 * ("doi soat" finds "Đối soát"), and the same fold drives the highlight so a
 * word is marked as it is written in the text, not as it was typed.
 */

/** Lower-case without accents; đ becomes d. Unlike `people.normalize` it keeps the spaces it is given. */
export function fold(s: string): string {
  return s
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .replace(/đ/g, 'd')
    .replace(/Đ/g, 'D')
    .toLowerCase()
}

/** The words of a query, folded; empty parts are dropped. */
export function searchTerms(query: string): string[] {
  return fold(query).split(/\s+/).filter(Boolean)
}

/** Every word of `query` occurs somewhere in `text`. A query with no words matches nothing. */
export function matchesAll(text: string, query: string): boolean {
  const terms = searchTerms(query)
  if (terms.length === 0) return false
  const hay = fold(text)
  return terms.every((t) => hay.includes(t))
}

export interface Part { text: string; hit: boolean }

/**
 * The ranges of `text` (in its own characters) that hold a word of `query`, in order. Hits that touch,
 * or that are different words of the query with only spaces between, become one range ("đối soát").
 */
function ranges(text: string, query: string): Array<[number, number]> {
  const terms = searchTerms(query)
  if (terms.length === 0 || !text) return []
  // Fold one character at a time so every folded index maps back to the original text.
  let folded = ''
  const start: number[] = []
  const end: number[] = []
  let at = 0
  for (const ch of text) {
    const f = fold(ch)
    for (let i = 0; i < f.length; i++) {
      start.push(at)
      end.push(at + ch.length)
    }
    folded += f
    at += ch.length
  }
  const found: Array<[number, number, number]> = []
  terms.forEach((t, n) => {
    for (let i = folded.indexOf(t); i >= 0; i = folded.indexOf(t, i + t.length)) {
      found.push([start[i]!, end[i + t.length - 1]!, n])
    }
  })
  found.sort((a, b) => a[0] - b[0])
  const merged: Array<[number, number]> = []
  let lastTerm = -1
  for (const [a, b, n] of found) {
    const last = merged[merged.length - 1]
    if (last && (a <= last[1] || (n !== lastTerm && /^\s+$/.test(text.slice(last[1], a))))) last[1] = Math.max(last[1], b)
    else merged.push([a, b])
    lastTerm = n
  }
  return merged
}

/** `text` cut into runs, the ones holding a word of `query` marked as hits. */
export function highlight(text: string, query: string): Part[] {
  if (!text) return []
  const marks = ranges(text, query)
  if (marks.length === 0) return [{ text, hit: false }]
  const parts: Part[] = []
  let at = 0
  for (const [a, b] of marks) {
    if (a > at) parts.push({ text: text.slice(at, a), hit: false })
    parts.push({ text: text.slice(a, b), hit: true })
    at = b
  }
  if (at < text.length) parts.push({ text: text.slice(at), hit: false })
  return parts
}

/** At most about `max` characters of `text` around its first match, with … on a cut side. */
export function snippet(text: string, query: string, max = 140): string {
  const flat = text.replace(/\s+/g, ' ').trim()
  if (flat.length <= max) return flat
  const first = ranges(flat, query)[0]
  const from = Math.max(0, Math.min((first?.[0] ?? 0) - Math.floor(max / 4), flat.length - max))
  const to = Math.min(flat.length, from + max)
  return `${from > 0 ? '…' : ''}${flat.slice(from, to).trim()}${to < flat.length ? '…' : ''}`
}
