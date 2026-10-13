import { vi } from 'vitest'

/**
 * A stand-in for `livekit-client` in tests: a `Room` that emits the same event
 * names, and participants whose tracks the test sets directly. Use it with
 *
 *   vi.mock('livekit-client', async () => (await import('../test/fake-livekit')).livekitMock)
 */

export const RoomEvent = {
  ParticipantConnected: 'participantConnected',
  ParticipantDisconnected: 'participantDisconnected',
  TrackSubscribed: 'trackSubscribed',
  TrackUnsubscribed: 'trackUnsubscribed',
  TrackPublished: 'trackPublished',
  TrackUnpublished: 'trackUnpublished',
  TrackMuted: 'trackMuted',
  TrackUnmuted: 'trackUnmuted',
  LocalTrackPublished: 'localTrackPublished',
  LocalTrackUnpublished: 'localTrackUnpublished',
  ConnectionQualityChanged: 'connectionQualityChanged',
  ParticipantNameChanged: 'participantNameChanged',
  ActiveSpeakersChanged: 'activeSpeakersChanged',
  DataReceived: 'dataReceived',
  Reconnecting: 'reconnecting',
  Reconnected: 'reconnected',
  Disconnected: 'disconnected',
  AudioPlaybackStatusChanged: 'audioPlaybackChanged',
  MediaDevicesError: 'mediaDevicesError',
} as const

export const Track = {
  Source: {
    Camera: 'camera', Microphone: 'microphone', ScreenShare: 'screen_share', ScreenShareAudio: 'screen_share_audio',
  },
} as const

export const ConnectionQuality = {
  Excellent: 'excellent', Good: 'good', Poor: 'poor', Lost: 'lost', Unknown: 'unknown',
} as const

export const DisconnectReason = {
  UNKNOWN_REASON: 0, CLIENT_INITIATED: 1, DUPLICATE_IDENTITY: 2, SERVER_SHUTDOWN: 3, PARTICIPANT_REMOVED: 4,
  ROOM_DELETED: 5, STATE_MISMATCH: 6, JOIN_FAILURE: 7, ROOM_CLOSED: 10, CONNECTION_TIMEOUT: 14,
} as const

export const VideoPresets = {
  h180: { resolution: { width: 320, height: 180 } },
  h360: { resolution: { width: 640, height: 360 } },
  h720: { resolution: { width: 1280, height: 720 } },
}

type Fn = (...a: unknown[]) => void

class Emitter {
  private handlers = new Map<string, Set<Fn>>()
  on(ev: string, fn: Fn) { (this.handlers.get(ev) ?? this.handlers.set(ev, new Set()).get(ev)!).add(fn); return this }
  off(ev: string, fn: Fn) { this.handlers.get(ev)?.delete(fn); return this }
  emit(ev: string, ...args: unknown[]) { this.handlers.get(ev)?.forEach((fn) => fn(...args)) }
  listenerCount(ev: string) { return this.handlers.get(ev)?.size ?? 0 }
}

export interface FakeTrack { sid: string; attach: ReturnType<typeof vi.fn>; detach: ReturnType<typeof vi.fn> }
export const fakeTrack = (sid = 't'): FakeTrack => ({ sid, attach: vi.fn(), detach: vi.fn() })

interface Pub { isMuted: boolean; track: FakeTrack | null; videoTrack?: { restartTrack: ReturnType<typeof vi.fn> } }

export class FakeParticipant {
  identity: string
  name: string
  isLocal = false
  isSpeaking = false
  connectionQuality: string = ConnectionQuality.Good
  pubs = new Map<string, Pub>()
  constructor(identity: string, name: string) { this.identity = identity; this.name = name }
  getTrackPublication(source: string) { return this.pubs.get(source) }
  setTrack(source: string, track: FakeTrack | null, isMuted = false) {
    if (track) this.pubs.set(source, { isMuted, track, videoTrack: { restartTrack: vi.fn().mockResolvedValue(undefined) } })
    else this.pubs.delete(source)
  }
  get isMicrophoneEnabled() { const p = this.pubs.get(Track.Source.Microphone); return !!p && !p.isMuted }
  get isCameraEnabled() { const p = this.pubs.get(Track.Source.Camera); return !!p && !p.isMuted }
  get isScreenShareEnabled() { return this.pubs.has(Track.Source.ScreenShare) }
}

export class FakeLocalParticipant extends FakeParticipant {
  isLocal = true
  publishData = vi.fn().mockResolvedValue(undefined)
  setMicrophoneEnabled = vi.fn(async (on: boolean) => { this.setTrack(Track.Source.Microphone, on ? fakeTrack('mic') : null) })
  setCameraEnabled = vi.fn(async (on: boolean) => { this.setTrack(Track.Source.Camera, on ? fakeTrack('cam') : null) })
  setScreenShareEnabled = vi.fn(async (on: boolean) => { this.setTrack(Track.Source.ScreenShare, on ? fakeTrack('scr') : null) })
}

export class Room extends Emitter {
  static instances: Room[] = []
  static getLocalDevices = vi.fn(async () => [] as MediaDeviceInfo[])
  options: unknown
  localParticipant = new FakeLocalParticipant('me', 'Tôi')
  remoteParticipants = new Map<string, FakeParticipant>()
  canPlaybackAudio = true
  connect = vi.fn().mockResolvedValue(undefined)
  disconnect = vi.fn().mockResolvedValue(undefined)
  startAudio = vi.fn().mockResolvedValue(undefined)
  switchActiveDevice = vi.fn().mockResolvedValue(true)
  constructor(options?: unknown) { super(); this.options = options; Room.instances.push(this) }

  /** Test helper: a person arrives. */
  join(identity: string, name: string): FakeParticipant {
    const p = new FakeParticipant(identity, name)
    this.remoteParticipants.set(identity, p)
    this.emit(RoomEvent.ParticipantConnected, p)
    return p
  }
  leave(identity: string) {
    const p = this.remoteParticipants.get(identity)
    this.remoteParticipants.delete(identity)
    if (p) this.emit(RoomEvent.ParticipantDisconnected, p)
  }
  /** Test helper: a data message from someone. */
  data(from: FakeParticipant, topic: string, body: object) {
    this.emit(RoomEvent.DataReceived, new TextEncoder().encode(JSON.stringify(body)), from, 0, topic)
  }
}

const localMedia = () => ({ stop: vi.fn(), mediaStreamTrack: {}, attach: vi.fn(), detach: vi.fn() })
export const createLocalVideoTrack = vi.fn(async () => localMedia())
export const createLocalAudioTrack = vi.fn(async () => localMedia())

export const livekitMock = {
  Room, RoomEvent, Track, ConnectionQuality, DisconnectReason, VideoPresets, createLocalVideoTrack, createLocalAudioTrack,
}
