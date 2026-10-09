import { FileText, FileSpreadsheet, Presentation, Image, Film, Music, Archive, FileCode, File } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

/**
 * What a file is, in words, with an icon and a tile tone from the semantic
 * tokens. Chat and the space's Tệp tab speak this; the drive keeps its own
 * per-extension colours (lib/fileIcons.ts).
 */
export interface FileKind {
  label: string
  icon: LucideIcon
  /** Utility classes for the 36px icon tile. */
  tile: string
}

const SHEET: FileKind = { label: 'Bảng tính', icon: FileSpreadsheet, tile: 'bg-success-wash text-success' }
const DOC: FileKind = { label: 'Văn bản', icon: FileText, tile: 'bg-info-wash text-info' }
const PDF: FileKind = { label: 'PDF', icon: FileText, tile: 'bg-danger-wash text-danger' }
const SLIDES: FileKind = { label: 'Trình chiếu', icon: Presentation, tile: 'bg-warning-wash text-warning' }
const IMAGE: FileKind = { label: 'Ảnh', icon: Image, tile: 'bg-accent-wash text-accent' }
const VIDEO: FileKind = { label: 'Video', icon: Film, tile: 'bg-accent-wash text-accent' }
const AUDIO: FileKind = { label: 'Âm thanh', icon: Music, tile: 'bg-accent-wash text-accent' }
const ARCHIVE: FileKind = { label: 'Tệp nén', icon: Archive, tile: 'bg-sunk text-ink-muted' }
const CODE: FileKind = { label: 'Mã nguồn', icon: FileCode, tile: 'bg-sunk text-ink-muted' }
const OTHER: FileKind = { label: 'Tệp', icon: File, tile: 'bg-sunk text-ink-muted' }

const BY_EXT: Record<string, FileKind> = {
  xls: SHEET, xlsx: SHEET, csv: SHEET, ods: SHEET,
  doc: DOC, docx: DOC, txt: DOC, md: DOC, rtf: DOC, odt: DOC,
  pdf: PDF,
  ppt: SLIDES, pptx: SLIDES, odp: SLIDES,
  png: IMAGE, jpg: IMAGE, jpeg: IMAGE, gif: IMAGE, webp: IMAGE, svg: IMAGE, avif: IMAGE,
  mp4: VIDEO, mov: VIDEO, webm: VIDEO, mkv: VIDEO,
  mp3: AUDIO, wav: AUDIO, ogg: AUDIO, flac: AUDIO,
  zip: ARCHIVE, rar: ARCHIVE, '7z': ARCHIVE, gz: ARCHIVE, tar: ARCHIVE,
  js: CODE, ts: CODE, tsx: CODE, py: CODE, go: CODE, json: CODE, yaml: CODE, yml: CODE, html: CODE, css: CODE,
}

export function fileKind(name: string, mime?: string): FileKind {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  const byExt = BY_EXT[ext]
  if (byExt) return byExt
  const top = mime?.split('/')[0]
  if (top === 'image') return IMAGE
  if (top === 'video') return VIDEO
  if (top === 'audio') return AUDIO
  if (top === 'text') return DOC
  return OTHER
}
