import { useEffect, useState } from 'react'
import { X, RotateCcw, AlertTriangle } from 'lucide-react'
import type { AttachmentUploadItem } from '../../hooks/useAttachmentUploads'
import { fileKind } from '../../lib/file-kind'
import { formatBytes } from '../../lib/format'
import { IconButton } from '../primitives'

export interface AttachmentTrayProps {
  items: AttachmentUploadItem[]
  onRemove: (key: string) => void
  onRetry: (key: string) => void
}

function ImageItem({
  item,
  onRemove,
  onRetry,
}: {
  item: AttachmentUploadItem
  onRemove: (key: string) => void
  onRetry: (key: string) => void
}) {
  const [src, setSrc] = useState('')

  useEffect(() => {
    const url = URL.createObjectURL(item.file)
    setSrc(url)
    return () => {
      URL.revokeObjectURL(url)
    }
  }, [item.file])

  const isUploading = item.status === 'uploading'
  const isError = item.status === 'error'
  const pct = item.progress !== undefined ? item.progress : 0
  const statusText = isUploading
    ? (item.progress !== undefined ? `Đang tải ${pct}%` : 'Đang tải')
    : isError
      ? (item.error ?? 'Lỗi tải lên')
      : formatBytes(item.file.size)

  return (
    <div
      role="group"
      aria-label={`${item.file.name}, ${statusText}`}
      title={`${item.file.name} · ${statusText}`}
      className={`relative w-16 h-16 rounded-lg overflow-hidden bg-base shrink-0 border border-line ${
        isError ? 'border-danger' : ''
      }`}
    >
      {src && (
        <img
          src={src}
          alt={item.file.name}
          className={`w-full h-full object-cover ${isUploading ? 'opacity-60' : ''}`}
        />
      )}

      {isUploading && (
        <div className="absolute inset-x-0 bottom-0 h-0.5 bg-line overflow-hidden">
          <div
            className="h-full bg-accent transition-all duration-quick"
            style={{ width: `${Math.max(5, pct)}%` }}
          />
        </div>
      )}

      {isError && (
        <div className="absolute inset-0 bg-base/80 flex items-center justify-center p-1">
          <IconButton
            type="button"
            onClick={() => onRetry(item.key)}
            title={`Thử lại ${item.file.name}`}
            aria-label={`Thử lại ${item.file.name}`}
            className="w-7 h-7 rounded-full bg-overlay shadow-overlay flex items-center justify-center text-ink hover:text-accent max-md:min-w-11 max-md:min-h-11"
          >
            <RotateCcw size={14} strokeWidth={2} />
          </IconButton>
        </div>
      )}

      <IconButton
        type="button"
        onClick={() => onRemove(item.key)}
        title={`Bỏ ${item.file.name}`}
        aria-label={`Bỏ ${item.file.name}`}
        className="absolute top-1 right-1 w-6 h-6 rounded-full bg-overlay/90 shadow-overlay flex items-center justify-center text-ink hover:bg-hover max-md:min-w-11 max-md:min-h-11 z-10"
      >
        <X size={12} strokeWidth={2.5} />
      </IconButton>
    </div>
  )
}

function DocItem({
  item,
  onRemove,
  onRetry,
}: {
  item: AttachmentUploadItem
  onRemove: (key: string) => void
  onRetry: (key: string) => void
}) {
  const isUploading = item.status === 'uploading'
  const isError = item.status === 'error'
  const kind = fileKind(item.file.name, item.file.type)
  const Icon = isError ? AlertTriangle : kind.icon
  const pct = item.progress !== undefined ? item.progress : 0

  const statusText = isUploading
    ? (item.progress !== undefined ? `Đang tải ${pct}%` : 'Đang tải…')
    : isError
      ? item.error || 'Lỗi tải lên'
      : `${kind.label} · ${formatBytes(item.file.size)}`

  return (
    <div
      role="group"
      aria-label={`${item.file.name}, ${statusText}`}
      className={`relative h-14 w-60 max-w-full rounded-lg bg-base border border-line pl-2.5 pr-8 flex items-center gap-2.5 shrink-0 ${
        isError ? 'border-danger' : ''
      }`}
    >
      <div
        className={`w-8 h-8 rounded-md flex items-center justify-center shrink-0 ${
          isError ? 'bg-danger-wash text-danger' : kind.tile
        }`}
      >
        <Icon size={16} strokeWidth={1.75} />
      </div>

      <div className="min-w-0 flex-1 py-1">
        <div className="text-xs font-semibold text-ink truncate leading-tight">
          {item.file.name}
        </div>
        <div
          className={`text-2xs truncate tnum leading-tight mt-0.5 ${
            isError ? 'text-danger font-medium' : 'text-ink-muted'
          }`}
        >
          {statusText}
        </div>
      </div>

      {isError && (
        <IconButton
          type="button"
          onClick={() => onRetry(item.key)}
          title={`Thử lại ${item.file.name}`}
          aria-label={`Thử lại ${item.file.name}`}
          className="w-7 h-7 rounded-full text-ink hover:text-accent flex items-center justify-center shrink-0 max-md:min-w-11 max-md:min-h-11"
        >
          <RotateCcw size={14} strokeWidth={2} />
        </IconButton>
      )}

      <IconButton
        type="button"
        onClick={() => onRemove(item.key)}
        title={`Bỏ ${item.file.name}`}
        aria-label={`Bỏ ${item.file.name}`}
        className="absolute top-1.5 right-1.5 w-6 h-6 rounded-full bg-overlay/90 shadow-overlay flex items-center justify-center text-ink hover:bg-hover max-md:min-w-11 max-md:min-h-11 z-10"
      >
        <X size={12} strokeWidth={2.5} />
      </IconButton>

      {isUploading && (
        <div className="absolute inset-x-0 bottom-0 h-0.5 bg-line overflow-hidden">
          <div
            className="h-full bg-accent transition-all duration-quick"
            style={{ width: `${Math.max(5, pct)}%` }}
          />
        </div>
      )}
    </div>
  )
}

export function AttachmentTray({ items, onRemove, onRetry }: AttachmentTrayProps) {
  if (items.length === 0) return null

  return (
    <div
      role="region"
      aria-label="Tệp đính kèm chờ gửi"
      className="flex flex-wrap gap-2 max-h-32 overflow-y-auto py-1 px-0.5"
    >
      {items.map((item) => {
        const isImage = item.file.type.startsWith('image/')
        return isImage ? (
          <ImageItem key={item.key} item={item} onRemove={onRemove} onRetry={onRetry} />
        ) : (
          <DocItem key={item.key} item={item} onRemove={onRemove} onRetry={onRetry} />
        )
      })}
    </div>
  )
}
