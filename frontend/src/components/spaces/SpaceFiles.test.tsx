import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { U, WS_ID } from '../../test/chat-fixtures'
import type { PeopleDirectory, Person } from '../../lib/people'
import type { DriveItem } from '../../api/drive'
import { useFileViewStore } from '../../stores/file-view.store'
import { SpaceFiles } from './SpaceFiles'

const CHANNEL_ID = '00000000-0000-0000-0000-000000000001'
const FOLDER_ID = '11111111-1111-4111-8111-111111111111'
const FILE_1_ID = '22222222-2222-4222-8222-222222222221'
const FILE_2_ID = '22222222-2222-4222-8222-222222222222'

const mockDuc: Person = {
  userId: U.duc.id,
  nodeId: U.duc.node,
  username: 'duc',
  name: 'Trần Minh Đức',
  role: '',
  avatarUrl: '',
}
const mockLan: Person = {
  userId: U.lan.id,
  nodeId: U.lan.node,
  username: 'lan',
  name: 'Nguyễn Thu Lan',
  role: '',
  avatarUrl: '',
}

const mockPeople: PeopleDirectory = {
  list: [mockDuc, mockLan],
  byUserId: new Map([
    [U.duc.id, mockDuc],
    [U.lan.id, mockLan],
  ]),
  byNodeId: new Map([
    [U.duc.node, mockDuc],
    [U.lan.node, mockLan],
  ]),
  byUsername: new Map([
    ['duc', mockDuc],
    ['lan', mockLan],
  ]),
}

const NOW = new Date('2026-10-16T12:00:00Z')

const folder: DriveItem = {
  id: FOLDER_ID,
  workspace_id: WS_ID,
  drive_context: 'channel',
  drive_context_id: CHANNEL_ID,
  item_type: 'folder',
  name: 'Đối soát',
  ngac_node_id: 'node-folder',
  owner_id: U.duc.id,
  status: 'active',
  created_at: '2026-10-10T08:00:00Z',
  updated_at: '2026-10-10T08:00:00Z',
}

let seq = 0
function file(name: string, mime: string, at: string, owner = U.duc.id): DriveItem {
  seq += 1
  return {
    id: `22222222-2222-4222-8222-${String(seq).padStart(12, '0')}`,
    workspace_id: WS_ID,
    drive_context: 'channel',
    drive_context_id: CHANNEL_ID,
    parent_id: FOLDER_ID,
    item_type: 'file',
    name,
    mime_type: mime,
    size_bytes: 1024 * 1024,
    ngac_node_id: `node-${seq}`,
    owner_id: owner,
    status: 'active',
    created_at: at,
    updated_at: at,
  }
}

const PDF = 'application/pdf'
const XLSX = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'

// Friday 16 Oct 2026: two pictures today, a PDF earlier this week, a sheet earlier this month, older ones.
function fullSet(): DriveItem[] {
  return [
    file('anh-1.jpg', 'image/jpeg', '2026-10-16T08:00:00Z'),
    file('anh-2.png', 'image/png', '2026-10-16T07:00:00Z'),
    file('bao-cao.pdf', PDF, '2026-10-13T08:00:00Z', U.lan.id),
    file('bang-tinh.xlsx', XLSX, '2026-10-05T08:00:00Z'),
    file('luu-tru.zip', 'application/zip', '2026-09-20T08:00:00Z'),
    file('anh-cu.jpg', 'image/jpeg', '2026-09-18T08:00:00Z'),
  ]
}

let files: DriveItem[] = []

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => {
  const actual = await orig<typeof import('../../api/client')>()
  return {
    ...actual,
    apiFetch: vi.fn((path: string) => {
      if (path.includes('/drive?drive_context=channel')) return Promise.resolve({ items: [folder] })
      if (path.startsWith(`/drive/folders/${FOLDER_ID}`)) return Promise.resolve({ items: files })
      if (path.includes('/preview')) return Promise.resolve({ kind: 'unavailable', reason: 'unsupported' })
      return Promise.resolve({})
    }),
  }
})

function renderTab(onGoToChat = vi.fn()) {
  return renderWithClient(
    <SpaceFiles workspaceId={WS_ID} channelId={CHANNEL_ID} people={mockPeople} onGoToChat={onGoToChat} now={NOW} />,
  )
}

const chip = (name: string) => screen.getByRole('radio', { name })
const names = () =>
  screen.getAllByRole('button').map((b) => b.getAttribute('aria-label') ?? '').filter((n) => /^[\w-]+\.(jpg|png|pdf|xlsx|zip)$/.test(n))

beforeEach(() => {
  routerState.navigate.mockReset()
  files = fullSet()
  useFileViewStore.setState({ modes: { drive: 'grid', 'space-files': 'grid' } })
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(null, { status: 404 }))))
})
afterEach(() => {
  resetClient()
  vi.unstubAllGlobals()
})

describe('SpaceFiles', () => {
  it('groups files by recency under Hôm nay, Tuần này, Tháng này and Cũ hơn', async () => {
    renderTab()
    await screen.findByRole('button', { name: 'bao-cao.pdf' })
    expect(screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual([
      'Hôm nay', 'Tuần này', 'Tháng này', 'Cũ hơn',
    ])
  })

  it('lays a group of only pictures out as a square album and the rest as cards', async () => {
    renderTab()
    await screen.findByRole('button', { name: 'anh-1.jpg' })
    const today = screen.getByRole('group', { name: 'Hôm nay' })
    expect(today.className).toContain('minmax(112px')
    const week = screen.getByRole('group', { name: 'Tuần này' })
    expect(week.className).toContain('minmax(200px')
    // The older group mixes a zip and a picture: cards, not an album.
    expect(screen.getByRole('group', { name: 'Cũ hơn' }).className).toContain('minmax(200px')
  })

  it('filters by kind with the chips', async () => {
    const user = userEvent.setup()
    renderTab()
    await screen.findByRole('button', { name: 'bao-cao.pdf' })
    expect(chip('Tất cả')).toBeChecked()

    await user.click(chip('Ảnh & video'))
    expect(chip('Ảnh & video')).toBeChecked()
    expect(names()).toEqual(['anh-1.jpg', 'anh-2.png', 'anh-cu.jpg'])

    await user.click(chip('Tài liệu'))
    expect(names()).toEqual(['bao-cao.pdf', 'bang-tinh.xlsx'])

    await user.click(chip('Khác'))
    expect(names()).toEqual(['luu-tru.zip'])
  })

  it('opens the viewer on the filtered list so the arrows never reach a hidden file', async () => {
    const user = userEvent.setup()
    renderTab()
    await screen.findByRole('button', { name: 'bao-cao.pdf' })
    await user.click(chip('Tài liệu'))
    await user.click(screen.getByRole('button', { name: 'bang-tinh.xlsx' }))

    const viewer = await screen.findByRole('dialog', { name: /bang-tinh\.xlsx/ })
    // Second of two documents: nothing after it, even though a zip follows in the unfiltered list.
    for (const next of within(viewer).getAllByRole('button', { name: 'Tệp sau' })) expect(next).toBeDisabled()
    await user.click(within(viewer).getAllByRole('button', { name: 'Tệp trước' })[0]!)
    expect(within(viewer).getByText('bao-cao.pdf')).toBeInTheDocument()
    for (const prev of within(viewer).getAllByRole('button', { name: 'Tệp trước' })) expect(prev).toBeDisabled()
  })

  it('opens the viewer at the clicked file, opens in Tài liệu on demand and returns focus on Esc', async () => {
    const user = userEvent.setup()
    renderTab()
    const tile = await screen.findByRole('button', { name: 'bao-cao.pdf' })
    await user.click(tile)

    const viewer = await screen.findByRole('dialog', { name: /bao-cao\.pdf/i })
    expect(within(viewer).getByText(/gửi lúc/i)).toBeInTheDocument()
    await user.click(within(viewer).getByRole('button', { name: 'Mở trong Tài liệu' }))
    const searchFn = routerState.navigate.mock.calls[0]![0].search
    expect(searchFn({})).toEqual({ folder: FOLDER_ID })

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(tile).toHaveFocus()

    const UUID_RE = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('shows who shared a file by name and offers no jump to the message the data cannot locate', async () => {
    const user = userEvent.setup()
    renderTab()
    const tile = await screen.findByRole('button', { name: 'bao-cao.pdf' })
    expect(screen.getAllByText(/Thu Lan/).length).toBeGreaterThan(0)
    await user.click(within(tile).getByRole('button', { name: 'Tùy chọn cho bao-cao.pdf' }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getByRole('menuitem', { name: 'Xem trước' })).toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: 'Mở trong Tài liệu' })).toBeInTheDocument()
    expect(within(menu).queryByRole('menuitem', { name: 'Tới tin nhắn' })).toBeNull()
  })

  it('says so per chip when nothing of that kind exists', async () => {
    const user = userEvent.setup()
    files = [file('bao-cao.pdf', PDF, '2026-10-16T08:00:00Z')]
    renderTab()
    await screen.findByRole('button', { name: 'bao-cao.pdf' })
    await user.click(chip('Ảnh & video'))
    expect(screen.getByText('Chưa có ảnh hay video nào trong nhóm.')).toBeInTheDocument()
  })

  it('says so for documents when there are only pictures', async () => {
    const user = userEvent.setup()
    files = [file('anh-1.jpg', 'image/jpeg', '2026-10-16T08:00:00Z')]
    renderTab()
    await screen.findByRole('button', { name: 'anh-1.jpg' })
    await user.click(chip('Tài liệu'))
    expect(screen.getByText('Chưa có tài liệu nào trong nhóm.')).toBeInTheDocument()
  })

  it('keeps the original empty text and the way to attach when the group has no files at all', async () => {
    const user = userEvent.setup()
    const onGoToChat = vi.fn()
    files = []
    renderTab(onGoToChat)
    expect(await screen.findByText('Chưa có tệp nào. Tệp đính kèm trong trò chuyện sẽ hiện ở đây.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Đính kèm trong Trò chuyện' }))
    expect(onGoToChat).toHaveBeenCalled()
  })

  it('switches to compact rows and remembers the choice', async () => {
    const user = userEvent.setup()
    const first = renderTab()
    await screen.findByRole('button', { name: 'bao-cao.pdf' })
    await user.click(screen.getByRole('button', { name: 'Danh sách' }))
    expect(await screen.findByRole('button', { name: 'Xem trước bao-cao.pdf' })).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'Hôm nay' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Danh sách' })).toHaveAttribute('aria-pressed', 'true')
    first.unmount()

    renderTab()
    expect(await screen.findByRole('button', { name: 'Xem trước bao-cao.pdf' })).toBeInTheDocument()
    expect(JSON.parse(localStorage.getItem('ngac-file-view') ?? '{}').state.modes['space-files']).toBe('list')
  })
})
