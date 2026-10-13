import { act, cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Pressable } from '../components/primitives'
import { resetClient } from '../test/render'
import { renderShell, viewport, answer, resetServer } from '../components/patterns/phone-shell-harness'
import { apiFetch } from '../api/client'
import { useChatOverlay } from './useChatOverlay'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))

function Host() {
  const { overlay, openSearch, openThreads, close } = useChatOverlay()
  return (
    <>
      <output aria-label="layer">{overlay ? JSON.stringify(overlay) : 'none'}</output>
      <Pressable onClick={openSearch}>open search</Pressable>
      <Pressable onClick={() => openThreads('following')}>open threads</Pressable>
      <Pressable onClick={close}>close</Pressable>
    </>
  )
}

beforeEach(() => {
  viewport(true)
  resetServer()
  vi.mocked(apiFetch).mockImplementation(answer as typeof apiFetch)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

const layer = () => screen.getByLabelText('layer').textContent
const search = (router: Awaited<ReturnType<typeof renderShell>>) => router.state.location.search as Record<string, unknown>

describe('a full-screen layer of the Chat list', () => {
  it('opens as a new history entry, and the system back closes it', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<Host />, '/channels')
    await user.click(screen.getByRole('button', { name: 'open search' }))
    expect(layer()).toBe('{"kind":"search"}')
    expect(search(router).search).toBe(1)
    await act(async () => router.history.back())
    await waitFor(() => expect(layer()).toBe('none'))
    expect(router.state.location.pathname).toBe('/channels')
    expect(search(router).search).toBeUndefined()
  })

  it('closes by going back when it pushed the entry, so no entry is left to come back to', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<Host />, '/channels?tab=more')
    await user.click(screen.getByRole('button', { name: 'open threads' }))
    expect(layer()).toBe('{"kind":"threads","chip":"following"}')
    expect(search(router)).toMatchObject({ tab: 'more', threads: 'followed' })
    await user.click(screen.getByRole('button', { name: 'close' }))
    await waitFor(() => expect(layer()).toBe('none'))
    expect(search(router)).toEqual({ tab: 'more' })
    expect(router.history.canGoBack()).toBe(false)
  })

  it('closes by replacing the address when the page was opened on it, keeping the rest of the address', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<Host />, '/channels?ws=w1&tab=direct&search=1')
    expect(layer()).toBe('{"kind":"search"}')
    await user.click(screen.getByRole('button', { name: 'close' }))
    expect(layer()).toBe('none')
    expect(search(router)).toEqual({ ws: 'w1', tab: 'direct' })
    // Nothing was added to history, and back does not return to the layer.
    expect(router.history.canGoBack()).toBe(false)
  })

  it('does not show a layer the address names wrongly', async () => {
    await renderShell(<Host />, '/channels?search=banana&threads=all')
    expect(layer()).toBe('none')
  })
})
