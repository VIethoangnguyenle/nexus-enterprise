import { act, renderHook } from '@testing-library/react'
import type { MouseEvent as ReactMouseEvent } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useResizable } from './useResizable'

beforeEach(() => {
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => { cb(0); return 1 })
  vi.stubGlobal('cancelAnimationFrame', () => {})
})
afterEach(() => vi.unstubAllGlobals())

const down = (x: number) => ({ preventDefault() {}, clientX: x, clientY: x }) as ReactMouseEvent
const move = (x: number) => act(() => { document.dispatchEvent(new MouseEvent('mousemove', { clientX: x, clientY: x })) })

describe('useResizable', () => {
  const base = { direction: 'horizontal' as const, defaultSize: 280, minSize: 200, maxSize: 400 }

  it('starts from initialSize when given, else the default', () => {
    expect(renderHook(() => useResizable(base)).result.current.size).toBe(280)
    expect(renderHook(() => useResizable({ ...base, initialSize: 320 })).result.current.size).toBe(320)
  })

  it('follows the drag by the pointer delta and reports each size', () => {
    const onResize = vi.fn()
    const { result } = renderHook(() => useResizable({ ...base, onResize }))
    act(() => result.current.handleProps.onMouseDown(down(100)))
    expect(result.current.isDragging).toBe(true)
    move(150)
    expect(result.current.size).toBe(330)
    expect(onResize).toHaveBeenLastCalledWith(330)
  })

  it('clamps to min and max', () => {
    const { result } = renderHook(() => useResizable(base))
    act(() => result.current.handleProps.onMouseDown(down(100)))
    move(900)
    expect(result.current.size).toBe(400)
    move(-900)
    expect(result.current.size).toBe(200)
  })

  it('stops dragging on mouseup and releases the body cursor', () => {
    const { result } = renderHook(() => useResizable(base))
    act(() => result.current.handleProps.onMouseDown(down(100)))
    expect(document.body.style.cursor).toBe('col-resize')
    act(() => { document.dispatchEvent(new MouseEvent('mouseup')) })
    expect(result.current.isDragging).toBe(false)
    expect(document.body.style.cursor).toBe('')
  })

  it('uses the vertical axis and cursor for vertical panels', () => {
    const { result } = renderHook(() => useResizable({ ...base, direction: 'vertical' }))
    expect(result.current.handleProps.style.cursor).toBe('row-resize')
  })

  it('double click resets to the default', () => {
    const onResize = vi.fn()
    const { result } = renderHook(() => useResizable({ ...base, initialSize: 350, onResize }))
    act(() => result.current.handleProps.onDoubleClick())
    expect(result.current.size).toBe(280)
    expect(onResize).toHaveBeenCalledWith(280)
  })
})
