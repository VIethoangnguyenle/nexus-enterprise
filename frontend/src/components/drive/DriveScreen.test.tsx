import { act, screen, waitFor, within, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { D, ITEMS, U, UUID_RE, WS_ID, calls, driveFixtureApi, resetFixtures } from '../../test/drive-fixtures'
import { validateDriveSearch } from '../../lib/drive-search'
import { ApiError, apiFetch } from '../../api/client'
import { driveApi } from '../../api/drive'
import { useToastStore } from '../primitives'
import { useWebSocketStore } from '../../stores/websocket.store'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { DriveScreen } from './DriveScreen'

vi.mock('../../api/client', async (orig) => {
  const { driveFixtureApi } = await import('../../test/drive-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(driveFixtureApi) }
})

const api = vi.mocked(apiFetch)
const ROOT_PATH = `/workspaces/${WS_ID}/drive`

/** The screen mounted on a real in-memory router, so the URL is the state under test. */
async function renderDrive(url = '/drive') {
  const root = createRootRoute()
  const drive = createRoute({
    getParentRoute: () => root, path: '/drive', validateSearch: validateDriveSearch, component: DriveScreen,
  })
  const router = createRouter({
    routeTree: root.addChildren([drive]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  // The router restores scroll on navigation; jsdom has no scrollTo.
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetFixtures()
  calls.length = 0
  api.mockImplementation(driveFixtureApi)
})
afterEach(() => {
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

/** No identifier in anything a person reads: text, names, tooltips, placeholders. */
function noIds() {
  expect(document.body.textContent).not.toMatch(UUID_RE)
  for (const el of document.body.querySelectorAll('[aria-label],[title],[alt],[placeholder]')) {
    for (const attr of ['aria-label', 'title', 'alt', 'placeholder']) {
      expect(el.getAttribute(attr) ?? '').not.toMatch(UUID_RE)
    }
  }
  for (const el of document.body.querySelectorAll('input')) expect(el.value).not.toMatch(UUID_RE)
}

const table = () => screen.getByRole('table')
const rowNames = () =>
  within(table()).getAllByRole('row').slice(1).map((r) => r.querySelector('[data-row]')?.textContent)
const search = (r: Awaited<ReturnType<typeof renderDrive>>) => r.state.location.search as Record<string, unknown>

describe('DriveScreen: listing', () => {
  it('lists the root with folders first, each row naming its owner', async () => {
    await renderDrive()
    expect(await screen.findByRole('heading', { level: 1, name: 'Tài liệu' })).toBeInTheDocument()
    await within(table()).findByText('Đối soát')

    expect(rowNames()).toEqual(['Đối soát', 'Hợp đồng', 'Thư mục trống', 'huong-dan.pdf'])
    const lan = within(table()).getAllByText('Nguyễn Thu Lan')
    expect(lan.length).toBeGreaterThan(0)
    expect(within(table()).getByText('Phạm Hải Yến')).toBeInTheDocument()
    expect(within(table()).getByText('300 KB')).toBeInTheDocument()
    expect(screen.getByText(/2,5 GB/)).toBeInTheDocument()
    noIds()
  })

  it('shows the file kind and size, and sizes only files', async () => {
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('doi-soat-08-10.xlsx')
    expect(within(table()).getByText('412 KB')).toBeInTheDocument()
    expect(within(table()).getByText('1,2 MB')).toBeInTheDocument()
    noIds()
  })

  it('has no file-type chips: the mockup shows only the search', async () => {
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('doi-soat-08-10.xlsx')
    expect(screen.queryByRole('button', { name: 'PDF' })).toBeNull()
  })

  it('puts the search at the top of the list panel, under the title', async () => {
    await renderDrive()
    await within(table()).findByText('Hợp đồng')
    const panel = screen.getByRole('navigation', { name: 'Thư mục' })
    expect(within(panel).getByRole('searchbox', { name: 'Tìm tệp hoặc thư mục' })).toBeInTheDocument()
  })

  it('narrows the folder by name from the search box, ignoring accents', async () => {
    const user = userEvent.setup()
    await renderDrive()
    await within(table()).findByText('Hợp đồng')
    await user.type(
      within(screen.getByRole('navigation', { name: 'Thư mục' })).getByRole('searchbox', { name: 'Tìm tệp hoặc thư mục' }),
      'hop dong',
    )
    await waitFor(() => expect(rowNames()).toEqual(['Hợp đồng']))
  })

  it('shows what was shared with me, with the owner the server named', async () => {
    const user = userEvent.setup()
    await renderDrive()
    await user.click(await screen.findByRole('link', { name: 'Được chia sẻ với tôi' }))
    await within(table()).findByText('ke-hoach-quy-4.xlsx')
    expect(within(table()).getByText('Phạm Văn Ngoài')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: 'Được chia sẻ với tôi' })).toBeInTheDocument()
    noIds()
  })
})

describe('DriveScreen: the folder lives in the URL', () => {
  it('opens the folder named in the URL, so a reload lands in place', async () => {
    await renderDrive(`/drive?folder=${D.y2026}`)
    expect(await screen.findByRole('heading', { level: 1, name: '2026' })).toBeInTheDocument()
    await within(table()).findByText('Tháng 10')

    const crumbs = screen.getByRole('navigation', { name: 'Đường dẫn' })
    expect(within(crumbs).getAllByRole('link').map((a) => a.textContent)).toEqual(['Khối Vận hành', 'Đối soát'])
    expect(within(crumbs).getByText('2026')).toHaveAttribute('aria-current', 'page')
    noIds()
  })

  it('opening a folder changes the URL, and Back returns to the folder before', async () => {
    const user = userEvent.setup()
    const router = await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('doi-soat-08-10.xlsx')

    await user.click(within(table()).getByRole('link', { name: '2026' }))
    await screen.findByRole('heading', { level: 1, name: '2026' })
    expect(search(router).folder).toBe(D.y2026)
    expect(rowNames()).toEqual(['Tháng 10'])

    await act(async () => { router.history.back() })
    await screen.findByRole('heading', { level: 1, name: 'Đối soát' })
    expect(search(router).folder).toBe(D.doisoat)
    await within(table()).findByText('doi-soat-08-10.xlsx')

    await act(async () => { router.history.forward() })
    await screen.findByRole('heading', { level: 1, name: '2026' })
  })

  it('goes up through the breadcrumb, and the first crumb is the root', async () => {
    const user = userEvent.setup()
    const router = await renderDrive(`/drive?folder=${D.thang10}`)
    const crumbs = await screen.findByRole('navigation', { name: 'Đường dẫn' })
    await within(crumbs).findByRole('link', { name: '2026' })

    await user.click(within(crumbs).getByRole('link', { name: 'Đối soát' }))
    await screen.findByRole('heading', { level: 1, name: 'Đối soát' })
    expect(search(router).folder).toBe(D.doisoat)

    await user.click(within(screen.getByRole('navigation', { name: 'Đường dẫn' })).getByRole('link', { name: 'Khối Vận hành' }))
    await screen.findByRole('heading', { level: 1, name: 'Tài liệu' })
    expect(search(router).folder).toBeUndefined()
  })

  it('keeps the workspace in the URL while moving between folders', async () => {
    const user = userEvent.setup()
    const router = await renderDrive(`/drive?ws=${WS_ID}`)
    await user.click(await within(table()).findByRole('link', { name: 'Đối soát' }))
    await screen.findByRole('heading', { level: 1, name: 'Đối soát' })
    expect(search(router)).toEqual({ ws: WS_ID, folder: D.doisoat })
  })

  it('a folder that no longer exists says so and offers the root', async () => {
    const user = userEvent.setup()
    const router = await renderDrive('/drive?folder=66666666-aaaa-4bbb-8ccc-00000000ffff')
    expect(await screen.findByText(/Không mở được thư mục này/)).toBeInTheDocument()
    await user.click(screen.getByRole('link', { name: 'Về thư mục gốc' }))
    await screen.findByRole('heading', { level: 1, name: 'Tài liệu' })
    expect(search(router).folder).toBeUndefined()
  })

  it('opens a folder from the tree and marks it current', async () => {
    const user = userEvent.setup()
    const router = await renderDrive()
    const tree = await screen.findByRole('tree', { name: 'Thư mục' })
    await user.click(await within(tree).findByRole('treeitem', { name: 'Hợp đồng' }))
    await screen.findByRole('heading', { level: 1, name: 'Hợp đồng' })
    expect(search(router).folder).toBe(D.hopdong)
    expect(within(tree).getByRole('treeitem', { name: 'Hợp đồng' })).toHaveAttribute('aria-current', 'page')
  })

  it('Enter on a folder row opens it, arrows move between rows', async () => {
    const user = userEvent.setup()
    const router = await renderDrive()
    await within(table()).findByText('Đối soát')
    within(table()).getByRole('link', { name: 'Đối soát' }).focus()
    await user.keyboard('{ArrowDown}')
    expect(within(table()).getByRole('link', { name: 'Hợp đồng' })).toHaveFocus()
    await user.keyboard('{ArrowUp}{Enter}')
    await screen.findByRole('heading', { level: 1, name: 'Đối soát' })
    expect(search(router).folder).toBe(D.doisoat)
  })
})

describe('DriveScreen: loading, empty and error', () => {
  it('shows row-shaped placeholders while the folder loads', async () => {
    api.mockImplementation((path, init) =>
      path === ROOT_PATH ? new Promise(() => {}) : driveFixtureApi(path, init))
    await renderDrive()
    const t = await screen.findByRole('table')
    expect(t).toHaveAttribute('aria-busy', 'true')
    expect(t.querySelectorAll('.skeleton').length).toBeGreaterThan(0)
    expect(screen.queryByText(/Thư mục này chưa có tệp/)).toBeNull()
  })

  it('says an empty folder is empty and offers the upload', async () => {
    await renderDrive(`/drive?folder=${D.trong}`)
    expect(await screen.findByText(/Thư mục này chưa có tệp/)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Tải lên' }).length).toBeGreaterThan(0)
    noIds()
  })

  it('says a failed list failed and retries on request', async () => {
    const user = userEvent.setup()
    let fail = true
    api.mockImplementation((path, init) =>
      path === ROOT_PATH && fail ? Promise.reject(new Error('boom')) : driveFixtureApi(path, init))
    await renderDrive()
    expect(await screen.findByText(/Không tải được danh sách tệp/)).toBeInTheDocument()
    fail = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    await within(await screen.findByRole('table')).findByText('Đối soát')
    expect(screen.queryByText(/Không tải được danh sách tệp/)).toBeNull()
  })
})

describe('DriveScreen: detail panel', () => {
  async function openXlsx(user: ReturnType<typeof userEvent.setup>) {
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await user.click(await within(table()).findByRole('button', { name: 'doi-soat-08-10.xlsx' }))
    return screen.findByRole('complementary', { name: 'Chi tiết tệp' })
  }

  it('names the file, its owner and who it is shared with, with no ids', async () => {
    const user = userEvent.setup()
    const panel = await openXlsx(user)
    expect(within(panel).getByRole('heading', { name: 'doi-soat-08-10.xlsx' })).toBeInTheDocument()
    expect(within(panel).getByText('Bảng tính · 412 KB')).toBeInTheDocument()

    const people = await within(panel).findByRole('list', { name: 'Người có quyền' })
    await within(people).findByText('Nguyễn Thu Lan')
    expect(within(people).getByText('Trần Minh Đức')).toBeInTheDocument()
    expect(within(people).getByText('chủ sở hữu')).toBeInTheDocument()
    expect(within(people).getByText('Lê Thị Hoa')).toBeInTheDocument()
    expect(within(people).getByRole('combobox', { name: 'Quyền của Nguyễn Thu Lan' })).toHaveValue('write')
    expect(within(people).getByRole('combobox', { name: 'Quyền của Lê Thị Hoa' })).toHaveValue('read')
    noIds()
  })

  it('records who created it and when, as an activity entry', async () => {
    const user = userEvent.setup()
    const panel = await openXlsx(user)
    const acts = within(panel).getByRole('list', { name: 'Hoạt động' })
    expect(within(acts).getByText('Trần Minh Đức')).toBeInTheDocument()
    expect(within(acts).getByText(/đã tải lên/)).toBeInTheDocument()
  })

  it('closes with Escape and with the close button', async () => {
    const user = userEvent.setup()
    await openXlsx(user)
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Chi tiết tệp' })).toBeNull())

    await user.click(within(table()).getByRole('button', { name: 'doi-soat-08-10.xlsx' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tệp' })
    await user.click(within(panel).getByRole('button', { name: 'Đóng chi tiết' }))
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Chi tiết tệp' })).toBeNull())
  })

  it('offers no sharing and no edits on a file I can only read', async () => {
    const user = userEvent.setup()
    await renderDrive()
    await user.click(await within(table()).findByRole('button', { name: 'huong-dan.pdf' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tệp' })
    expect(await within(panel).findByRole('list', { name: 'Người có quyền' })).toHaveTextContent('Phạm Hải Yến')
    expect(within(panel).queryByRole('button', { name: 'Chia sẻ' })).toBeNull()
    expect(calls.some((c) => c.path.endsWith('/shares'))).toBe(false)

    await user.click(within(table()).getByRole('button', { name: 'Thao tác với huong-dan.pdf' }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getByRole('menuitem', { name: 'Tải xuống' })).toBeInTheDocument()
    expect(within(menu).queryByRole('menuitem', { name: 'Xoá' })).toBeNull()
    expect(within(menu).queryByRole('menuitem', { name: 'Đổi tên' })).toBeNull()
  })

  it('closes when the user leaves the folder', async () => {
    const user = userEvent.setup()
    const router = await renderDrive(`/drive?folder=${D.doisoat}`)
    await user.click(await within(table()).findByRole('button', { name: 'doi-soat-08-10.xlsx' }))
    await screen.findByRole('complementary', { name: 'Chi tiết tệp' })
    await act(async () => { await router.navigate({ to: '/drive', search: {} }) })
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Chi tiết tệp' })).toBeNull())
  })
})

describe('DriveScreen: sharing', () => {
  it('shares with a person picked by name, sending the node id the server expects', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await user.click(await within(table()).findByRole('button', { name: 'doi-soat-08-10.xlsx' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tệp' })
    await user.click(await within(panel).findByRole('button', { name: 'Chia sẻ' }))

    const dialog = await screen.findByRole('dialog', { name: 'Chia sẻ “doi-soat-08-10.xlsx”' })
    await user.type(within(dialog).getByRole('combobox', { name: 'Thêm người' }), 'hai yen')
    await user.click(await screen.findByRole('option', { name: /Phạm Hải Yến/ }))
    await user.selectOptions(within(dialog).getByRole('combobox', { name: 'Quyền' }), 'write')
    noIds()
    await user.click(within(dialog).getByRole('button', { name: 'Chia sẻ' }))

    await waitFor(() => expect(calls.find((c) => c.path === `/drive/items/${D.xlsx}/share`)).toBeTruthy())
    expect(calls.find((c) => c.path === `/drive/items/${D.xlsx}/share`)?.body).toEqual({
      share_type: 'user', target_node_id: U.yen.node, permission: 'write',
    })
  })

  it('lists current shares by name in the same dialog and revokes one after confirming', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await user.click(await within(table()).findByRole('button', { name: 'doi-soat-08-10.xlsx' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tệp' })
    await user.click(await within(panel).findByRole('button', { name: 'Chia sẻ' }))

    const dialog = await screen.findByRole('dialog', { name: 'Chia sẻ “doi-soat-08-10.xlsx”' })
    const current = await within(dialog).findByRole('list', { name: 'Đang chia sẻ với' })
    expect(await within(current).findByText('Nguyễn Thu Lan')).toBeInTheDocument()
    noIds()

    await user.click(within(current).getByRole('button', { name: 'Bỏ quyền của Nguyễn Thu Lan' }))
    const confirm = await screen.findByRole('dialog', { name: 'Bỏ quyền truy cập' })
    await user.click(within(confirm).getByRole('button', { name: 'Bỏ quyền' }))

    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE' && c.path.startsWith('/drive/shares/'))).toBe(true))
  })
})

describe('DriveScreen: Escape', () => {
  const openShare = async (user: ReturnType<typeof userEvent.setup>) => {
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await user.click(await within(table()).findByRole('button', { name: 'doi-soat-08-10.xlsx' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tệp' })
    await user.click(await within(panel).findByRole('button', { name: 'Chia sẻ' }))
    return screen.findByRole('dialog', { name: 'Chia sẻ “doi-soat-08-10.xlsx”' })
  }

  it('closes only the share dialog, leaving the detail panel open', async () => {
    const user = userEvent.setup()
    await openShare(user)
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('complementary', { name: 'Chi tiết tệp' })).toBeInTheDocument()
    // The next Escape is the panel's.
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Chi tiết tệp' })).toBeNull())
  })

  it('closes only a row menu, leaving the detail panel open', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await user.click(await within(table()).findByRole('button', { name: 'doi-soat-08-10.xlsx' }))
    await screen.findByRole('complementary', { name: 'Chi tiết tệp' })
    await user.click(within(table()).getByRole('button', { name: 'Thao tác với sao-ke-ngan-hang-10-2026.pdf' }))
    await screen.findByRole('menu')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
    expect(screen.getByRole('complementary', { name: 'Chi tiết tệp' })).toBeInTheDocument()
  })
})

describe('DriveScreen: detail panel sharing', () => {
  it('changes a person\'s permission inline with the existing share calls', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await user.click(await within(table()).findByRole('button', { name: 'doi-soat-08-10.xlsx' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tệp' })
    const select = await within(panel).findByRole('combobox', { name: 'Quyền của Lê Thị Hoa' })
    await user.selectOptions(select, 'write')

    await waitFor(() => expect(calls.find((c) => c.path === `/drive/items/${D.xlsx}/share`)).toBeTruthy())
    const del = calls.findIndex((c) => c.method === 'DELETE' && c.path === '/drive/shares/88888888-aaaa-4bbb-8ccc-000000000002')
    const add = calls.findIndex((c) => c.path === `/drive/items/${D.xlsx}/share`)
    expect(del).toBeGreaterThan(-1)
    expect(del).toBeLessThan(add)
    expect(calls[add]?.body).toEqual({ share_type: 'user', target_node_id: U.hoa.node, permission: 'write' })
  })

  it('puts the old permission back when the new one cannot be granted', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await user.click(await within(table()).findByRole('button', { name: 'doi-soat-08-10.xlsx' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tệp' })
    const select = await within(panel).findByRole('combobox', { name: 'Quyền của Lê Thị Hoa' })
    let n = 0
    api.mockImplementation((path, init) => {
      if (path === `/drive/items/${D.xlsx}/share` && n++ === 0) {
        calls.push({ method: 'POST', path })
        return Promise.reject(new Error('denied'))
      }
      return driveFixtureApi(path, init)
    })
    await user.selectOptions(select, 'write')
    await waitFor(() => expect(calls.filter((c) => c.path === `/drive/items/${D.xlsx}/share`).length).toBe(2))
    const restore = calls.filter((c) => c.path === `/drive/items/${D.xlsx}/share`)[1]
    expect(restore?.body).toEqual({ share_type: 'user', target_node_id: U.hoa.node, permission: 'read' })
  })
})

describe('DriveScreen: a folder of another workspace', () => {
  const FOREIGN = '66666666-aaaa-4bbb-8ccc-0000000000f1'
  beforeEach(() => {
    ITEMS[FOREIGN] = { ...ITEMS[D.trong]!, id: FOREIGN, name: 'Của nơi khác', workspace_id: 'another-workspace', parent_id: 'elsewhere' }
  })
  afterEach(() => { delete ITEMS[FOREIGN] })

  it('is treated as missing, with no upload into it', async () => {
    await renderDrive(`/drive?folder=${FOREIGN}`)
    expect(await screen.findByText(/Không mở được thư mục này/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Tải lên' })).toBeNull()
    expect(screen.queryByText('Của nơi khác')).toBeNull()
    fireEvent.drop(screen.getByTestId('drive-dropzone'), {
      dataTransfer: { files: [new File(['a'], 'a.pdf')], types: ['Files'] },
    })
    expect(calls.some((c) => c.path.endsWith('/drive/files'))).toBe(false)
  })
})

describe('DriveScreen: connection', () => {
  afterEach(() => {
    vi.useRealTimers()
    useWebSocketStore.setState({ connected: false, reconnectAttempt: 0 })
  })

  it('says nothing while connected, and says it only after 3 seconds without a connection', async () => {
    await renderDrive()
    await within(table()).findByText('Hợp đồng')
    vi.useFakeTimers({ shouldAdvanceTime: true })
    expect(screen.queryByText(/Đang kết nối lại/)).toBeNull()
    act(() => { useWebSocketStore.setState({ connected: false, reconnectAttempt: 2 }) })
    act(() => { vi.advanceTimersByTime(2000) })
    expect(screen.queryByText(/Đang kết nối lại/)).toBeNull()
    act(() => { vi.advanceTimersByTime(1500) })
    expect(screen.getByText(/Đang kết nối lại/)).toBeInTheDocument()
    act(() => { useWebSocketStore.setState({ connected: true, reconnectAttempt: 0 }) })
    expect(screen.queryByText(/Đang kết nối lại/)).toBeNull()
  })
})

describe('DriveScreen: realtime', () => {
  const later = { seconds: Math.floor(new Date(2026, 9, 10, 10, 30).getTime() / 1000), nanos: 0 }
  const original = structuredClone(ITEMS)
  afterEach(() => {
    for (const id of Object.keys(ITEMS)) ITEMS[id] = structuredClone(original[id]!)
  })

  const rowOf = (name: string) => within(table()).getByText(name).closest('[role="row"]') as HTMLElement
  const refetch = () => act(() => queryClient.invalidateQueries({ queryKey: keys.drive.all(WS_ID) }))

  it('washes a row in the owner colour, with a label, when someone else changes it', async () => {
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('doi-soat-08-10.xlsx')
    expect(rowOf('doi-soat-08-10.xlsx')).not.toHaveClass('rt-wash')

    ITEMS[D.xlsx]!.name = 'doi-soat-10-10.xlsx'
    ITEMS[D.xlsx]!.updated_at = later
    await refetch()

    const row = await waitFor(() => {
      const r = rowOf('doi-soat-10-10.xlsx')
      expect(r).toHaveClass('rt-wash')
      return r
    })
    expect(within(row).getByText('vừa cập nhật')).toBeInTheDocument()
    expect(row.style.getPropertyValue('--pw')).toMatch(/^var\(--color-person-[1-8]-wash\)$/)
    // Rows nobody touched stay as they were.
    expect(rowOf('sao-ke-ngan-hang-10-2026.pdf')).not.toHaveClass('rt-wash')
    noIds()
  })

  it('does not wash a change I made myself', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('sao-ke-ngan-hang-10-2026.pdf')
    const menu = await (async () => {
      await user.click(within(table()).getByRole('button', { name: 'Thao tác với sao-ke-ngan-hang-10-2026.pdf' }))
      return screen.findByRole('menu')
    })()
    await user.click(within(menu).getByRole('menuitem', { name: 'Đổi tên' }))
    const dialog = await screen.findByRole('dialog', { name: 'Đổi tên' })
    const field = within(dialog).getByRole('textbox', { name: 'Tên' })
    await user.clear(field)
    await user.type(field, 'sao-ke-t10.pdf')

    // The server applies the rename; the file belongs to someone else (Yến).
    ITEMS[D.sakePdf]!.name = 'sao-ke-t10.pdf'
    ITEMS[D.sakePdf]!.updated_at = later
    await user.click(within(dialog).getByRole('button', { name: 'Đổi tên' }))

    await within(table()).findByText('sao-ke-t10.pdf')
    expect(rowOf('sao-ke-t10.pdf')).not.toHaveClass('rt-wash')
  })

  it('sums up three changes by others in one line instead of three washes', async () => {
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('doi-soat-08-10.xlsx')

    for (const id of [D.xlsx, D.sakePdf, D.bienban]) ITEMS[id]!.updated_at = later
    await refetch()

    const summary = await screen.findByRole('status')
    expect(summary).toHaveTextContent(/và 2 người khác vừa cập nhật thư mục này/)
    noIds()
  })
})

describe('DriveScreen: actions', () => {
  async function openMenu(user: ReturnType<typeof userEvent.setup>, name: string) {
    await user.click(await within(table()).findByRole('button', { name: `Thao tác với ${name}` }))
    return screen.findByRole('menu')
  }

  it('deletes after confirming, then offers to undo', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('bien-ban-chenh-lech.docx')
    const menu = await openMenu(user, 'bien-ban-chenh-lech.docx')
    await user.click(within(menu).getByRole('menuitem', { name: 'Xoá' }))

    const dialog = await screen.findByRole('dialog', { name: 'Xoá tệp' })
    expect(dialog).toHaveTextContent('bien-ban-chenh-lech.docx')
    await user.click(within(dialog).getByRole('button', { name: 'Xoá' }))

    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE' && c.path === `/drive/items/${D.bienban}`)).toBe(true))
    await waitFor(() => expect(within(table()).queryByText('bien-ban-chenh-lech.docx')).toBeNull())

    const toast = useToastStore.getState().toasts.slice(-1)[0]
    expect(toast?.message).toBe('Đã xoá “bien-ban-chenh-lech.docx”')
    expect(toast?.action?.label).toBe('Hoàn tác')
    await act(async () => { toast?.action?.onClick() })
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === `/drive/items/${D.bienban}/restore`)).toBe(true))
  })

  it('deleting the last row ends on the empty state', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.hopdong}`)
    await within(table()).findByText('hop-dong-doi-tac.pdf')
    const menu = await openMenu(user, 'hop-dong-doi-tac.pdf')
    await user.click(within(menu).getByRole('menuitem', { name: 'Xoá' }))
    const dialog = await screen.findByRole('dialog', { name: 'Xoá tệp' })
    await user.click(within(dialog).getByRole('button', { name: 'Xoá' }))
    expect(await screen.findByText(/Thư mục này chưa có tệp/)).toBeInTheDocument()
  })

  it('renames through a dialog, sending the field the server reads', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('sao-ke-ngan-hang-10-2026.pdf')
    const menu = await openMenu(user, 'sao-ke-ngan-hang-10-2026.pdf')
    await user.click(within(menu).getByRole('menuitem', { name: 'Đổi tên' }))

    const dialog = await screen.findByRole('dialog', { name: 'Đổi tên' })
    const field = within(dialog).getByRole('textbox', { name: 'Tên' })
    expect(field).toHaveValue('sao-ke-ngan-hang-10-2026.pdf')
    await user.clear(field)
    await user.type(field, 'sao-ke-t10.pdf')
    await user.click(within(dialog).getByRole('button', { name: 'Đổi tên' }))

    await waitFor(() => expect(calls.find((c) => c.path === `/drive/items/${D.sakePdf}/rename`)).toBeTruthy())
    expect(calls.find((c) => c.path === `/drive/items/${D.sakePdf}/rename`)?.body).toEqual({ name: 'sao-ke-t10.pdf' })
  })

  it('creates a folder inside the open folder, and rejects an empty name', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await user.click(await screen.findByRole('button', { name: 'Thư mục mới' }))
    const dialog = await screen.findByRole('dialog', { name: 'Thư mục mới' })

    await user.click(within(dialog).getByRole('button', { name: 'Tạo' }))
    expect(await within(dialog).findByText('Đặt tên cho thư mục')).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'POST' && c.path.endsWith('/drive/folders'))).toBe(false)

    await user.type(within(dialog).getByRole('textbox', { name: 'Tên' }), 'Chứng từ gốc')
    await user.click(within(dialog).getByRole('button', { name: 'Tạo' }))
    await waitFor(() => expect(calls.find((c) => c.path === `/workspaces/${WS_ID}/drive/folders`)).toBeTruthy())
    expect(calls.find((c) => c.path === `/workspaces/${WS_ID}/drive/folders`)?.body).toEqual({
      name: 'Chứng từ gốc', parent_id: D.doisoat,
    })
  })

  it('moves an item to a folder chosen in the tree', async () => {
    const user = userEvent.setup()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('bien-ban-chenh-lech.docx')
    const menu = await openMenu(user, 'bien-ban-chenh-lech.docx')
    await user.click(within(menu).getByRole('menuitem', { name: 'Di chuyển' }))

    const dialog = await screen.findByRole('dialog', { name: 'Di chuyển tới' })
    const move = within(dialog).getByRole('button', { name: 'Di chuyển vào đây' })
    expect(move).toBeDisabled()
    await user.click(await within(dialog).findByRole('treeitem', { name: 'Hợp đồng' }))
    expect(move).toBeEnabled()
    noIds()
    await user.click(move)

    await waitFor(() => expect(calls.find((c) => c.path === `/drive/items/${D.bienban}/move`)).toBeTruthy())
    expect(calls.find((c) => c.path === `/drive/items/${D.bienban}/move`)?.body).toEqual({ target_folder_id: D.hopdong })
  })

  it('uploads each chosen file in the open folder', async () => {
    const user = userEvent.setup()
    const put = vi.spyOn(driveApi, 'uploadToStorage').mockResolvedValue()
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await screen.findByRole('heading', { level: 1, name: 'Đối soát' })

    const file = new File(['x'.repeat(10)], 'chung-tu.pdf', { type: 'application/pdf' })
    await user.upload(screen.getByLabelText('Chọn tệp để tải lên'), file)

    await waitFor(() => expect(calls.some((c) => c.path === '/drive/files/new-file/confirm')).toBe(true))
    expect(calls.find((c) => c.path === `/workspaces/${WS_ID}/drive/files`)?.body).toEqual({
      name: 'chung-tu.pdf', mime_type: 'application/pdf', size_bytes: 10, parent_id: D.doisoat,
    })
    expect(put).toHaveBeenCalledWith('https://storage.test/put', file)
    put.mockRestore()
  })

  it('says the workspace storage is full when an upload is refused for quota', async () => {
    const user = userEvent.setup()
    api.mockImplementation(async (path: string, init?: RequestInit) => {
      if (path === `/workspaces/${WS_ID}/drive/files` && init?.method === 'POST') {
        throw new ApiError('quota', 413, { reason: 'quota_exceeded' })
      }
      return driveFixtureApi(path, init)
    })
    await renderDrive()
    await within(table()).findByText('Đối soát')
    const file = new File(['abc'], 'lon.pdf', { type: 'application/pdf' })
    await user.upload(screen.getByLabelText('Chọn tệp để tải lên'), file)
    await waitFor(() => expect(useToastStore.getState().toasts.length).toBeGreaterThan(0))
    const text = useToastStore.getState().toasts.map((t) => t.message).join(' ')
    expect(text).toContain('kho tài liệu của workspace đã đầy')
    expect(text).not.toContain('kết nối mạng')
  })

  it('accepts files dropped on the list', async () => {
    const put = vi.spyOn(driveApi, 'uploadToStorage').mockResolvedValue()
    await renderDrive()
    await within(table()).findByText('Đối soát')
    const file = new File(['abc'], 'ban-ve.pdf', { type: 'application/pdf' })
    fireEvent.drop(screen.getByTestId('drive-dropzone'), { dataTransfer: { files: [file], types: ['Files'] } })
    await waitFor(() => expect(calls.some((c) => c.path === '/drive/files/new-file/confirm')).toBe(true))
    expect(put).toHaveBeenCalledTimes(1)
    put.mockRestore()
  })

  it('downloads from the menu with a fresh link', async () => {
    const user = userEvent.setup()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    await renderDrive(`/drive?folder=${D.doisoat}`)
    await within(table()).findByText('doi-soat-08-10.xlsx')
    const menu = await openMenu(user, 'doi-soat-08-10.xlsx')
    await user.click(within(menu).getByRole('menuitem', { name: 'Tải xuống' }))
    await waitFor(() => expect(click).toHaveBeenCalled())
    expect(calls.some((c) => c.path === `/drive/files/${D.xlsx}/download`)).toBe(true)
    click.mockRestore()
  })
})

describe('fixtures', () => {
  it('cover a folder whose owner is a node id and a file whose owner is a user id', () => {
    expect(ITEMS[D.doisoat]!.owner_id).toBe(U.lan.node)
    expect(ITEMS[D.xlsx]!.owner_id).toBe(U.duc.id)
  })
})
