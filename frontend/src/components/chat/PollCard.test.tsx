import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import { pollApi } from '../../api/chat-cards'
import { toast } from '../primitives'
import { PollCard } from './PollCard'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, U } from '../../test/chat-fixtures'
import { OPT, POLL_ID, pollUnvoted, pollVoted } from '../../test/chat-cards-fixtures'

vi.mock('../../api/chat-cards', () => ({
  pollApi: { get: vi.fn(), vote: vi.fn(), unvote: vi.fn(), close: vi.fn(), create: vi.fn() },
}))
vi.mock('../../hooks/usePeople', () => ({ usePeople: () => ({ ...buildDirectory(CONTACTS), isLoading: false }) }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'ws', workspaces: [], isLoading: false }) }))
vi.mock('../primitives', async (orig) => {
  const real = await orig<typeof import('../primitives')>()
  return { ...real, toast: Object.assign(vi.fn(), { error: vi.fn() }) }
})

beforeEach(() => vi.clearAllMocks())
afterEach(resetClient)

describe('PollCard', () => {
  it('shows the question and choices but no counts or voters before the viewer votes', async () => {
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted())
    renderWithClient(<PollCard pollId={POLL_ID} />)
    expect(await screen.findByText('Chốt lịch họp đối soát với ngân hàng?')).toBeInTheDocument()
    expect(screen.getAllByRole('radio')).toHaveLength(3)
    expect(screen.queryByTestId('poll-count')).toBeNull()
    expect(screen.queryByText(/người đã bình chọn/)).toBeNull()
    expect(screen.queryByRole('button', { name: 'Rút phiếu' })).toBeNull()
    expectNoIds()
  })

  it('votes with one tap and then shows counts and voters from the refreshed poll', async () => {
    const user = userEvent.setup()
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted())
    vi.mocked(pollApi.vote).mockResolvedValue(pollVoted([OPT.a]))
    renderWithClient(<PollCard pollId={POLL_ID} />)
    await user.click(await screen.findByRole('radio', { name: /Thứ Ba 14\/10/ }))
    expect(pollApi.vote).toHaveBeenCalledWith(POLL_ID, OPT.a)
    await waitFor(() => expect(screen.getAllByTestId('poll-count').map((n) => n.textContent)).toEqual(['2', '1', '0']))
    expect(screen.getByRole('radio', { name: /Thứ Ba 14\/10/ })).toBeChecked()
    expect(screen.getByText('3 người đã bình chọn')).toBeInTheDocument()
    // Voters show as avatars of the people (initials), never ids.
    expect(screen.getByRole('radio', { name: /Thứ Ba 14\/10/ })).toHaveTextContent(/TH.*TL/)
    expectNoIds()
  })

  it('lets a multiple-choice poll take two answers and withdraws one on a second tap', async () => {
    const user = userEvent.setup()
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted({ is_multi: true }))
    vi.mocked(pollApi.vote)
      .mockResolvedValueOnce(pollVoted([OPT.a], { is_multi: true }))
      .mockResolvedValueOnce(pollVoted([OPT.a, OPT.b], { is_multi: true }))
    vi.mocked(pollApi.unvote).mockResolvedValue(pollVoted([OPT.b], { is_multi: true }))
    renderWithClient(<PollCard pollId={POLL_ID} />)
    await user.click(await screen.findByRole('checkbox', { name: /Thứ Ba/ }))
    await waitFor(() => expect(screen.getByRole('checkbox', { name: /Thứ Ba/ })).toBeChecked())
    await user.click(screen.getByRole('checkbox', { name: /Thứ Tư/ }))
    await waitFor(() => expect(screen.getByRole('checkbox', { name: /Thứ Tư/ })).toBeChecked())
    expect(screen.getByRole('checkbox', { name: /Thứ Ba/ })).toBeChecked()
    await user.click(screen.getByRole('checkbox', { name: /Thứ Ba/ }))
    expect(pollApi.unvote).toHaveBeenCalledWith(POLL_ID, OPT.a)
  })

  it('withdraws every own vote with "Rút phiếu"', async () => {
    const user = userEvent.setup()
    vi.mocked(pollApi.get).mockResolvedValue(pollVoted([OPT.a]))
    vi.mocked(pollApi.unvote).mockResolvedValue(pollUnvoted())
    renderWithClient(<PollCard pollId={POLL_ID} />)
    await user.click(await screen.findByRole('button', { name: 'Rút phiếu' }))
    expect(pollApi.unvote).toHaveBeenCalledWith(POLL_ID, OPT.a)
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Rút phiếu' })).toBeNull())
  })

  it('never names the voters of an anonymous poll', async () => {
    vi.mocked(pollApi.get).mockResolvedValue(pollVoted([OPT.a], { is_anonymous: true }))
    renderWithClient(<PollCard pollId={POLL_ID} />)
    await screen.findByText('3 người đã bình chọn')
    expect(screen.getAllByTestId('poll-count')).toHaveLength(3)
    expect(screen.queryByLabelText(/Lê Thị Hoa|Nguyễn Thu Lan/)).toBeNull()
    expect(screen.queryByTestId('poll-voters')).toBeNull()
  })

  it('shows results and no voting controls once closed, and marks the leading choice', async () => {
    vi.mocked(pollApi.get).mockResolvedValue(pollVoted([], { closed: true, closed_at: '2026-10-11T10:00:00Z' }))
    renderWithClient(<PollCard pollId={POLL_ID} />)
    await screen.findByText(/Đã đóng lúc/)
    expect(screen.queryByRole('radio')).toBeNull()
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Rút phiếu' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Đóng bình chọn' })).toBeNull()
    expect(screen.getAllByTestId('poll-count')).toHaveLength(3)
    expect(screen.getAllByText('Nhiều nhất')).toHaveLength(1)
  })

  it('treats a poll past its end time as closed even before the server says so', async () => {
    vi.mocked(pollApi.get).mockResolvedValue(pollVoted([], { ends_at: '2020-01-01T00:00:00Z', closed: false }))
    renderWithClient(<PollCard pollId={POLL_ID} />)
    await screen.findByText('3 người đã bình chọn')
    expect(screen.queryByRole('radio')).toBeNull()
  })

  it('offers "Đóng bình chọn" to the creator only', async () => {
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted({ created_by: U.hoa.id }))
    const mine = renderWithClient(<PollCard pollId={POLL_ID} />)
    expect(await screen.findByRole('button', { name: 'Đóng bình chọn' })).toBeInTheDocument()
    mine.unmount()
    resetClient()
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted({ created_by: U.duc.id }))
    renderWithClient(<PollCard pollId={POLL_ID} />)
    await screen.findByText('Chốt lịch họp đối soát với ngân hàng?')
    expect(screen.queryByRole('button', { name: 'Đóng bình chọn' })).toBeNull()
  })

  it('closes the poll and shows its results', async () => {
    const user = userEvent.setup()
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted({ created_by: U.hoa.id }))
    vi.mocked(pollApi.close).mockResolvedValue(pollVoted([], { closed: true, created_by: U.hoa.id }))
    renderWithClient(<PollCard pollId={POLL_ID} />)
    await user.click(await screen.findByRole('button', { name: 'Đóng bình chọn' }))
    await waitFor(() => expect(screen.getAllByTestId('poll-count')).toHaveLength(3))
    expect(screen.queryByRole('radio')).toBeNull()
  })

  it('says so when closing is refused, and leaves the poll open', async () => {
    const user = userEvent.setup()
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted({ created_by: U.hoa.id }))
    vi.mocked(pollApi.close).mockRejectedValue(new ApiError('forbidden', 403))
    renderWithClient(<PollCard pollId={POLL_ID} />)
    await user.click(await screen.findByRole('button', { name: 'Đóng bình chọn' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Bạn không đóng được bình chọn này'))
    expect(screen.getAllByRole('radio')).toHaveLength(3)
  })

  it('shows a plain line, with no details, when the viewer cannot read the poll', async () => {
    vi.mocked(pollApi.get).mockRejectedValue(new ApiError('forbidden', 403))
    renderWithClient(<PollCard pollId={POLL_ID} />)
    const note = await screen.findByText('Bình chọn không xem được')
    expect(within(note.closest('[data-card]') as HTMLElement).queryByRole('radio')).toBeNull()
  })

  it('keeps every choice a 44px target', async () => {
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted())
    renderWithClient(<PollCard pollId={POLL_ID} />)
    for (const r of await screen.findAllByRole('radio')) expect(r.className).toContain('min-h-11')
  })
})
