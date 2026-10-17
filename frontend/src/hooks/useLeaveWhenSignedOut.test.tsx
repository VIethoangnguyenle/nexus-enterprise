import { act, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch, publicFetch } from '../api/client'
import { renderAuthScreen, startAuthTest, where } from '../test/auth-render'
import { useAuthStore } from '../stores/auth.store'
import { useLeaveWhenSignedOut } from './useLeaveWhenSignedOut'

vi.mock('../api/client', async (orig) => ({
  ...(await orig<typeof import('../api/client')>()),
  apiFetch: vi.fn(),
  publicFetch: vi.fn(),
}))

function Host() {
  useLeaveWhenSignedOut()
  return <p>màn hình</p>
}

describe('useLeaveWhenSignedOut', () => {
  beforeEach(() => {
    vi.mocked(apiFetch).mockReset()
    vi.mocked(publicFetch).mockReset()
  })

  it('sends the person to sign-in, replacing the entry, when the session ends while the screen is open', async () => {
    startAuthTest({ signedIn: true })
    const router = await renderAuthScreen(Host, '/workspace-select')
    await screen.findByText('màn hình')
    act(() => useAuthStore.getState().logout())
    await waitFor(() => expect(where(router).path).toBe('/login'))
    // replace: Back does not return to a screen that needs a session.
    expect(router.history.length).toBe(1)
  })

  it('stays put while the session lasts', async () => {
    startAuthTest({ signedIn: true })
    const router = await renderAuthScreen(Host, '/workspace-select')
    await screen.findByText('màn hình')
    expect(where(router).path).toBe('/workspace-select')
  })
})
