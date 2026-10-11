import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { buildDirectory } from '../../lib/people'
import { expectNoIds } from '../../test/no-ids'
import { CONTACTS, CH, MSG, MSG_QUIET, MESSAGES, QUIET_ROOT, U, fixtureApi } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { ThreadsPanel } from './ThreadsPanel'

vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})

const people = buildDirectory(CONTACTS)
const urls = () => vi.mocked(apiFetch).mock.calls.map(([p]) => String(p))

beforeEach(() => {
  vi.mocked(apiFetch).mockImplementation(fixtureApi as never)
  vi.mocked(apiFetch).mockClear()
})
afterEach(resetClient)

function panel(over: Partial<Parameters<typeof ThreadsPanel>[0]> = {}) {
  return (
    <ThreadsPanel
      channelId={CH.doisoat}
      spaceName="Đối soát giao dịch"
      people={people}
      onOpenThread={vi.fn()}
      onClose={vi.fn()}
      {...over}
    />
  )
}

const rows = () => within(screen.getByRole('list', { name: 'Thread' })).getAllByRole('button')

describe('ThreadsPanel', () => {
  it('lists the threads with who started them, how many replies and who replied, by name', async () => {
    renderWithClient(panel())
    await screen.findByRole('list', { name: 'Thread' })
    expect(rows()).toHaveLength(2)
    const first = rows()[0]!
    expect(first).toHaveTextContent('Trần Minh Đức')
    expect(first).toHaveTextContent(/lệch 3 giao dịch/)
    expect(first).toHaveTextContent('2 trả lời')
    expect(first).toHaveTextContent('1 mới')
    expect(within(first).getByRole('img', { name: 'Đang theo dõi' })).toBeInTheDocument()
    expect([...first.querySelectorAll('[title]')].map((e) => e.getAttribute('title'))).toContain('Lê Quang Vinh')
    expectNoIds()
  })

  it('shows "mới" only for threads the reader follows, and the bell likewise', async () => {
    renderWithClient(panel())
    await screen.findByRole('list', { name: 'Thread' })
    const quiet = rows()[1]!
    expect(quiet).toHaveTextContent('Phạm Hải Yến')
    expect(quiet).toHaveTextContent('1 trả lời')
    expect(quiet).not.toHaveTextContent('mới')
    expect(within(quiet).queryByRole('img', { name: 'Đang theo dõi' })).toBeNull()
  })

  it('filters on the server: Đang theo dõi and Chưa đọc', async () => {
    const user = userEvent.setup()
    renderWithClient(panel())
    await screen.findByRole('list', { name: 'Thread' })
    expect(screen.getByRole('radio', { name: 'Tất cả' })).toBeChecked()

    await user.click(screen.getByRole('radio', { name: 'Đang theo dõi' }))
    await waitFor(() => expect(rows()).toHaveLength(1))
    expect(urls().some((u) => u.includes('filter=following'))).toBe(true)
    expect(screen.getByRole('radio', { name: 'Đang theo dõi' })).toBeChecked()
    expect(rows()[0]).toHaveTextContent('Trần Minh Đức')

    await user.click(screen.getByRole('radio', { name: /^Chưa đọc/ }))
    await waitFor(() => expect(urls().some((u) => u.includes('filter=unread'))).toBe(true))
    await waitFor(() => expect(rows()).toHaveLength(1))
  })

  it('counts the unread threads on the chip', async () => {
    renderWithClient(panel())
    expect(await screen.findByRole('radio', { name: 'Chưa đọc 1' })).toBeInTheDocument()
  })

  it('opens the chosen thread', async () => {
    const user = userEvent.setup()
    const onOpenThread = vi.fn()
    renderWithClient(panel({ onOpenThread }))
    await screen.findByRole('list', { name: 'Thread' })
    await user.click(rows()[1]!)
    expect(onOpenThread).toHaveBeenCalledWith(MSG_QUIET)
  })

  it('says which filter is empty (the server sends no messages field at all) and offers all threads', async () => {
    const user = userEvent.setup()
    vi.mocked(apiFetch).mockImplementation((p: string, i?: RequestInit) =>
      p.includes('/threads?filter=all') ? fixtureApi(p, i) : p.includes('/threads') ? Promise.resolve({}) : fixtureApi(p, i)) as never
    renderWithClient(panel())
    await screen.findByRole('list', { name: 'Thread' })
    await user.click(screen.getByRole('radio', { name: 'Đang theo dõi' }))
    expect(await screen.findByText('Bạn chưa theo dõi thread nào.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Xem tất cả' }))
    expect(await screen.findByRole('list', { name: 'Thread' })).toBeInTheDocument()
  })

  it('pages with the last thread\'s time and root as the cursor', async () => {
    const user = userEvent.setup()
    vi.mocked(apiFetch).mockImplementation((p: string, i?: RequestInit) => {
      if (p.includes('/threads?') && p.includes('filter=all')) {
        const second = p.includes('before_id=')
        return Promise.resolve(second
          ? { messages: [{ ...QUIET_ROOT, id: 'older' }], has_more: false }
          : { messages: [MESSAGES[1]!], has_more: true })
      }
      return fixtureApi(p, i)
    }) as never
    renderWithClient(panel())
    await screen.findByRole('list', { name: 'Thread' })
    await user.click(screen.getByRole('button', { name: 'Xem thêm thread' }))
    await waitFor(() => expect(rows()).toHaveLength(2))
    const next = urls().find((u) => u.includes('before_id='))!
    const q = new URLSearchParams(next.split('?')[1])
    expect(q.get('before_id')).toBe(MSG.t1)
    expect(q.get('before')).toMatch(/^2026-10-09T0[0-9]:20:00Z$/)
    expect(screen.queryByRole('button', { name: 'Xem thêm thread' })).toBeNull()
  })

  it('has a retry when the list cannot be loaded', async () => {
    const user = userEvent.setup()
    let fail = true
    vi.mocked(apiFetch).mockImplementation((p: string, i?: RequestInit) =>
      p.includes('/threads?') && fail ? Promise.reject(new Error('boom')) : fixtureApi(p, i)) as never
    renderWithClient(panel())
    expect(await screen.findByText('Không tải được danh sách thread.')).toBeInTheDocument()
    fail = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByRole('list', { name: 'Thread' })).toBeInTheDocument()
  })

  it('is the thread list of the conversation, named so', async () => {
    renderWithClient(panel())
    const side = await screen.findByRole('complementary', { name: 'Thread của nhóm' })
    expect(within(side).getByText('Đối soát giao dịch')).toBeInTheDocument()
    expect(within(side).getByRole('button', { name: 'Đóng danh sách thread' })).toBeInTheDocument()
    expect(U.lan.id).toBeTruthy()
  })
})
