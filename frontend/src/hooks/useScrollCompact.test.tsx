import { act, fireEvent, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useScrollCompact } from './useScrollCompact'

function screenWithScroller() {
  const scope = document.createElement('div')
  const scroller = document.createElement('div')
  scroller.className = 'overflow-y-auto'
  scroller.scrollTo = vi.fn() as never
  scope.append(scroller)
  document.body.append(scope)
  const scrollTo = (top: number) => act(() => {
    scroller.scrollTop = top
    fireEvent.scroll(scroller)
  })
  return { scope, scroller, scrollTo }
}

describe('useScrollCompact', () => {
  it('stays open until the screen is scrolled down past 24px', () => {
    const { scope, scrollTo } = screenWithScroller()
    const { result } = renderHook(() => useScrollCompact(scope, '/channels'))
    expect(result.current.compact).toBe(false)
    scrollTo(20)
    expect(result.current.compact).toBe(false)
    scrollTo(60)
    expect(result.current.compact).toBe(true)
  })

  it('opens again on a little scrolling up, and at the top', () => {
    const { scope, scrollTo } = screenWithScroller()
    const { result } = renderHook(() => useScrollCompact(scope, '/channels'))
    scrollTo(200)
    expect(result.current.compact).toBe(true)
    scrollTo(190)
    expect(result.current.compact).toBe(false)
    scrollTo(300)
    expect(result.current.compact).toBe(true)
    scrollTo(0)
    expect(result.current.compact).toBe(false)
  })

  it('never folds when disabled, as under reduced motion', () => {
    const { scope, scrollTo } = screenWithScroller()
    const { result } = renderHook(() => useScrollCompact(scope, '/channels', false))
    scrollTo(300)
    expect(result.current.compact).toBe(false)
  })

  it('starts open on another screen', () => {
    const { scope, scrollTo } = screenWithScroller()
    const { result, rerender } = renderHook(({ key }) => useScrollCompact(scope, key), { initialProps: { key: '/channels' } })
    scrollTo(300)
    expect(result.current.compact).toBe(true)
    rerender({ key: '/drive' })
    expect(result.current.compact).toBe(false)
  })

  it('ignores a sideways scroll, such as the row of chips, which moves no vertical position', () => {
    const { scope, scroller, scrollTo } = screenWithScroller()
    const chips = document.createElement('div')
    scope.append(chips)
    const { result } = renderHook(() => useScrollCompact(scope, '/channels'))
    scrollTo(300)
    act(() => { fireEvent.scroll(chips) })
    // The chips did not become the scroll root: pressing the slot still scrolls the list.
    expect(result.current.compact).toBe(true)
    expect(result.current.scrollToTop(true)).toBe(false)
    expect(scroller.scrollTo).toHaveBeenCalled()
  })

  it('ignores scrolling outside the screen it watches', () => {
    const { scope } = screenWithScroller()
    const outside = document.createElement('div')
    document.body.append(outside)
    const { result } = renderHook(() => useScrollCompact(scope, '/channels'))
    act(() => { outside.scrollTop = 300; fireEvent.scroll(outside) })
    expect(result.current.compact).toBe(false)
  })

  it('scrolls the scroll root to the top, and says so when it already is there', () => {
    const { scope, scroller, scrollTo } = screenWithScroller()
    const { result } = renderHook(() => useScrollCompact(scope, '/channels'))
    scrollTo(300)
    expect(result.current.scrollToTop(true)).toBe(false)
    expect(scroller.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' })
    scrollTo(0)
    expect(result.current.scrollToTop(true)).toBe(true)
  })

  it('finds the scroll root itself when nothing has scrolled yet', () => {
    const { scope, scroller } = screenWithScroller()
    scroller.scrollTop = 80
    const { result } = renderHook(() => useScrollCompact(scope, '/channels'))
    expect(result.current.scrollToTop(false)).toBe(false)
    expect(scroller.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'auto' })
  })
})
