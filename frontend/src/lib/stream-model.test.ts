import { describe, expect, it } from 'vitest'
import type { Message } from '../api/messaging'
import {
  GROUP_WINDOW_MS, buildStream, dayLabel, firstUnreadReplyId, formatLastReply, keepOlderPages, threadSummary, timestampToRfc3339, unreadStartId,
} from './stream-model'

const NOW = new Date(2026, 9, 11, 12, 0) // Sunday 11/10/2026 12:00
const at = (h: number, m: number, day = 11) => new Date(2026, 9, day, h, m).toISOString()

let n = 0
const msg = (sender: string, created: string, extra: Partial<Message> = {}): Message => ({
  id: `m${++n}`, channel_id: 'c', sender_id: sender, content: `tin ${n}`, created_at: created, ...extra,
})

describe('dayLabel', () => {
  it('says Hôm nay and Hôm qua', () => {
    expect(dayLabel(at(8, 0), NOW)).toBe('Hôm nay')
    expect(dayLabel(at(23, 59, 10), NOW)).toBe('Hôm qua')
  })
  it('names the weekday within a week and the full date after that', () => {
    expect(dayLabel(at(9, 0, 8), NOW)).toBe('Thứ Năm, 08/10')
    expect(dayLabel(at(9, 0, 5), NOW)).toBe('Thứ Hai, 05/10')
    expect(dayLabel(at(9, 0, 4), NOW)).toBe('04/10/2026')
    expect(dayLabel(new Date(2026, 8, 28, 9).toISOString(), NOW)).toBe('28/09/2026')
  })
  it('reads protobuf timestamps', () => {
    expect(dayLabel({ seconds: Math.floor(new Date(2026, 9, 11, 9).getTime() / 1000), nanos: 0 }, NOW)).toBe('Hôm nay')
  })
})

describe('buildStream grouping', () => {
  it('groups the same sender within five minutes', () => {
    const a = msg('x', at(9, 0))
    const b = msg('x', at(9, 5))
    const c = msg('x', at(9, 11))
    const items = buildStream([a, b, c], { me: 'me', now: NOW })
    const rows = items.filter((i) => i.kind === 'message')
    expect(rows.map((r) => r.kind === 'message' && r.grouped)).toEqual([false, true, false])
    expect(GROUP_WINDOW_MS).toBe(5 * 60_000)
  })

  it('does not group across senders', () => {
    const items = buildStream([msg('x', at(9, 0)), msg('y', at(9, 1))], { me: 'me', now: NOW })
    expect(items.filter((i) => i.kind === 'message').map((r) => r.kind === 'message' && r.grouped)).toEqual([false, false])
  })

  it('breaks the group after a message that has a thread', () => {
    const a = msg('x', at(9, 0), { reply_count: 2 })
    const b = msg('x', at(9, 1))
    const rows = buildStream([a, b], { me: 'me', now: NOW }).filter((i) => i.kind === 'message')
    expect(rows[1]!.kind === 'message' && rows[1]!.grouped).toBe(false)
  })

  it('breaks the group at a day separator and at the unread divider', () => {
    const a = msg('x', at(23, 58, 10))
    const b = msg('x', at(0, 1, 11))
    const c = msg('x', at(0, 2, 11))
    const items = buildStream([a, b, c], { me: 'me', now: NOW, unreadFromId: c.id })
    expect(items.map((i) => i.kind)).toEqual(['day', 'message', 'day', 'message', 'unread', 'message'])
    const rows = items.filter((i) => i.kind === 'message')
    expect(rows.map((r) => r.kind === 'message' && r.grouped)).toEqual([false, false, false])
  })

  it('breaks the group around a system line (task)', () => {
    const a = msg('x', at(9, 0))
    const t = msg('x', at(9, 1), { linked_entity_type: 'task', content: '📋 Task: Làm báo cáo' })
    const b = msg('x', at(9, 2))
    const rows = buildStream([a, t, b], { me: 'me', now: NOW }).filter((i) => i.kind === 'message')
    expect(rows.map((r) => r.kind === 'message' && [r.system, r.grouped])).toEqual([[false, false], [true, false], [false, false]])
  })

  it('treats the line a manager leaves when deleting a message as a system line too', () => {
    const a = msg('x', at(9, 0))
    const t = msg('x', at(9, 1), { message_type: 'system', linked_entity_type: 'message_deleted', content: 'Hoa đã xoá một tin nhắn của Lan' })
    const b = msg('x', at(9, 2))
    const rows = buildStream([a, t, b], { me: 'me', now: NOW }).filter((i) => i.kind === 'message')
    expect(rows.map((r) => r.kind === 'message' && [r.system, r.grouped])).toEqual([[false, false], [true, false], [false, false]])
  })

  it('marks your own messages', () => {
    const rows = buildStream([msg('me', at(9, 0)), msg('y', at(9, 1))], { me: 'me', now: NOW }).filter((i) => i.kind === 'message')
    expect(rows.map((r) => r.kind === 'message' && r.mine)).toEqual([true, false])
  })
})

describe('buildStream dividers', () => {
  it('puts one day separator per day, first', () => {
    const items = buildStream([msg('x', at(9, 0, 10)), msg('x', at(9, 0)), msg('x', at(9, 30))], { me: 'me', now: NOW })
    expect(items.map((i) => (i.kind === 'day' ? i.label : i.kind))).toEqual(['Hôm qua', 'message', 'Hôm nay', 'message', 'message'])
  })

  it('puts the "Tin mới" divider right before the first unread message', () => {
    const read = msg('x', at(9, 0))
    const unread = msg('y', at(9, 10))
    const later = msg('y', at(9, 11))
    const items = buildStream([read, unread, later], { me: 'me', now: NOW, unreadFromId: unread.id })
    expect(items.map((i) => i.kind)).toEqual(['day', 'message', 'unread', 'message', 'message'])
    const idx = items.findIndex((i) => i.kind === 'unread')
    const next = items[idx + 1]
    expect(next?.kind === 'message' && next.message.id).toBe(unread.id)
  })

  it('has no divider when nothing is unread or the message is gone', () => {
    expect(buildStream([msg('x', at(9, 0))], { me: 'me', now: NOW }).some((i) => i.kind === 'unread')).toBe(false)
    expect(buildStream([msg('x', at(9, 0))], { me: 'me', now: NOW, unreadFromId: 'nope' }).some((i) => i.kind === 'unread')).toBe(false)
  })
})

describe('unreadStartId', () => {
  const a = msg('x', at(9, 0))
  const b = msg('me', at(9, 5))
  const c = msg('y', at(9, 10))
  const d = msg('y', at(9, 12))
  const all = [a, b, c, d]

  it('starts after the last read message, skipping your own', () => {
    expect(unreadStartId(all, { lastReadId: a.id, unreadCount: 3, me: 'me' })).toBe(c.id)
  })
  it('is null when the last read message is the newest', () => {
    expect(unreadStartId(all, { lastReadId: d.id, unreadCount: 0, me: 'me' })).toBeNull()
  })
  it('is null when only your own messages follow', () => {
    expect(unreadStartId([a, b], { lastReadId: a.id, unreadCount: 1, me: 'me' })).toBeNull()
  })
  it('counts back from the newest when the read position is older than what is loaded', () => {
    expect(unreadStartId(all, { lastReadId: 'older', unreadCount: 2, me: 'me' })).toBe(c.id)
    expect(unreadStartId(all, { lastReadId: '', unreadCount: 2, me: 'me' })).toBe(c.id)
  })
  it('is null with nothing unread and no known position', () => {
    expect(unreadStartId(all, { lastReadId: '', unreadCount: 0, me: 'me' })).toBeNull()
  })
  it('never picks an optimistic message', () => {
    const temp = { ...msg('y', at(9, 13)), _optimistic: true } as Message
    expect(unreadStartId([a, temp], { lastReadId: a.id, unreadCount: 1, me: 'me' })).toBeNull()
  })
})

describe('thread summary', () => {
  const nameOf = (id: string) => ({ u1: 'Lê Quang Vinh', u2: 'Nguyễn Thu Lan', u3: 'Trần Minh Đức' })[id] ?? 'Thành viên'

  it('reads the server summary: count, faces newest first, last time', () => {
    const root = msg('x', at(9, 0), {
      reply_count: 3, last_reply_at: at(9, 20), reply_participant_ids: ['u1', 'u2', 'u3', 'u1'], following: true, thread_unread_count: 2,
    })
    const s = threadSummary(root, nameOf, NOW)!
    expect(s.count).toBe(3)
    expect(s.faces.map((f) => f.name)).toEqual(['Lê Quang Vinh', 'Nguyễn Thu Lan', 'Trần Minh Đức'])
    expect(s.last).toBe('09:20')
    expect(s.unread).toBe(2)
    expect(s.label).toBe('Thread, 3 trả lời, lần cuối 09:20, 2 trả lời mới')
  })

  it('shows "mới" only while following', () => {
    const base = { reply_count: 2, last_reply_at: at(9, 20), reply_participant_ids: ['u1'] }
    expect(threadSummary(msg('x', at(9, 0), { ...base, following: true, thread_unread_count: 1 }), nameOf, NOW)!.unread).toBe(1)
    expect(threadSummary(msg('x', at(9, 0), { ...base, following: false, thread_unread_count: 1 }), nameOf, NOW)!.unread).toBe(0)
    expect(threadSummary(msg('x', at(9, 0), { ...base }), nameOf, NOW)!.label).toBe('Thread, 2 trả lời, lần cuối 09:20')
  })

  it('has no summary without replies', () => {
    expect(threadSummary(msg('x', at(9, 0)), nameOf, NOW)).toBeNull()
    expect(threadSummary(msg('x', at(9, 0), { reply_count: 0 }), nameOf, NOW)).toBeNull()
  })

  it('formats the last reply time', () => {
    expect(formatLastReply(at(9, 20), NOW)).toBe('09:20')
    expect(formatLastReply(at(17, 5, 10), NOW)).toBe('Hôm qua 17:05')
    expect(formatLastReply(at(17, 5, 6), NOW)).toBe('06/10')
    expect(formatLastReply(new Date(2025, 11, 31, 8).toISOString(), NOW)).toBe('31/12/2025')
    expect(formatLastReply(undefined, NOW)).toBe('')
  })

  it('never exposes an id in the labels', () => {
    const root = msg('x', at(9, 0), { reply_count: 1, last_reply_at: at(9, 20), reply_participant_ids: ['11111111-aaaa-4bbb-8ccc-000000000001'] })
    const s = threadSummary(root, nameOf, NOW)!
    expect(JSON.stringify([s.label, s.faces.map((f) => f.name)])).not.toMatch(/[0-9a-f]{8}-[0-9a-f]{4}/)
  })
})

describe('timestampToRfc3339', () => {
  it('keeps the nanoseconds of a protobuf timestamp', () => {
    expect(timestampToRfc3339({ seconds: '1760000000', nanos: 123456000 })).toBe('2025-10-09T08:53:20.123456Z')
  })
  it('passes an ISO string through as UTC', () => {
    expect(timestampToRfc3339('2026-10-11T05:00:00.5+07:00')).toBe('2026-10-10T22:00:00.500Z')
  })
  it('is empty when unreadable', () => {
    expect(timestampToRfc3339(undefined)).toBe('')
  })
})

describe('firstUnreadReplyId', () => {
  const r1 = msg('x', at(9, 0))
  const r2 = msg('me', at(9, 1))
  const r3 = msg('y', at(9, 2))
  const r4 = msg('x', at(9, 3))
  const replies = [r1, r2, r3, r4]

  it('counts back over other people\'s replies, skipping yours', () => {
    expect(firstUnreadReplyId(replies, 2, 'me')).toBe(r3.id)
    expect(firstUnreadReplyId(replies, 3, 'me')).toBe(r1.id)
  })
  it('takes the oldest when more are unread than are loaded', () => {
    expect(firstUnreadReplyId(replies, 9, 'me')).toBe(r1.id)
  })
  it('is null with nothing unread', () => {
    expect(firstUnreadReplyId(replies, 0, 'me')).toBeNull()
    expect(firstUnreadReplyId([], 2, 'me')).toBeNull()
  })
})

describe('keepOlderPages', () => {
  // The server's JSON leaves an empty list out: a conversation with no message yet answers `{}`.
  it('reads an answer with no messages field as an empty conversation', () => {
    const empty = {} as { messages: Message[]; has_more: boolean }
    expect(keepOlderPages(empty, undefined)).toEqual({ messages: [], has_more: false })
    expect(keepOlderPages(empty, { messages: [], has_more: false })).toEqual({ messages: [], has_more: false })
  })

  // Lists are newest first, as the API returns them.
  const m = (id: string, h: number) => msg('x', at(h, 0), { id })
  const fresh = { messages: [m('n3', 12), m('n2', 11), m('n1', 10)], has_more: true }

  it('keeps the older pages already loaded when the newest page is fetched again', () => {
    const previous = { messages: [m('n3', 12), m('n2', 11), m('n1', 10), m('o2', 9), m('o1', 8)], has_more: false }
    const merged = keepOlderPages(fresh, previous)
    expect(merged.messages.map((x) => x.id)).toEqual(['n3', 'n2', 'n1', 'o2', 'o1'])
    expect(merged.has_more).toBe(false)
  })

  it('drops the old pages when the fresh one does not reach them, so no gap is hidden', () => {
    // More than a page arrived while away: nothing in the fresh page was cached.
    const previous = { messages: [m('o2', 9), m('o1', 8)], has_more: false }
    expect(keepOlderPages(fresh, previous)).toBe(fresh)
  })

  it('takes the fresh page as it is when nothing older was loaded', () => {
    expect(keepOlderPages(fresh, undefined)).toBe(fresh)
    expect(keepOlderPages(fresh, { messages: fresh.messages, has_more: true })).toEqual(fresh)
  })

  it('drops what the server no longer sends in the newest page, and keeps only what is older than it', () => {
    const previous = { messages: [m('gone', 11), m('n1', 10), m('o1', 8)], has_more: false }
    const merged = keepOlderPages({ messages: [m('n2', 11), m('n1', 10)], has_more: true }, previous)
    expect(merged.messages.map((x) => x.id)).toEqual(['n2', 'n1', 'o1'])
  })

  it('does not keep optimistic messages from the old list', () => {
    const temp = { ...m('temp-1', 7), _optimistic: true } as Message
    expect(keepOlderPages(fresh, { messages: [temp, ...fresh.messages], has_more: true }).messages.map((x) => x.id)).toEqual(['n3', 'n2', 'n1'])
  })
})
