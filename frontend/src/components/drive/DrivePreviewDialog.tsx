import { X, Download, FolderInput, Image } from 'lucide-react'
import { IconButton, Button } from '../primitives'
import type { DriveItem } from '../../api/drive'

interface DrivePreviewDialogProps {
  item: DriveItem
  sharedBy?: string
  onClose: () => void
  onSave?: (item: DriveItem) => void
  onDownload: (item: DriveItem) => void
}

/** Modal dialog for previewing shared files — image preview + metadata + Save/Download buttons. Matches Stitch design. */
export function DrivePreviewDialog({ item, sharedBy, onClose, onSave, onDownload }: DrivePreviewDialogProps) {
  const isImage = item.mime_type?.startsWith('image/')
  const sizeStr = formatSize(item.size_bytes)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/40 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* Dialog */}
      <div className="relative bg-surface-container-lowest rounded-2xl shadow-xl w-full max-w-lg mx-4
        overflow-hidden animate-scale-in">
        {/* Close */}
        <IconButton
          onClick={onClose}
          aria-label="Close"
          className="absolute top-3 right-3 z-10"
        >
          <X size={18} />
        </IconButton>

        {/* Preview area */}
        <div className="flex items-center justify-center bg-surface-container-low p-8 min-h-[200px]">
          {isImage ? (
            <img
              src={`/drive/files/${item.id}/download`}
              alt={item.name}
              className="max-w-full max-h-64 rounded-lg object-contain"
              onError={(e) => {
                (e.target as HTMLImageElement).style.display = 'none'
                e.currentTarget.parentElement?.classList.add('preview-fallback')
              }}
            />
          ) : (
            <div className="flex flex-col items-center gap-3 text-on-surface-variant">
              <Image size={48} strokeWidth={1.2} />
              <span className="text-sm">Shared Design Asset Preview</span>
            </div>
          )}
        </div>

        {/* Metadata */}
        <div className="px-6 py-4">
          <h3 className="text-h3 text-on-surface mb-1">{item.name}</h3>
          <p className="text-xs text-on-surface-variant">
            {sharedBy ? `Shared by ${sharedBy}` : 'Shared file'} • {sizeStr}
          </p>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-3 px-6 pb-6">
          {onSave && (
            <Button
              variant="outline"
              onClick={() => onSave(item)}
              className="flex-1"
            >
              <FolderInput size={16} />
              Save to My Drive
            </Button>
          )}
          <Button
            onClick={() => onDownload(item)}
            className="flex-1"
          >
            <Download size={16} />
            Download
          </Button>
        </div>
      </div>
    </div>
  )
}

function formatSize(bytes: number): string {
  if (!bytes) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
