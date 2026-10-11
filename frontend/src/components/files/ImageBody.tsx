import { useRef } from 'react'
import { Scan } from 'lucide-react'
import { Pressable } from '../primitives'

export type ImageZoom = 'fit' | 'one'

/**
 * An image fitted to the stage, or at 100% where it can be dragged (and
 * scrolled). Clicking the image switches between the two.
 */
export function ImageBody({ url, name, zoom, onZoom }: { url: string; name: string; zoom: ImageZoom; onZoom: (z: ImageZoom) => void }) {
  const box = useRef<HTMLDivElement>(null)
  const drag = useRef<{ x: number; y: number; left: number; top: number; moved: boolean } | null>(null)

  const onPointerDown = (e: React.PointerEvent) => {
    if (zoom !== 'one' || !box.current) return
    drag.current = { x: e.clientX, y: e.clientY, left: box.current.scrollLeft, top: box.current.scrollTop, moved: false }
    box.current.setPointerCapture?.(e.pointerId)
  }
  const onPointerMove = (e: React.PointerEvent) => {
    const d = drag.current
    if (!d || !box.current) return
    const dx = e.clientX - d.x
    const dy = e.clientY - d.y
    if (Math.abs(dx) + Math.abs(dy) > 3) d.moved = true
    box.current.scrollLeft = d.left - dx
    box.current.scrollTop = d.top - dy
  }
  const onPointerUp = () => {
    // A drag is not a click: the click that follows it must not toggle the zoom.
    const moved = drag.current?.moved
    drag.current = null
    if (moved) suppressClick.current = true
  }
  const suppressClick = useRef(false)
  const onClick = () => {
    if (suppressClick.current) {
      suppressClick.current = false
      return
    }
    onZoom(zoom === 'fit' ? 'one' : 'fit')
  }

  return (
    <div
      ref={box}
      data-testid="image-stage"
      className={`h-full overflow-auto ${zoom === 'one' ? 'cursor-grab active:cursor-grabbing touch-none' : 'grid place-items-center p-4'}`}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
    >
      <img
        src={url}
        alt={name}
        draggable={false}
        onClick={onClick}
        className={zoom === 'fit' ? 'max-w-full max-h-full object-contain cursor-zoom-in' : 'max-w-none m-auto select-none'}
      />
    </div>
  )
}

export function ImageTools({ zoom, onZoom }: { zoom: ImageZoom; onZoom: (z: ImageZoom) => void }) {
  const btn = (active: boolean) =>
    `inline-flex items-center gap-1.5 h-8 px-3 rounded-full text-small-ui ${
      active ? 'bg-hover text-ink font-semibold' : 'bg-transparent text-ink-muted hover:text-ink'
    }`
  return (
    <div
      role="toolbar"
      aria-label="Xem"
      className="absolute bottom-4 left-1/2 -translate-x-1/2 inline-flex items-center gap-1 p-1 rounded-full bg-overlay shadow-overlay"
    >
      <Pressable className={btn(zoom === 'fit')} aria-pressed={zoom === 'fit'} onClick={() => onZoom('fit')}>
        <Scan size={16} strokeWidth={1.75} aria-hidden="true" />
        Vừa khung
      </Pressable>
      <Pressable className={btn(zoom === 'one')} aria-pressed={zoom === 'one'} onClick={() => onZoom('one')}>
        100%
      </Pressable>
    </div>
  )
}
