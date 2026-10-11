import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { U, WS_ID } from '../../test/chat-fixtures'
import type { PeopleDirectory, Person } from '../../lib/people'
import type { DriveItem } from '../../api/drive'
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

const mockFolder: DriveItem = {
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

const mockFile1: DriveItem = {
  id: FILE_1_ID,
  workspace_id: WS_ID,
  drive_context: 'channel',
  drive_context_id: CHANNEL_ID,
  parent_id: FOLDER_ID,
  item_type: 'file',
  name: 'bao-cao.pdf',
  mime_type: 'application/pdf',
  size_bytes: 1024 * 1024,
  ngac_node_id: 'node-file-1',
  owner_id: U.duc.id,
  status: 'active',
  created_at: '2026-10-10T09:00:00Z',
  updated_at: '2026-10-10T09:00:00Z',
}

const mockFile2: DriveItem = {
  id: FILE_2_ID,
  workspace_id: WS_ID,
  drive_context: 'channel',
  drive_context_id: CHANNEL_ID,
  parent_id: FOLDER_ID,
  item_type: 'file',
  name: 'bang-tinh.xlsx',
  mime_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  size_bytes: 512 * 1024,
  ngac_node_id: 'node-file-2',
  owner_id: U.lan.id,
  status: 'active',
  created_at: '2026-10-10T10:00:00Z',
  updated_at: '2026-10-10T10:00:00Z',
}

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => {
  const actual = await orig<typeof import('../../api/client')>()
  return {
    ...actual,
    apiFetch: vi.fn((path: string) => {
      if (path.includes('/drive?drive_context=channel')) {
        return Promise.resolve({ items: [mockFolder] })
      }
      if (path.startsWith(`/drive/folders/${FOLDER_ID}`)) {
        return Promise.resolve({ items: [mockFile1, mockFile2] })
      }
      if (path.includes('/preview')) {
        return Promise.resolve({ kind: 'unavailable', reason: 'unsupported' })
      }
      return Promise.resolve({})
    }),
  }
})

beforeEach(() => {
  routerState.navigate.mockReset()
})
afterEach(resetClient)

describe('SpaceFiles', () => {
  it('clicking a file row opens FileViewer with the tab files list, origin chat, and opens in drive on demand', async () => {
    const user = userEvent.setup()
    const onGoToChat = vi.fn()

    renderWithClient(
      <SpaceFiles
        workspaceId={WS_ID}
        channelId={CHANNEL_ID}
        people={mockPeople}
        onGoToChat={onGoToChat}
      />,
    )

    // Wait for files to load
    const file1Btn = await screen.findByRole('button', { name: /bao-cao\.pdf/i })
    expect(file1Btn).toBeInTheDocument()

    // Clicking file opens FileViewer dialog
    await user.click(file1Btn)

    const viewer = await screen.findByRole('dialog', { name: /bao-cao\.pdf/i })
    expect(viewer).toBeInTheDocument()

    // origin: 'chat' words sender as "gửi lúc"
    expect(within(viewer).getByText(/gửi lúc/i)).toBeInTheDocument()

    // "Mở trong Tài liệu" is available because parentFolderId is present
    const openInDriveBtn = within(viewer).getByRole('button', { name: 'Mở trong Tài liệu' })
    expect(openInDriveBtn).toBeInTheDocument()

    await user.click(openInDriveBtn)
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/drive',
      search: expect.any(Function),
    })
    const searchFn = routerState.navigate.mock.calls[0]![0].search
    expect(searchFn({})).toEqual({ folder: FOLDER_ID })

    // Test navigation to prev file (bang-tinh.xlsx was updated at 10:00, bao-cao.pdf at 09:00)
    const prevBtn = within(viewer).getAllByRole('button', { name: 'Tệp trước' })[0]!
    await user.click(prevBtn)
    expect(within(viewer).getByText('bang-tinh.xlsx')).toBeInTheDocument()

    // Esc closes FileViewer and returns focus to clicked row
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(file1Btn).toHaveFocus()

    // UUID rule: no raw IDs in document
    const UUID_RE = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })
})
