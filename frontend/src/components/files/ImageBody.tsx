import { useCallback, useEffect, useRef, useState } from 'react'
import { Scan } from 'lucide-react'
import { Pressable } from '../primitives'

export type ImageZoom = 'fit' | 'one'

export interface ImageBodyProps {
  url: string
  name: string
  zoom: ImageZoom
  onZoom: (z: ImageZoom) => void
  onPrevious?: () => void
  onNext?: () => void
}

/**
 * An image fitted to the stage (1×), or zoomed (up to 4×) where it can be
 * panned. Supports two-finger pinch zoom, one-finger pan when zoomed,
 * double-tap toggle between fit and 2×, and horizontal swipe navigation at fit.
 */
export function ImageBody({ url, name, zoom, onZoom, onPrevious, onNext }: ImageBodyProps) {
  const box = useRef<HTMLDivElement>(null)

  const [scale, setScale] = useState(zoom === 'one' ? 2 : 1)
  const [translate, setTranslate] = useState({ x: 0, y: 0 })
  const [isInteracting, setIsInteracting] = useState(false)

  // Motion preference
  const [reducedMotion, setReducedMotion] = useState(false)
  useEffect(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)')
    setReducedMotion(mq.matches)
    const onChange = (e: MediaQueryListEvent) => setReducedMotion(e.matches)
    mq.addEventListener?.('change', onChange)
    return () => mq.removeEventListener?.('change', onChange)
  }, [])

  // Sync with url change
  const prevUrl = useRef(url)
  useEffect(() => {
    if (url !== prevUrl.current) {
      prevUrl.current = url
      setScale(zoom === 'one' ? 2 : 1)
      setTranslate({ x: 0, y: 0 })
    }
  }, [url, zoom])

  // Sync with zoom prop
  const prevZoom = useRef(zoom)
  useEffect(() => {
    if (zoom !== prevZoom.current) {
      prevZoom.current = zoom
      if (zoom === 'fit') {
        setScale(1)
        setTranslate({ x: 0, y: 0 })
      } else if (zoom === 'one' && scale <= 1) {
        setScale(2)
        setTranslate({ x: 0, y: 0 })
      }
    }
  }, [zoom, scale])

  // Active pointers
  const pointers = useRef<Map<number, { x: number; y: number }>>(new Map())
  const initialPinch = useRef<{
    dist: number
    mid: { x: number; y: number }
    scale: number
    translate: { x: number; y: number }
  } | null>(null)

  const drag = useRef<{
    startX: number
    startY: number
    startTime: number
    startTx: number
    startTy: number
    moved: boolean
  } | null>(null)

  const lastTap = useRef<{ time: number; x: number; y: number } | null>(null)

  // Current values ref to avoid stale closures in pointer event handlers
  const stateRef = useRef({ scale, translate, zoom })
  stateRef.current = { scale, translate, zoom }

  const onPointerDown = useCallback((e: React.PointerEvent) => {
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
    box.current?.setPointerCapture?.(e.pointerId)
    setIsInteracting(true)

    const count = pointers.current.size
    if (count === 1) {
      drag.current = {
        startX: e.clientX,
        startY: e.clientY,
        startTime: Date.now(),
        startTx: stateRef.current.translate.x,
        startTy: stateRef.current.translate.y,
        moved: false,
      }
      initialPinch.current = null
    } else if (count === 2) {
      const pts = Array.from(pointers.current.values())
      const p1 = pts[0]!
      const p2 = pts[1]!
      const dist = Math.hypot(p1.x - p2.x, p1.y - p2.y)
      const mid = { x: (p1.x + p2.x) / 2, y: (p1.y + p2.y) / 2 }
      initialPinch.current = {
        dist,
        mid,
        scale: stateRef.current.scale,
        translate: { ...stateRef.current.translate },
      }
      if (drag.current) drag.current.moved = true
    }
  }, [])

  const onPointerMove = useCallback((e: React.PointerEvent) => {
    if (!pointers.current.has(e.pointerId)) return
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })

    const count = pointers.current.size
    if (count === 2 && initialPinch.current) {
      const pts = Array.from(pointers.current.values())
      const p1 = pts[0]!
      const p2 = pts[1]!
      const dist = Math.hypot(p1.x - p2.x, p1.y - p2.y)
      const currentMid = { x: (p1.x + p2.x) / 2, y: (p1.y + p2.y) / 2 }
      const { dist: d0, mid: m0, scale: s0, translate: t0 } = initialPinch.current

      if (d0 > 0) {
        const rawScale = s0 * (dist / d0)
        const nextScale = Math.min(4, Math.max(1, rawScale))

        const rect = box.current?.getBoundingClientRect()
        const cx = rect && rect.width > 0 ? rect.left + rect.width / 2 : window.innerWidth / 2
        const cy = rect && rect.height > 0 ? rect.top + rect.height / 2 : window.innerHeight / 2

        const mid0Offset = { x: m0.x - cx, y: m0.y - cy }
        const mid1Offset = { x: currentMid.x - cx, y: currentMid.y - cy }

        const nextTx = nextScale === 1 ? 0 : mid1Offset.x - (mid0Offset.x - t0.x) * (nextScale / s0)
        const nextTy = nextScale === 1 ? 0 : mid1Offset.y - (mid0Offset.y - t0.y) * (nextScale / s0)

        setScale(nextScale)
        setTranslate({ x: nextTx, y: nextTy })
      }
    } else if (count === 1 && drag.current) {
      const dx = e.clientX - drag.current.startX
      const dy = e.clientY - drag.current.startY
      if (Math.abs(dx) > 3 || Math.abs(dy) > 3) {
        drag.current.moved = true
      }
      if (stateRef.current.scale > 1) {
        // One-finger drag pans when zoomed
        setTranslate({
          x: drag.current.startTx + dx,
          y: drag.current.startTy + dy,
        })
      }
    }
  }, [])

  const onPointerUp = useCallback((e: React.PointerEvent) => {
    pointers.current.delete(e.pointerId)
    const count = pointers.current.size

    if (count < 2) {
      initialPinch.current = null
    }

    if (count === 0) {
      setIsInteracting(false)

      const curScale = stateRef.current.scale
      // Clamp / resolve final zoom state notification
      if (curScale <= 1.001) {
        setScale(1)
        setTranslate({ x: 0, y: 0 })
        if (stateRef.current.zoom !== 'fit') onZoom('fit')
      } else {
        if (stateRef.current.zoom !== 'one') onZoom('one')
      }

      const d = drag.current
      if (d) {
        const dx = e.clientX - d.startX
        const dy = e.clientY - d.startY

        // Check horizontal swipe at fit
        if (curScale <= 1.001) {
          if (Math.abs(dx) >= 40 && Math.abs(dx) > Math.abs(dy) * 1.5) {
            if (dx < 0) onNext?.()
            else onPrevious?.()
          }
        }

        // Check double tap
        if (!d.moved) {
          const now = Date.now()
          const last = lastTap.current
          if (last && now - last.time < 300 && Math.hypot(e.clientX - last.x, e.clientY - last.y) < 25) {
            lastTap.current = null
            if (curScale > 1) {
              setScale(1)
              setTranslate({ x: 0, y: 0 })
              onZoom('fit')
            } else {
              const rect = box.current?.getBoundingClientRect()
              const cx = rect && rect.width > 0 ? rect.left + rect.width / 2 : window.innerWidth / 2
              const cy = rect && rect.height > 0 ? rect.top + rect.height / 2 : window.innerHeight / 2
              const tapOffset = { x: e.clientX - cx, y: e.clientY - cy }
              setScale(2)
              setTranslate({ x: -tapOffset.x, y: -tapOffset.y })
              onZoom('one')
            }
          } else {
            lastTap.current = { time: now, x: e.clientX, y: e.clientY }
          }
        }
      }
      drag.current = null
    }
  }, [onNext, onPrevious, onZoom])

  const isZoomed = scale > 1

  return (
    <div
      ref={box}
      data-testid="image-stage"
      // touch-none always: the page itself never zooms, so the first pinch on a
      // fitted picture must reach these pointer handlers rather than the browser.
      className={`h-full w-full overflow-hidden select-none grid place-items-center touch-none ${
        isZoomed ? 'cursor-grab active:cursor-grabbing' : ''
      }`}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
    >
      <img
        src={url}
        alt={name}
        draggable={false}
        className={isZoomed ? 'max-w-full max-h-full object-contain' : 'max-w-full max-h-full object-contain cursor-zoom-in'}
        style={{
          transform: `translate3d(${translate.x}px, ${translate.y}px, 0) scale(${scale})`,
          transformOrigin: 'center center',
          transition: isInteracting || reducedMotion ? 'none' : 'transform 200ms cubic-bezier(0.16, 1, 0.3, 1)',
        }}
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
