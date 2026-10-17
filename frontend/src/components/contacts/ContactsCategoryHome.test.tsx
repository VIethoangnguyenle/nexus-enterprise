import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CategoryDef } from './contacts-model'
import { ContactsCategoryHome } from './ContactsCategoryHome'

afterEach(cleanup)

const CATS: CategoryDef[] = [
  { id: 'org', title: 'Phòng ban', sub: 'NovaPay · 3 khối', hue: 1 },
  { id: 'roles', title: 'Vai trò', sub: '4 vai trò', hue: 3 },
  { id: 'groups', title: 'Nhóm của tôi', sub: '2 nhóm', hue: 5 },
]
const MINE = { id: 'd1', name: 'Kế toán', path: 'Khối Tài chính › Kế toán', count: 6 }
const nav = () => screen.getByRole('navigation', { name: 'Danh mục' })

describe('ContactsCategoryHome', () => {
  it('is the title and one 64 high tile per category, with a chevron, and nothing else: no people, no tree', () => {
    render(<ContactsCategoryHome categories={CATS} onOpen={() => {}} myDepartment={MINE} onOpenMine={() => {}} onSearch={() => {}} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
    const tiles = within(nav()).getAllByRole('button')
    expect(tiles.map((t) => t.textContent)).toEqual([
      expect.stringMatching(/^Phòng ban.*NovaPay · 3 khối/),
      expect.stringMatching(/^Vai trò.*4 vai trò/),
      expect.stringMatching(/^Nhóm của tôi.*2 nhóm/),
      expect.stringMatching(/^Kế toán.*Khối Tài chính/),
    ])
    expect(tiles[0]!.className).toMatch(/min-h-16/)
  })

  it('opens a category, and the shortcut opens the caller\'s department', async () => {
    const onOpen = vi.fn()
    const onOpenMine = vi.fn()
    const user = userEvent.setup()
    render(<ContactsCategoryHome categories={CATS} onOpen={onOpen} myDepartment={MINE} onOpenMine={onOpenMine} onSearch={() => {}} />)
    await user.click(within(nav()).getByRole('button', { name: /^Vai trò/ }))
    expect(onOpen).toHaveBeenCalledWith('roles')
    await user.click(within(nav()).getByRole('button', { name: /^Kế toán/ }))
    expect(onOpenMine).toHaveBeenCalled()
  })

  it('has no shortcut when there is no department', () => {
    render(<ContactsCategoryHome categories={CATS} onOpen={() => {}} onSearch={() => {}} />)
    expect(nav()).not.toHaveTextContent('Phòng của tôi')
  })

  it('answers the top bar\'s search: a search control it can be pointed at, that opens the search screen', () => {
    const onSearch = vi.fn()
    render(<ContactsCategoryHome categories={CATS} onOpen={() => {}} onSearch={onSearch} />)
    const search = document.querySelector<HTMLElement>('[data-module-search]')!
    expect(search).not.toBeNull()
    search.click()
    expect(onSearch).toHaveBeenCalled()
  })
})
