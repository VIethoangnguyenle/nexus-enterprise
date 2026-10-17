import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { DriveItem, DriveShare } from '../../api/drive'
import { NO_PERMS } from '../../api/access'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, U, UUID_RE } from '../../test/chat-fixtures'
import { renderWithClient } from '../../test/render'
import { DriveDetailPanel } from './DriveDetailPanel'

const navigate = vi.fn()
vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))

const shares = vi.hoisted(() => ({ list: [] as unknown[] }))
vi.mock('../../hooks/useDrive', async (orig) => ({
  ...(await orig<typeof import('../../hooks/useDrive')>()),
  useDriveShares: () => ({ data: shares.list, isLoading: false, isError: false }),
}))
vi.mock('../../hooks/useDownloadUrl', () => ({ useDownloadFile: () => ({ download: vi.fn(), isDownloading: false }) }))

const HIDDEN = '11111111-aaaa-4bbb-8ccc-0000000000ff'
const HIDDEN_NODE = '22222222-aaaa-4bbb-8ccc-0000000000ff'
const RESOLVED = { userId: HIDDEN, nodeId: HIDDEN_NODE, name: 'Nguyễn Hữu Thành', avatarUrl: '' }
const people = buildDirectory(CONTACTS, {
  resolved: { byUserId: new Map([[HIDDEN, RESOLVED]]), byNodeId: new Map([[HIDDEN_NODE, RESOLVED]]) },
})

const item = (owner_id: string, owner_name?: string): DriveItem => ({
  id: '77777777-aaaa-4bbb-8ccc-000000000001', workspace_id: 'w', drive_context: 'personal', item_type: 'file',
  name: 'bien-ban.pdf', mime_type: 'application/pdf', size_bytes: 1000, ngac_node_id: 'n', owner_id,
  ...(owner_name ? { owner_name } : {}), status: 'active', created_at: '2026-10-09T01:30:00Z', updated_at: '2026-10-09T01:30:00Z',
})
const share = (target: string, label: string): DriveShare => ({
  id: 's1', drive_item_id: 'i', share_type: 'user', target_ngac_id: target, target_label: label, operations: ['read'],
  created_at: '2026-10-09T01:30:00Z',
})

const panel = (i: DriveItem) =>
  renderWithClient(<DriveDetailPanel item={i} perms={NO_PERMS} people={people} onClose={() => {}} onShare={() => {}} />)

beforeEach(() => { navigate.mockReset(); shares.list = [] })

describe('DriveDetailPanel: pressing the owner and the people with access', () => {
  it('a hidden owner opens the small card from every place their name appears', async () => {
    panel(item(HIDDEN, 'Nguyễn Hữu Thành'))
    const owner = screen.getByText('Chủ sở hữu').nextElementSibling as HTMLElement
    await userEvent.click(within(owner).getByRole('button', { name: 'Nguyễn Hữu Thành' }))
    const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    expect(within(card).getByText('Hồ sơ này không còn hiển thị với bạn.')).toBeInTheDocument()
    expect(within(card).queryAllByRole('button')).toHaveLength(0)
    expect(navigate).not.toHaveBeenCalled()
    // Access list and activity line name the same person and are pressable too.
    expect(within(screen.getByRole('list', { name: 'Hoạt động' })).getAllByRole('button', { name: 'Nguyễn Hữu Thành' }).length).toBeGreaterThan(0)
    expect(within(screen.getByRole('list', { name: 'Người có quyền' })).getAllByRole('button', { name: 'Nguyễn Hữu Thành' }).length).toBeGreaterThan(0)
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('a person it was shared with opens their profile when visible, the card when not', async () => {
    shares.list = [share(U.duc.node, 'x'), share(HIDDEN_NODE, 'y')]
    panel(item(U.hoa.id, 'Lê Thị Hoa'))
    const access = screen.getByRole('list', { name: 'Người có quyền' })
    await userEvent.click(within(access).getAllByRole('button', { name: 'Trần Minh Đức' })[0]!)
    expect(navigate.mock.calls[0]![0]).toMatchObject({ to: '/contacts' })
    await userEvent.click(within(access).getAllByRole('button', { name: 'Nguyễn Hữu Thành' })[0]!)
    expect(await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })).toBeInTheDocument()
  })

  it('a group or public share has no profile to open', () => {
    shares.list = [{ ...share('g-node', 'Kế toán'), share_type: 'group' }, { ...share('', ''), share_type: 'public' }]
    panel(item(U.hoa.id, 'Lê Thị Hoa'))
    const access = screen.getByRole('list', { name: 'Người có quyền' })
    expect(within(access).queryByRole('button', { name: 'Kế toán' })).toBeNull()
    expect(within(access).queryByRole('button', { name: 'Bất kỳ ai có liên kết' })).toBeNull()
  })
})
