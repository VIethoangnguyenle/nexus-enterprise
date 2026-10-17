import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi } from '../../api/messaging'
import { useCanManageChannel } from '../../hooks/useCanManageChannel'
import { EMPTY_DIRECTORY } from '../../lib/people'
import { queryClient } from '../../lib/query-client'
import { CH } from '../../test/chat-fixtures'
import { renderWithClient } from '../../test/render'
import { SpaceInfoPanel } from './SpaceInfoPanel'

vi.mock('../../api/messaging', async (orig) => {
  const real = await orig<typeof import('../../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, listPins: vi.fn() } }
})
vi.mock('../../hooks/useCanManageChannel', () => ({ useCanManageChannel: vi.fn() }))

const panel = () => (
  <SpaceInfoPanel
    channelId={CH.doisoat}
    kind="space"
    title="Đối soát giao dịch"
    memberCount={14}
    people={EMPTY_DIRECTORY}
    onOpenTopic={() => {}}
    onClose={() => {}}
  />
)

beforeEach(() => {
  queryClient.clear()
  vi.clearAllMocks()
  vi.mocked(messagingApi.listPins).mockResolvedValue({ pins: [] } as never)
})

describe('SpaceInfoPanel rename', () => {
  it('offers the rename to someone who manages the group', () => {
    vi.mocked(useCanManageChannel).mockReturnValue(true)
    renderWithClient(panel())
    expect(screen.getByRole('button', { name: 'Đổi tên' })).toBeInTheDocument()
  })

  it('does not offer the rename to a member who cannot manage it', () => {
    vi.mocked(useCanManageChannel).mockReturnValue(false)
    renderWithClient(panel())
    expect(screen.queryByRole('button', { name: 'Đổi tên' })).toBeNull()
    expect(screen.getByText('Tên nhóm')).toBeInTheDocument()
  })
})
