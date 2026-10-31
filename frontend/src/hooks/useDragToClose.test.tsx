import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { touchSwipe } from '../test/gesture-events'
import { Textarea } from '../components/primitives'
import { NavSheet, SheetItem } from '../components/spaces/NavSheet'
import { CLOSE_FRACTION, shouldClose } from './useDragToClose'

const HEIGHT = 800

function Sheet({ onClose }: { onClose: () => void }) {
  return (
    <NavSheet label="Tuỳ chọn" onClose={onClose}>
      <SheetItem onClick={() => {}}>Gắn sao</SheetItem>
      <Textarea aria-label="Tên mục" />
    </NavSheet>
  )
}

const down = (y0: number, y1: number, t1 = 800) => [{ x: 200, y: y0, t: 0 }, { x: 201, y: y0 + 14, t: 16 }, { x: 202, y: y1, t: t1 }]

beforeEach(() => {
  Object.defineProperty(window, 'innerHeight', { value: HEIGHT, configurable: true, writable: true })
})

describe('shouldClose', () => {
  it('closes past 25% of the height or on a flick faster than 0.5 px/ms', () => {
    expect(shouldClose(HEIGHT * CLOSE_FRACTION + 1, HEIGHT, 0)).toBe(true)
    expect(shouldClose(HEIGHT * CLOSE_FRACTION - 1, HEIGHT, 0)).toBe(false)
    expect(shouldClose(40, HEIGHT, 0.6)).toBe(true)
  })
})

describe('a sheet dragged down', () => {
  it('closes from anywhere on it, not only the handle', () => {
    const onClose = vi.fn()
    render(<Sheet onClose={onClose} />)
    const moves = touchSwipe(screen.getByText('Gắn sao'), down(100, 400))
    expect(moves.every((e) => e.defaultPrevented)).toBe(true)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('settles back when the drag is short and slow', () => {
    const onClose = vi.fn()
    render(<Sheet onClose={onClose} />)
    touchSwipe(screen.getByText('Gắn sao'), down(100, 180))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('a list scrolled down scrolls back first: the drag does not close', () => {
    const onClose = vi.fn()
    render(<Sheet onClose={onClose} />)
    const item = screen.getByText('Gắn sao')
    const list = item.closest('.overflow-y-auto') as HTMLElement
    Object.defineProperty(list, 'scrollTop', { value: 60, configurable: true })
    const moves = touchSwipe(item, down(100, 500))
    expect(onClose).not.toHaveBeenCalled()
    expect(moves.some((e) => e.defaultPrevented)).toBe(false)
  })

  it('an upward or sideways drag is not a close', () => {
    const onClose = vi.fn()
    render(<Sheet onClose={onClose} />)
    const item = screen.getByText('Gắn sao')
    touchSwipe(item, down(500, 100))
    touchSwipe(item, [{ x: 100, y: 100, t: 0 }, { x: 130, y: 104, t: 16 }, { x: 300, y: 450, t: 800 }])
    expect(onClose).not.toHaveBeenCalled()
  })

  it('leaves a text field its own touches', () => {
    const onClose = vi.fn()
    render(<Sheet onClose={onClose} />)
    touchSwipe(screen.getByLabelText('Tên mục'), down(100, 500))
    expect(onClose).not.toHaveBeenCalled()
  })
})
