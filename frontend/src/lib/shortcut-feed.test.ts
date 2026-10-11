import { describe, expect, it } from 'vitest'
import type { FeedMessage } from '../api/shortcuts'
import { cardTime, feedCursor, groupByDay, messageSegments, openTarget } from './shortcut-feed'

const NOW = new Date(2026, 9, 11, 12, 0, 0)
const at = (y: number, mo: number, d: number, h = 9, mi = 0, nanos = 0) => ({
  seconds: Math.floor(new Date(y, mo - 1, d, h, mi).getTime() / 1000),
  nanos,
})
const msg = (id: string, over: Partial<FeedMessage> = {}): FeedMessage => ({
  id, channel_id: 'c1', sender_id: 'u2', content: 'x', created_at: at(2026, 10, 11), ...over,
})

describe('feedCursor', () => {
  it('pages by the nanosecond time of the last row and its id', () => {
    const page = { messages: [msg('a'), msg('b', { created_at: { seconds: 1760000000, nanos: 120000000 } })], has_more: true }
    expect(feedCursor(page, 'created_at')).toEqual({ before: '2025-10-09T08:53:20.12Z', beforeId: 'b' })
  })

  it('reads the star time for the starred feed', () => {
    const page = { messages: [msg('a', { starred_at: { seconds: 1760000000, nanos: 5 } })], has_more: true }
    expect(feedCursor(page, 'starred_at')).toEqual({ before: '2025-10-09T08:53:20.000000005Z', beforeId: 'a' })
  })

  it('ends when there is no more or the time is unreadable', () => {
    expect(feedCursor({ messages: [msg('a')], has_more: false }, 'created_at')).toBeUndefined()
    expect(feedCursor({ messages: [], has_more: true }, 'created_at')).toBeUndefined()
    expect(feedCursor({ messages: [msg('a')], has_more: true }, 'starred_at')).toBeUndefined()
  })
})

describe('groupByDay', () => {
  it('opens a group at each change of day, keeping the feed order', () => {
    const groups = groupByDay(
      [msg('a'), msg('b', { created_at: at(2026, 10, 11, 8) }), msg('c', { created_at: at(2026, 10, 10) }), msg('d', { created_at: at(2026, 9, 1) })],
      NOW,
    )
    expect(groups.map((g) => [g.label, g.items.map((m) => m.id)])).toEqual([
      ['Hôm nay', ['a', 'b']], ['Hôm qua', ['c']], ['01/09/2026', ['d']],
    ])
  })
})

describe('cardTime', () => {
  it('writes today as the time, yesterday with its word, then the date', () => {
    expect(cardTime(at(2026, 10, 11, 9, 12), NOW)).toBe('09:12')
    expect(cardTime(at(2026, 10, 10, 17, 20), NOW)).toBe('Hôm qua 17:20')
    expect(cardTime(at(2026, 10, 3), NOW)).toBe('03/10')
    expect(cardTime(at(2025, 12, 31), NOW)).toBe('31/12/2025')
  })
})

describe('messageSegments', () => {
  it('marks the reader as self and other people as mentions', () => {
    const html = '<p>Chào <span data-mention="me">@Hoa</span>, nhờ <span data-mention="u3">@Khải</span> xem giúp</p>'
    expect(messageSegments(html, 'html', 'me')).toEqual([
      { text: 'Chào ' }, { text: '@Hoa', mention: 'self' }, { text: ', nhờ ' }, { text: '@Khải', mention: 'other' }, { text: ' xem giúp' },
    ])
  })

  it('separates paragraphs with a space and never keeps markup', () => {
    const out = messageSegments('<p>một</p><p>hai<br>ba</p><script>alert(1)</script>', 'html', 'me')
    expect(out.map((s) => s.text).join('')).toBe('một hai ba')
  })

  it('shows a markdown or plain body as one folded line', () => {
    expect(messageSegments('xin  chào\n@hoa', 'markdown', 'me')).toEqual([{ text: 'xin chào @hoa' }])
  })
})

describe('openTarget', () => {
  it('opens a main message in its conversation, centred on it', () => {
    expect(openTarget(msg('m1'))).toEqual({ channelId: 'c1', messageId: 'm1', rootId: undefined })
  })

  it('opens a reply at its thread root', () => {
    expect(openTarget(msg('r1', { parent_message_id: 'm1' }))).toEqual({ channelId: 'c1', messageId: 'r1', rootId: 'm1' })
  })
})
