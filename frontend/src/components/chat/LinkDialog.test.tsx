import { createRef } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Button } from '../primitives'
import { LinkDialog } from './LinkDialog'

function show(props: Partial<Parameters<typeof LinkDialog>[0]> = {}) {
  const onSubmit = vi.fn()
  const onRemove = vi.fn()
  const onClose = vi.fn()
  const anchorRef = createRef<HTMLElement>()
  render(
    <>
      <Button ref={anchorRef as React.RefObject<HTMLButtonElement>}>Link</Button>
      <LinkDialog open anchorRef={anchorRef} text="" href="" onSubmit={onSubmit} onRemove={onRemove} onClose={onClose} {...props} />
    </>,
  )
  return { onSubmit, onRemove, onClose }
}

describe('LinkDialog', () => {
  it('has a labelled text field and a url field, and starts in the text field when it is empty', async () => {
    show()
    expect(screen.getByRole('dialog', { name: 'Chèn liên kết' })).toBeInTheDocument()
    expect(screen.getByLabelText('Văn bản')).toBeInTheDocument()
    expect(screen.getByLabelText('Đường liên kết')).toHaveAttribute('type', 'url')
    await waitFor(() => expect(screen.getByLabelText('Văn bản')).toHaveFocus())
  })

  it('starts in the url field when the selection already gave the text', async () => {
    show({ text: 'báo cáo' })
    expect(screen.getByLabelText('Văn bản')).toHaveValue('báo cáo')
    await waitFor(() => expect(screen.getByLabelText('Đường liên kết')).toHaveFocus())
  })

  it('keeps Chèn off until the address is one the server keeps', async () => {
    const user = userEvent.setup()
    show({ text: 'x' })
    const insert = screen.getByRole('button', { name: 'Chèn' })
    expect(insert).toBeDisabled()
    await user.type(screen.getByLabelText('Đường liên kết'), 'javascript:alert(1)')
    expect(insert).toBeDisabled()
    await user.clear(screen.getByLabelText('Đường liên kết'))
    await user.type(screen.getByLabelText('Đường liên kết'), 'https://nexus.zaneng.xyz')
    expect(insert).toBeEnabled()
  })

  it('explains the rule under the field when it loses focus with a bad address', async () => {
    const user = userEvent.setup()
    show({ text: 'x' })
    await user.type(screen.getByLabelText('Đường liên kết'), 'nexus.vn')
    await user.tab()
    expect(screen.getByText('Chỉ dùng liên kết bắt đầu bằng https://, http:// hoặc mailto:')).toBeInTheDocument()
  })

  it('inserts with Enter and with the button, handing over the text and the address', async () => {
    const user = userEvent.setup()
    const { onSubmit } = show({ text: 'báo cáo' })
    await user.type(screen.getByLabelText('Đường liên kết'), 'mailto:an@nova.vn{Enter}')
    expect(onSubmit).toHaveBeenCalledWith({ text: 'báo cáo', href: 'mailto:an@nova.vn' })
  })

  it('uses the address as the text when none was given', async () => {
    const user = userEvent.setup()
    const { onSubmit } = show()
    await user.type(screen.getByLabelText('Đường liên kết'), 'https://a.vn')
    await user.click(screen.getByRole('button', { name: 'Chèn' }))
    expect(onSubmit).toHaveBeenCalledWith({ text: 'https://a.vn', href: 'https://a.vn' })
  })

  it('closes on Esc and on Huỷ', async () => {
    const user = userEvent.setup()
    const { onClose } = show()
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('edits an existing link with its values and can remove it', async () => {
    const user = userEvent.setup()
    const { onRemove } = show({ text: 'trang', href: 'https://a.vn', hasLink: true })
    expect(screen.getByRole('dialog', { name: 'Sửa liên kết' })).toBeInTheDocument()
    expect(screen.getByLabelText('Đường liên kết')).toHaveValue('https://a.vn')
    await user.click(screen.getByRole('button', { name: 'Bỏ liên kết' }))
    expect(onRemove).toHaveBeenCalledTimes(1)
  })
})
