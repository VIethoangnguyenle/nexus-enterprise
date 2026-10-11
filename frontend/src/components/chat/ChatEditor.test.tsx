import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient } from '../../test/render'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { CH, MEMBERS } from '../../test/chat-fixtures'
import { useDraftsStore } from '../../stores/drafts.store'
import { ChatEditor } from './ChatEditor'

beforeEach(() => {
  localStorage.clear()
  useDraftsStore.getState().reset()
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
})

describe('ChatEditor: edit mode', () => {
  const edit = (over: Partial<Parameters<typeof ChatEditor>[0]['edit']> = {}) => ({
    initialHtml: '<p>Chị đã thêm 3 giao dịch</p>',
    onSave: vi.fn(),
    onCancel: vi.fn(),
    ...over,
  })

  it('shows the "Đang sửa tin" bar, populates the text, swaps Gửi for Lưu, hides attach', async () => {
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

  it('cancels with Esc and with Huỷ, without calling onSave', async () => {
    const user = userEvent.setup()
    const e = edit()
    const { box } = open({ edit: e })
    await waitFor(() => expect(box).toHaveFocus())
    await user.keyboard('{Escape}')
    expect(e.onCancel).toHaveBeenCalled()
    expect(e.onSave).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    expect(e.onCancel).toHaveBeenCalledTimes(2)
  })

  it('keeps the user draft when edit opens, and puts it back when cancelled', async () => {
    const user = userEvent.setup()
    const onSend = vi.fn()
    const { rerender } = renderWithClient(<ChatEditor channelId={CH.doisoat} onSend={onSend} />)
    const box = screen.getByRole('textbox')
    await user.click(box)
    await user.keyboard('đang viết dở')

    const onCancel = vi.fn()
    rerender(
      <ChatEditor
        channelId={CH.doisoat}
        onSend={onSend}
        edit={{ initialHtml: '<p>tin cũ</p>', onSave: vi.fn(), onCancel }}
      />,
    )
    expect(box).toHaveTextContent('tin cũ')

    rerender(<ChatEditor channelId={CH.doisoat} onSend={onSend} />)
    expect(box).toHaveTextContent('đang viết dở')
  })
})

describe('ChatEditor: edit last message', () => {
  it('opens the last message when ArrowUp is pressed in an empty box, not in a typed one', async () => {
    const user = userEvent.setup()
    const onEditLast = vi.fn()
    const { box } = open({ onEditLast })
    await user.click(box)
    await user.keyboard('{ArrowUp}')
    expect(onEditLast).toHaveBeenCalledTimes(1)

    await user.keyboard('chữ{ArrowUp}')
    expect(onEditLast).toHaveBeenCalledTimes(1)
  })
})

describe('ChatEditor: attachments and uploads', () => {
  it('disables Gửi while uploading, enables when ready or files only, shows why status next to locked Gửi', async () => {
    const user = userEvent.setup()
    const onSend = vi.fn()

    const mockUploads = {
      items: [
        {
          key: 'item-1',
          file: new File(['img'], 'photo.png', { type: 'image/png' }),
          status: 'uploading' as const,
        },
      ],
      add: vi.fn(),
      remove: vi.fn(),
      retry: vi.fn(),
      clear: vi.fn(),
      ready: false,
    }

    const { rerender } = renderWithClient(
      <ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={onSend} uploads={mockUploads} />,
    )

    // While uploading, send button is disabled and why status explains it
    const sendBtn = screen.getByRole('button', { name: /Gửi/ })
    expect(sendBtn).toBeDisabled()
    expect(screen.getByRole('status')).toHaveTextContent('Đang tải 1 tệp…')

    // When upload errored, why status prompts retry or remove
    const erroredUploads = {
      ...mockUploads,
      items: [
        {
          key: 'item-1',
          file: new File(['img'], 'photo.png', { type: 'image/png' }),
          status: 'error' as const,
          error: 'Mất kết nối',
        },
      ],
      ready: false,
    }
    rerender(
      <ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={onSend} uploads={erroredUploads} />,
    )
    expect(sendBtn).toBeDisabled()
    expect(screen.getByRole('status')).toHaveTextContent('Thử lại hoặc bỏ tệp lỗi để gửi')

    // When upload finishes, ready is true and files only enables Gửi even with empty text
    const doneUploads = {
      ...mockUploads,
      items: [
        {
          key: 'item-1',
          file: new File(['img'], 'photo.png', { type: 'image/png' }),
          status: 'done' as const,
          fileId: 'file-123',
          width: 1200,
          height: 800,
        },
      ],
      ready: true,
    }

    rerender(
      <ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={onSend} uploads={doneUploads} />,
    )

    expect(screen.getByRole('button', { name: /Gửi/ })).toBeEnabled()
    expect(screen.queryByRole('status')).toBeNull()

    // Click Gửi sends attachments with width/height and clears uploads
    await user.click(screen.getByRole('button', { name: /Gửi/ }))
    expect(onSend).toHaveBeenCalledWith('', expect.any(Array), [
      { drive_file_id: 'file-123', width: 1200, height: 800 },
    ])
    expect(doneUploads.clear).toHaveBeenCalled()
  })

  it('send failure retains attachments in tray and keeps editor text', async () => {
    const user = userEvent.setup()
    const onSend = vi.fn().mockRejectedValue(new Error('Send network failed'))

    const mockUploads = {
      items: [
        {
          key: 'item-1',
          file: new File(['img'], 'photo.png', { type: 'image/png' }),
          status: 'done' as const,
          fileId: 'file-123',
        },
      ],
      add: vi.fn(),
      remove: vi.fn(),
      retry: vi.fn(),
      clear: vi.fn(),
      ready: true,
    }

    const { box } = open({ uploads: mockUploads, onSend })
    await user.click(box)
    await user.keyboard('nội dung quan trọng')

    await user.click(screen.getByRole('button', { name: /Gửi/ }))
    expect(onSend).toHaveBeenCalled()

    // Clear was NOT called on failure
    expect(mockUploads.clear).not.toHaveBeenCalled()
    // Text was not emptied
    expect(box).toHaveTextContent('nội dung quan trọng')
  })

  it('paste of an image file calls add with the file', async () => {
    const add = vi.fn()
    const mockUploads = {
      items: [],
      add,
      remove: vi.fn(),
      retry: vi.fn(),
      clear: vi.fn(),
      ready: true,
    }

    const { box } = open({ uploads: mockUploads })
    const pasteFile = new File(['image'], 'pasted.png', { type: 'image/png' })

    const pasteEvent = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(pasteEvent, 'clipboardData', {
      value: {
        getData: vi.fn().mockReturnValue(''),
        items: [
          {
            kind: 'file',
            type: 'image/png',
            getAsFile: () => pasteFile,
          },
        ],
      },
    })

    box.dispatchEvent(pasteEvent)
    expect(add).toHaveBeenCalledWith([pasteFile])
  })

  it('paste containing text/plain does not intercept as attachment', () => {
    const add = vi.fn()
    const mockUploads = {
      items: [],
      add,
      remove: vi.fn(),
      retry: vi.fn(),
      clear: vi.fn(),
      ready: true,
    }

    const { box } = open({ uploads: mockUploads })
    const pasteFile = new File(['image'], 'pasted.png', { type: 'image/png' })

    const pasteEvent = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(pasteEvent, 'clipboardData', {
      value: {
        getData: (type: string) => (type === 'text/plain' ? 'Copied from Excel table' : ''),
        items: [
          {
            kind: 'file',
            type: 'image/png',
            getAsFile: () => pasteFile,
          },
        ],
      },
    })

    box.dispatchEvent(pasteEvent)
    expect(add).not.toHaveBeenCalled()
  })

  it('paste is disabled in edit mode', () => {
    const add = vi.fn()
    const mockUploads = {
      items: [],
      add,
      remove: vi.fn(),
      retry: vi.fn(),
      clear: vi.fn(),
      ready: true,
    }

    const { box } = open({
      uploads: mockUploads,
      edit: { initialHtml: '<p>edit</p>', onSave: vi.fn(), onCancel: vi.fn() },
    })
    const pasteFile = new File(['image'], 'pasted.png', { type: 'image/png' })

    const pasteEvent = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(pasteEvent, 'clipboardData', {
      value: {
        getData: vi.fn().mockReturnValue(''),
        items: [
          {
            kind: 'file',
            type: 'image/png',
            getAsFile: () => pasteFile,
          },
        ],
      },
    })

    box.dispatchEvent(pasteEvent)
    expect(add).not.toHaveBeenCalled()
  })

  it('opens attach menu on desktop with "Tải lên từ máy"', async () => {
    const user = userEvent.setup()
    const mockUploads = {
      items: [],
      add: vi.fn(),
      remove: vi.fn(),
      retry: vi.fn(),
      clear: vi.fn(),
      ready: true,
    }

    open({ uploads: mockUploads })

    const attachBtn = screen.getByRole('button', { name: 'Đính kèm' })
    await user.click(attachBtn)

    const menu = screen.getByRole('menu', { name: 'Đính kèm' })
    expect(menu).toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: /Tải lên từ máy/ })).toBeInTheDocument()
  })
})
