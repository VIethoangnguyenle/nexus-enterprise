/** Pure helpers behind the viewer's text, code, CSV and file-name displays. */

/** The viewer shows at most this much of a text file: 1 MB or 500 lines. */
export const MAX_BYTES = 1024 * 1024
export const MAX_LINES = 500

export const fileExt = (name: string) => (name.includes('.') ? name.split('.').pop()!.toLowerCase() : '')

/**
 * Reads a response body up to `maxBytes` and stops there (the rest is never
 * downloaded). `cut` says there was more. A character split by the cap is dropped.
 */
export async function readCappedText(res: Response, maxBytes: number): Promise<{ text: string; cut: boolean }> {
  const reader = res.body?.getReader()
  if (!reader) {
    const all = await res.text()
    return { text: all.slice(0, maxBytes), cut: all.length > maxBytes }
  }
  const chunks: Uint8Array[] = []
  let size = 0
  let cut = false
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    const room = maxBytes - size
    if (value.length > room) {
      chunks.push(value.subarray(0, room))
      size += room
      cut = true
      await reader.cancel().catch(() => undefined)
      break
    }
    chunks.push(value)
    size += value.length
  }
  const bytes = new Uint8Array(size)
  let at = 0
  for (const c of chunks) {
    bytes.set(c, at)
    at += c.length
  }
  // Streaming decode keeps a half character at the end out of the text.
  const text = new TextDecoder('utf-8').decode(bytes, { stream: cut })
  return { text, cut }
}

/** The first `max` lines; `total` is the line count of `text` (a trailing newline is not a line). */
export function takeLines(text: string, max: number): { lines: string[]; total: number } {
  const all = text.split(/\r\n|\n|\r/)
  if (all.length > 1 && all[all.length - 1] === '') all.pop()
  return { lines: all.slice(0, max), total: all.length }
}

/** RFC 4180 cells; the delimiter (`,` `;` or tab) is the commonest one in the first line. */
export function parseCsv(text: string): string[][] {
  const firstLine = text.split(/\r\n|\n|\r/, 1)[0] ?? ''
  const count = (ch: string) => firstLine.split(ch).length - 1
  const delim = [',', ';', '\t'].reduce((best, d) => (count(d) > count(best) ? d : best), ',')

  const rows: string[][] = []
  let row: string[] = []
  let cell = ''
  let quoted = false
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (quoted) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          cell += '"'
          i++
        } else quoted = false
      } else cell += ch
    } else if (ch === '"' && cell === '') quoted = true
    else if (ch === delim) {
      row.push(cell)
      cell = ''
    } else if (ch === '\n' || ch === '\r') {
      if (ch === '\r' && text[i + 1] === '\n') i++
      row.push(cell)
      rows.push(row)
      row = []
      cell = ''
    } else cell += ch
  }
  if (cell !== '' || row.length > 0) {
    row.push(cell)
    rows.push(row)
  }
  return rows
}

const CODE_EXT = new Set([
  'js', 'jsx', 'mjs', 'cjs', 'ts', 'tsx', 'py', 'go', 'rs', 'java', 'kt', 'c', 'h', 'cpp', 'hpp', 'cs', 'rb', 'php',
  'sh', 'bash', 'sql', 'json', 'yaml', 'yml', 'toml', 'ini', 'xml', 'html', 'htm', 'svg', 'css', 'scss', 'lua', 'swift',
])

export type TextFlavor = 'csv' | 'code' | 'text'

/** How a fetched text file is laid out: a table, numbered lines, or wrapped prose. */
export function textFlavor(name: string, mime: string): TextFlavor {
  const ext = fileExt(name)
  if (ext === 'csv' || mime.includes('csv')) return 'csv'
  if (CODE_EXT.has(ext) || mime === 'application/json') return 'code'
  return 'text'
}

const TAIL = 10

/** A long name split so the end (with the extension) survives truncation of the start. */
export function splitMiddle(name: string): [string, string] {
  if (name.length <= TAIL + 6) return [name, '']
  return [name.slice(0, name.length - TAIL), name.slice(name.length - TAIL)]
}

const OFFICE_LABEL: Record<string, string> = {
  doc: 'Word', docx: 'Word', rtf: 'Word', ppt: 'PowerPoint', pptx: 'PowerPoint', odt: 'OpenDocument', odp: 'OpenDocument',
}

/** What to call the source of a server-made PDF, for the "may differ from the original" note. */
export const officeLabel = (name: string) => OFFICE_LABEL[fileExt(name)] ?? 'tài liệu'

/** Files the drive converts: used to promise "đang chuẩn bị" instead of a skeleton while waiting. */
export const isConvertible = (name: string) => fileExt(name) in OFFICE_LABEL
