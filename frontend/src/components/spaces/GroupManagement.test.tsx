import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import { groupManagementApi } from '../../api/group-management'
import { keys } from '../../hooks/keys'
import { groupManagementKeys } from '../../hooks/keys/group-management'
import { queryClient } from '../../lib/query-client'
import { buildDirectory } from '../../lib/people'
import { CH, CONTACTS, MEMBERS, ME, U, UUID_RE } from '../../test/chat-fixtures'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { useToastStore } from '../primitives'
import { GroupManagement } from './GroupManagement'
import { seedGroup } from './group-management/seed-group'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())

const people = buildDirectory(CONTACTS)
const toasts = () => useToastStore.getState().toasts.map((t) => t.message)
const noIds = () => expect(document.body.textContent).not.toMatch(UUID_RE)

function open(props: Partial<Parameters<typeof GroupManagement>[0]> = {}) {
  const onBack = vi.fn()
  const onClose = vi.fn()
  renderWithClient(
    <GroupManagement channelId={CH.doisoat} groupName="Đối soát giao dịch" people={people} onBack={onBack} onClose={onClose} {...props} />,
  )
  return { onBack, onClose }
}

const ok = { status: 'ok' }

beforeEach(() => {
  useToastStore.getState().clear()
  routerState.navigate.mockReset()
  queryClient.setQueryData(keys.messaging.members(CH.doisoat), { members: MEMBERS })
  vi.spyOn(groupManagementApi, 'get').mockImplementation(async () =>
    queryClient.getQueryData(groupManagementKeys.management(CH.doisoat))!,
  )
})
afterEach(() => {
  resetClient()
  vi.restoreAllMocks()
})

describe('GroupManagement: the three rules', () => {
  it('shows each rule with its current choice and the thread hint, and no ids', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id, invite_policy: 'managers' })
    open()
    const post = await screen.findByRole('radiogroup', { name: 'Ai được gửi tin' })
    expect(within(post).getByRole('radio', { name: /Mọi thành viên/ })).toHaveAttribute('aria-checked', 'true')
    expect(within(post).getByText('Thành viên vẫn trả lời trong thread')).toBeInTheDocument()
    const invite = screen.getByRole('radiogroup', { name: 'Ai được thêm người' })
    expect(within(invite).getByRole('radio', { name: /Chỉ chủ nhóm và quản trị nhóm/ })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radiogroup', { name: 'Ai được nhắc @Tất cả' })).toBeInTheDocument()
    noIds()
  })

  it('sends all three rules when one changes', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id })
    const put = vi.spyOn(groupManagementApi, 'updateSettings').mockResolvedValue(ok)
    open()
    const post = await screen.findByRole('radiogroup', { name: 'Ai được gửi tin' })
    await userEvent.click(within(post).getByRole('radio', { name: /Chỉ chủ nhóm/ }))
    expect(put).toHaveBeenCalledWith(CH.doisoat, { post_policy: 'managers', invite_policy: 'everyone', mention_all_policy: 'everyone' })
    expect(within(post).getByRole('radio', { name: /Chỉ chủ nhóm/ })).toHaveAttribute('aria-checked', 'true')
  })

  it('returns to the old choice with "Bạn không còn quyền làm việc này" when the server refuses', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [ME.id] })
    vi.spyOn(groupManagementApi, 'updateSettings').mockRejectedValue(new ApiError('forbidden', 403))
    open()
    const mention = await screen.findByRole('radiogroup', { name: 'Ai được nhắc @Tất cả' })
    await userEvent.click(within(mention).getByRole('radio', { name: /Chỉ chủ nhóm/ }))
    await waitFor(() => expect(toasts()).toContain('Bạn không còn quyền làm việc này'))
    expect(within(mention).getByRole('radio', { name: /Mọi thành viên/ })).toHaveAttribute('aria-checked', 'true')
  })
})

describe('GroupManagement: managers', () => {
  it('lists the owner and the managers by name and role', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [U.duc.id, U.lan.id] })
    open()
    expect(await screen.findByText('Trần Minh Đức')).toBeInTheDocument()
    expect(screen.getByText('Chủ nhóm')).toBeInTheDocument()
    expect(screen.getByText('Nguyễn Thu Lan')).toBeInTheDocument()
    expect(screen.getByText('Quản trị nhóm')).toBeInTheDocument()
    noIds()
  })

  it('adds a manager picked from the members, never by typing an id', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id, manager_ids: [ME.id] })
    const appoint = vi.spyOn(groupManagementApi, 'appointManager').mockResolvedValue(ok)
    open()
    await userEvent.click(await screen.findByRole('button', { name: 'Thêm quản trị nhóm' }))
    const list = screen.getByRole('list', { name: 'Chọn quản trị nhóm' })
    // The owner is not offered; everyone else is.
    expect(within(list).queryByText('Lê Thị Hoa')).toBeNull()
    await userEvent.click(within(list).getByRole('button', { name: /Phạm Hải Yến/ }))
    expect(appoint).toHaveBeenCalledWith(CH.doisoat, U.yen.id)
    await waitFor(() => expect(toasts()).toContain('Đã đặt Phạm Hải Yến làm quản trị nhóm'))
    noIds()
  })

  it('removes a manager', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id, manager_ids: [ME.id, U.lan.id] })
    const remove = vi.spyOn(groupManagementApi, 'removeManager').mockResolvedValue(ok)
    open()
    await userEvent.click(await screen.findByRole('button', { name: 'Gỡ quyền quản trị nhóm của Nguyễn Thu Lan' }))
    expect(remove).toHaveBeenCalledWith(CH.doisoat, U.lan.id)
  })

  it('tells a manager who lost the right while adding', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [ME.id] })
    vi.spyOn(groupManagementApi, 'appointManager').mockRejectedValue(new ApiError('forbidden', 403))
    open()
    await userEvent.click(await screen.findByRole('button', { name: 'Thêm quản trị nhóm' }))
    await userEvent.click(within(screen.getByRole('list', { name: 'Chọn quản trị nhóm' })).getByRole('button', { name: /Lê Quang Vinh/ }))
    await waitFor(() => expect(toasts()).toContain('Bạn không còn quyền làm việc này'))
  })
})

describe('GroupManagement: owner-only actions', () => {
  it('are hidden from a manager who is not the owner', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [ME.id] })
    open()
    await screen.findByRole('radiogroup', { name: 'Ai được gửi tin' })
    expect(screen.queryByRole('button', { name: /Chuyển quyền chủ nhóm/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /Giải tán nhóm/ })).toBeNull()
  })

  it('transfers ownership to a picked member after confirming', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id })
    const transfer = vi.spyOn(groupManagementApi, 'transferOwnership').mockResolvedValue(ok)
    open()
    await userEvent.click(await screen.findByRole('button', { name: /Chuyển quyền chủ nhóm/ }))
    await userEvent.click(within(screen.getByRole('list', { name: 'Chọn chủ nhóm mới' })).getByRole('button', { name: /Trần Minh Đức/ }))
    expect(transfer).not.toHaveBeenCalled()
    const dialog = screen.getByRole('dialog', { name: /Chuyển quyền chủ nhóm cho Trần Minh Đức/ })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Chuyển quyền' }))
    expect(transfer).toHaveBeenCalledWith(CH.doisoat, U.duc.id)
    await waitFor(() => expect(toasts()).toContain('Trần Minh Đức là chủ nhóm mới'))
    noIds()
  })

  it('cancelling the transfer sends nothing', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id })
    const transfer = vi.spyOn(groupManagementApi, 'transferOwnership').mockResolvedValue(ok)
    open()
    await userEvent.click(await screen.findByRole('button', { name: /Chuyển quyền chủ nhóm/ }))
    await userEvent.click(within(screen.getByRole('list', { name: 'Chọn chủ nhóm mới' })).getByRole('button', { name: /Trần Minh Đức/ }))
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Huỷ' }))
    expect(transfer).not.toHaveBeenCalled()
  })

  it('disbands after confirming and leaves for the chat list', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id })
    const disband = vi.spyOn(groupManagementApi, 'disband').mockResolvedValue(ok)
    const { onClose } = open()
    await userEvent.click(await screen.findByRole('button', { name: /Giải tán nhóm/ }))
    const dialog = screen.getByRole('dialog', { name: /Giải tán “Đối soát giao dịch”/ })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Giải tán nhóm' }))
    expect(disband).toHaveBeenCalledWith(CH.doisoat)
    await waitFor(() => expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('a refused disband keeps the group and says why', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id })
    vi.spyOn(groupManagementApi, 'disband').mockRejectedValue(new ApiError('forbidden', 403))
    open()
    await userEvent.click(await screen.findByRole('button', { name: /Giải tán nhóm/ }))
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Giải tán nhóm' }))
    await waitFor(() => expect(toasts()).toContain('Bạn không còn quyền làm việc này'))
    expect(routerState.navigate).not.toHaveBeenCalled()
  })
})
