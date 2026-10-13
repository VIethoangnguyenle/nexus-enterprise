import { describe, expect, it } from 'vitest'
import { barHeights, whoIsIn } from './PreJoin'

describe('whoIsIn', () => {
  it('reads like the mockup for none, one, two and many', () => {
    expect(whoIsIn([])).toBe('Chưa có ai trong cuộc gọi')
    expect(whoIsIn(['Trần Minh Đức'])).toBe('Trần Minh Đức đang trong cuộc gọi')
    expect(whoIsIn(['Trần Minh Đức', 'Lê Quang Vinh'])).toBe('Trần Minh Đức và Lê Quang Vinh đang trong cuộc gọi')
    expect(whoIsIn(['Trần Minh Đức', 'Lê Quang Vinh', 'Nguyễn Thu Lan'])).toBe('Trần Minh Đức, Lê Quang Vinh và 1 người đang trong cuộc gọi')
  })
})

describe('barHeights', () => {
  it('five bars, flat when silent and lit from the first as the voice grows', () => {
    expect(barHeights(0)).toEqual([4, 4, 4, 4, 4])
    const quiet = barHeights(0.2)
    expect(quiet[0]).toBeGreaterThan(4)
    expect(quiet[4]).toBe(4)
    expect(barHeights(1)).toEqual([18, 18, 18, 18, 18].map((h, i) => (i === 4 ? barHeights(1)[4] : h)))
    expect(Math.max(...barHeights(5))).toBe(18)
  })
})
