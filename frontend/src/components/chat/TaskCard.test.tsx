import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import { messagingApi } from '../../api/messaging'
import { toast } from '../primitives'
import { TaskCard } from './TaskCard'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, U } from '../../test/chat-fixtures'
import { TASK_ID, task } from '../../test/chat-cards-fixtures'

vi.mock('../../api/messaging', async (orig) => {
  const real = await orig<typeof import('../../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, listTasks: vi.fn(), updateTask: vi.fn() } }
})
vi.mock('../../hooks/usePeople', () => ({ usePeople: () => ({ ...buildDirectory(CONTACTS), isLoading: false }) }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'ws', workspaces: [], isLoading: false }) }))
vi.mock('../primitives', async (orig) => {
  const real = await orig<typeof import('../primitives')>()
  return { ...real, toast: Object.assign(vi.fn(), { error: vi.fn() }) }
})

const CH = '33333333-aaaa-4bbb-8ccc-000000000001'
beforeEach(() => vi.clearAllMocks())
afterEach(resetClient)

describe('TaskCard', () => {
  it('shows the title, the assignee by name, the due date and the status in words', async () => {
    vi.mocked(messagingApi.listTasks).mockResolvedValue({ tasks: [task({ assignee_name: '' })] })
    renderWithClient(<TaskCard taskId={TASK_ID} channelId={CH} />)
    expect(await screen.findByText('Gửi danh sách 3 giao dịch lệch cho ngân hàng')).toBeInTheDocument()
    expect(screen.getByText('Nguyễn Thu Lan')).toBeInTheDocument()
    expect(screen.getByText('Chưa làm')).toBeInTheDocument()
    expect(screen.getByText(/Hạn/)).toHaveTextContent('15/10')
    expectNoIds()
  })

  it('marks the task done for its creator and shows the new state', async () => {
    const user = userEvent.setup()
    vi.mocked(messagingApi.listTasks)
      .mockResolvedValueOnce({ tasks: [task({ created_by: U.hoa.id })] })
      .mockResolvedValue({ tasks: [task({ created_by: U.hoa.id, status: 'done' })] })
    vi.mocked(messagingApi.updateTask).mockResolvedValue(task({ status: 'done' }))
    renderWithClient(<TaskCard taskId={TASK_ID} channelId={CH} />)
    await user.click(await screen.findByRole('checkbox', { name: /Đánh dấu xong/ }))
    expect(messagingApi.updateTask).toHaveBeenCalledWith(TASK_ID, { status: 'done' })
    await waitFor(() => expect(screen.getByRole('checkbox', { name: /Mở lại/ })).toBeChecked())
    expect(screen.getByText('Đã xong')).toBeInTheDocument()
  })

  it('reopens a finished task for its assignee', async () => {
    const user = userEvent.setup()
    vi.mocked(messagingApi.listTasks).mockResolvedValue({ tasks: [task({ assignee_id: U.hoa.id, status: 'done' })] })
    vi.mocked(messagingApi.updateTask).mockResolvedValue(task({ status: 'open' }))
    renderWithClient(<TaskCard taskId={TASK_ID} channelId={CH} />)
    await user.click(await screen.findByRole('checkbox', { name: /Mở lại/ }))
    expect(messagingApi.updateTask).toHaveBeenCalledWith(TASK_ID, { status: 'open' })
  })

  it('on a 403 says so and leaves the task as it was', async () => {
    const user = userEvent.setup()
    vi.mocked(messagingApi.listTasks).mockResolvedValue({ tasks: [task({ created_by: U.hoa.id })] })
    vi.mocked(messagingApi.updateTask).mockRejectedValue(new ApiError('forbidden', 403))
    renderWithClient(<TaskCard taskId={TASK_ID} channelId={CH} />)
    await user.click(await screen.findByRole('checkbox', { name: /Đánh dấu xong/ }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Bạn không đổi được trạng thái công việc này'))
    await waitFor(() => expect(screen.getByRole('checkbox', { name: /Đánh dấu xong/ })).not.toBeChecked())
    expect(screen.getByText('Chưa làm')).toBeInTheDocument()
  })

  it('gives the check to the assignee and the creator only', async () => {
    vi.mocked(messagingApi.listTasks).mockResolvedValue({ tasks: [task({ created_by: U.duc.id, assignee_id: U.lan.id })] })
    renderWithClient(<TaskCard taskId={TASK_ID} channelId={CH} />)
    await screen.findByText('Nguyễn Thu Lan')
    expect(screen.queryByRole('checkbox')).toBeNull()
  })

  it('names a task the viewer cannot find without leaking a title', async () => {
    vi.mocked(messagingApi.listTasks).mockResolvedValue({ tasks: [] })
    renderWithClient(<TaskCard taskId={TASK_ID} channelId={CH} />)
    expect(await screen.findByText('Công việc không xem được')).toBeInTheDocument()
  })

  it('offers "Mở trong Công việc" only when the caller says where it goes', async () => {
    const user = userEvent.setup()
    const onOpenTasks = vi.fn()
    vi.mocked(messagingApi.listTasks).mockResolvedValue({ tasks: [task()] })
    renderWithClient(<TaskCard taskId={TASK_ID} channelId={CH} onOpenTasks={onOpenTasks} />)
    await user.click(await screen.findByRole('button', { name: 'Mở trong Công việc' }))
    expect(onOpenTasks).toHaveBeenCalled()
  })

  it('makes the check a 44px target', async () => {
    vi.mocked(messagingApi.listTasks).mockResolvedValue({ tasks: [task({ created_by: U.hoa.id })] })
    renderWithClient(<TaskCard taskId={TASK_ID} channelId={CH} />)
    expect((await screen.findByRole('checkbox')).className).toContain('min-h-11')
  })
})
