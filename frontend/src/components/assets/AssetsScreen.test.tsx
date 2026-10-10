import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import {
  A, ACTIVITY, ASSETS, R, REQUESTS, T, TYPES, U, UUID_RE, WS_ID, assetFixtureApi, calls, mode, resetFixtures,
} from '../../test/asset-fixtures'
import { validateAssetsSearch } from '../../lib/assets-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { AssetsScreen } from './AssetsScreen'

vi.mock('../../api/client', async (orig) => {
  const { assetFixtureApi } = await import('../../test/asset-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(assetFixtureApi) }
})

const api = vi.mocked(apiFetch)

/** The screen mounted on a real in-memory router, so the URL is the state under test. */
async function renderAssets(url = '/assets') {
  const root = createRootRoute()
  const route = createRoute({
    getParentRoute: () => root, path: '/assets', validateSearch: validateAssetsSearch, component: AssetsScreen,
  })
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetFixtures()
  api.mockImplementation(assetFixtureApi)
})
afterEach(() => {
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

/** No identifier in anything a person reads: text, names, tooltips, placeholders, values. */
function noIds() {
  expect(document.body.textContent).not.toMatch(UUID_RE)
  for (const el of document.body.querySelectorAll('[aria-label],[title],[alt],[placeholder]')) {
    for (const attr of ['aria-label', 'title', 'alt', 'placeholder']) {
      expect(el.getAttribute(attr) ?? '').not.toMatch(UUID_RE)
    }
  }
  for (const el of document.body.querySelectorAll('input,textarea')) {
    expect((el as HTMLInputElement).value).not.toMatch(UUID_RE)
  }
  for (const el of document.body.querySelectorAll('select')) {
    expect(el.selectedOptions[0]?.textContent ?? '').not.toMatch(UUID_RE)
  }
}
/** And none of the internal words the backend speaks. */
function noCodes() {
  const text = document.body.textContent ?? ''
  for (const code of ['in_use', 'flag_maintenance', 'complete_maintenance', 'fulfilled', 'requested', 'assigned', 'x-kind', 'ngac']) {
    expect(text).not.toContain(code)
  }
}

const table = (name: string) => screen.getByRole('table', { name })
const rowNames = (name: string) =>
  within(table(name)).getAllByRole('row').slice(1).map((r) => r.querySelector('[data-row]')?.textContent)
const search = (r: Awaited<ReturnType<typeof renderAssets>>) => r.state.location.search as Record<string, unknown>
const toasts = () => useToastStore.getState().toasts.map((t) => t.message)
const posts = (path: string) => calls.filter((c) => c.method === 'POST' && c.path === path)
const MAC7 = ASSETS[A.mac7]!.name

describe('AssetsScreen: overview', () => {
  it('shows the four figures, the spread, who is waiting on me and the recent activity with who did it', async () => {
    await renderAssets()
    expect(await screen.findByRole('heading', { level: 1, name: 'Tài sản' })).toBeInTheDocument()
    const total = await screen.findByRole('link', { name: /Tổng tài sản/ })
    await within(total).findByText('9')
    expect(total).toHaveTextContent('4 loại')
    expect(screen.getByRole('link', { name: /Đang giao/ })).toHaveTextContent('2')
    expect(screen.getByRole('link', { name: /Đang giao/ })).toHaveTextContent('cho 2 người')
    expect(screen.getByRole('link', { name: /Đang bảo trì/ })).toHaveTextContent('1')
    expect(screen.getByRole('link', { name: /Đang bảo trì/ })).toHaveTextContent('1 quá 14 ngày')
    expect(await screen.findByRole('link', { name: /Yêu cầu chờ duyệt/ })).toHaveTextContent('3')
    expect(screen.getByRole('link', { name: /Yêu cầu chờ duyệt/ })).toHaveTextContent('1 khẩn')

    // The spread names the states the backend has.
    const spread = screen.getByRole('region', { name: 'Phân bố tài sản' })
    expect(within(spread).getByText('Sẵn sàng')).toBeInTheDocument()
    expect(within(spread).getByText('Đang giao')).toBeInTheDocument()
    expect(within(spread).getByRole('img').getAttribute('aria-label')).toMatch(/2 đang giao/)

    const waiting = screen.getByRole('region', { name: 'Chờ bạn duyệt' })
    expect(within(waiting).getByText('Phạm Hải Yến · 08:52')).toBeInTheDocument()
    expect(within(waiting).getByText('Khẩn')).toBeInTheDocument()

    const feed = await screen.findByRole('list', { name: 'Hoạt động gần đây' })
    expect(within(feed).getAllByText('Trần Minh Đức').length).toBeGreaterThan(0)
    expect(feed).toHaveTextContent('đã thu hồi MacBook Pro 14 inch, máy số 4 từ Lê Quang Vinh')
    expect(feed).toHaveTextContent('đã giao MacBook Pro 14 inch, máy số 7 cho Nguyễn Thu Lan')
    expect(feed).toHaveTextContent('đã đưa Máy in HP LaserJet tầng 3 đi bảo trì')
    // A decision on a request is in the feed too.
    expect(feed).toHaveTextContent('Lê Thị Hoa đã từ chối yêu cầu Màn hình của Trần Minh Đức')
    noIds()
    noCodes()
  })

  it('keeps the figures as skeletons until the data is in, never a "0"', async () => {
    mode.hang = true
    await renderAssets()
    await screen.findByRole('heading', { level: 1, name: 'Tài sản' })
    expect(await screen.findAllByLabelText(/Đang tải/)).not.toHaveLength(0)
    const total = screen.getByRole('link', { name: /Tổng tài sản/ })
    expect(total).not.toHaveTextContent('0')
    expect(within(total).getByLabelText('Đang tải tổng tài sản')).toBeInTheDocument()
  })

  it('says there is nothing yet, and offers a new type only to whoever may add one', async () => {
    mode.empty = true
    await renderAssets()
    expect(await screen.findByText(/Chưa có tài sản nào\. Thêm loại tài sản/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Thêm loại tài sản' })).toBeInTheDocument()
  })

  it('tells a person who may not manage types that there is nothing yet, without offering to add', async () => {
    mode.empty = true
    mode.member = true
    await renderAssets()
    expect(await screen.findByText('Chưa có tài sản nào trong phạm vi bạn xem được.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Thêm loại tài sản' })).toBeNull()
  })

  it('offers a retry when the summary cannot be read', async () => {
    mode.listError = true
    const user = userEvent.setup()
    await renderAssets()
    expect(await screen.findByText(/Không tải được tổng quan tài sản/)).toBeInTheDocument()
    mode.listError = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByRole('link', { name: /Tổng tài sản/ })).toHaveTextContent('9')
  })

  it('shows requests waiting on me only to a person who may decide them', async () => {
    mode.member = true
    await renderAssets()
    await screen.findByRole('link', { name: /Tổng tài sản/ })
    expect(screen.queryByRole('region', { name: 'Chờ bạn duyệt' })).toBeNull()
    expect(screen.getByRole('tab', { name: 'Yêu cầu' })).not.toHaveTextContent(/\d/)
  })

  it('counts the requests waiting on me on the tab', async () => {
    await renderAssets()
    await waitFor(() => expect(screen.getByRole('tab', { name: /Yêu cầu/ })).toHaveTextContent('2'))
  })

  it('each figure leads to the matching list through the URL', async () => {
    const user = userEvent.setup()
    const router = await renderAssets()
    await user.click(await screen.findByRole('link', { name: /Đang giao/ }))
    await waitFor(() => expect(search(router)).toMatchObject({ section: 'list', state: 'assigned' }))
    await within(table('Tài sản')).findByText(MAC7)
    expect(rowNames('Tài sản')).toEqual([MAC7, 'Màn hình Dell 27 inch, bàn 12'])
  })
})

describe('AssetsScreen: tabs', () => {
  it('moves between the four tabs through the URL', async () => {
    const user = userEvent.setup()
    const router = await renderAssets()
    await screen.findByRole('link', { name: /Tổng tài sản/ })
    await user.click(screen.getByRole('tab', { name: 'Danh sách' }))
    await within(table('Tài sản')).findByText(MAC7)
    expect(search(router).section).toBe('list')
    await user.click(screen.getByRole('tab', { name: /Yêu cầu/ }))
    await within(table('Yêu cầu tài sản')).findAllByText('Laptop')
    expect(search(router).section).toBe('requests')
    await user.click(screen.getByRole('tab', { name: 'Loại tài sản' }))
    await within(table('Loại tài sản')).findByText('Laptop')
    expect(search(router).section).toBe('types')
    await user.click(screen.getByRole('tab', { name: 'Tổng quan' }))
    await screen.findByRole('link', { name: /Tổng tài sản/ })
    expect(search(router).section).toBeUndefined()
  })

  it('opens on the tab named in the URL, so a reload lands in place', async () => {
    await renderAssets('/assets?section=types')
    expect(screen.getByRole('tab', { name: 'Loại tài sản' })).toHaveAttribute('aria-selected', 'true')
    await within(table('Loại tài sản')).findByText('Giấy phép phần mềm')
  })
})

describe('AssetsScreen: list', () => {
  const openMac7 = async () => {
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=list')
    await within(table('Tài sản')).findByText(MAC7)
    await user.click(within(table('Tài sản')).getByRole('button', { name: MAC7 }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
    return { user, router, panel }
  }

  it('lists assets with their type, holder, state in words and last change', async () => {
    await renderAssets('/assets?section=list')
    await within(table('Tài sản')).findByText(MAC7)
    const rows = within(table('Tài sản')).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(9)
    const mac7 = rows.find((r) => r.textContent?.includes(MAC7))!
    expect(within(mac7).getByText('Nguyễn Thu Lan')).toBeInTheDocument()
    expect(within(mac7).getByText('Đang giao')).toBeInTheDocument()
    expect(within(mac7).getByText('Laptop')).toBeInTheDocument()
    const printer = rows.find((r) => r.textContent?.includes('Máy in HP'))!
    expect(within(printer).getByText('Chưa giao')).toBeInTheDocument()
    expect(within(printer).getByText('Sẵn sàng')).toBeInTheDocument()
    expect(within(rows.find((r) => r.textContent?.includes('iPhone'))!).getByText('Bảo trì')).toBeInTheDocument()
    expect(within(rows.find((r) => r.textContent?.includes('Ghế'))!).getByText('Ngừng dùng')).toBeInTheDocument()
    noIds()
    noCodes()
  })

  it('filters by state through the URL and the request, with counts on the chips', async () => {
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=list')
    await within(table('Tài sản')).findByText(MAC7)
    expect(await screen.findByRole('button', { name: /Sẵn sàng 5/ })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /Sẵn sàng/ }))
    await waitFor(() => expect(search(router).state).toBe('available'))
    await waitFor(() => expect(rowNames('Tài sản')).not.toContain(MAC7))
    expect(calls.some((c) => c.method === 'GET' && c.path.endsWith('/assets') && c.query.get('state') === 'available')).toBe(true)
    expect(screen.getByRole('button', { name: /Sẵn sàng/ })).toHaveAttribute('aria-pressed', 'true')

    await user.click(screen.getByRole('button', { name: /^Tất cả/ }))
    await waitFor(() => expect(search(router).state).toBeUndefined())
  })

  it('filters by type from a chip in the filter row that opens a menu of type names', async () => {
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=list')
    await within(table('Tài sản')).findByText(MAC7)
    const chip = await screen.findByRole('button', { name: 'Loại tài sản: Tất cả' })
    expect(chip).toHaveTextContent('Loại: Tất cả')
    expect(screen.queryByRole('combobox', { name: 'Loại tài sản' })).toBeNull()
    await user.click(chip)
    const menu = await screen.findByRole('menu', { name: 'Chọn loại tài sản' })
    expect(within(menu).getAllByRole('menuitem').map((o) => o.textContent)).toEqual(['Tất cả', 'Laptop', 'Màn hình', 'Giấy phép phần mềm', 'Máy in'])
    await user.click(within(menu).getByRole('menuitem', { name: 'Màn hình' }))
    await waitFor(() => expect(search(router).kind).toBe(T.monitor))
    await waitFor(() => expect(rowNames('Tài sản')).toEqual(['Màn hình Dell 27 inch, bàn 12', 'Màn hình Dell 24 inch, kho']))
    expect(screen.getByRole('button', { name: 'Loại tài sản: Màn hình' })).toHaveTextContent('Loại: Màn hình')
  })

  it('searches by name or holder after a pause and keeps the text in the URL', async () => {
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=list')
    await within(table('Tài sản')).findByText(MAC7)
    await user.type(screen.getByRole('searchbox', { name: 'Tìm tài sản' }), 'vinh')
    await waitFor(() => expect(search(router).q).toBe('vinh'))
    await waitFor(() => expect(rowNames('Tài sản')).toEqual(['Màn hình Dell 27 inch, bàn 12']))
    expect(calls.some((c) => c.query.get('search') === 'vinh')).toBe(true)
  })

  it('says nothing matched, and clears the filters', async () => {
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=list&q=khong-co')
    expect(await screen.findByText(/Không có tài sản nào khớp “khong-co”/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Xoá bộ lọc' }))
    await waitFor(() => expect(search(router).q).toBeUndefined())
    await within(table('Tài sản')).findByText(MAC7)
  })

  it('pages with Trước and Sau and tells where it is', async () => {
    for (let i = 0; i < 20; i++) {
      const id = `88888888-aaaa-4bbb-8ccc-0000000001${String(i).padStart(2, '0')}`
      ASSETS[id] = { id, name: `Bàn phím số ${i}`, type_id: T.laptop, type_name: 'Laptop', state: 'available' }
    }
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=list')
    await within(table('Tài sản')).findByText(MAC7)
    expect(screen.getByText('1 đến 25 trong 29')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Trước' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Sau' }))
    await waitFor(() => expect(search(router).page).toBe(2))
    expect(await screen.findByText('26 đến 29 trong 29')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sau' })).toBeDisabled()
  })

  it('says it cannot load, and retries', async () => {
    mode.listError = true
    const user = userEvent.setup()
    await renderAssets('/assets?section=list')
    expect(await screen.findByText(/Không tải được danh sách tài sản/)).toBeInTheDocument()
    mode.listError = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    await within(table('Tài sản')).findByText(MAC7)
  })

  it('shows a skeleton table while loading', async () => {
    mode.hang = true
    await renderAssets('/assets?section=list')
    await screen.findByRole('tab', { name: 'Danh sách' })
    await waitFor(() => expect(table('Tài sản')).toHaveAttribute('aria-busy', 'true'))
  })

  it('rows are real buttons: ↑/↓ move between them and Enter opens the panel', async () => {
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=list')
    await within(table('Tài sản')).findByText(MAC7)
    const buttons = within(table('Tài sản')).getAllByRole('button')
    buttons[0]!.focus()
    await user.keyboard('{ArrowDown}')
    expect(buttons[1]).toHaveFocus()
    await user.keyboard('{ArrowUp}')
    expect(buttons[0]).toHaveFocus()
    await user.keyboard('{Enter}')
    await waitFor(() => expect(search(router).asset).toBeTruthy())
    expect(await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })).toBeInTheDocument()
  })

  it('opens an asset linked in the URL', async () => {
    await renderAssets(`/assets?section=list&asset=${A.mac7}`)
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
    expect(await within(panel).findByRole('heading', { name: MAC7 })).toBeInTheDocument()
  })

  describe('detail panel', () => {
    it('shows the holder, the type\'s own fields in words, and the history with who did each step', async () => {
      const { panel } = await openMac7()
      await within(panel).findAllByText('Nguyễn Thu Lan')
      expect(within(panel).getByText('Đang giao')).toBeInTheDocument()
      // Custom fields: text, a date as dd/mm/yyyy, a person by name.
      expect(within(panel).getByText('Cấu hình')).toBeInTheDocument()
      expect(within(panel).getByText('M3 Pro, 18 GB, 512 GB')).toBeInTheDocument()
      expect(within(panel).getByText('14/03/2027')).toBeInTheDocument()
      expect(within(panel).getAllByText('Nguyễn Thu Lan').length).toBeGreaterThanOrEqual(2)

      const history = await within(panel).findByRole('list', { name: 'Lịch sử tài sản' })
      const steps = within(history).getAllByRole('listitem')
      // Newest first.
      expect(steps[0]).toHaveTextContent('Lê Thị Hoa đã giao cho Nguyễn Thu Lan · “Thay máy cũ hỏng bản lề”')
      expect(steps[1]).toHaveTextContent('Trần Minh Đức đã thu hồi từ Lê Quang Vinh')
      expect(steps[2]).toHaveTextContent('Trần Minh Đức đã giao cho Lê Quang Vinh')
      expect(steps[3]).toHaveTextContent('Trần Minh Đức đã duyệt nhập kho')
      noIds()
      noCodes()
    })

    it('offers the steps of the lifecycle from this state, as verbs, and a person picker to give it away', async () => {
      const { panel } = await openMac7()
      await within(panel).findByRole('button', { name: 'Giao cho người khác' })
      expect(within(panel).getByRole('button', { name: 'Thu hồi' })).toBeInTheDocument()
      expect(within(panel).getByRole('button', { name: 'Đưa đi bảo trì' })).toBeInTheDocument()
      // Not from this state, and never as a transition name.
      expect(within(panel).queryByRole('button', { name: 'Ngừng dùng' })).toBeNull()
      expect(within(panel).queryByRole('button', { name: /assign|→/ })).toBeNull()
    })

    it('hands the asset to a person picked by name, sending their id and nothing typed', async () => {
      const { user, panel } = await openMac7()
      await user.click(await within(panel).findByRole('button', { name: 'Giao cho người khác' }))
      const box = await within(panel).findByRole('combobox', { name: 'Chọn người nhận' })
      await user.type(box, 'Vinh')
      const option = await within(panel).findByRole('option', { name: /Lê Quang Vinh/ })
      // The current holder is not offered.
      await user.clear(box)
      await user.type(box, 'Lan')
      expect(await within(panel).findByText(/Không tìm thấy ai khớp/)).toBeInTheDocument()
      await user.clear(box)
      await user.type(box, 'Vinh')
      await user.click(await within(panel).findByRole('option', { name: /Lê Quang Vinh/ }))
      void option

      await waitFor(() => expect(posts(`/assets/${A.mac7}/assign`)).toHaveLength(1))
      expect(posts(`/assets/${A.mac7}/assign`)[0]!.body).toEqual({ assignee_id: U.vinh.id })
      await waitFor(() => expect(toasts()).toContain('Đã giao cho Lê Quang Vinh'))
      await waitFor(() => expect(within(screen.getByRole('complementary', { name: 'Chi tiết tài sản' })).getAllByText('Lê Quang Vinh').length).toBeGreaterThan(0))
    })

    it('takes a step with one press and says so', async () => {
      const { user, panel } = await openMac7()
      await user.click(await within(panel).findByRole('button', { name: 'Thu hồi' }))
      await waitFor(() => expect(posts(`/assets/${A.mac7}/transition`)).toHaveLength(1))
      expect(posts(`/assets/${A.mac7}/transition`)[0]!.body).toEqual({ action: 'return' })
      await waitFor(() => expect(toasts()).toContain('Đã thu hồi tài sản'))
      await waitFor(() => expect(within(screen.getByRole('complementary', { name: 'Chi tiết tài sản' })).getByText('Chưa giao')).toBeInTheDocument())
      await within(panel).findByRole('button', { name: 'Giao tài sản' })
      expect(within(panel).queryByRole('button', { name: 'Thu hồi' })).toBeNull()
    })

    it('confirms retiring, and Esc closes only the dialog, then only the panel', async () => {
      const user = userEvent.setup()
      const router = await renderAssets(`/assets?section=list&asset=${A.mac4}`)
      const panel = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
      await user.click(await within(panel).findByRole('button', { name: 'Ngừng dùng' }))
      const dialog = await screen.findByRole('dialog', { name: 'Ngừng dùng?' })
      expect(dialog).toHaveTextContent('Bước này không hoàn tác được')
      expect(within(dialog).getByRole('button', { name: 'Huỷ' })).toHaveFocus()
      expect(posts(`/assets/${A.mac4}/transition`)).toHaveLength(0)

      await user.keyboard('{Escape}')
      await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Ngừng dùng?' })).toBeNull())
      expect(screen.getByRole('complementary', { name: 'Chi tiết tài sản' })).toBeInTheDocument()
      expect(search(router).asset).toBe(A.mac4)

      await user.keyboard('{Escape}')
      await waitFor(() => expect(search(router).asset).toBeUndefined())
      await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Chi tiết tài sản' })).toBeNull())
    })

    it('retires after confirming', async () => {
      const user = userEvent.setup()
      await renderAssets(`/assets?section=list&asset=${A.mac4}`)
      const panel = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
      await user.click(await within(panel).findByRole('button', { name: 'Ngừng dùng' }))
      const dialog = await screen.findByRole('dialog', { name: 'Ngừng dùng?' })
      await user.click(within(dialog).getByRole('button', { name: 'Ngừng dùng' }))
      await waitFor(() => expect(posts(`/assets/${A.mac4}/transition`)[0]?.body).toEqual({ action: 'retire' }))
      await waitFor(() => expect(toasts()).toContain('Đã cho ngừng dùng'))
    })

    it('offers nothing to someone who may only read', async () => {
      mode.member = false
      TYPES[T.laptop]!.permissions = ['read']
      await renderAssets(`/assets?section=list&asset=${A.mac7}`)
      const panel = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
      await within(panel).findAllByText('Nguyễn Thu Lan')
      await waitFor(() => expect(calls.some((c) => c.path === `/assets/${A.mac7}/transitions`)).toBe(true))
      expect(within(panel).queryByRole('button', { name: 'Thu hồi' })).toBeNull()
      expect(within(panel).queryByRole('button', { name: 'Giao cho người khác' })).toBeNull()
      expect(within(panel).queryByText('Thao tác')).toBeNull()
    })

    it('says the asset cannot be opened when the server refuses it', async () => {
      mode.detail403 = true
      await renderAssets(`/assets?section=list&asset=${A.mac7}`)
      const panel = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
      expect(await within(panel).findByText(/Không mở được tài sản này/)).toBeInTheDocument()
      expect(panel.textContent).not.toContain(MAC7)
    })

    it('says the history is not readable without blanking the rest', async () => {
      mode.historyError = true
      await renderAssets(`/assets?section=list&asset=${A.mac7}`)
      const panel = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
      expect(await within(panel).findByText('Bạn không có quyền xem lịch sử của tài sản này.')).toBeInTheDocument()
      expect(within(panel).getAllByText('Nguyễn Thu Lan').length).toBeGreaterThan(0)
    })

    it('starts every asset from a closed picker', async () => {
      const { user, panel, router } = await openMac7()
      await user.click(await within(panel).findByRole('button', { name: 'Giao cho người khác' }))
      await within(panel).findByRole('combobox', { name: 'Chọn người nhận' })
      await act(async () => {
        await router.navigate({ to: '/assets', search: { section: 'list', asset: A.dell12 } as never })
      })
      const next = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
      await within(next).findByRole('heading', { name: 'Màn hình Dell 27 inch, bàn 12' })
      expect(within(next).queryByRole('combobox', { name: 'Chọn người nhận' })).toBeNull()
    })

    it('Esc closes the panel and the URL forgets the asset', async () => {
      const { user, router } = await openMac7()
      await user.keyboard('{Escape}')
      await waitFor(() => expect(search(router).asset).toBeUndefined())
    })
  })

  it('adds an asset of a type, with the type\'s own fields', async () => {
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=list')
    await within(table('Tài sản')).findByText(MAC7)
    await user.click(screen.getByRole('button', { name: 'Thêm tài sản' }))
    const dialog = await screen.findByRole('dialog', { name: 'Thêm tài sản' })

    // Required parts are checked and nothing is sent.
    await user.click(within(dialog).getByRole('button', { name: 'Thêm tài sản' }))
    expect(await within(dialog).findByText('Chọn loại tài sản.')).toBeInTheDocument()
    expect(within(dialog).getByText('Nhập tên tài sản.')).toBeInTheDocument()
    expect(posts(`/workspaces/${WS_ID}/assets`)).toHaveLength(0)

    await user.type(within(dialog).getByRole('combobox', { name: 'Chọn loại tài sản' }), 'Lap')
    await user.click(await within(dialog).findByRole('option', { name: /Laptop/ }))
    await user.type(within(dialog).getByRole('textbox', { name: 'Tên tài sản' }), 'MacBook Pro 14 inch, máy số 8')
    // The type's required field is asked for.
    await user.click(within(dialog).getByRole('button', { name: 'Thêm tài sản' }))
    expect(await within(dialog).findByText('Nhập cấu hình.')).toBeInTheDocument()
    await user.type(within(dialog).getByRole('textbox', { name: 'Cấu hình' }), 'M3 Pro, 18 GB')
    await user.type(within(dialog).getByLabelText(/Hết bảo hành/), '2027-05-01')
    // The person field's label names its picker.
    expect(within(dialog).getByRole('combobox', { name: /Người phụ trách/ })).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Thêm tài sản' }))

    await waitFor(() => expect(posts(`/workspaces/${WS_ID}/assets`)).toHaveLength(1))
    expect(posts(`/workspaces/${WS_ID}/assets`)[0]!.body).toEqual({
      type_id: T.laptop, name: 'MacBook Pro 14 inch, máy số 8', custom_fields: { cfg: 'M3 Pro, 18 GB', exp: '2027-05-01' },
    })
    await waitFor(() => expect(toasts()).toContain('Đã thêm tài sản'))
    await waitFor(() => expect(search(router).asset).toBeTruthy())
  })

  it('does not offer to add an asset to someone who may write nowhere', async () => {
    mode.member = true
    await renderAssets('/assets?section=list')
    await screen.findByRole('tab', { name: 'Danh sách' })
    expect(screen.queryByRole('button', { name: 'Thêm tài sản' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Yêu cầu tài sản' })).toBeNull()
  })
})

describe('AssetsScreen: requests', () => {
  const openRequest = async (id: string, url?: string) => {
    const user = userEvent.setup()
    const router = await renderAssets(url ?? `/assets?section=requests&request=${id}`)
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết yêu cầu' })
    return { user, router, panel }
  }

  it('lists what waits, with who asked, how urgent and when — and not the status, which every row shares', async () => {
    await renderAssets('/assets?section=requests')
    const t = table('Yêu cầu tài sản')
    await within(t).findAllByText('Laptop')
    expect(rowNames('Yêu cầu tài sản')).toEqual(['Laptop', 'Màn hình', 'Máy in'])
    expect(within(t).getByText('Phạm Hải Yến')).toBeInTheDocument()
    expect(within(t).getAllByText('Khẩn').length).toBeGreaterThan(0)
    expect(within(t).getAllByText('Bình thường').length).toBeGreaterThan(0)
    expect(within(t).queryByText('Đang chờ')).toBeNull()
    expect(within(t).queryByRole('columnheader', { name: 'Trạng thái' })).toBeNull()
    noIds()
    noCodes()
  })

  it('filters by what became of them, and then says so in a column', async () => {
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=requests')
    await within(table('Yêu cầu tài sản')).findAllByText('Laptop')
    await user.click(screen.getByRole('button', { name: 'Đã duyệt' }))
    await waitFor(() => expect(search(router).show).toBe('approved'))
    await waitFor(() => expect(rowNames('Yêu cầu tài sản')).toEqual(['Giấy phép phần mềm', 'Laptop']))
    expect(calls.some((c) => c.path.endsWith('/asset-requests') && c.query.get('status') === 'approved,fulfilled')).toBe(true)
    const t = table('Yêu cầu tài sản')
    expect(within(t).getAllByText('Đã duyệt, chờ giao').length).toBeGreaterThan(0)
    expect(within(t).getAllByText('Đã giao').length).toBeGreaterThan(0)
    expect(within(t).getByRole('columnheader', { name: 'Trạng thái' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Từ chối' }))
    await waitFor(() => expect(rowNames('Yêu cầu tài sản')).toEqual(['Màn hình']))
    await user.click(screen.getByRole('button', { name: 'Tôi đã gửi' }))
    await waitFor(() => expect(rowNames('Yêu cầu tài sản')).toEqual(['Máy in']))
    expect(calls.some((c) => c.path.endsWith('/asset-requests') && c.query.get('mine') === 'true')).toBe(true)
  })

  it('says there is nothing waiting, with the copy of the empty state', async () => {
    for (const id of Object.keys(REQUESTS)) delete REQUESTS[id]
    await renderAssets('/assets?section=requests')
    expect(await screen.findByText(/Không còn yêu cầu nào chờ bạn/)).toBeInTheDocument()
  })

  it('says it cannot load, and retries', async () => {
    mode.listError = true
    const user = userEvent.setup()
    await renderAssets('/assets?section=requests')
    expect(await screen.findByText(/Không tải được danh sách yêu cầu/)).toBeInTheDocument()
    mode.listError = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    await within(table('Yêu cầu tài sản')).findAllByText('Laptop')
  })

  it('titles the panel with what the request is about, not the type', async () => {
    const { panel } = await openRequest(R.laptop)
    expect(await within(panel).findByRole('heading', { name: 'Cần máy cài sẵn phần mềm đối chiếu và VPN trước ngày đầu.' })).toBeInTheDocument()
  })

  it('shows the request in the panel: sender with role, type and how many are ready, reason, urgency', async () => {
    const { panel } = await openRequest(R.laptop)
    await within(panel).findByText('Phạm Hải Yến')
    expect(within(panel).getByText('Chuyên viên · Vận hành thanh toán')).toBeInTheDocument()
    expect(within(panel).getByText('Đang chờ')).toBeInTheDocument()
    expect(within(panel).getByText('Khẩn')).toBeInTheDocument()
    expect(within(panel).getByText(/2 tài sản sẵn sàng/)).toBeInTheDocument()
    expect(within(panel).getAllByText(/Cần máy cài sẵn phần mềm/).length).toBe(2)
    noIds()
  })

  it('approves and gives a chosen asset in one dialog', async () => {
    const { user, panel } = await openRequest(R.laptop)
    await user.click(await within(panel).findByRole('button', { name: 'Duyệt và giao' }))
    const dialog = await screen.findByRole('dialog', { name: 'Duyệt và giao tài sản' })
    expect(dialog).toHaveTextContent('Phạm Hải Yến')

    // Nothing is chosen yet: it asks, and sends nothing.
    await user.click(within(dialog).getByRole('button', { name: 'Duyệt và giao' }))
    expect(await within(dialog).findByText('Chọn tài sản để giao.')).toBeInTheDocument()
    expect(posts(`/asset-requests/${R.laptop}/approve`)).toHaveLength(0)

    // Only available assets of the requested type are on offer, found by name, told apart by their fields.
    await user.click(within(dialog).getByRole('combobox', { name: 'Chọn tài sản' }))
    const list = await within(dialog).findByRole('listbox')
    expect(within(list).getAllByRole('option').map((o) => o.textContent)).toEqual([
      'MacBook Pro 14 inch, máy số 4M3 Pro, 18 GB',
      'MacBook Air 13 inch, máy số 5M2, 8 GB',
    ])
    expect(calls.some((c) => c.query.get('type_id') === T.laptop && c.query.get('state') === 'available')).toBe(true)
    await user.click(within(list).getByRole('option', { name: /máy số 4/ }))
    expect(within(dialog).getByText('MacBook Pro 14 inch, máy số 4')).toBeInTheDocument()
    noIds()

    await user.click(within(dialog).getByRole('button', { name: 'Duyệt và giao' }))
    await waitFor(() => expect(posts(`/asset-requests/${R.laptop}/approve`)).toHaveLength(1))
    expect(posts(`/asset-requests/${R.laptop}/approve`)[0]!.body).toEqual({ asset_id: A.mac4 })
    await waitFor(() => expect(toasts()).toContain('Đã duyệt và giao tài sản'))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Duyệt và giao tài sản' })).toBeNull())
    // The request is closed and the asset is now Yến's.
    expect(ASSETS[A.mac4]).toMatchObject({ state: 'assigned', assigned_to_user_id: U.yen.id })
    expect(REQUESTS[R.laptop]).toMatchObject({ status: 'fulfilled', assigned_asset_id: A.mac4 })
    await waitFor(() => expect(within(screen.getByRole('complementary', { name: 'Chi tiết yêu cầu' })).getAllByText('Đã giao').length).toBeGreaterThan(0))
  })

  it('tells the approver when the asset was taken meanwhile, and keeps the dialog for another choice', async () => {
    const { user, panel } = await openRequest(R.laptop)
    await user.click(await within(panel).findByRole('button', { name: 'Duyệt và giao' }))
    const dialog = await screen.findByRole('dialog', { name: 'Duyệt và giao tài sản' })
    await user.click(within(dialog).getByRole('combobox', { name: 'Chọn tài sản' }))
    await user.click(await within(dialog).findByRole('option', { name: /máy số 4/ }))
    // Somebody else gives it away first.
    ASSETS[A.mac4]!.state = 'assigned'
    await user.click(within(dialog).getByRole('button', { name: 'Duyệt và giao' }))
    expect(await within(dialog).findByText(/vừa được giao hoặc không còn Sẵn sàng/)).toBeInTheDocument()
    expect(REQUESTS[R.laptop]!.status).toBe('pending')
    expect(screen.getByRole('dialog', { name: 'Duyệt và giao tài sản' })).toBeInTheDocument()
  })

  it('starts every dialog from nothing chosen and gives focus back', async () => {
    const { user, panel } = await openRequest(R.laptop)
    const opener = await within(panel).findByRole('button', { name: 'Duyệt và giao' })
    await user.click(opener)
    let dialog = await screen.findByRole('dialog', { name: 'Duyệt và giao tài sản' })
    await user.click(within(dialog).getByRole('combobox', { name: 'Chọn tài sản' }))
    await user.click(await within(dialog).findByRole('option', { name: /máy số 4/ }))
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Duyệt và giao tài sản' })).toBeNull())
    expect(screen.getByRole('complementary', { name: 'Chi tiết yêu cầu' })).toBeInTheDocument()
    await waitFor(() => expect(opener).toHaveFocus())
    await user.click(opener)
    dialog = await screen.findByRole('dialog', { name: 'Duyệt và giao tài sản' })
    expect(within(dialog).getByRole('combobox', { name: 'Chọn tài sản' })).toHaveValue('')
  })

  it('offers only to approve when nothing is ready to give, and does exactly that', async () => {
    for (const a of Object.values(ASSETS)) if (a.type_id === T.laptop && a.state === 'available') a.state = 'retired'
    const { user, panel } = await openRequest(R.laptop)
    await user.click(await within(panel).findByRole('button', { name: 'Duyệt và giao' }))
    const dialog = await screen.findByRole('dialog', { name: 'Duyệt và giao tài sản' })
    expect(await within(dialog).findByText(/Chưa có tài sản nào Sẵn sàng thuộc loại Laptop/)).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Duyệt và giao' })).toBeDisabled()
    await user.click(within(dialog).getByRole('button', { name: 'Chỉ duyệt' }))
    await waitFor(() => expect(posts(`/asset-requests/${R.laptop}/approve`)).toHaveLength(1))
    expect(posts(`/asset-requests/${R.laptop}/approve`)[0]!.body).toEqual({})
    await waitFor(() => expect(toasts()).toContain('Đã duyệt yêu cầu'))
    expect(REQUESTS[R.laptop]!.status).toBe('approved')
  })

  it('approves at once when the caller may decide but not give assets', async () => {
    const { user, panel } = await openRequest(R.monitor)
    await user.click(await within(panel).findByRole('button', { name: 'Duyệt' }))
    await waitFor(() => expect(posts(`/asset-requests/${R.monitor}/approve`)).toHaveLength(1))
    expect(posts(`/asset-requests/${R.monitor}/approve`)[0]!.body).toEqual({})
    expect(screen.queryByRole('dialog')).toBeNull()
    await waitFor(() => expect(toasts()).toContain('Đã duyệt yêu cầu'))
  })

  it('gives an asset to a request approved earlier, with the same picker', async () => {
    const { user, panel } = await openRequest(R.approved)
    await user.click(await within(panel).findByRole('button', { name: 'Giao tài sản' }))
    const dialog = await screen.findByRole('dialog', { name: 'Giao tài sản' })
    await user.click(within(dialog).getByRole('combobox', { name: 'Chọn tài sản' }))
    await user.click(await within(dialog).findByRole('option', { name: /Microsoft 365/ }))
    await user.click(within(dialog).getByRole('button', { name: 'Giao tài sản' }))
    await waitFor(() => expect(posts(`/asset-requests/${R.approved}/assign`)).toHaveLength(1))
    expect(posts(`/asset-requests/${R.approved}/assign`)[0]!.body).toEqual({ asset_id: A.m365 })
    await waitFor(() => expect(toasts()).toContain('Đã giao tài sản'))
  })

  it('refuses a rejection without a reason and sends nothing', async () => {
    const { user, panel } = await openRequest(R.laptop)
    await user.click(await within(panel).findByRole('button', { name: 'Từ chối' }))
    const dialog = await screen.findByRole('dialog', { name: 'Từ chối yêu cầu?' })
    expect(within(dialog).getByText(/Phạm Hải Yến/)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Từ chối' }))
    expect(await within(dialog).findByText(/Nhập lý do từ chối/)).toBeInTheDocument()
    expect(within(dialog).getByRole('textbox', { name: 'Lý do từ chối' })).toHaveAttribute('aria-invalid', 'true')
    await user.type(within(dialog).getByRole('textbox', { name: 'Lý do từ chối' }), '   ')
    await user.click(within(dialog).getByRole('button', { name: 'Từ chối' }))
    expect(posts(`/asset-requests/${R.laptop}/reject`)).toHaveLength(0)
  })

  it('rejects with the reason the approver wrote, not a fixed text', async () => {
    const { user, panel } = await openRequest(R.laptop)
    await user.click(await within(panel).findByRole('button', { name: 'Từ chối' }))
    const dialog = await screen.findByRole('dialog', { name: 'Từ chối yêu cầu?' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Lý do từ chối' }), 'Kho còn máy đời cũ đủ dùng.')
    await user.click(within(dialog).getByRole('button', { name: 'Từ chối' }))
    await waitFor(() => expect(posts(`/asset-requests/${R.laptop}/reject`)).toHaveLength(1))
    expect(posts(`/asset-requests/${R.laptop}/reject`)[0]!.body).toEqual({ reason: 'Kho còn máy đời cũ đủ dùng.' })
    await waitFor(() => expect(toasts()).toContain('Đã từ chối yêu cầu'))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Từ chối yêu cầu?' })).toBeNull())
  })

  it('starts every rejection from an empty reason', async () => {
    const { user, panel } = await openRequest(R.laptop)
    const opener = await within(panel).findByRole('button', { name: 'Từ chối' })
    await user.click(opener)
    let dialog = await screen.findByRole('dialog', { name: 'Từ chối yêu cầu?' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Lý do từ chối' }), 'Dở dang')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Từ chối yêu cầu?' })).toBeNull())
    expect(screen.getByRole('complementary', { name: 'Chi tiết yêu cầu' })).toBeInTheDocument()
    await waitFor(() => expect(opener).toHaveFocus())
    await user.click(opener)
    dialog = await screen.findByRole('dialog', { name: 'Từ chối yêu cầu?' })
    expect(within(dialog).getByRole('textbox', { name: 'Lý do từ chối' })).toHaveValue('')
  })

  it('offers no decision to someone who may not decide, and no one decides their own request', async () => {
    mode.member = true
    const { panel } = await openRequest(R.mine)
    await within(panel).findAllByText('Máy in')
    expect(within(panel).queryByRole('button', { name: /Duyệt|Từ chối|Giao tài sản/ })).toBeNull()
  })

  it('keeps the request where it was when the server refuses the decision, and says why', async () => {
    const { user, panel } = await openRequest(R.laptop)
    await user.click(await within(panel).findByRole('button', { name: 'Từ chối' }))
    const dialog = await screen.findByRole('dialog', { name: 'Từ chối yêu cầu?' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Lý do từ chối' }), 'Không đủ ngân sách')
    mode.mutationStatus = 403
    await user.click(within(dialog).getByRole('button', { name: 'Từ chối' }))
    expect(await within(dialog).findByText(/Bạn chưa có quyền từ chối yêu cầu này/)).toBeInTheDocument()
    expect(REQUESTS[R.laptop]!.status).toBe('pending')
  })

  it('says "already decided" differently from "asset just given away", and a lost right differently again', async () => {
    const { user, panel } = await openRequest(R.laptop)
    await user.click(await within(panel).findByRole('button', { name: 'Từ chối' }))
    const dialog = await screen.findByRole('dialog', { name: 'Từ chối yêu cầu?' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Lý do từ chối' }), 'Không đủ ngân sách')
    mode.mutationStatus = 409
    mode.mutationReason = 'request_not_open'
    await user.click(within(dialog).getByRole('button', { name: 'Từ chối' }))
    expect(await within(dialog).findByText(/Yêu cầu này vừa được xử lý/)).toBeInTheDocument()
    expect(within(dialog).queryByText(/vừa được giao/)).toBeNull()
  })

  it('says the person who was to receive an asset is no longer a member', async () => {
    const user = userEvent.setup()
    await renderAssets(`/assets?section=list&asset=${A.mac4}`)
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
    await user.click(await within(panel).findByRole('button', { name: 'Giao tài sản' }))
    await user.type(await within(panel).findByRole('combobox', { name: 'Chọn người nhận' }), 'Vinh')
    mode.mutationStatus = 400
    mode.mutationReason = 'not_a_member'
    await user.click(await within(panel).findByRole('option', { name: /Lê Quang Vinh/ }))
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message).join(' ')).toMatch(/không còn là thành viên/))
  })

  it('says the asset changed when a step was taken on a state that moved', async () => {
    const user = userEvent.setup()
    await renderAssets(`/assets?section=list&asset=${A.mac7}`)
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết tài sản' })
    const button = await within(panel).findByRole('button', { name: 'Thu hồi' })
    mode.mutationStatus = 409
    mode.mutationReason = 'state_changed'
    await user.click(button)
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message).join(' ')).toMatch(/Tài sản vừa thay đổi/))
  })

  it('says the request cannot be opened when the server refuses it', async () => {
    mode.detail403 = true
    const { panel } = await openRequest(R.laptop)
    expect(await within(panel).findByText(/Không mở được yêu cầu này/)).toBeInTheDocument()
  })

  describe('a new request', () => {
    it('opens as a form beside the table from the header and from the old URL', async () => {
      const user = userEvent.setup()
      const router = await renderAssets('/assets?section=requests')
      await within(table('Yêu cầu tài sản')).findAllByText('Laptop')
      await user.click(screen.getByRole('button', { name: 'Yêu cầu tài sản' }))
      await screen.findByRole('complementary', { name: 'Yêu cầu tài sản mới' })
      expect(search(router).compose).toBe('request')
    })

    it('is checked before it is sent: type and reason are asked for', async () => {
      const user = userEvent.setup()
      await renderAssets('/assets?section=requests&compose=request')
      const panel = await screen.findByRole('complementary', { name: 'Yêu cầu tài sản mới' })
      await user.click(within(panel).getByRole('button', { name: 'Gửi yêu cầu' }))
      expect(await within(panel).findByText('Chọn loại tài sản bạn cần.')).toBeInTheDocument()
      expect(within(panel).getByText('Nhập lý do để người duyệt hiểu bạn cần gì.')).toBeInTheDocument()
      expect(posts(`/workspaces/${WS_ID}/asset-requests`)).toHaveLength(0)
    })

    it('offers only the types the caller may ask for, by name with the category and what is ready', async () => {
      const user = userEvent.setup()
      await renderAssets('/assets?section=requests&compose=request')
      const panel = await screen.findByRole('complementary', { name: 'Yêu cầu tài sản mới' })
      await user.click(await within(panel).findByRole('combobox', { name: 'Chọn loại tài sản' }))
      const list = await within(panel).findByRole('listbox')
      expect(within(list).getAllByRole('option').map((o) => o.textContent)).toEqual([
        'LaptopPhần cứng · 2 sẵn sàng',
        'Màn hìnhPhần cứng · 1 sẵn sàng',
        'Giấy phép phần mềmGiấy phép · 1 sẵn sàng',
      ])
      noIds()
    })

    it('sends the type, the urgency and the reason, then shows my own requests', async () => {
      const user = userEvent.setup()
      const router = await renderAssets('/assets?section=requests&compose=request')
      const panel = await screen.findByRole('complementary', { name: 'Yêu cầu tài sản mới' })
      await user.type(await within(panel).findByRole('combobox', { name: 'Chọn loại tài sản' }), 'Màn')
      await user.click(await within(panel).findByRole('option', { name: /Màn hình/ }))
      await user.click(within(panel).getByRole('radio', { name: 'Cao' }))
      expect(within(panel).getByRole('radio', { name: 'Cao' })).toHaveAttribute('aria-checked', 'true')
      await user.type(within(panel).getByRole('textbox', { name: 'Lý do' }), 'Màn hình thứ hai để đối chiếu sao kê.')
      await user.click(within(panel).getByRole('button', { name: 'Gửi yêu cầu' }))

      await waitFor(() => expect(posts(`/workspaces/${WS_ID}/asset-requests`)).toHaveLength(1))
      expect(posts(`/workspaces/${WS_ID}/asset-requests`)[0]!.body).toEqual({
        type_id: T.monitor, reason: 'Màn hình thứ hai để đối chiếu sao kê.', urgency: 'high',
      })
      await waitFor(() => expect(toasts()).toContain('Đã gửi yêu cầu'))
      await waitFor(() => expect(search(router)).toMatchObject({ section: 'requests', show: 'mine' }))
      expect(search(router).compose).toBeUndefined()
      await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Yêu cầu tài sản mới' })).toBeNull())
    })

    it('says why when it could not be sent, and keeps what was typed', async () => {
      const user = userEvent.setup()
      await renderAssets('/assets?section=requests&compose=request')
      const panel = await screen.findByRole('complementary', { name: 'Yêu cầu tài sản mới' })
      await user.type(await within(panel).findByRole('combobox', { name: 'Chọn loại tài sản' }), 'Lap')
      await user.click(await within(panel).findByRole('option', { name: /Laptop/ }))
      await user.type(within(panel).getByRole('textbox', { name: 'Lý do' }), 'Cần gấp')
      mode.mutationStatus = 403
      await user.click(within(panel).getByRole('button', { name: 'Gửi yêu cầu' }))
      expect(await within(panel).findByText(/Bạn chưa có quyền yêu cầu loại tài sản này/)).toBeInTheDocument()
      expect(within(panel).getByRole('textbox', { name: 'Lý do' })).toHaveValue('Cần gấp')
    })

    it('says so when the caller may ask for nothing', async () => {
      mode.member = true
      await renderAssets('/assets?section=requests&compose=request')
      const panel = await screen.findByRole('complementary', { name: 'Yêu cầu tài sản mới' })
      expect(await within(panel).findByText(/Bạn chưa được phép yêu cầu loại tài sản nào/)).toBeInTheDocument()
    })
  })
})

describe('AssetsScreen: asset types', () => {
  it('lists the types with their category in words, their assets and their fields', async () => {
    await renderAssets('/assets?section=types')
    const t = table('Loại tài sản')
    await within(t).findByText('Laptop')
    const laptop = within(t).getAllByRole('row').find((r) => r.textContent?.includes('Laptop'))!
    expect(laptop).toHaveTextContent('Phần cứng')
    expect(laptop).toHaveTextContent('Laptop')
    expect(within(t).getAllByRole('row').find((r) => r.textContent?.includes('Giấy phép phần mềm'))).toHaveTextContent('Giấy phép')
    expect(document.body.textContent).not.toMatch(/\bhardware\b|\blicense\b/)
    noIds()
  })

  it('shows the type\'s fields, and its lifecycle in words with the right each step needs', async () => {
    const user = userEvent.setup()
    await renderAssets('/assets?section=types')
    await user.click(await within(table('Loại tài sản')).findByRole('button', { name: 'Laptop' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết loại tài sản' })
    expect(within(panel).getByRole('textbox', { name: 'Tên trường 1' })).toHaveValue('Cấu hình')
    expect(within(panel).getByRole('textbox', { name: 'Tên trường 2' })).toHaveValue('Hết bảo hành')
    expect(within(panel).getByRole('combobox', { name: 'Kiểu 2' })).toHaveDisplayValue('Ngày')
    expect(within(panel).getByRole('combobox', { name: 'Kiểu 3' })).toHaveDisplayValue('Người')
    const life = within(panel).getByRole('list', { name: 'Vòng đời tài sản' })
    expect(within(life).getAllByRole('listitem').map((l) => l.textContent)).toEqual(['Chờ duyệt', 'Sẵn sàng', 'Đang giao', 'Bảo trì', 'Ngừng dùng', 'Đã thanh lý'])
    expect(panel).toHaveTextContent('Cần quyền Duyệt: Duyệt nhập kho.')
    expect(panel).toHaveTextContent('Cần quyền Quản lý: Giao tài sản, Thu hồi, Đưa đi bảo trì, Hoàn tất bảo trì, Ngừng dùng, Thanh lý.')
    noIds()
    noCodes()
  })

  it('adds a field and saves the schema the server validates; Lưu is off until something changed', async () => {
    const user = userEvent.setup()
    await renderAssets(`/assets?section=types&type=${T.laptop}`)
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết loại tài sản' })
    const save = await within(panel).findByRole('button', { name: 'Lưu' })
    expect(save).toBeDisabled()

    await user.click(within(panel).getByRole('button', { name: 'Thêm trường' }))
    // An unnamed field is refused on save.
    await user.click(save)
    expect(await within(panel).findByText('Nhập tên trường.')).toBeInTheDocument()
    expect(calls.filter((c) => c.method === 'PUT')).toHaveLength(0)

    await user.type(within(panel).getByRole('textbox', { name: 'Tên trường 4' }), 'Màu')
    await user.selectOptions(within(panel).getByRole('combobox', { name: 'Kiểu 4' }), 'Danh sách chọn')
    await user.type(within(panel).getByRole('textbox', { name: /Các lựa chọn 4/ }), 'Bạc, Đen,')
    expect(within(panel).getByRole('textbox', { name: /Các lựa chọn 4/ })).toHaveValue('Bạc, Đen,')
    await user.click(save)

    await waitFor(() => expect(calls.filter((c) => c.method === 'PUT')).toHaveLength(1))
    const put = calls.find((c) => c.method === 'PUT')!
    expect(put.path).toBe(`/asset-types/${T.laptop}/schema`)
    const schema = JSON.parse(String(put.body!.fields_schema))
    expect(schema.required).toEqual(['cfg'])
    const added = Object.values(schema.properties).find((p) => (p as { title: string }).title === 'Màu')
    expect(added).toMatchObject({ type: 'string', 'x-kind': 'choice', enum: ['Bạc', 'Đen'] })
    // Existing fields keep their keys: nothing already stored is orphaned.
    expect(Object.keys(schema.properties).slice(0, 3)).toEqual(['cfg', 'exp', 'owner'])
    await waitFor(() => expect(toasts()).toContain('Đã lưu trường thông tin'))
  })

  it('removes a field and refuses two with one name', async () => {
    const user = userEvent.setup()
    await renderAssets(`/assets?section=types&type=${T.laptop}`)
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết loại tài sản' })
    await user.click(await within(panel).findByRole('button', { name: 'Thêm trường' }))
    await user.type(within(panel).getByRole('textbox', { name: 'Tên trường 4' }), 'cấu hình')
    await user.click(within(panel).getByRole('button', { name: 'Lưu' }))
    expect(await within(panel).findByText('Đã có trường tên này.')).toBeInTheDocument()
    await user.click(within(panel).getByRole('button', { name: 'Xoá trường cấu hình' }))
    expect(within(panel).queryByRole('textbox', { name: 'Tên trường 4' })).toBeNull()
  })

  it('says why a save was refused', async () => {
    const user = userEvent.setup()
    await renderAssets(`/assets?section=types&type=${T.laptop}`)
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết loại tài sản' })
    await user.click(await within(panel).findByRole('button', { name: 'Thêm trường' }))
    await user.type(within(panel).getByRole('textbox', { name: 'Tên trường 4' }), 'Ghi chú')
    mode.mutationStatus = 403
    await user.click(within(panel).getByRole('button', { name: 'Lưu' }))
    expect(await within(panel).findByText(/Bạn chưa có quyền sửa loại tài sản này/)).toBeInTheDocument()
  })

  it('adds a type from a name and a category in words', async () => {
    const user = userEvent.setup()
    const router = await renderAssets('/assets?section=types')
    await within(table('Loại tài sản')).findByText('Laptop')
    await user.click(screen.getByRole('button', { name: 'Thêm loại' }))
    const dialog = await screen.findByRole('dialog', { name: 'Thêm loại tài sản' })
    await user.click(within(dialog).getByRole('button', { name: 'Thêm loại' }))
    expect(await within(dialog).findByText('Nhập tên loại tài sản.')).toBeInTheDocument()
    expect(within(dialog).getByText('Chọn danh mục.')).toBeInTheDocument()
    expect(calls.filter((c) => c.method === 'POST' && c.path.endsWith('/asset-types'))).toHaveLength(0)

    const select = within(dialog).getByRole('combobox', { name: 'Danh mục' })
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual(['Chọn danh mục', 'Phần cứng', 'Phần mềm', 'Giấy phép', 'Nội thất', 'Khác'])
    await user.type(within(dialog).getByRole('textbox', { name: 'Tên loại' }), 'Điện thoại kiểm thử')
    await user.selectOptions(select, 'Phần cứng')
    await user.click(within(dialog).getByRole('button', { name: 'Thêm loại' }))

    await waitFor(() => expect(posts(`/workspaces/${WS_ID}/asset-types`)).toHaveLength(1))
    expect(posts(`/workspaces/${WS_ID}/asset-types`)[0]!.body).toEqual({ name: 'Điện thoại kiểm thử', category: 'hardware' })
    await waitFor(() => expect(toasts()).toContain('Đã thêm loại tài sản'))
    await waitFor(() => expect(search(router).type).toBeTruthy())
  })

  it('shows read-only fields and no way to add or save to someone who may not manage types', async () => {
    mode.member = false
    TYPES[T.laptop]!.permissions = ['read', 'write']
    const user = userEvent.setup()
    // The fixture's can_manage follows mode.member; model a reader with types but no manage.
    api.mockImplementation(async (path, init) => {
      const res = await assetFixtureApi(path, init)
      return path.endsWith('/asset-types') && (init?.method ?? 'GET') === 'GET' ? { ...(res as object), can_manage: false } : res
    })
    await renderAssets('/assets?section=types')
    expect(screen.queryByRole('button', { name: 'Thêm loại' })).toBeNull()
    await user.click(await within(table('Loại tài sản')).findByRole('button', { name: 'Laptop' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết loại tài sản' })
    expect(within(panel).getByText('Cấu hình')).toBeInTheDocument()
    expect(within(panel).getByText('(bắt buộc)')).toBeInTheDocument()
    expect(within(panel).queryByRole('button', { name: 'Thêm trường' })).toBeNull()
    expect(within(panel).queryByRole('button', { name: 'Lưu' })).toBeNull()
    expect(within(panel).queryByRole('textbox')).toBeNull()
  })

  it('says there are no types yet, and offers to add one only to whoever may', async () => {
    mode.empty = true
    await renderAssets('/assets?section=types')
    expect(await screen.findByText(/Chưa có loại tài sản nào\. Tạo loại/)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Thêm loại' }).length).toBeGreaterThan(0)
  })

  it('says it cannot load, and retries', async () => {
    mode.listError = true
    const user = userEvent.setup()
    await renderAssets('/assets?section=types')
    expect(await screen.findByText(/Không tải được danh sách loại tài sản/)).toBeInTheDocument()
    mode.listError = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    await within(table('Loại tài sản')).findByText('Laptop')
  })
})

describe('AssetsScreen: realtime', () => {
  it('washes a step someone else just took in the activity feed, and not my own', async () => {
    await renderAssets()
    const feed = await screen.findByRole('list', { name: 'Hoạt động gần đây' })
    await waitFor(() => expect(feed).toHaveTextContent('đã đưa Máy in HP LaserJet tầng 3 đi bảo trì'))
    expect(feed.querySelector('.rt-wash')).toBeNull()

    ACTIVITY.unshift({
      id: 'act-new', action: 'flag_maintenance', from_state: 'available', to_state: 'maintenance',
      actor_id: U.duc.id, actor_name: 'Trần Minh Đức', created_at: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
      asset_id: A.dell13, asset_name: 'Màn hình Dell 24 inch, kho', type_name: 'Màn hình',
    })
    ACTIVITY.unshift({
      id: 'act-mine', action: 'retire', from_state: 'available', to_state: 'retired',
      actor_id: U.hoa.id, actor_name: 'Lê Thị Hoa', created_at: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
      asset_id: A.chair, asset_name: 'Ghế công thái học, phòng họp nhỏ', type_name: 'Máy in',
    })
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: keys.assets.activitiesAll() })
    })
    await waitFor(() => expect(feed).toHaveTextContent('đã đưa Màn hình Dell 24 inch, kho đi bảo trì'))
    const washed = [...feed.querySelectorAll('.rt-wash')]
    expect(washed).toHaveLength(1)
    expect(washed[0]).toHaveTextContent('Trần Minh Đức')
  })
})
