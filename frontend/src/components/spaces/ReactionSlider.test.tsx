import { render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ReactionSlider, SLIDE_REACTIONS } from './ReactionSlider'
import { pointer, touch } from '../../test/gesture-events'

const START = { pointerId: 1, x: 100, y: 400 }
const ANCHOR = { top: 380, bottom: 420, left: 50, width: 200 }

/** jsdom has no layout: lay the row out as on a 390px phone, 44px per emoji from x = 20, at y 300–344. */
function layOut() {
  const row = screen.getByRole('toolbar', { name: 'Thả cảm xúc' })
  within(row).getAllByRole('button').forEach((b, i) => {
    b.getBoundingClientRect = () => new DOMRect(20 + i * 56, 300, 44, 44)
  })
}

function slider(props: Partial<Parameters<typeof ReactionSlider>[0]> = {}) {
  const handlers = { onSelect: vi.fn(), onLift: vi.fn(), onCancel: vi.fn() }
  render(<ReactionSlider start={START} anchor={ANCHOR} {...handlers} {...props} />)
  layOut()
  return handlers
}

afterEach(() => vi.restoreAllMocks())

describe('ReactionSlider: long press, then slide onto a reaction', () => {
  it('floats the row of the mockup over the message as a toolbar named for what it does', () => {
    slider()
    const row = screen.getByRole('toolbar', { name: 'Thả cảm xúc' })
    expect(within(row).getAllByRole('button').map((b) => b.textContent)).toEqual([...SLIDE_REACTIONS])
    // Above the message: 56px of row plus a 12px gap over its top.
    expect(row.style.top).toBe(`${ANCHOR.top - 56 - 12}px`)
  })

  it('goes under the message when there is no room above it', () => {
    slider({ anchor: { top: 30, bottom: 70, left: 0, width: 200 } })
    expect(screen.getByRole('toolbar', { name: 'Thả cảm xúc' }).style.top).toBe(`${70 + 12}px`)
  })

  it('raises the emoji under the finger and picks it on release', () => {
    const h = slider()
    const target = document.body
    // Slide up from the message onto ❤️ (the second slot, x 76–120).
    pointer(target, 'pointermove', { x: 98, y: 360, t: 10 })
    pointer(target, 'pointermove', { x: 98, y: 322, t: 30 })
    expect(screen.getByRole('button', { name: '❤️' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: '❤️' }).className).toContain('scale-150')
    expect(screen.getByRole('button', { name: '👍' })).toHaveAttribute('aria-pressed', 'false')
    pointer(target, 'pointerup', { x: 98, y: 322, t: 50 })
    expect(h.onSelect).toHaveBeenCalledWith('❤️')
    expect(h.onLift).not.toHaveBeenCalled()
    expect(h.onCancel).not.toHaveBeenCalled()
  })

  it('still picks a finger a little above or below the row', () => {
    const h = slider()
    pointer(document.body, 'pointermove', { x: 30, y: 290, t: 10 })
    pointer(document.body, 'pointerup', { x: 30, y: 290, t: 20 })
    expect(h.onSelect).toHaveBeenCalledWith('👍')
  })

  it('hands over to the sheet when the finger lifts before sliding', () => {
    const h = slider()
    pointer(document.body, 'pointermove', { x: 103, y: 402, t: 10 })
    pointer(document.body, 'pointerup', { x: 103, y: 402, t: 20 })
    expect(h.onLift).toHaveBeenCalledTimes(1)
    expect(h.onSelect).not.toHaveBeenCalled()
    expect(h.onCancel).not.toHaveBeenCalled()
  })

  it('cancels when the finger slid off the row before lifting', () => {
    const h = slider()
    pointer(document.body, 'pointermove', { x: 98, y: 322, t: 10 })
    pointer(document.body, 'pointermove', { x: 98, y: 600, t: 30 })
    pointer(document.body, 'pointerup', { x: 98, y: 600, t: 40 })
    expect(h.onCancel).toHaveBeenCalledTimes(1)
    expect(h.onSelect).not.toHaveBeenCalled()
    expect(h.onLift).not.toHaveBeenCalled()
  })

  it('cancels when the system takes the touch', () => {
    const h = slider()
    pointer(document.body, 'pointercancel', { x: 100, y: 400, t: 10 })
    expect(h.onCancel).toHaveBeenCalledTimes(1)
    expect(h.onSelect).not.toHaveBeenCalled()
  })

  it('follows only the finger that pressed', () => {
    const h = slider()
    pointer(document.body, 'pointerup', { x: 30, y: 320, t: 10 }, { pointerId: 2 })
    expect(h.onSelect).not.toHaveBeenCalled()
    expect(h.onLift).not.toHaveBeenCalled()
    expect(h.onCancel).not.toHaveBeenCalled()
  })

  it('holds the page still while the row is up', () => {
    slider()
    const move = touch(document.body, 'touchmove', { x: 100, y: 380, t: 10 })
    expect(move.defaultPrevented).toBe(true)
  })

  it('lets the page scroll again once it is gone', () => {
    const { unmount } = render(<ReactionSlider start={START} anchor={ANCHOR} onSelect={vi.fn()} onLift={vi.fn()} onCancel={vi.fn()} />)
    unmount()
    const move = touch(document.body, 'touchmove', { x: 100, y: 380, t: 10 })
    expect(move.defaultPrevented).toBe(false)
  })

  it('lifts the message above the scrim', () => {
    slider({ preview: <span>Chiều nay họp đối soát lúc 3h nhé</span> })
    expect(screen.getByText('Chiều nay họp đối soát lúc 3h nhé')).toBeInTheDocument()
  })
})
