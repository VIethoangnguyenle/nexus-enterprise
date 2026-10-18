import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../test/render'
import { routerState } from '../test/router-mock'
import { CH, U, fixtureApi } from '../test/chat-fixtures'
import { apiFetch } from '../api/client'
import { presetRange } from '../lib/chat-search'
import { SearchRoute } from './_workspace/-search-route'

vi.mock('@tanstack/react-router', async () => (await import('../test/router-mock')).routerMockFactory())
vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))

let calls: URLSearchParams[]
const route = (path: string, init?: RequestInit): Promise<unknown> => {
  if (path.startsWith('/messages/search')) {
    calls.push(new URLSearchParams(path.split('?')[1]))
    return Promise.resolve({ messages: [], has_more: false })
  }
  return fixtureApi(path, init)
}
const year = new Date().getFullYear()

beforeEach(() => {
  calls = []
  localStorage.clear()
  routerState.navigate.mockClear()
  routerState.back.mockClear()
  routerState.canGoBack = false
  routerState.pathname = '/channels/search'
  routerState.search = {}
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: false, media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  vi.mocked(apiFetch).mockImplementation(route as never)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

describe('the results page of a desktop', () => {
  it('reads the words and every filter from the address, and sends each under the name the server reads', async () => {
    routerState.search = {
      q: 'đối soát', from: U.vinh.id, in: CH.doisoat, since: `${year}-01-05`, until: `${year}-01-20`, file: 1, link: 1, me: 1,
    }
    renderWithClient(<SearchRoute />)
    await waitFor(() => expect(calls).toHaveLength(1))
    expect(Object.fromEntries(calls[0]!)).toEqual({
      q: 'đối soát', from: U.vinh.id, channel_id: CH.doisoat, after: `${year}-01-05`, before_date: `${year}-01-20`,
      tz: Intl.DateTimeFormat().resolvedOptions().timeZone, has_file: 'true', has_link: 'true', mentions_me: 'true', limit: '25',
    })
    // The screen shows names and days; the keys of the address stay in the address.
    expect(await screen.findByRole('button', { name: 'Người gửi: Lê Quang Vinh' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Trong: Đối soát giao dịch' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Ngày: 05/01 – 20/01' })).toBeInTheDocument()
    for (const name of ['Có tệp', 'Có liên kết', 'Nhắc đến tôi']) expect(screen.getByRole('button', { name })).toHaveAttribute('aria-pressed', 'true')
    expect(document.body.textContent).not.toContain(U.vinh.id)
    expect(document.body.textContent).not.toContain(CH.doisoat)
  })

  it('puts a chosen filter in the address, replacing the entry so back still leaves the search', async () => {
    routerState.search = { q: 'đối soát' }
    const user = userEvent.setup()
    renderWithClient(<SearchRoute />)
    await screen.findByRole('toolbar', { name: 'Bộ lọc' })
    await user.click(screen.getByRole('button', { name: 'Có tệp' }))
    expect(routerState.navigate).toHaveBeenLastCalledWith({ to: '/channels/search', search: { q: 'đối soát', file: 1 }, replace: true })

    await user.click(screen.getByRole('button', { name: 'Ngày' }))
    await user.click(await screen.findByRole('radio', { name: 'Hôm nay' }))
    const today = presetRange('today', new Date())
    expect(routerState.navigate).toHaveBeenLastCalledWith({
      to: '/channels/search', search: { q: 'đối soát', since: today.after, until: today.beforeDate }, replace: true,
    })
  })

  it('takes every filter out of the address with Xoá bộ lọc and keeps the words', async () => {
    routerState.search = { q: 'đối soát', from: U.vinh.id, link: 1 }
    const user = userEvent.setup()
    renderWithClient(<SearchRoute />)
    await user.click(await screen.findByRole('button', { name: 'Xoá bộ lọc' }))
    expect(routerState.navigate).toHaveBeenLastCalledWith({ to: '/channels/search', search: { q: 'đối soát' }, replace: true })
  })

  it('drops a malformed value from the address instead of failing', async () => {
    routerState.search = { q: 'đối soát', since: 'hôm qua', until: '2026-13-40', file: 'maybe' }
    renderWithClient(<SearchRoute />)
    await waitFor(() => expect(calls).toHaveLength(1))
    expect([...calls[0]!.keys()].sort()).toEqual(['limit', 'q'])
  })

  it('closes with ← to the conversation it was opened from', async () => {
    routerState.search = { q: 'đối soát' }
    routerState.canGoBack = true
    const user = userEvent.setup()
    renderWithClient(<SearchRoute />)
    await user.click(await screen.findByRole('button', { name: 'Đóng tìm kiếm' }))
    expect(routerState.back).toHaveBeenCalledTimes(1)
  })

  it('goes to Trang chủ instead when the page was the first one opened', async () => {
    routerState.search = { q: 'đối soát' }
    const user = userEvent.setup()
    renderWithClient(<SearchRoute />)
    await user.click(await screen.findByRole('button', { name: 'Đóng tìm kiếm' }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels' })
  })

  it('asks for the words when the address has none', async () => {
    renderWithClient(<SearchRoute />)
    expect(await screen.findByText('Gõ từ cần tìm vào ô tìm, rồi nhấn Enter.')).toBeInTheDocument()
    expect(calls).toHaveLength(0)
  })
})

describe('the same address below 1024px', () => {
  it('hands the words and filters to the full-screen search', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
    }))
    routerState.search = { q: 'đối soát', from: U.vinh.id, file: 1 }
    renderWithClient(<SearchRoute />)
    const redirect = await screen.findByTestId('redirect')
    expect(redirect).toHaveAttribute('data-to', '/channels')
    expect(JSON.parse(redirect.getAttribute('data-search')!)).toEqual({ search: 1, q: 'đối soát', from: U.vinh.id, file: 1 })
    expect(within(document.body).queryByRole('toolbar')).toBeNull()
  })
})
