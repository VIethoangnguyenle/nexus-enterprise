/**
 * WebAudio two-tone ringtone for incoming calls. No audio files — the pattern
 * is generated from two sine oscillators at 440 Hz and 480 Hz (classic phone
 * ring). The ring cycles 2 s on / 4 s off and auto-stops after 45 s.
 *
 * Safe when AudioContext is blocked by the browser or unavailable (tests).
 */
import { RING_MS as MAX_RING_MS } from './call-model'

/** Tone on duration (seconds). */
const ON_S = 2
/** Silence gap between tones (seconds). */
const OFF_S = 4
/** Cycle period (seconds). */
const CYCLE_S = ON_S + OFF_S

export interface Ringtone {
  start: () => void
  stop: () => void
  playing: () => boolean
}

export function createRingtone(): Ringtone {
  let ctx: AudioContext | null = null
  let intervalId: ReturnType<typeof setInterval> | null = null
  let timeoutId: ReturnType<typeof setTimeout> | null = null
  let isPlaying = false

  /** Current active oscillators; cleared each cycle. */
  let oscs: OscillatorNode[] = []
  let gain: GainNode | null = null
  /** Each tone's own "switch off" timer, so stopping the ring cancels them and a new ring is not cut short by an old one. */
  const toneTimers = new Set<ReturnType<typeof setTimeout>>()

  function playTone() {
    if (!ctx) return
    try {
      const g = ctx.createGain()
      g.gain.setValueAtTime(0.3, ctx.currentTime)
      g.connect(ctx.destination)
      gain = g

      const osc1 = ctx.createOscillator()
      osc1.type = 'sine'
      osc1.frequency.setValueAtTime(440, ctx.currentTime)
      osc1.connect(g)
      osc1.start()

      const osc2 = ctx.createOscillator()
      osc2.type = 'sine'
      osc2.frequency.setValueAtTime(480, ctx.currentTime)
      osc2.connect(g)
      osc2.start()

      oscs = [osc1, osc2]

      // Fade out and stop after ON_S
      const off = setTimeout(() => {
        toneTimers.delete(off)
        for (const o of [osc1, osc2]) {
          try { o.stop(); o.disconnect() } catch { /* already stopped */ }
        }
        if (oscs[0] === osc1) oscs = []
        try { g.disconnect() } catch { /* ok */ }
      }, ON_S * 1000)
      toneTimers.add(off)
    } catch {
      // AudioContext API error — degrade gracefully.
    }
  }

  function start() {
    if (isPlaying) return
    const Ctor: typeof AudioContext | undefined =
      typeof AudioContext !== 'undefined' ? AudioContext : undefined
    if (!Ctor) return

    try {
      ctx = new Ctor()
    } catch {
      return
    }
    isPlaying = true

    // If suspended (autoplay policy), try to resume.
    if (ctx.state === 'suspended') {
      void ctx.resume().catch(() => { /* best effort */ })
    }

    // Immediately play the first tone, then repeat.
    playTone()
    intervalId = setInterval(playTone, CYCLE_S * 1000)

    // Auto-stop after MAX_RING_MS.
    timeoutId = setTimeout(stop, MAX_RING_MS)
  }

  function stop() {
    if (!isPlaying && !timeoutId && !intervalId) return
    isPlaying = false
    for (const t of toneTimers) clearTimeout(t)
    toneTimers.clear()
    if (intervalId) { clearInterval(intervalId); intervalId = null }
    if (timeoutId) { clearTimeout(timeoutId); timeoutId = null }
    for (const o of oscs) {
      try { o.stop(); o.disconnect() } catch { /* ok */ }
    }
    oscs = []
    if (gain) { try { gain.disconnect() } catch { /* ok */ }; gain = null }
    if (ctx) { void ctx.close().catch(() => { /* ok */ }); ctx = null }
  }

  function playing() { return isPlaying }

  return { start, stop, playing }
}
