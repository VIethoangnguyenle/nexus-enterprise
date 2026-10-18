import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { swipe } from '../../test/gesture-events'
import { REQUESTS, approvalFixtureApi, resetFixtures } from '../../test/approval-fixtures'
import { validateApprovalSearch } from '../../lib/approval-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { useWebSocketStore } from '../../stores/websocket.store'
import { ApprovalScreen } from './ApprovalScreen'

vi.mock('../../api/client', async (orig) => {
  const { approvalFixtureApi } = await import('../../test/approval-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(approvalFixtureApi) }
})

const api = vi.mocked(apiFetch)
const ORIGINAL = structuredClone(REQUESTS)
let phone = true

async function renderApproval(url = '/approval') {
  const root = createRootRoute()
  const route = createRoute({
    getParentRoute: () => root, path: '/approval', validateSearch: validateApprovalSearch, component: ApprovalScreen,
  })
  const router = createRouter({ routeTree: root.addChildren([route]), history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  phone = true
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  Object.defineProperty(window, 'innerWidth', { value: 400, configurable: true, writable: true })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetFixtures()
  for (const [id, r] of Object.entries(structuredClone(ORIGINAL))) REQUESTS[id] = r
  api.mockClear()
  api.mockImplementation(approvalFixtureApi)
  useWebSocketStore.setState({ approvalActivity: {} })
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

/** The first request of Chờ tôi: its title, and the element a finger drags. */
async function firstRow() {
  const table = await screen.findByRole('table')
  const title = await waitFor(() => {
    const t = table.querySelector<HTMLElement>('[data-row]')
    expect(t).not.toBeNull()
    return t!
  })
  return { title: title.textContent!, surface: title.closest('[data-swipe-row]')!.lastElementChild! }
}
const decided = () => api.mock.calls.filter(([p]) => p === '/approval/approve' || p === '/approval/reject')
const fullRight = (el: Element) => swipe(el, [{ x: 40, t: 0 }, { x: 52, t: 16 }, { x: 395, t: 600 }])
const fullLeft = (el: Element) => swipe(el, [{ x: 395, t: 0 }, { x: 383, t: 16 }, { x: 25, t: 600 }])

describe('Phê duyệt on a phone: a swipe never decides', () => {
  it('a full swipe right approves nothing; it leaves Duyệt… revealed', async () => {
    await renderApproval()
    const row = await firstRow()
    fullRight(row.surface)
    expect(within(row.surface.parentElement!).getByRole('button', { name: 'Duyệt…' })).toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 50))
    expect(decided()).toEqual([])
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('a full swipe left rejects nothing; it leaves Từ chối… and Thêm revealed', async () => {
    await renderApproval()
    const row = await firstRow()
    fullLeft(row.surface)
    const revealed = within(row.surface.parentElement!)
    expect(revealed.getByRole('button', { name: 'Từ chối…' })).toBeInTheDocument()
    expect(revealed.getByRole('button', { name: 'Thêm' })).toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 50))
    expect(decided()).toEqual([])
  })

  it('Duyệt… opens a confirmation; cancelling it approves nothing', async () => {
    const user = userEvent.setup()
    await renderApproval()
    const row = await firstRow()
    fullRight(row.surface)
    await user.click(within(row.surface.parentElement!).getByRole('button', { name: 'Duyệt…' }))
    const dialog = await screen.findByRole('dialog', { name: 'Duyệt đề nghị?' })
    expect(dialog).toHaveTextContent(row.title)
    await user.click(within(dialog).getByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(decided()).toEqual([])
  })

  it('only Duyệt in the confirmation approves, with the comment typed there', async () => {
    const user = userEvent.setup()
    await renderApproval()
    const row = await firstRow()
    fullRight(row.surface)
    await user.click(within(row.surface.parentElement!).getByRole('button', { name: 'Duyệt…' }))
    const dialog = await screen.findByRole('dialog', { name: 'Duyệt đề nghị?' })
    await user.type(within(dialog).getByLabelText('Ý kiến'), 'Đồng ý, chuyển kế toán')
    await user.click(within(dialog).getByRole('button', { name: 'Duyệt' }))
    await waitFor(() => expect(decided()).toHaveLength(1))
    const [path, init] = decided()[0]!
    expect(path).toBe('/approval/approve')
    expect(JSON.parse(String(init!.body)).comment).toBe('Đồng ý, chuyển kế toán')
  })

  it('Từ chối… opens the return dialog, which asks for a reason before anything is sent', async () => {
    const user = userEvent.setup()
    await renderApproval()
    const row = await firstRow()
    fullLeft(row.surface)
    await user.click(within(row.surface.parentElement!).getByRole('button', { name: 'Từ chối…' }))
    const dialog = await screen.findByRole('dialog', { name: 'Trả lại đề nghị?' })
    await user.click(within(dialog).getByRole('button', { name: 'Trả lại' }))
    expect(decided()).toEqual([])
  })

  it('rows of the other tabs do not swipe', async () => {
    await renderApproval('/approval?tab=mine')
    const table = await screen.findByRole('table')
    await waitFor(() => expect(table.querySelector('[data-row]')).not.toBeNull())
    expect(table.querySelector('[data-swipe-row]')).toBeNull()
  })

  it('a desktop has no swipe', async () => {
    phone = false
    await renderApproval()
    const table = await screen.findByRole('table')
    await waitFor(() => expect(table.querySelector('[data-row]')).not.toBeNull())
    expect(table.querySelector('[data-swipe-row]')).toBeNull()
  })
})
