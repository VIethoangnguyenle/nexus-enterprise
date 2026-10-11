import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient } from '../../test/render'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { CH, MEMBERS } from '../../test/chat-fixtures'
import { ChatEditor } from './ChatEditor'

beforeEach(() => {
  localStorage.clear()
  queryClient.setQueryData(keys.messaging.members(CH.doisoat), { members: MEMBERS })
})

function open(props: Partial<Parameters<typeof ChatEditor>[0]> = {}) {
  const onSend = vi.fn()
  renderWithClient(<ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={onSend} {...props} />)
  return { onSend, box: screen.getByRole('textbox', { name: 'Tin nhắn cho Đối soát' }) }
}

describe('ChatEditor: sending', () => {
  it('sends the typed text as html with Enter and empties the box', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open()
    await user.click(box)
    await user.keyboard('xin chào{Enter}')
    expect(onSend).toHaveBeenCalledWith('<p>xin chào</p>', expect.any(Array))
    expect(box).toHaveTextContent('')
  })

  it('breaks the line with Shift+Enter', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open()
    await user.click(box)
    await user.keyboard('một{Shift>}{Enter}{/Shift}hai')
    expect(onSend).not.toHaveBeenCalled()
    expect(box.querySelector('br')).not.toBeNull()
  })

  it('makes a new list item with Enter inside a list, and sends with Ctrl+Enter', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open()
    await user.click(box)
    await user.keyboard('- một{Enter}hai')
    expect(onSend).not.toHaveBeenCalled()
    expect(box.querySelectorAll('li')).toHaveLength(2)
    await user.keyboard('{Control>}{Enter}{/Control}')
    expect(onSend).toHaveBeenCalledWith('<ul><li><p>một</p></li><li><p>hai</p></li></ul>', expect.any(Array))
  })

  it('does not send nothing', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open()
    await user.click(box)
    await user.keyboard('{Enter}')
    expect(onSend).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: /Gửi/ })).toBeDisabled()
  })
})

describe('ChatEditor: the formatting bar', () => {
  it('is hidden until the "A" button opens it, which says what it controls', async () => {
    const user = userEvent.setup()
    open()
    expect(screen.queryByRole('toolbar', { name: 'Định dạng' })).toBeNull()
    const a = screen.getByRole('button', { name: 'Định dạng' })
    expect(a).toHaveAttribute('aria-pressed', 'false')
    await user.click(a)
    const bar = screen.getByRole('toolbar', { name: 'Định dạng' })
    expect(a).toHaveAttribute('aria-pressed', 'true')
    expect(a).toHaveAttribute('aria-controls', bar.id)
    await user.click(a)
    expect(screen.queryByRole('toolbar', { name: 'Định dạng' })).toBeNull()
  })

  it('remembers whether it was open for the next composer', async () => {
    const user = userEvent.setup()
    const first = renderWithClient(<ChatEditor channelId={CH.doisoat} onSend={vi.fn()} />)
    await user.click(screen.getByRole('button', { name: 'Định dạng' }))
    first.unmount()
    renderWithClient(<ChatEditor channelId={CH.doisoat} onSend={vi.fn()} />)
    expect(screen.getByRole('toolbar', { name: 'Định dạng' })).toBeInTheDocument()
  })

  it('formats what is typed after pressing a button and sends the markup', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open()
    await user.click(screen.getByRole('button', { name: 'Định dạng' }))
    await user.click(box)
    await user.click(screen.getByRole('button', { name: 'Đậm' }))
    await user.keyboard('quan trọng{Enter}')
    expect(onSend).toHaveBeenCalledWith('<p><strong>quan trọng</strong></p>', expect.any(Array))
  })

  it('formats with the keyboard: Ctrl+B and Ctrl+U', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open()
    await user.click(box)
    await user.keyboard('{Control>}b{/Control}đậm{Control>}b{/Control} {Control>}u{/Control}gạch{Enter}')
    expect(onSend.mock.calls[0]![0]).toBe('<p><strong>đậm</strong> <u>gạch</u></p>')
  })

  it('inserts a link from Ctrl+K and sends it opening in a new tab', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open()
    await user.click(screen.getByRole('button', { name: 'Định dạng' }))
    await user.click(box)
    await user.keyboard('{Control>}k{/Control}')
    const dialog = await screen.findByRole('dialog', { name: 'Chèn liên kết' })
    await user.type(within(dialog).getByLabelText('Văn bản'), 'Nexus')
    await user.type(within(dialog).getByLabelText('Đường liên kết'), 'https://nexus.zaneng.xyz{Enter}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Chèn liên kết' })).toBeNull())
    await waitFor(() => expect(box).toHaveFocus())
    await user.keyboard('{Enter}')
    const html = onSend.mock.calls[0]![0] as string
    expect(html).toContain('<a ')
    expect(html).toContain('href="https://nexus.zaneng.xyz"')
    expect(html).toContain('rel="noopener noreferrer"')
    expect(html).toContain('>Nexus</a>')
  })
})

describe('ChatEditor: editing a message in the composer (phones)', () => {
  const edit = (over: Partial<NonNullable<Parameters<typeof ChatEditor>[0]['edit']>> = {}) => ({
    initialHtml: '<p>Chị đã thêm 3 giao dịch</p>', onSave: vi.fn(), onCancel: vi.fn(), ...over,
  })

  it('shows the "Đang sửa tin" bar with Huỷ, the text, and Lưu in place of Gửi', async () => {
    const e = edit()
    const { box } = open({ edit: e, onFileUpload: vi.fn() })
    expect(screen.getByText('Đang sửa tin')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Huỷ' })).toBeInTheDocument()
    expect(box).toHaveTextContent('Chị đã thêm 3 giao dịch')
    expect(screen.getByRole('button', { name: 'Lưu' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: /Gửi/ })).toBeNull()
    // Files stay as they were.
    expect(screen.queryByRole('button', { name: 'Đính kèm' })).toBeNull()
    await waitFor(() => expect(box).toHaveFocus())
  })

  it('saves with Enter and with Lưu, handing over the html and whether it is empty', async () => {
    const user = userEvent.setup()
    const e = edit()
    const { box } = open({ edit: e })
    await waitFor(() => expect(box).toHaveFocus())
    await user.keyboard(' nữa{Enter}')
    expect(e.onSave).toHaveBeenCalledWith('<p>Chị đã thêm 3 giao dịch nữa</p>', false)
    await user.click(screen.getByRole('button', { name: 'Lưu' }))
    expect(e.onSave).toHaveBeenCalledTimes(2)
  })

  it('reports an emptied box as empty, so the owner asks to delete', async () => {
    const user = userEvent.setup()
    const e = edit({ initialHtml: '<p>a</p>' })
    const { box } = open({ edit: e })
    await waitFor(() => expect(box).toHaveFocus())
    await user.keyboard('{Backspace}')
    await user.click(screen.getByRole('button', { name: 'Lưu' }))
    expect(e.onSave).toHaveBeenCalledWith('', true)
  })

  it('cancels with Huỷ and with Esc, and does not send anything', async () => {
    const user = userEvent.setup()
    const e = edit()
    const { onSend, box } = open({ edit: e })
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    await user.click(box)
    await user.keyboard('{Escape}')
    expect(e.onCancel).toHaveBeenCalledTimes(2)
    expect(onSend).not.toHaveBeenCalled()
  })

  it('gives the composer back empty when editing ends', async () => {
    const { unmount } = renderWithClient(
      <ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={vi.fn()} edit={edit()} />,
    )
    unmount()
    renderWithClient(<ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={vi.fn()} />)
    expect(screen.getByRole('textbox', { name: 'Tin nhắn cho Đối soát' })).toHaveTextContent('')
  })
})

describe('ChatEditor: ↑ edits your last message', () => {
  it('calls for it when the box is empty', async () => {
    const user = userEvent.setup()
    const onEditLast = vi.fn()
    const { box } = open({ onEditLast })
    await user.click(box)
    await user.keyboard('{ArrowUp}')
    expect(onEditLast).toHaveBeenCalledTimes(1)
  })

  it('leaves the arrow to the text once there is some', async () => {
    const user = userEvent.setup()
    const onEditLast = vi.fn()
    const { box } = open({ onEditLast })
    await user.click(box)
    await user.keyboard('a{ArrowUp}')
    expect(onEditLast).not.toHaveBeenCalled()
  })
})
