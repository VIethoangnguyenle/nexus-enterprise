import { useState } from 'react'
import { driveApi, type DriveItem } from '../../api/drive'
import { Spinner } from '../primitives'

/** Accept either a full DriveItem or just fileId+filename for lightweight rendering from messages. */
type FilePreviewCardProps =
  | { item: DriveItem; fileId?: never; filename?: never }
  | { fileId: string; filename: string; item?: never }

/** Derives a display icon from a filename extension. */
function getIcon(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  const map: Record<string, string> = {
    pdf: '📕', doc: '📘', docx: '📘', xls: '📗', xlsx: '📗', csv: '📊',
    jpg: '🖼️', jpeg: '🖼️', png: '🖼️', gif: '🖼️', svg: '🖼️', webp: '🖼️',
    mp4: '🎬', webm: '🎬', mov: '🎬',
    mp3: '🎵', wav: '🎵', ogg: '🎵',
    zip: '📦', rar: '📦', tar: '📦', gz: '📦',
    js: '💻', ts: '💻', go: '💻', py: '💻', rs: '💻',
    md: '📝', txt: '📝',
  }
  return map[ext] || '📄'
}

/** Formats byte count into a human-readable string. */
function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

/** Inline file card for chat messages — shows icon, filename, size, and download action. */
export function FilePreviewCard(props: FilePreviewCardProps) {
  const [downloading, setDownloading] = useState(false)

  const id = props.item?.id ?? props.fileId!
  const name = props.item?.name ?? props.filename!
  const sizeBytes = props.item?.size_bytes ?? 0
  const icon = getIcon(name)
  const extension = name.split('.').pop()?.toUpperCase() || 'FILE'

  const handleDownload = async () => {
    setDownloading(true)
    try {
      const { download_url } = await driveApi.getDownloadUrl(id)
      const a = document.createElement('a')
      a.href = download_url
      a.download = name
      a.click()
    } catch (err: any) {
      alert(`Download failed: ${err.message}`)
    } finally {
      setDownloading(false)
    }
  }

  return (
    <div
      id={`file-card-${id}`}
      className="inline-flex items-center gap-3 bg-bg-glass border border-border/60
        rounded-[var(--radius-md)] px-3 py-2.5 max-w-[340px] cursor-pointer
        hover:border-accent/40 hover:bg-bg-hover hover:shadow-[0_2px_12px_var(--color-accent-glow)]
        transition-all duration-200 group mt-1.5"
      onClick={handleDownload}
      role="button"
      tabIndex={0}
      onKeyDown={e => e.key === 'Enter' && handleDownload()}
    >
      {/* Icon */}
      <div className="w-9 h-9 rounded-[var(--radius-sm)] flex items-center justify-center
        bg-accent/10 text-lg flex-shrink-0 group-hover:bg-accent/20 transition-colors">
        {downloading ? <Spinner size="sm" /> : icon}
      </div>

      {/* File info */}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-text-primary m-0 truncate leading-tight">{name}</p>
        <p className="text-xs text-text-muted m-0 mt-0.5">
          {sizeBytes > 0 ? `${formatSize(sizeBytes)} · ` : ''}{extension} file
        </p>
      </div>

      {/* Download indicator */}
      <div className="w-6 h-6 flex items-center justify-center rounded-full
        text-text-muted opacity-0 group-hover:opacity-100
        group-hover:text-accent transition-all duration-200 flex-shrink-0">
        ⬇
      </div>
    </div>
  )
}
