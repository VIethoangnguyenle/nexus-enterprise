import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FileGrid } from './FileGrid'
import { Pressable } from '../primitives/Pressable'

/** The browser resolves the grid's tracks to one length per column; jsdom has no layout, so say what it would. */
function withColumns(cols: number) {
  const real = window.getComputedStyle.bind(window)
  vi.spyOn(window, 'getComputedStyle').mockImplementation((el: Element, pseudo?: string | null) => {
    const style = real(el, pseudo)
    if ((el as HTMLElement).dataset?.testid !== 'grid') return style
    return new Proxy(style, {
      get: (t, p) => (p === 'gridTemplateColumns' ? Array(cols).fill('200px').join(' ') : Reflect.get(t, p)),
    })
  })
}

function renderGrid(count: number, props: Partial<React.ComponentProps<typeof FileGrid>> = {}) {
  return render(
    <FileGrid label="Tệp" {...props}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} data-grid-item role="button" aria-label={`item ${i}`}>
          <Pressable>⋯</Pressable>
        </div>
      ))}
    </FileGrid>,
  )
}
const item = (i: number) => screen.getByRole('button', { name: `item ${i}` })

describe('FileGrid', () => {
  afterEach(() => vi.restoreAllMocks())

  it.each([2, 3, 4])('moves by the real column count (%i columns)', (cols) => {
    withColumns(cols)
    renderGrid(13)
    item(0).focus()
    fireEvent.keyDown(item(0), { key: 'ArrowDown' })
    expect(item(cols)).toHaveFocus()
    fireEvent.keyDown(item(cols), { key: 'ArrowRight' })
    expect(item(cols + 1)).toHaveFocus()
    fireEvent.keyDown(item(cols + 1), { key: 'ArrowUp' })
    expect(item(1)).toHaveFocus()
    fireEvent.keyDown(item(1), { key: 'ArrowLeft' })
    expect(item(0)).toHaveFocus()
  })

  it('stays put at the edges', () => {
    withColumns(3)
    renderGrid(7)
    item(0).focus()
    fireEvent.keyDown(item(0), { key: 'ArrowLeft' })
    fireEvent.keyDown(item(0), { key: 'ArrowUp' })
    expect(item(0)).toHaveFocus()
    item(6).focus()
    fireEvent.keyDown(item(6), { key: 'ArrowRight' })
    fireEvent.keyDown(item(6), { key: 'ArrowDown' })
    expect(item(6)).toHaveFocus()
  })

  it('goes down into a shorter last row, landing on its last item', () => {
    withColumns(3)
    renderGrid(7) // rows: 0 1 2 / 3 4 5 / 6
    item(5).focus()
    fireEvent.keyDown(item(5), { key: 'ArrowDown' })
    expect(item(6)).toHaveFocus()
  })

  it('jumps to the first and last item with Home and End', () => {
    withColumns(3)
    renderGrid(7)
    item(3).focus()
    fireEvent.keyDown(item(3), { key: 'End' })
    expect(item(6)).toHaveFocus()
    fireEvent.keyDown(item(6), { key: 'Home' })
    expect(item(0)).toHaveFocus()
  })

  it('opens the focused item with Enter or Space, by index', () => {
    withColumns(3)
    const onOpen = vi.fn()
    renderGrid(7, { onOpen })
    fireEvent.keyDown(item(4), { key: 'Enter' })
    fireEvent.keyDown(item(5), { key: ' ' })
    expect(onOpen.mock.calls).toEqual([[4], [5]])
  })

  it('leaves keys alone when they come from a control inside an item', () => {
    withColumns(3)
    const onOpen = vi.fn()
    renderGrid(4, { onOpen })
    const inner = item(1).querySelector('button')!
    inner.focus()
    fireEvent.keyDown(inner, { key: 'Enter' })
    fireEvent.keyDown(inner, { key: 'ArrowRight' })
    expect(onOpen).not.toHaveBeenCalled()
    expect(inner).toHaveFocus()
  })

  it('opens the menu with Shift+F10, the menu key and the right button', () => {
    withColumns(3)
    const onMenu = vi.fn()
    renderGrid(4, { onMenu })
    fireEvent.keyDown(item(2), { key: 'F10', shiftKey: true })
    fireEvent.keyDown(item(3), { key: 'ContextMenu' })
    expect(onMenu.mock.calls.map((c) => c[0])).toEqual([2, 3])
    expect(onMenu.mock.calls[0]![1]).toEqual({ x: expect.any(Number), y: expect.any(Number) })

    fireEvent.contextMenu(item(1), { clientX: 40, clientY: 50 })
    expect(onMenu).toHaveBeenLastCalledWith(1, { x: 40, y: 50 })
  })

  it('keeps the native menu where no item was hit', () => {
    const onMenu = vi.fn()
    renderGrid(2, { onMenu })
    fireEvent.contextMenu(screen.getByTestId('grid'))
    expect(onMenu).not.toHaveBeenCalled()
  })

  it('is one tab stop that follows focus', () => {
    withColumns(3)
    renderGrid(4)
    expect([0, 1, 2, 3].map((i) => item(i).tabIndex)).toEqual([0, -1, -1, -1])
    item(2).focus()
    expect([0, 1, 2, 3].map((i) => item(i).tabIndex)).toEqual([-1, -1, 0, -1])
  })

  it('lays out by variant: cards min 200 gap 12, folders gap 10, media min 112 gap 4', () => {
    const { rerender } = renderGrid(1)
    expect(screen.getByTestId('grid').className).toContain('minmax(200px,1fr)')
    expect(screen.getByTestId('grid')).toHaveClass('md:gap-3')
    rerender(<FileGrid label="Thư mục" variant="folders"><div data-grid-item /></FileGrid>)
    expect(screen.getByTestId('grid')).toHaveClass('md:gap-2.5')
    rerender(<FileGrid label="Ảnh" variant="media"><div data-grid-item /></FileGrid>)
    expect(screen.getByTestId('grid').className).toContain('minmax(112px,1fr)')
    expect(screen.getByTestId('grid')).toHaveClass('gap-1')
  })
})
