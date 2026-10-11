import { act, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { CH, U, UUID_RE } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import { useWebSocketStore } from '../../stores/websocket.store'
import { HomeView } from './HomeView'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

beforeEach(() => {
  const server = createChatStateServer({
    [CH.hopdong]: { notification_level: 'off', unread_count: 0, mention_count: 1 },
  })
  vi.mocked(apiFetch).mockImplementation(server.api as never)
})
afterEach(() => {
  resetClient()
  useWebSocketStore.setState({ lastMessages: {} })
})

const rows = () => within(screen.getByRole('list', { name: 'Cuộc trò chuyện' }))
const speak = (channelId: string) =>
  act(() => {
    useWebSocketStore.setState({
      lastMessages: {
        ...useWebSocketStore.getState().lastMessages,
        [channelId]: { content: 'Xin chào', timestamp: new Date().toISOString(), senderName: 'yen', senderId: U.yen.id },
      },
    })
  })

describe('Trang chủ with a muted conversation', () => {
  it('draws it quiet: bell-off, "@" for the mention, no count, spoken as muted', async () => {
    renderWithClient(<HomeView />)
    const link = await waitFor(() => rows().getByRole('link', { name: 'Hợp đồng đối tác, đã tắt thông báo' }))
    expect(link).toHaveAttribute('data-muted', 'true')
    expect(within(link).getByLabelText('1 lần nhắc tên bạn')).toBeInTheDocument()
    expect(link.querySelector('svg.lucide-bell-off')).not.toBeNull()
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('washes a conversation that speaks, but not a muted one', async () => {
    renderWithClient(<HomeView />)
    await waitFor(() => rows().getByRole('link', { name: 'Hợp đồng đối tác, đã tắt thông báo' }))
    await waitFor(() => expect(rows().getAllByRole('link')).toHaveLength(5))
    speak(CH.vanhanh)
    speak(CH.hopdong)
    await waitFor(() => expect(rows().getByRole('link', { name: 'Vận hành chung' })).toHaveClass('rt-wash'))
    const muted = rows().getByRole('link', { name: 'Hợp đồng đối tác, đã tắt thông báo' })
    expect(muted).not.toHaveClass('rt-wash')
    expect(within(muted).queryByText('vừa nhắn')).toBeNull()
  })
})
