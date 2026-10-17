import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CategoryDef } from './contacts-model'
import { ContactsChips } from './ContactsChips'

afterEach(cleanup)

const CATS: CategoryDef[] = [
  { id: 'org', title: 'Phòng ban', sub: 'NovaPay · 3 khối', hue: 1 },
  { id: 'roles', title: 'Vai trò', sub: '4 vai trò', hue: 3 },
  { id: 'groups', title: 'Nhóm của tôi', sub: '2 nhóm', hue: 5 },
]

describe('ContactsChips', () => {
  it('replaces the left column with one chip per category, in the same order, naming only the category', () => {
    render(<ContactsChips categories={CATS} current="org" onOpen={() => {}} />)
    const chips = within(screen.getByRole('group', { name: 'Danh mục' })).getAllByRole('button')
    expect(chips.map((c) => c.textContent)).toEqual(['Phòng ban', 'Vai trò', 'Nhóm của tôi'])
  })

  it('marks the open category as pressed and opens another on a press', async () => {
    const onOpen = vi.fn()
    render(<ContactsChips categories={CATS} current="roles" onOpen={onOpen} />)
    expect(screen.getByRole('button', { name: 'Vai trò' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Phòng ban' })).toHaveAttribute('aria-pressed', 'false')
    await userEvent.setup().click(screen.getByRole('button', { name: 'Nhóm của tôi' }))
    expect(onOpen).toHaveBeenCalledWith('groups')
  })

  it('has chips tall enough to press (36), and scrolls sideways inside itself rather than the page', () => {
    render(<ContactsChips categories={CATS} current="org" onOpen={() => {}} />)
    expect(screen.getByRole('button', { name: 'Phòng ban' }).className).toMatch(/\bh-9\b/)
    expect(screen.getByRole('group', { name: 'Danh mục' }).className).toMatch(/overflow-x-auto/)
  })

  it('has Phòng của tôi after the categories when there is a department to go to, pressed while it is the page', async () => {
    const onOpenMine = vi.fn()
    const mine = { id: 'd1', name: 'Kế toán' }
    const { rerender } = render(<ContactsChips categories={CATS} current="org" onOpen={() => {}} myDepartment={mine} onOpenMine={onOpenMine} />)
    const names = within(screen.getByRole('group', { name: 'Danh mục' })).getAllByRole('button').map((b) => b.textContent)
    expect(names).toEqual(['Phòng ban', 'Vai trò', 'Nhóm của tôi', 'Phòng của tôi'])
    await userEvent.setup().click(screen.getByRole('button', { name: 'Phòng của tôi' }))
    expect(onOpenMine).toHaveBeenCalled()
    rerender(<ContactsChips categories={CATS} current="org" onOpen={() => {}} myDepartment={mine} mineIsCurrent onOpenMine={onOpenMine} />)
    expect(screen.getByRole('button', { name: 'Phòng của tôi' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Phòng ban' })).toHaveAttribute('aria-pressed', 'false')
  })
})
