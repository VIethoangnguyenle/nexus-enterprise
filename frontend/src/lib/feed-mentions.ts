export interface TextPart {
  text: string
  /** The part is "@" and the display name of someone the message mentions. */
  mention: boolean
}

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

/**
 * Cuts the plain-text preview of a message where it names someone. Only the
 * exact "@<display name>" of a person in `names` counts, and not when more of
 * a name follows, so "@Lê Thị Hoang" is not a mention of "Lê Thị Hoa".
 */
export function splitMentions(text: string, names: string[]): TextPart[] {
  if (!text) return []
  const wanted = [...new Set(names.filter(Boolean))].sort((a, b) => b.length - a.length)
  if (wanted.length === 0) return [{ text, mention: false }]
  const re = new RegExp(`@(?:${wanted.map(escape).join('|')})(?![\\p{L}\\p{N}])`, 'gu')
  const parts: TextPart[] = []
  let from = 0
  for (const m of text.matchAll(re)) {
    if (m.index > from) parts.push({ text: text.slice(from, m.index), mention: false })
    parts.push({ text: m[0], mention: true })
    from = m.index + m[0].length
  }
  if (from < text.length) parts.push({ text: text.slice(from), mention: false })
  return parts
}
