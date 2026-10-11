import { describe, expect, it } from 'vitest'
import { levelOptions, optionFor } from './notification-level-copy'

describe('levelOptions', () => {
  it('offers a space three levels with the default in the middle', () => {
    const o = levelOptions('space')
    expect(o.map((x) => x.level)).toEqual(['all', 'main', 'off'])
    expect(o.filter((x) => x.isDefault).map((x) => x.level)).toEqual(['main'])
  })

  it('offers a direct message Bật and Tắt only: never "all"', () => {
    const o = levelOptions('dm')
    expect(o.map((x) => [x.level, x.title])).toEqual([['main', 'Bật'], ['off', 'Tắt']])
    expect(o.some((x) => x.level === 'all')).toBe(false)
  })
})

describe('optionFor', () => {
  it('names the level of the conversation', () => {
    expect(optionFor('space', 'all').title).toBe('Tất cả')
    expect(optionFor('space', 'main').title).toBe('Tin chính và thread đang theo dõi')
    expect(optionFor('dm', 'off').title).toBe('Tắt')
  })

  it('reads a stale "all" on a direct message as Bật', () => {
    expect(optionFor('dm', 'all').title).toBe('Bật')
  })
})
