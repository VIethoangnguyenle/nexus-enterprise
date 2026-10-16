import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { useKeyboardOpen } from './useKeyboardOpen'

/** A visualViewport stand-in whose height the test sets, firing resize like the browser. */
function fakeViewport(height: number) {
  const target = new EventTarget() as EventTarget & { height: number }
  target.height = height
  Object.defineProperty(window, 'visualViewport', { configurable: true, value: target })
  return {
    resize(h: number) {
      target.height = h
      target.dispatchEvent(new Event('resize'))
    },
  }
}

function setWindow(width: number, height: number) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: width })
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: height })
}

afterEach(() => {
  Object.defineProperty(window, 'visualViewport', { configurable: true, value: undefined })
  setWindow(1024, 768)
})

describe('useKeyboardOpen', () => {
  it('is false on a phone with the full height visible, true once the keyboard takes it', () => {
    setWindow(390, 844)
    const vv = fakeViewport(844)
    const { result } = renderHook(() => useKeyboardOpen())
    expect(result.current).toBe(false)
    act(() => vv.resize(508))
    expect(result.current).toBe(true)
    act(() => vv.resize(844))
    expect(result.current).toBe(false)
  })

  it('ignores a small change such as the browser bar collapsing', () => {
    setWindow(390, 844)
    const vv = fakeViewport(844)
    const { result } = renderHook(() => useKeyboardOpen())
    act(() => vv.resize(780))
    expect(result.current).toBe(false)
  })

  it('is always false from 768 px up', () => {
    setWindow(820, 1180)
    const vv = fakeViewport(1180)
    const { result } = renderHook(() => useKeyboardOpen())
    act(() => vv.resize(700))
    expect(result.current).toBe(false)
  })

  it('is false without a visualViewport', () => {
    setWindow(390, 844)
    const { result } = renderHook(() => useKeyboardOpen())
    expect(result.current).toBe(false)
  })
})
