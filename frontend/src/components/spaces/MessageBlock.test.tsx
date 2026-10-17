import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient } from '../../test/render'
import { CONTACTS, ME, MESSAGES, MSG_LEGACY, MSG_NO_TEXT, U, UUID_RE } from '../../test/chat-fixtures'
import { buildDirectory } from '../../lib/people'
import { MessageBlock } from './MessageBlock'
import type { Message } from '../../api/messaging'
import { driveApi } from '../../api/drive'

const navigate = vi.fn()
vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))

vi.mock('../chat/PollCard', () => ({ PollCard: ({ pollId }: { pollId: string }) => <div data-testid="poll-card">poll:{pollId}</div> }))
vi.mock('../chat/TaskCard', () => ({ TaskCard: (p: { taskId: string; channelId: string; onOpenTasks?: unknown }) => <div data-testid="task-card">task:{p.taskId}:{p.channelId}:{p.onOpenTasks ? 'link' : 'nolink'}</div> }))
vi.mock('../chat/DocCard', () => ({ DocCard: ({ docId }: { docId: string }) => <div data-testid="doc-card">doc:{docId}</div> }))
vi.mock('../chat/ApprovalCard', () => ({ ApprovalCard: ({ requestId }: { requestId: string }) => <div data-testid="approval-card">approval:{requestId}</div> }))

beforeEach(() => {
  navigate.mockReset()
  vi.spyOn(driveApi, 'getDownloadUrl').mockResolvedValue({ download_url: 'https://s.test/dl' })
  vi.spyOn(driveApi, 'preview').mockResolvedValue({ kind: 'native', url: 'https://s.test/x', mime: 'image/jpeg' })
})

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

  it('opens the file viewer on a thumbnail, Esc closes it and focus returns to the thumbnail', async () => {
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

    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('hinh-anh.jpg')).toBeInTheDocument()
    expect(dialog.textContent).not.toMatch(UUID_RE)

    await user.keyboard('{Escape}')

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
      expect(document.activeElement).toBe(thumbnail)
    })
  })

  it('walks the pictures and the files of one message as a single list in the viewer', async () => {
    const user = userEvent.setup()
    const mixed: Message = {
      ...MSG_NO_TEXT,
      id: 'msg-mixed',
      attachments: [
        { drive_file_id: 'f-pdf', file_name: 'bien-ban.pdf', mime_type: 'application/pdf', size_bytes: 2048 },
        { drive_file_id: 'f-img', file_name: 'hinh-anh.jpg', mime_type: 'image/jpeg', size_bytes: 4096, width: 600, height: 400 },
      ],
    }
    renderWithClient(<MessageBlock message={mixed} people={people} idBase="msg-mixed" me={ME.id} />)

    await user.click(screen.getByRole('button', { name: 'Mở tệp bien-ban.pdf' }))
    const dialog = screen.getByRole('dialog')
    // Pictures first: the pdf is the second of two.
    expect(within(dialog).getByText('bien-ban.pdf')).toBeInTheDocument()
    expect(within(dialog).getByLabelText('Tệp 2 trên 2')).toBeInTheDocument()
    await user.click(within(dialog).getAllByRole('button', { name: 'Tệp trước' })[0]!)
    expect(within(dialog).getByLabelText('Tệp 1 trên 2')).toBeInTheDocument()
    expect(within(dialog).getByText('hinh-anh.jpg')).toBeInTheDocument()
  })

  it('names the sender in the viewer and never offers "Mở trong Tài liệu" (a message file has no folder to open)', async () => {
    const user = userEvent.setup()
    renderWithClient(<MessageBlock message={MSG_NO_TEXT} people={people} idBase="msg-viewer-sender" me={ME.id} />)
    await user.click(screen.getByRole('button', { name: /Mở ảnh hinh-anh\.jpg/ }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText(/Lê Quang Vinh|lan/i)).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: 'Mở trong Tài liệu' })).toBeNull()
  })
})

describe('MessageBlock: cards', () => {
  const card = (type: string, id: string, over: Partial<Message> = {}): Message => ({
    id: `m-${type}`, channel_id: 'ch-1', sender_id: U.lan.id, content: '', linked_entity_type: type, linked_entity_id: id, ...over,
  })

  it.each([
    ['poll', 'poll-card', 'poll:p-1'],
    ['document', 'doc-card', 'doc:p-1'],
    ['approval', 'approval-card', 'approval:p-1'],
    ['task', 'task-card', 'task:p-1:ch-1:nolink'],
  ])('renders the %s card for its linked object, without a link to Công việc', (type, testId, text) => {
    renderWithClient(<MessageBlock message={card(type, 'p-1')} people={people} idBase={`m-${type}`} me={ME.id} />)
    expect(screen.getByTestId(testId)).toHaveTextContent(text)
  })

  it('shows the sender above the card, not an empty bubble', () => {
    renderWithClient(<MessageBlock message={card('document', 'd-1')} people={people} idBase="m-doc" me={ME.id} />)
    expect(document.getElementById('m-doc-text')).toBeNull()
    expect(screen.getByTestId('doc-card')).toBeInTheDocument()
  })

  it('does not print the fallback line a poll or task message carries as its text', () => {
    renderWithClient(
      <MessageBlock
        message={card('poll', 'p-1', { content: '📊 Poll: Họp lúc nào?', message_type: 'system' })}
        people={people}
        idBase="m-poll"
        me={ME.id}
      />,
    )
    expect(screen.queryByText(/Poll: Họp lúc nào/)).toBeNull()
    expect(screen.getByTestId('poll-card')).toBeInTheDocument()
  })

  it('keeps the text of a document or approval message that has some', () => {
    renderWithClient(<MessageBlock message={card('document', 'd-1', { content: '<p>Tài liệu tháng này</p>' })} people={people} idBase="m-doc" me={ME.id} />)
    expect(screen.getByText('Tài liệu tháng này')).toBeInTheDocument()
    expect(screen.getByTestId('doc-card')).toBeInTheDocument()
  })

  it('shows no card for a deleted message, for an unknown kind, or without an object', () => {
    const { rerender } = renderWithClient(<MessageBlock message={card('poll', 'p-1', { deleted: true })} people={people} idBase="m-1" me={ME.id} />)
    expect(screen.queryByTestId('poll-card')).toBeNull()
    rerender(<MessageBlock message={card('weird', 'x')} people={people} idBase="m-2" me={ME.id} />)
    expect(screen.queryByTestId('poll-card')).toBeNull()
    rerender(<MessageBlock message={card('poll', '')} people={people} idBase="m-3" me={ME.id} />)
    expect(screen.queryByTestId('poll-card')).toBeNull()
  })
})

describe('MessageBlock: starred', () => {
  const text: Message = { id: 'm-star', channel_id: 'ch-1', sender_id: U.lan.id, content: '<p>xin chào</p>', content_format: 'html', created_at: '2026-10-10T09:31:00Z' }

  it('puts a small star after the time of a starred message, and none otherwise', () => {
    const { rerender } = renderWithClient(<MessageBlock message={text} people={people} idBase="m-star" me={ME.id} />)
    expect(screen.queryByRole('img', { name: 'Bạn đã gắn sao' })).toBeNull()
    rerender(<MessageBlock message={{ ...text, starred: true }} people={people} idBase="m-star" me={ME.id} />)
    const star = screen.getByRole('img', { name: 'Bạn đã gắn sao' })
    const time = document.querySelector('time')!
    expect(time.compareDocumentPosition(star) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('shows the star on a grouped line too', () => {
    renderWithClient(<MessageBlock message={{ ...text, starred: true }} people={people} idBase="m-star" me={ME.id} grouped />)
    expect(screen.getByRole('img', { name: 'Bạn đã gắn sao' })).toBeInTheDocument()
  })

  it('does not show a star on a deleted message', () => {
    renderWithClient(<MessageBlock message={{ ...text, starred: true, deleted: true }} people={people} idBase="m-star" me={ME.id} />)
    expect(screen.queryByRole('img', { name: 'Bạn đã gắn sao' })).toBeNull()
  })
})

describe('MessageBlock: your own lines on a phone', () => {
  const mine: Message = { id: 'm-mine', channel_id: 'ch-1', sender_id: ME.id, content: '<p>xin chào</p>', content_format: 'html', created_at: '2026-10-10T09:31:00Z' }
  const rowOf = (idBase: string) => document.getElementById(`${idBase}-text`)!.closest('.group')!
  const columnOf = (idBase: string) => document.getElementById(`${idBase}-text`)!.parentElement!

  it('keeps no empty time column at the right edge below md, only from md up', () => {
    renderWithClient(<MessageBlock message={mine} people={people} idBase="m-mine" me={ME.id} mine />)
    const cls = rowOf('m-mine').className.split(/\s+/)
    expect(cls).toContain('grid-cols-[minmax(0,1fr)]')
    expect(cls).toContain('md:grid-cols-[minmax(0,1fr)_2rem]')
    expect(cls).not.toContain('grid-cols-[minmax(0,1fr)_2rem]')
  })

  it('leaves the other side as it was: avatar column on the left', () => {
    renderWithClient(<MessageBlock message={{ ...mine, sender_id: U.lan.id }} people={people} idBase="m-other" me={ME.id} />)
    expect(rowOf('m-other').className.split(/\s+/)).toContain('grid-cols-[2rem_minmax(0,1fr)]')
  })

  it('shows a grouped line\'s time above the bubble when tapped, and the edge column only from md up', async () => {
    const user = userEvent.setup()
    renderWithClient(<MessageBlock message={mine} people={people} idBase="m-grp" me={ME.id} mine grouped />)
    const gutter = rowOf('m-grp').querySelector(':scope > span time')!.parentElement!
    expect(gutter.className.split(/\s+/)).toEqual(expect.arrayContaining(['max-md:hidden', 'md:col-start-2']))
    expect(columnOf('m-grp').querySelector('time')).toBeNull()

    await user.click(document.getElementById('m-grp-text')!)
    const inline = columnOf('m-grp').querySelector('time')!
    expect(inline).toHaveTextContent(/\d{2}:\d{2}/)
    expect(inline.parentElement!.className).toContain('md:hidden')

    await user.click(document.getElementById('m-grp-text')!)
    expect(columnOf('m-grp').querySelector('time')).toBeNull()
  })
})

describe('MessageBlock: an author the directory does not list', () => {
  const HIDDEN = '11111111-aaaa-4bbb-8ccc-0000000000ff'
  const hiddenAuthor: Message = { ...MESSAGES[0]!, sender_id: HIDDEN, sender_name: '' }

  it('shows the name the resolver found, never "Thành viên" or the id', () => {
    const dir = buildDirectory(CONTACTS, {
      resolved: {
        byUserId: new Map([[HIDDEN, { userId: HIDDEN, nodeId: 'n-hidden', name: 'Phạm Ẩn Danh', avatarUrl: '' }]]),
        byNodeId: new Map(),
      },
    })
    const { container } = renderWithClient(<MessageBlock message={hiddenAuthor} people={dir} idBase="hidden" me={ME.id} />)
    expect(screen.getByText('Phạm Ẩn Danh')).toBeInTheDocument()
    expect(container.textContent).not.toMatch(UUID_RE)
  })

  it('asks the resolver for an author nobody has named yet', () => {
    const request = vi.fn()
    renderWithClient(<MessageBlock message={hiddenAuthor} people={buildDirectory(CONTACTS, { request })} idBase="hidden2" me={ME.id} />)
    expect(request).toHaveBeenCalledWith(HIDDEN)
  })
})

describe('MessageBlock: pressing the author', () => {
  const HIDDEN = '11111111-aaaa-4bbb-8ccc-0000000000ff'
  const hiddenDir = buildDirectory(CONTACTS, {
    resolved: {
      byUserId: new Map([[HIDDEN, { userId: HIDDEN, nodeId: 'n-hidden', name: 'Nguyễn Hữu Thành', avatarUrl: '' }]]),
      byNodeId: new Map(),
    },
  })
  const fromHidden: Message = { ...MESSAGES[0]!, sender_id: HIDDEN, sender_name: '' }

  it('a hidden author opens the small card from the name, with the sentence and no actions', async () => {
    const { container } = renderWithClient(<MessageBlock message={fromHidden} people={hiddenDir} idBase="h1" me={ME.id} />)
    await userEvent.click(screen.getByRole('button', { name: 'Nguyễn Hữu Thành' }))
    const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    expect(within(card).getByText('Hồ sơ này không còn hiển thị với bạn.')).toBeInTheDocument()
    expect(within(card).queryAllByRole('button')).toHaveLength(0)
    expect(navigate).not.toHaveBeenCalled()
    expect(container.textContent).not.toMatch(UUID_RE)
  })

  it('the avatar is a second target for the same card, but not a second control to tab to', async () => {
    const { container } = renderWithClient(<MessageBlock message={fromHidden} people={hiddenDir} idBase="h2" me={ME.id} />)
    expect(screen.getAllByRole('button', { name: 'Nguyễn Hữu Thành' })).toHaveLength(1)
    const avatarLink = container.querySelector<HTMLElement>('button[aria-hidden="true"]')!
    expect(avatarLink).toHaveAttribute('tabindex', '-1')
    await userEvent.click(avatarLink)
    expect(await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })).toBeInTheDocument()
  })

  it('a visible author opens their profile in Danh bạ', async () => {
    const msg: Message = { ...MESSAGES[0]!, sender_id: U.duc.id }
    renderWithClient(<MessageBlock message={msg} people={people} idBase="v1" me={ME.id} />)
    await userEvent.click(screen.getByRole('button', { name: /Trần Minh Đức/ }))
    expect(navigate).toHaveBeenCalledTimes(1)
    expect(navigate.mock.calls[0]![0]).toMatchObject({ to: '/contacts' })
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('while the directory is not ready, pressing the author does nothing', async () => {
    const notReady = { ...buildDirectory([]), ready: false }
    renderWithClient(<MessageBlock message={fromHidden} people={notReady} idBase="u1" me={ME.id} />)
    const btn = screen.getByRole('button')
    expect(btn).toHaveAttribute('aria-busy', 'true')
    await userEvent.click(btn)
    expect(navigate).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('your own message has nothing to press for the author', () => {
    renderWithClient(<MessageBlock message={{ ...MESSAGES[0]!, sender_id: ME.id }} people={people} idBase="me1" me={ME.id} mine />)
    expect(screen.queryByRole('button', { name: new RegExp(ME.username, 'i') })).toBeNull()
  })

  it('a grouped line has no author to press', () => {
    renderWithClient(<MessageBlock message={fromHidden} people={hiddenDir} idBase="g1" me={ME.id} grouped />)
    expect(screen.queryByRole('button', { name: 'Nguyễn Hữu Thành' })).toBeNull()
  })

  it('an author nobody can name yet is the same word in the label, on screen and in the card', async () => {
    const stranger: Message = { ...MESSAGES[0]!, sender_id: '11111111-aaaa-4bbb-8ccc-000000000abc', sender_name: 'thanh.nh' }
    renderWithClient(<MessageBlock message={stranger} people={people} idBase="s1" me={ME.id} />)
    await userEvent.click(screen.getByRole('button', { name: 'thanh.nh' }))
    expect(await screen.findByRole('dialog', { name: 'thanh.nh' })).toBeInTheDocument()
  })

  it('pressing inside the card does not start the message long press or open its action sheet', async () => {
    const onOpenSheet = vi.fn()
    renderWithClient(<MessageBlock message={fromHidden} people={hiddenDir} idBase="ls" me={ME.id} onOpenSheet={onOpenSheet} />)
    await userEvent.click(screen.getByRole('button', { name: 'Nguyễn Hữu Thành' }))
    const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    vi.useFakeTimers()
    try {
      fireEvent.pointerDown(card, { pointerType: 'touch' })
      vi.advanceTimersByTime(800)
    } finally {
      vi.useRealTimers()
    }
    expect(onOpenSheet).not.toHaveBeenCalled()
  })
})
