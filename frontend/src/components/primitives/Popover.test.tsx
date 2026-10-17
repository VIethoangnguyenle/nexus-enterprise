import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useRef } from 'react'
import { MenuItem, Popover } from '.'

function Harness() {
  const ref = useRef<HTMLButtonElement>(null)
  return (
    <>
      <button ref={ref} type="button">Mở</button>
      <Popover open onClose={() => {}} anchorRef={ref} role="menu" label="Menu">
        <MenuItem>Tải ảnh lên</MenuItem>
      </Popover>
    </>
  )
}

afterEach(() => vi.restoreAllMocks())

describe('Popover', () => {
  it('stays inside the viewport with a gutter when its anchor sits at the right edge', () => {
    vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(1440)
    vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(224)
    vi.spyOn(HTMLButtonElement.prototype, 'getBoundingClientRect').mockReturnValue(
      { left: 1400, right: 1440, top: 100, bottom: 140, width: 40, height: 40, x: 1400, y: 100, toJSON: () => ({}) },
    )
    render(<Harness />)
    const left = parseFloat(screen.getByRole('menu').style.left)
    expect(left + 224).toBeLessThanOrEqual(1440 - 16)
  })

  it('keeps the anchor-aligned position when there is room', () => {
    vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(1440)
    vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(224)
    vi.spyOn(HTMLButtonElement.prototype, 'getBoundingClientRect').mockReturnValue(
      { left: 200, right: 240, top: 100, bottom: 140, width: 40, height: 40, x: 200, y: 100, toJSON: () => ({}) },
    )
    render(<Harness />)
    expect(parseFloat(screen.getByRole('menu').style.left)).toBe(200)
  })
})
