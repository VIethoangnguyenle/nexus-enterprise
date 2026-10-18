import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useDoubleTap, DOUBLE_TAP_MS } from './useDoubleTap'
import { pointer } from '../test/gesture-events'

function Tap({ onDoubleTap, enabled = true }: { onDoubleTap: () => void; enabled?: boolean }) {
  return (
    <div data-testid="target" {...useDoubleTap(onDoubleTap, enabled)}>
      <a href="#x">liên kết</a>
    </div>
  )
}

/** React reads a zero `timeStamp` as missing and puts the clock in, so test time starts at 1s. */
const T0 = 1000

function tap(el: Element, x: number, t: number, init: PointerEventInit = {}, held = 40) {
  pointer(el, 'pointerdown', { x, y: 100, t: T0 + t }, init)
  pointer(el, 'pointerup', { x, y: 100, t: T0 + t + held }, init)
}

function setup(enabled = true) {
  const fn = vi.fn()
  render(<Tap onDoubleTap={fn} enabled={enabled} />)
  return { fn, el: screen.getByTestId('target') }
}

describe('useDoubleTap', () => {
  it('fires on the second of two taps within 300ms, not after a timer', () => {
    const { fn, el } = setup()
    tap(el, 100, 0)
    expect(fn).not.toHaveBeenCalled()
    tap(el, 104, 150)
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('does not fire on a single tap', () => {
    const { fn, el } = setup()
    tap(el, 100, 0)
    expect(fn).not.toHaveBeenCalled()
  })

  it('does not fire when the second tap comes later than 300ms', () => {
    const { fn, el } = setup()
    tap(el, 100, 0)
    tap(el, 100, 40 + DOUBLE_TAP_MS + 1)
    expect(fn).not.toHaveBeenCalled()
  })

  it('counts a third tap as the start of a new pair', () => {
    const { fn, el } = setup()
    tap(el, 100, 0)
    tap(el, 100, 120)
    tap(el, 100, 240)
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('is not a tap when the first touch dragged', () => {
    const { fn, el } = setup()
    pointer(el, 'pointerdown', { x: 100, y: 100, t: T0 })
    pointer(el, 'pointerup', { x: 130, y: 100, t: T0 + 50 })
    tap(el, 100, 150)
    expect(fn).not.toHaveBeenCalled()
  })

  it('is not a tap when the press was held (a long press)', () => {
    const { fn, el } = setup()
    tap(el, 100, 0, {}, 450)
    tap(el, 100, 500)
    expect(fn).not.toHaveBeenCalled()
  })

  it('needs both taps on the same spot', () => {
    const { fn, el } = setup()
    tap(el, 100, 0)
    tap(el, 200, 120)
    expect(fn).not.toHaveBeenCalled()
  })

  it('leaves taps on a control inside the element to the control', () => {
    const { fn } = setup()
    const link = screen.getByText('liên kết')
    tap(link, 100, 0)
    tap(link, 100, 120)
    expect(fn).not.toHaveBeenCalled()
  })

  it('does nothing when disabled', () => {
    const { fn, el } = setup(false)
    tap(el, 100, 0)
    tap(el, 100, 120)
    expect(fn).not.toHaveBeenCalled()
  })

  it('ignores a mouse, which has the hover bar', () => {
    const { fn, el } = setup()
    tap(el, 100, 0, { pointerType: 'mouse' })
    tap(el, 100, 120, { pointerType: 'mouse' })
    expect(fn).not.toHaveBeenCalled()
  })
})
