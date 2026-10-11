import { act, render } from '@testing-library/react'
import { useRef, useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Pressable } from '../components/primitives'
import { FLASH_MS, useFocusMessage } from './useFocusMessage'

function Harness({ target, scope, loadMore }: {
  target?: string
  scope: 'stream' | 'panel'
  loadMore?: () => boolean
}) {
  const ref = useRef<HTMLDivElement>(null)
  useFocusMessage(ref, target, scope, loadMore)
  return (
    <div ref={ref} data-testid="root">
      <div role="log"><article data-message-id="m1" /><article data-message-id="m2" /></div>
      <aside>
        <div className="group"><span id="thread-root-t1-text" /></div>
        <div className="group" data-testid="reply"><span id="reply-r1-text" /></div>
      </aside>
    </div>
  )
}

const scrolled: Element[] = []
beforeEach(() => {
  vi.useFakeTimers()
  scrolled.length = 0
  Element.prototype.scrollIntoView = function (this: Element) { scrolled.push(this) }
})
afterEach(() => {
  vi.useRealTimers()
  delete (Element.prototype as { scrollIntoView?: unknown }).scrollIntoView
})

const flashing = (root: HTMLElement) => [...root.querySelectorAll('[data-flash]')]

describe('useFocusMessage', () => {
  it('centres a stream message and flashes it for 2.4 s', () => {
    const { getByTestId } = render(<Harness target="m2" scope="stream" />)
    const root = getByTestId('root')
    expect(scrolled[0]).toBe(root.querySelector('[data-message-id="m2"]'))
    expect(flashing(root)).toEqual([root.querySelector('[data-message-id="m2"]')])
    act(() => { vi.advanceTimersByTime(FLASH_MS + 1) })
    expect(flashing(root)).toHaveLength(0)
  })

  it('in the panel it flashes a reply and the thread root, not the stream copy', () => {
    const { getByTestId, rerender } = render(<Harness target="r1" scope="panel" />)
    expect(flashing(getByTestId('root'))).toEqual([getByTestId('reply')])
    rerender(<Harness target="t1" scope="panel" />)
    expect(flashing(getByTestId('root'))[0]?.querySelector('#thread-root-t1-text')).not.toBeNull()
  })

  it('does not look in the stream when the scope is the panel, and the other way round', () => {
    const a = render(<Harness target="m1" scope="panel" />)
    expect(flashing(a.container)).toHaveLength(0)
    a.unmount()
    const b = render(<Harness target="r1" scope="stream" />)
    expect(flashing(b.container)).toHaveLength(0)
  })

  it('waits for a message that is not on screen yet and flashes it when it appears', async () => {
    function Late() {
      const ref = useRef<HTMLDivElement>(null)
      const [shown, setShown] = useState(false)
      useFocusMessage(ref, 'late', 'stream')
      return (
        <div ref={ref} data-testid="root">
          <Pressable onClick={() => setShown(true)}>show</Pressable>
          <div role="log">{shown && <article data-message-id="late" />}</div>
        </div>
      )
    }
    const { getByTestId, getByText } = render(<Late />)
    expect(flashing(getByTestId('root'))).toHaveLength(0)
    await act(async () => { getByText('show').click() })
    expect(flashing(getByTestId('root'))).toHaveLength(1)
  })

  it('asks for older messages while the target is missing, and stops when there are none', () => {
    const loadMore = vi.fn(() => false)
    render(<Harness target="gone" scope="stream" loadMore={loadMore} />)
    expect(loadMore).toHaveBeenCalledTimes(1)
  })

  it('does nothing without a target, and never flashes under a selector-breaking id', () => {
    const a = render(<Harness scope="stream" />)
    expect(flashing(a.container)).toHaveLength(0)
    a.unmount()
    const bad = render(<Harness target={'"] x[y="'} scope="stream" />)
    expect(flashing(bad.container)).toHaveLength(0)
  })

  it('keeps the flash still under reduced motion: no smooth scrolling', () => {
    const matches = vi.fn().mockReturnValue({ matches: true, addEventListener() {}, removeEventListener() {} })
    vi.stubGlobal('matchMedia', matches)
    const spy = vi.fn()
    Element.prototype.scrollIntoView = spy
    render(<Harness target="m1" scope="stream" />)
    expect(spy).toHaveBeenCalledWith({ block: 'center', behavior: 'auto' })
    vi.unstubAllGlobals()
  })
})
