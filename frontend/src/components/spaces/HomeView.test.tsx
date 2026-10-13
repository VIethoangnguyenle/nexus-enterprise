import { screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { apiFetch } from '../../api/client'
import { routerState } from '../../test/router-mock'
import { CH, fixtureApi } from '../../test/chat-fixtures'
import { UUID_RE } from '../../test/chat-fixtures'
import { HomeView } from './HomeView'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})

afterEach(resetClient)

const rowNames = () =>
  within(screen.getByRole('list', { name: 'Cuộc trò chuyện' }))
    .getAllByRole('link')
    .map((a) => a.getAttribute('aria-label'))

describe('HomeView filter chips', () => {
  it('starts on "Tất cả" with every conversation, unread first', async () => {
    renderWithClient(<HomeView />)
    await screen.findByText('Phạm Hải Yến')
    await waitFor(() => expect(rowNames()).toHaveLength(5))
    expect(screen.getByRole('button', { name: 'Tất cả' })).toHaveAttribute('aria-pressed', 'true')
    // No activity seen this session, so unread rows lead.
    const names = rowNames()
    expect(names.slice(0, 2).sort()).toEqual(['Phạm Hải Yến', 'Đối soát giao dịch'].sort())
  })

  it('narrows to unread, spaces and direct messages', async () => {
    const user = userEvent.setup()
    renderWithClient(<HomeView />)
    await screen.findByText('Phạm Hải Yến')
    await waitFor(() => expect(rowNames()).toHaveLength(5))

    await user.click(screen.getByRole('button', { name: 'Chưa đọc' }))
    expect(screen.getByRole('button', { name: 'Chưa đọc' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Tất cả' })).toHaveAttribute('aria-pressed', 'false')
    await waitFor(() => expect(rowNames().sort()).toEqual(['Phạm Hải Yến', 'Đối soát giao dịch'].sort()))

    await user.click(screen.getByRole('button', { name: 'Nhóm' }))
    await waitFor(() =>
      expect(rowNames().sort()).toEqual(['Hợp đồng đối tác', 'Vận hành chung', 'Đối soát giao dịch'].sort()),
    )

    await user.click(screen.getByRole('button', { name: 'Trực tiếp' }))
    await waitFor(() => expect(rowNames().sort()).toEqual(['Nguyễn Thu Lan', 'Phạm Hải Yến'].sort()))
  })

  it('never renders an identifier', async () => {
    renderWithClient(<HomeView />)
    await screen.findByText('Phạm Hải Yến')
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })
})

describe('HomeView Thread có tin mới', () => {
  it('lists followed threads with news above the conversations and opens one in its group', async () => {
    vi.mocked(apiFetch).mockImplementation((path: string, init?: RequestInit) =>
      path.startsWith('/threads/unread')
        ? Promise.resolve({
            messages: [{
              id: '44444444-aaaa-4bbb-8ccc-000000000099', channel_id: CH.doisoat, sender_id: 'x', sender_name: 'Lê Quang Vinh',
              content: 'Ảnh sao kê', thread_unread_count: 2, reply_count: 3, last_reply_at: { seconds: 1760000000, nanos: 0 },
            }],
          })
        : fixtureApi(path, init))
    const user = userEvent.setup()
    renderWithClient(<HomeView />)
    const row = await screen.findByRole('button', { name: /^Thread của Lê Quang Vinh trong Đối soát giao dịch, 2 trả lời mới/ })
    await user.click(row)
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels/$channelId', params: { channelId: CH.doisoat }, search: { thread: '44444444-aaaa-4bbb-8ccc-000000000099' },
    })
    expect(document.body.textContent).not.toMatch(UUID_RE)
    vi.mocked(apiFetch).mockImplementation(fixtureApi as never)
  })
})
