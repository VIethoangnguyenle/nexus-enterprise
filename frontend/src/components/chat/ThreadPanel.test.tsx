import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { buildDirectory } from '../../lib/people'
import { expectNoIds } from '../../test/no-ids'
import { CONTACTS, MESSAGES, MSG, THREAD, CH, ME, MSG_IMG1, fixtureApi } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import type { Message } from '../../api/messaging'
import { useDraftsStore } from '../../stores/drafts.store'
import { ThreadPanel } from './ThreadPanel'

vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})

vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'ws-test', workspaces: [], isLoading: false }) }))

vi.mock('../../api/drive', () => ({
  driveApi: {
    channelDrive: vi.fn().mockResolvedValue({
      items: [{ id: 'folder-root', item_type: 'folder', name: 'Channel Files' }],
    }),
    createFile: vi.fn().mockImplementation(async (_ws, name) => ({
      file_id: `file-id-${name}`,
      upload_url: `https://storage.local/upload/${name}`,
      object_key: `key-${name}`,
    })),
    uploadToStorage: vi.fn().mockResolvedValue(undefined),
    confirmFile: vi.fn().mockResolvedValue({}),
    getDownloadUrl: vi.fn().mockResolvedValue({ download_url: 'https://example.com/dl' }),
  },
}))

const people = buildDirectory(CONTACTS)
const calls = () => vi.mocked(apiFetch).mock.calls.map(([path, init]) => `${(init?.method ?? 'GET')} ${path}`)

// jsdom has no layout boxes; the editor asks for them when the caret moves.
beforeAll(() => {
  const none = () => ({ length: 0, item: () => null, [Symbol.iterator]: function* () {} }) as unknown as DOMRectList
  Range.prototype.getClientRects = none
  Range.prototype.getBoundingClientRect = () => new DOMRect()
  document.elementFromPoint = () => null
  window.scrollTo = () => {}
  globalThis.createImageBitmap = vi.fn().mockImplementation(async () => ({
    width: 800,
    height: 600,
    close: vi.fn(),
  }))
})

beforeEach(() => {
  useDraftsStore.getState().reset()
  vi.mocked(apiFetch).mockImplementation(fixtureApi as never)
  vi.mocked(apiFetch).mockClear()
  // The channel stream the thread was opened from.
  queryClient.setQueryData(keys.messaging.messages(CH.doisoat), { messages: MESSAGES, has_more: false })
})
afterEach(resetClient)

function panel(over: Partial<Parameters<typeof ThreadPanel>[0]> = {}) {
  return (
    <ThreadPanel
      channelId={CH.doisoat}
      workspaceId="ws-test"
      messageId={MSG.t1}
      spaceName="Đối soát giao dịch"
      people={people}
      me={ME.id}
      onClose={vi.fn()}
      {...over}
    />
  )
}

describe('ThreadPanel', () => {
  it('closes itself for a thread that belongs to another conversation than the one on screen', async () => {
    const onClose = vi.fn()
    // An old or crafted link: the thread is real, but of a different channel.
    renderWithClient(panel({ channelId: CH.vanhanh, onClose }))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(calls().filter((c) => c.startsWith('POST'))).not.toContain(`POST /channels/${CH.vanhanh}/messages`)
  })

  it('shows the root, the reply count and the replies by name', async () => {
    renderWithClient(panel())
    const side = await screen.findByRole('complementary', { name: 'Thread' })
    await within(side).findByText(/1\.284\.500/)
    expect(within(side).getByText(/lệch 3 giao dịch/)).toBeInTheDocument()
    expect(within(side).getByRole('separator', { name: '2 trả lời' })).toBeInTheDocument()
    expect(within(side).getByText('Nguyễn Thu Lan')).toBeInTheDocument()
    expect(within(side).getByText('Lê Quang Vinh')).toBeInTheDocument()
    expect(within(side).getByText('Đối soát giao dịch')).toBeInTheDocument()
    expectNoIds()
  })

  it('marks the thread read when it opens, after the unread replies are known', async () => {
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    await waitFor(() => expect(calls()).toContain(`POST /messages/${MSG.t1}/thread/read`))
  })

  it('puts "Tin mới" before the first unread reply and keeps it after the thread is read', async () => {
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    // The fixture says one reply is unread: the newest, by Vinh.
    const divider = await screen.findByRole('separator', { name: 'Tin mới' })
    const rows = [...divider.closest('aside')!.querySelectorAll('article, [role="separator"], [id^="reply-"]')]
    const vinh = screen.getByText(/Đã đối chiếu xong/)
    const lan = screen.getByText(/Tổng chênh lệch/)
    expect(divider.compareDocumentPosition(vinh) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(divider.compareDocumentPosition(lan) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy()
    expect(rows.length).toBeGreaterThan(0)
    await waitFor(() => expect(calls()).toContain(`POST /messages/${MSG.t1}/thread/read`))
    expect(screen.getByRole('separator', { name: 'Tin mới' })).toBeInTheDocument()
  })

  it('has no "Tin mới" when the reader does not follow the thread', async () => {
    queryClient.setQueryData(keys.messaging.thread(MSG.t1), {
      messages: THREAD.map((m) => (m.id === MSG.t1 ? { ...m, following: false, thread_unread_count: 3 } : m)),
    })
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    expect(screen.queryByRole('separator', { name: 'Tin mới' })).toBeNull()
    expect(calls()).not.toContain(`POST /messages/${MSG.t1}/thread/read`)
  })

  it('toggles follow with the server, announcing the change once', async () => {
    const user = userEvent.setup()
    renderWithClient(panel())
    const follow = await screen.findByRole('button', { name: 'Đang theo dõi' })
    expect(follow).toHaveAttribute('aria-pressed', 'true')
    await user.click(follow)
    await waitFor(() => expect(calls()).toContain(`DELETE /messages/${MSG.t1}/follow`))
    expect(await screen.findByRole('button', { name: 'Theo dõi' })).toHaveAttribute('aria-pressed', 'false')
    expect(await screen.findByText('Đã bỏ theo dõi thread')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Theo dõi' }))
    await waitFor(() => expect(calls()).toContain(`POST /messages/${MSG.t1}/follow`))
    expect(await screen.findByText('Đã theo dõi thread')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Đang theo dõi' })).toBeInTheDocument()
  })

  it('says nothing about following until the reader changes it', async () => {
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    expect(screen.queryByText(/Đã (bỏ )?theo dõi thread/)).toBeNull()
  })

  it('replies to the root message', async () => {
    const user = userEvent.setup()
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    const box = await screen.findByRole('textbox', { name: 'Trả lời trong thread' })
    await user.click(box)
    await user.keyboard('Đã nhận{Enter}')
    await waitFor(() => {
      const post = vi.mocked(apiFetch).mock.calls.find(([p, i]) => p === `/channels/${CH.doisoat}/messages` && i?.method === 'POST')
      expect(post).toBeTruthy()
      expect(JSON.parse(String(post![1]!.body))).toMatchObject({ parent_message_id: MSG.t1 })
    })
  })

  it('offers a way back when it was opened from the thread list', async () => {
    const user = userEvent.setup()
    const onBack = vi.fn()
    renderWithClient(panel({ onBack }))
    await user.click(await screen.findByRole('button', { name: 'Quay lại danh sách Thread' }))
    expect(onBack).toHaveBeenCalled()
  })

  it('says so when the thread cannot be loaded', async () => {
    vi.mocked(apiFetch).mockImplementation((p: string, i?: RequestInit) =>
      p === `/messages/${MSG.t1}/thread` ? Promise.reject(new Error('boom')) : fixtureApi(p, i)) as never
    queryClient.setQueryData(keys.messaging.messages(CH.doisoat), { messages: [] as Message[], has_more: false })
    renderWithClient(panel())
    expect(await screen.findByText('Chưa tải được thread này.')).toBeInTheDocument()
  })

  it('shows drop zone on dragover and sends attachments with reply', async () => {
    const user = userEvent.setup()
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)

    const side = screen.getByRole('complementary', { name: 'Thread' })
    fireEvent.dragOver(side, {
      dataTransfer: { types: ['Files'] },
    })

    // Should show drop zone prompt
    expect(await screen.findByText('Thả tệp để đính kèm')).toBeInTheDocument()
    expect(screen.getByText('Tối đa 20 tệp mỗi tin')).toBeInTheDocument()

    const file = new File(['content'], 'test.pdf', { type: 'application/pdf' })
    fireEvent.drop(side, {
      dataTransfer: { files: [file] },
    })
    expect(screen.queryByText('Thả tệp để đính kèm')).toBeNull()

    // Chip appears
    await screen.findByText('test.pdf')

    // Wait until upload completes
    await waitFor(() => {
      expect(screen.queryByText(/Đang tải/)).toBeNull()
    })

    // Click Gửi button to send attachment reply
    const sendBtn = screen.getByRole('button', { name: /Gửi/ })
    expect(sendBtn).not.toBeDisabled()
    await user.click(sendBtn)

    // Check that apiFetch was called to post the message with attachments
    await waitFor(() => {
      const post = vi.mocked(apiFetch).mock.calls.find(
        ([p, i]) => p === `/channels/${CH.doisoat}/messages` && i?.method === 'POST',
      )
      expect(post).toBeTruthy()
      const body = JSON.parse(String(post![1]!.body))
      expect(body).toMatchObject({
        parent_message_id: MSG.t1,
        attachments: [
          expect.objectContaining({
            drive_file_id: 'file-id-test.pdf',
          }),
        ],
      })
    })
  })

  it('Esc with viewer open inside open thread panel closes ONLY the viewer', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    vi.mocked(apiFetch).mockImplementation((p: string, i?: RequestInit) => {
      if (p === `/messages/${MSG.t1}/thread`) {
        return Promise.resolve({
          messages: [...THREAD, { ...MSG_IMG1, id: 'reply-with-img', parent_message_id: MSG.t1 }],
        })
      }
      return fixtureApi(p, i)
    })
    renderWithClient(panel({ onClose }))
    await screen.findByText(/1\.284\.500/)

    const imgBtn = await screen.findByRole('button', { name: 'Mở ảnh bien-lai-phi-08-10.jpg' })
    await user.click(imgBtn)

    const dialog = await screen.findByRole('dialog', { name: /bien-lai-phi-08-10\.jpg/ })
    expect(dialog).toBeInTheDocument()

    // Press Escape
    fireEvent.keyDown(document, { key: 'Escape' })

    // Viewer dialog is closed
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: /bien-lai-phi-08-10\.jpg/ })).toBeNull()
    })

    // Thread panel onClose was NOT called
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('complementary', { name: 'Thread' })).toBeInTheDocument()
  })
})

/** The block of the message whose text matches (its hover bar is inside). */
const blockOf = (text: RegExp) => screen.getByText(text).closest<HTMLElement>('.group')!

describe('ThreadPanel: cards and quotes in the thread composer', () => {
  const bodyOf = (path: string) => {
    const call = vi.mocked(apiFetch).mock.calls.find(([p, i]) => p === path && i?.method === 'POST')
    return call ? JSON.parse(String(call[1]!.body)) : undefined
  }

  it('creates a poll as a reply to the root of the thread', async () => {
    const user = userEvent.setup()
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    await user.click(screen.getByRole('button', { name: 'Tạo trong chat' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Bình chọn' }))
    await user.type(await screen.findByLabelText('Câu hỏi'), 'Họp lúc nào?')
    await user.type(screen.getByLabelText('Lựa chọn 1'), 'Sáng')
    await user.type(screen.getByLabelText('Lựa chọn 2'), 'Chiều')
    vi.mocked(apiFetch).mockImplementation((p: string, i?: RequestInit) =>
      p === `/channels/${CH.doisoat}/polls` ? Promise.resolve({ id: 'poll-1' }) : fixtureApi(p, i))
    await user.click(screen.getByRole('button', { name: 'Gửi bình chọn' }))
    await waitFor(() => expect(bodyOf(`/channels/${CH.doisoat}/polls`)).toMatchObject({ parent_message_id: MSG.t1, question: 'Họp lúc nào?' }))
  })

  it('quoting a reply puts the quote in the thread composer, and the reply goes out with it', async () => {
    const user = userEvent.setup()
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    await user.click(within(blockOf(/Tổng chênh lệch/)).getByRole('button', { name: 'Thêm' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Trích dẫn khi trả lời' }))

    const side = screen.getByRole('complementary', { name: 'Thread' })
    const strip = within(side).getByRole('region', { name: 'Đang trích dẫn tin của Nguyễn Thu Lan' })
    expect(strip).toHaveTextContent('Tổng chênh lệch')
    expect(useDraftsStore.getState().drafts[`thread:${MSG.t1}`]?.quote?.id).toBe(MSG.r1)
    expect(useDraftsStore.getState().drafts[CH.doisoat]).toBeUndefined()

    await user.click(within(side).getByRole('textbox', { name: 'Trả lời trong thread' }))
    await user.keyboard('Cảm ơn Lan{Enter}')
    await waitFor(() => expect(bodyOf(`/channels/${CH.doisoat}/messages`)).toMatchObject({
      parent_message_id: MSG.t1, quoted_message_id: MSG.r1,
    }))
    await waitFor(() => expect(within(side).queryByRole('region', { name: /Đang trích dẫn/ })).toBeNull())
  })

  it('stars a reply from its "Thêm" menu', async () => {
    const user = userEvent.setup()
    vi.mocked(apiFetch).mockImplementation((p: string, i?: RequestInit) =>
      p === `/messages/${MSG.r1}/star` ? Promise.resolve(undefined) : fixtureApi(p, i))
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    await user.click(within(blockOf(/Tổng chênh lệch/)).getByRole('button', { name: 'Thêm' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Gắn sao tin' }))
    await waitFor(() => expect(calls()).toContain(`PUT /messages/${MSG.r1}/star`))
    expect(await screen.findByRole('img', { name: 'Bạn đã gắn sao' })).toBeInTheDocument()
  })

  it('tags each reply so a quote can scroll to it', async () => {
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    expect(document.querySelector(`[data-message-id="${MSG.r1}"]`)).not.toBeNull()
  })
})

describe('ThreadPanel: marking a reply unread inside the thread', () => {
  const readCalls = () => calls().filter((c) => c === `POST /messages/${MSG.t1}/thread/read`).length
  const unreadAgain = () =>
    queryClient.setQueryData(keys.messaging.thread(MSG.t1), {
      messages: THREAD.map((m) => (m.id === MSG.t1 ? { ...m, thread_unread_count: 2 } : m)),
    })
  const markReplyUnread = async (user: ReturnType<typeof userEvent.setup>) => {
    vi.mocked(apiFetch).mockImplementation((p: string, i?: RequestInit) =>
      p === `/messages/${MSG.r2}/unread` ? Promise.resolve({ status: 'ok' }) : fixtureApi(p, i))
    await user.click(within(blockOf(/Đã đối chiếu xong/)).getByRole('button', { name: 'Thêm' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Đánh dấu chưa đọc' }))
    await waitFor(() => expect(calls()).toContain(`POST /messages/${MSG.r2}/unread`))
  }

  it('does not read the thread again while the panel stays open, and reads it the next time it opens', async () => {
    const user = userEvent.setup()
    const first = renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    await waitFor(() => expect(readCalls()).toBe(1))

    await markReplyUnread(user)
    // The thread is unread again (the server's frame, or the refetch).
    act(() => unreadAgain())
    await screen.findByText(/Đã đối chiếu xong/)
    await new Promise((r) => setTimeout(r, 50))
    expect(readCalls()).toBe(1)

    // Left and opened again: now it is read.
    first.unmount()
    queryClient.setQueryData(keys.messaging.thread(MSG.t1), {
      messages: THREAD.map((m) => (m.id === MSG.t1 ? { ...m, thread_unread_count: 2 } : m)),
    })
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    await waitFor(() => expect(readCalls()).toBe(2))
  })

  it('still reads a reply that arrives while open when the reader did not mark anything unread', async () => {
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    await waitFor(() => expect(readCalls()).toBe(1))
    act(() => unreadAgain())
    await waitFor(() => expect(readCalls()).toBe(2))
  })

  it('does not hold the read back when the mark failed', async () => {
    const user = userEvent.setup()
    renderWithClient(panel())
    await screen.findByText(/1\.284\.500/)
    await waitFor(() => expect(readCalls()).toBe(1))
    vi.mocked(apiFetch).mockImplementation((p: string, i?: RequestInit) =>
      p === `/messages/${MSG.r2}/unread` ? Promise.reject(new Error('403')) : fixtureApi(p, i))
    await user.click(within(blockOf(/Đã đối chiếu xong/)).getByRole('button', { name: 'Thêm' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Đánh dấu chưa đọc' }))
    await waitFor(() => expect(calls()).toContain(`POST /messages/${MSG.r2}/unread`))
    act(() => unreadAgain())
    await waitFor(() => expect(readCalls()).toBe(2))
  })
})
