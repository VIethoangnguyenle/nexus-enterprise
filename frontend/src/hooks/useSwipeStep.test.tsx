import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { touch, touchSwipe } from '../test/gesture-events'
import { WeekStrip } from '../components/calendar/PhoneCalendarHeader'

function strip(onStep = vi.fn(), onPick = vi.fn()) {
  render(<WeekStrip date={new Date(2026, 9, 14)} now={new Date(2026, 9, 14)} hasEvents={() => false} onPick={onPick} onStep={onStep} />)
  return { onStep, onPick, el: screen.getByTestId('week-strip') }
}

describe('a swipe across the week strip', () => {
  it('left is the next week, right the week before', () => {
    const { onStep, el } = strip()
    touchSwipe(el, [{ x: 300, t: 0 }, { x: 285, t: 16 }, { x: 200, t: 400 }])
    expect(onStep).toHaveBeenLastCalledWith(1)
    touchSwipe(el, [{ x: 100, t: 1000 }, { x: 115, t: 1016 }, { x: 200, t: 1400 }])
    expect(onStep).toHaveBeenLastCalledWith(-1)
    expect(onStep).toHaveBeenCalledTimes(2)
  })

  it('a short slow one does nothing', () => {
    const { onStep, el } = strip()
    touchSwipe(el, [{ x: 300, t: 0 }, { x: 288, t: 16 }, { x: 270, t: 800 }])
    expect(onStep).not.toHaveBeenCalled()
  })

  it('a vertical one is a scroll, not a step', () => {
    const { onStep, el } = strip()
    touchSwipe(el, [{ x: 300, y: 100, t: 0 }, { x: 302, y: 120, t: 16 }, { x: 150, y: 140, t: 400 }])
    expect(onStep).not.toHaveBeenCalled()
  })

  it('one that starts in the 20px edge zone is the back, never a step', () => {
    const { onStep, el } = strip()
    touchSwipe(el, [{ x: 10, t: 0 }, { x: 25, t: 16 }, { x: 200, t: 400 }])
    expect(onStep).not.toHaveBeenCalled()
  })

  it('reads the axis off the release when no move came between', () => {
    const { onStep, el } = strip()
    touch(el, 'touchstart', { x: 300, t: 0 })
    touch(el, 'touchend', { x: 200, t: 120 })
    expect(onStep).toHaveBeenCalledWith(1)
  })

  it('a tap still picks the day', () => {
    const { onStep, onPick } = strip()
    fireEvent.click(screen.getByRole('button', { name: 'Chọn ngày 16 tháng 10' }))
    expect(onPick).toHaveBeenCalledTimes(1)
    expect(onStep).not.toHaveBeenCalled()
  })
})
