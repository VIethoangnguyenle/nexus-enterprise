import { useEffect, useMemo, useState } from 'react'
import { ChevronLeft, ChevronRight, Lock, Maximize2, Signal, Users } from 'lucide-react'
import type { Call } from '../../api/calls'
import type { CallRoom, CallSnapshot, ParticipantView } from '../../lib/call-room'
import { arrange } from '../../lib/call-arrange'
import { pageCount, pageOf } from '../../lib/call-model'
import { useAuthStore } from '../../stores/auth.store'
import { useCallStore, type CallSession } from '../../stores/call.store'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useElapsed } from '../../hooks/useElapsed'
import { useMessagesSince } from '../../hooks/useCalls'
import { usePeople } from '../../hooks/usePeople'
import { IconButton, Pressable } from '../primitives'
import { CallControls, LayoutMenu } from './CallControls'
import { CallSidePanel } from './CallSidePanel'
import { DeviceSettings } from './DeviceSettings'
import { ParticipantTile } from './ParticipantTile'
import { ReactionsLayer } from './ReactionsLayer'
import { AudioBlockedButton, ReconnectingOverlay } from './CallStates'
import './call-stage.css'

const PROBLEM_COPY: Record<NonNullable<CallSnapshot['mediaProblem']>, string> = {
  mic: 'Trình duyệt đang chặn micro. Bấm biểu tượng ổ khoá cạnh địa chỉ trang → Micro → Cho phép.',
  camera: 'Trình duyệt đang chặn camera. Bấm biểu tượng ổ khoá cạnh địa chỉ trang → Camera → Cho phép.',
  screen: 'Chưa trình bày được màn hình. Kiểm tra quyền chia sẻ màn hình của trình duyệt rồi thử lại.',
}

interface Props {
  callRoom: CallRoom
  snapshot: CallSnapshot
  session: CallSession
  call: Call | undefined
  onLeave: () => void
}

/** The desktop call (mockup §4 and §5): top bar, tiles or presentation, side panel, control bar. */
export function CallStage({ callRoom, snapshot, session, call, onLeave }: Props) {
  const store = useCallStore()
  const { panel, layout, pinnedId, devicesOpen } = store
  const user = useAuthStore((s) => s.user)
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const clock = useElapsed(session.startedAt)
  // The call of an event is in no conversation: no chat to read or badge.
  const hasChat = !!session.channelId
  const newMessages = useMessagesSince(session.channelId, store.chatSeenAt)
  const [page, setPage] = useState(0)
  const { markChatSeen } = store

  // Looking at the chat is reading it: the badge counts only what came after.
  useEffect(() => {
    if (panel === 'chat') markChatSeen()
  }, [panel, newMessages, markChatSeen])

  const arrangement = useMemo(
    () => arrange({ participants: snapshot.participants, presenterId: snapshot.presenterId, speakerId: snapshot.speakerId, pinnedId, mode: layout }),
    [snapshot.participants, snapshot.presenterId, snapshot.speakerId, pinnedId, layout],
  )

  const avatarOf = (p: ParticipantView) => people.byUserId.get(p.id)?.avatarUrl
  const presenter = snapshot.participants.find((p) => p.id === snapshot.presenterId && p.screen)
  const otherPresenter = presenter && !presenter.isLocal ? presenter.name : null
  const handsUp = snapshot.participants.filter((p) => p.handRank !== null).length
  const selfPoor = snapshot.participants.find((p) => p.isLocal)?.quality === 'poor'
  const pin = store.togglePin

  let body
  if (arrangement.kind === 'empty') {
    body = <div />
  } else if (arrangement.kind === 'single') {
    body = (
      <div className="cs-single-wrap">
        <ParticipantTile person={arrangement.self} longSelf avatarUrl={avatarOf(arrangement.self)} avatarSize={64} />
      </div>
    )
  } else if (arrangement.kind === 'pip') {
    body = (
      <div className="cs-single-wrap">
        <ParticipantTile person={arrangement.main} avatarUrl={avatarOf(arrangement.main)} speakerMark pinned={false} onPin={pin} />
        <div className="cs-pip-self"><ParticipantTile person={arrangement.self} avatarSize={40} avatarUrl={avatarOf(arrangement.self)} compact /></div>
      </div>
    )
  } else if (arrangement.kind === 'focus') {
    body = (
      <div className="cs-present">
        <div className="cs-slide">
          <ParticipantTile
            person={arrangement.main} show={arrangement.show} avatarUrl={avatarOf(arrangement.main)}
            speakerMark pinned={pinnedId === arrangement.main.id} onPin={arrangement.show === 'camera' ? pin : undefined}
          />
          {arrangement.show === 'screen' && (
            <IconButton className="cs-expand" aria-label="Phóng to bản trình bày" onClick={() => {
              const el = document.querySelector('.cs-slide')
              if (document.fullscreenElement) void document.exitFullscreen()
              else void (el as HTMLElement | null)?.requestFullscreen?.().catch(() => {})
            }}><Maximize2 size={18} /></IconButton>
          )}
        </div>
        <div className="cs-strip">
          {arrangement.strip.map((p) => (
            <ParticipantTile key={p.id} person={p} avatarSize={40} avatarUrl={avatarOf(p)} pinned={pinnedId === p.id} onPin={pin} />
          ))}
          {arrangement.more > 0 && <div className="cs-more-people">{`${arrangement.more} người khác`}</div>}
        </div>
      </div>
    )
  } else {
    const cols = panel ? 2 : arrangement.cols
    const perPage = panel ? 6 : arrangement.perPage
    const total = pageCount(arrangement.tiles.length, perPage)
    const at = Math.min(page, total - 1)
    body = (
      <div className={`cs-grid cs-cols-${cols}`}>
        {pageOf(arrangement.tiles, at, perPage).map((p) => (
          <ParticipantTile
            key={p.id} person={p} longSelf avatarUrl={avatarOf(p)} pinned={pinnedId === p.id} onPin={pin}
            avatarSize={64}
          />
        ))}
        {total > 1 && (
          <div className="cs-pager">
            <IconButton aria-label="Trang trước" disabled={at === 0} onClick={() => setPage(at - 1)}><ChevronLeft size={18} /></IconButton>
            <span className="cs-pager-text" aria-live="polite">{`${at + 1}/${total}`}</span>
            <IconButton aria-label="Trang sau" disabled={at >= total - 1} onClick={() => setPage(at + 1)}><ChevronRight size={18} /></IconButton>
          </div>
        )}
      </div>
    )
  }

  const problem = snapshot.mediaProblem
  const notice = snapshot.notice

  return (
    <div className={`cs-stage ${panel ? 'cs-with-panel' : ''}`} data-testid="call-stage">
      <div className="cs-top">
        <b>{session.title}</b>
        <span className="cs-time">{clock}</span>
        <span className="cs-chip" aria-label={`${snapshot.participants.length} người trong cuộc gọi`}>
          <Users className="cs-icon-xs" aria-hidden="true" />{snapshot.participants.length}
        </span>
        <span className="cs-grow" />
        {selfPoor && <span className="cs-chip cs-chip-warn"><Signal className="cs-icon-xs" aria-hidden="true" />Mạng yếu</span>}
        {!session.isDm && <span className="cs-chip"><Lock className="cs-icon-xs" aria-hidden="true" />{hasChat ? 'Chỉ thành viên nhóm' : 'Chỉ khách của sự kiện'}</span>}
        <LayoutMenu layout={layout} onChange={store.setLayout} />
      </div>

      {snapshot.sharing && (
        <div className="cs-prebar">
          <span>Bạn đang trình bày</span>
          <Pressable onClick={() => void callRoom.stopShare()}>Dừng trình bày</Pressable>
        </div>
      )}

      {body}

      {panel && (
        <CallSidePanel
          panel={panel} onPanel={store.setPanel} channelId={session.channelId} callId={session.callId}
          title={session.title} isDm={session.isDm} call={call} snapshot={snapshot}
        />
      )}

      <CallControls
        callRoom={callRoom} snapshot={snapshot} title={session.title} clock={clock} panel={panel} onPanel={store.setPanel} hasChat={hasChat}
        newMessages={newMessages} handsUp={handsUp} otherPresenter={otherPresenter}
        onLeave={onLeave} onOpenDevices={() => store.setDevicesOpen(true)}
      />

      <ReactionsLayer reactions={snapshot.reactions} />

      {(problem || notice) && (
        <div className="cs-notice" role="status">
          <span>{notice ? `${notice.name} đã bắt đầu trình bày. Bản trình bày của bạn đã dừng.` : PROBLEM_COPY[problem!]}</span>
          <Pressable onClick={() => (notice ? callRoom.dismissNotice() : callRoom.dismissProblem())}>Đóng</Pressable>
        </div>
      )}

      {snapshot.audioBlocked && <AudioBlockedButton onTap={() => void callRoom.enableAudio()} />}
      {snapshot.link === 'reconnecting' && <ReconnectingOverlay onLeave={onLeave} />}

      <DeviceSettings open={devicesOpen} onClose={() => store.setDevicesOpen(false)} callRoom={callRoom} userId={user?.id} />
    </div>
  )
}
