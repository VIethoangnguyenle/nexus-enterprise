import '@testing-library/jest-dom/vitest'
import { useState } from 'react'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { UUID_RE } from '../../test/chat-fixtures'
import { apiFetch, logoutSession } from '../../api/client'
import { Pressable } from '../primitives'
import { AccountSheet } from './AccountSheet'
import { answer, dialogOf, renderShell, resetServer, viewport } from './phone-shell-harness'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  logoutSession: vi.fn(),
}))

function Host() {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Pressable onClick={() => setOpen(true)}>Tài khoản của bạn</Pressable>
      {open && <AccountSheet onClose={() => setOpen(false)} />}
    </>
  )
}

async function openSheet() {
  const router = await renderShell(<Host />, '/channels')
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: 'Tài khoản của bạn' }))
  return { router, user, sheet: await dialogOf('Tài khoản') }
}

beforeEach(() => {
  viewport(true)
  resetServer()
  vi.mocked(apiFetch).mockReset()
  vi.mocked(apiFetch).mockImplementation(answer as typeof apiFetch)
  vi.mocked(logoutSession).mockClear()
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

describe('AccountSheet', () => {
  it('says who the person is here: name, role and workspace, status, and no identifier', async () => {
    const { sheet } = await openSheet()
    expect(sheet).toHaveAttribute('aria-modal', 'true')
    expect(await within(sheet).findByText('Lê Thị Hoa')).toBeInTheDocument()
    await waitFor(() => expect(sheet).toHaveTextContent(/·\s*Khối Vận hành/))
    expect(within(sheet).getByText('Đang hoạt động')).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('lists the notification settings, the account, Cài đặt and Đăng xuất', async () => {
    const { sheet } = await openSheet()
    expect(within(sheet).getAllByRole('link').map((a) => a.textContent)).toEqual([
      'Cài đặt thông báoGiờ yên lặng và các loại thông báo',
      'Cài đặt tài khoản',
      'Cài đặt',
    ])
    expect(within(sheet).getByRole('button', { name: 'Đăng xuất' })).toBeInTheDocument()
  })

  it('opens the notification tab of Cài đặt and closes', async () => {
    const { router, user, sheet } = await openSheet()
    await user.click(within(sheet).getByRole('link', { name: /Cài đặt thông báo/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
    expect(router.state.location.search).toMatchObject({ tab: 'thong-bao' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('opens Cài đặt and closes', async () => {
    const { router, user, sheet } = await openSheet()
    await user.click(within(sheet).getByRole('link', { name: 'Cài đặt' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('signs out through the one logout path', async () => {
    const { user, sheet } = await openSheet()
    await user.click(within(sheet).getByRole('button', { name: 'Đăng xuất' }))
    expect(logoutSession).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('gives focus back to the avatar on Esc', async () => {
    const { user } = await openSheet()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('button', { name: 'Tài khoản của bạn' })).toHaveFocus()
  })
})
