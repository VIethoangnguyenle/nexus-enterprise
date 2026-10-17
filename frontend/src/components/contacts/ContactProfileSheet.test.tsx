import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRef, useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Pressable, TextField } from '../primitives'
import { ContactProfileSheet } from './ContactProfileSheet'
import { PhoneLayer } from './ContactsPhone'

afterEach(cleanup)

function Harness({ onClose = () => {} }: { onClose?: () => void }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Pressable onClick={() => setOpen(true)}>Mở</Pressable>
      {open && (
        <ContactProfileSheet label="Hồ sơ Nguyễn Thu Lan" onClose={() => { setOpen(false); onClose() }}>
          <Pressable type="button">Nhắn tin</Pressable>
        </ContactProfileSheet>
      )}
    </>
  )
}

describe('ContactProfileSheet', () => {
  it('is a modal dialog beside a scrim, with its content, outside the page it covers', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    await user.click(screen.getByRole('button', { name: 'Mở' }))
    const dialog = await screen.findByRole('dialog', { name: 'Hồ sơ Nguyễn Thu Lan' })
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(document.body.contains(dialog)).toBe(true)
    expect(dialog.closest('#root')).toBeNull()
    expect(screen.getByRole('button', { name: 'Nhắn tin' })).toBeInTheDocument()
    expect(document.querySelector('.bg-scrim')).not.toBeNull()
  })

  it('moves focus in, closes on Esc, and gives focus back to what opened it', async () => {
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<Harness onClose={onClose} />)
    const opener = screen.getByRole('button', { name: 'Mở' })
    await user.click(opener)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Nhắn tin' })).toHaveFocus())
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(opener).toHaveFocus()
  })

  it('closes when the scrim is pressed', async () => {
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<Harness onClose={onClose} />)
    await user.click(screen.getByRole('button', { name: 'Mở' }))
    await screen.findByRole('dialog')
    await user.click(document.querySelector('.bg-scrim') as HTMLElement)
    expect(onClose).toHaveBeenCalled()
  })
})

describe('PhoneLayer', () => {
  it('is a full-screen layer in the body that takes Esc as one step back', async () => {
    const onBack = vi.fn()
    const user = userEvent.setup()
    render(<PhoneLayer label="Phòng ban" onBack={onBack}><Pressable type="button">Khối Vận hành</Pressable></PhoneLayer>)
    const layer = screen.getByRole('dialog', { name: 'Phòng ban' })
    expect(layer).toHaveAttribute('aria-modal', 'true')
    expect(layer.className).toMatch(/fixed/)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Khối Vận hành' })).toHaveFocus())
    await user.keyboard('{Escape}')
    expect(onBack).toHaveBeenCalled()
  })

  it('puts focus where it is told to (the search box of the search screen) instead of on the first control', async () => {
    const field = createRef<HTMLInputElement>()
    render(
      <PhoneLayer label="Tìm trong danh bạ" onBack={() => {}} initialFocusRef={field}>
        <Pressable type="button">Quay lại</Pressable>
        <TextField ref={field} label="Tìm người" />
      </PhoneLayer>,
    )
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Tìm người' })).toHaveFocus())
  })
})
