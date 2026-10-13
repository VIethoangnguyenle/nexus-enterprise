import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { WS_ID } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import { PhoneMoreList } from './PhoneMoreList'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

const onOpenFollowedThreads = vi.fn()
const show = (mentionsUnread = 0, threadsWithNews = 0) => {
  vi.mocked(apiFetch).mockImplementation(createChatStateServer({}, mentionsUnread).api as never)
  return renderWithClient(
    <PhoneMoreList workspaceId={WS_ID} threadsWithNews={threadsWithNews} onOpenFollowedThreads={onOpenFollowedThreads} />,
  )
}

beforeEach(() => onOpenFollowedThreads.mockClear())
afterEach(() => {
  cleanup()
  resetClient()
})

describe('Thêm on a phone', () => {
  it('has a title and the four shortcuts of Chat, in order', () => {
    show()
    expect(screen.getByRole('heading', { level: 1, name: 'Thêm' })).toBeInTheDocument()
    const nav = screen.getByRole('navigation', { name: 'Lối tắt của Chat' })
    expect(within(nav).getAllByRole('link').concat(within(nav).getAllByRole('button')).length).toBe(4)
    expect(within(nav).getAllByText(/./, { selector: 'span.flex-1' }).map((e) => e.textContent)).toEqual([
      'Được nhắc tới', 'Tin đã gắn sao', 'Thread đang theo dõi', 'Thông báo của Chat',
    ])
  })

  it('opens Được nhắc tới and Tin đã gắn sao, 60px rows', () => {
    show()
    const mentions = screen.getByRole('link', { name: 'Được nhắc tới' })
    expect(mentions).toHaveAttribute('href', '/channels/mentions')
    expect(mentions.className).toContain('min-h-15')
    expect(screen.getByRole('link', { name: 'Tin đã gắn sao' })).toHaveAttribute('href', '/channels/starred')
  })

  it('counts the unread mentions in a pill', async () => {
    show(3)
    const link = await screen.findByRole('link', { name: 'Được nhắc tới, 3 tin chưa đọc' })
    expect(link).toHaveTextContent('3')
  })

  it('says how many followed threads have news, and opens the Thread screen on them', async () => {
    const user = userEvent.setup()
    show(0, 2)
    const row = screen.getByRole('button', { name: /Thread đang theo dõi/ })
    expect(row).toHaveTextContent('2 có tin mới')
    await user.click(row)
    expect(onOpenFollowedThreads).toHaveBeenCalledTimes(1)
  })

  it('says nothing extra when no followed thread has news', () => {
    show(0, 0)
    expect(screen.getByRole('button', { name: 'Thread đang theo dõi' })).not.toHaveTextContent('có tin mới')
  })

  it('leads to the notification settings of the app', () => {
    show()
    const link = screen.getByRole('link', { name: /Thông báo của Chat/ })
    expect(link).toHaveAttribute('href', '/settings')
    expect(JSON.parse(link.getAttribute('data-search')!)).toEqual({ tab: 'thong-bao' })
  })

  it('does not repeat the modules of the ☰ menu, and has no group browser (nothing serves it)', async () => {
    show()
    await waitFor(() => expect(apiFetch).toHaveBeenCalled())
    for (const name of ['Tài liệu', 'Phê duyệt', 'Tài sản', 'Danh bạ', 'Quản trị', 'Duyệt nhóm']) {
      expect(screen.queryByText(name)).toBeNull()
    }
  })
})
