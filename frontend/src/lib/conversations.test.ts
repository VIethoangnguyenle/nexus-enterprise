import { describe, it, expect } from 'vitest'
import { buildConversations, dmPartner, filterConversations, sortConversations, totalUnread } from './conversations'
import { buildDirectory } from './people'
import { CONTACTS, DMS, SPACES, CH, U, ME, UUID_RE } from '../test/chat-fixtures'

const dir = buildDirectory(CONTACTS)
const me = { id: ME.id, username: ME.username }
const unread = { [CH.doisoat]: 3, [CH.dmYen]: 1 }

describe('dmPartner', () => {
  it('titles a DM after the other person, by display name', () => {
    expect(dmPartner({ name: 'hoa, yen' }, me, dir).title).toBe('Phạm Hải Yến')
    expect(dmPartner({ name: 'lan, hoa' }, me, dir).title).toBe('Nguyễn Thu Lan')
  })
  it('falls back to the channel name when the name does not split cleanly', () => {
    expect(dmPartner({ name: 'Một cuộc trò chuyện' }, me, dir).title).toBe('Một cuộc trò chuyện')
  })
})

describe('buildConversations', () => {
  it('names the preview author by display name, and "Bạn" for me', () => {
    const list = buildConversations({
      spaces: SPACES, dms: DMS, unread, me, dir,
      lastMessages: {
        [CH.hopdong]: { content: 'Em đã gửi bản scan', timestamp: new Date().toISOString(), senderName: 'yen', senderId: U.yen.id },
        [CH.vanhanh]: { content: 'Lịch trực đã cập nhật', timestamp: new Date().toISOString(), senderName: 'hoa', senderId: ME.id },
      },
    })
    expect(list.find((c) => c.id === CH.hopdong)?.previewAuthor).toBe('Phạm Hải Yến')
    expect(list.find((c) => c.id === CH.vanhanh)?.previewAuthor).toBe('Bạn')
    expect(JSON.stringify(list.map((c) => [c.title, c.preview, c.previewAuthor]))).not.toMatch(UUID_RE)
  })
})

describe('sortConversations / filterConversations', () => {
  const base = buildConversations({ spaces: SPACES, dms: DMS, unread, me, dir, lastMessages: {} })

  it('puts recent activity first, then unread, then name', () => {
    const withActivity = base.map((c) => (c.id === CH.hopdong ? { ...c, lastActivity: Date.now() } : c))
    const sorted = sortConversations(withActivity).map((c) => c.title)
    expect(sorted[0]).toBe('Hợp đồng đối tác')
    expect(sorted.slice(1, 3).sort()).toEqual(['Phạm Hải Yến', 'Đối soát giao dịch'].sort())
  })

  it('filters by unread, spaces and direct', () => {
    expect(filterConversations(base, 'unread').map((c) => c.id).sort()).toEqual([CH.doisoat, CH.dmYen].sort())
    expect(filterConversations(base, 'spaces').every((c) => c.kind === 'space')).toBe(true)
    expect(filterConversations(base, 'direct').every((c) => c.kind === 'dm')).toBe(true)
    expect(totalUnread(base)).toBe(4)
  })
})
