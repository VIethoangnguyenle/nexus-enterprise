import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../test/render'
import { routerState } from '../test/router-mock'
import { CH, ME, U, WS_ID } from '../test/chat-fixtures'
import { createChatStateServer } from '../test/chat-state-server'
import { apiFetch } from '../api/client'
import type { FeedMessage } from '../api/shortcuts'
import { useToastStore } from '../components/primitives'
import { MentionsRoute } from './_workspace/-mentions-route'
import { StarredRoute } from './_workspace/-starred-route'

vi.mock('@tanstack/react-router', async () => {
  const mock = (await import('../test/router-mock')).routerMockFactory()
  return { ...mock, createFileRoute: () => (opts: unknown) => opts }
})
vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))

const stamp = () => ({ seconds: Math.floor(Date.now() / 1000) - 600, nanos: 0 })
const mention = (over: Partial<FeedMessage> = {}): FeedMessage => ({
  id: 'mm-1', channel_id: CH.vanhanh, sender_id: U.yen.id, sender_name: 'Phạm Hải Yến',
  content: `<p>Nhờ <span data-mention="${ME.id}">@Lê Thị Hoa</span> xem lại</p>`, content_format: 'html',
  created_at: stamp(), unread: true, ...over,
})

let server: ReturnType<typeof createChatStateServer>
const serve = (...args: Parameters<typeof createChatStateServer>) => {
  server = createChatStateServer(...args)
  vi.mocked(apiFetch).mockImplementation(server.api as never)
}

beforeEach(() => {
  routerState.params = {}
  routerState.pathname = '/channels/mentions'
  routerState.search = {}
  routerState.navigate.mockClear()
  routerState.canGoBack = false
  serve({}, 1, { mentions: [mention()] })
})
afterEach(() => {
  resetClient()
  useToastStore.getState().clear()
  vi.unstubAllGlobals()
})

describe('Được nhắc tới as a screen', () => {
  it('lists the mentions with the conversation each came from', async () => {
    renderWithClient(<MentionsRoute />)
    expect(await screen.findByRole('heading', { name: 'Được nhắc tới' })).toBeInTheDocument()
    const card = await screen.findByRole('link', { name: /xem lại/ })
    expect(within(card).getByText('Vận hành chung')).toBeInTheDocument()
  })

  it('opens a main message in its conversation, lighting that message', async () => {
    const user = userEvent.setup()
    renderWithClient(<MentionsRoute />)
    await user.click(await screen.findByRole('link', { name: /xem lại/ }))
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels/$channelId', params: { channelId: CH.vanhanh }, search: { message: 'mm-1' },
    })
  })

  it('opens a reply in its thread, lighting the reply in the panel', async () => {
    const user = userEvent.setup()
    serve({}, 1, { mentions: [mention({ id: 'rr-1', parent_message_id: 'root-9' })] })
    renderWithClient(<MentionsRoute />)
    await user.click(await screen.findByRole('link', { name: /xem lại/ }))
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels/$channelId', params: { channelId: CH.vanhanh }, search: { thread: 'root-9', message: 'rr-1' },
    })
  })

  it('gives each card a real link that carries the thread, the message and the workspace', async () => {
    serve({}, 1, { mentions: [mention({ id: 'rr-1', parent_message_id: 'root-9' })] })
    renderWithClient(<MentionsRoute />)
    const card = await screen.findByRole('link', { name: /xem lại/ })
    const url = new URL(card.getAttribute('href')!, 'http://x')
    expect(url.pathname).toBe(`/channels/${CH.vanhanh}`)
    expect(Object.fromEntries(url.searchParams)).toEqual({ thread: 'root-9', message: 'rr-1', ws: WS_ID })
  })

  it('goes back to Trang chủ from the empty state', async () => {
    const user = userEvent.setup()
    serve({}, 0, { mentions: [] })
    renderWithClient(<MentionsRoute />)
    await user.click(await screen.findByRole('button', { name: 'Về Trang chủ' }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels' })
  })

  it('has no Quay lại bar on a desktop, and goes back through history on a phone', async () => {
    const user = userEvent.setup()
    const { unmount } = renderWithClient(<MentionsRoute />)
    await screen.findByRole('heading', { name: 'Được nhắc tới' })
    // Hidden from lg up by the view's own class, and not even offered without the phone layout.
    expect(screen.queryByRole('button', { name: /Quay lại/ })).toBeNull()
    unmount()

    vi.stubGlobal('matchMedia', (q: string) => ({
      matches: q.includes('1023.98px'), media: q, addEventListener() {}, removeEventListener() {},
    }))
    routerState.canGoBack = true
    routerState.back.mockClear()
    renderWithClient(<MentionsRoute />)
    await user.click(await screen.findByRole('button', { name: /Quay lại/ }))
    expect(routerState.back).toHaveBeenCalled()
  })

  it('on a phone with nothing to go back to, Quay lại lands on Tin nhắn (Trang chủ)', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('matchMedia', (q: string) => ({
      matches: q.includes('1023.98px'), media: q, addEventListener() {}, removeEventListener() {},
    }))
    // The browser's own history may be long; only the app's own entries count.
    routerState.canGoBack = false
    vi.spyOn(window.history, 'length', 'get').mockReturnValue(5)
    renderWithClient(<MentionsRoute />)
    await user.click(await screen.findByRole('button', { name: /Quay lại/ }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels' })
  })
})

describe('Đã gắn sao as a screen', () => {
  beforeEach(() => {
    routerState.pathname = '/channels/starred'
    serve({ [CH.hopdong]: { starred: true } }, 0, {
      starred: [mention({ id: 'st-1', unread: undefined, starred: true, content_format: 'plain', content: 'Bản chốt hợp đồng' })],
    })
  })

  it('lists the starred conversations (from the unread list) and the starred messages', async () => {
    renderWithClient(<StarredRoute />)
    const convs = await screen.findByRole('list', { name: 'Cuộc trò chuyện' })
    await waitFor(() => expect(within(convs).getByText('Hợp đồng đối tác')).toBeInTheDocument())
    expect(await screen.findByText('Bản chốt hợp đồng')).toBeInTheDocument()
    expect(within(convs).queryByText('Vận hành chung')).toBeNull()
  })

  it('orders the starred conversations by when each was starred, newest first', async () => {
    serve({
      [CH.hopdong]: { starred: true, starred_at: { seconds: 1760000000, nanos: 0 } },
      [CH.vanhanh]: { starred: true, starred_at: { seconds: 1760009000, nanos: 0 } },
    }, 0, { starred: [] })
    renderWithClient(<StarredRoute />)
    const convs = await screen.findByRole('list', { name: 'Cuộc trò chuyện' })
    await waitFor(() => expect(within(convs).getAllByRole('link')).toHaveLength(2))
    expect(within(convs).getAllByRole('link').map((a) => a.textContent)).toEqual([
      expect.stringContaining('Vận hành chung'), expect.stringContaining('Hợp đồng đối tác'),
    ])
  })

  it('opens a starred conversation, and a starred message with its message id', async () => {
    const user = userEvent.setup()
    renderWithClient(<StarredRoute />)
    const convs = await screen.findByRole('list', { name: 'Cuộc trò chuyện' })
    await user.click(await within(convs).findByRole('link', { name: /Hợp đồng đối tác/ }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: CH.hopdong } })
    await user.click(await screen.findByRole('link', { name: /Bản chốt hợp đồng/ }))
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels/$channelId', params: { channelId: CH.vanhanh }, search: { message: 'st-1' },
    })
  })

  it('unstarring a conversation calls the server and takes the row away', async () => {
    const user = userEvent.setup()
    renderWithClient(<StarredRoute />)
    await user.click(await screen.findByRole('button', { name: 'Gắn sao Hợp đồng đối tác' }))
    await waitFor(() => expect(server.calls).toContainEqual(
      expect.objectContaining({ method: 'DELETE', path: `/channels/${CH.hopdong}/star` })))
    await waitFor(() => expect(screen.queryByRole('link', { name: /Hợp đồng đối tác/ })).toBeNull())
  })
})
