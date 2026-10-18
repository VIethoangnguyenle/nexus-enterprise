import {
  forwardRef, useCallback, useLayoutEffect, useRef, useState,
  type CSSProperties, type InputHTMLAttributes, type KeyboardEvent, type ReactNode,
} from 'react'
import { DURATION, useMotionPresets } from '../../lib/motion'
import type { SuggestionPanel } from '../../hooks/useSuggestionPanel'
import { SearchField } from '../primitives'
import { Kbd, SearchSuggestions } from './SearchSuggestions'

/** How far the surface reaches past the box on the sides and below, so the clip does not cut its shadow. */
const OPEN_CLIP = 'inset(-24px -24px -56px -24px round 12px)'
/** Space kept between the surface and the window's edges. */
const EDGE = 16
const LIST_MAX = 448
/** The surface's foot (36) and the room kept under it. */
const FOOT = 36

interface Props extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  /** Accessible name; also the placeholder unless one is given. */
  label: string
  /** The box's suggestions. Without them, or on a phone (`screen`), this is a plain `SearchField`. */
  panel?: SuggestionPanel
  /** `base` sits on a raised panel, `sunk` on the page. */
  tone?: 'base' | 'sunk'
  moduleSearch?: boolean
  /** Desktop only: sizes the box in its row (the control inside fills it): width, margins, a taller box on a tablet. */
  slotClassName?: string
  /** Desktop: 40px high (Chat) instead of 36. */
  tall?: boolean
  /** Desktop: a pill (Chat, Tài liệu) or a box with 8px corners (the rest). */
  shape?: 'pill' | 'box'
  /** Desktop: 640 wide (Chat) instead of at least 440 and as wide as the box. */
  wide?: boolean
  /** Desktop: what sits at the box's right end, and at the surface's right end while it is open (a clear button, a key hint). */
  trailing?: ReactNode
  /** Said on the desktop surface, and on a phone screen, when there is nothing to suggest. */
  emptyText?: string
  /** The label of the listbox for a screen reader. */
  listLabel?: string
}

/**
 * A module's search box (DESIGN.md "Gợi ý trong ô tìm"). On a desktop the box is one surface: at rest it is the
 * control itself, and when its suggestions open it grows out of it (`clip-path`, an exception to the transform-and-
 * opacity rule: scaling would warp the text and the corners, and animating the width would lay the page out again on
 * every frame) into a raised sheet that holds the suggestions under the box. The surface is always laid out at its open
 * size and only revealed, so the input is one element that never moves and its caret and input method are not
 * interrupted. On a phone (`screen`) it is just the field; the screen draws the suggestions itself.
 */
export const SearchBox = forwardRef<HTMLInputElement, Props>(function SearchBox(
  { panel, tone = 'base', shape = 'box', tall = false, wide = false, slotClassName = '', trailing, emptyText, listLabel, className = '', ...field },
  ref,
) {
  if (!panel || panel.variant === 'screen') {
    return <SearchField ref={ref} tone={tone} className={className} {...field} />
  }
  return (
    <Surface
      ref={ref} panel={panel} tone={tone} shape={shape} tall={tall} wide={wide} slotClassName={slotClassName}
      trailing={trailing} emptyText={emptyText} listLabel={listLabel} field={field}
    />
  )
})

type SurfaceProps = Pick<Props, 'tone' | 'shape' | 'tall' | 'wide' | 'slotClassName' | 'trailing' | 'emptyText' | 'listLabel'> & {
  panel: SuggestionPanel
  field: Omit<Props, 'panel' | 'tone' | 'shape' | 'tall' | 'wide' | 'slotClassName' | 'trailing' | 'emptyText' | 'listLabel' | 'className'>
}

const Surface = forwardRef<HTMLInputElement, SurfaceProps>(function Surface(
  { panel, tone, shape, tall, wide, slotClassName, trailing, emptyText, listLabel, field }, ref,
) {
  const m = useMotionPresets()
  const slotRef = useRef<HTMLDivElement>(null)
  const open = panel.visible
  const closing = useClosing(open, m.reduced)
  const room = useRoom(slotRef, open || closing)

  const removable = panel.groups.some((g) => g.items.some((it) => it.onRemove))
  const radius = shape === 'pill' ? 'calc(100cqh / 2)' : 'var(--radius-md, 8px)'
  const style = {
    // Custom properties carry the sizes, so what the surface does can be read back without a layout engine.
    '--sx-open-w': wide
      ? `min(640px, calc(100vw - ${room.left}px - ${EDGE}px))`
      : `min(max(100cqw, 440px), calc(100vw - ${room.left}px - ${EDGE}px))`,
    '--sx-rest-clip': `inset(4px calc(100% - 100cqw) calc(100% - 100cqh - 4px) 0 round ${radius})`,
    width: 'var(--sx-open-w)',
    clipPath: open ? OPEN_CLIP : 'var(--sx-rest-clip)',
    // Opening takes 280ms on an expo curve; folding back 210ms on ease-in. Reduced motion changes it at once.
    transition: m.reduced ? 'none' : open ? `clip-path ${DURATION.layout}s var(--ease-out-expo)` : `clip-path ${DURATION.exit}s var(--ease-in)`,
  } as CSSProperties
  const layer = (visible: boolean, ms: number, ease: string): CSSProperties => ({
    opacity: visible ? 1 : 0,
    transition: `opacity ${m.reduced ? DURATION.reducedFade : ms}s ${ease}`,
  })

  // Esc on a button of the surface (Xoá tất cả) goes back to the box; it does not close anything else.
  const onKeyDown = (e: KeyboardEvent<HTMLElement>) => {
    if (e.key === 'Escape' && e.target instanceof HTMLButtonElement) {
      e.preventDefault()
      e.stopPropagation()
      panel.anchorRef.current?.focus()
    }
  }

  return (
    // Size containment (for the `cq` units) makes the slot a stacking context of its own, so the slot, not the surface,
    // is what has to rise above what the screen draws (sticky headers, side panels) and below dialogs.
    <div ref={slotRef} className={`relative z-backdrop ${tall ? 'h-10' : 'h-9'} ${slotClassName}`} style={{ containerType: 'size' }}>
      <div
        ref={panel.surface.ref}
        onBlur={panel.surface.onBlur}
        onKeyDown={onKeyDown}
        // The surface is the control while it rests, so a press on it is a press on the box.
        onMouseDown={(e) => { if (open && !(e.target instanceof HTMLInputElement)) e.preventDefault() }}
        style={style}
        className="absolute left-0 -top-1 bg-overlay rounded-overlay"
      >
        <span aria-hidden="true" className={`absolute inset-0 rounded-[inherit] pointer-events-none ${tone === 'sunk' ? 'bg-sunk' : 'bg-base'}`} style={layer(!open, DURATION.press, 'var(--ease-out)')} />
        <span aria-hidden="true" className="absolute inset-0 rounded-[inherit] pointer-events-none shadow-overlay" style={layer(open, open ? DURATION.base : DURATION.exit, open ? 'var(--ease-out)' : 'var(--ease-in)')} />

        <div className="relative z-[1]" style={{ height: 'calc(100cqh + 8px)' }}>
          <SearchField
            ref={ref}
            bare
            {...field}
            // The wide box (Chat) is a 16px pill with its magnifier 14px in; the others keep the 14px text of their screen.
            className={`${wide ? 'pl-3.5 [&_input]:text-16' : ''} ${trailing ? 'pr-11 [&_input::-webkit-search-cancel-button]:hidden' : ''}`}
          />
          {trailing && (
            // At rest it sits at the box's right end; the open surface is wider, so it moves to the surface's.
            <div
              className="absolute top-1/2 -translate-y-1/2 flex items-center"
              style={{ right: open ? 8 : 'calc(100% - 100cqw + 8px)' }}
            >
              {trailing}
            </div>
          )}
        </div>

        {(open || closing) && (
          <div
            aria-hidden={!open || undefined}
            // The rows and the foot go first when the surface folds back.
            style={{ opacity: open ? 1 : 0, transition: open ? 'none' : `opacity ${DURATION.reducedFade}s var(--ease-in)` }}
          >
            <div className="border-t border-line overflow-y-auto" style={{ maxHeight: Math.min(LIST_MAX, room.list) }}>
              <SearchSuggestions panel={panel} emptyText={emptyText} label={listLabel} keep={closing} />
            </div>
            {panel.hasRows && (
              <div aria-hidden="true" className="flex flex-wrap items-center gap-x-3.5 min-h-9 px-4 py-2 border-t border-line text-xs text-ink-subtle">
                <span><Kbd>↑</Kbd> <Kbd>↓</Kbd> chọn</span>
                <span><Kbd>Enter</Kbd> mở</span>
                {removable && <span><Kbd>Delete</Kbd> xoá lần tìm cũ</span>}
                <span><Kbd>Esc</Kbd> đóng</span>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
})

/** True for the time the surface needs to fold back after it closed, so its rows stay drawn until then. */
function useClosing(open: boolean, reduced: boolean): boolean {
  const [closing, setClosing] = useState(false)
  const was = useRef(open)
  useLayoutEffect(() => {
    const wasOpen = was.current
    was.current = open
    if (open) {
      setClosing(false)
      return
    }
    if (!wasOpen || reduced) return
    setClosing(true)
    const id = setTimeout(() => setClosing(false), DURATION.exit * 1000)
    return () => clearTimeout(id)
  }, [open, reduced])
  return closing
}

/**
 * Where the box is in the window while its surface shows: how far from the left edge (the surface may reach no
 * further than the window's right edge) and how much height is left under it for the rows.
 */
function useRoom(slotRef: React.RefObject<HTMLDivElement | null>, active: boolean) {
  const [room, setRoom] = useState({ left: 0, list: LIST_MAX })
  const measure = useCallback(() => {
    const r = slotRef.current?.getBoundingClientRect()
    if (!r) return
    // The box's row is its height plus 8; the foot and a gap to the window's edge come after the rows.
    const list = window.innerHeight - r.top - (r.height + 8) - FOOT - EDGE
    setRoom((prev) => (prev.left === r.left && prev.list === list ? prev : { left: r.left, list }))
  }, [slotRef])
  useLayoutEffect(() => {
    if (!active) return
    measure()
    window.addEventListener('resize', measure)
    window.addEventListener('scroll', measure, true)
    return () => {
      window.removeEventListener('resize', measure)
      window.removeEventListener('scroll', measure, true)
    }
  }, [active, measure])
  return room
}
