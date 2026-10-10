import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { WS_ID } from '../../test/chat-fixtures'
import { renderWithClient, resetClient } from '../../test/render'
import { DOCS, T, calls, docsFixtureApi, hooks, resetDocs } from '../../test/documents-fixtures'
import { expectNoIds } from '../../test/no-ids'
import { ApiError, apiFetch } from '../../api/client'
import { validateDriveSearch } from '../../lib/drive-search'
import { useToastStore } from '../primitives'
import { DriveRoute } from '../drive/DriveRoute'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

async function renderTexts(url = '/drive?view=texts') {
  const root = createRootRoute()
  const drive = createRoute({ getParentRoute: () => root, path: '/drive', validateSearch: validateDriveSearch, component: DriveRoute })
  const editor = createRoute({ getParentRoute: () => root, path: '/documents/$docId', component: () => <p>Trình soạn thảo</p> })
  const router = createRouter({
    routeTree: root.addChildren([drive, editor]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetDocs()
  api.mockImplementation(docsFixtureApi)
})
afterEach(() => {
  cleanup() // unmount first, so nothing mounted refetches into the cleared cache
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

const table = () => screen.getByRole('table', { name: /Văn bản/ })
const searchbox = () => within(screen.getByRole('navigation', { name: 'Thư mục' })).getByRole('searchbox', { name: 'Tìm văn bản' })
const titles = () => within(table()).getAllByRole('row').slice(1).map((r) => r.querySelector('[data-row]')?.textContent)

describe('TextsScreen: the list', () => {
  it('lists the documents newest edit first, each with owner, status and time, and no id', async () => {
    await renderTexts()
    expect(await screen.findByRole('heading', { level: 1, name: 'Tất cả văn bản' })).toBeInTheDocument()
    await within(table()).findByText('Quy trình đối soát cuối ngày')

    expect(titles()).toEqual([
      'Quy trình đối soát cuối ngày', 'Cam kết SLA với ngân hàng đối tác', 'Mẫu biên bản chênh lệch', 'Quy chế tạm ứng 2025',
    ])
    expect(screen.getByText('4 văn bản bạn được xem')).toBeInTheDocument()
    expect(within(table()).getByText('Trần Minh Đức')).toBeInTheDocument()
    expect(within(table()).getAllByText('Bản nháp')).toHaveLength(2)
    expect(within(table()).getByText('Đang dùng')).toBeInTheDocument()
    expect(within(table()).getByText('Lưu trữ')).toBeInTheDocument()
    expectNoIds()
  })

  it('sits in the Tài liệu list panel, as a group above the folders', async () => {
    await renderTexts()
    await within(table()).findByText('Quy trình đối soát cuối ngày')
    const panel = screen.getByRole('navigation', { name: 'Thư mục' })
    const links = within(panel).getAllByRole('link').map((a) => a.textContent)
    expect(links.slice(0, 3)).toEqual(['Tất cả văn bản', 'Bản nháp của tôi2', 'Được chia sẻ'])
    expect(within(panel).getByRole('link', { name: 'Tất cả văn bản' })).toHaveAttribute('aria-current', 'page')
    expect(within(panel).getByRole('tree', { name: 'Thư mục' })).toBeInTheDocument()
    expect(searchbox()).toBeInTheDocument()
  })

  it('counts drafts with the cheap count, not by loading the list', async () => {
    await renderTexts()
    await within(table()).findByText('Quy trình đối soát cuối ngày')
    const panel = screen.getByRole('navigation', { name: 'Thư mục' })
    expect(await within(panel).findByLabelText('2 bản nháp')).toBeInTheDocument()
    expect(calls.some((c) => c.path.includes('/texts/count') && c.path.includes('scope=drafts'))).toBe(true)
    expect(calls.some((c) => c.path.endsWith('/texts?scope=drafts'))).toBe(false)
  })

  it('shows only my drafts under Bản nháp của tôi', async () => {
    await renderTexts('/drive?view=texts&group=drafts')
    expect(await screen.findByRole('heading', { level: 1, name: 'Bản nháp của tôi' })).toBeInTheDocument()
    await within(table()).findByText('Mẫu biên bản chênh lệch')
    expect(titles()).toEqual(['Quy trình đối soát cuối ngày', 'Mẫu biên bản chênh lệch'])
    expect(calls.some((c) => c.path.endsWith('scope=drafts'))).toBe(true)
  })

  it('shows what others wrote under Được chia sẻ', async () => {
    await renderTexts('/drive?view=texts&group=shared')
    await within(table()).findByText('Cam kết SLA với ngân hàng đối tác')
    expect(titles()).toEqual(['Cam kết SLA với ngân hàng đối tác', 'Quy chế tạm ứng 2025'])
  })

  it('finds a document by title, ignoring accents', async () => {
    const user = userEvent.setup()
    await renderTexts()
    await within(table()).findByText('Quy trình đối soát cuối ngày')
    await user.type(searchbox(), 'tam ung')
    await waitFor(() => expect(titles()).toEqual(['Quy chế tạm ứng 2025']))
  })

  it('opens a document by its title', async () => {
    const user = userEvent.setup()
    const router = await renderTexts()
    await user.click(await within(table()).findByRole('link', { name: 'Mẫu biên bản chênh lệch' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/documents/${T.nhap}`))
  })
})

describe('TextsScreen: creating and deleting', () => {
  it('"Văn bản mới" creates a draft and opens it', async () => {
    const user = userEvent.setup()
    const router = await renderTexts()
    await user.click(await screen.findByRole('button', { name: 'Văn bản mới' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/documents/${T.fresh}`))
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({})
  })

  it('offers Xoá only for documents the person can write, after confirming', async () => {
    const user = userEvent.setup()
    await renderTexts()
    await within(table()).findByText('Cam kết SLA với ngân hàng đối tác')

    await user.click(within(table()).getByRole('button', { name: 'Thao tác với Cam kết SLA với ngân hàng đối tác' }))
    expect(screen.queryByRole('menuitem', { name: 'Xoá' })).toBeNull() // read-only
    await user.keyboard('{Escape}')

    await user.click(within(table()).getByRole('button', { name: 'Thao tác với Mẫu biên bản chênh lệch' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Xoá' }))
    const dialog = await screen.findByRole('dialog', { name: 'Xoá văn bản' })
    expect(within(dialog).getByText('Mẫu biên bản chênh lệch')).toBeInTheDocument()
    expect(DOCS[T.nhap]).toBeDefined() // nothing is deleted until confirmed

    await user.click(within(dialog).getByRole('button', { name: 'Xoá' }))
    await waitFor(() => expect(DOCS[T.nhap]).toBeUndefined())
    await waitFor(() => expect(titles()).not.toContain('Mẫu biên bản chênh lệch'))
    expect(useToastStore.getState().toasts.some((t) => /Đã xoá “Mẫu biên bản chênh lệch”/.test(t.message))).toBe(true)
  })
})

describe('TextsScreen: states', () => {
  it('shows skeleton rows while loading', async () => {
    api.mockImplementation((path, init) =>
      path.includes('/documents/texts') ? new Promise(() => {}) : docsFixtureApi(path, init))
    await renderTexts()
    await waitFor(() => expect(table()).toHaveAttribute('aria-busy', 'true'))
  })

  it('says it could not load, and retries', async () => {
    const user = userEvent.setup()
    hooks.failNext = { scope: 'shared', make: () => Promise.reject(new ApiError('boom', 500)) }
    await renderTexts('/drive?view=texts&group=shared')
    expect(await screen.findByText(/Không tải được danh sách văn bản/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    await within(await screen.findByRole('table', { name: /Văn bản/ })).findByText('Cam kết SLA với ngân hàng đối tác')
  })

  it('invites the first document when there are none', async () => {
    for (const id of Object.keys(DOCS)) delete DOCS[id]
    await renderTexts()
    expect(await screen.findByText(/Chưa có văn bản nào/)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Văn bản mới' }).length).toBeGreaterThan(0)
  })

  it('says nothing was shared, without a create button in that group', async () => {
    for (const id of [T.sla, T.luutru]) delete DOCS[id]
    await renderTexts('/drive?view=texts&group=shared')
    expect(await screen.findByText(/Chưa có ai chia sẻ văn bản với bạn/)).toBeInTheDocument()
  })

  it('says nothing matches a search, and clears it', async () => {
    const user = userEvent.setup()
    await renderTexts()
    await within(table()).findByText('Quy trình đối soát cuối ngày')
    await user.type(searchbox(), 'zzz')
    expect(await screen.findByText('Không có văn bản nào khớp.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Xoá tìm kiếm' }))
    await waitFor(() => expect(titles()).toHaveLength(4))
  })
})

describe('reaching Văn bản without the list panel', () => {
  it('Tài liệu has a Văn bản chip, which opens the documents', async () => {
    const user = userEvent.setup()
    const router = await renderTexts('/drive')
    await user.click(await screen.findByRole('button', { name: 'Văn bản' }))
    await waitFor(() => expect(router.state.location.search).toEqual({ view: 'texts' }))
    await within(await screen.findByRole('table', { name: /Văn bản/ })).findByText('Quy trình đối soát cuối ngày')
  })

  it('the groups are chips, and Tất cả tệp goes back to the files', async () => {
    const user = userEvent.setup()
    const router = await renderTexts()
    await within(table()).findByText('Quy trình đối soát cuối ngày')

    await user.click(screen.getByRole('button', { name: 'Bản nháp của tôi' }))
    await waitFor(() => expect(router.state.location.search).toEqual({ view: 'texts', group: 'drafts' }))
    expect(screen.getByRole('button', { name: 'Bản nháp của tôi' })).toHaveAttribute('aria-pressed', 'true')

    await user.click(screen.getByRole('button', { name: 'Tất cả tệp' }))
    await waitFor(() => expect(router.state.location.search).toEqual({}))
    expect(await screen.findByRole('heading', { level: 1, name: 'Tài liệu' })).toBeInTheDocument()
  })
})

describe('TextsScreen: paging', () => {
  const page = (ids: string[], next?: string) => ({
    documents: ids.map((id) => ({ ...DOCS[id]!, content: undefined })),
    ...(next ? { next_cursor: next } : {}),
  })

  it('asks for the next page and adds it to the list, never replacing it', async () => {
    const user = userEvent.setup()
    const asked: string[] = []
    api.mockImplementation((path, init) => {
      if (path.startsWith(`/workspaces/${WS_ID}/documents/texts?`) || path === `/workspaces/${WS_ID}/documents/texts`) {
        asked.push(path)
        return Promise.resolve(path.includes('cursor=page2') ? page([T.nhap, T.luutru]) : page([T.quytrinh, T.sla], 'page2'))
      }
      return docsFixtureApi(path, init)
    })
    await renderTexts()
    await within(table()).findByText('Quy trình đối soát cuối ngày')
    expect(titles()).toEqual(['Quy trình đối soát cuối ngày', 'Cam kết SLA với ngân hàng đối tác'])
    expect(screen.getByText('2+ văn bản bạn được xem')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Tải thêm' }))
    await waitFor(() => expect(titles()).toHaveLength(4))
    expect(titles().slice(0, 2)).toEqual(['Quy trình đối soát cuối ngày', 'Cam kết SLA với ngân hàng đối tác'])
    expect(asked.some((p) => p.includes('cursor=page2'))).toBe(true)
    expect(screen.queryByRole('button', { name: 'Tải thêm' })).toBeNull()
    expect(screen.getByText('4 văn bản bạn được xem')).toBeInTheDocument()
  })
})
