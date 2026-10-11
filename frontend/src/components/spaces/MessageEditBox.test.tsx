import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { renderWithClient } from '../../test/render'
import { MessageEditBox } from './MessageEditBox'

function open(html = '<p>Chị đã thêm 3 giao dịch</p>') {
  const onSave = vi.fn()
  const onCancel = vi.fn()
  renderWithClient(<MessageEditBox initialHtml={html} onSave={onSave} onCancel={onCancel} />)
  return { onSave, onCancel, box: screen.getByRole('textbox', { name: 'Sửa tin' }) }
}

describe('MessageEditBox', () => {
  it('opens with the text, focused at its end, and says how to leave', async () => {
    const { box } = open()
    expect(box).toHaveTextContent('Chị đã thêm 3 giao dịch')
    await waitFor(() => expect(box).toHaveFocus())
    expect(screen.getByText('Esc huỷ · Enter lưu')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Huỷ' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Lưu' })).toBeInTheDocument()
  })

  it('saves what was typed with Enter', async () => {
    const user = userEvent.setup()
    const { onSave, box } = open('<p>cũ</p>')
    await waitFor(() => expect(box).toHaveFocus())
    await user.keyboard(' thêm{Enter}')
    expect(onSave).toHaveBeenCalledTimes(1)
    expect(onSave).toHaveBeenCalledWith('<p>cũ thêm</p>', false)
  })

  it('breaks the line with Shift+Enter instead of saving', async () => {
    const user = userEvent.setup()
    const { onSave, box } = open('<p>cũ</p>')
    await waitFor(() => expect(box).toHaveFocus())
    await user.keyboard('{Shift>}{Enter}{/Shift}hai')
    expect(onSave).not.toHaveBeenCalled()
    expect(box.querySelector('br:not(.ProseMirror-trailingBreak)')).not.toBeNull()
    expect(box).toHaveTextContent('cũhai')
  })

  it('gives the text back untouched on Esc', async () => {
    const user = userEvent.setup()
    const { onSave, onCancel, box } = open()
    await waitFor(() => expect(box).toHaveFocus())
    await user.keyboard('xyz{Escape}')
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onSave).not.toHaveBeenCalled()
  })

  it('does not let the Esc key reach a panel behind it', async () => {
    const user = userEvent.setup()
    // What a panel does: close on Esc unless the key was taken.
    const closePanel = vi.fn()
    const behind = (e: KeyboardEvent) => { if (e.key === 'Escape' && !e.defaultPrevented) closePanel() }
    window.addEventListener('keydown', behind)
    const { box } = open()
    await waitFor(() => expect(box).toHaveFocus())
    await user.keyboard('{Escape}')
    window.removeEventListener('keydown', behind)
    expect(closePanel).not.toHaveBeenCalled()
  })

  it('saves and cancels with its buttons', async () => {
    const user = userEvent.setup()
    const { onSave, onCancel } = open('<p>cũ</p>')
    await user.click(screen.getByRole('button', { name: 'Lưu' }))
    expect(onSave).toHaveBeenCalledWith('<p>cũ</p>', false)
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('reports an emptied editor as empty, so the owner can ask to delete', async () => {
    const user = userEvent.setup()
    const { onSave, box } = open('<p>cũ</p>')
    await waitFor(() => expect(box).toHaveFocus())
    await user.keyboard('{Backspace}{Backspace}{Backspace}{Enter}')
    expect(onSave).toHaveBeenCalledWith('', true)
  })
})
