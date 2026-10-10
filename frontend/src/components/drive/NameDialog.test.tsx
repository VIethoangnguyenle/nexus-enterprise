import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { NameDialog } from './NameDialog'

const base = {
  onClose: () => {}, title: 'Đổi tên', submitLabel: 'Đổi tên', emptyMessage: 'Đặt tên', pending: false, onSubmit: () => {},
}

describe('NameDialog', () => {
  it('seeds the name when it opens, and keeps what was typed if the initial name changes meanwhile', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<NameDialog {...base} open initialName="a.pdf" />)
    const field = screen.getByRole('textbox', { name: 'Tên' })
    expect(field).toHaveValue('a.pdf')
    await user.clear(field)
    await user.type(field, 'moi.pdf')

    // A realtime update renames the item underneath the open dialog.
    rerender(<NameDialog {...base} open initialName="b.pdf" />)
    expect(screen.getByRole('textbox', { name: 'Tên' })).toHaveValue('moi.pdf')
  })

  it('starts from the current name on the next open', () => {
    const { rerender } = render(<NameDialog {...base} open initialName="a.pdf" />)
    rerender(<NameDialog {...base} open={false} initialName="a.pdf" />)
    rerender(<NameDialog {...base} open initialName="b.pdf" />)
    expect(screen.getByRole('textbox', { name: 'Tên' })).toHaveValue('b.pdf')
  })
})
