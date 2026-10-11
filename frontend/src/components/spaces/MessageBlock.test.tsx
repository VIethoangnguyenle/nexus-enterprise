import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { renderWithClient } from '../../test/render'
import { CONTACTS, ME, MSG_LEGACY, MSG_NO_TEXT, U } from '../../test/chat-fixtures'
import { buildDirectory } from '../../lib/people'
import { MessageBlock } from './MessageBlock'
import type { Message } from '../../api/messaging'

const people = buildDirectory(CONTACTS)

describe('MessageBlock: attachments', () => {
  it('renders only the grid when message has attachments and no text', () => {
    renderWithClient(
      <MessageBlock
        message={MSG_NO_TEXT}
        people={people}
        idBase="msg-no-text"
        me={ME.id}
      />,
    )

    // Should render the image attachment
    expect(screen.getByRole('button', { name: /Mở ảnh hinh-anh\.jpg/ })).toBeInTheDocument()

    // No text bubble
    expect(screen.queryByText('Tin nhắn đã bị xoá')).toBeNull()
    const textContainer = document.getElementById('msg-no-text-text')
    // Text container should not have a speech bubble div or empty text block
    expect(textContainer).toBeNull()
  })

  it('renders legacy drive_file message as one card', () => {
    renderWithClient(
      <MessageBlock
        message={MSG_LEGACY}
        people={people}
        idBase="msg-legacy"
        me={ME.id}
      />,
    )

    // Renders as a file card with filename and download button
    expect(screen.getByText('bien-ban.pdf')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Tải xuống bien-ban\.pdf/ })).toBeInTheDocument()

    // Should NOT render the raw "📎 bien-ban.pdf" text bubble
    expect(screen.queryByText('📎 bien-ban.pdf')).toBeNull()
  })

  it('preserves text when linked_entity_type is drive_file but content does not match legacy attachment pattern', () => {
    const customMsg: Message = {
      id: 'custom-drive-msg',
      channel_id: 'c1',
      sender_id: U.lan.id,
      content: 'Tài liệu hướng dẫn sử dụng hệ thống',
      linked_entity_type: 'drive_file',
      linked_entity_id: 'drive-file-123',
    }

    renderWithClient(
      <MessageBlock
        message={customMsg}
        people={people}
        idBase="custom-drive-msg"
        me={ME.id}
      />,
    )

    // Text must NOT be hidden
    expect(screen.getByText('Tài liệu hướng dẫn sử dụng hệ thống')).toBeInTheDocument()
    // Does not fall back to a bogus "file" card
    expect(screen.queryByText('file')).toBeNull()
  })

  it('renders QuoteBlock even on files-only message when quoting another message', () => {
    const quoteMsg: Message = {
      ...MSG_NO_TEXT,
      id: 'msg-quote-only',
      quoted: {
        id: 'quoted-parent',
        sender_id: U.hoa.id,
        sender_name: 'Lê Thị Hoa',
        text: 'Nội dung trích dẫn quan trọng',
      },
    }

    renderWithClient(
      <MessageBlock
        message={quoteMsg}
        people={people}
        idBase="msg-quote-only"
        me={ME.id}
      />,
    )

    // Quote block must be rendered inside the bubble
    expect(screen.getByText('Nội dung trích dẫn quan trọng')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Mở ảnh hinh-anh\.jpg/ })).toBeInTheDocument()
  })

  it('opens ImageViewer on thumbnail click, Esc closes and returns focus to thumbnail', async () => {
    const user = userEvent.setup()

    renderWithClient(
      <MessageBlock
        message={MSG_NO_TEXT}
        people={people}
        idBase="msg-focus-test"
        me={ME.id}
      />,
    )

    const thumbnail = screen.getByRole('button', { name: /Mở ảnh hinh-anh\.jpg/ })
    await user.click(thumbnail)

    // Viewer modal opens
    expect(screen.getByRole('dialog', { name: 'Trình xem ảnh' })).toBeInTheDocument()

    // Esc closes viewer
    await user.keyboard('{Escape}')

    // Viewer is unmounted and focus returns to thumbnail
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Trình xem ảnh' })).toBeNull()
      expect(document.activeElement).toBe(thumbnail)
    })
  })
})
