import { describe, expect, it } from 'vitest'
import { ALPHABET_FROM, indexLettersOf, letterOf } from './alphabet'

describe('letterOf', () => {
  it('is the first letter of the given name (the last word), as the list is sorted', () => {
    expect(letterOf('Nguyễn Văn An')).toBe('A')
    expect(letterOf('Trần Gia Bảo')).toBe('B')
    expect(letterOf('Lê Minh Châu')).toBe('C')
  })

  it('keeps Đ its own letter, after D, and files the other marked letters under their base letter', () => {
    expect(letterOf('Trần Minh Đức')).toBe('Đ')
    expect(letterOf('Phạm Ngọc Diệp')).toBe('D')
    expect(letterOf('Lê Thị Ánh')).toBe('A')
    expect(letterOf('Vũ Ơn')).toBe('O')
    expect(letterOf('Hà Ưng')).toBe('U')
    expect(letterOf('bảo')).toBe('B')
  })

  it('files a name that does not start with a letter under #', () => {
    expect(letterOf('')).toBe('#')
    expect(letterOf('Nhóm 3')).toBe('#')
  })
})

describe('indexLettersOf', () => {
  it('lists each letter once, in the order the rows come', () => {
    expect(indexLettersOf(['Nguyễn Văn An', 'Lê Thị Ánh', 'Trần Gia Bảo', 'Phạm Ngọc Diệp', 'Trần Minh Đức', 'Lê Văn Đạt']))
      .toEqual(['A', 'B', 'D', 'Đ'])
  })

  it('starts at thirty people', () => {
    expect(ALPHABET_FROM).toBe(30)
  })
})
