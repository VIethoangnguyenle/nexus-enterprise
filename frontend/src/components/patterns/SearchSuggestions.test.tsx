import { useRef, useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button, SearchField } from '../primitives'
import { SearchSuggestions } from './SearchSuggestions'
import { useSuggestionPanel, type SuggestionGroup, type SuggestionVariant } from '../../hooks/useSuggestionPanel'
import { expectNoIds } from '../../test/no-ids'

interface HarnessProps {
  groups: SuggestionGroup[]
  variant?: SuggestionVariant
  emptyText?: string
}

/** A screen's search box with its suggestions, the way every module wires them. */
function Harness({ groups, variant = 'popover', emptyText }: HarnessProps) {
  const [value, setValue] = useState('')
  const anchorRef = useRef<HTMLInputElement>(null)
  const panel = useSuggestionPanel({ value, groups, variant, anchorRef })
  return (
    <>
      <SearchField ref={anchorRef} label="Tìm tài liệu" value={value} onChange={(e) => setValue(e.target.value)} {...panel.inputProps} />
      <Button type="button">Ở ngoài</Button>
      <Button type="button" onClick={panel.close}>Quay lại</Button>
      <SearchSuggestions panel={panel} emptyText={emptyText} />
    </>
  )
}

const recent = (words: string[], onSelect = vi.fn(), onRemove = vi.fn()): SuggestionGroup => ({
  label: 'Tìm gần đây',
  recent: true,
  items: words.map((w) => ({ key: w, title: w, onSelect: () => onSelect(w), onRemove: () => onRemove(w) })),
})

const files = (names: string[], onSelect = vi.fn(), label = 'Sửa gần đây'): SuggestionGroup => ({
  label,
  items: names.map((n) => ({ key: n, title: n, meta: 'Lê Quang Vinh · 09:12', onSelect: () => onSelect(n) })),
})

/** The desktop box is a combobox; the phone box stays a plain search box over a list of buttons. */
const box = () => screen.getByRole('combobox', { name: 'Tìm tài liệu' })
const phoneBox = () => screen.getByRole('searchbox', { name: 'Tìm tài liệu' })

afterEach(() => vi.restoreAllMocks())

describe('opening and closing', () => {
  it('shows nothing until the box is focused', () => {
    render(<Harness groups={[recent(['hợp đồng'])]} />)
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(box()).toHaveAttribute('aria-expanded', 'false')
    expect(box()).not.toHaveAttribute('aria-controls')
  })

  it('opens on focus while the box is empty, one named group per kind', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent(['hợp đồng']), files(['Đối soát T10.xlsx'])]} />)
    await user.click(box())
    const list = screen.getByRole('listbox', { name: 'Gợi ý tìm kiếm' })
    expect(box()).toHaveAttribute('aria-expanded', 'true')
    expect(box()).toHaveAttribute('aria-controls', list.id)
    expect(within(list).getByRole('group', { name: 'Tìm gần đây' })).toBeInTheDocument()
    const group = within(list).getByRole('group', { name: 'Sửa gần đây' })
    expect(within(group).getByRole('option', { name: /Đối soát T10\.xlsx/ })).toBeInTheDocument()
    expect(within(group).getByText('Lê Quang Vinh · 09:12')).toBeInTheDocument()
  })

  it('hides a group that has nothing in it, label and all', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent([]), files(['Sổ cái.xlsx'])]} />)
    await user.click(box())
    expect(screen.queryByText('Tìm gần đây')).toBeNull()
  })

  it('gives way to results once a letter is typed, and comes back when the box is emptied', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent(['hợp đồng'])]} />)
    await user.click(box())
    await user.keyboard('h')
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(box()).toHaveAttribute('aria-expanded', 'false')
    await user.keyboard('{Backspace}')
    expect(screen.getByRole('listbox')).toBeInTheDocument()
  })

  it('closes on Esc and keeps the focus in the box', async () => {
    const user = userEvent.setup()
    const outer = vi.fn()
    render(<div onKeyDown={(e) => e.key === 'Escape' && outer()}><Harness groups={[recent(['hợp đồng'])]} /></div>)
    await user.click(box())
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(box()).toHaveFocus()
    // A dialog behind the box must not close with it.
    expect(outer).not.toHaveBeenCalled()
  })

  it('closes when the box is left', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent(['hợp đồng'])]} />)
    await user.click(box())
    await user.click(screen.getByRole('button', { name: 'Ở ngoài' }))
    expect(screen.queryByRole('listbox')).toBeNull()
  })
})

describe('keyboard', () => {
  it('moves through every row across groups with ↑/↓, the box keeping focus', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent(['hợp đồng']), files(['A.xlsx', 'B.pdf'])]} />)
    await user.click(box())
    expect(box()).not.toHaveAttribute('aria-activedescendant')
    await user.keyboard('{ArrowDown}')
    const first = screen.getByRole('option', { name: /hợp đồng/ })
    expect(box()).toHaveAttribute('aria-activedescendant', first.id)
    expect(first).toHaveAttribute('aria-selected', 'true')
    await user.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}')
    expect(box()).toHaveAttribute('aria-activedescendant', first.id)
    await user.keyboard('{ArrowUp}')
    expect(box()).toHaveAttribute('aria-activedescendant', screen.getByRole('option', { name: /B\.pdf/ }).id)
    expect(box()).toHaveFocus()
  })

  it('starts from the last row with ↑', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[files(['A.xlsx', 'B.pdf'])]} />)
    await user.click(box())
    await user.keyboard('{ArrowUp}')
    expect(box()).toHaveAttribute('aria-activedescendant', screen.getByRole('option', { name: /B\.pdf/ }).id)
  })

  it('chooses the marked row on Enter and closes', async () => {
    const user = userEvent.setup()
    const pick = vi.fn()
    render(<Harness groups={[files(['A.xlsx', 'B.pdf'], pick)]} />)
    await user.click(box())
    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}')
    expect(pick).toHaveBeenCalledWith('B.pdf')
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('leaves Enter to the screen while no row is marked', async () => {
    const user = userEvent.setup()
    const pick = vi.fn()
    render(<Harness groups={[files(['A.xlsx'], pick)]} />)
    await user.click(box())
    await user.keyboard('{Enter}')
    expect(pick).not.toHaveBeenCalled()
  })

  it('removes the marked recent search with Delete', async () => {
    const user = userEvent.setup()
    const remove = vi.fn()
    render(<Harness groups={[recent(['một', 'hai'], vi.fn(), remove)]} />)
    await user.click(box())
    await user.keyboard('{ArrowDown}{Delete}')
    expect(remove).toHaveBeenCalledWith('một')
  })

  it('does nothing on Delete over a row that cannot be removed', async () => {
    const user = userEvent.setup()
    const pick = vi.fn()
    render(<Harness groups={[files(['A.xlsx'], pick)]} />)
    await user.click(box())
    await user.keyboard('{ArrowDown}{Delete}')
    expect(pick).not.toHaveBeenCalled()
    expect(screen.getByRole('option', { name: /A\.xlsx/ })).toHaveAttribute('aria-selected', 'true')
  })

  it('keeps the same row marked when the suggestions change under it', async () => {
    const user = userEvent.setup()
    const pick = vi.fn()
    const { rerender } = render(<Harness groups={[files(['A.xlsx', 'B.pdf'], pick)]} />)
    await user.click(box())
    await user.keyboard('{ArrowDown}{ArrowDown}')
    rerender(<Harness groups={[files(['Mới.docx', 'A.xlsx', 'B.pdf'], pick)]} />)
    expect(box()).toHaveAttribute('aria-activedescendant', screen.getByRole('option', { name: /B\.pdf/ }).id)
    await user.keyboard('{Enter}')
    expect(pick).toHaveBeenCalledWith('B.pdf')
  })

  it('marks nothing once the marked row is gone', async () => {
    const user = userEvent.setup()
    const pick = vi.fn()
    const { rerender } = render(<Harness groups={[files(['A.xlsx', 'B.pdf'], pick)]} />)
    await user.click(box())
    await user.keyboard('{ArrowDown}{ArrowDown}')
    rerender(<Harness groups={[files(['A.xlsx'], pick)]} />)
    expect(box()).not.toHaveAttribute('aria-activedescendant')
    await user.keyboard('{Enter}')
    expect(pick).not.toHaveBeenCalled()
  })
})

describe('pointer', () => {
  it('chooses a row on click without taking the focus from the box first', async () => {
    const user = userEvent.setup()
    const pick = vi.fn()
    render(<Harness groups={[recent(['hợp đồng'], pick)]} />)
    await user.click(box())
    await user.click(screen.getByRole('option', { name: /hợp đồng/ }))
    expect(pick).toHaveBeenCalledWith('hợp đồng')
  })

  it('removes a recent search with its ✕ without choosing it; the ✕ is for the pointer only', async () => {
    const user = userEvent.setup()
    const pick = vi.fn()
    const remove = vi.fn()
    render(<Harness groups={[recent(['hợp đồng'], pick, remove)]} />)
    await user.click(box())
    // Keyboard and screen reader users remove with Delete; the option's name is the search alone.
    expect(screen.queryByRole('button', { name: /Xoá/ })).toBeNull()
    expect(screen.getByRole('option', { name: 'hợp đồng' })).toBeInTheDocument()
    await user.click(screen.getByTitle('Xoá khỏi Tìm gần đây'))
    expect(remove).toHaveBeenCalledWith('hợp đồng')
    expect(pick).not.toHaveBeenCalled()
    expect(box()).toHaveFocus()
  })
})

describe('on the phone screen', () => {
  it('is a list of buttons under a plain search box', async () => {
    const user = userEvent.setup()
    render(<Harness variant="screen" groups={[recent(['hợp đồng']), files(['A.xlsx'])]} />)
    await user.click(phoneBox())
    expect(screen.queryByRole('combobox')).toBeNull()
    expect(screen.queryByRole('listbox')).toBeNull()
    const group = screen.getByRole('group', { name: 'Tìm gần đây' })
    expect(within(group).getByRole('button', { name: 'Tìm lại hợp đồng' })).toBeInTheDocument()
    expect(within(group).getByRole('button', { name: 'Xoá “hợp đồng” khỏi Tìm gần đây' })).toBeInTheDocument()
  })

  it('chooses with a tap and removes with ✕, the box keeping focus', async () => {
    const user = userEvent.setup()
    const pick = vi.fn()
    const remove = vi.fn()
    render(<Harness variant="screen" groups={[recent(['một', 'hai'], pick, remove)]} />)
    await user.click(phoneBox())
    await user.click(screen.getByRole('button', { name: 'Xoá “hai” khỏi Tìm gần đây' }))
    expect(remove).toHaveBeenCalledWith('hai')
    expect(phoneBox()).toHaveFocus()
    await user.click(screen.getByRole('button', { name: 'Tìm lại một' }))
    expect(pick).toHaveBeenCalledWith('một')
  })

  it('stays up when the keyboard is put away, until Quay lại', async () => {
    const user = userEvent.setup()
    render(<Harness variant="screen" groups={[recent(['hợp đồng'])]} />)
    await user.click(phoneBox())
    await user.click(screen.getByRole('button', { name: 'Ở ngoài' }))
    expect(screen.getByRole('button', { name: 'Tìm lại hợp đồng' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Quay lại' }))
    expect(screen.queryByRole('button', { name: 'Tìm lại hợp đồng' })).toBeNull()
  })
})

describe('how much is shown', () => {
  const many = (n: number, prefix: string) => Array.from({ length: n }, (_, i) => `${prefix} ${i + 1}`)

  it('shows at most 5 recent searches and 3 of anything else on a desktop', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent(many(8, 'tìm')), files(many(6, 'tệp'))]} />)
    await user.click(box())
    expect(within(screen.getByRole('group', { name: 'Tìm gần đây' })).getAllByRole('option')).toHaveLength(5)
    expect(within(screen.getByRole('group', { name: 'Sửa gần đây' })).getAllByRole('option')).toHaveLength(3)
  })

  it('shows 8 recent searches and 5 of anything else on the phone screen', async () => {
    const user = userEvent.setup()
    render(<Harness variant="screen" groups={[recent(many(8, 'tìm')), files(many(6, 'tệp'))]} />)
    await user.click(phoneBox())
    expect(screen.getAllByRole('button', { name: /^Tìm lại/ })).toHaveLength(8)
    expect(within(screen.getByRole('group', { name: 'Sửa gần đây' })).getAllByRole('button')).toHaveLength(5)
  })

  it('keeps two groups with the same label apart', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[files(['A.xlsx'], vi.fn(), 'Tệp'), files(['B.pdf'], vi.fn(), 'Tệp')]} />)
    await user.click(box())
    expect(screen.getAllByRole('group', { name: 'Tệp' })).toHaveLength(2)
  })
})

describe('placement', () => {
  it('stays inside the window and never narrower than the box', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(1024)
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(500)
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(
      { left: 900, top: 100, right: 1000, bottom: 136, width: 100, height: 36, x: 900, y: 100, toJSON: () => ({}) },
    )
    render(<Harness groups={[recent(['hợp đồng'])]} />)
    await user.click(box())
    const surface = screen.getByRole('listbox').parentElement!
    expect(surface.style.width).toBe('380px')
    expect(surface.style.left).toBe(`${1024 - 12 - 380}px`)
    expect(surface.style.top).toBe('142px')
    expect(surface.style.maxHeight).toBe(`${500 - 142 - 12}px`)
  })
})

describe('states', () => {
  it('keeps the place of a group still loading with skeleton rows', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent(['hợp đồng']), { label: 'Được chia sẻ với tôi', items: [], loading: true }]} />)
    await user.click(box())
    const group = screen.getByRole('group', { name: 'Được chia sẻ với tôi' })
    expect(group).toHaveAttribute('aria-busy', 'true')
    expect(within(group).queryAllByRole('option')).toHaveLength(0)
  })

  it('opens no popover when there is nothing to suggest', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent([]), files([])]} />)
    await user.click(box())
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(box()).toHaveAttribute('aria-expanded', 'false')
  })

  it('says how to search on the phone screen when there is nothing to suggest', async () => {
    const user = userEvent.setup()
    render(<Harness variant="screen" emptyText="Gõ tên tệp hoặc người sửa để tìm." groups={[recent([])]} />)
    await user.click(phoneBox())
    expect(screen.getByText('Gõ tên tệp hoặc người sửa để tìm.')).toBeInTheDocument()
  })

  it('shows names, never an identifier', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent(['hợp đồng']), files(['Đối soát T10.xlsx'])]} />)
    await user.click(box())
    expectNoIds()
  })
})
