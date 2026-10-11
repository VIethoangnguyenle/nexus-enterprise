import { useLayoutEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import { DURATION } from '../../lib/motion'

/** Where the address was typed: the field's box and the text it held. */
export interface FlightOrigin {
  rect: DOMRect
  text: string
}

/** The field's own text starts this far in from its edge (12px padding). */
const FIELD_TEXT_INSET = 12
/** Waits for the incoming step to be on its way before the address leaves the field. */
const START_DELAY_MS = 60
const OUT_EXPO = 'cubic-bezier(0.16, 1, 0.3, 1)' // --ease-out-expo

/** Can this browser fly the address? Without the Web Animations API it simply lands. */
export const canFly = () => typeof Element !== 'undefined' && typeof Element.prototype.animate === 'function'

/**
 * The email address leaving its field and landing in "Đã gửi tới …" on the code
 * step (a FLIP on the text itself): one copy, fixed to the viewport, moved with
 * transform only, 280ms on the expo curve. It is a drawing of text the page
 * already has, so it is hidden from assistive technology and takes no input.
 * `shift` is how far the incoming step still has to travel in (its slide), which
 * the measured target includes.
 */
export function AddressFlight({ from, target, shift, onDone }: {
  from: FlightOrigin
  target: React.RefObject<HTMLElement | null>
  shift: number
  onDone: () => void
}) {
  const ref = useRef<HTMLSpanElement>(null)

  useLayoutEffect(() => {
    const el = ref.current
    const dest = target.current
    if (!el || !dest || !canFly()) {
      onDone()
      return
    }
    const to = dest.getBoundingClientRect()
    const left = from.rect.left + FIELD_TEXT_INSET
    const top = from.rect.top + (from.rect.height - el.offsetHeight) / 2
    el.style.left = `${left}px`
    el.style.top = `${top}px`
    const scale = el.offsetWidth > 0 ? to.width / el.offsetWidth : 1
    const dx = to.left - shift - left
    const dy = to.top - top + (to.height - el.offsetHeight * scale) / 2
    const flight = el.animate(
      [{ transform: 'translate(0, 0) scale(1)' }, { transform: `translate(${dx}px, ${dy}px) scale(${scale})` }],
      { duration: DURATION.layout * 1000, easing: OUT_EXPO, delay: START_DELAY_MS, fill: 'both' },
    )
    let live = true
    flight.finished.then(() => live && onDone(), () => live && onDone())
    return () => {
      live = false
      flight.cancel()
    }
    // One flight per mount: the origin and target are fixed for it.
  }, [])

  return createPortal(
    <span ref={ref} className="auth-flyer" data-auth-flyer aria-hidden="true">{from.text}</span>,
    document.body,
  )
}
