import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiFetch } from '../../api/client'
import { addRecent } from '../../lib/recent-searches'
import { CH, ME, MESSAGES, U, WS_ID } from '../../test/chat-fixtures'
import { resetClient } from '../../test/render'
import { answer, renderShell, resetServer, server, viewport } from '../patterns/phone-shell-harness'
import { UnifiedSearch } from './UnifiedSearch'
import type { SearchScope } from './search-scopes'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
}))
const api = vi.mocked(apiFetch)

const doc = (id: string, name: string, over: Record<string, unknown> = {}) => ({
  id, workspace_id: WS_ID, drive_context: 'workspace', item_type: 'file', name, ngac_node_id: `n-${id}`,
  owner_id: U.duc.id, owner_name: 'Trần Minh Đức', status: 'active',
  created_at: '2026-10-09T08:00:00Z', updated_at: '2026-10-09T08:00:00Z', ...over,
})
const DOCS = [
  doc('d1', 'Đối soát tháng 9.xlsx'), doc('d2', 'Đối soát tháng 8.xlsx'), doc('d3', 'Biên bản đối soát.docx'),
  doc('d4', 'Đối soát quý 3.pdf'), doc('d5', 'Mẫu đối soát.xlsx'), doc('d6', 'Đối soát', { item_type: 'folder' }),
]

/** What the services answer this test; `null` = the route is not served (404). */
const svc = {
  drive: DOCS as unknown[] | 'refuse' | 'hang',
  approvals: null as unknown[] | null,
  events: [] as unknown[],
  assets: [{ id: 'a1', name: 'Laptop đối soát', type_id: 't1', type_name: 'Laptop', state: 'available' }] as unknown[],
  templates: [{ id: 'tpl1', name: 'Mua sắm' }] as unknown[],
  pending: [] as unknown[],
}
const calls: string[] = []

function search(path: string, init?: RequestInit): Promise<unknown> {
  calls.push(path)
  const p = path.split('?')[0]!
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))
  if (p === '/drive/search') {
    if (svc.drive === 'refuse') return Promise.reject(new ApiError('access denied', 403))
    if (svc.drive === 'hang') return new Promise(() => {})
    return ok({ items: svc.drive })
  }
  if (p === '/approval/search') {
    return svc.approvals === null
      ? Promise.reject(new ApiError('Not Found', 404))
      : ok({ items: svc.approvals, total: svc.approvals.length })
  }
  if (p === '/approval/templates') return ok({ templates: svc.templates })
  if (p === '/approval/pending') return ok({ items: svc.pending, total: svc.pending.length })
  if (p === `/workspaces/${WS_ID}/calendar/events/search`) return ok({ occurrences: svc.events })
  if (p === `/workspaces/${WS_ID}/assets`) return ok({ assets: svc.assets, total: svc.assets.length })
  if (p === '/messages/search') return ok({ messages: [MESSAGES[1]], has_more: false })
  if (p === '/drive/shared-with-me') return ok({ items: [] })
  return answer(path, init)
}

beforeEach(() => {
  localStorage.clear()
  viewport(true)
  resetServer()
  calls.length = 0
  svc.drive = DOCS
  svc.approvals = null
  svc.events = []
  svc.assets = [{ id: 'a1', name: 'Laptop đối soát', type_id: 't1', type_name: 'Laptop', state: 'available' }]
  svc.templates = [{ id: 'tpl1', name: 'Mua sắm' }]
  svc.pending = []
  api.mockReset()
  api.mockImplementation(search as typeof apiFetch)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

const props = { onClose: vi.fn(), onSwitchWorkspace: vi.fn(), onCreateRequest: vi.fn() }
async function open(home: SearchScope, url = '/drive') {
  const router = await renderShell(<UnifiedSearch home={home} {...props} />, url)
  const dialog = await screen.findByRole('dialog', { name: 'Tìm kiếm' })
  return { router, dialog, user: userEvent.setup() }
}
const scopeChip = (dialog: HTMLElement, name: string) => within(dialog).getByRole('button', { name, pressed: false })
const UUID = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i

describe('before anything is typed', () => {
  it('shows Tìm gần đây of the scope, Gợi ý and Làm nhanh', async () => {
    addRecent('approval', ME.id, 'laptop kế toán')
    addRecent('drive', ME.id, 'của Tài liệu')
    svc.pending = [{
      request: { id: 'r9', template_name: 'Mua 3 laptop', status: 'pending', created_by_name: 'Lê Quang Vinh', created_at: '2026-10-09T08:00:00Z' },
      assignment: { id: 'as1' },
    }]
    const { dialog } = await open('approval', '/approval')
    await waitFor(() => expect(within(dialog).getByRole('searchbox', { name: 'Tìm trong Phê duyệt' })).toHaveFocus())
    expect(within(dialog).getByRole('button', { name: 'Tìm lại laptop kế toán' })).toBeInTheDocument()
    expect(within(dialog).queryByText('của Tài liệu')).toBeNull()
    expect(await within(dialog).findByText('Mua 3 laptop')).toBeInTheDocument()
    expect(within(dialog).getByText('Chờ bạn · Lê Quang Vinh')).toBeInTheDocument()
    const quick = within(dialog).getByRole('region', { name: 'Làm nhanh' })
    await waitFor(() => expect(within(quick).getByRole('button', { name: 'Tạo đề nghị' })).toBeInTheDocument())
    expect(within(quick).getByRole('button', { name: 'Cài đặt Phê duyệt' })).toBeInTheDocument()
  })

  it('opens the settings page of the module from Cài đặt <phân hệ>', async () => {
    const { router, dialog, user } = await open('approval', '/approval')
    const quick = within(dialog).getByRole('region', { name: 'Làm nhanh' })
    await user.click(await within(quick).findByRole('button', { name: 'Cài đặt Phê duyệt' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/approval'))
  })

  it('orders the scope chips: the module, Tất cả, then the others', async () => {
    const { dialog } = await open('drive')
    const chips = within(within(dialog).getByRole('toolbar', { name: 'Phạm vi tìm' })).getAllByRole('button')
    expect(chips.slice(0, 7).map((c) => c.textContent)).toEqual(['Tài liệu', 'Tất cả', 'Chat', 'Người', 'Lịch', 'Phê duyệt', 'Tài sản'])
  })
})

describe('Tất cả, while typing', () => {
  it('keeps the words when the scope changes, groups by kind with three rows and Xem tất cả · N', async () => {
    const { dialog, user } = await open('drive')
    await user.type(within(dialog).getByRole('searchbox'), 'đối soát')
    await user.click(scopeChip(dialog, 'Tất cả'))
    expect(within(dialog).getByRole('searchbox', { name: 'Tìm ở mọi nơi' })).toHaveValue('đối soát')

    const docs = await within(dialog).findByRole('group', { name: 'Tài liệu' })
    await waitFor(() => expect(within(docs).getAllByRole('button', { name: /đối soát|Đối soát/i })).toHaveLength(3))
    expect(within(dialog).getByRole('button', { name: 'Xem tất cả · 6 Tài liệu' })).toBeInTheDocument()
    expect(await within(dialog).findByRole('group', { name: 'Trò chuyện' })).toHaveTextContent('Đối soát giao dịch')
    await waitFor(() => expect(within(dialog).getByRole('group', { name: 'Tài sản' })).toHaveTextContent('Laptop đối soát'))
  })

  it('puts the actions the words call for first, by the module it was opened in', async () => {
    const { dialog, user } = await open('approval', '/approval')
    await user.type(within(dialog).getByRole('searchbox'), 'đối soát')
    await user.click(scopeChip(dialog, 'Tất cả'))
    const actions = await within(dialog).findByRole('group', { name: 'Hành động' })
    await waitFor(() => expect(within(actions).getByRole('button', { name: 'Tạo đề nghị “đối soát”' })).toBeInTheDocument())
    await user.click(within(actions).getByRole('button', { name: 'Tạo đề nghị “đối soát”' }))
    expect(props.onCreateRequest).toHaveBeenCalledTimes(1)
  })

  it('offers to switch workspace when the words ask for it', async () => {
    const { dialog, user } = await open('all')
    await user.type(within(dialog).getByRole('searchbox'), 'đổi work')
    await user.click(await within(dialog).findByRole('button', { name: 'Đổi workspace' }))
    expect(props.onSwitchWorkspace).toHaveBeenCalledTimes(1)
  })

  it('shows no Đề nghị group and no error while the approval route is not served', async () => {
    const { dialog, user } = await open('all')
    await user.type(within(dialog).getByRole('searchbox'), 'đối soát')
    await within(dialog).findByRole('group', { name: 'Tài liệu' })
    await waitFor(() => expect(calls.some((c) => c.startsWith('/approval/search?q='))).toBe(true))
    expect(within(dialog).queryByRole('group', { name: 'Đề nghị' })).toBeNull()
    expect(within(dialog).queryByText(/lỗi|không tải được/i)).toBeNull()
  })

  it('shows the requests the approval service answers, named, never by id', async () => {
    svc.approvals = [{ id: 'r1', template_name: 'Thanh toán phí đối soát Q3', status: 'approved', created_by_name: 'Lê Quang Vinh' }]
    const { dialog, user } = await open('all')
    await user.type(within(dialog).getByRole('searchbox'), 'đối soát')
    const group = await within(dialog).findByRole('group', { name: 'Đề nghị' })
    await waitFor(() => expect(group).toHaveTextContent('Đã duyệt · Lê Quang Vinh'))
    expect(dialog.textContent).not.toMatch(UUID)
  })

  it('leaves out a group whose service refuses, and keeps the others', async () => {
    svc.drive = 'refuse'
    const { dialog, user } = await open('all')
    await user.type(within(dialog).getByRole('searchbox'), 'đối soát')
    await within(dialog).findByRole('group', { name: 'Tài sản' })
    await waitFor(() => expect(within(dialog).queryByRole('group', { name: 'Tài liệu' })).toBeNull())
  })

  it('shows the other groups while a slow one is still waiting', async () => {
    svc.drive = 'hang'
    const { dialog, user } = await open('all')
    await user.type(within(dialog).getByRole('searchbox'), 'đối soát')
    const assets = await within(dialog).findByRole('group', { name: 'Tài sản' })
    await waitFor(() => expect(assets).toHaveTextContent('Laptop đối soát'))
    expect(within(dialog).getByRole('group', { name: 'Tài liệu' })).toHaveAttribute('aria-busy', 'true')
  })
})

describe('a personal workspace', () => {
  it('has no Phê duyệt or Tài sản, asks for no requests or assets, and offers no Tạo đề nghị', async () => {
    server.personal = true
    const { dialog, user } = await open('drive')
    await waitFor(() => expect(within(dialog).queryByRole('button', { name: 'Phê duyệt' })).toBeNull())
    expect(within(dialog).queryByRole('button', { name: 'Tài sản' })).toBeNull()
    expect(within(within(dialog).getByRole('region', { name: 'Làm nhanh' })).queryByRole('button', { name: 'Tạo đề nghị' })).toBeNull()
    await user.click(scopeChip(dialog, 'Tất cả'))
    await user.type(within(dialog).getByRole('searchbox'), 'tạo đề nghị')
    await within(dialog).findByRole('group', { name: 'Tài liệu' })
    expect(within(dialog).queryByRole('button', { name: /Tạo đề nghị/ })).toBeNull()
    expect(calls.some((c) => c.startsWith('/approval/search'))).toBe(false)
    expect(calls.some((c) => c.startsWith(`/workspaces/${WS_ID}/assets`))).toBe(false)
  })
})

describe('one module', () => {
  it('lists every result, narrows with a filter chip, and offers to search everywhere', async () => {
    const { dialog, user } = await open('drive')
    await user.type(within(dialog).getByRole('searchbox'), 'đối soát')
    const list = await within(dialog).findByRole('region', { name: 'Kết quả tìm kiếm' })
    await waitFor(() => expect(within(list).getAllByRole('button', { name: /đối soát/i }).length).toBeGreaterThanOrEqual(6))

    await user.click(within(dialog).getByRole('button', { name: 'Loại' }))
    const sheet = await screen.findByRole('dialog', { name: 'Loại' })
    await user.click(within(sheet).getByRole('radio', { name: 'Bảng tính' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Loại' })).toBeNull())
    expect(within(dialog).getByRole('button', { name: 'Loại: Bảng tính' })).toBeInTheDocument()
    await waitFor(() => expect(within(list).queryByText(/Biên bản/)).toBeNull())
    expect(within(list).getByText(/tháng 9/)).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'Bỏ lọc Loại: Bảng tính' }))
    expect(await within(list).findByText(/Biên bản/)).toBeInTheDocument()

    const everywhere = within(list).getByRole('button', { name: /Tìm ở mọi nơi/ })
    await waitFor(() => expect(everywhere).toHaveTextContent(/Có \d+ kết quả ở phân hệ khác/))
    await user.click(everywhere)
    expect(within(dialog).getByRole('button', { name: 'Tất cả', pressed: true })).toBeInTheDocument()
    expect(within(dialog).getByRole('searchbox')).toHaveValue('đối soát')
  })

  it('opens a document in its folder and keeps the words as a recent search of the scope', async () => {
    const { dialog, user, router } = await open('drive')
    await user.type(within(dialog).getByRole('searchbox'), 'quý 3')
    svc.drive = [doc('d4', 'Đối soát quý 3.pdf', { parent_id: 'f-tai-chinh' })]
    await user.click(await within(dialog).findByRole('button', { name: /quý 3/ }))
    await waitFor(() => expect(router.state.location.search).toMatchObject({ folder: 'f-tai-chinh' }))
    expect(props.onClose).toHaveBeenCalled()
    expect(JSON.parse(localStorage.getItem(`ngac-recent-searches:drive:${ME.id}`) ?? '[]')).toEqual(['quý 3'])
  })
})

describe('Chat', () => {
  it('hands the words to the Tìm trong chat screen', async () => {
    const { dialog, user, router } = await open('drive')
    await user.type(within(dialog).getByRole('searchbox'), 'phí chuyển khoản')
    await user.click(scopeChip(dialog, 'Chat'))
    await waitFor(() => expect(router.state.location.pathname).toBe('/channels'))
    expect(router.state.location.search).toMatchObject({ search: 1, q: 'phí chuyển khoản' })
    expect(router.state.location.state.chatOverlay).toBe(true)
  })

  it('opens a conversation found by its name', async () => {
    const { dialog, user, router } = await open('all')
    await user.type(within(dialog).getByRole('searchbox'), 'giao dịch')
    await user.click(await within(dialog).findByRole('button', { name: /Đối soát giao dịch/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/channels/${CH.doisoat}`))
  })
})
