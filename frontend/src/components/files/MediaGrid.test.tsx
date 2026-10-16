import { fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MediaGrid, MediaTile } from './MediaGrid'

function renderTiles(onSelect = vi.fn(), onOpen = vi.fn()) {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  const client = new QueryClient()
  return render(
    <QueryClientProvider client={client}>
      <MediaGrid label="Ảnh và video tuần này">
        <MediaTile fileId="a" name="Ảnh 1.jpg" mimeType="image/jpeg" onSelect={onSelect} onOpen={onOpen} onMenu={vi.fn()} />
        <MediaTile fileId="b" name="Clip.mp4" mimeType="video/mp4" selected onMenu={vi.fn()} />
      </MediaGrid>
    </QueryClientProvider>,
  )
}

describe('MediaGrid', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('lays tiles out tight, 112 minimum and 4 apart', () => {
    renderTiles()
    const grid = screen.getByRole('group', { name: 'Ảnh và video tuần này' })
    expect(grid.className).toContain('minmax(112px,1fr)')
    expect(grid).toHaveClass('gap-1')
  })

  it('makes each tile a square frame named after its file', () => {
    renderTiles()
    const tile = screen.getByRole('button', { name: 'Ảnh 1.jpg' })
    const frame = tile.querySelector<HTMLElement>('[data-testid="file-thumb"]')!
    expect(frame.style.aspectRatio).toBe('1 / 1')
    expect(frame.style.borderRadius).toBe('6px')
  })

  it('selects on click, opens on double click, and shows selection as a ring', () => {
    const onSelect = vi.fn()
    const onOpen = vi.fn()
    renderTiles(onSelect, onOpen)
    const tile = screen.getByRole('button', { name: 'Ảnh 1.jpg' })
    fireEvent.click(tile)
    fireEvent.doubleClick(tile)
    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(onOpen).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: 'Clip.mp4', pressed: true })).toHaveClass('ring-2', 'ring-inset', 'ring-accent')
  })
})
