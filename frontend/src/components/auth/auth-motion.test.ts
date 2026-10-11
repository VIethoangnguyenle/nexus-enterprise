import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { DURATION, EASE } from '../../lib/motion'
import { LOGO_SEEN_KEY, stepPresets, useLogoAssembly } from './auth-motion'

describe('stepPresets', () => {
  it('forward: the new step comes in 16px from the right on expo, the old leaves 12px to the left on ease-in', () => {
    const p = stepPresets(false, 1)
    expect(p.initial).toEqual({ opacity: 0, x: 16 })
    expect(p.animate).toMatchObject({ opacity: 1, x: 0, transition: { duration: DURATION.layout, ease: EASE.outExpo } })
    expect(p.exit).toMatchObject({ opacity: 0, x: -12, transition: { duration: DURATION.exit, ease: EASE.in } })
  })

  it('back: the same, mirrored', () => {
    const p = stepPresets(false, -1)
    expect(p.initial).toEqual({ opacity: 0, x: -16 })
    expect(p.exit).toMatchObject({ x: 12 })
  })

  it('reduced motion: a fade of at most 120ms and no transform at all', () => {
    const p = stepPresets(true, 1)
    expect(JSON.stringify(p)).not.toMatch(/"x"|"y"|scale/)
    expect(p.animate.transition).toMatchObject({ duration: DURATION.reducedFade })
    expect(p.exit.transition).toMatchObject({ duration: DURATION.reducedFade })
  })
})

describe('useLogoAssembly', () => {
  beforeEach(() => sessionStorage.clear())

  it('assembles the first time in a browser session and is at rest after that', () => {
    const first = renderHook(() => useLogoAssembly())
    expect(first.result.current).toBe(true)
    expect(sessionStorage.getItem(LOGO_SEEN_KEY)).toBe('1')
    first.unmount()
    const later = renderHook(() => useLogoAssembly())
    expect(later.result.current).toBe(false)
  })

  it('keeps assembling for the whole life of the screen that started it', () => {
    const { result, rerender } = renderHook(() => useLogoAssembly())
    rerender()
    expect(result.current).toBe(true)
  })

  it('is at rest, not broken, when storage is unavailable', () => {
    const spy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    const { result } = renderHook(() => useLogoAssembly())
    expect(result.current).toBe(false)
    spy.mockRestore()
  })
})
