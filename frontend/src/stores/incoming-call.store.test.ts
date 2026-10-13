import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { useIncomingCallStore } from './incoming-call.store'

beforeEach(() => {
  vi.useFakeTimers()
  useIncomingCallStore.setState({ incoming: null })
})

afterEach(() => {
  vi.useRealTimers()
})

describe('incoming-call store', () => {
  it('ring sets the incoming call with mode ringing', () => {
    useIncomingCallStore.getState().ring({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Phạm Hải Yến',
      callerAvatarUrl: '',
    })
    const s = useIncomingCallStore.getState().incoming
    expect(s).toMatchObject({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Phạm Hải Yến',
      callerAvatarUrl: '',
      mode: 'ringing',
    })
  })

  it('invite sets mode invited', () => {
    useIncomingCallStore.getState().invite({
      callId: 'c2',
      channelId: 'ch2',
      kind: 'audio',
      callerName: 'Trần Minh Đức',
      callerAvatarUrl: '/avatar.png',
    })
    expect(useIncomingCallStore.getState().incoming?.mode).toBe('invited')
  })

  it('answeredElsewhere sets mode answered_elsewhere', () => {
    useIncomingCallStore.getState().ring({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Yến',
      callerAvatarUrl: '',
    })
    useIncomingCallStore.getState().answeredElsewhere('c1')
    expect(useIncomingCallStore.getState().incoming?.mode).toBe('answered_elsewhere')
  })

  it('answeredElsewhere is a no-op when the call id does not match', () => {
    useIncomingCallStore.getState().ring({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Yến',
      callerAvatarUrl: '',
    })
    useIncomingCallStore.getState().answeredElsewhere('other')
    expect(useIncomingCallStore.getState().incoming?.mode).toBe('ringing')
  })

  it('clear removes the incoming call', () => {
    useIncomingCallStore.getState().ring({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Yến',
      callerAvatarUrl: '',
    })
    useIncomingCallStore.getState().clear('c1')
    expect(useIncomingCallStore.getState().incoming).toBeNull()
  })

  it('clear is a no-op when the call id does not match', () => {
    useIncomingCallStore.getState().ring({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Yến',
      callerAvatarUrl: '',
    })
    useIncomingCallStore.getState().clear('other')
    expect(useIncomingCallStore.getState().incoming?.callId).toBe('c1')
  })

  it('ringing auto-clears after 45 seconds', () => {
    useIncomingCallStore.getState().ring({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Yến',
      callerAvatarUrl: '',
    })
    expect(useIncomingCallStore.getState().incoming).not.toBeNull()
    vi.advanceTimersByTime(44_999)
    expect(useIncomingCallStore.getState().incoming).not.toBeNull()
    vi.advanceTimersByTime(1)
    expect(useIncomingCallStore.getState().incoming).toBeNull()
  })

  it('a second ring replaces the first and resets the timer', () => {
    useIncomingCallStore.getState().ring({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Yến',
      callerAvatarUrl: '',
    })
    vi.advanceTimersByTime(30_000) // 30 s into the first
    useIncomingCallStore.getState().ring({
      callId: 'c2',
      channelId: 'ch2',
      kind: 'audio',
      callerName: 'Đức',
      callerAvatarUrl: '',
    })
    vi.advanceTimersByTime(30_000) // 60 s total, but only 30 s into the second
    expect(useIncomingCallStore.getState().incoming?.callId).toBe('c2')
    vi.advanceTimersByTime(15_000) // 45 s into the second
    expect(useIncomingCallStore.getState().incoming).toBeNull()
  })

  it('answered_elsewhere auto-clears after 3 seconds', () => {
    useIncomingCallStore.getState().ring({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Yến',
      callerAvatarUrl: '',
    })
    useIncomingCallStore.getState().answeredElsewhere('c1')
    expect(useIncomingCallStore.getState().incoming?.mode).toBe('answered_elsewhere')
    vi.advanceTimersByTime(2_999)
    expect(useIncomingCallStore.getState().incoming?.mode).toBe('answered_elsewhere')
    vi.advanceTimersByTime(1)
    expect(useIncomingCallStore.getState().incoming).toBeNull()
  })

  it('invite does not auto-clear (no timeout)', () => {
    useIncomingCallStore.getState().invite({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Yến',
      callerAvatarUrl: '',
    })
    vi.advanceTimersByTime(120_000)
    expect(useIncomingCallStore.getState().incoming).not.toBeNull()
  })
})
