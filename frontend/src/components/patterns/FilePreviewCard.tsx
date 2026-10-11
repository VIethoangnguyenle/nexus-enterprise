import { useMemo, useRef, useState } from 'react'
import { Download } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import { useDownloadFile } from '../../hooks/useDownloadUrl'
import { IconButton, Pressable, Spinner } from '../primitives'
import { fileKind } from '../../lib/file-kind'
import { formatBytes } from '../../lib/format'
import { FileViewer, type ViewerFile } from '../files/FileViewer'

interface FilePreviewCardBaseProps {
  files?: ViewerFile[]
  index?: number
  origin?: 'chat' | 'drive'
}

/** Accept either a full DriveItem or just fileId+filename for lightweight rendering from messages. */
type FilePreviewCardProps = FilePreviewCardBaseProps & (
  | { item: DriveItem; fileId?: never; filename?: never }
  | { fileId: string; filename: string; item?: never }
)

/**
 * File attached to a message (design/mockups/spaces.html §2, DESIGN.md §6): kind tile, name,
 * "Bảng tính · 412 KB". Activating it opens FileViewer; download button at the end.
 */
export function FilePreviewCard(props: FilePreviewCardProps) {
  const id = props.item?.id ?? props.fileId!
  const name = props.item?.name ?? props.filename!
  const sizeBytes = props.item?.size_bytes ?? 0
  const kind = fileKind(name, props.item?.mime_type)
  const Icon = kind.icon

  const { download, isDownloading } = useDownloadFile()
  const [open, setOpen] = useState(false)
  const cardRef = useRef<HTMLButtonElement>(null)

  const viewerFiles = useMemo<ViewerFile[]>(() => {
    if (props.files && props.files.length > 0) return props.files
    return [
      {
        id,
        name,
        mime: props.item?.mime_type,
        size: sizeBytes,
        uploaderName: props.item?.owner_name,
        uploaderId: props.item?.owner_id,
        time: props.item?.created_at,
        parentFolderId: props.item?.parent_id,
      },
    ]
  }, [props.files, id, name, props.item, sizeBytes])

  return (
    <div className="relative inline-flex items-center group max-w-85 mt-2">
      <Pressable
        ref={cardRef}
        onClick={() => setOpen(true)}
        aria-label={`Xem trước ${name}`}
        className="flex items-center gap-3 w-full py-2.5 pl-2.5 pr-11 rounded-surface bg-base
          transition-colors duration-quick hover:bg-hover text-left"
      >
        <span className={`grid place-items-center w-9 h-9 rounded-md shrink-0 ${kind.tile}`} aria-hidden="true">
          <Icon size={18} strokeWidth={1.75} />
        </span>
        <span className="grid min-w-0">
          <span className="font-semibold text-ink truncate">{name}</span>
          <span className="text-xs text-ink-muted tnum">
            {[kind.label, sizeBytes > 0 ? formatBytes(sizeBytes) : ''].filter(Boolean).join(' · ')}
          </span>
        </span>
      </Pressable>

      <div className="absolute right-2 top-1/2 -translate-y-1/2">
        <IconButton
          size="sm"
          onClick={(e) => {
            e.stopPropagation()
            void download({ id, name })
          }}
          disabled={isDownloading}
          aria-label={`Tải xuống ${name}`}
          title="Tải xuống"
          className="opacity-80 group-hover:opacity-100 transition-opacity"
        >
          {isDownloading ? <Spinner size="sm" /> : <Download size={16} strokeWidth={1.75} />}
        </IconButton>
      </div>

      {open && (
        <FileViewer
          files={viewerFiles}
          index={props.index ?? 0}
          origin={props.origin ?? 'chat'}
          onClose={() => setOpen(false)}
          returnFocusRef={cardRef}
        />
      )}
    </div>
  )
}
