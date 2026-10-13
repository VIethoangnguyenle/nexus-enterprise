import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { createRingtone } from './ringtone'

class FakeOscillator {
  type = 'sine' as OscillatorType
  frequency = { setValueAtTime: vi.fn() }
  connect = vi.fn().mockReturnThis()
  start = vi.fn()
  stop = vi.fn()
  disconnect = vi.fn()
}

class FakeGain {
  gain = { setValueAtTime: vi.fn(), linearRampToValueAtTime: vi.fn() }
  connect = vi.fn().mockReturnThis()
  disconnect = vi.fn()
}

class FakeAudioContext {
  state = 'running'
  currentTime = 0
  destination = {}
  createOscillator = vi.fn(() => new FakeOscillator())
  createGain = vi.fn(() => new FakeGain())
  resume = vi.fn().mockResolvedValue(undefined)
  close = vi.fn().mockResolvedValue(undefined)
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.stubGlobal('AudioContext', FakeAudioContext)
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('ringtone', () => {
  it('start creates oscillators and stop cleans up', () => {
    const ring = createRingtone()
    ring.start()
    expect(FakeAudioContext.prototype.createOscillator).toHaveBeenCalled
    ring.stop()
  })

  it('start is safe when called twice', () => {
    const ring = createRingtone()
    ring.start()
    ring.start() // should not throw
    ring.stop()
  })

  it('stop is safe when called before start', () => {
    const ring = createRingtone()
    ring.stop() // should not throw
  })

  it('auto-stops after 45 seconds', () => {
    const ring = createRingtone()
    ring.start()
    expect(ring.playing()).toBe(true)
    vi.advanceTimersByTime(45_000)
    expect(ring.playing()).toBe(false)
  })

  it('survives when AudioContext is unavailable', () => {
    vi.stubGlobal('AudioContext', undefined)
    const ring = createRingtone()
    ring.start() // should not throw
    expect(ring.playing()).toBe(false)
    ring.stop()
  })

  it('survives when AudioContext constructor throws', () => {
    vi.stubGlobal('AudioContext', class { constructor() { throw new Error('blocked') } })
    const ring = createRingtone()
    ring.start() // should not throw
    expect(ring.playing()).toBe(false)
  })
})
