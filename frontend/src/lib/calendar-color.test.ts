import { describe, expect, it } from 'vitest'
import { colorName, colorStyle, colorTokens } from './calendar-color'

describe('calendar colours', () => {
  it('maps hue index 0..7 to person tokens 1..8', () => {
    expect(colorTokens(0)).toEqual({ fg: 'var(--color-person-1)', wash: 'var(--color-person-1-wash)' })
    expect(colorTokens(7).fg).toBe('var(--color-person-8)')
  })

  it('maps -1 and anything out of range to the accent', () => {
    expect(colorTokens(-1).fg).toBe('var(--color-accent)')
    expect(colorTokens(9).wash).toBe('var(--color-accent-wash)')
    expect(colorTokens(2.5).fg).toBe('var(--color-accent)')
  })

  it('hands blocks the pair as custom properties and names the choices', () => {
    expect(colorStyle(4)).toEqual({ '--c': 'var(--color-person-5)', '--w': 'var(--color-person-5-wash)' })
    expect(colorName(4)).toBe('Lam')
    expect(colorName(-1)).toBe('Xanh ngọc')
  })
})
