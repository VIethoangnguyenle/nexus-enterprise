import { Profiler } from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { pointer } from '../../test/gesture-events'
import { Pressable } from '../primitives'
import { SwipeRow, SwipeRowGroup, type SwipeAction } from './SwipeRow'

const ROWS = 300
/** Commits per row, from a Profiler around each: a gesture on one row must not re-render the others. */
const renders = new Map<number, number>()
const counted = (i: number) => () => renders.set(i, (renders.get(i) ?? 0) + 1)

const action = (key: string, label: string): SwipeAction => ({ key, label, icon: <span />, tone: 'warning', onAction: () => {} })
const END = [action('mute', 'Tắt thông báo'), action('star', 'Gắn sao'), action('more', 'Thêm')]
const START = [action('read', 'Đã đọc')]

function List() {
  return (
    <SwipeRowGroup>
      <ul data-testid="list" style={{ overflowY: 'auto', height: 600 }}>
        {Array.from({ length: ROWS }, (_, i) => (
          <li key={i}>
            <Profiler id={String(i)} onRender={counted(i)}>
              <SwipeRow id={String(i)} fullSwipe start={START} end={END}>
                <Pressable style={{ height: 56 }}>Hàng {i}</Pressable>
              </SwipeRow>
            </Profiler>
          </li>
        ))}
      </ul>
    </SwipeRowGroup>
  )
}

// By text, not by role: a role query walks the accessibility of all 600 buttons and is what would be measured.
const surfaceOf = (i: number) => screen.getByText(`Hàng ${i}`).parentElement!
const openPanels = () => document.querySelectorAll('[data-swipe-side][aria-hidden="false"]')

/** Every read that makes a browser lay the page out, counted while a gesture runs. */
function countLayoutReads() {
  const spies = [
    vi.spyOn(Element.prototype, 'getBoundingClientRect'),
    vi.spyOn(Element.prototype, 'getClientRects'),
    vi.spyOn(window, 'getComputedStyle'),
  ]
  const getters = (['offsetWidth', 'offsetHeight', 'offsetTop', 'offsetLeft'] as const).map((k) =>
    vi.spyOn(HTMLElement.prototype, k, 'get'))
  const clients = (['clientWidth', 'clientHeight', 'scrollTop', 'scrollHeight'] as const).map((k) =>
    vi.spyOn(Element.prototype, k, 'get'))
  return () => [...spies, ...getters, ...clients].reduce((n, s) => n + s.mock.calls.length, 0)
}

beforeEach(() => {
  renders.clear()
  Object.defineProperty(window, 'innerWidth', { value: 400, configurable: true, writable: true })
})
afterEach(() => vi.restoreAllMocks())

describe('SwipeRow in a 300-row list', () => {
  it('renders the list in budget, every row once', () => {
    const start = performance.now()
    render(<List />)
    const elapsed = performance.now() - start
    expect(renders.size).toBe(ROWS)
    expect([...renders.values()].every((n) => n === 1)).toBe(true)
    // jsdom under a parallel suite is far slower than a phone; this catches a per-row cost gone quadratic.
    expect(elapsed).toBeLessThan(3000)
  })

  it('drags a row with no layout read and no re-render of any row', () => {
    render(<List />)
    const row = surfaceOf(150)
    pointer(row, 'pointerdown', { x: 300, t: 0 })
    const reads = countLayoutReads()
    for (let k = 1; k <= 30; k++) pointer(row, 'pointermove', { x: 300 - k * 6, t: k * 16 })
    expect(reads()).toBe(0)
    // The dragged row re-renders when its buttons arm, never per move; no other row renders at all.
    expect(renders.get(150)).toBeLessThanOrEqual(3)
    expect([...renders].filter(([i, n]) => i !== 150 && n !== 1)).toEqual([])
  })

  it('opening a row and then another touches only those two rows; a scroll closes the open one alone', () => {
    render(<List />)
    const open = (i: number, t: number) => {
      const row = surfaceOf(i)
      pointer(row, 'pointerdown', { x: 300, t })
      pointer(row, 'pointermove', { x: 288, t: t + 16 })
      pointer(row, 'pointermove', { x: 160, t: t + 300 })
      pointer(row, 'pointerup', { x: 160, t: t + 310 })
    }
    open(10, 0)
    expect(openPanels()).toHaveLength(1)
    open(200, 1000)
    expect(openPanels()).toHaveLength(1)
    expect(openPanels()[0]!.closest('[data-swipe-row]')!.getAttribute('data-swipe-row')).toBe('200')
    act(() => { fireEvent.scroll(screen.getByTestId('list')) })
    expect([...renders].filter(([i, n]) => i !== 10 && i !== 200 && n !== 1)).toEqual([])
    expect(openPanels()).toHaveLength(0)
  })
})
