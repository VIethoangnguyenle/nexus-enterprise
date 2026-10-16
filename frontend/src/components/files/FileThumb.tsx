import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Play } from 'lucide-react'
import { driveApi, type ThumbnailSize } from '../../api/drive'
import { kindOf } from '../drive/drive-model'

/** `card` is a grid card or tile (320 wide), `panel` the detail panel's preview (640). */
export type ThumbSize = 'card' | 'panel'

/**
 * Which rendition to ask for. The picture arrives through an authenticated
 * fetch, so a `srcset` cannot choose between renditions; the screen's pixel
 * density does instead: cards stay at 320 on a 1x screen and take 640 on denser
 * ones, and panels always take 640.
 */
export function thumbnailSizeFor(size: ThumbSize, dpr: number): ThumbnailSize {
  return size === 'panel' || dpr > 1 ? 640 : 320
}

interface FileThumbProps {
  fileId: string
  name: string
  mimeType?: string
  /** Changes when the file's content does (`itemVersion`), so an edited file gets a fresh picture. */
  version?: string
  size?: ThumbSize
  /** CSS `aspect-ratio` of the frame. Always reserved, so the grid never jumps when pictures arrive. */
  aspect?: string
  /** Corner radius in px. */
  radius?: number
  /** Side of the file-kind icon shown when there is no picture, in px. */
  iconSize?: number
  className?: string
}

/** Becomes true once the frame is near the viewport; true at once where observing is unavailable. */
function useNearViewport(): [React.RefObject<HTMLDivElement | null>, boolean] {
  const ref = useRef<HTMLDivElement>(null)
  const [near, setNear] = useState(() => typeof IntersectionObserver === 'undefined')
  useEffect(() => {
    const el = ref.current
    if (near || !el) return
    const io = new IntersectionObserver(
      (entries) => { if (entries.some((e) => e.isIntersecting)) { setNear(true); io.disconnect() } },
      { rootMargin: '240px' },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [near])
  return [ref, near]
}

/**
 * A file's server-made picture in a fixed frame. While it loads the frame
 * shimmers (still under reduced motion); when the server has none (unsupported,
 * too large, busy, no access, offline) the frame shows the file-kind icon on
 * `--color-sunk` and says nothing about an error.
 */
export function FileThumb({
  fileId, name, mimeType, version, size = 'card', aspect = '16 / 10', radius = 8, iconSize = 34, className = '',
}: FileThumbProps) {
  const [frameRef, near] = useNearViewport()
  const px = thumbnailSizeFor(size, typeof window === 'undefined' ? 1 : window.devicePixelRatio || 1)
  const { data: blob, isError } = useQuery({
    queryKey: ['drive-thumbnail', fileId, px, version ?? ''],
    queryFn: ({ signal }) => driveApi.fetchThumbnail(fileId, px, signal),
    enabled: near,
    retry: false,
    staleTime: Infinity,
    gcTime: 5 * 60_000,
    refetchOnWindowFocus: false,
  })

  const [src, setSrc] = useState<string>()
  useEffect(() => {
    if (!blob) { setSrc(undefined); return }
    const url = URL.createObjectURL(blob)
    setSrc(url)
    return () => URL.revokeObjectURL(url)
  }, [blob])

  // A picture that fails to decode is no picture: same quiet icon.
  const [broken, setBroken] = useState<string>()
  const kind = kindOf({ item_type: 'file', name, mime_type: mimeType })
  const Icon = kind.icon
  const showImage = !!src && broken !== src
  const showFallback = isError || (!!src && broken === src)
  const isVideo = kind.label === 'Video'

  return (
    <div
      ref={frameRef}
      data-testid="file-thumb"
      className={`relative w-full overflow-hidden bg-sunk grid place-items-center ${className}`}
      style={{ aspectRatio: aspect, borderRadius: radius }}
    >
      {showImage && (
        <>
          <img
            src={src}
            alt={`Bản xem trước của ${name}`}
            loading="lazy"
            decoding="async"
            draggable={false}
            className="absolute inset-0 w-full h-full object-cover"
            onError={() => setBroken(src)}
          />
          {isVideo && (
            <span
              data-testid="file-thumb-play"
              className="relative grid place-items-center w-9 h-9 rounded-full bg-raised/90 text-ink"
            >
              <Play size={16} fill="currentColor" aria-hidden="true" />
            </span>
          )}
        </>
      )}
      {!showImage && !showFallback && <div data-testid="file-thumb-skeleton" className="absolute inset-0 skeleton" />}
      {showFallback && (
        <span data-testid="file-thumb-fallback" className="grid place-items-center w-full h-full bg-sunk text-ink-subtle">
          <Icon size={iconSize} aria-hidden="true" />
        </span>
      )}
    </div>
  )
}
