import { useRef, useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button, useToastStore } from '../primitives'
import { SearchBox } from './SearchBox'
import { SearchSuggestions } from './SearchSuggestions'
import { switchPreferencesTo, updatePreferences } from '../../lib/preferences'
import { useRecentSearches } from '../../hooks/useRecentSearches'
import { useAuthStore } from '../../stores/auth.store'
import { ME } from '../../test/chat-fixtures'
import { useSuggestionPanel, type SuggestionGroup, type SuggestionVariant } from '../../hooks/useSuggestionPanel'
import { expectNoIds } from '../../test/no-ids'

interface HarnessProps {
  groups: SuggestionGroup[]
  variant?: SuggestionVariant
  emptyText?: string
}

/** A screen's search box with its suggestions, the way every module wires them. */
function Harness({ groups, variant = 'popover', emptyText, initiallyOpen, showWhenEmpty }: HarnessProps & { initiallyOpen?: boolean; showWhenEmpty?: boolean }) {
  const [value, setValue] = useState('')
  const anchorRef = useRef<HTMLInputElement>(null)
  const panel = useSuggestionPanel({ value, groups, variant, anchorRef, initiallyOpen, showWhenEmpty })
  return (
    <>
      <SearchBox
        ref={anchorRef} panel={panel} label="Tìm tài liệu" emptyText={emptyText}
        value={value} onChange={(e) => setValue(e.target.value)} {...panel.inputProps}
      />
      <Button type="button">Ở ngoài</Button>
      <Button type="button" onClick={panel.close}>Quay lại</Button>
      {variant === 'screen' && <SearchSuggestions panel={panel} emptyText={emptyText} />}
    </>
  )
}

/** The same box with its Tìm gần đây kept the way a module keeps it, so removing really removes. */
function KeptHarness({ words }: { words: string[] }) {
  const [value, setValue] = useState('')
  const anchorRef = useRef<HTMLInputElement>(null)
  const recent = useRecentSearches('drive')
  useState(() => words.forEach((w) => recent.add(w)))
  const panel = useSuggestionPanel({ value, groups: [recent.group(setValue)], variant: 'popover', anchorRef })
  return (
    <>
      <SearchBox ref={anchorRef} panel={panel} label="Tìm tài liệu" value={value} onChange={(e) => setValue(e.target.value)} {...panel.inputProps} />
      <Button type="button">Ở ngoài</Button>
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

  it('opens again on a click into the box after a choice or Esc, the box having kept the focus', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[files(['A.xlsx'])]} />)
    await user.click(box())
    await user.click(screen.getByRole('option', { name: /A\.xlsx/ }))
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(box()).toHaveFocus()
    await user.click(box())
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await user.click(box())
    expect(screen.getByRole('listbox')).toBeInTheDocument()
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
    expect(screen.getByRole('option', { name: 'Tìm lại hợp đồng' })).toBeInTheDocument()
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

  it('shows at once on a screen opened to search, before the box has the focus', () => {
    render(<Harness variant="screen" initiallyOpen groups={[recent(['hợp đồng'])]} />)
    expect(phoneBox()).not.toHaveFocus()
    expect(screen.getByRole('button', { name: 'Tìm lại hợp đồng' })).toBeInTheDocument()
  })

  it('leaves Esc to the screen, which closes as a whole', async () => {
    const user = userEvent.setup()
    const outer = vi.fn()
    render(<div onKeyDown={(e) => e.key === 'Escape' && outer()}><Harness variant="screen" groups={[recent(['hợp đồng'])]} /></div>)
    await user.click(phoneBox())
    await user.keyboard('{Escape}')
    expect(outer).toHaveBeenCalledTimes(1)
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
  it('lets the surface reach no further than the window and keeps the rows inside the room left under the box', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(1024)
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(500)
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(
      { left: 900, top: 100, right: 1000, bottom: 136, width: 100, height: 36, x: 900, y: 100, toJSON: () => ({}) },
    )
    render(<Harness groups={[recent(['hợp đồng'])]} />)
    await user.click(box())
    const surface = screen.getByRole('listbox').closest('div[style*="--sx-open-w"]') as HTMLElement
    expect(surface.style.getPropertyValue('--sx-open-w')).toBe('min(max(100cqw, 440px), calc(100vw - 900px - 16px))')
    // 500 high, the box ends at 144 with its row, then the foot (36) and 16 to the edge.
    const list = screen.getByRole('listbox').parentElement as HTMLElement
    expect(list.style.maxHeight).toBe(`${500 - 100 - 44 - 36 - 16}px`)
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

describe('forgetting recent searches', () => {
  beforeEach(() => {
    localStorage.clear()
    useToastStore.getState().clear()
    useAuthStore.setState({ user: ME, accessToken: 't', bootstrapping: false })
  })
  afterEach(() => {
    switchPreferencesTo(undefined)
    localStorage.clear()
  })

  const rows = () => screen.queryAllByRole('option').map((o) => o.getAttribute('aria-label'))

  it('folds a row away with its ✕ while the others stay, and tells a screen reader', async () => {
    const user = userEvent.setup()
    render(<KeptHarness words={['một', 'hai', 'ba']} />)
    await user.click(box())
    await user.click(within(screen.getByRole('option', { name: 'Tìm lại hai' })).getByTitle('Xoá khỏi Tìm gần đây'))
    await waitFor(() => expect(rows()).toEqual(['Tìm lại ba', 'Tìm lại một']))
    expect(screen.getByText('Đã xoá “hai” khỏi Tìm gần đây')).toBeInTheDocument()
    expect(box()).toHaveFocus()
    expect(useToastStore.getState().toasts).toHaveLength(0)
  })

  it('moves the mark to the next row on Delete, and to the one before when the last goes', async () => {
    const user = userEvent.setup()
    render(<KeptHarness words={['một', 'hai', 'ba']} />)
    await user.click(box())
    // Newest first: ba, hai, một.
    await user.keyboard('{ArrowDown}{Delete}')
    await waitFor(() => expect(rows()).toEqual(['Tìm lại hai', 'Tìm lại một']))
    expect(screen.getByRole('option', { name: 'Tìm lại hai' })).toHaveAttribute('aria-selected', 'true')
    await user.keyboard('{ArrowDown}{Delete}')
    await waitFor(() => expect(rows()).toEqual(['Tìm lại hai']))
    expect(screen.getByRole('option', { name: 'Tìm lại hai' })).toHaveAttribute('aria-selected', 'true')
  })

  it('takes the group label with the last row', async () => {
    const user = userEvent.setup()
    render(<KeptHarness words={['một']} />)
    await user.click(box())
    expect(screen.getByRole('group', { name: 'Tìm gần đây' })).toBeInTheDocument()
    await user.keyboard('{ArrowDown}{Delete}')
    await waitFor(() => expect(screen.queryByText('Tìm gần đây')).toBeNull())
  })

  it('leaves Delete to the text while the box holds words', async () => {
    const user = userEvent.setup()
    render(<KeptHarness words={['một', 'hai']} />)
    await user.click(box())
    await user.keyboard('{ArrowDown}')
    await user.keyboard('x{Home}{Delete}')
    expect(box()).toHaveValue('')
    expect(JSON.parse(localStorage.getItem(`ngac-recent-searches:drive:${ME.id}`)!)).toEqual(['hai', 'một'])
  })

  it('offers Xoá tất cả from two searches on, with no confirmation, and a Hoàn tác toast that brings them back', async () => {
    const user = userEvent.setup()
    render(<KeptHarness words={['một', 'hai']} />)
    await user.click(box())
    await user.click(screen.getByRole('button', { name: 'Xoá tất cả' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    await waitFor(() => expect(rows()).toEqual([]))
    expect(box()).toHaveFocus()
    const [toast] = useToastStore.getState().toasts
    expect(toast).toMatchObject({ message: 'Đã xoá 2 lần tìm gần đây', action: { label: 'Hoàn tác' }, duration: 5000 })

    await user.click(box())
    act(() => toast!.action!.onClick())
    await user.click(box())
    await waitFor(() => expect(rows()).toEqual(['Tìm lại hai', 'Tìm lại một']))
  })

  it('has no Xoá tất cả for a single search', async () => {
    const user = userEvent.setup()
    render(<KeptHarness words={['một']} />)
    await user.click(box())
    expect(screen.queryByRole('button', { name: 'Xoá tất cả' })).toBeNull()
  })

  it('reaches Xoá tất cả with Tab and keeps the surface open; Esc there goes back to the box, a second Tab leaves', async () => {
    const user = userEvent.setup()
    render(<KeptHarness words={['một', 'hai']} />)
    await user.click(box())
    await user.tab()
    expect(screen.getByRole('button', { name: 'Xoá tất cả' })).toHaveFocus()
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(box()).toHaveFocus()
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    await user.tab()
    await user.tab()
    expect(screen.getByRole('button', { name: 'Ở ngoài' })).toHaveFocus()
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('shows ✕ on a phone row with a 44px target, and Xoá tất cả above the rows', async () => {
    const user = userEvent.setup()
    render(<Harness variant="screen" groups={[{ ...recent(['một', 'hai']), onClearAll: vi.fn() }]} />)
    await user.click(phoneBox())
    expect(screen.getByRole('button', { name: 'Xoá “một” khỏi Tìm gần đây' })).toHaveClass('w-11', 'h-11')
    expect(screen.getByRole('button', { name: 'Xoá tất cả' })).toHaveClass('h-11')
  })
})

describe('motion', () => {
  afterEach(() => switchPreferencesTo(undefined))

  const surface = () => screen.getByRole('combobox').closest('div[style*="--sx-open-w"]') as HTMLElement

  it('grows the surface out of the box with a clip, and slides the rows in', async () => {
    const user = userEvent.setup()
    render(<Harness groups={[recent(['hợp đồng'])]} />)
    expect(surface().style.transition).toContain('clip-path')
    expect(surface().style.getPropertyValue('--sx-rest-clip')).toContain('inset(4px')
    await user.click(box())
    expect(surface().style.clipPath).toContain('-24px')
    expect(screen.getByRole('option').parentElement!.style.transform).toContain('translateY(-4px)')
  })

  it('under reduced motion changes the surface at once and the rows only fade', async () => {
    switchPreferencesTo('user-a')
    updatePreferences({ motion: 'reduce' })
    const user = userEvent.setup()
    render(<Harness groups={[recent(['hợp đồng'])]} />)
    await user.click(box())
    expect(surface().style.transition).toBe('none')
    const row = screen.getByRole('option').parentElement!
    expect(row.style.transform).toBe('')
    expect(row.style.opacity).not.toBe('')
  })
})

describe('with nothing to suggest on a desktop', () => {
  it('opens the surface with how to search when it was asked to', async () => {
    const user = userEvent.setup()
    render(<Harness showWhenEmpty emptyText="Tìm tin nhắn, người và nhóm." groups={[recent([])]} />)
    await user.click(box())
    expect(screen.getByText('Tìm tin nhắn, người và nhóm.')).toBeInTheDocument()
    expect(box()).toHaveAttribute('aria-expanded', 'true')
    expect(box()).not.toHaveAttribute('aria-controls')
  })
})

describe('a row that leaves and comes back', () => {
  it('is not left folded: typing past a suggestion and back to it must not shrink it under the row below', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<Harness groups={[files(['A.xlsx', 'B.pdf'])]} />)
    await user.click(box())
    rerender(<Harness groups={[files(['A.xlsx'])]} />)
    rerender(<Harness groups={[files(['A.xlsx', 'B.pdf'])]} />)
    await new Promise((r) => setTimeout(r, 400))
    for (const row of screen.getAllByRole('option')) {
      const shell = row.parentElement!
      expect(shell.style.height === '' || shell.style.height === 'auto', `height ${shell.style.height}`).toBe(true)
      expect(shell.style.overflow).not.toBe('hidden')
    }
  })
})

describe('groups that come and go around one', () => {
  it('leave the others in place: a group appearing above does not rebuild the one below', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<Harness groups={[files(['A.xlsx'], vi.fn(), 'Tin nhắn')]} />)
    await user.click(box())
    const before = screen.getByRole('group', { name: 'Tin nhắn' })
    rerender(<Harness groups={[files(['P.pdf'], vi.fn(), 'Người'), files(['A.xlsx'], vi.fn(), 'Tin nhắn')]} />)
    expect(screen.getByRole('group', { name: 'Tin nhắn' })).toBe(before)
    rerender(<Harness groups={[files(['A.xlsx'], vi.fn(), 'Tin nhắn')]} />)
    expect(screen.getByRole('group', { name: 'Tin nhắn' })).toBe(before)
  })
})
