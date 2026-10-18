import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Contact } from '../../hooks/useContacts'
import { LONG_PRESS_MS } from '../../hooks/useLongPress'
import type { CategoryDef } from './contacts-model'
import { ContactsCategoryHome } from './ContactsCategoryHome'

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

// The phone's first screen only exists below 768px.
vi.mock('../../hooks/usePhone', () => ({ usePhone: () => true, useBottomBarLayout: () => true }))

const CATS: CategoryDef[] = [
  { id: 'org', title: 'Phòng ban', sub: 'NovaPay · 3 khối', hue: 1 },
  { id: 'roles', title: 'Vai trò', sub: '4 vai trò', hue: 3 },
  { id: 'groups', title: 'Nhóm của tôi', sub: '2 nhóm', hue: 5 },
]
const PERSONAL: CategoryDef[] = [
  { id: 'members', title: 'Thành viên', sub: '7 người trong không gian này', hue: 1 },
  { id: 'groups', title: 'Nhóm của tôi', sub: '2 nhóm', hue: 5 },
]
const MINE = { id: 'd1', name: 'Kế toán', path: 'Khối Tài chính', count: 6 }
const who = (n: number, name: string): Contact => ({
  user_id: `11111111-aaaa-4bbb-8ccc-00000000000${n}`, ngac_node_id: `22222222-aaaa-4bbb-8ccc-00000000000${n}`, username: `u${n}`,
  display_name: name, email: '', title: 'Kế toán viên', department: 'Kế toán', department_id: 'd1', roles: [], location: '', avatar_url: '', is_online: false,
})
const VINH = who(1, 'Lê Quang Vinh')
const HA = who(2, 'Phạm Thu Hà')
const nav = () => screen.getByRole('navigation', { name: 'Danh mục' })

function props(over: Partial<React.ComponentProps<typeof ContactsCategoryHome>> = {}): React.ComponentProps<typeof ContactsCategoryHome> {
  return {
    categories: CATS, onOpen: vi.fn(), myDepartment: MINE, onOpenMine: vi.fn(), onSearch: vi.fn(),
    frequent: [], online: [], isOnline: () => false, onOpenPerson: vi.fn(), onMessagePerson: vi.fn(), onSeeAllOnline: vi.fn(),
    canInvite: false, onInvite: vi.fn(), ...over,
  }
}

describe('ContactsCategoryHome (the phone\'s first screen)', () => {
  it('is the title and a two-column grid of categories, with Phòng của tôi second in an organization', () => {
    render(<ContactsCategoryHome {...props()} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
    const tiles = within(nav()).getAllByRole('button')
    expect(tiles.map((t) => t.textContent)).toEqual([
      expect.stringMatching(/^Phòng ban.*NovaPay · 3 khối/),
      expect.stringMatching(/^Phòng của tôi.*Kế toán · 6 người/),
      expect.stringMatching(/^Vai trò.*4 vai trò/),
      expect.stringMatching(/^Nhóm của tôi.*2 nhóm/),
    ])
    expect(nav().firstElementChild!.className).toMatch(/grid-cols-2/)
  })

  it('a personal workspace has Thành viên and Nhóm của tôi, and no department anywhere', () => {
    render(<ContactsCategoryHome {...props({ categories: PERSONAL, myDepartment: undefined })} />)
    expect(within(nav()).getAllByRole('button').map((t) => t.textContent)).toEqual([
      expect.stringMatching(/^Thành viên/), expect.stringMatching(/^Nhóm của tôi/),
    ])
    expect(document.body.textContent).not.toMatch(/Phòng ban|Phòng của tôi|Vai trò/)
  })

  it('opens a category, and Phòng của tôi opens the caller\'s department', async () => {
    const p = props()
    const user = userEvent.setup()
    render(<ContactsCategoryHome {...p} />)
    await user.click(within(nav()).getByRole('button', { name: /^Vai trò/ }))
    expect(p.onOpen).toHaveBeenCalledWith('roles')
    await user.click(within(nav()).getByRole('button', { name: /^Phòng của tôi/ }))
    expect(p.onOpenMine).toHaveBeenCalled()
  })

  it('Thường liên lạc: the people given, a tap opens the profile and a long press opens Nhắn tin; hidden when there is nobody', () => {
    vi.useFakeTimers()
    const p = props({ frequent: [VINH, HA] })
    const { unmount } = render(<ContactsCategoryHome {...p} />)
    const strip = screen.getByRole('list', { name: 'Thường liên lạc' })
    const vinh = within(strip).getByRole('button', { name: 'Lê Quang Vinh' })
    expect(vinh).toHaveTextContent('Quang Vinh')
    fireEvent.click(vinh)
    expect(p.onOpenPerson).toHaveBeenCalledWith(VINH)

    const ha = within(strip).getByRole('button', { name: 'Phạm Thu Hà' })
    fireEvent.pointerDown(ha, { pointerType: 'touch', clientX: 0, clientY: 0 })
    act(() => { vi.advanceTimersByTime(LONG_PRESS_MS + 10) })
    fireEvent.pointerUp(ha, { pointerType: 'touch' })
    fireEvent.click(ha)
    expect(p.onMessagePerson).toHaveBeenCalledWith(HA)
    expect(p.onOpenPerson).not.toHaveBeenCalledWith(HA)
    unmount()

    render(<ContactsCategoryHome {...props({ frequent: [] })} />)
    expect(screen.queryByText('Thường liên lạc')).toBeNull()
  })

  it('Đang trực tuyến · N: at most five people and Xem tất cả, none when nobody is online', async () => {
    const online = Array.from({ length: 7 }, (_, i) => who(i + 1, `Người Số ${String.fromCharCode(65 + i)}`))
    const p = props({ online, isOnline: () => true })
    const user = userEvent.setup()
    const { unmount } = render(<ContactsCategoryHome {...p} />)
    const section = screen.getByRole('region', { name: 'Đang trực tuyến · 7' })
    expect(within(section).getAllByRole('button', { name: /^Người Số/ })).toHaveLength(5)
    await user.click(within(section).getByRole('button', { name: 'Xem tất cả' }))
    expect(p.onSeeAllOnline).toHaveBeenCalled()
    unmount()
    render(<ContactsCategoryHome {...props()} />)
    expect(screen.queryByRole('region', { name: /Đang trực tuyến/ })).toBeNull()
  })

  it('⋯ offers Mời thành viên to an administrator only', async () => {
    const p = props({ canInvite: true })
    const user = userEvent.setup()
    const { unmount } = render(<ContactsCategoryHome {...p} />)
    await user.click(screen.getByRole('button', { name: 'Thêm tuỳ chọn Danh bạ' }))
    await user.click(within(await screen.findByRole('dialog', { name: 'Danh bạ' })).getByRole('button', { name: 'Mời thành viên' }))
    expect(p.onInvite).toHaveBeenCalled()
    unmount()
    render(<ContactsCategoryHome {...props({ canInvite: false })} />)
    expect(screen.queryByRole('button', { name: 'Thêm tuỳ chọn Danh bạ' })).toBeNull()
  })

  it('answers the top bar\'s search: a search control it can be pointed at, that opens the search screen', () => {
    const p = props()
    render(<ContactsCategoryHome {...p} />)
    const search = document.querySelector<HTMLElement>('[data-module-search]')!
    expect(search).not.toBeNull()
    search.click()
    expect(p.onSearch).toHaveBeenCalled()
  })
})
