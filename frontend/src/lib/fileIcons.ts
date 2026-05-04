import {
  FileText, Image, Film, Music, Archive, Code,
  Table, Presentation, File, FileSpreadsheet
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

type FileIconResult = {
  icon: LucideIcon
  color: string
}

const EXT_MAP: Record<string, FileIconResult> = {
  // Documents
  pdf: { icon: FileText, color: '#ef4444' },
  doc: { icon: FileText, color: '#3b82f6' },
  docx: { icon: FileText, color: '#3b82f6' },
  txt: { icon: FileText, color: '#6b7280' },
  md: { icon: FileText, color: '#6b7280' },
  rtf: { icon: FileText, color: '#3b82f6' },
  // Spreadsheets
  xls: { icon: FileSpreadsheet, color: '#10b981' },
  xlsx: { icon: FileSpreadsheet, color: '#10b981' },
  csv: { icon: Table, color: '#10b981' },
  // Presentations
  ppt: { icon: Presentation, color: '#f59e0b' },
  pptx: { icon: Presentation, color: '#f59e0b' },
  // Images
  png: { icon: Image, color: '#8b5cf6' },
  jpg: { icon: Image, color: '#8b5cf6' },
  jpeg: { icon: Image, color: '#8b5cf6' },
  gif: { icon: Image, color: '#8b5cf6' },
  svg: { icon: Image, color: '#8b5cf6' },
  webp: { icon: Image, color: '#8b5cf6' },
  // Video
  mp4: { icon: Film, color: '#ec4899' },
  mov: { icon: Film, color: '#ec4899' },
  avi: { icon: Film, color: '#ec4899' },
  mkv: { icon: Film, color: '#ec4899' },
  webm: { icon: Film, color: '#ec4899' },
  // Audio
  mp3: { icon: Music, color: '#06b6d4' },
  wav: { icon: Music, color: '#06b6d4' },
  flac: { icon: Music, color: '#06b6d4' },
  ogg: { icon: Music, color: '#06b6d4' },
  // Archives
  zip: { icon: Archive, color: '#78716c' },
  rar: { icon: Archive, color: '#78716c' },
  '7z': { icon: Archive, color: '#78716c' },
  tar: { icon: Archive, color: '#78716c' },
  gz: { icon: Archive, color: '#78716c' },
  // Code
  js: { icon: Code, color: '#eab308' },
  ts: { icon: Code, color: '#3b82f6' },
  jsx: { icon: Code, color: '#06b6d4' },
  tsx: { icon: Code, color: '#06b6d4' },
  py: { icon: Code, color: '#3b82f6' },
  go: { icon: Code, color: '#06b6d4' },
  rs: { icon: Code, color: '#f97316' },
  html: { icon: Code, color: '#ef4444' },
  css: { icon: Code, color: '#8b5cf6' },
  json: { icon: Code, color: '#6b7280' },
  yaml: { icon: Code, color: '#6b7280' },
  yml: { icon: Code, color: '#6b7280' },
}

const MIME_PREFIX_MAP: Record<string, FileIconResult> = {
  image: { icon: Image, color: '#8b5cf6' },
  video: { icon: Film, color: '#ec4899' },
  audio: { icon: Music, color: '#06b6d4' },
  text: { icon: FileText, color: '#6b7280' },
}

const DEFAULT_ICON: FileIconResult = { icon: File, color: '#94a3b8' }

/** Resolve a Lucide icon and color for a filename or MIME type. */
export function getFileIcon(filename?: string, mimeType?: string): FileIconResult {
  if (filename) {
    const ext = filename.split('.').pop()?.toLowerCase()
    if (ext && EXT_MAP[ext]) return EXT_MAP[ext]
  }
  if (mimeType) {
    const prefix = mimeType.split('/')[0]
    if (MIME_PREFIX_MAP[prefix]) return MIME_PREFIX_MAP[prefix]
  }
  return DEFAULT_ICON
}
