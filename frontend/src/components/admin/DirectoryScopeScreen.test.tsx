import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { D, RID, WS_ID, adminFixtureApi, calls, mode, resetFixtures } from '../../test/admin-fixtures'
import { expectNoIds } from '../../test/no-ids'
import { validateAdminSearch } from '../../lib/admin-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { DirectoryScopeScreen } from './DirectoryScopeScreen'
import { OverviewScreen } from './OverviewScreen'
import { RolesScreen } from './RolesScreen'
import { UsersScreen } from './UsersScreen'

vi.mock('../../api/client', async (orig) => {
  const { adminFixtureApi } = await import('../../test/admin-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(adminFixtureApi) }
})

const api = vi.mocked(apiFetch)

async function renderAdmin(url = '/admin/directory') {
  const root = createRootRoute()
  const admin = createRoute({ getParentRoute: () => root, path: '/admin', validateSearch: validateAdminSearch, component: Outlet })
  const children = [
    createRoute({ getParentRoute: () => admin, path: '/', component: OverviewScreen }),
    createRoute({ getParentRoute: () => admin, path: '/users', component: UsersScreen }),
    createRoute({ getParentRoute: () => admin, path: '/roles', component: RolesScreen }),
    createRoute({ getParentRoute: () => admin, path: '/directory', component: DirectoryScopeScreen }),
  ]
  const router = createRouter({
    routeTree: root.addChildren([admin.addChildren(children)]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetFixtures()
  api.mockImplementation(adminFixtureApi)
})
afterEach(() => {
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
  vi.unstubAllGlobals()
})

const writes = () => calls.filter((c) => c.method !== 'GET')
const toastTexts = () => useToastStore.getState().toasts.map((t) => t.message)
const scope = `/workspaces/${WS_ID}/admin/directory-scope`

/** A rule list once its rows have arrived (the section is there from the first paint, as skeletons). */
async function settled(name: string) {
  const section = await screen.findByRole('region', { name })
  await waitFor(() => expect(section.querySelector('[aria-busy="true"]')).toBeNull())
  return section
}
const hiddenList = () => settled('Phòng ẩn khỏi danh bạ')
const limitedList = () => settled('Chỉ thấy phòng của mình')

describe('Quản trị › Danh bạ', () => {
  it('says the default, then lists the hidden departments and the limited roles and departments by name', async () => {
    await renderAdmin()
    expect(await screen.findByText('Mặc định mọi thành viên thấy toàn bộ danh bạ. Quản trị viên luôn thấy tất cả.')).toBeInTheDocument()

    const hidden = await hiddenList()
    expect(within(hidden).getByText('1')).toBeInTheDocument()
    const hiddenRow = within(hidden).getByRole('listitem')
    expect(hiddenRow).toHaveTextContent('Vận hành thanh toán')
    expect(hiddenRow).toHaveTextContent('Khối Vận hành · Chỉ 52 người trong phòng và quản trị viên thấy')

    const limited = await limitedList()
    const rows = within(limited).getAllByRole('listitem')
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent('Vai trò Quản lý tài sản')
    expect(rows[0]).toHaveTextContent('1 người · thấy người trong phòng của họ')
    expect(rows[1]).toHaveTextContent('Phòng Kiểm soát nội bộ')
    expect(rows[1]).toHaveTextContent('Khối Vận hành · 6 người · thấy người trong phòng của họ')
    expectNoIds()
  })

  it('shows the fourth tab, and clicking it from Tổng quan opens the screen and keeps the workspace', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin(`/admin?ws=${WS_ID}`)
    await user.click(await screen.findByRole('tab', { name: 'Danh bạ' }))
    await hiddenList()
    expect(router.state.location.pathname).toBe('/admin/directory')
    expect(router.state.location.search).toEqual({ ws: WS_ID })
    expect(screen.getByRole('tab', { name: 'Danh bạ' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Tổng quan', 'Người dùng', 'Vai trò', 'Danh bạ'])
  })

  describe('hiding a department', () => {
    it('picks it by name, asks with the consequence in people, then hides it and says so', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      const hidden = await hiddenList()
      await user.click(within(hidden).getByRole('button', { name: 'Ẩn một phòng' }))
      const picker = await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })
      await user.type(within(picker).getByRole('searchbox', { name: 'Tìm phòng' }), 'ke toan')
      await user.click(within(picker).getByRole('option', { name: /Kế toán/ }))

      const dialog = await screen.findByRole('dialog', { name: 'Ẩn Kế toán khỏi danh bạ?' })
      expect(dialog).toHaveTextContent('14 người của phòng này chỉ còn được người trong phòng và quản trị viên thấy.')
      expect(dialog).toHaveTextContent('Tên và avatar của họ vẫn hiện ở tin nhắn, phê duyệt và tài liệu.')
      expect(writes()).toEqual([])
      await user.click(within(dialog).getByRole('button', { name: 'Ẩn phòng' }))

      await waitFor(() => expect(writes()).toEqual([{ method: 'PUT', path: `${scope}/hidden/${D.ketoan}` }]))
      await waitFor(() => expect(within(screen.getByRole('region', { name: 'Phòng ẩn khỏi danh bạ' })).getAllByRole('listitem')).toHaveLength(2))
      expect(toastTexts()).toContain('Đã ẩn Kế toán khỏi danh bạ')
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
      expectNoIds()
    })

    it('says a department hides everything beneath it, counting the departments', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      const picker = await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })
      await user.click(within(picker).getByRole('option', { name: /Khối Vận hành/ }))
      const dialog = await screen.findByRole('dialog', { name: 'Ẩn Khối Vận hành khỏi danh bạ?' })
      expect(dialog).toHaveTextContent('136 người của phòng này chỉ còn được người trong phòng và quản trị viên thấy, kể cả 5 phòng, nhóm bên dưới.')
    })

    it('counts everyone beneath a department, not only those placed directly in it, in the picker and the question', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      const picker = await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })
      const hint = (name: RegExp) => within(within(picker).getByRole('option', { name })).getByText(/người$/).textContent
      expect(hint(/^Khối Vận hành/)).toBe('136 người')
      expect(hint(/^Vận hành thanh toán/)).toBe('52 người')
      expect(hint(/^Đối soát/)).toBe('12 người')
    })

    it('cannot pick a department already hidden, nor one hidden by its parent, and says why', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      const picker = await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })
      const options = within(picker).getAllByRole('option')
      expect(options.map((o) => o.querySelector('[data-name]')?.textContent)).toEqual([
        'Khối Vận hành', 'Vận hành thanh toán', 'Đối soát', 'Hỗ trợ đối tác', 'Kế toán', 'Kiểm soát nội bộ', 'Hành chính',
      ])
      const own = within(picker).getByRole('option', { name: /^Vận hành thanh toán/ })
      expect(own).toHaveAttribute('aria-disabled', 'true')
      expect(own).toHaveTextContent('Đã ẩn')
      const child = within(picker).getByRole('option', { name: /Đối soát/ })
      expect(child).toHaveAttribute('aria-disabled', 'true')
      expect(child).toHaveTextContent('Ẩn theo Vận hành thanh toán')
      await user.click(child)
      expect(screen.queryByRole('dialog', { name: /Ẩn Đối soát/ })).toBeNull()
      expect(writes()).toEqual([])
    })

    it('does nothing when the question is answered with Huỷ, and returns focus to the button that opened it', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      const add = within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' })
      await user.click(add)
      await user.click(within(await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })).getByRole('option', { name: /Hành chính/ }))
      const dialog = await screen.findByRole('dialog', { name: 'Ẩn Hành chính khỏi danh bạ?' })
      await user.click(within(dialog).getByRole('button', { name: 'Huỷ' }))
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
      expect(writes()).toEqual([])
      expect(add).toHaveFocus()
    })

    it('closes the picker on Esc, and only it', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      const add = within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' })
      await user.click(add)
      await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })
      await user.keyboard('{Escape}')
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
      expect(add).toHaveFocus()
      expect(screen.getByRole('region', { name: 'Phòng ẩn khỏi danh bạ' })).toBeInTheDocument()
    })
  })

  describe('with the keyboard', () => {
    it('moves over the choosable rows with the arrows, skipping the ones that carry the rule, and Enter picks', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      const picker = await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })
      const box = within(picker).getByRole('searchbox', { name: 'Tìm phòng' })
      const active = () => document.getElementById(box.getAttribute('aria-activedescendant') ?? '')?.textContent ?? ''
      await waitFor(() => expect(active()).toMatch(/^Khối Vận hành/))
      await user.keyboard('{ArrowDown}')
      // Vận hành thanh toán is hidden, Đối soát and Hỗ trợ đối tác are hidden by it.
      expect(active()).toMatch(/^Kế toán/)
      expect(within(picker).getByRole('option', { name: /^Kế toán/ })).toHaveAttribute('aria-selected', 'true')
      await user.keyboard('{ArrowUp}{ArrowUp}')
      expect(active()).toMatch(/^Hành chính/)
      await user.keyboard('{ArrowDown}{ArrowDown}{Enter}')
      expect(await screen.findByRole('dialog', { name: 'Ẩn Kế toán khỏi danh bạ?' })).toBeInTheDocument()
    })

    it('does nothing on Enter when nothing matches', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      const picker = await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })
      await user.type(within(picker).getByRole('searchbox', { name: 'Tìm phòng' }), 'zzz{Enter}')
      expect(screen.queryByRole('dialog', { name: /Ẩn .* khỏi danh bạ/ })).toBeNull()
      expect(writes()).toEqual([])
    })

    it('closes the picker when focus leaves it', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })
      // Search box, then the rows: Tab through them all and out.
      for (let i = 0; i < 12 && screen.queryByRole('dialog', { name: 'Chọn phòng để ẩn' }); i++) await user.tab()
      expect(screen.queryByRole('dialog', { name: 'Chọn phòng để ẩn' })).toBeNull()
    })
  })

  describe('showing a department again', () => {
    it('asks first, then deletes the rule and puts the empty line in its place', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      const hidden = await hiddenList()
      await user.click(within(hidden).getByRole('button', { name: 'Bỏ ẩn Vận hành thanh toán' }))
      const dialog = await screen.findByRole('dialog', { name: 'Bỏ ẩn Vận hành thanh toán?' })
      expect(dialog).toHaveTextContent('Mọi thành viên sẽ lại thấy 52 người của phòng này trong danh bạ.')
      expect(writes()).toEqual([])
      await user.click(within(dialog).getByRole('button', { name: 'Bỏ ẩn' }))

      await waitFor(() => expect(writes()).toEqual([{ method: 'DELETE', path: `${scope}/hidden/${D.vanhanh}` }]))
      expect(await screen.findByText('Chưa ẩn phòng nào. Mọi thành viên thấy mọi phòng.')).toBeInTheDocument()
      expect(toastTexts()).toContain('Đã bỏ ẩn Vận hành thanh toán')
      // The row that held focus is gone: focus lands on the list's own button, not on the page.
      await waitFor(() => expect(screen.getByRole('button', { name: 'Ẩn một phòng' })).toHaveFocus())
    })
  })

  describe('limiting a role or a department', () => {
    it('offers only the roles an administrator made, then the departments, and marks what is limited', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await limitedList()).getByRole('button', { name: 'Thêm vai trò hoặc phòng' }))
      const picker = await screen.findByRole('dialog', { name: 'Chọn vai trò hoặc phòng' })
      const roles = within(within(picker).getByRole('group', { name: 'Vai trò' })).getAllByRole('option')
      expect(roles.map((o) => o.querySelector('[data-name]')?.textContent)).toEqual(['Kế toán', 'Kiểm soát nội bộ', 'Quản lý tài sản'])
      expect(within(picker).queryByRole('option', { name: /Chủ sở hữu/ })).toBeNull()
      expect(within(picker).queryByRole('option', { name: /^Thành viên/ })).toBeNull()
      expect(roles[2]).toHaveAttribute('aria-disabled', 'true')
      expect(roles[2]).toHaveTextContent('Đã giới hạn')
      const depts = within(within(picker).getByRole('group', { name: 'Phòng ban' })).getAllByRole('option')
      expect(depts).toHaveLength(7)
      expect(within(picker).getByText('Chủ sở hữu và Thành viên không giới hạn được. Quản trị viên luôn thấy tất cả.')).toBeInTheDocument()
      expectNoIds()
    })

    it('limits a role after asking, and the row appears', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await limitedList()).getByRole('button', { name: 'Thêm vai trò hoặc phòng' }))
      const picker = await screen.findByRole('dialog', { name: 'Chọn vai trò hoặc phòng' })
      await user.click(within(within(picker).getByRole('group', { name: 'Vai trò' })).getByRole('option', { name: /Kế toán/ }))
      const dialog = await screen.findByRole('dialog', { name: 'Giới hạn vai trò Kế toán?' })
      expect(dialog).toHaveTextContent('14 người giữ vai trò này chỉ còn thấy người trong phòng của họ và quản trị viên.')
      await user.click(within(dialog).getByRole('button', { name: 'Giới hạn' }))

      await waitFor(() => expect(writes()).toEqual([{ method: 'PUT', path: `${scope}/limited/roles/${RID.ketoan}` }]))
      await waitFor(() => expect(within(screen.getByRole('region', { name: 'Chỉ thấy phòng của mình' })).getAllByRole('listitem')).toHaveLength(3))
      expect(toastTexts()).toContain('Đã giới hạn vai trò Kế toán')
    })

    it('limits a department, counting the people beneath it', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await limitedList()).getByRole('button', { name: 'Thêm vai trò hoặc phòng' }))
      const picker = await screen.findByRole('dialog', { name: 'Chọn vai trò hoặc phòng' })
      await user.click(within(within(picker).getByRole('group', { name: 'Phòng ban' })).getByRole('option', { name: /Hành chính/ }))
      const dialog = await screen.findByRole('dialog', { name: 'Giới hạn phòng Hành chính?' })
      expect(dialog).toHaveTextContent('13 người của phòng này (cả các phòng bên dưới) chỉ còn thấy người trong phòng của họ')
      await user.click(within(dialog).getByRole('button', { name: 'Giới hạn' }))
      await waitFor(() => expect(writes()).toEqual([{ method: 'PUT', path: `${scope}/limited/departments/${D.hanhchinh}` }]))
      expect(toastTexts()).toContain('Đã giới hạn phòng Hành chính')
    })

    it('lifts a limit after asking', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      const limited = await limitedList()
      await user.click(within(limited).getByRole('button', { name: 'Bỏ giới hạn cho vai trò Quản lý tài sản' }))
      const dialog = await screen.findByRole('dialog', { name: 'Bỏ giới hạn cho vai trò Quản lý tài sản?' })
      expect(dialog).toHaveTextContent('1 người sẽ lại thấy toàn bộ danh bạ, trừ các phòng bị ẩn.')
      await user.click(within(dialog).getByRole('button', { name: 'Bỏ giới hạn' }))
      await waitFor(() => expect(writes()).toEqual([{ method: 'DELETE', path: `${scope}/limited/roles/${RID.taisan}` }]))
      await waitFor(() => expect(within(screen.getByRole('region', { name: 'Chỉ thấy phòng của mình' })).getAllByRole('listitem')).toHaveLength(1))
      expect(toastTexts()).toContain('Đã bỏ giới hạn cho vai trò Quản lý tài sản')
      await waitFor(() => expect(screen.getByRole('button', { name: 'Thêm vai trò hoặc phòng' })).toHaveFocus())
    })
  })

  describe('states', () => {
    it('shows rows of the same shape while loading, with the sentence already there', async () => {
      mode.rulesHang = true
      await renderAdmin()
      expect(await screen.findByText('Mặc định mọi thành viên thấy toàn bộ danh bạ. Quản trị viên luôn thấy tất cả.')).toBeInTheDocument()
      expect(document.querySelectorAll('.skeleton').length).toBeGreaterThan(0)
      expect(document.querySelector('[aria-busy="true"]')).not.toBeNull()
      expect(screen.queryByText(/Chưa ẩn phòng nào/)).toBeNull()
    })

    it('says each list is empty in a sentence and keeps the add button', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      const hidden = await hiddenList()
      await user.click(within(hidden).getByRole('button', { name: 'Bỏ ẩn Vận hành thanh toán' }))
      await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Bỏ ẩn' }))
      await screen.findByText('Chưa ẩn phòng nào. Mọi thành viên thấy mọi phòng.')
      const limited = await limitedList()
      for (const name of ['Bỏ giới hạn cho vai trò Quản lý tài sản', 'Bỏ giới hạn cho phòng Kiểm soát nội bộ']) {
        await user.click(within(limited).getByRole('button', { name }))
        await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Bỏ giới hạn' }))
        await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
      }
      expect(await screen.findByText('Chưa giới hạn ai. Mọi thành viên thấy toàn bộ danh bạ.')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Ẩn một phòng' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Thêm vai trò hoặc phòng' })).toBeInTheDocument()
    })

    it('says it could not load, and Thử lại asks again', async () => {
      const user = userEvent.setup()
      mode.rulesError = true
      await renderAdmin()
      // A server fault is asked about once more before it is believed.
      expect(await screen.findByText('Không tải được phạm vi hiển thị', {}, { timeout: 4000 })).toBeInTheDocument()
      expect(screen.getByText('Kết nối tới máy chủ bị gián đoạn. Kiểm tra mạng rồi thử lại.')).toBeInTheDocument()
      mode.rulesError = false
      await user.click(screen.getByRole('button', { name: 'Thử lại' }))
      await hiddenList()
      expect(screen.queryByText('Không tải được phạm vi hiển thị')).toBeNull()
    })

    it('keeps the list as it was and says how to fix a failed save, once, with Thử lại', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      await user.click(within(await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })).getByRole('option', { name: /Kiểm soát nội bộ/ }))
      mode.ruleWriteStatus = 500
      await user.click(within(await screen.findByRole('dialog', { name: 'Ẩn Kiểm soát nội bộ khỏi danh bạ?' })).getByRole('button', { name: 'Ẩn phòng' }))

      await waitFor(() => expect(toastTexts()).toEqual([
        'Máy chủ đang gặp sự cố nên chưa ẩn phòng ban khỏi danh bạ được. Thử lại sau ít phút.',
      ]))
      expect(useToastStore.getState().toasts[0]!.action?.label).toBe('Thử lại')
      expect(screen.getByRole('region', { name: 'Phòng ẩn khỏi danh bạ' })).toHaveTextContent('Vận hành thanh toán')
      expect(within(screen.getByRole('region', { name: 'Phòng ẩn khỏi danh bạ' })).getAllByRole('listitem')).toHaveLength(1)

      mode.ruleWriteStatus = 0
      act(() => useToastStore.getState().toasts[0]!.action!.onClick())
      await waitFor(() => expect(writes()[writes().length - 1]).toEqual({ method: 'PUT', path: `${scope}/hidden/${D.kiemsoat}` }))
      await waitFor(() => expect(within(screen.getByRole('region', { name: 'Phòng ẩn khỏi danh bạ' })).getAllByRole('listitem')).toHaveLength(2))
    })

    it('names the department and says to check the network when the connection itself fails', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      await user.click(within(await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })).getByRole('option', { name: /Kiểm soát nội bộ/ }))
      mode.ruleWriteStatus = -1
      await user.click(within(await screen.findByRole('dialog', { name: 'Ẩn Kiểm soát nội bộ khỏi danh bạ?' })).getByRole('button', { name: 'Ẩn phòng' }))
      await waitFor(() => expect(toastTexts()).toEqual(['Không ẩn được Kiểm soát nội bộ. Kiểm tra mạng rồi thử lại.']))
    })

    it('says nothing about a save when the session has ended (401): the sign-in takes over', async () => {
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      await user.click(within(await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })).getByRole('option', { name: /Kiểm soát nội bộ/ }))
      mode.ruleWriteStatus = 401
      await user.click(within(await screen.findByRole('dialog', { name: 'Ẩn Kiểm soát nội bộ khỏi danh bạ?' })).getByRole('button', { name: 'Ẩn phòng' }))
      await waitFor(() => expect(writes()).toHaveLength(1))
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
      expect(toastTexts()).toEqual([])
    })

    it('says it could not load, with Thử lại for the people, when the people list fails for a reason other than permission', async () => {
      const user = userEvent.setup()
      mode.membersError = true
      await renderAdmin()
      expect(await screen.findByText('Không tải được phạm vi hiển thị', {}, { timeout: 4000 })).toBeInTheDocument()
      expect(document.querySelector('.skeleton')).toBeNull()
      mode.membersError = false
      await user.click(screen.getByRole('button', { name: 'Thử lại' }))
      await hiddenList()
      expect(screen.queryByText('Không tải được phạm vi hiển thị')).toBeNull()
    })

    it('points the panel at a tab that exists, also for a person who may not see the Danh bạ tab', async () => {
      mode.forbidden = true
      await renderAdmin()
      await screen.findByText(/Bạn chưa có quyền quản trị/)
      const panel = screen.getByRole('tabpanel')
      const by = panel.getAttribute('aria-labelledby')
      expect(by === null || document.getElementById(by) !== null).toBe(true)
    })

    it('says a person who may not manage the workspace has no access, with no tab and no request for the rules', async () => {
      mode.forbidden = true
      await renderAdmin()
      expect(await screen.findByText(/Bạn chưa có quyền quản trị/)).toBeInTheDocument()
      expect(screen.queryByRole('tab', { name: 'Danh bạ' })).toBeNull()
      expect(screen.queryByRole('region', { name: 'Phòng ẩn khỏi danh bạ' })).toBeNull()
      expect(calls.some((c) => c.path.includes('directory-scope'))).toBe(false)
    })
  })

  describe('a workspace that has no directory yet', () => {
    it('has no Danh bạ tab and says nothing about it', async () => {
      mode.rulesNotReady = true
      await renderAdmin('/admin')
      await screen.findByRole('tree', { name: 'Phòng ban' })
      await waitFor(() => expect(calls.some((c) => c.path === scope)).toBe(true))
      expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Tổng quan', 'Người dùng', 'Vai trò'])
      expect(screen.queryByText(/Danh bạ/)).toBeNull()
    })

    it('sends a pasted /admin/directory link to Tổng quan, keeping the workspace', async () => {
      mode.rulesNotReady = true
      const router = await renderAdmin(`/admin/directory?ws=${WS_ID}`)
      await waitFor(() => expect(router.state.location.pathname).toBe('/admin'))
      expect(router.state.location.search).toEqual({ ws: WS_ID })
      expect(await screen.findByRole('tree', { name: 'Phòng ban' })).toBeInTheDocument()
      expect(screen.queryByRole('tab', { name: 'Danh bạ' })).toBeNull()
    })

    it('shows no tab on the other screens while the answer is still on its way', async () => {
      mode.rulesHang = true
      await renderAdmin('/admin/roles')
      await screen.findByRole('table', { name: 'Vai trò' })
      expect(screen.queryByRole('tab', { name: 'Danh bạ' })).toBeNull()
    })
  })

  describe('on a phone', () => {
    it('opens the picker as a bottom sheet and picks from it', async () => {
      vi.stubGlobal('matchMedia', (q: string) => ({
        matches: q.includes('max-width: 767'), media: q, addEventListener: () => {}, removeEventListener: () => {},
      }))
      const user = userEvent.setup()
      await renderAdmin()
      await user.click(within(await hiddenList()).getByRole('button', { name: 'Ẩn một phòng' }))
      const sheet = await screen.findByRole('dialog', { name: 'Chọn phòng để ẩn' })
      expect(sheet).toHaveAttribute('aria-modal', 'true')
      expect(within(sheet).getByRole('heading', { name: 'Ẩn một phòng' })).toBeInTheDocument()
      await user.click(within(sheet).getByRole('option', { name: /Hành chính/ }))
      expect(await screen.findByRole('dialog', { name: 'Ẩn Hành chính khỏi danh bạ?' })).toBeInTheDocument()
    })
  })
})

describe('deleting a hidden department that still has people in it', () => {
  it('says so in words and what to do, once, and keeps the department', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin?dept=${D.vanhanh}`)
    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    await user.click(within(panel).getByRole('button', { name: 'Xoá phòng' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Xoá phòng' }))

    await waitFor(() => expect(toastTexts()).toHaveLength(1))
    expect(toastTexts()[0]).toBe(
      'Chưa xoá được phòng Vận hành thanh toán: phòng đang ẩn khỏi danh bạ và còn người ở trực tiếp trong phòng. Chuyển họ sang phòng khác trước, rồi xoá phòng.',
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('complementary', { name: 'Phòng ban' })).toBeInTheDocument()
  })

  it('does not take any 400 for that refusal: without its reason the usual sentence stands', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin?dept=${D.hotro}`)
    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    mode.deleteDepartmentStatus = 400
    await user.click(within(panel).getByRole('button', { name: 'Xoá phòng' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Xoá phòng' }))
    await waitFor(() => expect(toastTexts()).toHaveLength(1))
    expect(toastTexts()[0]).toMatch(/thông tin chưa hợp lệ/)
    expect(toastTexts()[0]).not.toMatch(/đang ẩn khỏi danh bạ/)
  })

  it('keeps the usual sentence for any other failure to delete', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin?dept=${D.hotro}`)
    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    mode.forbidden = true
    await user.click(within(panel).getByRole('button', { name: 'Xoá phòng' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Xoá phòng' }))
    await waitFor(() => expect(toastTexts()).toHaveLength(1))
    expect(toastTexts()[0]).toMatch(/chưa có quyền xoá phòng ban/)
  })
})
