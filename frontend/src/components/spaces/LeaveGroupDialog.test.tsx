import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { messagingApi } from '../../api/messaging'
import { useAuthStore } from '../../stores/auth.store'
import { LeaveGroupDialog } from './LeaveGroupDialog'

const WS_ID = 'ws-test-1'
const CH_ID = 'ch-leave-1'
const NODE_ID = 'node-me-1'

let queryClient: QueryClient

function Wrapper({ children }: { children: React.ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}

// Mock navigate
const mockNavigate = vi.fn()
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mockNavigate,
}))

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  useAuthStore.setState({ user: { id: 'me', username: 'hoa', ngac_node_id: NODE_ID } })
  mockNavigate.mockReset()
})

afterEach(() => {
  queryClient.clear()
  vi.restoreAllMocks()
})

describe('LeaveGroupDialog', () => {
  it('shows the confirm dialog with the group name in the title', () => {
    render(
      <LeaveGroupDialog open channelId={CH_ID} workspaceId={WS_ID} groupName="Đối soát Q4" onClose={vi.fn()} />,
      { wrapper: Wrapper },
    )
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText(/Rời \u201CĐối soát Q4\u201D\?/)).toBeInTheDocument()
    expect(screen.getByText(/Bạn sẽ không còn thấy tin nhắn và tệp/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Rời nhóm/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Huỷ/ })).toBeInTheDocument()
  })

  it('shows last-member warning when memberCount is 1', () => {
    render(
      <LeaveGroupDialog open channelId={CH_ID} workspaceId={WS_ID} groupName="Đối soát Q4" memberCount={1} onClose={vi.fn()} />,
      { wrapper: Wrapper },
    )
    expect(screen.getByText(/Bạn là người cuối cùng trong nhóm/)).toBeInTheDocument()
  })

  it('does not show last-member warning when memberCount is not loaded (undefined)', () => {
    render(
      <LeaveGroupDialog open channelId={CH_ID} workspaceId={WS_ID} groupName="Đối soát Q4" onClose={vi.fn()} />,
      { wrapper: Wrapper },
    )
    expect(screen.queryByText(/Bạn là người cuối cùng/)).toBeNull()
  })

  it('calls the API with the caller own node id on confirm, then navigates', async () => {
    const user = userEvent.setup()
    const removeSpy = vi.spyOn(messagingApi, 'removeMember').mockResolvedValueOnce({ status: 'ok' })
    const onClose = vi.fn()

    render(
      <LeaveGroupDialog open channelId={CH_ID} workspaceId={WS_ID} groupName="Đối soát Q4" onClose={onClose} />,
      { wrapper: Wrapper },
    )

    await user.click(screen.getByRole('button', { name: /Rời nhóm/ }))
    await waitFor(() => expect(removeSpy).toHaveBeenCalledWith(CH_ID, NODE_ID))
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith({ to: '/channels' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('on error keeps the dialog open and shows the alert message', async () => {
    const user = userEvent.setup()
    vi.spyOn(messagingApi, 'removeMember').mockRejectedValueOnce(new Error('server error'))
    const onClose = vi.fn()

    render(
      <LeaveGroupDialog open channelId={CH_ID} workspaceId={WS_ID} groupName="Đối soát Q4" onClose={onClose} />,
      { wrapper: Wrapper },
    )

    await user.click(screen.getByRole('button', { name: /Rời nhóm/ }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Không rời được nhóm. Thử lại.'))
    // Dialog stays open, onClose not called
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('does not render dialog when open is false', () => {
    render(
      <LeaveGroupDialog open={false} channelId={CH_ID} workspaceId={WS_ID} groupName="Test" onClose={vi.fn()} />,
      { wrapper: Wrapper },
    )
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
