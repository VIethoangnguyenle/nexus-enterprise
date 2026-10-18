import '@testing-library/jest-dom/vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from '../../api/client'
import type { Contact } from '../../hooks/useContacts'
import { clearResolvers } from '../../lib/people-resolver'
import { switchPreferencesTo } from '../../lib/preferences'
import { useWebSocketStore } from '../../stores/websocket.store'
import { CH, ME, U, WS_ID } from '../../test/chat-fixtures'
import { D, HIDDEN_ID, PEOPLE, directory, fixtureApi, renderContacts, resetServer, said, server } from '../../test/contacts-fixtures'
import { expectNoIds } from '../../test/no-ids'
import { resetClient } from '../../test/render'
import { useToastStore } from '../primitives'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

/** What the two "Đã cùng làm việc" services answer, by person. */
const common = {
  groups: [] as unknown[],
  files: [] as unknown[],
}

function phoneApi(path: string, init?: RequestInit): Promise<unknown> {
  if (path.startsWith(`/workspaces/${encodeURIComponent(WS_ID)}/channels/in-common`)) {
    server.calls.push({ method: 'GET', path })
    return Promise.resolve({ channels: structuredClone(common.groups) })
  }
  if (path.startsWith(`/workspaces/${encodeURIComponent(WS_ID)}/drive/shared-with-me`)) {
    server.calls.push({ method: 'GET', path })
    return Promise.resolve({ items: structuredClone(common.files) })
  }
  return fixtureApi(path, init)
}

beforeEach(() => {
  vi.stubGlobal('matchMedia', (query: string) => {
    const max = /max-width:\s*([\d.]+)px/.exec(query)
    return {
      matches: max ? 390 <= Number(max[1]) : false, media: query,
      addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {},
    }
  })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetServer()
  common.groups = []
  common.files = []
  localStorage.clear()
  switchPreferencesTo(undefined)
  clearResolvers()
  useWebSocketStore.setState({ onlineUsers: {} })
  api.mockImplementation(phoneApi)
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  resetClient()
  useToastStore.getState().clear()
})

const layer = (name: string | RegExp) => screen.findByRole('dialog', { name })
const online = (...ids: string[]) => useWebSocketStore.setState({ onlineUsers: Object.fromEntries(ids.map((id) => [id, 'online'])) })

describe('Danh bạ on a phone at 390px: the first screen', () => {
  it('Thường liên lạc lists recent direct-message partners, latest first', async () => {
    server.activity = [said(CH.dmYen, 2), said(CH.dmLan, 1)]
    await renderContacts()
    const strip = await screen.findByRole('list', { name: 'Thường liên lạc' })
    await waitFor(() => expect(within(strip).getAllByRole('button').map((b) => b.getAttribute('aria-label')))
      .toEqual(['Nguyễn Thu Lan', 'Phạm Hải Yến']))
    expectNoIds()
  })

  it('deny: a partner the directory hides from the viewer is never in Thường liên lạc, and nobody messaged hides it', async () => {
    server.activity = [said(CH.dmYen, 2), said(CH.dmLan, 1)]
    server.contacts = directory(PEOPLE.filter((p) => p.user_id !== U.yen.id))
    await renderContacts()
    const strip = await screen.findByRole('list', { name: 'Thường liên lạc' })
    await waitFor(() => expect(within(strip).getAllByRole('button').map((b) => b.getAttribute('aria-label'))).toEqual(['Nguyễn Thu Lan']))
    expect(within(strip).queryByRole('button', { name: 'Phạm Hải Yến' })).toBeNull()
  })

  it('has no Thường liên lạc when the viewer has messaged nobody', async () => {
    await renderContacts()
    await screen.findByRole('navigation', { name: 'Danh mục' })
    expect(screen.queryByRole('list', { name: 'Thường liên lạc' })).toBeNull()
  })

  it('deny: Đang trực tuyến lists only people of the directory, never a hidden person or the viewer', async () => {
    online(U.lan.id, U.vinh.id, HIDDEN_ID, ME.id)
    await renderContacts()
    const section = await screen.findByRole('region', { name: 'Đang trực tuyến · 2' })
    expect(within(section).getByRole('button', { name: /^Nguyễn Thu Lan/ })).toBeInTheDocument()
    expect(within(section).getByRole('button', { name: /^Lê Quang Vinh/ })).toBeInTheDocument()
    expect(section).not.toHaveTextContent('Nguyễn Hữu Thành')
    expect(section).not.toHaveTextContent('Lê Thị Hoa')
  })

  it('Xem tất cả opens the search screen listing everyone online', async () => {
    const user = userEvent.setup()
    online(...PEOPLE.map((p) => p.user_id))
    await renderContacts()
    const section = await screen.findByRole('region', { name: 'Đang trực tuyến · 6' })
    expect(within(section).getAllByRole('button', { name: /Trực tuyến/ })).toHaveLength(5)
    await user.click(within(section).getByRole('button', { name: 'Xem tất cả' }))
    const search = await layer('Tìm trong danh bạ')
    expect(await within(search).findByText(/6 người khớp/)).toBeInTheDocument()
    expect(within(search).getByRole('button', { name: /^Trần Minh Đức/ })).toBeInTheDocument()
  })
})

describe('Danh bạ on a phone: a level', () => {
  const many = (n: number): Contact[] => Array.from({ length: n }, (_, i) => ({
    ...structuredClone(PEOPLE[0]!),
    user_id: `11111111-aaaa-4bbb-8ccc-0000000010${String(i).padStart(2, '0')}`,
    ngac_node_id: `22222222-aaaa-4bbb-8ccc-0000000010${String(i).padStart(2, '0')}`,
    username: `p${i}`,
    display_name: `Nguyễn Văn ${['An', 'Bình', 'Cường', 'Dũng', 'Đạt', 'Giang', 'Hùng', 'Khánh'][i % 8]} ${i}`.replace(/ \d+$/, ''),
    department: 'Kế toán', department_id: D.kt,
  }))

  it('has no search box: its search icon opens the search screen narrowed to the level, and the chip widens it', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    const level = await layer('Đối soát')
    expect(within(level).queryByRole('searchbox')).toBeNull()
    await user.click(within(level).getByRole('button', { name: 'Tìm người trong Đối soát' }))

    const search = await layer('Tìm trong danh bạ')
    await user.type(within(search).getByRole('searchbox', { name: /^Tìm người/ }), 'chuyen vien')
    expect(await within(search).findByText('Kết quả trong Đối soát · 2 người khớp')).toBeInTheDocument()
    expect(within(search).getByRole('button', { name: /^Nguyễn Thu Lan/ })).toBeInTheDocument()
    // Deny: the other Đối soát and the department above are outside the place.
    expect(within(search).queryByRole('button', { name: /^Trần Bảo Ngọc/ })).toBeNull()
    expect(within(search).queryByRole('button', { name: /^Phạm Hải Yến/ })).toBeNull()

    await user.click(within(search).getByRole('button', { name: 'Bỏ lọc Phòng ban: Đối soát' }))
    expect(await within(search).findByRole('button', { name: /^Trần Bảo Ngọc/ })).toBeInTheDocument()
    expect(within(search).getByText(/Kết quả trong cả Khối Vận hành · 4 người khớp/)).toBeInTheDocument()

    await user.click(within(search).getByRole('button', { name: 'Quay lại' }))
    expect(await layer('Đối soát')).toBeInTheDocument()
  })

  it('a personal workspace\'s search has no Phòng ban or Vai trò chip', async () => {
    server.workspaceType = 'personal'
    await renderContacts('/contacts?cat=search')
    const chips = within(await layer('Tìm trong danh bạ')).getByRole('group', { name: 'Bộ lọc' })
    await within(chips).findByRole('button', { name: /Đang trực tuyến/ })
    expect(within(chips).queryByRole('button', { name: /Phòng ban/ })).toBeNull()
    expect(within(chips).queryByRole('button', { name: /Vai trò/ })).toBeNull()
  })

  it('an organization\'s search has Phòng ban and Vai trò chips', async () => {
    await renderContacts('/contacts?cat=search')
    const org = within(await layer('Tìm trong danh bạ')).getByRole('group', { name: 'Bộ lọc' })
    expect(within(org).getByRole('button', { name: /Phòng ban/ })).toBeInTheDocument()
    expect(within(org).getByRole('button', { name: /Vai trò/ })).toBeInTheDocument()
  })

  it('rows are 60 high with avatar 40, and a swipe reveals Nhắn tin and Gọi, except on the viewer\'s own row', async () => {
    await renderContacts(`/contacts?cat=org&dept=${D.kt}`)
    const level = await layer('Kế toán')
    const row = await within(level).findByRole('button', { name: /^Lê Thị Hoa/ })
    expect(row.className).toMatch(/min-h-15/)
    // The viewer (Hoa) has no swipe; the revealed buttons of others are inert until swiped.
    expect(within(level).queryAllByRole('button', { name: 'Nhắn tin', hidden: true })).toHaveLength(0)

    cleanup()
    resetClient()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    const ds = await layer('Đối soát')
    await within(ds).findByRole('button', { name: /^Nguyễn Thu Lan/ })
    expect(within(ds).getAllByRole('button', { name: 'Nhắn tin', hidden: true })).toHaveLength(2)
    expect(within(ds).getAllByRole('button', { name: 'Gọi', hidden: true })).toHaveLength(2)
  })

  it('from thirty people a level has the A–Z column, which jumps to the first person of a letter', async () => {
    const into = vi.fn()
    Element.prototype.scrollIntoView = into
    const user = userEvent.setup()
    server.contacts = directory([...PEOPLE, ...many(30)])
    await renderContacts(`/contacts?cat=org&dept=${D.kt}`)
    const level = await layer('Kế toán')
    const az = await within(level).findByRole('navigation', { name: 'Nhảy theo chữ cái' })
    expect(within(az).getAllByRole('button').map((b) => b.textContent)).toEqual(['A', 'B', 'C', 'D', 'Đ', 'G', 'H', 'K'])
    await user.click(within(az).getByRole('button', { name: 'Nhảy tới Đ' }))
    expect(into).toHaveBeenCalled()
    const target = into.mock.contexts.at(-1) as HTMLElement
    expect(target.getAttribute('data-az')).toBe('Đ')
    expect(target).toHaveTextContent(/Nguyễn Văn Đạt/)
  })

  it('under thirty people there is no A–Z column', async () => {
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    const level = await layer('Đối soát')
    await within(level).findByRole('button', { name: /^Nguyễn Thu Lan/ })
    expect(within(level).queryByRole('navigation', { name: 'Nhảy theo chữ cái' })).toBeNull()
  })
})

describe('Danh bạ on a phone: the profile card', () => {
  it('shows Đã cùng làm việc from the two services, by name', async () => {
    common.groups = [{ id: CH.doisoat, name: 'Đối soát liên khối', channel_type: 'private', workspace_id: WS_ID }]
    common.files = [{ id: '66666666-aaaa-4bbb-8ccc-000000000001', name: 'Đối chiếu tháng 9.xlsx', item_type: 'file', workspace_id: WS_ID }]
    await renderContacts(`/contacts?person=${U.lan.id}`)
    const profile = await layer('Hồ sơ Nguyễn Thu Lan')
    const work = await within(profile).findByRole('region', { name: 'Đã cùng làm việc' })
    expect(await within(work).findByRole('button', { name: /1 nhóm chung/ })).toHaveTextContent('Đối soát liên khối')
    expect(within(work).getByRole('button', { name: /Tệp đã chia sẻ với bạn/ })).toHaveTextContent('1 tệp')
    expect(server.calls.some((c) => c.path === `/workspaces/${WS_ID}/channels/in-common?with=${U.lan.node}`)).toBe(true)
    expect(server.calls.some((c) => c.path === `/workspaces/${WS_ID}/drive/shared-with-me?owner=${U.lan.id}`)).toBe(true)
    expectNoIds()
  })

  it('one\'s own card has Sửa hồ sơ and asks nothing about common work', async () => {
    await renderContacts(`/contacts?person=${ME.id}`)
    const profile = await layer(/Hồ sơ/)
    expect(within(profile).getByRole('button', { name: 'Sửa hồ sơ' })).toBeInTheDocument()
    expect(within(profile).queryByRole('button', { name: 'Thêm tuỳ chọn' })).toBeNull()
    await act(() => new Promise((r) => setTimeout(r, 20)))
    expect(server.calls.some((c) => /in-common|shared-with-me/.test(c.path))).toBe(false)
  })

  it('a personal workspace\'s card has no department and no Cùng phòng', async () => {
    server.workspaceType = 'personal'
    await renderContacts(`/contacts?person=${U.lan.id}`)
    const profile = await layer('Hồ sơ Nguyễn Thu Lan')
    await within(profile).findByRole('group', { name: 'Thao tác' })
    expect(profile).not.toHaveTextContent(/Phòng ban|Cùng phòng/)
  })
})
