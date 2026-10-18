import { describe, expect, it } from 'vitest'
import { isDeviceBack, markOwnBack, markOwnStep } from './device-back'

describe('isDeviceBack', () => {
  it('is false on the first look and while the app only pushes forward', () => {
    const h = {}
    expect(isDeviceBack(h, 2)).toBe(false)
    expect(isDeviceBack(h, 3)).toBe(false)
  })

  it('is true when the history moved to a lower entry that the app did not ask for', () => {
    const h = {}
    isDeviceBack(h, 3)
    expect(isDeviceBack(h, 2)).toBe(true)
  })

  it('answers the same for every screen that asks in the same render, and keeps the answer until the next move', () => {
    const h = {}
    isDeviceBack(h, 3)
    isDeviceBack(h, 2)
    expect(isDeviceBack(h, 2)).toBe(true)
    expect(isDeviceBack(h, 3)).toBe(false)
  })

  it('is false for a back the app asked for itself, and the mark is spent after that one', () => {
    const h = {}
    isDeviceBack(h, 3)
    markOwnBack(h)
    expect(isDeviceBack(h, 2)).toBe(false)
    expect(isDeviceBack(h, 1)).toBe(true)
  })

  it('goes back to false when the app moves within the entry', () => {
    const h = {}
    isDeviceBack(h, 3)
    isDeviceBack(h, 2)
    markOwnStep(h)
    expect(isDeviceBack(h, 2)).toBe(false)
  })

  it('keeps one history apart from another', () => {
    const a = {}
    const b = {}
    isDeviceBack(a, 3)
    isDeviceBack(b, 1)
    expect(isDeviceBack(a, 2)).toBe(true)
    expect(isDeviceBack(b, 1)).toBe(false)
  })
})
