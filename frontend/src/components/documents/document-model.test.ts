import { describe, expect, it } from 'vitest'
import { diffLines, groupOf, saveStateLabel, statusPill, textLines, titleOf } from './document-model'

describe('statusPill', () => {
  it('words every status, and never shows a code', () => {
    expect(statusPill('draft').label).toBe('Bản nháp')
    expect(statusPill('active').label).toBe('Đang dùng')
    expect(statusPill('archived').label).toBe('Lưu trữ')
    expect(statusPill('weird').label).toBe('Bản nháp')
  })
})

describe('groups and titles', () => {
  it('falls back to all documents for an unknown group', () => {
    expect(groupOf('drafts').scope).toBe('drafts')
    expect(groupOf('shared').scope).toBe('shared')
    expect(groupOf('nope').scope).toBe('all')
    expect(groupOf(undefined).scope).toBe('all')
  })

  it('names an untitled document', () => {
    expect(titleOf({ title: '  ' })).toBe('Văn bản chưa đặt tên')
    expect(titleOf({ title: 'Quy trình' })).toBe('Quy trình')
  })
})

describe('saveStateLabel', () => {
  it('says what is going on in words, with the time once saved', () => {
    const at = new Date(2026, 9, 10, 9, 41)
    expect(saveStateLabel('saved', at)).toBe('Đã lưu 09:41')
    expect(saveStateLabel('saved', null)).toBe('Đã lưu')
    expect(saveStateLabel('saving', null)).toBe('Đang lưu…')
    expect(saveStateLabel('offline', null)).toBe('Chưa lưu, đang chờ mạng')
    expect(saveStateLabel('conflict', null)).toMatch(/có bản mới hơn/)
    expect(saveStateLabel('dirty', null)).toBe('Chưa lưu')
    expect(saveStateLabel('error', null)).toBe('Chưa lưu được')
  })
})

describe('textLines', () => {
  it('reads blocks as lines and ignores markup', () => {
    const html = '<h2>Quy trình</h2><p>Áp dụng từ <strong>15/10</strong>.</p><ul><li>Một</li><li><p>Hai</p></li></ul><blockquote><p>Trích</p></blockquote>'
    expect(textLines(html)).toEqual(['Quy trình', 'Áp dụng từ 15/10.', 'Một', 'Hai', 'Trích'])
  })

  it('never reads script or event handlers', () => {
    expect(textLines('<p onclick="x()">an toàn</p><script>alert(1)</script>')).toEqual(['an toàn'])
  })

  it('reads loose text, and nothing for nothing', () => {
    expect(textLines('chỉ có chữ')).toEqual(['chỉ có chữ'])
    expect(textLines('')).toEqual([])
    expect(textLines('<p></p>')).toEqual([])
  })
})

describe('diffLines', () => {
  it('marks only the lines the other side does not have', () => {
    const d = diffLines(['a', 'b', 'c'], ['a', 'x', 'c'])
    expect(d.mine).toEqual([{ text: 'a', changed: false }, { text: 'b', changed: true }, { text: 'c', changed: false }])
    expect(d.theirs).toEqual([{ text: 'a', changed: false }, { text: 'x', changed: true }, { text: 'c', changed: false }])
  })

  it('treats an added or removed line as a change on that side only', () => {
    const d = diffLines(['a'], ['a', 'b'])
    expect(d.mine.every((l) => !l.changed)).toBe(true)
    expect(d.theirs).toEqual([{ text: 'a', changed: false }, { text: 'b', changed: true }])
  })

  it('is empty for two empty documents, and unchanged for equal ones', () => {
    expect(diffLines([], [])).toEqual({ mine: [], theirs: [] })
    expect(diffLines(['a', 'b'], ['a', 'b']).mine.some((l) => l.changed)).toBe(false)
  })

  it('does not call a long document changed just because it is long', () => {
    const big = Array.from({ length: 2000 }, (_, i) => `line ${i}`)
    expect(diffLines(big, big).mine.some((l) => l.changed)).toBe(false)
  })
})
