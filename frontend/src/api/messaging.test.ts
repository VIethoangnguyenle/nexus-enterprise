import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock fetch and auth store before importing messaging API
const mockFetch = vi.fn()
globalThis.fetch = mockFetch

vi.mock('../stores/auth.store', () => ({
  useAuthStore: { getState: () => ({ accessToken: 'test-token', logout: vi.fn(), setAccessToken: vi.fn() }) },
}))

import { messagingApi } from './messaging'

beforeEach(() => {
  vi.clearAllMocks()
  mockFetch.mockResolvedValue({
    ok: true,
    status: 200,
    json: () => Promise.resolve({}),
  })
})

// ---------------------------------------------------------------------------
// 11.7: API URL construction correct
// ---------------------------------------------------------------------------

describe('messagingApi', () => {
  it('searchAll builds the query string and normalises an empty page', async () => {
    const page = await messagingApi.searchAll({ q: 'đối soát', hasFile: true, before: '2026-01-01T00:00:00Z', beforeId: 'm-1', limit: 25 })
    const url = mockFetch.mock.calls[0]?.[0] as string
    expect(url.startsWith('/api/messages/search?')).toBe(true)
    const qs = new URLSearchParams(url.split('?')[1] ?? '')
    expect(qs.get('q')).toBe('đối soát')
    expect(qs.get('has_file')).toBe('true')
    expect(qs.get('before')).toBe('2026-01-01T00:00:00Z')
    expect(qs.get('before_id')).toBe('m-1')
    expect(qs.get('limit')).toBe('25')
    expect(page).toEqual({ messages: [], has_more: false })

    await messagingApi.searchAll({ q: 'kế hoạch' })
    expect(mockFetch.mock.calls[1]?.[0]).toBe('/api/messages/search?q=k%E1%BA%BF+ho%E1%BA%A1ch')
  })

  it('listChannels builds correct URL', async () => {
    await messagingApi.listChannels('ws-123')
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/workspaces/ws-123/channels',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer test-token' }),
      }),
    )
  })

  it('createChannel sends POST with body', async () => {
    await messagingApi.createChannel('ws-123', { name: 'general', channel_type: 'workspace' })
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/workspaces/ws-123/channels',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ name: 'general', channel_type: 'workspace' }),
      }),
    )
  })

  it('listMessages builds URL with before param', async () => {
    await messagingApi.listMessages('ch-1', '2024-01-01')
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/channels/ch-1/messages?before=2024-01-01',
      expect.any(Object),
    )
  })

  it('listMessages builds URL without before param', async () => {
    await messagingApi.listMessages('ch-1')
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/channels/ch-1/messages',
      expect.any(Object),
    )
  })

  it('sendMessage builds correct URL and body', async () => {
    await messagingApi.sendMessage('ch-1', 'hello')
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/channels/ch-1/messages',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ content: 'hello' }),
      }),
    )
  })

  it('sendMessage carries the quoted message when there is one', async () => {
    await messagingApi.sendMessage('ch-1', { content: 'ok', content_format: 'html', quoted_message_id: 'msg-9' })
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/channels/ch-1/messages',
      expect.objectContaining({ body: JSON.stringify({ content: 'ok', content_format: 'html', quoted_message_id: 'msg-9' }) }),
    )
  })

  it('editMessage patches the message with its new text and format', async () => {
    await messagingApi.editMessage('msg-7', { content: '<p>sửa</p>', content_format: 'html' })
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/messages/msg-7',
      expect.objectContaining({ method: 'PATCH', body: JSON.stringify({ content: '<p>sửa</p>', content_format: 'html' }) }),
    )
  })

  it('deleteMessage and markMessageUnread address the message', async () => {
    await messagingApi.deleteMessage('msg-7')
    expect(mockFetch).toHaveBeenLastCalledWith('/api/messages/msg-7', expect.objectContaining({ method: 'DELETE' }))
    await messagingApi.markMessageUnread('msg-7')
    expect(mockFetch).toHaveBeenLastCalledWith('/api/messages/msg-7/unread', expect.objectContaining({ method: 'POST' }))
  })

  it('getThread builds correct URL', async () => {
    await messagingApi.getThread('msg-42')
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/messages/msg-42/thread',
      expect.any(Object),
    )
  })
})
