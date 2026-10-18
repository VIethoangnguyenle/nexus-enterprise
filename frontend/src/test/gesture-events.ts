import { fireEvent } from '@testing-library/react'

/**
 * Synthetic gestures with a chosen `timeStamp`: velocity is distance over event time, and jsdom stamps events with
 * the real clock, so a test that fires two moves back to back would otherwise flick at thousands of px/ms.
 */
function stamp<E extends Event>(ev: E, t: number): E {
  Object.defineProperty(ev, 'timeStamp', { value: t })
  return ev
}

export interface PointAt {
  x: number
  y?: number
  /** Event time in ms. */
  t: number
}

const PointerCtor: typeof PointerEvent = typeof PointerEvent === 'function'
  ? PointerEvent
  : (MouseEvent as unknown as typeof PointerEvent)

/** A touch pointer event (`pointerType: 'touch'`, primary, id 1) at `x, y` and time `t`. */
export function pointer(el: Element, type: 'pointerdown' | 'pointermove' | 'pointerup' | 'pointercancel', at: PointAt, init: PointerEventInit = {}) {
  const ev = new PointerCtor(type, {
    bubbles: true, cancelable: true, clientX: at.x, clientY: at.y ?? 100, pointerId: 1, pointerType: 'touch', isPrimary: true, ...init,
  })
  // A MouseEvent stand-in has none of these; a real PointerEvent already does.
  for (const [k, v] of Object.entries({ pointerId: 1, pointerType: 'touch', isPrimary: true, ...init })) {
    if ((ev as unknown as Record<string, unknown>)[k] === undefined) Object.defineProperty(ev, k, { value: v })
  }
  fireEvent(el, stamp(ev, at.t))
}

/** Press, move through `path`, release at its last point: a whole swipe. */
export function swipe(el: Element, path: PointAt[], init: PointerEventInit = {}) {
  const [first, ...rest] = path
  pointer(el, 'pointerdown', first!, init)
  for (const p of rest) pointer(el, 'pointermove', p, init)
  pointer(el, 'pointerup', path[path.length - 1]!, init)
}

/** A one-finger touch event at `x, y` and time `t`; `touches` is empty on the end. */
export function touch(el: Element, type: 'touchstart' | 'touchmove' | 'touchend' | 'touchcancel', at: PointAt) {
  const point = { identifier: 1, clientX: at.x, clientY: at.y ?? 100, target: el }
  const ending = type === 'touchend' || type === 'touchcancel'
  const ev = new Event(type, { bubbles: true, cancelable: true })
  Object.defineProperty(ev, 'touches', { value: ending ? [] : [point] })
  Object.defineProperty(ev, 'changedTouches', { value: [point] })
  fireEvent(el, stamp(ev, at.t))
  return ev
}

/** Touch down, move through `path`, lift at its last point. Returns the move events (to check `defaultPrevented`). */
export function touchSwipe(el: Element, path: PointAt[], end: 'touchend' | 'touchcancel' = 'touchend') {
  const [first, ...rest] = path
  touch(el, 'touchstart', first!)
  const moves = rest.map((p) => touch(el, 'touchmove', p))
  touch(el, end, path[path.length - 1]!)
  return moves
}
