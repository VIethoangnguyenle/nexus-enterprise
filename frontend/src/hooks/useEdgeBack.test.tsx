import { useRef } from 'react'
import { act, render, screen, waitFor } from '@testing-library/react'
import { motion } from 'motion/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { touch, touchSwipe } from '../test/gesture-events'
import {
  EDGE_ZONE, edgeBackRoot, isEdgeBackPath, isInEdgeZone, shouldGoBack, useEdgeBack,
} from './useEdgeBack'

const WIDTH = 400
const nextFrame = () => act(() => new Promise<void>((r) => requestAnimationFrame(() => requestAnimationFrame(() => r()))))

function Host({ onBack, enabled = true, reduced = false }: { onBack: () => void; enabled?: boolean; reduced?: boolean }) {
  const ref = useRef<HTMLDivElement>(null)
  const { x, phase } = useEdgeBack({ ref, enabled, reduced, onBack })
  return <motion.div ref={ref} style={{ x }} data-testid="screen" data-phase={phase}>màn chi tiết</motion.div>
}

beforeEach(() => {
  Object.defineProperty(window, 'innerWidth', { value: WIDTH, configurable: true, writable: true })
})

describe('edge-back rules', () => {
  it('goes back past 35% of the width or above 0.5 px/ms, not otherwise', () => {
    expect(shouldGoBack(141, WIDTH, 0)).toBe(true)
    expect(shouldGoBack(139, WIDTH, 0)).toBe(false)
    expect(shouldGoBack(40, WIDTH, 0.6)).toBe(true)
    expect(shouldGoBack(40, WIDTH, 0.4)).toBe(false)
  })

  it('the edge zone is the left 20px', () => {
    expect(isInEdgeZone(EDGE_ZONE)).toBe(true)
    expect(isInEdgeZone(EDGE_ZONE + 1)).toBe(false)
  })

  it('works on detail screens and settings pages, never on a list with the bottom bar', () => {
    expect(isEdgeBackPath('/channels/c1')).toBe(true)
    expect(isEdgeBackPath('/channels/starred')).toBe(true)
    expect(isEdgeBackPath('/settings/notifications')).toBe(true)
    expect(isEdgeBackPath('/calendar/event/e1')).toBe(true)
    expect(isEdgeBackPath('/channels')).toBe(false)
    expect(isEdgeBackPath('/drive')).toBe(false)
    expect(isEdgeBackPath('/approval')).toBe(false)
  })

  it('with nothing to go back to, lands on the root list of the screen’s module', () => {
    expect(edgeBackRoot('/channels/c1')).toBe('/channels')
    expect(edgeBackRoot('/calendar/event/e1')).toBe('/calendar')
    expect(edgeBackRoot('/documents/d1')).toBe('/drive')
    expect(edgeBackRoot('/settings/notifications')).toBe('/settings')
    // Already at a root: Chat.
    expect(edgeBackRoot('/settings')).toBe('/channels')
  })
})

describe('useEdgeBack', () => {
  it('a swipe from the edge past 35% goes back once the screen has slid away', async () => {
    const onBack = vi.fn()
    render(<Host onBack={onBack} />)
    const moves = touchSwipe(screen.getByTestId('screen'), [{ x: 5, t: 0 }, { x: 20, t: 16 }, { x: 200, t: 800 }])
    // Claimed from the browser once it is known to be horizontal.
    expect(moves.every((e) => e.defaultPrevented)).toBe(true)
    await waitFor(() => expect(onBack).toHaveBeenCalledTimes(1))
  })

  it('a fast flick goes back even when short', async () => {
    const onBack = vi.fn()
    render(<Host onBack={onBack} />)
    touchSwipe(screen.getByTestId('screen'), [{ x: 5, t: 0 }, { x: 20, t: 16 }, { x: 70, t: 56 }])
    await waitFor(() => expect(onBack).toHaveBeenCalledTimes(1))
  })

  it('a short slow swipe settles back and does not go back', async () => {
    const onBack = vi.fn()
    render(<Host onBack={onBack} />)
    const el = screen.getByTestId('screen')
    touchSwipe(el, [{ x: 5, t: 0 }, { x: 20, t: 16 }, { x: 60, t: 800 }])
    await waitFor(() => expect(el.dataset.phase).toBe('idle'))
    expect(onBack).not.toHaveBeenCalled()
  })

  it('a swipe that starts 40px in is not a back (it belongs to the row or message under it)', () => {
    const onBack = vi.fn()
    render(<Host onBack={onBack} reduced />)
    const moves = touchSwipe(screen.getByTestId('screen'), [{ x: 40, t: 0 }, { x: 55, t: 16 }, { x: 300, t: 800 }])
    expect(onBack).not.toHaveBeenCalled()
    expect(moves.some((e) => e.defaultPrevented)).toBe(false)
  })

  it('a vertical start is a scroll: the gesture lets it be', () => {
    const onBack = vi.fn()
    render(<Host onBack={onBack} reduced />)
    const moves = touchSwipe(screen.getByTestId('screen'), [{ x: 5, y: 100, t: 0 }, { x: 7, y: 130, t: 16 }, { x: 300, y: 140, t: 800 }])
    expect(onBack).not.toHaveBeenCalled()
    expect(moves.some((e) => e.defaultPrevented)).toBe(false)
  })

  it('a cancelled touch never goes back', () => {
    const onBack = vi.fn()
    render(<Host onBack={onBack} reduced />)
    touchSwipe(screen.getByTestId('screen'), [{ x: 5, t: 0 }, { x: 20, t: 16 }, { x: 300, t: 800 }], 'touchcancel')
    expect(onBack).not.toHaveBeenCalled()
  })

  it('does nothing while disabled (a browser tab, a list screen)', () => {
    const onBack = vi.fn()
    render(<Host onBack={onBack} enabled={false} reduced />)
    touchSwipe(screen.getByTestId('screen'), [{ x: 5, t: 0 }, { x: 20, t: 16 }, { x: 300, t: 800 }])
    expect(onBack).not.toHaveBeenCalled()
  })

  it('follows the finger 1:1 from where it landed', async () => {
    render(<Host onBack={vi.fn()} />)
    const el = screen.getByTestId('screen')
    touch(el, 'touchstart', { x: 5, t: 0 })
    touch(el, 'touchmove', { x: 20, t: 16 })
    touch(el, 'touchmove', { x: 125, t: 800 })
    await waitFor(() => expect(el.style.transform).toContain('translateX(120px)'))
  })

  it('under reduced motion the screen does not follow, and a release past the threshold goes back at once', async () => {
    const onBack = vi.fn()
    render(<Host onBack={onBack} reduced />)
    const el = screen.getByTestId('screen')
    touch(el, 'touchstart', { x: 5, t: 0 })
    touch(el, 'touchmove', { x: 20, t: 16 })
    touch(el, 'touchmove', { x: 200, t: 800 })
    await nextFrame()
    expect(el.style.transform === '' || el.style.transform === 'none').toBe(true)
    touch(el, 'touchend', { x: 200, t: 810 })
    expect(onBack).toHaveBeenCalledTimes(1)
  })
})
