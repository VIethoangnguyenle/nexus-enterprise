import { useState } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Button } from '../primitives'
import { SidePanel } from './SidePanel'

function mockViewport(phone: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}
afterEach(() => vi.unstubAllGlobals())

function Host() {
  const [open, setOpen] = useState(false)
  return (
    <div data-testid="content">
      <Button onClick={() => setOpen(true)}>Mở</Button>
      {open && (
        <SidePanel label="Chi tiết" title="Tiêu đề" closeLabel="chi tiết" onClose={() => setOpen(false)}>
          <Button>Hành động</Button>
        </SidePanel>
      )}
    </div>
  )
}

describe('SidePanel as a phone sheet', () => {
  it('is a modal dialog in document.body: focus goes in, stays in, Esc closes, focus returns', async () => {
    mockViewport(true)
    const user = userEvent.setup()
    render(<Host />)
    const opener = screen.getByRole('button', { name: 'Mở' })
    await user.click(opener)

    const sheet = await screen.findByRole('dialog', { name: 'Chi tiết' })
    expect(sheet).toHaveAttribute('aria-modal', 'true')
    // Not inside the content area: that has its own stacking context under the tab bar.
    expect(screen.getByTestId('content').contains(sheet)).toBe(false)
    expect(sheet.parentElement).toBe(document.body)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Đóng chi tiết' })).toHaveFocus())
    for (let i = 0; i < 4; i++) {
      await user.tab()
      expect(sheet.contains(document.activeElement)).toBe(true)
    }

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Chi tiết' })).toBeNull())
    await waitFor(() => expect(opener).toHaveFocus())
  })
})

describe('SidePanel beside the content', () => {
  it('stays a plain complementary region: no modal, no trap, in place', async () => {
    mockViewport(false)
    const user = userEvent.setup()
    render(<Host />)
    await user.click(screen.getByRole('button', { name: 'Mở' }))
    const panel = await screen.findByRole('complementary', { name: 'Chi tiết' })
    expect(panel).not.toHaveAttribute('aria-modal')
    expect(screen.getByTestId('content').contains(panel)).toBe(true)
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
