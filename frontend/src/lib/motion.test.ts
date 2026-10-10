import { describe, expect, it } from 'vitest'
import { presets, staggerDelay, STAGGER_ROWS, STAGGER_STEP, withDelay } from './motion'

describe('staggerDelay', () => {
  it('steps 30ms per row and stops growing after the eighth', () => {
    expect(staggerDelay(0, false)).toBe(0)
    expect(staggerDelay(3, false)).toBeCloseTo(3 * STAGGER_STEP)
    expect(staggerDelay(40, false)).toBeCloseTo(STAGGER_ROWS * STAGGER_STEP)
  })

  it('never delays under reduced motion', () => {
    expect(staggerDelay(5, true)).toBe(0)
  })
})

describe('withDelay', () => {
  it('delays the entrance and leaves the exit as it was', () => {
    const base = presets(false).row
    const delayed = withDelay(base, 0.09)
    expect((delayed.animate.transition as { delay: number }).delay).toBe(0.09)
    expect((delayed.animate.transition as { duration: number }).duration).toBe(
      (base.animate.transition as { duration: number }).duration,
    )
    expect(delayed.exit).toBe(base.exit)
  })

  it('returns the preset itself when there is nothing to add', () => {
    const base = presets(false).row
    expect(withDelay(base, 0)).toBe(base)
  })
})

describe('reduced-motion presets', () => {
  it('slide nothing: fades only, at most 120ms', () => {
    const p = presets(true)
    for (const preset of [p.panel, p.modal, p.sheet, p.popover, p.toast]) {
      expect(preset.initial).toEqual({ opacity: 0 })
      expect((preset.animate.transition as { duration: number }).duration).toBeLessThanOrEqual(0.12)
    }
    expect(p.layoutProp).toBe(false)
  })
})

describe('sheet preset', () => {
  it('rises from the bottom edge in 280ms and leaves in 210ms', () => {
    const p = presets(false).sheet
    expect(p.initial).toEqual({ y: '100%' })
    expect((p.animate.transition as { duration: number }).duration).toBe(0.28)
    expect((p.exit.transition as { duration: number }).duration).toBe(0.21)
  })
})
