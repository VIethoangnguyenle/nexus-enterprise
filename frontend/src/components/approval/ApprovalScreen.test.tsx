import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import {
  ASSIGNMENTS, R, REQUESTS, T, TEMPLATES, U, UUID_RE, WS_ID, approvalFixtureApi, calls, mode, resetFixtures,
} from '../../test/approval-fixtures'
import { validateApprovalSearch } from '../../lib/approval-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { useWebSocketStore } from '../../stores/websocket.store'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { ApprovalScreen } from './ApprovalScreen'

vi.mock('../../api/client', async (orig) => {
  const { approvalFixtureApi } = await import('../../test/approval-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(approvalFixtureApi) }
})

const api = vi.mocked(apiFetch)
const ORIGINAL = structuredClone(REQUESTS)
const ORIGINAL_TEMPLATES = structuredClone(TEMPLATES)

/** The screen mounted on a real in-memory router, so the URL is the state under test. */
async function renderApproval(url = '/approval') {
  const root = createRootRoute()
  const route = createRoute({
    getParentRoute: () => root, path: '/approval', validateSearch: validateApprovalSearch, component: ApprovalScreen,
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
  for (const [id, r] of Object.entries(structuredClone(ORIGINAL))) REQUESTS[id] = r
  for (const [id, t] of Object.entries(structuredClone(ORIGINAL_TEMPLATES))) TEMPLATES[id] = t
  api.mockImplementation(approvalFixtureApi)
  useWebSocketStore.setState({ approvalActivity: {} })
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
  // A list is read by the text of its chosen option; the option's key is not on screen.
  for (const el of document.body.querySelectorAll('select')) {
    expect(el.selectedOptions[0]?.textContent ?? '').not.toMatch(UUID_RE)
  }
}

const table = () => screen.getByRole('table')
const rowTitles = () =>
  within(table()).getAllByRole('row').slice(1).map((r) => r.querySelector('[data-row]')?.textContent)
const search = (r: Awaited<ReturnType<typeof renderApproval>>) => r.state.location.search as Record<string, unknown>
const posts = (path: string) => calls.filter((c) => c.method === 'POST' && c.path === path)
const puts = () => calls.filter((c) => c.method === 'PUT')

describe('ApprovalScreen: lists', () => {
  it('shows what waits on me, with requester, amount and a word for the status', async () => {
    await renderApproval()
    expect(await screen.findByRole('heading', { level: 1, name: 'Phê duyệt' })).toBeInTheDocument()
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')

    expect(rowTitles()).toEqual(['Tạm ứng công tác phí tháng 10', 'Mua 4 màn hình cho tổ đối soát'])
    expect(within(table()).getAllByText('Phạm Hải Yến').length).toBeGreaterThan(0)
    expect(within(table()).getByText('12.450.000 ₫')).toBeInTheDocument()
    expect(within(table()).getByText('18.960.000 ₫')).toBeInTheDocument()
    expect(within(table()).getAllByText('Chờ bạn')).toHaveLength(2)
    // The tab carries the count.
    expect(screen.getByRole('tab', { name: /Chờ tôi duyệt/ })).toHaveTextContent('2')
    noIds()
  })

  it('moves between tabs through the URL, and each lists its own requests', async () => {
    const user = userEvent.setup()
    const router = await renderApproval()
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')

    await user.click(screen.getByRole('tab', { name: 'Tôi đã gửi' }))
    await within(table()).findByText('Mua văn phòng phẩm')
    expect(search(router).tab).toBe('mine')

    await user.click(screen.getByRole('tab', { name: 'Đã xử lý' }))
    await within(table()).findByText('Gia hạn hợp đồng thuê kho Long Biên')
    expect(within(table()).getByText('Đã duyệt')).toBeInTheDocument()
    expect(within(table()).getByText('132.000.000 ₫')).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: /Chờ tôi duyệt/ }))
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')
    expect(search(router).tab).toBeUndefined()
    noIds()
  })

  it('opens on the tab named in the URL, so a reload lands in place', async () => {
    await renderApproval('/approval?tab=history')
    await within(table()).findByText('Gia hạn hợp đồng thuê kho Long Biên')
    expect(screen.getByRole('tab', { name: 'Đã xử lý' })).toHaveAttribute('aria-selected', 'true')
  })

  it('adds the next page under the rows already shown instead of replacing them', async () => {
    const user = userEvent.setup()
    await renderApproval('/approval?tab=department')
    await within(table()).findByText('Thanh toán phí SMS OTP quý III')
    expect(rowTitles()).toHaveLength(4)

    await user.click(screen.getByRole('button', { name: 'Xem thêm' }))
    await within(table()).findByText('Mua văn phòng phẩm')
    expect(rowTitles()).toHaveLength(5)
    expect(rowTitles()).toContain('Tạm ứng công tác phí tháng 10')
    expect(screen.queryByRole('button', { name: 'Xem thêm' })).toBeNull()
  })

  it('names the step a request waits on when it is not waiting on me', async () => {
    await renderApproval('/approval?tab=department')
    await within(table()).findByText('Thanh toán phí SMS OTP quý III')
    expect(within(table()).getByText('Chờ Giám đốc khối')).toBeInTheDocument()
  })
})

describe('ApprovalScreen: states', () => {
  it('shows a skeleton while the list loads, not a spinner', async () => {
    mode.hang = true
    await renderApproval()
    expect(await screen.findByRole('heading', { level: 1, name: 'Phê duyệt' })).toBeInTheDocument()
    expect(table()).toHaveAttribute('aria-busy', 'true')
    expect(document.querySelectorAll('.skeleton').length).toBeGreaterThan(0)
  })

  it('says so when nothing waits on me', async () => {
    mode.empty = true
    await renderApproval()
    expect(await screen.findByText(/Không có đề nghị nào đang chờ bạn duyệt/)).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()
  })

  it('has an empty state for every other tab, with the way forward where there is one', async () => {
    mode.empty = true
    const user = userEvent.setup()
    await renderApproval()
    await user.click(await screen.findByRole('tab', { name: 'Tôi đã gửi' }))
    expect(await screen.findByText(/Bạn chưa gửi đề nghị nào/)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Tạo đề nghị' }).length).toBeGreaterThan(1)
    await user.click(screen.getByRole('tab', { name: 'Đã xử lý' }))
    expect(await screen.findByText(/chưa duyệt hay trả lại/)).toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: 'Phòng ban' }))
    expect(await screen.findByText(/Chưa có đề nghị nào trong phạm vi/)).toBeInTheDocument()
  })

  it('says what failed and offers a retry', async () => {
    mode.listError = true
    const user = userEvent.setup()
    await renderApproval()
    expect(await screen.findByText(/Không tải được danh sách đề nghị/)).toBeInTheDocument()
    mode.listError = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')
  })

  it('explains an approval service that is not set up for the workspace', async () => {
    const { ApiError } = await import('../../api/client')
    api.mockImplementation((path, init) =>
      String(path).startsWith('/approval/') ? Promise.reject(new ApiError('tenant schema not provisioned', 404)) : approvalFixtureApi(path, init),
    )
    await renderApproval()
    expect(await screen.findByText(/Phê duyệt chưa được bật/)).toBeInTheDocument()
  })
})

describe('ApprovalScreen: detail panel', () => {
  async function openTamung(user = userEvent.setup()) {
    const router = await renderApproval()
    await user.click(await within(table()).findByRole('button', { name: 'Tạm ứng công tác phí tháng 10' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết đề nghị' })
    await within(panel).findByRole('list', { name: 'Chuỗi phê duyệt' })
    return { router, panel, user }
  }

  it('shows the amount, the requester, the reason and the chain by name', async () => {
    const { router, panel } = await openTamung()
    expect(search(router).request).toBe(R.tamung)
    expect(within(panel).getByText('12.450.000 ₫')).toBeInTheDocument()
    expect(within(panel).getAllByText('Phạm Hải Yến').length).toBeGreaterThan(0)
    expect(within(panel).getByText(/Công tác Đà Nẵng 14 đến 17\/10/)).toBeInTheDocument()

    const chain = within(panel).getByRole('list', { name: 'Chuỗi phê duyệt' })
    const items = within(chain).getAllByRole('listitem')
    expect(items.map((li) => li.textContent)).toEqual([
      expect.stringContaining('Trần Minh Đức'),
      expect.stringContaining('Lê Thị Hoa'),
      expect.stringContaining('Đỗ Văn Khải'),
    ])
    expect(within(items[0]!).getByText('Đã duyệt')).toBeInTheDocument()
    expect(within(items[1]!).getByText('Đang chờ')).toBeInTheDocument()
    expect(within(items[1]!).getByText('(bạn)')).toBeInTheDocument()
    expect(within(items[2]!).getByText('Chưa tới')).toBeInTheDocument()
    expect(within(items[0]!).getByText(/Trưởng phòng/)).toBeInTheDocument()
    noIds()
  })

  it('says in the header whose turn it is: mine, or the step it waits on', async () => {
    const user = userEvent.setup()
    const { panel } = await openTamung(user)
    expect(within(panel).getByRole('heading', { name: 'Tạm ứng công tác phí tháng 10' })
      .parentElement!.parentElement!.textContent).toContain('Chờ bạn')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Chi tiết đề nghị' })).toBeNull())
    await user.click(screen.getByRole('tab', { name: 'Phòng ban' }))
    await user.click(await within(table()).findByRole('button', { name: 'Thanh toán phí SMS OTP quý III' }))
    const other = await screen.findByRole('complementary', { name: 'Chi tiết đề nghị' })
    await within(other).findByRole('list', { name: 'Chuỗi phê duyệt' })
    expect(other.textContent).toContain('Chờ Giám đốc khối')
  })

  it('names the actor of every audit line, with a sentence instead of a code', async () => {
    const { panel } = await openTamung()
    const trail = await within(panel).findByRole('list', { name: 'Nhật ký' })
    const lines = within(trail).getAllByRole('listitem').map((li) => li.textContent ?? '')
    expect(lines[0]).toContain('Phạm Hải Yến đã tạo đề nghị')
    expect(lines[1]).toContain('Trần Minh Đức đã duyệt')
    expect(lines[1]).toContain('Đồng ý.')
    expect(lines[2]).toContain('Chuyển sang bước tiếp theo')
    expect(trail.textContent).not.toMatch(/step_advanced|created|approved/)
    noIds()
  })

  it('says the trail is not theirs to read when the server answers 403, and keeps the rest', async () => {
    mode.audit403 = true
    const { panel } = await openTamung()
    expect(await within(panel).findByText('Bạn không có quyền xem nhật ký của đề nghị này.')).toBeInTheDocument()
    expect(within(panel).queryByRole('list', { name: 'Nhật ký' })).toBeNull()
    // The request itself still reads.
    expect(within(panel).getByRole('list', { name: 'Chuỗi phê duyệt' })).toBeInTheDocument()
    expect(within(panel).getByRole('button', { name: 'Duyệt' })).toBeInTheDocument()
  })

  it('does not offer Duyệt or Trả lại on a request that is not waiting on me', async () => {
    const user = userEvent.setup()
    await renderApproval('/approval?tab=department')
    await user.click(await within(table()).findByRole('button', { name: 'Thanh toán phí SMS OTP quý III' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết đề nghị' })
    await within(panel).findByRole('list', { name: 'Chuỗi phê duyệt' })
    expect(within(panel).queryByRole('button', { name: 'Duyệt' })).toBeNull()
    expect(within(panel).queryByRole('button', { name: 'Trả lại' })).toBeNull()
  })

  it('closes with Esc, and the URL forgets the request', async () => {
    const user = userEvent.setup()
    const { router } = await openTamung()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Chi tiết đề nghị' })).toBeNull())
    expect(search(router).request).toBeUndefined()
  })

  it('opens a request named in the URL even when it is not in the list on screen', async () => {
    await renderApproval(`/approval?tab=history&request=${R.tamung}`)
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết đề nghị' })
    expect(await within(panel).findByRole('list', { name: 'Chuỗi phê duyệt' })).toBeInTheDocument()
    expect(within(panel).getByRole('heading', { name: 'Tạm ứng công tác phí tháng 10' })).toBeInTheDocument()
  })

  it('says a request cannot be opened when the server refuses it', async () => {
    await renderApproval('/approval?request=33333333-0000-4000-8000-000000000000')
    expect(await screen.findByText(/Không mở được đề nghị này/)).toBeInTheDocument()
    noIds()
  })
})

describe('ApprovalScreen: deciding', () => {
  async function openTamung() {
    const user = userEvent.setup()
    await renderApproval()
    await user.click(await within(table()).findByRole('button', { name: 'Tạm ứng công tác phí tháng 10' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết đề nghị' })
    await within(panel).findByRole('button', { name: 'Duyệt' })
    return { user, panel }
  }

  it('approves from the panel: the request leaves the list and the user is told', async () => {
    const { user, panel } = await openTamung()
    await user.click(within(panel).getByRole('button', { name: 'Duyệt' }))

    await waitFor(() => expect(posts('/approval/approve')).toHaveLength(1))
    expect(posts('/approval/approve')[0]!.body).toEqual({ request_id: R.tamung, comment: '' })
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Đã duyệt đề nghị'))
    await waitFor(() => expect(rowTitles()).toEqual(['Mua 4 màn hình cho tổ đối soát']))
  })

  it('refuses a return without a reason and sends nothing', async () => {
    const { user, panel } = await openTamung()
    await user.click(within(panel).getByRole('button', { name: 'Trả lại' }))
    const dialog = await screen.findByRole('dialog', { name: 'Trả lại đề nghị?' })
    expect(within(dialog).getByText(/Phạm Hải Yến/)).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'Trả lại' }))
    expect(await within(dialog).findByText(/Nhập lý do trả lại/)).toBeInTheDocument()
    expect(within(dialog).getByRole('textbox', { name: 'Lý do trả lại' })).toHaveAttribute('aria-invalid', 'true')
    // Spaces are not a reason.
    await user.type(within(dialog).getByRole('textbox', { name: 'Lý do trả lại' }), '   ')
    await user.click(within(dialog).getByRole('button', { name: 'Trả lại' }))
    expect(posts('/approval/reject')).toHaveLength(0)
  })

  it('returns with the reason, closes the dialog and says so', async () => {
    const { user, panel } = await openTamung()
    await user.click(within(panel).getByRole('button', { name: 'Trả lại' }))
    const dialog = await screen.findByRole('dialog', { name: 'Trả lại đề nghị?' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Lý do trả lại' }), 'Thiếu hoá đơn khách sạn.')
    await user.click(within(dialog).getByRole('button', { name: 'Trả lại' }))

    await waitFor(() => expect(posts('/approval/reject')).toHaveLength(1))
    expect(posts('/approval/reject')[0]!.body).toEqual({ request_id: R.tamung, comment: 'Thiếu hoá đơn khách sạn.' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Trả lại đề nghị?' })).toBeNull())
    expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Đã trả lại đề nghị')
  })

  it('starts every return from an empty reason and gives focus back to the opener', async () => {
    const { user, panel } = await openTamung()
    const opener = within(panel).getByRole('button', { name: 'Trả lại' })
    await user.click(opener)
    let dialog = await screen.findByRole('dialog', { name: 'Trả lại đề nghị?' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Lý do trả lại' }), 'Lý do dở dang')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Trả lại đề nghị?' })).toBeNull())
    // Esc closed only the dialog; the panel is still there.
    expect(screen.getByRole('complementary', { name: 'Chi tiết đề nghị' })).toBeInTheDocument()
    await waitFor(() => expect(opener).toHaveFocus())

    await user.click(opener)
    dialog = await screen.findByRole('dialog', { name: 'Trả lại đề nghị?' })
    expect(within(dialog).getByRole('textbox', { name: 'Lý do trả lại' })).toHaveValue('')
  })

  it('keeps focus inside the dialog while it is open', async () => {
    const { user, panel } = await openTamung()
    await user.click(within(panel).getByRole('button', { name: 'Trả lại' }))
    const dialog = await screen.findByRole('dialog', { name: 'Trả lại đề nghị?' })
    await waitFor(() => expect(within(dialog).getByRole('textbox', { name: 'Lý do trả lại' })).toHaveFocus())
    for (let i = 0; i < 5; i++) {
      await user.tab()
      expect(dialog.contains(document.activeElement)).toBe(true)
    }
  })

  it('leaves the request where it was when the server refuses the decision', async () => {
    mode.mutationError = true
    const { user, panel } = await openTamung()
    await user.click(within(panel).getByRole('button', { name: 'Duyệt' }))
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message).join(' ')).toMatch(/chưa duyệt đề nghị được/))
    await waitFor(() => expect(rowTitles()).toContain('Tạm ứng công tác phí tháng 10'))
  })

  it('approves several at once with the tick boxes', async () => {
    const user = userEvent.setup()
    await renderApproval()
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')
    expect(screen.queryByText(/Đã chọn/)).toBeNull()

    await user.click(screen.getByRole('checkbox', { name: 'Chọn Tạm ứng công tác phí tháng 10' }))
    expect(await screen.findByText('Đã chọn 1 đề nghị')).toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: 'Chọn tất cả đề nghị' }))
    expect(await screen.findByText('Đã chọn 2 đề nghị')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Duyệt 2 đề nghị' }))
    await waitFor(() => expect(posts('/approval/batch-approve')).toHaveLength(1))
    expect(posts('/approval/batch-approve')[0]!.body).toEqual({ request_ids: [R.tamung, R.manhinh], comment: '' })
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Đã duyệt 2 đề nghị'))
    expect(await screen.findByText(/Không có đề nghị nào đang chờ bạn duyệt/)).toBeInTheDocument()
  })

  it('tells the user how many were approved when the server skipped some', async () => {
    api.mockImplementation((path, init) =>
      path === '/approval/batch-approve'
        ? Promise.resolve({ approved_count: 1, approved_ids: [R.tamung] })
        : approvalFixtureApi(path, init),
    )
    const user = userEvent.setup()
    await renderApproval()
    await user.click(await screen.findByRole('checkbox', { name: 'Chọn tất cả đề nghị' }))
    await user.click(await screen.findByRole('button', { name: 'Duyệt 2 đề nghị' }))
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message).join(' ')).toMatch(/Đã duyệt 1\/2 đề nghị/))
  })

  it('offers batch approval only where there is something to approve', async () => {
    const user = userEvent.setup()
    await renderApproval()
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')
    await user.click(screen.getByRole('tab', { name: 'Tôi đã gửi' }))
    await within(table()).findByText('Mua văn phòng phẩm')
    expect(screen.queryByRole('checkbox')).toBeNull()
  })

  it('clears the ticks when the tab changes', async () => {
    const user = userEvent.setup()
    await renderApproval()
    await user.click(await screen.findByRole('checkbox', { name: 'Chọn Tạm ứng công tác phí tháng 10' }))
    await screen.findByText('Đã chọn 1 đề nghị')
    await user.click(screen.getByRole('tab', { name: 'Đã xử lý' }))
    await waitFor(() => expect(screen.queryByText(/Đã chọn/)).toBeNull())
    await user.click(screen.getByRole('tab', { name: /Chờ tôi duyệt/ }))
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')
    expect(screen.queryByText(/Đã chọn/)).toBeNull()
  })
})

describe('ApprovalScreen: keyboard', () => {
  it('makes every row a real button and moves between them with the arrow keys', async () => {
    const user = userEvent.setup()
    await renderApproval()
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')
    const first = screen.getByRole('button', { name: 'Tạm ứng công tác phí tháng 10' })
    const second = screen.getByRole('button', { name: 'Mua 4 màn hình cho tổ đối soát' })
    first.focus()
    await user.keyboard('{ArrowDown}')
    expect(second).toHaveFocus()
    await user.keyboard('{ArrowUp}')
    expect(first).toHaveFocus()
    // Enter opens it, as on any button.
    await user.keyboard('{Enter}')
    expect(await screen.findByRole('complementary', { name: 'Chi tiết đề nghị' })).toBeInTheDocument()
  })
})

describe('ApprovalScreen: realtime', () => {
  const approvedBy = (id: string, actor: string) =>
    act(async () => {
      useWebSocketStore.setState({ approvalActivity: { [id]: { actorNodeId: actor, action: 'approved', at: Date.now() } } })
      REQUESTS[id] = { ...REQUESTS[id]!, status: 'approved', completed_at: new Date().toISOString() }
      await queryClient.invalidateQueries({ queryKey: keys.approval.all() })
    })

  it("washes a row someone else just decided in their name, and tags it 'vừa duyệt'", async () => {
    await renderApproval('/approval?tab=department')
    await within(table()).findByText('Thanh toán phí SMS OTP quý III')
    expect(screen.queryByText(/vừa duyệt/)).toBeNull()

    await approvedBy(R.sms, U.duc.node)
    // Given name first, as people say it.
    expect(await screen.findByText('Đức vừa duyệt')).toBeInTheDocument()
    const row = screen.getByRole('button', { name: 'Thanh toán phí SMS OTP quý III' }).closest('[role="row"]')!
    expect(row.className).toContain('rt-wash')
    expect(within(row as HTMLElement).getByText('Đã duyệt')).toBeInTheDocument()
  })

  it('does not wash my own decision', async () => {
    await renderApproval('/approval?tab=department')
    await within(table()).findByText('Thanh toán phí SMS OTP quý III')
    await approvedBy(R.sms, U.hoa.node)
    await waitFor(() => expect(within(table()).getAllByText('Đã duyệt').length).toBeGreaterThan(1))
    expect(screen.queryByText(/vừa duyệt/)).toBeNull()
  })

  it('does not wash what was already there when the list first appeared', async () => {
    useWebSocketStore.setState({ approvalActivity: { [R.sms]: { actorNodeId: U.duc.node, action: 'approved', at: Date.now() } } })
    await renderApproval('/approval?tab=department')
    await within(table()).findByText('Thanh toán phí SMS OTP quý III')
    expect(screen.queryByText(/vừa/)).toBeNull()
  })
})

describe('ApprovalScreen: create a request', () => {
  async function openDialog() {
    const user = userEvent.setup()
    await renderApproval()
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')
    await user.click(screen.getByRole('button', { name: 'Tạo đề nghị' }))
    const dialog = await screen.findByRole('dialog', { name: 'Tạo đề nghị' })
    return { user, dialog }
  }

  it('picks a template by name, asks its fields, and sends the template and the answers, with no made-up id', async () => {
    const { user, dialog } = await openDialog()
    const select = await within(dialog).findByRole('combobox', { name: 'Loại đề nghị' })
    await waitFor(() => expect(within(select).getAllByRole('option').length).toBeGreaterThan(1))
    // Active templates only, by name.
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual(['Chọn loại đề nghị', 'Tạm ứng', 'Mua sắm thiết bị'])
    await user.selectOptions(select, 'Tạm ứng')

    await within(dialog).findByRole('textbox', { name: /Lý do/ })
    // The approval flow is previewed by step and approver name.
    const flow = await within(dialog).findByRole('list', { name: 'Chuỗi phê duyệt' })
    expect(flow.textContent).toContain('Trưởng phòng · Trần Minh Đức')
    noIds()

    await user.type(within(dialog).getByRole('textbox', { name: /Lý do/ }), 'Công tác Hải Phòng')
    await user.type(within(dialog).getByRole('textbox', { name: /Số tiền/ }), '2500000')
    await user.click(within(dialog).getByRole('button', { name: 'Gửi đề nghị' }))

    await waitFor(() => expect(posts('/approval/requests')).toHaveLength(1))
    const body = posts('/approval/requests')[0]!.body!
    expect(body).toEqual({
      template_id: T.tamung,
      form_data_json: JSON.stringify({ 'Lý do': 'Công tác Hải Phòng', 'Số tiền': '2500000' }),
    })
    expect(body).not.toHaveProperty('entity_id')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tạo đề nghị' })).toBeNull())
    expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Đã gửi đề nghị')
  })

  it('names the required fields that are empty and sends nothing', async () => {
    const { user, dialog } = await openDialog()
    await user.selectOptions(await within(dialog).findByRole('combobox', { name: 'Loại đề nghị' }), 'Tạm ứng')
    await within(dialog).findByRole('textbox', { name: /Lý do/ })
    await user.click(within(dialog).getByRole('button', { name: 'Gửi đề nghị' }))
    expect(await within(dialog).findByText('Nhập lý do.')).toBeInTheDocument()
    expect(within(dialog).getByText('Nhập số tiền.')).toBeInTheDocument()
    expect(posts('/approval/requests')).toHaveLength(0)
  })

  it('cannot be sent before a template is chosen', async () => {
    const { dialog } = await openDialog()
    expect(within(dialog).getByRole('button', { name: 'Gửi đề nghị' })).toBeDisabled()
  })

  it('starts blank every time it opens', async () => {
    const { user, dialog } = await openDialog()
    await user.selectOptions(await within(dialog).findByRole('combobox', { name: 'Loại đề nghị' }), 'Tạm ứng')
    await user.type(await within(dialog).findByRole('textbox', { name: /Lý do/ }), 'dở dang')
    await user.click(within(dialog).getByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tạo đề nghị' })).toBeNull())
    await user.click(screen.getByRole('button', { name: 'Tạo đề nghị' }))
    const again = await screen.findByRole('dialog', { name: 'Tạo đề nghị' })
    expect(within(again).getByRole('combobox', { name: 'Loại đề nghị' })).toHaveValue('')
  })

  it('says why on the form when the server refuses, and keeps what was typed', async () => {
    const { ApiError } = await import('../../api/client')
    const { user, dialog } = await openDialog()
    api.mockImplementation((path, init) =>
      init?.method === 'POST' && path === '/approval/requests'
        ? Promise.reject(new ApiError('nope', 403))
        : approvalFixtureApi(path, init),
    )
    await user.selectOptions(await within(dialog).findByRole('combobox', { name: 'Loại đề nghị' }), 'Tạm ứng')
    await user.type(await within(dialog).findByRole('textbox', { name: /Lý do/ }), 'Đi công tác')
    await user.type(within(dialog).getByRole('textbox', { name: /Số tiền/ }), '100')
    await user.click(within(dialog).getByRole('button', { name: 'Gửi đề nghị' }))
    expect(await within(dialog).findByText(/Bạn chưa có quyền gửi đề nghị/)).toBeInTheDocument()
    expect(within(dialog).getByRole('textbox', { name: /Lý do/ })).toHaveValue('Đi công tác')
    // The form reports the failure itself, so the shared handler adds no second message.
    expect(useToastStore.getState().toasts).toHaveLength(0)
  })
})

describe('ApprovalScreen: templates', () => {
  it('lists templates by name and kind in words, never the code', async () => {
    const user = userEvent.setup()
    const router = await renderApproval()
    await user.click(await screen.findByRole('tab', { name: 'Mẫu' }))
    const t = await screen.findByRole('table', { name: 'Mẫu phê duyệt' })
    await within(t).findByText('Mua sắm thiết bị')
    expect(search(router).tab).toBe('templates')
    expect(within(t).getByText('Mẫu cũ')).toBeInTheDocument()
    expect(within(t).getAllByText('Chi phí').length).toBeGreaterThan(0)
    expect(within(t).getAllByText('Mua sắm').length).toBeGreaterThan(0)
    expect(within(t).getByText('Đã tắt')).toBeInTheDocument()
    expect(t.textContent).not.toMatch(/\b(expense|purchase|custom)\b/)
    noIds()
  })

  it('opens a template in the panel: its fields and its chain by approver name', async () => {
    const user = userEvent.setup()
    await renderApproval('/approval?tab=templates')
    await user.click(await screen.findByRole('button', { name: 'Tạm ứng' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết mẫu' })
    const steps = await within(panel).findByRole('list', { name: 'Các bước duyệt' })
    expect(steps.textContent).toContain('Trần Minh Đức')
    expect(steps.textContent).toContain('Giám đốc khối')
    expect(within(panel).getAllByText(/Số tiền/).length).toBeGreaterThan(0)
    noIds()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Chi tiết mẫu' })).toBeNull())
  })

  it('shows a role approver by the role name alone', async () => {
    const user = userEvent.setup()
    await renderApproval('/approval?tab=templates')
    await user.click(await screen.findByRole('button', { name: 'Mua sắm thiết bị' }))
    const steps = await screen.findByRole('list', { name: 'Các bước duyệt' })
    expect(steps.textContent).toContain('Kế toán trưởng')
    expect(steps.textContent).not.toContain('_')
  })

  it('has loading, empty and error states of its own', async () => {
    mode.hang = true // lists only; templates answer
    api.mockImplementation((path, init) =>
      String(path).startsWith('/approval/templates') ? new Promise(() => {}) : approvalFixtureApi(path, init),
    )
    const user = userEvent.setup()
    await renderApproval('/approval?tab=templates')
    expect(await screen.findByRole('table', { name: 'Mẫu phê duyệt' })).toHaveAttribute('aria-busy', 'true')
    api.mockImplementation((path, init) =>
      String(path).startsWith('/approval/templates') ? Promise.resolve({ templates: [] }) : approvalFixtureApi(path, init),
    )
    queryClient.clear()
    await user.click(screen.getByRole('tab', { name: 'Phòng ban' }))
    await user.click(screen.getByRole('tab', { name: 'Mẫu' }))
    expect(await screen.findByText(/Chưa có mẫu phê duyệt nào/)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Mẫu mới' }).length).toBeGreaterThan(0)
  })
})

describe('ApprovalScreen: who may manage templates', () => {
  it('offers the templates tab to someone who may manage them', async () => {
    await renderApproval()
    expect(await screen.findByRole('tab', { name: 'Mẫu' })).toBeInTheDocument()
  })

  it('does not offer the templates tab or the builder to a member, and never asks for the templates', async () => {
    mode.member = true
    await renderApproval()
    await within(table()).findByText('Tạm ứng công tác phí tháng 10')
    expect(screen.queryByRole('tab', { name: 'Mẫu' })).toBeNull()
    expect(calls.some((c) => c.path.startsWith('/approval/templates'))).toBe(false)
    // "Tạo đề nghị" is for everyone.
    expect(screen.getByRole('button', { name: 'Tạo đề nghị' })).toBeInTheDocument()
  })

  it.each([
    ['the templates tab', '/approval?tab=templates'],
    ['the builder', `/approval?tab=templates&edit=${T.tamung}`],
    ['a template panel', `/approval?tab=templates&template=${T.tamung}`],
  ])('sends a member who follows a link to %s back to the first tab', async (_name, url) => {
    mode.member = true
    const router = await renderApproval(url)
    // The table on screen changes while the answer about permissions is awaited.
    await waitFor(() => within(table()).getByText('Tạm ứng công tác phí tháng 10'))
    await waitFor(() => expect(search(router).tab).toBeUndefined())
    expect(search(router).edit).toBeUndefined()
    expect(search(router).template).toBeUndefined()
    expect(screen.queryByRole('textbox', { name: 'Tên mẫu' })).toBeNull()
    expect(screen.queryByRole('complementary', { name: 'Chi tiết mẫu' })).toBeNull()
  })
})

describe('ApprovalScreen: a step given to a role', () => {
  it('shows it as waiting on me, opens it with the whole chain, and offers Duyệt', async () => {
    mode.group = true
    const user = userEvent.setup()
    await renderApproval()
    await user.click(await within(table()).findByRole('button', { name: 'Mua vật tư cho tổ kế toán' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết đề nghị' })
    const chain = await within(panel).findByRole('list', { name: 'Chuỗi phê duyệt' })
    expect(within(chain).getByText('Kế toán trưởng')).toBeInTheDocument()
    expect(within(chain).getByText('(bạn)')).toBeInTheDocument()
    expect(await within(panel).findByRole('button', { name: 'Duyệt' })).toBeInTheDocument()
    expect(within(table()).getAllByText('Chờ bạn')).toHaveLength(3)
    noIds()
  })
})

describe('ApprovalScreen: template builder', () => {
  const stepsList = () => screen.getByRole('list', { name: 'Các bước duyệt' })

  it('opens from the list, filled with the template, and has no box to type an id into', async () => {
    const user = userEvent.setup()
    const router = await renderApproval('/approval?tab=templates')
    await user.click(await screen.findByRole('button', { name: 'Sửa mẫu Tạm ứng' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Tạm ứng' })).toBeInTheDocument()
    expect(search(router).edit).toBe(T.tamung)

    expect(screen.getByRole('textbox', { name: 'Tên mẫu' })).toHaveValue('Tạm ứng')
    expect(screen.getByRole('textbox', { name: 'Tên bước 1' })).toHaveValue('Trưởng phòng')
    // The approver is shown as a person, picked from a list.
    expect(within(stepsList()).getAllByText('Trần Minh Đức').length).toBeGreaterThan(0)
    // Nothing in the builder is a free-text box for an approver, a user or a role id.
    for (const box of screen.getAllByRole('textbox')) {
      expect(box.getAttribute('aria-label') ?? '').not.toMatch(/\bID\b|UA|mã/i)
    }
    expect(screen.queryByPlaceholderText(/ID/)).toBeNull()
    noIds()
  })

  it('chooses a person for a step from the list and saves their key, keeping the active flag and priority', async () => {
    const user = userEvent.setup()
    await renderApproval(`/approval?tab=templates&edit=${T.tamung}`)
    await screen.findByRole('textbox', { name: 'Tên mẫu' })
    // Remove the current approver, then search for another by name.
    // The contacts directory arrives after the template; wait for it so the chip is the settled one.
    await waitFor(() => expect(calls.some((c) => c.path === `/workspaces/${WS_ID}/contacts`)).toBe(true))
    await act(async () => {})
    await user.click(within(stepsList()).getByRole('button', { name: 'Bỏ Trần Minh Đức' }))
    const box = within(stepsList()).getByRole('combobox', { name: 'Chọn người duyệt bước 1' })
    await user.type(box, 'ngoc')
    await user.click(await screen.findByRole('option', { name: /Trần Bảo Ngọc/ }))
    expect(within(stepsList()).getByText('Trần Bảo Ngọc')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Lưu mẫu' }))
    await waitFor(() => expect(puts()).toHaveLength(1))
    const sent = puts()[0]!
    expect(sent.path).toBe(`/approval/templates/${T.tamung}`)
    const body = sent.body as { is_active: boolean; priority: number; name: string; steps: Record<string, unknown>[] }
    // An update that omitted these would switch the template off and reset its priority.
    expect(body.is_active).toBe(true)
    expect(body.priority).toBe(5)
    expect(body.name).toBe('Tạm ứng')
    expect(body.steps).toHaveLength(3)
    expect(body.steps[0]).toEqual({
      step_order: 1, name: 'Trưởng phòng', approver_type: 'specific_user', approver_value: U.ngoc.node,
      required_count: 1, timeout_hours: 0,
    })
    expect(body.steps[1]).toMatchObject({ step_order: 2, approver_value: U.hoa.node })
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Đã lưu mẫu'))
  })

  it('keeps a template that was switched off switched off when only its name changes', async () => {
    const user = userEvent.setup()
    await renderApproval(`/approval?tab=templates&edit=${T.cu}`)
    const name = await screen.findByRole('textbox', { name: 'Tên mẫu' })
    await user.clear(name)
    await user.type(name, 'Mẫu cũ đã đổi tên')
    await user.click(screen.getByRole('button', { name: 'Lưu mẫu' }))
    await waitFor(() => expect(puts()).toHaveLength(1))
    expect(puts()[0]!.body).toMatchObject({ name: 'Mẫu cũ đã đổi tên', is_active: false, priority: 0 })
  })

  it('builds a new template with a role and a department chosen by name', async () => {
    const user = userEvent.setup()
    const router = await renderApproval('/approval?tab=templates')
    await user.click((await screen.findAllByRole('button', { name: 'Mẫu mới' }))[0]!)
    expect(await screen.findByRole('heading', { level: 1, name: 'Mẫu mới' })).toBeInTheDocument()
    expect(search(router).edit).toBe('new')

    await user.type(screen.getByRole('textbox', { name: 'Tên mẫu' }), 'Mua sắm lớn')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Loại đề nghị' }), 'Mua sắm')

    // Step 1: a role.
    await user.type(screen.getByRole('textbox', { name: 'Tên bước 1' }), 'Kế toán duyệt')
    await user.click(within(stepsList()).getByRole('radio', { name: 'Vai trò' }))
    const roleBox = within(stepsList()).getByRole('combobox', { name: 'Chọn vai trò duyệt bước 1' })
    await user.click(roleBox)
    const options = within(await screen.findByRole('listbox', { name: 'Gợi ý' })).getAllByRole('option')
    // The names the server gives, as they are.
    expect(options.map((o) => o.textContent)).toEqual(['Kế toán trưởng', 'Members'])
    await user.click(options[0]!)

    // Step 2: a department.
    await user.click(screen.getByRole('button', { name: 'Thêm bước' }))
    await user.type(screen.getByRole('textbox', { name: 'Tên bước 2' }), 'Phòng duyệt')
    const second = within(stepsList()).getAllByRole('radiogroup')[1]!
    await user.click(within(second).getByRole('radio', { name: 'Phòng ban' }))
    await user.click(within(stepsList()).getByRole('combobox', { name: 'Chọn phòng ban duyệt bước 2' }))
    await user.click(await screen.findByRole('option', { name: 'Vận hành thanh toán' }))

    // A form field.
    await user.click(screen.getByRole('button', { name: 'Thêm trường' }))
    await user.type(screen.getByRole('textbox', { name: 'Tên trường 1' }), 'Số tiền')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Kiểu trường 1' }), 'Số tiền')
    await user.click(screen.getByRole('checkbox', { name: 'Trường 1 bắt buộc' }))
    noIds()

    await user.click(screen.getByRole('button', { name: 'Tạo mẫu' }))
    const created = () => posts('/approval/templates')
    await waitFor(() => expect(created()).toHaveLength(1))
    expect(created()[0]!.body).toEqual({
      name: 'Mua sắm lớn', entity_type: 'purchase', priority: 0,
      form_fields: [{ label: 'Số tiền', field_type: 'currency', required: true, options: '', placeholder: '' }],
      steps: [
        { step_order: 1, name: 'Kế toán duyệt', approver_type: 'role_in_dept', approver_value: expect.any(String), required_count: 1, timeout_hours: 0 },
        { step_order: 2, name: 'Phòng duyệt', approver_type: 'department', approver_value: expect.any(String), required_count: 1, timeout_hours: 0 },
      ],
    })
    // Back on the list.
    await waitFor(() => expect(search(router).edit).toBeUndefined())
  })

  it('refuses to save a step with no name or no approver, and says which', async () => {
    const user = userEvent.setup()
    await renderApproval(`/approval?tab=templates&edit=new`)
    await user.type(await screen.findByRole('textbox', { name: 'Tên mẫu' }), 'Mẫu thiếu')
    await user.click(screen.getByRole('button', { name: 'Tạo mẫu' }))
    expect(await screen.findByText('Đặt tên cho bước này.')).toBeInTheDocument()
    expect(screen.getByText('Chọn người duyệt.')).toBeInTheDocument()
    expect(posts('/approval/templates')).toHaveLength(0)
  })

  it('refuses a template with no name', async () => {
    const user = userEvent.setup()
    await renderApproval(`/approval?tab=templates&edit=new`)
    await user.click(await screen.findByRole('button', { name: 'Tạo mẫu' }))
    expect(await screen.findByText(/Đặt tên cho mẫu/)).toBeInTheDocument()
    expect(posts('/approval/templates')).toHaveLength(0)
  })

  it('reorders and removes steps', async () => {
    const user = userEvent.setup()
    await renderApproval(`/approval?tab=templates&edit=${T.tamung}`)
    await screen.findByRole('textbox', { name: 'Tên mẫu' })
    expect(screen.getAllByRole('textbox', { name: /^Tên bước/ }).map((b) => (b as HTMLInputElement).value)).toEqual([
      'Trưởng phòng', 'Kế toán trưởng', 'Giám đốc khối',
    ])
    await user.click(screen.getByRole('button', { name: 'Đưa bước 3 lên trước' }))
    expect(screen.getAllByRole('textbox', { name: /^Tên bước/ }).map((b) => (b as HTMLInputElement).value)).toEqual([
      'Trưởng phòng', 'Giám đốc khối', 'Kế toán trưởng',
    ])
    await user.click(screen.getByRole('button', { name: 'Xoá bước 1' }))
    expect(screen.getAllByRole('textbox', { name: /^Tên bước/ })).toHaveLength(2)
    expect(screen.getByRole('button', { name: 'Đưa bước 1 lên trước' })).toBeDisabled()
  })

  it('never lets the last step be removed', async () => {
    await renderApproval(`/approval?tab=templates&edit=new`)
    expect(await screen.findByRole('button', { name: 'Xoá bước 1' })).toBeDisabled()
  })

  it('shows the kind of an existing template but does not let it be changed, and does not send it', async () => {
    const user = userEvent.setup()
    await renderApproval(`/approval?tab=templates&edit=${T.tamung}`)
    const kind = await screen.findByRole('combobox', { name: 'Loại đề nghị' })
    expect(kind).toBeDisabled()
    expect(kind).toHaveValue('expense')
    await user.click(screen.getByRole('button', { name: 'Lưu mẫu' }))
    await waitFor(() => expect(puts()).toHaveLength(1))
    expect(puts()[0]!.body).not.toHaveProperty('entity_type')
  })

  it('sends the stamp the template was read at, so a concurrent edit is refused rather than overwritten', async () => {
    const user = userEvent.setup()
    await renderApproval(`/approval?tab=templates&edit=${T.tamung}`)
    await screen.findByRole('textbox', { name: 'Tên mẫu' })
    await user.click(screen.getByRole('button', { name: 'Lưu mẫu' }))
    await waitFor(() => expect(puts()).toHaveLength(1))
    expect(puts()[0]!.body).toMatchObject({ expected_updated_at: TEMPLATES[T.tamung]!.updated_at })
  })

  it('says someone else changed the template on a 409 and keeps what was typed', async () => {
    mode.stale = true
    const user = userEvent.setup()
    const router = await renderApproval(`/approval?tab=templates&edit=${T.tamung}`)
    const name = await screen.findByRole('textbox', { name: 'Tên mẫu' })
    await user.type(name, ' sửa')
    await user.click(screen.getByRole('button', { name: 'Lưu mẫu' }))
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message).join(' ')).toMatch(/vừa được người khác thay đổi/))
    expect(screen.getByRole('textbox', { name: 'Tên mẫu' })).toHaveValue('Tạm ứng sửa')
    expect(search(router).edit).toBe(T.tamung)
  })

  it('goes back to the list without saving', async () => {
    const user = userEvent.setup()
    const router = await renderApproval(`/approval?tab=templates&edit=${T.tamung}`)
    await screen.findByRole('textbox', { name: 'Tên mẫu' })
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    await screen.findByRole('table', { name: 'Mẫu phê duyệt' })
    expect(search(router).edit).toBeUndefined()
    expect(puts()).toHaveLength(0)
  })

  it('says so when the template is gone', async () => {
    await renderApproval(`/approval?tab=templates&edit=99999999-0000-4000-8000-0000000000ff`)
    expect(await screen.findByText('Mẫu này không còn nữa.')).toBeInTheDocument()
  })

  it('starts another template from its own values, not the last one', async () => {
    const user = userEvent.setup()
    const router = await renderApproval(`/approval?tab=templates&edit=${T.tamung}`)
    expect(await screen.findByRole('textbox', { name: 'Tên mẫu' })).toHaveValue('Tạm ứng')
    await act(async () => { await router.navigate({ to: '/approval', search: { tab: 'templates', edit: T.muasam } }) })
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Tên mẫu' })).toHaveValue('Mua sắm thiết bị'))
    void user
  })
})

describe('ApprovalScreen: assignments keep their shape', () => {
  it('the fixtures hold an assignment for every request the lists show', () => {
    for (const id of Object.values(R)) expect(ASSIGNMENTS[id]?.length).toBeGreaterThan(0)
  })
})
