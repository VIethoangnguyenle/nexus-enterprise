import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { EditorContent, useEditor } from '@tiptap/react'
import { describe, expect, it, vi } from 'vitest'
import { chatExtensions } from './chat-extensions'
import { FormattingToolbar } from './FormattingToolbar'

function Harness({ html = '<p>chữ</p>', onLink = vi.fn() }: { html?: string; onLink?: (a: HTMLElement) => void }) {
  const editor = useEditor({ extensions: chatExtensions(''), content: html })
  return (
    <>
      <FormattingToolbar editor={editor} id="fmt" onLink={onLink} />
      <EditorContent editor={editor} />
    </>
  )
}
const NAMES = ['Đậm', 'Nghiêng', 'Gạch chân', 'Gạch ngang', 'Danh sách chấm', 'Danh sách số', 'Link', 'Code', 'Khối code', 'Trích dẫn']

describe('FormattingToolbar', () => {
  it('is a toolbar named Định dạng with the ten buttons in the design order', async () => {
    render(<Harness />)
    const bar = await screen.findByRole('toolbar', { name: 'Định dạng' })
    const buttons = Array.from(bar.querySelectorAll('button')).map((b) => b.getAttribute('aria-label'))
    expect(buttons).toEqual(NAMES)
    expect(bar.querySelectorAll('[role="separator"]')).toHaveLength(3)
  })

  it('gives the shortcut in the tooltip', async () => {
    render(<Harness />)
    expect(await screen.findByRole('button', { name: 'Đậm' })).toHaveAttribute('title', 'Đậm (Ctrl+B)')
    expect(screen.getByRole('button', { name: 'Gạch ngang' })).toHaveAttribute('title', 'Gạch ngang (Ctrl+Shift+X)')
    expect(screen.getByRole('button', { name: 'Link' })).toHaveAttribute('title', 'Link (Ctrl+K)')
  })

  it('applies a format to the selection and shows it pressed', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const bold = await screen.findByRole('button', { name: 'Đậm' })
    expect(bold).toHaveAttribute('aria-pressed', 'false')
    await act(async () => {
      document.querySelector<HTMLElement>('.ProseMirror')!.focus()
    })
    await user.keyboard('{Control>}a{/Control}')
    await user.click(bold)
    expect(document.querySelector('.ProseMirror strong')).not.toBeNull()
    await waitFor(() => expect(bold).toHaveAttribute('aria-pressed', 'true'))
    expect(screen.getByRole('button', { name: 'Nghiêng' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('shows a format pressed where the cursor already is inside it', async () => {
    render(<Harness html="<ul><li><p>mục</p></li></ul>" />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Danh sách chấm' })).toHaveAttribute('aria-pressed', 'true'))
    expect(screen.getByRole('button', { name: 'Danh sách số' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('moves between buttons with the arrow keys, one tab stop for the whole bar', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const first = await screen.findByRole('button', { name: 'Đậm' })
    expect(first).toHaveAttribute('tabindex', '0')
    expect(screen.getByRole('button', { name: 'Nghiêng' })).toHaveAttribute('tabindex', '-1')
    first.focus()
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('button', { name: 'Nghiêng' })).toHaveFocus()
    await user.keyboard('{ArrowLeft}{ArrowLeft}')
    expect(screen.getByRole('button', { name: 'Trích dẫn' })).toHaveFocus()
    await user.keyboard('{Home}')
    expect(first).toHaveFocus()
  })

  it('asks for the link box from the Link button, handing over the button to hang it from', async () => {
    const user = userEvent.setup()
    const onLink = vi.fn()
    render(<Harness onLink={onLink} />)
    await user.click(await screen.findByRole('button', { name: 'Link' }))
    expect(onLink).toHaveBeenCalledWith(screen.getByRole('button', { name: 'Link' }))
  })

  it('has touch targets of 44px on a phone', async () => {
    render(<Harness />)
    expect(await screen.findByRole('button', { name: 'Đậm' })).toHaveClass('max-md:w-11', 'max-md:h-11')
  })
})
