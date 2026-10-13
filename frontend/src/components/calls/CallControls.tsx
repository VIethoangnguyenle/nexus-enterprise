import { useRef, useState } from 'react'
import {
  Hand, LayoutGrid, MessageSquare, Mic, MicOff, Maximize, MoreHorizontal, MonitorUp, PhoneOff, Settings, Smile, Users, Video, VideoOff,
} from 'lucide-react'
import type { CallRoom, CallSnapshot } from '../../lib/call-room'
import { REACTIONS } from '../../lib/call-model'
import type { CallLayoutMode, CallPanel } from '../../stores/call.store'
import { MenuItem, MenuSeparator, Popover, Pressable } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import './call-stage.css'

const LAYOUTS: { id: CallLayoutMode; label: string }[] = [
  { id: 'auto', label: 'Tự động' },
  { id: 'speaker', label: 'Người đang nói' },
  { id: 'grid', label: 'Lưới' },
]

/** The layout menu in the top bar: Tự động / Người đang nói / Lưới. */
export function LayoutMenu({ layout, onChange }: { layout: CallLayoutMode; onChange: (l: CallLayoutMode) => void }) {
  const ref = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  return (
    <>
      <Pressable ref={ref} className="cs-btn cs-btn-sq" aria-label="Đổi bố cục" title="Bố cục" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen((o) => !o)}>
        <LayoutGrid className="cs-icon" />
      </Pressable>
      <Popover open={open} onClose={() => setOpen(false)} anchorRef={ref} placement="bottom-end" role="menu" label="Bố cục">
        {LAYOUTS.map((l) => (
          <MenuItem key={l.id} role="menuitemradio" aria-checked={layout === l.id} onClick={() => onChange(l.id)}>
            {l.label}
          </MenuItem>
        ))}
      </Popover>
    </>
  )
}

interface Props {
  callRoom: CallRoom
  snapshot: CallSnapshot
  title: string
  clock: string
  panel: CallPanel | null
  onPanel: (p: CallPanel | null) => void
  /** Messages in the conversation since the chat panel was last open. */
  newMessages: number
  /** Hands up, to badge the people button. */
  handsUp: number
  /** Someone else is presenting right now (the confirm "Thay người đang trình bày?"). */
  otherPresenter: string | null
  onLeave: () => void
  onOpenDevices: () => void
}

/** The bottom bar of the desktop stage (mockup §4): micro · camera · trình bày · giơ tay · cảm xúc | rời, with chat, people and more at the right. */
export function CallControls({
  callRoom, snapshot, title, clock, panel, onPanel, newMessages, handsUp, otherPresenter, onLeave, onOpenDevices,
}: Props) {
  const [reactOpen, setReactOpen] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [confirmShare, setConfirmShare] = useState(false)
  const moreRef = useRef<HTMLButtonElement>(null)
  const { micOn, camOn, sharing, handUp } = snapshot

  const onShare = () => {
    if (sharing) void callRoom.stopShare()
    else if (otherPresenter) setConfirmShare(true)
    else void callRoom.startShare()
  }

  const fullscreen = () => {
    if (document.fullscreenElement) void document.exitFullscreen()
    else void document.documentElement.requestFullscreen().catch(() => {})
  }

  return (
    <div className="cs-bar">
      <span className="cs-left"><span>{clock}</span>·<span>{title}</span></span>

      <Pressable
        className={`cs-btn ${micOn ? '' : 'cs-btn-off'}`}
        aria-label={micOn ? 'Tắt micro (Ctrl+D)' : 'Bật micro (Ctrl+D)'}
        aria-keyshortcuts="Control+D"
        title={micOn ? 'Tắt micro · Ctrl+D' : 'Bật micro · Ctrl+D'}
        onClick={() => void callRoom.toggleMic()}
      >
        {micOn ? <Mic className="cs-icon" /> : <MicOff className="cs-icon" />}
      </Pressable>
      <Pressable
        className={`cs-btn ${camOn ? '' : 'cs-btn-off'}`}
        aria-label={camOn ? 'Tắt camera (Ctrl+E)' : 'Bật camera (Ctrl+E)'}
        aria-keyshortcuts="Control+E"
        title={camOn ? 'Tắt camera · Ctrl+E' : 'Bật camera · Ctrl+E'}
        onClick={() => void callRoom.toggleCamera()}
      >
        {camOn ? <Video className="cs-icon" /> : <VideoOff className="cs-icon" />}
      </Pressable>
      <Pressable
        className={`cs-btn ${sharing ? 'cs-btn-on' : ''}`}
        aria-label={sharing ? 'Dừng trình bày' : 'Trình bày màn hình'}
        aria-pressed={sharing}
        title="Trình bày"
        onClick={onShare}
      >
        <MonitorUp className="cs-icon" />
      </Pressable>
      <Pressable
        className={`cs-btn ${handUp ? 'cs-btn-on' : ''}`}
        aria-label={handUp ? 'Hạ tay' : 'Giơ tay'}
        aria-pressed={handUp}
        aria-keyshortcuts="Control+Alt+H"
        title="Giơ tay · Ctrl+Alt+H"
        onClick={() => void callRoom.toggleHand()}
      >
        <Hand className="cs-icon" />
      </Pressable>
      <Pressable
        className="cs-btn"
        aria-label="Thả cảm xúc"
        aria-haspopup="menu"
        aria-expanded={reactOpen}
        onClick={() => setReactOpen((o) => !o)}
      >
        <Smile className="cs-icon" />
      </Pressable>
      <Pressable className="cs-btn cs-btn-end" aria-label="Rời cuộc gọi" onClick={onLeave}>
        <PhoneOff className="cs-icon" />
      </Pressable>

      <span className="cs-right">
        <Pressable className="cs-btn cs-btn-sq" aria-label="Chat" aria-pressed={panel === 'chat'} onClick={() => onPanel(panel === 'chat' ? null : 'chat')}>
          <MessageSquare className="cs-icon" />
          {newMessages > 0 && panel !== 'chat' && <span className="cs-badge" aria-label={`${newMessages} tin mới`}>{newMessages}</span>}
        </Pressable>
        <Pressable className="cs-btn cs-btn-sq" aria-label="Người tham gia" aria-pressed={panel === 'people'} onClick={() => onPanel(panel === 'people' ? null : 'people')}>
          <Users className="cs-icon" />
          {handsUp > 0 && <span className="cs-badge" aria-label={`${handsUp} người giơ tay`}>{handsUp}</span>}
        </Pressable>
        <Pressable ref={moreRef} className="cs-btn cs-btn-sq" aria-label="Thêm" aria-haspopup="menu" aria-expanded={moreOpen} onClick={() => setMoreOpen((o) => !o)}>
          <MoreHorizontal className="cs-icon" />
        </Pressable>
      </span>

      {reactOpen && (
        <div className="cs-react-pop" role="menu" aria-label="Cảm xúc" onKeyDown={(e) => { if (e.key === 'Escape') setReactOpen(false) }}>
          {REACTIONS.map((r) => (
            <Pressable
              key={r.emoji}
              role="menuitem"
              aria-label={r.label}
              onClick={() => { void callRoom.react(r.emoji); setReactOpen(false) }}
            >
              {r.emoji}
            </Pressable>
          ))}
        </div>
      )}

      <Popover open={moreOpen} onClose={() => setMoreOpen(false)} anchorRef={moreRef} placement="top-start" role="menu" label="Thêm">
        <MenuItem icon={<Settings size={16} />} onClick={onOpenDevices}>Micro, camera, loa</MenuItem>
        <MenuSeparator />
        <MenuItem icon={<Maximize size={16} />} onClick={fullscreen}>Toàn màn hình</MenuItem>
      </Popover>

      <ConfirmDialog
        open={confirmShare}
        onClose={() => setConfirmShare(false)}
        onConfirm={() => { setConfirmShare(false); void callRoom.startShare() }}
        title="Thay người đang trình bày?"
        description={`${otherPresenter ?? 'Một người'} đang trình bày. Nếu bạn trình bày, bản trình bày của họ sẽ dừng.`}
        confirmLabel="Trình bày"
        cancelLabel="Huỷ"
      />
    </div>
  )
}
