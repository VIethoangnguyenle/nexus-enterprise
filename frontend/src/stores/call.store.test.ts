import { beforeEach, describe, expect, it } from 'vitest'
import { isLive, useCallStore, type CallSession } from './call.store'

const session: CallSession = {
  callId: 'c1', channelId: 'ch1', kind: 'video', title: 'Đối soát giao dịch', isDm: false, joinedAt: 1, startedAt: 0,
}

describe('call store', () => {
  beforeEach(() => useCallStore.getState().clear())

  it('starts a call at "connecting" with a clean stage', () => {
    useCallStore.getState().setPanel('chat')
    useCallStore.getState().begin(session)
    const s = useCallStore.getState()
    expect(s.session).toEqual(session)
    expect(s.phase).toBe('connecting')
    expect(s.panel).toBeNull()
    expect(s.layout).toBe('auto')
  })

  it('pins a tile and unpins it with a second press', () => {
    useCallStore.getState().togglePin('u1')
    expect(useCallStore.getState().pinnedId).toBe('u1')
    useCallStore.getState().togglePin('u1')
    expect(useCallStore.getState().pinnedId).toBeNull()
  })

  it('choosing a layout releases the pin', () => {
    useCallStore.getState().togglePin('u1')
    useCallStore.getState().setLayout('grid')
    expect(useCallStore.getState()).toMatchObject({ layout: 'grid', pinnedId: null })
  })

  it('keeps the summary and closes the panel when the call finishes', () => {
    useCallStore.getState().begin(session)
    useCallStore.getState().setPanel('people')
    useCallStore.getState().finish({ durationMs: 5, people: 3, sinceMs: 1 }, 'ended')
    expect(useCallStore.getState()).toMatchObject({ phase: 'ended', panel: null, summary: { people: 3 } })
    expect(useCallStore.getState().session).toEqual(session)
  })

  it('reads which phases hold an open room', () => {
    expect(isLive('connected')).toBe(true)
    expect(isLive('reconnecting')).toBe(true)
    expect(isLive('ended')).toBe(false)
    expect(isLive(null)).toBe(false)
  })
})
