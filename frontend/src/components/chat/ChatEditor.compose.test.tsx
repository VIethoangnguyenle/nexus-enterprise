import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { buildDirectory } from '../../lib/people'
import { CH, CONTACTS, MEMBERS, U, UUID_RE } from '../../test/chat-fixtures'
import { messagingApi, type Message } from '../../api/messaging'
import { quoteFromMessage, useDraftsStore } from '../../stores/drafts.store'
import { ChatEditor } from './ChatEditor'

let phone = false
vi.mock('../../hooks/usePhone', () => ({ usePhone: () => phone }))
let isPersonal = false
vi.mock('../../hooks/useWorkspaceType', () => ({ useWorkspaceType: () => ({ type: undefined, isPersonal, isLoading: false }) }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'ws-1', workspaces: [], isLoading: false }) }))
vi.mock('../../api/messaging', async (orig) => {
  const real = await orig<typeof import('../../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, sendMessage: vi.fn(), listMembers: vi.fn() } }
})
vi.mock('./PollDialog', () => ({ PollDialog: (p: { channelId: string; parentMessageId?: string }) => <div role="dialog" aria-label="Tạo bình chọn">poll:{p.channelId}:{p.parentMessageId ?? '-'}</div> }))
vi.mock('./TaskDialog', () => ({ TaskDialog: (p: { channelId: string; parentMessageId?: string }) => <div role="dialog" aria-label="Tạo công việc">task:{p.channelId}:{p.parentMessageId ?? '-'}</div> }))
vi.mock('./DocDialog', () => ({ DocDialog: (p: { workspaceId: string; parentMessageId?: string }) => <div role="dialog" aria-label="Tạo tài liệu">doc:{p.workspaceId}:{p.parentMessageId ?? '-'}</div> }))
vi.mock('../approval/CreateRequestDialog', async () => {
  const { Button } = await import('../primitives')
  return {
    CreateRequestDialog: (p: { onCreated: (r: { id: string; title: string }) => void; onClose: () => void }) => (
      <div role="dialog" aria-label="Tạo đề nghị">
        <Button onClick={() => { p.onCreated({ id: 'req-9', title: 'Mua thiết bị' }); p.onClose() }}>Gửi đề nghị giả</Button>
      </div>
    ),
  }
})

const send = vi.mocked(messagingApi.sendMessage)
const people = buildDirectory(CONTACTS)
const CHAN = CH.doisoat
const PLACEHOLDER = 'Tin nhắn cho Đối soát'

beforeEach(() => {
  phone = false
  isPersonal = false
  localStorage.clear()
  useDraftsStore.getState().reset()
  queryClient.setQueryData(keys.messaging.members(CHAN), { members: MEMBERS })
  send.mockResolvedValue({ id: 'm-new', channel_id: CHAN, sender_id: U.hoa.id, content: '' } as never)
})
afterEach(() => {
  resetClient()
  vi.clearAllMocks()
})

const lan = (over: Partial<Message> = {}): Message => ({
  id: 'q-1', channel_id: CHAN, sender_id: U.lan.id, sender_name: 'lan', content: '<p>Em thấy kỳ trước để 6427</p>', content_format: 'html',
  created_at: new Date(2026, 9, 9, 10, 2).toISOString(), ...over,
})

type Props = Parameters<typeof ChatEditor>[0]
const editorOf = (props: Partial<Props> = {}) => (
  <ChatEditor channelId={CHAN} people={people} placeholder={PLACEHOLDER} onSend={vi.fn()} {...props} />
)
function open(props: Partial<Props> = {}) {
  const onSend = vi.fn()
  const view = renderWithClient(editorOf({ onSend, ...props }))
  return { onSend, view, box: () => screen.getByRole('textbox', { name: /Tin nhắn cho Đối soát|Trả lời/ }) }
}
const quote = (m = lan(), scope = CHAN) => act(() => useDraftsStore.getState().setQuote(scope, quoteFromMessage(m)))
const strip = () => screen.queryByRole('region', { name: /Đang trích dẫn/ })
const said = () => document.querySelector<HTMLElement>('[data-composer-live]')!

describe('ChatEditor: the pending quote', () => {
  it('shows the strip as the first row of the box, with the quoted person and a line of their text', () => {
    open()
    expect(strip()).toBeNull()
    quote()
    const region = screen.getByRole('region', { name: 'Đang trích dẫn tin của Nguyễn Thu Lan' })
    expect(region).toHaveTextContent('Em thấy kỳ trước để 6427')
    expect(region.parentElement!.firstElementChild).toBe(region)
  })

  it('moves focus to the editor when a quote is chosen, keeps the typed text, and says so', async () => {
    const user = userEvent.setup()
    const { box } = open()
    await user.click(box())
    await user.keyboard('đang gõ')
    quote()
    await waitFor(() => expect(box()).toHaveFocus())
    expect(box()).toHaveTextContent('đang gõ')
    expect(said()).toHaveTextContent('Đang trích dẫn tin của Nguyễn Thu Lan. Esc để bỏ.')
  })

  it('does not steal focus for a quote that was already there when the composer opened', () => {
    quote()
    const { box } = open()
    expect(box()).not.toHaveFocus()
  })

  it('replaces the quote when another message is chosen (one quote per composer)', () => {
    open()
    quote()
    quote(lan({ id: 'q-2', sender_id: U.vinh.id, sender_name: 'vinh', content: '<p>Khác</p>' }))
    expect(screen.getAllByRole('region', { name: /Đang trích dẫn/ })).toHaveLength(1)
    expect(strip()).toHaveTextContent('Lê Quang Vinh')
  })

  it('takes the quote away with ✕ and gives focus back to the editor', async () => {
    const user = userEvent.setup()
    const { box } = open()
    quote()
    await user.click(screen.getByRole('button', { name: 'Bỏ trích dẫn' }))
    expect(strip()).toBeNull()
    expect(useDraftsStore.getState().drafts[CHAN]?.quote).toBeUndefined()
    await waitFor(() => expect(box()).toHaveFocus())
    expect(said()).toHaveTextContent('Đã bỏ trích dẫn')
  })

  it('takes the quote away with Esc, keeping the typed text', async () => {
    const user = userEvent.setup()
    const { box } = open()
    await user.click(box())
    await user.keyboard('trả lời')
    quote()
    await user.keyboard('{Escape}')
    expect(strip()).toBeNull()
    expect(box()).toHaveTextContent('trả lời')
  })

  it('Esc closes the attach menu first and only then the quote', async () => {
    const user = userEvent.setup()
    const uploads = { items: [], add: vi.fn(), remove: vi.fn(), retry: vi.fn(), clear: vi.fn(), ready: true } as never
    const { box } = open({ uploads })
    quote()
    await user.click(screen.getByRole('button', { name: 'Đính kèm' }))
    expect(screen.getByRole('menu', { name: 'Đính kèm' })).toBeInTheDocument()
    // Focus by keyboard: a click in the box would close the menu as an outside press.
    act(() => box().focus())
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('menu', { name: 'Đính kèm' })).toBeNull()
    expect(strip()).not.toBeNull()
    await user.keyboard('{Escape}')
    expect(strip()).toBeNull()
  })

  it('leaves Esc alone when there is neither a popover nor a quote, so a side panel can close', async () => {
    const user = userEvent.setup()
    const onKey = vi.fn()
    window.addEventListener('keydown', onKey)
    const { box } = open()
    await user.click(box())
    await user.keyboard('{Escape}')
    expect(onKey.mock.calls[onKey.mock.calls.length - 1]![0].defaultPrevented).toBe(false)
    window.removeEventListener('keydown', onKey)
  })

  it('does not open "edit last message" with ↑ while a quote is pending', async () => {
    const user = userEvent.setup()
    const onEditLast = vi.fn()
    const { box } = open({ onEditLast })
    await user.click(box())
    await user.keyboard('{ArrowUp}')
    expect(onEditLast).toHaveBeenCalledTimes(1)
    quote()
    await user.keyboard('{ArrowUp}')
    expect(onEditLast).toHaveBeenCalledTimes(1)
  })

  it('a quote alone is not a message: Gửi stays off until there is text', async () => {
    const user = userEvent.setup()
    const { box, onSend } = open()
    quote()
    expect(screen.getByRole('button', { name: /Gửi/ })).toBeDisabled()
    await user.click(box())
    await user.keyboard('{Enter}')
    expect(onSend).not.toHaveBeenCalled()
    await user.keyboard('ok')
    expect(screen.getByRole('button', { name: /Gửi/ })).toBeEnabled()
  })

  it('goes to the quoted message when the strip is pressed', async () => {
    const user = userEvent.setup()
    const original = document.createElement('div')
    original.setAttribute('data-message-id', 'q-1')
    original.scrollIntoView = vi.fn()
    document.body.appendChild(original)
    open()
    quote()
    await user.click(screen.getByRole('button', { name: /Nguyễn Thu Lan/ }))
    expect(original.scrollIntoView).toHaveBeenCalled()
    original.remove()
  })

  it('shows a quote of a message that was deleted meanwhile as gone, and still lets the text be sent', async () => {
    const user = userEvent.setup()
    const older = lan({ id: 'older', created_at: new Date(2026, 9, 9, 9, 0).toISOString() })
    queryClient.setQueryData(keys.messaging.messages(CHAN), { messages: [older, lan()], has_more: false })
    const { box, onSend } = open()
    quote()
    act(() => {
      queryClient.setQueryData(keys.messaging.messages(CHAN), { messages: [older], has_more: false })
    })
    expect(await screen.findByRole('region', { name: /tin này đã bị xoá/ })).toHaveTextContent('Tin của bạn sẽ được gửi không kèm trích dẫn.')
    expect(said()).toHaveTextContent('Tin bạn đang trích dẫn đã bị xoá')
    await user.click(box())
    await user.keyboard('vẫn gửi{Enter}')
    expect(onSend).toHaveBeenCalledWith('<p>vẫn gửi</p>', [])
  })
})

describe('ChatEditor: drafts per conversation', () => {
  it('keeps the typed text and the quote when the composer is left and opened again', async () => {
    const user = userEvent.setup()
    const first = open()
    await user.click(first.box())
    await user.keyboard('nháp dở')
    quote()
    first.view.unmount()

    const { box } = open()
    expect(box()).toHaveTextContent('nháp dở')
    expect(strip()).toHaveTextContent('Em thấy kỳ trước để 6427')
    expect(screen.getByRole('button', { name: /Gửi/ })).toBeEnabled()
  })

  it('keeps one draft per conversation and swaps it when the conversation changes', async () => {
    const user = userEvent.setup()
    const { view, box } = open()
    await user.click(box())
    await user.keyboard('thư A')
    quote()
    view.rerender(editorOf({ channelId: CH.vanhanh }))
    await waitFor(() => expect(box()).toHaveTextContent(/^$/))
    expect(strip()).toBeNull()
    view.rerender(editorOf({ channelId: CHAN }))
    await waitFor(() => expect(box()).toHaveTextContent('thư A'))
    expect(strip()).not.toBeNull()
  })

  it('keeps the thread composer apart from the conversation composer', async () => {
    const user = userEvent.setup()
    const main = open()
    await user.click(main.box())
    await user.keyboard('chính')
    main.view.unmount()
    const thread = open({ draftKey: 'thread:root', placeholder: 'Trả lời trong thread' })
    expect(thread.box()).toHaveTextContent(/^$/)
    expect(useDraftsStore.getState().drafts[CHAN]?.html).toBe('<p>chính</p>')
  })

  it('forgets the draft once the message is sent', async () => {
    const user = userEvent.setup()
    const { box, onSend } = open()
    await user.click(box())
    await user.keyboard('gửi đi{Enter}')
    await waitFor(() => expect(onSend).toHaveBeenCalled())
    await waitFor(() => expect(useDraftsStore.getState().drafts[CHAN]).toBeUndefined())
  })

  it('keeps the draft when the send fails', async () => {
    const user = userEvent.setup()
    const { box } = open({ onSend: vi.fn().mockRejectedValue(new Error('offline')) })
    await user.click(box())
    await user.keyboard('chưa gửi được{Enter}')
    await waitFor(() => expect(box()).toHaveTextContent('chưa gửi được'))
    expect(useDraftsStore.getState().drafts[CHAN]?.html).toBe('<p>chưa gửi được</p>')
  })
})

describe('ChatEditor: editing on a phone with a quote pending', () => {
  const edit = () => ({ initialHtml: '<p>Chị đã thêm 3 giao dịch</p>', onSave: vi.fn(), onCancel: vi.fn() })

  it('puts the edit bar where the strip was, sets the quote and the draft aside, and brings them back', async () => {
    phone = true
    const user = userEvent.setup()
    const { view, box } = open()
    await user.click(box())
    await user.keyboard('nháp')
    quote()
    expect(strip()).not.toBeNull()

    view.rerender(editorOf({ edit: edit() }))
    expect(strip()).toBeNull()
    expect(screen.getByText('Đang sửa tin')).toBeInTheDocument()
    expect(screen.getByText('Trích dẫn tin của Nguyễn Thu Lan sẽ quay lại sau khi sửa xong.')).toBeInTheDocument()
    expect(box()).toHaveTextContent('Chị đã thêm 3 giao dịch')
    // The edit text never lands in the draft.
    expect(useDraftsStore.getState().drafts[CHAN]?.html).toBe('<p>nháp</p>')

    view.rerender(editorOf())
    expect(strip()).not.toBeNull()
    expect(screen.queryByText(/sẽ quay lại sau khi sửa xong/)).toBeNull()
    await waitFor(() => expect(box()).toHaveTextContent('nháp'))
  })

  it('says it is editing to the menus, and stops when it is done', () => {
    phone = true
    const { view } = open({ edit: edit() })
    expect(useDraftsStore.getState().editing[CHAN]).toBe(true)
    view.unmount()
    expect(useDraftsStore.getState().editing[CHAN]).toBeUndefined()
  })

  it('has no note when no quote is pending', () => {
    phone = true
    open({ edit: edit() })
    expect(screen.queryByText(/sẽ quay lại sau khi sửa xong/)).toBeNull()
  })
})

describe('ChatEditor: the "+" menu on a wide screen', () => {
  const uploads = { items: [], add: vi.fn(), remove: vi.fn(), retry: vi.fn(), clear: vi.fn(), ready: true } as never

  it('offers the cards in their own menu, next to the attach button', async () => {
    const user = userEvent.setup()
    open({ uploads })
    const plus = screen.getByRole('button', { name: 'Tạo trong chat' })
    expect(plus).toHaveAttribute('aria-expanded', 'false')
    await user.click(plus)
    const menu = await screen.findByRole('menu', { name: 'Tạo trong chat' })
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      'Bình chọn', 'Công việc', 'Tài liệu mới', 'Đề nghị phê duyệt',
    ])
    expect(within(menu).queryByRole('separator')).toBeNull()
    // The attach menu keeps only attaching.
    expect(screen.getByRole('button', { name: 'Đính kèm' })).toBeInTheDocument()
  })

  it('hides the approval request in a personal workspace', async () => {
    isPersonal = true
    const user = userEvent.setup()
    open()
    await user.click(screen.getByRole('button', { name: 'Tạo trong chat' }))
    const menu = await screen.findByRole('menu', { name: 'Tạo trong chat' })
    expect(within(menu).queryByRole('menuitem', { name: 'Đề nghị phê duyệt' })).toBeNull()
    expect(within(menu).getAllByRole('menuitem')).toHaveLength(3)
  })

  it.each([
    ['Bình chọn', 'Tạo bình chọn', `poll:${CHAN}:-`],
    ['Công việc', 'Tạo công việc', `task:${CHAN}:-`],
    ['Tài liệu mới', 'Tạo tài liệu', 'doc:ws-1:-'],
  ])('opens the %s dialog for this conversation, with no thread in the main composer', async (item, dialog, text) => {
    const user = userEvent.setup()
    open()
    await user.click(screen.getByRole('button', { name: 'Tạo trong chat' }))
    await user.click(await screen.findByRole('menuitem', { name: item }))
    expect(await screen.findByRole('dialog', { name: dialog })).toHaveTextContent(text)
  })

  it('passes the thread root to the dialog from the thread composer', async () => {
    const user = userEvent.setup()
    open({ parentMessageId: 'root-7', draftKey: 'thread:root-7' })
    await user.click(screen.getByRole('button', { name: 'Tạo trong chat' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Bình chọn' }))
    expect(await screen.findByRole('dialog', { name: 'Tạo bình chọn' })).toHaveTextContent(`poll:${CHAN}:root-7`)
  })

  it('is not there while a message is being edited', () => {
    open({ edit: { initialHtml: '<p>x</p>', onSave: vi.fn(), onCancel: vi.fn() } })
    expect(screen.queryByRole('button', { name: 'Tạo trong chat' })).toBeNull()
  })

  it('creates an approval request, then posts it to the conversation as a card message', async () => {
    const user = userEvent.setup()
    open()
    await user.click(screen.getByRole('button', { name: 'Tạo trong chat' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Đề nghị phê duyệt' }))
    expect(send).not.toHaveBeenCalled()
    await user.click(await screen.findByRole('button', { name: 'Gửi đề nghị giả' }))
    await waitFor(() => expect(send).toHaveBeenCalled())
    expect(send).toHaveBeenCalledWith(CHAN, expect.objectContaining({ linked_entity_type: 'approval', linked_entity_id: 'req-9', content: '' }))
    expect(send.mock.calls[0]![1]).not.toHaveProperty('parent_message_id')
    expect(screen.queryByText(/Gửi vào/)).toBeNull()
  })

  it('posts the approval card as a reply from the thread composer', async () => {
    const user = userEvent.setup()
    open({ parentMessageId: 'root-7', draftKey: 'thread:root-7' })
    await user.click(screen.getByRole('button', { name: 'Tạo trong chat' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Đề nghị phê duyệt' }))
    await user.click(await screen.findByRole('button', { name: 'Gửi đề nghị giả' }))
    await waitFor(() => expect(send).toHaveBeenCalledWith(CHAN, expect.objectContaining({ parent_message_id: 'root-7', linked_entity_id: 'req-9' })))
  })

  it('does not carry the pending quote on the card message', async () => {
    const user = userEvent.setup()
    open()
    quote()
    await user.click(screen.getByRole('button', { name: 'Tạo trong chat' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Đề nghị phê duyệt' }))
    await user.click(await screen.findByRole('button', { name: 'Gửi đề nghị giả' }))
    await waitFor(() => expect(send).toHaveBeenCalled())
    expect(send.mock.calls[0]![1]).not.toHaveProperty('quoted_message_id')
    expect(strip()).not.toBeNull()
  })

  it('shows no id in the menu', async () => {
    const user = userEvent.setup()
    open()
    await user.click(screen.getByRole('button', { name: 'Tạo trong chat' }))
    await screen.findByRole('menu', { name: 'Tạo trong chat' })
    expect(document.body.innerHTML).not.toMatch(UUID_RE)
  })
})

describe('ChatEditor: the "+" sheet on a phone', () => {
  const uploads = { items: [], add: vi.fn(), remove: vi.fn(), retry: vi.fn(), clear: vi.fn(), ready: true } as never

  it('has one button; its sheet holds the attach row and the cards below it, rows of 44px', async () => {
    phone = true
    const user = userEvent.setup()
    open({ uploads })
    expect(screen.queryByRole('button', { name: 'Tạo trong chat' })).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Đính kèm' }))
    const sheet = screen.getByRole('dialog', { name: 'Tạo và đính kèm' })
    for (const n of ['Máy ảnh', 'Ảnh', 'Tệp']) expect(within(sheet).getByRole('button', { name: n })).toBeInTheDocument()
    const rows = within(sheet).getAllByRole('menuitem')
    expect(rows.map((r) => r.textContent)).toEqual(['Bình chọn', 'Công việc', 'Tài liệu mới', 'Đề nghị phê duyệt'])
    for (const r of rows) expect(r.className).toContain('h-11')
  })

  it('hides the approval request in a personal workspace', async () => {
    phone = true
    isPersonal = true
    const user = userEvent.setup()
    open({ uploads })
    await user.click(screen.getByRole('button', { name: 'Đính kèm' }))
    expect(within(screen.getByRole('dialog', { name: 'Tạo và đính kèm' })).queryByRole('menuitem', { name: 'Đề nghị phê duyệt' })).toBeNull()
  })

  it('closes the sheet and opens the dialog of the chosen card', async () => {
    phone = true
    const user = userEvent.setup()
    open({ uploads, parentMessageId: 'root-7', draftKey: 'thread:root-7' })
    await user.click(screen.getByRole('button', { name: 'Đính kèm' }))
    await user.click(screen.getByRole('menuitem', { name: 'Công việc' }))
    expect(screen.queryByRole('dialog', { name: 'Tạo và đính kèm' })).toBeNull()
    expect(await screen.findByRole('dialog', { name: 'Tạo công việc' })).toHaveTextContent(`task:${CHAN}:root-7`)
  })

  it('still offers the cards when the composer cannot attach files', async () => {
    phone = true
    const user = userEvent.setup()
    open()
    await user.click(screen.getByRole('button', { name: 'Tạo trong chat' }))
    const sheet = screen.getByRole('dialog', { name: 'Tạo và đính kèm' })
    expect(within(sheet).queryByRole('button', { name: 'Máy ảnh' })).toBeNull()
    expect(within(sheet).getAllByRole('menuitem')).toHaveLength(4)
  })
})
