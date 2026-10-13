import { useCallback, useEffect, useRef, useState } from 'react'
import {
  ChevronDown, Hand, MessageSquare, Mic, MicOff, MoreHorizontal, PhoneOff, Settings, SwitchCamera, Users, Video, VideoOff, Volume2,
} from 'lucide-react'
import type { Call } from '../../api/calls'
import type { CallRoom, CallSnapshot, ParticipantView } from '../../lib/call-room'
import { pageCount, pageOf, REACTIONS } from '../../lib/call-model'
import { useAuthStore } from '../../stores/auth.store'
import { useCallStore, type CallSession } from '../../stores/call.store'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useElapsed } from '../../hooks/useElapsed'
import { useMessagesSince } from '../../hooks/useCalls'
import { usePeople } from '../../hooks/usePeople'
import { Pressable } from '../primitives'
import { CallSidePanel } from './CallSidePanel'
import { DeviceSettings } from './DeviceSettings'
import { ParticipantTile } from './ParticipantTile'
import { ReactionsLayer } from './ReactionsLayer'
import { AudioBlockedButton, ReconnectingOverlay } from './CallStates'
import './call-stage.css'

/** Controls hide after this long without a touch (mockup §6). */
export const HIDE_AFTER_MS = 4000
const PER_PAGE = 6
const PIP_W = 104
const PIP_H = 148
const PIP_GAP = 14

type Corner = 'tl' | 'tr' | 'bl' | 'br'

/** Your own tile, small, dragged to whichever corner is nearest when let go. */
function SelfPip({ self, avatarUrl, boundsRef }: { self: ParticipantView; avatarUrl?: string; boundsRef: React.RefObject<HTMLDivElement | null> }) {
  const [corner, setCorner] = useState<Corner>('br')
  const [drag, setDrag] = useState<{ x: number; y: number } | null>(null)
  const grab = useRef<{ dx: number; dy: number } | null>(null)

  const place = (c: Corner, w: number, h: number) => ({
    left: c.endsWith('l') ? PIP_GAP : w - PIP_W - PIP_GAP,
    top: c.startsWith('t') ? PIP_GAP : h - PIP_H - PIP_GAP,
  })

  const onDown = (e: React.PointerEvent<HTMLDivElement>) => {
    const box = boundsRef.current?.getBoundingClientRect()
    if (!box) return
    const at = place(corner, box.width, box.height)
    grab.current = { dx: e.clientX - box.left - at.left, dy: e.clientY - box.top - at.top }
    e.currentTarget.setPointerCapture?.(e.pointerId)
  }
  const onMove = (e: React.PointerEvent) => {
    const box = boundsRef.current?.getBoundingClientRect()
    if (!grab.current || !box) return
    setDrag({ x: e.clientX - box.left - grab.current.dx, y: e.clientY - box.top - grab.current.dy })
  }
  const onUp = () => {
    const box = boundsRef.current?.getBoundingClientRect()
    if (grab.current && drag && box) {
      const right = drag.x + PIP_W / 2 > box.width / 2
      const bottom = drag.y + PIP_H / 2 > box.height / 2
      setCorner(`${bottom ? 'b' : 't'}${right ? 'r' : 'l'}` as Corner)
    }
    grab.current = null
    setDrag(null)
  }

  const box = boundsRef.current?.getBoundingClientRect()
  const at = drag ?? (box ? { x: place(corner, box.width, box.height).left, y: place(corner, box.width, box.height).top } : { x: PIP_GAP, y: PIP_GAP })
  return (
    <div
      className="cs-pself"
      data-corner={corner}
      style={{ left: at.x, top: at.y, transition: drag ? 'none' : undefined }}
      onPointerDown={onDown}
      onPointerMove={onMove}
      onPointerUp={onUp}
      onPointerCancel={onUp}
    >
      <ParticipantTile person={self} avatarSize={40} avatarUrl={avatarUrl} compact />
    </div>
  )
}

interface Props {
  callRoom: CallRoom
  snapshot: CallSnapshot
  session: CallSession
  call: Call | undefined
  onLeave: () => void
  onMinimize: () => void
}

/** The call on a phone (mockup §6): full screen, round controls, a "Thêm" sheet, and no Trình bày. */
export function PhoneCallView({ callRoom, snapshot, session, call, onLeave, onMinimize }: Props) {
  const store = useCallStore()
  const user = useAuthStore((s) => s.user)
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const clock = useElapsed(session.startedAt)
  const newMessages = useMessagesSince(session.channelId, store.chatSeenAt)
  const [sheet, setSheet] = useState(false)
  const [hidden, setHidden] = useState(false)
  const [touches, setTouches] = useState(0)
  const [pageAt, setPageAt] = useState(0)
  const stageRef = useRef<HTMLDivElement>(null)
  const { panel, markChatSeen } = store

  const { participants } = snapshot
  const self = participants.find((p) => p.isLocal) ?? participants[0]
  const others = participants.filter((p) => !p.isLocal)
  const presenter = participants.find((p) => p.id === snapshot.presenterId && p.screen)
  const handsUp = participants.filter((p) => p.handRank !== null).length
  const avatarOf = (p: ParticipantView) => people.byUserId.get(p.id)?.avatarUrl

  useEffect(() => {
    if (panel === 'chat') markChatSeen()
  }, [panel, newMessages, markChatSeen])

  // Controls fade 4 s after the last touch, unless the sheet or a panel is open.
  useEffect(() => {
    if (sheet || panel || hidden) return
    const t = setTimeout(() => setHidden(true), HIDE_AFTER_MS)
    return () => clearTimeout(t)
  }, [sheet, panel, hidden, touches])

  const onStageTap = useCallback(() => {
    setHidden((h) => !h)
    setTouches((n) => n + 1)
  }, [])

  const subtitle = session.isDm ? clock : `${clock} · ${participants.length} người`
  const group = participants.length > 2 && !presenter
  const pages = pageCount(participants.length, PER_PAGE)

  const reveal = () => { setHidden(false); setTouches((n) => n + 1) }

  if (!self) return null

  let stage
  if (presenter) {
    stage = (
      <div className="cs-pstage" ref={stageRef}>
        <ParticipantTile person={presenter} show="screen" />
        <SelfPip self={self} avatarUrl={avatarOf(self)} boundsRef={stageRef} />
      </div>
    )
  } else if (!group) {
    const main = others[0] ?? self
    stage = (
      <div className="cs-pstage" ref={stageRef}>
        <ParticipantTile person={main} avatarSize={64} avatarUrl={avatarOf(main)} compact speakerMark />
        {main !== self && <SelfPip self={self} avatarUrl={avatarOf(self)} boundsRef={stageRef} />}
      </div>
    )
  } else {
    stage = (
      <div className="cs-pgrid-wrap">
        <div
          className="cs-pgrid-pages"
          onScroll={(e) => setPageAt(Math.round(e.currentTarget.scrollLeft / Math.max(1, e.currentTarget.clientWidth)))}
        >
          {Array.from({ length: pages }, (_, i) => (
            <div key={i} className="cs-pgrid">
              {pageOf(participants, i, PER_PAGE).map((p) => (
                <ParticipantTile key={p.id} person={p} avatarSize={52} avatarUrl={avatarOf(p)} compact />
              ))}
            </div>
          ))}
        </div>
        {pages > 1 && (
          <div className="cs-dots" aria-label={`Trang ${pageAt + 1} trên ${pages}`}>
            {Array.from({ length: pages }, (_, i) => <i key={i} className={i === pageAt ? 'cs-dot-on' : ''} />)}
          </div>
        )}
      </div>
    )
  }

  const { micOn, camOn, handUp } = snapshot
  return (
    <div className={`cs-phone ${hidden ? 'cs-phone-hidden' : ''}`} data-testid="phone-call">
      <div className="cs-ptop">
        <Pressable className="cs-pib" aria-label="Thu nhỏ cuộc gọi" onClick={onMinimize}><ChevronDown className="cs-icon" /></Pressable>
        <span className="cs-ti"><b>{session.isDm ? (others[0]?.name ?? session.title) : session.title}</b><small>{subtitle}</small></span>
        <Pressable className="cs-pib" aria-label="Loa ngoài" onClick={() => store.setDevicesOpen(true)}><Volume2 className="cs-icon" /></Pressable>
      </div>

      {/* A tap anywhere on the picture shows or hides the bars. */}
      <div className="cs-tap" onClick={onStageTap} role="presentation" style={{ display: 'grid', minHeight: 0, minWidth: 0 }}>
        {stage}
      </div>

      <div className="cs-pctl" onFocus={reveal}>
        <Pressable className={`cs-btn ${micOn ? '' : 'cs-btn-off'}`} aria-label={micOn ? 'Tắt micro' : 'Bật micro'} onClick={() => void callRoom.toggleMic()}>
          {micOn ? <Mic className="cs-icon" /> : <MicOff className="cs-icon" />}
        </Pressable>
        <Pressable className={`cs-btn ${camOn ? '' : 'cs-btn-off'}`} aria-label={camOn ? 'Tắt camera' : 'Bật camera'} onClick={() => void callRoom.toggleCamera()}>
          {camOn ? <Video className="cs-icon" /> : <VideoOff className="cs-icon" />}
        </Pressable>
        <Pressable className="cs-btn" aria-label="Đổi camera trước/sau" onClick={() => void callRoom.flipCamera()}><SwitchCamera className="cs-icon" /></Pressable>
        <Pressable className="cs-btn" aria-label="Thêm" aria-expanded={sheet} onClick={() => { setSheet((s) => !s); reveal() }}>
          <MoreHorizontal className="cs-icon" />
          {(newMessages > 0 || handsUp > 0) && <span className="cs-badge">{newMessages + handsUp}</span>}
        </Pressable>
        <Pressable className="cs-btn cs-btn-end" aria-label="Rời cuộc gọi" onClick={onLeave}><PhoneOff className="cs-icon" /></Pressable>
      </div>

      <ReactionsLayer reactions={snapshot.reactions} />

      {sheet && (
        <>
          <Pressable className="cs-sheet-scrim" aria-label="Đóng" onClick={() => setSheet(false)} />
          <div className="cs-sheet" role="dialog" aria-label="Thêm">
            <span className="cs-grab" aria-hidden="true" />
            <div className="cs-quick">
              {REACTIONS.map((r) => (
                <Pressable key={r.emoji} aria-label={r.label} onClick={() => { void callRoom.react(r.emoji); setSheet(false) }}>{r.emoji}</Pressable>
              ))}
            </div>
            <Pressable className="cs-item" aria-pressed={handUp} onClick={() => { void callRoom.toggleHand(); setSheet(false) }}>
              <Hand className="cs-icon" />{handUp ? 'Hạ tay' : 'Giơ tay'}
            </Pressable>
            <Pressable className="cs-item" onClick={() => { store.setPanel('chat'); setSheet(false) }}>
              <MessageSquare className="cs-icon" />Chat trong cuộc gọi
              {newMessages > 0 && <span className="cs-n2">{`${newMessages} mới`}</span>}
            </Pressable>
            <Pressable className="cs-item" onClick={() => { store.setPanel('people'); setSheet(false) }}>
              <Users className="cs-icon" />Người tham gia<span className="cs-n2">{participants.length}</span>
            </Pressable>
            <Pressable className="cs-item" onClick={() => { store.setDevicesOpen(true); setSheet(false) }}>
              <Settings className="cs-icon" />Micro, camera, loa
            </Pressable>
          </div>
        </>
      )}

      {panel && (
        <div className="cs-fullpanel">
          <CallSidePanel
            panel={panel} onPanel={store.setPanel} channelId={session.channelId} callId={session.callId}
            title={session.title} isDm={session.isDm} call={call} snapshot={snapshot}
          />
        </div>
      )}

      {snapshot.audioBlocked && <AudioBlockedButton onTap={() => void callRoom.enableAudio()} />}
      {snapshot.link === 'reconnecting' && <ReconnectingOverlay onLeave={onLeave} />}
      <DeviceSettings open={store.devicesOpen} onClose={() => store.setDevicesOpen(false)} callRoom={callRoom} userId={user?.id} />
    </div>
  )
}
