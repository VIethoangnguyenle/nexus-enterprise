import { afterEach, describe, expect, it } from 'vitest'
import { preventPageZoom } from './no-page-zoom'

/** Dispatches a cancelable WebKit gesture event on target and reports whether it was cancelled. */
function gesture(type: string, target: EventTarget): boolean {
  const e = new Event(type, { bubbles: true, cancelable: true })
  target.dispatchEvent(e)
  return e.defaultPrevented
}

let stop: () => void = () => {}
afterEach(() => {
  stop()
  document.body.innerHTML = ''
})

describe('preventPageZoom', () => {
  it('cancels the pinch gestures iOS uses to zoom the page', () => {
    stop = preventPageZoom(document)
    for (const type of ['gesturestart', 'gesturechange', 'gestureend']) {
      expect(gesture(type, document.body)).toBe(true)
    }
  })

  it('leaves other events alone', () => {
    stop = preventPageZoom(document)
    expect(gesture('touchstart', document.body)).toBe(false)
  })

  it('stops cancelling once removed', () => {
    stop = preventPageZoom(document)
    stop()
    expect(gesture('gesturestart', document.body)).toBe(false)
  })
})
