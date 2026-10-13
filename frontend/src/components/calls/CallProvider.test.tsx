import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, screen } from '@testing-library/react'
import { useEffect } from 'react'
import { ApiError } from '../../api/client'
import { keys } from '../../hooks/keys'
import { queryClient } from '../../lib/query-client'
import { ME } from '../../test/chat-fixtures'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { useCallStore, type CallSession } from '../../stores/call.store'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('livekit-client', async () => (await import('../../test/fake-livekit')).livekitMock)
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'w1' }) }))
vi.mock('../../hooks/usePeople', async () => {
  const { EMPTY_DIRECTORY } = await import('../../lib/people')
  return { usePeople: () => ({ ...EMPTY_DIRECTORY, isLoading: false }) }
})

const calls = vi.hoisted(() => ({ cancel: vi.fn() }))
vi.mock('../../api/calls', async (orig) => ({ ...(await orig<object>()), callsApi: { cancel: calls.cancel, token: vi.fn(), get: vi.fn() } }))

import { CallProvider, useCallApi } from './CallProvider'

function Leaver({ onReady }: { onReady: (leave: () => Promise<void>) => void }) {
  const { leave } = useCallApi()
  useEffect(() => { onReady(leave) }, [leave, onReady])
  return null
}


const session: CallSession = {
  callId: 'c1', channelId: 'ch1', kind: 'video', title: 'Đối soát giao dịch', isDm: false, joinedAt: 1, startedAt: 1,
}

describe('CallProvider', () => {
  beforeEach(() => {
    routerState.navigate.mockReset()
    routerState.pathname = '/channels'
    useCallStore.getState().clear()
  })
  afterEach(() => { cleanup(); resetClient() })

  it('renders its children and no floating tile without a call', () => {
    renderWithClient(<CallProvider><p>Trang chủ</p></CallProvider>)
    expect(screen.getByText('Trang chủ')).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Cuộc gọi đang thu nhỏ' })).toBeNull()
  })

  it('shows the floating tile when a call is open and its screen is not', async () => {
    useCallStore.getState().begin(session)
    useCallStore.getState().setPhase('connected')
    renderWithClient(<CallProvider><p>Trang chủ</p></CallProvider>)
    // Without a room there is no one to show, but the tile still offers the way back and the way out.
    expect(await screen.findByRole('region', { name: 'Cuộc gọi đang thu nhỏ' })).toBeInTheDocument()
  })

  it('has no floating tile on the call screen itself', () => {
    routerState.pathname = '/calls/c1'
    useCallStore.getState().begin(session)
    useCallStore.getState().setPhase('connected')
    renderWithClient(<CallProvider><p>x</p></CallProvider>)
    expect(screen.queryByRole('region', { name: 'Cuộc gọi đang thu nhỏ' })).toBeNull()
  })

  it('sends you to the call screen when you are cut off while elsewhere', () => {
    useCallStore.getState().begin(session)
    renderWithClient(<CallProvider><p>x</p></CallProvider>)
    act(() => { useCallStore.getState().finish({ durationMs: 1, people: 2, sinceMs: 1 }, 'removed') })
    expect(routerState.navigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/calls/$callId', params: { callId: 'c1' } }))
  })

  it('does not send you anywhere when the call simply ended', () => {
    useCallStore.getState().begin(session)
    renderWithClient(<CallProvider><p>x</p></CallProvider>)
    act(() => { useCallStore.getState().finish({ durationMs: 1, people: 2, sinceMs: 1 }, 'ended') })
    expect(routerState.navigate).not.toHaveBeenCalled()
  })

  describe('hanging up before anyone answered', () => {
    const seedCall = (status: string, startedBy: string) =>
      queryClient.setQueryData(keys.calls.detail('c1'), { id: 'c1', channel_id: 'ch1', kind: 'video', status, started_by: startedBy })

    async function hangUp() {
      let leave: () => Promise<void> = async () => {}
      renderWithClient(<CallProvider><Leaver onReady={(l) => { leave = l }} /></CallProvider>)
      useCallStore.getState().begin(session)
      await act(async () => { await leave() })
    }

    beforeEach(() => { calls.cancel.mockReset().mockResolvedValue({}); queryClient.clear() })

    it('cancels a call you started that still rings', async () => {
            seedCall('ringing', ME.id)
      await hangUp()
      expect(calls.cancel).toHaveBeenCalledWith('c1')
    })

    it('does not cancel one that is running, or one somebody else started', async () => {
            seedCall('active', ME.id)
      await hangUp()
      seedCall('ringing', 'someone-else')
      await hangUp()
      expect(calls.cancel).not.toHaveBeenCalled()
    })

    it('is quiet when the call had already stopped ringing', async () => {
            calls.cancel.mockRejectedValue(new ApiError('gone', 409, { code: 'call_ended' }))
      seedCall('ringing', ME.id)
      await expect(hangUp()).resolves.toBeUndefined()
      expect(useCallStore.getState().session).toBeNull()
    })
  })
})
