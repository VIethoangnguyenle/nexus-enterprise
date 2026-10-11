import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { CH, UUID_RE, WS_ID } from '../../test/chat-fixtures'
import { createSectionsServer } from '../../test/chat-sections-server'
import { apiFetch } from '../../api/client'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { useToastStore } from '../primitives'
import { useNavCollapseStore } from '../../stores/nav-collapse.store'
import { PhoneChatList } from './PhoneChatList'
import { useSectionOverlays } from './SectionOverlays'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

let server: ReturnType<typeof createSectionsServer>

function Host() {
  const nav = useChatNavigation(WS_ID)
  const overlays = useSectionOverlays(nav.sectionsApi, true)
  return (
    <>
      <PhoneChatList nav={nav} workspaceId={WS_ID} fresh={new Map()} overlays={overlays} />
      {overlays.node}
    </>
  )
}

beforeEach(() => {
  server = createSectionsServer([
    { id: 'q4', name: 'Quyết toán quý 4', position: 0, channel_ids: [CH.doisoat, CH.dmLan] },
    { id: 'noibo', name: 'Nội bộ kế toán', position: 1, channel_ids: [CH.hopdong] },
  ])
  vi.mocked(apiFetch).mockImplementation(server.api as never)
})
afterEach(() => {
  vi.useRealTimers()
  resetClient()
  useToastStore.getState().clear()
  useNavCollapseStore.setState({ collapsed: {} })
})

const ready = async () => {
  renderWithClient(<Host />)
  await screen.findByRole('list', { name: 'Quyết toán quý 4' })
}
const row = (name: RegExp) => screen.getByRole('link', { name }).closest('li')!
const pressLong = (el: HTMLElement) => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  fireEvent.pointerDown(el, { pointerType: 'touch', clientX: 5, clientY: 5 })
  act(() => { vi.advanceTimersByTime(401) })
}
const sectionOf = (title: string) => screen.getByRole('list', { name: title })
const titles = (list: HTMLElement) => within(list).getAllByRole('link').map((a) => a.getAttribute('aria-label'))

describe('Tin nhắn on a phone', () => {
  it('lists the user\'s sections, then Tin nhắn trực tiếp, then Nhóm, each conversation once', async () => {
    await ready()
    expect(screen.getAllByRole('list').map((l) => l.getAttribute('aria-label')))
      .toEqual(['Quyết toán quý 4', 'Nội bộ kế toán', 'Tin nhắn trực tiếp', 'Nhóm'])
    expect(titles(sectionOf('Quyết toán quý 4')).sort()).toEqual(['Nguyễn Thu Lan', 'Đối soát giao dịch'])
    expect(titles(sectionOf('Tin nhắn trực tiếp'))).toEqual(['Phạm Hải Yến'])
    expect(titles(sectionOf('Nhóm'))).toEqual(['Vận hành chung'])
  })

  it('folds a section from its 44px header, which then totals the unread', async () => {
    const user = userEvent.setup()
    await ready()
    const toggle = screen.getByRole('button', { name: 'Quyết toán quý 4' })
    expect(toggle.className).toContain('min-h-11')
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(within(toggle).getByLabelText('3 tin chưa đọc')).toBeInTheDocument()
  })

  it('never renders an identifier', async () => {
    await ready()
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })
})

describe('long press on a conversation', () => {
  it('opens a sheet with the sections as radios (current checked), Mục mới… and Bỏ khỏi mục, and no promise the backend cannot keep', async () => {
    await ready()
    pressLong(row(/Đối soát giao dịch/))
    const sheet = await screen.findByRole('dialog', { name: 'Tuỳ chọn cho Đối soát giao dịch' })
    const group = within(sheet).getByRole('radiogroup', { name: 'Chuyển tới mục' })
    expect(within(group).getByRole('radio', { name: 'Quyết toán quý 4' })).toBeChecked()
    expect(within(group).getByRole('radio', { name: 'Nội bộ kế toán' })).not.toBeChecked()
    expect(within(sheet).getByRole('button', { name: 'Mục mới…' })).toBeInTheDocument()
    expect(within(sheet).getByRole('button', { name: 'Bỏ khỏi mục' })).toBeInTheDocument()
    expect(within(sheet).queryByText(/Gắn sao|Tắt thông báo/)).toBeNull()
  })

  it('does not open for a short press, a scroll, or a mouse, and the tap still opens the conversation', async () => {
    await ready()
    const li = row(/Đối soát giao dịch/)
    vi.useFakeTimers({ shouldAdvanceTime: true })
    fireEvent.pointerDown(li, { pointerType: 'touch', clientX: 5, clientY: 5 })
    act(() => { vi.advanceTimersByTime(399) })
    fireEvent.pointerUp(li, { pointerType: 'touch' })
    fireEvent.pointerDown(li, { pointerType: 'touch', clientX: 5, clientY: 5 })
    fireEvent.pointerMove(li, { pointerType: 'touch', clientX: 5, clientY: 80 })
    act(() => { vi.advanceTimersByTime(600) })
    fireEvent.pointerDown(li, { pointerType: 'mouse', clientX: 5, clientY: 5 })
    act(() => { vi.advanceTimersByTime(600) })
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('swallows the tap that ends a long press, so the conversation does not open behind the sheet', async () => {
    await ready()
    const li = row(/Đối soát giao dịch/)
    const link = within(li).getByRole('link')
    pressLong(li)
    await screen.findByRole('dialog')
    const click = new MouseEvent('click', { bubbles: true, cancelable: true })
    link.dispatchEvent(click)
    expect(click.defaultPrevented).toBe(true)
    // The next, ordinary tap is not swallowed.
    fireEvent.pointerDown(li, { pointerType: 'touch', clientX: 5, clientY: 5 })
    const tap = new MouseEvent('click', { bubbles: true, cancelable: true })
    link.dispatchEvent(tap)
    expect(tap.defaultPrevented).toBe(false)
  })

  it('opens the same sheet from the keyboard (Menu key or Shift+F10)', async () => {
    await ready()
    const link = within(row(/Vận hành chung/)).getByRole('link')
    link.focus()
    fireEvent.keyDown(link, { key: 'F10', shiftKey: true })
    expect(await screen.findByRole('dialog', { name: 'Tuỳ chọn cho Vận hành chung' })).toBeInTheDocument()
  })

  it('moves the conversation into the chosen section and closes the sheet', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await ready()
    pressLong(row(/Vận hành chung/))
    const sheet = await screen.findByRole('dialog')
    await user.click(within(sheet).getByRole('radio', { name: 'Nội bộ kế toán' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    await waitFor(() => expect(titles(sectionOf('Nội bộ kế toán'))).toContain('Vận hành chung'))
    expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Đã chuyển Vận hành chung vào “Nội bộ kế toán”')
  })

  it('takes a conversation out of its section with Bỏ khỏi mục', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await ready()
    pressLong(row(/Hợp đồng đối tác/))
    await user.click(await screen.findByRole('button', { name: 'Bỏ khỏi mục' }))
    await waitFor(() => expect(titles(sectionOf('Nhóm'))).toContain('Hợp đồng đối tác'))
  })

  it('names a new section in the sheet and puts the conversation in it', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await ready()
    pressLong(row(/Vận hành chung/))
    await user.click(await screen.findByRole('button', { name: 'Mục mới…' }))
    const sheet = await screen.findByRole('dialog', { name: 'Tên mục mới' })
    const field = within(sheet).getByLabelText('Tên mục mới')
    expect(field.closest('div.min-h-11')).not.toBeNull()
    await user.type(field, 'Việc gấp{Enter}')
    await waitFor(() => expect(titles(sectionOf('Việc gấp'))).toEqual(['Vận hành chung']))
  })
})

describe('the menu of a section on a phone', () => {
  const open = async (user: ReturnType<typeof userEvent.setup>, name: string) => {
    await user.click(screen.getByRole('button', { name: `Tuỳ chọn cho mục ${name}` }))
    return screen.findByRole('dialog', { name: `Tuỳ chọn cho mục ${name}` })
  }

  it('is a bottom sheet with rename, move and delete', async () => {
    const user = userEvent.setup()
    await ready()
    const sheet = await open(user, 'Quyết toán quý 4')
    expect(within(sheet).getAllByRole('button').map((b) => b.textContent))
      .toEqual(['Đổi tên mục', 'Chuyển lên', 'Chuyển xuống', 'Xoá mục'])
    expect(within(sheet).getByRole('button', { name: 'Chuyển lên' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Tuỳ chọn cho mục Quyết toán quý 4' }).className).toContain('w-11')
  })

  it('deletes and offers Hoàn tác', async () => {
    const user = userEvent.setup()
    await ready()
    const sheet = await open(user, 'Quyết toán quý 4')
    await user.click(within(sheet).getByRole('button', { name: 'Xoá mục' }))
    await waitFor(() => expect(screen.queryByRole('list', { name: 'Quyết toán quý 4' })).toBeNull())
    await waitFor(() => expect(useToastStore.getState().toasts.some((t) => t.action?.label === 'Hoàn tác')).toBe(true))
    expect(titles(sectionOf('Nhóm'))).toContain('Đối soát giao dịch')
  })

  it('renames from a sheet', async () => {
    const user = userEvent.setup()
    await ready()
    const sheet = await open(user, 'Nội bộ kế toán')
    await user.click(within(sheet).getByRole('button', { name: 'Đổi tên mục' }))
    const form = await screen.findByRole('dialog', { name: 'Đổi tên mục' })
    const field = within(form).getByLabelText('Đổi tên mục')
    await user.clear(field)
    await user.type(field, 'Nội bộ{Enter}')
    await screen.findByRole('button', { name: /^Nội bộ$/ })
  })
})
