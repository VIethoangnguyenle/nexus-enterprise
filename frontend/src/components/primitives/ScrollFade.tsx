import { useCallback, useLayoutEffect, useRef, useState, type HTMLAttributes } from 'react'

type Fade = 'start' | 'end' | 'both' | null

const EDGE = 28
const MASKS: Record<Exclude<Fade, null>, string> = {
  end: `linear-gradient(to right, black calc(100% - ${EDGE}px), transparent)`,
  start: `linear-gradient(to right, transparent, black ${EDGE}px)`,
  both: `linear-gradient(to right, transparent, black ${EDGE}px, black calc(100% - ${EDGE}px), transparent)`,
}

/**
 * A row that scrolls sideways and shows it: the edge with more beyond it fades
 * out, so a cut-off tab or chip reads as "keeps going" and not as a clipped one.
 * The fade is gone when everything fits.
 */
export function ScrollFade({ className = '', style, onScroll, children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  const ref = useRef<HTMLDivElement>(null)
  const [fade, setFade] = useState<Fade>(null)

  const measure = useCallback(() => {
    const el = ref.current
    if (!el) return
    const more = el.scrollWidth - el.clientWidth
    if (more <= 1) return setFade(null)
    const atStart = el.scrollLeft <= 1
    const atEnd = el.scrollLeft >= more - 1
    setFade(atStart ? 'end' : atEnd ? 'start' : 'both')
  }, [])

  useLayoutEffect(() => {
    measure()
    const ro = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(measure) : null
    if (ro && ref.current) ro.observe(ref.current)
    window.addEventListener('resize', measure)
    return () => {
      ro?.disconnect()
      window.removeEventListener('resize', measure)
    }
  }, [measure])

  return (
    <div
      ref={ref}
      data-fade={fade ?? undefined}
      onScroll={(e) => { measure(); onScroll?.(e) }}
      style={fade ? { ...style, maskImage: MASKS[fade], WebkitMaskImage: MASKS[fade] } : style}
      className={`overflow-x-auto scrollbar-none ${className}`}
      {...rest}
    >
      {children}
    </div>
  )
}
