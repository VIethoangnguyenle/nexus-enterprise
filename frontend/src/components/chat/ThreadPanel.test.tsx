import { fireEvent, screen, waitFor, within } from '@testing-library/react'
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
import { ThreadPanel } from './ThreadPanel'

vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})

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

    const dialog = await screen.findByRole('dialog', { name: 'Trình xem ảnh' })
    expect(dialog).toBeInTheDocument()
    expect(within(dialog).getByText('bien-lai-phi-08-10.jpg')).toBeInTheDocument()

    // Press Escape
    fireEvent.keyDown(document, { key: 'Escape' })

    // Viewer dialog is closed
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Trình xem ảnh' })).toBeNull()
    })

    // Thread panel onClose was NOT called
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('complementary', { name: 'Thread' })).toBeInTheDocument()
  })
})


