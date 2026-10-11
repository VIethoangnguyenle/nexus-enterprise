import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi } from '../../api/messaging'
import { TaskDialog } from './TaskDialog'
import { renderWithClient, resetClient } from '../../test/render'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, U } from '../../test/chat-fixtures'

vi.mock('../../api/messaging', async (orig) => {
  const real = await orig<typeof import('../../api/messaging')>()
  return {
    ...real,
    messagingApi: {
      ...real.messagingApi,
      createTask: vi.fn(),
      listMembers: vi.fn(),
    },
  }
})
vi.mock('../../hooks/usePeople', () => ({ usePeople: () => ({ ...buildDirectory(CONTACTS), isLoading: false }) }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'ws', workspaces: [], isLoading: false }) }))

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(messagingApi.listMembers).mockResolvedValue({
    members: [
      { user_id: U.lan.id, username: 'lan', ngac_node_id: U.lan.node },
      { user_id: U.yen.id, username: 'yen', ngac_node_id: U.yen.node },
    ],
  } as never)
})
afterEach(resetClient)

describe('TaskDialog', () => {
  it('needs a title', async () => {
    const user = userEvent.setup()
    renderWithClient(<TaskDialog open onClose={vi.fn()} channelId="ch" />)
    await user.click(screen.getByRole('button', { name: 'Gửi công việc' }))
    expect(messagingApi.createTask).not.toHaveBeenCalled()
    expect(await screen.findByText('Nhập việc cần làm.')).toBeInTheDocument()
  })

  it('picks the assignee from the channel members by searching a name', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    vi.mocked(messagingApi.createTask).mockResolvedValue({} as never)
    renderWithClient(<TaskDialog open onClose={onClose} channelId="ch" />)
    await user.type(screen.getByLabelText('Việc cần làm'), 'Gửi danh sách')
    // Only members of the channel are offered: Trần Minh Đức is in the workspace but not here.
    await user.type(await screen.findByRole('combobox', { name: 'Giao cho' }), 'duc')
    expect(await screen.findByText(/Không tìm thấy ai/)).toBeInTheDocument()
    await user.clear(screen.getByRole('combobox', { name: 'Giao cho' }))
    await user.type(screen.getByRole('combobox', { name: 'Giao cho' }), 'lan')
    await user.click(await screen.findByRole('option', { name: /Nguyễn Thu Lan/ }))
    await user.type(screen.getByLabelText(/^Hạn/), '2026-10-15')
    await user.click(screen.getByRole('button', { name: 'Gửi công việc' }))
    await waitFor(() => expect(messagingApi.createTask).toHaveBeenCalledWith('ch', {
      title: 'Gửi danh sách', assignee_id: U.lan.id, due_date: '2026-10-15',
    }))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('sends a task with no assignee and no date', async () => {
    const user = userEvent.setup()
    vi.mocked(messagingApi.createTask).mockResolvedValue({} as never)
    renderWithClient(<TaskDialog open onClose={vi.fn()} channelId="ch" />)
    await user.type(screen.getByLabelText('Việc cần làm'), 'Gọi ngân hàng')
    await user.click(screen.getByRole('button', { name: 'Gửi công việc' }))
    await waitFor(() => expect(messagingApi.createTask).toHaveBeenCalledWith('ch', { title: 'Gọi ngân hàng' }))
  })

  it('creates the task as a reply when it is made inside a thread', async () => {
    const user = userEvent.setup()
    vi.mocked(messagingApi.createTask).mockResolvedValue({} as never)
    renderWithClient(<TaskDialog open onClose={vi.fn()} channelId="ch" parentMessageId="root-1" />)
    await user.type(screen.getByLabelText('Việc cần làm'), 'Gọi ngân hàng')
    await user.click(screen.getByRole('button', { name: 'Gửi công việc' }))
    await waitFor(() => expect(messagingApi.createTask).toHaveBeenCalledWith('ch', { title: 'Gọi ngân hàng', parent_message_id: 'root-1' }))
  })
})
