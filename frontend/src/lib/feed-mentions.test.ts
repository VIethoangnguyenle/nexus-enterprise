import { describe, expect, it } from 'vitest'
import { splitMentions } from './feed-mentions'

describe('splitMentions', () => {
  it('marks "@" plus the exact display name of someone mentioned', () => {
    expect(splitMentions('@Lê Thị Hoa em sửa xong rồi', ['Lê Thị Hoa'])).toEqual([
      { text: '@Lê Thị Hoa', mention: true },
      { text: ' em sửa xong rồi', mention: false },
    ])
  })

  it('marks every occurrence and every person named', () => {
    const parts = splitMentions('chị @Hoa và @Đức xem giúp, @Hoa nhé', ['Hoa', 'Đức'])
    expect(parts.filter((p) => p.mention).map((p) => p.text)).toEqual(['@Hoa', '@Đức', '@Hoa'])
    expect(parts.map((p) => p.text).join('')).toBe('chị @Hoa và @Đức xem giúp, @Hoa nhé')
  })

  it('does not mark a name that only starts like the mentioned one', () => {
    expect(splitMentions('@Lê Thị Hoang ơi', ['Lê Thị Hoa'])).toEqual([{ text: '@Lê Thị Hoang ơi', mention: false }])
  })

  it('prefers the longer of two names that overlap', () => {
    const parts = splitMentions('@Lê Thị Hoa ơi', ['Lê Thị', 'Lê Thị Hoa'])
    expect(parts.find((p) => p.mention)?.text).toBe('@Lê Thị Hoa')
  })

  it('leaves the text whole without names, and treats name characters literally', () => {
    expect(splitMentions('@Hoa ơi', [])).toEqual([{ text: '@Hoa ơi', mention: false }])
    expect(splitMentions('@A.B (kế toán) ơi', ['A.B (kế toán)'])[0]).toEqual({ text: '@A.B (kế toán)', mention: true })
    expect(splitMentions('', ['Hoa'])).toEqual([])
  })
})
