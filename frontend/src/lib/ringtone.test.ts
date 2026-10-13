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

  it('a quick stop and start does not let the old ring cut the new one short', () => {
    const made: FakeOscillator[] = []
    const real = FakeAudioContext.prototype.createOscillator
    FakeAudioContext.prototype.createOscillator = vi.fn(() => { const o = new FakeOscillator(); made.push(o); return o })
    try {
      const ring = createRingtone()
      ring.start()
      vi.advanceTimersByTime(1000)
      ring.stop()
      ring.start()
      const fresh = made.slice(-2)
      // The first ring's "tone off" was due at 2 s; it must have been cancelled with the ring.
      vi.advanceTimersByTime(1500)
      for (const o of fresh) expect(o.stop).not.toHaveBeenCalled()
      // The new ring's own tone ends on time.
      vi.advanceTimersByTime(600)
      for (const o of fresh) expect(o.stop).toHaveBeenCalled()
      ring.stop()
    } finally {
      FakeAudioContext.prototype.createOscillator = real
    }
  })

  it('leaves no timer behind once stopped', () => {
    const ring = createRingtone()
    ring.start()
    ring.stop()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('shares the 45 s with the incoming-call store', async () => {
    const { RING_MS } = await import('./call-model')
    expect(RING_MS).toBe(45_000)
  })
})
