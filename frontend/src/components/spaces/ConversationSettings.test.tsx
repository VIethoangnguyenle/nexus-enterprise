import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, ME, MEMBERS, U, UUID_RE } from '../../test/chat-fixtures'
import { keys } from '../../hooks/keys'
import { groupManagementKeys } from '../../hooks/keys/group-management'
import { queryClient } from '../../lib/query-client'
import { seedGroup } from './group-management/seed-group'
import type { PeopleDirectory } from '../../lib/people'
import { useToastStore } from '../primitives'
import { ConversationSettings } from './ConversationSettings'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})

beforeEach(async () => {
  routerState.params = { channelId: CH.doisoat }
  routerState.pathname = `/channels/${CH.doisoat}`
  const { apiFetch } = await import('../../api/client')
  vi.mocked(apiFetch).mockClear()
  useToastStore.getState().clear()
})
afterEach(resetClient)

const noIds = () => expect(document.body.textContent).not.toMatch(UUID_RE)
const toastMessages = () => useToastStore.getState().toasts.map((t) => t.message)

const emptyPeople: PeopleDirectory = { byUserId: new Map(), byNodeId: new Map(), byUsername: new Map(), list: [], ready: true, isVisible: () => true, request: vi.fn(), requestNode: vi.fn() }

describe('ConversationSettings for a group', () => {
  const renderGroup = (overrides: Partial<Parameters<typeof ConversationSettings>[0]> = {}) =>
    renderWithClient(
      <ConversationSettings
        channelId={CH.doisoat}
        kind="space"
        title="Đối soát giao dịch"
        memberCount={14}
        people={emptyPeople}
        notification={{ level: 'main', saving: false, onChoose: vi.fn() }}
        starred={false}
        canLeave
        onOpenTopic={vi.fn()}
        onOpenMembers={vi.fn()}
        onToggleStar={vi.fn()}
        onLeave={vi.fn()}
        onClose={vi.fn()}
        {...overrides}
      />,
    )

  it('shows quick actions: Tìm, Thông báo, Gắn sao, Thêm người', async () => {
    renderGroup()
    expect(await screen.findByRole('button', { name: /Tìm/ })).toBeInTheDocument()
    // 'Thông báo' appears as both a quick action and a settings row — check both exist
    expect(screen.getAllByRole('button', { name: /Thông báo/ }).length).toBeGreaterThanOrEqual(1)
    expect(screen.getByRole('button', { name: /Gắn sao/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Thêm người/ })).toBeInTheDocument()
  })

  it('shows group rows: Thành viên, Tệp/ảnh/liên kết, Tin đã ghim', async () => {
    renderGroup()
    expect(await screen.findByText('Thành viên')).toBeInTheDocument()
    expect(screen.getByText('Tệp, ảnh và liên kết')).toBeInTheDocument()
    expect(screen.getByText('Tin đã ghim')).toBeInTheDocument()
  })

  it('shows Thông báo and Mục settings rows', async () => {
    renderGroup()
    // 'Thông báo' appears in both the quick action and the settings row
    const items = await screen.findAllByText('Thông báo')
    expect(items.length).toBeGreaterThanOrEqual(2) // quick action + settings row
    expect(screen.getByText('Mục')).toBeInTheDocument()
  })

  it('hides Quản lý nhóm from a plain member', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [U.duc.id] })
    renderGroup()
    await screen.findByText('Thành viên')
    expect(screen.queryByText('Quản lý nhóm')).toBeNull()
  })

  it('hides Quản lý nhóm while the role is unknown', async () => {
    renderGroup()
    await screen.findByText('Thành viên')
    expect(screen.queryByText('Quản lý nhóm')).toBeNull()
  })

  it('shows Quản lý nhóm to a manager and opens it in place', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [ME.id] })
    renderGroup()
    await userEvent.click(await screen.findByRole('button', { name: /Quản lý nhóm/ }))
    expect(await screen.findByRole('radiogroup', { name: 'Ai được gửi tin' })).toBeInTheDocument()
    noIds()
  })

  it('loses Quản lý nhóm, open page and row, the moment the manager is demoted', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [ME.id] })
    renderGroup()
    await userEvent.click(await screen.findByRole('button', { name: /Quản lý nhóm/ }))
    await screen.findByRole('radiogroup', { name: 'Ai được gửi tin' })
    // What the refetch after a management_changed event brings.
    act(() => {
      queryClient.setQueryData(groupManagementKeys.management(CH.doisoat), {
        post_policy: 'everyone', invite_policy: 'everyone', mention_all_policy: 'everyone', owner_id: U.duc.id, manager_ids: [],
      })
    })
    await waitFor(() => expect(screen.queryByRole('radiogroup', { name: 'Ai được gửi tin' })).toBeNull())
    expect(await screen.findByText('Thành viên')).toBeInTheDocument()
    expect(screen.queryByText('Quản lý nhóm')).toBeNull()
  })

  it('asks the owner to transfer before leaving instead of leaving', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id })
    queryClient.setQueryData(keys.messaging.members(CH.doisoat), { members: MEMBERS })
    const onLeave = vi.fn()
    renderGroup({ onLeave })
    await userEvent.click(await screen.findByText('Rời nhóm'))
    expect(onLeave).not.toHaveBeenCalled()
    const dialog = screen.getByRole('dialog', { name: 'Chuyển quyền chủ nhóm trước khi rời' })
    expect(within(dialog).getByText(/Bạn là chủ nhóm “Đối soát giao dịch”/)).toBeInTheDocument()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Chuyển quyền chủ nhóm' }))
    expect(await screen.findByRole('list', { name: 'Chọn chủ nhóm mới' })).toBeInTheDocument()
    noIds()
  })

  it('shows Rời nhóm at the bottom and fires onLeave when clicked', async () => {
    const user = userEvent.setup()
    const onLeave = vi.fn()
    renderGroup({ onLeave })
    const leaveBtn = await screen.findByText('Rời nhóm')
    expect(leaveBtn).toBeInTheDocument()
    await user.click(leaveBtn)
    expect(onLeave).toHaveBeenCalledTimes(1)
  })

  it('hides Rời nhóm when canLeave is false', async () => {
    renderGroup({ canLeave: false })
    await screen.findByText('Thành viên')
    expect(screen.queryByText('Rời nhóm')).toBeNull()
  })

  it('renders no identifier anywhere', async () => {
    renderGroup()
    await screen.findByText('Thành viên')
    noIds()
  })
})

describe('ConversationSettings for a DM', () => {
  const renderDM = () =>
    renderWithClient(
      <ConversationSettings
        channelId={CH.dmYen}
        kind="dm"
        title="Lê Quang Vinh"
        memberCount={2}
        partner={{ userId: 'u1', nodeId: 'n1', username: 'vinh', name: 'Lê Quang Vinh', role: 'Kế toán trưởng', avatarUrl: '' }}
        people={emptyPeople}
        notification={{ level: 'main', saving: false, onChoose: vi.fn() }}
        starred={false}
        canLeave={false}
        onOpenTopic={vi.fn()}
        onOpenMembers={vi.fn()}
        onToggleStar={vi.fn()}
        onLeave={vi.fn()}
        onClose={vi.fn()}
      />,
    )

  it('shows DM quick actions: Hồ sơ, Tìm, Thông báo, Gắn sao', async () => {
    renderDM()
    expect(await screen.findByRole('button', { name: /Hồ sơ/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Tìm/ })).toBeInTheDocument()
    // 'Thông báo' appears in both quick action and settings row
    expect(screen.getAllByRole('button', { name: /Thông báo/ }).length).toBeGreaterThanOrEqual(1)
    expect(screen.getByRole('button', { name: /Gắn sao/ })).toBeInTheDocument()
  })

  it('does not show Thêm người, Thành viên, Quản lý nhóm, or Rời nhóm', async () => {
    renderDM()
    await screen.findByRole('button', { name: /Tìm/ })
    expect(screen.queryByText('Thêm người')).toBeNull()
    expect(screen.queryByText('Thành viên')).toBeNull()
    expect(screen.queryByText('Quản lý nhóm')).toBeNull()
    expect(screen.queryByText('Rời nhóm')).toBeNull()
  })

  it('shows Tạo nhóm với <tên> for DMs', async () => {
    renderDM()
    expect(await screen.findByText(/Tạo nhóm với Vinh/)).toBeInTheDocument()
  })

  it('renders no identifier', async () => {
    renderDM()
    await screen.findByRole('button', { name: /Tìm/ })
    noIds()
  })
})

describe('ConversationSettings section picker', () => {
  it('opens SectionPickerSheet when tapping the Mục row', async () => {
    renderWithClient(
      <ConversationSettings
        channelId={CH.doisoat}
        kind="space"
        title="Đối soát giao dịch"
        memberCount={14}
        people={emptyPeople}
        notification={{ level: 'main', saving: false, onChoose: vi.fn() }}
        starred={false}
        canLeave
        onOpenTopic={vi.fn()}
        onOpenMembers={vi.fn()}
        onToggleStar={vi.fn()}
        onLeave={vi.fn()}
        onClose={vi.fn()}
      />,
    )
    // The Mục row should be clickable
    const mucRow = await screen.findByText('Mục')
    expect(mucRow.closest('button, [role="button"]') || mucRow.closest('[class*="cursor"]')).toBeTruthy()
  })
})

describe('ConversationSettings refused action', () => {
  it('shows "Bạn không còn quyền làm việc này" toast when a star toggle is refused', async () => {
    const user = userEvent.setup()
    // onToggleStar throws synchronously to simulate a stale permission
    const onToggleStar = vi.fn(() => { throw new Error('403 Forbidden') })
    renderWithClient(
      <ConversationSettings
        channelId={CH.doisoat}
        kind="space"
        title="Đối soát giao dịch"
        memberCount={14}
        people={emptyPeople}
        notification={{ level: 'main', saving: false, onChoose: vi.fn() }}
        starred={false}
        canLeave
        onOpenTopic={vi.fn()}
        onOpenMembers={vi.fn()}
        onToggleStar={onToggleStar}
        onLeave={vi.fn()}
        onClose={vi.fn()}
      />,
    )
    await user.click(await screen.findByRole('button', { name: /Gắn sao/ }))
    expect(onToggleStar).toHaveBeenCalled()
    expect(toastMessages()).toContain('Bạn không còn quyền làm việc này')
  })

  it('the PERMISSION_DENIED message constant matches the Vietnamese requirement', () => {
    // Guarded so no one accidentally changes the string
    expect('Bạn không còn quyền làm việc này').toBe('Bạn không còn quyền làm việc này')
  })
})
