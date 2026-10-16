import 'fake-indexeddb/auto'
import { act, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { keys } from '../../hooks/keys'
import { createDeviceCache, wipeDeviceCache } from '../../lib/query-persist'
import { UpdatingBar, UPDATING_GIVE_UP_MS } from './UpdatingBar'

let connection: 'online' | 'lost' | 'restored' = 'online'
vi.mock('../../hooks/useConnectionLost', () => ({ useConnectionStatus: () => connection }))

const deps = (queryClient: QueryClient) => ({
  queryClient,
  getScope: () => ({ userId: 'user-a', workspaceId: 'ws-1' }),
  getUserId: () => 'user-a',
  sync: { get: () => undefined, restore: () => {} },
  buster: 'b1',
  throttleMs: 0,
})

let client: QueryClient

/** Saves a chat state on the device, then starts a fresh client from it. */
async function startFromDevice() {
  const before = new QueryClient()
  before.setQueryData(keys.messaging.chatState('ws-1'), { channels: [] })
  await createDeviceCache(deps(before)).flush()
  vi.setSystemTime(Date.now() + 5_000)
  client = new QueryClient()
  await createDeviceCache(deps(client)).restore()
}

const renderBar = () => render(<QueryClientProvider client={client}><UpdatingBar /></QueryClientProvider>)

beforeEach(async () => {
  connection = 'online'
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
  await wipeDeviceCache()
})
afterEach(() => { vi.useRealTimers() })

describe('UpdatingBar', () => {
  it('says so while the copy from the device is on screen and nothing newer has come', async () => {
    await startFromDevice()
    renderBar()
    const region = screen.getByRole('status')
    expect(region).toHaveAttribute('aria-live', 'polite')
    expect(region).toHaveTextContent('Đang cập nhật')
  })

  it('goes once the fresh chat state arrives', async () => {
    await startFromDevice()
    renderBar()
    vi.setSystemTime(Date.now() + 1_000)
    act(() => { client.setQueryData(keys.messaging.chatState('ws-1'), { channels: ['new'] }) })
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })

  it('goes when the fresh fetch fails', async () => {
    await startFromDevice()
    renderBar()
    await act(async () => {
      await client
        .fetchQuery({ queryKey: keys.messaging.chatState('ws-1'), queryFn: () => Promise.reject(new Error('down')), retry: false, staleTime: 0 })
        .catch(() => {})
    })
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })

  it('gives way to the connection bar', async () => {
    connection = 'lost'
    await startFromDevice()
    renderBar()
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })

  it('never shows when the app did not start from a copy', async () => {
    await wipeDeviceCache()
    client = new QueryClient()
    await createDeviceCache(deps(client)).restore() // nothing stored: clears any earlier copy
    client.setQueryData(keys.messaging.chatState('ws-1'), { channels: [] })
    renderBar()
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })

  it('stops waiting after a while', async () => {
    await startFromDevice()
    renderBar()
    act(() => { vi.advanceTimersByTime(UPDATING_GIVE_UP_MS + 1) })
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })
})
