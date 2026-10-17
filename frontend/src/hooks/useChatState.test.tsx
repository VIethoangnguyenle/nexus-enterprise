import type { ReactNode } from 'react'
import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { queryClient } from '../lib/query-client'
import { useAuthStore } from '../stores/auth.store'
import { forgetChatState, loadChatState, RECENT_MS, type ChatState } from '../api/chat-state'
import { keys } from './keys'
import { revalidateChatState, useChatState } from './useChatState'
import { useChannels, useDirectMessages, useUnreadCounts } from './useMessaging'
import { useConversationActivity } from './useConversationActivity'
import { useChatSections } from './useChatSections'
import { useMentionsUnreadCount } from './useShortcuts'
import { useUnreadThreads } from './useUnreadThreads'

const WS = 'ws-1'

function tokenFor(tenant: string) {
  const b64 = (o: object) => btoa(JSON.stringify(o)).replace(/=+$/, '')
  return `${b64({ alg: 'none' })}.${b64({ tenant_id: tenant })}.sig`
}

const state: ChatState = {
  channels: [{ id: 'c1', name: 'Kế toán', channel_type: 'workspace', workspace_id: WS }],
  dms: [{ id: 'd1', name: 'An, Bình', channel_type: 'dm', workspace_id: '' }],
  unread: [{ channel_id: 'c1', unread_count: 3, last_read_message_id: '' }],
  activity: [],
  sections: [
    { id: 's2', name: 'Sau', position: 1, channel_ids: [] },
    { id: 's1', name: 'Trước', position: 0, channel_ids: ['c1'] },
  ],
  mention_unread_count: 2,
  unread_thread_count: 0,
  unread_threads: [],
}

const json = (body: unknown, init: ResponseInit = {}) =>
  new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' }, ...init })

interface Call { url: string; ifNoneMatch: string | null }
let calls: Call[]
let respond: (call: Call) => Response

function stubFetch() {
  calls = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const headers = new Headers(init?.headers)
    const call = { url, ifNoneMatch: headers.get('If-None-Match') }
    calls.push(call)
    return respond(call)
  }))
}

const stateCalls = () => calls.filter((c) => c.url.endsWith('/chat-state'))

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}

/** Every query the start-up used to make on its own. */
function useStartupQueries() {
  return {
    chat: useChatState(WS),
    channels: useChannels(WS),
    dms: useDirectMessages(),
    unread: useUnreadCounts(),
    activity: useConversationActivity(),
    sections: useChatSections(WS),
    mentions: useMentionsUnreadCount(WS),
    threads: useUnreadThreads(),
  }
}

beforeEach(() => {
  forgetChatState()
  queryClient.clear()
  queryClient.setDefaultOptions({ queries: { retry: false } })
  useAuthStore.setState({ accessToken: tokenFor(WS), user: { id: 'u1', username: 'an' } })
  respond = (c) => (c.url.endsWith('/chat-state') ? json(state, { headers: { ETag: '"v1"' } }) : json({}, { status: 500 }))
  stubFetch()
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  useAuthStore.setState({ accessToken: null, user: null })
})

describe('start-up chat state', () => {
  it('is one request for everything the navigator needs, however many screens ask', async () => {
    const { result } = renderHook(useStartupQueries, { wrapper })

    await waitFor(() => {
      expect(result.current.channels.data?.channels).toHaveLength(1)
      expect(result.current.dms.data?.channels).toHaveLength(1)
      expect(result.current.unread.data?.channels[0]?.unread_count).toBe(3)
      expect(result.current.mentions.data).toBe(2)
      expect(result.current.threads.data).toEqual([])
      expect(result.current.activity.data).toEqual({})
    })
    expect(result.current.sections.sections.map((s) => s.name)).toEqual(['Trước', 'Sau'])
    expect(calls.map((c) => c.url)).toEqual([`/api/workspaces/${WS}/chat-state`])
  })

  it('puts the state under the keys the rest of the app reads', async () => {
    const { result } = renderHook(() => useChatState(WS), { wrapper })
    await waitFor(() => expect(result.current.data).toBeTruthy())

    expect(queryClient.getQueryData(keys.messaging.channels(WS))).toEqual({ channels: state.channels })
    expect(queryClient.getQueryData(keys.messaging.dms())).toEqual({ channels: state.dms })
    expect(queryClient.getQueryData(keys.messaging.unreadCounts())).toEqual({ channels: state.unread })
    expect(queryClient.getQueryData(keys.shortcuts.mentionsUnread(WS))).toBe(2)
    expect((queryClient.getQueryData(keys.messaging.sections(WS)) as { name: string }[]).map((s) => s.name)).toEqual(['Trước', 'Sau'])
  })

  it('a 304 keeps what the cache holds and fetches nothing else', async () => {
    const { result } = renderHook(useStartupQueries, { wrapper })
    await waitFor(() => expect(result.current.channels.data).toBeTruthy())

    // The app comes back later; the server says nothing changed.
    vi.spyOn(Date, 'now').mockReturnValue(Date.now() + RECENT_MS + 1000)
    respond = (c) => (c.url.endsWith('/chat-state') ? new Response(null, { status: 304 }) : json({}, { status: 500 }))
    const marker = { channels: [...state.channels, { id: 'live', name: 'Từ sự kiện trực tiếp', channel_type: 'workspace', workspace_id: WS }] }
    queryClient.setQueryData(keys.messaging.channels(WS), marker)

    await act(async () => {
      await result.current.chat.refetch()
    })

    expect(stateCalls()).toHaveLength(2)
    expect(stateCalls()[1]?.ifNoneMatch).toBe('"v1"')
    expect(queryClient.getQueryData(keys.messaging.channels(WS))).toEqual(marker)
    expect(calls).toHaveLength(2)
  })

  it('a changed state replaces the cached parts', async () => {
    const { result } = renderHook(useStartupQueries, { wrapper })
    await waitFor(() => expect(result.current.channels.data).toBeTruthy())

    vi.spyOn(Date, 'now').mockReturnValue(Date.now() + RECENT_MS + 1000)
    respond = (c) =>
      c.url.endsWith('/chat-state')
        ? json({ ...state, mention_unread_count: 9 }, { headers: { ETag: '"v2"' } })
        : json({}, { status: 500 })
    await act(async () => {
      await result.current.chat.refetch()
    })

    expect(queryClient.getQueryData(keys.shortcuts.mentionsUnread(WS))).toBe(9)
    expect(stateCalls()[1]?.ifNoneMatch).toBe('"v1"')
  })

  it('falls back to each endpoint when the chat state cannot be had', async () => {
    respond = (c) => {
      if (c.url.endsWith('/chat-state')) return json({ error: 'not found' }, { status: 404 })
      if (c.url === `/api/workspaces/${WS}/channels`) return json({ channels: state.channels })
      if (c.url === '/api/dms') return json({ channels: [] })
      if (c.url === '/api/channels/unread') return json({ channels: [] })
      if (c.url === '/api/channels/activity') return json({ channels: [] })
      if (c.url === `/api/workspaces/${WS}/chat-sections`) return json({ sections: [] })
      if (c.url === '/api/messages/mentions/unread-count') return json({ count: 4 })
      if (c.url.startsWith('/api/threads/unread')) return json({ messages: [] })
      return json({}, { status: 500 })
    }
    const { result } = renderHook(useStartupQueries, { wrapper })

    await waitFor(() => {
      expect(result.current.channels.data?.channels).toHaveLength(1)
      expect(result.current.mentions.data).toBe(4)
      expect(result.current.threads.data).toEqual([])
    })
  })

  it('asks the workspace\'s own endpoints when the session speaks for another one', async () => {
    useAuthStore.setState({ accessToken: tokenFor('another-ws') })
    respond = (c) => (c.url === `/api/workspaces/${WS}/channels` ? json({ channels: [] }) : json({}, { status: 500 }))
    const { result } = renderHook(() => useChannels(WS), { wrapper })

    await waitFor(() => expect(result.current.data).toEqual({ channels: [] }))
    expect(stateCalls()).toHaveLength(0)
  })

  it('does not take a query that already holds data back to the chat state', async () => {
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [] })
    respond = (c) => (c.url === '/api/channels/unread' ? json({ channels: [{ channel_id: 'x', unread_count: 1, last_read_message_id: '' }] }) : json({}, { status: 500 }))
    const { result } = renderHook(() => useUnreadCounts(), { wrapper })
    await act(async () => {
      await result.current.refetch()
    })
    expect(stateCalls()).toHaveLength(0)
    const held = queryClient.getQueryData<{ channels: { channel_id: string }[] }>(keys.messaging.unreadCounts())
    expect(held?.channels.map((u) => u.channel_id)).toEqual(['x'])
  })
})

describe('loadChatState', () => {
  it('never offers one person\'s version to another', async () => {
    await loadChatState('u1:ws-1', WS)
    vi.spyOn(Date, 'now').mockReturnValue(Date.now() + RECENT_MS + 1000)
    respond = () => json(state, { headers: { ETag: '"other"' } })

    await loadChatState('u2:ws-1', WS)

    expect(stateCalls()).toHaveLength(2)
    expect(stateCalls()[1]?.ifNoneMatch).toBeNull()
  })

  it('keeps no version when the server issues none', async () => {
    respond = () => json(state) // no ETag: no Redis behind the server
    await loadChatState('u1:ws-1', WS)
    vi.spyOn(Date, 'now').mockReturnValue(Date.now() + RECENT_MS + 1000)
    await loadChatState('u1:ws-1', WS)

    expect(stateCalls()[1]?.ifNoneMatch).toBeNull()
  })

  it('shares one request among callers that ask at once', async () => {
    await Promise.all([loadChatState('u1:ws-1', WS), loadChatState('u1:ws-1', WS), loadChatState('u1:ws-1', WS)])
    expect(stateCalls()).toHaveLength(1)
  })

  it('a revalidation asks about a copy confirmed a moment ago, with its version', async () => {
    await loadChatState('u1:ws-1', WS)
    respond = () => new Response(null, { status: 304 })

    const answer = await loadChatState('u1:ws-1', WS, { revalidate: true })

    expect(stateCalls()).toHaveLength(2)
    expect(stateCalls()[1]?.ifNoneMatch).toBe('"v1"')
    expect(answer.changed).toBe(false)
  })

  it('a revalidation does not ride on a request that started before it', async () => {
    const earlier = loadChatState('u1:ws-1', WS)
    const later = loadChatState('u1:ws-1', WS, { revalidate: true })
    await Promise.all([earlier, later])
    expect(stateCalls()).toHaveLength(2)
  })
})

describe('revalidateChatState', () => {
  it('a 304 leaves the parts as the cache holds them', async () => {
    await loadChatState('u1:ws-1', WS)
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [] })
    respond = () => new Response(null, { status: 304 })

    expect(await revalidateChatState(WS)).toBe(true)

    expect(queryClient.getQueryData(keys.messaging.unreadCounts())).toEqual({ channels: [] })
    expect(stateCalls()).toHaveLength(2)
  })

  it('a changed state replaces the parts and the state itself', async () => {
    await loadChatState('u1:ws-1', WS)
    const next = { ...state, unread: [{ channel_id: 'c1', unread_count: 9, last_read_message_id: '' }] }
    respond = () => json(next, { headers: { ETag: '"v2"' } })

    expect(await revalidateChatState(WS)).toBe(true)

    expect(queryClient.getQueryData(keys.messaging.unreadCounts())).toEqual({ channels: next.unread })
    expect(queryClient.getQueryData(keys.messaging.chatState(WS))).toEqual(next)
  })

  it('cannot answer for a workspace the session does not speak for', async () => {
    expect(await revalidateChatState('ws-2')).toBe(false)
    expect(stateCalls()).toHaveLength(0)
  })

  it('cannot answer when the chat state fails', async () => {
    respond = () => json({}, { status: 500 })
    expect(await revalidateChatState(WS)).toBe(false)
  })
})
