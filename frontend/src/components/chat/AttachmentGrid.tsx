import { Download } from 'lucide-react'
import type { MessageAttachment } from '../../api/messaging'
import { useDownloadUrl, useDownloadFile } from '../../hooks/useDownloadUrl'
import { fileKind } from '../../lib/file-kind'
import { formatBytes } from '../../lib/format'
import { IconButton, Pressable } from '../primitives'

export interface AttachmentGridProps {
  attachments: MessageAttachment[]
  tone?: 'raised' | 'base'
  onOpenImage?: (index: number, images: MessageAttachment[], openerEl?: HTMLElement | null) => void
}

export function isImageAttachment(att: MessageAttachment): boolean {
  if (att.mime_type?.startsWith('image/')) return true
  return /\.(png|jpe?g|gif|webp|svg|avif)$/i.test(att.file_name)
}

function singleAspectRatio(attachment: MessageAttachment): string {
  if (!attachment.width || !attachment.height) return '4 / 3'
  const ratio = attachment.width / attachment.height
  const clamped = Math.min(2, Math.max(0.5, ratio))
  return `${clamped}`
}

function ImageTile({
  attachment,
  total,
  extraCount,
  onClick,
}: {
  attachment: MessageAttachment
  total: number
  extraCount?: number
  onClick: (el: HTMLElement | null) => void
}) {
  const { data: downloadUrl } = useDownloadUrl(
    attachment.drive_file_id,
    !attachment.localUrl && !!attachment.drive_file_id,
  )
  const imageUrl = attachment.localUrl || downloadUrl

  const ariaLabel = extraCount
    ? `Mở ảnh ${attachment.file_name}, còn ${extraCount} ảnh nữa`
    : `Mở ảnh ${attachment.file_name}`

  return (
    <Pressable
      type="button"
      onClick={(e) => onClick(e.currentTarget)}
      aria-label={ariaLabel}
      className={`relative w-full h-full overflow-hidden bg-sunk block text-left group focus-visible:field-focus ${
        total === 1 ? 'rounded-surface' : 'rounded-lg'
      }`}
    >
      {imageUrl ? (
        <img
          src={imageUrl}
          alt={attachment.file_name}
          className="w-full h-full object-cover transition-transform group-hover:scale-[1.02]"
        />
      ) : (
        <div className="w-full h-full min-h-24 bg-sunk animate-pulse" />
      )}

      {extraCount !== undefined && extraCount > 0 && (
        <span className="absolute inset-0 bg-scrim/40 flex items-center justify-center pointer-events-none">
          <span className="px-2.5 py-1 rounded-full bg-overlay text-ink font-semibold text-sm shadow-overlay">
            +{extraCount}
          </span>
        </span>
      )}
    </Pressable>
  )
}

function FileCard({
  attachment,
  tone = 'raised',
}: {
  attachment: MessageAttachment
  tone?: 'raised' | 'base'
}) {
  const kind = fileKind(attachment.file_name || '', attachment.mime_type)
  const Icon = kind.icon
  const { download } = useDownloadFile()

  const handleDownload = (e: React.MouseEvent) => {
    e.stopPropagation()
    void download({ id: attachment.drive_file_id, name: attachment.file_name || 'file' })
  }

  return (
    <div
      className={`rounded-surface p-2.5 flex items-center gap-2.5 max-w-85 w-full border border-line ${
        tone === 'base' ? 'bg-base' : 'bg-raised'
      }`}
    >
      <div className={`w-9 h-9 rounded-lg flex items-center justify-center shrink-0 ${kind.tile}`}>
        <Icon size={18} strokeWidth={1.75} />
      </div>

      <div className="min-w-0 flex-1">
        <div className="text-xs sm:text-sm font-semibold text-ink truncate leading-snug">
          {attachment.file_name}
        </div>
        <div className="text-2xs sm:text-xs text-ink-muted tnum leading-tight mt-0.5 truncate">
          {kind.label} · {formatBytes(attachment.size_bytes)}
        </div>
      </div>

      <IconButton
        type="button"
        onClick={handleDownload}
        title={`Tải xuống ${attachment.file_name}`}
        aria-label={`Tải xuống ${attachment.file_name}`}
        className="shrink-0 max-md:min-w-11 max-md:min-h-11"
      >
        <Download size={16} strokeWidth={1.75} />
      </IconButton>
    </div>
  )
}

export function AttachmentGrid({
  attachments,
  tone = 'raised',
  onOpenImage,
}: AttachmentGridProps) {
  if (!attachments || attachments.length === 0) return null

  const images = attachments.filter(isImageAttachment)
  const docs = attachments.filter((a) => !isImageAttachment(a))

  const count = images.length
  const visibleImages = count > 4 ? images.slice(0, 4) : images
  const extraCount = count > 4 ? count - 4 : 0

  return (
    <div className="grid gap-2 mt-2 max-w-full">
      {count === 1 && (
        <div
          className="max-w-90 max-h-80 rounded-surface overflow-hidden"
          style={{ aspectRatio: singleAspectRatio(images[0]!) }}
        >
          <ImageTile
            attachment={images[0]!}
            total={1}
            onClick={(el) => onOpenImage?.(0, images, el)}
          />
        </div>
      )}

      {count > 1 && (
        <div
          className={
            count === 2
              ? 'grid grid-cols-2 gap-1 max-w-90 [&>button]:aspect-square'
              : count === 3
                ? 'grid grid-cols-2 grid-rows-2 gap-1 max-w-90 [&>button:first-child]:row-span-2 [&>button]:h-full'
                : 'grid grid-cols-2 gap-1 max-w-90 [&>button]:aspect-square'
          }
        >
          {visibleImages.map((img, idx) => (
            <ImageTile
              key={img.drive_file_id}
              attachment={img}
              total={count}
              extraCount={idx === 3 && extraCount > 0 ? extraCount : undefined}
              onClick={(el) => onOpenImage?.(idx, images, el)}
            />
          ))}
        </div>
      )}

      {docs.length > 0 && (
        <div className="grid gap-1.5 max-w-full">
          {docs.map((doc) => (
            <FileCard key={doc.drive_file_id} attachment={doc} tone={tone} />
          ))}
        </div>
      )}
    </div>
  )
}
