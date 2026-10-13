import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { apiFetch } from '../../api/client'
import { NotificationScreen } from './NotificationScreen'
import { answer, dialogOf, renderShell, resetServer, server, viewport } from './phone-shell-harness'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

const onClose = vi.fn()

const note = (id: string, over: Record<string, unknown> = {}) => ({
  id, type: 'approval_approved', title: 'Request approved', body: 'raw english body', read: false,
  created_at: new Date(Date.now() - 5 * 60_000).toISOString(), entity_type: 'approval', entity_id: 'r1', ...over,
})

beforeEach(() => {
  viewport(true)
  resetServer()
  server.notifications = [note('n1'), note('n2', { read: true })]
  server.notificationsUnread = 1
  onClose.mockClear()
  api.mockReset()
  api.mockImplementation(((path: string, init?: RequestInit) =>
    init?.method === 'POST' ? Promise.resolve({ status: 'ok' }) : answer(path, init)) as typeof apiFetch)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

describe('NotificationScreen', () => {
  it('is a full modal screen with Quay lại instead of Đóng', async () => {
    await renderShell(<NotificationScreen open onClose={onClose} />)
    const screenEl = await dialogOf('Thông báo')
    expect(screenEl).toHaveAttribute('aria-modal', 'true')
    expect(within(screenEl).getByRole('button', { name: 'Quay lại' })).toBeInTheDocument()
    expect(within(screenEl).queryByRole('button', { name: 'Đóng thông báo' })).toBeNull()
    expect(await within(screenEl).findByText('1 chưa đọc')).toBeInTheDocument()
  })

  it('draws nothing while closed', async () => {
    await renderShell(<NotificationScreen open={false} onClose={onClose} />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('goes back from Quay lại and from Esc', async () => {
    const user = userEvent.setup()
    await renderShell(<NotificationScreen open onClose={onClose} />)
    const screenEl = await dialogOf('Thông báo')
    await user.click(within(screenEl).getByRole('button', { name: 'Quay lại' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('marks all read from the same control as the panel', async () => {
    const user = userEvent.setup()
    await renderShell(<NotificationScreen open onClose={onClose} />)
    const screenEl = await dialogOf('Thông báo')
    await user.click(await within(screenEl).findByRole('button', { name: /Đánh dấu tất cả đã đọc/ }))
    await waitFor(() => expect(api).toHaveBeenCalledWith('/notifications/read-all', expect.objectContaining({ method: 'POST' })))
  })

  it('opens a notification, marks it read and leaves the screen', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<NotificationScreen open onClose={onClose} />)
    const screenEl = await dialogOf('Thông báo')
    const rows = await within(screenEl).findAllByRole('link')
    await user.click(rows[0]!)
    await waitFor(() => expect(router.state.location.pathname).toBe('/approval'))
    expect(onClose).toHaveBeenCalled()
    expect(api).toHaveBeenCalledWith('/notifications/n1/read', expect.objectContaining({ method: 'POST' }))
  })
})
