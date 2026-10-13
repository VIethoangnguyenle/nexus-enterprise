import { describe, it, expect } from 'vitest'
import {
  buildConversations, dmPartner, filterConversations, sortConversations, totalUnread, unreadByLevel, unreadOf,
  type Conversation, type LastMessage,
} from './conversations'
import type { ActivityMessage, ConversationActivity } from '../api/messaging'
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

  it('lists a muted conversation under "Chưa đọc" only while an @mention of the caller is unread', () => {
    const row = (over: Partial<Conversation>): Conversation => ({
      id: 'x', kind: 'space', title: 'x', hueKey: 'x', unread: 0, memberCount: 0, lastActivity: 0, ...over,
    })
    const list = [
      row({ id: 'muted-mention', level: 'off', unread: 0, mentions: 2 }),
      row({ id: 'muted-quiet', level: 'off', unread: 0, mentions: 0 }),
      row({ id: 'normal', unread: 1 }),
      row({ id: 'read' }),
    ]
    // The filter matches the badge the row would show.
    expect(filterConversations(list, 'unread').map((c) => c.id)).toEqual(['muted-mention', 'normal'])
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
        [CH.hopdong]: { starred: true, starredAt: 1760000000000 },
        [CH.dmYen]: { level: 'all' },
      },
    })
    const by = (id: string) => list.find((x) => x.id === id)!
    expect(by(CH.doisoat)).toMatchObject({ level: 'off', mentions: 2, unread: 0 })
    expect(by(CH.hopdong)).toMatchObject({ starred: true, starredAt: 1760000000000, level: 'main' })
    expect(by(CH.vanhanh).starred).toBe(false)
  })

  it('never offers "all" on a direct message: it is read as the default', () => {
    const list = buildConversations({
      spaces: SPACES, dms: DMS, unread, me, dir, lastMessages: {}, meta: { [CH.dmYen]: { level: 'all' } },
    })
    expect(list.find((x) => x.id === CH.dmYen)!.level).toBe('main')
  })
})

describe('buildConversations with server activity', () => {
  const at = (minutesAgo: number) => new Date(Date.now() - minutesAgo * 60_000).toISOString()
  const msg = (over: Partial<ActivityMessage>): ActivityMessage => ({
    id: 'm1', sender_id: U.yen.id, sender_name: 'yen', preview: 'Chị xem giúp em', kind: 'text', created_at: at(5), ...over,
  })
  const build = (
    activity: Record<string, ConversationActivity>,
    lastMessages: Record<string, LastMessage> = {},
  ) => buildConversations({ spaces: SPACES, dms: DMS, unread, me, dir, activity, lastMessages })
  const by = (list: Conversation[], id: string) => list.find((c) => c.id === id)!

  it('shows the server preview with no live event at all', () => {
    const list = build({ [CH.hopdong]: { channel_id: CH.hopdong, latest: msg({}) } })
    const c = by(list, CH.hopdong)
    expect(c.main).toMatchObject({ author: 'Phạm Hải Yến', text: 'Chị xem giúp em' })
    expect(c.reply).toBeUndefined()
    expect(c.preview).toBe('Chị xem giúp em')
    expect(c.previewAuthor).toBe('Phạm Hải Yến')
    expect(c.lastActivity).toBeGreaterThan(0)
  })

  it('writes "Bạn" for my own message', () => {
    const list = build({ [CH.hopdong]: { channel_id: CH.hopdong, latest: msg({ sender_id: ME.id }) } })
    expect(by(list, CH.hopdong).main?.author).toBe('Bạn')
  })

  it('carries a thread reply that is newer than the main message, and orders by the reply', () => {
    const list = build({
      [CH.hopdong]: {
        channel_id: CH.hopdong,
        latest: msg({ created_at: at(30) }),
        latest_reply: msg({ id: 'r1', sender_id: U.lan.id, preview: 'Em gửi lại ạ', parent_message_id: 'root-1', created_at: at(2) }),
      },
    })
    const c = by(list, CH.hopdong)
    expect(c.reply).toMatchObject({ author: 'Nguyễn Thu Lan', text: 'Em gửi lại ạ', parentId: 'root-1' })
    expect(c.main?.text).toBe('Chị xem giúp em')
    expect(c.lastActivity).toBe(c.reply!.at)
    // What the desktop row quotes is the newest message of the two.
    expect(c.preview).toBe('Em gửi lại ạ')
    expect(c.lastSenderId).toBe(U.lan.id)
  })

  it('says which people a message names, by display name, and whether it names me', () => {
    const list = build({
      [CH.hopdong]: {
        channel_id: CH.hopdong,
        latest: msg({ created_at: at(30) }),
        latest_reply: msg({
          id: 'r1', preview: '@Lê Thị Hoa xem dòng 14', parent_message_id: 'root-1', created_at: at(2),
          mentions: [ME.id, 'unknown-user'],
        }),
      },
    })
    expect(by(list, CH.hopdong).reply).toMatchObject({ mentionNames: ['Lê Thị Hoa'], mentionsMe: true })
    expect(by(list, CH.hopdong).main?.mentionsMe).toBe(false)
  })

  it('turns files and cards into words', () => {
    const one = (m: Partial<ActivityMessage>) =>
      by(build({ [CH.hopdong]: { channel_id: CH.hopdong, latest: msg(m) } }), CH.hopdong).main!.text
    expect(one({ kind: 'attachment', preview: '', attachment_name: 'Ke-hoach.xlsx' })).toBe('📎 Ke-hoach.xlsx')
    expect(one({ kind: 'attachment', preview: 'xem giúp', attachment_name: 'a.pdf' })).toBe('📎 a.pdf')
    expect(one({ kind: 'attachment', preview: '', attachment_name: undefined })).toBe('📎 Tệp đính kèm')
    expect(one({ kind: 'poll', preview: 'Liên hoan ngày nào?' })).toBe('Bình chọn: Liên hoan ngày nào?')
    expect(one({ kind: 'task', preview: 'Gửi báo cáo' })).toBe('Công việc: Gửi báo cáo')
    expect(one({ kind: 'poll', preview: '' })).toBe('Bình chọn')
    expect(one({ kind: 'card', preview: '' })).toBe('Thẻ đính kèm')
  })

  it('lets a live message newer than anything the server knows win', () => {
    const list = build(
      { [CH.hopdong]: { channel_id: CH.hopdong, latest: msg({ created_at: at(30) }) } },
      { [CH.hopdong]: { id: 'live', content: 'Mới nhất', timestamp: at(1), senderName: 'lan', senderId: U.lan.id } },
    )
    const c = by(list, CH.hopdong)
    expect(c.main).toMatchObject({ id: 'live', author: 'Nguyễn Thu Lan', text: 'Mới nhất' })
    expect(c.preview).toBe('Mới nhất')
  })

  it('keeps the server message when the live one is not newer', () => {
    const t = at(5)
    const list = build(
      { [CH.hopdong]: { channel_id: CH.hopdong, latest: msg({ created_at: t, kind: 'attachment', preview: '', attachment_name: 'a.pdf' }) } },
      { [CH.hopdong]: { content: '', timestamp: t, senderName: 'yen', senderId: U.yen.id } },
    )
    expect(by(list, CH.hopdong).main?.text).toBe('📎 a.pdf')
  })

  it('a live thread reply becomes the reply line and leaves the main message alone', () => {
    const list = build(
      { [CH.hopdong]: { channel_id: CH.hopdong, latest: msg({ created_at: at(30) }) } },
      {
        [CH.hopdong]: {
          id: 'live-r', content: 'Đã sửa xong', timestamp: at(1), senderName: 'lan', senderId: U.lan.id,
          parentId: 'root-9', mentions: [ME.id],
        },
      },
    )
    const c = by(list, CH.hopdong)
    expect(c.main?.text).toBe('Chị xem giúp em')
    expect(c.reply).toMatchObject({ author: 'Nguyễn Thu Lan', text: 'Đã sửa xong', parentId: 'root-9', mentionsMe: true })
  })

  it('a live main message drops an older server reply', () => {
    const list = build(
      {
        [CH.hopdong]: {
          channel_id: CH.hopdong, latest: msg({ created_at: at(30) }),
          latest_reply: msg({ id: 'r1', parent_message_id: 'root-1', created_at: at(10) }),
        },
      },
      { [CH.hopdong]: { id: 'live', content: 'Tin mới', timestamp: at(1), senderName: 'lan', senderId: U.lan.id } },
    )
    const c = by(list, CH.hopdong)
    expect(c.reply).toBeUndefined()
    expect(c.main?.text).toBe('Tin mới')
  })

  it('works from a live message alone (activity not loaded yet)', () => {
    const list = build({}, { [CH.hopdong]: { content: 'Chào', timestamp: at(1), senderName: 'yen', senderId: U.yen.id } })
    expect(by(list, CH.hopdong).main?.text).toBe('Chào')
  })

  it('has nothing to show for a conversation without a message, and never an id', () => {
    const list = build({ [CH.hopdong]: { channel_id: CH.hopdong, latest: msg({}) } })
    expect(by(list, CH.vanhanh)).toMatchObject({ lastActivity: 0, main: undefined, reply: undefined })
    expect(JSON.stringify(list.map((c) => [c.main?.author, c.main?.text, c.reply?.author]))).not.toMatch(UUID_RE)
  })
})
