import { useEffect, useState } from 'react'
import { CircleAlert, Info, Lock, Mic, MicOff, Video, VideoOff, Volume2 } from 'lucide-react'
import { MAX_CALL_PEOPLE, type Call, type CallRefusal } from '../../api/calls'
import { listDevices, type DeviceOption } from '../../lib/call-devices'
import { loadCallPrefs, saveCallPrefs, type CallPrefs } from '../../lib/call-prefs'
import { playChime } from '../../lib/call-sound'
import { Avatar, Button, Pressable, Spinner } from '../primitives'
import { DeviceSelect } from './DeviceSelect'
import { TrackVideo } from './media'
import './call-stage.css'

/** Who is in the call, as one sentence: "A, B và 1 người đang trong cuộc gọi". */
export function whoIsIn(names: string[]): string {
  if (names.length === 0) return 'Chưa có ai trong cuộc gọi'
  if (names.length === 1) return `${names[0]} đang trong cuộc gọi`
  if (names.length === 2) return `${names[0]} và ${names[1]} đang trong cuộc gọi`
  return `${names[0]}, ${names[1]} và ${names.length - 2} người đang trong cuộc gọi`
}

interface LocalTrack {
  mediaStreamTrack: MediaStreamTrack
  stop: () => void
  attach: (el?: HTMLMediaElement) => HTMLMediaElement
  detach: (el?: HTMLMediaElement) => HTMLMediaElement[]
}

const BARS = 5
const BAR_MAX = 18
const BAR_MIN = 4

/** Five bars from the live microphone level: the first lights at a whisper, the last needs a voice. */
export function barHeights(level: number): number[] {
  return Array.from({ length: BARS }, (_, i) => {
    const on = Math.max(0, Math.min(1, level * 1.6 - i * 0.18))
    return Math.round(BAR_MIN + on * (BAR_MAX - BAR_MIN))
  })
}

interface Props {
  call: Call | undefined
  selfId: string | undefined
  selfName: string
  refusal: CallRefusal | null
  joining: boolean
  onJoin: (prefs: CallPrefs, audio: boolean, video: boolean) => void
  onCancel: () => void
}

/**
 * The check before a group call (mockup §2): camera preview, microphone level,
 * device choices, then "Tham gia ngay". The microphone and camera are released
 * before joining so the room can take them.
 */
export function PreJoin({ call, selfId, selfName, refusal, joining, onJoin, onCancel }: Props) {
  const [prefs, setPrefs] = useState<CallPrefs>(() => loadCallPrefs(selfId))
  const [denied, setDenied] = useState(false)
  const [video, setVideo] = useState<LocalTrack | null>(null)
  const [level, setLevel] = useState(0)
  const [devices, setDevices] = useState<{ mic: DeviceOption[]; cam: DeviceOption[]; out: DeviceOption[] }>({ mic: [], cam: [], out: [] })
  const audioCall = call?.kind === 'audio'
  const videoOn = prefs.camOn && !audioCall

  // Camera preview: opened while the camera is on, released the moment it is off or the screen goes.
  useEffect(() => {
    if (!videoOn) { setVideo(null); return }
    let track: LocalTrack | null = null
    let live = true
    void (async () => {
      try {
        const { createLocalVideoTrack } = await import('livekit-client')
        const t = (await createLocalVideoTrack({ deviceId: prefs.camId })) as unknown as LocalTrack
        if (!live) { t.stop(); return }
        track = t
        setVideo(t)
        setDenied(false)
      } catch {
        if (live) setDenied(true)
      }
    })()
    return () => { live = false; track?.stop(); setVideo(null) }
  }, [videoOn, prefs.camId])

  // Microphone level: an analyser on the live track, read ten times a second.
  useEffect(() => {
    if (!prefs.micOn) { setLevel(0); return }
    let live = true
    let stop: (() => void) | null = null
    void (async () => {
      try {
        const { createLocalAudioTrack } = await import('livekit-client')
        const t = (await createLocalAudioTrack({ deviceId: prefs.micId })) as unknown as LocalTrack
        if (!live) { t.stop(); return }
        setDenied(false)
        const Ctx = window.AudioContext
        if (!Ctx) { stop = () => t.stop(); return }
        const ctx = new Ctx()
        const src = ctx.createMediaStreamSource(new MediaStream([t.mediaStreamTrack]))
        const an = ctx.createAnalyser()
        an.fftSize = 512
        src.connect(an)
        const buf = new Uint8Array(an.fftSize)
        const timer = setInterval(() => {
          an.getByteTimeDomainData(buf)
          let sum = 0
          for (const v of buf) sum += ((v - 128) / 128) ** 2
          setLevel(Math.sqrt(sum / buf.length))
        }, 100)
        stop = () => { clearInterval(timer); t.stop(); void ctx.close() }
      } catch {
        if (live) setDenied(true)
      }
    })()
    return () => { live = false; stop?.(); setLevel(0) }
  }, [prefs.micOn, prefs.micId])

  // Device names come once the browser has been asked; before that the lists are empty.
  useEffect(() => {
    let live = true
    void Promise.all([listDevices('audioinput', true), listDevices('videoinput', !audioCall), listDevices('audiooutput', false)])
      .then(([mic, cam, out]) => { if (live) setDevices({ mic, cam, out }) })
    return () => { live = false }
  }, [audioCall])

  const set = (patch: Partial<CallPrefs>) => setPrefs((p) => ({ ...p, ...patch }))

  const others = (call?.participants ?? []).filter((p) => p.user_id !== selfId)
  const names = others.map((p) => p.display_name?.trim() || 'Một người')
  const full = refusal === 'call_full' || (call?.taker_count ?? others.length) >= MAX_CALL_PEOPLE

  const join = () => {
    saveCallPrefs(selfId, prefs)
    setVideo(null)
    onJoin(prefs, prefs.micOn, videoOn)
  }

  const speakerSupported = typeof HTMLMediaElement !== 'undefined' && 'setSinkId' in HTMLMediaElement.prototype

  return (
    <div className="cs-prejoin-page">
      <div className="cs-prejoin">
        <div className="cs-preview" data-testid="preview">
          {videoOn && video ? <TrackVideo track={video as never} mirror /> : (
            <div className="cs-camoff">
              <Avatar name={selfName} hueKey={selfId} size={64} className="cs-big-avatar" />
              {audioCall ? 'Cuộc gọi thoại' : 'Camera đang tắt'}
            </div>
          )}
          {prefs.micOn && (
            <div className="cs-meter" aria-hidden="true" data-testid="mic-meter">
              {barHeights(level).map((h, i) => <i key={i} style={{ height: h }} />)}
            </div>
          )}
          <div className="cs-preview-controls">
            <Pressable
              className={`cs-btn ${prefs.micOn ? '' : 'cs-btn-off'}`}
              aria-label={prefs.micOn ? 'Tắt micro' : 'Bật micro'}
              onClick={() => set({ micOn: !prefs.micOn })}
            >
              {prefs.micOn ? <Mic className="cs-icon" /> : <MicOff className="cs-icon" />}
            </Pressable>
            {!audioCall && (
              <Pressable
                className={`cs-btn ${prefs.camOn ? '' : 'cs-btn-off'}`}
                aria-label={prefs.camOn ? 'Tắt camera' : 'Bật camera'}
                onClick={() => set({ camOn: !prefs.camOn })}
              >
                {prefs.camOn ? <Video className="cs-icon" /> : <VideoOff className="cs-icon" />}
              </Pressable>
            )}
          </div>
        </div>

        <div className="cs-pj">
          <h2>Sẵn sàng tham gia?</h2>
          <div className="cs-who">
            {others.length > 0 && (
              <span className="cs-faces">
                {others.slice(0, 3).map((p) => <Avatar key={p.user_id} name={p.display_name || 'Một người'} hueKey={p.user_id} size={24} src={p.avatar_url} />)}
              </span>
            )}
            <span>{whoIsIn(names)}</span>
          </div>

          {denied && (
            <div className="cs-banner cs-banner-danger" role="alert">
              <Lock className="cs-icon" aria-hidden="true" />
              <span>
                <b>Trình duyệt đang chặn camera và micro</b>
                Bấm biểu tượng ổ khoá cạnh địa chỉ trang → Camera, Micro → Cho phép, rồi tải lại. Bạn vẫn có thể vào để nghe và xem.
              </span>
            </div>
          )}
          {refusal === 'unavailable' && (
            <div className="cs-banner cs-banner-info" role="alert">
              <Info className="cs-icon" aria-hidden="true" />
              <span><b>Cuộc gọi chưa dùng được lúc này</b>Máy chủ cuộc gọi đang bận hoặc chưa bật. Thử lại sau ít phút.</span>
            </div>
          )}
          {refusal === 'unreachable' && (
            <div className="cs-banner cs-banner-danger" role="alert">
              <CircleAlert className="cs-icon" aria-hidden="true" />
              <span><b>Chưa vào được cuộc gọi</b>Kiểm tra kết nối mạng rồi thử lại.</span>
            </div>
          )}

          <div className="cs-field">
            <label htmlFor="pj-mic">Micro</label>
            <DeviceSelect id="pj-mic" icon={<Mic className="cs-icon-s" />} value={prefs.micId ?? ''} options={devices.mic} disabled={devices.mic.length === 0} onChange={(id) => set({ micId: id || undefined })} />
          </div>
          {!audioCall && (
            <div className="cs-field">
              <label htmlFor="pj-cam">Camera</label>
              <DeviceSelect id="pj-cam" icon={<Video className="cs-icon-s" />} value={prefs.camId ?? ''} options={devices.cam} disabled={devices.cam.length === 0} onChange={(id) => set({ camId: id || undefined })} />
            </div>
          )}
          <div className="cs-field">
            <label htmlFor="pj-out">Loa</label>
            <div className="cs-field-row">
              <DeviceSelect id="pj-out" icon={<Volume2 className="cs-icon-s" />} value={prefs.speakerId ?? ''} options={devices.out} disabled={!speakerSupported || devices.out.length === 0} onChange={(id) => set({ speakerId: id || undefined })} />
              <Button variant="soft" size="sm" onClick={() => void playChime(prefs.speakerId)}>Thử loa</Button>
            </div>
          </div>

          <div className="cs-actions">
            {full ? (
              <>
                <Button variant="primary" className="h-12 px-6" disabled>{`Cuộc gọi đã đầy (${MAX_CALL_PEOPLE}/${MAX_CALL_PEOPLE})`}</Button>
              </>
            ) : (
              <Button variant="primary" className="h-12 px-6" onClick={join} disabled={joining}>
                {joining ? <Spinner size="sm" /> : null}Tham gia ngay
              </Button>
            )}
            <Button variant="soft" className="h-12 px-6" onClick={onCancel}>Huỷ</Button>
          </div>
          {full && <p className="cs-sub">Cuộc gọi đã có đủ 20 người. Thử lại khi có người rời.</p>}
        </div>
      </div>
    </div>
  )
}
