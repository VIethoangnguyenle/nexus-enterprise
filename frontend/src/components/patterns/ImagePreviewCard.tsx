import { useMemo, useRef, useState } from 'react'
import { useDownloadUrl } from '../../hooks/useDownloadUrl'
import { Pressable } from '../primitives'
import { FileViewer, type ViewerFile } from '../files/FileViewer'
import { FilePreviewCard } from './FilePreviewCard'

const IMAGE_EXTENSIONS = new Set([
  'jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico', 'avif',
])

/** Returns true if the filename has a recognized image extension. */
export function isImageFile(filename: string): boolean {
  const ext = filename.split('.').pop()?.toLowerCase() || ''
  return IMAGE_EXTENSIONS.has(ext)
}

interface ImagePreviewCardProps {
  fileId: string
  filename: string
  files?: ViewerFile[]
  index?: number
  origin?: 'chat' | 'drive'
}

/** Inline image preview for chat messages: loads the presigned URL, renders a thumbnail, opens FileViewer on click. */
export function ImagePreviewCard({ fileId, filename, files, index, origin = 'chat' }: ImagePreviewCardProps) {
  const url = useDownloadUrl(fileId)
  const [broken, setBroken] = useState(false)
  const [open, setOpen] = useState(false)
  const triggerRef = useRef<HTMLButtonElement>(null)

  // Fall back to the plain file card when the image cannot be shown.
  if (url.isError || broken) {
    return <FilePreviewCard fileId={fileId} filename={filename} files={files} index={index} origin={origin} />
  }

  const viewerFiles = useMemo<ViewerFile[]>(() => {
    if (files && files.length > 0) return files
    const ext = filename.split('.').pop() || 'jpeg'
    return [{ id: fileId, name: filename, mime: `image/${ext}` }]
  }, [files, fileId, filename])

  return (
    <div className="inline-block mt-1 max-w-80 group">
      {!url.data ? (
        <div className="skeleton w-50 h-30 rounded-surface" aria-busy="true" />
      ) : (
        <Pressable
          ref={triggerRef}
          onClick={() => setOpen(true)}
          aria-label={`Mở ảnh ${filename}`}
          className="block rounded-surface"
        >
          <img
            src={url.data}
            alt={filename}
            className="rounded-surface object-contain max-w-80 max-h-60 bg-sunk
              transition-opacity duration-quick group-hover:opacity-90"
            onError={() => setBroken(true)}
            loading="lazy"
          />
        </Pressable>
      )}
      <p className="m-0 mt-1 text-xs text-ink-muted truncate">{filename}</p>

      {open && (
        <FileViewer
          files={viewerFiles}
          index={index ?? 0}
          origin={origin}
          onClose={() => setOpen(false)}
          returnFocusRef={triggerRef}
        />
      )}
    </div>
  )
}
