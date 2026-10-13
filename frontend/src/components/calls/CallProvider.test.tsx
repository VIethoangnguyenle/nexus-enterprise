import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, screen } from '@testing-library/react'
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

import { CallProvider } from './CallProvider'

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
})
