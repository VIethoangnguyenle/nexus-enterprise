import { screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
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

describe('HomeView on a phone', () => {
  it('offers Trò chuyện mới, so a chat or a group can be started without the navigator column', async () => {
    const user = userEvent.setup()
    renderWithClient(<HomeView />)
    await user.click(await screen.findByRole('button', { name: 'Trò chuyện mới' }))
    const menu = await screen.findByRole('menu', { name: 'Trò chuyện mới' })
    expect(within(menu).getByRole('menuitem', { name: 'Nhắn tin trực tiếp' })).toBeInTheDocument()
    await user.click(within(menu).getByRole('menuitem', { name: 'Tạo nhóm' }))
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })
})

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
