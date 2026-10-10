import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { useMotionPresets } from './motion'
import { switchPreferencesTo, updatePreferences } from './preferences'

afterEach(() => {
  localStorage.clear()
  switchPreferencesTo(undefined)
})

describe('useMotionPresets', () => {
  it('stays reduced when the person chose "Luôn bật", whatever the device says', () => {
    switchPreferencesTo('user-a')
    const { result } = renderHook(() => useMotionPresets())
    expect(result.current.reduced).toBe(false)

    act(() => updatePreferences({ motion: 'reduce' }))
    expect(result.current.reduced).toBe(true)
    expect(result.current.panel.initial).toEqual({ opacity: 0 })

    act(() => updatePreferences({ motion: 'system' }))
    expect(result.current.reduced).toBe(false)
  })
})
