import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { loadPreferences, switchPreferencesTo } from '../lib/preferences'
import { useSidebarCollapsed } from './useSidebarCollapsed'

beforeEach(() => {
  localStorage.clear()
  switchPreferencesTo('user-a')
})
afterEach(() => {
  cleanup()
  switchPreferencesTo(undefined)
})

describe('useSidebarCollapsed', () => {
  it('is collapsed until the person chooses otherwise', () => {
    const { result } = renderHook(() => useSidebarCollapsed())
    expect(result.current.collapsed).toBe(true)
  })

  it('toggles, and remembers the choice for this person on this device only', () => {
    const { result } = renderHook(() => useSidebarCollapsed())
    act(() => result.current.toggle())
    expect(result.current.collapsed).toBe(false)
    expect(loadPreferences('user-a').sidebarCollapsed).toBe(false)
    expect(loadPreferences('user-b').sidebarCollapsed).toBe(true)

    act(() => switchPreferencesTo('user-b'))
    expect(result.current.collapsed).toBe(true)
    act(() => switchPreferencesTo('user-a'))
    expect(result.current.collapsed).toBe(false)
  })

  it('reads a stored choice to expand', () => {
    localStorage.setItem('ngac-prefs:user-a', JSON.stringify({ sidebarCollapsed: false }))
    act(() => switchPreferencesTo('user-a'))
    const { result } = renderHook(() => useSidebarCollapsed())
    expect(result.current.collapsed).toBe(false)
  })

  it.each([
    ['Ctrl+\\', { ctrlKey: true }],
    ['⌘\\', { metaKey: true }],
  ])('%s toggles from anywhere, even inside a text field', (_name, mods) => {
    const { result } = renderHook(() => useSidebarCollapsed({ shortcut: true }))
    const field = document.createElement('input')
    document.body.append(field)
    field.focus()
    act(() => { field.dispatchEvent(new KeyboardEvent('keydown', { key: '\\', bubbles: true, cancelable: true, ...mods })) })
    expect(result.current.collapsed).toBe(false)
    act(() => { field.dispatchEvent(new KeyboardEvent('keydown', { key: '\\', bubbles: true, cancelable: true, ...mods })) })
    expect(result.current.collapsed).toBe(true)
    field.remove()
  })

  it('ignores a bare backslash and other modifier combinations, and stops listening on unmount', () => {
    const { result, unmount } = renderHook(() => useSidebarCollapsed({ shortcut: true }))
    act(() => { document.dispatchEvent(new KeyboardEvent('keydown', { key: '\\', bubbles: true })) })
    act(() => { document.dispatchEvent(new KeyboardEvent('keydown', { key: 'b', ctrlKey: true, bubbles: true })) })
    act(() => { document.dispatchEvent(new KeyboardEvent('keydown', { key: '\\', ctrlKey: true, altKey: true, bubbles: true })) })
    expect(result.current.collapsed).toBe(true)
    unmount()
    act(() => { document.dispatchEvent(new KeyboardEvent('keydown', { key: '\\', ctrlKey: true, bubbles: true })) })
    expect(loadPreferences('user-a').sidebarCollapsed).toBe(true)
  })

  it('hands the shortcut to a function when given one, so the caller can animate it', () => {
    const run = vi.fn()
    const { result } = renderHook(() => useSidebarCollapsed({ shortcut: run }))
    act(() => { document.dispatchEvent(new KeyboardEvent('keydown', { key: '\\', ctrlKey: true, bubbles: true })) })
    expect(run).toHaveBeenCalledTimes(1)
    expect(result.current.collapsed).toBe(true)
  })

  it('prevents the browser default for the shortcut', () => {
    renderHook(() => useSidebarCollapsed({ shortcut: true }))
    const e = new KeyboardEvent('keydown', { key: '\\', ctrlKey: true, bubbles: true, cancelable: true })
    act(() => { document.dispatchEvent(e) })
    expect(e.defaultPrevented).toBe(true)
  })
})
