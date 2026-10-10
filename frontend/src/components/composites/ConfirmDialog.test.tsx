import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ConfirmDialog } from './ConfirmDialog'

const base = {
  title: 'Xoá tệp',
  description: <>Xoá <strong>doi-soat.xlsx</strong>?</>,
  confirmLabel: 'Xoá',
}

describe('ConfirmDialog', () => {
  it('names the action and what it touches', () => {
    render(<ConfirmDialog open onClose={() => {}} onConfirm={() => {}} warning="Không hoàn tác được." {...base} />)
    const dialog = screen.getByRole('dialog', { name: 'Xoá tệp' })
    expect(dialog).toHaveTextContent('doi-soat.xlsx')
    expect(dialog).toHaveTextContent('Không hoàn tác được.')
  })

  it('renders nothing while closed', () => {
    render(<ConfirmDialog open={false} onClose={() => {}} onConfirm={() => {}} {...base} />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('confirms and cancels with their own buttons, and closes on Esc', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    const onConfirm = vi.fn()
    render(<ConfirmDialog open onClose={onClose} onConfirm={onConfirm} {...base} />)

    await user.click(screen.getByRole('button', { name: 'Xoá' }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('puts focus on Cancel when the action is destructive', async () => {
    render(<ConfirmDialog open onClose={() => {}} onConfirm={() => {}} confirmVariant="danger" {...base} />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Huỷ' })).toHaveFocus())
  })

  it('keeps focus inside the dialog when tabbing past the last button', async () => {
    const user = userEvent.setup()
    render(<ConfirmDialog open onClose={() => {}} onConfirm={() => {}} confirmVariant="danger" {...base} />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Huỷ' })).toHaveFocus())
    await user.tab()
    expect(screen.getByRole('button', { name: 'Xoá' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: 'Huỷ' })).toHaveFocus()
  })

  it('cannot be cancelled while the action runs', () => {
    render(<ConfirmDialog open onClose={() => {}} onConfirm={() => {}} loading {...base} />)
    expect(screen.getByRole('button', { name: 'Huỷ' })).toBeDisabled()
  })
})
