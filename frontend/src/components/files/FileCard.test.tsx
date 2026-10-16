import { act, fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { FileCard } from './FileCard'
import { FolderTile } from './FolderTile'

function setCoarse(coarse: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: coarse && query.includes('coarse'),
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })) as unknown as typeof window.matchMedia
}

function renderCard(props: Partial<React.ComponentProps<typeof FileCard>> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  // The picture never arrives here: the card is judged by everything around it.
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  return render(
    <QueryClientProvider client={client}>
      <FileCard
        fileId="f1"
        name="Đối soát giao dịch.xlsx"
        owner={{ name: 'Hà Ngân', hueKey: 'u1' }}
        timeLabel="09:02"
        onMenu={vi.fn()}
        {...props}
      />
    </QueryClientProvider>,
  )
}

describe('FileCard', () => {
  beforeEach(() => setCoarse(false))
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('shows the file name, the kind label and who and when, never an id', () => {
    renderCard()
    expect(screen.getByText('Đối soát giao dịch.xlsx')).toBeInTheDocument()
    expect(screen.getByText('Hà Ngân · 09:02')).toBeInTheDocument()
    expect(screen.queryByText(/f1/)).not.toBeInTheDocument()
  })

  it('shortens a long name to the last two words', () => {
    renderCard({ owner: { name: 'Nguyễn Thị Thu Lan', hueKey: 'u2' } })
    expect(screen.getByText('Thu Lan · 09:02')).toBeInTheDocument()
  })

  it('is raised, lifts on hover, and keeps ⋯ hidden until hover or focus', () => {
    renderCard()
    const card = screen.getByRole('button', { name: /Đối soát giao dịch\.xlsx/, pressed: false })
    expect(card).toHaveClass('bg-raised', 'hover:bg-overlay', 'hover:shadow-overlay')
    const more = screen.getByRole('button', { name: 'Tùy chọn cho Đối soát giao dịch.xlsx' })
    expect(more).toHaveClass('opacity-0', 'group-hover:opacity-100', 'group-focus-within:opacity-100')
  })

  it('marks selection with the accent wash and ring, and shows ⋯', () => {
    renderCard({ selected: true })
    const card = screen.getByRole('button', { pressed: true })
    expect(card).toHaveClass('bg-accent-wash')
    expect(card).toHaveClass('ring-2', 'ring-inset', 'ring-accent')
    expect(screen.getByRole('button', { name: /Tùy chọn cho/ })).toHaveClass('opacity-100')
  })

  it('always shows ⋯ on a coarse pointer', () => {
    setCoarse(true)
    renderCard()
    expect(screen.getByRole('button', { name: /Tùy chọn cho/ })).toHaveClass('opacity-100')
  })

  it('opens the menu from ⋯ without selecting the card', () => {
    const onMenu = vi.fn()
    const onSelect = vi.fn()
    renderCard({ onMenu, onSelect })
    fireEvent.click(screen.getByRole('button', { name: /Tùy chọn cho/ }))
    expect(onMenu).toHaveBeenCalledTimes(1)
    expect(onMenu.mock.calls[0]![0]).toEqual({ x: expect.any(Number), y: expect.any(Number) })
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('selects on click and opens on double click', () => {
    const onSelect = vi.fn()
    const onOpen = vi.fn()
    renderCard({ onSelect, onOpen })
    const card = screen.getByRole('button', { pressed: false })
    fireEvent.click(card)
    expect(onSelect).toHaveBeenCalledTimes(1)
    fireEvent.doubleClick(card)
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  it('opens the menu on a long press and swallows the click that follows', () => {
    vi.useFakeTimers()
    const onMenu = vi.fn()
    const onSelect = vi.fn()
    renderCard({ onMenu, onSelect })
    const card = screen.getByRole('button', { pressed: false })
    fireEvent.pointerDown(card, { pointerType: 'touch', clientX: 20, clientY: 30 })
    act(() => { vi.advanceTimersByTime(500) })
    expect(onMenu).toHaveBeenCalledWith({ x: 20, y: 30 })
    fireEvent.pointerUp(card, { pointerType: 'touch' })
    fireEvent.click(card)
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('does not long-press when the finger moves away or lifts early', () => {
    vi.useFakeTimers()
    const onMenu = vi.fn()
    renderCard({ onMenu })
    const card = screen.getByRole('button', { pressed: false })
    fireEvent.pointerDown(card, { pointerType: 'touch', clientX: 20, clientY: 30 })
    fireEvent.pointerMove(card, { pointerType: 'touch', clientX: 80, clientY: 30 })
    act(() => { vi.advanceTimersByTime(600) })
    fireEvent.pointerDown(card, { pointerType: 'touch', clientX: 20, clientY: 30 })
    act(() => { vi.advanceTimersByTime(200) })
    fireEvent.pointerUp(card, { pointerType: 'touch' })
    act(() => { vi.advanceTimersByTime(600) })
    expect(onMenu).not.toHaveBeenCalled()
  })

  it('never long-presses with a mouse', () => {
    vi.useFakeTimers()
    const onMenu = vi.fn()
    renderCard({ onMenu })
    fireEvent.pointerDown(screen.getByRole('button', { pressed: false }), { pointerType: 'mouse' })
    act(() => { vi.advanceTimersByTime(800) })
    expect(onMenu).not.toHaveBeenCalled()
  })
})

describe('FolderTile', () => {
  it('shows the name and the count, and opens on a plain click', () => {
    const onOpen = vi.fn()
    render(<FolderTile name="Báo cáo quý" count={12} onOpen={onOpen} />)
    expect(screen.getByText('Báo cáo quý')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Báo cáo quý/ }))
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  it('is a real link when it has an address, and leaves modified clicks to the browser', () => {
    const onOpen = vi.fn()
    render(<FolderTile name="Hợp đồng" href="/drive/folders/x" onOpen={onOpen} />)
    const link = screen.getByRole('link', { name: /Hợp đồng/ })
    expect(link).toHaveAttribute('href', '/drive/folders/x')
    fireEvent.click(link, { ctrlKey: true })
    expect(onOpen).not.toHaveBeenCalled()
    fireEvent.click(link)
    expect(onOpen).toHaveBeenCalledTimes(1)
  })
})
