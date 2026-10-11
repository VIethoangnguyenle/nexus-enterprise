import { afterEach, describe, expect, it } from 'vitest'
import { trackAppHeight } from './app-height'

/** A stand-in for window.visualViewport whose height the test controls. */
function fakeViewport(height: number) {
  const listeners = new Set<() => void>()
  return {
    height,
    offsetTop: 0,
    addEventListener: (_: string, fn: () => void) => listeners.add(fn),
    removeEventListener: (_: string, fn: () => void) => listeners.delete(fn),
    resize(h: number) {
      this.height = h
      listeners.forEach((fn) => fn())
    },
    /** iOS scrolls the page to keep the focused field visible: only offsetTop changes. */
    scroll(top: number) {
      this.offsetTop = top
      listeners.forEach((fn) => fn())
    },
    listeners,
  }
}

const appHeight = () => document.documentElement.style.getPropertyValue('--app-height')
const appShift = () => document.documentElement.style.getPropertyValue('--app-shift')

afterEach(() => {
  document.documentElement.style.removeProperty('--app-height')
  document.documentElement.style.removeProperty('--app-shift')
})

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

  it('follows the visual viewport down the page when iOS scrolls it for the keyboard, so the composer is not left at the top', () => {
    const vv = fakeViewport(844)
    const stop = trackAppHeight(vv)
    // No shift while the page is not scrolled: a translate would make the shell the
    // containing block of every fixed-position descendant (tab bar, popovers).
    expect(appShift()).toBe('')
    vv.height = 508 // the keyboard opened and iOS scrolled the page by its height
    vv.scroll(336)
    expect(appHeight()).toBe('508px')
    expect(appShift()).toBe('0 336px')
    vv.scroll(0)
    expect(appShift()).toBe('')
    vv.scroll(120)
    stop()
    expect(appShift()).toBe('')
  })

  it('does nothing where the browser has no visual viewport', () => {
    const stop = trackAppHeight(null)
    expect(appHeight()).toBe('')
    stop()
  })
})
