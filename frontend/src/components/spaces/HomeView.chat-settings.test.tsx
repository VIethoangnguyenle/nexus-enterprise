import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, U, WS_ID } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import type { ActivityMessage } from '../../api/messaging'
import type { ChatSettings } from '../../api/settings'
import { queryClient } from '../../lib/query-client'
import { settingsQueryKey } from '../../hooks/useServerSettings'
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

const ago = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString()
const msg = (over: Partial<ActivityMessage>): ActivityMessage => ({
  id: 'm', sender_id: U.yen.id, sender_name: 'yen', preview: 'xin chào', kind: 'text', created_at: ago(5), ...over,
})

beforeEach(() => {
  routerState.search = {}
  routerState.historyState = {}
  routerState.navigate.mockClear()
  localStorage.clear()
  const server = createChatStateServer({}, 0, {
    activity: [
      // Activity puts Vận hành first and Đối soát second, which is not the order of their names.
      { channel_id: CH.vanhanh, latest: msg({ id: 'a', preview: 'Lịch chạy lô', created_at: ago(1) }) },
      { channel_id: CH.doisoat, latest: msg({ id: 'b', preview: 'Bảng đối soát', created_at: ago(2) }) },
    ],
  })
  vi.mocked(apiFetch).mockImplementation(server.api as never)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
  useNavCollapseStore.setState({ collapsed: {} })
})

const seed = (s: ChatSettings) => queryClient.setQueryData(settingsQueryKey('chat', WS_ID), s)

const show = async (tab?: string) => {
  viewport(true)
  routerState.search = tab ? { tab } : {}
  renderWithClient(<HomeView />)
  await screen.findByRole('heading', { level: 1, name: 'Chat' })
}

/** The conversation rows of the list, by title, top to bottom. */
const titles = async () => {
  const list = await screen.findByRole('list', { name: 'Cuộc trò chuyện' })
  return within(list).getAllByRole('link').map((a) => a.getAttribute('aria-label') ?? '').filter((n) => !n.startsWith('Mở thread'))
}

describe('Cài đặt Chat → Mở Chat ở', () => {
  it('Mục: Chat opens on the Mục chip when the address names none', async () => {
    seed({ open_at: 'sections' })
    await show()
    await waitFor(() => expect(screen.getByRole('button', { name: /^Mục/ })).toHaveAttribute('aria-pressed', 'true'))
    expect(screen.getByRole('button', { name: /^Tất cả/ })).toHaveAttribute('aria-pressed', 'false')
  })

  it('Mục: choosing Tất cả names it in the address, so it holds', async () => {
    const user = userEvent.setup()
    seed({ open_at: 'sections' })
    await show()
    await waitFor(() => expect(screen.getByRole('button', { name: /^Mục/ })).toHaveAttribute('aria-pressed', 'true'))
    await user.click(screen.getByRole('button', { name: /^Tất cả/ }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels', search: { tab: 'all' }, replace: true })
  })

  it('Tất cả (the default): Chat opens on Tất cả', async () => {
    seed({ open_at: 'all' })
    await show()
    expect(screen.getByRole('button', { name: /^Tất cả/ })).toHaveAttribute('aria-pressed', 'true')
  })

  it('the address still wins over the setting', async () => {
    seed({ open_at: 'sections' })
    await show('direct')
    expect(screen.getByRole('button', { name: /^Trực tiếp/ })).toHaveAttribute('aria-pressed', 'true')
  })
})

describe('Cài đặt Chat → Sắp xếp', () => {
  it('Tên: the list is in alphabetical order; Hoạt động mới nhất keeps the newest first', async () => {
    seed({ sort: 'activity' })
    await show()
    const byActivity = await titles()
    expect(byActivity.indexOf('Vận hành chung')).toBeLessThan(byActivity.indexOf('Đối soát giao dịch'))
    cleanup()
    resetClient()

    seed({ sort: 'name' })
    await show()
    const collator = new Intl.Collator('vi', { sensitivity: 'base' })
    await waitFor(async () => {
      const names = await titles()
      expect(names).toEqual([...names].sort(collator.compare))
    })
    const byName = await titles()
    expect(byName.indexOf('Đối soát giao dịch')).toBeLessThan(byName.indexOf('Vận hành chung'))
  })
})
