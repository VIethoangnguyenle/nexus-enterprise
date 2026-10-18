import { act, fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { usePullToRefresh } from './usePullToRefresh'

let reduced = false
vi.mock('../lib/motion', async () => {
  const actual = await vi.importActual<typeof import('../lib/motion')>('../lib/motion')
  return { ...actual, useMotionPresets: () => actual.presets(reduced) }
})

function Host({ onRefresh }: { onRefresh: () => Promise<unknown> }) {
  const { bind, pull, refreshing } = usePullToRefresh(onRefresh)
  return (
    <div data-testid="scroller" {...bind}>
      <span data-testid="state">{refreshing ? 'refreshing' : `pull ${Math.round(pull)}`}</span>
    </div>
  )
}

const touch = (y: number) => ({ touches: [{ clientX: 10, clientY: y }], changedTouches: [{ clientX: 10, clientY: y }] })
const drag = (el: HTMLElement, from: number, to: number) => {
  fireEvent.touchStart(el, touch(from))
  fireEvent.touchMove(el, touch(to))
}

describe('usePullToRefresh', () => {
  it('refreshes after a pull past the threshold from the top, and shows it until it is done', async () => {
    let done!: () => void
    const onRefresh = vi.fn(() => new Promise<void>((r) => { done = r }))
    render(<Host onRefresh={onRefresh} />)
    const el = screen.getByTestId('scroller')
    drag(el, 100, 300)
    expect(Number(screen.getByTestId('state').textContent!.replace('pull ', ''))).toBeGreaterThan(0)
    fireEvent.touchEnd(el, touch(300))
    expect(onRefresh).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId('state')).toHaveTextContent('refreshing')
    await act(async () => { done() })
    expect(screen.getByTestId('state')).toHaveTextContent('pull 0')
  })

  it('does nothing for a short pull', () => {
    const onRefresh = vi.fn(async () => {})
    render(<Host onRefresh={onRefresh} />)
    const el = screen.getByTestId('scroller')
    drag(el, 100, 130)
    fireEvent.touchEnd(el, touch(130))
    expect(onRefresh).not.toHaveBeenCalled()
    expect(screen.getByTestId('state')).toHaveTextContent('pull 0')
  })

  it('does not start when the list is scrolled down', () => {
    const onRefresh = vi.fn(async () => {})
    render(<Host onRefresh={onRefresh} />)
    const el = screen.getByTestId('scroller')
    Object.defineProperty(el, 'scrollTop', { value: 120, configurable: true })
    drag(el, 100, 400)
    fireEvent.touchEnd(el, touch(400))
    expect(onRefresh).not.toHaveBeenCalled()
  })

  it('ignores an upward drag, and a pull that is cancelled', () => {
    const onRefresh = vi.fn(async () => {})
    render(<Host onRefresh={onRefresh} />)
    const el = screen.getByTestId('scroller')
    drag(el, 300, 100)
    fireEvent.touchEnd(el, touch(100))
    drag(el, 100, 400)
    fireEvent.touchCancel(el, touch(400))
    expect(onRefresh).not.toHaveBeenCalled()
    expect(screen.getByTestId('state')).toHaveTextContent('pull 0')
  })

  it('does not start a second refresh while one is running, and recovers when it fails', async () => {
    let fail!: (e: Error) => void
    const onRefresh = vi.fn(() => new Promise<void>((_, rej) => { fail = rej }))
    render(<Host onRefresh={onRefresh} />)
    const el = screen.getByTestId('scroller')
    drag(el, 100, 300)
    fireEvent.touchEnd(el, touch(300))
    drag(el, 100, 300)
    fireEvent.touchEnd(el, touch(300))
    expect(onRefresh).toHaveBeenCalledTimes(1)
    await act(async () => { fail(new Error('offline')) })
    expect(screen.getByTestId('state')).toHaveTextContent('pull 0')
  })
})

describe('usePullToRefresh beside the other gestures', () => {
  const at = (x: number, y: number) => ({ touches: [{ clientX: x, clientY: y }], changedTouches: [{ clientX: x, clientY: y }] })

  it('is not a pull when the touch turns out horizontal (a row swipe or the edge back)', () => {
    const onRefresh = vi.fn(async () => {})
    render(<Host onRefresh={onRefresh} />)
    const el = screen.getByTestId('scroller')
    fireEvent.touchStart(el, at(100, 100))
    fireEvent.touchMove(el, at(160, 112))
    fireEvent.touchMove(el, at(200, 300))
    fireEvent.touchEnd(el, at(200, 300))
    expect(onRefresh).not.toHaveBeenCalled()
    expect(screen.getByTestId('state')).toHaveTextContent('pull 0')
  })

  it('does nothing while disabled', () => {
    const onRefresh = vi.fn(async () => {})
    function Off() {
      const { bind, pull } = usePullToRefresh(onRefresh, false)
      return <div data-testid="off" {...bind}>pull {pull}</div>
    }
    render(<Off />)
    const el = screen.getByTestId('off')
    fireEvent.touchStart(el, at(100, 100))
    fireEvent.touchMove(el, at(100, 400))
    fireEvent.touchEnd(el, at(100, 400))
    expect(onRefresh).not.toHaveBeenCalled()
  })
})

describe('usePullToRefresh under reduced motion', () => {
  it('does not follow the finger: the indicator shows only once the pull has passed the threshold', async () => {
    reduced = true
    try {
      const onRefresh = vi.fn(async () => {})
      render(<Host onRefresh={onRefresh} />)
      const el = screen.getByTestId('scroller')
      drag(el, 100, 160)
      expect(screen.getByTestId('state')).toHaveTextContent('pull 0')
      fireEvent.touchMove(el, touch(300))
      expect(screen.getByTestId('state')).toHaveTextContent('pull 56')
      await act(async () => { fireEvent.touchEnd(el, touch(300)) })
      expect(onRefresh).toHaveBeenCalledTimes(1)
    } finally {
      reduced = false
    }
  })
})
