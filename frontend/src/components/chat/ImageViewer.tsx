import { useEffect, useState, useRef, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { X, ChevronLeft, ChevronRight, Download } from 'lucide-react'
import type { MessageAttachment } from '../../api/messaging'
import { useDownloadUrl, useDownloadFile } from '../../hooks/useDownloadUrl'
import { useModalFocus } from '../../hooks/useModalFocus'
import { formatBytes, formatTime } from '../../lib/format'
import { IconButton } from '../primitives'

export interface ImageViewerProps {
  open: boolean
  images: MessageAttachment[]
  initialIndex?: number
  senderName?: string
  sentAt?: unknown
  onClose: () => void
  returnFocusRef?: React.RefObject<HTMLElement | null>
}

export function ImageViewer({
  open,
  images,
  initialIndex = 0,
  senderName,
  sentAt,
  onClose,
  returnFocusRef,
}: ImageViewerProps) {
  const [index, setIndex] = useState(initialIndex)
  const surfaceRef = useRef<HTMLDivElement>(null)
  const closeBtnRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (open) {
      setIndex(Math.min(Math.max(0, initialIndex), Math.max(0, images.length - 1)))
    }
  }, [open, initialIndex, images.length])

  useModalFocus({
    active: open,
    surfaceRef,
    onClose,
    initialFocusRef: closeBtnRef,
  })

  // Ensure focus returns to the opener when unmounted
  useEffect(() => {
    return () => {
      returnFocusRef?.current?.focus()
    }
  }, [returnFocusRef])

  const handlePrev = useCallback(() => {
    setIndex((i) => Math.max(0, i - 1))
  }, [])

  const handleNext = useCallback(() => {
    setIndex((i) => Math.min(images.length - 1, i + 1))
  }, [images.length])

  useEffect(() => {
    if (!open) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft') {
        e.preventDefault()
        handlePrev()
      } else if (e.key === 'ArrowRight') {
        e.preventDefault()
        handleNext()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [open, handlePrev, handleNext])

  const current = images[index]
  const { data: downloadUrl } = useDownloadUrl(current?.drive_file_id, open && !!current?.drive_file_id)
  const { download } = useDownloadFile()

  if (!open || !current) return null

  const handleDownload = () => {
    void download({ id: current.drive_file_id, name: current.file_name })
  }

  const hasPrev = index > 0
  const hasNext = index < images.length - 1

  return createPortal(
    <div
      ref={surfaceRef}
      role="dialog"
      aria-modal="true"
      aria-label="Trình xem ảnh"
      className="fixed inset-0 z-modal grid grid-rows-[auto_minmax(0,1fr)_auto] bg-sunk select-none animate-fade-in"
    >
      {/* Top bar */}
      <div className="flex items-center gap-3 min-h-14 px-4 py-2 bg-base border-b border-line">
        <div className="min-w-0 flex-1">
          <div className="font-semibold text-sm text-ink truncate leading-tight">
            {current.file_name}
          </div>
          <div className="text-xs text-ink-muted truncate tnum mt-0.5">
            {senderName ? `${senderName} · ` : ''}
            {sentAt ? `${formatTime(sentAt)} · ` : ''}
            {formatBytes(current.size_bytes)}
          </div>
        </div>

        <div className="flex items-center gap-1 shrink-0 ml-auto">
          <IconButton
            type="button"
            onClick={handleDownload}
            title="Tải xuống"
            aria-label="Tải xuống"
            className="max-md:min-w-11 max-md:min-h-11"
          >
            <Download size={18} strokeWidth={1.75} />
          </IconButton>
          <IconButton
            ref={closeBtnRef}
            type="button"
            onClick={onClose}
            title="Đóng (Esc)"
            aria-label="Đóng trình xem"
            className="max-md:min-w-11 max-md:min-h-11"
          >
            <X size={18} strokeWidth={1.75} />
          </IconButton>
        </div>
      </div>

      {/* Stage */}
      <div className="relative flex items-center justify-center p-4 md:px-18 min-h-0">
        {downloadUrl ? (
          <img
            src={downloadUrl}
            alt={current.file_name}
            className="max-w-full max-h-full object-contain"
          />
        ) : (
          <div className="w-48 h-48 bg-base/50 rounded-xl animate-pulse" />
        )}

        {hasPrev && (
          <IconButton
            type="button"
            onClick={handlePrev}
            aria-label="Ảnh trước"
            title="Ảnh trước"
            className="absolute left-4 top-1/2 -translate-y-1/2 w-11 h-11 rounded-full bg-overlay/90 shadow-overlay flex items-center justify-center text-ink hover:bg-hover max-md:min-w-11 max-md:min-h-11"
          >
            <ChevronLeft size={22} strokeWidth={2} />
          </IconButton>
        )}

        {hasNext && (
          <IconButton
            type="button"
            onClick={handleNext}
            aria-label="Ảnh sau"
            title="Ảnh sau"
            className="absolute right-4 top-1/2 -translate-y-1/2 w-11 h-11 rounded-full bg-overlay/90 shadow-overlay flex items-center justify-center text-ink hover:bg-hover max-md:min-w-11 max-md:min-h-11"
          >
            <ChevronRight size={22} strokeWidth={2} />
          </IconButton>
        )}
      </div>

      {/* Bottom counter */}
      <div
        className="text-center py-2 pb-4 font-semibold text-xs text-ink-muted tnum"
        aria-live="polite"
      >
        {index + 1} / {images.length}
      </div>
    </div>,
    document.body,
  )
}
