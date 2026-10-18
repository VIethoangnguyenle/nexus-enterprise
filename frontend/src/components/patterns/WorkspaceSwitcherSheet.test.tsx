import '@testing-library/jest-dom/vitest'
import { useState } from 'react'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { WS_ID } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { queryClient } from '../../lib/query-client'
import { useAuthStore } from '../../stores/auth.store'
import { useDriveStore } from '../../stores/drive.store'
import { Pressable } from '../primitives'
import { WorkspaceSwitcherSheet } from './WorkspaceSwitcherSheet'
import { OTHER_WS, answer, dialogOf, renderShell, resetServer, server, tokenFor, viewport } from './phone-shell-harness'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

function Host() {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Pressable onClick={() => setOpen(true)}>Mở danh sách</Pressable>
      {open && <WorkspaceSwitcherSheet onClose={() => setOpen(false)} />}
    </>
  )
}

async function openSheet(url = '/channels') {
  const router = await renderShell(<Host />, url)
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: 'Mở danh sách' }))
  const dialog = await dialogOf('Đổi workspace')
  const list = await within(dialog).findByRole('listbox', { name: 'Workspace của bạn' })
  return { router, user, dialog, list }
}

beforeEach(() => {
  viewport(true)
  resetServer()
  api.mockReset()
  api.mockImplementation(answer as typeof apiFetch)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
  useDriveStore.setState({ selectedItemId: null, expandedFolders: new Set() })
})

describe('the workspace sheet', () => {
  it('names the open workspace and its kind', async () => {
    const { dialog } = await openSheet()
    expect(within(dialog).getAllByText('Khối Vận hành').length).toBeGreaterThan(0)
    await waitFor(() => expect(dialog).toHaveTextContent('Workspace tổ chức · 64 thành viên'))
  })

  it('says Workspace cá nhân for a personal one', async () => {
    server.personal = true
    const { dialog } = await openSheet()
    await waitFor(() => expect(dialog).toHaveTextContent('Workspace cá nhân · chỉ bạn'))
  })

  it('offers only the workspaces the person can enter, the open one ticked', async () => {
    const { list } = await openSheet()
    const options = within(list).getAllByRole('option')
    expect(options.map((o) => o.getAttribute('aria-selected'))).toEqual(['true', 'false'])
    expect(options[0]).toHaveTextContent('Khối Vận hành')
    expect(options[1]).toHaveTextContent('NovaPay Kế toán')
    expect(within(list).queryByText('Chỉ xem được')).toBeNull()
  })

  it('re-scopes the token, drops the old data, resets Tài liệu and lands on Chat of the new workspace', async () => {
    const { router, user, list, dialog } = await openSheet('/drive')
    queryClient.setQueryData(['drive', 'old'], { items: ['x'] })
    useDriveStore.setState({ selectedItemId: 'item-1', expandedFolders: new Set(['f1']) })

    await user.click(within(list).getByRole('option', { name: /NovaPay Kế toán/ }))

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(api).toHaveBeenCalledWith('/auth/switch-tenant', expect.objectContaining({ method: 'POST', body: JSON.stringify({ tenant_id: OTHER_WS }) }))
    expect(useAuthStore.getState().accessToken).toBe(tokenFor(OTHER_WS))
    expect(useAuthStore.getState().tenantId).toBe(OTHER_WS)
    expect(queryClient.getQueryData(['drive', 'old'])).toBeUndefined()
    expect(useDriveStore.getState().selectedItemId).toBeNull()
    expect(useDriveStore.getState().expandedFolders.size).toBe(0)
    expect(router.state.location.pathname).toBe('/channels')
    expect(router.state.location.search).toEqual({ ws: OTHER_WS })
    expect(dialog).not.toBeInTheDocument()
  })

  it('leaves everything as it was when the server refuses', async () => {
    server.switchError = Object.assign(new Error('forbidden'), { status: 403 })
    const { user, list } = await openSheet('/drive')
    queryClient.setQueryData(['drive', 'old'], { items: ['x'] })

    await user.click(within(list).getByRole('option', { name: /NovaPay Kế toán/ }))

    await waitFor(() => expect(api).toHaveBeenCalledWith('/auth/switch-tenant', expect.anything()))
    await waitFor(() => expect(within(list).getAllByRole('option')[1]).not.toBeDisabled())
    expect(useAuthStore.getState().accessToken).toBe(tokenFor(WS_ID))
    expect(queryClient.getQueryData(['drive', 'old'])).toBeDefined()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('does nothing but close when the open workspace is chosen', async () => {
    const { user, list } = await openSheet()
    await user.click(within(list).getAllByRole('option')[0]!)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(api).not.toHaveBeenCalledWith('/auth/switch-tenant', expect.anything())
  })

  it('ends the list with Tạo workspace, which remembers the screen to come back to', async () => {
    const { dialog } = await openSheet()
    const createLink = within(dialog).getByRole('link', { name: /Tạo workspace/ })
    const href = new URL(createLink.getAttribute('href')!, 'http://app.test')
    expect(href.pathname).toBe('/onboarding')
    expect(href.searchParams.get('from')).toBe('/channels')
  })

  it('opens Cài đặt workspace from the ⋯ on the open workspace, and closes', async () => {
    const { router, user, dialog } = await openSheet(`/drive?ws=${WS_ID}`)
    await user.click(within(dialog).getByRole('button', { name: 'Cài đặt workspace' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/workspace'))
    expect(router.state.location.search).toMatchObject({ ws: WS_ID })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Đổi workspace' })).toBeNull())
  })

  it('closes on Esc', async () => {
    const { user } = await openSheet()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})

describe('offers waiting', () => {
  it('lists Lời mời with the count; tapping it opens Chọn workspace', async () => {
    server.offers = 3
    const { router, user, dialog } = await openSheet()
    const row = await within(dialog).findByRole('link', { name: /Lời mời.*3 lời mời đang chờ/ })
    expect(row).toHaveTextContent('3')
    await user.click(row)
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Đổi workspace' })).toBeNull())
  })

  it('caps the pill at 99+', async () => {
    server.offers = 150
    const { dialog } = await openSheet()
    expect(await within(dialog).findByText('99+')).toBeInTheDocument()
  })

  it('shows no row without offers', async () => {
    const { dialog } = await openSheet()
    await waitFor(() => expect(api).toHaveBeenCalledWith('/invitations'))
    expect(within(dialog).queryByRole('link', { name: /Lời mời/ })).toBeNull()
  })

  it('shows nothing and never asks when the address is not verified', async () => {
    server.offers = 2
    server.addressVerified = false
    const { dialog } = await openSheet()
    await waitFor(() => expect(api).toHaveBeenCalledWith('/me'))
    expect(within(dialog).queryByRole('link', { name: /Lời mời/ })).toBeNull()
    expect(api).not.toHaveBeenCalledWith('/invitations')
  })
})
