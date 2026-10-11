import { screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { pollApi } from '../../api/chat-cards'
import type { Message } from '../../api/messaging'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, ME, U } from '../../test/chat-fixtures'
import { POLL_ID, pollUnvoted } from '../../test/chat-cards-fixtures'
import { MessageBlock } from './MessageBlock'

// The real cards, with only their data sources stubbed: what the stream shows is what a viewer sees.
vi.mock('../../api/chat-cards', () => ({
  pollApi: { get: vi.fn(), vote: vi.fn(), unvote: vi.fn(), close: vi.fn(), create: vi.fn() },
}))
vi.mock('../../hooks/usePeople', () => ({ usePeople: () => ({ ...buildDirectory(CONTACTS), isLoading: false }) }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'ws', workspaces: [], isLoading: false }) }))

beforeEach(() => vi.clearAllMocks())
afterEach(resetClient)

const people = buildDirectory(CONTACTS)
const pollMessage: Message = {
  id: 'm-poll', channel_id: 'ch-1', sender_id: U.duc.id, sender_name: 'duc', message_type: 'system',
  content: '📊 Poll: Chốt lịch họp đối soát với ngân hàng?', linked_entity_type: 'poll', linked_entity_id: POLL_ID,
  created_at: '2026-10-11T03:31:00Z',
}

describe('MessageBlock with the real poll card', () => {
  it('shows the question and choices under the creator, and no counts before the viewer votes', async () => {
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted())
    renderWithClient(<MessageBlock message={pollMessage} people={people} idBase="m-poll" me={ME.id} />)
    expect(await screen.findByText('Chốt lịch họp đối soát với ngân hàng?')).toBeInTheDocument()
    expect(screen.getByText('Trần Minh Đức')).toBeInTheDocument()
    expect(screen.getAllByRole('radio')).toHaveLength(3)
    expect(screen.queryByTestId('poll-count')).toBeNull()
    expect(screen.queryByText(/người đã bình chọn/)).toBeNull()
    // The fallback line of the message is not printed beside the card.
    expect(screen.queryByText(/Poll:/)).toBeNull()
    expectNoIds()
  })
})
