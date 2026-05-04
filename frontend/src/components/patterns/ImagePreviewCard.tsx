import { useState, useEffect } from 'react'
import { driveApi } from '../../api/drive'
import { FilePreviewCard } from './FilePreviewCard'
import { Spinner } from '../primitives'

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

/** Inline image preview for chat messages — lazy-loads presigned URL, renders thumbnail. */
export function ImagePreviewCard({ fileId, filename }: ImagePreviewCardProps) {
  const [imageUrl, setImageUrl] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(false)

    driveApi.getDownloadUrl(fileId)
      .then(({ download_url }) => {
        if (!cancelled) setImageUrl(download_url)
      })
      .catch(() => {
        if (!cancelled) setError(true)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => { cancelled = true }
  }, [fileId])

  // Fallback to regular file card on error
  if (error) {
    return <FilePreviewCard fileId={fileId} filename={filename} />
  }

  return (
    <div
      id={`image-card-${fileId}`}
      className="inline-block mt-1 max-w-[320px] cursor-pointer group"
      onClick={() => imageUrl && window.open(imageUrl, '_blank')}
      role="button"
      tabIndex={0}
      onKeyDown={e => e.key === 'Enter' && imageUrl && window.open(imageUrl, '_blank')}
    >
      {loading ? (
        <div className="w-[200px] h-[120px] rounded bg-surface-container border border-outline-variant
          flex items-center justify-center animate-pulse">
          <Spinner size="sm" />
        </div>
      ) : (
        <img
          src={imageUrl!}
          alt={filename}
          className="rounded border border-outline-variant object-contain
            max-w-[320px] max-h-[240px] bg-surface-container
            transition-all duration-150
            group-hover:border-primary/30 group-hover:shadow-[0_0_0_2px_rgba(var(--md-primary-rgb),0.15)]"
          onError={() => setError(true)}
          loading="lazy"
        />
      )}
      <p className="text-[10px] text-on-surface-variant mt-1 m-0 truncate">{filename}</p>
    </div>
  )
}
