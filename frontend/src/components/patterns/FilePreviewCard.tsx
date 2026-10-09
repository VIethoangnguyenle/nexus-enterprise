import { useState } from 'react'
import { driveApi, type DriveItem } from '../../api/drive'
import { Pressable, Spinner } from '../primitives'
import { fileKind } from '../../lib/file-kind'
import { formatBytes } from '../../lib/format'

/** Accept either a full DriveItem or just fileId+filename for lightweight rendering from messages. */
type FilePreviewCardProps =
  | { item: DriveItem; fileId?: never; filename?: never }
  | { fileId: string; filename: string; item?: never }

/**
 * File attached to a message (design/mockups/spaces.html §2): kind tile, name,
 * "Bảng tính · 412 KB". Activating it downloads the file.
 */
export function FilePreviewCard(props: FilePreviewCardProps) {
  const [downloading, setDownloading] = useState(false)
  const [downloadError, setDownloadError] = useState<string | null>(null)

  const id = props.item?.id ?? props.fileId!
  const name = props.item?.name ?? props.filename!
  const sizeBytes = props.item?.size_bytes ?? 0
  const kind = fileKind(name, props.item?.mime_type)
  const Icon = kind.icon

  const handleDownload = async () => {
    setDownloading(true)
    try {
      const { download_url } = await driveApi.getDownloadUrl(id)
      const a = document.createElement('a')
      a.href = download_url
      a.download = name
      a.click()
    } catch {
      setDownloadError('Chưa tải được tệp. Thử lại sau.')
      setTimeout(() => setDownloadError(null), 5000)
    } finally {
      setDownloading(false)
    }
  }

  return (
    <Pressable
      onClick={handleDownload}
      aria-label={`Tải xuống ${name}`}
      className="inline-flex items-center gap-3 mt-2 py-2.5 pl-2.5 pr-3.5 rounded-surface bg-base max-w-85
        transition-colors duration-quick hover:bg-hover"
    >
      <span className={`grid place-items-center w-9 h-9 rounded-md shrink-0 ${kind.tile}`} aria-hidden="true">
        {downloading ? <Spinner size="sm" /> : <Icon size={18} strokeWidth={1.75} />}
      </span>
      <span className="grid min-w-0">
        <span className="font-semibold text-ink truncate">{name}</span>
        <span className="text-xs text-ink-muted tnum">
          {downloadError ?? [kind.label, sizeBytes > 0 ? formatBytes(sizeBytes) : ''].filter(Boolean).join(' · ')}
        </span>
      </span>
    </Pressable>
  )
}
