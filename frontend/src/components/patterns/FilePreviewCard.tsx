import { useState } from 'react'
import { driveApi, type DriveItem } from '../../api/drive'
import { Spinner } from '../primitives'
import { getFileIcon } from '../../lib/fileIcons'
import { Download } from 'lucide-react'

/** Accept either a full DriveItem or just fileId+filename for lightweight rendering from messages. */
type FilePreviewCardProps =
  | { item: DriveItem; fileId?: never; filename?: never }
  | { fileId: string; filename: string; item?: never }

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
  const [downloadError, setDownloadError] = useState<string | null>(null)

  const id = props.item?.id ?? props.fileId!
  const name = props.item?.name ?? props.filename!
  const sizeBytes = props.item?.size_bytes ?? 0
  const mimeType = props.item?.mime_type
  const fi = getFileIcon(name, mimeType)
  const FileIcon = fi.icon
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
      setDownloadError(err.message || 'Download failed')
      setTimeout(() => setDownloadError(null), 5000)
    } finally {
      setDownloading(false)
    }
  }

  return (
    <div
      id={`file-card-${id}`}
      className="inline-flex items-center gap-3 bg-surface-container-low border border-outline-variant/40
        rounded-lg px-3 py-2.5 max-w-xs cursor-pointer
        hover:border-primary/30 hover:bg-surface-container
        transition-all duration-150 group mt-1"
      onClick={handleDownload}
      role="button"
      tabIndex={0}
      onKeyDown={e => e.key === 'Enter' && handleDownload()}
    >
      {/* Icon */}
      <div className="w-8 h-8 rounded-lg flex items-center justify-center
        bg-primary/10 flex-shrink-0">
        {downloading ? <Spinner size="sm" /> : <FileIcon size={18} color={fi.color} />}
      </div>

      {/* File info */}
      <div className="flex-1 min-w-0">
        <p className="text-small font-medium text-on-surface m-0 truncate leading-tight">{name}</p>
        <p className="text-micro text-on-surface-variant m-0">
          {downloadError
            ? <span className="text-error">{downloadError}</span>
            : <>{sizeBytes > 0 ? `${formatSize(sizeBytes)} · ` : ''}{extension}</>}
        </p>
      </div>

      {/* Download arrow */}
      <div className="w-6 h-6 flex items-center justify-center rounded-md
        text-on-surface-variant hover:text-primary hover:bg-primary/8
        transition-all duration-150 flex-shrink-0">
        <Download size={14} />
      </div>
    </div>
  )
}
