import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi } from '../../api/messaging'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, CH, U, UUID_RE } from '../../test/chat-fixtures'
import { renderWithClient, resetClient } from '../../test/render'
import { MembersPanel } from './MembersPanel'

const navigate = vi.fn()
vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))

vi.mock('../../api/messaging', async (orig) => {
  const real = await orig<typeof import('../../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, listMembers: vi.fn(), removeMember: vi.fn() } }
})

const people = buildDirectory(CONTACTS)
const member = (k: 'hoa' | 'yen' | 'lan') => ({ user_id: U[k].id, username: k, ngac_node_id: U[k].node })

function roster(canRemove: boolean | undefined) {
  vi.mocked(messagingApi.listMembers).mockResolvedValue({
    members: [member('hoa'), member('yen'), member('lan')],
    ...(canRemove === undefined ? {} : { can_remove_members: canRemove }),
  } as never)
}

function panel(kind: 'dm' | 'space' = 'space') {
  return (
    <MembersPanel
      channelId={CH.doisoat}
      kind={kind}
      spaceName="Đối soát giao dịch"
      people={people}
      me={U.hoa.id}
      onClose={() => {}}
    />
  )
}

async function openMenuOf(name: string) {
  await userEvent.click(await screen.findByRole('button', { name: `Tuỳ chọn cho ${name}` }))
}

beforeEach(() => {
  navigate.mockReset()
  vi.mocked(messagingApi.listMembers).mockReset()
  vi.mocked(messagingApi.removeMember).mockReset()
  vi.mocked(messagingApi.removeMember).mockResolvedValue({ status: 'ok' })
})
afterEach(resetClient)

describe('MembersPanel remove action', () => {
  it('is offered to someone who may remove people', async () => {
    roster(true)
    renderWithClient(panel())
    await openMenuOf('Phạm Hải Yến')
    expect(await screen.findByRole('menuitem', { name: 'Xoá khỏi nhóm' })).toBeInTheDocument()
  })

  it('is not offered to a plain member, who can still message the person', async () => {
    roster(false)
    renderWithClient(panel())
    await openMenuOf('Phạm Hải Yến')
    expect(await screen.findByRole('menuitem', { name: 'Nhắn tin trực tiếp' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: 'Xoá khỏi nhóm' })).toBeNull()
  })

  it('is not offered when the server does not say (the workspace channel, an old server)', async () => {
    roster(undefined)
    renderWithClient(panel())
    await openMenuOf('Phạm Hải Yến')
    await screen.findByRole('menuitem', { name: 'Nhắn tin trực tiếp' })
    expect(screen.queryByRole('menuitem', { name: 'Xoá khỏi nhóm' })).toBeNull()
  })

  it('is never offered in a direct message, whatever the flag says', async () => {
    roster(true)
    renderWithClient(panel('dm'))
    await openMenuOf('Phạm Hải Yến')
    await screen.findByRole('menuitem', { name: 'Nhắn tin trực tiếp' })
    expect(screen.queryByRole('menuitem', { name: 'Xoá khỏi nhóm' })).toBeNull()
  })

  it('stays offered for the others after one person is removed', async () => {
    roster(true)
    renderWithClient(panel())
    // The refetch after the removal never answers, so what is on screen is the
    // optimistic list the removal wrote into the cache.
    vi.mocked(messagingApi.listMembers).mockImplementation(() => new Promise(() => {}))
    await openMenuOf('Phạm Hải Yến')
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Xoá khỏi nhóm' }))
    await waitFor(() => expect(messagingApi.removeMember).toHaveBeenCalledWith(CH.doisoat, U.yen.node))
    await waitFor(() => expect(screen.queryByText('Phạm Hải Yến')).toBeNull())
    await openMenuOf(CONTACTS.find((c) => c.user_id === U.lan.id)!.display_name)
    expect(await screen.findByRole('menuitem', { name: 'Xoá khỏi nhóm' })).toBeInTheDocument()
  })
})

describe('MembersPanel: pressing a member', () => {
  const HIDDEN = '11111111-aaaa-4bbb-8ccc-0000000000ff'
  const hiddenPeople = buildDirectory(CONTACTS, {
    resolved: {
      byUserId: new Map([[HIDDEN, { userId: HIDDEN, nodeId: 'n-hidden', name: 'Nguyễn Hữu Thành', avatarUrl: '' }]]),
      byNodeId: new Map(),
    },
  })
  const withHidden = () => {
    vi.mocked(messagingApi.listMembers).mockResolvedValue({
      members: [member('hoa'), member('yen'), { user_id: HIDDEN, username: 'thanh', ngac_node_id: 'n-hidden' }],
      can_remove_members: true,
    } as never)
  }

  it('a hidden member keeps their name and opens the small card, which has no actions', async () => {
    withHidden()
    const { container } = renderWithClient(<MembersPanel channelId={CH.doisoat} kind="space" spaceName="Đối soát" people={hiddenPeople} me={U.hoa.id} onClose={() => {}} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Nguyễn Hữu Thành' }))
    const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    expect(within(card).getByText('Hồ sơ này không còn hiển thị với bạn.')).toBeInTheDocument()
    expect(within(card).queryAllByRole('button')).toHaveLength(0)
    expect(navigate).not.toHaveBeenCalled()
    expect(container.textContent).not.toMatch(UUID_RE)
  })

  it('a visible member opens their profile in Danh bạ', async () => {
    roster(true)
    renderWithClient(panel())
    await userEvent.click(await screen.findByRole('button', { name: /^Phạm Hải Yến/ }))
    expect(navigate.mock.calls[0]![0]).toMatchObject({ to: '/contacts' })
  })

  it('keeps the remove action in its own menu, outside the person button', async () => {
    roster(true)
    renderWithClient(panel())
    const link = await screen.findByRole('button', { name: /^Phạm Hải Yến/ })
    expect(within(link).queryByRole('button')).toBeNull()
    await openMenuOf('Phạm Hải Yến')
    expect(await screen.findByRole('menuitem', { name: 'Xoá khỏi nhóm' })).toBeInTheDocument()
  })

  it('the person button is named by the name and the role on screen, not by a label of its own', async () => {
    roster(true)
    renderWithClient(panel())
    const link = await screen.findByRole('button', { name: /^Phạm Hải Yến/ })
    expect(link).not.toHaveAttribute('aria-label')
    expect(link).toHaveAttribute('aria-labelledby')
    expect(link).toHaveAccessibleName('Phạm Hải Yến Chuyên viên · Vận hành thanh toán')
  })
})
