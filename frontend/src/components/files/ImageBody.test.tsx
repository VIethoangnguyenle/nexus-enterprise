import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ImageBody } from './ImageBody'

describe('ImageBody pinch and zoom gestures', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('clamps scale at 1× and 4× during pinch', () => {
    const onZoom = vi.fn()
    render(<ImageBody url="https://s.test/p.png" name="p.png" zoom="fit" onZoom={onZoom} />)
    const stage = screen.getByTestId('image-stage')
    const img = screen.getByRole('img', { name: 'p.png' })

    // Start pinch with distance = 100
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 100, clientY: 200 })
    fireEvent.pointerDown(stage, { pointerId: 2, clientX: 200, clientY: 200 })

    // Pinch in below 1× (distance = 50 -> rawScale = 0.5×)
    fireEvent.pointerMove(stage, { pointerId: 2, clientX: 150, clientY: 200 })
    expect(img.style.transform).toContain('scale(1)')

    // Pinch out way above 4× (distance = 500 -> rawScale = 5×)
    fireEvent.pointerMove(stage, { pointerId: 2, clientX: 600, clientY: 200 })
    expect(img.style.transform).toContain('scale(4)')

    fireEvent.pointerUp(stage, { pointerId: 2 })
    fireEvent.pointerUp(stage, { pointerId: 1 })
    expect(onZoom).toHaveBeenCalledWith('one')
  })

  it('scales proportionally between fit (1×) and 4×, centred on pinch midpoint', () => {
    const onZoom = vi.fn()
    render(<ImageBody url="https://s.test/p.png" name="p.png" zoom="fit" onZoom={onZoom} />)
    const stage = screen.getByTestId('image-stage')
    const img = screen.getByRole('img', { name: 'p.png' })

    // Initial 2 pointers: distance = 100, midpoint = (150, 200)
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 100, clientY: 200 })
    fireEvent.pointerDown(stage, { pointerId: 2, clientX: 200, clientY: 200 })

    // Distance doubles to 200 (scale 2×)
    fireEvent.pointerMove(stage, { pointerId: 2, clientX: 300, clientY: 200 })
    expect(img.style.transform).toContain('scale(2)')

    fireEvent.pointerUp(stage, { pointerId: 2 })
    fireEvent.pointerUp(stage, { pointerId: 1 })
    expect(onZoom).toHaveBeenCalledWith('one')
  })

  it('pans with one finger only when zoomed (> 1×)', () => {
    const onZoom = vi.fn()
    const { rerender } = render(<ImageBody url="https://s.test/p.png" name="p.png" zoom="fit" onZoom={onZoom} />)
    const stage = screen.getByTestId('image-stage')
    const img = screen.getByRole('img', { name: 'p.png' })

    // At fit (1×): dragging does NOT pan
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(stage, { pointerId: 1, clientX: 150, clientY: 150 })
    fireEvent.pointerUp(stage, { pointerId: 1 })
    expect(img.style.transform).toContain('translate3d(0px, 0px, 0)')

    // Now zoom to 100% (zoom="one")
    rerender(<ImageBody url="https://s.test/p.png" name="p.png" zoom="one" onZoom={onZoom} />)
    expect(img.style.transform).toContain('scale(2)')

    // Dragging when zoomed pans the image
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(stage, { pointerId: 1, clientX: 140, clientY: 160 })
    fireEvent.pointerUp(stage, { pointerId: 1 })
    expect(img.style.transform).toContain('translate3d(40px, 60px, 0)')
  })

  it('double-tap toggles between fit (1×) and 2×', () => {
    const onZoom = vi.fn()
    render(<ImageBody url="https://s.test/p.png" name="p.png" zoom="fit" onZoom={onZoom} />)
    const stage = screen.getByTestId('image-stage')
    const img = screen.getByRole('img', { name: 'p.png' })

    expect(img.style.transform).toContain('scale(1)')

    // Double-tap 1: toggles to 2×
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerUp(stage, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerUp(stage, { pointerId: 1, clientX: 100, clientY: 100 })

    expect(img.style.transform).toContain('scale(2)')
    expect(onZoom).toHaveBeenCalledWith('one')

    // Double-tap 2: toggles back to fit (1×)
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerUp(stage, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerUp(stage, { pointerId: 1, clientX: 100, clientY: 100 })

    expect(img.style.transform).toContain('scale(1)')
    expect(onZoom).toHaveBeenCalledWith('fit')
  })

  it('horizontal swipe at fit navigates to previous / next file', () => {
    const onPrevious = vi.fn()
    const onNext = vi.fn()
    const onZoom = vi.fn()
    render(
      <ImageBody
        url="https://s.test/p.png"
        name="p.png"
        zoom="fit"
        onZoom={onZoom}
        onPrevious={onPrevious}
        onNext={onNext}
      />,
    )
    const stage = screen.getByTestId('image-stage')

    // Swipe left (next)
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 200, clientY: 100 })
    fireEvent.pointerMove(stage, { pointerId: 1, clientX: 120, clientY: 100 })
    fireEvent.pointerUp(stage, { pointerId: 1, clientX: 120, clientY: 100 })
    expect(onNext).toHaveBeenCalledTimes(1)
    expect(onPrevious).not.toHaveBeenCalled()

    // Swipe right (prev)
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(stage, { pointerId: 1, clientX: 180, clientY: 100 })
    fireEvent.pointerUp(stage, { pointerId: 1, clientX: 180, clientY: 100 })
    expect(onPrevious).toHaveBeenCalledTimes(1)
  })

  it('horizontal swipe when zoomed does NOT navigate', () => {
    const onPrevious = vi.fn()
    const onNext = vi.fn()
    const onZoom = vi.fn()
    render(
      <ImageBody
        url="https://s.test/p.png"
        name="p.png"
        zoom="one"
        onZoom={onZoom}
        onPrevious={onPrevious}
        onNext={onNext}
      />,
    )
    const stage = screen.getByTestId('image-stage')

    // Drag left when zoomed
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 200, clientY: 100 })
    fireEvent.pointerMove(stage, { pointerId: 1, clientX: 120, clientY: 100 })
    fireEvent.pointerUp(stage, { pointerId: 1, clientX: 120, clientY: 100 })
    expect(onNext).not.toHaveBeenCalled()
    expect(onPrevious).not.toHaveBeenCalled()
  })

  it('keeps every touch for its own gestures, and shows the grab cursor only when zoomed', () => {
    const onZoom = vi.fn()
    const { rerender } = render(<ImageBody url="https://s.test/p.png" name="p.png" zoom="fit" onZoom={onZoom} />)
    const stage = screen.getByTestId('image-stage')

    // At fit the first pinch must reach the handlers too: the page never zooms on its own.
    expect(stage.className).toContain('touch-none')
    expect(stage.className).not.toContain('cursor-grab')

    rerender(<ImageBody url="https://s.test/p.png" name="p.png" zoom="one" onZoom={onZoom} />)
    expect(stage.className).toContain('touch-none')
    expect(stage.className).toContain('cursor-grab')
  })

  it('disables animated transitions when prefers-reduced-motion is active', () => {
    vi.stubGlobal('matchMedia', vi.fn().mockImplementation((query: string) => ({
      matches: query.includes('prefers-reduced-motion: reduce'),
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })))

    render(<ImageBody url="https://s.test/p.png" name="p.png" zoom="fit" onZoom={vi.fn()} />)
    const img = screen.getByRole('img', { name: 'p.png' })
    expect(img.style.transition).toBe('none')
  })
})
