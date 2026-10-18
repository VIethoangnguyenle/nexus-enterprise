import {
  createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, useSyncExternalStore,
  type MouseEvent, type PointerEvent, type ReactNode,
} from 'react'
import { animate, motion, useMotionValue, useTransform } from 'motion/react'
import { LOCK_THRESHOLD, useAxisLock } from '../../hooks/useAxisLock'
import { isInEdgeZone } from '../../hooks/useEdgeBack'
import { EASE, useMotionPresets } from '../../lib/motion'
import { Pressable } from '../primitives'

/** Width of one revealed button (DESIGN.md §6 "Vuốt": rộng 76, nhãn 12px dưới icon). */
export const ACTION_WIDTH = 76
/** A swipe past this share of the row's width runs the leading action, where the row allows it. */
export const FULL_SWIPE_FRACTION = 0.6
/** Released past this share of the revealed buttons' width, the row stays open. */
export const OPEN_FRACTION = 0.5
/** A flick faster than this (px/ms) opens or closes whatever the distance. */
export const FLICK_VELOCITY = 0.3
/** Release to rest: 240ms `--ease-out-expo` (DESIGN.md §6 "Quy tắc chung"). */
export const SETTLE_S = 0.24

/** Colour of a revealed button; meaning-coded, danger only for refusing or destroying (mockup §7). */
export type SwipeTone = 'accent' | 'muted' | 'warning' | 'ink' | 'success' | 'danger'
const TONE: Record<SwipeTone, string> = {
  accent: 'bg-accent',
  muted: 'bg-ink-muted',
  warning: 'bg-warning',
  ink: 'bg-ink',
  success: 'bg-success',
  danger: 'bg-danger',
}

export interface SwipeAction {
  key: string
  label: string
  icon: ReactNode
  tone: SwipeTone
  /** Wider than 76 for a longer label (Duyệt… is 92 in the mockup). */
  width?: number
  onAction: () => void
}

export type SwipeSide = 'start' | 'end'

/** What letting go does, from where the row is and how fast it moved. */
export type SwipeRelease = { kind: 'close' } | { kind: 'open'; side: SwipeSide } | { kind: 'act' }

/**
 * The decision of a release, kept pure so the thresholds are tested on their own.
 * `offset` is where the row sits (positive: moved right, the start buttons show).
 * Only the start side can act on a full swipe, and only when `fullSwipe` is set;
 * on the end side a full swipe just leaves the buttons revealed.
 */
export function releaseOf({ offset, velocity, width, startWidth, endWidth, fullSwipe }: {
  offset: number
  /** px/ms, positive to the right. */
  velocity: number
  /** The row's width. */
  width: number
  startWidth: number
  endWidth: number
  fullSwipe: boolean
}): SwipeRelease {
  if (offset > 0 && startWidth > 0) {
    if (fullSwipe && offset > width * FULL_SWIPE_FRACTION) return { kind: 'act' }
    if (velocity < -FLICK_VELOCITY) return { kind: 'close' }
    if (offset > startWidth * OPEN_FRACTION || velocity > FLICK_VELOCITY) return { kind: 'open', side: 'start' }
    return { kind: 'close' }
  }
  if (offset < 0 && endWidth > 0) {
    if (velocity > FLICK_VELOCITY) return { kind: 'close' }
    if (-offset > endWidth * OPEN_FRACTION || velocity < -FLICK_VELOCITY) return { kind: 'open', side: 'end' }
  }
  return { kind: 'close' }
}

/** Past the revealed buttons the row follows the finger at a third of its speed. */
function resist(offset: number, limit: number): number {
  if (Math.abs(offset) <= limit) return offset
  const over = Math.abs(offset) - limit
  return Math.sign(offset) * (limit + over / 3)
}

const widthOf = (actions: SwipeAction[]) => actions.reduce((sum, a) => sum + (a.width ?? ACTION_WIDTH), 0)

// ---- one open row at a time ----

interface GroupStore {
  open: () => string | null
  set: (id: string | null) => void
  subscribe: (fn: () => void) => () => void
}

function createGroupStore(): GroupStore {
  let open: string | null = null
  const subs = new Set<() => void>()
  return {
    open: () => open,
    set: (id) => {
      if (open === id) return
      open = id
      for (const fn of subs) fn()
    },
    subscribe: (fn) => {
      subs.add(fn)
      return () => subs.delete(fn)
    },
  }
}

const GroupContext = createContext<GroupStore | null>(null)

/**
 * Rows inside share one rule (mockup §7 "Một hàng mở tại một thời điểm"): opening a row closes the one that was
 * open, and scrolling anything closes it. Each row subscribes only to whether it is the open one, so opening a row
 * re-renders that row and the one it closes, never the list.
 */
export function SwipeRowGroup({ children }: { children: ReactNode }) {
  const store = useMemo(createGroupStore, [])
  useEffect(() => {
    const onScroll = () => store.set(null)
    document.addEventListener('scroll', onScroll, { capture: true, passive: true })
    return () => document.removeEventListener('scroll', onScroll, { capture: true })
  }, [store])
  return <GroupContext.Provider value={store}>{children}</GroupContext.Provider>
}

const NO_GROUP = () => () => {}

export interface SwipeRowProps {
  /** Unique within its `SwipeRowGroup`. */
  id: string
  /** Revealed by a swipe to the right, at the row's left edge (Đã đọc / Chưa đọc, Duyệt…). */
  start?: SwipeAction[]
  /** Revealed by a swipe to the left, at the row's right edge, in reading order. */
  end?: SwipeAction[]
  /**
   * A swipe to the right past 60% of the width runs the first start action at once. Only for actions that can be
   * undone (the action offers Hoàn tác itself); never for a decision.
   */
  fullSwipe?: boolean
  /** Render the row as it is, with no gesture and no extra element (the desktop). */
  disabled?: boolean
  children: ReactNode
}

/**
 * A list row whose swipes reveal buttons (DESIGN.md §6 "Vuốt", mockup §7). The row follows the finger 1:1 and
 * settles in 240ms expo; under reduced motion it does not follow, and a release past the threshold opens at once.
 * Touch and pen only: a mouse has the row's ⋯ and its long-press equivalents. A swipe that starts in the 20px edge
 * zone is never a row swipe, it is the app's back. The axis is decided after 10px, so scrolling never drags a row.
 * A tap on an open row closes it instead of opening the row. Every action here also lives in the row's long-press
 * sheet or ⋯ menu, which is what keyboard and screen-reader users reach; the revealed buttons are inert while hidden.
 */
export function SwipeRow({ id, start = [], end = [], fullSwipe = false, disabled = false, children }: SwipeRowProps) {
  if (disabled || (start.length === 0 && end.length === 0)) return <>{children}</>
  return <SwipeRowInner id={id} start={start} end={end} fullSwipe={fullSwipe}>{children}</SwipeRowInner>
}

function SwipeRowInner({ id, start, end, fullSwipe, children }: {
  id: string
  start: SwipeAction[]
  end: SwipeAction[]
  fullSwipe: boolean
  children: ReactNode
}) {
  const { reduced } = useMotionPresets()
  const group = useContext(GroupContext)
  const isOpenInGroup = useSyncExternalStore(
    group?.subscribe ?? NO_GROUP,
    () => (group ? group.open() === id : true),
    () => true,
  )
  const axis = useAxisLock()
  const x = useMotionValue(0)
  const rowRef = useRef<HTMLDivElement>(null)
  const startWidth = widthOf(start)
  const endWidth = widthOf(end)

  const [side, setSide] = useState<SwipeSide | null>(null)
  /** What a release would do right now: the buttons grow 1.15 and the full-swipe one turns accent. */
  const [armed, setArmed] = useState<SwipeRelease['kind'] | null>(null)
  const sideRef = useRef<SwipeSide | null>(null)
  const gesture = useRef<{
    pointerId: number
    startX: number
    base: number
    lastX: number
    lastT: number
    velocity: number
    locked: boolean
    /** The row's width, read once as the finger lands so a drag reads no layout. */
    width: number
  } | null>(null)
  /** The click that ends a swipe, or the tap that closes an open row, must not open the row. */
  const swallowClick = useRef(false)

  const settle = useCallback((to: number) => {
    if (reduced) x.jump(to)
    else void animate(x, to, { duration: SETTLE_S, ease: EASE.outExpo })
  }, [reduced, x])

  const setOpenSide = useCallback((next: SwipeSide | null) => {
    sideRef.current = next
    setSide(next)
    if (next) group?.set(id)
    else if (group?.open() === id) group.set(null)
  }, [group, id])

  const close = useCallback(() => {
    settle(0)
    setOpenSide(null)
  }, [settle, setOpenSide])

  // Another row opened, or the list scrolled.
  useEffect(() => {
    if (!isOpenInGroup && sideRef.current) {
      settle(0)
      sideRef.current = null
      setSide(null)
    }
  }, [isOpenInGroup, settle])

  const rowWidth = () => rowRef.current?.clientWidth || window.innerWidth || 1

  const position = (dx: number, base: number, width: number) => {
    const raw = base + dx
    if (raw > 0) return startWidth === 0 ? 0 : resist(raw, fullSwipe ? width : startWidth)
    return endWidth === 0 ? 0 : resist(raw, endWidth)
  }

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    swallowClick.current = false
    if (e.pointerType === 'mouse' || !e.isPrimary || isInEdgeZone(e.clientX)) return
    const base = sideRef.current === 'start' ? startWidth : sideRef.current === 'end' ? -endWidth : 0
    gesture.current = {
      pointerId: e.pointerId, startX: e.clientX, base, lastX: e.clientX, lastT: e.timeStamp, velocity: 0, locked: false,
      width: rowWidth(),
    }
    axis.start(e.clientX, e.clientY)
  }

  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const g = gesture.current
    if (!g || e.pointerId !== g.pointerId) return
    const locked = axis.update(e.clientX, e.clientY)
    if (locked === 'y') {
      gesture.current = null
      return
    }
    if (locked !== 'x') return
    if (!g.locked) {
      g.locked = true
      swallowClick.current = true
      try { e.currentTarget.setPointerCapture(e.pointerId) } catch { /* the pointer is already gone */ }
    }
    const dt = e.timeStamp - g.lastT
    if (dt > 0) g.velocity = (e.clientX - g.lastX) / dt
    g.lastX = e.clientX
    g.lastT = e.timeStamp
    const width = g.width
    const offset = position(e.clientX - g.startX, g.base, width)
    if (!reduced) x.set(offset)
    const next = releaseOf({ offset, velocity: 0, width, startWidth, endWidth, fullSwipe }).kind
    const shown = next === 'close' ? null : next
    if (shown !== armed) setArmed(shown)
  }

  const finish = (e: PointerEvent<HTMLDivElement>, cancelled: boolean) => {
    const g = gesture.current
    if (!g || e.pointerId !== g.pointerId) return
    gesture.current = null
    axis.reset()
    setArmed(null)
    if (!g.locked) {
      // A tap: on an open row it only closes the row.
      if (sideRef.current && !cancelled && Math.abs(e.clientX - g.startX) < LOCK_THRESHOLD) {
        swallowClick.current = true
        close()
      }
      return
    }
    if (cancelled) {
      settle(g.base)
      return
    }
    const width = g.width
    const offset = position(e.clientX - g.startX, g.base, width)
    const release = releaseOf({ offset, velocity: g.velocity, width, startWidth, endWidth, fullSwipe })
    if (release.kind === 'act') {
      close()
      start[0]?.onAction()
    } else if (release.kind === 'open') {
      settle(release.side === 'start' ? startWidth : -endWidth)
      setOpenSide(release.side)
    } else {
      close()
    }
  }

  const onClickCapture = (e: MouseEvent) => {
    if (!swallowClick.current) return
    swallowClick.current = false
    e.preventDefault()
    e.stopPropagation()
  }

  const startShown = useTransform(x, (v) => (v > 0 ? 1 : 0))
  const endShown = useTransform(x, (v) => (v < 0 ? 1 : 0))
  const run = (a: SwipeAction) => {
    close()
    a.onAction()
  }

  const panel = (actions: SwipeAction[], which: SwipeSide) => {
    const revealed = side === which
    const live = revealed || armed !== null
    return (
      <motion.div
        aria-hidden={!revealed}
        inert={!revealed}
        data-swipe-side={which}
        style={{ opacity: which === 'start' ? startShown : endShown }}
        className={`absolute inset-0 flex ${which === 'start' ? 'justify-start' : 'justify-end'}
          ${TONE[(which === 'start' ? actions[0] : actions[actions.length - 1])!.tone]}`}
      >
        {actions.map((a, i) => {
          const actArmed = which === 'start' && i === 0 && armed === 'act'
          return (
            <Pressable
              key={a.key}
              tabIndex={revealed ? 0 : -1}
              onClick={() => run(a)}
              style={{ width: a.width ?? ACTION_WIDTH }}
              className={`grid place-items-center content-center gap-1 h-full shrink-0 text-on-accent text-xs font-semibold
                leading-none ${actArmed ? 'bg-accent' : TONE[a.tone]}`}
            >
              <span
                aria-hidden="true"
                className={`grid place-items-center transition-transform duration-quick motion-reduce:transition-none
                  ${live && (armed === 'open' || actArmed) ? 'scale-[1.15]' : ''}`}
              >
                {a.icon}
              </span>
              <span>{a.label}</span>
            </Pressable>
          )
        })}
      </motion.div>
    )
  }

  return (
    <div ref={rowRef} className="relative overflow-hidden" data-swipe-row={id}>
      {start.length > 0 && panel(start, 'start')}
      {end.length > 0 && panel(end, 'end')}
      <motion.div
        style={{ x }}
        className="relative bg-base touch-pan-y"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={(e) => finish(e, false)}
        onPointerCancel={(e) => finish(e, true)}
        onClickCapture={onClickCapture}
      >
        {children}
      </motion.div>
    </div>
  )
}
