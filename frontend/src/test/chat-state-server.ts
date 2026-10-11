import { vi } from 'vitest'
import type { ChannelUnread, Message } from '../api/messaging'
import type { FeedMessage } from '../api/shortcuts'
import type { NotificationLevel } from '../api/notification-levels'
import { ApiError } from '../api/client'
import { CH, UNREAD, fixtureApi } from './chat-fixtures'

/**
 * A stateful stand-in for the per-conversation state the unread list carries:
 * notification level, @mentions and star. It sits on top of `fixtureApi`,
 * answers the star and level endpoints by changing that state, and records the
 * calls. `failNext(fragment)` makes the next call whose path has the fragment answer 500.
 */
export function createChatStateServer(
  initial: Record<string, Partial<ChannelUnread>> = {},
  mentionsUnread = 0,
  feeds: { mentions?: FeedMessage[]; starred?: FeedMessage[]; threads?: Message[] } = {},
) {
  const base = new Map<string, ChannelUnread>(UNREAD.map((u) => [u.channel_id, { ...u }]))
  const state = new Map<string, ChannelUnread>()
  const rows = () => [...state.values()]
  for (const id of Object.values(CH)) {
    state.set(id, { channel_id: id, unread_count: 0, last_read_message_id: '', ...base.get(id), ...initial[id] })
  }
  const calls: { method: string; path: string; body?: unknown }[] = []
  const failures: string[] = []

  const api = vi.fn(async (path: string, init?: RequestInit): Promise<unknown> => {
    const method = (init?.method || 'GET').toUpperCase()
    const p = path.split('?')[0]!
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    const star = p.match(/^\/channels\/([^/]+)\/star$/)
    const level = p.match(/^\/channels\/([^/]+)\/notification-level$/)
    if (method === 'GET' && (p === '/messages/mentions' || p === '/messages/starred')) {
      calls.push({ method, path: p })
      return { messages: structuredClone((p === '/messages/mentions' ? feeds.mentions : feeds.starred) ?? []), has_more: false }
    }
    if (method === 'DELETE' && /^\/messages\/[^/]+\/star$/.test(p)) {
      calls.push({ method, path: p })
      return {}
    }
    if (!(p === '/channels/unread' || star || level || p === '/messages/mentions/unread-count' || p === '/threads/unread')) {
      return fixtureApi(path, init)
    }
    calls.push({ method, path: p, body })
    const f = failures.findIndex((x) => p.includes(x))
    if (f >= 0 && method !== 'GET') {
      failures.splice(f, 1)
      throw new ApiError('failed', 500, {})
    }
    if (p === '/channels/unread') return structuredClone({ channels: rows() })
    if (p === '/messages/mentions/unread-count') return { count: mentionsUnread }
    if (p === '/threads/unread') return { messages: structuredClone(feeds.threads ?? []) }
    if (star) {
      const row = state.get(star[1]!)!
      if (method === 'PUT') row.starred = true
      else delete row.starred
      return {}
    }
    if (level) {
      const row = state.get(level[1]!)!
      if (method === 'GET') return { level: row.notification_level ?? 'main' }
      const next = body.level as NotificationLevel
      row.notification_level = next
      if (next === 'off') row.unread_count = 0
      return { level: next }
    }
    return {}
  })

  return {
    api,
    calls,
    state: (id: string) => state.get(id)!,
    failNext: (fragment: string) => failures.push(fragment),
  }
}
