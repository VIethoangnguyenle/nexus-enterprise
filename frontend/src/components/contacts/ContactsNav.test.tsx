import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CategoryDef } from './contacts-model'
import { ContactsNav } from './ContactsNav'

afterEach(cleanup)

const CATS: CategoryDef[] = [
  { id: 'org', title: 'Phòng ban', sub: 'NovaPay · 3 khối', hue: 1 },
  { id: 'roles', title: 'Vai trò', sub: '4 vai trò', hue: 3 },
  { id: 'groups', title: 'Nhóm của tôi', sub: '2 nhóm', hue: 5 },
]
const MINE = { id: 'd1', name: 'Kế toán', path: 'Khối Tài chính › Kế toán', count: 6 }

function setup(over: Partial<React.ComponentProps<typeof ContactsNav>> = {}) {
  const p = { categories: CATS, current: 'org' as const, onOpen: vi.fn(), myDepartment: MINE, onOpenMine: vi.fn(), ...over }
  render(<ContactsNav {...p} />)
  return p
}
const nav = () => screen.getByRole('navigation', { name: 'Danh mục' })

describe('ContactsNav', () => {
  it('is the Danh bạ title and the categories, each with what is inside', () => {
    setup()
    expect(screen.getByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
    const items = within(nav()).getAllByRole('button')
    expect(items.map((b) => b.textContent)).toEqual([
      expect.stringMatching(/^Phòng ban.*NovaPay · 3 khối/),
      expect.stringMatching(/^Vai trò.*4 vai trò/),
      expect.stringMatching(/^Nhóm của tôi.*2 nhóm/),
      expect.stringMatching(/^Kế toán.*Khối Tài chính.*6/),
    ])
  })

  it('marks the open category as the current page, and opens another on a press', async () => {
    const p = setup({ current: 'roles' })
    expect(within(nav()).getByRole('button', { name: /^Vai trò/ })).toHaveAttribute('aria-current', 'page')
    expect(within(nav()).getByRole('button', { name: /^Phòng ban/ })).not.toHaveAttribute('aria-current')
    await userEvent.setup().click(within(nav()).getByRole('button', { name: /^Nhóm của tôi/ }))
    expect(p.onOpen).toHaveBeenCalledWith('groups')
  })

  it('offers Phòng của tôi under its own label, and opens it', async () => {
    const p = setup()
    expect(nav()).toHaveTextContent('Phòng của tôi')
    await userEvent.setup().click(within(nav()).getByRole('button', { name: /^Kế toán/ }))
    expect(p.onOpenMine).toHaveBeenCalled()
  })

  it('has no shortcut for someone with no department, or in a personal workspace', () => {
    setup({ myDepartment: undefined })
    expect(nav()).not.toHaveTextContent('Phòng của tôi')
  })

  it('says under Phòng ban when the departments failed, with Thử lại', async () => {
    const onRetryTree = vi.fn()
    setup({ treeFailed: true, onRetryTree })
    expect(within(nav()).getByRole('alert')).toHaveTextContent('Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm.')
    await userEvent.setup().click(within(nav()).getByRole('button', { name: 'Thử lại' }))
    expect(onRetryTree).toHaveBeenCalled()
  })
})
