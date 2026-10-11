import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { CH, UUID_RE } from '../../test/chat-fixtures'
import { createSectionsServer } from '../../test/chat-sections-server'
import { apiFetch } from '../../api/client'
import { useNavCollapseStore } from '../../stores/nav-collapse.store'
import { HomeView } from './HomeView'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

function viewport(phone: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

beforeEach(() => {
  const server = createSectionsServer([{ id: 'q4', name: 'Quyết toán quý 4', position: 0, channel_ids: [CH.doisoat] }])
  vi.mocked(apiFetch).mockImplementation(server.api as never)
})
afterEach(() => {
  vi.unstubAllGlobals()
  resetClient()
  useNavCollapseStore.setState({ collapsed: {} })
})

describe('Trang chủ below 1024px', () => {
  it('is the Tin nhắn tab of the design: sections first, no filter chips', async () => {
    viewport(true)
    renderWithClient(<HomeView />)
    await screen.findByRole('list', { name: 'Quyết toán quý 4' })
    expect(screen.queryByRole('group', { name: 'Lọc' })).toBeNull()
    expect(screen.getAllByRole('list').map((l) => l.getAttribute('aria-label')))
      .toEqual(['Quyết toán quý 4', 'Tin nhắn trực tiếp', 'Nhóm'])
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('offers "Tạo mục" in Trò chuyện mới, which opens the name sheet', async () => {
    viewport(true)
    const user = userEvent.setup()
    renderWithClient(<HomeView />)
    await screen.findByRole('list', { name: 'Quyết toán quý 4' })
    await user.click(screen.getByRole('button', { name: 'Trò chuyện mới' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Tạo mục' }))
    const sheet = await screen.findByRole('dialog', { name: 'Tên mục mới' })
    expect(within(sheet).getByLabelText('Tên mục mới')).toBeInTheDocument()
  })
})

describe('Trang chủ at desktop width', () => {
  it('keeps the filtered feed', async () => {
    viewport(false)
    renderWithClient(<HomeView />)
    expect(await screen.findByRole('group', { name: 'Lọc' })).toBeInTheDocument()
    expect(await screen.findByRole('list', { name: 'Cuộc trò chuyện' })).toBeInTheDocument()
  })
})
