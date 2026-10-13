import { describe, expect, it } from 'vitest'
import { fold, highlight, matchesAll, searchTerms, snippet } from './search-text'

describe('fold', () => {
  it('drops accents, đ and case', () => {
    expect(fold('Đối soát GIAO dịch')).toBe('doi soat giao dich')
    expect(fold('Nguyễn Thị Hồng')).toBe('nguyen thi hong')
  })

  it('keeps the spaces it is given', () => {
    expect(fold(' a  b ')).toBe(' a  b ')
  })
})

describe('matchesAll', () => {
  it('finds a group typed without accents or capitals', () => {
    expect(matchesAll('Đối soát giao dịch', 'doi soat')).toBe(true)
    expect(matchesAll('Đối soát giao dịch', 'ĐỐI SOÁT')).toBe(true)
  })

  it('wants every word, in any order, and any part of a word', () => {
    expect(matchesAll('Đối soát giao dịch', 'dich soat')).toBe(true)
    expect(matchesAll('Đối soát giao dịch', 'soat thue')).toBe(false)
    expect(matchesAll('Đối soát giao dịch', 'gia')).toBe(true)
  })

  it('matches nothing for a query with no words', () => {
    expect(matchesAll('Đối soát', '   ')).toBe(false)
  })
})

describe('searchTerms', () => {
  it('splits on spaces and drops empty parts', () => {
    expect(searchTerms('  Đối   soát ')).toEqual(['doi', 'soat'])
  })
})

describe('highlight', () => {
  const joined = (parts: ReturnType<typeof highlight>) => parts.map((p) => (p.hit ? `[${p.text}]` : p.text)).join('')

  it('marks the words as written in the text, whatever the accents and case of the query', () => {
    expect(joined(highlight('Bảng Đối soát tháng 10', 'doi soat'))).toBe('Bảng [Đối soát] tháng 10')
    expect(joined(highlight('Bảng đối soát', 'ĐỐI'))).toBe('Bảng [đối] soát')
  })

  it('marks every occurrence and merges touching ones', () => {
    expect(joined(highlight('aa aa', 'a'))).toBe('[aa] [aa]')
    expect(joined(highlight('abab', 'ab ba'))).toBe('[abab]')
  })

  it('keeps two words of the query apart when other words lie between them', () => {
    expect(joined(highlight('đối chiếu rồi soát', 'doi soat'))).toBe('[đối] chiếu rồi [soát]')
  })

  it('returns the whole text unmarked when nothing matches or the query is empty', () => {
    expect(highlight('xin chào', 'zzz')).toEqual([{ text: 'xin chào', hit: false }])
    expect(highlight('xin chào', '')).toEqual([{ text: 'xin chào', hit: false }])
    expect(highlight('', 'a')).toEqual([])
  })

  it('keeps the original text intact when it is made of combining marks', () => {
    const decomposed = 'Đối soát'.normalize('NFD')
    expect(highlight(decomposed, 'soat').map((p) => p.text).join('')).toBe(decomposed)
    expect(joined(highlight(decomposed, 'soat'))).toBe(`${'Đối '.normalize('NFD')}[${'soát'.normalize('NFD')}]`)
  })
})

describe('snippet', () => {
  it('leaves a short text alone', () => {
    expect(snippet('Bảng đối soát', 'doi', 80)).toBe('Bảng đối soát')
  })

  it('cuts a long text around the first match, with an ellipsis on the cut sides', () => {
    const text = `${'a'.repeat(100)} đối soát ${'b'.repeat(100)}`
    const out = snippet(text, 'doi soat', 60)
    expect(out.startsWith('…')).toBe(true)
    expect(out.endsWith('…')).toBe(true)
    expect(out).toContain('đối soát')
    expect(out.length).toBeLessThanOrEqual(62)
  })

  it('starts at the beginning when the match is near it', () => {
    const out = snippet(`đối soát ${'b'.repeat(200)}`, 'doi', 60)
    expect(out.startsWith('đối soát')).toBe(true)
    expect(out.endsWith('…')).toBe(true)
  })
})
