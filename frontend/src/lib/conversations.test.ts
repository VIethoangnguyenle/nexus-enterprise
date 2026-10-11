import { describe, it, expect } from 'vitest'
import {
  buildConversations, dmPartner, filterConversations, sortConversations, totalUnread, unreadByLevel, unreadOf,
  type Conversation,
} from './conversations'
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

describe('what a conversation says it has unread, by notification level', () => {
  const c = (over: Partial<Conversation>): Conversation => ({
    id: 'x', kind: 'space', title: 'x', hueKey: 'x', unread: 0, memberCount: 0, lastActivity: 0, ...over,
  })

  it('counts every unread message at the default and "all" levels', () => {
    expect(unreadOf(c({ unread: 4, mentions: 1 }))).toBe(4)
    expect(unreadOf(c({ unread: 4, level: 'all' }))).toBe(4)
  })

  it('a muted conversation adds only its @mentions, never its unread messages', () => {
    expect(unreadOf(c({ unread: 9, level: 'off', mentions: 2 }))).toBe(2)
    expect(unreadOf(c({ unread: 9, level: 'off' }))).toBe(0)
  })

  it('totals the list by those rules', () => {
    const list = [c({ unread: 3 }), c({ unread: 7, level: 'off', mentions: 1 }), c({ unread: 2, level: 'all' })]
    expect(totalUnread(list)).toBe(3 + 1 + 2)
  })

  it('totals the raw unread list the same way (the sidebar badge)', () => {
    expect(unreadByLevel([
      { channel_id: 'a', unread_count: 3, last_read_message_id: '' },
      { channel_id: 'b', unread_count: 8, last_read_message_id: '', notification_level: 'off', mention_count: 2 },
      { channel_id: 'c', unread_count: 5, last_read_message_id: '', notification_level: 'off' },
    ])).toBe(3 + 2 + 0)
    expect(unreadByLevel(undefined)).toBe(0)
  })
})

describe('buildConversations with per-conversation state', () => {
  it('carries the level, the @mention count and the star; a muted conversation shows no unread count', () => {
    const list = buildConversations({
      spaces: SPACES, dms: DMS, unread, me, dir, lastMessages: {},
      meta: {
        [CH.doisoat]: { level: 'off', mentions: 2 },
        [CH.hopdong]: { starred: true },
        [CH.dmYen]: { level: 'all' },
      },
    })
    const by = (id: string) => list.find((x) => x.id === id)!
    expect(by(CH.doisoat)).toMatchObject({ level: 'off', mentions: 2, unread: 0 })
    expect(by(CH.hopdong)).toMatchObject({ starred: true, level: 'main' })
    expect(by(CH.vanhanh).starred).toBe(false)
  })

  it('never offers "all" on a direct message: it is read as the default', () => {
    const list = buildConversations({
      spaces: SPACES, dms: DMS, unread, me, dir, lastMessages: {}, meta: { [CH.dmYen]: { level: 'all' } },
    })
    expect(list.find((x) => x.id === CH.dmYen)!.level).toBe('main')
  })
})
