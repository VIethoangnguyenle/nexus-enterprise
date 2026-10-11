import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, UUID_RE, WS_ID, U } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import type { Message } from '../../api/messaging'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { useToastStore } from '../primitives'
import { useNavCollapseStore } from '../../stores/nav-collapse.store'
import { PhoneChatList } from './PhoneChatList'
import { useSectionOverlays } from './SectionOverlays'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

let server: ReturnType<typeof createChatStateServer>
const serve = (...args: Parameters<typeof createChatStateServer>) => {
  server = createChatStateServer(...args)
  vi.mocked(apiFetch).mockImplementation(server.api as never)
}

function Host() {
  const nav = useChatNavigation(WS_ID)
  const overlays = useSectionOverlays(nav.sectionsApi, true)
  return (
    <>
      <PhoneChatList nav={nav} workspaceId={WS_ID} fresh={new Map()} overlays={overlays} />
      {overlays.node}
    </>
  )
}

beforeEach(() => {
  routerState.navigate.mockClear()
  localStorage.clear()
  serve()
})
afterEach(() => {
  vi.useRealTimers()
  resetClient()
  useToastStore.getState().clear()
  useNavCollapseStore.setState({ collapsed: {} })
})

const ready = async () => {
  renderWithClient(<Host />)
  await waitFor(() => expect(screen.getAllByRole('link', { name: 'Đối soát giao dịch' }).length).toBeGreaterThan(0))
}
const row = (name: string | RegExp) => screen.getByRole('link', { name }).closest('li')!
const pressLong = (el: HTMLElement) => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  fireEvent.pointerDown(el, { pointerType: 'touch', clientX: 5, clientY: 5 })
  act(() => { vi.advanceTimersByTime(401) })
}
const titles = (name: string) => within(screen.getByRole('list', { name })).getAllByRole('link').map((a) => a.getAttribute('aria-label'))

describe('the two shortcut tiles', () => {
  it('open Được nhắc tới and Đã gắn sao from the top of the list, 48px high', async () => {
    serve({}, 3)
    await ready()
    const mentions = screen.getByRole('link', { name: /Được nhắc tới/ })
    const starred = screen.getByRole('link', { name: /Đã gắn sao/ })
    expect(mentions).toHaveAttribute('href', '/channels/mentions')
    expect(starred).toHaveAttribute('href', '/channels/starred')
    expect(mentions.className).toContain('min-h-12')
    await waitFor(() => expect(within(mentions).getByLabelText('3 tin nhắc tên bạn chưa đọc')).toBeInTheDocument())
    expect(within(starred).queryByLabelText(/chưa đọc/)).toBeNull()
  })

  it('come before every list, and are not part of one', async () => {
    await ready()
    const tiles = screen.getByRole('link', { name: /Được nhắc tới/ })
    const firstList = screen.getAllByRole('list')[0]!
    expect(tiles.compareDocumentPosition(firstList) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})

describe('Đã gắn sao on a phone', () => {
  it('is hidden with nothing starred', async () => {
    await ready()
    expect(screen.queryByRole('list', { name: 'Đã gắn sao' })).toBeNull()
  })

  it('lists starred conversations first, once, with no ⋯ and no star', async () => {
    serve({ [CH.hopdong]: { starred: true } })
    renderWithClient(<Host />)
    await waitFor(() => expect(screen.getByRole('list', { name: 'Đã gắn sao' })).toBeInTheDocument())
    expect(screen.getAllByRole('list').map((l) => l.getAttribute('aria-label'))[0]).toBe('Đã gắn sao')
    expect(titles('Đã gắn sao')).toEqual(['Hợp đồng đối tác'])
    expect(titles('Nhóm')).not.toContain('Hợp đồng đối tác')
    expect(screen.queryByRole('button', { name: /Tuỳ chọn cho mục Đã gắn sao/ })).toBeNull()
    expect(screen.getAllByText('Hợp đồng đối tác')).toHaveLength(1)
  })
})

describe('the long-press sheet: star and mute', () => {
  it('a plain row offers Gắn sao and Tắt thông báo after the section rows, 52px each', async () => {
    await ready()
    pressLong(row('Vận hành chung'))
    const sheet = await screen.findByRole('dialog', { name: 'Tuỳ chọn cho Vận hành chung' })
    const labels = within(sheet).getAllByRole('button').map((b) => b.textContent)
    expect(labels.slice(-2)).toEqual(['Gắn sao', 'Tắt thông báo'])
    expect(within(sheet).getByRole('button', { name: 'Gắn sao' }).className).toContain('min-h-13')
  })

  it('a starred row offers Bỏ gắn sao and the mute only', async () => {
    serve({ [CH.hopdong]: { starred: true } })
    renderWithClient(<Host />)
    await waitFor(() => expect(screen.getByRole('list', { name: 'Đã gắn sao' })).toBeInTheDocument())
    pressLong(row('Hợp đồng đối tác'))
    const sheet = await screen.findByRole('dialog', { name: 'Tuỳ chọn cho Hợp đồng đối tác' })
    expect(within(sheet).getAllByRole('button').map((b) => b.textContent)).toEqual(['Bỏ gắn sao', 'Tắt thông báo'])
  })

  it('a muted row offers Bật thông báo instead', async () => {
    serve({ [CH.vanhanh]: { notification_level: 'off' } })
    renderWithClient(<Host />)
    await waitFor(() => expect(row(/^Vận hành chung, đã tắt thông báo$/).querySelector('a')).toHaveAttribute('data-muted', 'true'))
    pressLong(row(/^Vận hành chung/))
    const sheet = await screen.findByRole('dialog')
    expect(within(sheet).getByRole('button', { name: 'Bật thông báo' })).toBeInTheDocument()
    expect(within(sheet).queryByRole('button', { name: 'Tắt thông báo' })).toBeNull()
  })

  it('choosing Gắn sao closes the sheet, calls the server, and moves the row', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await ready()
    pressLong(row('Vận hành chung'))
    await user.click(await screen.findByRole('button', { name: 'Gắn sao' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    await waitFor(() => expect(titles('Đã gắn sao')).toEqual(['Vận hành chung']))
    expect(server.calls).toContainEqual(expect.objectContaining({ method: 'PUT', path: `/channels/${CH.vanhanh}/star` }))
  })

  it('choosing Tắt thông báo mutes the row: no count, a bell-off', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await ready()
    pressLong(row('Đối soát giao dịch'))
    await user.click(await screen.findByRole('button', { name: 'Tắt thông báo' }))
    await waitFor(() => expect(row(/^Đối soát giao dịch, đã tắt thông báo$/).querySelector('a')).toHaveAttribute('data-muted', 'true'))
    expect(within(row(/^Đối soát giao dịch/)).queryByLabelText('3 tin chưa đọc')).toBeNull()
  })
})

describe('muted rows on a phone', () => {
  it('look like the desktop: no count, "@" for a mention, bell-off, spoken as muted', async () => {
    serve({ [CH.doisoat]: { notification_level: 'off', unread_count: 0, mention_count: 2 } })
    renderWithClient(<Host />)
    const link = await waitFor(() => row(/^Đối soát giao dịch, đã tắt thông báo$/))
    expect(within(link).getByLabelText('2 lần nhắc tên bạn')).toHaveTextContent('@')
    expect(within(link).queryByLabelText(/tin chưa đọc/)).toBeNull()
    expect(link.querySelector('svg.lucide-bell-off')).not.toBeNull()
  })

  it('a folded group totals by level: the muted conversation adds only its mentions', async () => {
    const user = userEvent.setup()
    serve({ [CH.doisoat]: { notification_level: 'off', unread_count: 0, mention_count: 1 } })
    renderWithClient(<Host />)
    const toggle = await screen.findByRole('button', { name: 'Nhóm' })
    await user.click(toggle)
    await waitFor(() => expect(within(toggle).getByLabelText('1 tin chưa đọc')).toBeInTheDocument())
  })
})

describe('Thread có tin mới on a phone', () => {
  const root = (n: number): Message => ({
    id: `1b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a0${n}`, channel_id: CH.doisoat, sender_id: U.vinh.id, sender_name: 'Lê Quang Vinh',
    content: `<p>Ảnh sao kê số ${n}</p>`, content_format: 'html', reply_count: 4, thread_unread_count: n,
    last_reply_at: { seconds: 1760000000 + n, nanos: 0 }, reply_participant_ids: [],
  })

  it('sits at the top of the list, two rows and "Xem thêm", and opens the group on that thread', async () => {
    const user = userEvent.setup()
    serve({}, 0, { threads: [root(1), root(2), root(3)] })
    await ready()
    const heading = await screen.findByRole('heading', { name: 'Thread có tin mới' })
    const rows = within(screen.getByRole('list', { name: 'Thread có tin mới' })).getAllByRole('button', { name: /^Thread của/ })
    expect(rows).toHaveLength(2)
    expect(screen.getByRole('button', { name: 'Xem thêm 1 thread' })).toBeInTheDocument()
    expect(heading.compareDocumentPosition(screen.getByRole('list', { name: 'Nhóm' })) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    await user.click(rows[0]!)
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels/$channelId', params: { channelId: CH.doisoat }, search: { thread: root(1).id },
    })
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('is not there when no followed thread has news', async () => {
    await ready()
    expect(screen.queryByRole('heading', { name: 'Thread có tin mới' })).toBeNull()
  })
})
