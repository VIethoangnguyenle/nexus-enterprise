import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import { MENTION_ALL } from '../../api/group-management'
import { keys } from '../../hooks/keys'
import { queryClient } from '../../lib/query-client'
import { useDraftsStore } from '../../stores/drafts.store'
import { CH, ME, MEMBERS, U } from '../../test/chat-fixtures'
import { renderWithClient, resetClient } from '../../test/render'
import { toast, useToastStore } from '../primitives'
import { seedGroup, seedWorkspaceAdmin } from '../spaces/group-management/seed-group'
import { ChatEditor } from './ChatEditor'

beforeEach(() => {
  localStorage.clear()
  useDraftsStore.getState().reset()
  useToastStore.getState().clear()
})
afterEach(resetClient)

const BOX = 'Tin nhắn cho Đối soát'

function open(props: Partial<Parameters<typeof ChatEditor>[0]> = {}) {
  queryClient.setQueryData(keys.messaging.members(CH.doisoat), { members: MEMBERS })
  const onSend = vi.fn()
  renderWithClient(<ChatEditor channelId={CH.doisoat} placeholder={BOX} onSend={onSend} {...props} />)
  return { onSend }
}

async function typeAt() {
  const user = userEvent.setup()
  await user.click(screen.getByRole('textbox', { name: BOX }))
  await user.keyboard('@')
  return user
}
const allOption = () => screen.queryByRole('option', { name: /@Tất cả/ })

describe('ChatEditor: restricted posting', () => {
  it('a plain member sees why the main box is closed, and is offered the threads', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, post_policy: 'managers' })
    const onReplyInThread = vi.fn()
    open({ onReplyInThread })
    expect(screen.getByRole('note')).toHaveTextContent('Chỉ chủ nhóm và quản trị nhóm được gửi tin mới')
    expect(screen.queryByRole('textbox', { name: BOX })).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: /Trả lời trong thread/ }))
    expect(onReplyInThread).toHaveBeenCalledTimes(1)
  })

  it('the same member still replies in a thread', () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, post_policy: 'managers' })
    open({ parentMessageId: 'root-1' })
    expect(screen.getByRole('textbox', { name: BOX })).toBeInTheDocument()
    expect(screen.queryByRole('note')).toBeNull()
  })

  it('a manager keeps the main box', () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [ME.id], post_policy: 'managers' })
    open()
    expect(screen.getByRole('textbox', { name: BOX })).toBeInTheDocument()
  })

  it('an open group keeps the main box for everyone', () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id })
    open()
    expect(screen.getByRole('textbox', { name: BOX })).toBeInTheDocument()
  })
})

describe('ChatEditor: @Tất cả', () => {
  it('is offered in an open group, and sends the mention-all marker', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id })
    const { onSend } = open()
    const user = await typeAt()
    await user.click(await screen.findByRole('option', { name: /@Tất cả/ }))
    await user.keyboard('{Enter}')
    expect(onSend).toHaveBeenCalledWith(expect.stringContaining('@Tất cả'), [MENTION_ALL])
  })

  it('is not offered to a plain member when the group keeps it for managers', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, mention_all_policy: 'managers' })
    open()
    await typeAt()
    await screen.findByRole('listbox', { name: 'Nhắc tới' })
    expect(allOption()).toBeNull()
  })

  it('is offered to a manager when the group keeps it for managers', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [ME.id], mention_all_policy: 'managers' })
    open()
    await typeAt()
    expect(await screen.findByRole('option', { name: /@Tất cả/ })).toBeInTheDocument()
  })

  it('is never offered in a direct message', async () => {
    seedGroup(CH.doisoat, 'dm')
    open()
    await typeAt()
    await screen.findByRole('listbox', { name: 'Nhắc tới' })
    expect(allOption()).toBeNull()
  })

  it('is not offered to a member in the workspace-wide channel', async () => {
    seedGroup(CH.doisoat, 'workspace')
    seedWorkspaceAdmin(false)
    open()
    await typeAt()
    await screen.findByRole('listbox', { name: 'Nhắc tới' })
    expect(allOption()).toBeNull()
  })

  it('is offered to an administrator in the workspace-wide channel', async () => {
    seedGroup(CH.doisoat, 'workspace')
    seedWorkspaceAdmin(true)
    open()
    await typeAt()
    expect(await screen.findByRole('option', { name: /@Tất cả/ })).toBeInTheDocument()
  })

  it('a refusal by the server reads "Bạn không còn quyền làm việc này", not the generic sentence', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id })
    // The conversation's own send handler toasts its generic sentence and rethrows, as SpaceView's does.
    const onSend = vi.fn(async () => {
      toast.error('Bạn chưa có quyền gửi tin nhắn. Nhờ quản trị viên hoặc quản lý nhóm cấp quyền.')
      throw new ApiError('forbidden', 403)
    })
    open({ onSend })
    const user = await typeAt()
    await user.click(await screen.findByRole('option', { name: /@Tất cả/ }))
    await user.keyboard('{Enter}')
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message)).toEqual(['Bạn không còn quyền làm việc này']))
  })
})
