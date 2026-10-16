import {
  ConnectionQuality,
  DisconnectReason,
  Room,
  RoomEvent,
  Track,
  VideoPresets,
  type Participant,
} from 'livekit-client'
import {
  HAND_AUTO_LOWER_MS,
  HAND_TOPIC,
  REACTION_MS,
  REACTION_TOPIC,
  SHARE_TOPIC,
  EMPTY_SNAPSHOT,
  encodeMessage,
  parseHand,
  parseReaction,
  parseShare,
  type SignalQuality,
} from './call-model'
import { playChime } from './call-sound'

/**
 * The one place that speaks to LiveKit. It wraps a `Room`, listens to its
 * events and keeps a small immutable snapshot that React reads through
 * `useSyncExternalStore`, so a screen never touches the SDK. Importing this
 * file loads the SDK: only the call code imports it, and only dynamically.
 *
 * Raising a hand and reactions travel as data messages (topics `nexus.hand`,
 * `nexus.reaction`). They could ride participant attributes, but the entry
 * ticket does not allow updating one's own metadata, so a hand is re-sent to
 * whoever joins later instead.
 */

export type LinkState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'
/** Why the room is gone: you left, the call ended, the server removed you (lost access), or the link broke for good. */
export type CallEnd = 'left' | 'ended' | 'removed' | 'failed'
export type MediaProblem = 'mic' | 'camera' | 'screen'

/** What a tile attaches to a `<video>`/`<audio>` element. */
export type MediaTrack = Track

export interface ParticipantView {
  /** The user id: used to match people and pin a tile, never shown. */
  id: string
  name: string
  isLocal: boolean
  speaking: boolean
  micOn: boolean
  camOn: boolean
  sharing: boolean
  quality: SignalQuality
  /** 1 for the first raised hand, 2 for the next; null when the hand is down. */
  handRank: number | null
  /** When the hand went up (ms), for "Giơ tay lúc 14:08". */
  handAt: number | null
  camera: MediaTrack | null
  screen: MediaTrack | null
}

export interface FloatingReaction { id: number; emoji: string; name: string }

export interface CallNotice { kind: 'share-replaced'; name: string }

export interface CallSnapshot {
  link: LinkState
  end: CallEnd | null
  /** You first, then everyone else in the order they joined. */
  participants: ParticipantView[]
  presenterId: string | null
  /** The one to show large in the speaker layout and the minimised tile. */
  speakerId: string | null
  /** Remote sound to play: microphones, and the sound of a shared tab. */
  audio: { key: string; track: MediaTrack }[]
  /** The browser refuses to play sound until a tap (Safari). */
  audioBlocked: boolean
  micOn: boolean
  camOn: boolean
  sharing: boolean
  handUp: boolean
  reactions: FloatingReaction[]
  mediaProblem: MediaProblem | null
  notice: CallNotice | null
  /** Most people seen in the room at once, for the summary. */
  peak: number
  facing: 'user' | 'environment'
}

export interface CallRoomOptions {
  selfId: string
  selfName: string
  micId?: string
  camId?: string
  speakerId?: string
}

const UNKNOWN_NAME = 'Người tham gia'

function qualityOf(q: ConnectionQuality | undefined): SignalQuality {
  if (q === ConnectionQuality.Poor || q === ConnectionQuality.Lost) return 'poor'
  if (q === ConnectionQuality.Unknown || q === undefined) return 'unknown'
  return 'good'
}

function endOf(reason: DisconnectReason | undefined): CallEnd {
  switch (reason) {
    case DisconnectReason.CLIENT_INITIATED: return 'left'
    case DisconnectReason.PARTICIPANT_REMOVED: return 'removed'
    case DisconnectReason.ROOM_DELETED:
    case DisconnectReason.ROOM_CLOSED:
    case DisconnectReason.SERVER_SHUTDOWN:
    case DisconnectReason.DUPLICATE_IDENTITY:
      return 'ended'
    default: return 'failed'
  }
}

/** Every room event that can change what the screen shows. */
const REFRESH_EVENTS = [
  RoomEvent.ParticipantConnected, RoomEvent.ParticipantDisconnected,
  RoomEvent.TrackSubscribed, RoomEvent.TrackUnsubscribed, RoomEvent.TrackPublished, RoomEvent.TrackUnpublished,
  RoomEvent.TrackMuted, RoomEvent.TrackUnmuted, RoomEvent.LocalTrackPublished, RoomEvent.LocalTrackUnpublished,
  RoomEvent.ConnectionQualityChanged, RoomEvent.ParticipantNameChanged,
] as const

export class CallRoom {
  readonly room: Room
  private readonly opts: CallRoomOptions
  private snapshot: CallSnapshot = EMPTY_SNAPSHOT
  private readonly listeners = new Set<() => void>()
  private readonly hands = new Map<string, number>()
  private readonly shareSeq = new Map<string, number>()
  private shareCounter = 0
  private readonly reactionTimers = new Set<ReturnType<typeof setTimeout>>()
  private reactionId = 0
  private lastSpeakerId: string | null = null
  private activeSpeakerIds = new Set<string>()
  private localSpeaking = false
  private peak = 0
  private link: LinkState = 'connecting'
  private end: CallEnd | null = null
  private leaving = false
  private problem: MediaProblem | null = null
  private notice: CallNotice | null = null
  private floating: FloatingReaction[] = []
  private facing: 'user' | 'environment' = 'user'
  private holdTalking = false
  private spokeSinceRaise = false
  private lowerTimer: ReturnType<typeof setTimeout> | null = null
  private readonly bound: [string, (...a: unknown[]) => void][] = []

  constructor(room: Room, opts: CallRoomOptions) {
    this.room = room
    this.opts = opts
    for (const ev of REFRESH_EVENTS) this.listen(ev, () => this.refresh())
    this.listen(RoomEvent.ParticipantConnected, (p) => this.onJoined(p as Participant))
    this.listen(RoomEvent.ActiveSpeakersChanged, (s) => this.onSpeakers(s as Participant[]))
    this.listen(RoomEvent.DataReceived, (payload, from, _kind, topic) =>
      this.onData(payload as Uint8Array, from as Participant | undefined, topic as string | undefined))
    this.listen(RoomEvent.Reconnecting, () => this.setLink('reconnecting'))
    this.listen(RoomEvent.Reconnected, () => this.setLink('connected'))
    this.listen(RoomEvent.Disconnected, (reason) => this.onDisconnected(reason as DisconnectReason | undefined))
    this.listen(RoomEvent.AudioPlaybackStatusChanged, () => this.refresh())
    this.listen(RoomEvent.MediaDevicesError, (err) => this.onMediaError(err))
    this.refresh()
  }

  // --- external store ---------------------------------------------------

  subscribe = (cb: () => void): (() => void) => {
    this.listeners.add(cb)
    return () => { this.listeners.delete(cb) }
  }

  getSnapshot = (): CallSnapshot => this.snapshot

  // --- life cycle -------------------------------------------------------

  /** Opens the room, then turns on whatever the person chose to start with. A failure to connect throws. */
  async connect(url: string, token: string, start: { micOn: boolean; camOn: boolean }): Promise<void> {
    this.setLink('connecting')
    await this.room.connect(url, token)
    this.setLink('connected')
    await this.enableMedia(start)
  }

  private async enableMedia(start: { micOn: boolean; camOn: boolean }): Promise<void> {
    const lp = this.room.localParticipant
    // The two are tried apart: a camera that is blocked must not keep the microphone off.
    if (start.micOn) await lp.setMicrophoneEnabled(true).catch(() => this.flag('mic'))
    if (start.camOn) await lp.setCameraEnabled(true).catch(() => this.flag('camera'))
    this.refresh()
  }

  async disconnect(): Promise<void> {
    this.leaving = true
    this.stopTimers()
    await this.room.disconnect()
    if (!this.end) { this.end = 'left'; this.setLink('disconnected') }
  }

  dispose(): void {
    this.stopTimers()
    for (const [ev, fn] of this.bound) (this.room as unknown as Emitter).off(ev, fn)
    this.bound.length = 0
    this.listeners.clear()
  }

  // --- what you do ------------------------------------------------------

  async toggleMic(): Promise<void> {
    const lp = this.room.localParticipant
    await lp.setMicrophoneEnabled(!lp.isMicrophoneEnabled).catch(() => this.flag('mic'))
    this.refresh()
  }

  async toggleCamera(): Promise<void> {
    const lp = this.room.localParticipant
    await lp.setCameraEnabled(!lp.isCameraEnabled).catch(() => this.flag('camera'))
    this.refresh()
  }

  /** Space held: speak while the microphone is off, mute again on release. */
  async holdToTalk(down: boolean): Promise<void> {
    const lp = this.room.localParticipant
    if (down) {
      if (lp.isMicrophoneEnabled || this.holdTalking) return
      this.holdTalking = true
      await lp.setMicrophoneEnabled(true).catch(() => { this.holdTalking = false; this.flag('mic') })
    } else if (this.holdTalking) {
      this.holdTalking = false
      await lp.setMicrophoneEnabled(false).catch(() => {})
    }
    this.refresh()
  }

  async toggleHand(): Promise<void> {
    if (this.hands.has(this.opts.selfId)) await this.setHand(false)
    else await this.setHand(true)
  }

  private async setHand(raised: boolean): Promise<void> {
    const at = Date.now()
    if (raised) { this.hands.set(this.opts.selfId, at); this.spokeSinceRaise = false }
    else { this.hands.delete(this.opts.selfId); this.clearLowerTimer() }
    this.refresh()
    await this.send(HAND_TOPIC, { raised, at }, true)
  }

  /** Floats the reaction for everyone, you included. Lossy: a dropped one is not worth re-sending. */
  async react(emoji: string): Promise<void> {
    this.addReaction(emoji, this.opts.selfName)
    await this.send(REACTION_TOPIC, { emoji }, false)
  }

  /** Starts presenting. The caller asks "Thay người đang trình bày?" first when someone else is on. */
  async startShare(): Promise<void> {
    const lp = this.room.localParticipant
    try {
      await lp.setScreenShareEnabled(true, { audio: true, contentHint: 'detail' })
    } catch {
      // Cancelling the browser's picker is not a fault; a blocked or unsupported capture is.
      if (!lp.isScreenShareEnabled) this.flag('screen')
      this.refresh()
      return
    }
    this.refresh()
    await this.send(SHARE_TOPIC, { action: 'start' }, true)
  }

  async stopShare(): Promise<void> {
    await this.room.localParticipant.setScreenShareEnabled(false).catch(() => {})
    this.refresh()
  }

  /** Front and back camera on a phone. */
  async flipCamera(): Promise<void> {
    const next = this.facing === 'user' ? 'environment' : 'user'
    const track = this.room.localParticipant.getTrackPublication(Track.Source.Camera)?.videoTrack
    if (!track) return
    try {
      await track.restartTrack({ facingMode: next })
      this.facing = next
    } catch {
      this.flag('camera')
    }
    this.refresh()
  }

  async useDevice(kind: MediaDeviceKind, deviceId: string): Promise<void> {
    await this.room.switchActiveDevice(kind, deviceId).catch(() => {})
    this.refresh()
  }

  /** Tapping to allow sound (Safari holds it back until a gesture). */
  async enableAudio(): Promise<void> {
    await this.room.startAudio().catch(() => {})
    this.refresh()
  }

  dismissNotice(): void {
    if (!this.notice) return
    this.notice = null
    this.refresh()
  }

  dismissProblem(): void {
    if (!this.problem) return
    this.problem = null
    this.refresh()
  }

  // --- events -----------------------------------------------------------

  private listen(event: string, fn: (...a: unknown[]) => void): void {
    ;(this.room as unknown as Emitter).on(event, fn)
    this.bound.push([event, fn])
  }

  private onJoined(p: Participant): void {
    // Someone arrives after you raised your hand: they have not seen it, so tell them alone.
    const at = this.hands.get(this.opts.selfId)
    if (at !== undefined && p.identity) {
      void this.send(HAND_TOPIC, { raised: true, at }, true, [p.identity])
    }
  }

  private onSpeakers(speakers: Participant[]): void {
    const was = this.localSpeaking
    this.activeSpeakerIds = new Set(speakers.map((s) => s.identity))
    const first = speakers.find((s) => !s.isLocal)
    if (first) this.lastSpeakerId = first.identity
    const now = speakers.some((s) => s.isLocal)
    this.localSpeaking = now
    if (this.hands.has(this.opts.selfId)) {
      if (now) { this.spokeSinceRaise = true; this.clearLowerTimer() }
      else if (was && this.spokeSinceRaise) this.armLowerTimer()
    }
    this.refresh()
  }

  private armLowerTimer(): void {
    this.clearLowerTimer()
    this.lowerTimer = setTimeout(() => {
      this.lowerTimer = null
      if (this.hands.has(this.opts.selfId)) void this.setHand(false)
    }, HAND_AUTO_LOWER_MS)
  }

  private clearLowerTimer(): void {
    if (this.lowerTimer) { clearTimeout(this.lowerTimer); this.lowerTimer = null }
  }

  private onData(payload: Uint8Array, from: Participant | undefined, topic: string | undefined): void {
    const sender = from?.identity
    if (!sender || sender === this.opts.selfId) return
    if (topic === HAND_TOPIC) {
      const m = parseHand(payload)
      if (!m) return
      if (m.raised) {
        const fresh = !this.hands.has(sender)
        this.hands.set(sender, m.at)
        if (fresh) void playChime()
      } else this.hands.delete(sender)
      this.refresh()
    } else if (topic === REACTION_TOPIC) {
      const m = parseReaction(payload)
      if (m) this.addReaction(m.emoji, from?.name || UNKNOWN_NAME)
    } else if (topic === SHARE_TOPIC) {
      if (parseShare(payload) && this.room.localParticipant.isScreenShareEnabled) {
        this.notice = { kind: 'share-replaced', name: from?.name || UNKNOWN_NAME }
        void this.stopShare()
      }
    }
  }

  private addReaction(emoji: string, name: string): void {
    const item: FloatingReaction = { id: ++this.reactionId, emoji, name }
    this.floating = [...this.floating, item]
    const timer = setTimeout(() => {
      this.reactionTimers.delete(timer)
      this.floating = this.floating.filter((r) => r.id !== item.id)
      this.refresh()
    }, REACTION_MS)
    this.reactionTimers.add(timer)
    this.refresh()
  }

  private onDisconnected(reason: DisconnectReason | undefined): void {
    this.end = this.leaving ? 'left' : endOf(reason)
    this.stopTimers()
    this.setLink('disconnected')
  }

  private onMediaError(err: unknown): void {
    const kind = (err as { kind?: string } | null)?.kind
    this.flag(kind === 'videoinput' ? 'camera' : 'mic')
  }

  private flag(p: MediaProblem): void {
    this.problem = p
    this.refresh()
  }

  private setLink(link: LinkState): void {
    this.link = link
    this.refresh()
  }

  private stopTimers(): void {
    this.clearLowerTimer()
    for (const t of this.reactionTimers) clearTimeout(t)
    this.reactionTimers.clear()
  }

  private async send(topic: string, body: object, reliable: boolean, to?: string[]): Promise<void> {
    try {
      await this.room.localParticipant.publishData(encodeMessage(body), {
        reliable, topic, ...(to ? { destinationIdentities: to } : {}),
      })
    } catch {
      /* a data message lost to a dropped link is not worth an error; the reconnect brings the room back */
    }
  }

  // --- snapshot ---------------------------------------------------------

  private viewOf(p: Participant, isLocal: boolean): ParticipantView {
    const camPub = p.getTrackPublication(Track.Source.Camera)
    const micPub = p.getTrackPublication(Track.Source.Microphone)
    const screenPub = p.getTrackPublication(Track.Source.ScreenShare)
    const camera = camPub && !camPub.isMuted ? camPub.track ?? null : null
    const screen = screenPub?.track ?? null
    const id = isLocal ? this.opts.selfId : p.identity
    const rank = this.rankOf(id)
    return {
      id,
      name: isLocal ? this.opts.selfName : p.name || UNKNOWN_NAME,
      isLocal,
      speaking: isLocal ? this.localSpeaking : this.activeSpeakerIds.has(p.identity),
      micOn: !!micPub && !micPub.isMuted,
      camOn: !!camera,
      sharing: !!screen,
      quality: qualityOf(p.connectionQuality),
      handRank: rank,
      handAt: this.hands.get(id) ?? null,
      camera,
      screen,
    }
  }

  private rankOf(id: string): number | null {
    const at = this.hands.get(id)
    if (at === undefined) return null
    let rank = 1
    for (const [other, t] of this.hands) {
      if (other !== id && (t < at || (t === at && other < id))) rank++
    }
    return rank
  }

  private refresh(): void {
    const lp = this.room.localParticipant
    const views: ParticipantView[] = [this.viewOf(lp, true)]
    for (const p of this.room.remoteParticipants.values()) views.push(this.viewOf(p, false))
    this.peak = Math.max(this.peak, views.length)

    // One presenter at a time: the one who started last.
    const sharingNow = new Set(views.filter((v) => v.sharing).map((v) => v.id))
    for (const id of [...this.shareSeq.keys()]) if (!sharingNow.has(id)) this.shareSeq.delete(id)
    for (const id of sharingNow) if (!this.shareSeq.has(id)) this.shareSeq.set(id, ++this.shareCounter)
    let presenterId: string | null = null
    let best = -1
    for (const [id, seq] of this.shareSeq) if (seq > best) { best = seq; presenterId = id }

    const remotes = views.filter((v) => !v.isLocal)
    const loud = remotes.find((v) => v.speaking)
    const speakerId = loud?.id ?? (remotes.some((v) => v.id === this.lastSpeakerId) ? this.lastSpeakerId : remotes[0]?.id ?? null)

    const audio: { key: string; track: MediaTrack }[] = []
    for (const p of this.room.remoteParticipants.values()) {
      for (const source of [Track.Source.Microphone, Track.Source.ScreenShareAudio]) {
        const t = p.getTrackPublication(source)?.track
        if (t) audio.push({ key: `${p.identity}:${source}`, track: t })
      }
    }

    this.snapshot = {
      link: this.link,
      end: this.end,
      participants: views,
      presenterId,
      speakerId,
      audio,
      audioBlocked: !this.room.canPlaybackAudio,
      micOn: lp.isMicrophoneEnabled,
      camOn: lp.isCameraEnabled,
      sharing: lp.isScreenShareEnabled,
      handUp: this.hands.has(this.opts.selfId),
      reactions: this.floating,
      mediaProblem: this.problem,
      notice: this.notice,
      peak: this.peak,
      facing: this.facing,
    }
    for (const cb of this.listeners) cb()
  }
}

interface Emitter {
  on(event: string, fn: (...a: unknown[]) => void): void
  off(event: string, fn: (...a: unknown[]) => void): void
}

/** A room tuned for calls: video that adapts to the tile it is shown in, simulcast, and the devices chosen in the check. */
export function createCallRoom(opts: CallRoomOptions): CallRoom {
  const room = new Room({
    adaptiveStream: true,
    dynacast: true,
    audioCaptureDefaults: { deviceId: opts.micId, echoCancellation: true, noiseSuppression: true, autoGainControl: true },
    videoCaptureDefaults: { deviceId: opts.camId, resolution: VideoPresets.h720.resolution },
    publishDefaults: { simulcast: true, videoSimulcastLayers: [VideoPresets.h180, VideoPresets.h360] },
    audioOutput: { deviceId: opts.speakerId },
  })
  return new CallRoom(room, opts)
}
