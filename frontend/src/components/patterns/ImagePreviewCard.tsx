import { useState } from 'react'
import { useDownloadUrl } from '../../hooks/useDownloadUrl'
import { Pressable } from '../primitives'
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
}

/** Inline image preview for chat messages: loads the presigned URL, renders a thumbnail. */
export function ImagePreviewCard({ fileId, filename }: ImagePreviewCardProps) {
  const url = useDownloadUrl(fileId)
  const [broken, setBroken] = useState(false)

  // Fall back to the plain file card when the image cannot be shown.
  if (url.isError || broken) {
    return <FilePreviewCard fileId={fileId} filename={filename} />
  }

  const open = () => url.data && window.open(url.data, '_blank', 'noopener')

  return (
    <div className="inline-block mt-1 max-w-80 group">
      {!url.data ? (
        <div className="skeleton w-50 h-30 rounded-surface" aria-busy="true" />
      ) : (
        <Pressable onClick={open} aria-label={`Mở ảnh ${filename}`} className="block rounded-surface">
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
    </div>
  )
}
