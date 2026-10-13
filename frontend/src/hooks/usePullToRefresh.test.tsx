import { act, fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { usePullToRefresh } from './usePullToRefresh'

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
