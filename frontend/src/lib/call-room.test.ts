import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Room as FakeRoomClass, DisconnectReason, RoomEvent, ConnectionQuality, Track, fakeTrack } from '../test/fake-livekit'
import type { Room } from 'livekit-client'
import { CallRoom } from './call-room'
import { HAND_TOPIC, REACTION_TOPIC, SHARE_TOPIC } from './call-model'

vi.mock('livekit-client', async () => (await import('../test/fake-livekit')).livekitMock)
vi.mock('./call-sound', () => ({ playChime: vi.fn() }))

function make() {
  const room = new FakeRoomClass()
  const cr = new CallRoom(room as unknown as Room, { selfId: 'me', selfName: 'Lê Thị Hoa' })
  return { room, cr }
}

const decode = (call: unknown[] | undefined) => JSON.parse(new TextDecoder().decode(call![0] as Uint8Array)) as Record<string, unknown>

describe('CallRoom', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('lists you first, then the others, and counts the peak', () => {
    const { room, cr } = make()
    room.join('u1', 'Trần Minh Đức')
    room.join('u2', 'Lê Quang Vinh')
    const names = cr.getSnapshot().participants.map((p) => p.name)
    expect(names).toEqual(['Lê Thị Hoa', 'Trần Minh Đức', 'Lê Quang Vinh'])
    room.leave('u2')
    expect(cr.getSnapshot().participants).toHaveLength(2)
    expect(cr.getSnapshot().peak).toBe(3)
  })

  it('connects and turns on the mic and camera that were chosen', async () => {
    const { room, cr } = make()
    await cr.connect('ws://x', 'tok', { micOn: true, camOn: false })
    expect(room.connect).toHaveBeenCalledWith('ws://x', 'tok')
    expect(room.localParticipant.setMicrophoneEnabled).toHaveBeenCalledWith(true)
    expect(room.localParticipant.setCameraEnabled).not.toHaveBeenCalled()
    expect(cr.getSnapshot()).toMatchObject({ link: 'connected', micOn: true, camOn: false })
  })

  it('keeps the microphone when the camera is blocked', async () => {
    const { room, cr } = make()
    room.localParticipant.setCameraEnabled.mockRejectedValueOnce(new Error('NotAllowedError'))
    await cr.connect('ws://x', 'tok', { micOn: true, camOn: true })
    expect(cr.getSnapshot()).toMatchObject({ micOn: true, camOn: false, mediaProblem: 'camera' })
  })

  it('toggles mic and camera', async () => {
    const { cr } = make()
    await cr.toggleMic()
    await cr.toggleCamera()
    expect(cr.getSnapshot()).toMatchObject({ micOn: true, camOn: true })
    await cr.toggleMic()
    expect(cr.getSnapshot().micOn).toBe(false)
  })

  it('shows a remote person with camera, muted mic and a poor network', () => {
    const { room, cr } = make()
    const p = room.join('u1', 'Nguyễn Thu Lan')
    p.setTrack(Track.Source.Camera, fakeTrack('c'))
    p.setTrack(Track.Source.Microphone, fakeTrack('m'), true)
    p.connectionQuality = ConnectionQuality.Poor
    room.emit(RoomEvent.TrackMuted)
    const v = cr.getSnapshot().participants[1]!
    expect(v).toMatchObject({ name: 'Nguyễn Thu Lan', camOn: true, micOn: false, quality: 'poor' })
    expect(v.camera).not.toBeNull()
  })

  it('marks the loudest remote speaker and keeps the last one when it goes quiet', () => {
    const { room, cr } = make()
    const a = room.join('u1', 'A')
    room.join('u2', 'B')
    room.emit(RoomEvent.ActiveSpeakersChanged, [a])
    expect(cr.getSnapshot().speakerId).toBe('u1')
    expect(cr.getSnapshot().participants.find((p) => p.id === 'u1')?.speaking).toBe(true)
    room.emit(RoomEvent.ActiveSpeakersChanged, [])
    expect(cr.getSnapshot().speakerId).toBe('u1')
    expect(cr.getSnapshot().participants.find((p) => p.id === 'u1')?.speaking).toBe(false)
  })

  describe('raised hands', () => {
    it('shows a hand received over data and orders by time', () => {
      const { room, cr } = make()
      const a = room.join('u1', 'A')
      const b = room.join('u2', 'B')
      room.data(b, HAND_TOPIC, { raised: true, at: 200 })
      room.data(a, HAND_TOPIC, { raised: true, at: 100 })
      const by = (id: string) => cr.getSnapshot().participants.find((p) => p.id === id)?.handRank
      expect(by('u1')).toBe(1)
      expect(by('u2')).toBe(2)
      room.data(a, HAND_TOPIC, { raised: false, at: 300 })
      expect(by('u1')).toBeNull()
      expect(by('u2')).toBe(1)
    })

    it('ignores malformed data and data without a sender', () => {
      const { room, cr } = make()
      const a = room.join('u1', 'A')
      room.data(a, HAND_TOPIC, { raised: 'yes' })
      room.emit(RoomEvent.DataReceived, new TextEncoder().encode('{"raised":true}'), undefined, 0, HAND_TOPIC)
      expect(cr.getSnapshot().participants[1]?.handRank).toBeNull()
    })

    it('publishes your own hand reliably and re-sends it to a late joiner only', async () => {
      const { room, cr } = make()
      await cr.toggleHand()
      const first = room.localParticipant.publishData.mock.calls[0]
      expect(decode(first)).toMatchObject({ raised: true })
      expect(first![1]).toMatchObject({ reliable: true, topic: HAND_TOPIC })
      expect(cr.getSnapshot().handUp).toBe(true)

      room.join('u9', 'Late')
      const resend = room.localParticipant.publishData.mock.calls[1]
      expect(resend![1]).toMatchObject({ topic: HAND_TOPIC, destinationIdentities: ['u9'] })
      expect(decode(resend)).toMatchObject({ raised: true })
    })

    it('lowers itself two seconds after you finish speaking', async () => {
      const { room, cr } = make()
      await cr.toggleHand()
      room.emit(RoomEvent.ActiveSpeakersChanged, [room.localParticipant])
      room.emit(RoomEvent.ActiveSpeakersChanged, [])
      expect(cr.getSnapshot().handUp).toBe(true)
      await vi.advanceTimersByTimeAsync(2100)
      expect(cr.getSnapshot().handUp).toBe(false)
    })

    it('does not lower a hand that has not spoken yet', async () => {
      const { cr } = make()
      await cr.toggleHand()
      await vi.advanceTimersByTimeAsync(5000)
      expect(cr.getSnapshot().handUp).toBe(true)
    })
  })

  describe('reactions', () => {
    it('floats a received reaction with the sender name for three seconds', async () => {
      const { room, cr } = make()
      const a = room.join('u1', 'Trần Minh Đức')
      room.data(a, REACTION_TOPIC, { emoji: '👏' })
      expect(cr.getSnapshot().reactions).toMatchObject([{ emoji: '👏', name: 'Trần Minh Đức' }])
      await vi.advanceTimersByTimeAsync(3100)
      expect(cr.getSnapshot().reactions).toHaveLength(0)
    })

    it('drops a reaction that is not one of the six', () => {
      const { room, cr } = make()
      const a = room.join('u1', 'A')
      room.data(a, REACTION_TOPIC, { emoji: 'evil' })
      expect(cr.getSnapshot().reactions).toHaveLength(0)
    })

    it('sends yours lossy and shows it to you too', async () => {
      const { room, cr } = make()
      await cr.react('🎉')
      const call = room.localParticipant.publishData.mock.calls[0]
      expect(call![1]).toMatchObject({ reliable: false, topic: REACTION_TOPIC })
      expect(cr.getSnapshot().reactions[0]).toMatchObject({ emoji: '🎉', name: 'Lê Thị Hoa' })
    })
  })

  describe('presenting', () => {
    it('names the presenter, the latest one winning', () => {
      const { room, cr } = make()
      const a = room.join('u1', 'A')
      const b = room.join('u2', 'B')
      a.setTrack(Track.Source.ScreenShare, fakeTrack('s1'))
      room.emit(RoomEvent.TrackSubscribed)
      expect(cr.getSnapshot().presenterId).toBe('u1')
      b.setTrack(Track.Source.ScreenShare, fakeTrack('s2'))
      room.emit(RoomEvent.TrackSubscribed)
      expect(cr.getSnapshot().presenterId).toBe('u2')
      b.setTrack(Track.Source.ScreenShare, null)
      room.emit(RoomEvent.TrackUnsubscribed)
      expect(cr.getSnapshot().presenterId).toBe('u1')
    })

    it('tells the others when you start, and steps down when someone else starts', async () => {
      const { room, cr } = make()
      const a = room.join('u1', 'Trần Minh Đức')
      await cr.startShare()
      expect(cr.getSnapshot().sharing).toBe(true)
      expect(room.localParticipant.publishData.mock.calls[room.localParticipant.publishData.mock.calls.length - 1]?.[1]).toMatchObject({ topic: SHARE_TOPIC, reliable: true })

      room.data(a, SHARE_TOPIC, { action: 'start' })
      await vi.advanceTimersByTimeAsync(0)
      expect(room.localParticipant.setScreenShareEnabled).toHaveBeenLastCalledWith(false)
      expect(cr.getSnapshot().notice).toEqual({ kind: 'share-replaced', name: 'Trần Minh Đức' })
      cr.dismissNotice()
      expect(cr.getSnapshot().notice).toBeNull()
    })

    it('is not a fault when the picker is cancelled, but a blocked capture is', async () => {
      const { room, cr } = make()
      room.localParticipant.setScreenShareEnabled.mockRejectedValueOnce(new Error('NotAllowedError'))
      await cr.startShare()
      expect(cr.getSnapshot()).toMatchObject({ sharing: false, mediaProblem: 'screen' })
    })
  })

  describe('hold to talk', () => {
    it('opens the mic while held and closes it on release', async () => {
      const { cr } = make()
      await cr.holdToTalk(true)
      expect(cr.getSnapshot().micOn).toBe(true)
      await cr.holdToTalk(false)
      expect(cr.getSnapshot().micOn).toBe(false)
    })

    it('leaves an open mic alone', async () => {
      const { cr } = make()
      await cr.toggleMic()
      await cr.holdToTalk(true)
      await cr.holdToTalk(false)
      expect(cr.getSnapshot().micOn).toBe(true)
    })
  })

  describe('connection', () => {
    it('shows reconnecting and recovers', () => {
      const { room, cr } = make()
      room.emit(RoomEvent.Reconnecting)
      expect(cr.getSnapshot().link).toBe('reconnecting')
      room.emit(RoomEvent.Reconnected)
      expect(cr.getSnapshot().link).toBe('connected')
    })

    it('reads removal from the room as being kicked', () => {
      const { room, cr } = make()
      room.emit(RoomEvent.Disconnected, DisconnectReason.PARTICIPANT_REMOVED)
      expect(cr.getSnapshot()).toMatchObject({ link: 'disconnected', end: 'removed' })
    })

    it.each([
      [DisconnectReason.ROOM_DELETED, 'ended'],
      [DisconnectReason.CLIENT_INITIATED, 'left'],
      [DisconnectReason.CONNECTION_TIMEOUT, 'failed'],
      [DisconnectReason.UNKNOWN_REASON, 'failed'],
    ])('reads disconnect reason %s as %s', (reason, end) => {
      const { room, cr } = make()
      room.emit(RoomEvent.Disconnected, reason)
      expect(cr.getSnapshot().end).toBe(end)
    })

    it('counts a disconnect you asked for as leaving, whatever the reason says', async () => {
      const { room, cr } = make()
      await cr.disconnect()
      room.emit(RoomEvent.Disconnected, DisconnectReason.UNKNOWN_REASON)
      expect(cr.getSnapshot().end).toBe('left')
    })

    it('reports blocked sound until a tap allows it', async () => {
      const { room, cr } = make()
      room.canPlaybackAudio = false
      room.emit(RoomEvent.AudioPlaybackStatusChanged)
      expect(cr.getSnapshot().audioBlocked).toBe(true)
      room.canPlaybackAudio = true
      await cr.enableAudio()
      expect(room.startAudio).toHaveBeenCalled()
      expect(cr.getSnapshot().audioBlocked).toBe(false)
    })
  })

  it('stops listening on dispose', () => {
    const { room, cr } = make()
    cr.dispose()
    expect(room.listenerCount(RoomEvent.Disconnected)).toBe(0)
  })
})
