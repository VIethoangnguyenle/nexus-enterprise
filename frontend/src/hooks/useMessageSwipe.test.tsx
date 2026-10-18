import { render, screen } from '@testing-library/react'
import { motion } from 'motion/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useMessageSwipe, MSG_SWIPE_THRESHOLD, type UseMessageSwipeReturn } from './useMessageSwipe'
import { pointer, swipe } from '../test/gesture-events'

let api: UseMessageSwipeReturn
function Swipy({ onQuote, onThread, enabled = true, reduced = false }: {
  onQuote?: () => void
  onThread?: () => void
  enabled?: boolean
  reduced?: boolean
}) {
  api = useMessageSwipe({ onQuote, onThread, enabled, reduced })
  return <motion.div data-testid="msg" style={{ x: api.x }} {...api.bind}>Tin nhắn</motion.div>
}

function setup(props: Parameters<typeof Swipy>[0] = {}) {
  const onQuote = 'onQuote' in props ? props.onQuote : vi.fn()
  const onThread = 'onThread' in props ? props.onThread : vi.fn()
  render(<Swipy {...props} onQuote={onQuote} onThread={onThread} />)
  return { onQuote, onThread, el: screen.getByTestId('msg') }
}

const right = (dx: number) => [{ x: 200, t: 0 }, { x: 215, t: 20 }, { x: 200 + dx, t: 80 }]
const left = (dx: number) => [{ x: 200, t: 0 }, { x: 185, t: 20 }, { x: 200 - dx, t: 80 }]

afterEach(() => {
  window.getSelection()?.removeAllRanges()
})

describe('useMessageSwipe', () => {
  it('quotes on a swipe right released past 64px', () => {
    const { onQuote, onThread, el } = setup()
    swipe(el, right(MSG_SWIPE_THRESHOLD + 6))
    expect(onQuote).toHaveBeenCalledTimes(1)
    expect(onThread).not.toHaveBeenCalled()
  })

  it('opens the thread on a swipe left released past 64px', () => {
    const { onQuote, onThread, el } = setup()
    swipe(el, left(MSG_SWIPE_THRESHOLD + 6))
    expect(onThread).toHaveBeenCalledTimes(1)
    expect(onQuote).not.toHaveBeenCalled()
  })

  it('does nothing when released short of 64px either way', () => {
    const { onQuote, onThread, el } = setup()
    swipe(el, right(MSG_SWIPE_THRESHOLD - 4))
    swipe(el, left(MSG_SWIPE_THRESHOLD - 4))
    expect(onQuote).not.toHaveBeenCalled()
    expect(onThread).not.toHaveBeenCalled()
  })

  it('follows the finger and reports reaching the threshold for the cue', () => {
    const { el } = setup()
    pointer(el, 'pointerdown', { x: 200, t: 0 })
    pointer(el, 'pointermove', { x: 215, t: 20 })
    pointer(el, 'pointermove', { x: 232, t: 40 })
    expect(api.x.get()).toBe(32)
    expect(api.rightProgress.get()).toBeCloseTo(0.5)
    pointer(el, 'pointermove', { x: 280, t: 60 })
    expect(api.rightProgress.get()).toBe(1)
    expect(api.leftProgress.get()).toBe(0)
  })

  it('does not move toward a direction that has no action', () => {
    const { el, onThread } = setup({ onThread: undefined })
    pointer(el, 'pointerdown', { x: 200, t: 0 })
    pointer(el, 'pointermove', { x: 185, t: 20 })
    pointer(el, 'pointermove', { x: 100, t: 40 })
    expect(api.x.get()).toBe(0)
    pointer(el, 'pointerup', { x: 100, t: 60 })
    expect(onThread).toBeUndefined()
  })

  it('leaves a vertical scroll alone, even if it later drifts sideways', () => {
    const { onQuote, onThread, el } = setup()
    swipe(el, [{ x: 200, y: 100, t: 0 }, { x: 202, y: 130, t: 20 }, { x: 300, y: 140, t: 60 }])
    expect(onQuote).not.toHaveBeenCalled()
    expect(onThread).not.toHaveBeenCalled()
  })

  it('never starts in the 20px edge zone, which is the back swipe', () => {
    const { onQuote, el } = setup()
    swipe(el, [{ x: 10, t: 0 }, { x: 30, t: 20 }, { x: 120, t: 60 }])
    expect(onQuote).not.toHaveBeenCalled()
  })

  it('quotes the same swipe when it starts 40px in', () => {
    const { onQuote, el } = setup()
    swipe(el, [{ x: 40, t: 0 }, { x: 55, t: 20 }, { x: 120, t: 60 }])
    expect(onQuote).toHaveBeenCalledTimes(1)
  })

  it('leaves a drag that starts over selected text to the selection', () => {
    const { onQuote, el } = setup()
    const range = document.createRange()
    range.selectNodeContents(el)
    window.getSelection()!.addRange(range)
    swipe(el, right(100))
    expect(onQuote).not.toHaveBeenCalled()
  })

  it('does nothing when the system cancels the touch', () => {
    const { onQuote, el } = setup()
    pointer(el, 'pointerdown', { x: 200, t: 0 })
    pointer(el, 'pointermove', { x: 215, t: 20 })
    pointer(el, 'pointermove', { x: 300, t: 40 })
    pointer(el, 'pointercancel', { x: 300, t: 60 })
    expect(onQuote).not.toHaveBeenCalled()
  })

  it('drops the gesture under way when told to (a long press took over)', () => {
    const { onQuote, el } = setup()
    pointer(el, 'pointerdown', { x: 200, t: 0 })
    api.cancel()
    pointer(el, 'pointermove', { x: 215, t: 20 })
    pointer(el, 'pointermove', { x: 300, t: 40 })
    pointer(el, 'pointerup', { x: 300, t: 60 })
    expect(onQuote).not.toHaveBeenCalled()
  })

  it('under reduced motion does not follow and acts as soon as the threshold is passed', () => {
    const { onQuote, el } = setup({ reduced: true })
    pointer(el, 'pointerdown', { x: 200, t: 0 })
    pointer(el, 'pointermove', { x: 215, t: 20 })
    expect(api.x.get()).toBe(0)
    pointer(el, 'pointermove', { x: 270, t: 40 })
    expect(onQuote).toHaveBeenCalledTimes(1)
    pointer(el, 'pointerup', { x: 280, t: 60 })
    expect(onQuote).toHaveBeenCalledTimes(1)
  })

  it('is off when disabled and for a mouse', () => {
    const { onQuote, el } = setup({ enabled: false })
    swipe(el, right(100))
    expect(onQuote).not.toHaveBeenCalled()
  })

  it('ignores a mouse', () => {
    const { onQuote, el } = setup()
    swipe(el, right(100), { pointerType: 'mouse' })
    expect(onQuote).not.toHaveBeenCalled()
  })
})
