import { afterEach, describe, expect, it } from 'vitest'
import { trackAppHeight } from './app-height'

/** A stand-in for window.visualViewport whose height the test controls. */
function fakeViewport(height: number) {
  const listeners = new Set<() => void>()
  return {
    height,
    addEventListener: (_: string, fn: () => void) => listeners.add(fn),
    removeEventListener: (_: string, fn: () => void) => listeners.delete(fn),
    resize(h: number) {
      this.height = h
      listeners.forEach((fn) => fn())
    },
    listeners,
  }
}

const appHeight = () => document.documentElement.style.getPropertyValue('--app-height')

afterEach(() => document.documentElement.style.removeProperty('--app-height'))

describe('trackAppHeight', () => {
  it('sizes the app to the visible viewport, so the on-screen keyboard never covers the composer', () => {
    const vv = fakeViewport(844)
    const stop = trackAppHeight(vv)
    expect(appHeight()).toBe('844px')
    vv.resize(500) // the keyboard opened
    expect(appHeight()).toBe('500px')
    vv.resize(844) // and closed
    expect(appHeight()).toBe('844px')
    stop()
    expect(vv.listeners.size).toBe(0)
    expect(appHeight()).toBe('')
  })

  it('does nothing where the browser has no visual viewport', () => {
    const stop = trackAppHeight(null)
    expect(appHeight()).toBe('')
    stop()
  })
})
