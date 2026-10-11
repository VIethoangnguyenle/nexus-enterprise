import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, UUID_RE, WS_ID } from '../../test/chat-fixtures'
import { createSectionsServer } from '../../test/chat-sections-server'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { useNavCollapseStore } from '../../stores/nav-collapse.store'
import { ChatNavigator } from './ChatNavigator'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

let server: ReturnType<typeof createSectionsServer>

beforeEach(() => {
  routerState.params = {}
  routerState.pathname = '/channels'
  server = createSectionsServer([
    { id: 'q4', name: 'Quyết toán quý 4', position: 0, channel_ids: [CH.doisoat, CH.dmLan] },
    { id: 'noibo', name: 'Nội bộ kế toán', position: 1, channel_ids: [CH.hopdong] },
  ])
  vi.mocked(apiFetch).mockImplementation(server.api as never)
})
afterEach(() => {
  resetClient()
  useToastStore.getState().clear()
  useNavCollapseStore.setState({ collapsed: {} })
})

const list = (name: string) => screen.getByRole('list', { name })
const rowNames = (name: string) =>
  within(list(name)).getAllByRole('link').map((a) => a.querySelector('.truncate')?.textContent)
/** "Tạo mục" lives in "Trò chuyện mới" (and "Mục mới…" on a row), not at the foot of the column. */
const openCreate = async (user: ReturnType<typeof userEvent.setup>) => {
  await user.click(screen.getByRole('button', { name: 'Trò chuyện mới' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Tạo mục' }))
}

const ready = async () => {
  renderWithClient(<ChatNavigator />)
  await screen.findByRole('list', { name: 'Quyết toán quý 4' })
}
const last = <T,>(xs: T[]) => xs[xs.length - 1]
const toastMessages = () => useToastStore.getState().toasts.map((t) => t.message)

describe('sections in the navigator', () => {
  it('lists the user\'s sections above Tin nhắn trực tiếp and Nhóm, each conversation once', async () => {
    await ready()
    const order = screen.getAllByRole('list').map((l) => l.getAttribute('aria-label'))
    expect(order).toEqual(['Quyết toán quý 4', 'Nội bộ kế toán', 'Tin nhắn trực tiếp', 'Nhóm'])
    expect(rowNames('Quyết toán quý 4').sort()).toEqual(['Nguyễn Thu Lan', 'Đối soát giao dịch'].sort())
    expect(rowNames('Nội bộ kế toán')).toEqual(['Hợp đồng đối tác'])
    expect(rowNames('Tin nhắn trực tiếp')).toEqual(['Phạm Hải Yến'])
    expect(rowNames('Nhóm')).toEqual(['Vận hành chung'])
  })

  it('shows the section\'s unread total on its header while it is collapsed, and remembers the fold', async () => {
    const user = userEvent.setup()
    await ready()
    const toggle = screen.getByRole('button', { name: 'Quyết toán quý 4' })
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(within(toggle).queryByText('3')).toBeNull()
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(within(toggle).getByLabelText('3 tin chưa đọc')).toBeInTheDocument()
    expect(Object.keys(useNavCollapseStore.getState().collapsed)).toEqual([`${WS_ID}:q4`])
  })

  it('keeps the default lists when the sections cannot be loaded', async () => {
    server.failNext('', 500)
    renderWithClient(<ChatNavigator />)
    const spaces = await screen.findByRole('list', { name: 'Nhóm' })
    await waitFor(() => expect(within(spaces).getByText('Đối soát giao dịch')).toBeInTheDocument())
    expect(screen.queryByRole('list', { name: 'Quyết toán quý 4' })).toBeNull()
  })

  // A touch screen has no hover: the row's ⋯ stays visible there, or a tablet
  // could never move a conversation into a section.
  it('keeps the row menu visible on touch screens', async () => {
    await ready()
    const more = screen.getByRole('button', { name: 'Tuỳ chọn cho Vận hành chung' })
    expect(more.className).toContain('pointer-coarse:opacity-100')
  })

  it('never renders an identifier or a raw section id', async () => {
    const user = userEvent.setup()
    await ready()
    await user.click(screen.getByRole('button', { name: 'Tuỳ chọn cho Vận hành chung' }))
    await screen.findByRole('menu')
    expect(document.body.textContent).not.toMatch(UUID_RE)
    expect(document.body.textContent).not.toMatch(/\bq4\b|noibo/)
  })
})

describe('creating a section', () => {
  it('"Tạo mục" opens a name popover; Enter creates the section at the end and says so', async () => {
    const user = userEvent.setup()
    await ready()
    await openCreate(user)
    const dialog = await screen.findByRole('dialog', { name: 'Tên mục mới' })
    const field = within(dialog).getByLabelText('Tên mục mới')
    await waitFor(() => expect(field).toHaveFocus())
    await user.type(field, '  Dự án Q4  {Enter}')

    // Empty, so it has a header and a hint but no list yet.
    const created = await screen.findByRole('button', { name: 'Dự án Q4' })
    const headers = screen.getAllByRole('button', { expanded: true }).map((b) => b.textContent)
    expect(headers.slice(0, 3)).toEqual(['Quyết toán quý 4', 'Nội bộ kế toán', 'Dự án Q4'])
    expect(created).toHaveAttribute('aria-expanded', 'true')
    expect(screen.queryByRole('dialog', { name: 'Tên mục mới' })).toBeNull()
    expect(toastMessages()).toContain('Đã tạo mục “Dự án Q4”')
    expect(server.calls.find((c) => c.method === 'POST')?.body).toMatchObject({ name: 'Dự án Q4' })
  })

  it('says why a name cannot be used, without calling the server', async () => {
    const user = userEvent.setup()
    await ready()
    await openCreate(user)
    const dialog = await screen.findByRole('dialog', { name: 'Tên mục mới' })
    const field = within(dialog).getByLabelText('Tên mục mới')

    await user.type(field, '   {Enter}')
    expect(await within(dialog).findByText('Nhập tên mục')).toBeInTheDocument()
    expect(field).toHaveAttribute('aria-invalid', 'true')

    await user.clear(field)
    await user.type(field, 'nội bộ KẾ TOÁN{Enter}')
    expect(await within(dialog).findByText('Bạn đã có mục tên này')).toBeInTheDocument()
    expect(server.calls.filter((c) => c.method === 'POST')).toHaveLength(0)
  })

  it('shows the server\'s refusal of a taken name inline and keeps the popover open', async () => {
    const user = userEvent.setup()
    await ready()
    await openCreate(user)
    const dialog = await screen.findByRole('dialog', { name: 'Tên mục mới' })
    server.failNext('', 409)
    await user.type(within(dialog).getByLabelText('Tên mục mới'), 'Mới{Enter}')
    expect(await within(dialog).findByText('Bạn đã có mục tên này')).toBeInTheDocument()
    expect(toastMessages()).toEqual([])
  })

  it('closes on Huỷ and on Esc, giving focus back to the button that opened it', async () => {
    const user = userEvent.setup()
    await ready()
    const open = screen.getByRole('button', { name: 'Trò chuyện mới' })
    await openCreate(user)
    await user.click(within(await screen.findByRole('dialog', { name: 'Tên mục mới' })).getByRole('button', { name: 'Huỷ' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    await openCreate(user)
    await screen.findByRole('dialog', { name: 'Tên mục mới' })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(open).toHaveFocus()
  })

  it('has no separate "Tạo mục" button at the foot of the column', async () => {
    await ready()
    expect(screen.queryByRole('button', { name: 'Tạo mục' })).toBeNull()
  })

  it('is also offered by "Trò chuyện mới"', async () => {
    const user = userEvent.setup()
    await ready()
    await user.click(screen.getByRole('button', { name: 'Trò chuyện mới' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Tạo mục' }))
    expect(await screen.findByRole('dialog', { name: 'Tên mục mới' })).toBeInTheDocument()
  })
})

describe('moving a conversation', () => {
  const rowMenu = async (user: ReturnType<typeof userEvent.setup>, name: string) => {
    await user.click(screen.getByRole('button', { name: `Tuỳ chọn cho ${name}` }))
    return screen.findByRole('menu', { name: `Tuỳ chọn cho ${name}` })
  }

  it('lists the sections as radio items with the current one checked, then "Mục mới…"', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await rowMenu(user, 'Hợp đồng đối tác')
    expect(within(menu).getByText('Chuyển tới mục')).toBeInTheDocument()
    expect(within(menu).getByRole('menuitemradio', { name: 'Nội bộ kế toán' })).toBeChecked()
    expect(within(menu).getByRole('menuitemradio', { name: 'Quyết toán quý 4' })).not.toBeChecked()
    expect(within(menu).getByRole('menuitem', { name: 'Mục mới…' })).toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: 'Bỏ khỏi mục' })).toBeInTheDocument()
    // No starring or muting: the backend has none, so they are not offered.
    expect(within(menu).queryByText(/Gắn sao|Tắt thông báo/)).toBeNull()
  })

  it('does not offer "Bỏ khỏi mục" for a conversation in no section', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await rowMenu(user, 'Vận hành chung')
    expect(within(menu).queryByRole('menuitem', { name: 'Bỏ khỏi mục' })).toBeNull()
  })

  it('moves the row into the chosen section at once and says so', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await rowMenu(user, 'Vận hành chung')
    await user.click(within(menu).getByRole('menuitemradio', { name: 'Nội bộ kế toán' }))
    await waitFor(() => expect(rowNames('Nội bộ kế toán')).toContain('Vận hành chung'))
    expect(screen.queryByRole('list', { name: 'Nhóm' })).toBeNull()
    expect(server.calls.find((c) => c.method === 'PUT')?.path).toBe(`/noibo/channels/${CH.vanhanh}`)
    await waitFor(() => expect(toastMessages()).toContain('Đã chuyển Vận hành chung vào “Nội bộ kế toán”'))
  })

  it('opens the destination if it was collapsed', async () => {
    const user = userEvent.setup()
    await ready()
    await user.click(screen.getByRole('button', { name: 'Nội bộ kế toán' }))
    expect(screen.getByRole('button', { name: 'Nội bộ kế toán' })).toHaveAttribute('aria-expanded', 'false')
    const menu = await rowMenu(user, 'Vận hành chung')
    await user.click(within(menu).getByRole('menuitemradio', { name: 'Nội bộ kế toán' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Nội bộ kế toán' })).toHaveAttribute('aria-expanded', 'true'))
  })

  it('"Mục mới…" names a new section and puts the conversation in it', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await rowMenu(user, 'Vận hành chung')
    await user.click(within(menu).getByRole('menuitem', { name: 'Mục mới…' }))
    const dialog = await screen.findByRole('dialog', { name: 'Tên mục mới' })
    await user.type(within(dialog).getByLabelText('Tên mục mới'), 'Việc gấp{Enter}')
    await waitFor(() => expect(rowNames('Việc gấp')).toEqual(['Vận hành chung']))
    expect(server.calls.find((c) => c.method === 'POST')?.body).toMatchObject({ name: 'Việc gấp', channel_ids: [CH.vanhanh] })
    await waitFor(() => expect(toastMessages()).toContain('Đã tạo mục “Việc gấp” và chuyển Vận hành chung vào'))
  })

  it('"Bỏ khỏi mục" sends the row back to its default list', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await rowMenu(user, 'Hợp đồng đối tác')
    await user.click(within(menu).getByRole('menuitem', { name: 'Bỏ khỏi mục' }))
    await waitFor(() => expect(rowNames('Nhóm')).toContain('Hợp đồng đối tác'))
    expect(screen.getByRole('button', { name: 'Nội bộ kế toán' })).toBeInTheDocument()
    await waitFor(() => expect(toastMessages()).toContain('Đã bỏ Hợp đồng đối tác khỏi “Nội bộ kế toán”'))
  })

  it('puts the row back and tells the person when the move is refused', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await rowMenu(user, 'Vận hành chung')
    server.failNext('/noibo/channels', 403)
    await user.click(within(menu).getByRole('menuitemradio', { name: 'Nội bộ kế toán' }))
    await waitFor(() => expect(rowNames('Nhóm')).toEqual(['Vận hành chung']))
    expect(useToastStore.getState().toasts.some((t) => t.tone === 'error')).toBe(true)
  })

  it('walks the menu with the arrow keys', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await rowMenu(user, 'Vận hành chung')
    await waitFor(() => expect(within(menu).getByRole('menuitemradio', { name: 'Quyết toán quý 4' })).toHaveFocus())
    await user.keyboard('{ArrowDown}')
    expect(within(menu).getByRole('menuitemradio', { name: 'Nội bộ kế toán' })).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(within(menu).getByRole('menuitem', { name: 'Mục mới…' })).toHaveFocus()
  })
})

describe('dragging a row onto a section', () => {
  const dataTransfer = () => ({ setData: vi.fn(), getData: vi.fn(), effectAllowed: '', dropEffect: '' })

  it('moves the conversation and opens the section', async () => {
    await ready()
    const row = screen.getByRole('link', { name: /Vận hành chung/ }).closest('li')!
    const header = screen.getByRole('button', { name: 'Nội bộ kế toán' }).closest('div')!
    const dt = dataTransfer()
    fireEvent.dragStart(row, { dataTransfer: dt })
    fireEvent.dragOver(header, { dataTransfer: dt })
    fireEvent.drop(header, { dataTransfer: dt })
    fireEvent.dragEnd(row, { dataTransfer: dt })
    await waitFor(() => expect(rowNames('Nội bộ kế toán')).toContain('Vận hành chung'))
    // The dragged payload names the conversation, never its id.
    expect(JSON.stringify(dt.setData.mock.calls)).not.toMatch(UUID_RE)
  })

  it('ignores a drop that carries no row', async () => {
    await ready()
    const header = screen.getByRole('button', { name: 'Nội bộ kế toán' }).closest('div')!
    fireEvent.drop(header, { dataTransfer: dataTransfer() })
    expect(server.calls.filter((c) => c.method === 'PUT')).toHaveLength(0)
  })
})

describe('the menu of a section', () => {
  const sectionMenu = async (user: ReturnType<typeof userEvent.setup>, name: string) => {
    await user.click(screen.getByRole('button', { name: `Tuỳ chọn cho mục ${name}` }))
    return screen.findByRole('menu', { name: `Tuỳ chọn cho mục ${name}` })
  }
  const order = () => screen.getAllByRole('list').map((l) => l.getAttribute('aria-label')).slice(0, 2)

  it('offers rename, move up, move down and delete; the first cannot go up, the last cannot go down', async () => {
    const user = userEvent.setup()
    await ready()
    let menu = await sectionMenu(user, 'Quyết toán quý 4')
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent))
      .toEqual(['Đổi tên mục', 'Chuyển lên', 'Chuyển xuống', 'Xoá mục'])
    expect(within(menu).getByRole('menuitem', { name: 'Chuyển lên' })).toBeDisabled()
    expect(within(menu).getByRole('menuitem', { name: 'Chuyển xuống' })).toBeEnabled()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())

    menu = await sectionMenu(user, 'Nội bộ kế toán')
    expect(within(menu).getByRole('menuitem', { name: 'Chuyển xuống' })).toBeDisabled()
    expect(within(menu).getByRole('menuitem', { name: 'Chuyển lên' })).toBeEnabled()
  })

  it('moves a section down by keyboard alone and announces it', async () => {
    const user = userEvent.setup()
    await ready()
    const trigger = screen.getByRole('button', { name: 'Tuỳ chọn cho mục Quyết toán quý 4' })
    trigger.focus()
    await user.keyboard('{Enter}')
    const menu = await screen.findByRole('menu')
    await waitFor(() => expect(within(menu).getByRole('menuitem', { name: 'Đổi tên mục' })).toHaveFocus())
    // Chuyển lên is disabled, so the arrow keys step over it.
    await user.keyboard('{ArrowDown}')
    expect(within(menu).getByRole('menuitem', { name: 'Chuyển xuống' })).toHaveFocus()
    await user.keyboard('{Enter}')
    await waitFor(() => expect(order()).toEqual(['Nội bộ kế toán', 'Quyết toán quý 4']))
    expect(await screen.findByText('Đã chuyển mục “Quyết toán quý 4” xuống')).toBeInTheDocument()
    expect(server.calls.find((c) => c.method === 'PATCH')?.body).toEqual({ move: 'down' })
  })

  it('renames in the popover, starting from the current name', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await sectionMenu(user, 'Nội bộ kế toán')
    await user.click(within(menu).getByRole('menuitem', { name: 'Đổi tên mục' }))
    const dialog = await screen.findByRole('dialog', { name: 'Đổi tên mục' })
    const field = within(dialog).getByLabelText('Đổi tên mục')
    expect(field).toHaveValue('Nội bộ kế toán')
    await user.clear(field)
    await user.type(field, 'Nội bộ{Enter}')
    await screen.findByRole('button', { name: 'Nội bộ' })
    expect(server.calls.find((c) => c.method === 'PATCH')?.body).toEqual({ name: 'Nội bộ' })
    expect(toastMessages()).toContain('Đã đổi tên mục thành “Nội bộ”')
  })

  it('refuses to rename to another section\'s name', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await sectionMenu(user, 'Nội bộ kế toán')
    await user.click(within(menu).getByRole('menuitem', { name: 'Đổi tên mục' }))
    const dialog = await screen.findByRole('dialog', { name: 'Đổi tên mục' })
    const field = within(dialog).getByLabelText('Đổi tên mục')
    await user.clear(field)
    await user.type(field, 'quyết toán quý 4{Enter}')
    expect(await within(dialog).findByText('Bạn đã có mục tên này')).toBeInTheDocument()
  })

  it('deletes without asking, sends the rows home, and Hoàn tác brings the section back in place', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await sectionMenu(user, 'Quyết toán quý 4')
    await user.click(within(menu).getByRole('menuitem', { name: 'Xoá mục' }))

    await waitFor(() => expect(screen.queryByRole('list', { name: 'Quyết toán quý 4' })).toBeNull())
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(rowNames('Nhóm')).toContain('Đối soát giao dịch')
    expect(rowNames('Tin nhắn trực tiếp')).toContain('Nguyễn Thu Lan')

    const toast = await waitFor(() => {
      const t = useToastStore.getState().toasts.find((x) => x.action?.label === 'Hoàn tác')
      expect(t).toBeDefined()
      return t!
    })
    expect(toast.message).toBe('Đã xoá mục “Quyết toán quý 4”, cuộc trò chuyện trở về Tin nhắn trực tiếp và Nhóm')
    expect(toast.duration).toBe(5000)

    await act(async () => { toast.action!.onClick() })
    await waitFor(() => expect(rowNames('Quyết toán quý 4').sort()).toEqual(['Nguyễn Thu Lan', 'Đối soát giao dịch'].sort()))
    expect(screen.getAllByRole('list').map((l) => l.getAttribute('aria-label')).slice(0, 2))
      .toEqual(['Quyết toán quý 4', 'Nội bộ kế toán'])
    expect(last(server.calls.filter((c) => c.method === 'POST'))?.body)
      .toMatchObject({ name: 'Quyết toán quý 4', position: 0, channel_ids: [CH.doisoat, CH.dmLan] })
  })

  it('keeps the section when the server refuses the delete', async () => {
    const user = userEvent.setup()
    await ready()
    const menu = await sectionMenu(user, 'Quyết toán quý 4')
    server.failNext('/q4', 500)
    await user.click(within(menu).getByRole('menuitem', { name: 'Xoá mục' }))
    await waitFor(() => expect(useToastStore.getState().toasts.some((t) => t.tone === 'error')).toBe(true))
    expect(screen.getByRole('list', { name: 'Quyết toán quý 4' })).toBeInTheDocument()
  })
})

describe('an empty section', () => {
  it('tells where to move a conversation from', async () => {
    const user = userEvent.setup()
    await ready()
    await openCreate(user)
    await user.type(within(await screen.findByRole('dialog')).getByLabelText('Tên mục mới'), 'Trống{Enter}')
    expect(await screen.findByText(/Chưa có cuộc trò chuyện\. Mở menu ⋯ của một cuộc trò chuyện để chuyển vào đây\./)).toBeInTheDocument()
  })
})

// A refused call that is not a name clash falls back to the shared wording.
describe('failure wording', () => {
  it('explains a failed create in the popover instead of a toast', async () => {
    const user = userEvent.setup()
    await ready()
    await openCreate(user)
    const dialog = await screen.findByRole('dialog')
    server.failNext('', 500)
    await user.type(within(dialog).getByLabelText('Tên mục mới'), 'Mới{Enter}')
    expect(await within(dialog).findByText(/Máy chủ đang gặp sự cố/)).toBeInTheDocument()
  })
})
